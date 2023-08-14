package deleteimages

import (
	"errors"
	"strings"
	"time"

	"github.com/redhatinsights/edge-api/pkg/models"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ErrDeleteImagesCleanUpNotAvailable error returned when the delete images clean up feature flag is disabled
var ErrDeleteImagesCleanUpNotAvailable = errors.New("delete images cleanup is not available")

// DeleteImagesOlderThan delete only the images that are older than this value in days, default value is 7 days
var DeleteImagesOlderThan = 7 * 24 * time.Hour

var ImagesWithNamesToKeep = []string{
	"DL-",
	"IQE-TEST-IMAGE-",
	"PopcornOS",
}

// deleteOrphanImages soft delete images that have image-set soft deleted
func deleteOrphanImages(gormDB *gorm.DB) error {

	imagesCollection := gormDB.Select("images.id").
		Joins("JOIN image_sets ON image_sets.id = images.image_set_id").
		Where("images.deleted_at IS NULL AND image_sets.deleted_at IS NOT NULL").
		Table("images")

	result := gormDB.Debug().Where("images.id IN (?) ", imagesCollection).Delete(&models.Image{})
	if result.Error != nil {
		log.WithField("error", result.Error.Error()).Error("error occurred while deleting orphan images")
		return result.Error
	}
	log.WithField("images-deleted", result.RowsAffected).Info("orphan images deleted successfully")
	return nil
}

// deleteImages soft delete all images that are not in names to keep and older than one week
func deleteImages(gormDB *gorm.DB) error {

	imagesCollection := gormDB.Table("images").Select("images.id").
		Joins("LEFT JOIN devices ON devices.image_id = images.id").
		Where("images.deleted_at IS NULL AND devices.id IS NULL").
		Where("images.updated_at < ? ", time.Now().Add(-DeleteImagesOlderThan))
	// build images names to keep
	for _, name := range ImagesWithNamesToKeep {
		imagesCollection = imagesCollection.Where("upper(images.name) NOT LIKE ?", strings.ToUpper(name)+"%")
	}

	result := gormDB.Debug().Where("images.id IN (?)", imagesCollection).Delete(&models.Image{})
	if result.Error != nil {
		log.WithField("error", result.Error.Error()).Error("error occurred while deleting images")
		return result.Error
	}

	log.WithField("images-deleted", result.RowsAffected).Info("images deleted successfully")
	return nil
}

func DeleteAllImages(gormDB *gorm.DB) error {
	if !feature.CleanUPDeleteImages.IsEnabled() {
		log.Warning("flag is disabled for cleanup delete images feature")
		return ErrDeleteImagesCleanUpNotAvailable
	}

	if err := deleteOrphanImages(gormDB); err != nil {
		return err
	}

	return deleteImages(gormDB)
}
