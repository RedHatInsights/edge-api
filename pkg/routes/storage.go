// FIXME: golangci-lint
// nolint:revive
package routes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	url2 "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/signature"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type installerTypeKey string
type updateTransactionTypeKey string

const installerKey installerTypeKey = "installer_key"
const updateTransactionKey updateTransactionTypeKey = "update_transaction__key"

func setContextInstaller(ctx context.Context, installer *models.Installer) context.Context {
	return context.WithValue(ctx, installerKey, installer)
}

func setContextUpdateTransaction(ctx context.Context, installer *models.UpdateTransaction) context.Context {
	return context.WithValue(ctx, updateTransactionKey, installer)
}

// MakeStorageRouter adds support for external storage
func MakeStorageRouter(sub chi.Router) {
	sub.Route("/isos/{installerID}", func(r chi.Router) {
		r.Use(InstallerByIDCtx)
		r.Get("/", GetInstallerIsoStorageContent)
	})
}

// MakeStorageUpdateReposRouter  adds support for update transaction repo serving
func MakeStorageUpdateReposRouter(sub chi.Router) {
	sub.Route("/{updateID}", func(r chi.Router) {
		r.Use(UpdateTransactionCtx)
		r.Get("/content/*", GetUpdateTransactionRepoFileContent)
		r.Get("/*", GetUpdateTransactionRepoFile)
	})
}

// InstallerByIDCtx is a handler for Installer ISOs requests
func InstallerByIDCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		installerIDString := chi.URLParam(r, "installerID")
		if installerIDString == "" {
			ctxServices.Log.Debug("Installer ID was not passed to the request or it was empty")
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("installer ID required"))
			return
		}
		installerID, err := strconv.Atoi(installerIDString)
		if err != nil {
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("installer id must be an integer"))
			return
		}

		account, orgID := readAccountOrOrgID(w, r, ctxServices.Log)
		if account == "" && orgID == "" {
			return
		}
		var installer models.Installer
		if result := db.AccountOrOrg(account, orgID, "").First(&installer, installerID); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				respondWithAPIError(w, ctxServices.Log, errors.NewNotFound("installer not found"))
				return
			}
			respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
			return
		}

		if (installer.Account != "" && installer.Account != account) || (installer.OrgID != "" && installer.OrgID != orgID) {
			ctxServices.Log.WithFields(log.Fields{
				"account": account,
				"org_id":  orgID,
			}).Error("installer doesn't belong to account or org_id")
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("installer doesn't belong to account or org_id"))
			return
		}

		ctx := setContextInstaller(r.Context(), &installer)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getContextInstaller(w http.ResponseWriter, r *http.Request) *models.Installer {
	ctx := r.Context()
	installer, ok := ctx.Value(installerKey).(*models.Installer)

	if !ok {
		ctxServices := dependencies.ServicesFromContext(ctx)
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("Failed getting installer from context"))
		return nil
	}
	return installer
}

// GetInstallerIsoStorageContent redirect to a signed installer iso url
func GetInstallerIsoStorageContent(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	installer := getContextInstaller(w, r)
	if installer == nil {
		return
	}
	if installer.ImageBuildISOURL == "" {
		respondWithAPIError(w, ctxServices.Log, errors.NewNotFound("empty installer iso url"))
		return
	}
	url, err := url2.Parse(installer.ImageBuildISOURL)
	if err != nil {
		ctxServices.Log.WithFields(log.Fields{
			"error": err.Error(),
			"URL":   installer.ImageBuildISOURL,
		}).Error("error occurred when parsing url")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("bad installer iso url"))
		return
	}
	signedURL, err := ctxServices.FilesService.GetSignedURL(url.Path)
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("error occurred when signing url")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
	http.Redirect(w, r, signedURL, http.StatusSeeOther)
}

// UpdateTransactionCtx is a handler for Update transaction requests
func UpdateTransactionCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		updateIDString := chi.URLParam(r, "updateID")
		if updateIDString == "" {
			ctxServices.Log.Debug("Update transaction ID was not passed to the request or it was empty")
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("update transaction ID required"))
			return
		}
		updateTransactionID, err := strconv.Atoi(updateIDString)
		if err != nil {
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("update transaction id must be an integer"))
			return
		}

		var updateTransaction models.UpdateTransaction
		if result := db.DB.Preload("Repo").First(&updateTransaction, updateTransactionID); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				ctxServices.Log.WithField("error", result.Error.Error()).Error("device update transaction not found")
				respondWithAPIError(w, ctxServices.Log, errors.NewNotFound("device update transaction not found"))
				return
			}
			ctxServices.Log.WithField("error", result.Error.Error()).Error("failed to retrieve update transaction")
			respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
			return
		}

		ctx := setContextUpdateTransaction(r.Context(), &updateTransaction)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getContextStorageUpdateTransaction(w http.ResponseWriter, r *http.Request) *models.UpdateTransaction {
	ctx := r.Context()
	updateTransaction, ok := ctx.Value(updateTransactionKey).(*models.UpdateTransaction)

	if !ok {
		ctxServices := dependencies.ServicesFromContext(ctx)
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("Failed getting update transaction from context"))
		return nil
	}
	return updateTransaction
}

// getRequestUpdateTransactionData return the update transaction device cookie data
func getRequestUpdateTransactionData(w http.ResponseWriter, r *http.Request, updateTransaction models.UpdateTransaction) (*signature.UpdateTransactionPayload, error) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	logContext := ctxServices.Log.WithField("service", "device-repository-storage")
	cookie, err := r.Cookie("device")
	if err != nil {
		logContext.WithField("error", err.Error()).Error("unable to read device cookies")
		respondWithAPIError(w, logContext, errors.NewBadRequest("unable to read device cookies"))
		return nil, err
	}

	signingKey := []byte(config.Get().PayloadSigningKey)

	var updateTransactionData signature.UpdateTransactionPayload
	if err := signature.DecodeUpdateTransactionCookieValue(signingKey, cookie.Value, updateTransaction, &updateTransactionData); err != nil {
		logContext.WithField("error", err.Error()).Error("Error when decoding cookie value")
		var apiError errors.APIError
		switch err {
		case signature.ErrInvalidDataAndSignatureString, signature.ErrSignatureValidationFailure:
			apiError = errors.NewBadRequest(err.Error())
		case signature.ErrSignatureKeyCannotBeEmpty:
			apiError = errors.NewInternalServerError()
		default:
			apiError = errors.NewBadRequest("error when unmarshalling cookie value")
		}
		respondWithAPIError(w, logContext, apiError)
		return nil, err
	}

	return &updateTransactionData, nil
}

// ValidateStorageUpdateTransaction validate storage update transaction and return the request path
func ValidateStorageUpdateTransaction(w http.ResponseWriter, r *http.Request) string {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	updateTransaction := getContextStorageUpdateTransaction(w, r)
	if updateTransaction == nil {
		return ""
	}
	logContext := ctxServices.Log.WithFields(log.Fields{
		"service":               "device-repository-storage",
		"orgID":                 updateTransaction.OrgID,
		"updateTransactionID":   updateTransaction.ID,
		"updateTransactionUUID": updateTransaction.UUID,
	})

	// check update transaction expiry
	updateTransactionExpire := time.Duration(config.Get().UpdateTransactionExpiry) * time.Minute
	expiryTimeRemaining := int64(time.Until(updateTransaction.CreatedAt.Time.Add(updateTransactionExpire)).Minutes())
	if expiryTimeRemaining < 0 {
		errMessage := fmt.Sprintf("update transaction expired: %d minutes ago", -1*expiryTimeRemaining)
		logContext.Errorf(errMessage)
		respondWithAPIError(w, logContext, errors.NewAccessForbidden(errMessage))
		return ""
	}

	filePath := chi.URLParam(r, "*")
	if filePath == "" {
		logContext.Error("target repository file path is missing")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("target repository file path is missing"))
		return ""
	}

	updateTransactionData, err := getRequestUpdateTransactionData(w, r, *updateTransaction)
	if err != nil {
		// response handled by getRequestUpdateTransactionData
		return ""
	}

	if updateTransactionData.OrgID != updateTransaction.OrgID {
		logContext.WithFields(
			log.Fields{"device_request_org_id": updateTransactionData.OrgID},
		).Error("update transaction org_id mismatch")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("update transaction org_id mismatch"))
		return ""
	}

	if updateTransactionData.UpdateTransactionID != updateTransaction.ID {
		logContext.WithFields(
			log.Fields{"device_request_id": updateTransactionData.UpdateTransactionID},
		).Error("update transaction id mismatch")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("update transaction id mismatch"))
		return ""
	}

	if updateTransaction.Repo == nil || updateTransaction.Repo.URL == "" {
		logContext.Error("update transaction repository does not exist")
		respondWithAPIError(w, logContext, errors.NewNotFound("update transaction repository does not exist"))
		return ""
	}

	RepoURL, err := url2.Parse(updateTransaction.Repo.URL)
	if err != nil {
		logContext.WithFields(log.Fields{
			"error": err.Error(),
			"URL":   updateTransaction.Repo.URL,
		}).Error("error occurred when parsing repository url")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("bad update transaction repository url"))
		return ""
	}

	requestPath := fmt.Sprintf(RepoURL.Path + "/" + filePath)
	return requestPath
}

// redirectToSignedURL redirect request to real content storage url using a signed url
func redirectToSignedURL(w http.ResponseWriter, r *http.Request, path string) error {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	logContext := ctxServices.Log.WithField("service", "device-repository-storage")
	signedURL, err := ctxServices.FilesService.GetSignedURL(path)
	if err != nil {
		logContext.WithFields(log.Fields{
			"error": err.Error(),
			path:    path,
		}).Error("error occurred when signing url")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return err
	}
	logContext.WithField("path", signedURL).Debug("redirect")
	http.Redirect(w, r, signedURL, http.StatusSeeOther)
	return nil
}

// GetUpdateTransactionRepoFileContent redirect to a signed url of an update-transaction repository path content
func GetUpdateTransactionRepoFileContent(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	logContext := ctxServices.Log.WithField("service", "device-repository-storage")
	updateTransaction := getContextStorageUpdateTransaction(w, r)
	if updateTransaction == nil {
		return
	}

	requestPath := ValidateStorageUpdateTransaction(w, r)
	if requestPath == "" {
		// ValidateStorageUpdateTransaction will handle errors
		return
	}

	logContext.WithFields(log.Fields{
		"orgID":                 updateTransaction.OrgID,
		"updateTransactionID":   updateTransaction.ID,
		"updateTransactionUUID": updateTransaction.UUID,
		"path":                  requestPath,
	}).Info("redirect storage update transaction repo resource")

	_ = redirectToSignedURL(w, r, requestPath)
	// redirectToSignedURL respond with proper response on failure
}

// GetUpdateTransactionRepoFile return the content of an update-transaction repository path
func GetUpdateTransactionRepoFile(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	logContext := ctxServices.Log.WithField("service", "device-repository-storage")
	updateTransaction := getContextStorageUpdateTransaction(w, r)
	if updateTransaction == nil {
		return
	}

	requestPath := ValidateStorageUpdateTransaction(w, r)
	if requestPath == "" {
		// ValidateStorageUpdateTransaction will handle errors
		return
	}

	logContext = logContext.WithFields(log.Fields{
		"orgID":                 updateTransaction.OrgID,
		"updateTransactionID":   updateTransaction.ID,
		"updateTransactionUUID": updateTransaction.UUID,
		"path":                  requestPath,
	})
	logContext.Info("return storage update transaction repo resource content")

	requestFile, err := ctxServices.FilesService.GetFile(requestPath)
	if err != nil {
		logContext.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("error occurred when getting file from request path")
		var apiError errors.APIError
		if strings.Contains(err.Error(), "was not found on the S3 bucket") {
			apiError = errors.NewNotFound(fmt.Sprintf("file '%s' was not found", requestPath))
		} else {
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}
	defer func(requestFile io.ReadCloser) {
		err := requestFile.Close()
		if err != nil {
			logContext.WithField("path", requestPath).Error("error closing request file")
		}
	}(requestFile)

	w.Header().Set("Content-Type", "application/octet-stream; charset=binary")
	w.WriteHeader(http.StatusOK)

	if ind, err := io.Copy(w, requestFile); err != nil {
		logContext.WithField("error", err.Error()).
			WithField("Content-Type", w.Header().Values("Content-Type")).
			WithField("len-content", ind).Error("error writing content")
	}
}
