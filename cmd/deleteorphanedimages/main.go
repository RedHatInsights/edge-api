package main

import (
	"errors"
	"os"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	"github.com/sirupsen/logrus"
)

func main() {
	if !feature.DeleteOrphanedImages.IsEnabled() {
		message := "delete orphaned images feature is disabled"
		logrus.Error(message)
		cleanupAndExit(errors.New(message))
	}

	config.Init()
	logger.InitLogger(os.Stdout)
	logrus.Info("Starting deletion of orphaned images...")
	db.InitDB()

	orphanedImagesQuery := db.DB.Debug().
		Model(&models.ImageSet{}).
		Select("count(images.id)").
		Joins("JOIN images ON image_sets.id=images.image_set_id").
		Where("deleted_at IS NOT NULL AND images.deleted_at IS NULL")

	if orphanedImagesQuery.Error != nil {
		logrus.WithError(orphanedImagesQuery.Error).Error("error when retrieving images")
		cleanupAndExit(orphanedImagesQuery.Error)
	}

	var count int64
	orphanedImagesQuery.Count(&count)
	logrus.Info("Found ", count, " orphaned images")

	if count == 0 {
		logrus.Info("No orphaned images to delete. Exiting...")
		cleanupAndExit(nil)
	}
}

func cleanupAndExit(err error) {
	// flush logger before app exit
	logger.FlushLogger()
	if err != nil {
		logrus.Exit(2)
	}
	logrus.Exit(0)
}
