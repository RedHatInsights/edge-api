// FIXME: golangci-lint
package routes // nolint:revive

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	url2 "net/url"
	"strings"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"

	"github.com/bxcodec/faker/v3"
	"github.com/go-chi/chi"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { // nolint:revive
				ctx := dependencies.ContextWithServices(r.Context(), edgeAPIServices) // nolint:revive
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
		installer := models.Installer{OrgID: orgID, ImageBuildISOURL: faker.URL()} // nolint:revive
		db.DB.Create(&installer)
		installerWithNoURL := models.Installer{OrgID: orgID, ImageBuildISOURL: ""} // nolint:revive
		db.DB.Create(&installerWithNoURL)
		installerWithBadURL := models.Installer{OrgID: orgID, ImageBuildISOURL: " + " + faker.URL()} // nolint:revive
		db.DB.Create(&installerWithBadURL)

		It("User redirected to a signed url", func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("/storage/isos/%d", installer.ID), nil) // nolint:revive
			Expect(err).ToNot(HaveOccurred())

			url, err := url2.Parse(installer.ImageBuildISOURL)
			Expect(err).ToNot(HaveOccurred())
			expectedURL := fmt.Sprintf("%s?signature", url)
			mockFilesService.EXPECT().GetSignedURL(url.Path).Return(expectedURL, nil) // nolint:revive

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
			req, err := http.NewRequest("GET", fmt.Sprintf("/storage/isos/%d", installerWithNoURL.ID), nil) // nolint:revive
			Expect(err).ToNot(HaveOccurred())

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			Expect(rr.Code).To(Equal(http.StatusNotFound))
			respBody, err := io.ReadAll(rr.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).To(ContainSubstring("empty installer iso url")) // nolint:revive
		})

		It("return Bad Request when iso url has a bad format", func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("/storage/isos/%d", installerWithBadURL.ID), nil) // nolint:revive
			Expect(err).ToNot(HaveOccurred())

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			Expect(rr.Code).To(Equal(http.StatusBadRequest))
			respBody, err := io.ReadAll(rr.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).To(ContainSubstring("bad installer iso url")) // nolint:revive
		})

		It("return Bad Request when fail to convert installerID to int", func() { // nolint:revive
			req, err := http.NewRequest("GET", "/storage/isos/hah", nil)
			Expect(err).ToNot(HaveOccurred())

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			Expect(rr.Code).To(Equal(http.StatusBadRequest))
			respBody, err := io.ReadAll(rr.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).To(ContainSubstring("installer id must be an integer")) // nolint:revive
		})
	})
	Context("update transaction repository storage content", func() {
		deviceUUID := faker.UUIDHyphenated()
		orgID := common.DefaultOrgID

		device := models.Device{OrgID: orgID, UUID: deviceUUID}
		db.DB.Create(&device)
		updateTransaction := models.UpdateTransaction{
			OrgID: orgID,
			Repo:  &models.Repo{URL: "https://repo-storage.org/path/to/bucket", Status: models.ImageStatusSuccess}, // nolint:revive
		}
		db.DB.Create(&updateTransaction)
		_ = db.DB.Model(&updateTransaction).Association("Devices").Append([]models.Device{device}) // nolint:errcheck,revive

		Context("GetUpdateTransactionRepoFile", func() {
			It("Should return the requested resource content", func() {
				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/%s", updateTransaction.ID, targetRepoFile), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				fileContent := "this is a simple file content"

				url, err := url2.Parse(updateTransaction.Repo.URL)
				Expect(err).ToNot(HaveOccurred())
				targetPath := fmt.Sprintf("%s/%s", url.Path, targetRepoFile)

				fileContentReader := strings.NewReader(fileContent)
				fileContentReadCloser := io.NopCloser(fileContentReader)
				mockFilesService.EXPECT().GetFile(targetPath).Return(fileContentReadCloser, nil) // nolint:revive

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(Equal(fileContent))
			})

			It("should return error when the update transaction does not exists", func() { // nolint:revive

				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/summary.sig", 9999), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("device update transaction not found")) // nolint:revive
			})

			It("should return error when requested transaction id is not a number", func() { // nolint:revive
				req, err := http.NewRequest("GET", "/storage/update-repos/not-a-number/summary.sig", nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction id must be an integer")) // nolint:revive
			})

			It("should return error when requested transaction id is empty", func() { // nolint:revive
				req, err := http.NewRequest("GET", "/storage/update-repos//summary.sig", nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction ID required")) // nolint:revive
			})

			It("should return error when target file path is missing", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/", updateTransaction.ID), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("target repository file path is missing")) // nolint:revive
			})

			It("Should return error when update transaction not found", func() {
				targetRepoFile := "summary.sig"
				updateTransactionID := uint(9999)
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/%s", updateTransactionID, targetRepoFile), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("device update transaction not found")) // nolint:revive
			})

			It("should return error when update transaction has empty repo", func() { // nolint:revive
				updateTransaction := models.UpdateTransaction{ // nolint:govet
					OrgID: orgID,
					Repo:  &models.Repo{URL: ""},
				}
				db.DB.Create(&updateTransaction)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/%s", updateTransaction.ID, targetRepoFile), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction repository does not exist")) // nolint:revive
			})

			It("should return error when update transaction is without repo", func() { // nolint:revive
				updateTransaction := models.UpdateTransaction{ // nolint:govet
					OrgID: orgID,
					Repo:  nil,
				}
				db.DB.Create(&updateTransaction)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/%s", updateTransaction.ID, targetRepoFile), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction repository does not exist")) // nolint:revive
			})

			It("should return error when update transaction repo has an un-parseable url", func() { // nolint:revive
				updateTransaction := models.UpdateTransaction{ // nolint:govet
					OrgID: orgID,
					Repo:  &models.Repo{URL: "https:\t//repo-storage.org\n/path/to/bucket", Status: models.ImageStatusSuccess}, // nolint:revive
				}
				db.DB.Create(&updateTransaction)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/%s", updateTransaction.ID, targetRepoFile), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("bad update transaction repository url")) // nolint:revive
			})
		})

		Context("GetUpdateTransactionRepoFileContent", func() {
			It("Should redirect to the requested resource content file", func() { // nolint:revive
				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/%s", updateTransaction.ID, targetRepoFile), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				url, err := url2.Parse(updateTransaction.Repo.URL)
				Expect(err).ToNot(HaveOccurred())
				targetPath := fmt.Sprintf("%s/%s", url.Path, targetRepoFile)
				expectedURL := fmt.Sprintf("%s/%s?signature", url, targetRepoFile)          // nolint:revive
				mockFilesService.EXPECT().GetSignedURL(targetPath).Return(expectedURL, nil) // nolint:revive

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusSeeOther))
				Expect(httpTestRecorder.Header()["Location"][0]).To(Equal(expectedURL)) // nolint:revive
			})

			It("should return error when the update transaction does not exists", func() { // nolint:revive

				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/summary.sig", 9999), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("device update transaction not found")) // nolint:revive
			})

			It("should return error when requested transaction id is not a number", func() { // nolint:revive
				req, err := http.NewRequest("GET", "/storage/update-repos/not-a-number/content/summary.sig", nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction id must be an integer")) // nolint:revive
			})

			It("should return error when requested transaction id is empty", func() { // nolint:revive
				req, err := http.NewRequest("GET", "/storage/update-repos//content/summary.sig", nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction ID required")) // nolint:revive
			})

			It("should return error when target file path is missing", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/", updateTransaction.ID), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("target repository file path is missing")) // nolint:revive
			})

			It("Should return error when update transaction not found", func() {
				targetRepoFile := "summary.sig"
				updateTransactionID := uint(9999)
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/%s", updateTransactionID, targetRepoFile), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("device update transaction not found")) // nolint:revive
			})

			It("should return error when update transaction has empty repo", func() { // nolint:revive
				updateTransaction := models.UpdateTransaction{ // nolint:govet
					OrgID: orgID,
					Repo:  &models.Repo{URL: ""},
				}
				db.DB.Create(&updateTransaction)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/%s", updateTransaction.ID, targetRepoFile), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction repository does not exist")) // nolint:revive
			})

			It("should return error when update transaction is without repo", func() { // nolint:revive
				updateTransaction := models.UpdateTransaction{ // nolint:govet
					OrgID: orgID,
					Repo:  nil,
				}
				db.DB.Create(&updateTransaction)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/%s", updateTransaction.ID, targetRepoFile), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction repository does not exist")) // nolint:revive
			})

			It("should return error when update transaction repo has an un-parseable url", func() { // nolint:revive
				updateTransaction := models.UpdateTransaction{ // nolint:govet
					OrgID: orgID,
					Repo:  &models.Repo{URL: "https:\t//repo-storage.org\n/path/to/bucket", Status: models.ImageStatusSuccess}, // nolint:revive
				}
				db.DB.Create(&updateTransaction)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/%s", updateTransaction.ID, targetRepoFile), nil) // nolint:revive
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("bad update transaction repository url")) // nolint:revive
			})
		})
	})
})
