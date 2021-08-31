package routes

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
)

// MakeImageSetsRouter adds support for operations on image-sets
func MakeImageSetsRouter(sub chi.Router) {
	sub.With(validateGetAllImagesSearchParams).With(common.Paginate).Get("/", ImageSets)
	sub.Get("/reserved-usernames", GetReservedUsernames)

}

// ImageSets image objects from the database for ParentID
func ImageSets(w http.ResponseWriter, r *http.Request) {
	var count int64
	var images []models.Image
	var image models.Image
	result := imageFilters(r, db.DB)
	pagination := common.GetPagination(r)

	countResult := imageFilters(r, db.DB.Model(&models.Image{})).Where("images.parent_id is ?", image.ParentId).Count(&count)
	if countResult.Error != nil {
		countErr := errors.NewInternalServerError()
		log.Error(countErr)
		w.WriteHeader(countErr.Status)
		json.NewEncoder(w).Encode(&countErr)
		return
	}
	result = result.Limit(pagination.Limit).Offset(pagination.Offset).Where("images.parent_id is ?", image.ParentId).Find(&images)
	if result.Error != nil {
		// log.Error(err)
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"data": &images, "count": count})
}
