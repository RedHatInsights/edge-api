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
	URL    string
	GetURL func(string) (resp *http.Response, err error)
}

// Add description
func (c *ConfigurableWebGetter) Get() (resp *http.Response, err error) {
	return c.GetURL(c.URL)
}

type ReadinessStatus struct {
	Readiness string `json:"readiness"`
}

// Checks that web server is running and ready.
func GetReadinessStatus(g WebGetter) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := dependencies.ServicesFromContext(r.Context())
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, err := g.Get()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			s.Log.Info("Readiness probe failed.", err)
			respondWithJSONBody(w, s.Log, ReadinessStatus{
				Readiness: "not ready",
			})
		} else {
			w.WriteHeader(http.StatusOK)
			s.Log.Info("Readiness probe succeeded.")
			respondWithJSONBody(w, s.Log, ReadinessStatus{
				Readiness: "ready",
			})
		}
	})
}
