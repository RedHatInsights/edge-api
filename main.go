// Package main Edge API
//
// An API server for fleet edge management capabilities.
// FIXME: golangci-lint
// nolint:errcheck,revive,typecheck,unused
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/Unleash/unleash-client-go/v4"
	"github.com/getsentry/sentry-go"
	slg "github.com/getsentry/sentry-go/logrus"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	redoc "github.com/go-openapi/runtime/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redhatinsights/edge-api/config"
	em "github.com/redhatinsights/edge-api/internal/middleware"
	l "github.com/redhatinsights/edge-api/logger"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/jobs"
	"github.com/redhatinsights/edge-api/pkg/metrics"
	"github.com/redhatinsights/edge-api/pkg/routes"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	edgeunleash "github.com/redhatinsights/edge-api/unleash"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
	"github.com/redhatinsights/platform-go-middlewares/v2/request_id"
	log "github.com/sirupsen/logrus"
)

func setupDocsMiddleware(handler http.Handler) http.Handler {
	opt := redoc.RedocOpts{
		SpecURL: "/api/edge/v1/openapi.json",
	}
	return redoc.Redoc(opt, handler)
}

func initDependencies() {
	config.Init()
	l.InitLogger(os.Stdout)
	db.InitDB()
}

func serveMetrics(port int) *http.Server {
	metricsRoute := chi.NewRouter()
	metricsRoute.Get("/", routes.StatusOK)
	metricsRoute.Handle("/metrics", promhttp.Handler())
	server := http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      metricsRoute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			l.LogErrorAndPanic("metrics service stopped unexpectedly", err)
		}
	}()
	log.Infof("metrics service started at port %d", port)
	return &server
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}

		t1 := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		org_id, _ := common.GetOrgID(r)
		fields := log.Fields{
			"request_id": request_id.GetReqID(r.Context()),
			"org_id":     org_id,
			"method":     r.Method,
		}

		log.WithContext(r.Context()).WithFields(fields).Debugf("Started %s request %s", r.Method, r.URL.Path)

		defer func() {
			latency := time.Since(t1).Milliseconds()
			fields["latency_ms"] = latency
			fields["status_code"] = ww.Status()
			fields["bytes"] = ww.BytesWritten()
			log.WithContext(r.Context()).WithFields(fields).Infof("Finished %s request %s with %d", r.Method, r.URL.Path, ww.Status())
		}()

		next.ServeHTTP(ww, r)
	})
}

func webRoutes(cfg *config.EdgeConfig) *chi.Mux {
	route := chi.NewRouter()
	route.Use(
		em.NewPatternMiddleware(),
		request_id.ConfiguredRequestID("x-rh-insights-request-id"),
		middleware.RealIP,
		middleware.Recoverer,
		setupDocsMiddleware,
		dependencies.Middleware,
		logMiddleware,
	)

	// Unauthenticated routes
	route.Get("/", routes.StatusOK)
	route.Get("/api/edge/v1/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, cfg.OpenAPIFilePath)
	})

	// Authenticated routes
	authRoute := route.Group(nil)
	if cfg.Auth {
		authRoute.Use(
			identity.EnforceIdentity,
			dependencies.Middleware,
		)
	}

	authRoute.Route("/api/edge/v1", func(s chi.Router) {
		s.Route("/images", routes.MakeImagesRouter)
		s.Route("/updates", routes.MakeUpdatesRouter)
		s.Route("/image-sets", routes.MakeImageSetsRouter)
		s.Route("/devices", routes.MakeDevicesRouter)
		s.Route("/thirdpartyrepo", routes.MakeThirdPartyRepoRouter)
		s.Route("/fdo", routes.MakeFDORouter)
		s.Route("/device-groups", routes.MakeDeviceGroupsRouter)
		s.Route("/storage", routes.MakeStorageRouter)

		// this is meant for testing the job queue
		s.Post("/ops/jobs/noop", services.CreateNoopJob)
		s.Post("/ops/jobs/fallback", services.CreateFallbackJob)
	})
	return route
}

func serveWeb(cfg *config.EdgeConfig, consumers []services.ConsumerService) *http.Server {
	server := http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.WebPort),
		Handler:      webRoutes(cfg),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	server.RegisterOnShutdown(func() {
		for _, consumer := range consumers {
			if consumer != nil {
				consumer.Close()
			}
		}
	})
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			l.LogErrorAndPanic("web service stopped unexpectedly", err)
		}
	}()
	log.Infof("web service started at port %d", cfg.WebPort)
	return &server
}

func gracefulTermination(server *http.Server, serviceName string) {
	log.Infof("%s service stopped", serviceName)
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second) // 5 seconds for graceful shutdown
	defer cancel()
	if err := server.Shutdown(ctxShutdown); err != nil {
		l.LogErrorAndPanic(fmt.Sprintf("%s service shutdown failed", serviceName), err)
	}
	log.Infof("%s service shutdown complete", serviceName)
}

func featureFlagsConfigPresent() bool {
	conf := config.Get()
	return conf.FeatureFlagsURL != ""
}

func featureFlagsServiceUnleash() bool {
	conf := config.Get()
	return conf.FeatureFlagsService == "unleash"
}

func main() {
	ctx := context.Background()
	// this only catches interrupts for main
	// see images for image build interrupt
	interruptSignal := make(chan os.Signal, 1)
	signal.Notify(interruptSignal, os.Interrupt, syscall.SIGTERM)

	initDependencies()

	cfg := config.Get()

	if feature.GlitchtipLogging.IsEnabled() {
		// Set up Sentry client for GlitchTip error tracking
		sentry.Init(sentry.ClientOptions{
			Dsn: cfg.GlitchtipDsn,
			Tags: map[string]string{
				"service": "edge-api",
				"binary":  filepath.Base(os.Args[0]),
			},
		})

		// Initialize logrus hook for Sentry
		sh := slg.NewFromClient([]log.Level{
			log.PanicLevel,
			log.FatalLevel,
			log.ErrorLevel,
		}, sentry.CurrentHub().Client())
		log.AddHook(sh)

		// Flush client after main exits
		defer sentry.Flush(2 * time.Second)
		// Report captured errors to GlitchTip
		defer sentry.Recover()
	}

	if cfg.Debug {
		if buildInfo, ok := debug.ReadBuildInfo(); ok {
			b := new(bytes.Buffer)
			enc := json.NewEncoder(b)
			enc.SetIndent("", "  ")
			err := enc.Encode(buildInfo)
			if err == nil {
				log.WithField("buildInfo", b).Debug("Build information")
			} else {
				log.WithField("ok", ok).Debug("Unable to encode buildInfo")
			}
		} else {
			log.WithField("ok", ok).Debug("Unable to get Build Info")
		}
	}

	config.LogConfigAtStartup(cfg)

	if featureFlagsConfigPresent() {
		err := unleash.Initialize(
			unleash.WithListener(&edgeunleash.EdgeListener{}),
			unleash.WithAppName("edge-api"),
			unleash.WithUrl(cfg.UnleashURL),
			unleash.WithRefreshInterval(5*time.Second),
			unleash.WithMetricsInterval(5*time.Second),
			unleash.WithCustomHeaders(http.Header{"Authorization": {cfg.FeatureFlagsAPIToken}}),
		)
		if err != nil {
			log.WithField("Error", err).Error("Unleash client failed to initialize")
		} else {
			log.WithField("FeatureFlagURL", cfg.UnleashURL).Info("Unleash client initialized successfully")
		}
	} else {
		log.WithField("FeatureFlagURL", cfg.UnleashURL).Warning("FeatureFlag service initialization was skipped.")
	}

	metrics.RegisterAPIMetrics()

	jobs.InitMemoryWorker()
	jobs.Worker().Start(ctx)
	defer jobs.Worker().Stop(ctx)

	consumers := []services.ConsumerService{
		services.NewKafkaConsumerService(cfg.KafkaConfig, kafkacommon.TopicPlaybookDispatcherRuns),
		services.NewKafkaConsumerService(cfg.KafkaConfig, kafkacommon.TopicInventoryEvents),
	}

	webServer := serveWeb(cfg, consumers)
	defer gracefulTermination(webServer, "web")

	metricsServer := serveMetrics(cfg.MetricsPort)
	defer gracefulTermination(metricsServer, "metrics")

	if cfg.KafkaConfig != nil {
		log.Info("Starting Kafka Consumers")
		for _, consumer := range consumers {
			if consumer != nil {
				go consumer.Start()
			}
		}
	}

	// block here and shut things down on interrupt
	<-interruptSignal
	log.Info("Shutting down gracefully...")
	// temporarily adding a sleep to help troubleshoot interrupts
	time.Sleep(10 * time.Second)
}
