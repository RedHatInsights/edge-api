// Package ownershipvoucher implements Ownershipvoucher deserialization from CBOR
// As for our needs we'll deserialize its header only
package ownershipvoucher

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/fxamacker/cbor/v2"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

// CBOR unmarshal of OV header, receives []byte from unmarshalOwnershipVoucher
// returns OV header as pointer to OwnershipVoucherHeader struct & err
func unmarshalOwnershipVoucherHeader(ovhb []byte) (ovh *models.OwnershipVoucherHeader, err error) {
	err = cbor.Unmarshal(ovhb, &ovh)
	return ovh, err
}

// If CBOR unmarshal fails => panic
// Something might be wrong with OV
func unmarshalCheck(e error, ovORovh string) {
	if e != nil {
		panic(map[string]interface{}{
			"method":  "deserializer.unmarshalCheck",
			"what":    ovORovh,
			"details": e.Error(),
		})
	}
}

// Add error code & details as a method to avoid duplicated lines, return JSON bytes
func addErrLogFields(fields log.Fields, counter int, err string, details interface{}, logMsg string) []byte {
	fields["ovs_parsed"] = counter
	fields["error_code"] = err
	fields["error_details"] = details
	log.WithFields(fields).Error(logMsg)
	ejson, _ := json.Marshal(fields)
	return ejson
}

// ParseBytes is CBOR unmarshal of OV, receives []byte from loading the OV file (either reading/receiving)
// do some validation checks and returns OV header as pointer to OwnershipVoucherHeader struct
func ParseBytes(ovb []byte) (ovha []models.OwnershipVoucherHeader, err error) {
	var (
		ov        models.OwnershipVoucher
		counter   int        = 0
		logFields log.Fields = map[string]interface{}{"method": "deserializer.ParseBytes"}
	)
	defer func() { // in a panic case, stop the parsing but keep alive
		if recErr := recover(); recErr != nil {
			ejson := addErrLogFields(logFields, counter, "parse_error", recErr, "panic occurred")
			err = errors.New(string(ejson))
		}
	}()
	if err := cbor.Valid(ovb); err == nil { // checking whether the CBOR data is complete and well-formed
		dec := cbor.NewDecoder(bytes.NewReader(ovb))
		for { // stream OVs
			if decErr := dec.Decode(&ov); decErr == io.EOF {
				break
			} else if decErr != nil { // couldn't decode into ownershipvoucher
				unmarshalCheck(decErr, "ownershipvoucher")
			} else {
				singleOvh, err := unmarshalOwnershipVoucherHeader(ov.Header)
				unmarshalCheck(err, "ownershipvoucher header")
				ovha = append(ovha, *singleOvh)
				counter++
			}
		}
	} else {
		ejson := addErrLogFields(logFields, counter, "non_ended_voucher", "invalid ownershipvoucher bytes", "Invalid ownershipvoucher bytes")
		return nil, errors.New(string(ejson))
	}
	logFields["ovs_parsed"] = counter
	log.WithFields(logFields).Infof("%d ownershipvouchers parsed successfully", counter)
	return ovha, nil
}

// MinimumParse gets one or more OVs as []byte,
// parse them & extract minimum data required without marshal the whole
// OV header to JSON (though possible)
func MinimumParse(ovb []byte) ([]map[string]interface{}, error) {
	ovh, err := ParseBytes(ovb)
	var minimumDataReq []map[string]interface{}
	for _, header := range ovh {
		data := models.ExtractMinimumData(&header)
		minimumDataReq = append(minimumDataReq, data)
		data["method"] = "deserializer.MinimumParse"
		log.WithFields(data).Debug("New device added")
	}
	return minimumDataReq, err
}
