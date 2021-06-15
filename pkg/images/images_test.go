package images

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/commits"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/imagebuilder"
	"github.com/redhatinsights/edge-api/pkg/models"
)

func TestCreateWasCalledWithWrongBody(t *testing.T) {
	var jsonStr = []byte(`{bad json}`)
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(Create)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
	}
}

type MockImageBuilderClient struct{}

func (c *MockImageBuilderClient) Compose(image *models.Image) (*models.Image, error) {
	return image, nil
}

func TestCreateWasCalledWithAccountNotSet(t *testing.T) {
	imagebuilder.Client = &MockImageBuilderClient{}
	var jsonStr = []byte(`{"Distribution": "rhel-8", "OutputType": "tar", "Commit": {"Arch": "x86_64", "Packages" : [ { "name" : "vim"  } ]}}`)
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(Create)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestCreate(t *testing.T) {
	config.Init()
	config.Get().Debug = true
	db.InitDB()
	db.DB.AutoMigrate(&models.Commit{}, &commits.UpdateRecord{}, &models.Package{}, &models.Image{}) // We need to discuss all-things testing against databases and setup, teardown and test suites

	imagebuilder.Client = &MockImageBuilderClient{}
	var jsonStr = []byte(`{"Distribution": "rhel-8", "OutputType": "tar", "Commit": {"Arch": "x86_64", "Packages" : [ { "name" : "vim"  } ]}}`)
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(Create)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}
