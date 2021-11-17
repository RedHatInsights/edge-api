package services_test

import (
	"context"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/redhatinsights/edge-api/pkg/services"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Ownershipvoucher", func() {
	ovb, err := ioutil.ReadFile("/testdevice1.ov")
	Context("read ov", func() {
		It("should succeed", func() {
			Expect(err).To(BeNil())
			Expect(ovb).ToNot(BeNil())
		})
	})
	Context("parse ov", func() {
		It("should parse without error", func() {

			ovs := services.NewOwnershipVoucherService(context.Background(), log.NewEntry(log.New()))
			data, err := ovs.ParseVouchers(ovb)
			Expect(err).To(BeNil())
			Expect(data[0].ProtocolVersion).To(Equal(uint(100)))
			Expect(data[0].DeviceName).To(Equal("testdevice1"))
			Expect(data[0].GUID).To(Equal("214d64be-3227-92da-0333-b1e1fe832f24"))
		})
		It("should parse with error", func() {
			badOV := ovb[1:]
			ovs := services.NewOwnershipVoucherService(context.Background(), log.NewEntry(log.New()))
			data, err := ovs.ParseVouchers(badOV)
			Expect(err).ToNot(BeNil()) 
			Expect(err.Error()).To(Equal("failed to parse ownership voucher"))
			Expect(data).To(BeEmpty())
		})
	})
})
