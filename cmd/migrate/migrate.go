package main

import (
	"github.com/redhatinsights/edge-api/config"
	l "github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/commits"
	"github.com/redhatinsights/edge-api/pkg/db"
	log "github.com/sirupsen/logrus"
)

func main() {
	config.Init()
	l.InitLogger()
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
	db.InitDB()
	err := db.DB.AutoMigrate(&commits.Commit{}, &commits.UpdateRecord{})
	if err != nil {
		panic(err)
	}
	log.Info("Migration Completed")
}
