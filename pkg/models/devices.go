package models

// DeviceDetails is a Device with Image and Update transactions
// It contains data from multiple tables on the database
type DeviceDetails struct {
	Device             *Device              `json:"Device,omitempty"`
	Image              *ImageInfo           `json:"ImageInfo"`
	UpdateTransactions *[]UpdateTransaction `json:"UpdateTransactions,omitempty"`
}

// Device is a record of Edge Devices referenced by their UUID as per the
// cloud.redhat.com Inventory.
//
//	Connected refers to the devices Cloud Connector state, 0 is unavailable
//	and 1 is reachable.
type Device struct {
	Model
	UUID        string `json:"UUID"`
	DesiredHash string `json:"DesiredHash"`
	RHCClientID string `json:"RHCClientID"`
	Connected   bool   `gorm:"default:true" json:"Connected"`
}
