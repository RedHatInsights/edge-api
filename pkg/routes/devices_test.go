package routes_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/go-chi/chi"
	"github.com/golang/mock/gomock"
	log "github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
)

var _ = Describe("Devices Router", func() {
	var deviceUUID string
	var mockDeviceService *mock_services.MockDeviceServiceInterface
	var router chi.Router

	BeforeEach(func() {
		// Given
		deviceUUID = faker.UUIDHyphenated()
		ctrl := gomock.NewController(GinkgoT())
		defer ctrl.Finish()

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
		router.Route("/devices", routes.MakeDevicesRouter)
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
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), false).Return(nil, new(services.DeviceNotFoundError))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			})
		})
		When("device UUID is passed", func() {
			BeforeEach(func() {
				var err error
				req, err = http.NewRequest("GET", fmt.Sprintf("/devices/%s/updates", deviceUUID), nil)
				Expect(err).ToNot(HaveOccurred())
			})
			It("should fail when device is not found", func() {
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), false).Return(nil, new(services.DeviceNotFoundError))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))
			})
			It("should fail when unexpected error happens", func() {
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), false).Return(nil, errors.New("random error"))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			})
			It("should return when everything is okay", func() {
				updates := make([]models.ImageUpdateAvailable, 0)
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), false).Return(updates, nil)
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
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), true).Return(nil, new(services.DeviceNotFoundError))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			})
		})
		When("device UUID is passed", func() {
			BeforeEach(func() {
				var err error
				req, err = http.NewRequest("GET", fmt.Sprintf("/devices/%s/updates?latest=true", deviceUUID), nil)
				Expect(err).ToNot(HaveOccurred())
			})
			It("should fail when device is not found", func() {
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), true).Return(nil, new(services.DeviceNotFoundError))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))
			})
			It("should fail when unexpected error happens", func() {
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), true).Return(nil, errors.New("random error"))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			})
			It("should return when everything is okay", func() {
				updates := make([]models.ImageUpdateAvailable, 0)
				mockDeviceService.EXPECT().GetUpdateAvailableForDeviceByUUID(gomock.Eq(deviceUUID), true).Return(updates, nil)
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
				handler := http.HandlerFunc(routes.GetDevice)
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

	BeforeEach(func() {
		// Given
		ctrl := gomock.NewController(GinkgoT())
		defer ctrl.Finish()

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
		router.Route("/devicesview", routes.MakeDevicesRouter)
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
				handler := http.HandlerFunc(routes.GetDevicesView)

				ctx := dependencies.ContextWithServices(req.Context(), mockServices)
				req = req.WithContext(ctx)
				handler.ServeHTTP(rr, req)
				Expect(rr.Code).To(Equal(http.StatusOK))

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

type validationError struct {
	Key    string
	Reason string
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

		routes.ValidateGetDevicesViewFilterParams(next).ServeHTTP(w, req)

		resp := w.Result()
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
