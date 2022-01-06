package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	log "github.com/sirupsen/logrus"
)

// This provides type safety in the context object for our "image" key.  We
// _could_ use a string but we shouldn't just in case someone else decides that
// "image" would make the perfect key in the context object.  See the
// documentation: https://golang.org/pkg/context/#WithValue for further
// rationale.
type imageTypeKey int

const imageKey imageTypeKey = iota

// MakeImagesRouter adds support for operations on images
func MakeImagesRouter(sub chi.Router) {
	sub.With(validateGetAllImagesSearchParams).With(common.Paginate).Get("/", GetAllImages)
	sub.Post("/", CreateImage)
	sub.Post("/checkImageName", CheckImageName)
	sub.Route("/{ostreeCommitHash}/info", func(r chi.Router) {
		r.Use(ImageByOSTreeHashCtx)
		r.Get("/", GetImageByOstree)
	})
	sub.Route("/{imageId}", func(r chi.Router) {
		r.Use(ImageByIDCtx)
		r.Get("/", GetImageByID)
		r.Get("/details", GetImageDetailsByID)
		r.Get("/status", GetImageStatusByID)
		r.Get("/repo", GetRepoForImage)
		r.Get("/metadata", GetMetadataForImage)
		r.Post("/installer", CreateInstallerForImage)
		r.Post("/kickstart", CreateKickStartForImage)
		r.Post("/update", CreateImageUpdate)
		r.Post("/retry", RetryCreateImage)
	})
}

var validStatuses = []string{models.ImageStatusCreated, models.ImageStatusBuilding, models.ImageStatusError, models.ImageStatusSuccess}

// ImageByOSTreeHashCtx is a handler for Images but adds finding images by Ostree Hash
func ImageByOSTreeHashCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := dependencies.ServicesFromContext(r.Context())
		if commitHash := chi.URLParam(r, "ostreeCommitHash"); commitHash != "" {
			s.Log = s.Log.WithField("ostreeCommitHash", commitHash)
			image, err := s.ImageService.GetImageByOSTreeCommitHash(commitHash)
			if err != nil {
				var responseErr errors.APIError
				switch err.(type) {
				case *services.ImageNotFoundError:
					responseErr = errors.NewNotFound(err.Error())
				case *services.AccountNotSet:
					responseErr = errors.NewBadRequest(err.Error())
				default:
					responseErr = errors.NewInternalServerError()
				}
				w.WriteHeader(responseErr.GetStatus())
				json.NewEncoder(w).Encode(&responseErr)
				return
			}
			ctx := context.WithValue(r.Context(), imageKey, image)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			err := errors.NewBadRequest("OSTreeCommitHash required")
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
	})

}

// ImageByIDCtx is a handler for Image requests
func ImageByIDCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := dependencies.ServicesFromContext(r.Context())
		if imageID := chi.URLParam(r, "imageId"); imageID != "" {
			s.Log = s.Log.WithField("imageID", imageID)
			image, err := s.ImageService.GetImageByID(imageID)
			if err != nil {
				var responseErr errors.APIError
				switch err.(type) {
				case *services.ImageNotFoundError:
					responseErr = errors.NewNotFound(err.Error())
				case *services.AccountNotSet:
					responseErr = errors.NewBadRequest(err.Error())
				case *services.IDMustBeInteger:
					responseErr = errors.NewBadRequest(err.Error())
				default:
					responseErr = errors.NewInternalServerError()
				}
				w.WriteHeader(responseErr.GetStatus())
				json.NewEncoder(w).Encode(&responseErr)
				return
			}
			account, err := common.GetAccount(r)
			if err != nil || image.Account != account {
				s.Log.WithFields(log.Fields{
					"error":   err.Error(),
					"account": account,
				}).Error("Error retrieving account or image doesn't belong to account")
				err := errors.NewBadRequest(err.Error())
				w.WriteHeader(err.GetStatus())
				json.NewEncoder(w).Encode(&err)
				return
			}
			ctx := context.WithValue(r.Context(), imageKey, image)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			s.Log.Debug("Image ID was not passed to the request or it was empty")
			err := errors.NewBadRequest("Image ID required")
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
	})
}

// A CreateImageRequest model.
type CreateImageRequest struct {
	// The image to create.
	//
	// in: body
	// required: true
	Image *models.Image
}

// CreateImage creates an image on hosted image builder.
// It always creates a commit on Image Builder.
// Then we create our repo with the ostree commit and if needed, create the installer.
func CreateImage(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	defer r.Body.Close()
	image, err := initImageCreateRequest(w, r)
	if err != nil {
		// initImageCreateRequest() already writes the response
		return
	}
	account, err := common.GetAccount(r)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Failed retrieving account from request")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	services.Log.Debug("Creating image")
	err = services.ImageService.CreateImage(image, account)
	if err != nil {
		services.Log.Error(err)
		err := errors.NewInternalServerError()
		err.SetTitle("Failed creating image")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	services.Log.WithFields(log.Fields{
		"imageId": image.ID,
	}).Info("Image created")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&image)

}

// CreateImageUpdate creates an update for an exitent image on hosted image builder.
func CreateImageUpdate(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	defer r.Body.Close()
	image, err := initImageCreateRequest(w, r)
	if err != nil {
		// initImageCreateRequest() already writes the response
		return
	}
	previousImage := getImage(w, r)
	if image == nil {
		// getImage already writes the response
		return
	}
	err = services.ImageService.UpdateImage(image, previousImage)
	if err != nil {
		services.log.WithField("error", err.Error()).Error("Failed creating an update to an image")
		err := errors.NewInternalServerError()
		err.SetTitle("Failed creating image")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&image)
}

// initImageCreateRequest validates request to create/update an image.
func initImageCreateRequest(w http.ResponseWriter, r *http.Request) (*models.Image, error) {
	services := dependencies.ServicesFromContext(r.Context())
	var image *models.Image
	if err := json.NewDecoder(r.Body).Decode(&image); err != nil {
		services.Log.WithField("error", err.Error()).Error("Error decoding image json")
		err := errors.NewInternalServerError()
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return nil, err
	}
	if err := image.ValidateRequest(); err != nil {
		services.Log.WithField("error", err.Error()).Info("Error validating image")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return nil, err
	}
	services.Log = services.Log.WithField("imageName", image.Name)
	return image, nil
}

var imageFilters = common.ComposeFilters(
	common.OneOfFilterHandler(&common.Filter{
		QueryParam: "status",
		DBField:    "images.status",
	}),
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "name",
		DBField:    "images.name",
	}),
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "distribution",
		DBField:    "images.distribution",
	}),
	common.CreatedAtFilterHandler(&common.Filter{
		QueryParam: "created_at",
		DBField:    "images.created_at",
	}),
	common.SortFilterHandler("images", "created_at", "DESC"),
)

type validationError struct {
	Key    string
	Reason string
}

func validateGetAllImagesSearchParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errs := []validationError{}
		if statuses, ok := r.URL.Query()["status"]; ok {
			for _, status := range statuses {
				if status != models.ImageStatusCreated && status != models.ImageStatusBuilding && status != models.ImageStatusError && status != models.ImageStatusSuccess {
					errs = append(errs, validationError{Key: "status", Reason: fmt.Sprintf("%s is not a valid status. Status must be %s", status, strings.Join(validStatuses, " or "))})
				}
			}
		}
		if val := r.URL.Query().Get("created_at"); val != "" {
			if _, err := time.Parse(common.LayoutISO, val); err != nil {
				errs = append(errs, validationError{Key: "created_at", Reason: err.Error()})
			}
		}
		if val := r.URL.Query().Get("sort_by"); val != "" {
			name := val
			if string(val[0]) == "-" {
				name = val[1:]
			}
			if name != "status" && name != "name" && name != "distribution" && name != "created_at" {
				errs = append(errs, validationError{Key: "sort_by", Reason: fmt.Sprintf("%s is not a valid sort_by. Sort-by must be status or name or distribution or created_at", name)})
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

// GetAllImages image objects from the database for an account
func GetAllImages(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	services.Log.Debug("Getting all images")
	var count int64
	var images []models.Image
	result := imageFilters(r, db.DB)
	pagination := common.GetPagination(r)
	account, err := common.GetAccount(r)
	if err != nil {
		services.Log.WithField("error", err).Debug("Account not found")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	countResult := imageFilters(r, db.DB.Model(&models.Image{})).Where("images.account = ?", account).Count(&count)
	if countResult.Error != nil {
		services.Log.WithField("error", countResult.Error.Error()).Error("Error retrieving images")
		countErr := errors.NewInternalServerError()
		w.WriteHeader(countErr.GetStatus())
		json.NewEncoder(w).Encode(&countErr)
		return
	}
	result = result.Limit(pagination.Limit).Offset(pagination.Offset).Preload("Packages").Preload("Commit.Repo").Where("images.account = ?", account).Joins("Commit").Joins("Installer").Find(&images)
	if result.Error != nil {
		services.Log.WithField("error", result.Error.Error()).Error("Error retrieving images")
		err := errors.NewInternalServerError()
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"data": &images, "count": count})
}

func getImage(w http.ResponseWriter, r *http.Request) *models.Image {
	ctx := r.Context()
	image, ok := ctx.Value(imageKey).(*models.Image)
	if !ok {
		err := errors.NewBadRequest("Must pass image identifier")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return nil
	}
	return image
}

// GetImageStatusByID returns the image status.
func GetImageStatusByID(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		json.NewEncoder(w).Encode(struct {
			Status string
			Name   string
			ID     uint
		}{
			image.Status,
			image.Name,
			image.ID,
		})
	}
}

//ImageDetail return the structure to inform package info to images
type ImageDetail struct {
	Image              *models.Image `json:"image"`
	AdditionalPackages int           `json:"additional_packages"`
	Packages           int           `json:"packages"`
	UpdateAdded        int           `json:"update_added"`
	UpdateRemoved      int           `json:"update_removed"`
	UpdateUpdated      int           `json:"update_updated"`
}

// GetImageByID obtains a image from the database for an account
func GetImageByID(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		json.NewEncoder(w).Encode(image)
	}
}

// GetImageDetailsByID obtains a image from the database for an account
func GetImageDetailsByID(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		services := dependencies.ServicesFromContext(r.Context())
		var imgDetail ImageDetail
		imgDetail.Image = image
		imgDetail.Packages = len(image.Commit.InstalledPackages)
		imgDetail.AdditionalPackages = len(image.Packages)

		upd, err := services.ImageService.GetUpdateInfo(*image)
		if err != nil {
			services.Log.WithField("error", err.Error()).Error("Error getting update info")
		}
		if upd != nil {
			imgDetail.UpdateAdded = len(upd[len(upd)-1].PackageDiff.Removed)
			imgDetail.UpdateRemoved = len(upd[len(upd)-1].PackageDiff.Added)
			imgDetail.UpdateUpdated = len(upd[len(upd)-1].PackageDiff.Upgraded)
		} else {
			imgDetail.UpdateAdded = 0
			imgDetail.UpdateRemoved = 0
			imgDetail.UpdateUpdated = 0
		}
		json.NewEncoder(w).Encode(imgDetail)
	}
}

// GetImageByOstree obtains a image from the database for an account based on Commit Ostree
func GetImageByOstree(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		json.NewEncoder(w).Encode(&image)
	}
}

// CreateInstallerForImage creates a installer for a Image
// It requires a created image and a repo with a successful status
func CreateInstallerForImage(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	image := getImage(w, r)
	if err := json.NewDecoder(r.Body).Decode(&image.Installer); err != nil {
		services.Log.WithField("error", err).Error("Failed to decode installer")
		err := errors.NewInternalServerError()
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	image, _, err := services.ImageService.CreateInstallerForImage(image)
	if err != nil {
		services.Log.WithField("error", err).Error("Failed to create installer")
		err := errors.NewInternalServerError()
		err.SetTitle("Failed to create installer")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&image)
}

// CreateRepoForImage creates a repo for a Image
func CreateRepoForImage(w http.ResponseWriter, r *http.Request) {
	image := getImage(w, r)

	go func(id uint, ctx context.Context) {
		services := dependencies.ServicesFromContext(r.Context())
		var i *models.Image
		db.DB.Joins("Commit").Joins("Installer").First(&i, id)
		db.DB.First(&i.Commit, i.CommitID)
		services.ImageService.CreateRepoForImage(i)
	}(image.ID, r.Context())

	w.WriteHeader(http.StatusOK)
}

//GetRepoForImage gets the repository for a Image
func GetRepoForImage(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		services := dependencies.ServicesFromContext(r.Context())
		services.Log = services.Log.WithField("repoID", image.Commit.RepoID)
		repo, err := services.RepoService.GetRepoByID(image.Commit.RepoID)
		if err != nil {
			err := errors.NewNotFound(fmt.Sprintf("Commit repo wasn't found in the database: #%v", image.CommitID))
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
		json.NewEncoder(w).Encode(repo)
	}
}

//GetMetadataForImage gets the metadata from image-builder on /metadata endpoint
func GetMetadataForImage(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	if image := getImage(w, r); image != nil {
		meta, err := services.ImageService.GetMetadata(image)
		if err != nil {
			err := errors.NewInternalServerError()
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
		json.NewEncoder(w).Encode(meta)
	}
}

// CreateKickStartForImage creates a kickstart file for an existent image
func CreateKickStartForImage(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		services := dependencies.ServicesFromContext(r.Context())
		err := services.ImageService.AddUserInfo(image)
		if err != nil {
			// TODO: Temporary. Handle error better.
			services.Log.Errorf("Kickstart file injection failed %s", err.Error())
			err := errors.NewInternalServerError()
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
	}
}

// CheckImageNameResponse indicates whether or not the image exists
type CheckImageNameResponse struct {
	ImageExists bool `json:"ImageExists"`
}

// CheckImageName verifies that ImageName exists
func CheckImageName(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	services.Log.Debug("Checking image name")
	var image *models.Image
	if err := json.NewDecoder(r.Body).Decode(&image); err != nil {
		services.Log.WithField("error", err.Error()).Debug("Bad request")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
	}
	account, err := common.GetAccount(r)
	if err != nil {
		services.Log.WithField("error", err.Error()).Debug("Bad request")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	if image == nil {
		err := errors.NewInternalServerError()
		services.Log.WithField("error", err.Error()).Error("Internal Server Error")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
	}
	imageExists, err := services.ImageService.CheckImageName(image.Name, account)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Internal Server Error")
		err := errors.NewInternalServerError()
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(CheckImageNameResponse{
		ImageExists: imageExists,
	})
}

// RetryCreateImage retries the image creation
func RetryCreateImage(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		services := dependencies.ServicesFromContext(r.Context())
		err := services.ImageService.RetryCreateImage(image)
		if err != nil {
			services.Log.WithField("error", err.Error()).Error("Failed to retry to create image")
			err := errors.NewInternalServerError()
			err.SetTitle("Failed creating image")
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(&image)
	}
}
