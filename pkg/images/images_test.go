package images

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
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
func (c *MockImageBuilderClient) GetStatus(image *models.Image) (*models.Image, error) {
	image.Status = models.ImageStatusError
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
	db.DB.AutoMigrate(&models.Commit{}, &commits.UpdateRecord{}, &models.Package{}, &models.Image{})

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
func TestGetStatus(t *testing.T) {
	config.Init()
	config.Get().Debug = true
	db.InitDB()
	db.DB.AutoMigrate(&models.Commit{}, &commits.UpdateRecord{}, &models.Package{}, &models.Image{})

	imagebuilder.Client = &MockImageBuilderClient{}
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	var image = models.Image{
		Account:      "0000000",
		ComposeJobID: "123",
		Status:       models.ImageStatusBuilding,
		Commit:       &models.Commit{},
	}
	db.DB.Create(&image)
	ctx := context.WithValue(req.Context(), imageKey, &image)
	handler := http.HandlerFunc(GetStatusByID)
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return
	}

	var ir struct {
		Status string
	}
	respBody, err := ioutil.ReadAll(rr.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	err = json.Unmarshal(respBody, &ir)
	if err != nil {
		t.Errorf(err.Error())
	}

	if ir.Status != models.ImageStatusError { // comes from the mock above
		t.Errorf("wrong image status: got %v want %v",
			ir.Status, models.ImageStatusError)
	}
}
