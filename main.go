// Package main Edge API
//
//  An API server for fleet edge management capabilities.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	redoc "github.com/go-openapi/runtime/middleware"
	"github.com/redhatinsights/edge-api/config"
	l "github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/routes"
	"github.com/redhatinsights/edge-api/pkg/services"

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

func main() {
	initDependencies()
	cfg := config.Get()
	log.WithFields(log.Fields{
		"Hostname":                 cfg.Hostname,
		"Auth":                     cfg.Auth,
		"WebPort":                  cfg.WebPort,
		"MetricsPort":              cfg.MetricsPort,
		"LogLevel":                 cfg.LogLevel,
		"Debug":                    cfg.Debug,
		"BucketName":               cfg.BucketName,
		"BucketRegion":             cfg.BucketRegion,
		"RepoTempPath ":            cfg.RepoTempPath,
		"OpenAPIFilePath ":         cfg.OpenAPIFilePath,
		"ImageBuilderURL":          cfg.ImageBuilderConfig.URL,
		"DefaultOSTreeRef":         cfg.DefaultOSTreeRef,
		"InventoryURL":             cfg.InventoryConfig.URL,
		"PlaybookDispatcherConfig": cfg.PlaybookDispatcherConfig.URL,
		"TemplatesPath":            cfg.TemplatesPath,
		"DatabaseType":             cfg.Database.Type,
		"DatabaseName":             cfg.Database.Name,
		"IsKafkaEnabled":           cfg.KafkaConfig != nil,
		"FDOHostURL":               cfg.FDO.URL,
	}).Info("Configuration Values")

	r := chi.NewRouter()
	r.Use(
		request_id.ConfiguredRequestID("x-rh-insights-request-id"),
		middleware.RealIP,
		middleware.Recoverer,
		middleware.Logger,
		setupDocsMiddleware,
		dependencies.Middleware,
	)

	// Unauthenticated routes
	r.Get("/", routes.StatusOK)
	r.Get("/api/edge/v1/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, cfg.OpenAPIFilePath)
	})

	// Authenticated routes
	ar := r.Group(nil)
	if cfg.Auth {
		ar.Use(
			identity.EnforceIdentity,
			dependencies.Middleware,
		)
	}

	ar.Route("/api/edge/v1", func(s chi.Router) {
		s.Route("/images", routes.MakeImagesRouter)
		s.Route("/updates", routes.MakeUpdatesRouter)
		s.Route("/image-sets", routes.MakeImageSetsRouter)
		s.Route("/devices", routes.MakeDevicesRouter)
		s.Route("/thirdpartyrepo", routes.MakeThirdPartyRepoRouter)
		s.Route("/fdo", routes.MakeFDORouter)
		s.Route("/device-groups", routes.MakeDeviceGroupsRouter)
	})

	mr := chi.NewRouter()
	mr.Get("/", routes.StatusOK)
	mr.Handle("/metrics", promhttp.Handler())

	srv := http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.WebPort),
		Handler: r,
	}

	msrv := http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.MetricsPort),
		Handler: mr,
	}

	gracefulStop := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint
		log.Info("Shutting down gracefully...")
		if err := srv.Shutdown(context.Background()); err != nil {
			log.WithFields(log.Fields{"error": err}).Fatal("HTTP Server Shutdown failed")
		}
		if err := msrv.Shutdown(context.Background()); err != nil {
			log.WithFields(log.Fields{"error": err}).Fatal("HTTP Server Shutdown failed")
		}
		services.WaitGroup.Wait()
		close(gracefulStop)
	}()

	go func() {
		if err := msrv.ListenAndServe(); err != http.ErrServerClosed {
			log.WithFields(log.Fields{"error": err}).Fatal("Metrics Service Stopped")
		}
	}()
	if cfg.KafkaConfig != nil {
		log.Info("Starting Kafka Consumers")
		playbookConsumer := services.NewKafkaConsumerService(cfg.KafkaConfig, "platform.playbook-dispatcher.runs")
		go playbookConsumer.Start()
		platformInvConsumer := services.NewKafkaConsumerService(cfg.KafkaConfig, "platform.inventory.events")
		go platformInvConsumer.Start()

	}

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.WithFields(log.Fields{"error": err}).Fatal("Service Stopped")
	}

	<-gracefulStop
	log.Info("Everything has shut down, goodbye")
}
