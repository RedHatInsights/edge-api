package ovde

import (
	"encoding/json"
	"fmt"
	"os"
)

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
		fmt.Fprintln(os.Stderr, s)
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
		fmt.Fprintln(os.Stderr, s)
	}
	return s
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
