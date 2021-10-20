package ovde

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
