// FIXME: golangci-lint
// nolint:gocritic,govet,revive
package routes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	url2 "net/url"
	"strconv"
	"strings"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"

	"github.com/redhatinsights/edge-api/config"

	"github.com/go-chi/chi/v5"
	"github.com/redhatinsights/edge-api/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type installerTypeKey string
type updateTransactionTypeKey string
type storageImageTypeKey string
type keyType int

const fileServiceKey keyType = iota
const installerKey installerTypeKey = "installer_key"
const updateTransactionKey updateTransactionTypeKey = "update_transaction_key"
const storageImageKey storageImageTypeKey = "storage_image_key"

type UpdateRepo struct {
	ID      uint
	OrgID   string
	RepoURL string
}

func setContextInstaller(ctx context.Context, installer *models.Installer) context.Context {
	return context.WithValue(ctx, installerKey, installer)
}

func setContextUpdateTransaction(ctx context.Context, updateRepo *UpdateRepo) context.Context {
	return context.WithValue(ctx, updateTransactionKey, updateRepo)
}

func setContextStorageImage(ctx context.Context, image *models.Image) context.Context {
	return context.WithValue(ctx, storageImageKey, image)
}

func WithFileService(ctx context.Context, fileService services.FilesService) context.Context {
	return context.WithValue(ctx, fileServiceKey, &fileService)
}

func getFileService(ctx context.Context) (services.FilesService, bool) {
	fileService, ok := ctx.Value(fileServiceKey).(*services.FilesService)
	if !ok || fileService == nil {
		return nil, false
	}
	return *fileService, ok
}

// MakeStorageRouter adds support for external storage
func MakeStorageRouter(sub chi.Router) {
	sub.Use(fileServiceCtx)
	MakeStorageRouterWithoutFilesService(sub)
}

func MakeStorageRouterWithoutFilesService(sub chi.Router) {
	sub.Route("/update-repos/{updateID}", func(r chi.Router) {
		r.Use(UpdateTransactionCtx)
		r.Get("/content/*", GetUpdateTransactionRepoFileContent)
		r.Get("/*", GetUpdateTransactionRepoFile)
	})
	sub.Route("/isos/{installerID}", func(r chi.Router) {
		r.Use(InstallerByIDCtx)
		r.Get("/", GetInstallerIsoStorageContent)
	})
	sub.Route("/images-repos/{imageID}", func(r chi.Router) {
		r.Use(storageImageCtx)
		r.Get("/content/*", GetImageRepoFileContent)
		r.Get("/*", GetImageRepoFile)
	})
}

// fileServiceCtx is a handler to initiate fileService in context
func fileServiceCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := log.WithContext(r.Context())
		fileService := services.NewFilesService(logger)
		ctx := WithFileService(r.Context(), fileService)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// redirectToStorageSignedURL redirect request to real content storage url using a signed url
func redirectToStorageSignedURL(w http.ResponseWriter, r *http.Request, path string) {
	logger := log.WithContext(r.Context()).WithField("service", "device-repository-storage")
	fileService, ok := getFileService(r.Context())
	if !ok {
		respondWithAPIError(w, logger, errors.NewBadRequest("Failed getting file service from context"))
		return
	}
	signedURL, err := fileService.GetSignedURL(path)
	if err != nil {
		logger.WithFields(log.Fields{
			"error": err.Error(),
			path:    path,
		}).Error("error occurred when signing url")
		respondWithAPIError(w, logger, errors.NewInternalServerError())
		return
	}
	logger.WithField("path", signedURL).Debug("redirect")
	http.Redirect(w, r, signedURL, http.StatusSeeOther)
}

// serveStorageContent return the real content from storage
func serveStorageContent(w http.ResponseWriter, r *http.Request, path string) {
	logger := log.WithContext(r.Context()).WithField("service", "device-repository-storage")
	fileService, ok := getFileService(r.Context())
	if !ok {
		respondWithAPIError(w, logger, errors.NewBadRequest("Failed getting file service from context"))
		return
	}
	requestFile, err := fileService.GetFile(path)
	if err != nil {
		logger.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("error occurred when getting file from request path")
		var apiError errors.APIError
		if strings.Contains(err.Error(), "was not found on the S3 bucket") {
			apiError = errors.NewNotFound(fmt.Sprintf("file '%s' was not found", path))
		} else {
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, logger, apiError)
		return
	}
	defer func(requestFile io.ReadCloser) {
		closeErr := requestFile.Close()
		if closeErr != nil {
			logger.WithFields(log.Fields{"path": path, "error": closeErr.Error()}).Error("error closing request file")
		}
	}(requestFile)

	w.Header().Set("Content-Type", "application/octet-stream; charset=binary")
	w.WriteHeader(http.StatusOK)
	if ind, err := io.Copy(w, requestFile); err != nil {
		logger.WithField("error", err.Error()).
			WithField("Content-Type", w.Header().Values("Content-Type")).
			WithField("len-content", ind).Error("error writing content")
	}
}

// InstallerByIDCtx is a handler for Installer ISOs requests
func InstallerByIDCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := log.WithContext(r.Context())
		installerIDString := chi.URLParam(r, "installerID")
		if installerIDString == "" {
			logger.Debug("Installer ID was not passed to the request or it was empty")
			respondWithAPIError(w, logger, errors.NewBadRequest("installer ID required"))
			return
		}
		installerID, err := strconv.Atoi(installerIDString)
		if err != nil {
			respondWithAPIError(w, logger, errors.NewBadRequest("installer id must be an integer"))
			return
		}

		orgID := readOrgID(w, r, logger)
		if orgID == "" {
			return
		}
		var installer models.Installer
		if result := db.Org(orgID, "").First(&installer, installerID); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				respondWithAPIError(w, logger, errors.NewNotFound("installer not found"))
				return
			}
			respondWithAPIError(w, logger, errors.NewInternalServerError())
			return
		}

		if installer.OrgID != orgID {
			logger.WithFields(log.Fields{
				"org_id": orgID,
			}).Error("installer doesn't belong to org_id")
			respondWithAPIError(w, logger, errors.NewBadRequest("installer doesn't belong to org_id"))
			return
		}

		ctx := setContextInstaller(r.Context(), &installer)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getContextInstaller(ctx context.Context) (*models.Installer, bool) {
	installer, ok := ctx.Value(installerKey).(*models.Installer)
	return installer, ok
}

// GetInstallerIsoStorageContent redirect to a signed installer iso url
// @Summary			Redirect to a signed installer
// @ID				RedirectSignedInstaller
// @Description		This method will redirect request to a signed installer iso url
// @Tags			Storage
// @Accept			json
// @Produce			octet-stream
// @Param			installerID path string true "Installer ID"
// @Success			303 {string} string "URL to redirect"
// @Failure			400 {object} errors.BadRequest "The request send couln't be processed."
// @Failure			404 {object} errors.NotFound "installer not found."
// @Failure			500 {object} errors.InternalServerError
// @Router			/storage/isos/{installerID}/ [get]
func GetInstallerIsoStorageContent(w http.ResponseWriter, r *http.Request) {
	logger := log.WithContext(r.Context())
	installer, ok := getContextInstaller(r.Context())
	if !ok || installer == nil {
		respondWithAPIError(w, logger, errors.NewBadRequest("Failed getting installer from context"))
		return
	}
	if installer.ImageBuildISOURL == "" {
		respondWithAPIError(w, logger, errors.NewNotFound("empty installer iso url"))
		return
	}
	url, err := url2.Parse(installer.ImageBuildISOURL)
	if err != nil {
		logger.WithFields(log.Fields{
			"error": err.Error(),
			"URL":   installer.ImageBuildISOURL,
		}).Error("error occurred when parsing url")
		respondWithAPIError(w, logger, errors.NewBadRequest("bad installer iso url"))
		return
	}
	fileService, ok := getFileService(r.Context())
	if !ok {
		respondWithAPIError(w, logger, errors.NewBadRequest("Failed getting file service from context"))
		return
	}
	signedURL, err := fileService.GetSignedURL(url.Path)
	if err != nil {
		logger.WithField("error", err.Error()).Error("error occurred when signing url")
		respondWithAPIError(w, logger, errors.NewInternalServerError())
		return
	}
	http.Redirect(w, r, signedURL, http.StatusSeeOther)
}

// UpdateTransactionCtx is a handler for Update transaction requests
func UpdateTransactionCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := log.WithContext(r.Context())
		orgID := readOrgID(w, r, logger)
		if orgID == "" {
			// readOrgID handle response and logging on failure
			return
		}

		updateIDString := chi.URLParam(r, "updateID")
		if updateIDString == "" {
			logger.Debug("Update transaction ID was not passed to the request or it was empty")
			respondWithAPIError(w, logger, errors.NewBadRequest("update transaction ID required"))
			return
		}
		updateTransactionID, err := strconv.Atoi(updateIDString)
		if err != nil {
			respondWithAPIError(w, logger, errors.NewBadRequest("update transaction id must be an integer"))
			return
		}

		var updateRepo UpdateRepo
		dbQuery := db.Org(orgID, "").
			Model(models.UpdateTransaction{}).
			Joins("LEFT JOIN repos AS r ON r.id = update_transactions.repo_id").
			Select("update_transactions.id as id, update_transactions.org_id as org_id, r.url as repo_url")
		if result := dbQuery.First(&updateRepo, updateTransactionID); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				logger.WithField("error", result.Error.Error()).Error("device update transaction not found")
				respondWithAPIError(w, logger, errors.NewNotFound("device update transaction not found"))
				return
			}
			logger.WithField("error", result.Error.Error()).Error("failed to retrieve update transaction")
			respondWithAPIError(w, logger, errors.NewInternalServerError())
			return
		}

		ctx := setContextUpdateTransaction(r.Context(), &updateRepo)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getContextStorageUpdateTransaction(ctx context.Context) (*UpdateRepo, bool) {
	updateRepo, ok := ctx.Value(updateTransactionKey).(*UpdateRepo)
	return updateRepo, ok
}

// ValidateStorageUpdateTransaction validate storage update transaction and return the request path
func ValidateStorageUpdateTransaction(w http.ResponseWriter, r *http.Request) string {
	logger := log.WithContext(r.Context())
	updateRepo, ok := getContextStorageUpdateTransaction(r.Context())
	if !ok || updateRepo == nil {
		respondWithAPIError(w, logger, errors.NewBadRequest("Failed getting update transaction from context"))
		return ""
	}
	logger = logger.WithFields(log.Fields{
		"service":             "device-repository-storage",
		"orgID":               updateRepo.OrgID,
		"updateTransactionID": updateRepo.ID,
	})

	filePath := chi.URLParam(r, "*")
	if filePath == "" {
		logger.Error("target repository file path is missing")
		respondWithAPIError(w, logger, errors.NewBadRequest("target repository file path is missing"))
		return ""
	}

	if updateRepo.RepoURL == "" {
		logger.Error("update transaction repository does not exist")
		respondWithAPIError(w, logger, errors.NewNotFound("update transaction repository does not exist"))
		return ""
	}

	RepoURL, err := url2.Parse(updateRepo.RepoURL)
	if err != nil {
		logger.WithFields(log.Fields{
			"error": err.Error(),
			"URL":   updateRepo.RepoURL,
		}).Error("error occurred when parsing repository url")
		respondWithAPIError(w, logger, errors.NewBadRequest("bad update transaction repository url"))
		return ""
	}

	requestPath := fmt.Sprintf(RepoURL.Path + "/" + filePath)
	return requestPath
}

// GetUpdateTransactionRepoFileContent redirect to a signed url of an update-transaction repository path content
// @Summary		redirect to a signed url of an update-transaction repository path content
// @ID			RedirectUpdateTransactionRepositoryPath
// @Description	Method will redirect to asigned url of an update-transaction based on repository content
// @Tags		Storage
// @Accept		json
// @Produce		octet-stream
// @Param		updateTransactionID path int true "id for update transaction id"
// @Param		repoFilePath path string true "path to repository to be checked"
// @Success		303 {string} string "URL signed to be redirect"
// @Failure		400 {object} errors.BadRequest
// @Failure		404 {object} errors.NotFound
// @Failure		500 {object} errors.InternalServerError
// @Router		/storage/update-repos/{updateTransactionID}/content/{repoFilePath} [get]
func GetUpdateTransactionRepoFileContent(w http.ResponseWriter, r *http.Request) {
	logger := log.WithContext(r.Context()).WithField("service", "device-repository-storage")
	updateRepo, ok := getContextStorageUpdateTransaction(r.Context())
	if !ok || updateRepo == nil {
		respondWithAPIError(w, logger, errors.NewBadRequest("Failed getting update transaction from context"))
		return
	}

	requestPath := ValidateStorageUpdateTransaction(w, r)
	if requestPath == "" {
		// ValidateStorageUpdateTransaction will handle errors
		return
	}

	logger.WithFields(log.Fields{
		"orgID":               updateRepo.OrgID,
		"updateTransactionID": updateRepo.ID,
		"path":                requestPath,
	}).Debug("redirect storage update transaction repo resource")

	redirectToStorageSignedURL(w, r, requestPath)
}

// GetUpdateTransactionRepoFile return the content of an update-transaction repository path
// @Summary		Return the content od an update-transaction repository path
// @ID			RedirectUpdateTransactionRepositoryContent
// @Description	Request will get access to content of an update-transaction file based on the path
// @Tags		Storage
// @Accept		json
// @Produce		octet-stream
// @Param		updateTransactionID path int true "Update Transaction Id"
// @Param		repoFilePath path int true "path for repository file"
// @Success		200 {string} string	"Stream object from file content"
// @Failure		400 {object} errors.BadRequest
// @Failure		404 {object} errors.NotFound
// @Failure		500 {object} errors.InternalServerError
// @Router		/storage/update-repos/{updateTransactionID}/{repoFilePath} [get]
func GetUpdateTransactionRepoFile(w http.ResponseWriter, r *http.Request) {
	logger := log.WithContext(r.Context()).WithField("service", "device-repository-storage")
	updateRepo, ok := getContextStorageUpdateTransaction(r.Context())
	if !ok || updateRepo == nil {
		respondWithAPIError(w, logger, errors.NewBadRequest("Failed getting update transaction from context"))
		return
	}

	requestPath := ValidateStorageUpdateTransaction(w, r)
	if requestPath == "" {
		// ValidateStorageUpdateTransaction will handle errors
		return
	}

	logger.WithFields(log.Fields{
		"orgID":               updateRepo.OrgID,
		"updateTransactionID": updateRepo.ID,
		"path":                requestPath,
	}).Debug("return storage update transaction repo resource content")
	serveStorageContent(w, r, requestPath)
}

// storageImageCtx is a handler for storage image requests
func storageImageCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := log.WithContext(r.Context())
		orgID := readOrgID(w, r, logger)
		if orgID == "" {
			// readOrgID handle response and logging on failure
			return
		}

		imageIDString := chi.URLParam(r, "imageID")
		if imageIDString == "" {
			logger.Debug("storage image ID was not passed to the request or it was empty")
			respondWithAPIError(w, logger, errors.NewBadRequest("storage image ID required"))
			return
		}
		imageID, err := strconv.Atoi(imageIDString)
		if err != nil {
			respondWithAPIError(w, logger, errors.NewBadRequest("storage image ID must be an integer"))
			return
		}
		imageBuilderOrgID := config.Get().ImageBuilderOrgID
		var dbFilter *gorm.DB
		if imageBuilderOrgID != "" && orgID == imageBuilderOrgID {
			// image-builder have read access to all image commit repositories
			dbFilter = db.DB
		} else {
			dbFilter = db.Org(orgID, "images")
		}
		var image models.Image
		if result := dbFilter.Preload("Commit.Repo").Joins("Commit").First(&image, imageID); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				logger.WithField("error", result.Error.Error()).Error("storage image not found")
				respondWithAPIError(w, logger, errors.NewNotFound("storage image not found"))
				return
			}
			logger.WithField("error", result.Error.Error()).Error("failed to retrieve storage image")
			respondWithAPIError(w, logger, errors.NewInternalServerError())
			return
		}

		ctx := setContextStorageImage(r.Context(), &image)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getContextStorageImage(ctx context.Context) (*models.Image, bool) {
	image, ok := ctx.Value(storageImageKey).(*models.Image)
	return image, ok
}

// ValidateStorageImage validate storage image and return the request path
func ValidateStorageImage(w http.ResponseWriter, r *http.Request) string {
	logger := log.WithContext(r.Context())
	image, ok := getContextStorageImage(r.Context())
	if !ok || image == nil {
		respondWithAPIError(w, logger, errors.NewBadRequest("Failed getting storage image from context"))
		return ""
	}
	logger.WithFields(log.Fields{
		"service": "image-repository-storage",
		"orgID":   image.OrgID,
		"imageID": image.ID,
	})

	filePath := chi.URLParam(r, "*")
	if filePath == "" {
		logger.Error("target repository file path is missing")
		respondWithAPIError(w, logger, errors.NewBadRequest("target repository file path is missing"))
		return ""
	}

	if image.Commit.Repo == nil || image.Commit.Repo.URL == "" {
		logger.Error("image repository does not exist")
		respondWithAPIError(w, logger, errors.NewNotFound("image repository does not exist"))
		return ""
	}

	RepoURL, err := url2.Parse(image.Commit.Repo.URL)
	if err != nil {
		logger.WithFields(log.Fields{
			"error": err.Error(),
			"URL":   image.Commit.Repo.URL,
		}).Error("error occurred when parsing repository url")
		respondWithAPIError(w, logger, errors.NewBadRequest("bad image repository url"))
		return ""
	}

	requestPath := fmt.Sprintf(RepoURL.Path + "/" + filePath)
	return requestPath
}

// GetImageRepoFileContent redirect to a signed url of an image commit repository path content
// @Summary			redirect to a signed url of an image commit repository path content
// @Description		Redirect request to a signed and valid url for an image commit repository from the path content
// @ID				RedirectSignedImageCommitRepository
// @Tags			Storage
// @Accept			json
// @Produce			json
// @Param			imageID path string true "Id to identify Image"
// @Param			repoFilePath path string true "path to file repository"
// @Success			303 {string} url response
// @Failure			400 {object} errors.BadRequest
// @Failure			404 {object} errors.NotFound
// @Failure			500 {object} errors.InternalServerError
// @Router			/storage/images-repos/{imageID}/content/{repoFilePath} [get]
func GetImageRepoFileContent(w http.ResponseWriter, r *http.Request) {
	logger := log.WithContext(r.Context()).WithField("service", "image-repository-storage")
	image, ok := getContextStorageImage(r.Context())
	if !ok || image == nil {
		respondWithAPIError(w, logger, errors.NewBadRequest("Failed getting storage image from context"))
		return
	}

	requestPath := ValidateStorageImage(w, r)
	if requestPath == "" {
		// ValidateStorageImage will handle errors
		return
	}

	logger.WithFields(log.Fields{
		"orgID":   image.OrgID,
		"imageID": image.ID,
		"path":    requestPath,
	}).Debug("redirect storage image repo resource")
	redirectToStorageSignedURL(w, r, requestPath)
}

// GetImageRepoFile return the content of an image commit repository path
// @Summary		return the content of an image commit repository path
// @ID			ContentImageCommitRepositoryPath
// @Description	Bring the content for a image commit in a repository path
// @Tags		Storage
// @Accept		json
// @Produce		octet-stream
// @Param		imageID path string true "Id to identify Image"
// @Param		repoFilePath path string true "path to file repository"
// @Success		200 {string} string "Stream object from file content"
// @Failure		400 {object} errors.BadRequest
// @Failure		404 {object} errors.NotFound
// @Failure		500 {object} errors.InternalServerError
// @Router		/storage/images-repos/{imageID}/{repoFilePath} [get]
func GetImageRepoFile(w http.ResponseWriter, r *http.Request) {
	logger := log.WithContext(r.Context()).WithField("service", "image-repository-storage")
	image, ok := getContextStorageImage(r.Context())
	if !ok || image == nil {
		respondWithAPIError(w, logger, errors.NewBadRequest("Failed getting storage image from context"))
		return
	}
	requestPath := ValidateStorageImage(w, r)
	if requestPath == "" {
		// ValidateStorageImage will handle errors
		return
	}

	logger.WithFields(log.Fields{
		"orgID":   image.OrgID,
		"imageID": image.ID,
		"path":    requestPath,
	}).Debug("return storage image repo resource content")
	serveStorageContent(w, r, requestPath)
}
