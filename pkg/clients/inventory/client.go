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
	ReturnDevices(parameters *Params) (Response, error)
	ReturnDevicesByID(deviceID string) (Response, error)
	ReturnDevicesByTag(tag string) (Response, error)
	BuildURL(parameters *Params) string
}

// Client is the implementation of an ClientInterface
type Client struct {
	ctx context.Context
	log *log.Entry
}

// InitClient initializes the client for Image Builder
func InitClient(ctx context.Context, log *log.Entry) *Client {
	return &Client{ctx: ctx, log: log}
}

// Response lists devices returned by InventoryAPI
type Response struct {
	Total  int      `json:"total"`
	Count  int      `json:"count"`
	Result []Device `json:"results"`
}

// Device represents the struct of a Device on Inventory API
type Device struct {
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

// Params represents the struct of params to be sent
type Params struct {
	PerPage      string
	Page         string
	OrderBy      string
	OrderHow     string
	HostnameOrID string
	DeviceStatus string
}

// BuildURL call the inventoryApi endpoint
func (c *Client) BuildURL(parameters *Params) string {
	URL, err := url.Parse(config.Get().InventoryConfig.URL)
	if err != nil {
		c.log.WithField("url", config.Get().InventoryConfig.URL).Error("Couldn't parse inventory host")
		return ""
	}
	URL.Path += inventoryAPI
	params := url.Values{}
	params.Add("filter[system_profile][host_type]", "edge")
	params.Add("fields[system_profile]", fmt.Sprintf("%s=%s", "fields[system_profile]", Fields))
	if parameters != nil && parameters.PerPage != "" {
		params.Add("per_page", parameters.PerPage)
	}
	if parameters != nil && parameters.Page != "" {
		params.Add("page", parameters.Page)
	}
	if parameters != nil && parameters.OrderBy != "" {
		params.Add("order_by", parameters.OrderBy)
	}
	if parameters != nil && parameters.OrderHow != "" {
		params.Add("order_how", parameters.OrderHow)
	}
	if parameters != nil && parameters.HostnameOrID != "" {
		params.Add("hostname_or_id", parameters.HostnameOrID)
	}
	URL.RawQuery = params.Encode()
	c.log.WithField("URL", URL.String()).Debug("Inventory URL built")
	return URL.String()
}

// ReturnDevices will return the list of devices without filter by tag or uuid
func (c *Client) ReturnDevices(parameters *Params) (Response, error) {

	url := c.BuildURL(parameters)
	c.log.WithFields(log.Fields{
		"url": url,
	}).Info("Inventory ReturnDevices Request Started")
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		c.log.WithFields(log.Fields{
			"statusCode": res.StatusCode,
			"error":      err,
		}).Error("Inventory ReturnDevices Request Error")
		return Response{}, err
	}
	body, err := ioutil.ReadAll(res.Body)
	c.log.WithFields(log.Fields{
		"statusCode":   res.StatusCode,
		"responseBody": string(body),
		"error":        err,
	}).Info("Inventory ReturnDevices Response")
	if err != nil {
		return Response{}, err
	}

	defer res.Body.Close()
	var bodyResp Response
	err = json.Unmarshal(body, &bodyResp)
	if err != nil {
		return Response{}, err
	}
	return bodyResp, nil

}

// ReturnDevicesByID will return the list of devices by uuid
func (c *Client) ReturnDevicesByID(deviceID string) (Response, error) {
	url := fmt.Sprintf("%s/%s/%s", config.Get().InventoryConfig.URL, inventoryAPI, deviceID)
	c.log.WithFields(log.Fields{
		"url": url,
	}).Info("Inventory ReturnDevicesByID Request Started")
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	for key, value := range clients.GetOutgoingHeaders(c.ctx) {
		req.Header.Add(key, value)
	}
	client := &http.Client{}
	res, err := client.Do(req)

	if err != nil {
		c.log.WithFields(log.Fields{
			"statusCode": res.StatusCode,
			"error":      err,
		}).Error("Inventory ReturnDevicesByID Request Error")
		return Response{}, err
	}

	body, err := ioutil.ReadAll(res.Body)
	c.log.WithFields(log.Fields{
		"statusCode":   res.StatusCode,
		"responseBody": string(body),
		"error":        err,
	}).Info("Inventory ReturnDevicesByID Response")
	if err != nil {
		return Response{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("error requesting InventoryResponse, got status code %d and body %s", res.StatusCode, body)
	}
	var inventory Response
	if err := json.Unmarshal([]byte(body), &inventory); err != nil {
		c.log.Error("Error while trying to unmarshal ", &inventory)
		return Response{}, err
	}
	return inventory, nil

}

// ReturnDevicesByTag will return the list of devices by tag
func (c *Client) ReturnDevicesByTag(tag string) (Response, error) {
	tagsParam := "?tags=" + tag
	url := fmt.Sprintf("%s/%s%s%s", config.Get().InventoryConfig.URL, inventoryAPI, FilterParams, tagsParam)
	c.log.WithFields(log.Fields{
		"url": url,
	}).Info("Inventory ReturnDevicesByTag Request Started")
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	client := &http.Client{}
	res, err := client.Do(req)

	if err != nil {
		c.log.WithFields(log.Fields{
			"statusCode": res.StatusCode,
			"error":      err,
		}).Error("Inventory ReturnDevicesByTag Request Error")
		return Response{}, err
	}
	body, err := ioutil.ReadAll(res.Body)
	c.log.WithFields(log.Fields{
		"statusCode":   res.StatusCode,
		"responseBody": string(body),
		"error":        err,
	}).Info("Inventory ReturnDevicesByTag Response")
	if err != nil {
		return Response{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("error requesting InventoryResponse, got status code %d and body %s", res.StatusCode, body)
	}
	var inventory Response
	if err := json.Unmarshal([]byte(body), &inventory); err != nil {
		c.log.Error("Error while trying to unmarshal ", &inventory)
		return Response{}, err
	}
	return inventory, nil
}
