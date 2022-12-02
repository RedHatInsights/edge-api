package routes

import (
	"fmt"
	"net/http"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
)

type ReadinessStatus struct {
	Readiness string `json:"readiness"`
}

func GetReadinessStatus(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	ctxServices.Log.Debug("Checking service readiness")

	url := fmt.Sprintf("%s:%d", config.Get().Hostname, 31972)
	_, err := http.Get(url)

	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		respondWithJSONBody(w, ctxServices.Log, ReadinessStatus{
			Readiness: "not ready",
		})
	} else {
		w.WriteHeader(http.StatusOK)
		respondWithJSONBody(w, ctxServices.Log, ReadinessStatus{
			Readiness: "ready",
		})
	}
}
