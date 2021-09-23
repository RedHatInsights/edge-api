package routes

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
)

// MakeImageSetsRouter adds support for operations on image-sets
func MakeImageSetsRouter(sub chi.Router) {
	sub.With(common.Paginate).Get("/", ListAllImageSets)
	sub.Route("/{imageSetId}", func(r chi.Router) {
		r.Get("/", GetImageSetByID)
	})
}

// ListAllImageSets image objects from the database for ParentID
func ListAllImageSets(w http.ResponseWriter, r *http.Request) {
	var image *models.Image
	services, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	err := services.ImageSetService.ListAllImageSets(w, r)
	if err != nil {
		err := errors.NewNotFound(fmt.Sprintf("Image is not found for: #%v Image Set ID", image.ImageSetID))
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	json.NewEncoder(w).Encode(err)
}

func GetImageSetByID(w http.ResponseWriter, r *http.Request) {
	var imageSet *models.ImageSet
	services, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	err := services.ImageSetService.GetImageSetsByID(w, r)
	if err != nil {
		err := errors.NewNotFound(fmt.Sprintf("Image is not found for: #%v Image Set ID", imageSet.ID))
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	json.NewEncoder(w).Encode(err)

}
