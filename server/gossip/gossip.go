package gossip

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/buildbuddy-io/buildbuddy/server/hostid"
	"github.com/buildbuddy-io/buildbuddy/server/interfaces"
	"github.com/buildbuddy-io/buildbuddy/server/real_environment"
	"github.com/buildbuddy-io/buildbuddy/server/util/flag"
	"github.com/buildbuddy-io/buildbuddy/server/util/log"
	"github.com/buildbuddy-io/buildbuddy/server/util/network"
	"github.com/buildbuddy-io/buildbuddy/server/util/retry"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"
	"github.com/buildbuddy-io/buildbuddy/server/util/statusz"
	"github.com/rs/zerolog"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
)

var (
	listenAddr     = flag.String("gossip.listen_addr", "", "The address to listen for gossip traffic on. Ex. 'localhost:1991'")
	join           = flag.Slice("gossip.join", []string{}, "The nodes to join/gossip with. Ex. '1.2.3.4:1991,2.3.4.5:1991...'")
	nodeName       = flag.String("gossip.node_name", "", "The gossip node's name. If empty will default to host_id.'")
	secretKey      = flag.String("gossip.secret_key", "", "The value should be either 16, 24, or 32 bytes.")
	retransmitMult = flag.Int("gossip.retransmit_mult", 0, "The retransmit multiplier for a failed broadcast over gossip. If zero, use the default value")
	logLevel       = flag.String("gossip.log_level", "", "The desired log level for the gossip package. If empty, this flag will be ignored and app.log_level will be applied. Logs with a level >= this level will be emitted. One of {'fatal', 'error', 'warn', 'info', 'debug', ''}")
)

var (
	// Logs that are not that useful.
	excludedWarningLog = []string{
		"serf: received old query registry_query_event from time",
		"serf: reply for non-running query",
		"serf: received old event",
		"serf: received old query",
	}
)

// A GossipManager will listen (on `advertiseAddress`), connect to `seeds`,
// and gossip any information provided via the broker interface. To leave
// gracefully, clients should call GossipManager.Leave() followed by
// GossipManager.Shutdown().
type GossipManager struct {
	cancelFunc    context.CancelFunc
	serfInstance  *serf.Serf
	serfEventChan chan serf.Event

	listenAddr string
	join       []string

	mu        sync.Mutex
	listeners []interfaces.GossipListener
	tags      map[string]string
}

func (gm *GossipManager) processEvents() {
	for {
		event := <-gm.serfEventChan
		gm.mu.Lock()
		listeners := gm.listeners
		gm.mu.Unlock()

		for _, listener := range listeners {
			listener.OnEvent(event.EventType(), event)
		}
	}
}

func (gm *GossipManager) ListenAddr() string {
	return gm.listenAddr
}

func (gm *GossipManager) JoinList() []string {
	joinList := make([]string, len(gm.join))
	copy(joinList, gm.join)
	return joinList
}

func (gm *GossipManager) AddListener(listener interfaces.GossipListener) {
	if listener == nil {
		log.Error("listener cannot be nil")
		return
	}
	// The listener may be added after the gossip manager has already been
	// started, so notify it of any already connected nodes.
	existingMembersEvent := serf.MemberEvent{
		Type:    serf.EventMemberUpdate,
		Members: gm.serfInstance.Members(),
	}
	listener.OnEvent(existingMembersEvent.Type, existingMembersEvent)
	gm.mu.Lock()
	gm.listeners = append(gm.listeners, listener)
	gm.mu.Unlock()
}

func (gm *GossipManager) LocalMember() serf.Member {
	return gm.serfInstance.LocalMember()
}
func (gm *GossipManager) Members() []serf.Member {
	return gm.serfInstance.Members()
}
func (gm *GossipManager) Leave() error {
	return gm.serfInstance.Leave()
}
func (gm *GossipManager) Shutdown() error {
	gm.cancelFunc()
	return gm.serfInstance.Shutdown()
}
func (gm *GossipManager) SetTags(tags map[string]string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	for tagName, tagValue := range tags {
		if tagValue == "" {
			delete(gm.tags, tagName)
		} else {
			gm.tags[tagName] = tagValue
		}
	}
	return gm.serfInstance.SetTags(gm.tags)
}

func (gm *GossipManager) getTags() map[string]string {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	rmap := make(map[string]string, len(gm.tags))
	for tagName, tagValue := range gm.tags {
		rmap[tagName] = tagValue
	}
	return rmap
}

func (gm *GossipManager) SendUserEvent(name string, payload []byte, coalesce bool) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	return gm.serfInstance.UserEvent(name, payload, coalesce)
}

func (gm *GossipManager) Query(name string, payload []byte, params *serf.QueryParam) (*serf.QueryResponse, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	return gm.serfInstance.Query(name, payload, params)
}

func formatMember(m serf.Member) string {
	return fmt.Sprintf("Name: %s Addr: %s:%d Status: %+v", m.Name, m.Addr.String(), m.Port, m.Status)
}

func (gm *GossipManager) Statusz(ctx context.Context) string {
	buf := "<pre>"
	thisNode := gm.LocalMember()
	buf += fmt.Sprintf("Node: %+v\n", formatMember(thisNode))

	buf += "Tags:\n"
	tagStrings := make([]string, len(gm.getTags()))
	for tagKey, tagValue := range gm.getTags() {
		tagStrings = append(tagStrings, fmt.Sprintf("\t%q => %q\n", tagKey, tagValue))
	}
	sort.Strings(tagStrings)
	for _, tagString := range tagStrings {
		buf += tagString
	}

	buf += "Peers:\n"
	peers := gm.Members()
	sort.Slice(peers, func(i, j int) bool { return peers[i].Name < peers[j].Name })
	for _, peerMember := range peers {
		if peerMember.Name == thisNode.Name {
			continue
		}
		buf += fmt.Sprintf("\t%s\n", formatMember(peerMember))
	}

	buf += "Stats:\n"
	var statStrings []string
	for k, v := range gm.serfInstance.Stats() {
		statStrings = append(statStrings, fmt.Sprintf("\t%s: %s\n", k, v))
	}
	sort.Strings(statStrings)
	for _, statString := range statStrings {
		buf += statString
	}
	buf += "</pre>"
	return buf
}

// Adapt our log writer into one that is compatible with
// serf.
type logWriter struct {
	log.Logger
}

func (lw *logWriter) Write(d []byte) (int, error) {
	s := strings.TrimSuffix(string(d), "\n")
	// Gossip logs are very verbose and there is
	// very little useful info in DEBUG/INFO level logs.
	if strings.Contains(s, "[DEBUG]") {
		lw.Logger.Debug(s)
	} else if strings.Contains(s, "[INFO]") {
		// Excludes "EventMemberUpdate", because it's verbose and not that useful.
		if strings.Contains(s, "EventMemberUpdate") {
			return 0, nil
		}
		lw.Logger.Info(s)
	} else {
		for _, excluded := range excludedWarningLog {
			if strings.Contains(s, excluded) {
				return 0, nil
			}

		}
		lw.Logger.Warning(s)
	}

	return len(d), nil
}

func Register(env *real_environment.RealEnv) error {
	if *listenAddr == "" {
		return nil
	}
	if len(*join) == 0 {
		return status.FailedPreconditionError("Gossip listen address specified but no join target set")
	}
	name := *nodeName
	if name == "" {
		name = hostid.GetFailsafeHostID("")
	}

	// Initialize a gossip manager, which will contact other nodes
	// and exchange information.
	gossipManager, err := New(name, *listenAddr, *join)
	if err != nil {
		return err
	}
	env.SetGossipService(gossipManager)
	return nil
}

func New(nodeName, listenAddress string, join []string) (*GossipManager, error) {
	subLog := log.NamedSubLogger(fmt.Sprintf("GossipManager(%s)", nodeName))
	if *logLevel != "" {
		if l, err := zerolog.ParseLevel(*logLevel); err != nil {
			return nil, err
		} else {
			subLog = subLog.Level(l)
		}
	}
	log.Infof("Starting GossipManager on %q", listenAddress)

	bindAddr, bindPort, err := network.ParseAddress(listenAddress)
	if err != nil {
		return nil, err
	}
	memberlistConfig := memberlist.DefaultLANConfig()
	memberlistConfig.BindAddr = bindAddr
	memberlistConfig.BindPort = bindPort
	if mult := *retransmitMult; mult != 0 {
		memberlistConfig.RetransmitMult = mult
	}
	memberlistConfig.LogOutput = &logWriter{subLog}
	if *secretKey != "" {
		memberlistConfig.SecretKey = []byte(*secretKey)
	}

	serfConfig := serf.DefaultConfig()
	serfConfig.NodeName = nodeName
	serfConfig.MemberlistConfig = memberlistConfig
	serfConfig.LogOutput = &logWriter{subLog}
	// this is the maximum value that serf supports.
	serfConfig.UserEventSizeLimit = 9 * 1024
	serfConfig.BroadcastTimeout = time.Second

	serfConfig.CoalescePeriod = 10 * time.Second
	serfConfig.QuiescentPeriod = time.Second

	serfConfig.UserCoalescePeriod = 10 * time.Second
	serfConfig.UserQuiescentPeriod = time.Second

	ctx, cancel := context.WithCancel(context.TODO())

	// spoiler: gossip girl was actually a:
	gossipMan := &GossipManager{
		cancelFunc:    cancel,
		listenAddr:    listenAddress,
		join:          join,
		serfEventChan: make(chan serf.Event, 16),
		mu:            sync.Mutex{},
		listeners:     make([]interfaces.GossipListener, 0),
		tags:          make(map[string]string, 0),
	}
	serfConfig.EventCh = gossipMan.serfEventChan
	go gossipMan.processEvents()

	serfInstance, err := serf.Create(serfConfig)
	if err != nil {
		return nil, err
	}

	otherNodes := make([]string, 0, len(join))
	for _, node := range join {
		if node != listenAddress {
			otherNodes = append(otherNodes, node)
		}
	}
	if len(otherNodes) > 0 {
		wg := sync.WaitGroup{}
		wg.Add(1)

		go func() {
			retrier := retry.New(ctx, &retry.Options{
				InitialBackoff: 10 * time.Second,
				MaxBackoff:     180 * time.Second,
				Multiplier:     2,
			})
			once := sync.Once{}
			for retrier.Next() {
				log.Debugf("I am %q, attempting to join %+v", listenAddress, otherNodes)
				_, err := serfInstance.Join(otherNodes, false)
				once.Do(wg.Done)
				if err == nil {
					return
				}
				log.Debugf("Join failed: %s", err)
			}
			log.Warningf("Gossip: %q failed to join other nodes: %+v", listenAddress, otherNodes)
		}()
		wg.Wait()
	}
	gossipMan.serfInstance = serfInstance
	statusz.AddSection("gossip_manager", "Serf Gossip Network", gossipMan)
	return gossipMan, nil
}
