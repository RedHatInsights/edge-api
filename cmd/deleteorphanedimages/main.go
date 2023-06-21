package deleteorphanedimages

import (
	"errors"
	"os"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// FEATURE_DELETE_ORPHANED_IMAGES=1 go run cmd/deleteorphanedimages/main.go
func main() {
	if feature.DeleteOrphanedImages.IsEnabled() == false {
		message := "delete orphaned images feature is disabled"
		logrus.Error(message)
		cleanupAndExit(errors.New(message))
	}

	config.Init()
	logger.InitLogger(os.Stdout)
	logrus.Info("Starting deletion of orphaned images...")
	db.InitDB()

	var orphanedImages []models.Image
	var count int64

	r := findOrphanedImages(db.DB, &count, &orphanedImages)

	if r.Error != nil {
		logrus.WithError(r.Error).Error("error when retrieving images")
		cleanupAndExit(r.Error)
	}

	logrus.Info("Found ", count, " orphaned images")
	if count == 0 {
		logrus.Info("No orphaned images to delete. Exiting...")
		cleanupAndExit(nil)
	}

	db.DB.Delete(&orphanedImages)
	logrus.WithFields(logrus.Fields{"count": count}).Info("finished deleting orphaned images")
	cleanupAndExit(nil)
}

func cleanupAndExit(err error) {
	// flush logger before app exit
	logger.FlushLogger()
	if err != nil {
		logrus.Exit(2)
	}
	logrus.Exit(0)
}

func findOrphanedImages(
	db *gorm.DB,
	count *int64,
	orphanedImages *[]models.Image,
) *gorm.DB {
	return db.Debug().
		Model(&models.ImageSet{}).
		Joins("JOIN images ON image_sets.id=images.image_set_id").Unscoped().
		Where("image_sets.deleted_at IS NOT NULL AND images.deleted_at IS NULL").
		Find(&orphanedImages)
}
