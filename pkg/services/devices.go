// FIXME: golangci-lint
// nolint:errcheck,gocritic,gosec,govet,revive
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	version "github.com/knqyf263/go-rpm-version"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	// InventoryEventTypeCreated represent the Created inventory event type
	InventoryEventTypeCreated = "created"
	// InventoryEventTypeUpdated represent the Updated inventory event type
	InventoryEventTypeUpdated = "updated"
	// InventoryEventTypeDelete represent the "delete" inventory event type
	InventoryEventTypeDelete = "delete"
	// InventoryHostTypeEdge represent the inventory host_ype = "edge"
	InventoryHostTypeEdge = "edge"
)

// DeviceServiceInterface defines the interface to handle the business logic of RHEL for Edge Devices
type DeviceServiceInterface interface {
	GetDevices(params *inventory.Params) (*models.DeviceDetailsList, error)
	GetDeviceByID(deviceID uint) (*models.Device, error)
	GetDevicesView(limit int, offset int, tx *gorm.DB) (*models.DeviceViewList, error)
	GetDevicesCount(tx *gorm.DB) (int64, error)
	GetDeviceByUUID(deviceUUID string) (*models.Device, error)
	// Device by UUID methods
	GetDeviceDetailsByUUID(deviceUUID string) (*models.DeviceDetails, error)
	GetUpdateAvailableForDeviceByUUID(deviceUUID string, latest bool) ([]models.ImageUpdateAvailable, error)
	GetDeviceImageInfoByUUID(deviceUUID string) (*models.ImageInfo, error)
	GetLatestCommitFromDevices(orgID string, devicesUUID []string) (uint, error)
	// Device Object Methods
	GetDeviceDetails(device inventory.Device) (*models.DeviceDetails, error)
	GetUpdateAvailableForDevice(device inventory.Device, latest bool) ([]models.ImageUpdateAvailable, error)
	GetDeviceImageInfo(device inventory.Device) (*models.ImageInfo, error)
	GetDeviceLastDeployment(device inventory.Device) *inventory.OSTree
	GetDeviceLastBootedDeployment(device inventory.Device) *inventory.OSTree
	ProcessPlatformInventoryCreateEvent(message []byte) error
	ProcessPlatformInventoryUpdatedEvent(message []byte) error
	ProcessPlatformInventoryDeleteEvent(message []byte) error
}

// RpmOSTreeDeployment is the member of PlatformInsightsCreateUpdateEventPayload host system profile rpm ostree deployments list
type RpmOSTreeDeployment struct {
	Booted   bool   `json:"booted"`
	Checksum string `json:"checksum"`
}

// PlatformInsightsCreateUpdateEventPayload is the body of the create event found on the platform.inventory.events kafka topic.
type PlatformInsightsCreateUpdateEventPayload struct {
	Type string `json:"type"`
	Host host   `json:"host"`
}

type systemProfile struct {
	HostType             string                `json:"host_type"`
	RpmOSTreeDeployments []RpmOSTreeDeployment `json:"rpm_ostree_deployments"`
	RHCClientID          string                `json:"rhc_client_id,omitempty"`
}

type host struct {
	ID            string             `json:"id"`
	Name          string             `json:"display_name"`
	OrgID         string             `json:"org_id"`
	InsightsID    string             `json:"insights_id"`
	Updated       models.EdgeAPITime `json:"updated"`
	SystemProfile systemProfile      `json:"system_profile"`
}

// PlatformInsightsDeleteEventPayload is the body of the delete event found on the platform.inventory.events kafka topic.
type PlatformInsightsDeleteEventPayload struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	OrgID string `json:"org_id"`
}

// NewDeviceService gives a instance of the main implementation of DeviceServiceInterface
func NewDeviceService(ctx context.Context, log *log.Entry) DeviceServiceInterface {
	return &DeviceService{
		UpdateService: NewUpdateService(ctx, log),
		ImageService:  NewImageService(ctx, log),
		Inventory:     inventory.InitClient(ctx, log),
		Service:       Service{ctx: ctx, log: log.WithField("service", "image")},
	}
}

// DeviceService is the main implementation of a DeviceServiceInterface
type DeviceService struct {
	Service
	UpdateService UpdateServiceInterface
	ImageService  ImageServiceInterface
	Inventory     inventory.ClientInterface
}

// GetDeviceByID receives DeviceID uint and get a *models.Device back
func (s *DeviceService) GetDeviceByID(deviceID uint) (*models.Device, error) {
	s.log = s.log.WithField("deviceID", deviceID)
	s.log.Info("Get device by id")
	var device models.Device
	result := db.DB.First(&device, deviceID)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error finding device")
		return nil, new(DeviceNotFoundError)
	}

	// Load from device DB the groups info and add to the new struct
	err := db.DB.Model(&device).Association("DevicesGroups").Find(&device.DevicesGroups)
	if err != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error finding associated devicegroups for device")
		return nil, new(DeviceGroupNotFound)
	}

	return &device, nil
}

// GetDeviceByUUID receives UUID string and get a *models.Device back
func (s *DeviceService) GetDeviceByUUID(deviceUUID string) (*models.Device, error) {
	ulog := s.log.WithField("deviceUUID", deviceUUID)
	ulog.Info("Get device by uuid")
	var device models.Device
	if result := db.DB.Where("uuid = ?", deviceUUID).Preload("DevicesGroups").First(&device); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			ulog.WithField("error", result.Error.Error()).Error("device not found by UUID")
			return nil, new(DeviceNotFoundError)
		}
		ulog.WithField("error", result.Error.Error()).Error("unknown db error occurred when getting device by UUID")
		return nil, result.Error
	}

	return &device, nil
}

// GetDeviceDetails provides details for a given Device by going to inventory API and trying to also merge with the information on our database
func (s *DeviceService) GetDeviceDetails(device inventory.Device) (*models.DeviceDetails, error) {
	ulog := s.log.WithField("deviceUUID", device.ID)
	ulog.Info("Get device details")

	// Get device's running image
	imageInfo, err := s.GetDeviceImageInfo(device)
	if err != nil {
		ulog.WithField("error", err.Error()).Error("Could not find information about the running image on the device")
		return nil, err
	}
	// Get device on Edge API, not on inventory
	databaseDevice, err := s.GetDeviceByUUID(device.ID)

	// In order to have an update transaction for a device it must be a least created
	var updates *[]models.UpdateTransaction
	if databaseDevice != nil {
		updates, err = s.UpdateService.GetUpdateTransactionsForDevice(databaseDevice)
		if err != nil {
			ulog.WithField("error", err.Error()).Error("Could not find information about updates for this device")
			return nil, err
		}
	}
	if err != nil {
		ulog.Info("Could not find device on the devices table yet - returning just the data from inventory")
		// if err != nil then databaseDevice is nil pointer
		databaseDevice = &models.Device{
			UUID:        device.ID,
			RHCClientID: device.Ostree.RHCClientID,
		}
	}
	details := &models.DeviceDetails{
		Device: models.EdgeDevice{
			Device:     databaseDevice,
			DeviceName: device.DisplayName,
			LastSeen:   device.LastSeen,
			OrgID:      device.OrgID},
		Image:              imageInfo,
		UpdateTransactions: updates,
		// Given we are concat the info from inventory with our db, we need to add this field to the struct to be able to see the result
		DevicesGroups: &databaseDevice.DevicesGroups,
	}
	return details, nil
}

// GetDeviceDetailsByUUID provides details for a given Device UUID by going to inventory API and trying to also merge with the information on our database
func (s *DeviceService) GetDeviceDetailsByUUID(deviceUUID string) (*models.DeviceDetails, error) {
	// s.log = s.log.WithField("deviceUUID", deviceUUID)
	resp, err := s.Inventory.ReturnDevicesByID(deviceUUID)
	if err != nil || resp.Total != 1 {
		return nil, new(DeviceNotFoundError)
	}
	return s.GetDeviceDetails(resp.Result[len(resp.Result)-1])
}

// GetUpdateAvailableForDeviceByUUID returns if it exists an update for the current image at the device given its UUID.
func (s *DeviceService) GetUpdateAvailableForDeviceByUUID(deviceUUID string, latest bool) ([]models.ImageUpdateAvailable, error) {
	// s.log = s.log.WithField("deviceUUID", deviceUUID)
	resp, err := s.Inventory.ReturnDevicesByID(deviceUUID)
	if err != nil || resp.Total != 1 {
		return nil, new(DeviceNotFoundError)
	}
	return s.GetUpdateAvailableForDevice(resp.Result[len(resp.Result)-1], latest)
}

// GetUpdateAvailableForDevice returns if it exists an update for the current image at the device.
func (s *DeviceService) GetUpdateAvailableForDevice(device inventory.Device, latest bool) ([]models.ImageUpdateAvailable, error) {
	var imageDiff []models.ImageUpdateAvailable
	lastDeployment := s.GetDeviceLastBootedDeployment(device)
	if lastDeployment == nil {
		return nil, new(DeviceNotFoundError)
	}

	var currentImage models.Image
	result := db.DB.Model(&models.Image{}).Joins("Commit").Where("OS_Tree_Commit = ?", lastDeployment.Checksum).First(&currentImage)
	if result.Error != nil || result.RowsAffected == 0 {
		s.log.WithField("error", result.Error.Error()).Error("Could not find device")
		return nil, new(DeviceNotFoundError)
	}

	err := db.DB.Model(&currentImage.Commit).Association("InstalledPackages").Find(&currentImage.Commit.InstalledPackages)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Could not find device")
		return nil, new(DeviceNotFoundError)
	}

	var images []models.Image
	query := db.DB.Where("Image_set_id = ? and Images.Status = ? and Images.Id > ?",
		currentImage.ImageSetID, models.ImageStatusSuccess, currentImage.ID,
	).Joins("Commit").Order("Images.version desc")

	var updates *gorm.DB
	if latest {
		updates = query.First(&images)
	} else {
		updates = query.Find(&images)
	}

	if updates.Error != nil {
		return nil, new(UpdateNotFoundError)
	}
	if updates.RowsAffected == 0 {
		return imageDiff, nil
	}

	for _, upd := range images {
		upd := upd // this will prevent implicit memory aliasing in the loop
		db.DB.First(&upd.Commit, upd.CommitID)

		if err := db.DB.Model(&upd.Commit).Association("InstalledPackages").Find(&upd.Commit.InstalledPackages); err != nil {
			s.log.WithField("error", err.Error()).Error("Could not find installed packages")
			return nil, err
		}
		if err := db.DB.Model(&upd).Association("Packages").Find(&upd.Packages); err != nil {
			s.log.WithField("error", err.Error()).Error("Could not find packages")
			return nil, err
		}
		if err := db.DB.Model(&upd).Association("CustomPackages").Find(&upd.CustomPackages); err != nil {
			s.log.WithField("error", err.Error()).Error("Could not find custom packages")
			return nil, err
		}
		var delta models.ImageUpdateAvailable
		SystemRunning, err := s.GetDevicesCountByImage(upd.ID)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Could not find device image info")
			return nil, new(ImageNotFoundError)
		}
		diff := GetDiffOnUpdate(currentImage, upd)
		upd.Commit.InstalledPackages = nil // otherwise the frontend will get the whole list of installed packages
		delta.Image = upd
		delta.PackageDiff = diff
		delta.SystemRunningCurrentImage = SystemRunning
		delta.CanUpdate = s.CanUpdate(currentImage.Distribution, upd.Distribution)
		delta.TotalPackages = len(upd.Commit.InstalledPackages)
		imageDiff = append(imageDiff, delta)

	}
	return imageDiff, nil
}

func getPackageDiff(a, b []models.InstalledPackage) []models.InstalledPackage {
	diff := make([]models.InstalledPackage, 0)
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

// GetDiffOnUpdate returns the diff between two images.
// TODO: Move out to a different package, as this is devices related, either to image service or image models.
func GetDiffOnUpdate(oldImg models.Image, newImg models.Image) models.PackageDiff {
	results := models.PackageDiff{
		Added:    getPackageDiff(newImg.Commit.InstalledPackages, oldImg.Commit.InstalledPackages),
		Removed:  getPackageDiff(oldImg.Commit.InstalledPackages, newImg.Commit.InstalledPackages),
		Upgraded: getVersionDiff(newImg.Commit.InstalledPackages, oldImg.Commit.InstalledPackages),
	}
	return results
}

// GetDeviceImageInfoByUUID returns the information of a running image for a device given its UUID
func (s *DeviceService) GetDeviceImageInfoByUUID(deviceUUID string) (*models.ImageInfo, error) {
	// s.log = s.log.WithField("deviceUUID", deviceUUID)
	resp, err := s.Inventory.ReturnDevicesByID(deviceUUID)
	if err != nil || resp.Total != 1 {
		return nil, new(DeviceNotFoundError)
	}
	return s.GetDeviceImageInfo(resp.Result[len(resp.Result)-1])
}

// GetDeviceImageInfo returns the information of a the running image for a device
func (s *DeviceService) GetDeviceImageInfo(device inventory.Device) (*models.ImageInfo, error) {
	var ImageInfo models.ImageInfo
	var currentImage *models.Image
	var rollback *models.Image
	var lastDeployment *inventory.OSTree

	for _, rpmOstree := range device.Ostree.RpmOstreeDeployments {
		rpmOstree := rpmOstree
		if rpmOstree.Booted {
			lastDeployment = &rpmOstree
			break
		}
	}
	if lastDeployment == nil {
		return nil, new(ImageNotFoundError)
	}
	currentImage, err := s.ImageService.GetImageByOSTreeCommitHash(lastDeployment.Checksum)

	if err != nil {
		s.log.WithField("error", err.Error()).Error("Could not find device image info")
		return nil, new(ImageNotFoundError)
	}
	SystemRunning, err := s.GetDevicesCountByImage(currentImage.ID)
	fmt.Print("**************************************")
	fmt.Printf("\n SystemRunning %v \n", SystemRunning)
	fmt.Printf("\n currentImage.ID %v \n", currentImage.ID)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Could not find device image info")
		return nil, new(ImageNotFoundError)
	}
	if currentImage.Version > 1 {
		rollback, err = s.ImageService.GetRollbackImage(currentImage)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Could not find rollback image info")
			return nil, new(ImageNotFoundError)
		}
	}

	updateAvailable, err := s.GetUpdateAvailableForDevice(device, false) // false = get all updates
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Could not find updates available to get image info")
		return nil, err
	} else if updateAvailable != nil {
		ImageInfo.UpdatesAvailable = &updateAvailable

	}
	ImageInfo.Rollback = rollback
	ImageInfo.Image = *currentImage
	ImageInfo.SystemRunningCurrentImage = SystemRunning
	ImageInfo.TotalPackages = len(currentImage.Commit.InstalledPackages)

	return &ImageInfo, nil
}

// GetDevices returns a list of EdgeDevices, which is a mix of device information from EdgeAPI and InventoryAPI
func (s *DeviceService) GetDevices(params *inventory.Params) (*models.DeviceDetailsList, error) {
	s.log.Info("Getting devices...")
	inventoryDevices, err := s.Inventory.ReturnDevices(params)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error retrieving devices from inventory")
		return nil, err
	}
	list := &models.DeviceDetailsList{
		Devices: make([]models.DeviceDetails, inventoryDevices.Count),
		Count:   inventoryDevices.Count,
		Total:   inventoryDevices.Total,
	}
	if inventoryDevices.Count == 0 {
		return list, nil
	}
	// Build a map from the received devices UUIDs and the already saved db devices IDs
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		return nil, err
	}
	devicesUUIDs := make([]string, 0, len(inventoryDevices.Result))
	for _, device := range inventoryDevices.Result {
		devicesUUIDs = append(devicesUUIDs, device.ID)
	}
	var storedDevices []models.Device
	if res := db.Org(orgID, "").Where("uuid IN ?", devicesUUIDs).Find(&storedDevices); res.Error != nil {
		return nil, res.Error
	}

	mapDevicesUUIDToID := make(map[string]uint, len(devicesUUIDs))
	for _, device := range storedDevices {
		mapDevicesUUIDToID[device.UUID] = device.ID
	}

	s.log.Info("Adding Edge Device information...")
	for i, device := range inventoryDevices.Result {
		dbDeviceID, ok := mapDevicesUUIDToID[device.ID]
		if !ok {
			dbDeviceID = 0
		}

		// Load from device DB the groups info and add to the new struct
		var storeDevice models.Device
		// Don't throw error if device not found
		db.DB.Where("id=?", dbDeviceID).First(&storeDevice)
		err := db.DB.Model(&storeDevice).Debug().Association("UpdateTransaction").Find(&storeDevice.UpdateTransaction)

		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error finding associated updates for device")
			return nil, new(DeviceGroupNotFound)
		}

		err = db.DB.Model(&storeDevice).Association("DevicesGroups").Find(&storeDevice.DevicesGroups)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error finding associated devicegroups for device")
			return nil, new(DeviceGroupNotFound)
		}

		deviceUpdating := false

		if len(*storeDevice.UpdateTransaction) > 0 {
			last := (*storeDevice.UpdateTransaction)[len(*storeDevice.UpdateTransaction)-1]
			if last.Status == models.UpdateStatusCreated || last.Status == models.UpdateStatusBuilding {
				deviceUpdating = true
			}
		}
		dd := models.DeviceDetails{}

		dd.Device = models.EdgeDevice{
			Device: &models.Device{
				Model:             models.Model{ID: dbDeviceID},
				UUID:              device.ID,
				RHCClientID:       device.Ostree.RHCClientID,
				Name:              device.DisplayName,
				OrgID:             device.OrgID,
				DevicesGroups:     storeDevice.DevicesGroups,
				UpdateTransaction: storeDevice.UpdateTransaction,
			},
			OrgID:      device.OrgID,
			DeviceName: device.DisplayName,
			LastSeen:   device.LastSeen,
		}
		dd.Updating = &deviceUpdating
		lastDeployment := s.GetDeviceLastDeployment(device)
		if lastDeployment != nil {
			dd.Device.Booted = lastDeployment.Booted
		}
		// s.log.WithField("deviceID", device.ID).Info("Getting image info for device...")
		// imageInfo, err := s.GetDeviceImageInfo(device)
		// if err != nil {
		// 	dd.Image = nil
		// } else if imageInfo != nil {
		// 	dd.Image = imageInfo
		// }
		// TODO: Add back the ability to filter by status when we figure out how to do pagination
		// if params != nil && imageInfo != nil {
		// 	if params.DeviceStatus == "update_available" && imageInfo.UpdatesAvailable != nil {
		// 		list.Devices = append(list.Devices, dd)
		// 	} else if params.DeviceStatus == "running" && imageInfo.UpdatesAvailable == nil {
		// 		list.Devices = append(list.Devices, dd)
		// 	} else if params.DeviceStatus == "" {
		// 		list.Devices = append(list.Devices, dd)
		// 	}
		list.Devices[i] = dd
	}
	return list, nil
}

// GetDeviceLastBootedDeployment returns the last booted deployment for a device
func (s *DeviceService) GetDeviceLastBootedDeployment(device inventory.Device) *inventory.OSTree {
	var lastDeployment *inventory.OSTree
	for _, rpmOstree := range device.Ostree.RpmOstreeDeployments {
		rpmOstree := rpmOstree
		if rpmOstree.Booted {
			lastDeployment = &rpmOstree
			break
		}
	}
	return lastDeployment
}

// GetDeviceLastDeployment returns the last deployment for a device
func (s *DeviceService) GetDeviceLastDeployment(device inventory.Device) *inventory.OSTree {
	if len(device.Ostree.RpmOstreeDeployments) > 0 {
		return &device.Ostree.RpmOstreeDeployments[0]
	}
	return nil
}

// SetDeviceUpdateAvailability set whether there is a device Updates available ot not.
func (s *DeviceService) SetDeviceUpdateAvailability(orgID string, deviceID uint) error {

	var device models.Device
	if result := db.Org(orgID, "").First(&device, deviceID); result.Error != nil {
		return result.Error
	}
	if device.ImageID == 0 {
		return new(DeviceHasImageUndefined)
	}

	// get the device image
	var deviceImage models.Image
	if result := db.Org(orgID, "").First(&deviceImage, device.ImageID); result.Error != nil {
		return result.Error
	}

	// check for updates , find if any later images exists
	var updateImages []models.Image
	if result := db.Org(orgID, "").Select("id").Where("image_set_id = ? AND status = ? AND created_at > ?",
		deviceImage.ImageSetID, models.ImageStatusSuccess, deviceImage.CreatedAt).Find(&updateImages); result.Error != nil {
		return result.Error
	}

	device.UpdateAvailable = len(updateImages) > 0

	if result := db.DB.Save(device); result.Error != nil {
		return result.Error
	}

	return nil
}

// processPlatformInventoryEventUpdateDevice update device image id and set update availability
func (s *DeviceService) processPlatformInventoryEventUpdateDevice(eventData PlatformInsightsCreateUpdateEventPayload) error {

	// Get the event db device
	device, err := s.GetDeviceByUUID(eventData.Host.ID)
	if err != nil {
		return err
	}

	logger := s.log.WithFields(log.Fields{
		"host_id":    device.UUID,
		"event_type": eventData.Type,
	})
	// Get the last rpmOSTree deployment commit checksum
	deployments := eventData.Host.SystemProfile.RpmOSTreeDeployments
	if len(deployments) == 0 {
		return new(ImageNotFoundError)
	}
	CommitCheck := deployments[0].Checksum
	// Get the related commit image
	var deviceImage models.Image
	if result := db.Org(device.OrgID, "images").Select("images.id").
		Joins("JOIN commits ON commits.id = images.commit_id").Where("commits.os_tree_commit = ? ", CommitCheck).
		First(&deviceImage); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			logger.WithField("error", result.Error.Error()).Error("device image not found")
		} else {
			logger.WithField("error", result.Error.Error()).Error("unknown error occurred when getting device image")
		}
		return result.Error
	}

	device.ImageID = deviceImage.ID

	if result := db.DB.Save(device); result.Error != nil {
		logger.WithField("error", result.Error.Error()).Error("error occurred saving device image")
		return result.Error
	}

	return s.SetDeviceUpdateAvailability(device.OrgID, device.ID)
}

// ProcessPlatformInventoryUpdatedEvent processes messages from platform.inventory.events kafka topic with event_type="updated"
func (s *DeviceService) ProcessPlatformInventoryUpdatedEvent(message []byte) error {
	var eventData PlatformInsightsCreateUpdateEventPayload
	if err := json.Unmarshal(message, &eventData); err != nil {
		s.log.WithFields(log.Fields{"value": string(message), "error": err}).Debug(
			"Skipping kafka message - it's not a Platform Insights Inventory message with event type: updated, as unable to unmarshal the message")
		return err
	}
	if eventData.Type != InventoryEventTypeUpdated || eventData.Host.SystemProfile.HostType != InventoryHostTypeEdge {
		// s.log.Debug("Skipping kafka message - Platform Insights Inventory message host type is not edge and event type is not updated")
		return nil
	}
	deviceUUID := eventData.Host.ID
	deviceOrgID := eventData.Host.OrgID
	deviceName := eventData.Host.Name
	device, err := s.GetDeviceByUUID(deviceUUID)
	if err != nil {
		if _, ok := err.(*DeviceNotFoundError); ok {
			// create a new device if it does not exist.
			var newDevice = models.Device{
				UUID:        deviceUUID,
				RHCClientID: eventData.Host.SystemProfile.RHCClientID,
				OrgID:       deviceOrgID,
				Name:        deviceName,
				LastSeen:    eventData.Host.Updated,
			}
			if result := db.DB.Create(&newDevice); result.Error != nil {
				s.log.WithFields(log.Fields{"host_id": deviceUUID, "error": result.Error.Error()}).Error("Error creating device")
				return result.Error
			}
			s.log.WithField("host_id", deviceUUID).Debug("Device Org created")
			return s.processPlatformInventoryEventUpdateDevice(eventData)
		}
		s.log.WithFields(log.Fields{"host_id": deviceUUID, "error": err.Error()}).Error("unknown error when getting device by uuid")
		return err
	}
	if device.OrgID == "" {
		// Update orgID if undefined
		device.OrgID = deviceOrgID
	}
	// update rhc client id if undefined
	if eventData.Host.SystemProfile.RHCClientID != "" && device.RHCClientID != eventData.Host.SystemProfile.RHCClientID {
		device.RHCClientID = eventData.Host.SystemProfile.RHCClientID
	}
	// always update device name and last seen datetime
	device.LastSeen = eventData.Host.Updated
	device.Name = deviceName

	if result := db.DB.Save(device); result.Error != nil {
		s.log.WithFields(log.Fields{"host_id": deviceUUID, "error": result.Error}).Error("Error updating device")
		return result.Error
	}
	s.log.WithField("host_id", deviceUUID).Debug("Device OrgID updated")

	return s.processPlatformInventoryEventUpdateDevice(eventData)
}

// GetDevicesCount get the device groups Org records count from the database
func (s *DeviceService) GetDevicesCount(tx *gorm.DB) (int64, error) {
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		return 0, err
	}

	if tx == nil {
		tx = db.DB
	}

	var count int64

	res := db.OrgDB(orgID, tx, "").Model(&models.Device{}).Count(&count)

	if res.Error != nil {
		s.log.WithField("error", res.Error.Error()).Error("Error getting device groups count")
		return 0, res.Error
	}

	return count, nil
}

// GetDevicesCountByImage returns a list of EdgeDevices for a given org.
func (s *DeviceService) GetDevicesCountByImage(imageId uint) (int64, error) {
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		return 0, err
	}

	var count int64
	res := db.OrgDB(orgID, db.DB.Model(&models.Device{}), "devices").Debug().Find(&models.Device{}, "image_id =? ", imageId).Count(&count)
	if res.Error != nil {
		s.log.WithField("error", res.Error.Error()).Error("Error getting device groups count")
		return 0, res.Error
	}
	return count, nil
}

// GetDevicesView returns a list of EdgeDevices for a given org.
func (s *DeviceService) GetDevicesView(limit int, offset int, tx *gorm.DB) (*models.DeviceViewList, error) {
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		return nil, err
	}

	if tx == nil {
		tx = db.DB
	}

	var storedDevices []models.Device
	// search for all stored devices that are also in inventory
	if res := db.OrgDB(orgID, tx, "").Limit(limit).Offset(offset).
		Preload("UpdateTransaction", func(db *gorm.DB) *gorm.DB {
			return db.Order("update_transactions.created_at DESC")
		}).
		Preload("UpdateTransaction.DispatchRecords").
		Preload("DevicesGroups").Find(&storedDevices); res.Error != nil {
		return nil, res.Error
	}

	// Check inventory to see if you return the same number of devices, else, sync with inventory in a routine
	if feature.DeviceSync.IsEnabled() {
		var params *inventory.Params
		inventoryDevices, err := s.Inventory.ReturnDevices(params)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error retrieving devices from inventory")
			return nil, err
		}

		var total int64
		if res := db.Org(orgID, "").Model(&models.Device{}).Count(&total); res.Error != nil {
			s.log.WithField("error", res.Error.Error()).Error("Error getting device count")
			return nil, res.Error
		}
		s.log.WithFields(log.Fields{"edge_count": total, "insights_count": inventoryDevices.Total}).Debug("Comparing edge and insights inventory counts")
		if int64(inventoryDevices.Total) != total {
			s.log.WithFields(log.Fields{"edge_count": total, "insights_count": inventoryDevices.Total}).Debug("Inventory counts do not match. Calling syncDevicesWithInventory")
			go s.syncDevicesWithInventory(orgID)
		}
	}

	returnDevices, err := ReturnDevicesView(storedDevices, orgID)
	if err != nil {
		return nil, err
	}
	list := &models.DeviceViewList{
		Devices: returnDevices,
		Total:   int64(len(storedDevices)),
	}
	return list, nil
}

// ReturnDevicesView returns the devices association and status properly
func ReturnDevicesView(storedDevices []models.Device, orgID string) ([]models.DeviceView, error) {
	deviceToGroupMap := make(map[uint][]models.DeviceDeviceGroup)
	for _, device := range storedDevices {
		var tempArr []models.DeviceDeviceGroup
		for _, deviceGroups := range device.DevicesGroups {
			tempArr = append(tempArr, models.DeviceDeviceGroup{ID: deviceGroups.ID, Name: deviceGroups.Name})
		}
		deviceToGroupMap[device.ID] = tempArr
	}

	type neededImageInfo struct {
		Name       string
		ImageSetID uint
	}
	type neededDeviceInfo struct {
		Status           string
		DispatcherStatus string
		DispatcherReason string
	}

	// create a map of unique image id's. We don't want to look at a given image id more than once.
	setOfDeviceInfo := make(map[uint]*neededDeviceInfo)
	setOfImages := make(map[uint]*neededImageInfo)
	for index, device := range storedDevices {
		var deviceInfo = neededDeviceInfo{
			Status: models.DeviceViewStatusRunning,
		}
		crtDevice := storedDevices[index]

		if crtDevice.UpdateTransaction != nil && len(*crtDevice.UpdateTransaction) > 0 {
			updateTransactions := *crtDevice.UpdateTransaction

			log.WithFields(log.Fields{
				"deviceID":           crtDevice.ID,
				"updateTransactions": updateTransactions,
			}).Debug("Found update transactions for device")

			latestUpdateTransaction := updateTransactions[0]
			updateStatus := latestUpdateTransaction.Status

			log.WithFields(log.Fields{
				"deviceID":     crtDevice.ID,
				"updateStatus": updateStatus,
			}).Debug("Found update status for device")

			if latestUpdateTransaction.DispatchRecords != nil && len(latestUpdateTransaction.DispatchRecords) > 0 {
				for _, dispatcherRecord := range latestUpdateTransaction.DispatchRecords {
					if dispatcherRecord.DeviceID == device.ID {
						deviceInfo.DispatcherStatus = dispatcherRecord.Status
						deviceInfo.DispatcherReason = dispatcherRecord.Reason
						break
					}
				}
				if deviceInfo.DispatcherStatus == models.DispatchRecordStatusComplete {
					deviceInfo.DispatcherStatus = models.UpdateStatusSuccess
				}
			}

			if updateStatus == models.UpdateStatusBuilding {
				deviceInfo.Status = models.DeviceViewStatusUpdating
			} else if updateStatus == models.UpdateStatusDeviceDisconnected {
				deviceInfo.DispatcherStatus = models.UpdateStatusDeviceUnresponsive
			}
		}

		setOfDeviceInfo[crtDevice.ID] = &deviceInfo
		if device.ImageID != 0 {
			setOfImages[device.ImageID] = &neededImageInfo{}
		}
	}

	// using the map of unique image ID's, get the corresponding image name and status.
	var imagesIDS []uint
	for key := range setOfImages {
		imagesIDS = append(imagesIDS, key)
	}

	var images []models.Image
	if result := db.Org(orgID, "").Where("id IN (?) AND image_set_id IS NOT NULL", imagesIDS).Find(&images); result.Error != nil {
		return nil, result.Error
	}

	for _, image := range images {
		setOfImages[image.ID] = &neededImageInfo{Name: image.Name, ImageSetID: *image.ImageSetID}
	}

	// build the return object
	var returnDevices []models.DeviceView
	for _, device := range storedDevices {
		var imageName string
		deviceInfo := &neededDeviceInfo{}
		var imageSetID uint
		var deviceGroups []models.DeviceDeviceGroup
		if _, ok := setOfImages[device.ImageID]; ok {
			imageName = setOfImages[device.ImageID].Name
			imageSetID = setOfImages[device.ImageID].ImageSetID
		}
		if _, ok := deviceToGroupMap[device.ID]; ok {
			deviceGroups = deviceToGroupMap[device.ID]
		}
		if _, ok := setOfDeviceInfo[device.ID]; ok {
			deviceInfo = setOfDeviceInfo[device.ID]
		}
		currentDeviceView := models.DeviceView{
			DeviceID:         device.ID,
			DeviceName:       device.Name,
			DeviceUUID:       device.UUID,
			ImageID:          device.ImageID,
			ImageName:        imageName,
			LastSeen:         device.LastSeen,
			UpdateAvailable:  device.UpdateAvailable,
			Status:           deviceInfo.Status,
			ImageSetID:       imageSetID,
			DeviceGroups:     deviceGroups,
			DispatcherStatus: deviceInfo.DispatcherStatus,
			DispatcherReason: deviceInfo.DispatcherReason,
		}
		returnDevices = append(returnDevices, currentDeviceView)
	}
	return returnDevices, nil
}

// GetLatestCommitFromDevices fetches the commitID from the latest Device Image
func (s *DeviceService) GetLatestCommitFromDevices(orgID string, devicesUUID []string) (uint, error) {
	var devices []models.Device

	if result := db.Org(orgID, "").Where("uuid IN ?", devicesUUID).Find(&devices); result.Error != nil {
		return 0, result.Error
	}

	devicesImageID := make([]uint, 0, len(devices))

	for _, device := range devices {
		if int(device.ImageID) == 0 {
			return 0, new(DeviceHasImageUndefined)
		}
		devicesImageID = append(devicesImageID, device.ImageID)
	}
	var devicesImage []models.Image
	if result := db.Org(orgID, "").Find(&devicesImage, devicesImageID); result.Error != nil {
		return 0, result.Error
	}
	// finding unique ImageSetID for device Image
	devicesImageSetID := make(map[uint]bool, len(devicesImage))
	var imageSetID uint
	for _, image := range devicesImage {
		if image.ImageSetID == nil {
			return 0, new(ImageHasNoImageSet)
		}
		imageSetID = *image.ImageSetID
		devicesImageSetID[*image.ImageSetID] = true

	}
	if len(devicesImageSetID) > 1 {
		return 0, new(DevicesHasMoreThanOneImageSet)
	}

	// check for updates , find if any later images exists to get the commitID
	var updateImages []models.Image
	if result := db.Org(orgID, "").Model(&models.Image{}).Where("image_set_id = ? AND status = ?", imageSetID, models.ImageStatusSuccess).Order("version desc").Find(&updateImages); result.Error != nil {
		return 0, result.Error
	}

	if len(updateImages) == 0 {
		return 0, new(DeviceHasNoImageUpdate)
	}

	return updateImages[0].CommitID, nil
}

// ProcessPlatformInventoryCreateEvent is a method to processes messages from platform.inventory.events kafka topic and save them as devices in the DB
func (s *DeviceService) ProcessPlatformInventoryCreateEvent(message []byte) error {
	var e *PlatformInsightsCreateUpdateEventPayload
	err := json.Unmarshal(message, &e)
	if err != nil {
		s.log.WithFields(log.Fields{
			"value": string(message),
			"error": err.Error(),
		}).Debug("Skipping message - it is not a create message")
	} else {
		if e.Type == InventoryEventTypeCreated && e.Host.SystemProfile.HostType == InventoryHostTypeEdge {
			s.log.WithField("message", string(message)).Debug("Inventory create event message body")
			return s.platformInventoryCreateEventHelper(*e)
		}
		if feature.KafkaLogging.IsEnabled() {
			s.log.Debug("Skipping message - not an edge create message from platform insights")
		}
	}
	return nil
}

func (s *DeviceService) platformInventoryCreateEventHelper(e PlatformInsightsCreateUpdateEventPayload) error {
	s.log.WithFields(log.Fields{
		"host_id": string(e.Host.ID),
		"name":    string(e.Host.Name),
	}).Debug("Saving newly created edge device")
	var newDevice = models.Device{
		UUID:        e.Host.ID,
		RHCClientID: e.Host.SystemProfile.RHCClientID,
		OrgID:       e.Host.OrgID,
		Name:        e.Host.Name,
		LastSeen:    e.Host.Updated,
	}

	// We should not create a new device if UUID already exists
	result := db.DB.Debug().Where(&models.Device{UUID: newDevice.UUID}).FirstOrCreate(&newDevice)

	if result.Error != nil {
		s.log.WithFields(log.Fields{
			"host_id": string(e.Host.ID),
			"error":   result.Error,
		}).Error("Error creating device")
		return result.Error
	}

	return s.processPlatformInventoryEventUpdateDevice(e)
}

// ProcessPlatformInventoryDeleteEvent processes messages from platform.inventory.events kafka topic with event_type="delete"
func (s *DeviceService) ProcessPlatformInventoryDeleteEvent(message []byte) error {

	var eventData PlatformInsightsDeleteEventPayload

	if err := json.Unmarshal(message, &eventData); err != nil {
		s.log.WithFields(log.Fields{"value": string(message), "error": err}).Debug(
			"Skipping kafka message - it's not a Platform Insights Inventory message with event type: delete, as unable to unmarshal the message",
		)
		return err
	}
	if eventData.Type != InventoryEventTypeDelete || eventData.ID == "" {
		s.log.Debug("Skipping kafka message - Platform Insights Inventory message host id is undefined or event type is not delete")
		return nil
	}

	deviceUUID := eventData.ID
	deviceOrgID := eventData.OrgID
	var devices []models.Device
	if result := db.Org(deviceOrgID, "").Where("uuid = ?", deviceUUID).Find(&devices); result.Error != nil {
		s.log.WithFields(
			log.Fields{"host_id": deviceUUID, "OrgID": deviceOrgID, "error": result.Error},
		).Debug("Error retrieving the devices")
		return result.Error
	}
	if len(devices) == 0 {
		return nil
	}
	if result := db.DB.Delete(&devices[0]); result.Error != nil {
		s.log.WithFields(
			log.Fields{"host_id": deviceUUID, "OrgID": deviceOrgID, "error": result.Error},
		).Error("Error when deleting devices")
		return result.Error
	}
	s.log.WithFields(log.Fields{
		"host_id": string(eventData.ID),
	}).Debug("Deleting edge device")
	return nil
}

func (s *DeviceService) syncDevicesWithInventory(orgID string) {
	s.log.Debug("Syncing edge and insights inventories")
	// use stored devices to check inventory in chunks and see if we have any stale devices.
	// Delete devices in Edge Inventory that are not in Insights Inventory
	var params *inventory.Params
	var total int64
	// setting limit to a small amount as this is passed as a URL param, URL params have a limit of 2000 chars.
	limit := 30
	offset := 0
	var edgeDevices, devicesToBeDeleted []models.Device

	if res := db.Org(orgID, "").Model(&models.Device{}).Count(&total); res.Error != nil {
		s.log.WithField("error", res.Error.Error()).Error("Error getting device count")
		return
	}

	var insightsCount int64
	var edgeCount int64
	for int64(offset) <= total {
		s.log.WithFields(log.Fields{"offset": int64(offset), "total": total}).Debug("Comparing offset to total")
		if res := db.Org(orgID, "").Debug().Limit(limit).Offset(offset).Find(&edgeDevices); res.Error != nil {
			s.log.WithField("error", res.Error.Error()).Error("Error getting devices in device sync")
			return
		}
		deviceIDS := []string{}
		for _, devices := range edgeDevices {
			deviceIDS = append(deviceIDS, devices.UUID)
			s.log.WithField("host_id", devices.UUID).Debug("Appending device to deviceIDS")
		}
		response, err := s.Inventory.ReturnDeviceListByID(deviceIDS)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error getting device data from inventory in device sync")
		}
		s.log.WithFields(log.Fields{"total": response.Total, "count": response.Count}).Debug("Received device list from Insights")

		edgeCount = int64(len(edgeDevices))
		insightsCount = int64(response.Count)
		s.log.WithFields(log.Fields{"edge_count": edgeCount, "insights_count": insightsCount}).Debug("Comparing inventory counts before delete list compilation")
		if edgeCount > insightsCount {
			s.log.WithFields(log.Fields{"edge_count": edgeCount, "insights_count": insightsCount}).Debug("Inventory counts do not match")
			type void struct{}
			var nothing void
			// discover which devices need to be deleted
			inventoryDeviceSet := make(map[string]void)
			for _, invDevice := range response.Result {
				inventoryDeviceSet[invDevice.ID] = nothing
			}
			for _, edgeDevice := range edgeDevices {
				if _, exists := inventoryDeviceSet[edgeDevice.UUID]; !exists {
					devicesToBeDeleted = append(devicesToBeDeleted, edgeDevice)
					s.log.WithField("host_id", edgeDevice.UUID).Debug("Appending device to devicesToBeDeleted")
				}
			}
		}
		offset += limit
	}

	// Delete invalid devices
	s.log.WithFields(log.Fields{"edge_count": edgeCount, "insights_count": insightsCount, "devicesToBeDeleted size": len(devicesToBeDeleted)}).Debug("Comparing inventory counts before device deletion")

	if len(devicesToBeDeleted) > 0 {
		s.log.WithFields(log.Fields{"edge_count": edgeCount, "insights_count": insightsCount}).Debug("Inventory counts do not match. Going through delete list")

		if feature.DeviceSyncDelete.IsEnabled() {
			for _, device := range devicesToBeDeleted {
				s.log.WithFields(
					log.Fields{"host_id": device.UUID, "OrgID": device.OrgID},
				).Debug("Deleting device")

				if result := db.DB.Debug().Delete(&device); result.Error != nil {
					s.log.WithFields(
						log.Fields{"host_id": device.UUID, "OrgID": device.OrgID, "error": result.Error},
					).Error("Error when deleting device in device sync")
				}
			}
		}
	}

	// Get the count of our db and from inventory and compare them
	// If they are the same, sync is done
	// If not, we have missed some "create" events from inventory, we need to discover and add these devices to our DB
	if res := db.Org(orgID, "").Debug().Model(&models.Device{}).Count(&total); res.Error != nil {
		s.log.WithField("error", res.Error.Error()).Error("Error getting device count")
		return
	}
	inventoryResponse, err := s.Inventory.ReturnDevices(params)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error retrieving devices from inventory for sync")
		return
	}

	edgeCount = total
	insightsCount = int64(inventoryResponse.Total)
	s.log.WithFields(log.Fields{"edge_count": edgeCount, "insights_count": insightsCount}).Debug("Comparing inventory counts before syncInventoryWithDevices()")
	if int64(inventoryResponse.Total) != total {
		s.log.WithFields(log.Fields{"edge_count": edgeCount, "insights_count": insightsCount, "orgID": orgID}).Debug("Sync syncDevicesWithInventory complete but db and inventory still dont match, continuing sync")
		s.syncInventoryWithDevices(orgID)
	}
	s.log.WithField("orgID", orgID).Debug("Sync syncDevicesWithInventory complete for orgID")
}

func (s *DeviceService) syncInventoryWithDevices(orgID string) {
	// We have missed one or more create device events from inventory
	// Go through this users inventory devices in chunks and check they are in our DB.
	// If not, add them
	var params inventory.Params
	// setting limit to a small amount as this is passed as a URL param, URL params have a limit of 2000 chars.
	limit := 30
	params.PerPage = strconv.Itoa(limit)
	page := 1
	params.Page = strconv.Itoa(page)
	searchInventory := true

	var insightsCount int64
	var edgeCount int64
	for searchInventory {
		inventoryDevices, err := s.Inventory.ReturnDevices(&params)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error retrieving devices from inventory for sync")
			return
		}
		var inveDeviceIds []string
		for _, inDevice := range inventoryDevices.Result {
			inveDeviceIds = append(inveDeviceIds, inDevice.ID)
		}

		var dbDevices []models.Device
		if res := db.Org(orgID, "").Debug().Where("UUID IN ?", inveDeviceIds).Find(&dbDevices); res.Error != nil {
			s.log.WithField("error", res.Error.Error()).Error("Error getting devices in device sync")
			return
		}

		// check to see that all requested inventory devices are in the db
		edgeCount = int64(len(dbDevices))
		insightsCount = int64(len(inventoryDevices.Result))
		s.log.WithFields(log.Fields{"edge_count": edgeCount, "insights_count": insightsCount}).Debug("Comparing inventory counts before device add loop")

		if edgeCount < insightsCount {
			s.log.WithFields(log.Fields{"edge_count": len(dbDevices), "insights_count": len(inventoryDevices.Result)}).Debug("Inventory counts do not match")

			// discover which inventory device is missing and add it.
			// make a set of db services
			type void struct{}
			var nothing void
			dbDeviceSet := make(map[string]void)
			for _, device := range dbDevices {
				dbDeviceSet[device.UUID] = nothing
			}
			// using the set, discover which inventory device is missing and add it
			for _, inDevice := range inventoryDevices.Result {
				if _, exists := dbDeviceSet[inDevice.ID]; !exists {
					var deps []RpmOSTreeDeployment

					for _, dep := range inDevice.Ostree.RpmOstreeDeployments {
						idep := RpmOSTreeDeployment{
							Booted:   dep.Booted,
							Checksum: dep.Checksum,
						}
						deps = append(deps, idep)
					}
					profile := systemProfile{
						HostType:             inDevice.Ostree.HostType,
						RpmOSTreeDeployments: deps,
						RHCClientID:          inDevice.Ostree.RHCClientID,
					}
					iHost := host{
						ID:            inDevice.ID,
						Name:          inDevice.DisplayName,
						OrgID:         inDevice.OrgID,
						Updated:       models.EdgeAPITime{Time: time.Now(), Valid: true},
						SystemProfile: profile,
					}
					createEvent := PlatformInsightsCreateUpdateEventPayload{
						Type: InventoryEventTypeCreated,
						Host: iHost,
					}

					if feature.DeviceSyncCreate.IsEnabled() {
						s.log.WithField("deviceUUID", inDevice.ID).Debug("Passing fake device event to be added")
						s.platformInventoryCreateEventHelper(createEvent)
					}
				}
			}
		}
		// check end condition
		if inventoryDevices.Count < limit {
			searchInventory = false
		}
		// increment
		page++
		params.Page = strconv.Itoa(page)
	}
	s.log.WithField("orgID", orgID).Debug("Sync complete for orgID")

	// Get the count of our db and from inventory and compare them
	// If they are the same, sync is done
	// If not, sync failed
	var total int64
	if res := db.Org(orgID, "").Debug().Model(&models.Device{}).Count(&total); res.Error != nil {
		s.log.WithField("error", res.Error.Error()).Error("Error getting device count")
		return
	}
	var params2 *inventory.Params
	inventoryResponse, err := s.Inventory.ReturnDevices(params2)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error retrieving devices from inventory for sync")
		return
	}

	edgeCount = total
	insightsCount = int64(inventoryResponse.Total)
	s.log.WithFields(log.Fields{"edge_count": edgeCount, "insights_count": insightsCount}).Debug("Comparing inventory counts after sync")
	if edgeCount != insightsCount {
		s.log.WithFields(log.Fields{"edge_count": edgeCount, "insights_count": insightsCount, "orgID": orgID}).Error("Sync failed. Inventory counts do not match")
	}
}

func (s *DeviceService) CanUpdate(currentDistribution string, updDistribution string) bool {
	if config.DistributionsRefs[currentDistribution] == config.DistributionsRefs[updDistribution] {
		return true
	}
	updPkgs := config.DistributionsPackages[updDistribution]
	for i, pkg := range config.DistributionsPackages[currentDistribution] {
		if updPkgs[i] == pkg {
			return true
		}
	}
	return false
}
