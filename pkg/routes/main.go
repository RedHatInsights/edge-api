package routes

import (
	"encoding/json"
	"net/http"

	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/routes/common"

	log "github.com/sirupsen/logrus"
)

func readAccount(w http.ResponseWriter, r *http.Request, logEntry *log.Entry) string {
	account, err := common.GetAccount(r)
	if err != nil {
		logEntry.WithField("error", err.Error()).Error("Error retrieving account")
		respondWithAPIError(w, logEntry, errors.NewBadRequest(err.Error()))
		return ""
	}
	return account
}

func respondWithAPIError(w http.ResponseWriter, logEntry *log.Entry, apiError errors.APIError) {
	w.WriteHeader(apiError.GetStatus())
	if err := json.NewEncoder(w).Encode(&apiError); err != nil {
		logEntry.WithField("error", err.Error()).Error("Error while trying to encode api error")
	}
}

func respondWithJSONBody(w http.ResponseWriter, logEntry *log.Entry, data interface{}) {
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logEntry.WithField("error", data).Error("Error while trying to encode data")
		respondWithAPIError(w, logEntry, errors.NewInternalServerError())
	}
}

func readRequestJSONBody(w http.ResponseWriter, r *http.Request, logEntry *log.Entry, dataReceiver interface{}) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dataReceiver); err != nil {
		logEntry.WithField("error", err.Error()).Error("Error parsing json from request body")
		respondWithAPIError(w, logEntry, errors.NewBadRequest("invalid JSON request"))
		return err
	}
	return nil
}
