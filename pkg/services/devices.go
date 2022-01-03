package services

import (
	"context"

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
	GetDeviceImageInfo(deviceUUID string) (*ImageInfo, error)
	GetDeviceDetails(deviceUUID string) (*DeviceDetails, error)
}

// NewDeviceService gives a instance of the main implementation of DeviceServiceInterface
func NewDeviceService(ctx context.Context, log *log.Entry) DeviceServiceInterface {
	return &DeviceService{
		updateService: NewUpdateService(ctx),
		inventory:     inventory.InitClient(ctx),
		Service:       Service{ctx: ctx, log: log.WithField("service", "image")},
	}
}

// DeviceService is the main implementation of a DeviceServiceInterface
type DeviceService struct {
	Service
	updateService UpdateServiceInterface
	inventory     inventory.ClientInterface
}

// DeviceDetails is a Device with Image and Update transactions
type DeviceDetails struct {
	Device             *models.Device              `json:"Device,omitempty"`
	Image              *ImageInfo                  `json:"ImageInfo"`
	UpdateTransactions *[]models.UpdateTransaction `json:"UpdateTransactions,omitempty"`
}

// GetDeviceByID receives DeviceID uint and get a *models.Device back
func (s *DeviceService) GetDeviceByID(deviceID uint) (*models.Device, error) {
	s.log = s.log.WithField("deviceID", deviceID)
	s.log.Info("Get device by id")
	var device models.Device
	result := db.DB.First(&device, deviceID)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error finding device")
		return nil, result.Error
	}
	return &device, nil
}

// GetDeviceByUUID receives UUID string and get a *models.Device back
func (s *DeviceService) GetDeviceByUUID(deviceUUID string) (*models.Device, error) {
	s.log = s.log.WithField("deviceUUID", deviceUUID)
	s.log.Info("Get device by uuid")
	var device models.Device
	result := db.DB.Where("uuid = ?", deviceUUID).First(&device)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error finding device")
		return nil, result.Error
	}
	return &device, nil
}

// GetDeviceDetails provides details for a given Device UUID
func (s *DeviceService) GetDeviceDetails(deviceUUID string) (*DeviceDetails, error) {
	s.log = s.log.WithField("deviceUUID", deviceUUID)
	s.log.Info("Get device by uuid")

	imageInfo, err := s.GetDeviceImageInfo(deviceUUID)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Could not find information about the running image on the device")
		return nil, err
	}
	device, err := s.GetDeviceByUUID(deviceUUID)
	if err != nil {
		s.log.Info("Could not find device on the devices table yet - returning just the data from inventory")
	}
	// In order to have an update transaction for a device it must be a least created
	var updates *[]models.UpdateTransaction
	if device != nil {
		updates, err = s.updateService.GetUpdateTransactionsForDevice(device)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Could not find information about updates for this device")
			return nil, err
		}
	}
	details := &DeviceDetails{
		Device:             device,
		Image:              imageInfo,
		UpdateTransactions: updates,
	}
	return details, nil
}

// ImageUpdateAvailable contains image and differences between current and available commits
type ImageUpdateAvailable struct {
	Image       models.Image `json:"Image"`
	PackageDiff DeltaDiff    `json:"PackageDiff"`
}

// DeltaDiff provides package difference details between current and available commits
type DeltaDiff struct {
	Added    []models.InstalledPackage `json:"Added"`
	Removed  []models.InstalledPackage `json:"Removed"`
	Upgraded []models.InstalledPackage `json:"Upgraded"`
}

// ImageInfo contains Image with updates available and rollback image
type ImageInfo struct {
	Image            models.Image            `json:"Image"`
	UpdatesAvailable *[]ImageUpdateAvailable `json:"UpdatesAvailable,omitempty"`
	Rollback         *models.Image           `json:"RollbackImage,omitempty"`
}

// GetUpdateAvailableForDeviceByUUID returns if exists update for the current image at the device.
func (s *DeviceService) GetUpdateAvailableForDeviceByUUID(deviceUUID string) ([]ImageUpdateAvailable, error) {
	s.log = s.log.WithField("deviceUUID", deviceUUID)
	var lastDeployment inventory.OSTree
	var imageDiff []ImageUpdateAvailable
	device, err := s.inventory.ReturnDevicesByID(deviceUUID)
	if err != nil || device.Total != 1 {
		return nil, new(DeviceNotFoundError)
	}

	lastDevice := device.Result[len(device.Result)-1]
	for _, rpmOstree := range lastDevice.Ostree.RpmOstreeDeployments {
		if rpmOstree.Booted {
			lastDeployment = rpmOstree
			break
		}
	}

	var images []models.Image
	var currentImage models.Image
	result := db.DB.Model(&models.Image{}).Joins("Commit").Where("OS_Tree_Commit = ?", lastDeployment.Checksum).First(&currentImage)
	if result.Error != nil || result.RowsAffected == 0 {
		s.log.WithField("error", result.Error.Error()).Error("Could not find device")
		return nil, new(DeviceNotFoundError)
	}

	err = db.DB.Model(&currentImage.Commit).Association("InstalledPackages").Find(&currentImage.Commit.InstalledPackages)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Could not find device")
		return nil, new(DeviceNotFoundError)
	}

	updates := db.DB.Where("Image_set_id = ? and Images.Status = ? and Images.Id > ?", currentImage.ImageSetID, models.ImageStatusSuccess, currentImage.ID).Joins("Commit").Order("Images.updated_at desc").Find(&images)
	if updates.Error != nil {
		return nil, new(UpdateNotFoundError)
	}
	if updates.RowsAffected == 0 {
		return imageDiff, nil
	}

	for _, upd := range images {
		db.DB.First(&upd.Commit, upd.CommitID)
		db.DB.Model(&upd.Commit).Association("InstalledPackages").Find(&upd.Commit.InstalledPackages)
		db.DB.Model(&upd).Association("Packages").Find(&upd.Packages)
		var delta ImageUpdateAvailable
		diff := getDiffOnUpdate(currentImage, upd)
		upd.Commit.InstalledPackages = nil // otherwise the frontend will get the whole list of installed packages
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

// GetDeviceImageInfo returns the information of a running image for a device
func (s *DeviceService) GetDeviceImageInfo(deviceUUID string) (*ImageInfo, error) {
	s.log = s.log.WithField("deviceUUID", deviceUUID)
	var ImageInfo ImageInfo
	var currentImage models.Image
	var rollback *models.Image
	var lastDeployment inventory.OSTree
	device, err := s.inventory.ReturnDevicesByID(deviceUUID)
	if err != nil || device.Total != 1 {
		return nil, new(DeviceNotFoundError)
	}

	lastDevice := device.Result[len(device.Result)-1]

	for _, rpmOstree := range lastDevice.Ostree.RpmOstreeDeployments {
		if rpmOstree.Booted {
			lastDeployment = rpmOstree
			break
		}
	}

	result := db.DB.Model(&models.Image{}).Joins("Commit").Where("OS_Tree_Commit = ?", lastDeployment.Checksum).First(&currentImage)

	if result.Error != nil || result == nil {
		s.log.WithField("error", result.Error.Error()).Error("Could not find device image info")
		return nil, new(ImageNotFoundError)
	}
	if currentImage.ImageSetID != nil {
		db.DB.Where("Image_Set_Id = ? and id < ?", currentImage.ImageSetID, currentImage.ID).Last(&rollback)
	}
	updateAvailable, err := s.GetUpdateAvailableForDeviceByUUID(deviceUUID)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Could not find updates available to get image info")
		return nil, err
	} else if updateAvailable != nil {
		ImageInfo.UpdatesAvailable = &updateAvailable
	}
	ImageInfo.Rollback = rollback
	ImageInfo.Image = currentImage

	return &ImageInfo, nil
}
