package services

import (
	"context"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
)

// ImageSetsServiceInterface defines the interface that helps handle
// the business logic of ImageSets
type ImageSetsServiceInterface interface {
	GetImageSetsByID(imageSetID int) (*models.ImageSet, error)
}

// NewImageSetsService gives a instance of the main implementation of a ImageSetsServiceInterface
func NewImageSetsService(ctx context.Context, log *log.Entry) ImageSetsServiceInterface {
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
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		s.log.WithField("error", err).Error("Error retrieving org_id")
		return nil, new(OrgIDNotSet)
	}
	result := db.Org(orgID, "image_sets").Debug().First(&imageSet, imageSetID)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error getting image set by id")
		return nil, new(ImageSetNotFoundError)
	}
	result = db.Org(orgID, "").Debug().Where("image_set_id = ?", imageSetID).Find(&imageSet.Images)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error getting image set's images")
		return nil, new(ImageNotFoundError)
	}
	return &imageSet, nil
}
