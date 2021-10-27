package models

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// OwnershipVoucher CBOR representation
type OwnershipVoucher struct {
	_                      struct{} `cbor:",toarray"`
	Header                 []byte
	HeaderHmac             interface{}
	DeviceCertificateChain [][]byte
	Entries                [][]byte
}

// OwnershipVoucherHeader CBOR representation
// OwnershipVoucher.Header is from type []byte and not OwnershipVoucherHeader
// OwnershipVoucherHeader also needs deserialization.
type OwnershipVoucherHeader struct {
	_                          struct{}                   `cbor:",toarray"`
	ProtocolVersion            uint16                     `json:"protocol_version"`
	GUID                       []byte                     `json:"guid"`
	RendezvousInfo             [][]RendezvousInfo         `json:"rendezvous_info"`
	DeviceInfo                 string                     `json:"device_info"`
	PublicKey                  PublicKey                  `json:"public_key"`
	DeviceCertificateChainHash DeviceCertificateChainHash `json:"device_certificate_chain_hash"`
}

// RendezvousInfo represent information about the rendezvous server that
// stored inside the OV.
// RendezvousInfo is a 2D array with pairs of (rendezvous_variable, cbor_simple_type)
// such as (Dns, "10.89.0.3"), (OwnerPort, 8081) etc.
type RendezvousInfo struct {
	_                  struct{}    `cbor:",toarray"`
	RendezvousVariable int         `json:"rendezvous_variable"`
	CborSimpleType     interface{} `json:"cbor_simple_type"`
}

// PublicKey represent information about the public key stored inside the OV.
// KeyType field not relevant for now, therefore not resolved.
type PublicKey struct {
	_        struct{} `cbor:",toarray"`
	KeyType  int      `json:"key_type"`
	Encoding byte     `json:"encoding"`
	Data     []byte   `json:"data"`
}

// DeviceCertificateChainHash not relevant for now, therefore not resolved.
// Needed for deserialization.
type DeviceCertificateChainHash struct {
	_    struct{} `cbor:",toarray"`
	Key  int
	Info []byte
}

// MarshalJSON - custom serialization of FDO uuid to json
// Panic if can not be serialized into a valid uuid
func (ovh *OwnershipVoucherHeader) MarshalJSON() ([]byte, error) {
	type Alias OwnershipVoucherHeader
	guid, err := guidAsString(ovh)
	b, _ := json.Marshal(&struct {
		GUID string `json:"guid"`
		*Alias
	}{
		GUID:  guid,
		Alias: (*Alias)(ovh),
	})
	return b, err
}

// MarshalJSON - custom serialization of RendezvousInfo to json
func (ri *RendezvousInfo) MarshalJSON() ([]byte, error) {
	type Alias RendezvousInfo
	variable, err := ResolveRendezvousVariableCode(ri.RendezvousVariable)
	b, _ := json.Marshal(&struct {
		RendezvousVariable string `json:"rendezvous_variable"`
		*Alias
	}{
		RendezvousVariable: variable,
		Alias:              (*Alias)(ri),
	})
	return b, err
}

// MarshalJSON - custom serialization of PublicKey to json
func (pk *PublicKey) MarshalJSON() ([]byte, error) {
	type Alias PublicKey
	enc, err := ResolvePublicKeyEncoding(int(pk.Encoding))
	b, _ := json.Marshal(&struct {
		Encoding string `json:"encoding"`
		*Alias
	}{
		Encoding: enc,
		Alias:    (*Alias)(pk),
	})
	return b, err
}

// ResolvePublicKeyEncoding resolves PublicKey.Encoding to a readable string
func ResolvePublicKeyEncoding(PublicKeyEncoding int) (enc string, err error) {
	switch PublicKeyEncoding {
	case 0:
		enc = "Crypto"
	case 1:
		enc = "X509"
	case 2:
		enc = "COSEX509"
	case 3:
		enc = "Cosekey"
	default:
		ejson, _ := json.Marshal(map[string]interface{}{
			"method":              "models.ResolvePublicKeyEncoding",
			"public_key_encoding": PublicKeyEncoding,
			"msg":                 fmt.Sprintln("Could't resolve PublicKeyEncoding: ", PublicKeyEncoding),
		})
		err = errors.New(string(ejson))
	}
	return enc, err
}

// ResolveRendezvousVariableCode resolves RendezvousVariable to a readable string
// RendezvousVariable is the left side arg of RendezvousInfo pair
func ResolveRendezvousVariableCode(RendezvousVariable int) (variable string, err error) {
	switch RendezvousVariable {
	case 0:
		variable = "DeviceOnly"
	case 1:
		variable = "OwnerOnly"
	case 2:
		variable = "IPAddress"
	case 3:
		variable = "DevicePort"
	case 4:
		variable = "OwnerPort"
	case 5:
		variable = "Dns"
	case 6:
		variable = "ServerCertHash"
	case 7:
		variable = "CaCertHash"
	case 8:
		variable = "UserInput"
	case 9:
		variable = "WifiSsid"
	case 10:
		variable = "WifiPw"
	case 11:
		variable = "Medium"
	case 12:
		variable = "Protocol"
	case 13:
		variable = "Delaysec"
	case 14:
		variable = "Bypass"
	case 15:
		variable = "Extended"
	default:
		ejson, _ := json.Marshal(map[string]interface{}{
			"method":              "models.ResolveRendezvousVariableCode",
			"rendezvous_variable": RendezvousVariable,
			"msg":                 fmt.Sprintln("Could't resolve RendezvousVariableCode: ", RendezvousVariable),
		})
		err = errors.New(string(ejson))
	}
	return variable, err
}

// Extract FDO uuid from the OV's header to a valid uuid string
// Panic if can't
func guidAsString(ovh *OwnershipVoucherHeader) (guid string, err error) {
	defer func() { // in a panic case, keep alive
		if recErr := recover(); recErr != nil {
			guid = "null"
			err = recErr.(error)
		}
	}()
	guid = fmt.Sprint(uuid.Must(uuid.FromBytes(ovh.GUID)))
	return guid, err
}

// Extract device name from the OV's header
func deviceName(ovh *OwnershipVoucherHeader) string {
	return ovh.DeviceInfo
}

// Extract device protocol version from the OV's header
func deviceProtocolVersion(ovh *OwnershipVoucherHeader) uint16 {
	return ovh.ProtocolVersion
}

// ExtractMinimumData extracts minimum data required from an OV header
func ExtractMinimumData(ovh *OwnershipVoucherHeader) (map[string]interface{}, error) {
	guid, err := guidAsString(ovh)
	return map[string]interface{}{
		"device_name":      deviceName(ovh),
		"fdo_uuid":         guid,
		"protocol_version": deviceProtocolVersion(ovh),
	}, err
}
