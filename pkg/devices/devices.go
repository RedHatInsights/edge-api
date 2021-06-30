package devices

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/config"
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
	sub.Get("/", getAll)
	sub.Route("/{deviceId}", func(r chi.Router) {
		r.Get("/", GetByID)
	})
}

// getAll registered for an account
func getAll(w http.ResponseWriter, r *http.Request) {
	proxyURL, err := url.Parse(PROXY)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	client := &http.Client{Transport: transport}
	inventoryUrl, err := url.Parse(config.Get().InventoryConfig.URL)
	inventoryUrl.Path = path.Join(inventoryUrl.Path, inventoryAPI)
	fullUrl := inventoryUrl.String() + filterParams

	req, err := http.NewRequest("GET", fullUrl+filterParams, nil)
	req.SetBasicAuth(usr, pwd)

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	fmt.Printf("%v\n", string(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	//Convert the body to type string
	sb := string(body)
	log.Printf(sb)
	fmt.Fprintf(w, string(body))
}

// GetByID obtains a specifc device info
func GetByID(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("%v\n", r.Context())
	// ctx := r.Context().Value("deviceId")
	deviceId := chi.URLParam(r, "deviceId")
	fmt.Printf("commitID: %v\n", deviceId)
	deviceIdParam := "&hostname_or_id=" + deviceId
	proxyURL, err := url.Parse(PROXY)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	client := &http.Client{Transport: transport}
	inventoryUrl, err := url.Parse(config.Get().InventoryConfig.URL)
	inventoryUrl.Path = path.Join(inventoryUrl.Path, inventoryAPI)
	fullUrl := inventoryUrl.String() + filterParams + deviceIdParam
	fmt.Printf("fullUrl: %v\n", fullUrl)
	req, err := http.NewRequest("GET", fullUrl+filterParams, nil)
	req.SetBasicAuth(usr, pwd)

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	fmt.Printf("%v\n", string(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	//Convert the body to type string
	sb := string(body)
	log.Printf(sb)
	fmt.Fprintf(w, string(body))
}
