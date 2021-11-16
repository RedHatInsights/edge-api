package ownershipvoucher

// #cgo LDFLAGS: -l:libfdo_data.so.0
// #include <stdlib.h>
// #include <fdo_data.h>
import "C"
import (
	"errors"
	"github.com/redhatinsights/edge-api/pkg/models"
)

// ParseVouchers parses vouchers from a byte array, returning the data and error if any
func ParseVouchers(voucherBytes []byte) ([]models.OwnershipVoucherData, error) {
	voucherBytesLen := C.size_t(len(voucherBytes))
	voucherCBytes := C.CBytes(voucherBytes)
	defer C.free(voucherCBytes)

	voucher := C.fdo_ownershipvoucher_from_data(voucherCBytes, voucherBytesLen)
	defer C.fdo_ownershipvoucher_free(voucher)
	if voucher == nil {
		return []models.OwnershipVoucherData{}, errors.New("Failed to parse voucher")
	}

	guidC := C.fdo_ownershipvoucher_header_get_guid(voucher)
	defer C.fdo_free_string(guidC)
	guid := C.GoString(guidC)

	devinfoC := C.fdo_ownershipvoucher_header_get_device_info_string(voucher)
	defer C.fdo_free_string(devinfoC)
	devinfo := C.GoString(devinfoC)

	return []models.OwnershipVoucherData{
		models.OwnershipVoucherData{
			ProtocolVersion: uint(C.fdo_ownershipvoucher_header_get_protocol_version(voucher)),
			GUID:            guid,
			DeviceName:      devinfo,
		},
	}, nil
}
