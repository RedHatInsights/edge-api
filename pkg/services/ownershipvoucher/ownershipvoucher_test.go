package ownershipvoucher_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"

	ov "github.com/redhatinsights/edge-api/pkg/services/ownershipvoucher"
)

var _ = Describe("Ownershipvoucher", func() {
	Context("parse ov", func() {
		ovb, err := os.ReadFile("./testdevice1.ov")
		It("should read ov", func() {
			Expect(err).To(BeNil())
			Expect(ovb).ToNot(BeNil())
		})
		data, err := ov.ParseVoucher(ovb)
		It("should parse without error", func() {
			Expect(err).To(BeNil())
			Expect(data.ProtocolVersion).To(Equal(uint(100)))
			Expect(data.DeviceName).To(Equal("testdevice1"))
			Expect(data.GUID).To(Equal("214d64be-3227-92da-0333-b1e1fe832f24"))
		})
	})
})
