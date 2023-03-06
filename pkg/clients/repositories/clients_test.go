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
	"github.com/redhatinsights/edge-api/pkg/clients/repositories"

	"github.com/bxcodec/faker/v3"
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
	// restore the initial content sources url and apiRepositoriesPath
	defer func(contentSourcesURL, reposPath string) {
		config.Get().ContentSourcesURL = contentSourcesURL
		repositories.APIRepositoriesPath = reposPath
	}(initialContentSourceURL, initialAPIRepositoriesPath)

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
	}{
		{
			Name:       "should return the expected repos",
			HTTPStatus: http.StatusOK,
			IOReadAll:  io.ReadAll,
			Params:     repositories.ListRepositoriesParams{Limit: 40, Offset: 41, SortBy: "name", SortType: "asc"},
			Filters:    defaultFilters,
			Response: repositories.ListRepositoriesResponse{Data: []repositories.Repository{
				{Name: repoName},
			}},
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

			client := repositories.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
			assert.NotNil(t, client)

			repos, err := client.ListRepositories(testCase.Params, testCase.Filters)
			if testCase.ExpectedError == nil {
				assert.NoError(t, err)
				assert.Equal(t, repos, testCase.Response.Data)
			} else {
				assert.ErrorContains(t, err, testCase.ExpectedError.Error())
			}
		})
	}
}
