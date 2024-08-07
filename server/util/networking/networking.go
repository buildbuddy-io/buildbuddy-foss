package networking

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/buildbuddy-io/buildbuddy/server/util/alert"
	"github.com/buildbuddy-io/buildbuddy/server/util/background"
	"github.com/buildbuddy-io/buildbuddy/server/util/log"
	"github.com/buildbuddy-io/buildbuddy/server/util/random"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"
	"github.com/buildbuddy-io/buildbuddy/server/util/uuid"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

var (
	routePrefix                   = flag.String("executor.route_prefix", defaultRoute, "The prefix in the ip route to locate a device: either 'default' or the ip range of the subnet e.g. 172.24.0.0/18")
	blackholePrivateRanges        = flag.Bool("executor.blackhole_private_ranges", false, "If true, no traffic will be allowed to RFC1918 ranges.")
	preserveExistingNetNamespaces = flag.Bool("executor.preserve_existing_netns", false, "Preserve existing bb-executor net namespaces. By default all \"bb-executor\" net namespaces are removed on executor startup, but if multiple executors are running on the same machine this behavior should be disabled to prevent them interfering with each other.")

	// Private IP ranges, as defined in RFC1918.
	PrivateIPRanges = []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16"}
)

const (
	defaultRoute         = "default"
	routingTableFilename = "/etc/iproute2/rt_tables"
	// The routingTableID for the new routing table we add.
	routingTableID = 1
	// The routingTableName for the new routing table we add.
	routingTableName = "rt1"
	// netns prefix to use to identify executor namespaces.
	netNamespacePrefix = "bb-executor-"
	// Total number of available host IP ranges that can be allocated to VMs.
	numAssignableNetworks = 1000
	// Time to allow for networking cleanup. We intentionally use a long-ish
	// timeout here because if cleanup fails then we might leave the network in
	// a bad state, preventing a host IP from being usable.
	networkingCleanupTimeout = 1 * time.Minute

	// CIDR suffix for veth-based networks. We only need 2 IP addresses, one for
	// the host end and one for the namespaced end.
	cidrSuffix = "/30"

	// CIDR matching all container networks on the host.
	containerNetworkingCIDR = "192.168.0.0/16"
)

// runCommand runs the provided command, prepending sudo if the calling user is
// not already root. Output and errors are returned.
func sudoCommand(ctx context.Context, args ...string) ([]byte, error) {
	// If we're not running as root, use sudo.
	// Use "-A" to ensure we never get stuck prompting for
	// a password interactively.
	if unix.Getuid() != 0 {
		args = append([]string{"sudo", "-A"}, args...)
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, status.InternalErrorf("run %q: %s: %s", cmd, err, string(out))
	}
	return out, nil
}

// runCommand runs the provided command, prepending sudo if the calling user is
// not already root, and returns any error encountered.
func runCommand(ctx context.Context, args ...string) error {
	_, err := sudoCommand(ctx, args...)
	return err
}

// namespace prepends the provided command with 'ip netns exec "netNamespace"'
// so that the provided command is run inside the network namespace.
func namespace(netNamespace string, args ...string) []string {
	return append([]string{"ip", "netns", "exec", NetNamespace(netNamespace)}, args...)
}

// Returns the full network namespace of the provided network namespace ID.
func NetNamespace(netNamespaceId string) string {
	return netNamespacePrefix + netNamespaceId
}

// Returns the full filesystem path of the provided network namespace ID.
func NetNamespacePath(netNamespaceId string) string {
	return "/var/run/netns/" + NetNamespace(netNamespaceId)
}

// Deletes all of the executor net namespaces. These can be left behind if the
// executor doesn't exit gracefully.
func DeleteNetNamespaces(ctx context.Context) error {
	// "ip netns delete" doesn't support patterns, so we list all
	// namespaces then delete the ones that match the bb executor pattern.
	b, err := sudoCommand(ctx, "ip", "netns", "list")
	if err != nil {
		return err
	}
	output := strings.TrimSpace(string(b))
	if len(output) == 0 {
		return nil
	}
	var lastErr error
	for _, ns := range strings.Split(output, "\n") {
		// Sometimes the output contains spaces, like
		//     bb-executor-1
		//     bb-executor-2
		//     3fe4313e-eb76-4b6d-9d61-53caf12b87e6 (id: 344)
		//     2ab15e85-d1c3-47bc-ad40-74e2941157a4 (id: 332)
		// So we get just the first column here.
		fields := strings.Fields(ns)
		if len(fields) > 0 {
			ns = fields[0]
		}
		if !strings.HasPrefix(ns, netNamespacePrefix) {
			continue
		}
		if _, err := sudoCommand(ctx, "ip", "netns", "delete", ns); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// CreateNetNamespace is equivalent to:
//
//	$ sudo ip netns add "netNamespace"
func CreateNetNamespace(ctx context.Context, netNamespace string) error {
	return runCommand(ctx, "ip", "netns", "add", NetNamespace(netNamespace))
}

// CreateTapInNamespace is equivalent to:
//
//	$ sudo ip netns exec "netNamespace" ip tuntap add name "tapName" mode tap
func CreateTapInNamespace(ctx context.Context, netNamespace, tapName string) error {
	return runCommand(ctx, namespace(netNamespace, "ip", "tuntap", "add", "name", tapName, "mode", "tap")...)
}

// ConfigureTapInNamespace is equivalent to:
//
//	$ sudo ip netns exec "netNamespace" ip addr add "address" dev "tapName"
func ConfigureTapInNamespace(ctx context.Context, netNamespace, tapName, tapAddr string) error {
	return runCommand(ctx, namespace(netNamespace, "ip", "addr", "add", tapAddr, "dev", tapName)...)
}

// BringUpTapInNamespace is equivalent to:
//
//	$ sudo ip netns exec "netNamespace" ip link set "tapName" up
func BringUpTapInNamespace(ctx context.Context, netNamespace, tapName string) error {
	return runCommand(ctx, namespace(netNamespace, "ip", "link", "set", tapName, "up")...)
}

// randomVethName picks a random veth name like "veth0cD42A"
func randomVethName(prefix string) (string, error) {
	suffix, err := random.RandomString(5)
	if err != nil {
		return "", err
	}
	return prefix + suffix, nil
}

// createRandomVethPair attempts to create a veth pair with random names, the veth1 end of which will
// be in the root namespace.
func createRandomVethPair(ctx context.Context, netNamespace string) (string, string, error) {
	// compute unique veth names
	var veth0, veth1 string
	var err error
	for i := 0; i < 100; i++ {
		veth0, err = randomVethName("veth0")
		if err != nil {
			break
		}
		veth1, err = randomVethName("veth1")
		if err != nil {
			break
		}
		err = runCommand(ctx, namespace(netNamespace, "ip", "link", "add", veth1, "type", "veth", "peer", "name", veth0)...)
		if err != nil {
			if strings.Contains(err.Error(), "File exists") {
				continue
			}
			return "", "", err
		}
		break
	}
	//# move the veth1 end of the pair into the root namespace
	//sudo ip netns exec fc0 ip link set veth1 netns 1
	if err == nil {
		err = runCommand(ctx, namespace(netNamespace, "ip", "link", "set", veth1, "netns", "1")...)
	}
	return veth0, veth1, err
}

func attachAddressToVeth(ctx context.Context, netNamespace, ipAddr, vethName string) error {
	if netNamespace != "" {
		return runCommand(ctx, namespace(netNamespace, "ip", "addr", "add", ipAddr, "dev", vethName)...)
	} else {
		return runCommand(ctx, "ip", "addr", "add", ipAddr, "dev", vethName)
	}
}

func RemoveNetNamespace(ctx context.Context, netNamespace string) error {
	// This will delete the veth pair too, and the address attached
	// to the host-size of the veth pair.
	return runCommand(ctx, "ip", "netns", "delete", NetNamespace(netNamespace))
}

// HostNetAllocator assigns unique /30 networks from the host for use in VMs.
type HostNetAllocator struct {
	mu sync.Mutex
	// Next index to try locking; wraps around at numAssignableNetworks.
	// Since most tasks are short-lived, this usually will point to an index
	// that is not currently in use.
	idx   int
	inUse [numAssignableNetworks]bool
}

var hostNetAllocator = &HostNetAllocator{}

// HostNet represents a reserved /30 network from the host for use in a VM.
type HostNet struct {
	netIdx int
	unlock func()
}

func (n *HostNet) CIDR() string {
	return fmt.Sprintf("192.168.%d.%d", n.netIdx/30, (n.netIdx%30)*8+4) + cidrSuffix
}

func (n *HostNet) HostIP() string {
	return fmt.Sprintf("192.168.%d.%d", n.netIdx/30, (n.netIdx%30)*8+5)
}

func (n *HostNet) HostIPWithCIDR() string {
	return n.HostIP() + cidrSuffix
}

func (n *HostNet) NamespacedIP() string {
	return fmt.Sprintf("192.168.%d.%d", n.netIdx/30, ((n.netIdx%30)*8)+6)
}

func (n *HostNet) NamespacedIPWithCIDR() string {
	return n.NamespacedIP() + cidrSuffix
}

func (n *HostNet) Unlock() {
	n.unlock()
}

// Get assigns a host network IP for the given VM index.
func (a *HostNetAllocator) Get() (*HostNet, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for attempt := 0; attempt < numAssignableNetworks; attempt++ {
		netIdx := a.idx
		a.idx = (a.idx + 1) % numAssignableNetworks
		if a.inUse[netIdx] {
			continue
		}
		a.inUse[netIdx] = true
		return &HostNet{
			netIdx: netIdx,
			unlock: func() { a.unlock(netIdx) },
		}, nil
	}
	return nil, status.ResourceExhaustedError("host IP address space exhausted")
}

func (a *HostNetAllocator) unlock(netIdx int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.inUse[netIdx] = false
}

// VethPair represents a veth pair with one end in the host and the other end
// in a namespace.
type VethPair struct {
	// hostDevice is the name of the end of the veth pair which is in the
	// root net namespace.
	hostDevice string

	// namespacedDevice is the name of the end of the veth pair which is in the
	// namespace.
	namespacedDevice string

	// The net namespace name.
	netNamespace string

	// Network information for the veth pair.
	network *HostNet

	// Cleanup deletes the veth pair and associated host IP configuration
	// changes.
	Cleanup func(ctx context.Context) error
}

// SetupVethPair creates a new veth pair with one end in the given network
// namespace and the other end in the root namespace. It returns a cleanup
// function that removes firewall rules associated with the pair.
//
// It is equivalent to:
//
//	# create a new veth pair
//	$ sudo ip netns exec fc0 ip link add veth1 type veth peer name veth0
//
//	# move the veth1 end of the pair into the root namespace
//	$ sudo ip netns exec fc0 ip link set veth1 netns 1
//
//	# add the ip addr 10.0.0.2/24 to the veth0 end of the pair
//	$ sudo ip netns exec fc0 ip addr add 10.0.0.2/24 dev veth0
//
//	# bring the veth0 end of the pair up
//	$ sudo ip netns exec fc0 ip link set dev veth0 up
//
//	# add the ip addr 10.0.0.1/24 to the veth1 end of the pair
//	$ sudo ip addr add 10.0.0.1/24 dev veth1
//
//	# add a firewall rule to allow forwarding traffic from the veth1 pair to the
//	# default device, in case forwarding is not allowed by default
//	$ sudo iptables -A FORWARD -i veth1 -o eth0 -j ACCEPT
//
//	# bring the veth1 end of the pair up
//	$ sudo ip link set dev veth1 up
//
//	# add a default route in the namespace to use 10.0.0.1 (aka the veth1 end of the pair)
//	$ sudo ip netns exec fc0 ip route add default via 10.0.0.1
//
//	# add an iptables mapping inside the namespace to rewrite 192.168.241.2 -> 192.168.0.3
//	$ sudo ip netns exec fc0 iptables -t nat -A POSTROUTING -o veth0 -s 192.168.241.2 -j SNAT --to 192.168.0.3
//
//	# add an iptables mapping inside the namespace to rewrite 192.168.0.3 -> 192.168.241.2
//	$ sudo ip netns exec fc0 iptables -t nat -A PREROUTING -i veth0 -d 192.168.0.3 -j DNAT --to 192.168.241.2
//
//	# add a route in the root namespace so that traffic to 192.168.0.3 hits 10.0.0.2, the veth0 end of the pair
//	$ sudo ip route add 192.168.0.3 via 10.0.0.2
func SetupVethPair(ctx context.Context, netNamespace string) (_ *VethPair, err error) {
	// Keep a list of cleanup work to be done.
	var cleanupStack cleanupStack
	// If we return an error from this func then we need to clean up any
	// resources that were created before returning.
	defer func() {
		if err != nil {
			_ = cleanupStack.Cleanup(ctx)
		}
	}()

	r, err := findRoute(*routePrefix)
	if err != nil {
		return nil, err
	}
	device := r.device

	// compute unique addresses for endpoints
	veth0, veth1, err := createRandomVethPair(ctx, netNamespace)
	if err != nil {
		return nil, err
	}
	// Deleting the net namespace deletes veth1 automatically, but it does so
	// asynchronously. To avoid race conditions, we delete it manually here to
	// ensure that the cleanup happens before we release the lock on the
	// host IP range.
	cleanupStack = append(cleanupStack, func(ctx context.Context) error {
		return runCommand(ctx, "ip", "link", "delete", veth1)
	})

	// This addr will be used for the host-side of the veth pair, so it
	// needs to to be unique on the host.
	network, err := hostNetAllocator.Get()
	if err != nil {
		return nil, status.WrapError(err, "assign host network to VM")
	}
	cleanupStack = append(cleanupStack, func(ctx context.Context) error {
		network.Unlock()
		return nil
	})

	// Attach IP addresses to the host and namespaced ends of the veth pair,
	// and bring them up.
	err = attachAddressToVeth(ctx, netNamespace, network.NamespacedIPWithCIDR(), veth0)
	if err != nil {
		return nil, status.WrapError(err, "attach address to veth device in namespace")
	}
	err = runCommand(ctx, namespace(netNamespace, "ip", "link", "set", "dev", veth0, "up")...)
	if err != nil {
		return nil, err
	}
	err = attachAddressToVeth(ctx, "" /*no namespace*/, network.HostIPWithCIDR(), veth1)
	if err != nil {
		return nil, err
	}
	err = runCommand(ctx, "ip", "link", "set", "dev", veth1, "up")
	if err != nil {
		return nil, err
	}

	// Route traffic via the host end of the pair by default.
	err = runCommand(ctx, namespace(netNamespace, "ip", "route", "add", "default", "via", network.HostIP())...)
	if err != nil {
		return nil, status.WrapError(err, "add default route in namespace")
	}

	if IsSecondaryNetworkEnabled() {
		err = runCommand(ctx, "ip", "rule", "add", "from", network.NamespacedIP(), "lookup", routingTableName)
		if err != nil {
			return nil, err
		}
		cleanupStack = append(cleanupStack, func(ctx context.Context) error {
			return runCommand(ctx, "ip", "rule", "del", "from", network.NamespacedIP())
		})
	}

	iptablesRules := [][]string{
		// Allow forwarding traffic between the host side of the veth pair and
		// the device associated with the configured route prefix (usually the
		// default route). This is necessary on hosts with default-deny policies
		// in place.
		{"FORWARD", "-i", veth1, "-o", device, "-j", "ACCEPT"},
		{"FORWARD", "-i", device, "-o", veth1, "-j", "ACCEPT"},

		// Drop any traffic from the namespace that is targeting another
		// namespace.
		{"FORWARD", "-s", network.CIDR(), "-d", containerNetworkingCIDR, "-j", "DROP"},
	}

	for _, rule := range iptablesRules {
		if err := runCommand(ctx, append([]string{"iptables", "--wait", "-A"}, rule...)...); err != nil {
			return nil, err
		}
		cleanupStack = append(cleanupStack, func(ctx context.Context) error {
			return runCommand(ctx, append([]string{"iptables", "--wait", "--delete"}, rule...)...)
		})
	}

	return &VethPair{
		hostDevice:       veth1,
		namespacedDevice: veth0,
		netNamespace:     netNamespace,
		network:          network,
		Cleanup:          cleanupStack.Cleanup,
	}, nil
}

// List of cleanup tasks which should be executed in the reverse order in which
// the corresponding resources were created.
type cleanupStack []func(ctx context.Context) error

func (s cleanupStack) Cleanup(ctx context.Context) error {
	ctx, cancel := background.ExtendContextForFinalization(ctx, networkingCleanupTimeout)
	defer cancel()
	// Pop and run cleanup funcs from the stack until empty.
	for len(s) > 0 {
		f := s[len(s)-1]
		s = s[:len(s)-1]
		if err := f(ctx); err != nil {
			// Short-circuit on the first error.
			alert.UnexpectedEvent("network_cleanup_failed", "Networking cleanup failed. If too many of these errors accumulate, networking may stop functioning correctly. Error: %s", err)
			return err
		}
	}
	return nil
}

// ConfigureNATForTapInNamespace configures NAT in the namespace so that
// outgoing IP packets from the tap device in the namespace are translated to
// the namespaced veth device IP, and incoming IP packets to the tap device are
// translated to the VM tap device IP.
//
// Since the host-side of the veth pair also has NAT configured, this allows the
// VM to communicate with external networks.
func ConfigureNATForTapInNamespace(ctx context.Context, vethPair *VethPair, vmIP string) error {
	if err := runCommand(ctx, namespace(vethPair.netNamespace, "iptables", "--wait", "-t", "nat", "-A", "POSTROUTING", "-o", vethPair.namespacedDevice, "-s", vmIP, "-j", "SNAT", "--to", vethPair.network.NamespacedIP())...); err != nil {
		return err
	}
	if err := runCommand(ctx, namespace(vethPair.netNamespace, "iptables", "--wait", "-t", "nat", "-A", "PREROUTING", "-i", vethPair.namespacedDevice, "-d", vethPair.network.NamespacedIP(), "-j", "DNAT", "--to", vmIP)...); err != nil {
		return err
	}
	return nil
}

// ContainerNetwork represents a fully-provisioned container network, which
// consists of a net namespace and associated host configuration.
//
// Deleting a container network deletes the net namespace as well as all
// associated resources, and reverts the applied host configuration.
type ContainerNetwork struct {
	netns    string
	vethPair *VethPair
	cleanup  func(ctx context.Context) error
}

// CreateContainerNetwork initializes a network namespace, networking
// interfaces, and host configuration required for container networking.
//
// If loopbackOnly is true, only a loopback interface will be created in the
// namespace, and the container will not be able to reach external addresses.
func CreateContainerNetwork(ctx context.Context, loopbackOnly bool) (_ *ContainerNetwork, err error) {
	var cleanupStack cleanupStack
	defer func() {
		// If we failed to fully set up the network, make sure to clean up any
		// resources that were partially set up.
		if err != nil {
			_ = cleanupStack.Cleanup(ctx)
		}
	}()

	// Create a net namespace.
	nsid := uuid.New()
	if err := CreateNetNamespace(ctx, nsid); err != nil {
		return nil, status.WrapError(err, "create net namespace")
	}
	cleanupStack = append(cleanupStack, func(ctx context.Context) error {
		return RemoveNetNamespace(ctx, nsid)
	})

	// Bring up the loopback device in the namespace.
	if err := runCommand(ctx, namespace(nsid, "ip", "link", "set", "lo", "up")...); err != nil {
		return nil, status.WrapError(err, "bring up loopback device")
	}

	var vethPair *VethPair
	if !loopbackOnly {
		// Create a veth pair with one end in the namespace.
		vp, err := SetupVethPair(ctx, nsid)
		if err != nil {
			return nil, status.WrapError(err, "setup veth pair")
		}
		cleanupStack = append(cleanupStack, vp.Cleanup)
		vethPair = vp
	}

	return &ContainerNetwork{
		netns:    NetNamespace(nsid),
		vethPair: vethPair,
		cleanup:  cleanupStack.Cleanup,
	}, nil
}

func (c *ContainerNetwork) NetNamespace() string {
	return c.netns
}

// HostNetwork returns the externally-connected network routed through the host.
// Returns nil if this is a loopback-only network.
func (c *ContainerNetwork) HostNetwork() *HostNet {
	if c.vethPair == nil {
		return nil
	}
	return c.vethPair.network
}

func (c *ContainerNetwork) Cleanup(ctx context.Context) error {
	return c.cleanup(ctx)
}

// DefaultIP returns the IPv4 address for the primary network.
func DefaultIP(ctx context.Context) (net.IP, error) {
	r, err := findRoute(defaultRoute)
	if err != nil {
		return nil, err
	}
	device := r.device
	return ipFromDevice(ctx, device)
}

// ipFromDevice return IPv4 address for the device.
func ipFromDevice(ctx context.Context, device string) (net.IP, error) {
	netIface, err := interfaceFromDevice(ctx, device)
	if err != nil {
		return nil, err
	}
	addrs, err := netIface.Addrs()
	if err != nil {
		return nil, err
	}
	for _, a := range addrs {
		if v, ok := a.(*net.IPNet); ok {
			if ipv4Addr := v.IP.To4(); ipv4Addr != nil {
				return ipv4Addr, nil
			}
		}
	}
	return nil, fmt.Errorf("could not determine IP for device %q", device)
}

func interfaceFromDevice(ctx context.Context, device string) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		if iface.Name == device {
			return &iface, nil
		}
	}
	return nil, status.NotFoundErrorf("could not find interface %q", device)
}

type route struct {
	device  string
	gateway net.IP
	// Source IP for the route. May be nil.
	src net.IP
}

// findRoute finds the highest priority route with the given destination.
// Destination may be defaultRoute to find a default route.
func findRoute(destination string) (route, error) {
	// Get all routes.
	rs, err := netlink.RouteList(nil, 0)
	if err != nil {
		return route{}, status.UnknownErrorf("could not get ip routes: %s", err)
	}

	var targetDst *net.IPNet
	if destination != defaultRoute {
		_, targetDst, err = net.ParseCIDR(destination)
		if err != nil {
			return route{}, status.InvalidArgumentErrorf("could not parse destination: %s", err)
		}
	}

	for _, r := range rs {
		if targetDst.String() == r.Dst.String() {
			l, err := netlink.LinkByIndex(r.LinkIndex)
			if err != nil {
				return route{}, status.UnknownErrorf("could not lookup interface for route: %s", err)
			}
			return route{
				device:  l.Attrs().Name,
				gateway: r.Gw,
				src:     r.Src,
			}, nil
		}
	}

	return route{}, status.FailedPreconditionErrorf("Unable to determine device with prefix: %s", destination)
}

// EnableMasquerading turns on ipmasq for the device with --device_prefix. This is required
// for networking to work on vms.
func EnableMasquerading(ctx context.Context) error {
	route, err := findRoute(*routePrefix)
	if err != nil {
		return err
	}
	device := route.device
	// Skip appending the rule if it's already in the table.
	err = runCommand(ctx, "iptables", "--wait", "-t", "nat", "--check", "POSTROUTING", "-o", device, "-j", "MASQUERADE")
	if err == nil {
		return nil
	}
	return runCommand(ctx, "iptables", "--wait", "-t", "nat", "-A", "POSTROUTING", "-o", device, "-j", "MASQUERADE")
}

// AddRoutingTableEntryIfNotPresent adds [tableID, tableName] name pair to /etc/iproute2/rt_tables if
// the pair is not present.
// Equilvalent to 'echo "1 rt1" | sudo tee -a /etc/iproute2/rt_tables'.
func addRoutingTableEntryIfNotPresent(ctx context.Context) error {
	tableEntry := fmt.Sprintf("%d %s", routingTableID, routingTableName)
	exists, err := routingTableContainsTable(tableEntry)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	f, err := os.OpenFile(routingTableFilename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return status.InternalErrorf("failed to add routing table %s: %s", routingTableName, err)
	}
	defer f.Close()
	if _, err = f.WriteString(tableEntry + "\n"); err != nil {
		return status.InternalErrorf("failed to add routing table %s: %s", routingTableName, err)
	}
	return nil
}

// routingTableContainTable checks if /etc/iproute2/rt_tables contains <tableEntry>.
func routingTableContainsTable(tableEntry string) (bool, error) {
	file, err := os.Open(routingTableFilename)
	if err != nil {
		return false, status.InternalErrorf("failed to open routing table: %s", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == tableEntry {
			return true, nil
		}
	}
	return false, nil
}

func ConfigurePrivateRangeBlackholing(ctx context.Context, sourceRange string) error {
	for _, r := range PrivateIPRanges {
		if err := runCommand(ctx, "iptables", "--wait", "-I", "FORWARD", "-s", sourceRange, "-d", r, "-j", "DROP"); err != nil {
			return err
		}
	}
	return nil
}

// ConfigureRoutingForIsolation sets up a routing table for handling network
// isolation via either a secondary network interface or blackholing.
func ConfigureRoutingForIsolation(ctx context.Context) error {
	if !IsSecondaryNetworkEnabled() && !IsPrivateRangeBlackholingEnabled() {
		// No need to add IP rule when we don't use secondary network
		return nil
	}

	if IsSecondaryNetworkEnabled() {
		// Adds a new routing table
		if err := addRoutingTableEntryIfNotPresent(ctx); err != nil {
			return err
		}
		return configurePolicyBasedRoutingForSecondaryNetwork(ctx)
	} else {
		if err := ConfigurePrivateRangeBlackholing(ctx, containerNetworkingCIDR); err != nil {
			return err
		}
	}

	return nil
}

// configurePolicyBasedRoutingForNetworkWIthRoutePrefix configures policy routing for secondary
// network interface. The secondary interface is identified by the --route_prefix.
// This function:
//   - adds a new routing table tableName.
//     Equivalent to: " echo '1 rt1' | sudo tee -a /etc/iproute2/rt_tables'
//   - adds two ip rules.
//     Equivalent to: 'ip rule add from 172.24.0.24 table rt1' and
//     'ip rule add to 172.24.0.24 table rt1' where 172.24.0.24 is the internal ip for the
//     secondary network interface.
//   - adds routes to table rt1.
//     Equivalent to: 'ip route add 172.24.0.1 src 172.24.0.24 dev ens5 table rt1' and
//     'ip route add default via 172.24.0.1 dev ens5 table rt1' where 172.24.0.1 and ens5 are
//     the gateway and interface name of the secondary network interface.
func configurePolicyBasedRoutingForSecondaryNetwork(ctx context.Context) error {
	if !IsSecondaryNetworkEnabled() {
		// No need to add IP rule when we don't use secondary network
		return nil
	}

	route, err := findRoute(*routePrefix)
	if err != nil {
		return err
	}

	// Adds two ip rules
	ip, err := ipFromDevice(ctx, route.device)
	if err != nil {
		return err
	}
	ipStr := ip.String()

	if err := AddIPRuleIfNotPresent(ctx, []string{"to", ipStr}); err != nil {
		return err
	}
	if err := AddIPRuleIfNotPresent(ctx, []string{"from", ipStr}); err != nil {
		return err
	}

	// Adds routes to routing table.
	ipRouteArgs := []string{route.gateway.String(), "dev", route.device, "scope", "link", "src", ipStr}
	if err := AddRouteIfNotPresent(ctx, ipRouteArgs); err != nil {
		return err
	}
	ipRouteArgs = []string{"default", "via", route.gateway.String(), "dev", route.device}
	if err := AddRouteIfNotPresent(ctx, ipRouteArgs); err != nil {
		return err
	}

	return nil
}

func appendRoutingTable(args []string) []string {
	return append(args, "table", routingTableName)
}

// AddIPRuleIfNotPresent adds a ip rule to look up routingTableName if this rule is not present.
func AddIPRuleIfNotPresent(ctx context.Context, ruleArgs []string) error {
	listArgs := append([]string{"ip", "rule", "list"}, ruleArgs...)
	listArgs = appendRoutingTable(listArgs)
	out, err := sudoCommand(ctx, listArgs...)
	if err != nil {
		return err
	}

	if len(out) == 0 {
		addArgs := append([]string{"ip", "rule", "add"}, ruleArgs...)
		addArgs = appendRoutingTable(addArgs)
		if err := runCommand(ctx, addArgs...); err != nil {
			return err
		}
	}
	log.Debugf("ip rule %v already exists", ruleArgs)
	return nil
}

// AddRouteIfNotPresent adds a route in the routing table with routingTableName if the route is not
// present.
func AddRouteIfNotPresent(ctx context.Context, routeArgs []string) error {
	listArgs := routeArgs
	if len(listArgs) > 0 && listArgs[0] == "blackhole" {
		listArgs = listArgs[1:]
	}
	listArgs = append([]string{"ip", "route", "list"}, listArgs...)
	listArgs = appendRoutingTable(listArgs)
	out, err := sudoCommand(ctx, listArgs...)
	addRoute := false

	if err == nil {
		addRoute = len(out) == 0
	} else {
		if strings.Contains(err.Error(), "ipv4: FIB table does not exist") {
			// if no routes has been added to rt1, "ip route list table rt1" will return an error.
			addRoute = true
		} else {
			return err
		}
	}

	if addRoute {
		addArgs := append([]string{"ip", "route", "add"}, routeArgs...)
		addArgs = appendRoutingTable(addArgs)
		if err := runCommand(ctx, addArgs...); err != nil {
			return err
		}
	}
	log.Debugf("ip route %v already exists", routeArgs)
	return nil
}

func IsSecondaryNetworkEnabled() bool {
	return *routePrefix != "default"
}

func IsPrivateRangeBlackholingEnabled() bool {
	return *blackholePrivateRanges
}

func PreserveExistingNetNamespaces() bool {
	return *preserveExistingNetNamespaces
}
