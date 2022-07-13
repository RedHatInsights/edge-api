package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
)

type imageSetTypeKey int

const imageSetKey imageSetTypeKey = iota

var sortOption = []string{"created_at", "updated_at", "name"}
var statusOption = []string{models.ImageStatusCreated, models.ImageStatusBuilding, models.ImageStatusError, models.ImageStatusSuccess}

// MakeImageSetsRouter adds support for operations on image-sets
func MakeImageSetsRouter(sub chi.Router) {
	sub.With(validateFilterParams).With(common.Paginate).Get("/", ListAllImageSets)
	sub.Route("/{imageSetID}", func(r chi.Router) {
		r.Use(ImageSetCtx)
		r.With(validateFilterParams).With(common.Paginate).Get("/", GetImageSetsByID)
	})
}

func getStorageInstallerIsoURL(installerID uint) string {
	return fmt.Sprintf("/api/edge/v1/storage/isos/%d", installerID)
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

//ImageSetInstallerURL returns Imageset structure with last installer available
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

	countResult := imageSetFilters(r, db.OrgDB(orgID, db.DB, "Image_Sets").Model(&models.ImageSet{})).
		Joins(`JOIN Images ON Image_Sets.id = Images.image_set_id AND Images.id = (Select Max(id) from Images where Images.image_set_id = Image_Sets.id)`).Count(&count)
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
		result = imageSetFilters(r, db.OrgDB(orgID, db.DB, "Image_Sets").Debug().Model(&models.ImageSet{})).
			Limit(pagination.Limit).Offset(pagination.Offset).
			Preload("Images").
			Preload("Images.Commit").
			Preload("Images.Installer").
			Preload("Images.Commit.Repo").
			Joins(`JOIN Images ON Image_Sets.id = Images.image_set_id AND Images.id = (Select Max(id) from Images where Images.image_set_id = Image_Sets.id)`).
			Find(&imageSet)
	} else {
		// this code is no longer run, but would be used if sorting by status is re-implemented.
		result = imageStatusFilters(r, db.OrgDB(orgID, db.DB, "Image_Sets").Debug().Model(&models.ImageSet{})).
			Limit(pagination.Limit).Offset(pagination.Offset).
			Preload("Images", "lower(status) in (?)", strings.ToLower(r.URL.Query().Get("status"))).
			Preload("Images.Commit").
			Preload("Images.Installer").
			Preload("Images.Commit.Repo").
			Joins(`JOIN Images ON Image_Sets.id = Images.image_set_id AND Images.id = (Select Max(id) from Images where Images.image_set_id = Image_Sets.id)`).
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

//ImageSetImagePackages return info related to details on images from imageset
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
	result := imageDetailFilters(r, db.OrgDB(orgID, db.DB, "Image_Sets").Model(&models.Image{})).Limit(pagination.Limit).Offset(pagination.Offset).
		Preload("Commit.Repo").Preload("Commit.InstalledPackages").Preload("Installer").
		Joins(`JOIN Image_Sets ON Image_Sets.id = Images.image_set_id`).
		Where(`Image_sets.id = ?`, &imageSet.ID).
		Find(&images)

	if result.Error != nil {
		err := errors.NewBadRequest("Error to filter images")
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
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
