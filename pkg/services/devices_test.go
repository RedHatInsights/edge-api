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
	mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(inventory.Response{}, errors.New("error on inventory api"))

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
	resp := inventory.Response{}
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
	resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Devices{
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

	imageSet := &models.ImageSet{
		Name:    "test",
		Version: 1,
	}
	db.DB.Create(imageSet)
	oldImage := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: checksum,
			InstalledPackages: []models.InstalledPackage{
				{
					Name:    "ansible",
					Version: "1.0.0",
				},
				{
					Name:    "yum",
					Version: "2:6.0-1",
				},
			},
		},
		Status:     models.ImageStatusSuccess,
		ImageSetID: &imageSet.ID,
	}
	db.DB.Create(oldImage.Commit)
	db.DB.Create(oldImage)
	newImage := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: fmt.Sprintf("a-new-%s", checksum),
			InstalledPackages: []models.InstalledPackage{
				{
					Name:    "yum",
					Version: "3:6.0-1",
				},
				{
					Name:    "vim",
					Version: "2.0.0",
				},
			},
		},
		Status:     models.ImageStatusSuccess,
		ImageSetID: &imageSet.ID,
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
	if len(newUpdate.PackageDiff.Upgraded) != 1 {
		t.Errorf("Expected package diff upgraded len to be 1, got %d", len(newUpdate.PackageDiff.Upgraded))
	}
	if len(newUpdate.PackageDiff.Added) != 1 {
		t.Errorf("Expected package diff added len to be 1, got %d", len(newUpdate.PackageDiff.Added))
	}
	if len(newUpdate.PackageDiff.Removed) != 1 {
		t.Errorf("Expected package diff removed len to be 1, got %d", len(newUpdate.PackageDiff.Removed))
	}

}
func TestGetUpdateAvailableForDeviceByUUIDWhenNoUpdateIsAvailable(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uuid := faker.UUIDHyphenated()
	checksum := "fake-checksum-2"
	resp := inventory.Response{
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

	if err != nil {
		t.Errorf("Expected nil err, got %#v", err)
	}
}

func TestGetUpdateAvailableForDeviceByUUIDWhenNoChecksumIsFound(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uuid := faker.UUIDHyphenated()
	checksum := "fake-checksum-3"
	resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Devices{
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
			InstalledPackages: []models.InstalledPackage{
				{
					Name:    "vim",
					Version: "2.2",
				},
				{
					Name:    "ansible",
					Version: "1",
				},
				{
					Name:    "yum",
					Version: "2:6.0-1",
				},
				{
					Name:    "dnf",
					Version: "2:6.0-1",
				},
			},
		},
	}
	newImage := models.Image{
		Commit: &models.Commit{
			InstalledPackages: []models.InstalledPackage{
				{
					Name:    "zsh",
					Version: "1",
				},
				{
					Name:    "yum",
					Version: "2:6.0-2.el6",
				},
				{
					Name:    "dnf",
					Version: "2:6.0-1",
				},
			},
		},
	}
	deltaDiff := getDiffOnUpdate(oldImage, newImage)

	if len(deltaDiff.Added) != 1 {
		t.Errorf("Expected one package on the diff added,, got %d", len(deltaDiff.Added))
	}
	if len(deltaDiff.Removed) != 2 {
		t.Errorf("Expected two packages on the diff removed, got %d", len(deltaDiff.Removed))
	}
	if len(deltaDiff.Upgraded) != 1 {
		t.Errorf("Expected one package upgraded, got %d", len(deltaDiff.Removed))
	}
}

func TestGetImageForDeviceByUUID(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uuid := faker.UUIDHyphenated()
	checksum := "fake-checksum"
	resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Devices{
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
	imageSet := &models.ImageSet{
		Name:    "test",
		Version: 1,
	}
	db.DB.Create(imageSet)
	oldImage := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: checksum,
		},
		Status:     models.ImageStatusSuccess,
		ImageSetID: &imageSet.ID,
	}
	db.DB.Create(oldImage.Commit)
	db.DB.Create(oldImage)
	fmt.Printf("Old image was created with id %d\n", oldImage.ID)
	newImage := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: fmt.Sprintf("a-new-%s", checksum),
		},
		Status:     models.ImageStatusSuccess,
		ImageSetID: &imageSet.ID,
	}
	db.DB.Create(newImage.Commit)
	db.DB.Create(newImage)
	fmt.Printf("New image was created with id %d\n", newImage.ID)
	fmt.Printf("New image was created with id %d\n", *newImage.ImageSetID)
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
	resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Devices{
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
