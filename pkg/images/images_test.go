package images

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/imagebuilder"
	"github.com/redhatinsights/edge-api/pkg/models"
	"gorm.io/gorm"
)

var tx *gorm.DB

func setUp() {
	config.Init()
	config.Get().Debug = true
	db.InitDB()
	db.DB.AutoMigrate(&models.Commit{}, &models.UpdateRecord{}, &models.Package{}, &models.Image{})
	tx = db.DB.Begin()
}

func tearDown() {
	tx.Rollback()
}
func TestMain(m *testing.M) {
	setUp()
	retCode := m.Run()
	tearDown()
	os.Exit(retCode)
}

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

func (c *MockImageBuilderClient) Compose(image *models.Image, headers map[string]string) (*models.Image, error) {
	return image, nil
}
func (c *MockImageBuilderClient) GetStatus(image *models.Image, headers map[string]string) (*models.Image, error) {
	image.Status = models.ImageStatusError
	return image, nil
}

func TestCreateWasCalledWithAccountNotSet(t *testing.T) {
	config.Get().Debug = false
	imagebuilder.Client = &MockImageBuilderClient{}
	var jsonStr = []byte(`{"Distribution": "rhel-8", "ImageType": "rhel-edge-installer", "Commit": {"Arch": "x86_64", "Packages" : [ { "name" : "vim"  } ]}}`)
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
	config.Get().Debug = true
}

func TestCreate(t *testing.T) {
	imagebuilder.Client = &MockImageBuilderClient{}
	var jsonStr = []byte(`{"Distribution": "rhel-8", "ImageType": "rhel-edge-installer", "Commit": {"Arch": "x86_64", "Packages" : [ { "name" : "vim"  } ]}}`)
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
	tx.Create(&image)
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

func TestGetById(t *testing.T) {
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
	tx.Create(&image)
	ctx := context.WithValue(req.Context(), imageKey, &image)
	handler := http.HandlerFunc(GetByID)
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return
	}

	var ir models.Image
	respBody, err := ioutil.ReadAll(rr.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	err = json.Unmarshal(respBody, &ir)
	if err != nil {
		t.Errorf(err.Error())
	}

	if ir.ID != image.ID {
		t.Errorf("wrong image status: got %v want %v",
			ir.ID, image.ID)
	}
}

func TestGetStatuses(t *testing.T) {
	expected := []APIImage{
		{ID: "1", Name: "", Status: models.ImageStatusBuilding},
		{ID: "blabla", Name: "", Status: "ENOTEXIST"},
	}
	r, err := http.NewRequest("GET", "/images/status?id=1&id=blabla", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	result := []APIImage{}
	GetStatuses(w, r)
	err = json.NewDecoder(w.Result().Body).Decode(&result)
	if err != nil {
		t.Errorf("Failed decoding response body (%v): %s", w, err)
	}
	for ix, _ := range result {
		if result[ix].Name != expected[ix].Name {
			t.Errorf("Expected in line %d the name to be %q but got %q", ix, expected[ix].Name, result[ix].Name)
		}
		if result[ix].ID != expected[ix].ID {
			t.Errorf("Expected in line %d the id to be %q but got %q", ix, expected[ix].ID, result[ix].ID)
		}
		if result[ix].Status != expected[ix].Status {
			t.Errorf("Expected in line %d the status to be %q but got %q", ix, expected[ix].Status, result[ix].Status)
		}
	}
}
