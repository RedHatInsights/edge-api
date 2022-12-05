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
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		resp, err := g.Get()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			log.Info("Readiness probe failed.", err)
			respondWithJSONBody(w, nil, ReadinessStatus{
				Readiness: "not ready",
			})
		} else {
			if resp.StatusCode != http.StatusOK {
				w.WriteHeader(http.StatusServiceUnavailable)
				log.Info("Readiness probe failed with code.", resp.StatusCode)
				respondWithJSONBody(w, nil, ReadinessStatus{
					Readiness: "not ready",
				})
			} else {
				w.WriteHeader(http.StatusOK)
				log.Info("Readiness probe succeeded.")
				respondWithJSONBody(w, nil, ReadinessStatus{
					Readiness: "ready",
				})
			}
		}
	})
}
