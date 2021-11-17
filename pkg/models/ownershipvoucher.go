package models

// OwnershipVoucherData represents the data of an ownership voucher
type OwnershipVoucherData struct {
	Model
	ProtocolVersion uint   `json:"protocol_version"`
	GUID            string `json:"guid"`
	DeviceName      string `json:"device_name"`
}
