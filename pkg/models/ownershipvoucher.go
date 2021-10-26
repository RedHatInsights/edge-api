package models

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"os"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

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
	return json.Marshal(&struct {
		GUID string `json:"guid"`
		*Alias
	}{
		GUID:  guidAsString(ovh),
		Alias: (*Alias)(ovh),
	})
}

// MarshalJSON - custom serialization of RendezvousInfo to json
func (ri *RendezvousInfo) MarshalJSON() ([]byte, error) {
	type Alias RendezvousInfo
	return json.Marshal(&struct {
		RendezvousVariable string `json:"rendezvous_variable"`
		*Alias
	}{
		RendezvousVariable: ResolveRendezvousVariableCode(ri.RendezvousVariable),
		Alias:              (*Alias)(ri),
	})
}

// MarshalJSON - custom serialization of PublicKey to json
func (pk *PublicKey) MarshalJSON() ([]byte, error) {
	type Alias PublicKey
	return json.Marshal(&struct {
		Encoding string `json:"encoding"`
		*Alias
	}{
		Encoding: ResolvePublicKeyEncoding(int(pk.Encoding)),
		Alias:    (*Alias)(pk),
	})
}

// ResolvePublicKeyEncoding resolves PublicKey.Encoding to a readable string
func ResolvePublicKeyEncoding(PublicKeyEncoding int) string {
	s := fmt.Sprintln("Could't resolve PublicKeyEncoding: ", PublicKeyEncoding)
	switch PublicKeyEncoding {
	case 0:
		s = "Crypto"
	case 1:
		s = "X509"
	case 2:
		s = "COSEX509"
	case 3:
		s = "Cosekey"
	default:
		log.WithFields(log.Fields{
			"method":              "models.ResolvePublicKeyEncoding",
			"public_key_encoding": PublicKeyEncoding,
		}).Error(s)
	}
	return s
}

// ResolveRendezvousVariableCode resolves RendezvousVariable to a readable string
// RendezvousVariable is the left side arg of RendezvousInfo pair
func ResolveRendezvousVariableCode(RendezvousVariable int) string {
	s := fmt.Sprintln("Could't resolve RendezvousVariableCode: ", RendezvousVariable)
	switch RendezvousVariable {
	case 0:
		s = "DeviceOnly"
	case 1:
		s = "OwnerOnly"
	case 2:
		s = "IPAddress"
	case 3:
		s = "DevicePort"
	case 4:
		s = "OwnerPort"
	case 5:
		s = "Dns"
	case 6:
		s = "ServerCertHash"
	case 7:
		s = "CaCertHash"
	case 8:
		s = "UserInput"
	case 9:
		s = "WifiSsid"
	case 10:
		s = "WifiPw"
	case 11:
		s = "Medium"
	case 12:
		s = "Protocol"
	case 13:
		s = "Delaysec"
	case 14:
		s = "Bypass"
	case 15:
		s = "Extended"
	default:
		log.WithFields(log.Fields{
			"method":              "models.ResolveRendezvousVariableCode",
			"rendezvous_variable": RendezvousVariable,
		}).Error(s)
	}
	return s
}

// Extract FDO uuid from the OV's header to a valid uuid string
// Panic if can't
func guidAsString(ovh *OwnershipVoucherHeader) (guid string) {
	defer func() { // in a panic case, keep alive
		if recErr := recover(); recErr != nil {
			log.WithFields(log.Fields{
				"method": "models.guidAsString",
				"what":   "panic occurred",
			}).Error(recErr)
			guid = "null"
		}
	}()
	guid = fmt.Sprint(uuid.Must(uuid.FromBytes(ovh.GUID)))
	return guid
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
func ExtractMinimumData(ovh *OwnershipVoucherHeader) map[string]interface{} {
	return map[string]interface{}{
		"device_name":      deviceName(ovh),
		"fdo_uuid":         guidAsString(ovh),
		"protocol_version": deviceProtocolVersion(ovh),
	}
}
