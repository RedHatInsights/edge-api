package repositories_test

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
	"github.com/redhatinsights/edge-api/pkg/clients"
	"github.com/redhatinsights/edge-api/pkg/clients/repositories"

	"github.com/bxcodec/faker/v3"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestGetBaseURL(t *testing.T) {

	initialContentSourceURL := config.Get().ContentSourcesURL
	// restore the initial content sources url
	defer func(contentSourcesURL string) {
		config.Get().ContentSourcesURL = contentSourcesURL
	}(initialContentSourceURL)

	validContentSourcesHostURL := "http://content-sources:8000"
	testCases := []struct {
		Name              string
		ContentSourcesURL string
		ExpectedBaseURL   string
		ExpectedError     error
	}{
		{
			Name:              "should return the expected url",
			ContentSourcesURL: validContentSourcesHostURL,
			ExpectedBaseURL:   validContentSourcesHostURL + "/api/content-sources",
			ExpectedError:     nil,
		},
		{
			Name:              "should return error when content-sources is un-parsable",
			ContentSourcesURL: "\t",
			ExpectedBaseURL:   "",
			ExpectedError:     repositories.ErrParsingRawURL,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			config.Get().ContentSourcesURL = testCase.ContentSourcesURL
			client := repositories.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
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

func TestListRepositories(t *testing.T) {
	initialContentSourceURL := config.Get().ContentSourcesURL
	initialAPIRepositoriesPath := repositories.APIRepositoriesPath
	originalTLScaPATH := config.Get().TlsCAPath
	// restore the initial content sources url and apiRepositoriesPath
	defer func(contentSourcesURL, reposPath string, tlsCAPath string) {
		config.Get().ContentSourcesURL = contentSourcesURL
		repositories.APIRepositoriesPath = reposPath
		config.Get().TlsCAPath = tlsCAPath
	}(initialContentSourceURL, initialAPIRepositoriesPath, originalTLScaPATH)

	repoName := faker.UUIDHyphenated()
	defaultFilters := repositories.NewListRepositoryFilters()
	defaultFilters.Add("name", repoName)

	testCases := []struct {
		Name                string
		ContentSourcesURL   string
		APIRepositoriesPath string
		IOReadAll           func(r io.Reader) ([]byte, error)
		HTTPStatus          int
		Response            repositories.ListRepositoriesResponse
		ResponseText        string
		Params              repositories.ListRepositoriesParams
		Filters             repositories.ListRepositoriesFilters
		ExpectedError       error
		ExpectedURLParams   map[string]string
		TLSCAPath           string
	}{
		{
			Name:       "should return the expected repos",
			TLSCAPath:  "/test_TLS",
			HTTPStatus: http.StatusOK,
			IOReadAll:  io.ReadAll,
			Params:     repositories.ListRepositoriesParams{Limit: 40, Offset: 41, SortBy: "name", SortType: "asc"},
			Filters:    defaultFilters,
			Response: repositories.ListRepositoriesResponse{
				Data: []repositories.Repository{
					{Name: repoName},
				},
				Meta: repositories.ListRepositoriesMeta{Count: 1, Limit: 40, Offset: 41},
			},
			ExpectedError: nil,
			ExpectedURLParams: map[string]string{
				"limit":   strconv.Itoa(40),
				"offset":  strconv.Itoa(41),
				"sort_by": "name:asc",
				"name":    repoName,
			},
		},
		{
			Name:          "should return error when http status is not 200",
			HTTPStatus:    http.StatusBadRequest,
			IOReadAll:     io.ReadAll,
			Params:        repositories.ListRepositoriesParams{SortBy: "name", SortType: "desc"},
			Filters:       defaultFilters,
			Response:      repositories.ListRepositoriesResponse{},
			ExpectedError: repositories.ErrRepositoryRequestResponse,
			ExpectedURLParams: map[string]string{
				"limit":   strconv.Itoa(repositories.DefaultLimit),
				"offset":  strconv.Itoa(0),
				"sort_by": "name:desc",
				"name":    repoName,
			},
		},
		{
			Name:              "should return error when parsing base url fails",
			ContentSourcesURL: "\t",
			IOReadAll:         io.ReadAll,
			HTTPStatus:        http.StatusOK,
			Params:            repositories.ListRepositoriesParams{SortBy: "name", SortType: "desc"},
			Filters:           defaultFilters,
			Response:          repositories.ListRepositoriesResponse{},
			ExpectedError:     repositories.ErrParsingRawURL,
		},
		{
			Name:                "should return error when parsing apiRepositoriesPath fails",
			APIRepositoriesPath: "\t",
			IOReadAll:           io.ReadAll,
			HTTPStatus:          http.StatusOK,
			Params:              repositories.ListRepositoriesParams{SortBy: "name", SortType: "desc"},
			Filters:             defaultFilters,
			Response:            repositories.ListRepositoriesResponse{},
			ExpectedError:       repositories.ErrParsingRawURL,
		},
		{
			Name:              "should return error when client Do fail",
			ContentSourcesURL: "host-without-schema",
			IOReadAll:         io.ReadAll,
			HTTPStatus:        http.StatusOK,
			Params:            repositories.ListRepositoriesParams{SortBy: "name", SortType: "desc"},
			Filters:           defaultFilters,
			Response:          repositories.ListRepositoriesResponse{},
			ExpectedError:     errors.New("unsupported protocol scheme"),
		},
		{
			Name:          "should return error when malformed json is returned",
			HTTPStatus:    http.StatusOK,
			IOReadAll:     io.ReadAll,
			Params:        repositories.ListRepositoriesParams{SortBy: "name", SortType: "desc"},
			Filters:       defaultFilters,
			ResponseText:  `{"data: {}}`,
			ExpectedError: errors.New("unexpected end of JSON input"),
		},
		{
			Name:       "should return error when body readAll fails",
			HTTPStatus: http.StatusOK,
			IOReadAll: func(r io.Reader) ([]byte, error) {
				return nil, errors.New("expected error for when reading response body fails")
			},
			Params:        repositories.ListRepositoriesParams{SortBy: "name", SortType: "desc"},
			Filters:       defaultFilters,
			Response:      repositories.ListRepositoriesResponse{},
			ExpectedError: errors.New("expected error for when reading response body fails"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			initialAPIRepositoriesPath := repositories.APIRepositoriesPath
			// restore the initial apiRepositoriesPath and IOReadAll
			defer func(reposPath string) {
				repositories.APIRepositoriesPath = reposPath
				repositories.IOReadAll = io.ReadAll
			}(initialAPIRepositoriesPath)

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

			repositories.IOReadAll = testCase.IOReadAll
			if testCase.ContentSourcesURL == "" {
				config.Get().ContentSourcesURL = ts.URL
			} else {
				config.Get().ContentSourcesURL = testCase.ContentSourcesURL
			}
			if testCase.APIRepositoriesPath != "" {
				repositories.APIRepositoriesPath = testCase.APIRepositoriesPath
			}
			if testCase.TLSCAPath != "" {
				config.Get().TlsCAPath = testCase.TLSCAPath
			}

			client := repositories.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
			assert.NotNil(t, client)
			clientWithTLSPath := clients.ConfigureClientWithTLS(&http.Client{})
			assert.NotNil(t, clientWithTLSPath)

			response, err := client.ListRepositories(testCase.Params, testCase.Filters)
			if testCase.ExpectedError == nil {
				assert.NoError(t, err)
				assert.Equal(t, response.Data, testCase.Response.Data)
				assert.Equal(t, response.Meta.Count, testCase.Response.Meta.Count)
				assert.Equal(t, response.Meta.Limit, testCase.Response.Meta.Limit)
				assert.Equal(t, response.Meta.Offset, testCase.Response.Meta.Offset)
			} else {
				assert.ErrorContains(t, err, testCase.ExpectedError.Error())
			}
		})
	}
}

func TestGetRepositoryByName(t *testing.T) {
	initialContentSourceURL := config.Get().ContentSourcesURL
	// restore the initial content sources url
	defer func(contentSourcesURL string) {
		config.Get().ContentSourcesURL = contentSourcesURL
	}(initialContentSourceURL)
	repoName := faker.UUIDHyphenated()
	testCases := []struct {
		Name              string
		HTTPStatus        int
		RepoName          string
		Repository        *repositories.Repository
		ExpectedURLParams map[string]string
		ExpectedError     error
	}{
		{
			Name:              "should return repository successfully",
			RepoName:          repoName,
			Repository:        &repositories.Repository{Name: repoName},
			HTTPStatus:        http.StatusOK,
			ExpectedURLParams: map[string]string{"limit": strconv.Itoa(1), "offset": strconv.Itoa(0), "name": repoName},
			ExpectedError:     nil,
		},
		{
			Name:              "should return error when repository not found",
			RepoName:          repoName,
			Repository:        nil,
			HTTPStatus:        http.StatusOK,
			ExpectedURLParams: map[string]string{"limit": strconv.Itoa(1), "offset": strconv.Itoa(0), "name": repoName},
			ExpectedError:     repositories.ErrRepositoryNotFound,
		},
		{
			Name:          "should return error when repo name is empty",
			RepoName:      "",
			ExpectedError: repositories.ErrRepositoryNameIsMandatory,
		},
		{
			Name:          "should return error when ListRepositories fails",
			RepoName:      repoName,
			HTTPStatus:    http.StatusBadRequest,
			ExpectedError: repositories.ErrRepositoryRequestResponse,
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
				response := repositories.ListRepositoriesResponse{Data: []repositories.Repository{}}
				if testCase.Repository != nil {
					response.Data = append(response.Data, *testCase.Repository)
				}
				err := json.NewEncoder(w).Encode(&response)
				assert.NoError(t, err)
			}))
			defer ts.Close()
			config.Get().ContentSourcesURL = ts.URL

			client := repositories.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
			assert.NotNil(t, client)

			repository, err := client.GetRepositoryByName(testCase.RepoName)
			if testCase.ExpectedError == nil {
				assert.NoError(t, err)
				assert.NotNil(t, repository)
				assert.NotNil(t, testCase.Repository)
				assert.Equal(t, *testCase.Repository, *repository)
			} else {
				assert.Error(t, err)
				assert.ErrorIs(t, err, testCase.ExpectedError)
			}
		})
	}
}

func TestGetRepositoryByURL(t *testing.T) {
	initialContentSourceURL := config.Get().ContentSourcesURL
	// restore the initial content sources url
	defer func(contentSourcesURL string) {
		config.Get().ContentSourcesURL = contentSourcesURL
	}(initialContentSourceURL)
	repoURL := faker.URL()
	repoName := faker.UUIDHyphenated()
	testCases := []struct {
		Name              string
		HTTPStatus        int
		RepoURL           string
		Repository        *repositories.Repository
		ExpectedURLParams map[string]string
		ExpectedError     error
	}{
		{
			Name:              "should return repository successfully",
			RepoURL:           repoURL,
			Repository:        &repositories.Repository{Name: repoName, URL: repoURL},
			HTTPStatus:        http.StatusOK,
			ExpectedURLParams: map[string]string{"limit": strconv.Itoa(1), "offset": strconv.Itoa(0), "url": repoURL},
			ExpectedError:     nil,
		},
		{
			Name:              "should return error when repository not found",
			RepoURL:           repoURL,
			Repository:        nil,
			HTTPStatus:        http.StatusOK,
			ExpectedURLParams: map[string]string{"limit": strconv.Itoa(1), "offset": strconv.Itoa(0), "url": repoURL},
			ExpectedError:     repositories.ErrRepositoryNotFound,
		},
		{
			Name:          "should return error when repo name is empty",
			RepoURL:       "",
			ExpectedError: repositories.ErrRepositoryURLIsMandatory,
		},
		{
			Name:          "should return error when ListRepositories fails",
			RepoURL:       repoURL,
			HTTPStatus:    http.StatusBadRequest,
			ExpectedError: repositories.ErrRepositoryRequestResponse,
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
				response := repositories.ListRepositoriesResponse{Data: []repositories.Repository{}}
				if testCase.Repository != nil {
					response.Data = append(response.Data, *testCase.Repository)
				}
				err := json.NewEncoder(w).Encode(&response)
				assert.NoError(t, err)
			}))
			defer ts.Close()
			config.Get().ContentSourcesURL = ts.URL

			client := repositories.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
			assert.NotNil(t, client)

			repository, err := client.GetRepositoryByURL(testCase.RepoURL)
			if testCase.ExpectedError == nil {
				assert.NoError(t, err)
				assert.NotNil(t, repository)
				assert.NotNil(t, testCase.Repository)
				assert.Equal(t, *testCase.Repository, *repository)
			} else {
				assert.Error(t, err)
				assert.ErrorIs(t, err, testCase.ExpectedError)
			}
		})
	}
}

func TestGetRepositoryByUUID(t *testing.T) {
	initialContentSourceURL := config.Get().ContentSourcesURL
	initialAPIRepositoriesPath := repositories.APIRepositoriesPath
	// restore the initial content sources url and apiRepositoriesPath
	defer func(contentSourcesURL, reposPath string) {
		config.Get().ContentSourcesURL = contentSourcesURL
		repositories.APIRepositoriesPath = reposPath
	}(initialContentSourceURL, initialAPIRepositoriesPath)

	repoUUID := faker.UUIDHyphenated()
	repo := repositories.Repository{Name: faker.UUIDHyphenated(), URL: faker.URL(), UUID: uuid.MustParse(repoUUID)}

	testCases := []struct {
		Name                string
		ContentSourcesURL   string
		APIRepositoriesPath string
		UUID                string
		IOReadAll           func(r io.Reader) ([]byte, error)
		HTTPStatus          int
		Response            *repositories.Repository
		ResponseText        string
		ExpectedError       error
	}{
		{
			Name:          "should return the expected repos",
			UUID:          repoUUID,
			HTTPStatus:    http.StatusOK,
			IOReadAll:     io.ReadAll,
			Response:      &repo,
			ExpectedError: nil,
		},
		{
			Name:          "should return ErrRepositoryNotFound when http status is 404",
			UUID:          repoUUID,
			HTTPStatus:    http.StatusNotFound,
			IOReadAll:     io.ReadAll,
			ExpectedError: repositories.ErrRepositoryNotFound,
		},
		{
			Name:          "should return error when http status is not 200",
			UUID:          repoUUID,
			HTTPStatus:    http.StatusBadRequest,
			IOReadAll:     io.ReadAll,
			ExpectedError: repositories.ErrRepositoryRequestResponse,
		},
		{
			Name:              "should return error when parsing base url fails",
			UUID:              repoUUID,
			ContentSourcesURL: "\t",
			IOReadAll:         io.ReadAll,
			HTTPStatus:        http.StatusOK,
			ExpectedError:     repositories.ErrParsingRawURL,
		},
		{
			Name:                "should return error when parsing apiRepositoriesPath fails",
			UUID:                repoUUID,
			APIRepositoriesPath: "\t",
			IOReadAll:           io.ReadAll,
			HTTPStatus:          http.StatusOK,
			ExpectedError:       repositories.ErrParsingRawURL,
		},
		{
			Name:              "should return error when client Do fail",
			UUID:              repoUUID,
			ContentSourcesURL: "host-without-schema",
			IOReadAll:         io.ReadAll,
			HTTPStatus:        http.StatusOK,
			ExpectedError:     errors.New("unsupported protocol scheme"),
		},
		{
			Name:          "should return error when malformed json is returned",
			UUID:          repoUUID,
			HTTPStatus:    http.StatusOK,
			IOReadAll:     io.ReadAll,
			ResponseText:  `{"data: {}}`,
			ExpectedError: errors.New("unexpected end of JSON input"),
		},
		{
			Name:       "should return error when body readAll fails",
			UUID:       repoUUID,
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
			ExpectedError: repositories.ErrRepositoryUUIDIsMandatory,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			initialAPIRepositoriesPath := repositories.APIRepositoriesPath
			// restore the initial apiRepositoriesPath and IOReadAll
			defer func(reposPath string) {
				repositories.APIRepositoriesPath = reposPath
				repositories.IOReadAll = io.ReadAll
			}(initialAPIRepositoriesPath)

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

			repositories.IOReadAll = testCase.IOReadAll
			if testCase.ContentSourcesURL == "" {
				config.Get().ContentSourcesURL = ts.URL
			} else {
				config.Get().ContentSourcesURL = testCase.ContentSourcesURL
			}
			if testCase.APIRepositoriesPath != "" {
				repositories.APIRepositoriesPath = testCase.APIRepositoriesPath
			}

			client := repositories.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
			assert.NotNil(t, client)

			response, err := client.GetRepositoryByUUID(testCase.UUID)
			if testCase.ExpectedError == nil {
				assert.NoError(t, err)
				assert.Equal(t, response, testCase.Response)
			} else {
				assert.ErrorContains(t, err, testCase.ExpectedError.Error())
			}
		})
	}
}
