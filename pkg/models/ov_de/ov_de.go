// Package ovde implements Ownershipvoucher deserialization from CBOR
// As for our needs we'll deserialize its header only
package ovde

import (
	"bytes"
	"fmt"
	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"
	"io"
)

// Extract FDO uuid from the OV's header to a valid uuid string
// Panic if can't
func guidAsString(ovh *OwnershipVoucherHeader) string {
	return fmt.Sprint(uuid.Must(uuid.FromBytes(ovh.GUID)))
}

// Extract device name from the OV's header
func deviceName(ovh *OwnershipVoucherHeader) string {
	return ovh.DeviceInfo
}

// Extract device protocol version from the OV's header
func deviceProtocolVersion(ovh *OwnershipVoucherHeader) uint16 {
	return ovh.ProtocolVersion
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
func parseBytes(ovb []byte) []OwnershipVoucherHeader {
	var ovh []OwnershipVoucherHeader

	dec := cbor.NewDecoder(bytes.NewReader(ovb))
	for {
		var ov OwnershipVoucher
		if err := dec.Decode(&ov); err == io.EOF {
			break
		} else if err != nil {
			unmarshalCheck(err, "ov")
		}
		singleOvh, err := unmarshalOwnershipVoucherHeader(ov.Header)
		unmarshalCheck(err, "Ownershipvoucher header")
		ovh = append(ovh, *singleOvh)
	}
	return ovh
}

// Get minimum data required from parseBytes without marshal the whole OV header to JSON (though possible)
func minimumParse(ovb []byte) []map[string]interface{} {
	ovh := parseBytes(ovb)
	var minimumDataReq []map[string]interface{}
	for _, header := range ovh {
		minimumDataReq = append(minimumDataReq, map[string]interface{}{
			"device_name":      deviceName(&header),
			"fdo_uuid":         guidAsString(&header),
			"protocol_version": deviceProtocolVersion(&header),
		})
	}
	return minimumDataReq
}
