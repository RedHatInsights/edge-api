//go:build fdo
// +build fdo

package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Ownershipvoucher", func() {
	// fdo mock side
	config.Init()
	var fdoMockServer *http.Server

	ovs := services.NewOwnershipVoucherService(context.Background(), log.NewEntry(log.New()))
	ovb, err := ioutil.ReadFile("/testdevice1.ov")
	fdoUUIDList := []string{"fdo-uuid-1", "fdo-uuid-2"}

	_ = BeforeSuite(func() {
		// FDO mock server
		listener, _ := net.Listen("tcp", ":0")
		fdoMockServer = &http.Server{
			Addr: fmt.Sprint("http://localhost:", listener.Addr().(*net.TCPAddr).Port),
		}
		config.Get().FDO.URL = fdoMockServer.Addr // set FDO mock server address in config
		http.HandleFunc(fmt.Sprintf("/management/%s/ownership_voucher", config.Get().FDO.APIVersion), func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(models.OwnershipVoucherData{
				ProtocolVersion: 100,
				GUID:            "12345678-1234-1234-1234-123456789012",
				DeviceName:      "test-device",
			})
		})
		http.HandleFunc(fmt.Sprintf("/management/%s/ownership_voucher/delete", config.Get().FDO.APIVersion), func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"op": "delete", "status": "OK"})
		})
		go fdoMockServer.Serve(listener)
	})

	_ = AfterSuite(func() {
		// close FDO mock server
		fdoMockServer.Close()
	})

	Context("read ov", func() {
		It("should succeed", func() {
			Expect(err).To(BeNil())
			Expect(ovb).ToNot(BeNil())
		})
	})

	Context("parse ov", func() {
		It("should parse without error", func() {
			data, err := ovs.ParseOwnershipVouchers(ovb)
			Expect(err).To(BeNil())
			Expect(data[0].ProtocolVersion).To(Equal(uint32(100)))
			Expect(data[0].DeviceName).To(Equal("testdevice1"))
			Expect(data[0].GUID).To(Equal("214d64be-3227-92da-0333-b1e1fe832f24"))
		})
		It("should parse with error", func() {
			badOV := ovb[1:]
			data, err := ovs.ParseOwnershipVouchers(badOV)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to parse ownership vouchers"))
			Expect(data).To(BeNil())
		})
	})

	Context("create ownership vouchers", func() {
		It("should create ownership vouchers", func() {
			j, err := ovs.BatchUploadOwnershipVouchers(ovb, 1)
			Expect(err).To(BeNil())
			Expect(j).ToNot(BeNil())
		})
		It("should create ownership vouchers with bad OV", func() {
			badOV := ovb[1:]
			j, err := ovs.BatchUploadOwnershipVouchers(badOV, 1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to parse ownership vouchers"))
			Expect(j).To(BeNil())
		})
		It("should create ownership vouchers with bad number of OVs", func() {
			j, err := ovs.BatchUploadOwnershipVouchers(ovb, 0)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("no ownership vouchers provided"))
			Expect(j).To(BeNil())
		})
	})

	Context("delete ownership vouchers", func() {
		It("should delete ownership vouchers", func() {
			j, err := ovs.BatchDeleteOwnershipVouchers(fdoUUIDList)
			Expect(err).To(BeNil())
			Expect(j).ToNot(BeNil())
		})
		It("should delete ownership vouchers with error", func() {
			j, err := ovs.BatchDeleteOwnershipVouchers([]string{})
			Expect(err).ToNot(BeNil())
			Expect(j).To(BeNil())
		})
	})

	Context("connect devices", func() {
		It("should succeed", func() {
			resp, err := ovs.ConnectDevices([]string{})
			Expect(err).To(BeNil())
			Expect(resp).To(BeNil())
		})
		It("should not connect", func() {
			resp, err := ovs.ConnectDevices(fdoUUIDList)
			var errList []error
			for _, uuid := range fdoUUIDList {
				errList = append(errList, errors.New(uuid))
			}
			Expect(err).To(Equal(errList))
			Expect(resp).To(BeNil())
		})
	})
})
