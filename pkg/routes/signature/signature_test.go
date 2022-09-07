package signature_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"strings"

	"github.com/redhatinsights/edge-api/pkg/routes/signature"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Signature", func() {

	key := []byte("OJ_6Ww7BIpWAqktkelIkPDHRO6j0vtb6prME7uXXZXZLVtBjiAiHFnTK1XUv74fn")

	Context("Pack strings", func() {
		It("should pack strings successfully", func() {
			data := "some valid data"
			signatureString := base64.URLEncoding.EncodeToString([]byte("aSimpleSignature"))
			packedData := signature.PackDataAndSignature([]byte(data), signatureString)
			expectedString := fmt.Sprintf("%s::%s", base64.URLEncoding.EncodeToString([]byte(data)), signatureString)
			Expect(packedData).To(Equal(expectedString))
		})
	})

	Context("UnPack strings", func() {
		It("should unpack strings successfully", func() {
			data := "some valid data"
			signatureString := base64.URLEncoding.EncodeToString([]byte("aSimpleSignature"))
			value := fmt.Sprintf("%s::%s", base64.URLEncoding.EncodeToString([]byte(data)), signatureString)
			responseData, responseSignature, err := signature.UnpackDataAndSignature(value)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(responseData)).To(Equal(data))
			Expect(responseSignature).To(Equal(signatureString))
		})

		It("unpack should fail when invalid separator", func() {
			data := "some valid data"
			signatureString := base64.URLEncoding.EncodeToString([]byte("aSimpleSignature"))
			value := fmt.Sprintf("%s00%s", base64.URLEncoding.EncodeToString([]byte(data)), signatureString)
			_, _, err := signature.UnpackDataAndSignature(value)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(signature.ErrInvalidDataAndSignatureString))
		})
		It("unpack should fail when data is not encoded", func() {
			data := "some valid data"
			signatureString := base64.URLEncoding.EncodeToString([]byte("aSimpleSignature"))
			value := fmt.Sprintf("%s::%s", []byte(data), signatureString)
			_, _, err := signature.UnpackDataAndSignature(value)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("illegal base64 data at input"))
		})
	})

	Context("Signed payload string", func() {
		updateTransactionID := uint(102)
		orgID := faker.UUIDHyphenated()
		var payloadString string
		data := signature.UpdateTransactionPayload{UpdateTransactionID: updateTransactionID, OrgID: orgID}
		It("payload string should be generated successfully", func() {
			encodedString, err := signature.EncodeSignedPayloadValue(key, &data)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(encodedString) > 0).To(BeTrue())
			Expect(strings.Contains(encodedString, "::")).To(BeTrue())
			payloadString = encodedString
		})
		It("payload string should be decoded successfully", func() {
			var resultDeviceData signature.UpdateTransactionPayload
			err := signature.DecodeSignedPayloadValue(key, payloadString, &resultDeviceData)
			Expect(err).ToNot(HaveOccurred())
			Expect(resultDeviceData.OrgID).To(Equal(orgID))
			Expect(resultDeviceData.UpdateTransactionID).To(Equal(updateTransactionID))
		})
		It("payload string should not validate if data changed and same signature", func() {
			// modify device data in payload but preserve the signature string
			newUpdateTransactionID := uint(121)
			newData, err := json.Marshal(&signature.UpdateTransactionPayload{UpdateTransactionID: newUpdateTransactionID, OrgID: orgID})
			Expect(err).ToNot(HaveOccurred())
			_, signatureString, err := signature.UnpackDataAndSignature(payloadString)
			Expect(err).ToNot(HaveOccurred())
			newPayloadString := signature.PackDataAndSignature(newData, signatureString)
			// try decode new value
			var resultDeviceData signature.UpdateTransactionPayload
			err = signature.DecodeSignedPayloadValue(key, newPayloadString, &resultDeviceData)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(signature.ErrSignatureValidationFailure))
		})
		It("payload string should not validate when encoded with an other key", func() {
			newPayloadString, err := signature.EncodeSignedPayloadValue([]byte("a very secure key :)"), &data)
			Expect(err).ToNot(HaveOccurred())

			var resultDeviceData signature.UpdateTransactionPayload
			err = signature.DecodeSignedPayloadValue(key, newPayloadString, &resultDeviceData)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(signature.ErrSignatureValidationFailure))
		})
		It("payload string should not validate when decoded with an other key", func() {
			var resultDeviceData signature.UpdateTransactionPayload
			err := signature.DecodeSignedPayloadValue([]byte("a very secure key :)"), payloadString, &resultDeviceData)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(signature.ErrSignatureValidationFailure))
		})
		It("invalid error when empty payload and signature", func() {
			var resultDeviceData signature.UpdateTransactionPayload
			err := signature.DecodeSignedPayloadValue(key, "::", &resultDeviceData)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(signature.ErrInvalidDataAndSignatureString))
		})
		It("invalid error when empty payload", func() {
			var resultDeviceData signature.UpdateTransactionPayload
			_, signatureString, err := signature.UnpackDataAndSignature(payloadString)
			Expect(err).ToNot(HaveOccurred())
			err = signature.DecodeSignedPayloadValue(key, "::"+signatureString, &resultDeviceData)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(signature.ErrInvalidDataAndSignatureString))
		})
		It("invalid error when empty signature", func() {
			var resultDeviceData signature.UpdateTransactionPayload
			dataPayload, _, err := signature.UnpackDataAndSignature(payloadString)
			Expect(err).ToNot(HaveOccurred())
			err = signature.DecodeSignedPayloadValue(key, string(dataPayload)+"::", &resultDeviceData)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(signature.ErrInvalidDataAndSignatureString))
		})
		It("invalid error when wrong separator", func() {
			var resultDeviceData signature.UpdateTransactionPayload
			dataPayload, signatureString, err := signature.UnpackDataAndSignature(payloadString)
			Expect(err).ToNot(HaveOccurred())
			err = signature.DecodeSignedPayloadValue(key, string(dataPayload)+"."+signatureString, &resultDeviceData)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(signature.ErrInvalidDataAndSignatureString))
		})
		Context("UpdateTransaction Cookie Value", func() {
			var cookieValue string
			updateTransaction := models.UpdateTransaction{
				OrgID: orgID,
			}
			db.DB.Create(&updateTransaction)

			payloadData := signature.UpdateTransactionPayload{OrgID: orgID, UpdateTransactionID: updateTransaction.ID}
			It("should return error when signing key is empty", func() {
				update := models.UpdateTransaction{OrgID: orgID, Status: models.ImageStatusSuccess}
				_, err := signature.EncodeUpdateTransactionCookieValue([]byte{}, update, &signature.UpdateTransactionPayload{OrgID: orgID})
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(signature.ErrSignatureKeyCannotBeEmpty))
			})

			It("should return error when update transaction has uuid value not set", func() {
				update := models.UpdateTransaction{OrgID: orgID, Status: models.ImageStatusSuccess}
				_, err := signature.EncodeUpdateTransactionCookieValue(key, update, &signature.UpdateTransactionPayload{OrgID: orgID})
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrUpdateTransactionEmptyUUID))
			})

			It("should encode the update transaction cookie value successfully", func() {
				updateTransactionCookieValue, err := signature.EncodeUpdateTransactionCookieValue(key, updateTransaction, &payloadData)
				Expect(err).ToNot(HaveOccurred())
				Expect(updateTransactionCookieValue).ToNot(BeEmpty())
				Expect(updateTransactionCookieValue).To(ContainSubstring("::"))
				cookieValue = updateTransactionCookieValue
			})

			It("should decode the update transaction cookie value successfully", func() {
				var decodedPayload signature.UpdateTransactionPayload
				err := signature.DecodeUpdateTransactionCookieValue(key, cookieValue, updateTransaction, &decodedPayload)
				Expect(err).ToNot(HaveOccurred())
				Expect(decodedPayload.UpdateTransactionID).To(Equal(payloadData.UpdateTransactionID))
				Expect(decodedPayload.OrgID).To(Equal(payloadData.OrgID))
			})

			It("should not decode the update transaction cookie value when using an inner update transaction", func() {
				innerUpdate := models.UpdateTransaction{OrgID: orgID, Status: models.ImageStatusSuccess}
				res := db.DB.Create(&innerUpdate)
				Expect(res.Error).ToNot(HaveOccurred())
				var decodedPayload signature.UpdateTransactionPayload
				err := signature.DecodeUpdateTransactionCookieValue(key, cookieValue, innerUpdate, &decodedPayload)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(signature.ErrSignatureValidationFailure))
			})
		})
	})
})
