// FIXME: golangci-lint
// nolint:revive,unused
package routes

import (
	"encoding/json"
	"net/http"

	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/routes/common"

	log "github.com/osbuild/logging/pkg/logrus"
)

func readAccount(w http.ResponseWriter, r *http.Request, logEntry log.FieldLogger) string {
	account, err := common.GetAccount(r)
	if err != nil {
		logEntry.WithField("error", err.Error()).Error("Error retrieving account")
		respondWithAPIError(w, logEntry, errors.NewBadRequest(err.Error()))
		return ""
	}
	return account
}

func readOrgID(w http.ResponseWriter, r *http.Request, logEntry log.FieldLogger) string {
	orgID, err := common.GetOrgID(r)
	if err != nil {
		logEntry.WithField("error", err.Error()).Error("Error retrieving org_id")
		respondWithAPIError(w, logEntry, errors.NewBadRequest(err.Error()))
		return ""
	}
	return orgID
}

func readAccountOrOrgID(w http.ResponseWriter, r *http.Request, logEntry log.FieldLogger) (string, string) {
	account, accountErr := common.GetAccount(r)
	orgID, orgIDError := common.GetOrgID(r)
	if accountErr != nil && orgIDError != nil {
		logEntry.WithField(
			"error", "cannot retrieve account and orgID from request context").Error("Error retrieving account and orgID")
		respondWithAPIError(w, logEntry, errors.NewBadRequest("Error retrieving account and orgID"))
		return "", ""
	}
	return account, orgID
}

func respondWithAPIError(w http.ResponseWriter, logEntry log.FieldLogger, apiError errors.APIError) {
	w.WriteHeader(apiError.GetStatus())
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	if err := json.NewEncoder(w).Encode(&apiError); err != nil {
		logEntry.WithField("error", err.Error()).Error("Error while trying to encode api error")
	}
}

func respondWithJSONBody(w http.ResponseWriter, logEntry log.FieldLogger, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logEntry.WithField("error", data).Error("Error while trying to encode data")
		respondWithAPIError(w, logEntry, errors.NewInternalServerError())
	}
}

func readRequestJSONBody(w http.ResponseWriter, r *http.Request, logEntry log.FieldLogger, dataReceiver interface{}) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dataReceiver); err != nil {
		logEntry.WithField("error", err.Error()).Error("Error parsing json from request body")
		respondWithAPIError(w, logEntry, errors.NewBadRequest("invalid JSON request"))
		return err
	}
	return nil
}
