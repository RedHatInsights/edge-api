package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const joinSeparator = "::"

// ErrInvalidDataAndSignatureString  returned when data and signature string is invalid
var ErrInvalidDataAndSignatureString = errors.New("invalid data and signature string")

// ErrSignatureValidationFailure returned when validation error occur
var ErrSignatureValidationFailure = errors.New("signature validation failure")

// ErrSignatureKeyCannotBeEmpty return when trying to create signature or validate with an empty key
var ErrSignatureKeyCannotBeEmpty = errors.New("signature key cannot be empty")

// UpdateTransactionPayload  The structure used to save device data in ostree remote cookie
type UpdateTransactionPayload struct {
	OrgID               string `json:"org_id"`
	UpdateTransactionID uint   `json:"update_transaction_id"`
}

// GetSignatureString  return a signature string of data, using a private key
func GetSignatureString(key, data []byte) string {
	return urlEncode(sign(key, data))
}

func sign(key []byte, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func urlEncode(data []byte) string {
	return base64.URLEncoding.EncodeToString(data)
}

func urlDecode(data string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(data)
}

// ValidateSignature check that the signature is valid for the requested data
func ValidateSignature(key []byte, data []byte, signature string) bool {
	if givenSignature, err := urlDecode(signature); err == nil {
		return hmac.Equal(sign(key, data), givenSignature)
	}
	return false
}

// PackDataAndSignature return a joined string, formed from the data payload and signature
func PackDataAndSignature(data []byte, signature string) string {
	return fmt.Sprintf("%s%s%s", urlEncode(data), joinSeparator, signature)
}

// UnpackDataAndSignature split a value to data and signature, from a unified value string
func UnpackDataAndSignature(value string) ([]byte, string, error) {
	// a valid data and signature value looks like:
	// eyJvcmdfaWQiOiIxMTc4OTc3MiIsImRldmljZV91dWlkIjoiMjhhOWFjN2YtZjliYi00NTE4LTljMTMtNTNlNjMwZjAzYThmIiwidXBkYXRlX3RyYW5zYWN0aW9uX2lkIjoyfQ==::RRlcLYl_oeD6dyV0or59iud1150I227Q5u4s3eBrti8=
	// where "::" is a separator between data and signature
	if len(value) == 0 ||
		!strings.Contains(value, joinSeparator) ||
		strings.HasPrefix(value, joinSeparator) ||
		strings.HasSuffix(value, joinSeparator) {
		log.WithField("error", ErrInvalidDataAndSignatureString.Error()).Error("unpack value is empty")
		return nil, "", ErrInvalidDataAndSignatureString
	}
	values := strings.Split(value, joinSeparator)
	if len(values) != 2 {
		log.WithField("error", ErrInvalidDataAndSignatureString.Error()).Error("unpack value must have 2 items, the payload and signature")
		return nil, "", ErrInvalidDataAndSignatureString
	}
	data, err := urlDecode(values[0])
	if err != nil {
		log.WithField("error", err.Error()).Error("payload data is valid for url decode")
		return nil, "", err
	}
	// return data, signature, error
	return data, values[1], nil
}

// EncodeSignedPayloadValue create a one string from a data payload and a signature of the data payload using the key
func EncodeSignedPayloadValue(key []byte, data interface{}) (string, error) {
	if len(key) == 0 {
		log.WithField("error", ErrSignatureKeyCannotBeEmpty.Error()).Error("encoding key is empty")
		return "", ErrSignatureKeyCannotBeEmpty
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.WithField("error", err.Error()).Error("error occurred when marshaling payload data")
		return "", err
	}
	signature := GetSignatureString(key, dataBytes)
	return PackDataAndSignature(dataBytes, signature), nil
}

// DecodeSignedPayloadValue decode the data payload from a given cookie string and validate it
func DecodeSignedPayloadValue(key []byte, value string, dataReceiver interface{}) error {
	if len(key) == 0 {
		log.WithField("error", ErrSignatureKeyCannotBeEmpty.Error()).Error("encoding key is empty")
		return ErrSignatureKeyCannotBeEmpty
	}
	data, signature, err := UnpackDataAndSignature(value)
	if err != nil {
		log.WithField("error", err.Error()).Error("error when unpacking value")
		return err
	}
	if !ValidateSignature(key, data, signature) {
		log.WithField("error", ErrSignatureValidationFailure.Error()).Error("payload data validation error")
		return ErrSignatureValidationFailure
	}
	if err := json.Unmarshal(data, &dataReceiver); err != nil {
		log.WithField("error", err.Error()).Error("payload data unmarshalling error")
		return err
	}
	return nil
}

// EncodeUpdateTransactionCookieValue create a one string update transaction data payload and a signature of the payload
// using the key and update transaction uuid
func EncodeUpdateTransactionCookieValue(key []byte, update models.UpdateTransaction, data *UpdateTransactionPayload) (string, error) {
	if len(key) == 0 {
		log.WithField("error", ErrSignatureKeyCannotBeEmpty.Error()).Error("signature key is empty")
		return "", ErrSignatureKeyCannotBeEmpty
	}
	if update.UUID == uuid.Nil {
		log.WithField("error", models.ErrUpdateTransactionEmptyUUID.Error()).Error("update transaction uuid is empty")
		return "", models.ErrUpdateTransactionEmptyUUID
	}
	// extend the key with update transaction uuid
	key = append(key, []byte(update.UUID.String())...)

	return EncodeSignedPayloadValue(key, data)
}

// DecodeUpdateTransactionCookieValue decode the data payload from a given cookie string
// and validate it among the update transaction, use the update transaction uuid as part of the key
func DecodeUpdateTransactionCookieValue(key []byte, cookieValue string, update models.UpdateTransaction, dataReceiver *UpdateTransactionPayload) error {
	if len(key) == 0 {
		log.WithField("error", ErrSignatureKeyCannotBeEmpty.Error()).Error("signature key is empty")
		return ErrSignatureKeyCannotBeEmpty
	}
	if update.UUID == uuid.Nil {
		log.WithField("error", models.ErrUpdateTransactionEmptyUUID.Error()).Error("update transaction uuid is empty")
		return models.ErrUpdateTransactionEmptyUUID
	}
	key = append(key, []byte(update.UUID.String())...)

	return DecodeSignedPayloadValue(key, cookieValue, dataReceiver)
}
