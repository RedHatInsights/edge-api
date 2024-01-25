// FIXME: golangci-lint
// nolint:revive,typecheck
package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/go-chi/chi"
	"github.com/golang/mock/gomock"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory/mock_inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/rbac"
	"github.com/redhatinsights/edge-api/pkg/clients/rbac/mock_rbac"
	"github.com/redhatinsights/edge-api/pkg/common/seeder"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	log "github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	feature "github.com/redhatinsights/edge-api/unleash/features"
)

var _ = Describe("Devices Router", func() {
	var deviceUUID string
	var mockDeviceService *mock_services.MockDeviceServiceInterface
	var router chi.Router
	var ctrl *gomock.Controller

	BeforeEach(func() {
		// Given
		deviceUUID = faker.UUIDHyphenated()
		ctrl = gomock.NewController(GinkgoT())

		mockDeviceService = mock_services.NewMockDeviceServiceInterface(ctrl)
		mockServices := &dependencies.EdgeAPIServices{
			DeviceService: mockDeviceService,
			Log:           log.NewEntry(log.StandardLogger()),
		}
		router = chi.NewRouter()
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Println(mockServices)
				ctx := dependencies.ContextWithServices(r.Context(), mockServices)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		router.Route("/devices", MakeDevicesRouter)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("get available updates", func() {
		var req *http.Request

		When("device UUID is empty", func() {
			BeforeEach(func() {
				var err error
				req, err = http.NewRequest("GET", "/devices//updates", nil)
				Expect(err).ToNot(HaveOccurred())
			})
			It("should give an error", func() {
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(recorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("DeviceUUID must be sent"))
			})
		})
		When("device UUID is passed", func() {
			BeforeEach(func() {
				var err error
				req, err = http.NewRequest("GET", fmt.Sprintf("/devices/%s/updates", deviceUUID), nil)
				Expect(err).ToNot(HaveOccurred())
			})
			It("should fail when device is not found", func() {
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), false, models.DeviceUpdateImagesFilters{Limit: 30, Offset: 0}).Return(nil, int64(0), new(services.DeviceNotFoundError))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))
			})
			It("should fail when unexpected error happens", func() {
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), false, models.DeviceUpdateImagesFilters{Limit: 30, Offset: 0}).Return(nil, int64(0), errors.New("random error"))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			})
			It("should return when everything is okay", func() {
				updates := make([]models.ImageUpdateAvailable, 0)
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), false, models.DeviceUpdateImagesFilters{Limit: 30, Offset: 0}).Return(updates, int64(0), nil)
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})
		})
	})
	Context("get latest available update", func() {
		var req *http.Request

		When("device UUID is empty", func() {
			BeforeEach(func() {
				var err error
				req, err = http.NewRequest("GET", "/devices//updates?latest=true", nil)
				Expect(err).ToNot(HaveOccurred())
			})
			It("should give an error", func() {
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(recorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("DeviceUUID must be sent"))
			})
		})
		When("device UUID is passed", func() {
			BeforeEach(func() {
				var err error
				req, err = http.NewRequest("GET", fmt.Sprintf("/devices/%s/updates?latest=true", deviceUUID), nil)
				Expect(err).ToNot(HaveOccurred())
			})
			It("should fail when device is not found", func() {
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), true, models.DeviceUpdateImagesFilters{Limit: 30, Offset: 0}).Return(nil, int64(0), new(services.DeviceNotFoundError))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))
			})
			It("should fail when unexpected error happens", func() {
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), true, models.DeviceUpdateImagesFilters{Limit: 30, Offset: 0}).Return(nil, int64(0), errors.New("random error"))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			})
			It("should return when everything is okay", func() {
				updates := make([]models.ImageUpdateAvailable, 0)
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), true, models.DeviceUpdateImagesFilters{Limit: 30, Offset: 0}).Return(updates, int64(0), nil)
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})
		})
	})
	Context("get list of device", func() {

		When("when device is not found", func() {
			It("should return 200", func() {
				req, err := http.NewRequest("GET", "/devices", nil)
				if err != nil {
					Expect(err).ToNot(HaveOccurred())
				}
				rr := httptest.NewRecorder()
				handler := http.HandlerFunc(GetDevice)
				ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
					Log: log.NewEntry(log.StandardLogger()),
				})
				req = req.WithContext(ctx)
				handler.ServeHTTP(rr, req)
				Expect(rr.Code).To(Equal(http.StatusOK))

			})
		})
	})
})

var _ = Describe("Devices View Router", func() {
	var mockDeviceService *mock_services.MockDeviceServiceInterface
	var router chi.Router
	var mockServices *dependencies.EdgeAPIServices
	var ctrl *gomock.Controller

	BeforeEach(func() {
		// Given
		ctrl = gomock.NewController(GinkgoT())

		mockDeviceService = mock_services.NewMockDeviceServiceInterface(ctrl)
		mockServices = &dependencies.EdgeAPIServices{
			DeviceService: mockDeviceService,
			Log:           log.NewEntry(log.StandardLogger()),
		}
		router = chi.NewRouter()
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Println(mockServices)
				ctx := dependencies.ContextWithServices(r.Context(), mockServices)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		router.Route("/devicesview", MakeDevicesRouter)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("get devicesview", func() {

		When("when devices are not found", func() {
			It("should return 200", func() {
				mockDeviceService.EXPECT().GetDevicesCount(gomock.Any()).Return(int64(0), nil)
				mockDeviceService.EXPECT().GetDevicesView(gomock.Any(), gomock.Any(), gomock.Any()).Return(&models.DeviceViewList{}, nil)
				req, err := http.NewRequest("GET", "/", nil)
				if err != nil {
					Expect(err).ToNot(HaveOccurred())
				}
				rr := httptest.NewRecorder()
				handler := http.HandlerFunc(GetDevicesView)

				ctx := dependencies.ContextWithServices(req.Context(), mockServices)
				req = req.WithContext(ctx)
				handler.ServeHTTP(rr, req)
				Expect(rr.Code).To(Equal(http.StatusOK))

			})
		})
	})
})

var _ = Describe("Device Router", func() {
	var mockDeviceService *mock_services.MockDeviceServiceInterface
	// var router chi.Router
	var mockServices *dependencies.EdgeAPIServices
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockDeviceService = mock_services.NewMockDeviceServiceInterface(ctrl)
		mockServices = &dependencies.EdgeAPIServices{
			DeviceService: mockDeviceService,
			Log:           log.NewEntry(log.StandardLogger()),
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("device update images filter params", func() {
		var IntPointer = func(value int) *int { return &value }

		When("GetDevice", func() {
			It("filter params should be created as expected", func() {
				deviceUUID := faker.UUIDHyphenated()
				expectedDeviceUpdateImagesFilters := models.DeviceUpdateImagesFilters{
					Limit:              10,
					Offset:             1,
					Version:            4,
					Release:            "rhel-8.8",
					Created:            "2024-01-16",
					AdditionalPackages: IntPointer(10),
					AllPackages:        IntPointer(200),
					SystemsRunning:     IntPointer(15),
				}

				mockDeviceService.EXPECT().GetDeviceDetailsByUUID(deviceUUID, gomock.Any()).DoAndReturn(
					func(deviceID string, deviceUpdateImagesFilters models.DeviceUpdateImagesFilters) (*models.DeviceDetails, error) {
						Expect(deviceID).To(Equal(deviceUUID))
						Expect(deviceUpdateImagesFilters.Limit).To(Equal(expectedDeviceUpdateImagesFilters.Limit))
						Expect(deviceUpdateImagesFilters.Offset).To(Equal(expectedDeviceUpdateImagesFilters.Offset))
						Expect(deviceUpdateImagesFilters.Version).To(Equal(expectedDeviceUpdateImagesFilters.Version))
						Expect(deviceUpdateImagesFilters.Release).To(Equal(expectedDeviceUpdateImagesFilters.Release))
						Expect(deviceUpdateImagesFilters.Created).To(Equal(expectedDeviceUpdateImagesFilters.Created))
						Expect(deviceUpdateImagesFilters.AdditionalPackages).ToNot(BeNil())
						Expect(*deviceUpdateImagesFilters.AdditionalPackages).To(Equal(*expectedDeviceUpdateImagesFilters.AdditionalPackages))
						Expect(deviceUpdateImagesFilters.AllPackages).ToNot(BeNil())
						Expect(*deviceUpdateImagesFilters.AllPackages).To(Equal(*expectedDeviceUpdateImagesFilters.AllPackages))
						Expect(deviceUpdateImagesFilters.SystemsRunning).ToNot(BeNil())
						Expect(*deviceUpdateImagesFilters.SystemsRunning).To(Equal(*expectedDeviceUpdateImagesFilters.SystemsRunning))
						return &models.DeviceDetails{}, nil
					},
				)

				params := fmt.Sprintf(
					"limit=%d&offset=%d&version=%d&release=%s&created=%s&additional%%20packages=%d&all%%20packages=%d&systems%%20running=%d",
					expectedDeviceUpdateImagesFilters.Limit,
					expectedDeviceUpdateImagesFilters.Offset,
					expectedDeviceUpdateImagesFilters.Version,
					expectedDeviceUpdateImagesFilters.Release,
					expectedDeviceUpdateImagesFilters.Created,
					*expectedDeviceUpdateImagesFilters.AdditionalPackages,
					*expectedDeviceUpdateImagesFilters.AllPackages,
					*expectedDeviceUpdateImagesFilters.SystemsRunning,
				)
				path := fmt.Sprintf("/devices/%s/?%s", deviceUUID, params)
				req, err := http.NewRequest("GET", path, nil)
				if err != nil {
					Expect(err).ToNot(HaveOccurred())
				}

				ctx := dependencies.ContextWithServices(req.Context(), mockServices)
				ctx = context.WithValue(ctx, deviceContextKey, DeviceContext{DeviceUUID: deviceUUID})
				ctx = context.WithValue(ctx, common.PaginationKey, common.Pagination{
					Limit: expectedDeviceUpdateImagesFilters.Limit, Offset: expectedDeviceUpdateImagesFilters.Offset,
				})
				req = req.WithContext(ctx)

				recorder := httptest.NewRecorder()
				handler := http.HandlerFunc(GetDevice)
				handler.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})
		})

		When("GetUpdateAvailableForDevice", func() {
			It("filter params should be created as expected", func() {
				deviceUUID := faker.UUIDHyphenated()
				expectedDeviceUpdateImagesFilters := models.DeviceUpdateImagesFilters{
					Limit:              10,
					Offset:             1,
					Version:            4,
					Release:            "rhel-8.8",
					Created:            "2024-01-16",
					AdditionalPackages: IntPointer(10),
					AllPackages:        IntPointer(200),
					SystemsRunning:     IntPointer(15),
				}

				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(deviceUUID, false, gomock.Any()).DoAndReturn(
					func(deviceID string, latestUpdate bool, deviceUpdateImagesFilters models.DeviceUpdateImagesFilters) (*models.DeviceDetails, int64, error) {
						Expect(deviceID).To(Equal(deviceUUID))
						Expect(latestUpdate).To(BeFalse())
						Expect(deviceUpdateImagesFilters.Limit).To(Equal(expectedDeviceUpdateImagesFilters.Limit))
						Expect(deviceUpdateImagesFilters.Offset).To(Equal(expectedDeviceUpdateImagesFilters.Offset))
						Expect(deviceUpdateImagesFilters.Version).To(Equal(expectedDeviceUpdateImagesFilters.Version))
						Expect(deviceUpdateImagesFilters.Release).To(Equal(expectedDeviceUpdateImagesFilters.Release))
						Expect(deviceUpdateImagesFilters.Created).To(Equal(expectedDeviceUpdateImagesFilters.Created))
						Expect(deviceUpdateImagesFilters.AdditionalPackages).ToNot(BeNil())
						Expect(*deviceUpdateImagesFilters.AdditionalPackages).To(Equal(*expectedDeviceUpdateImagesFilters.AdditionalPackages))
						Expect(deviceUpdateImagesFilters.AllPackages).ToNot(BeNil())
						Expect(*deviceUpdateImagesFilters.AllPackages).To(Equal(*expectedDeviceUpdateImagesFilters.AllPackages))
						Expect(deviceUpdateImagesFilters.SystemsRunning).ToNot(BeNil())
						Expect(*deviceUpdateImagesFilters.SystemsRunning).To(Equal(*expectedDeviceUpdateImagesFilters.SystemsRunning))
						return &models.DeviceDetails{}, 0, nil
					},
				)

				params := fmt.Sprintf(
					"limit=%d&offset=%d&version=%d&release=%s&created=%s&additional%%20packages=%d&all%%20packages=%d&systems%%20running=%d",
					expectedDeviceUpdateImagesFilters.Limit,
					expectedDeviceUpdateImagesFilters.Offset,
					expectedDeviceUpdateImagesFilters.Version,
					expectedDeviceUpdateImagesFilters.Release,
					expectedDeviceUpdateImagesFilters.Created,
					*expectedDeviceUpdateImagesFilters.AdditionalPackages,
					*expectedDeviceUpdateImagesFilters.AllPackages,
					*expectedDeviceUpdateImagesFilters.SystemsRunning,
				)
				path := fmt.Sprintf("/devices/%s/updates?%s", deviceUUID, params)
				req, err := http.NewRequest("GET", path, nil)
				if err != nil {
					Expect(err).ToNot(HaveOccurred())
				}

				ctx := dependencies.ContextWithServices(req.Context(), mockServices)
				ctx = context.WithValue(ctx, deviceContextKey, DeviceContext{DeviceUUID: deviceUUID})
				ctx = context.WithValue(ctx, common.PaginationKey, common.Pagination{
					Limit: expectedDeviceUpdateImagesFilters.Limit, Offset: expectedDeviceUpdateImagesFilters.Offset,
				})
				req = req.WithContext(ctx)

				recorder := httptest.NewRecorder()
				handler := http.HandlerFunc(GetUpdateAvailableForDevice)
				handler.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})
		})

		When("GetDeviceImageInfo", func() {
			It("filter params should be created as expected", func() {
				deviceUUID := faker.UUIDHyphenated()
				expectedDeviceUpdateImagesFilters := models.DeviceUpdateImagesFilters{
					Limit:              10,
					Offset:             1,
					Version:            4,
					Release:            "rhel-8.8",
					Created:            "2024-01-16",
					AdditionalPackages: IntPointer(10),
					AllPackages:        IntPointer(200),
					SystemsRunning:     IntPointer(15),
				}

				mockDeviceService.EXPECT().GetDeviceImageInfoByUUID(deviceUUID, gomock.Any()).DoAndReturn(
					func(deviceID string, deviceUpdateImagesFilters models.DeviceUpdateImagesFilters) (*models.ImageInfo, error) {
						Expect(deviceID).To(Equal(deviceUUID))
						Expect(deviceUpdateImagesFilters.Limit).To(Equal(expectedDeviceUpdateImagesFilters.Limit))
						Expect(deviceUpdateImagesFilters.Offset).To(Equal(expectedDeviceUpdateImagesFilters.Offset))
						Expect(deviceUpdateImagesFilters.Version).To(Equal(expectedDeviceUpdateImagesFilters.Version))
						Expect(deviceUpdateImagesFilters.Release).To(Equal(expectedDeviceUpdateImagesFilters.Release))
						Expect(deviceUpdateImagesFilters.Created).To(Equal(expectedDeviceUpdateImagesFilters.Created))
						Expect(deviceUpdateImagesFilters.AdditionalPackages).ToNot(BeNil())
						Expect(*deviceUpdateImagesFilters.AdditionalPackages).To(Equal(*expectedDeviceUpdateImagesFilters.AdditionalPackages))
						Expect(deviceUpdateImagesFilters.AllPackages).ToNot(BeNil())
						Expect(*deviceUpdateImagesFilters.AllPackages).To(Equal(*expectedDeviceUpdateImagesFilters.AllPackages))
						Expect(deviceUpdateImagesFilters.SystemsRunning).ToNot(BeNil())
						Expect(*deviceUpdateImagesFilters.SystemsRunning).To(Equal(*expectedDeviceUpdateImagesFilters.SystemsRunning))
						return &models.ImageInfo{}, nil
					},
				)

				params := fmt.Sprintf(
					"limit=%d&offset=%d&version=%d&release=%s&created=%s&additional%%20packages=%d&all%%20packages=%d&systems%%20running=%d",
					expectedDeviceUpdateImagesFilters.Limit,
					expectedDeviceUpdateImagesFilters.Offset,
					expectedDeviceUpdateImagesFilters.Version,
					expectedDeviceUpdateImagesFilters.Release,
					expectedDeviceUpdateImagesFilters.Created,
					*expectedDeviceUpdateImagesFilters.AdditionalPackages,
					*expectedDeviceUpdateImagesFilters.AllPackages,
					*expectedDeviceUpdateImagesFilters.SystemsRunning,
				)
				path := fmt.Sprintf("/devices/%s/image?%s", deviceUUID, params)
				req, err := http.NewRequest("GET", path, nil)
				if err != nil {
					Expect(err).ToNot(HaveOccurred())
				}

				ctx := dependencies.ContextWithServices(req.Context(), mockServices)
				ctx = context.WithValue(ctx, deviceContextKey, DeviceContext{DeviceUUID: deviceUUID})
				ctx = context.WithValue(ctx, common.PaginationKey, common.Pagination{
					Limit: expectedDeviceUpdateImagesFilters.Limit, Offset: expectedDeviceUpdateImagesFilters.Offset,
				})
				req = req.WithContext(ctx)

				recorder := httptest.NewRecorder()
				handler := http.HandlerFunc(GetDeviceImageInfo)
				handler.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})
		})
	})
})

var _ = Describe("Devices Router Integration", func() {
	var mockInventory *mock_inventory.MockClientInterface
	var router chi.Router
	var apiServices *dependencies.EdgeAPIServices
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctx := context.Background()
		log := log.NewEntry(log.StandardLogger())
		ctrl = gomock.NewController(GinkgoT())

		mockInventory = mock_inventory.NewMockClientInterface(ctrl)

		deviceService := &services.DeviceService{
			UpdateService: services.NewUpdateService(ctx, log),
			ImageService:  services.NewImageService(ctx, log),
			Inventory:     mockInventory,
			Service:       services.NewService(ctx, log),
		}
		apiServices = &dependencies.EdgeAPIServices{
			DeviceService: deviceService,
			Log:           log,
		}
		router = chi.NewRouter()
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctxServices := dependencies.ContextWithServices(r.Context(), apiServices)
				next.ServeHTTP(w, r.WithContext(ctxServices))
			})
		})
		router.Route("/devices", MakeDevicesRouter)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("get device by uuid", func() {
		var device *models.Device
		var image *models.Image
		var imageUpdate *models.Image
		var imageSet *models.ImageSet

		BeforeEach(func() {
			image, imageSet = seeder.Images().Create()
			device = seeder.Devices().WithImageID(image.ID).Create()

			imageUpdate, _ = seeder.Images().WithImageSetID(imageSet.ID).Create()
		})

		When("when device exist", func() {
			BeforeEach(func() {
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{
						ID: device.UUID,
						Ostree: inventory.SystemProfile{
							RHCClientID: faker.UUIDHyphenated(),
							RpmOstreeDeployments: []inventory.OSTree{
								{Checksum: image.Commit.OSTreeCommit, Booted: true},
							},
						},
						OrgID: common.DefaultOrgID,
					},
				}}

				mockInventory.EXPECT().ReturnDevicesByID(gomock.Eq(device.UUID)).Return(resp, nil)
			})

			It("should return a device", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/devices/%s", device.UUID), nil)
				if err != nil {
					Expect(err).ToNot(HaveOccurred())
				}
				rr := httptest.NewRecorder()
				handler := http.HandlerFunc(GetDevice)

				ctx := context.WithValue(req.Context(), deviceContextKey, DeviceContext{DeviceUUID: device.UUID})
				ctx = dependencies.ContextWithServices(ctx, apiServices)

				req = req.WithContext(ctx)
				handler.ServeHTTP(rr, req)
				Expect(rr.Code).To(Equal(http.StatusOK))

				body, err := io.ReadAll(rr.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(body)).ToNot(BeEmpty())

				var deviceDetails *models.DeviceDetails
				err = json.Unmarshal(body, &deviceDetails)
				Expect(err).ToNot(HaveOccurred())

				deviceImage := deviceDetails.Image.Image
				deviceImageUpdate := (*deviceDetails.Image.UpdatesAvailable)[0]

				deviceResponse := deviceDetails.Device

				Expect(deviceImage.Name).To(Equal(image.Name))
				Expect(deviceImage.OrgID).To(Equal(image.OrgID))
				Expect(deviceImage.Status).To(Equal(image.Status))
				Expect(deviceImage.Distribution).To(Equal(image.Distribution))
				Expect(deviceImage.Version).To(Equal(image.Version))
				Expect(deviceImage.Commit.OSTreeCommit).To(Equal(image.Commit.OSTreeCommit))

				Expect(deviceImageUpdate.CanUpdate).To(BeTrue())
				Expect(deviceImageUpdate.Image.OrgID).To(Equal(imageUpdate.OrgID))
				Expect(deviceImageUpdate.Image.Status).To(Equal(imageUpdate.Status))
				Expect(deviceImageUpdate.Image.Name).To(Equal(imageUpdate.Name))
				Expect(deviceImageUpdate.Image.Distribution).To(Equal(imageUpdate.Distribution))
				Expect(deviceImageUpdate.Image.Commit.OSTreeCommit).To(Equal(imageUpdate.Commit.OSTreeCommit))

				Expect(deviceResponse.OrgID).To(Equal(device.OrgID))
				Expect(deviceResponse.UUID).To(Equal(device.UUID))
				Expect(deviceResponse.Name).To(Equal(device.Name))
				Expect(deviceResponse.RHCClientID).To(Equal(device.RHCClientID))
				Expect(deviceResponse.ImageID).To(Equal(image.ID))

				Expect(deviceDetails.Image.Count).To(Equal(int64(1)))

				Expect(deviceResponse.OrgID).To(Equal(deviceImage.OrgID))
			})
		})
	})
})

var _ = Describe("Devices View Filters", func() {
	var imageV1 *models.Image
	var deviceCreateedAt *time.Time
	var deviceUUID string

	BeforeEach(func() {

		orgID := common.DefaultOrgID

		imageSet := &models.ImageSet{
			Name:    "test",
			Version: 2,
			OrgID:   orgID,
		}
		result := db.DB.Create(imageSet)
		Expect(result.Error).ToNot(HaveOccurred())
		imageV1 = &models.Image{
			Commit: &models.Commit{
				OSTreeCommit: faker.UUIDHyphenated(),
				OrgID:        orgID,
			},
			Status:     models.ImageStatusSuccess,
			ImageSetID: &imageSet.ID,
			Version:    1,
			OrgID:      common.DefaultOrgID,
		}
		result = db.DB.Create(imageV1.Commit)
		Expect(result.Error).ToNot(HaveOccurred())
		result = db.DB.Create(imageV1)
		Expect(result.Error).ToNot(HaveOccurred())
		deviceUUID = faker.UUIDHyphenated()
		device1 := models.Device{OrgID: orgID, Name: "device-1", UpdateAvailable: true, ImageID: imageV1.ID, UUID: deviceUUID}

		device2 := models.Device{OrgID: orgID, Name: "device-2", UpdateAvailable: false, ImageID: 99, UUID: faker.UUIDHyphenated()}

		result = db.DB.Create(&device1)
		Expect(result.Error).To(BeNil())
		result = db.DB.Create(&device2)
		Expect(result.Error).To(BeNil())
		deviceCreateedAt = &device1.CreatedAt.Time

	})
	It("when filter by name, return devices with given name", func() {
		name := "device-1"
		var devicesFilters = common.ComposeFilters(
			// Filter handler for "name"
			common.ContainFilterHandler(&common.Filter{
				QueryParam: "name",
				DBField:    "devices.name",
			}),
		)

		req, err := http.NewRequest("GET", fmt.Sprintf("?name=%s", name), nil)
		Expect(err).ToNot(HaveOccurred())
		dbFilters := devicesFilters(req, db.DB)
		devices := []models.Device{}
		dbFilters.Find(&devices)
		Expect(len(devices)).To(Equal(1))
		Expect(devices[0].Name).To(Equal(name))
	})
	It("when filter by update_available, return devices with given value", func() {
		var devicesFilters = common.ComposeFilters(
			// Filter handler for "update_available"
			common.BoolFilterHandler(&common.Filter{
				QueryParam: "update_available",
				DBField:    "devices.update_available",
			}),
		)

		req, err := http.NewRequest("GET", "?update_available=true", nil)
		Expect(err).ToNot(HaveOccurred())
		dbFilters := devicesFilters(req, db.DB)
		devices := []models.Device{}
		dbFilters.Find(&devices)
		Expect(len(devices)).ToNot(Equal(0))
		for _, device := range devices {
			Expect(device.UpdateAvailable).To(Equal(true))
		}
	})
	It("when filter by uuid, return devices with given uuid", func() {
		var devicesFilters = common.ComposeFilters(
			// Filter handler for "uuid"
			common.ContainFilterHandler(&common.Filter{
				QueryParam: "uuid",
				DBField:    "devices.uuid",
			}),
		)

		req, err := http.NewRequest("GET", fmt.Sprintf("?uuid=%s", deviceUUID), nil)
		Expect(err).ToNot(HaveOccurred())
		dbFilters := devicesFilters(req, db.DB)
		devices := []models.Device{}
		dbFilters.Find(&devices)
		Expect(len(devices)).To(Equal(1))
		Expect(devices[0].UUID).To(Equal(deviceUUID))
	})
	It("when filter by image_id, return devices with given image_id", func() {
		var devicesFilters = common.ComposeFilters(
			// Filter handler for "image_id"
			common.IntegerNumberFilterHandler(&common.Filter{
				QueryParam: "image_id",
				DBField:    "devices.image_id",
			}),
		)

		req, err := http.NewRequest("GET", fmt.Sprintf("?image_id=%d", imageV1.ID), nil)
		Expect(err).ToNot(HaveOccurred())
		dbFilters := devicesFilters(req, db.DB)
		devices := []models.Device{}
		dbFilters.Find(&devices)
		Expect(len(devices)).To(Equal(1))
		Expect(devices[0].ImageID).To(Equal(imageV1.ID))
	})
	It("when filter by created_at, return devices with matching value", func() {
		var devicesFilters = common.ComposeFilters(
			// Filter handler for "image_id"
			common.CreatedAtFilterHandler(&common.Filter{
				QueryParam: "created_at",
				DBField:    "devices.created_at",
			}),
		)

		req, err := http.NewRequest("GET", fmt.Sprintf("/?created_at=%s", deviceCreateedAt.String()), nil)
		Expect(err).ToNot(HaveOccurred())
		dbFilters := devicesFilters(req, db.DB)
		devices := []models.Device{}
		dbFilters.Find(&devices)
		Expect(devices[0].CreatedAt.Time.Year()).To(Equal(deviceCreateedAt.Year()))
		Expect(devices[0].CreatedAt.Time.Month()).To(Equal(deviceCreateedAt.Month()))
		Expect(devices[0].CreatedAt.Time.Day()).To(Equal(deviceCreateedAt.Day()))
		Expect(devices[0].CreatedAt.Time.Hour()).To(Equal(deviceCreateedAt.Hour()))
		minutesArray := [3]int{deviceCreateedAt.Minute() - 1, deviceCreateedAt.Minute(), deviceCreateedAt.Minute() + 1}
		Expect(minutesArray).To(ContainElement(devices[0].CreatedAt.Time.Minute()))
	})
})

func TestValidateGetAllDevicesFilterParams(t *testing.T) {
	RegisterTestingT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tt := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
		{
			name:   "invalid uuid",
			params: "uuid=9e7",
			expectedError: []validationError{
				{Key: "uuid", Reason: "invalid UUID length: 3"},
			},
		},
		{
			name:   "invalid created_at",
			params: "created_at=AAAA",
			expectedError: []validationError{
				{Key: "created_at", Reason: "parsing time \"AAAA\" as \"2006-01-02\": cannot parse \"AAAA\" as \"2006\""},
			},
		},
		{
			name:          "valid uuid",
			params:        "uuid=9e7a7e3c-daa8-41cf-82d0-13e3a0224cf7",
			expectedError: nil,
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	for _, te := range tt {
		req, err := http.NewRequest("GET", fmt.Sprintf("/devices?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()

		mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
		ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
			ImageService: mockImageService,
			Log:          log.NewEntry(log.StandardLogger()),
		})
		req = req.WithContext(ctx)

		ValidateGetAllDevicesFilterParams(next).ServeHTTP(w, req)

		if te.expectedError == nil {
			Expect(w.Code).To(Equal(http.StatusOK))
			continue
		}
		Expect(w.Code).To(Equal(http.StatusBadRequest))
		resp := w.Result()
		defer resp.Body.Close()
		validationsErrors := []validationError{}
		err = json.NewDecoder(resp.Body).Decode(&validationsErrors)
		if err != nil {
			Expect(err).ToNot(HaveOccurred())
		}
		for _, exErr := range te.expectedError {
			found := false
			for _, jsErr := range validationsErrors {
				if jsErr.Key == exErr.Key && jsErr.Reason == exErr.Reason {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), fmt.Sprintf("in %q: was expected to have %v but not found in %v", te.name, exErr, validationsErrors))
		}
	}
}

func TestValidateGetDevicesViewFilterParams(t *testing.T) {
	tt := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
		{
			name:   "invalid update_available",
			params: "update_available=abc",
			expectedError: []validationError{
				{Key: "update_available", Reason: "abc is not a valid value for update_available. update_available must be boolean"},
			},
		},
		{
			name:   "invalid update_available",
			params: "update_available=123",
			expectedError: []validationError{
				{Key: "update_available", Reason: "123 is not a valid value for update_available. update_available must be boolean"},
			},
		},
		{
			name:   "invalid image_id",
			params: "image_id=abc",
			expectedError: []validationError{
				{Key: "image_id", Reason: "abc is not a valid value for image_id. image_id must be integer"},
			},
		},
		{
			name:   "invalid image_id",
			params: "image_id=123abc",
			expectedError: []validationError{
				{Key: "image_id", Reason: "123abc is not a valid value for image_id. image_id must be integer"},
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	for _, te := range tt {
		req, err := http.NewRequest("GET", fmt.Sprintf("/devices/devicesview?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
		ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
			ImageService: mockImageService,
			Log:          log.NewEntry(log.StandardLogger()),
		})
		req = req.WithContext(ctx)

		ValidateGetDevicesViewFilterParams(next).ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		jsonBody := []validationError{}
		err = json.NewDecoder(resp.Body).Decode(&jsonBody)
		if err != nil {
			t.Errorf("failed decoding response body: %s", err.Error())
		}
		for _, exErr := range te.expectedError {
			found := false
			for _, jsErr := range jsonBody {
				if jsErr.Key == exErr.Key && jsErr.Reason == exErr.Reason {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("in %q: was expected to have %v but not found in %v", te.name, exErr, jsonBody)
			}
		}
	}
}

func TestValidateDeviceUpdateImagesFilterParams(t *testing.T) {
	testCases := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
		{
			name:   "invalid version",
			params: "version=not_a_number",
			expectedError: []validationError{
				{Key: "version", Reason: `"version" is not a valid "version" type, "version" must be number`},
			},
		},
		{
			name:          "valid version",
			params:        "version=2",
			expectedError: nil,
		},
		{
			name:   "invalid additional packages",
			params: "additional%20packages=not_a_number",
			expectedError: []validationError{
				{Key: "additional packages", Reason: `"additional packages" is not a valid "additional packages" type, "additional packages" must be number`},
			},
		},
		{
			name:          "valid additional packages",
			params:        "additional%20packages=10",
			expectedError: nil,
		},
		{
			name:   "invalid all packages",
			params: "all%20packages=not_a_number",
			expectedError: []validationError{
				{Key: "all packages", Reason: `"all packages" is not a valid "all packages" type, "all packages" must be number`},
			},
		},
		{
			name:          "valid all packages",
			params:        "all%20packages=220",
			expectedError: nil,
		},
		{
			name:   "invalid systems running",
			params: "systems%20running=not_a_number",
			expectedError: []validationError{
				{Key: "systems running", Reason: `"systems running" is not a valid "systems running" type, "systems running" must be number`},
			},
		},
		{
			name:          "valid systems running",
			params:        "systems%20running=10",
			expectedError: nil,
		},
		{
			name:   "invalid created",
			params: "created=AAAA",
			expectedError: []validationError{
				{Key: "created", Reason: `parsing time "AAAA" as "2006-01-02": cannot parse "AAAA" as "2006"`},
			},
		},
		{
			name:          "valid created",
			params:        "created=2006-01-02",
			expectedError: nil,
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			RegisterTestingT(t)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDeviceService := mock_services.NewMockDeviceServiceInterface(ctrl)
			mockServices := &dependencies.EdgeAPIServices{
				DeviceService: mockDeviceService,
				Log:           log.NewEntry(log.StandardLogger()),
			}

			req, err := http.NewRequest("GET", fmt.Sprintf("/devices/9e7a7e3c-daa8-41cf-82d0-13e3a0224cf7?%s", testCase.params), nil)
			Expect(err).ToNot(HaveOccurred())

			ctx := dependencies.ContextWithServices(req.Context(), mockServices)
			req = req.WithContext(ctx)

			recorder := httptest.NewRecorder()
			ValidateDeviceUpdateImagesFilterParams(next).ServeHTTP(recorder, req)

			if testCase.expectedError == nil {
				Expect(recorder.Code).To(Equal(http.StatusOK))
			} else {
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
				resp := recorder.Result()
				defer resp.Body.Close()
				validationsErrors := []validationError{}
				err = json.NewDecoder(resp.Body).Decode(&validationsErrors)
				if err != nil {
					Expect(err).ToNot(HaveOccurred())
				}
				for _, exErr := range testCase.expectedError {
					found := false
					for _, jsErr := range validationsErrors {
						if jsErr.Key == exErr.Key && jsErr.Reason == exErr.Reason {
							found = true
							break
						}
					}
					Expect(found).To(BeTrue(), fmt.Sprintf("in %q: was expected to have %v but not found in %v", testCase.name, exErr, validationsErrors))
				}
			}
		})
	}
}

func TestGetDevicesViewWithinDevices(t *testing.T) {
	RegisterTestingT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	orgID := faker.UUIDHyphenated()
	account := faker.UUIDHyphenated()
	imageSet := &models.ImageSet{
		Name:    "test",
		Version: 1,
		OrgID:   orgID,
	}

	result := db.DB.Create(imageSet)
	Expect(result.Error).ToNot(HaveOccurred())
	imageV1 := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: faker.UUIDHyphenated(),
			OrgID:        orgID,
		},
		Status:     models.ImageStatusSuccess,
		ImageSetID: &imageSet.ID,
		Version:    1,
		OrgID:      orgID,
	}
	result = db.DB.Create(imageV1)
	Expect(result.Error).ToNot(HaveOccurred())

	devices := []models.Device{
		{
			Name:    faker.Name(),
			UUID:    faker.UUIDHyphenated(),
			Account: account,
			OrgID:   orgID,
			ImageID: imageV1.ID,
		},
		{
			Name:    faker.Name(),
			UUID:    faker.UUIDHyphenated(),
			Account: account,
			OrgID:   orgID,
			ImageID: imageV1.ID,
		},
		{
			Name:    faker.Name(),
			UUID:    faker.UUIDHyphenated(),
			Account: account,
			OrgID:   orgID,
			ImageID: imageV1.ID,
		},
	}
	result = db.DB.Create(devices)
	Expect(result.Error).ToNot(HaveOccurred())

	deviceView := []models.DeviceView{
		{
			DeviceID:   devices[0].ID,
			DeviceUUID: devices[0].UUID,
		},
		{
			DeviceID:   devices[1].ID,
			DeviceUUID: devices[1].UUID,
		},
		{
			DeviceID:   devices[2].ID,
			DeviceUUID: devices[2].UUID,
		},
	}
	var mockDeviceService *mock_services.MockDeviceServiceInterface
	var router chi.Router
	var mockServices *dependencies.EdgeAPIServices
	ctrl = gomock.NewController(GinkgoT())

	mockDeviceService = mock_services.NewMockDeviceServiceInterface(ctrl)
	mockServices = &dependencies.EdgeAPIServices{
		DeviceService: mockDeviceService,
		Log:           log.NewEntry(log.StandardLogger()),
	}
	router = chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := dependencies.ContextWithServices(r.Context(), mockServices)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Route("/", MakeDevicesRouter)
	mockDeviceService.EXPECT().GetDevicesCount(gomock.Any()).Return(int64(3), nil)
	mockDeviceService.EXPECT().GetDevicesView(30, 0, gomock.Any()).Return(&models.DeviceViewList{Total: 3, Devices: deviceView}, nil)

	jsonDeviceViewListBytes, err := json.Marshal(models.FilterByDevicesAPI{DevicesUUID: []string{devices[0].UUID, devices[1].UUID, devices[2].UUID}})
	Expect(err).To(BeNil())

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonDeviceViewListBytes))
	Expect(err).To(BeNil())

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetDevicesViewWithinDevices)

	ctx := dependencies.ContextWithServices(req.Context(), mockServices)
	req = req.WithContext(ctx)
	handler.ServeHTTP(rr, req)
	Expect(rr.Code).To(Equal(http.StatusOK))

}

func TestGetDevicesViewWithoutDevices(t *testing.T) {
	RegisterTestingT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var mockDeviceService *mock_services.MockDeviceServiceInterface
	var router chi.Router
	var mockServices *dependencies.EdgeAPIServices
	ctrl = gomock.NewController(GinkgoT())

	mockDeviceService = mock_services.NewMockDeviceServiceInterface(ctrl)
	mockServices = &dependencies.EdgeAPIServices{
		DeviceService: mockDeviceService,
		Log:           log.NewEntry(log.StandardLogger()),
	}
	router = chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := dependencies.ContextWithServices(r.Context(), mockServices)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Route("/", MakeDevicesRouter)
	mockDeviceService.EXPECT().GetDevicesCount(gomock.Any()).Return(int64(0), nil)
	mockDeviceService.EXPECT().GetDevicesView(30, 0, gomock.Any()).Return(&models.DeviceViewList{Total: 0}, nil)

	jsonDeviceViewListBytes, err := json.Marshal(models.FilterByDevicesAPI{DevicesUUID: []string{}})
	Expect(err).To(BeNil())

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonDeviceViewListBytes))
	Expect(err).To(BeNil())

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetDevicesViewWithinDevices)

	ctx := dependencies.ContextWithServices(req.Context(), mockServices)
	req = req.WithContext(ctx)
	handler.ServeHTTP(rr, req)
	Expect(rr.Code).To(Equal(http.StatusBadRequest))

}

func TestValidateGetDevicesViewWithDevicesFilterParams(t *testing.T) {
	tt := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
		{
			name:   "invalid update_available",
			params: "update_available=abc",
			expectedError: []validationError{
				{Key: "update_available", Reason: "abc is not a valid value for update_available. update_available must be boolean"},
			},
		},
		{
			name:   "invalid update_available",
			params: "update_available=123",
			expectedError: []validationError{
				{Key: "update_available", Reason: "123 is not a valid value for update_available. update_available must be boolean"},
			},
		},
		{
			name:   "invalid image_id",
			params: "image_id=abc",
			expectedError: []validationError{
				{Key: "image_id", Reason: "abc is not a valid value for image_id. image_id must be integer"},
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	for _, te := range tt {
		req, err := http.NewRequest("POST", fmt.Sprintf("/devices/devicesview?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
		ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
			ImageService: mockImageService,
			Log:          log.NewEntry(log.StandardLogger()),
		})
		req = req.WithContext(ctx)

		ValidateGetDevicesViewFilterParams(next).ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		jsonBody := []validationError{}
		err = json.NewDecoder(resp.Body).Decode(&jsonBody)
		if err != nil {
			t.Errorf("failed decoding response body: %s", err.Error())
		}
		for _, exErr := range te.expectedError {
			found := false
			for _, jsErr := range jsonBody {
				if jsErr.Key == exErr.Key && jsErr.Reason == exErr.Reason {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("in %q: was expected to have %v but not found in %v", te.name, exErr, jsonBody)
			}
		}
	}
}

func TestGetDevicesViewWithinDevicesAndGetInternalErrorInCountDevice(t *testing.T) {
	RegisterTestingT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	orgID := faker.UUIDHyphenated()
	account := faker.UUIDHyphenated()
	imageSet := &models.ImageSet{
		Name:    "test",
		Version: 1,
		OrgID:   orgID,
	}

	result := db.DB.Create(imageSet)
	Expect(result.Error).ToNot(HaveOccurred())
	imageV1 := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: faker.UUIDHyphenated(),
			OrgID:        orgID,
		},
		Status:     models.ImageStatusSuccess,
		ImageSetID: &imageSet.ID,
		Version:    1,
		OrgID:      orgID,
	}
	result = db.DB.Create(imageV1)
	Expect(result.Error).ToNot(HaveOccurred())

	devices := []models.Device{
		{
			Name:    faker.Name(),
			UUID:    faker.UUIDHyphenated(),
			Account: account,
			OrgID:   orgID,
			ImageID: imageV1.ID,
		},
	}
	result = db.DB.Create(devices)
	Expect(result.Error).ToNot(HaveOccurred())

	var mockDeviceService *mock_services.MockDeviceServiceInterface
	var router chi.Router
	var mockServices *dependencies.EdgeAPIServices
	ctrl = gomock.NewController(GinkgoT())

	mockDeviceService = mock_services.NewMockDeviceServiceInterface(ctrl)
	mockServices = &dependencies.EdgeAPIServices{
		DeviceService: mockDeviceService,
		Log:           log.NewEntry(log.StandardLogger()),
	}
	router = chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := dependencies.ContextWithServices(r.Context(), mockServices)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Route("/", MakeDevicesRouter)
	mockDeviceService.EXPECT().GetDevicesCount(gomock.Any()).Return(int64(1), errors.New("random error"))
	jsonDeviceViewListBytes, err := json.Marshal(models.FilterByDevicesAPI{DevicesUUID: []string{devices[0].UUID}})
	Expect(err).To(BeNil())

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonDeviceViewListBytes))
	Expect(err).To(BeNil())

	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(GetDevicesViewWithinDevices)
	ctx := dependencies.ContextWithServices(req.Context(), mockServices)
	req = req.WithContext(ctx)
	handler.ServeHTTP(recorder, req)
	Expect(recorder.Code).To(Equal(http.StatusInternalServerError))

}

func TestGetDevicesViewWithinDevicesAndGetInternalErrorInGetDevicesView(t *testing.T) {
	RegisterTestingT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	orgID := faker.UUIDHyphenated()
	account := faker.UUIDHyphenated()
	imageSet := &models.ImageSet{
		Name:    "test",
		Version: 1,
		OrgID:   orgID,
	}

	result := db.DB.Create(imageSet)
	Expect(result.Error).ToNot(HaveOccurred())
	imageV1 := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: faker.UUIDHyphenated(),
			OrgID:        orgID,
		},
		Status:     models.ImageStatusSuccess,
		ImageSetID: &imageSet.ID,
		Version:    1,
		OrgID:      orgID,
	}
	result = db.DB.Create(imageV1)
	Expect(result.Error).ToNot(HaveOccurred())

	devices := []models.Device{
		{
			Name:    faker.Name(),
			UUID:    faker.UUIDHyphenated(),
			Account: account,
			OrgID:   orgID,
			ImageID: imageV1.ID,
		},
	}
	result = db.DB.Create(devices)
	Expect(result.Error).ToNot(HaveOccurred())

	var mockDeviceService *mock_services.MockDeviceServiceInterface
	var router chi.Router
	var mockServices *dependencies.EdgeAPIServices
	ctrl = gomock.NewController(GinkgoT())

	mockDeviceService = mock_services.NewMockDeviceServiceInterface(ctrl)
	mockServices = &dependencies.EdgeAPIServices{
		DeviceService: mockDeviceService,
		Log:           log.NewEntry(log.StandardLogger()),
	}
	router = chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := dependencies.ContextWithServices(r.Context(), mockServices)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Route("/", MakeDevicesRouter)
	mockDeviceService.EXPECT().GetDevicesCount(gomock.Any()).Return(int64(1), nil)
	mockDeviceService.EXPECT().GetDevicesView(30, 0, gomock.Any()).Return(&models.DeviceViewList{Total: 1}, errors.New("random error"))
	jsonDeviceViewListBytes, err := json.Marshal(models.FilterByDevicesAPI{DevicesUUID: []string{devices[0].UUID}})
	Expect(err).To(BeNil())

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonDeviceViewListBytes))
	Expect(err).To(BeNil())

	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(GetDevicesViewWithinDevices)
	ctx := dependencies.ContextWithServices(req.Context(), mockServices)
	req = req.WithContext(ctx)
	handler.ServeHTTP(recorder, req)
	Expect(recorder.Code).To(Equal(http.StatusInternalServerError))

}

func TestGetDevicesViewWithinDevicesWitUUIDNotExist(t *testing.T) {
	RegisterTestingT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	orgID := faker.UUIDHyphenated()
	account := faker.UUIDHyphenated()
	imageSet := &models.ImageSet{
		Name:    "test",
		Version: 1,
		OrgID:   orgID,
	}

	result := db.DB.Create(imageSet)
	Expect(result.Error).ToNot(HaveOccurred())
	imageV1 := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: faker.UUIDHyphenated(),
			OrgID:        orgID,
		},
		Status:     models.ImageStatusSuccess,
		ImageSetID: &imageSet.ID,
		Version:    1,
		OrgID:      orgID,
	}
	result = db.DB.Create(imageV1)
	Expect(result.Error).ToNot(HaveOccurred())

	devices := []models.Device{
		{
			Name:    faker.Name(),
			UUID:    faker.UUIDHyphenated(),
			Account: account,
			OrgID:   orgID,
			ImageID: imageV1.ID,
		},
	}
	result = db.DB.Create(devices)
	Expect(result.Error).ToNot(HaveOccurred())

	var mockDeviceService *mock_services.MockDeviceServiceInterface
	var router chi.Router
	var mockServices *dependencies.EdgeAPIServices
	ctrl = gomock.NewController(GinkgoT())

	mockDeviceService = mock_services.NewMockDeviceServiceInterface(ctrl)
	mockServices = &dependencies.EdgeAPIServices{
		DeviceService: mockDeviceService,
		Log:           log.NewEntry(log.StandardLogger()),
	}
	router = chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := dependencies.ContextWithServices(r.Context(), mockServices)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Route("/", MakeDevicesRouter)
	jsonDeviceViewListBytes, err := json.Marshal([]string{"uuid-1", "uuid-2"})
	Expect(err).To(BeNil())

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonDeviceViewListBytes))
	Expect(err).To(BeNil())

	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(GetDevicesViewWithinDevices)
	ctx := dependencies.ContextWithServices(req.Context(), mockServices)
	req = req.WithContext(ctx)
	handler.ServeHTTP(recorder, req)
	Expect(recorder.Code).To(Equal(http.StatusBadRequest))

}

func TestGetDevicesViewFilteringByGroup(t *testing.T) {
	RegisterTestingT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	orgID := faker.UUIDHyphenated()
	account := faker.UUIDHyphenated()
	imageSet := &models.ImageSet{
		Name:    "test",
		Version: 1,
		OrgID:   orgID,
	}

	result := db.DB.Create(imageSet)
	Expect(result.Error).ToNot(HaveOccurred())
	imageV1 := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit: faker.UUIDHyphenated(),
			OrgID:        orgID,
		},
		Status:     models.ImageStatusSuccess,
		ImageSetID: &imageSet.ID,
		Version:    1,
		OrgID:      orgID,
	}
	result = db.DB.Create(imageV1)
	Expect(result.Error).ToNot(HaveOccurred())
	groupUUID := "123"
	devices := []models.Device{
		{
			Name:      faker.Name(),
			UUID:      faker.UUIDHyphenated(),
			Account:   account,
			OrgID:     orgID,
			ImageID:   imageV1.ID,
			GroupUUID: groupUUID,
		},
		{
			Name:    faker.Name(),
			UUID:    faker.UUIDHyphenated(),
			Account: account,
			OrgID:   orgID,
			ImageID: imageV1.ID,
		},
	}
	result = db.DB.Create(devices)
	Expect(result.Error).ToNot(HaveOccurred())
	deviceView := []models.DeviceView{
		{
			DeviceID:   devices[0].ID,
			DeviceUUID: devices[0].UUID,
			GroupUUID:  devices[0].GroupUUID,
		},
	}

	var mockDeviceService *mock_services.MockDeviceServiceInterface
	var router chi.Router
	var mockServices *dependencies.EdgeAPIServices
	ctrl = gomock.NewController(GinkgoT())

	mockDeviceService = mock_services.NewMockDeviceServiceInterface(ctrl)
	mockServices = &dependencies.EdgeAPIServices{
		DeviceService: mockDeviceService,
		Log:           log.NewEntry(log.StandardLogger()),
	}
	router = chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := dependencies.ContextWithServices(r.Context(), mockServices)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Route("/devices", MakeDevicesRouter)

	mockDeviceService.EXPECT().GetDevicesCount(gomock.Any()).Return(int64(1), nil)
	mockDeviceService.EXPECT().GetDevicesView(30, 0, gomock.Any()).Return(&models.DeviceViewList{Total: 1, Devices: deviceView}, nil)

	url := fmt.Sprintf("/devices/devicesview?groupUUID=%v", groupUUID)
	req, err := http.NewRequest("GET", url, nil)
	fmt.Printf(req.URL.Host)
	if err != nil {
		Expect(err).ToNot(HaveOccurred())
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	Expect(rr.Code).To(Equal(http.StatusOK))

}

func TestEnforceEdgeGroups(t *testing.T) {
	RegisterTestingT(t)
	conf := config.Get()
	initialAuth := conf.Auth
	// restore initial conf properties and env variables
	defer func(auth bool) {
		conf := config.Get()
		conf.Auth = auth
		err := os.Unsetenv(feature.EnforceEdgeGroups.EnvVar)
		Expect(err).ToNot(HaveOccurred())
	}(initialAuth)

	orgID := faker.UUIDHyphenated()

	testCases := []struct {
		Name          string
		EnvVar        string
		HTTPMethod    string
		BodyData      *models.FilterByDevicesAPI
		OrgID         string
		ExpectedValue bool
	}{
		{
			Name:          "should return enforce edge groups value true for http GET method",
			EnvVar:        "true",
			HTTPMethod:    "GET",
			OrgID:         orgID,
			ExpectedValue: true,
		},
		{
			Name:          "should return enforce edge groups value false for http GET method",
			EnvVar:        "",
			HTTPMethod:    "GET",
			OrgID:         faker.UUIDHyphenated(),
			ExpectedValue: false,
		},
		{
			Name:          "should return enforce edge groups value true for http POST method",
			EnvVar:        "true",
			HTTPMethod:    "POST",
			BodyData:      &models.FilterByDevicesAPI{DevicesUUID: []string{faker.UUIDHyphenated()}},
			OrgID:         orgID,
			ExpectedValue: true,
		},
		{
			Name:          "should return enforce edge groups value false for http POST method",
			EnvVar:        "",
			HTTPMethod:    "POST",
			BodyData:      &models.FilterByDevicesAPI{DevicesUUID: []string{faker.UUIDHyphenated()}},
			OrgID:         faker.UUIDHyphenated(),
			ExpectedValue: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			if testCase.EnvVar == "" {
				err := os.Unsetenv(feature.EnforceEdgeGroups.EnvVar)
				Expect(err).ToNot(HaveOccurred())
			} else {
				err := os.Setenv(feature.EnforceEdgeGroups.EnvVar, testCase.EnvVar)
				Expect(err).ToNot(HaveOccurred())
			}

			// set conf  auth to true to force use the supplied identity
			conf.Auth = true

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var mockDeviceService *mock_services.MockDeviceServiceInterface
			var router chi.Router
			var edgeAPIServices *dependencies.EdgeAPIServices
			ctrl = gomock.NewController(GinkgoT())

			mockDeviceService = mock_services.NewMockDeviceServiceInterface(ctrl)
			edgeAPIServices = &dependencies.EdgeAPIServices{
				DeviceService: mockDeviceService,
				Log:           log.NewEntry(log.StandardLogger()),
			}
			router = chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := dependencies.ContextWithServices(r.Context(), edgeAPIServices)
					// set identity orgID
					ctx = context.WithValue(ctx, identity.Key, identity.XRHID{Identity: identity.Identity{OrgID: testCase.OrgID}})
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			router.Route("/devices", MakeDevicesRouter)

			mockDeviceService.EXPECT().GetDevicesCount(gomock.Any()).Return(int64(0), nil)
			mockDeviceService.EXPECT().GetDevicesView(30, 0, gomock.Any()).Return(&models.DeviceViewList{}, nil)

			var body io.Reader
			if testCase.BodyData != nil {
				jsonBodyData, err := json.Marshal(*testCase.BodyData)
				Expect(err).To(BeNil())
				body = bytes.NewBuffer(jsonBodyData)
			}
			req, err := http.NewRequest(testCase.HTTPMethod, "/devices/devicesview", body)
			Expect(err).ToNot(HaveOccurred())

			responseRecorder := httptest.NewRecorder()
			router.ServeHTTP(responseRecorder, req)

			Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			respBody, err := io.ReadAll(responseRecorder.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).ToNot(BeEmpty())

			var responseDevicesView models.DeviceViewListResponseAPI
			err = json.Unmarshal(respBody, &responseDevicesView)
			Expect(err).ToNot(HaveOccurred())
			Expect(responseDevicesView.Data.EnforceEdgeGroups).To(Equal(testCase.ExpectedValue))
		})
	}
}

func TestDevicesViewInventoryHostsRbac(t *testing.T) {
	RegisterTestingT(t)
	conf := config.Get()
	orgID := faker.UUIDHyphenated()

	defer func() {
		_ = os.Unsetenv(feature.EnforceEdgeGroups.EnvVar)
		_ = os.Unsetenv(feature.EdgeParityInventoryRbac.EnvVar)
		_ = os.Unsetenv(feature.EdgeParityInventoryGroupsEnabled.EnvVar)
		config.Get().Auth = false
	}()

	// enable authentication in config
	conf.Auth = true

	// ensure enforce edge groups is disabled
	err := os.Unsetenv(feature.EnforceEdgeGroups.EnvVar)
	Expect(err).ToNot(HaveOccurred())
	// ensure edgeParty inventory rbac feature is enabled
	err = os.Setenv(feature.EdgeParityInventoryRbac.EnvVar, "true")
	Expect(err).ToNot(HaveOccurred())
	// ensure edgeParty inventory groups feature is enabled
	err = os.Setenv(feature.EdgeParityInventoryGroupsEnabled.EnvVar, "true")
	Expect(err).ToNot(HaveOccurred())

	inventoryGroups := []struct {
		ID   string
		Name string
	}{
		{ID: faker.UUIDHyphenated(), Name: faker.Name()},
		{ID: faker.UUIDHyphenated(), Name: faker.Name()},
	}

	image := models.Image{Name: faker.Name(), OrgID: orgID}
	err = db.DB.Create(&image).Error
	Expect(err).ToNot(HaveOccurred())

	// create 3 devices two with different inventory groups and one without inventory group
	devices := []models.Device{
		{OrgID: orgID, UUID: faker.UUIDHyphenated(), GroupUUID: inventoryGroups[0].ID, GroupName: inventoryGroups[0].Name, ImageID: image.ID},
		{OrgID: orgID, UUID: faker.UUIDHyphenated(), GroupUUID: inventoryGroups[1].ID, GroupName: inventoryGroups[1].Name, ImageID: image.ID},
		{OrgID: orgID, UUID: faker.UUIDHyphenated(), ImageID: image.ID},
	}
	err = db.DB.Create(&devices).Error
	Expect(err).ToNot(HaveOccurred())

	// for http post method will include all devices uuids
	PostDefaultData := &models.FilterByDevicesAPI{DevicesUUID: []string{devices[0].UUID, devices[1].UUID, devices[2].UUID}}

	errClientExpectedError := errors.New("expected client error")

	testCases := []struct {
		Name                            string
		HTTPMethod                      string
		BodyData                        *models.FilterByDevicesAPI
		RbacACL                         rbac.AccessList
		UseIdentity                     bool
		UseIdentityType                 string
		ClientCallExpected              bool
		ClientError                     error
		ClientAccessCallExpected        bool
		ClientAccessError               error
		ResultAllowedAccess             bool
		ResultGroupsID                  []string
		ResultHostsWithNoGroupsAssigned bool
		ExpectedDevices                 []models.Device
		ExpectedHTTPStatus              int
		ExpectedErrorMessage            string
	}{
		// GET http method
		{
			Name:       "should return all the devices",
			HTTPMethod: "GET",
			RbacACL: rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{},
					Permission:          "inventory:hosts:read",
				},
			},
			UseIdentity:                     true,
			UseIdentityType:                 common.IdentityTypeUser,
			ClientCallExpected:              true,
			ClientAccessCallExpected:        true,
			ResultAllowedAccess:             true,
			ResultGroupsID:                  []string{},
			ResultHostsWithNoGroupsAssigned: false,
			ExpectedHTTPStatus:              http.StatusOK,
			ExpectedDevices:                 devices,
		},
		{
			Name:       "should return the devices device from groups in ACL",
			HTTPMethod: "GET",
			RbacACL: rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{&inventoryGroups[0].ID},
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
			},
			UseIdentity:                     true,
			UseIdentityType:                 common.IdentityTypeUser,
			ClientCallExpected:              true,
			ClientAccessCallExpected:        true,
			ResultAllowedAccess:             true,
			ResultGroupsID:                  []string{inventoryGroups[0].ID},
			ResultHostsWithNoGroupsAssigned: false,
			ExpectedHTTPStatus:              http.StatusOK,
			ExpectedDevices:                 []models.Device{devices[0]},
		},
		{
			Name:       "should return the devices with no groups",
			HTTPMethod: "GET",
			RbacACL: rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{nil},
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
			},
			UseIdentity:                     true,
			UseIdentityType:                 common.IdentityTypeUser,
			ClientCallExpected:              true,
			ClientAccessCallExpected:        true,
			ResultAllowedAccess:             true,
			ResultGroupsID:                  []string{},
			ResultHostsWithNoGroupsAssigned: true,
			ExpectedHTTPStatus:              http.StatusOK,
			ExpectedDevices:                 []models.Device{devices[2]},
		},
		{
			Name:       "should return the devices with group and with no groups",
			HTTPMethod: "GET",
			RbacACL: rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{&inventoryGroups[0].ID, nil},
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
			},
			UseIdentity:                     true,
			UseIdentityType:                 common.IdentityTypeUser,
			ClientCallExpected:              true,
			ClientAccessCallExpected:        true,
			ClientError:                     nil,
			ResultAllowedAccess:             true,
			ResultGroupsID:                  []string{inventoryGroups[0].ID},
			ResultHostsWithNoGroupsAssigned: true,
			ExpectedHTTPStatus:              http.StatusOK,
			ExpectedDevices:                 []models.Device{devices[0], devices[2]},
		},
		{
			Name:               `should not filter by inventory rbac groups when identity type is not "User"`,
			HTTPMethod:         "GET",
			RbacACL:            rbac.AccessList{},
			UseIdentity:        true,
			UseIdentityType:    "System",
			ClientCallExpected: false,
			ExpectedHTTPStatus: http.StatusOK,
			ExpectedDevices:    devices,
		},
		{
			Name:               "should return error when identity not found",
			HTTPMethod:         "GET",
			RbacACL:            rbac.AccessList{},
			UseIdentity:        false,
			ClientCallExpected: false,
			ExpectedHTTPStatus: http.StatusBadRequest,
		},
		{
			Name:                     "should return error when rbac GetAccessList fails",
			HTTPMethod:               "GET",
			RbacACL:                  rbac.AccessList{},
			UseIdentity:              true,
			UseIdentityType:          common.IdentityTypeUser,
			ClientCallExpected:       true,
			ClientError:              errClientExpectedError,
			ClientAccessCallExpected: false,
			ExpectedHTTPStatus:       http.StatusInternalServerError,
		},
		{
			Name:                     "should return error when rbac GetInventoryGroupsAccess fails",
			HTTPMethod:               "GET",
			RbacACL:                  rbac.AccessList{},
			UseIdentity:              true,
			UseIdentityType:          common.IdentityTypeUser,
			ClientCallExpected:       true,
			ClientAccessCallExpected: true,
			ClientAccessError:        errClientExpectedError,
			ExpectedHTTPStatus:       http.StatusServiceUnavailable,
			ExpectedErrorMessage:     errClientExpectedError.Error(),
		},
		{
			Name:                     "should return error when not allowed access",
			HTTPMethod:               "GET",
			RbacACL:                  rbac.AccessList{},
			UseIdentity:              true,
			UseIdentityType:          common.IdentityTypeUser,
			ClientCallExpected:       true,
			ClientAccessCallExpected: true,
			ResultAllowedAccess:      false,
			ExpectedHTTPStatus:       http.StatusForbidden,
			ExpectedErrorMessage:     "access to hosts is forbidden",
		},
		// POST http method
		{
			Name:       "should return the devices device from groups in ACL",
			HTTPMethod: "POST",
			BodyData:   PostDefaultData,
			RbacACL: rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{&inventoryGroups[0].ID},
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
			},
			UseIdentity:                     true,
			UseIdentityType:                 common.IdentityTypeUser,
			ClientCallExpected:              true,
			ClientAccessCallExpected:        true,
			ResultAllowedAccess:             true,
			ResultGroupsID:                  []string{inventoryGroups[0].ID},
			ResultHostsWithNoGroupsAssigned: false,
			ExpectedHTTPStatus:              http.StatusOK,
			ExpectedDevices:                 []models.Device{devices[0]},
		},
		{
			Name:       "should return the devices with no groups",
			HTTPMethod: "POST",
			BodyData:   PostDefaultData,
			RbacACL: rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{nil},
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
			},
			UseIdentity:                     true,
			UseIdentityType:                 common.IdentityTypeUser,
			ClientCallExpected:              true,
			ClientAccessCallExpected:        true,
			ResultAllowedAccess:             true,
			ResultGroupsID:                  []string{},
			ResultHostsWithNoGroupsAssigned: true,
			ExpectedHTTPStatus:              http.StatusOK,
			ExpectedDevices:                 []models.Device{devices[2]},
		},
		{
			Name:       "should return the devices with group and with no groups",
			HTTPMethod: "POST",
			BodyData:   PostDefaultData,
			RbacACL: rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{&inventoryGroups[0].ID, nil},
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
			},
			UseIdentity:                     true,
			UseIdentityType:                 common.IdentityTypeUser,
			ClientCallExpected:              true,
			ClientAccessCallExpected:        true,
			ClientError:                     nil,
			ResultAllowedAccess:             true,
			ResultGroupsID:                  []string{inventoryGroups[0].ID},
			ResultHostsWithNoGroupsAssigned: true,
			ExpectedHTTPStatus:              http.StatusOK,
			ExpectedDevices:                 []models.Device{devices[0], devices[2]},
		},
		{
			Name:               `should not filter by inventory rbac groups when identity type is not "User"`,
			HTTPMethod:         "POST",
			BodyData:           PostDefaultData,
			RbacACL:            rbac.AccessList{},
			UseIdentity:        true,
			UseIdentityType:    "System",
			ClientCallExpected: false,
			ExpectedHTTPStatus: http.StatusOK,
			ExpectedDevices:    devices,
		},
		{
			Name:               "should return error when identity not found",
			HTTPMethod:         "POST",
			BodyData:           PostDefaultData,
			RbacACL:            rbac.AccessList{},
			UseIdentity:        false,
			ClientCallExpected: false,
			ExpectedHTTPStatus: http.StatusBadRequest,
		},
		{
			Name:                     "should return error when rbac GetAccessList fails",
			HTTPMethod:               "POST",
			BodyData:                 PostDefaultData,
			RbacACL:                  rbac.AccessList{},
			UseIdentity:              true,
			UseIdentityType:          common.IdentityTypeUser,
			ClientCallExpected:       true,
			ClientError:              errClientExpectedError,
			ClientAccessCallExpected: false,
			ExpectedHTTPStatus:       http.StatusInternalServerError,
		},
		{
			Name:                     "should return error when rbac GetInventoryGroupsAccess fails",
			HTTPMethod:               "POST",
			BodyData:                 PostDefaultData,
			RbacACL:                  rbac.AccessList{},
			UseIdentity:              true,
			UseIdentityType:          common.IdentityTypeUser,
			ClientCallExpected:       true,
			ClientAccessCallExpected: true,
			ClientAccessError:        errClientExpectedError,
			ExpectedHTTPStatus:       http.StatusServiceUnavailable,
			ExpectedErrorMessage:     errClientExpectedError.Error(),
		},
		{
			Name:                     "should return error when not allowed access",
			HTTPMethod:               "POST",
			BodyData:                 PostDefaultData,
			RbacACL:                  rbac.AccessList{},
			UseIdentity:              true,
			UseIdentityType:          common.IdentityTypeUser,
			ClientCallExpected:       true,
			ClientAccessCallExpected: true,
			ResultAllowedAccess:      false,
			ExpectedHTTPStatus:       http.StatusForbidden,
			ExpectedErrorMessage:     "access to hosts is forbidden",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			RegisterTestingT(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var router chi.Router
			var edgeAPIServices *dependencies.EdgeAPIServices

			mockRbacClient := mock_rbac.NewMockClientInterface(ctrl)

			router = chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					rLog := log.NewEntry(log.StandardLogger())
					ctx := r.Context()
					if testCase.UseIdentity {
						ctx = context.WithValue(ctx, identity.Key, identity.XRHID{Identity: identity.Identity{OrgID: orgID, Type: testCase.UseIdentityType}})
					}
					edgeAPIServices = &dependencies.EdgeAPIServices{
						DeviceService: services.NewDeviceService(ctx, rLog),
						RbacService:   mockRbacClient,
						Log:           rLog,
					}
					ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)

					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			router.Route("/devices", MakeDevicesRouter)

			if testCase.ClientCallExpected {
				mockRbacClient.EXPECT().GetAccessList(rbac.ApplicationInventory).Return(testCase.RbacACL, testCase.ClientError)
			}
			if testCase.ClientAccessCallExpected {
				mockRbacClient.EXPECT().GetInventoryGroupsAccess(testCase.RbacACL, rbac.ResourceTypeHOSTS, rbac.AccessTypeRead).Return(
					testCase.ResultAllowedAccess, testCase.ResultGroupsID, testCase.ResultHostsWithNoGroupsAssigned, testCase.ClientAccessError,
				)
			}

			var body io.Reader
			if testCase.BodyData != nil {
				jsonBodyData, err := json.Marshal(*testCase.BodyData)
				Expect(err).To(BeNil())
				body = bytes.NewBuffer(jsonBodyData)
			}
			req, err := http.NewRequest(testCase.HTTPMethod, "/devices/devicesview?sort_by=created_at", body)
			Expect(err).ToNot(HaveOccurred())

			responseRecorder := httptest.NewRecorder()
			router.ServeHTTP(responseRecorder, req)

			Expect(responseRecorder.Code).To(Equal(testCase.ExpectedHTTPStatus))
			respBody, err := io.ReadAll(responseRecorder.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).ToNot(BeEmpty())
			if testCase.ExpectedHTTPStatus == http.StatusOK {
				var responseDevicesView models.DeviceViewListResponseAPI
				err = json.Unmarshal(respBody, &responseDevicesView)
				Expect(err).ToNot(HaveOccurred())
				if len(testCase.ExpectedDevices) > 0 {
					Expect(len(responseDevicesView.Data.Devices)).To(Equal(len(testCase.ExpectedDevices)))
					for ind, device := range responseDevicesView.Data.Devices {
						Expect(device.DeviceUUID).To(Equal(testCase.ExpectedDevices[ind].UUID))
					}
				}
			} else if testCase.ExpectedErrorMessage != "" {
				Expect(string(respBody)).To(ContainSubstring(testCase.ExpectedErrorMessage))
			}
		})
	}
}
