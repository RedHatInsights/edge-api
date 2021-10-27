package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
)

type imageSetTypeKey int

const imageSetKey imageSetTypeKey = iota

var sort_option = []string{"created_at", "updated_at", "name", "status"}
var status_option = []string{models.ImageStatusCreated, models.ImageStatusBuilding, models.ImageStatusError, models.ImageStatusSuccess}

// MakeImageSetsRouter adds support for operations on image-sets
func MakeImageSetsRouter(sub chi.Router) {
	sub.With(validateFilterParams).With(common.Paginate).Get("/", ListAllImageSets)
	sub.Route("/{imageSetId}", func(r chi.Router) {
		r.Use(ImageSetCtx)
		r.Get("/", GetImageSetsByID)
	})
}

var imageSetFilters = common.ComposeFilters(
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "status",
		DBField:    "images.status",
	}),
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "name",
		DBField:    "image_sets.name",
	}),
	common.SortFilterHandler("image_sets", "created_at", "DESC"),
)

var imageStatusFilters = common.ComposeFilters(
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "status",
		DBField:    "images.status",
	}),
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "name",
		DBField:    "image_sets.name",
	}),
	common.SortFilterHandler("images", "created_at", "DESC"),
)

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
	result := imageSetFilters(r, db.DB)
	pagination := common.GetPagination(r)
	account, err := common.GetAccount(r)

	if err != nil {
		log.Info(err)
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}

	countResult := imageSetFilters(r, db.DB.Model(&models.ImageSet{})).Joins(`JOIN Images ON Image_Sets.id = Images.image_set_id AND Images.id = (Select Max(id) from Images where Images.image_set_id = Image_Sets.id)`).Where(`Image_Sets.account = ? `, account).Count(&count)
	if countResult.Error != nil {
		countErr := errors.NewInternalServerError()
		log.Error(countErr)
		w.WriteHeader(countErr.GetStatus())
		json.NewEncoder(w).Encode(&countErr)
		return
	}
	fmt.Printf("r.URL.Query() %v \n", r.URL.Query().Get("sort_by"))
	if r.URL.Query().Get("sort_by") != "status" && r.URL.Query().Get("sort_by") != "-status" {
		result = imageSetFilters(r, db.DB.Model(&models.ImageSet{})).Limit(pagination.Limit).Offset(pagination.Offset).Preload("Images").Joins(`JOIN Images ON Image_Sets.id = Images.image_set_id AND Images.id = (Select Max(id) from Images where Images.image_set_id = Image_Sets.id)`).Where(`Image_Sets.account = ? `, account).Find(&imageSet)

	} else {
		result = imageStatusFilters(r, db.DB.Model(&models.ImageSet{})).Limit(pagination.Limit).Offset(pagination.Offset).Preload("Images").Joins(`JOIN Images ON Image_Sets.id = Images.image_set_id AND Images.id = (Select Max(id) from Images where Images.image_set_id = Image_Sets.id)`).Where(`Image_Sets.account = ? `, account).Find(&imageSet)

	}

	if result.Error != nil {
		err := errors.NewBadRequest("Not Found")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
	}
	var response common.EdgeAPIPaginatedResponse
	response.Count = count
	response.Data = &imageSet
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

func validateFilterParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		errs := []common.ValidationError{}
		if statuses, ok := r.URL.Query()["status"]; ok {
			for _, status := range statuses {
				if !contains(status_option, strings.ToUpper(status)) {
					errs = append(errs, common.ValidationError{Key: "status", Reason: fmt.Sprintf("%s is not a valid status. Status must be %s", status, strings.Join(validStatuses, " or "))})
				}
			}
		}
		if val := r.URL.Query().Get("sort_by"); val != "" {
			name := val
			if string(val[0]) == "-" {
				name = val[1:]
			}
			if !contains(sort_option, name) {
				errs = append(errs, common.ValidationError{Key: "sort_by", Reason: fmt.Sprintf("%s is not a valid sort_by. Sort-by must %v", name, strings.Join(sort_option, " or "))})
			}
		}

		if len(errs) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(&errs)
	})
}

func contains(s []string, searchterm string) bool {
	for _, a := range s {
		if a == searchterm {
			return true
		}
	}
	return false
}
