package models

// EdgeDeviceAPI is the entity that represents and Edge Device
// It is a combination of the data of a Device owned by Inventory API
// and the Device data saved on Edge API
type EdgeDeviceAPI struct {
	UUID              string               `json:"UUID"                     example:"ba-93ba-49a3-b4ae-a6c8acdc4736"` // UUID of edge device
	AvailableHash     string               `json:"AvailableHash,omitempty"  example:"true"`                           // Hash that available
	RHCClientID       string               `json:"RHCClientID"`                                                       // RHC Client ID
	Connected         bool                 `json:"Connected"                example:"true"`                           // If Device connect of not
	Name              string               `json:"Name"`                                                              // Name of Edge Device
	LastSeen          EdgeAPITime          `json:"LastSeen"`                                                          // Last datetime that device updated
	CurrentHash       string               `json:"CurrentHash,omitempty"`
	ImageID           uint                 `json:"ImageID"                  example:"12834"` // image id of device
	UpdateAvailable   bool                 `json:"UpdateAvailable"          example:"true"`  // If there is update available
	DevicesGroups     []DeviceGroupAPI     `json:"DevicesGroups"`                            // device groups
	UpdateTransaction *[]UpdateTransaction `json:"UpdateTransaction"`
	DeviceName        string               `example:"test_device_api_static"` // The device name
	Booted            bool                 `example:"true"`                   // Booted status is referring to the LastDeployment of this device
}

// DeviceAPI is entity for device
type DeviceAPI struct {
	UUID              string               `json:"UUID"                     example:"ba-93ba-49a3-b4ae-a6c8acdc4736"` // UUID of edge device
	AvailableHash     string               `json:"AvailableHash,omitempty"  example:"true"`                           // Hash that available
	RHCClientID       string               `json:"RHCClientID"`                                                       // RHC Client ID
	Connected         bool                 `json:"Connected"                example:"true"`                           // If Device connect of not
	Name              string               `json:"Name"                     example:"device_name"`                    // Name of device
	LastSeen          string               // Last datetime that device updated
	CurrentHash       string               `json:"CurrentHash,omitempty"`
	ImageID           uint                 `json:"ImageID"                  example:"12834"` // image id of device`
	UpdateAvailable   bool                 `json:"UpdateAvailable"          example:"true"`  // If there is Update available
	DevicesGroups     []DeviceGroupAPI     // device groups
	UpdateTransaction *[]UpdateTransaction `json:"UpdateTransaction"`
	DeviceName        string
	Booted            bool `example:"true"` // Booted status is referring to the LastDeployment of this device
}

// DeviceGroupAPI is a record of Edge Devices Groups
// Account is the account associated with the device group
// Type is the device group type and must be "static" or "dynamic"
type DeviceGroupAPI struct {
	Name        string      `json:"Name"        example:"device_group name"` // The device group name`
	Type        string      `json:"Type"        example:"static"`            // The device group type``
	Devices     []DeviceAPI `json:"Devices"`                                 // Devices that belong to the group
	ValidUpdate bool        `json:"ValidUpdate" example:"true"`              // indicate if the update is valid
}

// DispatchRecordAPI represents the combination of a Playbook Dispatcher (https://github.com/RedHatInsights/playbook-dispatcher),
// of a PlaybookURL, a pointer to a Device, and the status.
// This is used within UpdateTransaction for accounting purposes.
type DispatchRecordAPI struct {
	PlaybookURL          string     `json:"PlaybookURL"`
	DeviceID             uint       `json:"DeviceID"        example:"1913277"` // ID of device
	Device               *DeviceAPI `json:"Device"`
	Status               string     `json:"Status"          example:"SUCCESS"` // Status of device
	Reason               string     `json:"Reason"`
	PlaybookDispatcherID string     `json:"PlaybookDispatcherID"`
}

// UpdateTransactionAPI represents the combination of an OSTree commit and a set of Inventory
type UpdateTransactionAPI struct {
	Commit          *Commit             `json:"Commit"`
	CommitID        uint                `json:"CommitID"    example:"1754"`       // Commit ID of device
	OldCommits      []Commit            `json:"OldCommits"`                       // Old Commit ID if the device has one
	Devices         []DeviceAPI         `json:"Devices"`                          // List of Devices
	Tag             string              `json:"Tag"         example:"device_tag"` // Tag og Device if device has one
	Status          string              `json:"Status"      example:"SUCCESS"`    // Status of device
	RepoID          *uint               `json:"RepoID"      example:"2256"`       // Repo ID
	Repo            *Repo               `json:"Repo"`
	ChangesRefs     bool                `json:"ChangesRefs" example:"false"`
	DispatchRecords []DispatchRecordAPI `json:"DispatchRecords"`
}

// DeviceDetailsAPI is a Device with Image and Update transactions
// It contains data from multiple tables on the database
type DeviceDetailsAPI struct {
	Device             EdgeDeviceAPI           `json:"Device,omitempty"` // Details of device like name, LastSeen and more
	Image              *ImageInfo              `json:"ImageInfo"`        // Information of device's image
	UpdateTransactions *[]UpdateTransactionAPI `json:"UpdateTransactions,omitempty"`
	DevicesGroups      *[]DeviceGroupAPI       `json:"DevicesGroups,omitempty"`                 // Device's groups
	Updating           *bool                   `json:"DeviceUpdating,omitempty" example:"true"` // If there is update to device
}

// DeviceDetailsListAPI is the list of devices with details from Inventory and Edge API
type DeviceDetailsListAPI struct {
	Total   int                `json:"total"  example:"40"` // total number of device
	Count   int                `json:"count"  example:"40"` // total number of device
	Devices []DeviceDetailsAPI `json:"data"`                // List of Devices
}

// FilterByDevicesAPI is the deviceUUID list
type FilterByDevicesAPI struct {
	DevicesUUID []string `json:"devices_uuid"` // Devices UUID
}

// DeviceViewAPI is the device information needed for the UI
type DeviceViewAPI struct {
	DeviceID         uint             `json:"DeviceID"        example:"1913277"`             // ID of device
	DeviceName       string           `json:"DeviceName"      example:"device_name"`         // Name of device
	DeviceUUID       string           `json:"DeviceUUID"      example:"a-8bdf-a21accb24925"` // UUID of Device
	ImageID          uint             `json:"ImageID"         example:"323241"`              // ID of image
	ImageName        string           `json:"ImageName"       example:"image_name"`          // Name of image
	LastSeen         EdgeAPITime      `json:"LastSeen"`                                      // Last datetime that device updated
	UpdateAvailable  bool             `json:"UpdateAvailable" example:"true"`                // indicate if there is update to device
	Status           string           `json:"Status"          example:"SUCCESS"`             // Status of device
	ImageSetID       uint             `json:"ImageSetID"      example:"33341"`               // ID of image set
	DeviceGroups     []DeviceGroupAPI `json:"DeviceGroups"`                                  // Device's groups
	DispatcherStatus string           `json:"DispatcherStatus"`                              // Status of Dispatch
	DispatcherReason string           `json:"DispatcherReason"`                              // Reason of Dispatch
	GroupName        string           `json:"GroupName"`                                     // the inventory group name
	GroupUUID        string           `json:"GroupUUID"`                                     // the inventory group id
}

// DeviceViewListAPI is the list of devices for a given account, formatted for the UI
type DeviceViewListAPI struct {
	Total             int64           `json:"total" example:"40"`  // Total number of device
	Devices           []DeviceViewAPI `json:"devices"`             // List of Devices
	EnforceEdgeGroups bool            `json:"enforce_edge_groups"` // Whether to enforce the edge groups usage
}

// DeviceViewListResponseAPI is the struct returned by the devices view endpoint
type DeviceViewListResponseAPI struct {
	Data  DeviceViewListAPI `json:"data"`               // The devices view data
	Count int64             `json:"count" example:"40"` // The overall number of devices
}
