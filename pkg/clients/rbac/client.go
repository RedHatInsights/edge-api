package rbac

import (
	"context"
	"encoding/json"
	"errors"
	url2 "net/url"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients"
	"github.com/redhatinsights/edge-api/pkg/routes/common"

	rbacClient "github.com/RedHatInsights/rbac-client-go"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

var ErrCreatingRbacURL = errors.New("error occurred when creating rbac url")
var ErrGettingIdentityFromContext = errors.New("error getting x-rh-identity from context")
var ErrInvalidAttributeFilterKey = errors.New("invalid value for attributeFilter.key in RBAC response")
var ErrInvalidAttributeFilterOperation = errors.New("invalid value for attributeFilter.operation in RBAC response")
var ErrInvalidAttributeFilterValueType = errors.New("did not receive a list for attributeFilter.value in RBAC response")
var ErrInvalidAttributeFilterValue = errors.New("received invalid UUIDs for attributeFilter.value in RBAC response")

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

// ClientInterface is an Interface to make request to insights rbac
type ClientInterface interface {
	GetAccessList(application Application) (rbacClient.AccessList, error)
	GetInventoryGroupsAccess(acl rbacClient.AccessList, resource ResourceType, accessType AccessType) (bool, []string, bool, error)
}

// WrappedClientInterface is an interface of the original rbac client
type WrappedClientInterface interface {
	GetAccess(ctx context.Context, identity string, username string) (rbacClient.AccessList, error)
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

// NewRbacClient create a new rbac client
func (c *Client) NewRbacClient(application Application) (WrappedClientInterface, error) {
	url, err := url2.JoinPath(config.Get().RbacBaseURL, APIPath)
	if err != nil {
		c.log.WithField("error", err.Error()).Error(ErrCreatingRbacURL.Error())
		return nil, ErrCreatingRbacURL
	}

	wrappedClient := rbacClient.NewClient(url, string(application))
	wrappedClient.HTTPClient = clients.ConfigureClientWithTLS(wrappedClient.HTTPClient)
	return &wrappedClient, nil
}

// GetAccessList return the application rbac access list
func (c *Client) GetAccessList(application Application) (rbacClient.AccessList, error) {
	conf := config.Get()
	rbacTimeout := time.Duration(conf.RbacTimeout) * DefaultTimeDuration

	ctx, cancel := context.WithTimeout(c.ctx, rbacTimeout)
	defer cancel()

	wrappedClient, err := c.NewRbacClient(application)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("error occurred when creating rbac client")
		return nil, err
	}

	var identity string
	if config.Get().Auth {
		identity, err = common.GetOriginalIdentity(ctx)
		if err != nil {
			c.log.WithField("error", err.Error()).Error("error getting identity from context")
			return nil, ErrGettingIdentityFromContext
		}
	}

	acl, err := wrappedClient.GetAccess(ctx, identity, "")
	if err != nil {
		c.log.WithField("error", err.Error()).Error("error occurred getting rbac AccessList")
		return nil, err
	}
	return acl, nil
}

// getAssessGroupsFromResourceDefinition validate and return the access groups
func (c *Client) getAssessGroupsFromResourceDefinition(resourceDefinition rbacClient.ResourceDefinition) ([]*string, error) {
	if resourceDefinition.Filter.Key != "group.id" {
		c.log.WithField("filter-key", resourceDefinition.Filter.Key).Error("received an unexpected resource filter key value")
		return nil, ErrInvalidAttributeFilterKey
	}
	if resourceDefinition.Filter.Operation != "in" {
		c.log.WithField("filter-operation", resourceDefinition.Filter.Key).Error("received an unexpected resource filter operation value")
		return nil, ErrInvalidAttributeFilterOperation
	}
	var accessGroups []*string
	if err := json.Unmarshal([]byte(resourceDefinition.Filter.Value), &accessGroups); err != nil {
		c.log.WithField("filter-value", resourceDefinition.Filter.Value).Error("received an unexpected resource filter value type")
		return nil, ErrInvalidAttributeFilterValueType
	}
	return accessGroups, nil
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
func (c *Client) GetInventoryGroupsAccess(acl rbacClient.AccessList, resource ResourceType, accessType AccessType) (bool, []string, bool, error) {
	var overallGroupIDS []string
	var overallGroupIDSMap = make(map[string]bool)
	var allowedAccess bool
	var globalUnGroupedHosts bool
	for _, ac := range acl {
		if ac.Application() == string(ApplicationInventory) && ResourceMatch(ResourceType(ac.Resource()), resource) && AccessMatch(AccessType(ac.Verb()), accessType) {
			allowedAccess = true
			for _, resourceDef := range ac.ResourceDefinitions {
				accessGroups, err := c.getAssessGroupsFromResourceDefinition(resourceDef)
				if err != nil {
					return false, nil, false, err
				}
				groups, unGroupedHosts, err := c.getGroupsFromAccessGroups(accessGroups)
				if err != nil {
					return false, nil, false, err
				}
				if unGroupedHosts {
					globalUnGroupedHosts = true
				}
				for _, groupUUID := range groups {
					if _, ok := overallGroupIDSMap[groupUUID]; !ok {
						overallGroupIDSMap[groupUUID] = true
						overallGroupIDS = append(overallGroupIDS, groupUUID)
					}
				}
			}
		}
	}
	return allowedAccess, overallGroupIDS, globalUnGroupedHosts, nil
}

// AccessMatch return whether the access type matches the required resource type
func AccessMatch(access1, access2 AccessType) bool {
	return access1 == access2 || access1 == AccessTypeAny
}

// ResourceMatch return whether the resource type matches the required resource type
func ResourceMatch(resource1, resource2 ResourceType) bool {
	return resource1 == resource2 || resource1 == ResourceTypeAny
}
