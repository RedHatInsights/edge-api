package routes_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/bxcodec/faker/v3"
	"github.com/go-chi/chi"
	"github.com/golang/mock/gomock"
	log "github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes"
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
