package updates

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/pkg/devices"
	"github.com/redhatinsights/edge-api/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// MakeRouter adds support for operations on update
func MakeRouter(sub chi.Router) {
	sub.Get("/", deviceCtx)

}

func deviceCtx(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("getDevices \n")
	device_uuid := r.URL.Query().Get("device_uuid")
	log.Infof("updates::deviceCtx::device_uuid: %s", device_uuid)
	tag := r.URL.Query().Get("tag")
	log.Infof("updates::deviceCtx::tag: %s", tag)

	if device_uuid != "" {
		getDevicesByID(w, r)
	}
	if tag != "" {
		getDevicesByTag(w, r)
	}
}

func getDevicesByID(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("device_uuid")
	log.Debugf("updates::deviceCtx::uuid: %s", uuid)
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
			json.NewEncoder(w).Encode(&devices)
		} else {
			err := errors.NewInternalServerError()
			err.Title = fmt.Sprintf("Invalid UUID - %s", uuid)
			w.WriteHeader(err.Status)
			return
		}
	}

}
func getDevicesByTag(w http.ResponseWriter, r *http.Request) {
	tags := r.URL.Query().Get("tag")
	log.Debugf("updates::getDevicesByTag::tag: %s", tags)
	if len(tags) > 0 {
		devices, err := devices.ReturnDevicesByTag(w, r)
		fmt.Printf("devices: %v\n", devices)
		if err != nil {
			err := errors.NewInternalServerError()
			err.Title = fmt.Sprintf("Failed to get devices from tag %s", tags)
			w.WriteHeader(err.Status)
			return
		}
		json.NewEncoder(w).Encode(&devices)

	}

}

//FIXME: Identify better option to see if is uniq or tag
func isUUID(param string) bool {
	_, err := uuid.Parse(param)
	return err == nil

}
