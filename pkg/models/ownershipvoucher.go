package models

// ownership voucher data
type OwnershipVoucherData struct {
	ProtocolVersion uint   `json:"protocol_version"`
	GUID            string `json:"guid"`
	DeviceName      string `json:"device_name"`
}
