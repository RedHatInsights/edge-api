// nolint:revive
package routes

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

// Add description
type WebGetter interface {
	Get() (resp *http.Response, err error)
}

// Add description
type ConfigurableWebGetter struct { // nolint:gofmt,goimports,govet
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
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		resp, err := g.Get()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			log.WithField("error", err.Error()).Error("Readiness probe failed.")
			respondWithJSONBody(w, nil, ReadinessStatus{
				Readiness: "not ready",
			})
		} else {
			if resp.StatusCode != http.StatusOK {
				w.WriteHeader(http.StatusServiceUnavailable)
				log.WithField("http_status_code", resp.StatusCode).Info("Readiness probe failed with code.")
				respondWithJSONBody(w, nil, ReadinessStatus{
					Readiness: "not ready",
				})
			} else {
				w.WriteHeader(http.StatusOK)
				respondWithJSONBody(w, nil, ReadinessStatus{
					Readiness: "ready",
				})
			}
		}
		defer resp.Body.Close()
	})
}
