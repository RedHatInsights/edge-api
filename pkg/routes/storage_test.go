// FIXME: golangci-lint
// nolint:revive
package routes

import (
	"fmt"
	"io"
	url2 "net/url"

	"github.com/bxcodec/faker/v3"
	"github.com/go-chi/chi"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"

	// "github.com/redhatinsights/edge-api/pkg/routes"
	"net/http"
	"net/http/httptest"

	log "github.com/sirupsen/logrus"
)

var _ = Describe("Storage Router", func() {

	var ctrl *gomock.Controller
	var router chi.Router
	var mockFilesService *mock_services.MockFilesService
	var edgeAPIServices *dependencies.EdgeAPIServices

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockFilesService = mock_services.NewMockFilesService(ctrl)
		edgeAPIServices = &dependencies.EdgeAPIServices{
			FilesService: mockFilesService,
			Log:          log.NewEntry(log.StandardLogger()),
		}
		router = chi.NewRouter()
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := dependencies.ContextWithServices(r.Context(), edgeAPIServices)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		router.Route("/storage", MakeStorageRouter)
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Context("isos url", func() {
		orgID := common.DefaultOrgID
		installer := models.Installer{OrgID: orgID, ImageBuildISOURL: faker.URL()}
		db.DB.Create(&installer)
		installerWithNoURL := models.Installer{OrgID: orgID, ImageBuildISOURL: ""}
		db.DB.Create(&installerWithNoURL)
		installerWithBadURL := models.Installer{OrgID: orgID, ImageBuildISOURL: " + " + faker.URL()}
		db.DB.Create(&installerWithBadURL)

		It("User redirected to a signed url", func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("/storage/isos/%d", installer.ID), nil)
			Expect(err).ToNot(HaveOccurred())

			url, err := url2.Parse(installer.ImageBuildISOURL)
			Expect(err).ToNot(HaveOccurred())
			expectedURL := fmt.Sprintf("%s?signature", url)
			mockFilesService.EXPECT().GetSignedURL(url.Path).Return(expectedURL, nil)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			Expect(rr.Code).To(Equal(http.StatusSeeOther))
			Expect(rr.Header()["Location"][0]).To(Equal(expectedURL))
		})

		It("return Not found when installer does not exist", func() {
			req, err := http.NewRequest("GET", "/storage/isos/9999", nil)
			Expect(err).ToNot(HaveOccurred())
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			Expect(rr.Code).To(Equal(http.StatusNotFound))
			respBody, err := io.ReadAll(rr.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).To(ContainSubstring("installer not found"))
		})

		It("return Not found when iso url empty", func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("/storage/isos/%d", installerWithNoURL.ID), nil)
			Expect(err).ToNot(HaveOccurred())

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			Expect(rr.Code).To(Equal(http.StatusNotFound))
			respBody, err := io.ReadAll(rr.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).To(ContainSubstring("empty installer iso url"))
		})

		It("return Bad Request when iso url has a bad format", func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("/storage/isos/%d", installerWithBadURL.ID), nil)
			Expect(err).ToNot(HaveOccurred())

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			Expect(rr.Code).To(Equal(http.StatusBadRequest))
			respBody, err := io.ReadAll(rr.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).To(ContainSubstring("bad installer iso url"))
		})

		It("return Bad Request when fail to convert installerID to int", func() {
			req, err := http.NewRequest("GET", "/storage/isos/hah", nil)
			Expect(err).ToNot(HaveOccurred())

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			Expect(rr.Code).To(Equal(http.StatusBadRequest))
			respBody, err := io.ReadAll(rr.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).To(ContainSubstring("installer id must be an integer"))
		})
	})
})
