// FIXME: golangci-lint
// nolint:errcheck,gosimple,govet,revive,typecheck
package routes

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/platform-go-middlewares/request_id"
	log "github.com/sirupsen/logrus"
)

// This provides type safety in the context object for our "image" key.  We
// _could_ use a string, but we shouldn't just in case someone else decides that
// "image" would make the perfect key in the context object.  See the
// documentation: https://golang.org/pkg/context/#WithValue for further
// rationale.
type imageTypeKey int

const imageKey imageTypeKey = iota

// MakeImagesRouter adds support for operations on images
func MakeImagesRouter(sub chi.Router) {
	sub.With(ValidateQueryParams("images")).With(ValidateGetAllImagesSearchParams).With(common.Paginate).Get("/", GetAllImages)
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
		r.Post("/resume", ResumeCreateImage)       // temporary to be replaced with EDA
		r.Get("/notify", SendNotificationForImage) // TMP ROUTE TO SEND THE NOTIFICATION
		r.Delete("/", DeleteImage)
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
				case *services.OrgIDNotSet:
					responseErr = errors.NewBadRequest(err.Error())
				default:
					responseErr = errors.NewInternalServerError()
				}
				respondWithAPIError(w, s.Log, responseErr)
				return
			}
			ctx := context.WithValue(r.Context(), imageKey, image)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			respondWithAPIError(w, s.Log, errors.NewBadRequest("OSTreeCommitHash required"))
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
				case *services.OrgIDNotSet:
					responseErr = errors.NewBadRequest(err.Error())
				case *services.IDMustBeInteger:
					responseErr = errors.NewBadRequest(err.Error())
				default:
					responseErr = errors.NewInternalServerError()
				}
				respondWithAPIError(w, s.Log, responseErr)
				return
			}
			orgID := readOrgID(w, r, s.Log)
			if orgID == "" {
				s.Log.WithFields(log.Fields{
					"image_id": imageID,
				}).Error("image doesn't belong to org_id")
				respondWithAPIError(w, s.Log, errors.NewBadRequest("image doesn't belong to org_id"))
				return
			}
			if image.OrgID != "" && image.OrgID != orgID {
				errMessage := "image and session org mismatch"
				s.Log.WithFields(log.Fields{
					"image_org_id": image.OrgID,
					"org_id":       orgID,
				}).Error(errMessage)
				respondWithAPIError(w, s.Log, errors.NewBadRequest(errMessage))
				return
			}
			ctx := context.WithValue(r.Context(), imageKey, image)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			s.Log.Debug("Image ID was not passed to the request or it was empty")
			respondWithAPIError(w, s.Log, errors.NewBadRequest("Image ID required"))
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

// GetImageWithIdentity pre-populates the image with Account, OrgID, requestID and also returns an identity header
// This is used by every image endpoint, so it reduces copy/paste risk
func GetImageWithIdentity(w http.ResponseWriter, r *http.Request) (*models.Image, identity.XRHID, error) {
	ctxServices := dependencies.ServicesFromContext(r.Context())

	ident, err := common.GetIdentityFromContext(r.Context())
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Failed retrieving identity from request")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))

		return nil, ident, err
	}

	image, err := initImageCreateRequest(w, r)
	if err != nil {
		// initImageCreateRequest() already writes the response
		return image, ident, err
	}

	image.Account, err = common.GetAccount(r)
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Failed retrieving account from request")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))

		return image, ident, err
	}
	image.OrgID, err = common.GetOrgID(r)
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Failed retrieving org_id from request")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))

		return image, ident, err
	}
	image.RequestID = request_id.GetReqID(r.Context())

	return image, ident, nil
}

// CreateImage creates an image on hosted image builder.
// It always creates a commit on Image Builder.
// Then we create our repo with the ostree commit and if needed, create the installer.
func CreateImage(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())

	// get the initial image with identity fields set
	image, ident, err := GetImageWithIdentity(w, r)
	if err != nil {
		log.WithField("error", err).Error("Failed to get an image with identity added")
		return
	}

	ctxServices.Log.Debug("Creating image from API request")
	// initial checks and filling in necessary image info
	if err = ctxServices.ImageService.CreateImage(image); err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Failed creating the image")
		var apiError errors.APIError
		switch err.(type) {
		case *services.PackageNameDoesNotExist, *services.ThirdPartyRepositoryInfoIsInvalid, *services.ThirdPartyRepositoryNotFound, *services.ImageNameAlreadyExists, *services.ImageSetAlreadyExists:
			apiError = errors.NewBadRequest(err.Error())
		default:
			apiError = errors.NewInternalServerError()
			apiError.SetTitle("Failed creating image")
		}
		// TODO: does this respond with the appropriate HTTP response code?
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}

	if feature.ImageCreateEDA.IsEnabled() {
		ctxServices.Log.Debug("Creating image from API request with EDA")

		// create payload for ImageRequested event
		edgePayload := &models.EdgeImageRequestedEventPayload{
			EdgeBasePayload: models.EdgeBasePayload{
				Identity:       ident,
				LastHandleTime: time.Now().Format(time.RFC3339),
				RequestID:      image.RequestID,
			},
			NewImage: *image,
		}

		// create the edge event
		edgeEvent := kafkacommon.CreateEdgeEvent(ident.Identity.OrgID, models.SourceEdgeEventAPI, image.RequestID,
			models.EventTypeEdgeImageRequested, image.Name, edgePayload)

		// put the event on the bus
		if err = kafkacommon.NewProducerService().ProduceEvent(kafkacommon.TopicFleetmgmtImageBuild, models.EventTypeEdgeImageRequested, edgeEvent); err != nil {
			log.WithField("request_id", edgeEvent.ID).Error("Producing the event failed")
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))

			return
		}

		// return to Edge UI
		w.WriteHeader(http.StatusOK)
		respondWithJSONBody(w, ctxServices.Log, image)

		return
	}

	// FALL THROUGH IF NOT EDA

	// TODO: this is going to go away with EDA
	ctxServices.ImageService.ProcessImage(r.Context(), image)

	ctxServices.Log.WithFields(log.Fields{
		"imageId": image.ID,
	}).Info("Image build process started from API request")
	w.WriteHeader(http.StatusOK)
	respondWithJSONBody(w, ctxServices.Log, image)
}

// CreateImageUpdate creates an update for an existing image on hosted image builder.
func CreateImageUpdate(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())

	// get the initial image with identity fields set
	image, ident, err := GetImageWithIdentity(w, r)
	if err != nil {
		log.WithField("error", err).Error("Failed to get an image with identity added")
		return
	}

	previousImage := getImage(w, r)
	if previousImage == nil {
		// getImage already writes the response
		return
	}

	ctxServices.Log.Debug("Updating an image from API request")
	err = ctxServices.ImageService.UpdateImage(image, previousImage)
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Failed creating an update to an image")
		var apiError errors.APIError
		switch err.(type) {
		case *services.PackageNameDoesNotExist, *services.ThirdPartyRepositoryInfoIsInvalid, *services.ThirdPartyRepositoryNotFound,
			*services.ImageNameAlreadyExists, *services.ImageSetAlreadyExists, *services.ImageNameChangeIsProhibited,
			*services.ImageOnlyLatestCanModify:
			apiError = errors.NewBadRequest(err.Error())
		default:
			apiError = errors.NewInternalServerError()
			apiError.SetTitle("Failed creating image")
		}
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}

	if feature.ImageUpdateEDA.IsEnabled() {
		ctxServices.Log.Debug("Creating image from API request with EDA")

		// create payload for ImageRequested event
		edgePayload := &models.EdgeImageUpdateRequestedEventPayload{
			EdgeBasePayload: models.EdgeBasePayload{
				Identity:       ident,
				LastHandleTime: time.Now().Format(time.RFC3339),
				RequestID:      image.RequestID,
			},
			NewImage: *image,
		}

		// create the edge event
		edgeEvent := kafkacommon.CreateEdgeEvent(ident.Identity.OrgID, models.SourceEdgeEventAPI, image.RequestID,
			models.EventTypeEdgeImageUpdateRequested, image.Name, edgePayload)

		// put the event on the bus
		if err = kafkacommon.NewProducerService().ProduceEvent(kafkacommon.TopicFleetmgmtImageBuild, models.EventTypeEdgeImageUpdateRequested, edgeEvent); err != nil {
			log.WithField("request_id", edgeEvent.ID).Error("Producing the event failed")
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))

			return
		}

		// return to Edge UI
		w.WriteHeader(http.StatusOK)
		respondWithJSONBody(w, ctxServices.Log, image)

		return
	}

	// FALL THROUGH IF NOT EDA

	// TODO: this is going to go away with EDA
	ctxServices.ImageService.ProcessImage(r.Context(), image)

	w.WriteHeader(http.StatusOK)
	respondWithJSONBody(w, ctxServices.Log, image)

	return
}

// initImageCreateRequest validates request to create/update an image.
func initImageCreateRequest(w http.ResponseWriter, r *http.Request) (*models.Image, error) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	var image models.Image
	if err := readRequestJSONBody(w, r, ctxServices.Log, &image); err != nil {
		return nil, err
	}

	if image.Name == "" {
		// look if a previous image exists, to handle image name properly.
		if previousImage, ok := r.Context().Value(imageKey).(*models.Image); ok && previousImage != nil {
			// when updating from a previousImage and we do not supply an image name set it by default to previousImage.Name
			image.Name = previousImage.Name
		}
	}

	if err := image.ValidateRequest(); err != nil {
		ctxServices.Log.WithField("error", err.Error()).Info("Error validating image")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
		return nil, err
	}
	ctxServices.Log = ctxServices.Log.WithField("imageName", image.Name)
	return &image, nil
}

var imageFilters = common.ComposeFilters(
	// Filter handler for "status"
	common.OneOfFilterHandler(&common.Filter{
		QueryParam: "status",
		DBField:    "images.status",
	}),
	// Filter handler for "name"
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "name",
		DBField:    "images.name",
	}),
	// Filter handler for "distribution"
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "distribution",
		DBField:    "images.distribution",
	}),
	// Filter handler for "created_at"
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

// ValidateGetAllImagesSearchParams validate the query params that sent to /images endpoint
func ValidateGetAllImagesSearchParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var errs []validationError
		// "status" validation
		if statuses, ok := r.URL.Query()["status"]; ok {
			for _, status := range statuses {
				if status != models.ImageStatusCreated && status != models.ImageStatusBuilding && status != models.ImageStatusError && status != models.ImageStatusSuccess {
					errs = append(errs, validationError{Key: "status", Reason: fmt.Sprintf("%s is not a valid status. Status must be %s", status, strings.Join(validStatuses, " or "))})
				}
			}
		}
		// "created_at" validation
		if val := r.URL.Query().Get("created_at"); val != "" {
			if _, err := time.Parse(common.LayoutISO, val); err != nil {
				errs = append(errs, validationError{Key: "created_at", Reason: err.Error()})
			}
		}
		// "sort_by" validation for "status", "name", "distribution", "created_at", "version"
		if val := r.URL.Query().Get("sort_by"); val != "" {
			name := val
			if string(val[0]) == "-" {
				name = val[1:]
			}
			if name != "status" && name != "name" && name != "distribution" && name != "created_at" && name != "version" {
				errs = append(errs, validationError{Key: "sort_by", Reason: fmt.Sprintf("%s is not a valid sort_by. Sort-by must be status or name or distribution or created_at", name)})
			}
		}

		if len(errs) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		ctxServices := dependencies.ServicesFromContext(r.Context())
		w.WriteHeader(http.StatusBadRequest)
		respondWithJSONBody(w, ctxServices.Log, &errs)
	})
}

// GetAllImages image objects from the database for an orgID
func GetAllImages(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	ctxServices.Log.Debug("Getting all images")
	var count int64
	var images []models.Image
	result := imageFilters(r, db.DB)
	pagination := common.GetPagination(r)
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return
	}
	countResult := db.OrgDB(orgID, imageFilters(r, db.DB.Model(&models.Image{})), "images").Count(&count)
	if countResult.Error != nil {
		ctxServices.Log.WithField("error", countResult.Error.Error()).Error("Error retrieving images")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
	result = db.OrgDB(orgID, result, "images").Limit(pagination.Limit).Offset(pagination.Offset).Preload("Packages").Preload("Commit.Repo").Preload("CustomPackages").Preload("ThirdPartyRepositories").Joins("Commit").Joins("Installer").Find(&images)
	if result.Error != nil {
		ctxServices.Log.WithField("error", result.Error.Error()).Error("Error retrieving images")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
	respondWithJSONBody(w, ctxServices.Log, map[string]interface{}{"data": &images, "count": count})
}

func getImage(w http.ResponseWriter, r *http.Request) *models.Image {
	ctx := r.Context()
	image, ok := ctx.Value(imageKey).(*models.Image)
	if !ok {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("Must pass image identifier"))
		return nil
	}
	return image
}

// GetImageStatusByID returns the image status.
func GetImageStatusByID(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		log.WithField("status", image.Status).Debug("Returning image status to UI")
		respondWithJSONBody(w, ctxServices.Log,
			struct {
				Status string
				Name   string
				ID     uint
			}{
				image.Status,
				image.Name,
				image.ID,
			},
		)
	}
}

// ImageDetail return the structure to inform package info to images
type ImageDetail struct {
	Image              *models.Image `json:"image"`
	AdditionalPackages int           `json:"additional_packages"`
	Packages           int           `json:"packages"`
	UpdateAdded        int           `json:"update_added"`
	UpdateRemoved      int           `json:"update_removed"`
	UpdateUpdated      int           `json:"update_updated"`
}

// GetImageByID obtains an image from the database for an orgID
func GetImageByID(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		respondWithJSONBody(w, ctxServices.Log, image)
	}
}

// GetImageDetailsByID obtains an image from the database for an orgID
func GetImageDetailsByID(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		var imgDetail ImageDetail
		imgDetail.Image = image
		imgDetail.Packages = len(image.Commit.InstalledPackages)
		imgDetail.AdditionalPackages = len(image.Packages)

		upd, err := ctxServices.ImageService.GetUpdateInfo(*image)
		if err != nil {
			ctxServices.Log.WithField("error", err.Error()).Error("Error getting update info")
			respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
			return
		}
		if upd != nil {
			imgDetail.UpdateAdded = len(upd.PackageDiff.Removed)
			imgDetail.UpdateRemoved = len(upd.PackageDiff.Added)
			imgDetail.UpdateUpdated = len(upd.PackageDiff.Upgraded)
			imgDetail.Image.TotalDevicesWithImage = upd.Image.TotalDevicesWithImage
			imgDetail.Image.TotalPackages = upd.Image.TotalPackages
		} else {
			imgDetail.UpdateAdded = 0
			imgDetail.UpdateRemoved = 0
			imgDetail.UpdateUpdated = 0
		}
		respondWithJSONBody(w, ctxServices.Log, &imgDetail)
	}
}

// GetImageByOstree obtains an image from the database for an orgID based on Commit Ostree
func GetImageByOstree(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		respondWithJSONBody(w, ctxServices.Log, image)
	}
}

// CreateInstallerForImage creates an installer for an Image
// It requires a created image and a repo with a successful status
func CreateInstallerForImage(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())

	image := getImage(w, r)
	if image == nil {
		return
	}
	if err := readRequestJSONBody(w, r, ctxServices.Log, &image.Installer); err != nil {
		return
	}

	image, _, err := ctxServices.ImageService.CreateInstallerForImage(r.Context(), image)
	if err != nil {
		ctxServices.Log.WithField("error", err).Error("Failed to create installer")
		err := errors.NewInternalServerError()
		err.SetTitle("Failed to create installer")
		respondWithAPIError(w, ctxServices.Log, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	respondWithJSONBody(w, ctxServices.Log, image)
}

// CreateRepoForImage creates a repo for an Image
func CreateRepoForImage(w http.ResponseWriter, r *http.Request) {
	image := getImage(w, r)

	go func(id uint, ctx context.Context) {
		ctxServices := dependencies.ServicesFromContext(ctx)
		var img *models.Image
		if result := db.DB.Joins("Commit").Joins("Installer").First(&img, id); result.Error != nil {
			ctxServices.Log.WithField("error", result.Error.Error()).Debug("error while trying to get image")
			return
		}
		if result := db.DB.First(&img.Commit, img.CommitID); result.Error != nil {
			ctxServices.Log.WithField("error", result.Error.Error()).Debug("error while trying to get image commit")
			return
		}
		if _, err := ctxServices.ImageService.CreateRepoForImage(ctx, img); err != nil {
			ctxServices.Log.WithField("error", err).Error("Failed to create repo")
		}
	}(image.ID, r.Context())

	w.WriteHeader(http.StatusOK)
}

// GetRepoForImage gets the repository for an Image
func GetRepoForImage(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		ctxServices.Log = ctxServices.Log.WithField("repoID", image.Commit.RepoID)
		repo, err := ctxServices.RepoService.GetRepoByID(image.Commit.RepoID)
		if err != nil {
			err := errors.NewNotFound(fmt.Sprintf("Commit repo wasn't found in the database: #%v", image.CommitID))
			respondWithAPIError(w, ctxServices.Log, err)
			return
		}
		respondWithJSONBody(w, ctxServices.Log, repo)
	}
}

// GetMetadataForImage gets the metadata from image-builder on /metadata endpoint
func GetMetadataForImage(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		meta, err := ctxServices.ImageService.GetMetadata(image)
		if err != nil {
			respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
			return
		}
		respondWithJSONBody(w, ctxServices.Log, meta)
	}
}

// CreateKickStartForImage creates a kickstart file for an existent image
func CreateKickStartForImage(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())

	image := getImage(w, r)
	if image == nil {
		return
	}

	if err := ctxServices.ImageService.AddUserInfo(image); err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Kickstart file injection failed")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
}

// CheckImageNameResponse indicates whether the image exists
type CheckImageNameResponse struct {
	ImageExists bool `json:"ImageExists"`
}

// CheckImageName verifies that ImageName exists
func CheckImageName(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	var image *models.Image
	if err := readRequestJSONBody(w, r, ctxServices.Log, &image); err != nil {
		return
	}
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return
	}
	if image == nil {
		err := errors.NewInternalServerError()
		ctxServices.Log.WithField("error", err.Error()).Error("Internal Server Error")
		respondWithAPIError(w, ctxServices.Log, err)
		return
	}
	imageExists, err := ctxServices.ImageService.CheckImageName(image.Name, orgID)
	if err != nil {
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
	respondWithJSONBody(w, ctxServices.Log, &CheckImageNameResponse{ImageExists: imageExists})
}

// RetryCreateImage retries the image creation
func RetryCreateImage(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		err := ctxServices.ImageService.RetryCreateImage(r.Context(), image)
		if err != nil {
			ctxServices.Log.WithField("error", err.Error()).Error("Failed to retry to create image")
			err := errors.NewInternalServerError()
			err.SetTitle("Failed creating image")
			respondWithAPIError(w, ctxServices.Log, err)
			return
		}
		w.WriteHeader(http.StatusCreated)
		respondWithJSONBody(w, ctxServices.Log, image)
	}
}

// ResumeCreateImage retries the image creation
func ResumeCreateImage(w http.ResponseWriter, r *http.Request) {
	/* This endpoint rebuilds context from the stored image.
	Unlike the other routes (e.g., /retry), the request r is only
	used to get the image number and request id and then for the return.
	A new context is created and the image to be resumed is
	retrieved from the database.
	*/
	if tempImage := getImage(w, r); tempImage != nil {
		// TODO: move this to its own context function
		// ctx := context.Background()
		ctx := r.Context()
		// using the Middleware() steps to be similar to the front door
		edgeAPIServices := dependencies.Init(ctx)
		ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)

		// re-grab the image from the database
		var image *models.Image
		db.DB.Preload("Commit.Repo").Joins("Commit").Joins("Installer").First(&image, tempImage.ID)

		resumeLog := edgeAPIServices.Log.WithField("originalRequestId", image.RequestID)
		resumeLog.Info("Resuming image build")

		// recreate a stripped down identity header
		strippedIdentity := `{ "identity": {"account_number": ` + image.Account + `, "type": "User", "internal": {"org_id": ` + image.OrgID + `, }, }, }`
		resumeLog.WithField("identity_text", strippedIdentity).Debug("Creating a new stripped identity")
		base64Identity := base64.StdEncoding.EncodeToString([]byte(strippedIdentity))
		resumeLog.WithField("identity_base64", base64Identity).Debug("Using a base64encoded stripped identity")

		// add the new identity to the context and create ctxServices with that context
		ctx = common.SetOriginalIdentity(ctx, base64Identity)
		ctxServices := dependencies.ServicesFromContext(ctx)
		// TODO: consider a bitwise& param to only add needed ctxServices

		// use the new ctxServices w/ context to make the imageService.ResumeCreateImage call
		err := ctxServices.ImageService.ResumeCreateImage(ctx, image)

		// finish out the original API call
		if err != nil {
			edgeAPIServices.Log.WithField("error", err.Error()).Error("Failed to retry to create image")
			err := errors.NewInternalServerError()
			err.SetTitle("Failed creating image")
			respondWithAPIError(w, ctxServices.Log, err)
			return
		}
		w.WriteHeader(http.StatusCreated)
		respondWithJSONBody(w, ctxServices.Log, image)
	}
}

// SendNotificationForImage TMP route to validate
func SendNotificationForImage(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		notify, err := ctxServices.ImageService.SendImageNotification(image)
		if err != nil {
			ctxServices.Log.WithField("error", err.Error()).Error("Failed to retry to send notification")
			err := errors.NewInternalServerError()
			err.SetTitle("Failed to send notification")
			respondWithAPIError(w, ctxServices.Log, err)
			return
		}
		ctxServices.Log.WithField("StatusOK", http.StatusOK).Info("Writing Header")
		w.WriteHeader(http.StatusOK)
		respondWithJSONBody(w, ctxServices.Log, &notify)
	}
}

func DeleteImage(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	image := getImage(w, r)
	if image == nil {
		return
	}
	orgID := readOrgID(w, r, ctxServices.Log)

	if orgID != image.OrgID {
		err := errors.NewBadRequest("Not the image owner, unable to delete")
		err.SetTitle("Failed deleting image")
		respondWithAPIError(w, ctxServices.Log, err)
		return
	}

	err := ctxServices.ImageService.DeleteImage(image)
	if err != nil {
		var responseErr errors.APIError
		switch err.(type) {
		case *services.ImageNotInErrorState:
			responseErr = errors.NewBadRequest("Given image is not in ERROR state, it can't be deleted")
		default:
			responseErr = errors.NewInternalServerError()
			responseErr.SetTitle("Failed deleting image")
		}
		responseErr.SetTitle("Failed deleting image")
		respondWithAPIError(w, ctxServices.Log, responseErr)
		return
	}
	w.WriteHeader(http.StatusOK)
}
