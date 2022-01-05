package services

import (
	"context"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// ImageSetsServiceInterface defines the interface that helps handle
// the business logic of ImageSets
type ImageSetsServiceInterface interface {
	GetImageSetsByID(imageSetID int) (*models.ImageSet, error)
}

// NewImageSetsService gives a instance of the main implementation of a ImageSetsServiceInterface
func NewImageSetsService(ctx context.Context) ImageSetsServiceInterface {
	return &ImageSetsService{
		Service: Service{ctx: ctx, log: log.WithField("service", "image-sets")},
	}
}

// ImageSetsService is the main implementation of a ImageSetsServiceInterface
type ImageSetsService struct {
	Service
}

// GetImageSetsByID to get image set by id
func (s *ImageSetsService) GetImageSetsByID(imageSetID int) (*models.ImageSet, error) {
	var imageSet models.ImageSet
	result := db.DB.Where("Image_sets.id = ?", imageSetID).Find(&imageSet)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error getting image set by id")
		err := errors.NewInternalServerError()
		return nil, err
	}
	result = db.DB.Where("image_set_id = ?", imageSetID).Find(&imageSet.Images)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error getting image set's images")
		err := errors.NewInternalServerError()
		return nil, err
	}
	return &imageSet, nil
}
