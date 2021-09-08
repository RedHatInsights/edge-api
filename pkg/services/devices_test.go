package services

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory/mock_inventory"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
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
func TestGetUpdateAvailableForDeviceByUUID(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uuid := faker.UUIDHyphenated()
	checksum := "fake-checksum"
	resp := inventory.InventoryResponse{Total: 1, Count: 1, Result: []inventory.Devices{
		{ID: uuid, Ostree: inventory.SystemProfile{
			RHCClientID: faker.UUIDHyphenated(),
			RpmOstreeDeployments: []inventory.OSTree{
				{Checksum: checksum, Booted: true},
			},
		}},
	}}
	mockInventoryClient := mock_inventory.NewMockClientInterface(ctrl)
	mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

	deviceService := DeviceService{
		ctx:       context.Background(),
		inventory: mockInventoryClient,
	}

	oldImage := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: checksum,
		},
		Status: models.ImageStatusSuccess,
	}
	db.DB.Create(oldImage.Commit)
	db.DB.Create(oldImage)
	newImage := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: fmt.Sprintf("a-new-%s", checksum),
		},
		Status:   models.ImageStatusSuccess,
		ParentId: &oldImage.CommitID,
	}
	db.DB.Create(newImage.Commit)
	db.DB.Create(newImage)
	updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid)
	if err != nil {
		t.Errorf("Expected nil err, got %#v", err)
	}
	if len(updatesAvailable) != 1 {
		t.Errorf("Expected one update available, got %d", len(updatesAvailable))
	}
	newUpdate := updatesAvailable[0]
	if newUpdate.Image.ID != newImage.ID {
		t.Errorf("Expected new image to be %d, got %d", newImage.ID, newUpdate.Image.ID)

	}
}

func TestGetUpdateAvailableForDeviceByUUIDWhenNoUpdateIsAvailable(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uuid := faker.UUIDHyphenated()
	checksum := "fake-checksum-2"
	resp := inventory.InventoryResponse{Total: 1, Count: 1, Result: []inventory.Devices{
		{ID: uuid, Ostree: inventory.SystemProfile{
			RHCClientID: faker.UUIDHyphenated(),
			RpmOstreeDeployments: []inventory.OSTree{
				{Checksum: checksum, Booted: true},
			},
		}},
	}}
	mockInventoryClient := mock_inventory.NewMockClientInterface(ctrl)
	mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

	deviceService := DeviceService{
		ctx:       context.Background(),
		inventory: mockInventoryClient,
	}

	oldImage := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: checksum,
		},
		Status: models.ImageStatusSuccess,
	}
	db.DB.Create(oldImage)

	updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid)
	if err != nil {
		t.Errorf("Expected nil err, got %#v", err)
	}
	if len(updatesAvailable) != 0 {
		t.Errorf("Expected zero updates available, got %d", len(updatesAvailable))
	}
}

func TestGetUpdateAvailableForDeviceByUUIDWhenNoChecksumIsFound(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uuid := faker.UUIDHyphenated()
	checksum := "fake-checksum-3"
	resp := inventory.InventoryResponse{Total: 1, Count: 1, Result: []inventory.Devices{
		{ID: uuid, Ostree: inventory.SystemProfile{
			RHCClientID: faker.UUIDHyphenated(),
			RpmOstreeDeployments: []inventory.OSTree{
				{Checksum: checksum, Booted: true},
			},
		}},
	}}
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

func TestGetDiffOnUpdate(t *testing.T) {

	oldImage := models.Image{
		Commit: &models.Commit{
			Packages: []models.Package{
				{
					Name: "vim",
				},
				{
					Name: "ansible",
				},
			}},
	}
	newImage := models.Image{
		Commit: &models.Commit{
			Packages: []models.Package{
				{
					Name: "zsh",
				},
				{
					Name: "yum",
				},
			},
		},
	}
	deltaDiff := getDiffOnUpdate(oldImage, newImage)

	if len(deltaDiff.Added) != 2 {
		t.Errorf("Expected one update available, got %d", len(deltaDiff.Added))
	}
	if len(deltaDiff.Removed) != 2 {
		t.Errorf("Expected one update available, got %d", len(deltaDiff.Removed))
	}
}
