// Ownershipvoucher deserialization package from CBOR
// As for our needs we'll deserialize its header only
package ov_de

import (
	"fmt"
	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"
)

// Extract FDO uuid from the OV's header to a valid uuid string
// Panic if can't
func guidAsString(ovh *OwnershipVoucherHeader) string {
	return fmt.Sprint(uuid.Must(uuid.FromBytes(ovh.Guid)))
}

// Extract device name from the OV's header
func deviceName(ovh *OwnershipVoucherHeader) string {
	return ovh.DeviceInfo
}

// CBOR unmarshal of OV, receives []byte from loading the OV file (either reading/receiving)
// returns OV header as []byte & err
func unmarshalOwnershipVoucher(ovb []byte) ([]byte, error) {
	var ov OwnershipVoucher
	err := cbor.Unmarshal(ovb, &ov)
	return ov.Header, err
}

// CBOR unmarshal of OV header, receives []byte from unmarshalOwnershipVoucher
// returns OV header as pointer to OwnershipVoucherHeader struct & err
func unmarshalOwnershipVoucherHeader(ovhb []byte) (*OwnershipVoucherHeader, error) {
	var ovh OwnershipVoucherHeader
	err := cbor.Unmarshal(ovhb, &ovh)
	return &ovh, err
}

// If CBOR unmarshal fails => panic
// Something might be wrong with OV
func unmarshalCheck(e error, ovORovh string) {
	if e != nil {
		panic(fmt.Sprintf("Can't unmarshal %s from bytes", ovORovh))
	}
}

// CBOR unmarshal of OV, receives []byte from loading the OV file (either reading/receiving)
// do some validation checks and returns OV header as pointer to OwnershipVoucherHeader struct
func parseBytes(ovb []byte) *OwnershipVoucherHeader {
	var (
		err  error
		ovh  *OwnershipVoucherHeader
		ovhb []byte
	)
	ovhb, err = unmarshalOwnershipVoucher(ovb)
	unmarshalCheck(err, "Ownershipvoucher")
	ovh, err = unmarshalOwnershipVoucherHeader(ovhb)
	unmarshalCheck(err, "Ownershipvoucher header")
	return ovh
}

// Get minimum data required from parseBytes without marshal the whole OV header to JSON (though possible)
func minimumParse(ovb []byte) map[string]string {
	ovh := parseBytes(ovb)
	return map[string]string{"device_name": deviceName(ovh), "fdo_uuid": guidAsString(ovh)}
}
