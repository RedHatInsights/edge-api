package routes

import (
	"encoding/json"
	"net/http"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
)

func internalError(err error, r *http.Request, w http.ResponseWriter) error {
	services := dependencies.ServicesFromContext(r.Context())
	services.Log.WithField("error", err.Error()).Error("Internal Server Error")
	internalError := errors.NewInternalServerError()
	w.WriteHeader(internalError.GetStatus())
	if err := json.NewEncoder(w).Encode(&internalError); err != nil {
		services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		return err
	}
	return nil
}
