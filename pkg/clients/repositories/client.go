package repositories

import (
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

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type Repository struct {
	UUID                         uuid.UUID `json:"uuid"`
	Name                         string    `json:"name"`
	URL                          string    `json:"url"`
	DistributionVersions         []string  `json:"distribution_versions"`
	DistributionArch             string    `json:"distribution_arch"`
	AccountID                    string    `json:"account_id"`
	OrgID                        string    `json:"org_id"`
	LastIntrospectionTime        string    `json:"last_introspection_time"`
	LastSuccessIntrospectionTime string    `json:"last_success_introspection_time"`
	LastUpdateIntrospectionTime  string    `json:"last_update_introspection_time"`
	LastIntrospectionError       string    `json:"last_introspection_error"`
	PackageCount                 int       `json:"package_count"`
	Status                       string    `json:"status"`
	GpgKey                       string    `json:"gpg_key"`
	MetadataVerification         bool      `json:"metadata_verification"`
}

type ListRepositoriesParams struct {
	Limit    int
	Offset   int
	SortBy   string
	SortType string
}

type ListRepositoriesMeta struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Count  int `json:"count"`
}

type ListRepositoriesResponse struct {
	Data []Repository         `json:"data"`
	Meta ListRepositoriesMeta `json:"meta"`
}

type ListRepositoriesFilters map[string]string

func NewListRepositoryFilters() ListRepositoriesFilters {
	return make(ListRepositoriesFilters)
}

func (filters ListRepositoriesFilters) Add(name, value string) {
	filters[name] = value
}

// ClientInterface is an Interface to make request to content sources repositories
type ClientInterface interface {
	GetBaseURL() (*url2.URL, error)
	GetRepositoryByName(name string) (*Repository, error)
	ListRepositories(requestParams ListRepositoriesParams, filters ListRepositoriesFilters) (*ListRepositoriesResponse, error)
}

// Client is the implementation of an ClientInterface
type Client struct {
	ctx context.Context
	log *log.Entry
}

// InitClient initializes the client for Content source repositories
func InitClient(ctx context.Context, log *log.Entry) *Client {
	return &Client{ctx: ctx, log: log}
}

// DefaultLimit The default data list Limit to be returned
const DefaultLimit = 20

// APIPath The content-sources base api path
const APIPath = "/api/content-sources"

// APIVersion The content-sources api version
const APIVersion = "v1"

// APIRepositoriesPath The api repositories path
var APIRepositoriesPath = "repositories"

// IOReadAll The io body reader
var IOReadAll = io.ReadAll

var ErrRepositoryRequestResponse = errors.New("repository request error response")
var ErrParsingRawURL = errors.New("error occurred while parsing raw url")
var ErrRepositoryNameIsMandatory = errors.New("repository name is mandatory")
var ErrRepositoryNoFound = errors.New("repository not found")

// GetBaseURL return the base url of content sources service
func (c *Client) GetBaseURL() (*url2.URL, error) {
	baseURL := config.Get().ContentSourcesURL + APIPath
	url, err := url2.Parse(baseURL)
	if err != nil {
		c.log.WithFields(log.Fields{"url": baseURL, "error": err.Error()}).Error("failed to parse content-sources base url")
		return nil, ErrParsingRawURL
	}
	return url, nil
}

// GetRepositoryByName return the content-sources repository filtering by name
func (c *Client) GetRepositoryByName(name string) (*Repository, error) {
	if name == "" {
		c.log.Error("repository name is mandatory")
		return nil, ErrRepositoryNameIsMandatory
	}
	repos, err := c.ListRepositories(ListRepositoriesParams{Limit: 1}, ListRepositoriesFilters{"name": name})
	if err != nil {
		c.log.WithFields(log.Fields{"repository-name": name, "error": err.Error()}).Error("failed when calling to ListRepositories")
		return nil, err
	}
	if len(repos.Data) == 0 {
		c.log.WithField("repository-name", name).Error("repository not found")
		return nil, ErrRepositoryNoFound
	}
	return &repos.Data[0], nil
}

// ListRepositories return the list of content source repositories
func (c *Client) ListRepositories(requestParams ListRepositoriesParams, filters ListRepositoriesFilters) (*ListRepositoriesResponse, error) {
	url, err := c.GetBaseURL()
	if err != nil {
		return nil, err
	}
	repositoriesRawURL := url.String() + fmt.Sprintf("/%s/%s/", APIVersion, APIRepositoriesPath)
	repositoriesURL, err := url.Parse(repositoriesRawURL)
	if err != nil {
		c.log.WithFields(log.Fields{"url": repositoriesRawURL, "error": err.Error()}).Error("failed to parse repositories url")
		return nil, ErrParsingRawURL
	}

	queryValues := repositoriesURL.Query()
	if requestParams.Limit == 0 {
		requestParams.Limit = DefaultLimit
	}
	queryValues.Add("limit", strconv.Itoa(requestParams.Limit))
	queryValues.Add("offset", strconv.Itoa(requestParams.Offset))
	if requestParams.SortBy != "" {
		sortBy := requestParams.SortBy
		if requestParams.SortType != "" {
			sortBy = sortBy + ":" + requestParams.SortType
		}
		queryValues.Add("sort_by", sortBy)
	}

	// add filters to queryValues
	for fieldName, fieldValue := range filters {
		queryValues.Add(fieldName, fieldValue)
	}
	// set queryValues to repository url
	repositoriesURL.RawQuery = queryValues.Encode()
	requestURL := repositoriesURL.String()

	c.log.WithField("url", requestURL).Info("content source repositories request started")
	req, _ := http.NewRequest("GET", requestURL, nil)
	req.Header.Add("Content-Type", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	client := clients.ConfigureHttpClient(&http.Client{})
	res, err := client.Do(req)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("content source repositories request error")
		return nil, err
	}
	defer res.Body.Close()

	body, err := IOReadAll(res.Body)
	if err != nil {
		c.log.WithFields(log.Fields{"statusCode": res.StatusCode, "error": err.Error()}).Error("content source repositories response error")
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		c.log.WithFields(log.Fields{"statusCode": res.StatusCode, "responseBody": string(body)}).Error("content source repositories error response")
		return nil, ErrRepositoryRequestResponse
	}

	var response ListRepositoriesResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("error occurred when unmarshalling response body")
		return nil, err
	}
	return &response, nil
}
