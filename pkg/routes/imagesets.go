// FIXME: golangci-lint
// nolint:gocritic,govet,revive
package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type imageSetTypeKey int
type imageSetImageTypeKey int

const imageSetKey imageSetTypeKey = iota
const imageSetImageKey imageSetImageTypeKey = iota

var sortOption = []string{"created_at", "updated_at", "name"}
var statusOption = []string{models.ImageStatusCreated, models.ImageStatusBuilding, models.ImageStatusError, models.ImageStatusSuccess}

// MakeImageSetsRouter adds support for operations on image-sets
func MakeImageSetsRouter(sub chi.Router) {
	sub.With(ValidateQueryParams("image-sets")).With(validateFilterParams).With(common.Paginate).Get("/", ListAllImageSets)
	sub.With(ValidateQueryParams("image-sets")).With(validateFilterParams).With(common.Paginate).Get("/view", GetImageSetsView)
	sub.Route("/{imageSetID}", func(r chi.Router) {
		r.Use(ImageSetCtx)
		r.With(validateFilterParams).With(common.Paginate).Get("/", GetImageSetsByID)
	})
	sub.With(validateFilterParams).Route("/view/{imageSetID}", func(r chi.Router) {
		r.Use(ImageSetViewCtx)
		r.With(ValidateGetAllImagesSearchParams).With(common.Paginate).Get("/", GetImageSetViewByID)
		r.With(ValidateGetAllImagesSearchParams).With(common.Paginate).Get("/versions", GetAllImageSetImagesView)
		r.Route("/versions/{imageID}", func(rVersion chi.Router) {
			rVersion.Use(ImageSetImageViewCtx)
			rVersion.Get("/", GetImageSetImageView)
		})
	})
}

func getStorageInstallerIsoURL(installerID uint) string {
	return services.GetStorageInstallerIsoURL(installerID)
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
	common.IntegerNumberFilterHandler(&common.Filter{
		QueryParam: "id",
		DBField:    "image_sets.id",
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
	common.IntegerNumberFilterHandler(&common.Filter{
		QueryParam: "version",
		DBField:    "images.version",
	}),
	common.SortFilterHandler("images", "created_at", "DESC"),
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
		s := dependencies.ServicesFromContext(r.Context())
		var imageSet models.ImageSet
		orgID := readOrgID(w, r, s.Log)
		if orgID == "" {
			return
		}
		if imageSetID := chi.URLParam(r, "imageSetID"); imageSetID != "" {
			s.Log = s.Log.WithField("imageSetID", imageSetID)
			_, err := strconv.Atoi(imageSetID)
			if err != nil {
				err := errors.NewBadRequest(err.Error())
				w.WriteHeader(err.GetStatus())
				if err := json.NewEncoder(w).Encode(&err); err != nil {
					s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
				}
				return
			}
			result := db.Org(orgID, "").Where("Image_sets.id = ?", imageSetID).First(&imageSet)

			if result.Error != nil {
				err := errors.NewNotFound(result.Error.Error())
				w.WriteHeader(err.GetStatus())
				if err := json.NewEncoder(w).Encode(&err); err != nil {
					s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
				}
				return
			}
			if imageSet.Images != nil {
				result := db.DB.Where("image_set_id = ?", imageSetID).Find(&imageSet.Images)
				if result.Error != nil {
					s.Log.WithField("error", result.Error.Error()).Debug("Result error")
					err := errors.NewBadRequest(result.Error.Error())
					w.WriteHeader(err.GetStatus())
					if err := json.NewEncoder(w).Encode(&err); err != nil {
						s.Log.WithField("error", result.Error.Error()).Error("Error while trying to encode")
					}
					return
				}
				db.DB.Where("id = ?", &imageSet.Images[len(imageSet.Images)-1].InstallerID).Find(&imageSet.Images[len(imageSet.Images)-1].Installer)
			}
			ctx := context.WithValue(r.Context(), imageSetKey, &imageSet)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

// ImageSetInstallerURL returns Imageset structure with last installer available
type ImageSetInstallerURL struct {
	ImageSetData     models.ImageSet `json:"image_set"`
	ImageBuildISOURL *string         `json:"image_build_iso_url"`
}

// ListAllImageSets return the list of image sets and images
func ListAllImageSets(w http.ResponseWriter, r *http.Request) {
	s := dependencies.ServicesFromContext(r.Context())

	var imageSet []models.ImageSet
	var imageSetInfo []ImageSetInstallerURL
	var count int64
	var result *gorm.DB
	pagination := common.GetPagination(r)
	orgID := readOrgID(w, r, s.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return
	}

	countResult := imageSetFilters(r, db.OrgDB(orgID, db.DB, "Image_Sets").Debug().Model(&models.ImageSet{})).
		Joins(`JOIN Images ON Image_Sets.id = Images.image_set_id`).Distinct("image_sets.id").Count(&count)

	if countResult.Error != nil {
		s.Log.WithField("error", countResult.Error.Error()).Error("Error counting results for image sets list")
		countErr := errors.NewInternalServerError()
		w.WriteHeader(countErr.GetStatus())
		if err := json.NewEncoder(w).Encode(&countErr); err != nil {
			s.Log.WithField("error", countErr.Error()).Error("Error while trying to encode")
		}
		return
	}

	if r.URL.Query().Get("sort_by") != "-status" && r.URL.Query().Get("sort_by") != "status" {
		result = imageSetFilters(r, db.OrgDB(orgID, db.DB, "Image_Sets").Debug().Model(&models.ImageSet{})).Distinct("image_sets.*").
			Limit(pagination.Limit).Offset(pagination.Offset).
			Preload("Images").
			Preload("Images.Commit").
			Preload("Images.Installer").
			Preload("Images.Commit.Repo").
			Joins(`JOIN Images ON Image_Sets.id = Images.image_set_id`).
			Find(&imageSet)
	} else {
		// this code is no longer run, but would be used if sorting by status is re-implemented.
		result = imageStatusFilters(r, db.OrgDB(orgID, db.DB, "Image_Sets").Debug().Model(&models.ImageSet{})).Distinct("image_sets.*").
			Limit(pagination.Limit).Offset(pagination.Offset).
			Preload("Images", "lower(status) in (?)", strings.ToLower(r.URL.Query().Get("status"))).
			Preload("Images.Commit").
			Preload("Images.Installer").
			Preload("Images.Commit.Repo").
			Joins(`JOIN Images ON Image_Sets.id = Images.image_set_id`).
			Joins("Commit").Joins("Installer").
			Find(&imageSet)

	}
	if result.Error != nil {
		s.Log.WithField("error", result.Error.Error()).Error("Image sets not found")
		err := errors.NewBadRequest("Not Found")
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	for idx, img := range imageSet {
		var imgSet ImageSetInstallerURL
		imgSet.ImageSetData = imageSet[idx]
		sort.Slice(img.Images, func(i, j int) bool {
			return img.Images[i].ID > img.Images[j].ID
		})
		imageSetIsoURLSetten := false
		for _, i := range img.Images {
			if i.InstallerID != nil {
				if i.Installer == nil {
					result = db.DB.First(&i.Installer, &i.InstallerID)
				}
				if i.Installer.ImageBuildISOURL != "" {
					installerIsoURL := getStorageInstallerIsoURL(i.Installer.ID)
					if !imageSetIsoURLSetten {
						// imageSet iso url should be set from the latest image installer
						// e.g. the first one defined in this list
						imgSet.ImageBuildISOURL = &installerIsoURL
						imageSetIsoURLSetten = true
					}
					// update the image installer iso url
					i.Installer.ImageBuildISOURL = installerIsoURL
				}
			}
		}
		imageSetInfo = append(imageSetInfo, imgSet)
	}
	if result.Error != nil {
		s.Log.WithField("error", result.Error.Error()).Error("Image sets not found")
		err := errors.NewBadRequest("Not Found")
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}

	if err := json.NewEncoder(w).Encode(&common.EdgeAPIPaginatedResponse{
		Count: count,
		Data:  imageSetInfo,
	}); err != nil {
		s.Log.WithField("error", &common.EdgeAPIPaginatedResponse{
			Count: count,
			Data:  imageSetInfo,
		}).Error("Error while trying to encode")
	}
}

// ImageSetImagePackages return info related to details on images from imageset
type ImageSetImagePackages struct {
	ImageSetData     models.ImageSet `json:"image_set"`
	Images           []ImageDetail   `json:"images"`
	ImageBuildISOURL string          `json:"image_build_iso_url"`
}

// GetImageSetsByID returns the list of Image Sets by a given Image Set ID
func GetImageSetsByID(w http.ResponseWriter, r *http.Request) {
	var images []models.Image
	var details ImageSetImagePackages
	s := dependencies.ServicesFromContext(r.Context())

	pagination := common.GetPagination(r)
	orgID := readOrgID(w, r, s.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return
	}
	ctx := r.Context()
	imageSet, ok := ctx.Value(imageSetKey).(*models.ImageSet)
	if !ok {
		err := errors.NewBadRequest("Must pass image set id")
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
	}
	result := imageDetailFilters(r, db.OrgDB(orgID, db.DB, "Image_Sets").Debug().Model(&models.Image{})).Limit(pagination.Limit).Offset(pagination.Offset).
		Preload("Commit.Repo").Preload("Commit.InstalledPackages").Preload("Installer").
		Joins(`JOIN Image_Sets ON Image_Sets.id = Images.image_set_id`).
		Where(`Image_sets.id = ?`, &imageSet.ID).
		Find(&images)

	if result.Error != nil {
		err := errors.NewBadRequest("Error to filter images")
		respondWithAPIError(w, s.Log.WithError(err), err)
		return
	}

	Imgs := returnImageDetails(images, s)

	details.ImageSetData = *imageSet
	details.Images = Imgs

	// update image installer iso URL for all images with the internal application storage end-point
	for _, imageDetail := range details.Images {
		if imageDetail.Image.InstallerID != nil && imageDetail.Image.Installer.ImageBuildISOURL != "" {
			imageDetail.Image.Installer.ImageBuildISOURL = getStorageInstallerIsoURL(imageDetail.Image.Installer.ID)
		}
	}

	if Imgs != nil && Imgs[len(Imgs)-1].Image != nil && Imgs[len(Imgs)-1].Image.InstallerID != nil {
		img := Imgs[len(Imgs)-1].Image
		details.ImageBuildISOURL = img.Installer.ImageBuildISOURL
	}

	if err := json.NewEncoder(w).Encode(&common.EdgeAPIPaginatedResponse{
		Data:  &details,
		Count: int64(len(images)),
	}); err != nil {
		s.Log.WithField("error", &common.EdgeAPIPaginatedResponse{Data: &details,
			Count: int64(len(images)),
		}).Error("Error while trying to encode")
	}
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

		if val := r.URL.Query().Get("version"); val != "" {
			_, err := strconv.Atoi(val)
			if err != nil {
				errs = append(errs, common.ValidationError{Key: "version", Reason: fmt.Sprintf("%s is not a valid version type, version must be number", val)})
			}
		}

		if val := r.URL.Query().Get("id"); val != "" {
			_, err := strconv.Atoi(val)
			if err != nil {
				errs = append(errs, common.ValidationError{Key: "id", Reason: fmt.Sprintf("%s is not a valid id type, id must be number", val)})
			}

		}

		if len(errs) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(&errs); err != nil {
			services := dependencies.ServicesFromContext(r.Context())
			services.Log.WithField("error", errs).Error("Error while trying to encode")
		}
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
			s.Log.Error("Image detail not found")
		}
		Imgs = append(Imgs, ImageDetail(img))
	}

	return Imgs
}

// GetImageSetsView return a list of image-sets view
func GetImageSetsView(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return
	}

	pagination := common.GetPagination(r)

	imageSetsCount, err := ctxServices.ImageSetService.GetImageSetsViewCount(imageSetFilters(r, db.DB))
	if err != nil {
		ctxServices.Log.WithFields(log.Fields{"error": err.Error(), "orgID": orgID}).Error("error getting image-sets view count")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}

	imageSetsViewList, err := ctxServices.ImageSetService.GetImageSetsView(pagination.Limit, pagination.Offset, imageSetFilters(r, db.DB))
	if err != nil {
		ctxServices.Log.WithFields(log.Fields{"error": err.Error(), "orgID": orgID}).Error("error getting image-sets view")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
	respondWithJSONBody(w, ctxServices.Log, map[string]interface{}{"data": imageSetsViewList, "count": imageSetsCount})
}

// ImageSetViewCtx provides the handler for ImageSet view details
func ImageSetViewCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		orgID := readOrgID(w, r, ctxServices.Log)
		if orgID == "" {
			return
		}
		imageSetIDString := chi.URLParam(r, "imageSetID")
		if imageSetIDString == "" {
			return
		}
		ctxServices.Log = ctxServices.Log.WithField("imageSetID", imageSetIDString)
		imageSetID, err := strconv.Atoi(imageSetIDString)
		if err != nil {
			ctxServices.Log.WithField("error", err.Error()).Error("error while converting image-set id from string")
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("bad image-set id"))
			return
		}
		var imageSet models.ImageSet
		if result := db.Org(orgID, "").First(&imageSet, imageSetID); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				respondWithAPIError(w, ctxServices.Log, errors.NewNotFound("image-set not found"))
				return
			}
			apiError := errors.NewInternalServerError()
			apiError.SetTitle("internal server error occurred while getting image-set")
			respondWithAPIError(w, ctxServices.Log, apiError)
			return
		}

		ctx := context.WithValue(r.Context(), imageSetKey, &imageSet)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getContextImageSet(w http.ResponseWriter, r *http.Request) *models.ImageSet {
	ctx := r.Context()
	imageSet, ok := ctx.Value(imageSetKey).(*models.ImageSet)
	if !ok {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("failed to get image-set from context"))
		return nil
	}
	return imageSet
}

// GetImageSetViewByID handle the image-set view
func GetImageSetViewByID(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	imageSet := getContextImageSet(w, r)
	if imageSet == nil {
		// log and response handled by getContextImageSet
		return
	}
	imagesDBFilters := imageDetailFilters(r, db.DB)
	pagination := common.GetPagination(r)
	imageSetIDView, err := ctxServices.ImageSetService.GetImageSetViewByID(imageSet.ID, pagination.Limit, pagination.Offset, imagesDBFilters)
	if err != nil {
		var apiError errors.APIError
		switch err.(type) {
		case *services.ImageSetNotFoundError:
			apiError = errors.NewNotFound("image-set not found")
		case *services.ImageNotFoundError:
			apiError = errors.NewNotFound("image-set has no image")
		case *services.OrgIDNotSet:
			apiError = errors.NewBadRequest("org-id not set")
		default:
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}
	respondWithJSONBody(w, ctxServices.Log, imageSetIDView)
}

// GetAllImageSetImagesView handle the image-set images view
func GetAllImageSetImagesView(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	imageSet := getContextImageSet(w, r)
	if imageSet == nil {
		// log and response handled by getContextImageSet
		return
	}
	imagesDBFilters := imageDetailFilters(r, db.DB)
	pagination := common.GetPagination(r)

	imageSetImagesView, err := ctxServices.ImageSetService.GetImagesViewData(imageSet.ID, pagination.Limit, pagination.Offset, imagesDBFilters)
	if err != nil {
		var apiError errors.APIError
		switch err.(type) {
		case *services.OrgIDNotSet:
			apiError = errors.NewBadRequest("org-id not set")
		default:
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}
	respondWithJSONBody(w, ctxServices.Log, imageSetImagesView)
}

// ImageSetImageViewCtx provides the handler for Image view details
func ImageSetImageViewCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		orgID := readOrgID(w, r, ctxServices.Log)
		if orgID == "" {
			return
		}
		imageSet := getContextImageSet(w, r)
		if imageSet == nil {
			return
		}
		imageIDString := chi.URLParam(r, "imageID")
		if imageIDString == "" {
			return
		}
		ctxServices.Log = ctxServices.Log.WithField("imageID", imageIDString)
		imageID, err := strconv.Atoi(imageIDString)
		if err != nil {
			ctxServices.Log.WithField("error", err.Error()).Error("error while converting image id from string")
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("bad image id"))
			return
		}
		var image models.Image
		if result := db.Org(orgID, "").Where("image_set_id", imageSet.ID).First(&image, imageID); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				respondWithAPIError(w, ctxServices.Log, errors.NewNotFound("image for image set view not found"))
				return
			}
			apiError := errors.NewInternalServerError()
			apiError.SetTitle("internal server error occurred while getting image set image")
			respondWithAPIError(w, ctxServices.Log, apiError)
			return
		}

		ctx := context.WithValue(r.Context(), imageSetImageKey, &image)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getContextImageSetImage(w http.ResponseWriter, r *http.Request) *models.Image {
	ctx := r.Context()
	image, ok := ctx.Value(imageSetImageKey).(*models.Image)
	if !ok {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("failed to get image from context"))
		return nil
	}
	return image
}

// GetImageSetImageView handle the image-set image view
func GetImageSetImageView(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	imageSet := getContextImageSet(w, r)
	if imageSet == nil {
		return
	}

	image := getContextImageSetImage(w, r)
	if image == nil {
		return
	}

	imageSetImageView, err := ctxServices.ImageSetService.GetImageSetImageViewByID(imageSet.ID, image.ID)
	if err != nil {
		var apiError errors.APIError
		switch err.(type) {
		case *services.ImageSetNotFoundError:
			apiError = errors.NewNotFound("image-set not found")
		case *services.ImageNotFoundError:
			apiError = errors.NewNotFound("image-set has no image")
		case *services.OrgIDNotSet:
			apiError = errors.NewBadRequest("org-id not set")
		default:
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}
	respondWithJSONBody(w, ctxServices.Log, imageSetImageView)
}
