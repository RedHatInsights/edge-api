// FIXME: golangci-lint
// nolint:revive,typecheck
package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/repositories"
	"github.com/redhatinsights/edge-api/pkg/clients/repositories/mock_repositories"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	"github.com/redhatinsights/platform-go-middlewares/identity"

	"github.com/bxcodec/faker/v3"
	"github.com/go-chi/chi"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
)

func TestCreateWasCalledWithURLNotSet(t *testing.T) {
	config.Get().Debug = false
	jsonRepo := &models.ThirdPartyRepo{
		Description: "This is Third Party repository",
		Name:        "Repository1",
	}
	jsonRepoBytes, err := json.Marshal(jsonRepo)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(jsonRepoBytes))
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		Log: log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateThirdPartyRepo)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
	config.Get().Debug = true
}

func TestCreateThirdPartyRepo(t *testing.T) {
	jsonRepo := &models.ThirdPartyRepo{
		URL:         "http://www.thirdpartyurl.com/in/thisrepo",
		Description: "This is Third Party repository",
		Name:        "Repository1",
	}
	jsonRepoBytes, err := json.Marshal(jsonRepo)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(jsonRepoBytes))
	if err != nil {
		t.Fatal(err)
	}
	ctx := req.Context()
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockThirdPartyRepoService := mock_services.NewMockThirdPartyRepoServiceInterface(ctrl)
	mockThirdPartyRepoService.EXPECT().CreateThirdPartyRepo(gomock.Any(), gomock.Any()).Return(&tprepo, nil)
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		ThirdPartyRepoService: mockThirdPartyRepoService,
		Log:                   log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateThirdPartyRepo)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)

	}

}
func TestGetAllThirdPartyRepo(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetAllThirdPartyRepo)
	ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
		Log: log.NewEntry(log.StandardLogger()),
	})
	req = req.WithContext(ctx)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)

	}
}

func TestGetAllThirdPartyRepoQueryParams(t *testing.T) {
	tt := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
		{
			name:   "invalid query param",
			params: "bla=1",
			expectedError: []validationError{
				{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: %s", GetQueryParamsArray("thirdpartyrepo"))},
			},
		},
		{
			name:   "valid query param and invalid query param",
			params: "sort_by=created_at&bla=1",
			expectedError: []validationError{
				{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: %s", GetQueryParamsArray("thirdpartyrepo"))},
			},
		},
		{
			name:   "invalid query param and valid query param",
			params: "bla=1&sort_by=created_at",
			expectedError: []validationError{
				{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: %s", GetQueryParamsArray("thirdpartyrepo"))},
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	for _, te := range tt {
		req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockThirdPartyRepoService := mock_services.NewMockThirdPartyRepoServiceInterface(ctrl)
		ctx := req.Context()
		ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
			ThirdPartyRepoService: mockThirdPartyRepoService,
			Log:                   log.NewEntry(log.StandardLogger()),
		})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		ValidateQueryParams("thirdpartyrepo")(next).ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		jsonBody := []validationError{}
		err = json.NewDecoder(resp.Body).Decode(&jsonBody)
		if err != nil {
			t.Errorf("failed decoding response body: %s", err.Error())
		}
		for _, exErr := range te.expectedError {
			found := false
			for _, jsErr := range jsonBody {
				if jsErr.Key == exErr.Key && jsErr.Reason == exErr.Reason {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("in %q: was expected to have %v but not found in %v", te.name, exErr, jsonBody)
			}
		}
	}
}

func TestGetAllThirdPartyRepoFilterParams(t *testing.T) {
	tt := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
		{
			name:   "bad created_at date",
			params: "created_at=today",
			expectedError: []validationError{
				{Key: "created_at", Reason: `parsing time "today" as "2006-01-02": cannot parse "today" as "2006"`},
			},
		},
		{
			name:   "bad sort_by",
			params: "sort_by=test",
			expectedError: []validationError{
				{Key: "sort_by", Reason: "test is not a valid sort_by. Sort-by must be name or created_at or updated_at"},
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	for _, te := range tt {
		req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockThirdPartyRepoService := mock_services.NewMockThirdPartyRepoServiceInterface(ctrl)
		ctx := req.Context()
		ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
			ThirdPartyRepoService: mockThirdPartyRepoService,
			Log:                   log.NewEntry(log.StandardLogger()),
		})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		validateGetAllThirdPartyRepoFilterParams(next).ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		jsonBody := []validationError{}
		err = json.NewDecoder(resp.Body).Decode(&jsonBody)
		if err != nil {
			t.Errorf("failed decoding response body: %s", err.Error())
		}
		for _, exErr := range te.expectedError {
			found := false
			for _, jsErr := range jsonBody {
				if jsErr.Key == exErr.Key && jsErr.Reason == exErr.Reason {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("in %q: was expected to have %v but not found in %v", te.name, exErr, jsonBody)
			}
		}
	}
}

var _ = Describe("ThirdPartyRepos basic routes", func() {
	var ctrl *gomock.Controller
	var router chi.Router
	var mockThirdPartyRepoService *mock_services.MockThirdPartyRepoServiceInterface
	var edgeAPIServices *dependencies.EdgeAPIServices

	Context("CheckThirdPartyRepoName", func() {
		orgID := common.DefaultOrgID
		type ResponseData struct {
			IsValid bool `json:"isValid"`
		}
		type Response struct {
			Data ResponseData `json:"data"`
		}

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockThirdPartyRepoService = mock_services.NewMockThirdPartyRepoServiceInterface(ctrl)
			edgeAPIServices = &dependencies.EdgeAPIServices{
				ThirdPartyRepoService: mockThirdPartyRepoService,
				Log:                   log.NewEntry(log.StandardLogger()),
			}
			router = chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := dependencies.ContextWithServices(r.Context(), edgeAPIServices)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			router.Route("/thirdpartyrepo", MakeThirdPartyRepoRouter)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return isValid true when third party repository exists", func() {
			repoName := faker.UUIDHyphenated()
			req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo/checkName/%s", repoName), nil)
			Expect(err).ToNot(HaveOccurred())

			mockThirdPartyRepoService.EXPECT().ThirdPartyRepoNameExists(orgID, repoName).Return(true, nil)
			httpTestRecorder := httptest.NewRecorder()
			router.ServeHTTP(httpTestRecorder, req)

			Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
			respBody, err := io.ReadAll(httpTestRecorder.Body)
			Expect(err).ToNot(HaveOccurred())
			var response Response
			err = json.Unmarshal(respBody, &response)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.Data.IsValid).To(BeTrue())
		})

		It("should return isValid false when third party repository does not exist", func() {
			repoName := faker.UUIDHyphenated()
			req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo/checkName/%s", repoName), nil)
			Expect(err).ToNot(HaveOccurred())

			mockThirdPartyRepoService.EXPECT().ThirdPartyRepoNameExists(orgID, repoName).Return(false, nil)
			httpTestRecorder := httptest.NewRecorder()
			router.ServeHTTP(httpTestRecorder, req)

			Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
			respBody, err := io.ReadAll(httpTestRecorder.Body)
			Expect(err).ToNot(HaveOccurred())

			var response Response
			err = json.Unmarshal(respBody, &response)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.Data.IsValid).To(BeFalse())
		})

		It("should return error when third party repository name is empty", func() {
			// now the router will return 404 when repo name is empty (this happens early at router level),
			// but we need to test our handler that it handles the case well
			// (this increases handler coverage, and cover the case of router's behavior change where empty name is passed to our handler)
			repoName := faker.UUIDHyphenated()
			req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo/checkName/%s", repoName), nil)
			Expect(err).ToNot(HaveOccurred())

			expectedError := new(services.ThirdPartyRepositoryNameIsEmpty)

			mockThirdPartyRepoService.EXPECT().ThirdPartyRepoNameExists(orgID, repoName).Return(false, expectedError)
			httpTestRecorder := httptest.NewRecorder()
			router.ServeHTTP(httpTestRecorder, req)

			Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
			respBody, err := io.ReadAll(httpTestRecorder.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).To(ContainSubstring(expectedError.Error()))
		})
	})

	Context("when orgID cannot be defined", func() {
		var originalAuth bool
		conf := config.Get()

		BeforeEach(func() {
			// save original config auth value
			originalAuth = conf.Auth
			// set auth to True to force use identity
			conf.Auth = true
			ctrl = gomock.NewController(GinkgoT())
			edgeAPIServices = &dependencies.EdgeAPIServices{
				ThirdPartyRepoService: mock_services.NewMockThirdPartyRepoServiceInterface(ctrl),
				Log:                   log.NewEntry(log.StandardLogger()),
			}
			router = chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// set identity with empty orgID, this should provoke to return an error from GetOrgIDFromContext
					ctx := context.WithValue(r.Context(), identity.Key, identity.XRHID{Identity: identity.Identity{OrgID: ""}})
					ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			router.Route("/thirdpartyrepo", MakeThirdPartyRepoRouter)
		})

		AfterEach(func() {
			// restore config auth value
			conf.Auth = originalAuth
			ctrl.Finish()
		})

		It("returns error when orgID is not defined", func() {
			repoName := faker.UUIDHyphenated()
			req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo/checkName/%s", repoName), nil)
			Expect(err).ToNot(HaveOccurred())

			httpTestRecorder := httptest.NewRecorder()
			router.ServeHTTP(httpTestRecorder, req)

			Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
			respBody, err := io.ReadAll(httpTestRecorder.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).To(ContainSubstring("cannot find org-id"))
		})
	})

	Context("#GetAllContentSourcesRepositories", func() {
		var orgID string
		var image models.Image
		var mockRepositoriesService *mock_repositories.MockClientInterface
		var mockImageService *mock_services.MockImageServiceInterface
		var ctrl *gomock.Controller
		var listRepositoryResponse repositories.ListRepositoriesResponse
		var thirdPartyRepo models.ThirdPartyRepo

		BeforeEach(func() {
			// set content source feature enabled
			err := os.Setenv("FEATURE_CONTENT_SOURCES", "enabled")
			Expect(err).ToNot(HaveOccurred())
			orgID = common.DefaultOrgID

			thirdPartyRepo = models.ThirdPartyRepo{
				Name:  faker.Name(),
				URL:   faker.URL(),
				UUID:  faker.UUIDHyphenated(),
				OrgID: orgID,
			}
			err = db.DB.Create(&thirdPartyRepo).Error
			Expect(err).ToNot(HaveOccurred())

			// an image that has a third party repo
			image = models.Image{
				OrgID:                  orgID,
				Name:                   faker.Name(),
				ThirdPartyRepositories: []models.ThirdPartyRepo{thirdPartyRepo},
			}
			err = db.DB.Create(&image).Error
			Expect(err).ToNot(HaveOccurred())
			ctrl = gomock.NewController(GinkgoT())
			mockRepositoriesService = mock_repositories.NewMockClientInterface(ctrl)
			mockImageService = mock_services.NewMockImageServiceInterface(ctrl)

			listRepositoryResponse = repositories.ListRepositoriesResponse{
				Data: []repositories.Repository{
					{Name: faker.Name(), URL: faker.URL(), UUID: uuid.MustParse(faker.UUIDHyphenated())},
					{Name: thirdPartyRepo.Name, URL: thirdPartyRepo.URL, UUID: uuid.MustParse(thirdPartyRepo.UUID)},
					{Name: faker.Name(), URL: faker.URL(), UUID: uuid.MustParse(faker.UUIDHyphenated())},
				},
				Meta: repositories.ListRepositoriesMeta{
					Count: 3,
				},
			}

			router = chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := dependencies.ContextWithServices(r.Context(), &dependencies.EdgeAPIServices{
						ImageService:        mockImageService,
						RepositoriesService: mockRepositoriesService,
						Log:                 log.NewEntry(log.StandardLogger()),
					})
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			router.Route("/thirdpartyrepo", MakeThirdPartyRepoRouter)
		})

		AfterEach(func() {
			ctrl.Finish()
			err := os.Unsetenv("FEATURE_CONTENT_SOURCES")
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return content-source repositories successfully", func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo?imageID=%d&sort_by=-name&limit=20&offset=0", image.ID), nil)
			Expect(err).ToNot(HaveOccurred())

			mockRepositoriesService.EXPECT().ListRepositories(
				repositories.ListRepositoriesParams{Limit: 20, Offset: 0, SortBy: "name", SortType: "desc"},
				repositories.ListRepositoriesFilters{},
			).Return(&listRepositoryResponse, nil)

			mockImageService.EXPECT().GetImageByIDExtended(image.ID, gomock.Any()).Return(&image, nil)

			httpTestRecorder := httptest.NewRecorder()
			router.ServeHTTP(httpTestRecorder, req)

			Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
			respBody, err := io.ReadAll(httpTestRecorder.Body)
			Expect(err).ToNot(HaveOccurred())

			type Response struct {
				Data  []models.ThirdPartyRepo `json:"data"`
				Count int                     `json:"count"`
			}
			var response Response
			err = json.Unmarshal(respBody, &response)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.Count).To(Equal(listRepositoryResponse.Meta.Count))
			Expect(len(response.Data)).To(Equal(len(listRepositoryResponse.Data)))

			// Check that all the remote repos has been returned
			var found bool
			var responseImageRepo *models.ThirdPartyRepo
			for _, CSRepo := range listRepositoryResponse.Data {
				found = false
				for _, EMRepo := range response.Data {
					// create a new variable to escape any memory aliasing
					EMRepo := EMRepo
					if CSRepo.UUID.String() == thirdPartyRepo.UUID {
						responseImageRepo = &EMRepo
					}
					if CSRepo.UUID.String() == EMRepo.UUID {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue())
			}
			// Check that the existing in local db repo is returned with corresponding image
			Expect(responseImageRepo).ToNot(BeNil())
			Expect(responseImageRepo.Name).To(Equal(thirdPartyRepo.Name))
			Expect(responseImageRepo.URL).To(Equal(thirdPartyRepo.URL))
			Expect(responseImageRepo.UUID).To(Equal(thirdPartyRepo.UUID))
			Expect(len(responseImageRepo.Images)).To(Equal(1))
			Expect(responseImageRepo.Images[0].ID).To(Equal(image.ID))
		})

		It("should filter by name", func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo?name=%s&sort_by=name&limit=20", thirdPartyRepo.Name), nil)
			Expect(err).ToNot(HaveOccurred())

			mockRepositoriesService.EXPECT().ListRepositories(
				repositories.ListRepositoriesParams{Limit: 20, Offset: 0, SortBy: "name", SortType: "asc"},
				repositories.ListRepositoriesFilters{"name": thirdPartyRepo.Name},
			).Return(
				&repositories.ListRepositoriesResponse{
					Data: []repositories.Repository{listRepositoryResponse.Data[1]},
					Meta: repositories.ListRepositoriesMeta{Count: 1},
				},
				nil,
			)

			httpTestRecorder := httptest.NewRecorder()
			router.ServeHTTP(httpTestRecorder, req)

			Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
			respBody, err := io.ReadAll(httpTestRecorder.Body)
			Expect(err).ToNot(HaveOccurred())

			type Response struct {
				Data  []models.ThirdPartyRepo `json:"data"`
				Count int                     `json:"count"`
			}
			var response Response
			err = json.Unmarshal(respBody, &response)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.Count).To(Equal(1))
			Expect(len(response.Data)).To(Equal(1))
			Expect(response.Data[0].UUID).To(Equal(listRepositoryResponse.Data[1].UUID.String()))
		})

		It("should return error when ListRepositories return error", func() {
			// ListRepositories should never return error , any error occurred is an internal server error
			req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo?imageID=%d&sort_by=-name&limit=20&offset=0", image.ID), nil)
			Expect(err).ToNot(HaveOccurred())

			expectedError := errors.New("expected ListRepository error")

			mockRepositoriesService.EXPECT().ListRepositories(
				repositories.ListRepositoriesParams{Limit: 20, Offset: 0, SortBy: "name", SortType: "desc"},
				repositories.ListRepositoriesFilters{},
			).Return(nil, expectedError)

			httpTestRecorder := httptest.NewRecorder()
			router.ServeHTTP(httpTestRecorder, req)

			Expect(httpTestRecorder.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should return error when passing a bad imageID param value", func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo?imageID=%s&sort_by=name&limit=20&offset=0", "bad_image_id"), nil)
			Expect(err).ToNot(HaveOccurred())

			mockRepositoriesService.EXPECT().ListRepositories(
				repositories.ListRepositoriesParams{Limit: 20, Offset: 0, SortBy: "name", SortType: "asc"},
				repositories.ListRepositoriesFilters{},
			).Return(&listRepositoryResponse, nil)

			httpTestRecorder := httptest.NewRecorder()
			router.ServeHTTP(httpTestRecorder, req)

			Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
			respBody, err := io.ReadAll(httpTestRecorder.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).To(ContainSubstring("image_id must be of type integer"))
		})

		It("should return NotFound error when GetImageByIDExtended return image not found error", func() {
			requestedImageID := uint(9999999)
			req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo?imageID=%d&sort_by=name&limit=20&offset=0", requestedImageID), nil)
			Expect(err).ToNot(HaveOccurred())

			mockRepositoriesService.EXPECT().ListRepositories(
				repositories.ListRepositoriesParams{Limit: 20, Offset: 0, SortBy: "name", SortType: "asc"},
				repositories.ListRepositoriesFilters{},
			).Return(&listRepositoryResponse, nil)

			expectedError := new(services.ImageNotFoundError)
			mockImageService.EXPECT().GetImageByIDExtended(requestedImageID, gomock.Any()).Return(nil, expectedError)

			httpTestRecorder := httptest.NewRecorder()
			router.ServeHTTP(httpTestRecorder, req)

			Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
			respBody, err := io.ReadAll(httpTestRecorder.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).To(ContainSubstring("requested image not found"))
		})

		It("should return NewInternalServerError when GetImageByIDExtended return unknown error", func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("/thirdpartyrepo?imageID=%d&sort_by=name&limit=20&offset=0", image.ID), nil)
			Expect(err).ToNot(HaveOccurred())

			mockRepositoriesService.EXPECT().ListRepositories(
				repositories.ListRepositoriesParams{Limit: 20, Offset: 0, SortBy: "name", SortType: "asc"},
				repositories.ListRepositoriesFilters{},
			).Return(&listRepositoryResponse, nil)

			expectedError := errors.New("expected GetImageByIDExtended errors, not known to routes function ")
			mockImageService.EXPECT().GetImageByIDExtended(image.ID, gomock.Any()).Return(nil, expectedError)

			httpTestRecorder := httptest.NewRecorder()
			router.ServeHTTP(httpTestRecorder, req)
			Expect(httpTestRecorder.Code).To(Equal(http.StatusInternalServerError))
		})

		Context("org id is undefined", func() {
			var originalAuth bool
			conf := config.Get()

			BeforeEach(func() {
				// save original config auth value
				originalAuth = conf.Auth
				// set auth to True to force use identity
				conf.Auth = true

				router = chi.NewRouter()
				router.Use(func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// set identity with empty orgID, this should provoke to return an error from GetOrgIDFromContext
						ctx := context.WithValue(r.Context(), identity.Key, identity.XRHID{Identity: identity.Identity{OrgID: ""}})
						ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
							ImageService:        mockImageService,
							RepositoriesService: mockRepositoriesService,
							Log:                 log.NewEntry(log.StandardLogger()),
						})
						next.ServeHTTP(w, r.WithContext(ctx))
					})
				})
				router.Route("/thirdpartyrepo", MakeThirdPartyRepoRouter)
			})

			AfterEach(func() {
				// restore config auth value
				conf.Auth = originalAuth
			})

			It("should return error when org_id is undefined ", func() {
				req, err := http.NewRequest("GET", "/thirdpartyrepo", nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("cannot find org-id"))
			})
		})
	})
})
