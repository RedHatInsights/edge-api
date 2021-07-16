package images

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/commits"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/imagebuilder"
	"github.com/redhatinsights/edge-api/pkg/models"
)

var image models.Image

func setUp() {
	config.Init()
	config.Get().Debug = true
	db.InitDB()
	db.DB.AutoMigrate(&models.Commit{}, &models.UpdateTransaction{}, &models.Package{}, &models.Image{})
	image = models.Image{
		Account: "0000000",
		Status:  models.ImageStatusBuilding,
		Commit: &models.Commit{
			Status: models.ImageStatusBuilding,
		},
	}
	db.DB.Create(&image)
	imagebuilder.Client = &MockImageBuilderClient{}
	commits.RepoBuilderInstance = &MockRepositoryBuilder{}
}

func tearDown() {
	db.DB.Delete(&image)
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

func (c *MockImageBuilderClient) ComposeCommit(image *models.Image, headers map[string]string) (*models.Image, error) {
	return image, nil
}
func (c *MockImageBuilderClient) ComposeInstaller(c *models.Commit, image *models.Image, headers map[string]string) (*models.Image, error) {
	return image, nil
}
func (c *MockImageBuilderClient) GetCommitStatus(image *models.Image, headers map[string]string) (*models.Image, error) {
	image.Status = models.ImageStatusError
	image.Commit.Status = models.ImageStatusError
	return image, nil
}

func (c *MockImageBuilderClient) GetInstallerStatus(image *models.Image, headers map[string]string) (*models.Image, error) {
	image.Status = models.ImageStatusError
	image.Installer.Status = models.ImageStatusError
	return image, nil
}

type MockRepositoryBuilder struct{}

func (rb *MockRepositoryBuilder) BuildRepo(u *models.UpdateTransaction) (*models.UpdateTransaction, error) {
	return nil, nil
}

func TestCreateWasCalledWithAccountNotSet(t *testing.T) {
	config.Get().Debug = false
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
	var jsonStr = []byte(`{"Name": "image1", "Distribution": "rhel-8", "ImageType": "rhel-edge-installer", "Commit": {"Arch": "x86_64", "Packages" : [ { "name" : "vim"  } ]}}`)
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
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
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

	// if ir.Status != models.ImageStatusError { // comes from the mock above
	// 	t.Errorf("wrong image status: got %v want %v",
	// 		ir.Status, models.ImageStatusError)
	// }
}

func TestGetById(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

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

func TestValidateGetAllSearchParams(t *testing.T) {
	tt := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
		{
			name:   "bad status name",
			params: "name=image1&status=ORPHANED",
			expectedError: []validationError{
				{Key: "status", Reason: "ORPHANED is not a valid status. Status must be CREATED or BUILDING or ERROR or SUCCESS"},
			},
		},
		{
			name:   "bad created_at date",
			params: "created_at=today",
			expectedError: []validationError{
				{Key: "created_at", Reason: `parsing time "today" as "2006-01-02": cannot parse "today" as "2006"`},
			},
		},
		{
			name:   "bad sort_by",
			params: "sort_by=host",
			expectedError: []validationError{
				{Key: "sort_by", Reason: "host is not a valid sort_by. Sort-by must be status or name or distribution or created_at"},
			},
		},
		{
			name:   "bad sort_by and status",
			params: "sort_by=host&status=CREATED&status=ONHOLD",
			expectedError: []validationError{
				{Key: "sort_by", Reason: "host is not a valid sort_by. Sort-by must be status or name or distribution or created_at"},
				{Key: "status", Reason: "ONHOLD is not a valid status. Status must be CREATED or BUILDING or ERROR or SUCCESS"},
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	for _, te := range tt {
		req, err := http.NewRequest("GET", fmt.Sprintf("/images?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		validateGetAllSearchParams(next).ServeHTTP(w, req)

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
