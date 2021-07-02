package updates

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/pkg/devices"
)

func MakeRouter(sub chi.Router) {
	sub.Get("/", GetDevices)
	sub.Route("/{devices}", func(r chi.Router) {
		r.Get("/", GetDevices)
	})
}

func GetDevices(w http.ResponseWriter, r *http.Request) {
	params := parseParams(r)
	if params != "" {
		validUuid := isUuid(params)
		if validUuid {
			devices, err := devices.ReturnDevicesById(w, r)
			//FIXME: Load results into DB
			// var d models.Device
			// for _, device :=  devices {
			// 	d.UUID = device.Uuid
			// 	d.ConnectionState = len(device.IpAddresses) ==0 -> how is the better way to check connectivity?
			// }
			fmt.Printf("devices: %v\n", devices)
			if err != nil {
				return
			}
		} else {
			devices, err := devices.ReturnDevicesByTag(w, r)
			fmt.Printf("devices: %v\n", devices)
			if err != nil {
				return
			}
		}
	}

}

func parseParams(r *http.Request) string {
	param := chi.URLParam(r, "devices")
	fmt.Printf("param: %v\n", param)
	return param
}

//FIXME: Identify better option to see if is uniq or tag
func isUuid(param string) bool {
	_, err := uuid.Parse(param)
	return err == nil

}
