// FIXME: golangci-lint
// nolint:errcheck,govet,revive
package unleash

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/Unleash/unleash-client-go/v4/api"
)

// FakeUnleashServer is the server object
type FakeUnleashServer struct {
	sync.RWMutex
	srv      *httptest.Server
	features map[string]bool
}

// URL returns the given fake URL
func (f *FakeUnleashServer) URL() string {
	return f.srv.URL
}

// Enable turns on a passed feature
func (f *FakeUnleashServer) Enable(feature string) {
	f.setEnabled(feature, true)
}

// Disable turns off a passed feature
func (f *FakeUnleashServer) Disable(feature string) {
	f.setEnabled(feature, false)
}

// setEnable is a helper function to turn on and off features
func (f *FakeUnleashServer) setEnabled(feature string, enabled bool) {
	f.Lock()
	wasEnabled := f.features[feature]
	if enabled != wasEnabled {
		f.features[feature] = enabled
	}
	f.Unlock()
}

// IsEnabled returns a bool of if a feature is enabled or not
func (f *FakeUnleashServer) IsEnabled(feature string) bool {
	f.RLock()
	enabled := f.features[feature]
	f.RUnlock()
	return enabled
}

// setAll is a helper function to set all features
func (f *FakeUnleashServer) setAll(enabled bool) {
	for k := range f.features {
		f.setEnabled(k, enabled)
	}
}

// EnableAll turns on all features
func (f *FakeUnleashServer) EnableAll() {
	f.setAll(true)
}

// DisableAll turns off all features
func (f *FakeUnleashServer) DisableAll() {
	f.setAll(false)
}

// handler handles http requests to the fake server
func (f *FakeUnleashServer) handler(w http.ResponseWriter, req *http.Request) {
	switch req.Method + " " + req.URL.Path {
	case "GET /client/features":

		features := []api.Feature{}
		f.Lock()
		for k, v := range f.features {
			features = append(features, api.Feature{
				Name:    k,
				Enabled: v,
				Strategies: []api.Strategy{
					{
						Id:   0,
						Name: "default",
					},
				},
				CreatedAt: time.Time{},
			})
		}
		f.Unlock()

		res := api.FeatureResponse{
			Response: api.Response{Version: 2},
			Features: features,
		}
		dec := json.NewEncoder(w)
		if err := dec.Encode(res); err != nil {
			println(err.Error())
		}
	case "POST /client/register":
		fallthrough
	case "POST /client/metrics":
		w.WriteHeader(200)
	default:
		w.Write([]byte("Unknown route"))
		w.WriteHeader(500)
	}
}

// NewFakeUnleash is the init function for the fake server
func NewFakeUnleash() *FakeUnleashServer {
	faker := &FakeUnleashServer{
		features: map[string]bool{},
	}
	faker.srv = httptest.NewServer(http.HandlerFunc(faker.handler))
	return faker
}
