package ownershipvoucher

// #include <stdlib.h>
// #include <fdo_data.h>
import "C"
import "errors"

// Data requierd for a voucher
type Data struct {
	ProtocolVersion uint   `json:"protocol_version"`
	GUID            string `json:"guid"`
	DeviceName      string `json:"device_name"`
}

// ParseVoucher parses a voucher from a byte array, returning the data and error if any
func ParseVoucher(voucherBytes []byte) (Data, error) {
	voucherBytesLen := C.size_t(len(voucherBytes))
	voucherCBytes := C.CBytes(voucherBytes)
	defer C.free(voucherCBytes)

	voucher := C.fdo_ownershipvoucher_from_data(voucherCBytes, voucherBytesLen)
	defer C.fdo_ownershipvoucher_free(voucher)
	if voucher == nil {
		return Data{}, errors.New("Failed to parse voucher")
	}

	guidC := C.fdo_ownershipvoucher_header_get_guid(voucher)
	defer C.fdo_free_string(guidC)
	guid := C.GoString(guidC)

	devinfoC := C.fdo_ownershipvoucher_header_get_device_info_string(voucher)
	defer C.fdo_free_string(devinfoC)
	devinfo := C.GoString(devinfoC)

	return Data{
		ProtocolVersion: uint(C.fdo_ownershipvoucher_header_get_protocol_version(voucher)),
		GUID:            guid,
		DeviceName:      devinfo,
	}, nil
}
