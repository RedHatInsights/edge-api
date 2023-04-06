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

	"github.com/golang/mock/gomock"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients"
	"github.com/redhatinsights/edge-api/pkg/clients/repositories"
	"github.com/redhatinsights/edge-api/pkg/clients/repositories/mock_repositories"

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

// ErrFaultyWriter the error returned by FaultWriter's Write function
var ErrFaultyWriter = errors.New("faulty writer expected write error")

// FaultyWriter a struct that implement a Writer that return always error when writing
type FaultyWriter struct{}

func (f *FaultyWriter) Write(_ []byte) (n int, err error) {
	return 0, ErrFaultyWriter
}

func TestCreateRepository(t *testing.T) {
	initialContentSourceURL := config.Get().ContentSourcesURL
	initialAPIRepositoriesPath := repositories.APIRepositoriesPath
	// restore the initial content sources url and apiRepositoriesPath
	defer func(contentSourcesURL, reposPath string) {
		config.Get().ContentSourcesURL = contentSourcesURL
		repositories.APIRepositoriesPath = reposPath
	}(initialContentSourceURL, initialAPIRepositoriesPath)

	repoToCreate := repositories.Repository{Name: faker.UUIDHyphenated(), URL: faker.URL()}
	responseRepo := repositories.Repository{UUID: uuid.New(), Name: repoToCreate.Name, URL: repoToCreate.URL}

	testCases := []struct {
		Name                string
		ContentSourcesURL   string
		APIRepositoriesPath string
		RepoToCreate        repositories.Repository
		IOReadAll           func(r io.Reader) ([]byte, error)
		NewJSONEncoder      func(w io.Writer) *json.Encoder
		HTTPStatus          int
		Response            *repositories.Repository
		ResponseText        string
		ExpectedError       error
	}{
		{
			Name:           "should create repository successfully",
			RepoToCreate:   repoToCreate,
			HTTPStatus:     http.StatusCreated,
			IOReadAll:      io.ReadAll,
			NewJSONEncoder: json.NewEncoder,
			Response:       &responseRepo,
			ExpectedError:  nil,
		},
		{
			Name:           "should return error when http status is not 201",
			RepoToCreate:   repoToCreate,
			HTTPStatus:     http.StatusBadRequest,
			IOReadAll:      io.ReadAll,
			NewJSONEncoder: json.NewEncoder,
			ExpectedError:  repositories.ErrRepositoryRequestResponse,
		},
		{
			Name:              "should return error when parsing base url fails",
			RepoToCreate:      repoToCreate,
			ContentSourcesURL: "\t",
			IOReadAll:         io.ReadAll,
			NewJSONEncoder:    json.NewEncoder,
			HTTPStatus:        http.StatusCreated,
			ExpectedError:     repositories.ErrParsingRawURL,
		},
		{
			Name:                "should return error when parsing apiRepositoriesPath fails",
			RepoToCreate:        repoToCreate,
			APIRepositoriesPath: "\t",
			IOReadAll:           io.ReadAll,
			NewJSONEncoder:      json.NewEncoder,
			HTTPStatus:          http.StatusCreated,
			ExpectedError:       repositories.ErrParsingRawURL,
		},
		{
			Name:              "should return error when client Do fail",
			RepoToCreate:      repoToCreate,
			ContentSourcesURL: "host-without-schema",
			IOReadAll:         io.ReadAll,
			NewJSONEncoder:    json.NewEncoder,
			HTTPStatus:        http.StatusCreated,
			ExpectedError:     errors.New("unsupported protocol scheme"),
		},
		{
			Name:           "should return error when malformed json is returned",
			RepoToCreate:   repoToCreate,
			HTTPStatus:     http.StatusCreated,
			IOReadAll:      io.ReadAll,
			NewJSONEncoder: json.NewEncoder,
			ResponseText:   `{"data: {}}`,
			ExpectedError:  errors.New("unexpected end of JSON input"),
		},
		{
			Name:         "should return error when body readAll fails",
			RepoToCreate: repoToCreate,
			HTTPStatus:   http.StatusCreated,
			IOReadAll: func(r io.Reader) ([]byte, error) {
				return nil, errors.New("expected error for when reading response body fails")
			},
			NewJSONEncoder: json.NewEncoder,
			Response:       nil,
			ExpectedError:  errors.New("expected error for when reading response body fails"),
		},
		{
			Name:         "should return error when repository json encode fails",
			RepoToCreate: repoToCreate,
			IOReadAll:    io.ReadAll,
			NewJSONEncoder: func(_ io.Writer) *json.Encoder {
				// ignore any argument a return the FaultyWriter, that returns error when calling its Write function
				return json.NewEncoder(&FaultyWriter{})
			},
			ExpectedError: ErrFaultyWriter,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			initialAPIRepositoriesPath := repositories.APIRepositoriesPath
			// restore the initial apiRepositoriesPath and IOReadAll
			defer func(reposPath string) {
				repositories.APIRepositoriesPath = reposPath
				repositories.IOReadAll = io.ReadAll
				repositories.NewJSONEncoder = json.NewEncoder
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
			repositories.NewJSONEncoder = testCase.NewJSONEncoder

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

			response, err := client.CreateRepository(testCase.RepoToCreate)
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

func TestSearchContentPackage(t *testing.T) {

	rp := []repositories.SearchPackageResponse{{PackageName: "cat", Summary: "cat test"}}

	testCases := []struct {
		Name                string
		ContentSourcesURL   string
		PackageName         string
		APIRepositoriesPath string
		URLS                []string
		IOReadAll           func(r io.Reader) ([]byte, error)
		HTTPStatus          int
		Response            *[]repositories.SearchPackageResponse
		ResponseText        string
		ExpectedError       error
	}{
		{
			Name:          "should return the expected repos",
			URLS:          []string{"https://test.com"},
			PackageName:   "cat",
			HTTPStatus:    http.StatusInternalServerError,
			IOReadAll:     io.ReadAll,
			Response:      &rp,
			ExpectedError: nil,
		},
		{
			Name:          "should return error",
			URLS:          []string{"https://google.com"},
			PackageName:   "",
			HTTPStatus:    http.StatusInternalServerError,
			IOReadAll:     io.ReadAll,
			Response:      &[]repositories.SearchPackageResponse{},
			ExpectedError: &repositories.PackageRequestError{},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepositoriesService := mock_repositories.NewMockClientInterface(ctrl)

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
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

			mockRepositoriesService.EXPECT().SearchContentPackage(gomock.Any(), gomock.Any()).
				Return(&rp, testCase.ExpectedError)

			response, err := mockRepositoriesService.SearchContentPackage(testCase.PackageName, testCase.URLS)

			if testCase.ExpectedError == nil {
				assert.NoError(t, err)
				assert.Equal(t, response, testCase.Response)
			} else {
				assert.ErrorContains(t, err, testCase.ExpectedError.Error())
			}
		})
	}
}
