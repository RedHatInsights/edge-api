package services

import (
	"context"
	"encoding/json"

	version "github.com/knqyf263/go-rpm-version"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	GetLatestCommitFromDevices(account string, devicesUUID []string) (uint, error)
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
	Host struct {
		ID            string `json:"id"`
		Name          string `json:"display_name"`
		Account       string `json:"account"`
		InsightsID    string `json:"insights_id"`
		SystemProfile struct {
			HostType             string                `json:"host_type"`
			RpmOSTreeDeployments []RpmOSTreeDeployment `json:"rpm_ostree_deployments"`
		} `json:"system_profile"`
	} `json:"host"`
}

// PlatformInsightsDeleteEventPayload is the body of the delete event found on the platform.inventory.events kafka topic.
type PlatformInsightsDeleteEventPayload struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Account string `json:"account"`
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

	//Load from device DB the groups info and add to the new struct
	err := db.DB.Model(&device).Association("DevicesGroups").Find(&device.DevicesGroups)
	if err != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error finding associated devicegroups for device")
		return nil, new(DeviceGroupNotFound)
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
		return nil, new(DeviceNotFoundError)
	}

	//Load from device DB the groups info and add to the new struct
	err := db.DB.Model(&device).Association("DevicesGroups").Find(&device.DevicesGroups)
	if err != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error finding associated devicegroups for device")
		return nil, new(DeviceGroupNotFound)
	}
	return &device, nil
}

// GetDeviceDetails provides details for a given Device by going to inventory API and trying to also merge with the information on our database
func (s *DeviceService) GetDeviceDetails(device inventory.Device) (*models.DeviceDetails, error) {
	s.log = s.log.WithField("deviceUUID", device.ID)
	s.log.Info("Get device by uuid")

	// Get device's running image
	imageInfo, err := s.GetDeviceImageInfo(device)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Could not find information about the running image on the device")
		return nil, err
	}
	// Get device on Edge API, not on inventory
	databaseDevice, err := s.GetDeviceByUUID(device.ID)

	// In order to have an update transaction for a device it must be a least created
	var updates *[]models.UpdateTransaction
	if databaseDevice != nil {
		updates, err = s.UpdateService.GetUpdateTransactionsForDevice(databaseDevice)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Could not find information about updates for this device")
			return nil, err
		}
	}
	if err != nil {
		s.log.Info("Could not find device on the devices table yet - returning just the data from inventory")
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
			Account:    device.Account,
		},
		Image:              imageInfo,
		UpdateTransactions: updates,
		//Given we are concat the info from inventory with our db, we need to add this field to the struct to be able to see the result
		DevicesGroups: &databaseDevice.DevicesGroups,
	}
	return details, nil
}

// GetDeviceDetailsByUUID provides details for a given Device UUID by going to inventory API and trying to also merge with the information on our database
func (s *DeviceService) GetDeviceDetailsByUUID(deviceUUID string) (*models.DeviceDetails, error) {
	s.log = s.log.WithField("deviceUUID", deviceUUID)
	resp, err := s.Inventory.ReturnDevicesByID(deviceUUID)
	if err != nil || resp.Total != 1 {
		return nil, new(DeviceNotFoundError)
	}
	return s.GetDeviceDetails(resp.Result[len(resp.Result)-1])
}

// GetUpdateAvailableForDeviceByUUID returns if it exists an update for the current image at the device given its UUID.
func (s *DeviceService) GetUpdateAvailableForDeviceByUUID(deviceUUID string, latest bool) ([]models.ImageUpdateAvailable, error) {
	s.log = s.log.WithField("deviceUUID", deviceUUID)
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
	).Joins("Commit").Order("Images.updated_at desc")

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
			s.log.WithField("error", err.Error()).Error("Could not find CustomPackages")
			return nil, err
		}
		var delta models.ImageUpdateAvailable
		diff := GetDiffOnUpdate(currentImage, upd)
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
	s.log = s.log.WithField("deviceUUID", deviceUUID)
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
	account, err := common.GetAccountFromContext(s.ctx)
	if err != nil {
		return nil, err
	}
	devicesUUIDs := make([]string, 0, len(inventoryDevices.Result))
	for _, device := range inventoryDevices.Result {
		devicesUUIDs = append(devicesUUIDs, device.ID)
	}
	var storedDevices []models.Device
	if res := db.DB.Where("account = ? AND uuid IN ?", account, devicesUUIDs).Find(&storedDevices); res.Error != nil {
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

		//Load from device DB the groups info and add to the new struct
		var storeDevice models.Device
		// Don't throw error if device not found
		db.DB.Where("id=?", dbDeviceID).First(&storeDevice)

		err := db.DB.Model(&storeDevice).Association("UpdateTransaction").Find(&storeDevice.UpdateTransaction)

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

		if len(storeDevice.UpdateTransaction) > 0 {
			last := storeDevice.UpdateTransaction[len(storeDevice.UpdateTransaction)-1]
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
				Account:           device.Account,
				DevicesGroups:     storeDevice.DevicesGroups,
				UpdateTransaction: storeDevice.UpdateTransaction,
			},
			Account:    device.Account,
			DeviceName: device.DisplayName,
			LastSeen:   device.LastSeen,
		}
		dd.Updating = &deviceUpdating
		lastDeployment := s.GetDeviceLastDeployment(device)
		if lastDeployment != nil {
			dd.Device.Booted = lastDeployment.Booted
		}
		s.log.WithField("deviceID", device.ID).Info("Getting image info for device...")
		imageInfo, err := s.GetDeviceImageInfo(device)
		if err != nil {
			dd.Image = nil
		} else if imageInfo != nil {
			dd.Image = imageInfo
		}
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
func (s *DeviceService) SetDeviceUpdateAvailability(account string, deviceID uint) error {

	var device models.Device
	if result := db.DB.Where(models.Device{Account: account}).First(&device, deviceID); result.Error != nil {
		return result.Error
	}
	if device.ImageID == 0 {
		return new(DeviceHasImageUndefined)
	}

	// get the device image
	var deviceImage models.Image
	if result := db.DB.Where(models.Image{Account: account}).First(&deviceImage, device.ImageID); result.Error != nil {
		return result.Error
	}

	// check for updates , find if any later images exists
	var updateImages []models.Image
	if result := db.DB.Select("id").Where("account = ? AND image_set_id = ? AND status = ? AND created_at > ?",
		deviceImage.Account, deviceImage.ImageSetID, models.ImageStatusSuccess, deviceImage.CreatedAt).Find(&updateImages); result.Error != nil {
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
		return new(DeviceNotFoundError)
	}

	// Get the last rpmOSTree deployment commit checksum
	deployments := eventData.Host.SystemProfile.RpmOSTreeDeployments
	if len(deployments) == 0 {
		return new(ImageNotFoundError)
	}
	CommitCheck := deployments[0].Checksum
	// Get the related commit image
	var deviceImage models.Image
	if result := db.DB.Select("images.id").Joins("JOIN commits ON commits.id = images.commit_id").Where(
		"images.account = ? AND commits.os_tree_commit = ? ", device.Account, CommitCheck).First(&deviceImage); result.Error != nil {
		return result.Error
	}

	device.ImageID = deviceImage.ID

	if result := db.DB.Save(device); result.Error != nil {
		return result.Error
	}

	return s.SetDeviceUpdateAvailability(device.Account, device.ID)
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
		//s.log.Debug("Skipping kafka message - Platform Insights Inventory message host type is not edge and event type is not updated")
		return nil
	}
	deviceUUID := eventData.Host.ID
	deviceAccount := eventData.Host.Account
	deviceName := eventData.Host.Name
	device, err := s.GetDeviceByUUID(deviceUUID)
	if err != nil {
		// create a new device if it does not exist.
		var newDevice = models.Device{
			UUID:        deviceUUID,
			RHCClientID: eventData.Host.InsightsID,
			Account:     deviceAccount,
			Name:        deviceName,
		}
		if result := db.DB.Create(&newDevice); result.Error != nil {
			s.log.WithFields(log.Fields{"host_id": deviceUUID, "error": result.Error}).Error("Error creating device")
			return result.Error
		}
		s.log.WithFields(log.Fields{"host_id": deviceUUID}).Debug("Device account created")
		return s.processPlatformInventoryEventUpdateDevice(eventData)
	}
	if device.Account == "" {
		// Update account if undefined
		device.Account = deviceAccount
		if result := db.DB.Save(device); result.Error != nil {
			s.log.WithFields(log.Fields{"host_id": deviceUUID, "error": result.Error}).Error("Error updating device account")
			return result.Error
		}
		s.log.WithFields(log.Fields{"host_id": deviceUUID}).Debug("Device account updated")
	}

	return s.processPlatformInventoryEventUpdateDevice(eventData)
}

// GetDevicesCount get the device groups account records count from the database
func (s *DeviceService) GetDevicesCount(tx *gorm.DB) (int64, error) {
	account, err := common.GetAccountFromContext(s.ctx)
	if err != nil {
		return 0, err
	}

	if tx == nil {
		tx = db.DB
	}

	var count int64

	res := tx.Model(&models.Device{}).Where("account = ?", account).Count(&count)

	if res.Error != nil {
		s.log.WithField("error", res.Error.Error()).Error("Error getting device groups count")
		return 0, res.Error
	}

	return count, nil
}

// GetDevicesView returns a list of EdgeDevices for a given account.
func (s *DeviceService) GetDevicesView(limit int, offset int, tx *gorm.DB) (*models.DeviceViewList, error) {
	account, err := common.GetAccountFromContext(s.ctx)
	if err != nil {
		return nil, err
	}

	if tx == nil {
		tx = db.DB
	}

	var storedDevices []models.Device
	if res := tx.Limit(limit).Offset(offset).Where("account = ?", account).Preload("UpdateTransaction").Preload("DevicesGroups").Find(&storedDevices); res.Error != nil {
		return nil, res.Error
	}

	type deviceGroupInfo struct {
		info []models.DeviceDeviceGroup
	}
	deviceToGroupMap := make(map[uint]*deviceGroupInfo)
	for _, device := range storedDevices {
		for _, deviceGroups := range device.DevicesGroups {
			if _, ok := deviceToGroupMap[device.ID]; ok {
				temp := models.DeviceDeviceGroup{ID: deviceGroups.ID, Name: deviceGroups.Name}
				deviceToGroupMap[device.ID].info = append(deviceToGroupMap[device.ID].info, temp)
			}
		}
	}

	type neededImageInfo struct {
		Name       string
		Status     string
		ImageSetID uint
	}
	// create a map of unique image id's. We dont want to look of a given image id more than once.
	setOfImages := make(map[uint]*neededImageInfo)
	for _, devices := range storedDevices {
		var status = models.DeviceViewStatusRunning
		if devices.UpdateTransaction != nil && len(*devices.UpdateTransaction) > 0 {
			updateStatus := (*devices.UpdateTransaction)[len(*devices.UpdateTransaction)-1].Status
			if updateStatus == models.UpdateStatusBuilding {
				status = models.DeviceViewStatusUpdating
			}
		}
		if devices.ImageID != 0 {
			setOfImages[devices.ImageID] = &neededImageInfo{Name: "", Status: status, ImageSetID: 0}
		}
	}

	// using the map of unique image ID's, get the corresponding image name and status.
	imagesIDS := []uint{}
	for key := range setOfImages {
		imagesIDS = append(imagesIDS, key)
	}

	var images []models.Image
	if result := db.DB.Where("account = ? AND id IN (?) AND image_set_id IS NOT NULL", account, imagesIDS).Find(&images); result.Error != nil {
		return nil, result.Error
	}

	for _, image := range images {
		status := models.DeviceViewStatusRunning
		if setOfImages[image.ID] != nil {
			status = setOfImages[image.ID].Status
		}
		setOfImages[image.ID] = &neededImageInfo{Name: image.Name, Status: status, ImageSetID: *image.ImageSetID}
	}

	// build the return object
	// TODO: add device group info
	returnDevices := []models.DeviceView{}
	for _, device := range storedDevices {
		var imageName string
		var imageStatus string
		var imageSetID uint
		var deviceGroups []models.DeviceDeviceGroup
		if _, ok := setOfImages[device.ImageID]; ok {
			imageName = setOfImages[device.ImageID].Name
			imageStatus = setOfImages[device.ImageID].Status
			imageSetID = setOfImages[device.ImageID].ImageSetID
		}
		if _, ok := deviceToGroupMap[device.ID]; ok {
			deviceGroups = deviceToGroupMap[device.ID].info
		}
		currentDeviceView := models.DeviceView{
			DeviceID:        device.ID,
			DeviceName:      device.Name,
			DeviceUUID:      device.UUID,
			ImageID:         device.ImageID,
			ImageName:       imageName,
			LastSeen:        device.LastSeen.Time.String(),
			UpdateAvailable: device.UpdateAvailable,
			Status:          imageStatus,
			ImageSetID:      imageSetID,
			DeviceGroups:    deviceGroups,
		}
		returnDevices = append(returnDevices, currentDeviceView)
	}

	list := &models.DeviceViewList{
		Devices: returnDevices,
		Total:   len(storedDevices),
	}
	return list, nil
}

// GetLatestCommitFromDevices fetches the commitID from the latest Device Image
func (s *DeviceService) GetLatestCommitFromDevices(account string, devicesUUID []string) (uint, error) {
	var devices []models.Device

	if result := db.DB.Where("account = ? AND uuid IN ?", account, devicesUUID).Find(&devices); result.Error != nil {
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
	if result := db.DB.Where(models.Image{Account: account}).Find(&devicesImage, devicesImageID); result.Error != nil {
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
		return 0, new(DeviceHasMoreThanOneImageSet)
	}

	// check for updates , find if any later images exists to get the commitID
	var updateImages []models.Image
	if result := db.DB.Model(&models.Image{}).Where("account = ? AND image_set_id = ? AND status = ?", account, imageSetID, models.ImageStatusSuccess).Order("version desc").Find(&updateImages); result.Error != nil {
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
		log.WithFields(log.Fields{
			"value": string(message),
		}).Debug("Skipping message - it is not a create message" + err.Error())
	} else {
		if e.Type == InventoryEventTypeCreated && e.Host.SystemProfile.HostType == InventoryHostTypeEdge {
			log.WithFields(log.Fields{
				"host_id": string(e.Host.ID),
				"value":   string(message),
			}).Debug("Saving newly created edge device")
			var newDevice = models.Device{
				UUID:        string(e.Host.ID),
				RHCClientID: string(e.Host.InsightsID),
				Account:     string(e.Host.Account),
				Name:        string(e.Host.Name),
			}
			result := db.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&newDevice)
			if result.Error != nil {
				log.WithFields(log.Fields{
					"host_id": string(e.Host.ID),
					"error":   result.Error,
				}).Error("Error writing Kafka message to DB")
				return result.Error
			}

			return s.processPlatformInventoryEventUpdateDevice(*e)
		}
		log.Debug("Skipping message - not an edge create message from platform insights")
	}
	return nil
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
	deviceAccount := eventData.Account
	var device models.Device
	if result := db.DB.Where(models.Device{Account: deviceAccount, UUID: deviceUUID}).First(&device); result.Error != nil {
		s.log.WithFields(
			log.Fields{"host_id": deviceUUID, "Account": deviceAccount, "error": result.Error},
		).Error("Error retrieving the device")
		return result.Error
	}
	if result := db.DB.Delete(&device); result.Error != nil {
		s.log.WithFields(
			log.Fields{"host_id": deviceUUID, "Account": deviceAccount, "error": result.Error},
		).Error("Error when deleting device")
		return result.Error
	}
	return nil
}
