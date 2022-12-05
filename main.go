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
	"runtime/debug"
	"syscall"
	"time"

	redoc "github.com/go-openapi/runtime/middleware"
	"github.com/redhatinsights/edge-api/config"
	l "github.com/redhatinsights/edge-api/logger"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/routes"
	"github.com/redhatinsights/edge-api/pkg/services"
	edgeunleash "github.com/redhatinsights/edge-api/unleash"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/platform-go-middlewares/request_id"
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
	l.InitLogger()
	db.InitDB()
}

func serveMetrics(port int) *http.Server {
	metricsRoute := chi.NewRouter()
	SpecURL := "api/edge/v1/openapi.json"

	readinessHandlerFunc := &routes.ConfigurableWebGetter{
		URL:    fmt.Sprintf("%s:%d/%s", config.Get().MetricsBaseURL, config.Get().WebPort, SpecURL),
		GetURL: http.Get,
	}

	metricsRoute.Get("/", routes.GetReadinessStatus(readinessHandlerFunc))
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
	log.Info("metrics service started")
	return &server
}

func webRoutes(cfg *config.EdgeConfig) *chi.Mux {
	route := chi.NewRouter()
	route.Use(
		request_id.ConfiguredRequestID("x-rh-insights-request-id"),
		middleware.RealIP,
		middleware.Recoverer,
		middleware.Logger,
		setupDocsMiddleware,
		dependencies.Middleware,
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
	log.Info("web service started")
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
	// this only catches interrupts for main
	// see images for image build interrupt
	interruptSignal := make(chan os.Signal, 1)
	signal.Notify(interruptSignal, os.Interrupt, syscall.SIGTERM)

	initDependencies()

	cfg := config.Get()
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

	var configValues map[string]interface{}
	cfgBytes, _ := json.Marshal(cfg)
	_ = json.Unmarshal(cfgBytes, &configValues)
	// TODO: remove this next line once we have all allowed config values moved to config.LogConfigAtStartup
	log.WithFields(configValues).Info("Configuration Values")
	config.LogConfigAtStartup(cfg)

	if featureFlagsConfigPresent() {
		err := unleash.Initialize(
			unleash.WithListener(&edgeunleash.EdgeListener{}),
			unleash.WithAppName("edge-api"),
			unleash.WithUrl(cfg.UnleashURL),
			unleash.WithRefreshInterval(5*time.Second),
			unleash.WithMetricsInterval(5*time.Second),
			unleash.WithCustomHeaders(http.Header{"Authorization": {fmt.Sprintf("Bearer %s", cfg.UnleashSecretName)}}),
		)
		if err != nil {
			log.WithField("Error", err).Error("Unleash client failed to initialize")
		} else {
			log.WithField("FeatureFlagURL", cfg.UnleashURL).Info("Unleash client initialized successfully")
		}
	} else {
		log.WithField("FeatureFlagURL", cfg.UnleashURL).Warning("FeatureFlag service initialization was skipped.")
	}

	consumers := []services.ConsumerService{
		services.NewKafkaConsumerService(cfg.KafkaConfig, kafkacommon.TopicPlaybookDispatcherRuns),
		services.NewKafkaConsumerService(cfg.KafkaConfig, kafkacommon.TopicInventoryEvents),
	}
	webServer := serveWeb(cfg, consumers)
	metricsServer := serveMetrics(cfg.MetricsPort)

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
	time.Sleep(20 * time.Second)
	gracefulTermination(webServer, "web")
	gracefulTermination(metricsServer, "metrics")
	log.Info("Everything has shut down, goodbye")
}
