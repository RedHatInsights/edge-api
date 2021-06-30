package devices

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/models"
)

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

// MakeRouter adds support for operations on commits
func MakeRouter(sub chi.Router) {
	sub.Get("/", GetAll)
	sub.Route("/{deviceId}", func(r chi.Router) {
		r.Get("/", GetByID)
	})
}

// GetAll obtains list of devices
func GetAll(w http.ResponseWriter, r *http.Request) {
	transport := SetProxy()
	client := &http.Client{Transport: transport}
	fullUrl := SetUrl() + filterParams
	req, err := http.NewRequest("GET", fullUrl, nil)
	req.SetBasicAuth(usr, pwd)
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	log.Printf(string(body))
	fmt.Printf("%v\n", string(body))
	fmt.Fprintf(w, string(body))

	var bodyResp *models.UpdateRecord
	json.NewDecoder(resp.Body).Decode(&bodyResp)
	fmt.Printf("%v\n", bodyResp)
	log.Printf(string(body))
}

// GetByID obtains a specifc device info
func GetByID(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("%v\n", r.Context())
	deviceId := chi.URLParam(r, "deviceId")
	fmt.Printf("commitID: %v\n", deviceId)
	deviceIdParam := "?hostname_or_id=" + deviceId

	transport := SetProxy()
	client := &http.Client{Transport: transport}
	fullUrl := SetUrl() + filterParams + deviceIdParam
	req, err := http.NewRequest("GET", fullUrl, nil)
	req.SetBasicAuth(usr, pwd)
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	log.Printf(string(body))
	fmt.Printf("%v\n", string(body))
	fmt.Fprintf(w, string(body))
}

func SetProxy() *http.Transport {
	proxyURL, err := url.Parse(PROXY)
	if err != nil {
		return &http.Transport{}
	}
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	return transport
}
func SetUrl() string {
	inventoryUrl, err := url.Parse(config.Get().InventoryConfig.URL)
	if err != nil {
		return "Error to parse inventory url"
	}
	inventoryUrl.Path = path.Join(inventoryUrl.Path, inventoryAPI)
	fullUrl := inventoryUrl.String()
	return fullUrl
}
