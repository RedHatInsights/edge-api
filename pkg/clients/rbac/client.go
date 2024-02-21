package rbac

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	url2 "net/url"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

var ErrCreatingRbacURL = errors.New("error occurred when creating rbac url")
var ErrInvalidAttributeFilterKey = errors.New("invalid value for attributeFilter.key in RBAC response")
var ErrInvalidAttributeFilterOperation = errors.New("invalid value for attributeFilter.operation in RBAC response")
var ErrInvalidAttributeFilterValue = errors.New("received invalid UUIDs for attributeFilter.value in RBAC response")
var ErrFailedToBuildAccessRequest = errors.New("failed to build access request")
var ErrRbacRequestResponse = errors.New("rbac response error")

// IOReadAll The io body reader
var IOReadAll = io.ReadAll

// HTTPGetCommand the http get command
var HTTPGetCommand = http.MethodGet

// APIPath the rbac base path
const APIPath = "/api/rbac/v1"

type ResourceType string
type AccessType string
type Application string

const (
	AccessTypeAny  AccessType = "*"
	AccessTypeRead AccessType = "read"
)

const ApplicationInventory Application = "inventory"

const (
	ResourceTypeAny   ResourceType = "*"
	ResourceTypeHOSTS ResourceType = "hosts"
)

const DefaultTimeDuration = 1 * time.Second

// PaginationLimit to get a maximum of 1000 records
const PaginationLimit = "1000"

// ResponseBody represents the response body format from the RBAC service
type ResponseBody struct {
	Meta  PaginationMeta  `json:"meta"`
	Links PaginationLinks `json:"links"`
	Data  AccessList      `json:"data"`
}

// PaginationMeta contains metadata for pagination
type PaginationMeta struct {
	Count  int `json:"count"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// PaginationLinks provides links to additional pages of response data
type PaginationLinks struct {
	First    string `json:"first"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Last     string `json:"last"`
}

// ClientInterface is an Interface to make request to insights rbac
type ClientInterface interface {
	GetAccessList(application Application) (AccessList, error)
	GetInventoryGroupsAccess(acl AccessList, resource ResourceType, accessType AccessType) (bool, []string, bool, error)
}

// Client is the implementation of an ClientInterface
type Client struct {
	ctx context.Context
	log *log.Entry
}

// InitClient initializes the client for Rbac service
func InitClient(ctx context.Context, log *log.Entry) ClientInterface {
	return &Client{ctx: ctx, log: log.WithField("client-context", "rbac-client")}
}

func (c *Client) GetRBacAccessHTTPRequest(ctx context.Context, application Application) (*http.Request, error) {
	url, err := url2.JoinPath(config.Get().RbacBaseURL, APIPath, "access/")
	if err != nil {
		c.log.WithField("error", err.Error()).Error(ErrCreatingRbacURL.Error())
		return nil, ErrCreatingRbacURL
	}

	req, err := http.NewRequestWithContext(ctx, HTTPGetCommand, url, nil)
	if err != nil {
		return nil, ErrFailedToBuildAccessRequest
	}
	q := req.URL.Query()
	q.Add("application", string(application))
	q.Add("limit", PaginationLimit)
	req.URL.RawQuery = q.Encode()
	req.Header.Add("Content-Type", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	return req, nil
}

// GetAccessList return the application rbac access list
func (c *Client) GetAccessList(application Application) (AccessList, error) {
	rbacTimeout := time.Duration(config.Get().RbacTimeout) * DefaultTimeDuration
	ctx, cancel := context.WithTimeout(c.ctx, rbacTimeout)
	defer cancel()

	req, err := c.GetRBacAccessHTTPRequest(ctx, application)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("error occurred while creating rbac access request")
		return nil, err
	}

	client := clients.ConfigureClientWithTLS(&http.Client{})
	res, err := client.Do(req)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("rbac request failed")
		return nil, err
	}
	defer res.Body.Close()

	body, err := IOReadAll(res.Body)
	if err != nil {
		c.log.WithFields(log.Fields{"statusCode": res.StatusCode, "error": err.Error()}).Error("rbac read response body error")
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		c.log.WithFields(
			log.Fields{"statusCode": res.StatusCode, "responseBody": string(body)},
		).Error("rbac request error response")
		return nil, ErrRbacRequestResponse
	}

	var responseAccess ResponseBody
	err = json.Unmarshal(body, &responseAccess)
	if err != nil {
		c.log.WithFields(log.Fields{"responseBody": string(body), "error": err.Error()}).Error("error occurred when unmarshalling response body to repository")
		return nil, err
	}

	return responseAccess.Data, nil
}

// getAssessGroupsFromResourceDefinition validate and return the access groups
func (c *Client) getAssessGroupsFromResourceDefinition(resourceDefinition ResourceDefinition) ([]*string, error) {
	if resourceDefinition.Filter.Key != "group.id" {
		c.log.WithField("filter-key", resourceDefinition.Filter.Key).Error("received an unexpected resource filter key value")
		return nil, ErrInvalidAttributeFilterKey
	}
	if resourceDefinition.Filter.Operation != "in" {
		c.log.WithField("filter-operation", resourceDefinition.Filter.Key).Error("received an unexpected resource filter operation value")
		return nil, ErrInvalidAttributeFilterOperation
	}

	return resourceDefinition.Filter.Value, nil
}

// getGroupsFromAccessGroups validate access groups and return groups and whether to ungrouped hosts should be included
func (c *Client) getGroupsFromAccessGroups(accessGroups []*string) ([]string, bool, error) {
	var unGroupedHosts bool
	var groups []string
	for _, groupUUID := range accessGroups {
		if groupUUID == nil {
			unGroupedHosts = true
		} else {
			if _, err := uuid.Parse(*groupUUID); err != nil {
				c.log.WithField("filter-uuid", *groupUUID).Error("error occurred while parsing uuid value")
				return nil, false, ErrInvalidAttributeFilterValue
			}
			groups = append(groups, *groupUUID)
		}
	}
	return groups, unGroupedHosts, nil
}

// GetInventoryGroupsAccess return whether access is allowed and the groups configurations
func (c *Client) GetInventoryGroupsAccess(acl AccessList, resource ResourceType, accessType AccessType) (bool, []string, bool, error) {
	var overallGroupIDs []string
	var overallGroupIDSMap = make(map[string]bool)
	var allowedAccess bool
	var globalUnGroupedHosts bool
	for _, ac := range acl {
		// check if the resource with accessType has access to the current access item
		if ac.Application() == string(ApplicationInventory) && ResourceMatch(ResourceType(ac.Resource()), resource) && AccessMatch(AccessType(ac.AccessType()), accessType) {
			allowedAccess = true
			if len(ac.ResourceDefinitions) == 0 {
				// we should have global access to the resource in the context of this access type
				// reset the values
				globalUnGroupedHosts = false
				overallGroupIDs = nil
				break
			}
			for _, resourceDef := range ac.ResourceDefinitions {
				// validate if the resource definition is correct and get all access groups from the resource definition value
				accessGroups, err := c.getAssessGroupsFromResourceDefinition(resourceDef)
				if err != nil {
					return false, nil, false, err
				}
				// validate if all access groups are valid, as access groups is a list of groups uuids with pointers to string []*string
				// this function call will return static groups list []string and if any null value is in the list, this means that ungrouped Hosts
				// are needed and unGroupedHosts will be set to true
				groups, unGroupedHosts, err := c.getGroupsFromAccessGroups(accessGroups)
				if err != nil {
					return false, nil, false, err
				}
				if unGroupedHosts {
					globalUnGroupedHosts = true
				}
				for _, groupUUID := range groups {
					// add the group to global groups list when it's not in the global map, to make sure there is no duplicates
					if _, ok := overallGroupIDSMap[groupUUID]; !ok {
						// put it in the map for later duplicate check
						overallGroupIDSMap[groupUUID] = true
						overallGroupIDs = append(overallGroupIDs, groupUUID)
					}
				}
			}
		}
	}
	return allowedAccess, overallGroupIDs, globalUnGroupedHosts, nil
}

// AccessMatch return whether the access type matches the required resource type
func AccessMatch(access1, access2 AccessType) bool {
	return access1 == access2 || access1 == AccessTypeAny
}

// ResourceMatch return whether the resource type matches the required resource type
func ResourceMatch(resource1, resource2 ResourceType) bool {
	return resource1 == resource2 || resource1 == ResourceTypeAny
}
