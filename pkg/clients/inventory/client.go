package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients"
)

// ClientInterface is an Interface to make request to InventoryAPI
type ClientInterface interface {
	ReturnDevices() (Response, error)
	ReturnDevicesByID(deviceID string) (Response, error)
	ReturnDevicesByTag(tag string) (Response, error)
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
	ID              string `json:"id"`
	DisplayName     string `json:"display_name"`
	LastSeen        string `json:"updated"`
	UpdateAvailable bool
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
	filterParams = "?staleness=fresh&filter[system_profile][host_type]=edge&fields[system_profile]=host_type,operating_system,greenboot_status,greenboot_fallback_detected,rpm_ostree_deployments,rhc_client_id,rhc_config_state"
)

// ReturnDevices will return the list of devices without filter by tag or uuid
func (c *Client) ReturnDevices() (Response, error) {
	url := fmt.Sprintf("%s/%s", config.Get().InventoryConfig.URL, inventoryAPI)
	fullURL := url + filterParams
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
		log.Error(fmt.Printf("ReturnDevices: %s", err))
		return Response{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(fmt.Printf("ReturnDevices: %s", err))
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
	fullURL := url + filterParams + deviceIDParam
	log.Infof("Requesting url: %s\n", fullURL)
	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Add("Content-Type", "application/json")
	for key, value := range clients.GetOutgoingHeaders(c.ctx) {
		req.Header.Add(key, value)
	}
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Error(fmt.Printf("ReturnDevicesByID: %s", err))
		return Response{}, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Errorf("error requesting InventoryResponse, got status code %d and body %s", resp.StatusCode, body)
		return Response{}, fmt.Errorf("error requesting InventoryResponse, got status code %d and body %s", resp.StatusCode, body)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(fmt.Printf("ReturnDevicesByID: %s", err))
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
	fullURL := url + filterParams + tagsParam
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
		log.Error(fmt.Printf("ReturnDevicesByTag: %s", err))
		return Response{}, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Errorf("error requesting inventory, got status code %d and body %s", resp.StatusCode, body)
		return Response{}, fmt.Errorf("error requesting inventory, got status code %d and body %s", resp.StatusCode, body)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(fmt.Printf("ReturnDevicesByTag: %s", err))
		return Response{}, err
	}
	var inventory Response
	json.Unmarshal([]byte(body), &inventory)
	log.Infof("struct: %v\n", inventory)
	return inventory, nil
}
