package devices

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/config"
	log "github.com/sirupsen/logrus"
)

// Inventory list of devices
type Inventory struct {
	Total  int       `json:"total"`
	Count  int       `json:"count"`
	Result []devices `json:"results"`
}

type devices struct {
	ID     string        `json:"id"`
	Ostree systemProfile `json:"system_profile"`
}

type systemProfile struct {
	RpmOstreeDeployments []ostree `json:"rpm_ostree_deployments"`
}

type ostree struct {
	Checksum string `json:"checksum"`
	Booted   string `json:"booted"`
}

const (
	inventoryAPI = "api/inventory/v1/hosts"
	orderBy      = "updated"
	orderHow     = "DESC"
	filterParams = "?filter[system_profile][host_type]=edge&fields[system_profile]=host_type,operating_system,greenboot_status,greenboot_fallback_detected,rpm_ostree_deployments"
)

// ReturnDevices will return the list of devices without filter by tag or uuid
func ReturnDevices(w http.ResponseWriter, r *http.Request) (Inventory, error) {
	url := fmt.Sprintf("%s/api/inventory/v1/hosts", config.Get().InventoryConfig.URL)
	fullURL := url + filterParams
	log.Infof("Requesting url: %s\n", fullURL)
	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Inventory{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Inventory{}, err
	}
	defer resp.Body.Close()
	var bodyResp Inventory
	json.Unmarshal([]byte(body), &bodyResp)
	log.Infof("struct: %v\n", bodyResp)
	return bodyResp, nil

}

// ReturnDevicesByID will return the list of devices by uuid
func ReturnDevicesByID(w http.ResponseWriter, r *http.Request) (Inventory, error) {
	deviceID := chi.URLParam(r, "device_uuid")
	deviceIDParam := "&hostname_or_id=" + deviceID

	url := fmt.Sprintf("%s/api/inventory/v1/hosts", config.Get().InventoryConfig.URL)
	fullURL := url + filterParams + deviceIDParam
	log.Infof("Requesting url: %s\n", fullURL)
	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return Inventory{}, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return Inventory{}, fmt.Errorf("error requesting inventory, got status code %d and body %s", resp.StatusCode, body)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Inventory{}, err
	}
	log.Infof("fullbody: %v\n", string(body))
	defer resp.Body.Close()
	var bodyResp Inventory
	json.Unmarshal([]byte(body), &bodyResp)
	log.Infof("struct: %v\n", bodyResp)

	return bodyResp, nil

}

// ReturnDevicesByTag will return the list of devices by tag
func ReturnDevicesByTag(w http.ResponseWriter, r *http.Request) (Inventory, error) {

	tags := chi.URLParam(r, "devices")
	tagsParam := "?tags=" + tags

	url := fmt.Sprintf("%s/api/inventory/v1/hosts", config.Get().InventoryConfig.URL)
	fullURL := url + filterParams + tagsParam
	log.Infof("Requesting url: %s\n", fullURL)
	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Inventory{}, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return Inventory{}, fmt.Errorf("error requesting inventory, got status code %d and body %s", resp.StatusCode, body)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Inventory{}, err
	}
	var bodyResp Inventory
	json.Unmarshal([]byte(body), &bodyResp)
	log.Infof("struct: %v\n", bodyResp)
	return bodyResp, nil
}
