// FIXME: golangci-lint
// nolint:revive
package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	log "github.com/sirupsen/logrus"
)

func TestCreateWasCalledWithURLNotSet(t *testing.T) {
	config.Get().Debug = false
	jsonRepo := &models.ThirdPartyRepo{
		Description: "This is Third Party repository",
		Name:        "Repository1",
	}
	jsonRepoBytes, err := json.Marshal(jsonRepo)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(jsonRepoBytes))
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		Log: log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateThirdPartyRepo)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
	config.Get().Debug = true
}

func TestCreateThirdPartyRepo(t *testing.T) {
	jsonRepo := &models.ThirdPartyRepo{
		URL:         "http://www.thirdpartyurl.com/in/thisrepo",
		Description: "This is Third Party repository",
		Name:        "Repository1",
	}
	jsonRepoBytes, err := json.Marshal(jsonRepo)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(jsonRepoBytes))
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockThirdPartyRepoService := mock_services.NewMockThirdPartyRepoServiceInterface(ctrl)
	mockThirdPartyRepoService.EXPECT().CreateThirdPartyRepo(gomock.Any(), gomock.Any()).Return(&tprepo, nil)
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ThirdPartyRepoService: mockThirdPartyRepoService,
		Log:                   log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateThirdPartyRepo)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)

	}

}
func TestGetAllThirdPartyRepo(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetAllThirdPartyRepo)
	ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
		Log: log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)

	}
}

func TestGetAllThirdPartyRepoQueryParams(t *testing.T) {
	tt := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
		{
			name:   "invalid query param",
			params: "bla=1",
			expectedError: []validationError{
				{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: %s", GetQueryParamsArray("thirdpartyrepo"))},
			},
		},
		{
			name:   "valid query param and invalid query param",
			params: "sort_by=created_at&bla=1",
			expectedError: []validationError{
				{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: %s", GetQueryParamsArray("thirdpartyrepo"))},
			},
		},
		{
			name:   "invalid query param and valid query param",
			params: "bla=1&sort_by=created_at",
			expectedError: []validationError{
				{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: %s", GetQueryParamsArray("thirdpartyrepo"))},
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	for _, te := range tt {
		req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockThirdPartyRepoService := mock_services.NewMockThirdPartyRepoServiceInterface(ctrl)
		ctx := req.Context()
		ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
			ThirdPartyRepoService: mockThirdPartyRepoService,
			Log:                   log.NewEntry(log.StandardLogger()),
		})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		ValidateQueryParams("thirdpartyrepo")(next).ServeHTTP(w, req)

		resp := w.Result()
		jsonBody := []validationError{}
		err = json.NewDecoder(resp.Body).Decode(&jsonBody)
		if err != nil {
			t.Errorf("failed decoding response body: %s", err.Error())
		}
		for _, exErr := range te.expectedError {
			found := false
			for _, jsErr := range jsonBody {
				if jsErr.Key == exErr.Key && jsErr.Reason == exErr.Reason {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("in %q: was expected to have %v but not found in %v", te.name, exErr, jsonBody)
			}
		}
	}
}

func TestGetAllThirdPartyRepoFilterParams(t *testing.T) {
	tt := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
		{
			name:   "bad created_at date",
			params: "created_at=today",
			expectedError: []validationError{
				{Key: "created_at", Reason: `parsing time "today" as "2006-01-02": cannot parse "today" as "2006"`},
			},
		},
		{
			name:   "bad sort_by",
			params: "sort_by=test",
			expectedError: []validationError{
				{Key: "sort_by", Reason: "test is not a valid sort_by. Sort-by must be name or created_at or updated_at"},
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	for _, te := range tt {
		req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockThirdPartyRepoService := mock_services.NewMockThirdPartyRepoServiceInterface(ctrl)
		ctx := req.Context()
		ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
			ThirdPartyRepoService: mockThirdPartyRepoService,
			Log:                   log.NewEntry(log.StandardLogger()),
		})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		validateGetAllThirdPartyRepoFilterParams(next).ServeHTTP(w, req)

		resp := w.Result()
		jsonBody := []validationError{}
		err = json.NewDecoder(resp.Body).Decode(&jsonBody)
		if err != nil {
			t.Errorf("failed decoding response body: %s", err.Error())
		}
		for _, exErr := range te.expectedError {
			found := false
			for _, jsErr := range jsonBody {
				if jsErr.Key == exErr.Key && jsErr.Reason == exErr.Reason {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("in %q: was expected to have %v but not found in %v", te.name, exErr, jsonBody)
			}
		}
	}
}
