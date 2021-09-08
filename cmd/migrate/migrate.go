package main

import (
	"github.com/redhatinsights/edge-api/config"
	l "github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

func main() {
	config.Init()
	l.InitLogger()
	cfg := config.Get()
	log.WithFields(log.Fields{
		"Hostname":           cfg.Hostname,
		"Auth":               cfg.Auth,
		"WebPort":            cfg.WebPort,
		"MetricsPort":        cfg.MetricsPort,
		"LogLevel":           cfg.LogLevel,
		"Debug":              cfg.Debug,
		"BucketName":         cfg.BucketName,
		"BucketRegion":       cfg.BucketRegion,
		"ImageBuilderConfig": cfg.ImageBuilderConfig.URL,
	}).Info("Configuration Values:")
	db.InitDB()
	err := db.DB.AutoMigrate(&models.ImageSet{}, &models.Commit{}, &models.UpdateTransaction{}, &models.Package{}, &models.Image{}, &models.Repo{}, &models.DispatchRecord{})
	if err != nil {
		panic(err)
	}
	log.Info("Migration Completed")
}
