package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients"
)

// ClientInterface is an Interface to make request to InventoryAPI
type ClientInterface interface {
	ReturnDevices(parameters *InventoryParams) (Response, error)
	ReturnDevicesByID(deviceID string) (Response, error)
	ReturnDevicesByTag(tag string) (Response, error)
	BuildURL(parameters *InventoryParams) string
}

// Client is the implementation of an ClientInterface
type Client struct {
	ctx context.Context
}

// InitClient initializes the client for InventoryAPI
func InitClient(ctx context.Context) *Client {
	return &Client{ctx: ctx}
}

// Response lists devices returned by InventoryAPI
type Response struct {
	Total  int       `json:"total"`
	Count  int       `json:"count"`
	Result []Devices `json:"results"`
}

// Devices represents the struct of a Device on Inventory API
type Devices struct {
	ID              string        `json:"id"`
	DisplayName     string        `json:"display_name"`
	LastSeen        string        `json:"updated"`
	UpdateAvailable bool          `json:"update_available"`
	Ostree          SystemProfile `json:"system_profile"`
}

// SystemProfile represents the struct of a SystemProfile on Inventory API
type SystemProfile struct {
	RHCClientID          string   `json:"rhc_client_id"`
	RpmOstreeDeployments []OSTree `json:"rpm_ostree_deployments"`
}

// OSTree represents the struct of a SystemProfile on Inventory API
type OSTree struct {
	Checksum string `json:"checksum"`
	Booted   bool   `json:"booted"`
}

const (
	inventoryAPI = "api/inventory/v1/hosts"
	orderBy      = "updated"
	orderHow     = "DESC"
	// Fields represents field we get from inventory
	Fields = "host_type,operating_system,greenboot_status,greenboot_fallback_detected,rpm_ostree_deployments,rhc_client_id,rhc_config_state"
	// FilterParams represents params to retrieve data from inventory
	FilterParams = "?staleness=fresh&filter[system_profile][host_type]=edge&fields[system_profile]=host_type,operating_system,greenboot_status,greenboot_fallback_detected,rpm_ostree_deployments,rhc_client_id,rhc_config_state"
)

// InventoryParams represents the struct of params to be sent
type InventoryParams struct {
	PerPage      string
	Page         string
	OrderBy      string
	OrderHow     string
	HostnameOrID string
	DeviceStatus string
}

// BuildURL call the inventoryApi endpoint
func (c *Client) BuildURL(parameters *InventoryParams) string {
	URL, err := url.Parse(config.Get().InventoryConfig.URL)
	if err != nil {
		log.Println("Couldn't parse inventory host")
		return ""
	}
	fmt.Printf("Url:: %v\n", URL)
	URL.Path += inventoryAPI
	fmt.Printf("UrlPath:: %v\n", URL.Path)
	params := url.Values{}
	params.Add("filter[system_profile][host_type]", "edge")
	params.Add("fields[system_profile]", fmt.Sprintf("%s=%s", "fields[system_profile]", Fields))
	if parameters.PerPage != "" {
		params.Add("per_page", parameters.PerPage)
	}
	if parameters.Page != "" {
		params.Add("page", parameters.Page)
	}
	if parameters.OrderBy != "" {
		params.Add("order_by", parameters.OrderBy)
	}
	if parameters.OrderHow != "" {
		params.Add("order_how", parameters.OrderHow)
	}
	if parameters.HostnameOrID != "" {
		params.Add("hostname_or_id", parameters.HostnameOrID)
	}
	URL.RawQuery = params.Encode()

	return URL.String()
}

// ReturnDevices will return the list of devices without filter by tag or uuid
func (c *Client) ReturnDevices(parameters *InventoryParams) (Response, error) {

	fullURL := c.BuildURL(parameters)

	// url := fmt.Sprintf("%s/%s", config.Get().InventoryConfig.URL, inventoryAPI)
	// fullURL := url + FilterParams
	fmt.Printf("fullURL:: %v\n", fullURL)
	log.Infof("Requesting url: %s\n", fullURL)
	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Add("Content-Type", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("ReturnDevices: %s", err)
		return Response{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("ReturnDevices: %s", err)
		return Response{}, err
	}
	defer resp.Body.Close()
	var bodyResp Response
	json.Unmarshal([]byte(body), &bodyResp)
	log.Infof("struct: %v\n", bodyResp)
	return bodyResp, nil

}

// ReturnDevicesByID will return the list of devices by uuid
func (c *Client) ReturnDevicesByID(deviceID string) (Response, error) {
	deviceIDParam := "&hostname_or_id=" + deviceID
	log.Infof("::deviceIDParam: %s\n", deviceIDParam)
	url := fmt.Sprintf("%s/%s", config.Get().InventoryConfig.URL, inventoryAPI)
	fullURL := url + FilterParams + deviceIDParam
	log.Infof("Requesting url: %s\n", fullURL)
	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Add("Content-Type", "application/json")
	for key, value := range clients.GetOutgoingHeaders(c.ctx) {
		req.Header.Add(key, value)
	}
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Errorf("ReturnDevicesByID: %s", err)
		return Response{}, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Errorf("error requesting InventoryResponse, got status code %d and body %s", resp.StatusCode, body)
		return Response{}, fmt.Errorf("error requesting InventoryResponse, got status code %d and body %s", resp.StatusCode, body)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("ReturnDevicesByID: %s", err)
		return Response{}, err
	}
	defer resp.Body.Close()
	var inventory Response
	json.Unmarshal([]byte(body), &inventory)
	log.Infof("::Updates::ReturnDevicesByID::inventory: %v\n", inventory)

	return inventory, nil

}

// ReturnDevicesByTag will return the list of devices by tag
func (c *Client) ReturnDevicesByTag(tag string) (Response, error) {
	tagsParam := "?tags=" + tag
	url := fmt.Sprintf("%s/%s", config.Get().InventoryConfig.URL, inventoryAPI)
	fullURL := url + FilterParams + tagsParam
	log.Infof("Requesting url: %s\n", fullURL)
	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Add("Content-Type", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Errorf("ReturnDevicesByTag: %s", err)
		return Response{}, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Errorf("error requesting inventory, got status code %d and body %s", resp.StatusCode, body)
		return Response{}, fmt.Errorf("error requesting inventory, got status code %d and body %s", resp.StatusCode, body)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("ReturnDevicesByTag: %s", err)
		return Response{}, err
	}
	var inventory Response
	json.Unmarshal([]byte(body), &inventory)
	log.Infof("struct: %v\n", inventory)
	return inventory, nil
}
