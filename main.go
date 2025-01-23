// Package main Edge API
//
// An API server for fleet edge management capabilities.
// FIXME: golangci-lint
// nolint:errcheck,revive,typecheck,unused
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Unleash/unleash-client-go/v4"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	redoc "github.com/go-openapi/runtime/middleware"
	log "github.com/osbuild/logging/pkg/logrus"
	"github.com/osbuild/logging/pkg/strc"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redhatinsights/edge-api/config"
	em "github.com/redhatinsights/edge-api/internal/middleware"
	"github.com/redhatinsights/edge-api/logger"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/jobs"
	"github.com/redhatinsights/edge-api/pkg/metrics"
	"github.com/redhatinsights/edge-api/pkg/routes"
	"github.com/redhatinsights/edge-api/pkg/services"
	edgeunleash "github.com/redhatinsights/edge-api/unleash"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
	"github.com/redhatinsights/platform-go-middlewares/v2/request_id"
)

func setupDocsMiddleware(handler http.Handler) http.Handler {
	opt := redoc.RedocOpts{
		SpecURL: "/api/edge/v1/openapi.json",
	}
	return redoc.Redoc(opt, handler)
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
			logger.LogErrorAndPanic("metrics service stopped unexpectedly", err)
		}
	}()
	log.Infof("metrics service started at port %d", port)
	return &server
}

func webRoutes(cfg *config.EdgeConfig) *chi.Mux {
	route := chi.NewRouter()
	route.Use(
		em.NewPatternMiddleware(),
		request_id.ConfiguredRequestID("x-rh-insights-request-id"),
		middleware.RealIP,
		middleware.Recoverer,
		setupDocsMiddleware,
		strc.NewMiddlewareWithFilters(
			slog.Default(),
			strc.IgnorePathPrefix("/metrics"),
			strc.IgnorePathPrefix("/status"),
			strc.IgnorePathPrefix("/ready"),
		),
		strc.HeadfieldPairMiddleware(logger.HeadfieldPairs),
		strc.RecoverPanicMiddleware(slog.Default()),
	)

	// Unauthenticated routes
	route.Get("/", routes.StatusOK)
	route.Get("/api/edge/v1/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, cfg.OpenAPIFilePath)
	})

	// Authenticated routes
	authRoute := route.Group(nil)
	if cfg.Auth {
		authRoute.Use(identity.EnforceIdentity)
	}

	authRoute.Route("/api/edge/v1", func(edgerRouter chi.Router) {
		// Untangling the dependencies, these routes do not use the dependencies context
		edgerRouter.Route("/storage", routes.MakeStorageRouter)
		// The group below is still not sanitized
		edgerRouter.Group(func(s chi.Router) {
			s.Use(dependencies.Middleware)
			s.Route("/images", routes.MakeImagesRouter)
			s.Route("/updates", routes.MakeUpdatesRouter)
			s.Route("/image-sets", routes.MakeImageSetsRouter)
			s.Route("/devices", routes.MakeDevicesRouter)
			s.Route("/thirdpartyrepo", routes.MakeThirdPartyRepoRouter)
			s.Route("/device-groups", routes.MakeDeviceGroupsRouter)

			// this is meant for testing the job queue
			s.Post("/ops/jobs/noop", services.CreateNoopJob)
			s.Post("/ops/jobs/fallback", services.CreateFallbackJob)
		})
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
			logger.LogErrorAndPanic("web service stopped unexpectedly", err)
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
		logger.LogErrorAndPanic(fmt.Sprintf("%s service shutdown failed", serviceName), err)
	}
	log.Infof("%s service shutdown complete", serviceName)
}

func main() {
	ctx := context.Background()
	// this only catches interrupts for main
	// see images for image build interrupt
	interruptSignal := make(chan os.Signal, 1)
	signal.Notify(interruptSignal, os.Interrupt, syscall.SIGTERM)

	config.Init()
	cfg := config.Get()
	err := logger.InitializeLogging(ctx, cfg)
	if err != nil {
		panic(err)
	}
	db.InitDB()
	defer logger.Flush()

	config.LogConfigAtStartup(cfg)

	if config.FeatureFlagsConfigured() {
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

	defer routes.UpdateTransCache.Stop()

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
