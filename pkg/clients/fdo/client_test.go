//go:build fdo
// +build fdo

package fdo_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"

	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/redhatinsights/edge-api/config"

	"github.com/redhatinsights/edge-api/pkg/clients/fdo"
	"github.com/redhatinsights/edge-api/pkg/services"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Client", func() {
	config.Init()
	testLogger := log.New()
	testLogger.SetLevel(log.DebugLevel)
	testLogger.SetFormatter(&log.JSONFormatter{})
	var ctx context.Context = context.Background()

	Describe("New client", func() {
		It("should return a new client", func() {
			client := fdo.InitClient(ctx, log.NewEntry(testLogger))
			Expect(client).ToNot(BeNil())
		})
	})
	Describe("BatchUpload", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			numOfOVs := r.Header.Get("X-Number-Of-Vouchers")
			numOfOVsInt, _ := strconv.Atoi(numOfOVs)
			It("request headers are valid", func() {
				Expect(r.Header.Get("Content-Type")).To(Equal("application/cbor"))
				Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + config.Get().FDO.AuthorizationBearer))
				Expect(r.Header.Get("Accept")).To(Equal("application/json"))
				Expect(numOfOVsInt).To(Or(Equal(int(0)), Equal(int(1)), Equal(int(10))))
			})

			body, err := ioutil.ReadAll(r.Body)
			defer r.Body.Close()
			It("reading request body without error", func() {
				Expect(err).To(BeNil())
			})
			if numOfOVsInt == 1 {
				ovs := services.NewOwnershipVoucherService(context.Background(), log.NewEntry(log.New()))
				ovData, _ := ovs.ParseOwnershipVouchers(body)
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(ovData)
			} else if numOfOVsInt == 10 {
				w.WriteHeader(http.StatusCreated)
				ovData := models.OwnershipVoucherData{
					ProtocolVersion: 101,
					GUID:            "12345678-1234-1234-1234-123456789012",
					DeviceName:      "test-device",
				}
				json.NewEncoder(w).Encode([10]models.OwnershipVoucherData{ovData})
			} else {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode([0]models.OwnershipVoucherData{})
			}
		}))
		defer ts.Close()
		config.Get().FDO.URL = ts.URL
		client := fdo.InitClient(ctx, log.NewEntry(testLogger))
		testDeviceOV, err := ioutil.ReadFile("/testdevice1.ov")
		It("should successfully read ov", func() {
			Expect(err).To(BeNil())
			Expect(testDeviceOV).ToNot(BeNil())
		})
		Context("upload zero ov", func() {
			j, err := client.BatchUpload([]byte{}, 0)
			It("should fail upload ov", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("no ownership vouchers provided"))
				Expect(j).To(BeNil())
			})
		})
		Context("upload single ov", func() {
			j, err := client.BatchUpload(testDeviceOV, 1)
			It("should successfully upload ov", func() {
				Expect(err).To(BeNil())
				Expect(j).ToNot(BeNil())
			})
			ovsData := [1]models.OwnershipVoucherData{}
			resJSON, _ := json.Marshal(j)
			err = json.Unmarshal(resJSON, &ovsData)
			It("should successfully unmarshal json", func() {
				Expect(err).To(BeNil())
				Expect(ovsData).ToNot(BeNil())
				Expect(ovsData[0].ProtocolVersion).To(Equal(uint32(101)))
				Expect(ovsData[0].GUID).To(Equal("18907279-a41d-049a-ae3c-4da4ce61c14b"))
				Expect(ovsData[0].DeviceName).To(Equal("testdevice"))
			})
		})
		Context("upload multiple ov", func() {
			multipleOVs := testDeviceOV
			for i := 0; i < 9; i++ {
				multipleOVs = append(multipleOVs, testDeviceOV...)
			}
			It("multipleOVs is 10 times bigger than ov", func() {
				Expect(len(multipleOVs)).To(Equal(len(testDeviceOV) * 10))
			})
			j, err := client.BatchUpload(multipleOVs, 10)
			It("should successfully upload ov", func() {
				Expect(err).To(BeNil())
				Expect(j).ToNot(BeNil())
			})
			ovsData := [10]models.OwnershipVoucherData{}
			resJSON, _ := json.Marshal(j)
			err = json.Unmarshal(resJSON, &ovsData)
			It("should successfully unmarshal json", func() {
				Expect(err).To(BeNil())
				Expect(ovsData).ToNot(BeNil())
				Expect(ovsData[0].ProtocolVersion).To(Equal(uint32(101)))
				Expect(ovsData[0].GUID).To(Equal("12345678-1234-1234-1234-123456789012"))
				Expect(ovsData[0].DeviceName).To(Equal("test-device"))
			})
		})
	})

	Describe("BatchDelete", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			It("request headers are valid", func() {
				Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))
				Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + config.Get().FDO.AuthorizationBearer))
				Expect(r.Header.Get("Accept")).To(Equal("application/json"))
			})

			body, err := ioutil.ReadAll(r.Body)
			defer r.Body.Close()
			It("reading request body without error", func() {
				Expect(err).To(BeNil())
			})
			w.WriteHeader(http.StatusOK)
			fdoUUIDList := []string{}
			err = json.Unmarshal(body, &fdoUUIDList)
			It("should unmarshal fdoUUIDList", func() {
				Expect(err).To(BeNil())
			})
			It("body should be equal", func() {
				Expect(fdoUUIDList).To(Equal([]string{"a9bcd683-a7e4-46ed-80b2-6e55e8610d04", "1ea69fcb-b784-4d0f-ab4d-94589c6cc7ad"}))
			})
			json.NewEncoder(w).Encode(map[string]string{"op": "delete", "status": "OK"})
		}))
		defer ts.Close()
		config.Get().FDO.URL = ts.URL
		client := fdo.InitClient(ctx, log.NewEntry(testLogger))
		ov, err := ioutil.ReadFile("/testdevice1.ov")
		It("should successfully read ov", func() {
			Expect(err).To(BeNil())
			Expect(ov).ToNot(BeNil())
		})
		Context("delete zero ov", func() {
			j, err := client.BatchDelete([]string{})
			It("should fail delete ov", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("no FDO UUIDs provided"))
				Expect(j).To(BeNil())
			})
		})
		Context("delete multiple ov", func() {
			j, err := client.BatchDelete([]string{"a9bcd683-a7e4-46ed-80b2-6e55e8610d04", "1ea69fcb-b784-4d0f-ab4d-94589c6cc7ad"})
			It("should successfully delete ov", func() {
				Expect(err).To(BeNil())
				Expect(j).ToNot(BeNil())
				Expect(j.(map[string]interface{})["op"]).To(Equal("delete"))
				Expect(j.(map[string]interface{})["status"]).To(Equal("OK"))
			})
		})
	})

	Describe("API errors handling", func() {
		client := fdo.InitClient(ctx, log.NewEntry(testLogger))
		ov, _ := ioutil.ReadFile("/testdevice1.ov")
		Context("upload ov - bad request", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(errors.NewBadRequest("bad request"))
			}))
			defer ts.Close()
			config.Get().FDO.URL = ts.URL
			_, err := client.BatchUpload(ov, 1)
			It("should fail upload ov", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("bad request"))
			})
		})
		Context("delete ov - bad request", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(errors.NewBadRequest("bad request"))
			}))
			defer ts.Close()
			config.Get().FDO.URL = ts.URL
			_, err := client.BatchDelete([]string{"a9bcd683-a7e4-46ed-80b2-6e55e8610d04", "1ea69fcb-b784-4d0f-ab4d-94589c6cc7ad"})
			It("should fail delete ov", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("bad request"))
			})
		})
		Context("upload ov - internal server error", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(errors.NewInternalServerError())
			}))
			defer ts.Close()
			config.Get().FDO.URL = ts.URL
			_, err := client.BatchUpload(ov, 1)
			It("should fail upload ov", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unknown error with status code: " + strconv.Itoa(http.StatusInternalServerError)))
			})
		})
		Context("delete ov - internal server error", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(errors.NewInternalServerError())
			}))
			defer ts.Close()
			config.Get().FDO.URL = ts.URL
			_, err := client.BatchDelete([]string{"a9bcd683-a7e4-46ed-80b2-6e55e8610d04", "1ea69fcb-b784-4d0f-ab4d-94589c6cc7ad"})
			It("should fail delete ov", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unknown error with status code: " + strconv.Itoa(http.StatusInternalServerError)))
			})
		})
		Context("upload ov - no FDO server available", func() {
			_, err := client.BatchUpload(ov, 1)
			It("should fail upload ov", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("connection refused"))
			})
		})
		Context("delete ov - no FDO server available", func() {
			_, err := client.BatchDelete([]string{"a9bcd683-a7e4-46ed-80b2-6e55e8610d04", "1ea69fcb-b784-4d0f-ab4d-94589c6cc7ad"})
			It("should fail delete ov", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("connection refused"))
			})
		})
	})
})
