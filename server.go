package october

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"strconv"

	"go.uber.org/zap"
)

/*var (
	octoberMetricNS     = metrics.NewNamespace("october")
	octoberHealthSubsys = octoberMetricNS.WithSubsystem("health")

	healthCounterRequests = octoberHealthSubsys.NewCounter(metrics.Opts{
		Name: "endpoint_requests",
		Help: "Total number of October health check endpoint requests",
	})

	healthCounterResponses = octoberHealthSubsys.NewCounterVec(
		metrics.Opts{
			Name: "endpoint_responses",
			Help: "Total number of October health check endpoint responses served, by response code",
		},
		[]string{"code", "health_status"},
	)

	healthLatencySummary = octoberHealthSubsys.NewSummary(metrics.SummaryOpts{
		Name:       "latency",
		Help:       "Summary of October health check endpoint latency",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001, 0.999: 0.0001},
	})
)*/

func init() {

	//octoberMetricNS.MustRegister()
	//octoberHealthSubsys.MustRegister()
	//prometheus.MustRegister(healthCounterRequests)
	//prometheus.MustRegister(healthCounterResponses)
	//prometheus.MustRegister(lastHealthEndpointLatency)
}

func WithHealthCheck(o *OctoberServer, name string, hc HealthCheck) *OctoberServer {
	o.healthChecks.AddCheck(name, hc)
	return o
}

func NewOctoberServer(mode Mode, port int) *OctoberServer {

	if port == 0 {
		port = 10010
	}

	logger := zap.S().Named("OCTOBER")
	// /healthChecks.AddCheck("october", check)

	logger.Infof("Configuring server with mode %s", mode)

	return &OctoberServer{
		logger: logger,
		server:       &http.Server{},
		mode:         mode,
		healthChecks: make(HealthChecks),
		checkLock:    &sync.Mutex{},

		octoberBindAddress: "0.0.0.0",
		octoberBindPort:    port,
	}
}

type OctoberServer struct {
	logger   *zap.SugaredLogger
	server                     *http.Server
	mode                       Mode
	healthChecks               HealthChecks
	checkLock                  *sync.Mutex // We only want 1 check to go on at once
	lastHealthResult           HealthCheckResult
	lastHealthResultBackground bool

	backgroundCheckHistory []HealthCheckResult
	endpointCheckHistory   []HealthCheckResult

	octoberBindAddress string
	octoberBindPort    int
}

func (o *OctoberServer) buildServerMux() *http.ServeMux {
	mux := http.NewServeMux()

	// Serve prometheus metrics
	mux.Handle("/metrics", promhttp.Handler())
	//mux.Handle("/influxdb", metrichttp.InfluxDBHandler())
	mux.HandleFunc("/health", healthHTTPHandler(o.healthChecks))

	if o.mode != PROD {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	return mux
}


func (o *OctoberServer) GenerateGRPCServerFromEnv() (*GRPCServer, error) {

	zap.L().Named("OCTOBER").Info("Generating controlled GRPC from environment variables")

	address := "0.0.0.0"
	port := 10000

	envPort := strings.TrimSpace(os.Getenv(grpcPortEnvVariable))
	if envPort != "" {
		var err error
		port, err = strconv.Atoi(envPort)
		if err != nil {
			return nil, err
		}
	}

	tlsCrt := strings.TrimSpace(os.Getenv(tlsBundleCRTEnvVariable))
	tlsKey := strings.TrimSpace(os.Getenv(tlsKeyEnvVariable))

	if tlsCrt == "" {
		o.logger.Infof("%s: (empty)", tlsBundleCRTEnvVariable)
	} else {
		o.logger.Infof("%s: %s", tlsBundleCRTEnvVariable, tlsCrt)
	}

	if tlsKey == "" {
		o.logger.Infof("%s: (empty)", tlsKeyEnvVariable)
	} else {
		o.logger.Infof("%s: %s", tlsKeyEnvVariable, tlsKey)
	}

	server := &GRPCServer{
		mode:   o.mode,
		Server: nil,

		address: address,
		port:    port,
	}

	tlsErr := server.WithTLS(tlsCrt, tlsKey)
	if tlsErr != nil {
		return nil, tlsErr
	}

	return server, nil

}

func (o *OctoberServer) GenerateGQLGenServerServerFromEnv() (*GQLGenServer, error) {

	o.logger.Info("Generating controlled gqlgen server from environment variables")

	address := "0.0.0.0"
	port := 8080

	envPort := strings.TrimSpace(os.Getenv(gqlPortEnvVariable))
	if envPort != "" {
		var err error
		port, err = strconv.Atoi(envPort)
		if err != nil {
			return nil, err
		}
	}


	server := &GQLGenServer{
		mode:   o.mode,

		serverLock: &sync.Mutex{},
		healthChecks: o.healthChecks,
		address: address,
		port:    port,
	}

	return server, nil

}

func (o *OctoberServer) MustGenerateGQLGenServerServerFromEnv() *GQLGenServer {

	server, err := o.GenerateGQLGenServerServerFromEnv()

	if err != nil {
		zap.L().Named("OCTOBER").Fatal("Failed to generate controlled gqlgen server from environment variables", zap.Error(err))
	}

	return server
}

func (o *OctoberServer) Start(controllableServers []ControllableServer, gracefulSignals ...os.Signal) {


	o.server.Addr = fmt.Sprintf("%s:%d", o.octoberBindAddress, o.octoberBindPort)
	o.server.Handler = o.buildServerMux()

	controllableServers = append(
		[]ControllableServer{ControlHttpServer(o.server, "october")},
		controllableServers...
	)

	// Initialize stop coordinators
	// Used to coordinate stopping the October server and the controllable servers
	stopChan := make(chan struct{})
	stopping := false
	stoppingMu := &sync.Mutex{}

	// Send stop signal if not already sent
	// Takes in an error that is logged if shutdown started, returns a boolean if we sent the signal
	sendStopSignal := func(err error) bool {
		stoppingMu.Lock()
		defer stoppingMu.Unlock()

		if !stopping {
			stopping = true
			if err != nil {
				o.logger.Error("Shutting down October from error", zap.Error(err))
			} else {
				o.logger.Info("Shutting down October from graceful signal")
			}

			stopChan <- struct{}{}
			return true
		} else {
			if err != nil {
				o.logger.Named("OCTOBER").Info("Ignored graceful shutdown signal, graceful shutdown already initiated")
			}
			return false
		}


	}


	shutdownCount := make(chan bool, len(controllableServers))

	// Start by checking if we have graceful signals to handle
	// We want to be sure this is started before anything else
	if len(gracefulSignals) > 0 {
		o.logger.Infof("Starting graceful shutdown handler for signals:")
		for _, gs := range gracefulSignals {
			o.logger.Named("OCTOBER").Infof("    %s", gs)
		}

		OnSignal(func(sig os.Signal) {

			sendStopSignal(nil)

		}, gracefulSignals...)

	}

	for _, controllable := range controllableServers {

		go func(c ControllableServer) {


			logger := o.logger.Named(c.Name())

			logger.Infof("Starting controller server %s", c.Name())
			shutdownControlled, err := c.Start()

			if !shutdownControlled {
				logger.Error(c.Name()+" shut down unexpectedly", zap.Error(err))
			}

			sendStopSignal(err)

			shutdownCount <- true

		}(controllable)
	}




	var closeGroup sync.WaitGroup
	select {
	case <-stopChan:

		shutdownCtx := context.Background()

		// We received stop signal, call shutdown on all of our controllable servers
		for _, controllable := range controllableServers {

			closeGroup.Add(1)
			go func(c ControllableServer) {
				defer closeGroup.Done()

				err := c.Shutdown(shutdownCtx)
				if err != nil {
					o.logger.Named(c.Name()).Error(c.Name()+" error during shutdown", zap.Error(err))
				}

			}(controllable)
		}
	}



	closeGroup.Wait()
	o.logger.Info("Stopped")


}

func (o *OctoberServer) Shutdown(ctx context.Context) error {

	address := fmt.Sprintf("%s:%d", o.octoberBindAddress, o.octoberBindPort)
	zap.S().Named("OCTOBER").Infof("Gracefully stopping server (%s)...", address)

	return o.server.Shutdown(ctx)

}
