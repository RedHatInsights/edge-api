package updates

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/pkg/devices"
	"github.com/redhatinsights/edge-api/pkg/errors"
)

// MakeRouter adds support for operations on update
func MakeRouter(sub chi.Router) {
	sub.Get("/{device}", deviceCtx)

}

func deviceCtx(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("getDevices \n")
	param := parseParams(r)
	fmt.Printf("getDevices %s\n", param)
	if isUUID(param) {
		getDevicesByID(w, r)
	} else {
		getDevicesByTag(w, r)
	}
}

func getDevicesByID(w http.ResponseWriter, r *http.Request) {
	uuid := parseParams(r)
	fmt.Printf("getDevicesById: %v\n", len(uuid))
	if len(uuid) > 0 {
		validUUID := isUUID(uuid)
		if validUUID {
			devices, err := devices.ReturnDevicesByID(w, r)
			//FIXME: Load results into DB
			fmt.Printf("validUuid devices: %v\n", devices)
			if err != nil {
				err := errors.NewInternalServerError()
				err.Title = fmt.Sprintf("Failed to get device %s", uuid)
				w.WriteHeader(err.Status)
				return
			}
		} else {
			err := errors.NewInternalServerError()
			err.Title = fmt.Sprintf("Invalid UUID - %s", uuid)
			w.WriteHeader(err.Status)
			return
		}
	}

}
func getDevicesByTag(w http.ResponseWriter, r *http.Request) {
	tags := parseParams(r)
	fmt.Printf("getDevicesByTag: %v\n", len(tags))
	if len(tags) > 0 {
		devices, err := devices.ReturnDevicesByTag(w, r)
		fmt.Printf("devices: %v\n", devices)
		if err != nil {
			err := errors.NewInternalServerError()
			err.Title = fmt.Sprintf("Failed to get devices from tag %s", tags)
			w.WriteHeader(err.Status)
			return
		}

	}

}

func parseParams(r *http.Request) string {
	param := chi.URLParam(r, "device")
	fmt.Printf("param: %v\n", param)
	return param
}

//FIXME: Identify better option to see if is uniq or tag
func isUUID(param string) bool {
	_, err := uuid.Parse(param)
	return err == nil

}
