package devices

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/config"
)

type Inventory struct {
	Total  int       `json:"total"`
	Count  int       `json:"count"`
	Result []Devices `json:"results"`
}

type Devices struct {
	IpAddresses []string `json:"ip_addresses"`
	Uuid        string   `json:"bios_uuid"`
}
type key int

const (
	PROXY            = "http://squid.corp.redhat.com:3128"
	inventoryAPI     = "api/inventory/v1/hosts"
	orderBy          = "updated"
	orderHow         = "DESC"
	filterParams     = "?filter[system_profile][host_type]=edge&fields[system_profile][]=host_type"
	usr              = "insights-qa"
	pwd              = "redhat"
	commitKey    key = 0
)

// must move to HTTP_PROXY
func setProxy() *http.Transport {
	proxyURL, err := url.Parse(PROXY)
	if err != nil {
		return &http.Transport{}
	}
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	return transport
}
func setUrl() string {
	inventoryUrl, err := url.Parse(config.Get().InventoryConfig.URL)
	if err != nil {
		return "Error to parse inventory url"
	}
	inventoryUrl.Path = path.Join(inventoryUrl.Path, inventoryAPI)
	fullUrl := inventoryUrl.String()
	return fullUrl
}

func ReturnDevices(w http.ResponseWriter, r *http.Request) (Inventory, error) {
	transport := setProxy()
	client := &http.Client{Transport: transport}
	fullUrl := setUrl() + filterParams
	req, err := http.NewRequest("GET", fullUrl, nil)
	req.SetBasicAuth(usr, pwd)
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
	deviceId := chi.URLParam(r, "devices")
	deviceIdParam := "?hostname_or_id=" + deviceId

	transport := setProxy()
	client := &http.Client{Transport: transport}
	fullUrl := setUrl() + filterParams + deviceIdParam
	req, err := http.NewRequest("GET", fullUrl, nil)
	req.SetBasicAuth(usr, pwd)
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		return Inventory{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)

	var bodyResp Inventory
	json.Unmarshal([]byte(body), &bodyResp)
	fmt.Printf("struct: %v\n", bodyResp)
	return bodyResp, nil

}

func ReturnDevicesByTag(w http.ResponseWriter, r *http.Request) (Inventory, error) {

	tags := chi.URLParam(r, "devices")
	tagsParam := "?tags=" + tags

	transport := setProxy()
	client := &http.Client{Transport: transport}
	fullUrl := setUrl() + tagsParam
	req, err := http.NewRequest("GET", fullUrl, nil)
	req.SetBasicAuth(usr, pwd)
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
