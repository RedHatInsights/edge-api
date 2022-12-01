package routes

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestReadinessStatus(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

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

	data, err := ioutil.ReadAll(rr.Body)
	assert.NoError(t, err, "Error encountered while reading response.")

	err = json.Unmarshal(data, &expectedValue)
	assert.NoError(t, err, "Error encountered while unmarshalling response.")
	assert.Equal(t, "ready", expectedValue.Readiness, "Readiness value did not match expectation")
}
