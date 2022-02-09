package models

// EdgeDevice is the entity that represents and Edge Device
// It is a combination of the data of a Device owned by Inventory API
// and the Device data saved on Edge API
type EdgeDevice struct {
	*Device
	DeviceName string
	LastSeen   string
	// Booted status is referring to the LastDeployment of this device
	// TODO: Needs to be rethinked when we get to the greenbot epic
	Booted bool
}

// DeviceDetails is a Device with Image and Update transactions
// It contains data from multiple tables on the database
type DeviceDetails struct {
	Device             EdgeDevice           `json:"Device,omitempty"`
	Image              *ImageInfo           `json:"ImageInfo"`
	UpdateTransactions *[]UpdateTransaction `json:"UpdateTransactions,omitempty"`
}

// DeviceDetailsList is the list of devices with details from Inventory and Edge API
type DeviceDetailsList struct {
	Total   int             `json:"total"`
	Count   int             `json:"count"`
	Devices []DeviceDetails `json:"data"`
}

// Device is a record of Edge Devices referenced by their UUID as per the
// cloud.redhat.com Inventory.
//
//	Connected refers to the devices Cloud Connector state, 0 is unavailable
//	and 1 is reachable.
type Device struct {
	Model
	UUID          string      `json:"UUID"`
	AvailableHash string      `json:"AvailableHash,omitempty"`
	RHCClientID   string      `json:"RHCClientID"`
	Connected     bool        `gorm:"default:true" json:"Connected"`
	Name          string      `json:"Name"`
	LastSeen      EdgeAPITime `json:"LastSeen"`
	CurrentHash   string      `json:"CurrentHash,omitempty"`
	Account       string      `json:"Account"`
}
