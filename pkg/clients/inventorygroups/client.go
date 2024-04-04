package inventorygroups

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	url2 "net/url"
	"strconv"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients"

	log "github.com/sirupsen/logrus"
)

var ErrParsingURL = errors.New("error occurred while parsing raw url")

var ErrGroupsRequestResponse = errors.New("inventory groups request error response")

var ErrGroupNotFound = errors.New("inventory group not found")

var ErrGroupNameIsMandatory = errors.New("inventory group name is mandatory")

var ErrGroupUUIDIsMandatory = errors.New("inventory group uuid is mandatory")

var ErrGroupHostsAreMandatory = errors.New("inventory group hosts are mandatory")

// BasePath the inventory groups base path
const BasePath = "api/inventory/v1/groups"

// DefaultPerPage the default per page limit when listing inventory groups
const DefaultPerPage = 50

// IOReadAll The io body reader
var IOReadAll = io.ReadAll

// NewJSONEncoder  create a new json encoder
var NewJSONEncoder = json.NewEncoder

// Group the type representation returned from inventory groups endpoint
type Group struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	OrgID     string `json:"org_id"`
	HostCount int    `json:"host_count"`
}

// Response the result type returned when getting a list of inventory groups
type Response struct {
	Total   int     `json:"total"`
	Count   int     `json:"count"`
	Page    int     `json:"page"`
	PerPage int     `json:"per_page"`
	Results []Group `json:"results"`
}

// ListGroupsParams the parameters used for list request
type ListGroupsParams struct {
	Name     string
	Page     int
	PerPage  int
	OrderBy  string
	OrderHow string
}

type CreateGroup struct {
	Name    string   `json:"name"`
	HostIDS []string `json:"host_ids,omitempty"` // nolint:revive
}

// ClientInterface is an Interface to make request to inventory Groups
type ClientInterface interface {
	GetBaseURL() (*url2.URL, error)
	GetGroupByName(name string) (*Group, error)
	GetGroupByUUID(groupUUID string) (*Group, error)
	CreateGroup(groupName string, hostIDS []string) (*Group, error) // nolint:revive
	AddHostsToGroup(groupUUID string, hosts []string) (*Group, error)
	ListGroups(requestParams ListGroupsParams) (*Response, error)
}

// Client is the implementation of an ClientInterface
type Client struct {
	ctx context.Context
	log *log.Entry
}

// InitClient initializes the client for Image Builder
func InitClient(ctx context.Context, log *log.Entry) ClientInterface {
	return &Client{ctx: ctx, log: log}
}

// GetBaseURL return the base url of inventory groups
func (c *Client) GetBaseURL() (*url2.URL, error) {
	baseURL := config.Get().InventoryConfig.URL + "/" + BasePath
	url, err := url2.Parse(baseURL)
	if err != nil {
		c.log.WithFields(log.Fields{"url": baseURL, "error": err.Error()}).Error("failed to parse inventory groups base url")
		return nil, ErrParsingURL
	}
	return url, nil
}

func (c *Client) ListGroups(requestParams ListGroupsParams) (*Response, error) {
	groupsURL, err := c.GetBaseURL()
	if err != nil {
		return nil, err
	}

	if requestParams.Page <= 0 {
		requestParams.Page = 1
	}
	if requestParams.PerPage <= 0 {
		requestParams.PerPage = DefaultPerPage
	}

	queryValues := groupsURL.Query()
	if requestParams.Name != "" {
		queryValues.Add("name", requestParams.Name)
	}
	if requestParams.OrderBy != "" {
		queryValues.Add("order_by", requestParams.OrderBy)
	}
	if requestParams.OrderHow != "" {
		queryValues.Add("order_how", requestParams.OrderHow)
	}
	queryValues.Add("per_page", strconv.Itoa(requestParams.PerPage))
	queryValues.Add("page", strconv.Itoa(requestParams.Page))

	// set queryValues to groups url
	groupsURL.RawQuery = queryValues.Encode()
	requestURL := groupsURL.String()

	c.log.WithField("url", requestURL).Info("inventory groups request started")
	req, _ := http.NewRequest(http.MethodGet, requestURL, nil)
	req.Header.Add("Content-Type", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	client := clients.ConfigureClientWithTLS(&http.Client{})
	res, err := client.Do(req)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("inventory groups request error")
		return nil, err
	}
	defer res.Body.Close()

	body, err := IOReadAll(res.Body)
	if err != nil {
		c.log.WithFields(log.Fields{"statusCode": res.StatusCode, "error": err.Error()}).Error("inventory groups response error")
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		c.log.WithFields(log.Fields{"statusCode": res.StatusCode, "responseBody": string(body)}).Error("inventory groups error response")
		return nil, ErrGroupsRequestResponse
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		c.log.WithFields(log.Fields{"error": err.Error(), "response-body": string(body)}).Error("error occurred when unmarshalling response body")
		return nil, err
	}

	return &response, nil
}

func (c *Client) GetGroupByName(name string) (*Group, error) {
	if name == "" {
		c.log.Error("inventory group name is mandatory")
		return nil, ErrGroupNameIsMandatory
	}
	response, err := c.ListGroups(ListGroupsParams{Name: name, PerPage: 1, Page: 1, OrderBy: "name", OrderHow: "ASC"})
	if err != nil {
		c.log.WithFields(log.Fields{"group-name": name, "error": err.Error()}).Error("failed when calling to ListGroups")
		return nil, err
	}
	if len(response.Results) == 0 || response.Results[0].Name != name {
		// return group not found when no group returned,
		// or when the first found group has name different from the requested one.
		c.log.WithFields(log.Fields{"group-name": name, "results-length": strconv.Itoa(len(response.Results))}).Error("group not found")
		return nil, ErrGroupNotFound
	}
	return &response.Results[0], nil
}

func (c *Client) GetGroupByUUID(groupUUID string) (*Group, error) {
	if groupUUID == "" {
		c.log.Error("inventory group uuid is mandatory")
		return nil, ErrGroupUUIDIsMandatory
	}

	groupsURL, err := c.GetBaseURL()
	if err != nil {
		return nil, err
	}

	requestURL := groupsURL.String() + "/" + groupUUID

	c.log.WithField("url", requestURL).Debug("inventory group request started")
	req, _ := http.NewRequest(http.MethodGet, requestURL, nil)
	req.Header.Add("Content-Type", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	client := clients.ConfigureClientWithTLS(&http.Client{})
	res, err := client.Do(req)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("inventory group request error")
		return nil, err
	}
	defer res.Body.Close()

	body, err := IOReadAll(res.Body)
	if err != nil {
		c.log.WithFields(log.Fields{"statusCode": res.StatusCode, "error": err.Error()}).Error("inventory group response error")
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		c.log.WithFields(log.Fields{"statusCode": res.StatusCode, "responseBody": string(body)}).Error("inventory group error response")
		return nil, ErrGroupsRequestResponse
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		c.log.WithFields(log.Fields{"error": err.Error(), "response-body": string(body)}).Error("error occurred when unmarshalling response body")
		return nil, err
	}

	if len(response.Results) == 0 {
		c.log.WithField("group-uuid", groupUUID).Error("group not found")
		return nil, ErrGroupNotFound
	}

	return &response.Results[0], nil
}

func (c *Client) CreateGroup(groupName string, hostIDS []string) (*Group, error) { // nolint:revive
	if groupName == "" {
		c.log.Error("inventory group name is mandatory")
		return nil, ErrGroupNameIsMandatory
	}

	groupsURL, err := c.GetBaseURL()
	if err != nil {
		return nil, err
	}

	payloadBuffer := new(bytes.Buffer)
	if err := NewJSONEncoder(payloadBuffer).Encode(&CreateGroup{Name: groupName, HostIDS: hostIDS}); err != nil {
		c.log.WithField("error", err.Error()).Error("error occurred while encoding create group")
		return nil, err
	}

	requestURL := groupsURL.String()
	c.log.WithField("url", requestURL).Info("inventory create group request started")

	req, _ := http.NewRequest(http.MethodPost, requestURL, payloadBuffer)
	req.Header.Add("Content-Type", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	client := clients.ConfigureClientWithTLS(&http.Client{})
	res, err := client.Do(req)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("inventory create group request error")
		return nil, err
	}
	defer res.Body.Close()

	body, err := IOReadAll(res.Body)
	if err != nil {
		c.log.WithFields(log.Fields{"statusCode": res.StatusCode, "error": err.Error()}).Error("inventory create group response error")
		return nil, err
	}
	if res.StatusCode != http.StatusCreated {
		c.log.WithFields(log.Fields{"statusCode": res.StatusCode, "responseBody": string(body)}).Error("inventory create group error response")
		return nil, ErrGroupsRequestResponse
	}

	var group Group
	err = json.Unmarshal(body, &group)
	if err != nil {
		c.log.WithFields(log.Fields{"error": err.Error(), "response-body": string(body)}).Error("error occurred when unmarshalling response body")
		return nil, err
	}

	return &group, nil
}

func (c *Client) AddHostsToGroup(groupUUID string, hosts []string) (*Group, error) {
	if groupUUID == "" {
		c.log.Error("inventory group uuid is mandatory")
		return nil, ErrGroupUUIDIsMandatory
	}

	if len(hosts) == 0 {
		c.log.Error("inventory group hosts are mandatory")
		return nil, ErrGroupHostsAreMandatory

	}

	groupsURL, err := c.GetBaseURL()
	if err != nil {
		return nil, err
	}

	payloadBuffer := new(bytes.Buffer)
	if err := NewJSONEncoder(payloadBuffer).Encode(&hosts); err != nil {
		c.log.WithField("error", err.Error()).Error("error occurred while encoding group hosts")
		return nil, err
	}

	requestURL := fmt.Sprintf("%s/%s/hosts", groupsURL.String(), groupUUID)
	c.log.WithField("url", requestURL).Info("inventory add group hosts request started")

	req, _ := http.NewRequest(http.MethodPost, requestURL, payloadBuffer)
	req.Header.Add("Content-Type", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	client := clients.ConfigureClientWithTLS(&http.Client{})
	res, err := client.Do(req)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("inventory add group hosts request error")
		return nil, err
	}
	defer res.Body.Close()

	body, err := IOReadAll(res.Body)
	if err != nil {
		c.log.WithFields(log.Fields{"statusCode": res.StatusCode, "error": err.Error()}).Error("inventory add group hosts response error")
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		c.log.WithFields(log.Fields{"statusCode": res.StatusCode, "responseBody": string(body)}).Error("inventory add group hosts error response")
		return nil, ErrGroupsRequestResponse
	}

	var group Group
	err = json.Unmarshal(body, &group)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("error occurred when unmarshalling response body")
		return nil, err
	}

	return &group, nil
}
