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
	fmt.Printf("Old image was created with id %d\n", oldImage.ID)
	newImage := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: fmt.Sprintf("a-new-%s", checksum),
		},
		Status:   models.ImageStatusSuccess,
		ParentId: &oldImage.ID,
	}
	db.DB.Create(newImage.Commit)
	db.DB.Create(newImage)
	fmt.Printf("New image was created with id %d\n", newImage.ID)
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
	resp := inventory.InventoryResponse{
		Total: 1,
		Count: 1,
		Result: []inventory.Devices{
			{
				ID: uuid,
				Ostree: inventory.SystemProfile{
					RHCClientID: faker.UUIDHyphenated(),
					RpmOstreeDeployments: []inventory.OSTree{
						{
							Checksum: checksum,
							Booted:   true,
						},
					},
				}},
		},
	}
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
	if updatesAvailable != nil {
		t.Errorf("Expected nil updates available, got %#v", updatesAvailable)
	}

	if _, ok := err.(*UpdateNotFoundError); !ok {
		t.Errorf("Expected DeviceNotFoundError, got %#v", err)
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
			},
			InstalledPackages: []models.InstalledPackage{
				{
					Name: "vim",
				},
				{
					Name: "ansible",
				},
			},
		},
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
			InstalledPackages: []models.InstalledPackage{
				{
					Name: "zsh",
				},
			},
		},
	}
	deltaDiff := getDiffOnUpdate(oldImage, newImage)

	if len(deltaDiff.Added) != 1 {
		t.Errorf("Expected one packages on the diff, got %d", len(deltaDiff.Added))
	}
	if len(deltaDiff.Removed) != 2 {
		t.Errorf("Expected two packages on the diff, got %d", len(deltaDiff.Removed))
	}
}

func TestGetImageForDeviceByUUID(t *testing.T) {

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
	mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil).Times(2)

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
	fmt.Printf("Old image was created with id %d\n", oldImage.ID)
	newImage := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: fmt.Sprintf("a-new-%s", checksum),
		},
		Status:   models.ImageStatusSuccess,
		ParentId: &oldImage.ID,
	}
	db.DB.Create(newImage.Commit)
	db.DB.Create(newImage)
	fmt.Printf("New image was created with id %d\n", newImage.ID)
	fmt.Printf("New image was created with id %d\n", newImage.ParentId)
	imageInfo, err := deviceService.GetDeviceImageInfo(uuid)
	if err != nil {
		t.Errorf("Expected nil err, got %#v", err)
	}
	fmt.Printf("imageInfo:: %v \n", imageInfo.Image.ID)
	fmt.Printf("imageInfo:: %v \n", oldImage.ID)
	if oldImage.Commit.OSTreeCommit != imageInfo.Image.Commit.OSTreeCommit {
		t.Errorf("Expected image info to be %d, got %d", imageInfo.Image.ID, oldImage.ID)

	}
}

func TestGetNoImageForDeviceByUUID(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uuid := faker.UUIDHyphenated()
	checksum := "123"
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

	_, err := deviceService.GetDeviceImageInfo(uuid)
	if err == nil {
		t.Errorf("Expected ImageNotFoundError, got Nil")
	}

}
