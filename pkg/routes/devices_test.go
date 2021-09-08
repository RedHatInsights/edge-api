package routes

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/models"
)

func TestGetDevicesStatus(t *testing.T) {
	tt := []struct {
		name         string
		searchUUID   string
		expectedHash string
	}{
		{
			name:         "display devices for uuid under account (0000000)",
			searchUUID:   "1",
			expectedHash: "11",
		},
		{
			name:         "no devices for uuid not under account (0000000)",
			searchUUID:   "3",
			expectedHash: "",
		},
	}

	for _, te := range tt {
		req, err := http.NewRequest("GET", "/", nil)
		if err != nil {
			t.Errorf("%s: Failed creating a new request: %s", te.name, err)
			return
		}
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
			URLParams: chi.RouteParams{
				Keys:   []string{"DeviceUUID"},
				Values: []string{te.searchUUID},
			},
		})
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(GetDeviceStatus)
		handler.ServeHTTP(rr, req.WithContext(ctx))

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("%s: handler returned wrong status code: got %v want %v", te.name, status, http.StatusOK)
			return
		}
		var dvcs []models.Device
		respBody, err := ioutil.ReadAll(rr.Body)
		if err != nil {
			t.Errorf("%s: Failed reading response body: %s", te.name, err.Error())
			return
		}

		err = json.Unmarshal(respBody, &dvcs)
		if err != nil {
			t.Errorf("%s: Failed unmarshaling json from the response body: %s", te.name, err.Error())
			return
		}

		if te.expectedHash == "" && len(dvcs) > 0 {
			t.Errorf("%s was expecting not to have any results but got %+v", te.name, dvcs)
			return
		}
		for _, dvc := range dvcs {
			if dvc.UUID != te.searchUUID {
				t.Errorf("%s was expecting UUID to be %s but got %s", te.name, te.searchUUID, dvc.UUID)
			}
			if dvc.DesiredHash != te.expectedHash {
				t.Errorf("%s was expecting hash to be %s but got %s", te.name, te.expectedHash, dvc.DesiredHash)
			}
		}
	}
}
func TestGetImageInfo(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{
			Keys:   []string{"DeviceUUID"},
		},
	})
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetDeviceImageInfo)
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return
	}

	var ir models.Image
	respBody, err := ioutil.ReadAll(rr.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	err = json.Unmarshal(respBody, &ir)
	if err != nil {
		t.Errorf(err.Error())
	}

	if ir.ID != testImage.ID {
		t.Errorf("wrong image status: got %v want %v",
			ir.ID, testImage.ID)
	}
}