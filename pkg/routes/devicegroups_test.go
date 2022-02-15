package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/routes/common"

	"github.com/redhatinsights/edge-api/pkg/services"

	"github.com/golang/mock/gomock"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	log "github.com/sirupsen/logrus"
)

func TestGetAllDeviceGroups(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetAllDeviceGroups)
	ctx := req.Context()
	controller := gomock.NewController(t)
	defer controller.Finish()

	deviceGroupsService := mock_services.NewMockDeviceGroupsServiceInterface(controller)
	deviceGroupsService.EXPECT().GetDeviceGroupsCount(gomock.Any(), gomock.Any()).Return(int64(0), nil)
	deviceGroupsService.EXPECT().GetDeviceGroups(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&[]models.DeviceGroup{}, nil)

	edgeAPIServices := &dependencies.EdgeAPIServices{
		DeviceGroupsService: deviceGroupsService,
		Log:                 log.NewEntry(log.StandardLogger()),
	}

	req = req.WithContext(dependencies.ContextWithServices(ctx, edgeAPIServices))
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)
	}
}

func TestGetAllDeviceGroupsFilterParams(t *testing.T) {
	tt := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
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
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	for _, te := range tt {
		req, err := http.NewRequest("GET", fmt.Sprintf("/device-groups?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		validateGetAllDeviceGroupsFilterParams(next).ServeHTTP(w, req)

		resp := w.Result()
		var jsonBody []validationError
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

func TestCreateGroupWithoutAccount(t *testing.T) {
	config.Get().Debug = false
	jsonRepo := &models.DeviceGroup{
		Name: "Group1",
	}
	jsonRepoBytes, err := json.Marshal(jsonRepo)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(jsonRepoBytes))
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		Log: log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateDeviceGroup)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
	config.Get().Debug = true
}

func TestCreateDeviceGroup(t *testing.T) {
	jsonRepo := &models.DeviceGroup{
		Name:    "Group1",
		Type:    "static",
		Account: "000000",
	}
	jsonRepoBytes, err := json.Marshal(jsonRepo)
	if err != nil {
		t.Errorf(err.Error())
	}
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(jsonRepoBytes))
	if err != nil {
		t.Errorf(err.Error())
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDeviceGroupsService := mock_services.NewMockDeviceGroupsServiceInterface(ctrl)
	mockDeviceGroupsService.EXPECT().CreateDeviceGroup(gomock.Any()).Return(&models.DeviceGroup{}, nil)
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		DeviceGroupsService: mockDeviceGroupsService,
		Log:                 log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateDeviceGroup)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)

	}

}

func TestGetDeviceGroupByID(t *testing.T) {
	deviceGroupID := &models.DeviceGroup{}
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(req.Context(), deviceGroupKey, deviceGroupID)
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	ctx = dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{})
	req = req.WithContext(ctx)
	handler := http.HandlerFunc(GetDeviceGroupByID)

	handler.ServeHTTP(rr, req.WithContext(ctx))
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)

	}
}

func TestGetDeviceGroupByIDInvalid(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(req.Context(), deviceGroupKey, "a")
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	ctx = dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{})
	req = req.WithContext(ctx)
	handler := http.HandlerFunc(GetDeviceGroupByID)

	handler.ServeHTTP(rr, req.WithContext(ctx))
	if status := rr.Code; status == http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)

	}
}

var _ = Describe("DeviceGroup routes", func() {
	var (
		ctrl                    *gomock.Controller
		mockDeviceGroupsService *mock_services.MockDeviceGroupsServiceInterface
		edgeAPIServices         *dependencies.EdgeAPIServices
	)
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockDeviceGroupsService = mock_services.NewMockDeviceGroupsServiceInterface(ctrl)
		edgeAPIServices = &dependencies.EdgeAPIServices{
			DeviceGroupsService: mockDeviceGroupsService,
			Log:                 log.NewEntry(log.StandardLogger()),
		}
		Expect(ctrl).ToNot(BeNil())
		Expect(mockDeviceGroupsService).ToNot(BeNil())
		Expect(edgeAPIServices).ToNot(BeNil())
	})
	AfterEach(func() {
		ctrl.Finish()
	})
	Context("adding devices to DeviceGroup", func() {
		account := faker.UUIDHyphenated()
		deviceGroupName := faker.Name()
		devices := []models.Device{
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
			},
		}
		deviceGroup := models.DeviceGroup{Name: deviceGroupName, Account: account, Type: models.DeviceGroupTypeDefault}
		Context("adding Devices & DeviceGroup to DB", func() {
			for _, device := range devices {
				dbResult := db.DB.Create(&device).Error
				Expect(dbResult).To(BeNil())
			}
			dbResult := db.DB.Create(&deviceGroup).Error
			Expect(dbResult).To(BeNil())
		})

		Context("get DeviceGroup from DB", func() {
			dbResult := db.DB.Where(models.DeviceGroup{Account: account}).First(&deviceGroup).Error
			Expect(dbResult).To(BeNil())
			dbResult = db.DB.Where(models.Device{Account: account}).Find(&devices).Error
			Expect(dbResult).To(BeNil())
		})
		jsonDeviceBytes, err := json.Marshal(models.DeviceGroup{Devices: devices})
		Expect(err).To(BeNil())

		url := fmt.Sprintf("/%d/devices", deviceGroup.ID)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonDeviceBytes))
		Expect(err).To(BeNil())

		When("all is valid", func() {
			It("should add devices to DeviceGroup", func() {
				ctx := req.Context()
				ctx = setContextDeviceGroup(ctx, &deviceGroup)
				ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
				req = req.WithContext(ctx)
				rr := httptest.NewRecorder()

				// setup mock for DeviceGroupsService
				mockDeviceGroupsService.EXPECT().AddDeviceGroupDevices(account, deviceGroup.ID, devices).Return(&devices, nil)

				handler := http.HandlerFunc(AddDeviceGroupDevices)
				handler.ServeHTTP(rr, req)
				// Check the status code is what we expect.
				Expect(rr.Code).To(Equal(http.StatusOK))
			})
		})
	})
	Context("update DeviceGroup", func() {
		updDevice := &models.DeviceGroup{
			Name:    "Group1",
			Type:    models.DeviceGroupTypeDefault,
			Account: common.DefaultAccount,
		}
		jsonDeviceBytes, err := json.Marshal(updDevice)
		Expect(err).To(BeNil())

		url := fmt.Sprintf("/%d", updDevice.ID)
		req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonDeviceBytes))
		Expect(err).To(BeNil())

		When("all is valid", func() {
			It("should update DeviceGroup", func() {
				ctx := req.Context()
				ctx = setContextDeviceGroup(ctx, updDevice)
				ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
				req = req.WithContext(ctx)
				rr := httptest.NewRecorder()

				// setup mock for DeviceGroupsService
				mockDeviceGroupsService.EXPECT().GetDeviceGroupByID(fmt.Sprintf("%d", updDevice.ID)).Return(updDevice, nil)
				mockDeviceGroupsService.EXPECT().UpdateDeviceGroup(updDevice, common.DefaultAccount, fmt.Sprintf("%d", updDevice.ID)).Return(nil)

				handler := http.HandlerFunc(UpdateDeviceGroup)
				handler.ServeHTTP(rr, req)
				// Check the status code is what we expect.
				Expect(rr.Code).To(Equal(http.StatusOK))
			})
		})
	})
	Context("delete DeviceGroup", func() {
		account := common.DefaultAccount
		deviceGroupName := faker.Name()
		devices := []models.Device{
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
			},
		}
		deviceGroup := &models.DeviceGroup{
			Name:    deviceGroupName,
			Type:    models.DeviceGroupTypeDefault,
			Account: account,
			Devices: devices,
		}
		Context("saving DeviceGroup", func() {
			dbResult := db.DB.Create(&deviceGroup).Error
			Expect(dbResult).To(BeNil())
		})
		Context("getting DeviceGroup", func() {
			dbResult := db.DB.Where(models.DeviceGroup{Name: deviceGroupName, Account: account}).First(&deviceGroup).Error
			Expect(dbResult).To(BeNil())
		})
		When("all is valid", func() {
			url := fmt.Sprintf("/%d", deviceGroup.ID)
			req, err := http.NewRequest(http.MethodDelete, url, nil)
			Expect(err).To(BeNil())

			It("should return status code 200", func() {
				ctx := req.Context()
				ctx = setContextDeviceGroup(ctx, deviceGroup)
				ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
				req = req.WithContext(ctx)
				rr := httptest.NewRecorder()

				// setup mock for DeviceGroupsService
				mockDeviceGroupsService.EXPECT().DeleteDeviceGroupByID(fmt.Sprintf("%d", deviceGroup.ID)).Return(nil)

				handler := http.HandlerFunc(DeleteDeviceGroupByID)
				handler.ServeHTTP(rr, req)
				// Check the status code is what we expect.
				Expect(rr.Code).To(Equal(http.StatusOK))
			})
		})
		When("no device group in context", func() {
			url := fmt.Sprintf("/%d", deviceGroup.ID)
			req, err := http.NewRequest(http.MethodDelete, url, nil)
			Expect(err).To(BeNil())

			It("should return status code 400", func() {
				ctx := req.Context()
				ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
				req = req.WithContext(ctx)
				rr := httptest.NewRecorder()

				handler := http.HandlerFunc(DeleteDeviceGroupByID)
				handler.ServeHTTP(rr, req)
				// Check the status code is what we expect.
				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})
		})
		When("no account", func() {
			fakeID, _ := faker.RandomInt(1000, 2000, 1)
			fakeIDUint := uint(fakeID[0])
			url := fmt.Sprintf("/%d", fakeIDUint)
			req, err := http.NewRequest(http.MethodDelete, url, nil)
			Expect(err).To(BeNil())

			It("should return status code 400", func() {
				ctx := req.Context()
				ctx = setContextDeviceGroup(ctx, &models.DeviceGroup{
					Model: models.Model{
						ID: fakeIDUint,
					},
					Account: "",
				})
				ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
				req = req.WithContext(ctx)
				rr := httptest.NewRecorder()

				// setup mock for DeviceGroupsService
				mockDeviceGroupsService.EXPECT().DeleteDeviceGroupByID(fmt.Sprint(fakeIDUint)).Return(new(services.AccountNotSet))

				handler := http.HandlerFunc(DeleteDeviceGroupByID)
				handler.ServeHTTP(rr, req)
				// Check the status code is what we expect.
				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})
		})
		When("no such ID", func() {
			fakeID, _ := faker.RandomInt(1000, 2000, 1)
			fakeIDUint := uint(fakeID[0])
			url := fmt.Sprintf("/%d", fakeIDUint)
			req, err := http.NewRequest(http.MethodDelete, url, nil)
			Expect(err).To(BeNil())

			It("should return status code 404", func() {
				ctx := req.Context()
				ctx = setContextDeviceGroup(ctx, &models.DeviceGroup{
					Model: models.Model{
						ID: fakeIDUint,
					},
				})
				ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
				req = req.WithContext(ctx)
				rr := httptest.NewRecorder()

				// setup mock for DeviceGroupsService
				mockDeviceGroupsService.EXPECT().DeleteDeviceGroupByID(fmt.Sprint(fakeIDUint)).Return(new(services.DeviceGroupNotFound))

				handler := http.HandlerFunc(DeleteDeviceGroupByID)
				handler.ServeHTTP(rr, req)
				// Check the status code is what we expect.
				Expect(rr.Code).To(Equal(http.StatusNotFound))
			})
		})
		When("something bad happened", func() {
			fakeID, _ := faker.RandomInt(1000, 2000, 1)
			fakeIDUint := uint(fakeID[0])
			url := fmt.Sprintf("/%d", fakeIDUint)
			req, err := http.NewRequest(http.MethodDelete, url, nil)
			Expect(err).To(BeNil())

			It("should return status code 500", func() {
				ctx := req.Context()
				ctx = setContextDeviceGroup(ctx, &models.DeviceGroup{
					Model: models.Model{
						ID: fakeIDUint,
					},
				})
				ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
				req = req.WithContext(ctx)
				rr := httptest.NewRecorder()

				// setup mock for DeviceGroupsService
				mockDeviceGroupsService.EXPECT().DeleteDeviceGroupByID(fmt.Sprint(fakeIDUint)).Return(errors.NewInternalServerError())

				handler := http.HandlerFunc(DeleteDeviceGroupByID)
				handler.ServeHTTP(rr, req)
				// Check the status code is what we expect.
				Expect(rr.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})
})
