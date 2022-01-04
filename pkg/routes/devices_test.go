package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	faker "github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
)

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
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
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
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
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
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
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
