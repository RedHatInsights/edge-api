package inventorygroups_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/inventorygroups"

	"github.com/bxcodec/faker/v3"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestGetBaseURL(t *testing.T) {
	initialInventoryURL := config.Get().InventoryConfig.URL
	// restore the initial insights inventory url
	defer func(inventoryURL string) {
		config.Get().InventoryConfig.URL = inventoryURL
	}(initialInventoryURL)

	validInventoryHostURL := "http://insights-inventory:8000"
	testCases := []struct {
		Name            string
		InventoryURL    string
		ExpectedBaseURL string
		ExpectedError   error
	}{
		{
			Name:            "should return the expected url",
			InventoryURL:    validInventoryHostURL,
			ExpectedBaseURL: validInventoryHostURL + "/api/inventory/v1/groups",
			ExpectedError:   nil,
		},
		{
			Name:            "should return error when content-sources is un-parsable",
			InventoryURL:    "\t",
			ExpectedBaseURL: "",
			ExpectedError:   inventorygroups.ErrParsingURL,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			config.Get().InventoryConfig.URL = testCase.InventoryURL
			client := inventorygroups.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
			assert.NotNil(t, client)
			url, err := client.GetBaseURL()
			if testCase.ExpectedError == nil {
				assert.NoError(t, err)
				assert.NotNil(t, url)
				assert.Equal(t, testCase.ExpectedBaseURL, url.String())

			} else {
				assert.ErrorIs(t, err, testCase.ExpectedError)
			}
		})
	}
}

func TestListGroups(t *testing.T) {
	initialInventoryURL := config.Get().InventoryConfig.URL

	// restore the initial variables
	defer func(inventoryURL string) {
		config.Get().InventoryConfig.URL = inventoryURL
	}(initialInventoryURL)

	groupName := faker.UUIDHyphenated()
	defaultParams := inventorygroups.ListGroupsParams{Name: groupName, OrderBy: "name", OrderHow: "ASC"}

	testCases := []struct {
		Name              string
		InventoryURL      string
		IOReadAll         func(r io.Reader) ([]byte, error)
		HTTPStatus        int
		Response          inventorygroups.Response
		ResponseText      string
		Params            inventorygroups.ListGroupsParams
		ExpectedError     error
		ExpectedURLParams map[string]string
	}{
		{
			Name:       "should return the expected groups",
			HTTPStatus: http.StatusOK,
			IOReadAll:  io.ReadAll,
			Params:     defaultParams,
			Response: inventorygroups.Response{
				Results: []inventorygroups.Group{
					{Name: groupName},
				},
				Page: 1, PerPage: inventorygroups.DefaultPerPage, Total: 1, Count: 1,
			},
			ExpectedError: nil,
			ExpectedURLParams: map[string]string{
				"per_page":  strconv.Itoa(inventorygroups.DefaultPerPage),
				"page":      strconv.Itoa(1),
				"order_by":  "name",
				"order_how": "ASC",
				"name":      groupName,
			},
		},
		{
			Name:          "should return error when http status is not 200",
			HTTPStatus:    http.StatusBadRequest,
			IOReadAll:     io.ReadAll,
			Params:        defaultParams,
			Response:      inventorygroups.Response{},
			ExpectedError: inventorygroups.ErrGroupsRequestResponse,
			ExpectedURLParams: map[string]string{
				"per_page":  strconv.Itoa(inventorygroups.DefaultPerPage),
				"page":      strconv.Itoa(1),
				"order_by":  "name",
				"order_how": "ASC",
				"name":      groupName,
			},
		},
		{
			Name:          "should return error when parsing base url fails",
			InventoryURL:  "\t",
			IOReadAll:     io.ReadAll,
			HTTPStatus:    http.StatusOK,
			Params:        defaultParams,
			Response:      inventorygroups.Response{},
			ExpectedError: inventorygroups.ErrParsingURL,
		},
		{
			Name:          "should return error when client Do fail",
			InventoryURL:  "host-without-schema",
			IOReadAll:     io.ReadAll,
			HTTPStatus:    http.StatusOK,
			Params:        defaultParams,
			Response:      inventorygroups.Response{},
			ExpectedError: errors.New("unsupported protocol scheme"),
		},
		{
			Name:          "should return error when malformed json is returned",
			HTTPStatus:    http.StatusOK,
			IOReadAll:     io.ReadAll,
			Params:        defaultParams,
			ResponseText:  `{"data: {}}`,
			ExpectedError: errors.New("unexpected end of JSON input"),
		},
		{
			Name:       "should return error when body readAll fails",
			HTTPStatus: http.StatusOK,
			IOReadAll: func(r io.Reader) ([]byte, error) {
				return nil, errors.New("expected error for when reading response body fails")
			},
			Params:        defaultParams,
			Response:      inventorygroups.Response{},
			ExpectedError: errors.New("expected error for when reading response body fails"),
		},
	}

	for _, testCase := range testCases {
		// avoid Implicit memory aliasing
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			// restore the initial variable
			defer func() {
				inventorygroups.IOReadAll = io.ReadAll
				config.Get().InventoryConfig.URL = initialInventoryURL
			}()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				urlQueryValues := r.URL.Query()
				for key, value := range testCase.ExpectedURLParams {
					assert.Equal(t, value, urlQueryValues.Get(key))
				}
				if testCase.ResponseText != "" {
					_, err := fmt.Fprint(w, testCase.ResponseText)
					assert.NoError(t, err)
					return
				}
				w.WriteHeader(testCase.HTTPStatus)
				err := json.NewEncoder(w).Encode(&testCase.Response)
				assert.NoError(t, err)
			}))
			defer ts.Close()

			inventorygroups.IOReadAll = testCase.IOReadAll
			if testCase.InventoryURL == "" {
				config.Get().InventoryConfig.URL = ts.URL
			} else {
				config.Get().InventoryConfig.URL = testCase.InventoryURL
			}

			client := inventorygroups.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
			assert.NotNil(t, client)

			response, err := client.ListGroups(testCase.Params)
			if testCase.ExpectedError == nil {
				assert.NoError(t, err)
				assert.Equal(t, response.Results, testCase.Response.Results)
				assert.Equal(t, response.Count, testCase.Response.Count)
				assert.Equal(t, response.Total, testCase.Response.Total)
				assert.Equal(t, response.Page, testCase.Response.Page)
				assert.Equal(t, response.PerPage, testCase.Response.PerPage)
			} else {
				assert.ErrorContains(t, err, testCase.ExpectedError.Error())
			}
		})
	}
}

func TestGetGroupByName(t *testing.T) {
	initialInventoryURL := config.Get().InventoryConfig.URL
	// restore the initial content sources url
	defer func(inventoryURL string) {
		config.Get().InventoryConfig.URL = inventoryURL
	}(initialInventoryURL)
	groupName := faker.UUIDHyphenated()
	testCases := []struct {
		Name              string
		HTTPStatus        int
		groupName         string
		Group             *inventorygroups.Group
		ExpectedURLParams map[string]string
		ExpectedError     error
	}{
		{
			Name:              "should return group successfully",
			groupName:         groupName,
			Group:             &inventorygroups.Group{Name: groupName},
			HTTPStatus:        http.StatusOK,
			ExpectedURLParams: map[string]string{"page": strconv.Itoa(1), "per_page": strconv.Itoa(1), "name": groupName},
			ExpectedError:     nil,
		},
		{
			Name:              "should return error when group not found",
			groupName:         groupName,
			Group:             nil,
			HTTPStatus:        http.StatusOK,
			ExpectedURLParams: map[string]string{"page": strconv.Itoa(1), "per_page": strconv.Itoa(1), "name": groupName},
			ExpectedError:     inventorygroups.ErrGroupNotFound,
		},
		{
			Name:              "should return error when the group found has a different name",
			groupName:         groupName,
			Group:             &inventorygroups.Group{Name: groupName + faker.UUIDHyphenated()},
			HTTPStatus:        http.StatusOK,
			ExpectedURLParams: map[string]string{"page": strconv.Itoa(1), "per_page": strconv.Itoa(1), "name": groupName},
			ExpectedError:     inventorygroups.ErrGroupNotFound,
		},
		{
			Name:          "should return error when group name is empty",
			groupName:     "",
			ExpectedError: inventorygroups.ErrGroupNameIsMandatory,
		},
		{
			Name:          "should return error when ListGroups fails",
			groupName:     groupName,
			HTTPStatus:    http.StatusBadRequest,
			ExpectedError: inventorygroups.ErrGroupsRequestResponse,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				urlQueryValues := r.URL.Query()
				for key, value := range testCase.ExpectedURLParams {
					assert.Equal(t, value, urlQueryValues.Get(key))
				}
				w.WriteHeader(testCase.HTTPStatus)
				response := inventorygroups.Response{Results: []inventorygroups.Group{}}
				if testCase.Group != nil {
					response.Results = append(response.Results, *testCase.Group)
				}
				err := json.NewEncoder(w).Encode(&response)
				assert.NoError(t, err)
			}))
			defer ts.Close()
			config.Get().InventoryConfig.URL = ts.URL

			client := inventorygroups.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
			assert.NotNil(t, client)

			group, err := client.GetGroupByName(testCase.groupName)
			if testCase.ExpectedError == nil {
				assert.NoError(t, err)
				assert.NotNil(t, group)
				assert.NotNil(t, testCase.Group)
				assert.Equal(t, *testCase.Group, *group)
			} else {
				assert.Error(t, err)
				assert.ErrorIs(t, err, testCase.ExpectedError)
			}
		})
	}
}

func TestGetGroupByUUID(t *testing.T) {
	initialInventoryURL := config.Get().InventoryConfig.URL

	// restore the initial inventory url
	defer func(inventoryURL string) {
		config.Get().InventoryConfig.URL = inventoryURL
	}(initialInventoryURL)

	groupUUID := faker.UUIDHyphenated()
	group := inventorygroups.Group{Name: faker.UUIDHyphenated(), ID: groupUUID}

	testCases := []struct {
		Name          string
		InventoryURL  string
		UUID          string
		IOReadAll     func(r io.Reader) ([]byte, error)
		HTTPStatus    int
		Response      *inventorygroups.Response
		ResponseText  string
		ExpectedError error
	}{
		{
			Name:          "should return the expected group",
			UUID:          groupUUID,
			HTTPStatus:    http.StatusOK,
			IOReadAll:     io.ReadAll,
			Response:      &inventorygroups.Response{Results: []inventorygroups.Group{group}},
			ExpectedError: nil,
		},
		{
			Name:          "should return ErrGroupNotFound when response result is empty",
			UUID:          groupUUID,
			HTTPStatus:    http.StatusOK,
			IOReadAll:     io.ReadAll,
			Response:      &inventorygroups.Response{},
			ExpectedError: inventorygroups.ErrGroupNotFound,
		},
		{
			Name:          "should return error when http status is not 200",
			UUID:          groupUUID,
			HTTPStatus:    http.StatusBadRequest,
			IOReadAll:     io.ReadAll,
			ExpectedError: inventorygroups.ErrGroupsRequestResponse,
		},
		{
			Name:          "should return error when parsing base url fails",
			UUID:          groupUUID,
			InventoryURL:  "\t",
			IOReadAll:     io.ReadAll,
			HTTPStatus:    http.StatusOK,
			ExpectedError: inventorygroups.ErrParsingURL,
		},
		{
			Name:          "should return error when client Do fail",
			UUID:          groupUUID,
			InventoryURL:  "host-without-schema",
			IOReadAll:     io.ReadAll,
			HTTPStatus:    http.StatusOK,
			ExpectedError: errors.New("unsupported protocol scheme"),
		},
		{
			Name:          "should return error when malformed json is returned",
			UUID:          groupUUID,
			HTTPStatus:    http.StatusOK,
			IOReadAll:     io.ReadAll,
			ResponseText:  `{"data: {}}`,
			ExpectedError: errors.New("unexpected end of JSON input"),
		},
		{
			Name:       "should return error when body readAll fails",
			UUID:       groupUUID,
			HTTPStatus: http.StatusOK,
			IOReadAll: func(r io.Reader) ([]byte, error) {
				return nil, errors.New("expected error for when reading response body fails")
			},
			Response:      nil,
			ExpectedError: errors.New("expected error for when reading response body fails"),
		},
		{
			Name:          "should return error when uuid is empty",
			UUID:          "",
			ExpectedError: inventorygroups.ErrGroupUUIDIsMandatory,
		},
	}

	for _, testCase := range testCases {
		// avoid Implicit memory aliasing
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			// restore the initial IOReadAll
			defer func() {
				inventorygroups.IOReadAll = io.ReadAll
			}()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(testCase.HTTPStatus)
				if testCase.ResponseText != "" {
					_, err := fmt.Fprint(w, testCase.ResponseText)
					assert.NoError(t, err)
					return
				}
				if testCase.Response != nil {
					err := json.NewEncoder(w).Encode(&testCase.Response)
					assert.NoError(t, err)
				}
			}))
			defer ts.Close()

			inventorygroups.IOReadAll = testCase.IOReadAll
			if testCase.InventoryURL == "" {
				config.Get().InventoryConfig.URL = ts.URL
			} else {
				config.Get().InventoryConfig.URL = testCase.InventoryURL
			}

			client := inventorygroups.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
			assert.NotNil(t, client)

			response, err := client.GetGroupByUUID(testCase.UUID)
			if testCase.ExpectedError == nil {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				assert.Equal(t, *response, testCase.Response.Results[0])
			} else {
				assert.ErrorContains(t, err, testCase.ExpectedError.Error())
			}
		})
	}
}

// ErrFaultyWriter the error returned by FaultWriter's Write function
var ErrFaultyWriter = errors.New("faulty writer expected write error")

// FaultyWriter a struct that implement a Writer that return always error when writing
type FaultyWriter struct{}

func (f *FaultyWriter) Write(_ []byte) (n int, err error) {
	return 0, ErrFaultyWriter
}

func TestCreateGroup(t *testing.T) {
	initialInventoryURL := config.Get().InventoryConfig.URL

	// restore the initial inventory url
	defer func(inventoryURL string) {
		config.Get().InventoryConfig.URL = inventoryURL
	}(initialInventoryURL)

	groupNameToCreate := faker.UUIDHyphenated()
	groupHostIDS := []string{faker.UUIDHyphenated(), faker.UUIDHyphenated()}
	responseGroup := inventorygroups.Group{
		Name: groupNameToCreate, ID: faker.UUIDHyphenated(), OrgID: faker.UUIDHyphenated(), HostCount: len(groupHostIDS),
	}

	testCases := []struct {
		Name              string
		InventoryURL      string
		GroupNameToCreate string
		GroupHostsToAdd   []string
		IOReadAll         func(r io.Reader) ([]byte, error)
		NewJSONEncoder    func(w io.Writer) *json.Encoder
		HTTPStatus        int
		Response          *inventorygroups.Group
		ResponseText      string
		ExpectedError     error
	}{
		{
			Name:              "should create group successfully",
			GroupNameToCreate: groupNameToCreate,
			GroupHostsToAdd:   groupHostIDS,
			HTTPStatus:        http.StatusCreated,
			IOReadAll:         io.ReadAll,
			NewJSONEncoder:    json.NewEncoder,
			Response:          &responseGroup,
			ExpectedError:     nil,
		},
		{
			Name:              "should return error when group name is empty",
			GroupNameToCreate: "",
			HTTPStatus:        http.StatusCreated,
			IOReadAll:         io.ReadAll,
			NewJSONEncoder:    json.NewEncoder,
			ExpectedError:     inventorygroups.ErrGroupNameIsMandatory,
		},
		{
			Name:              "should return error when http status is not 201",
			GroupNameToCreate: groupNameToCreate,
			HTTPStatus:        http.StatusBadRequest,
			IOReadAll:         io.ReadAll,
			NewJSONEncoder:    json.NewEncoder,
			ExpectedError:     inventorygroups.ErrGroupsRequestResponse,
		},
		{
			Name:              "should return error when parsing base url fails",
			GroupNameToCreate: groupNameToCreate,
			InventoryURL:      "\t",
			IOReadAll:         io.ReadAll,
			NewJSONEncoder:    json.NewEncoder,
			HTTPStatus:        http.StatusCreated,
			ExpectedError:     inventorygroups.ErrParsingURL,
		},
		{
			Name:              "should return error when client Do fail",
			GroupNameToCreate: groupNameToCreate,
			InventoryURL:      "host-without-schema",
			IOReadAll:         io.ReadAll,
			NewJSONEncoder:    json.NewEncoder,
			HTTPStatus:        http.StatusCreated,
			ExpectedError:     errors.New("unsupported protocol scheme"),
		},
		{
			Name:              "should return error when malformed json is returned",
			GroupNameToCreate: groupNameToCreate,
			HTTPStatus:        http.StatusCreated,
			IOReadAll:         io.ReadAll,
			NewJSONEncoder:    json.NewEncoder,
			ResponseText:      `{"data: {}}`,
			ExpectedError:     errors.New("unexpected end of JSON input"),
		},
		{
			Name:              "should return error when body readAll fails",
			GroupNameToCreate: groupNameToCreate,
			HTTPStatus:        http.StatusCreated,
			IOReadAll: func(r io.Reader) ([]byte, error) {
				return nil, errors.New("expected error for when reading response body fails")
			},
			NewJSONEncoder: json.NewEncoder,
			Response:       nil,
			ExpectedError:  errors.New("expected error for when reading response body fails"),
		},
		{
			Name:              "should return error when payload json encode fails",
			GroupNameToCreate: groupNameToCreate,
			IOReadAll:         io.ReadAll,
			NewJSONEncoder: func(_ io.Writer) *json.Encoder {
				// ignore any argument and return the FaultyWriter, that returns error when calling its Write function
				return json.NewEncoder(&FaultyWriter{})
			},
			ExpectedError: ErrFaultyWriter,
		},
	}

	for _, testCase := range testCases {
		// avoid Implicit memory aliasing
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			// restore the initial IOReadAll and NewJSONEncoder
			defer func() {
				inventorygroups.IOReadAll = io.ReadAll
				inventorygroups.NewJSONEncoder = json.NewEncoder
			}()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(testCase.HTTPStatus)
				if testCase.ResponseText != "" {
					_, err := fmt.Fprint(w, testCase.ResponseText)
					assert.NoError(t, err)
					return
				}
				if testCase.Response != nil {
					err := json.NewEncoder(w).Encode(&testCase.Response)
					assert.NoError(t, err)
				}
			}))
			defer ts.Close()

			inventorygroups.IOReadAll = testCase.IOReadAll
			inventorygroups.NewJSONEncoder = testCase.NewJSONEncoder

			if testCase.InventoryURL == "" {
				config.Get().InventoryConfig.URL = ts.URL
			} else {
				config.Get().InventoryConfig.URL = testCase.InventoryURL
			}

			client := inventorygroups.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
			assert.NotNil(t, client)

			response, err := client.CreateGroup(testCase.GroupNameToCreate, testCase.GroupHostsToAdd)
			if testCase.ExpectedError == nil {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				assert.NotNil(t, testCase.Response)
				assert.Equal(t, *response, *testCase.Response)
			} else {
				assert.ErrorContains(t, err, testCase.ExpectedError.Error())
			}
		})
	}
}

func TestAddHostsToGroup(t *testing.T) {
	initialInventoryURL := config.Get().InventoryConfig.URL

	// restore the initial inventory url
	defer func(inventoryURL string) {
		config.Get().InventoryConfig.URL = inventoryURL
	}(initialInventoryURL)

	groupUUID := faker.UUIDHyphenated()
	hostsToAdd := []string{faker.UUIDHyphenated(), faker.UUIDHyphenated()}
	responseGroup := inventorygroups.Group{
		Name: faker.UUIDHyphenated(), ID: groupUUID, OrgID: faker.UUIDHyphenated(), HostCount: len(hostsToAdd),
	}

	testCases := []struct {
		Name           string
		InventoryURL   string
		GroupUUID      string
		HostsToAdd     []string
		IOReadAll      func(r io.Reader) ([]byte, error)
		NewJSONEncoder func(w io.Writer) *json.Encoder
		HTTPStatus     int
		Response       *inventorygroups.Group
		ResponseText   string
		ExpectedError  error
	}{
		{
			Name:           "should add hosts to group successfully",
			GroupUUID:      groupUUID,
			HostsToAdd:     hostsToAdd,
			HTTPStatus:     http.StatusOK,
			IOReadAll:      io.ReadAll,
			NewJSONEncoder: json.NewEncoder,
			Response:       &responseGroup,
			ExpectedError:  nil,
		},
		{
			Name:           "should return error when group uuid is empty",
			GroupUUID:      "",
			HostsToAdd:     hostsToAdd,
			HTTPStatus:     http.StatusOK,
			IOReadAll:      io.ReadAll,
			NewJSONEncoder: json.NewEncoder,
			ExpectedError:  inventorygroups.ErrGroupUUIDIsMandatory,
		},
		{
			Name:           "should return error when no hosts supplied",
			GroupUUID:      groupUUID,
			HostsToAdd:     []string{},
			HTTPStatus:     http.StatusOK,
			IOReadAll:      io.ReadAll,
			NewJSONEncoder: json.NewEncoder,
			ExpectedError:  inventorygroups.ErrGroupHostsAreMandatory,
		},
		{
			Name:           "should return error when http status is not 200",
			GroupUUID:      groupUUID,
			HostsToAdd:     hostsToAdd,
			HTTPStatus:     http.StatusBadRequest,
			IOReadAll:      io.ReadAll,
			NewJSONEncoder: json.NewEncoder,
			ExpectedError:  inventorygroups.ErrGroupsRequestResponse,
		},
		{
			Name:           "should return error when parsing base url fails",
			GroupUUID:      groupUUID,
			HostsToAdd:     hostsToAdd,
			InventoryURL:   "\t",
			IOReadAll:      io.ReadAll,
			NewJSONEncoder: json.NewEncoder,
			HTTPStatus:     http.StatusOK,
			ExpectedError:  inventorygroups.ErrParsingURL,
		},
		{
			Name:           "should return error when client Do fail",
			GroupUUID:      groupUUID,
			HostsToAdd:     hostsToAdd,
			InventoryURL:   "host-without-schema",
			IOReadAll:      io.ReadAll,
			NewJSONEncoder: json.NewEncoder,
			HTTPStatus:     http.StatusOK,
			ExpectedError:  errors.New("unsupported protocol scheme"),
		},
		{
			Name:           "should return error when malformed json is returned",
			GroupUUID:      groupUUID,
			HostsToAdd:     hostsToAdd,
			HTTPStatus:     http.StatusOK,
			IOReadAll:      io.ReadAll,
			NewJSONEncoder: json.NewEncoder,
			ResponseText:   `{"data: {}}`,
			ExpectedError:  errors.New("unexpected end of JSON input"),
		},
		{
			Name:       "should return error when body readAll fails",
			GroupUUID:  groupUUID,
			HostsToAdd: hostsToAdd,
			HTTPStatus: http.StatusOK,
			IOReadAll: func(r io.Reader) ([]byte, error) {
				return nil, errors.New("expected error for when reading response body fails")
			},
			NewJSONEncoder: json.NewEncoder,
			Response:       nil,
			ExpectedError:  errors.New("expected error for when reading response body fails"),
		},
		{
			Name:       "should return error when payload json encode fails",
			GroupUUID:  groupUUID,
			HostsToAdd: hostsToAdd,
			IOReadAll:  io.ReadAll,
			NewJSONEncoder: func(_ io.Writer) *json.Encoder {
				// ignore any argument and return the FaultyWriter, that returns error when calling its Write function
				return json.NewEncoder(&FaultyWriter{})
			},
			ExpectedError: ErrFaultyWriter,
		},
	}

	for _, testCase := range testCases {
		// avoid Implicit memory aliasing
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			// restore the initial IOReadAll and NewJSONEncoder
			defer func() {
				inventorygroups.IOReadAll = io.ReadAll
				inventorygroups.NewJSONEncoder = json.NewEncoder
			}()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(testCase.HTTPStatus)
				if testCase.ResponseText != "" {
					_, err := fmt.Fprint(w, testCase.ResponseText)
					assert.NoError(t, err)
					return
				}
				if testCase.Response != nil {
					err := json.NewEncoder(w).Encode(&testCase.Response)
					assert.NoError(t, err)
				}
			}))
			defer ts.Close()

			inventorygroups.IOReadAll = testCase.IOReadAll
			inventorygroups.NewJSONEncoder = testCase.NewJSONEncoder

			if testCase.InventoryURL == "" {
				config.Get().InventoryConfig.URL = ts.URL
			} else {
				config.Get().InventoryConfig.URL = testCase.InventoryURL
			}

			client := inventorygroups.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
			assert.NotNil(t, client)

			response, err := client.AddHostsToGroup(testCase.GroupUUID, testCase.HostsToAdd)
			if testCase.ExpectedError == nil {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				assert.NotNil(t, testCase.Response)
				assert.Equal(t, *response, *testCase.Response)
			} else {
				assert.ErrorContains(t, err, testCase.ExpectedError.Error())
			}
		})
	}
}
