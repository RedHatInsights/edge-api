package updates

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/pkg/devices"
)

func MakeRouter(sub chi.Router) {
	sub.Get("/", getDevices)
	sub.Route("/device_uuid={device_uuid}", func(r chi.Router) {
		r.Get("/", getDevicesById)
	})
	sub.Route("/tags={tags}", func(r chi.Router) {
		r.Get("/", getDevicesByTag)
	})
}

func getDevices(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("getDevices")
	devices, err := devices.ReturnDevices(w, r)
	fmt.Printf("devices: %v\n", devices)
	if err != nil {
		return
	}
}

func getDevicesById(w http.ResponseWriter, r *http.Request) {
	uuid, tags := parseParams(r)
	fmt.Printf("tags: %v\n", len(tags))
	fmt.Printf("getDevicesById: %v\n", len(uuid))
	if len(uuid) > 0 {
		validUuid := isUuid(uuid)
		if validUuid {
			devices, err := devices.ReturnDevicesById(w, r)
			//FIXME: Load results into DB
			fmt.Printf("validUuid devices: %v\n", devices)
			if err != nil {
				return
			}
		}
	}

}
func getDevicesByTag(w http.ResponseWriter, r *http.Request) {
	uuid, tags := parseParams(r)
	fmt.Printf("tags: %v\n", len(uuid))
	fmt.Printf("getDevicesByTag: %v\n", len(tags))
	if len(tags) > 0 {
		devices, err := devices.ReturnDevicesByTag(w, r)
		fmt.Printf("devices: %v\n", devices)
		if err != nil {
			return
		}

	}

}

func parseParams(r *http.Request) (string, string) {
	uuid := chi.URLParam(r, "device_uuid")
	tags := chi.URLParam(r, "tags")
	fmt.Printf("uuid: %v\n", uuid)
	fmt.Printf("tags: %v\n", tags)
	return uuid, tags
}

//FIXME: Identify better option to see if is uniq or tag
func isUuid(param string) bool {
	_, err := uuid.Parse(param)
	return err == nil

}
