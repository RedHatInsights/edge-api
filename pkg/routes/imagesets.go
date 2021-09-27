package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
)

type imageSetTypeKey int

const imageSetId imageSetTypeKey = iota

// MakeImageSetsRouter adds support for operations on image-sets
func MakeImageSetsRouter(sub chi.Router) {
	sub.With(common.Paginate).Get("/", ListAllImageSets)
	sub.Route("/{imageSetId}", func(r chi.Router) {
		r.Get("/", GetImageSetsByID)
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

func GetImageSetsByID(w http.ResponseWriter, r *http.Request) {
	fmt.Print("Im here \n")
	var imageSet *models.ImageSet
	imageSetID := chi.URLParam(r, "imageSetId")
	fmt.Printf("::imagesetid: %v\n", imageSetID)
	id, err := strconv.Atoi(imageSetID)
	fmt.Printf(":: id:: %v\n", id)
	if err != nil {
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	services, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	imageSet, err = services.ImageSetService.GetImageSetsByID(id)
	fmt.Printf("imageSet %v\n", imageSet)
	fmt.Printf("Err: %v\n", err)
	if err != nil {
		err := errors.NewNotFound(fmt.Sprintf("Image is not found for: #%v Image Set ID", imageSet.ID))
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	json.NewEncoder(w).Encode(&imageSet)

}
