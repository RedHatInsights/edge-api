package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/redhatinsights/edge-api/pkg/db"
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var imageSet models.ImageSet
		account, err := common.GetAccount(r)
		if err != nil {
			log.Info(err)
			err := errors.NewBadRequest(err.Error())
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
		if imageSetID := chi.URLParam(r, "imageSetId"); imageSetID != "" {
			_, err := strconv.Atoi(imageSetID)
			if err != nil {
				err := errors.NewBadRequest(err.Error())
				w.WriteHeader(err.GetStatus())
				json.NewEncoder(w).Encode(&err)
				return
			}
			result := db.DB.Where("account = ? and Image_sets.id = ?", account, imageSetID).Find(&imageSet)
			if result.Error != nil {
				err := errors.NewNotFound(result.Error.Error())
				w.WriteHeader(err.GetStatus())
				json.NewEncoder(w).Encode(&err)
				return
			}
			db.DB.Where("image_set_id = ?", imageSetID).Find(&imageSet.Images)
			ctx := context.WithValue(r.Context(), imageSetKey, &imageSet)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

// ListAllImageSets return the list of image sets and images
func ListAllImageSets(w http.ResponseWriter, r *http.Request) {
	var imageSet *[]models.ImageSet
	var count int64
	pagination := common.GetPagination(r)
	account, err := common.GetAccount(r)

	if err != nil {
		log.Info(err)
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}

	countResult := db.DB.Model(&models.ImageSet{}).Where("account = ?", account).Count(&count)
	if countResult.Error != nil {
		countErr := errors.NewInternalServerError()
		log.Error(countErr)
		w.WriteHeader(countErr.GetStatus())
		json.NewEncoder(w).Encode(&countErr)
		return
	}
	result := db.DB.Limit(pagination.Limit).Offset(pagination.Offset).Preload("Images").Where("account = ? ", account).Find(&imageSet)
	if result.Error != nil {
		err := errors.NewBadRequest("Not Found")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
	}
	var response common.EdgeAPIPaginatedResponse
	response.Data = &imageSet
	response.Count = count
	json.NewEncoder(w).Encode(response)

}

func GetImageSetsByID(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	imageSet, ok := ctx.Value(imageSetKey).(*models.ImageSet)
	if !ok {
		err := errors.NewBadRequest("Must pass image set id")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
	}
	json.NewEncoder(w).Encode(&imageSet)

}
