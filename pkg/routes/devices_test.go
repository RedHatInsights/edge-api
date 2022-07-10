package routes_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/bxcodec/faker/v3"
	"github.com/go-chi/chi"
	"github.com/golang/mock/gomock"
	log "github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
)

type validationError struct {
	Key    string
	Reason string
}

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
	Context("get list of devices", func() {
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
	Context("get all devices with filter parameters", func() {
		tt := []struct {
			name          string
			params        string
			expectedError []validationError
		}{
			{
				name:   "bad uuid",
				params: "uuid=123456789",
				expectedError: []validationError{
					{Key: "uuid", Reason: `invalid UUID length: 9`},
				},
			},
			{
				name:   "bad created_at date",
				params: "created_at=today",
				expectedError: []validationError{
					{Key: "created_at", Reason: `parsing time "today" as "2006-01-02": cannot parse "today" as "2006"`},
				},
			},
			{
				name:   "bad sort_by",
				params: "sort_by=test",
				expectedError: []validationError{
					{Key: "sort_by", Reason: "test is not a valid sort_by. Sort-by must be name or created_at or updated_at"},
				},
			},
			{
				name:   "invalid query param",
				params: "bla=1",
				expectedError: []validationError{
					{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: [%s]", strings.Join(common.GetDevicesFiltersArray(), ", "))},
				},
			},
			{
				name:   "valid query param and invalid query param",
				params: "sort_by=created_at&bla=1",
				expectedError: []validationError{
					{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: [%s]", strings.Join(common.GetDevicesFiltersArray(), ", "))},
				},
			},
			{
				name:   "invalid query param and valid query param",
				params: "bla=1&sort_by=created_at",
				expectedError: []validationError{
					{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: [%s]", strings.Join(common.GetDevicesFiltersArray(), ", "))},
				},
			},
		}

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
		for _, te := range tt {
			req, err := http.NewRequest("GET", fmt.Sprintf("/device-groups?%s", te.params), nil)
			Expect(err).ToNot(HaveOccurred())
			w := httptest.NewRecorder()
			routes.ValidateGetAllDevicesFilterParams(next).ServeHTTP(w, req)

			resp := w.Result()
			var jsonBody []validationError
			err = json.NewDecoder(resp.Body).Decode(&jsonBody)
			Expect(err).ToNot(HaveOccurred())
			for _, exErr := range te.expectedError {
				found := false
				for _, jsErr := range jsonBody {
					if jsErr.Key == exErr.Key && jsErr.Reason == exErr.Reason {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), fmt.Sprintf("in %q: was expected to have %v but not found in %v", te.name, exErr, jsonBody))
			}
		}
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
