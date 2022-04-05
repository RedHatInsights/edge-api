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

	/*
		// FIXME: this can create issues when only one out of many replicas evicts
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

		// FIXME: this runs into an issue when only one of many pods is evicted and restarts...
		// If there any updates in progress, in the current architecture, we need to set them as errors because this is a brand new deployment
		var updates []models.UpdateTransaction
		db.DB.Where(&models.UpdateTransaction{Status: models.UpdateStatusBuilding}).Or(&models.UpdateTransaction{Status: models.UpdateStatusCreated}).Find(&updates)
		for _, update := range updates {
			log.WithField("updateID", update.ID).Debug("Found update with building status")
			update.Status = models.UpdateStatusError
			if update.Repo != nil {
				update.Repo.Status = models.RepoStatusError
				db.DB.Save(update.Repo)
			}
			db.DB.Save(update)
		}
	*/
	// Automigration
	// Order should match Deleting of models in cmd/db/wipe.go
	// Order is not strictly alphabetical due to dependencies (e.g. Image needs ImageSet)
	err := db.DB.Debug().AutoMigrate(&models.Commit{},
		&models.DeviceGroup{},
		&models.DispatchRecord{},
		&models.FDODevice{},
		&models.FDOUser{},
		&models.ImageSet{},
		&models.Image{},
		&models.Installer{},
		&models.OwnershipVoucherData{},
		&models.Package{},
		&models.Repo{},
		&models.SSHKey{},
		&models.ThirdPartyRepo{},
		&models.UpdateTransaction{})
	if err != nil {
		l.LogErrorAndPanic("database automigrate failure", err)
	}

	log.Info("Migration Completed")
}
