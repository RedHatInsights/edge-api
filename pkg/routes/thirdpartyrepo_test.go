// FIXME: golangci-lint
// nolint:revive,typecheck
package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	"github.com/redhatinsights/platform-go-middlewares/identity"

	"github.com/bxcodec/faker/v3"
	"github.com/go-chi/chi"
	"github.com/golang/mock/gomock"
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
})
