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

	redoc "github.com/go-openapi/runtime/middleware"
	"github.com/redhatinsights/edge-api/config"
	l "github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/commits"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/imagebuilder"
	"github.com/redhatinsights/edge-api/pkg/images"
	"github.com/redhatinsights/edge-api/pkg/repo"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/platform-go-middlewares/request_id"
	log "github.com/sirupsen/logrus"
)

func setupDocsMiddleware(handler http.Handler) http.Handler {
	opt := redoc.RedocOpts{
		SpecURL: "./openapi.json",
	}
	return redoc.Redoc(opt, handler)
}

func initDependencies() {
	config.Init()
	l.InitLogger()
	db.InitDB()
	imagebuilder.InitClient()
}

func main() {
	initDependencies()
	cfg := config.Get()
	log.WithFields(log.Fields{
		"Hostname":    cfg.Hostname,
		"Auth":        cfg.Auth,
		"WebPort":     cfg.WebPort,
		"MetricsPort": cfg.MetricsPort,
		"LogLevel":    cfg.LogLevel,
		"Debug":       cfg.Debug,
		"BucketName":  cfg.BucketName,
	}).Info("Configuration Values:")

	r := chi.NewRouter()
	r.Use(
		request_id.ConfiguredRequestID("x-rh-insights-request-id"),
		middleware.RealIP,
		middleware.Recoverer,
		middleware.Logger,
		setupDocsMiddleware,
	)

	if cfg.Auth {
		r.Use(identity.EnforceIdentity)
	}

	r.Get("/", common.StatusOK)
	r.Get("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./cmd/spec/openapi.json")
	})

	var server repo.Server
	server = &repo.FileServer{
		BasePath: "/tmp",
	}
	if cfg.BucketName != "" {
		server = repo.NewS3Proxy()
	}

	r.Route("/api/edge/v1", func(s chi.Router) {
		s.Route("/commits", commits.MakeRouter)
		s.Route("/repos", repo.MakeRouter(server))
		s.Route("/images", images.MakeRouter)
	})

	mr := chi.NewRouter()
	mr.Get("/", common.StatusOK)
	mr.Handle("/metrics", promhttp.Handler())

	srv := http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.WebPort),
		Handler: r,
	}

	msrv := http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.MetricsPort),
		Handler: mr,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint
		if err := srv.Shutdown(context.Background()); err != nil {
			log.WithFields(log.Fields{"error": err}).Fatal("HTTP Server Shutdown failed")
		}
		if err := msrv.Shutdown(context.Background()); err != nil {
			log.WithFields(log.Fields{"error": err}).Fatal("HTTP Server Shutdown failed")
		}
		close(idleConnsClosed)
	}()

	go func() {

		if err := msrv.ListenAndServe(); err != http.ErrServerClosed {
			log.WithFields(log.Fields{"error": err}).Fatal("Metrics Service Stopped")
		}
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.WithFields(log.Fields{"error": err}).Fatal("Service Stopped")
	}

	<-idleConnsClosed
	log.Info("Everything has shut down, goodbye")
}
