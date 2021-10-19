package routes

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
)

func TestCreateWasCalledWithURLNotSet(t *testing.T) {
	config.Get().Debug = false
	var jsonStr = []byte(`{
		"Description": "This is Third Party repository",
  	    "Name": "Repository1"
	}`)
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateThirdPartyRepo)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
	config.Get().Debug = true
}

func TestCreateThirdPartyRepo(t *testing.T) {
	var jsonStr = []byte(`{
		"URL": "http://www.thirdpartyurl.com/in/thisrepo",
    	"Description": "This is Third Party repository",
    	"Name": "Repository1"
		}
	}`)
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockThirdPartyRepoService := mock_services.NewMockThirdPartyRepoServiceInterface(ctrl)
	mockThirdPartyRepoService.EXPECT().CreateThirdPartyRepo(gomock.Any(), gomock.Any()).Return(nil)
	ctx = context.WithValue(ctx, dependencies.Key, &dependencies.EdgeAPIServices{
		ThirdPartyRepoService: mockThirdPartyRepoService,
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
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)

	}
}
