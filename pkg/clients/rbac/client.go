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

func (c *Client) NewRbacClient(application Application) (WrappedClientInterface, error) {
	url, err := url2.JoinPath(config.Get().RbacBaseURL, APIPath)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("error occurred while creating rbac url")
		return nil, ErrCreatingRbacURL
	}

	wrappedClient := rbacClient.NewClient(url, string(application))
	wrappedClient.HTTPClient = clients.ConfigureClientWithTLS(wrappedClient.HTTPClient)
	return &wrappedClient, nil
}

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

func (c *Client) GetInventoryGroupsAccess(acl rbacClient.AccessList, resource ResourceType, accessType AccessType) (bool, []string, bool, error) {
	var overallGroupIDS []string
	var overallGroupIDSMap = make(map[string]bool)
	var allowedAccess bool
	var hostsWithNoGroupsAssigned bool
	for _, ac := range acl {
		if ac.Application() == string(ApplicationInventory) && ResourceMatch(ResourceType(ac.Resource()), resource) && AccessMatch(AccessType(ac.Verb()), accessType) {
			allowedAccess = true
			for _, resourceDef := range ac.ResourceDefinitions {
				if resourceDef.Filter.Key != "group.id" {
					c.log.WithField("filter-key", resourceDef.Filter.Key).Error("received an unexpected resource filter key value")
					return false, nil, false, ErrInvalidAttributeFilterKey
				}
				if resourceDef.Filter.Operation != "in" {
					c.log.WithField("filter-operation", resourceDef.Filter.Key).Error("received an unexpected resource filter operation value")
					return false, nil, false, ErrInvalidAttributeFilterOperation
				}
				var accessGroupIDS []*string
				if err := json.Unmarshal([]byte(resourceDef.Filter.Value), &accessGroupIDS); err != nil {
					c.log.WithField("filter-value", resourceDef.Filter.Value).Error("received an unexpected resource filter value type")
					return false, nil, false, ErrInvalidAttributeFilterValueType
				}
				for _, groupUUID := range accessGroupIDS {
					if groupUUID == nil {
						hostsWithNoGroupsAssigned = true
					} else {
						if _, err := uuid.Parse(*groupUUID); err != nil {
							c.log.WithField("filter-uuid", *groupUUID).Error("error occurred while parsing uuid value")
							return false, nil, false, ErrInvalidAttributeFilterValue
						}
						if _, ok := overallGroupIDSMap[*groupUUID]; !ok {
							overallGroupIDSMap[*groupUUID] = true
							overallGroupIDS = append(overallGroupIDS, *groupUUID)
						}
					}
				}
			}
		}
	}
	return allowedAccess, overallGroupIDS, hostsWithNoGroupsAssigned, nil
}

func AccessMatch(access1, access2 AccessType) bool {
	return access1 == access2 || access1 == AccessTypeAny
}

func ResourceMatch(resource1, resource2 ResourceType) bool {
	return resource1 == resource2 || resource1 == ResourceTypeAny
}
