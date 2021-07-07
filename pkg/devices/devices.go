package devices

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/config"
)

type Inventory struct {
	Total  int       `json:"total"`
	Count  int       `json:"count"`
	Result []Devices `json:"results"`
}

type Devices struct {
	IpAddresses []string      `json:"ip_addresses"`
	Uuid        string        `json:"bios_uuid"`
	SP          SystemProfile `json:"system_profile"`
}

type SystemProfile struct {
	RpmOstreeDeployments []Ostree `json:"rpm_ostree_deployments"`
}

type Ostree struct {
	Checksum string `json:"checksum"`
	Booted   string `json:"booted"`
}

type key int

const (
	inventoryAPI     = "api/inventory/v1/hosts"
	orderBy          = "updated"
	orderHow         = "DESC"
	filterParams     = "?filter[system_profile][host_type]=edge&fields[system_profile]=host_type,operating_system,greenboot_status,greenboot_fallback_detected,rpm_ostree_deployments"
	commitKey    key = 0
)

func ReturnDevices(w http.ResponseWriter, r *http.Request) (Inventory, error) {
	url := fmt.Sprintf("%s/api/inventory/v1/hosts", config.Get().InventoryConfig.URL)
	fullUrl := url + filterParams
	fmt.Printf("Requesting url: %s\n", fullUrl)
	req, _ := http.NewRequest("GET", fullUrl, nil)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Inventory{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	var bodyResp Inventory
	json.Unmarshal([]byte(body), &bodyResp)
	fmt.Printf("struct: %v\n", bodyResp)
	return bodyResp, nil

}

func ReturnDevicesById(w http.ResponseWriter, r *http.Request) (Inventory, error) {
	deviceId := chi.URLParam(r, "device_uuid")
	deviceIdParam := "&hostname_or_id=" + deviceId
	url := fmt.Sprintf("%s/api/inventory/v1/hosts", config.Get().InventoryConfig.URL)
	fullUrl := url + filterParams + deviceIdParam
	fmt.Printf("Requesting url: %s\n", fullUrl)
	req, _ := http.NewRequest("GET", fullUrl, nil)
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
	fmt.Printf("fullbody: %v\n", string(body))
	defer resp.Body.Close()
	var bodyResp Inventory
	json.Unmarshal([]byte(body), &bodyResp)
	fmt.Printf("struct: %v\n", bodyResp)

	return bodyResp, nil

}

func ReturnDevicesByTag(w http.ResponseWriter, r *http.Request) (Inventory, error) {

	tags := chi.URLParam(r, "devices")
	tagsParam := "?tags=" + tags

	url := fmt.Sprintf("%s/api/inventory/v1/hosts", config.Get().InventoryConfig.URL)
	fullUrl := url + filterParams + tagsParam
	fmt.Printf("Requesting url: %s\n", fullUrl)
	req, _ := http.NewRequest("GET", fullUrl, nil)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Inventory{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)

	var bodyResp Inventory
	json.Unmarshal([]byte(body), &bodyResp)
	fmt.Printf("struct: %v\n", bodyResp)
	return bodyResp, nil
}
