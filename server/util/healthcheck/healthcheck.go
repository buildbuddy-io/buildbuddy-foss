package healthcheck

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"sync"
	"syscall"
	"time"

	"github.com/buildbuddy-io/buildbuddy/server/interfaces"
	"github.com/buildbuddy-io/buildbuddy/server/metrics"
	"github.com/buildbuddy-io/buildbuddy/server/util/log"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"
	"github.com/buildbuddy-io/buildbuddy/server/util/statusz"
	"github.com/buildbuddy-io/buildbuddy/server/util/watchdog"
	"github.com/mattn/go-isatty"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	hlpb "google.golang.org/grpc/health/grpc_health_v1"
)

var (
	maxShutdownDuration           = flag.Duration("max_shutdown_duration", 25*time.Second, "Time to wait for shutdown")
	shutdownLameduckDuration      = flag.Duration("shutdown_lameduck_duration", 0, "If set, the app will be marked unready but not run shutdown functions until this period passes.")
	logGoroutineProfileOnShutdown = flag.Bool("log_goroutine_profile_on_shutdown", false, "Whether to log all goroutine stack traces on shutdown.")
	reportNotReady                = flag.Bool("report_not_ready", false, "If set to true, the app will always report as being unready.")
	maxUnreadyDuration            = flag.Duration("max_unready_duration", 0, "If > 0, the app will terminate if it does not become ready after this long")
)

const (
	healthCheckPeriod  = 3 * time.Second // The time to wait between health checks.
	healthCheckTimeout = 2 * time.Second // How long a health check may take, max.
)

type serviceStatus struct {
	Name  string
	Error error
}

type HealthChecker struct {
	done          chan bool
	quit          chan struct{}
	checkersMu    sync.Mutex
	checkers      map[string]interfaces.Checker
	lastStatus    []*serviceStatus
	serverType    string
	shutdownOnce  sync.Once
	mu            sync.RWMutex // protects: shutdownFuncs, readyToServe, shuttingDown
	shutdownFuncs []interfaces.CheckerFunc
	readyToServe  bool
	shuttingDown  bool
	watchdogTimer *watchdog.Timer
}

func NewHealthChecker(serverType string) *HealthChecker {
	hc := HealthChecker{
		serverType:    serverType,
		done:          make(chan bool),
		quit:          make(chan struct{}),
		shutdownFuncs: make([]interfaces.CheckerFunc, 0),
		readyToServe:  true,
		checkersMu:    sync.Mutex{},
		checkers:      make(map[string]interfaces.Checker, 0),
		lastStatus:    make([]*serviceStatus, 0),
		watchdogTimer: watchdog.New(*maxUnreadyDuration),
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go hc.handleSignals(signalChan)

	go hc.handleShutdownFuncs()
	go func() {
		ticker := time.NewTicker(healthCheckPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-hc.quit:
				return
			case <-ticker.C:
				hc.runHealthChecks(context.Background())
			}
		}
	}()
	statusz.AddSection("watchdog", "Watchdog Timer", hc.watchdogTimer)
	statusz.AddSection("healthcheck", "Backend service health checks", &hc)
	return &hc
}

func (h *HealthChecker) Statusz(ctx context.Context) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	buf := `<table style="width: 150px;"><tr><th>Name</th><th>Status</th></tr>`
	for _, serviceStatus := range h.lastStatus {
		statusString := "OK"
		if serviceStatus.Error != nil {
			statusString = serviceStatus.Error.Error()
		}
		buf += fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", serviceStatus.Name, statusString)
	}
	buf += "</table>"
	return buf
}

func (h *HealthChecker) handleSignals(signalChan <-chan os.Signal) {
	// When running in a terminal, the ^C character echoed back to the user
	// messes up the log output a bit. So, print a newline every time we get
	// a signal to make the output a little cleaner.
	isTTY := isatty.IsTerminal(uintptr(os.Stderr.Fd()))
	sig := <-signalChan
	if isTTY {
		fmt.Println()
	}
	log.Infof("Caught %s signal; starting graceful shutdown (hard-stopping in %s)", sig, *maxShutdownDuration)
	hardStopTime := time.Now().Add(*maxShutdownDuration)
	h.Shutdown()
	numSignalsReceived := 1
	for sig := range signalChan {
		numSignalsReceived++
		// If we're running in a TTY and we keep getting SIGINT/SIGTERM, the
		// user is probably mashing Ctrl+C and really wants to kill the server.
		// After 3 signals, exit immediately. Before then, report the current
		// status so the user knows we got their request but are just still
		// shutting down.
		if isTTY {
			fmt.Println()
			if numSignalsReceived >= 3 {
				log.Fatalf("Caught %s signal; third shutdown request; exiting immediately.", sig)
			}
		}
		d := time.Until(hardStopTime)
		if d > 0 {
			log.Infof("Caught %s signal; still shutting down; will hard-stop in %s", sig, d)
		} else {
			log.Warningf("Caught %s signal; still waiting for server handlers to finish after hard-stop %s ago.", sig, -d)
		}
	}
}

func (h *HealthChecker) handleShutdownFuncs() {
	<-h.quit

	h.mu.Lock()
	h.readyToServe = false
	h.shuttingDown = true
	h.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), *maxShutdownDuration)
	defer cancel()

	if *logGoroutineProfileOnShutdown {
		logGoroutineProfile()
	}

	time.Sleep(*shutdownLameduckDuration)

	eg, egCtx := errgroup.WithContext(ctx)
	for _, fn := range h.shutdownFuncs {
		f := fn
		eg.Go(func() error {
			if err := f(egCtx); err != nil {
				log.CtxErrorf(ctx, "Error gracefully shutting down: %s", err)
			}
			return nil
		})
	}
	eg.Wait()
	if err := ctx.Err(); err != nil {
		log.CtxErrorf(ctx, "MaxShutdownDuration exceeded. Non-graceful exit.")
	}
	time.Sleep(10 * time.Millisecond)
	log.Infof("Server %q stopped.", h.serverType)
	close(h.done)
}

func (h *HealthChecker) RegisterShutdownFunction(f interfaces.CheckerFunc) {
	h.mu.Lock()
	h.shutdownFuncs = append(h.shutdownFuncs, f)
	h.mu.Unlock()
}

func (h *HealthChecker) AddHealthCheck(name string, f interfaces.Checker) {
	h.checkersMu.Lock()
	h.checkers[name] = f
	h.checkersMu.Unlock()

	// Mark the service as unhealthy until the healthcheck runs
	// and it becomes healthy.
	h.mu.Lock()
	h.readyToServe = false
	h.mu.Unlock()
}

func (h *HealthChecker) WaitForGracefulShutdown() {
	h.runHealthChecks(context.Background())
	<-h.done
}

func (h *HealthChecker) Shutdown() {
	h.shutdownOnce.Do(func() {
		close(h.quit)
	})
}

func (h *HealthChecker) runHealthChecks(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	statusData := make([]*serviceStatus, 0)
	statusDataMu := sync.Mutex{}

	h.checkersMu.Lock()
	checkers := make(map[string]interfaces.Checker, len(h.checkers))
	for k, v := range h.checkers {
		checkers[k] = v
	}
	h.checkersMu.Unlock()

	eg, ctx := errgroup.WithContext(ctx)
	for name, ck := range checkers {
		name := name
		checkFn := ck
		eg.Go(func() error {
			err := checkFn.Check(ctx)

			// Update per-service statusData
			statusDataMu.Lock()
			statusData = append(statusData, &serviceStatus{name, err})
			statusDataMu.Unlock()

			if err != nil {
				metrics.HealthCheck.With(prometheus.Labels{
					metrics.HealthCheckName: name,
				}).Set(0)
				return status.UnavailableErrorf("Service %s is unhealthy: %s", name, err)
			}

			metrics.HealthCheck.With(prometheus.Labels{
				metrics.HealthCheckName: name,
			}).Set(1)
			return nil
		})
	}
	err := eg.Wait()
	newReadinessState := true
	if err != nil {
		newReadinessState = false
		log.Warningf("Checker err: %s", err)
	}

	previousReadinessState := false
	h.mu.Lock()
	if !h.shuttingDown {
		previousReadinessState = h.readyToServe
		h.readyToServe = newReadinessState
		h.lastStatus = statusData
	}
	if h.readyToServe {
		h.watchdogTimer.Reset()
	} else {
		if !h.watchdogTimer.Live() {
			log.Warningf("Watchdog timer expired; triggering shutdown!")
			go func() {
				h.Shutdown()
			}()
		}
	}
	h.mu.Unlock()

	if newReadinessState != previousReadinessState {
		log.Infof("HealthChecker transitioning from ready: %t => ready: %t", previousReadinessState, newReadinessState)
	}
}

func (h *HealthChecker) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqServerType := serverType(r)
		if reqServerType == h.serverType {
			h.mu.RLock()
			ready := h.readyToServe && !*reportNotReady
			h.mu.RUnlock()

			if ready {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			return
		}
		err := fmt.Errorf("Server type: '%s' unknown (did not match: %q)", reqServerType, h.serverType)
		log.Warningf("Readiness check returning error: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	})
}

// serverType is derived from either the headers or a query parameter
func serverType(r *http.Request) string {
	if r.Header.Get("server-type") != "" {
		return r.Header.Get("server-type")
	}
	// GCP load balancer healthchecks do not allow sending headers.
	return r.URL.Query().Get("server-type")
}

func (h *HealthChecker) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqServerType := serverType(r)
		if reqServerType == h.serverType {
			w.Write([]byte("OK"))
			return
		}
		err := fmt.Errorf("Server type: '%s' unknown (did not match: %q)", reqServerType, h.serverType)
		log.Warningf("Liveness check returning error: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	})
}

func (h *HealthChecker) Check(ctx context.Context, req *hlpb.HealthCheckRequest) (*hlpb.HealthCheckResponse, error) {
	// GRPC does not have indepenent health and readiness checks like HTTP does.
	// An additional wrinkle is that AWS ALB's do not support sending a service
	// name to the GRPC health check. To maximize compatibility and usefulness
	// we ignore the service name for now (sad face), and return:
	//   - SERVING when the service is ready
	//   - NOT_SERVING when the service is not ready
	//   - UNKNOWN when the service is shutting down.
	h.mu.RLock()
	ready := h.readyToServe
	shuttingDown := h.shuttingDown
	h.mu.RUnlock()
	rsp := &hlpb.HealthCheckResponse{}
	if ready {
		rsp.Status = hlpb.HealthCheckResponse_SERVING
	} else {
		rsp.Status = hlpb.HealthCheckResponse_NOT_SERVING
	}

	if shuttingDown {
		rsp.Status = hlpb.HealthCheckResponse_UNKNOWN
	}
	return rsp, nil
}

func (h *HealthChecker) List(ctx context.Context, req *hlpb.HealthListRequest) (*hlpb.HealthListResponse, error) {
	return nil, status.UnimplementedError("List not implemented")
}

func (h *HealthChecker) Watch(req *hlpb.HealthCheckRequest, stream hlpb.Health_WatchServer) error {
	return status.UnimplementedError("Watch not implemented")
}

func logGoroutineProfile() {
	p := pprof.Lookup("goroutine")
	if p == nil {
		return
	}
	b := &bytes.Buffer{}
	// debug=1 results in more compact output like "64 goroutines @ <location>"
	// compared to debug=2 which would show the full stack 64 times.
	const debugParam = 1
	p.WriteTo(b, debugParam)
	log.Info(b.String())
}
