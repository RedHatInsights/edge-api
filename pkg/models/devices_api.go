package models

// EdgeDeviceAPI is the entity that represents and Edge Device
// It is a combination of the data of a Device owned by Inventory API
// and the Device data saved on Edge API
type EdgeDeviceAPI struct {
	UUID              string      `json:"UUID"`
	AvailableHash     string      `json:"AvailableHash,omitempty"`
	RHCClientID       string      `json:"RHCClientID"`
	Connected         bool        `json:"Connected"`
	Name              string      `json:"Name"`
	LastSeen          EdgeAPITime `json:"LastSeen"`
	CurrentHash       string      `json:"CurrentHash,omitempty"`
	ImageID           uint        `json:"ImageID"`
	UpdateAvailable   bool        `json:"UpdateAvailable"`
	DevicesGroups     []DeviceGroupAPI
	UpdateTransaction *[]UpdateTransaction `json:"UpdateTransaction"`
	DeviceName        string
	Booted            bool // Booted status is referring to the LastDeployment of this device
}

// DeviceAPI is entity for device
type DeviceAPI struct {
	UUID              string      `json:"UUID"`
	AvailableHash     string      `json:"AvailableHash,omitempty"`
	RHCClientID       string      `json:"RHCClientID"`
	Connected         bool        `json:"Connected"`
	Name              string      `json:"Name"`
	LastSeen          EdgeAPITime `json:"LastSeen"`
	CurrentHash       string      `json:"CurrentHash,omitempty"`
	ImageID           uint        `json:"ImageID"`
	UpdateAvailable   bool        `json:"UpdateAvailable"`
	DevicesGroups     []DeviceGroupAPI
	UpdateTransaction *[]UpdateTransaction `json:"UpdateTransaction"`
	DeviceName        string
	Booted            bool // Booted status is referring to the LastDeployment of this device
}

// DeviceGroupAPI is a record of Edge Devices Groups
// Account is the account associated with the device group
// Type is the device group type and must be "static" or "dynamic"
type DeviceGroupAPI struct {
	Name        string      `json:"Name"`
	Type        string      `json:"Type"`
	Devices     []DeviceAPI `json:"Devices"`
	ValidUpdate bool        `json:"ValidUpdate"`
}

// DispatchRecordAPI represents the combination of a Playbook Dispatcher (https://github.com/RedHatInsights/playbook-dispatcher),
// of a PlaybookURL, a pointer to a Device, and the status.
// This is used within UpdateTransaction for accounting purposes.
type DispatchRecordAPI struct {
	PlaybookURL          string     `json:"PlaybookURL"`
	DeviceID             uint       `json:"DeviceID"`
	Device               *DeviceAPI `json:"Device"`
	Status               string     `json:"Status"`
	Reason               string     `json:"Reason"`
	PlaybookDispatcherID string     `json:"PlaybookDispatcherID"`
}

// UpdateTransactionAPI represents the combination of an OSTree commit and a set of Inventory
type UpdateTransactionAPI struct {
	Commit          *Commit             `json:"Commit"`
	CommitID        uint                `json:"CommitID"`
	OldCommits      []Commit            `json:"OldCommits"`
	Devices         []DeviceAPI         `json:"Devices"`
	Tag             string              `json:"Tag"`
	Status          string              `json:"Status"`
	RepoID          *uint               `json:"RepoID"`
	Repo            *Repo               `json:"Repo"`
	ChangesRefs     bool                `json:"ChangesRefs"`
	DispatchRecords []DispatchRecordAPI `json:"DispatchRecords"`
}

// DeviceDetailsAPI is a Device with Image and Update transactions
// It contains data from multiple tables on the database
type DeviceDetailsAPI struct {
	Device             EdgeDeviceAPI           `json:"Device,omitempty"`
	Image              *ImageInfo              `json:"ImageInfo"`
	UpdateTransactions *[]UpdateTransactionAPI `json:"UpdateTransactions,omitempty"`
	DevicesGroups      *[]DeviceGroupAPI       `json:"DevicesGroups,omitempty"`
	Updating           *bool                   `json:"DeviceUpdating,omitempty"`
}

// DeviceDetailsListAPI is the list of devices with details from Inventory and Edge API
type DeviceDetailsListAPI struct {
	Total   int                `json:"total"`
	Count   int                `json:"count"`
	Devices []DeviceDetailsAPI `json:"data"`
}

// DeviceDeviceGroupAPI is a struct of device group name and id needed for DeviceView
type DeviceDeviceGroupAPI struct {
	ID   uint
	Name string
}

// DeviceViewAPI is the device information needed for the UI
type DeviceViewAPI struct {
	DeviceID         uint                   `json:"DeviceID"`
	DeviceName       string                 `json:"DeviceName"`
	DeviceUUID       string                 `json:"DeviceUUID"`
	ImageID          uint                   `json:"ImageID"`
	ImageName        string                 `json:"ImageName"`
	LastSeen         EdgeAPITime            `json:"LastSeen"`
	UpdateAvailable  bool                   `json:"UpdateAvailable"`
	Status           string                 `json:"Status"`
	ImageSetID       uint                   `json:"ImageSetID"`
	DeviceGroups     []DeviceDeviceGroupAPI `json:"DeviceGroups"`
	DispatcherStatus string                 `json:"DispatcherStatus"`
	DispatcherReason string                 `json:"DispatcherReason"`
}

// DeviceViewListAPI is the list of devices for a given account, formatted for the UI
type DeviceViewListAPI struct {
	Total   int64           `json:"total"`
	Devices []DeviceViewAPI `json:"devices"`
}
