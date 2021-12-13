package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm"

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

var sortOption = []string{"created_at", "updated_at", "name", "status"}
var statusOption = []string{models.ImageStatusCreated, models.ImageStatusBuilding, models.ImageStatusError, models.ImageStatusSuccess}

// MakeImageSetsRouter adds support for operations on image-sets
func MakeImageSetsRouter(sub chi.Router) {
	sub.With(validateFilterParams).With(common.Paginate).Get("/", ListAllImageSets) // TODO: Consistent logging
	sub.Route("/{imageSetId}", func(r chi.Router) {
		r.Use(ImageSetCtx)                                                            // TODO: Consistent logging
		r.With(validateFilterParams).With(common.Paginate).Get("/", GetImageSetsByID) // TODO: Consistent logging
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

var imageDetailFilters = common.ComposeFilters(
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "status",
		DBField:    "images.status",
	}),

	common.ContainFilterHandler(&common.Filter{
		QueryParam: "name",
		DBField:    "images.name",
	}),
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "version",
		DBField:    "images.version",
	}),
	common.SortFilterHandler("images", "updated_at", "DESC"),
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

// ImageSetCtx provides the handler for Image Sets
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
			if imageSet.Images != nil {
				db.DB.Where("image_set_id = ?", imageSetID).Find(&imageSet.Images)
				db.DB.Where("id = ?", &imageSet.Images[len(imageSet.Images)-1].InstallerID).Find(&imageSet.Images[len(imageSet.Images)-1].Installer)
			}
			ctx := context.WithValue(r.Context(), imageSetKey, &imageSet)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

// ListAllImageSets return the list of image sets and images
func ListAllImageSets(w http.ResponseWriter, r *http.Request) {
	var imageSet *[]models.ImageSet
	var count int64
	var result *gorm.DB
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
	log.Debugf("r.URL.Query() %v \n", r.URL.Query().Get("sort_by"))
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

//ImageSetImagePackages return info related to details on images from imageset
type ImageSetImagePackages struct {
	ImageSetData     models.ImageSet `json:"image_set"`
	Images           []ImageDetail   `json:"images"`
	ImageBuildISOURL string          `json:"image_build_iso_url"`
}

// GetImageSetsByID returns the list of Image Sets by a given Image Set ID
func GetImageSetsByID(w http.ResponseWriter, r *http.Request) {
	// var imageSetData models.ImageSet
	var images []models.Image
	var details ImageSetImagePackages
	var response common.EdgeAPIPaginatedResponse
	pagination := common.GetPagination(r)
	account, err := common.GetAccount(r)
	if err != nil {
		log.Info(err)
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	ctx := r.Context()
	imageSet, ok := ctx.Value(imageSetKey).(*models.ImageSet)
	if !ok {
		err := errors.NewBadRequest("Must pass image set id")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
	}
	result := imageDetailFilters(r, db.DB.Model(&models.Image{})).Limit(pagination.Limit).Offset(pagination.Offset).
		Preload("Commit.Repo").Preload("Commit.InstalledPackages").Preload("Installer").
		Joins(`JOIN Image_Sets ON Image_Sets.id = Images.image_set_id`).
		Where(`Image_Sets.account = ? and  Image_sets.id = ?`, account, &imageSet.ID).Find(&images)

	if result.Error != nil {
		err := errors.NewBadRequest("Error to filter images")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
	}

	s, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	Imgs := returnImageDetails(images, s)

	details.ImageSetData = *imageSet
	details.Images = Imgs

	if Imgs != nil && Imgs[len(Imgs)-1].Image != nil && Imgs[len(Imgs)-1].Image.InstallerID != nil {
		img := Imgs[len(Imgs)-1].Image
		details.ImageBuildISOURL = img.Installer.ImageBuildISOURL
	}

	response.Data = &details
	response.Count = int64(len(images))
	json.NewEncoder(w).Encode(response)
}

func validateFilterParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		errs := []common.ValidationError{}
		if statuses, ok := r.URL.Query()["status"]; ok {
			for _, status := range statuses {
				if !contains(statusOption, strings.ToUpper(status)) {
					errs = append(errs, common.ValidationError{Key: "status", Reason: fmt.Sprintf("%s is not a valid status. Status must be %s", status, strings.Join(validStatuses, " or "))})
				}
			}
		}
		if val := r.URL.Query().Get("sort_by"); val != "" {
			name := val
			if string(val[0]) == "-" {
				name = val[1:]
			}
			if !contains(sortOption, name) {
				errs = append(errs, common.ValidationError{Key: "sort_by", Reason: fmt.Sprintf("%s is not a valid sort_by. Sort-by must %v", name, strings.Join(sortOption, " or "))})
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

func returnImageDetails(images []models.Image, s *dependencies.EdgeAPIServices) []ImageDetail {
	var Imgs []ImageDetail

	for idx, i := range images {
		err := db.DB.Model(i).Association("Packages").Find(&images[idx].Packages)
		if err != nil {
			return nil
		}
		img, err := s.ImageService.AddPackageInfo(&images[idx])

		if err != nil {
			log.Error("Image detail not found \n")
		}
		Imgs = append(Imgs, ImageDetail(img))
	}

	return Imgs
}
