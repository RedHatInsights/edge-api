package models

// CreateDeviceGroupAPI is the /device-group POST endpoint struct for openapi.json auto-gen
type CreateDeviceGroupAPI struct {
	Name    string                    `json:"name" example:"my-device-group"` // the device group name
	Type    string                    `json:"type" example:"static"`          // the device group type
	Devices []DeviceForDeviceGroupAPI `json:"DevicesAPI,omitempty"`           // Devices of group

} // CreateDeviceGroup

// DeviceImageInfoAPI is a record of group with the current images running on the device
type DeviceImageInfoAPI struct {
	Name            string `json:"name" example:"my-image-name"`    // image name
	Version         int    `json:"version" example:"1"`             // image version
	Distribution    string `json:"distribution" example:"RHEL-92"`  // image distribution
	UpdateAvailable bool   `json:"update Available" example:"true"` // image update availability
	CommitID        uint   `json:"commitID" example:"2"`            // image commit id
} // DeviceImage

// DeviceForDeviceGroupAPI is a device array expected to create groups with devices
type DeviceForDeviceGroupAPI struct {
	UUID string `json:"UUID" example:"68485bb8-6427-40ad-8711-93b6a5b4deac"` // device uuid
} // Device

// DeviceGroupDevicesAPI is the /device-group return endpoint struct for openapi.json auto-gen
type DeviceGroupDevicesAPI struct {
	Account     string `json:"Account" example:"1000"`         // account that the device group belongs to
	OrgID       string `json:"org_id" example:"2000"`          // orgId that the device group belongs to
	Name        string `json:"Name" example:"my-device-group"` // device group name
	Type        string `json:"Type" example:"static"`          // device group type
	ValidUpdate bool   `json:"ValidUpdate" example:"false"`    // device group validation to update devices
} // DeviceGroup

// CheckGroupNameParamAPI is the /checkName parameter to check uniqueness
type CheckGroupNameParamAPI struct {
	Name string `json:"Name" example:"my-device-group"` // device group name
}

// PutGroupNameParamAPI is the parameter to check update device group
type PutGroupNameParamAPI struct {
	Name string `json:"Name" example:"my-device-group"` // device group name
	Type string `json:"Type" example:"static"`          // device group type
}

// DeviceGroupViewResponseAPI is the detail return of /view endpoint
type DeviceGroupViewResponseAPI struct {
	Total   int                      `json:"Total" example:"10"` // count of devices
	Devices ImageSetImagePackagesAPI `json:"Devices"`            // all devices in a group
}

// DeviceGroupViewAPI is the return of /view endpoint
type DeviceGroupViewAPI struct {
	DeviceGroup DeviceGroup                `json:"DeviceGroup"` // device group data
	DevicesView DeviceGroupViewResponseAPI `json:"DevicesView"` // device group detail
}

// PostDeviceForDeviceGroupAPI is the expected values to add device to a group
type PostDeviceForDeviceGroupAPI struct {
	Name string `json:"Name" example:"localhost"`                            // device name
	UUID string `json:"UUID" example:"68485bb8-6427-40ad-8711-93b6a5b4deac"` // device uuid
}
