package routes

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	faker "github.com/bxcodec/faker/v3"
	"github.com/go-chi/chi"
	"github.com/golang/mock/gomock"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
)

func TestGetDevicesStatus(t *testing.T) {
	tt := []struct {
		name         string
		searchUUID   string
		expectedHash string
	}{
		{
			name:         "display devices for uuid under account (0000000)",
			searchUUID:   "1",
			expectedHash: "11",
		},
		{
			name:         "no devices for uuid not under account (0000000)",
			searchUUID:   "3",
			expectedHash: "",
		},
	}

	for _, te := range tt {
		req, err := http.NewRequest("GET", "/", nil)
		if err != nil {
			t.Errorf("%s: Failed creating a new request: %s", te.name, err)
			return
		}
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
			URLParams: chi.RouteParams{
				Keys:   []string{"DeviceUUID"},
				Values: []string{te.searchUUID},
			},
		})
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(GetDeviceStatus)
		handler.ServeHTTP(rr, req.WithContext(ctx))

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("%s: handler returned wrong status code: got %v want %v", te.name, status, http.StatusOK)
			return
		}
		var dvcs []models.Device
		respBody, err := ioutil.ReadAll(rr.Body)
		if err != nil {
			t.Errorf("%s: Failed reading response body: %s", te.name, err.Error())
			return
		}

		err = json.Unmarshal(respBody, &dvcs)
		if err != nil {
			t.Errorf("%s: Failed unmarshaling json from the response body: %s", te.name, err.Error())
			return
		}

		if te.expectedHash == "" && len(dvcs) > 0 {
			t.Errorf("%s was expecting not to have any results but got %+v", te.name, dvcs)
			return
		}
		for _, dvc := range dvcs {
			if dvc.UUID != te.searchUUID {
				t.Errorf("%s was expecting UUID to be %s but got %s", te.name, te.searchUUID, dvc.UUID)
			}
			if dvc.DesiredHash != te.expectedHash {
				t.Errorf("%s was expecting hash to be %s but got %s", te.name, te.expectedHash, dvc.DesiredHash)
			}
		}
	}
}

func TestGetAvailableUpdateForDevice(t *testing.T) {
	// Given
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	dc := DeviceContext{
		DeviceUUID: faker.UUIDHyphenated(),
	}
	ctx := context.WithValue(req.Context(), DeviceContextKey, dc)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expected := []services.ImageUpdateAvailable{
		{Image: testImage, PackageDiff: services.DeltaDiff{}},
	}
	mockDeviceService := mock_services.NewMockDeviceServiceInterface(ctrl)
	mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(dc.DeviceUUID)).Return(expected, nil)
	ctx = context.WithValue(ctx, dependencies.Key, &dependencies.EdgeAPIServices{
		DeviceService: mockDeviceService,
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetUpdateAvailableForDevice)

	// When
	handler.ServeHTTP(rr, req.WithContext(ctx))

	// Then
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return
	}
	respBody, err := ioutil.ReadAll(rr.Body)
	if err != nil {
		t.Errorf(err.Error())
	}
	var actual []services.ImageUpdateAvailable
	err = json.Unmarshal(respBody, &actual)
	if err != nil {
		t.Errorf(err.Error())
	}

	if len(actual) != 1 {
		t.Errorf("wrong response: length is %d",
			len(actual))
	}
	if actual[0].Image.ID != expected[0].Image.ID {
		t.Errorf("wrong response: got %v want %v",
			actual[0], expected[0])
	}

}

func TestGetAvailableUpdateForDeviceWithEmptyUUID(t *testing.T) {
	// Given
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	dc := DeviceContext{
		DeviceUUID: "",
	}
	ctx := context.WithValue(req.Context(), DeviceContextKey, dc)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDeviceService := mock_services.NewMockDeviceServiceInterface(ctrl)
	ctx = context.WithValue(ctx, dependencies.Key, &dependencies.EdgeAPIServices{
		DeviceService: mockDeviceService,
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := DeviceCtx(http.HandlerFunc(GetUpdateAvailableForDevice))

	// When
	handler.ServeHTTP(rr, req.WithContext(ctx))

	// Then
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
		return
	}
}
func TestGetAvailableUpdateForDeviceWhenDeviceIsNotFound(t *testing.T) {
	// Given
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	dc := DeviceContext{
		DeviceUUID: faker.UUIDHyphenated(),
	}
	ctx := context.WithValue(req.Context(), DeviceContextKey, dc)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDeviceService := mock_services.NewMockDeviceServiceInterface(ctrl)
	mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(dc.DeviceUUID)).Return(nil, new(services.DeviceNotFoundError))
	ctx = context.WithValue(ctx, dependencies.Key, &dependencies.EdgeAPIServices{
		DeviceService: mockDeviceService,
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetUpdateAvailableForDevice)

	// When
	handler.ServeHTTP(rr, req.WithContext(ctx))

	// Then
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
		return
	}
}
func TestGetAvailableUpdateForDeviceWhenAUnexpectedErrorHappens(t *testing.T) {
	// Given
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	dc := DeviceContext{
		DeviceUUID: faker.UUIDHyphenated(),
	}
	ctx := context.WithValue(req.Context(), DeviceContextKey, dc)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDeviceService := mock_services.NewMockDeviceServiceInterface(ctrl)
	mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(dc.DeviceUUID)).Return(nil, errors.New("random error"))
	ctx = context.WithValue(ctx, dependencies.Key, &dependencies.EdgeAPIServices{
		DeviceService: mockDeviceService,
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetUpdateAvailableForDevice)

	// When
	handler.ServeHTTP(rr, req.WithContext(ctx))

	// Then
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
		return
	}
}
