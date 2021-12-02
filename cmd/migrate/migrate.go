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
	}).Info("Configuration Values:")
	db.InitDB()

	// If there any image builds in progress, in the current architecture, we need to set them as errors because this is a brand new deployment
	var images []models.Image
	db.DB.Where(&models.Image{Status: models.ImageStatusBuilding}).Find(&images)
	for _, image := range images {
		log.WithField("imageID", image.ID).Debug("Found image with building status")
		image.Status = models.ImageStatusError
		if image.Commit != nil {
			image.Commit.Status = models.ImageStatusError
			if image.Commit.Repo != nil {
				image.Commit.Repo.Status = models.RepoStatusError
				db.DB.Save(image.Commit.Repo)
			}
			db.DB.Save(image.Commit)
		}
		if image.Installer != nil {
			image.Installer.Status = models.ImageStatusError
			db.DB.Save(image.Installer)
		}
		db.DB.Save(image)
	}

	// Automigration
	err := db.DB.AutoMigrate(&models.ImageSet{},
		&models.Commit{}, &models.UpdateTransaction{},
		&models.Package{}, &models.Image{}, &models.Repo{},
		&models.DispatchRecord{}, &models.ThirdPartyRepo{},
		&models.OwnershipVoucherData{})
	if err != nil {
		panic(err)
	}
	log.Info("Migration Completed")
}
