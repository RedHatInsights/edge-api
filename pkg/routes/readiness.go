package routes

import (
	"net/http"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
)

// Add description
type WebGetter interface {
	Get() (resp *http.Response, err error)
}

// Add description
type ConfigurableWebGetter struct {
	Url string
	GetUrl func(string) (resp *http.Response, err error)
}

// Add description
func (c *ConfigurableWebGetter) Get() (resp *http.Response, err error) {
	return c.GetUrl(c.Url)
}

type ReadinessStatus struct {
	Readiness string `json:"readiness"`
}

// Checks that web server is running and ready.
func GetReadinessStatus(g WebGetter) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		ctxServices.Log.Debug("Checking service readiness")

		_, err := g.Get()
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
	})
}
