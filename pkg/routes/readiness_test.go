package routes

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestReadinessStatus200(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	h := &GoodHandler{}
	g := &ConfigurableWebGetter{h.fauxUrl, h.Get}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetReadinessStatus(g))
	ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
		Log: log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	handler.ServeHTTP(rr, req)

	// Assert that we got a 200 response code
	assert.Equal(t, http.StatusOK, rr.Code, "Handler returned the wrong status code.")

	// Assert that response body contains expected readiness value
	var expectedValue ReadinessStatus

	data, err := io.ReadAll(rr.Body)
	assert.NoError(t, err, "Error encountered while reading response.")

	err = json.Unmarshal(data, &expectedValue)
	assert.NoError(t, err, "Error encountered while unmarshalling response.")
	assert.Equal(t, "ready", expectedValue.Readiness, "Readiness value did not match expectation")
}

func TestReadinessStatus503(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	h := &BadHandler{}
	g := &ConfigurableWebGetter{h.fauxUrl, h.Get}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetReadinessStatus(g))
	ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
		Log: log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	handler.ServeHTTP(rr, req)

	// Assert that we got a 503 response code
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "Handler returned the wrong status code.")

	// Assert that response body contains expected readiness value
	var expectedValue ReadinessStatus

	data, err := io.ReadAll(rr.Body)
	assert.NoError(t, err, "Error encountered while reading response.")

	err = json.Unmarshal(data, &expectedValue)
	assert.NoError(t, err, "Error encountered while unmarshalling response.")
	assert.Equal(t, "not ready", expectedValue.Readiness, "Readiness value did not match expectation")
}

type GoodHandler struct {
	fauxUrl string
}

func (fh *GoodHandler) Get(url string) (resp *http.Response, err error) {
	return nil, nil
}

type BadHandler struct {
	fauxUrl string
}

func (fh *BadHandler) Get(url string) (resp *http.Response, err error) {
	return nil, http.ErrServerClosed
}
