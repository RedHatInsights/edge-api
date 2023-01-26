// FIXME: golangci-lint
// nolint:revive,typecheck
package routes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/go-chi/chi"
	"github.com/golang/mock/gomock"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory/mock_inventory"
	log "github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
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
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), false, 30, 0).Return(nil, int64(0), new(services.DeviceNotFoundError))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))
			})
			It("should fail when unexpected error happens", func() {
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), false, 30, 0).Return(nil, int64(0), errors.New("random error"))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			})
			It("should return when everything is okay", func() {
				updates := make([]models.ImageUpdateAvailable, 0)
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), false, 30, 0).Return(updates, int64(0), nil)
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
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), true, 30, 0).Return(nil, int64(0), new(services.DeviceNotFoundError))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))
			})
			It("should fail when unexpected error happens", func() {
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), true, 30, 0).Return(nil, int64(0), errors.New("random error"))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			})
			It("should return when everything is okay", func() {
				updates := make([]models.ImageUpdateAvailable, 0)
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), true, 30, 0).Return(updates, int64(0), nil)
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
			orgID := common.DefaultOrgID

			imageSet = &models.ImageSet{
				Name:    fmt.Sprintf("image-test-%s", faker.UUIDHyphenated()),
				Version: 1,
				OrgID:   orgID,
			}
			db.DB.Create(imageSet)

			image = &models.Image{
				Commit: &models.Commit{
					OSTreeCommit: faker.UUIDHyphenated(),
					InstalledPackages: []models.InstalledPackage{
						{
							Name:    "ansible",
							Version: "1.0.0",
						},
						{
							Name:    "yum",
							Version: "2:6.0-1",
						},
					},
					OrgID: orgID,
				},
				Version:    1,
				Status:     models.ImageStatusSuccess,
				ImageSetID: &imageSet.ID,
				OrgID:      orgID,
			}

			db.DB.Create(image.Commit)
			db.DB.Create(image)

			device = &models.Device{
				UUID:            faker.UUIDHyphenated(),
				RHCClientID:     faker.UUIDHyphenated(),
				UpdateAvailable: false,
				ImageID:         image.ID,
				OrgID:           orgID,
			}

			db.DB.Create(&device)

			imageUpdate = &models.Image{
				Commit: &models.Commit{
					OSTreeCommit: faker.UUIDHyphenated(),
					InstalledPackages: []models.InstalledPackage{
						{
							Name:    "ansible",
							Version: "1.0.0",
						},
						{
							Name:    "yum",
							Version: "2:6.0-1",
						},
					},
					OrgID: orgID,
				},
				Version:    2,
				Status:     models.ImageStatusSuccess,
				ImageSetID: &imageSet.ID,
				OrgID:      orgID,
			}

			db.DB.Create(imageUpdate.Commit)
			db.DB.Create(imageUpdate)
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
