package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/redhatinsights/edge-api/config"
	l "github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/commits"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/repo"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/platform-go-middlewares/request_id"
	log "github.com/sirupsen/logrus"
)

func main() {
	config.Init()
	l.InitLogger()
	db.InitDB()

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
	)

	if cfg.Auth {
		r.Use(identity.EnforceIdentity)
	}

	r.Get("/", common.StatusOK)

	var server repo.Server
	server = &repo.FileServer{
		BasePath: "/tmp",
	}

	r.Route("/api/edge/v1", func(s chi.Router) {
		s.Route("/commits", commits.MakeRouter)
		s.Route("/repos", repo.MakeRouter(server))
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
