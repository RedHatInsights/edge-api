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

	resp := &http.Response{
		StatusCode: http.StatusOK,
	}
	goodHandler := &ConfigurableWebGetter{
		URL: "",
		GetURL: func(string) (*http.Response, error) {
			return resp, nil
		},
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetReadinessStatus(goodHandler))
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

func TestReadinessStatus404(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp := &http.Response{
		StatusCode: http.StatusNotFound,
	}
	badHandler := &ConfigurableWebGetter{
		URL: "",
		GetURL: func(string) (*http.Response, error) {
			return resp, nil
		},
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetReadinessStatus(badHandler))
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

func TestReadinessStatus503(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	badHandler := &ConfigurableWebGetter{
		URL: "",
		GetURL: func(string) (*http.Response, error) {
			return nil, http.ErrServerClosed
		},
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetReadinessStatus(badHandler))
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
