package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
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
			ExpectedError:     errors.New(`net/url: invalid control character in URL`),
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
				assert.ErrorContains(t, err, testCase.ExpectedError.Error())
			}
		})
	}
}

func TestListRepositories(t *testing.T) {
	initialContentSourceURL := config.Get().ContentSourcesURL
	// restore the initial content sources url
	defer func(contentSourcesURL string) {
		config.Get().ContentSourcesURL = contentSourcesURL
	}(initialContentSourceURL)

	repoName := faker.UUIDHyphenated()
	defaultFilters := repositories.NewListRepositoryFilters()
	defaultFilters.Add("name", repoName)

	testCases := []struct {
		Name              string
		HTTPStatus        int
		Response          repositories.ListRepositoriesResponse
		Params            repositories.ListRepositoriesParams
		Filters           repositories.ListRepositoriesFilters
		ExpectedError     error
		ExpectedURLParams map[string]string
	}{
		{
			Name:       "should return the expected repos",
			HTTPStatus: http.StatusOK,
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
				err := json.NewEncoder(w).Encode(&testCase.Response)
				assert.NoError(t, err)
			}))
			defer ts.Close()
			config.Get().ContentSourcesURL = ts.URL
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
