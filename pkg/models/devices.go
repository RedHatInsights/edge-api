// FIXME: golangci-lint
// nolint:govet,revive
package models

import (
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// EdgeDevice is the entity that represents and Edge Device
// It is a combination of the data of a Device owned by Inventory API
// and the Device data saved on Edge API
type EdgeDevice struct {
	*Device
	DeviceName string // TODO: Can be deleted when we start saving the name field on the Device object
	LastSeen   string // TODO: Can be deleted when we start saving lastseen field on the Device object
	// Booted status is referring to the LastDeployment of this device
	// TODO: Needs to be rethinked when we get to the greenbot epic
	Booted  bool
	Account string
	OrgID   string
}

// DeviceDetails is a Device with Image and Update transactions
// It contains data from multiple tables on the database
type DeviceDetails struct {
	Device             EdgeDevice           `json:"Device,omitempty"`
	Image              *ImageInfo           `json:"ImageInfo"`
	UpdateTransactions *[]UpdateTransaction `json:"UpdateTransactions,omitempty"`
	DevicesGroups      *[]DeviceGroup       `json:"DevicesGroups,omitempty"`
	Updating           *bool                `json:"DeviceUpdating,omitempty"`
}

// DeviceDetailsList is the list of devices with details from Inventory and Edge API
type DeviceDetailsList struct {
	Total   int             `json:"total"`
	Count   int             `json:"count"`
	Devices []DeviceDetails `json:"data"`
}

// DeviceViewList is the list of devices for a given account, formatted for the UI
type DeviceViewList struct {
	Total             int64        `json:"total"`
	Devices           []DeviceView `json:"devices"`
	EnforceEdgeGroups bool         `json:"enforce_edge_groups"` // Whether to enforce the edge groups usage
}

// DeviceView is the device information needed for the UI
type DeviceView struct {
	DeviceID         uint                `json:"DeviceID"`
	DeviceName       string              `json:"DeviceName"`
	DeviceUUID       string              `json:"DeviceUUID"`
	ImageID          uint                `json:"ImageID"`
	ImageName        string              `json:"ImageName"`
	LastSeen         EdgeAPITime         `json:"LastSeen"`
	UpdateAvailable  bool                `json:"UpdateAvailable"`
	Status           string              `json:"Status"`
	ImageSetID       uint                `json:"ImageSetID"`
	DeviceGroups     []DeviceDeviceGroup `json:"DeviceGroups"`
	DispatcherStatus string              `json:"DispatcherStatus"`
	DispatcherReason string              `json:"DispatcherReason"`
	GroupName        string              `json:"GroupName"` // the inventory group name
	GroupUUID        string              `json:"GroupUUID"` // the inventory group id
}

// DeviceDeviceGroup is a struct of device group name and id needed for DeviceView
type DeviceDeviceGroup struct {
	ID   uint
	Name string
}

const (
	// DeviceViewStatusRunning is for when a device is in a normal state
	DeviceViewStatusRunning = "RUNNING"
	// DeviceViewStatusUpdating is for when a update is sent to a device
	DeviceViewStatusUpdating = "UPDATING"
	// DeviceViewStatusUpdateAvail is for when a update available for a device
	DeviceViewStatusUpdateAvail = "UPDATE AVAILABLE"
)

// Device is a record of Edge Devices referenced by their UUID as per the
// cloud.redhat.com Inventory.
//
//	Connected refers to the devices Cloud Connector state, 0 is unavailable
//	and 1 is reachable.
//
// THEEDGE-1921 created 2 temporary indexes to address production issue
type Device struct {
	Model
	UUID                  string               `gorm:"index" json:"UUID"`
	AvailableHash         string               `json:"AvailableHash,omitempty"`
	RHCClientID           string               `json:"RHCClientID"`
	Connected             bool                 `gorm:"default:true" json:"Connected"`
	Name                  string               `json:"Name"`
	LastSeen              EdgeAPITime          `json:"LastSeen"`
	CurrentHash           string               `json:"CurrentHash,omitempty"`
	Account               string               `gorm:"index" json:"Account"`
	OrgID                 string               `json:"org_id" gorm:"index;<-:create"`
	ImageID               uint                 `json:"ImageID" gorm:"index"`
	UpdateAvailable       bool                 `json:"UpdateAvailable"`
	DevicesGroups         []DeviceGroup        `faker:"-" gorm:"many2many:device_groups_devices;save_association:false" json:"DevicesGroups"`
	UpdateTransaction     *[]UpdateTransaction `faker:"-" gorm:"many2many:updatetransaction_devices;" json:"UpdateTransaction"`
	GroupName             string               `json:"group_name"` // the inventory group name
	GroupUUID             string               `json:"group_uuid"` // the inventory group id
	SubscriptionManagerId string               `json:"SubscriptionManagerId"`
}

// BeforeCreate method is called before creating devices, it make sure org_id is not empty
func (d *Device) BeforeCreate(tx *gorm.DB) error {
	if d.OrgID == "" {
		log.Error("device do not have an org_id")
		return ErrOrgIDIsMandatory
	}
	var device Device
	result := tx.First(&device, "UUID = ? ", d.UUID).Unscoped()
	if result.RowsAffected > 0 {
		log.Errorf("device uuid %v already exists %v", d.UUID, result.RowsAffected)
		return ErrDeviceExists
	}

	return nil
}
