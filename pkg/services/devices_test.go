package services

import (
	"context"
	"errors"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory/mock_inventory"
)

func TestGetUpdateAvailableForDeviceByUUIDWhenErrorOnInventoryAPI(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uuid := faker.UUIDHyphenated()
	mockInventoryClient := mock_inventory.NewMockClientInterface(ctrl)
	mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(inventory.InventoryResponse{}, errors.New("error on inventory api"))

	deviceService := DeviceService{
		ctx:       context.Background(),
		inventory: mockInventoryClient,
	}

	updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid)
	if updatesAvailable != nil {
		t.Errorf("Expected nil updates available, got %#v", updatesAvailable)
	}

	if _, ok := err.(*DeviceNotFoundError); !ok {
		t.Errorf("Expected DeviceNotFoundError, got %#v", err)
	}
}
func TestGetUpdateAvailableForDeviceByUUIDWhenDeviceIsNotFoundOnInventoryAPI(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uuid := faker.UUIDHyphenated()
	resp := inventory.InventoryResponse{}
	mockInventoryClient := mock_inventory.NewMockClientInterface(ctrl)
	mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

	deviceService := DeviceService{
		ctx:       context.Background(),
		inventory: mockInventoryClient,
	}

	updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid)
	if updatesAvailable != nil {
		t.Errorf("Expected nil updates available, got %#v", updatesAvailable)
	}

	if _, ok := err.(*DeviceNotFoundError); !ok {
		t.Errorf("Expected DeviceNotFoundError, got %#v", err)
	}
}
