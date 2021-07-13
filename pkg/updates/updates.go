package updates

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/devices"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
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
			fmt.Printf("validUuid devices: %v\n", devices)
			if err != nil {
				err := errors.NewInternalServerError()
				err.Title = fmt.Sprintf("Failed to get device %s", uuid)
				w.WriteHeader(err.Status)
				return
			}
			json.NewEncoder(w).Encode(&devices)
		} else {
			err := errors.NewBadRequest("Invalid UUID")
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

func updateOSTree(w http.ResponseWriter, r *http.Request) {

	var updateRec models.UpdateRecord
	var inventory devices.Inventory
	inventoryHosts := updateRec.InventoryHosts
	oldCommits := updateRec.OldCommits
	deviceUUID := r.URL.Query().Get("device_uuid")
	log.Infof("updates::deviceCtx::deviceUUID: %s", deviceUUID)
	tag := r.URL.Query().Get("tag")
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal([]byte(reqBody), &updateRec)
	if err != nil {
		return
	}

	if tag != "" {
		// - query Hosted Inventory for all devices in Inventory Tag
		inventory, err = devices.ReturnDevicesByTag(w, r)
	} else {
		if deviceUUID != "" {
			// - query Hosted Inventory for device UUID
			inventory, err = devices.ReturnDevicesByID(w, r)
		}
	}
	if err != nil {
		err := errors.NewInternalServerError()
		err.Title = fmt.Sprintf("No devices in this tag %s", updateRec.Tag)
		w.WriteHeader(err.Status)
		return
	}
	// - populate the updateRec.InventoryHosts []Device data
	fmt.Printf("Devices in this tag %v", inventory.Result)
	for _, device := range inventory.Result {
		updateDevice := new(models.Device)
		updateDevice.UUID = device.ID
		updateDevice.DesiredHash = updateRec.Commit.OSTreeCommit
		inventoryHosts = append(inventoryHosts, *updateDevice)
		updateRec.InventoryHosts = inventoryHosts
		for _, ostreeDeployment := range device.Ostree.RpmOstreeDeployments {
			if ostreeDeployment.Booted {
				var oldCommit models.Commit
				result := db.DB.Where("ostreecommit = ?", ostreeDeployment.Checksum).Take(&oldCommit)
				if result.Error != nil {
					http.Error(w, result.Error.Error(), http.StatusBadRequest)
					return
				}
				oldCommits = append(oldCommits, oldCommit)
				updateRec.OldCommits = oldCommits
			}
		}

	}

	// FIXME - need to remove duplicate OldCommit values from UpdateRecord

	json.NewEncoder(w).Encode(&updateRec)
	db.DB.Create(&updateRec)

	// call RepoBuilderInstance
	// go commits.RepoBuilderInstance(updateRec)

}

func isUUID(param string) bool {
	_, err := uuid.Parse(param)
	return err == nil

}
