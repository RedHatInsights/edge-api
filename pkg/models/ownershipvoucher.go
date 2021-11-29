package models

// OwnershipVoucherData represents the data of an ownership voucher
type OwnershipVoucherData struct {
	Model
	ProtocolVersion uint   `json:"protocol_version"`
	GUID            string `json:"guid"`
	DeviceName      string `json:"device_name"`
	DeviceConnected bool   `json:"device_connected" gorm:"default:false"`
	DeviceUUID      string `json:"device_uuid"`
}
