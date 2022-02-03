package routes

import (
	"encoding/json"
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	"net/http"
	"net/http/httptest"
	"testing"

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
