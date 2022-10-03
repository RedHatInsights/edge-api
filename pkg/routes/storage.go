// FIXME: golangci-lint
// nolint:revive
package routes

import (
	"context"
	"net/http"
	url2 "net/url"
	"strconv"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type installerTypeKey string

const installerKey installerTypeKey = "installer_key"

func setContextInstaller(ctx context.Context, installer *models.Installer) context.Context {
	return context.WithValue(ctx, installerKey, installer)
}

// MakeStorageRouter adds support for external storage
func MakeStorageRouter(sub chi.Router) {
	sub.Route("/isos/{installerID}", func(r chi.Router) {
		r.Use(InstallerByIDCtx)
		r.Get("/", GetInstallerIsoStorageContent)
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
