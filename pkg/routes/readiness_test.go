package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func serveWeb() *http.Server {
	server := http.Server{
		Addr:         fmt.Sprintf(":%d", 31972),
		Handler:      http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Info("web service stopped unexpectedly", err)
		}
	}()
	log.Info("web service started")
	return &server
}

func TestReadinessStatusOK(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	server := serveWeb()
	defer server.Shutdown(context.Background())

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetReadinessStatus)
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

func TestReadinessStatusNotOK(t *testing.T) {
	req, _ := http.NewRequest("GET", "/DOESNTEXIT", nil)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetReadinessStatus)
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
