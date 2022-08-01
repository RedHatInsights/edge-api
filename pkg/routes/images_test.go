package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redhatinsights/edge-api/pkg/services"

	"github.com/golang/mock/gomock"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
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
	respBody, err := ioutil.ReadAll(rr.Body)
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
	respBody, err := ioutil.ReadAll(rr.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	err = json.Unmarshal(respBody, &ir)
	if err != nil {
		t.Errorf(err.Error())
	}

	if ir.Packages != 0 {
		t.Errorf("wrong image packages: got %v want %v",
			ir.Packages, 0)
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
	respBody, err := ioutil.ReadAll(rr.Body)
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
	respBody, err := ioutil.ReadAll(rr.Body)
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
	respBody, err := ioutil.ReadAll(rr.Body)
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
