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
	log "github.com/sirupsen/logrus"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
)

type imageSetTypeKey int

const imageSetKey imageSetTypeKey = iota

// MakeImageSetsRouter adds support for operations on image-sets
func MakeImageSetsRouter(sub chi.Router) {
	sub.With(common.Paginate).Get("/", ListAllImageSets)
	sub.Route("/{imageSetId}", func(r chi.Router) {
		r.Use(ImageSetCtx)
		r.Get("/", GetImageSetsByID)
	})
}

func ImageSetCtx(next http.Handler) http.Handler {
	var imageSet models.ImageSet
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account, err := common.GetAccount(r)
		if err != nil {
			log.Info(err)
			err := errors.NewBadRequest(err.Error())
			w.WriteHeader(err.Status)
			json.NewEncoder(w).Encode(&err)
			return
		}
		if imageSetID := chi.URLParam(r, "imageSetId"); imageSetID != "" {
			_, err := strconv.Atoi(imageSetID)
			if err != nil {
				err := errors.NewBadRequest(err.Error())
				w.WriteHeader(err.Status)
				json.NewEncoder(w).Encode(&err)
				return
			}
			result := db.DB.Where("account = ? and Image_sets.id = ?", account, imageSetID).Find(&imageSet)
			if result.Error != nil {
				err := errors.NewNotFound(result.Error.Error())
				w.WriteHeader(err.Status)
				json.NewEncoder(w).Encode(&err)
				return
			}
			db.DB.Where("image_set_id = ?", imageSetID).Find(&imageSet.Images)
			ctx := context.WithValue(r.Context(), imageSetKey, &imageSet)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
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

	ctx := r.Context()
	imageSet, ok := ctx.Value(imageSetKey).(*models.ImageSet)
	if !ok {
		err := errors.NewBadRequest("Must pass image set id")
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
	}
	json.NewEncoder(w).Encode(&imageSet)

}
