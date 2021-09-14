package services

import (
	"context"
	"fmt"

	version "github.com/knqyf263/go-rpm-version"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// DeviceServiceInterface defines the interface to handle the business logic of RHEL for Edge Devices
type DeviceServiceInterface interface {
	GetDeviceByID(deviceID uint) (*models.Device, error)
	GetDeviceByUUID(deviceUUID string) (*models.Device, error)
	GetUpdateAvailableForDeviceByUUID(deviceUUID string) ([]ImageUpdateAvailable, error)
	GetDeviceImageInfo(deviceUUID string) (ImageInfo, error)
}

// NewDeviceService gives a instance of the main implementation of DeviceServiceInterface
func NewDeviceService(ctx context.Context) DeviceServiceInterface {
	return &DeviceService{
		ctx:       ctx,
		inventory: inventory.InitClient(ctx),
	}
}

// DeviceService is the main implementation of a DeviceServiceInterface
type DeviceService struct {
	ctx       context.Context
	inventory inventory.ClientInterface
}

// GetDeviceByID receives DeviceID uint and get a *models.Device back
func (s *DeviceService) GetDeviceByID(deviceID uint) (*models.Device, error) {
	log.Debugf("GetDeviceByID::deviceID: %#v", deviceID)
	var device models.Device
	result := db.DB.First(&device, deviceID)
	log.Debugf("GetDeviceByID::result: %#v", result)
	log.Debugf("GetDeviceByID::device: %#v", device)
	if result.Error != nil {
		return nil, result.Error
	}
	return &device, nil
}

// GetDeviceByUUID receives UUID string and get a *models.Device back
func (s *DeviceService) GetDeviceByUUID(deviceUUID string) (*models.Device, error) {
	log.Debugf("GetDeviceByUUID::deviceUUID: %#v", deviceUUID)
	var device models.Device
	result := db.DB.Where("uuid = ?", deviceUUID).First(&device)
	log.Debugf("GetDeviceByUUID::result: %#v", result)
	log.Debugf("GetDeviceByUUID::device: %#v", device)
	if result.Error != nil {
		return nil, result.Error
	}
	return &device, nil
}

type ImageUpdateAvailable struct {
	Image       models.Image
	PackageDiff DeltaDiff
}
type DeltaDiff struct {
	Added    []models.InstalledPackage
	Removed  []models.InstalledPackage
	Upgraded []models.InstalledPackage
}

type ImageInfo struct {
	Image           models.Image
	UpdateAvailable []ImageUpdateAvailable
	Rollback        models.Image
}

type DeviceNotFoundError struct {
	error
}

type UpdateNotFoundError struct {
	error
}

type ImageNotFoundError struct {
	error
}

// GetUpdateAvailableForDeviceByUUID returns if exists update for the current image at the device.
func (s *DeviceService) GetUpdateAvailableForDeviceByUUID(deviceUUID string) ([]ImageUpdateAvailable, error) {

	device, err := s.inventory.ReturnDevicesByID(deviceUUID)
	if err != nil || device.Total != 1 {
		return nil, new(DeviceNotFoundError)
	}

	lastDevice := device.Result[len(device.Result)-1]
	lastDeploymentIdx := len(lastDevice.Ostree.RpmOstreeDeployments) - 1
	// TODO: Only consider applied update (check if booted = true)
	lastDeployment := lastDevice.Ostree.RpmOstreeDeployments[lastDeploymentIdx]

	var images []models.Image
	var currentImage models.Image
	result := db.DB.Model(&models.Image{}).Joins("Commit").Where("OS_Tree_Commit = ?", lastDeployment.Checksum).First(&currentImage)
	if result.Error != nil || result.RowsAffected == 0 {
		log.Error(result.Error)
		return nil, new(DeviceNotFoundError)
	}

	err = db.DB.Model(&currentImage.Commit).Association("Packages").Find(&currentImage.Commit.Packages)
	if err != nil {
		log.Error(result.Error)
		return nil, new(DeviceNotFoundError)
	}

	updates := db.DB.Where("Parent_Id = ? and Images.Status = ?", currentImage.ID, models.ImageStatusSuccess).Joins("Commit").Find(&images)
	if updates.Error != nil || updates.RowsAffected == 0 {
		return nil, new(UpdateNotFoundError)
	}

	var imageDiff []ImageUpdateAvailable
	for _, upd := range images {
		db.DB.First(&upd.Commit, upd.CommitID)
		db.DB.Model(&upd.Commit).Association("Packages").Find(&upd.Commit.Packages)
		var delta ImageUpdateAvailable
		diff := getDiffOnUpdate(currentImage, upd)
		delta.Image = upd
		delta.PackageDiff = diff
		imageDiff = append(imageDiff, delta)
	}
	return imageDiff, nil
}

func getPackageDiff(a, b []models.InstalledPackage) []models.InstalledPackage {
	var diff []models.InstalledPackage
	pkgs := make(map[string]models.InstalledPackage)
	for _, pkg := range b {
		pkgs[pkg.Name] = pkg
	}
	for _, pkg := range a {
		if _, ok := pkgs[pkg.Name]; !ok {
			diff = append(diff, pkg)
		}
	}
	return diff
}

func getVersionDiff(new, old []models.InstalledPackage) []models.InstalledPackage {
	var diff []models.InstalledPackage
	oldPkgs := make(map[string]models.InstalledPackage)
	for _, pkg := range old {
		oldPkgs[pkg.Name] = pkg
	}
	for _, pkg := range new {
		if oldPkg, ok := oldPkgs[pkg.Name]; ok {
			oldPkgVersion := version.NewVersion(oldPkg.Version)
			newPkgVersion := version.NewVersion(pkg.Version)
			if newPkgVersion.GreaterThan(oldPkgVersion) {
				diff = append(diff, pkg)
			}
		}
	}
	return diff
}

func getDiffOnUpdate(oldImg models.Image, newImg models.Image) DeltaDiff {
	results := DeltaDiff{
		Added:    getPackageDiff(newImg.Commit.InstalledPackages, oldImg.Commit.InstalledPackages),
		Removed:  getPackageDiff(oldImg.Commit.InstalledPackages, newImg.Commit.InstalledPackages),
		Upgraded: getVersionDiff(newImg.Commit.InstalledPackages, oldImg.Commit.InstalledPackages),
	}
	return results
}

func (s *DeviceService) GetDeviceImageInfo(deviceUUID string) (ImageInfo, error) {
	fmt.Printf(":: GetDeviceImageInfo :: \n")
	var ImageInfo ImageInfo
	var currentImage models.Image
	var rollback models.Image

	device, err := s.inventory.ReturnDevicesByID(deviceUUID)
	if err != nil || device.Total != 1 {
		return ImageInfo, new(DeviceNotFoundError)
	}

	lastDevice := device.Result[len(device.Result)-1]
	lastDeployment := lastDevice.Ostree.RpmOstreeDeployments[len(lastDevice.Ostree.RpmOstreeDeployments)-1]

	result := db.DB.Model(&models.Image{}).Joins("Commit").Where("OS_Tree_Commit = ?", lastDeployment.Checksum).First(&currentImage)

	if result.Error != nil || result == nil {
		log.Error(result.Error)
		return ImageInfo, new(ImageNotFoundError)
	} else {
		if currentImage.ParentId != nil {
			db.DB.Where("ID = ?", currentImage.ParentId).First(&rollback)
		}
	}
	updateAvailable, err := s.GetUpdateAvailableForDeviceByUUID(deviceUUID)
	if err != nil {
		fmt.Printf("err:: %v \n", err)
	} else {
		ImageInfo.UpdateAvailable = updateAvailable
	}
	ImageInfo.Rollback = rollback
	ImageInfo.Image = currentImage

	return ImageInfo, nil
}
