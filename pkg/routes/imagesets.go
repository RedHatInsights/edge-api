package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/redhatinsights/edge-api/pkg/db"
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
		r.Use(ImageSetCtx)
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

// ImageCtx is a handler for Image requests
func ImageSetCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var imageSet models.ImageSet
		account, err := common.GetAccount(r)
		fmt.Printf("Account: %v\n: ", account)
		if err != nil {
			err := errors.NewBadRequest(err.Error())
			w.WriteHeader(err.Status)
			json.NewEncoder(w).Encode(&err)
			return
		}
		if imageSetID := chi.URLParam(r, "imageSetId"); imageSetID != "" {
			id, err := strconv.Atoi(imageSetID)
			if err != nil {
				err := errors.NewBadRequest(err.Error())
				w.WriteHeader(err.Status)
				json.NewEncoder(w).Encode(&err)
				return
			}
			result := db.DB.Where("id = ?", id).First(&imageSet)
			if result.Error != nil {
				err := errors.NewInternalServerError()
				w.WriteHeader(err.Status)
				json.NewEncoder(w).Encode(&err)
				return
			}
			ctx := context.WithValue(r.Context(), imageKey, &imageSet)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}
func GetImageSetByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	fmt.Printf("HASDJSAHDJKSAHDKJHA \n")
	imageSet, ok := ctx.Value(imageKey).(*models.ImageSet)
	if !ok {
		err := errors.NewBadRequest("Must pass image id")
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
	}
	if imageSet != nil {
		json.NewEncoder(w).Encode(imageSet)
	}
}
