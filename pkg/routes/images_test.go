// FIXME: golangci-lint
// nolint:govet,revive,typecheck
package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"

	"github.com/bxcodec/faker/v3"
	"github.com/go-chi/chi"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
)

func TestCreateWasCalledWithWrongBody(t *testing.T) {
	var jsonStr = []byte(`{bad json}`)
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
		Log: log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateImage)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
	}
}

func TestCreateWasCalledWithNameNotSet(t *testing.T) {
	config.Get().Debug = false
	jsonImage := &models.Image{
		Distribution: "rhel-8",
		OutputTypes:  []string{"rhel-edge-installer"},
		Commit: &models.Commit{
			Arch: "x86_64",
			InstalledPackages: []models.InstalledPackage{
				{Name: "vim"},
			},
		},
		Installer: &models.Installer{
			Username: "root",
			SSHKey:   "ssh-rsa d9:f158:00:abcd",
		},
	}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(jsonImage)
	if err != nil {
		t.Errorf(err.Error())
	}
	req, err := http.NewRequest("POST", "/", &buf)
	if err != nil {
		t.Fatal(err)
	}
	ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
		Log: log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateImage)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
	config.Get().Debug = true
}

func TestCreate(t *testing.T) {
	jsonImage := &models.Image{
		Name:         "image2",
		Distribution: "rhel-8",
		OutputTypes:  []string{"rhel-edge-installer"},
		Commit: &models.Commit{
			Arch: "x86_64",
			InstalledPackages: []models.InstalledPackage{
				{Name: "vim"},
			},
		},
		Installer: &models.Installer{
			Username: "test",
			SSHKey:   "ssh-rsa d9:f158:00:abcd",
		},
	}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(jsonImage)
	if err != nil {
		t.Errorf(err.Error())
	}
	req, err := http.NewRequest("POST", "/", &buf)
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	mockImageService.EXPECT().CreateImage(gomock.Any()).Return(nil)
	mockImageService.EXPECT().ProcessImage(gomock.Any(), gomock.Any()).Return(nil)
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateImage)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)

	}
}
func TestCreateWithInvalidPackageName(t *testing.T) {
	jsonImage := &models.Image{
		Name:         "image2",
		Distribution: "rhel-85",
		OutputTypes:  []string{"rhel-edge-installer"},
		Commit: &models.Commit{
			Arch: "x86_64",
		},
		Packages: []models.Package{
			{Name: "vanilla"},
		},
		Installer: &models.Installer{
			Username: "test",
			SSHKey:   "ssh-rsa d9:f158:00:abcd",
		},
	}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(jsonImage)
	if err != nil {
		t.Errorf(err.Error())
	}
	req, err := http.NewRequest("POST", "/", &buf)
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	mockImageService.EXPECT().CreateImage(gomock.Any()).Return(new(services.PackageNameDoesNotExist))
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})

	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateImage)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}
func TestCreateWithThirdPartyRepositoryInfoInvalid(t *testing.T) {
	jsonImage := &models.Image{
		Name:         "image2",
		Distribution: "rhel-85",
		OutputTypes:  []string{"rhel-edge-installer"},
		Commit: &models.Commit{
			Arch: "x86_64",
		},
		Packages: []models.Package{
			{Name: "vim-common"},
		},
		Installer: &models.Installer{
			Username: "test",
			SSHKey:   "ssh-rsa d9:f158:00:abcd",
		},
	}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(jsonImage)
	if err != nil {
		t.Errorf(err.Error())
	}
	req, err := http.NewRequest("POST", "/", &buf)
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	mockImageService.EXPECT().CreateImage(gomock.Any()).Return(new(services.ThirdPartyRepositoryInfoIsInvalid))
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})

	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateImage)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}
func TestCreateWithImageNameAlreadyExist(t *testing.T) {
	jsonImage := &models.Image{
		Name:         "ImageNameAlreadyExist",
		Distribution: "rhel-85",
		OutputTypes:  []string{"rhel-edge-installer"},
		Commit: &models.Commit{
			Arch: "x86_64",
		},
		Packages: []models.Package{
			{Name: "vim-common"},
		},
		Installer: &models.Installer{
			Username: "test",
			SSHKey:   "ssh-rsa d9:f158:00:abcd",
		},
	}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(jsonImage)
	if err != nil {
		t.Errorf(err.Error())
	}
	req, err := http.NewRequest("POST", "/", &buf)
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	mockImageService.EXPECT().CreateImage(gomock.Any()).Return(new(services.ImageNameAlreadyExists))

	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})

	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateImage)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}
func TestGetStatus(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})
	rr := httptest.NewRecorder()
	ctx = context.WithValue(ctx, imageKey, &testImage)
	handler := http.HandlerFunc(GetImageStatusByID)
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return
	}

	var ir struct {
		Status string
	}
	respBody, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	err = json.Unmarshal(respBody, &ir)
	if err != nil {
		t.Errorf(err.Error())
	}
}

type ImageResponse struct {
	Image              models.Image `json:"image"`
	AdditionalPackages int          `json:"additional_packages"`
	Packages           int          `json:"packages"`
	UpdateAdded        int          `json:"update_added"`
	UpdateRemoved      int          `json:"update_removed"`
	UpdateUpdated      int          `json:"update_updated"`
}

func TestGetImageDetailsById(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	mockImageService.EXPECT().GetUpdateInfo(gomock.Any()).Return(nil, nil)

	ctx := context.WithValue(req.Context(), imageKey, &testImage)

	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})

	handler := http.HandlerFunc(GetImageDetailsByID)
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return
	}

	var ir ImageResponse
	respBody, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	err = json.Unmarshal(respBody, &ir)
	if err != nil {
		t.Errorf(err.Error())
	}

	if ir.Packages != 1 {
		t.Errorf("wrong image packages: got %v want %v",
			ir.Packages, 1)
	}
	if ir.AdditionalPackages != 0 {
		t.Errorf("wrong image AdditionalPackages: got %v want %v",
			ir.AdditionalPackages, 0)
	}
	if ir.UpdateAdded != 0 {
		t.Errorf("wrong image UpdateAdded: got %v want %v",
			ir.UpdateAdded, 0)
	}
	if ir.UpdateRemoved != 0 {
		t.Errorf("wrong image UpdateRemoved: got %v want %v",
			ir.UpdateRemoved, 0)
	}
	if ir.UpdateUpdated != 0 {
		t.Errorf("wrong image UpdateUpdated: got %v want %v",
			ir.UpdateUpdated, 0)
	}
	if ir.Image.ID != testImage.ID {
		t.Errorf("wrong image status: got %v want %v",
			ir.Image.ID, testImage.ID)
	}
}

func TestGetImageDetailsByIdWithUpdate(t *testing.T) {
	imaged := models.ImageUpdateAvailable{}
	imaged.Image.TotalDevicesWithImage = 4
	imaged.Image.TotalPackages = 3

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	mockImageService.EXPECT().GetUpdateInfo(gomock.Any()).Return(&imaged, nil)
	ctx := context.WithValue(req.Context(), imageKey, &testImage)

	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})

	handler := http.HandlerFunc(GetImageDetailsByID)
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return
	}

	var ir ImageResponse
	respBody, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	err = json.Unmarshal(respBody, &ir)
	if err != nil {
		t.Errorf(err.Error())
	}

	if ir.Packages != 1 {
		t.Errorf("wrong image packages: got %v want %v",
			ir.Packages, 1)
	}
	if ir.AdditionalPackages != 0 {
		t.Errorf("wrong image AdditionalPackages: got %v want %v",
			ir.AdditionalPackages, 0)
	}
	if ir.UpdateAdded != 0 {
		t.Errorf("wrong image UpdateAdded: got %v want %v",
			ir.UpdateAdded, 0)
	}
	if ir.UpdateRemoved != 0 {
		t.Errorf("wrong image UpdateRemoved: got %v want %v",
			ir.UpdateRemoved, 0)
	}
	if ir.UpdateUpdated != 0 {
		t.Errorf("wrong image UpdateUpdated: got %v want %v",
			ir.UpdateUpdated, 0)
	}
	if ir.Image.ID != testImage.ID {
		t.Errorf("wrong image status: got %v want %v",
			ir.Image.ID, testImage.ID)
	}
	if ir.Image.TotalDevicesWithImage != testImage.TotalDevicesWithImage {
		t.Errorf("wrong total devices for image: got %v want %v",
			ir.Image.TotalDevicesWithImage, testImage.TotalDevicesWithImage)
	}
	if ir.Image.TotalPackages != testImage.TotalPackages {
		t.Errorf("wrong total packages for image: got %v want %v",
			ir.Image.TotalPackages, testImage.TotalPackages)
	}

}

func TestGetImageDetailsByIDWithError(t *testing.T) {
	// The Buffer type implements the Writer interface
	var buffer bytes.Buffer
	testLog := log.NewEntry(log.StandardLogger())
	// Set the output to use our local buffer
	testLog.Logger.SetOutput(&buffer)
	// An error to be raised by GetUpdateInfo call
	forcedErr := io.EOF

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	mockImageService.EXPECT().GetUpdateInfo(gomock.Any()).Return(nil, forcedErr)
	ctx := context.WithValue(req.Context(), imageKey, &testImage)

	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})

	handler := http.HandlerFunc(GetImageDetailsByID)
	handler.ServeHTTP(rr, req.WithContext(ctx))

	got := buffer.String()
	expected := "Error getting update info"

	if !strings.Contains(got, expected) {
		t.Errorf("got %q expected %q", got, expected)
	}

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
		return
	}
}

func TestValidateGetAllFilterParameters(t *testing.T) {
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
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
		ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
			ImageService: mockImageService,
			Log:          log.NewEntry(log.StandardLogger()),
		})
		req = req.WithContext(ctx)

		ValidateGetAllImagesSearchParams(next).ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()
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

func TestValidateGetAllQueryParameters(t *testing.T) {
	tt := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
		{
			name:   "invalid query param",
			params: "bla=1",
			expectedError: []validationError{
				{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: %s", GetQueryParamsArray("images"))},
			},
		},
		{
			name:   "valid query param and invalid query param",
			params: "sort_by=created_at&bla=1",
			expectedError: []validationError{
				{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: %s", GetQueryParamsArray("images"))},
			},
		},
		{
			name:   "invalid query param and valid query param",
			params: "bla=1&sort_by=created_at",
			expectedError: []validationError{
				{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: %s", GetQueryParamsArray("images"))},
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
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
		ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
			ImageService: mockImageService,
			Log:          log.NewEntry(log.StandardLogger()),
		})
		req = req.WithContext(ctx)

		ValidateQueryParams("images")(next).ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()
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

func TestGetRepoForImage(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	ctx := req.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockRepoService := mock_services.NewMockRepoServiceInterface(ctrl)
	mockRepoService.EXPECT().GetRepoByID(gomock.Any()).Return(&testRepo, nil)
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		RepoService: mockRepoService,
		Log:         log.NewEntry(log.StandardLogger()),
	})
	ctx = context.WithValue(ctx, imageKey, &testImage)
	req = req.WithContext(ctx)
	handler := http.HandlerFunc(GetRepoForImage)
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return
	}

	var repoResponse models.Repo
	respBody, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	err = json.Unmarshal(respBody, &repoResponse)
	if err != nil {
		t.Errorf(err.Error())
	}

	if repoResponse.ID != testRepo.ID {
		t.Errorf("wrong repo id: got %v want %v",
			repoResponse.ID, testRepo.ID)
	}
}

func TestGetRepoForImageWhenNotFound(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	ctx := req.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockRepoService := mock_services.NewMockRepoServiceInterface(ctrl)
	mockRepoService.EXPECT().GetRepoByID(gomock.Any()).Return(nil, errors.New("not found"))
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		RepoService: mockRepoService,
		Log:         log.NewEntry(log.StandardLogger()),
	})
	ctx = context.WithValue(ctx, imageKey, &testImage)
	req = req.WithContext(ctx)
	handler := http.HandlerFunc(GetRepoForImage)
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
		return
	}
}

func TestGetImageByOstree(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})
	ctx = context.WithValue(ctx, imageKey, &testImage)
	handler := http.HandlerFunc(GetImageByOstree)
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return
	}

	var ir models.Image
	respBody, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	err = json.Unmarshal(respBody, &ir)
	if err != nil {
		t.Errorf(err.Error())
	}

	if ir.ID != testImage.ID {
		t.Errorf("wrong image status: got %v want %v",
			ir.ID, testImage.ID)
	}
}

func TestPostCheckImageNameAlreadyExist(t *testing.T) {
	jsonImage := &models.Image{
		Name:         "Image Name in DB",
		Distribution: "rhel-8",
		OutputTypes:  []string{"rhel-edge-installer"},
		Commit: &models.Commit{
			Arch: "x86_64",
			InstalledPackages: []models.InstalledPackage{
				{Name: "vim"},
			},
		},
		Installer: &models.Installer{
			Username: "root",
			SSHKey:   "ssh-rsa d9:f158:00:abcd",
		},
	}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(jsonImage)
	if err != nil {
		t.Errorf(err.Error())
	}
	req, err := http.NewRequest("POST", "/", &buf)
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	mockImageService.EXPECT().CheckImageName(gomock.Any(), gomock.Any()).Return(true, nil)
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(CheckImageName)

	handler.ServeHTTP(rr, req)
	t.Log(rr.Body)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
	}

}

func TestPostCheckImageNameDoesNotExist(t *testing.T) {
	jsonImage := &models.Image{
		Name:         "Image Name not in DB",
		Distribution: "rhel-8",
		OutputTypes:  []string{"rhel-edge-installer"},
		Commit: &models.Commit{
			Arch: "x86_64",
			InstalledPackages: []models.InstalledPackage{
				{Name: "vim"},
			},
		},
		Installer: &models.Installer{
			Username: "root",
			SSHKey:   "ssh-rsa d9:f158:00:abcd",
		},
	}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(jsonImage)
	if err != nil {
		t.Errorf(err.Error())
	}
	req, err := http.NewRequest("POST", "/checkImageName", &buf)
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	mockImageService.EXPECT().CheckImageName(gomock.Any(), gomock.Any()).Return(false, nil)
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CheckImageName)

	handler.ServeHTTP(rr, req)
	respBody, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Errorf(err.Error())
	}
	var ir bool
	if err := json.Unmarshal(respBody, &ir); err != nil {
		log.Error("Error while trying to unmarshal ", &ir)
	}
	if ir != false {
		t.Errorf("fail to validate name should exists")
	}
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)
	}
}

func TestGetImage(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(req.Context(), imageKey, &testImage)
	w := new(http.ResponseWriter)

	if image := getImage(*w, req.WithContext(ctx)); image == nil {
		t.Errorf("image should not be nil")
	}
}

func TestDeleteImage(t *testing.T) {
	errorImage := &models.Image{
		Name:   "Image Name",
		Status: models.ImageStatusError,
		OrgID:  common.DefaultOrgID,
	}
	result := db.DB.Create(errorImage)
	if result.Error != nil {
		t.Errorf("saving image error got %s", result.Error)
	}
	req, err := http.NewRequest("DELETE", fmt.Sprintf("/images/%v/", errorImage.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	mockImageService.EXPECT().DeleteImage(gomock.Any()).Return(nil)
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})
	ctx = context.WithValue(ctx, imageKey, errorImage)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(DeleteImage)

	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)
	}
}

func TestDeleteImageFail(t *testing.T) {
	successImage := &models.Image{
		Name:   "Image Name",
		Status: models.ImageStatusSuccess,
		OrgID:  common.DefaultOrgID,
	}
	result := db.DB.Create(successImage)
	if result.Error != nil {
		t.Errorf("saving image error got %s", result.Error)
	}
	req, err := http.NewRequest("DELETE", fmt.Sprintf("/images/%v/", successImage.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	mockImageService.EXPECT().DeleteImage(gomock.Any()).Return(new(services.ImageNotInErrorState))
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})
	ctx = context.WithValue(ctx, imageKey, successImage)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(DeleteImage)

	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusBadRequest)
	}
}

func TestDeleteImageInvalidImage(t *testing.T) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("/images/%v/", nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(DeleteImage)

	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusBadRequest)
	}
}

func TestDeleteImageOrgIDMissmatch(t *testing.T) {
	errorImage := &models.Image{
		Name:   "Image Name",
		Status: models.ImageStatusError,
		OrgID:  "00001111",
	}
	result := db.DB.Create(errorImage)
	if result.Error != nil {
		t.Errorf("saving image error got %s", result.Error)
	}
	req, err := http.NewRequest("DELETE", fmt.Sprintf("/images/%v/", errorImage.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ImageService: mockImageService,
		Log:          log.NewEntry(log.StandardLogger()),
	})
	ctx = context.WithValue(ctx, imageKey, errorImage)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(DeleteImage)

	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusBadRequest)
	}
}

var _ = Describe("Images Route Tests", func() {

	Context("Routes", func() {
		var ctrl *gomock.Controller
		var router chi.Router
		var mockImagesService *mock_services.MockImageServiceInterface
		var edgeAPIServices *dependencies.EdgeAPIServices

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockImagesService = mock_services.NewMockImageServiceInterface(ctrl)
			edgeAPIServices = &dependencies.EdgeAPIServices{
				ImageService: mockImagesService,
				Log:          log.NewEntry(log.StandardLogger()),
			}
			router = chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := dependencies.ContextWithServices(r.Context(), edgeAPIServices)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			router.Route("/images", MakeImagesRouter)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Context("GetAllImages", func() {
			name := faker.UUIDHyphenated()
			orgID := common.DefaultOrgID
			images := []models.Image{
				{Name: name, OrgID: orgID},
				{Name: name, OrgID: orgID},
			}
			res := db.DB.Create(&images)

			It("should return the requested images", func() {
				Expect(res.Error).ToNot(HaveOccurred())
				req, err := http.NewRequest("GET", fmt.Sprintf("/images?name=%s", name), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).ToNot(BeEmpty())

				type ResponseData struct {
					Count  int64          `json:"count"`
					Images []models.Image `json:"data"`
				}

				var responseData ResponseData
				err = json.Unmarshal(respBody, &responseData)
				Expect(err).ToNot(HaveOccurred())
				Expect(responseData.Count).To(Equal(int64(len(images))))
				Expect(len(responseData.Images)).To(Equal(len(images)))
				for _, image := range responseData.Images {
					Expect(image.Name).To(Equal(name))
					Expect(image.OrgID).To(Equal(orgID))
				}
			})

			When("org undefined", func() {
				var originalAuth bool
				conf := config.Get()

				BeforeEach(func() {
					// save original config values
					originalAuth = conf.Auth
					// set auth to True to force use identity
					conf.Auth = true
					router = chi.NewRouter()
					router.Use(func(next http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							// set identity orgID as empty
							ctx := context.WithValue(r.Context(), identity.Key, identity.XRHID{Identity: identity.Identity{OrgID: ""}})
							ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
							next.ServeHTTP(w, r.WithContext(ctx))
						})
					})
					router.Route("/images", MakeImagesRouter)
				})

				AfterEach(func() {
					// restore config values
					conf.Auth = originalAuth
					ctrl.Finish()
				})

				It("should return error when org undefined", func() {
					Expect(res.Error).ToNot(HaveOccurred())
					req, err := http.NewRequest("GET", fmt.Sprintf("/images?name=%s", name), nil)
					Expect(err).ToNot(HaveOccurred())

					httpTestRecorder := httptest.NewRecorder()
					router.ServeHTTP(httpTestRecorder, req)

					Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
					respBody, err := io.ReadAll(httpTestRecorder.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(respBody)).To(ContainSubstring("cannot find org-id"))
				})
			})
		})

		Context("GetImageByID", func() {
			image := models.Image{Name: faker.Name(), OrgID: common.DefaultOrgID}
			res := db.DB.Create(&image)

			It("Should return the image data successfully", func() {
				Expect(res.Error).ToNot(HaveOccurred())
				req, err := http.NewRequest("GET", fmt.Sprintf("/images/%d", image.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).ToNot(BeEmpty())

				var resImage models.Image
				err = json.Unmarshal(respBody, &resImage)
				Expect(err).ToNot(HaveOccurred())
				Expect(resImage.ID).To(Equal(image.ID))
				Expect(resImage.Name).To(Equal(image.Name))
			})

			It("should return error when image does not exist", func() {
				imageID := "99999999999"
				req, err := http.NewRequest("GET", fmt.Sprintf("/images/%s", imageID), nil)
				Expect(err).ToNot(HaveOccurred())

				expectedError := new(services.ImageNotFoundError)
				mockImagesService.EXPECT().GetImageByID(imageID).Return(nil, expectedError)
				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring(expectedError.Error()))
			})

			It("should return error when image id is not passed", func() {
				req, err := http.NewRequest("GET", "/images//", nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("Image ID required"))
			})

			It("should return error when org id is undefined", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/images/%d", image.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				expectedError := new(services.OrgIDNotSet)
				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(nil, expectedError)
				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring(expectedError.Error()))
			})

			It("Should return error when image id is not integer", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/images/%d", image.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				expectedError := new(services.IDMustBeInteger)
				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(nil, expectedError)
				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring(expectedError.Error()))
			})

			It("should return error when error occurred when getting image", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/images/%d", image.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				expectedError := errors.New("an unknown error occurred")
				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(nil, expectedError)
				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusInternalServerError))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("Something went wrong."))
			})
			It("should return error when image org and session org mismatch", func() {
				image := models.Image{Name: faker.Name(), OrgID: faker.UUIDHyphenated()}
				res := db.DB.Create(&image)
				Expect(res.Error).ToNot(HaveOccurred())
				req, err := http.NewRequest("GET", fmt.Sprintf("/images/%d", image.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("image and session org mismatch"))
			})

			When("org undefined", func() {
				var originalAuth bool
				conf := config.Get()

				BeforeEach(func() {
					// save original config values
					originalAuth = conf.Auth
					// set auth to True to force use identity
					conf.Auth = true
					router = chi.NewRouter()
					router.Use(func(next http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							// set identity orgID as empty
							ctx := context.WithValue(r.Context(), identity.Key, identity.XRHID{Identity: identity.Identity{OrgID: ""}})
							ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
							next.ServeHTTP(w, r.WithContext(ctx))
						})
					})
					router.Route("/images", MakeImagesRouter)
				})

				AfterEach(func() {
					// restore config values
					conf.Auth = originalAuth
					ctrl.Finish()
				})

				It("should return error when org undefined", func() {
					Expect(res.Error).ToNot(HaveOccurred())
					req, err := http.NewRequest("GET", fmt.Sprintf("/images/%d", image.ID), nil)
					Expect(err).ToNot(HaveOccurred())

					mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
					httpTestRecorder := httptest.NewRecorder()
					router.ServeHTTP(httpTestRecorder, req)

					Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
					respBody, err := io.ReadAll(httpTestRecorder.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(respBody)).To(ContainSubstring("image doesn't belong to org_id"))
				})
			})
		})

		Context("CreateImageUpdate", func() {
			imageName := faker.UUIDHyphenated()
			image := models.Image{Name: imageName, OrgID: common.DefaultOrgID}
			res := db.DB.Create(&image)
			packageName := "vim"
			updateImage := models.Image{
				Name:         imageName,
				Distribution: "rhel-85",
				OutputTypes:  []string{models.ImageTypeCommit},
				Commit: &models.Commit{
					Arch: "x86_64",
					InstalledPackages: []models.InstalledPackage{
						{Name: packageName},
					},
				},
			}

			It("should update image successfully", func() {
				Expect(res.Error).ToNot(HaveOccurred())

				var buf bytes.Buffer
				err := json.NewEncoder(&buf).Encode(&updateImage)
				Expect(err).ToNot(HaveOccurred())

				req, err := http.NewRequest("POST", fmt.Sprintf("/images/%d/update", image.ID), &buf)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				// we cannot predict the instance value of first argument, we know that it's updateImage
				// but as it's un-marshaled it's using an other pointer, most important assertions are those at the end of the test
				mockImagesService.EXPECT().UpdateImage(gomock.Any(), &image).Return(nil)
				// same here for context and updateImage
				mockImagesService.EXPECT().ProcessImage(gomock.Any(), gomock.Any())
				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).ToNot(BeEmpty())

				var resImage models.Image
				err = json.Unmarshal(respBody, &resImage)
				Expect(err).ToNot(HaveOccurred())
				Expect(resImage.Name).To(Equal(updateImage.Name))
				Expect(resImage.Distribution).To(Equal(updateImage.Distribution))
				Expect(len(resImage.Commit.InstalledPackages) > 0).To(BeTrue())
				Expect(resImage.Commit.InstalledPackages[0].Name).To(Equal(packageName))
			})

			It("should return error when update image fail with PackageNameDoesNotExist", func() {
				var buf bytes.Buffer
				err := json.NewEncoder(&buf).Encode(&updateImage)
				Expect(err).ToNot(HaveOccurred())

				req, err := http.NewRequest("POST", fmt.Sprintf("/images/%d/update", image.ID), &buf)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				// we cannot predict the instance value of first argument, we know that it's updateImage
				// but as it's un-marshaled it's using an other pointer, most important assertions are those at the end of the test
				expectedErr := new(services.PackageNameDoesNotExist)
				mockImagesService.EXPECT().UpdateImage(gomock.Any(), &image).Return(expectedErr)
				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring(expectedErr.Error()))
			})

			It("should return error when update image fail with unknown error", func() {
				var buf bytes.Buffer
				err := json.NewEncoder(&buf).Encode(&updateImage)
				Expect(err).ToNot(HaveOccurred())

				req, err := http.NewRequest("POST", fmt.Sprintf("/images/%d/update", image.ID), &buf)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				// we cannot predict the instance value of first argument, we know that it's updateImage
				// but as it's un-marshaled it's using an other pointer, most important assertions are those at the end of the test
				expectedErr := errors.New("unknown error occurred")
				mockImagesService.EXPECT().UpdateImage(gomock.Any(), &image).Return(expectedErr)
				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusInternalServerError))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("Failed creating image"))
			})

			It("should accept empty update image name", func() {
				updateImage := models.Image{
					Name:         "",
					Distribution: "rhel-85",
					OutputTypes:  []string{models.ImageTypeCommit},
					Commit: &models.Commit{Arch: "x86_64",
						InstalledPackages: []models.InstalledPackage{
							{Name: packageName},
						},
					},
				}
				Expect(updateImage.Name).To(BeEmpty())
				var buf bytes.Buffer
				err := json.NewEncoder(&buf).Encode(&updateImage)
				Expect(err).ToNot(HaveOccurred())

				req, err := http.NewRequest("POST", fmt.Sprintf("/images/%d/update", image.ID), &buf)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				// we cannot predict the instance value of first argument, we know that it's updateImage
				// but as it's un-marshaled it's using another pointer.
				mockImagesService.EXPECT().UpdateImage(gomock.AssignableToTypeOf(&updateImage), &image).Return(nil)
				// same here for context and updateImage
				mockImagesService.EXPECT().ProcessImage(gomock.Any(), gomock.AssignableToTypeOf(&updateImage))

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).ToNot(BeEmpty())

				var resUpdateImage models.Image
				err = json.Unmarshal(respBody, &resUpdateImage)
				Expect(err).ToNot(HaveOccurred())
				// the updateImage Name should equal the previousImage name
				Expect(resUpdateImage.Name).To(Equal(image.Name))
			})

			It("should return error when update image name is not empty and is different from previous image name", func() {
				// the image service has a unittest that check and return ImageNameChangeIsProhibited error when trying
				// to change the image name in this unittest we check that the user receive the appropriate validation error message
				updateImage := models.Image{
					Name:         faker.UUIDHyphenated(),
					Distribution: "rhel-85",
					OutputTypes:  []string{models.ImageTypeCommit},
					Commit: &models.Commit{Arch: "x86_64",
						InstalledPackages: []models.InstalledPackage{
							{Name: packageName},
						},
					},
				}
				Expect(updateImage.Name).ToNot(BeEmpty())
				Expect(updateImage.Name).ToNot(Equal(image.Name))
				var buf bytes.Buffer
				err := json.NewEncoder(&buf).Encode(&updateImage)
				Expect(err).ToNot(HaveOccurred())

				req, err := http.NewRequest("POST", fmt.Sprintf("/images/%d/update", image.ID), &buf)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				// we cannot predict the instance value of first argument, we know that it's updateImage
				// but as it's un-marshaled it's using another pointer.
				mockImagesService.EXPECT().UpdateImage(gomock.AssignableToTypeOf(&updateImage), &image).Return(new(services.ImageNameChangeIsProhibited))

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring(services.ImageNameChangeIsProhibitedMsg))
			})
		})
		Context("GetImageByOSTree", func() {
			imageName := faker.UUIDHyphenated()
			ostreeHash := faker.UUIDHyphenated()
			orgID := common.DefaultOrgID
			image := models.Image{
				Name: imageName, OrgID: orgID,
				Commit: &models.Commit{OSTreeCommit: ostreeHash, OrgID: orgID},
			}
			res := db.DB.Create(&image)

			It("should return the image data successfully", func() {
				Expect(res.Error).ToNot(HaveOccurred())
				req, err := http.NewRequest("GET", fmt.Sprintf("/images/%s/info", ostreeHash), nil)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByOSTreeCommitHash(ostreeHash).Return(&image, nil)
				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).ToNot(BeEmpty())

				var resImage models.Image
				err = json.Unmarshal(respBody, &resImage)
				Expect(err).ToNot(HaveOccurred())
				Expect(resImage.ID).To(Equal(image.ID))
				Expect(resImage.Name).To(Equal(image.Name))
			})

			It("should return error when image does not exist", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/images/%s/info", ostreeHash), nil)
				Expect(err).ToNot(HaveOccurred())

				expectedError := new(services.ImageNotFoundError)
				mockImagesService.EXPECT().GetImageByOSTreeCommitHash(ostreeHash).Return(nil, expectedError)
				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring(expectedError.Error()))
			})

			It("should return error when orgID not set", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/images/%s/info", ostreeHash), nil)
				Expect(err).ToNot(HaveOccurred())

				expectedError := new(services.OrgIDNotSet)
				mockImagesService.EXPECT().GetImageByOSTreeCommitHash(ostreeHash).Return(nil, expectedError)
				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring(expectedError.Error()))
			})

			It("should return error when unknown error", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/images/%s/info", ostreeHash), nil)
				Expect(err).ToNot(HaveOccurred())

				expectedError := errors.New("unknown error")
				mockImagesService.EXPECT().GetImageByOSTreeCommitHash(ostreeHash).Return(nil, expectedError)
				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusInternalServerError))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("Something went wrong."))
			})

			It("should return error when ostreehash not supplied", func() {
				req, err := http.NewRequest("GET", "/images//info", nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("OSTreeCommitHash required"))
			})
		})

		Context("CreateInstallerForImage", func() {
			imageName := faker.UUIDHyphenated()
			image := models.Image{Name: imageName, OrgID: common.DefaultOrgID}
			res := db.DB.Create(&image)
			installer := models.Installer{Username: "root", SSHKey: "ssh-rsa d9:f158:00:abcd"}

			It("should create installer successfully", func() {
				Expect(res.Error).ToNot(HaveOccurred())
				var buf bytes.Buffer
				err := json.NewEncoder(&buf).Encode(&installer)
				Expect(err).ToNot(HaveOccurred())

				req, err := http.NewRequest("POST", fmt.Sprintf("/images/%d/installer", image.ID), &buf)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				mockImagesService.EXPECT().CreateInstallerForImage(gomock.Any(), &image).Return(&image, nil, nil)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).ToNot(BeEmpty())

				var resImage models.Image
				err = json.Unmarshal(respBody, &resImage)
				Expect(err).ToNot(HaveOccurred())
				Expect(resImage.ID).To(Equal(image.ID))
				Expect(resImage.Name).To(Equal(image.Name))
			})

			It("should return error when error occurred", func() {
				var buf bytes.Buffer
				err := json.NewEncoder(&buf).Encode(&installer)
				Expect(err).ToNot(HaveOccurred())

				req, err := http.NewRequest("POST", fmt.Sprintf("/images/%d/installer", image.ID), &buf)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				expectedError := errors.New("unknown error")
				mockImagesService.EXPECT().CreateInstallerForImage(gomock.Any(), &image).Return(nil, nil, expectedError)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusInternalServerError))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("Failed to create installer"))
			})
		})
		Context("RetryCreateImage", func() {
			imageName := faker.UUIDHyphenated()
			image := models.Image{Name: imageName, OrgID: common.DefaultOrgID}
			res := db.DB.Create(&image)

			It("should recreate image  successfully", func() {
				Expect(res.Error).ToNot(HaveOccurred())

				req, err := http.NewRequest("POST", fmt.Sprintf("/images/%d/retry", image.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				mockImagesService.EXPECT().RetryCreateImage(gomock.Any(), &image).Return(nil)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusCreated))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).ToNot(BeEmpty())

				var resImage models.Image
				err = json.Unmarshal(respBody, &resImage)
				Expect(err).ToNot(HaveOccurred())
				Expect(resImage.ID).To(Equal(image.ID))
				Expect(resImage.Name).To(Equal(image.Name))
			})

			It("should return error when error occurred", func() {
				req, err := http.NewRequest("POST", fmt.Sprintf("/images/%d/retry", image.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				expectedError := errors.New("unknown error")
				mockImagesService.EXPECT().RetryCreateImage(gomock.Any(), &image).Return(expectedError)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusInternalServerError))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("Failed creating image"))
			})
		})

		Context("CreateKickStartForImage", func() {
			imageName := faker.UUIDHyphenated()
			image := models.Image{Name: imageName, OrgID: common.DefaultOrgID}
			res := db.DB.Create(&image)

			It("should create kickstart for image successfully", func() {
				Expect(res.Error).ToNot(HaveOccurred())

				req, err := http.NewRequest("POST", fmt.Sprintf("/images/%d/kickstart", image.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				mockImagesService.EXPECT().AddUserInfo(&image).Return(nil)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(BeEmpty())
			})

			It("should return error when error occurred", func() {
				req, err := http.NewRequest("POST", fmt.Sprintf("/images/%d/kickstart", image.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				expectedError := errors.New("unknown error")
				mockImagesService.EXPECT().AddUserInfo(&image).Return(expectedError)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusInternalServerError))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("Something went wrong."))
			})
		})

		Context("SendNotificationForImage", func() {
			imageName := faker.UUIDHyphenated()
			image := models.Image{Name: imageName, OrgID: common.DefaultOrgID}
			res := db.DB.Create(&image)

			notification := services.ImageNotification{
				Version:     services.NotificationConfigVersion,
				Bundle:      services.NotificationConfigBundle,
				Application: services.NotificationConfigApplication,
				EventType:   services.NotificationConfigEventTypeImage,
				Timestamp:   time.Now().Format(time.RFC3339),
			}

			It("should send notification successfully", func() {
				Expect(res.Error).ToNot(HaveOccurred())

				req, err := http.NewRequest("GET", fmt.Sprintf("/images/%d/notify", image.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				mockImagesService.EXPECT().GetImageByID(strconv.Itoa(int(image.ID))).Return(&image, nil)
				mockImagesService.EXPECT().SendImageNotification(&image).Return(notification, nil)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).ToNot(BeEmpty())

				var resNotification services.ImageNotification
				err = json.Unmarshal(respBody, &resNotification)
				Expect(err).ToNot(HaveOccurred())
				Expect(resNotification.Version).To(Equal(notification.Version))
				Expect(resNotification.EventType).To(Equal(notification.EventType))
			})
		})
	})
})
