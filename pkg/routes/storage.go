// FIXME: golangci-lint
package routes // nolint:revive

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

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type installerTypeKey string
type updateTransactionTypeKey string

const installerKey installerTypeKey = "installer_key"
const updateTransactionKey updateTransactionTypeKey = "update_transaction_key"

func setContextInstaller(ctx context.Context, installer *models.Installer) context.Context { // nolint:revive
	return context.WithValue(ctx, installerKey, installer)
}

func setContextUpdateTransaction(ctx context.Context, installer *models.UpdateTransaction) context.Context { // nolint:revive
	return context.WithValue(ctx, updateTransactionKey, installer)
}

// MakeStorageRouter adds support for external storage
func MakeStorageRouter(sub chi.Router) {
	sub.Route("/isos/{installerID}", func(r chi.Router) {
		r.Use(InstallerByIDCtx)
		r.Get("/", GetInstallerIsoStorageContent)
	})
	sub.Route("/update-repos/{updateID}", func(r chi.Router) {
		r.Use(UpdateTransactionCtx)
		r.Get("/content/*", GetUpdateTransactionRepoFileContent)
		r.Get("/*", GetUpdateTransactionRepoFile)
	})
}

// nolint:revive // redirectToStorageSignedURL redirect request to real content storage url using a signed url
func redirectToStorageSignedURL(w http.ResponseWriter, r *http.Request, path string) { // nolint:revive
	ctxServices := dependencies.ServicesFromContext(r.Context())
	logContext := ctxServices.Log.WithField("service", "device-repository-storage") // nolint:revive
	signedURL, err := ctxServices.FilesService.GetSignedURL(path)
	if err != nil {
		logContext.WithFields(log.Fields{
			"error": err.Error(),
			path:    path,
		}).Error("error occurred when signing url")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
	logContext.WithField("path", signedURL).Debug("redirect")
	http.Redirect(w, r, signedURL, http.StatusSeeOther)
}

// serveStorageContent return the real content from storage
func serveStorageContent(w http.ResponseWriter, r *http.Request, path string) { // nolint:revive
	ctxServices := dependencies.ServicesFromContext(r.Context())
	logContext := ctxServices.Log.WithField("service", "device-repository-storage") // nolint:revive
	requestFile, err := ctxServices.FilesService.GetFile(path)
	if err != nil {
		logContext.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("error occurred when getting file from request path")
		var apiError errors.APIError
		if strings.Contains(err.Error(), "was not found on the S3 bucket") {
			apiError = errors.NewNotFound(fmt.Sprintf("file '%s' was not found", path)) // nolint:revive
		} else {
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}
	defer func(requestFile io.ReadCloser) {
		err := requestFile.Close() // nolint:govet
		if err != nil {
			logContext.WithField("path", path).Error("error closing request file") // nolint:revive
		}
	}(requestFile)

	w.Header().Set("Content-Type", "application/octet-stream; charset=binary")
	w.WriteHeader(http.StatusOK)
	if ind, err := io.Copy(w, requestFile); err != nil { // nolint:govet
		logContext.WithField("error", err.Error()). // nolint:revive
								WithField("Content-Type", w.Header().Values("Content-Type")). // nolint:revive
								WithField("len-content", ind).Error("error writing content")  // nolint:revive
	}
}

// InstallerByIDCtx is a handler for Installer ISOs requests
func InstallerByIDCtx(next http.Handler) http.Handler { // nolint:revive
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		installerIDString := chi.URLParam(r, "installerID")
		if installerIDString == "" {
			ctxServices.Log.Debug("Installer ID was not passed to the request or it was empty")    // nolint:revive
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("installer ID required")) // nolint:revive
			return
		}
		installerID, err := strconv.Atoi(installerIDString)
		if err != nil {
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("installer id must be an integer")) // nolint:revive
			return
		}

		account, orgID := readAccountOrOrgID(w, r, ctxServices.Log)
		if account == "" && orgID == "" { // nolint:revive
			return
		}
		var installer models.Installer
		if result := db.AccountOrOrg(account, orgID, "").First(&installer, installerID); result.Error != nil { // nolint:revive
			if result.Error == gorm.ErrRecordNotFound {
				respondWithAPIError(w, ctxServices.Log, errors.NewNotFound("installer not found")) // nolint:revive
				return
			}
			respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError()) // nolint:revive
			return
		}

		if (installer.Account != "" && installer.Account != account) || (installer.OrgID != "" && installer.OrgID != orgID) { // nolint:revive
			ctxServices.Log.WithFields(log.Fields{
				"account": account,
				"org_id":  orgID,
			}).Error("installer doesn't belong to account or org_id")
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("installer doesn't belong to account or org_id")) // nolint:revive
			return
		}

		ctx := setContextInstaller(r.Context(), &installer)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getContextInstaller(w http.ResponseWriter, r *http.Request) *models.Installer { // nolint:revive
	ctx := r.Context()
	installer, ok := ctx.Value(installerKey).(*models.Installer)

	if !ok {
		ctxServices := dependencies.ServicesFromContext(ctx)
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("Failed getting installer from context")) // nolint:revive
		return nil
	}
	return installer
}

// GetInstallerIsoStorageContent redirect to a signed installer iso url
func GetInstallerIsoStorageContent(w http.ResponseWriter, r *http.Request) { // nolint:revive
	ctxServices := dependencies.ServicesFromContext(r.Context())
	installer := getContextInstaller(w, r)
	if installer == nil {
		return
	}
	if installer.ImageBuildISOURL == "" {
		respondWithAPIError(w, ctxServices.Log, errors.NewNotFound("empty installer iso url")) // nolint:revive
		return
	}
	url, err := url2.Parse(installer.ImageBuildISOURL)
	if err != nil {
		ctxServices.Log.WithFields(log.Fields{
			"error": err.Error(),
			"URL":   installer.ImageBuildISOURL,
		}).Error("error occurred when parsing url")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("bad installer iso url")) // nolint:revive
		return
	}
	signedURL, err := ctxServices.FilesService.GetSignedURL(url.Path)
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("error occurred when signing url") // nolint:revive
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
	http.Redirect(w, r, signedURL, http.StatusSeeOther)
}

// UpdateTransactionCtx is a handler for Update transaction requests
func UpdateTransactionCtx(next http.Handler) http.Handler { // nolint:revive
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		orgID := readOrgID(w, r, ctxServices.Log)
		if orgID == "" {
			// readOrgID handle response and logging on failure
			return
		}

		updateIDString := chi.URLParam(r, "updateID")
		if updateIDString == "" {
			ctxServices.Log.Debug("Update transaction ID was not passed to the request or it was empty")    // nolint:revive
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("update transaction ID required")) // nolint:revive
			return
		}
		updateTransactionID, err := strconv.Atoi(updateIDString)
		if err != nil {
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("update transaction id must be an integer")) // nolint:revive
			return
		}

		var updateTransaction models.UpdateTransaction
		if result := db.Org(orgID, "").Preload("Repo").First(&updateTransaction, updateTransactionID); result.Error != nil { // nolint:revive
			if result.Error == gorm.ErrRecordNotFound {
				ctxServices.Log.WithField("error", result.Error.Error()).Error("device update transaction not found") // nolint:revive
				respondWithAPIError(w, ctxServices.Log, errors.NewNotFound("device update transaction not found"))    // nolint:revive
				return
			}
			ctxServices.Log.WithField("error", result.Error.Error()).Error("failed to retrieve update transaction") // nolint:revive
			respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())                                // nolint:revive
			return
		}

		ctx := setContextUpdateTransaction(r.Context(), &updateTransaction)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getContextStorageUpdateTransaction(w http.ResponseWriter, r *http.Request) *models.UpdateTransaction { // nolint:revive
	ctx := r.Context()
	updateTransaction, ok := ctx.Value(updateTransactionKey).(*models.UpdateTransaction) // nolint:revive

	if !ok {
		ctxServices := dependencies.ServicesFromContext(ctx)
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("Failed getting update transaction from context")) // nolint:revive
		return nil
	}
	return updateTransaction
}

// nolint:revive // ValidateStorageUpdateTransaction validate storage update transaction and return the request path
func ValidateStorageUpdateTransaction(w http.ResponseWriter, r *http.Request) string { // nolint:revive
	ctxServices := dependencies.ServicesFromContext(r.Context())
	updateTransaction := getContextStorageUpdateTransaction(w, r)
	if updateTransaction == nil {
		return ""
	}
	logContext := ctxServices.Log.WithFields(log.Fields{
		"service":             "device-repository-storage", // nolint:revive
		"orgID":               updateTransaction.OrgID,
		"updateTransactionID": updateTransaction.ID,
	})

	filePath := chi.URLParam(r, "*")
	if filePath == "" {
		logContext.Error("target repository file path is missing")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("target repository file path is missing")) // nolint:revive
		return ""
	}

	if updateTransaction.Repo == nil || updateTransaction.Repo.URL == "" {
		logContext.Error("update transaction repository does not exist")
		respondWithAPIError(w, logContext, errors.NewNotFound("update transaction repository does not exist")) // nolint:revive
		return ""
	}

	RepoURL, err := url2.Parse(updateTransaction.Repo.URL) // nolint:gocritic,revive
	if err != nil {
		logContext.WithFields(log.Fields{
			"error": err.Error(),
			"URL":   updateTransaction.Repo.URL,
		}).Error("error occurred when parsing repository url")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("bad update transaction repository url")) // nolint:revive
		return ""
	}

	requestPath := fmt.Sprintf(RepoURL.Path + "/" + filePath)
	return requestPath
}

// nolint:revive // GetUpdateTransactionRepoFileContent redirect to a signed url of an update-transaction repository path content
func GetUpdateTransactionRepoFileContent(w http.ResponseWriter, r *http.Request) { // nolint:revive
	ctxServices := dependencies.ServicesFromContext(r.Context())
	logContext := ctxServices.Log.WithField("service", "device-repository-storage") // nolint:revive
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
		"orgID":               updateTransaction.OrgID,
		"updateTransactionID": updateTransaction.ID,
		"path":                requestPath, // nolint:revive
	}).Info("redirect storage update transaction repo resource")

	redirectToStorageSignedURL(w, r, requestPath)
}

// nolint:revive // GetUpdateTransactionRepoFile return the content of an update-transaction repository path
func GetUpdateTransactionRepoFile(w http.ResponseWriter, r *http.Request) { // nolint:revive
	ctxServices := dependencies.ServicesFromContext(r.Context())
	logContext := ctxServices.Log.WithField("service", "device-repository-storage") // nolint:revive
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
		"orgID":               updateTransaction.OrgID, // nolint:revive
		"updateTransactionID": updateTransaction.ID,    // nolint:revive
		"path":                requestPath,
	})
	logContext.Info("return storage update transaction repo resource content")
	serveStorageContent(w, r, requestPath)
}
