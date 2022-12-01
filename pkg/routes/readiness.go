package routes

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
)

type ReadinessStatus struct {
	Readiness string `json:"readiness"`
}

func MakeReadinessRouter(sub chi.Router) {
	sub.Get("/", GetReadinessStatus)
}

func GetReadinessStatus(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	ctxServices.Log.Debug("Checking service readiness")

	w.WriteHeader(http.StatusOK)
	respondWithJSONBody(w, ctxServices.Log, ReadinessStatus{
		Readiness: "ready",
	})
}
