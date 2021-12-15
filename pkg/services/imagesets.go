package services

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
)

// ImageSetsServiceInterface defines the interface that helps handle
// the business logic of ImageSets
type ImageSetsServiceInterface interface {
	ListAllImageSets(w http.ResponseWriter, r *http.Request) error
	GetImageSetsByID(imageSetID int) (*models.ImageSet, error)
}

// NewImageSetsService gives a instance of the main implementation of a ImageSetsServiceInterface
func NewImageSetsService(ctx context.Context) ImageSetsServiceInterface {
	return &ImageSetsService{
		ctx: ctx,
	}
}

// ImageSetsService is the main implementation of a ImageSetsServiceInterface
type ImageSetsService struct {
	ctx context.Context
}

// ListAllImageSets to org group of images into one
func (s *ImageSetsService) ListAllImageSets(w http.ResponseWriter, r *http.Request) error {

	var imageFilters = common.ComposeFilters(
		common.OneOfFilterHandler(&common.Filter{
			QueryParam: "status",
			DBField:    "images.status",
		}),
		common.ContainFilterHandler(&common.Filter{
			QueryParam: "name",
			DBField:    "images.name",
		}),
		common.ContainFilterHandler(&common.Filter{
			QueryParam: "distribution",
			DBField:    "images.distribution",
		}),
		common.CreatedAtFilterHandler(&common.Filter{
			QueryParam: "created_at",
			DBField:    "images.created_at",
		}),
		common.SortFilterHandler("images", "created_at", "DESC"),
	)
	var count int64
	var images []models.Image
	var image models.Image
	result := imageFilters(r, db.DB)
	pagination := common.GetPagination(r)

	countResult := imageFilters(r, db.DB.Model(&models.Image{})).Where("images.Image_set_id  is ?", image.ImageSetID).Count(&count)
	if countResult.Error != nil {
		countErr := errors.NewInternalServerError()
		log.Error(countErr)
		w.WriteHeader(countErr.GetStatus())
		_ = json.NewEncoder(w).Encode(&countErr)
	}
	result = result.Limit(pagination.Limit).Offset(pagination.Offset).Where("images.Image_set_id  is ?", image.ImageSetID).Find(&images)
	if result.Error != nil {
		err := errors.NewInternalServerError()
		w.WriteHeader(err.GetStatus())
		_ = json.NewEncoder(w).Encode(&err)
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": &images, "count": count})
	return nil
}

// GetImageSetsByID to get image set by id
func (s *ImageSetsService) GetImageSetsByID(imageSetID int) (*models.ImageSet, error) {
	var imageSet models.ImageSet
	result := db.DB.Where("Image_sets.id = ?", imageSetID).Find(&imageSet)
	db.DB.Where("image_set_id = ?", imageSetID).Find(&imageSet.Images)
	if result.Error != nil {
		err := errors.NewInternalServerError()
		return nil, err
	}
	return &imageSet, nil
}
