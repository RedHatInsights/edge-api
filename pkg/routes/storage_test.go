// FIXME: golangci-lint
// nolint:errcheck,govet,revive,typecheck
package routes

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	url2 "net/url"
	"strings"

	testHelpers "github.com/redhatinsights/edge-api/internal/testing"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"

	"github.com/redhatinsights/edge-api/config"

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
	Context("update transaction repository storage content", func() {
		deviceUUID := faker.UUIDHyphenated()
		orgID := common.DefaultOrgID

		device := models.Device{OrgID: orgID, UUID: deviceUUID}
		db.DB.Create(&device)
		updateTransaction := models.UpdateTransaction{
			OrgID: orgID,
			Repo:  &models.Repo{URL: "https://repo-storage.org/path/to/bucket", Status: models.ImageStatusSuccess},
		}
		db.DB.Create(&updateTransaction)
		_ = db.DB.Model(&updateTransaction).Association("Devices").Append([]models.Device{device})

		Context("GetUpdateTransactionRepoFile", func() {
			It("Should return the requested resource content", func() {
				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/%s", updateTransaction.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				fileContent := "this is a simple file content"

				url, err := url2.Parse(updateTransaction.Repo.URL)
				Expect(err).ToNot(HaveOccurred())
				targetPath := fmt.Sprintf("%s/%s", url.Path, targetRepoFile)

				fileContentReader := strings.NewReader(fileContent)
				fileContentReadCloser := io.NopCloser(fileContentReader)
				mockFilesService.EXPECT().GetFile(targetPath).Return(fileContentReadCloser, nil)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(Equal(fileContent))
			})

			It("should return error when the update transaction does not exists", func() {

				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/summary.sig", 9999), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("device update transaction not found"))
			})

			It("should return error when requested transaction id is not a number", func() {
				req, err := http.NewRequest("GET", "/storage/update-repos/not-a-number/summary.sig", nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction id must be an integer"))
			})

			It("should return error when requested transaction id is empty", func() {
				req, err := http.NewRequest("GET", "/storage/update-repos//summary.sig", nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction ID required"))
			})

			It("should return error when target file path is missing", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/", updateTransaction.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("target repository file path is missing"))
			})

			It("Should return error when update transaction not found", func() {
				targetRepoFile := "summary.sig"
				updateTransactionID := uint(9999)
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/%s", updateTransactionID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("device update transaction not found"))
			})

			It("should return error when update transaction has empty repo", func() {
				updateTransaction := models.UpdateTransaction{
					OrgID: orgID,
					Repo:  &models.Repo{URL: ""},
				}
				db.DB.Create(&updateTransaction)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/%s", updateTransaction.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction repository does not exist"))
			})

			It("should return error when update transaction is without repo", func() {
				updateTransaction := models.UpdateTransaction{
					OrgID: orgID,
					Repo:  nil,
				}
				db.DB.Create(&updateTransaction)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/%s", updateTransaction.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction repository does not exist"))
			})

			It("should return error when update transaction repo has an un-parseable url", func() {
				updateTransaction := models.UpdateTransaction{
					OrgID: orgID,
					Repo:  &models.Repo{URL: "https:\t//repo-storage.org\n/path/to/bucket", Status: models.ImageStatusSuccess},
				}
				db.DB.Create(&updateTransaction)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/%s", updateTransaction.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("bad update transaction repository url"))
			})
		})

		Context("GetUpdateTransactionRepoFileContent", func() {
			It("Should redirect to the requested resource content file", func() {
				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/%s", updateTransaction.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				url, err := url2.Parse(updateTransaction.Repo.URL)
				Expect(err).ToNot(HaveOccurred())
				targetPath := fmt.Sprintf("%s/%s", url.Path, targetRepoFile)
				expectedURL := fmt.Sprintf("%s/%s?signature", url, targetRepoFile)
				mockFilesService.EXPECT().GetSignedURL(targetPath).Return(expectedURL, nil)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusSeeOther))
				Expect(httpTestRecorder.Header()["Location"][0]).To(Equal(expectedURL))
			})

			It("should return error when the update transaction does not exists", func() {

				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/summary.sig", 9999), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("device update transaction not found"))
			})

			It("should return error when requested transaction id is not a number", func() {
				req, err := http.NewRequest("GET", "/storage/update-repos/not-a-number/content/summary.sig", nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction id must be an integer"))
			})

			It("should return error when requested transaction id is empty", func() {
				req, err := http.NewRequest("GET", "/storage/update-repos//content/summary.sig", nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction ID required"))
			})

			It("should return error when target file path is missing", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/", updateTransaction.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("target repository file path is missing"))
			})

			It("Should return error when update transaction not found", func() {
				targetRepoFile := "summary.sig"
				updateTransactionID := uint(9999)
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/%s", updateTransactionID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("device update transaction not found"))
			})

			It("should return error when update transaction has empty repo", func() {
				updateTransaction := models.UpdateTransaction{
					OrgID: orgID,
					Repo:  &models.Repo{URL: ""},
				}
				db.DB.Create(&updateTransaction)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/%s", updateTransaction.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction repository does not exist"))
			})

			It("should return error when update transaction is without repo", func() {
				updateTransaction := models.UpdateTransaction{
					OrgID: orgID,
					Repo:  nil,
				}
				db.DB.Create(&updateTransaction)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/%s", updateTransaction.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update transaction repository does not exist"))
			})

			It("should return error when update transaction repo has an un-parseable url", func() {
				updateTransaction := models.UpdateTransaction{
					OrgID: orgID,
					Repo:  &models.Repo{URL: "https:\t//repo-storage.org\n/path/to/bucket", Status: models.ImageStatusSuccess},
				}
				db.DB.Create(&updateTransaction)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/update-repos/%d/content/%s", updateTransaction.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("bad update transaction repository url"))
			})
		})
	})
	Context("image repository storage content", func() {
		orgID := common.DefaultOrgID
		image := models.Image{
			OrgID: orgID,
			Name:  faker.UUIDHyphenated(),
			Commit: &models.Commit{
				OrgID: orgID,
				Repo: &models.Repo{
					URL:    "https://repo-storage.org/path/to/bucket",
					Status: models.ImageStatusSuccess,
				},
			},
		}
		result := db.DB.Create(&image)

		It("initial image created", func() {
			Expect(result.Error).ToNot(HaveOccurred())
		})

		Context("GetImageRepoFile", func() {
			It("Should return the requested resource content", func() {
				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/%s", image.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				fileContent := "this is a simple file content"

				url, err := url2.Parse(image.Commit.Repo.URL)
				Expect(err).ToNot(HaveOccurred())
				targetPath := fmt.Sprintf("%s/%s", url.Path, targetRepoFile)

				fileContentReader := strings.NewReader(fileContent)
				fileContentReadCloser := io.NopCloser(fileContentReader)
				mockFilesService.EXPECT().GetFile(targetPath).Return(fileContentReadCloser, nil)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(Equal(fileContent))
			})
			It("should return error when the image does not exists", func() {

				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/summary.sig", 9999), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("storage image not found"))
			})

			It("should return error when requested image id is not a number", func() {
				req, err := http.NewRequest("GET", "/storage/images-repos/not-a-number/summary.sig", nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("storage image ID must be an integer"))
			})

			It("should return error when requested image id is empty", func() {
				req, err := http.NewRequest("GET", "/storage/images-repos//summary.sig", nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("storage image ID required"))
			})

			It("should return error when target file path is missing", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/", image.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("target repository file path is missing"))
			})

			It("should return error when image commit has empty repo", func() {
				image := models.Image{
					OrgID: orgID,
					Name:  faker.UUIDHyphenated(),
					Commit: &models.Commit{
						OrgID: orgID,
						Repo: &models.Repo{
							URL:    "",
							Status: models.ImageStatusSuccess,
						},
					},
				}
				db.DB.Create(&image)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/%s", image.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("image repository does not exist"))
			})

			It("should return error when image commit is without repo", func() {
				image := models.Image{
					OrgID: orgID,
					Name:  faker.UUIDHyphenated(),
					Commit: &models.Commit{
						OrgID: orgID,
						Repo:  nil,
					},
				}
				result := db.DB.Create(&image)
				Expect(result.Error).ToNot(HaveOccurred())

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/%s", image.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("image repository does not exist"))
			})

			It("should return error when image commit repo has an un-parseable url", func() {
				image := models.Image{
					OrgID: orgID,
					Name:  faker.UUIDHyphenated(),
					Commit: &models.Commit{
						OrgID: orgID,
						Repo: &models.Repo{
							URL:    "https:\t//repo-storage.org\n/path/to/bucket",
							Status: models.ImageStatusSuccess,
						},
					},
				}
				result := db.DB.Create(&image)
				Expect(result.Error).ToNot(HaveOccurred())

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/%s", image.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("bad image repository url"))
			})

			Context("org access", func() {
				// by org access we mean the org that created/own the image and repo
				var originalAuth bool
				var originalImageBuilderOrgID string
				conf := config.Get()
				imageBuilderOrgID := faker.UUIDHyphenated()

				BeforeEach(func() {
					// save original config auth and imageBuilderOrgID values
					originalAuth = conf.Auth
					originalImageBuilderOrgID = conf.ImageBuilderOrgID
					// set auth to True to force use identity
					conf.Auth = true
					// set ImageBuilderOrgID to the current defined one
					conf.ImageBuilderOrgID = imageBuilderOrgID
					ctrl = gomock.NewController(GinkgoT())
					router = chi.NewRouter()
					router.Use(func(next http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							// set identity orgID to orgID the creator/owner of the image
							ctx := testHelpers.WithCustomIdentity(r.Context(), orgID)
							ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
							next.ServeHTTP(w, r.WithContext(ctx))
						})
					})
					router.Route("/storage", MakeStorageRouter)
				})

				AfterEach(func() {
					// restore config values
					conf.Auth = originalAuth
					conf.ImageBuilderOrgID = originalImageBuilderOrgID
					ctrl.Finish()
				})

				It("org user should return the requested resource content", func() {
					targetRepoFile := "summary.sig"
					req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/%s", image.ID, targetRepoFile), nil)
					Expect(err).ToNot(HaveOccurred())

					fileContent := "this is a simple file content"

					url, err := url2.Parse(image.Commit.Repo.URL)
					Expect(err).ToNot(HaveOccurred())
					targetPath := fmt.Sprintf("%s/%s", url.Path, targetRepoFile)

					fileContentReader := strings.NewReader(fileContent)
					fileContentReadCloser := io.NopCloser(fileContentReader)
					mockFilesService.EXPECT().GetFile(targetPath).Return(fileContentReadCloser, nil)

					httpTestRecorder := httptest.NewRecorder()
					router.ServeHTTP(httpTestRecorder, req)

					Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
					respBody, err := io.ReadAll(httpTestRecorder.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(respBody)).To(Equal(fileContent))
				})
			})
			Context("image builder org access", func() {
				var originalAuth bool
				var originalImageBuilderOrgID string
				conf := config.Get()
				imageBuilderOrgID := faker.UUIDHyphenated()

				BeforeEach(func() {
					// save original config auth and imageBuilderOrgID values
					originalAuth = conf.Auth
					originalImageBuilderOrgID = conf.ImageBuilderOrgID
					// set auth to True to force use identity
					conf.Auth = true
					// set ImageBuilderOrgID to the current defined one
					conf.ImageBuilderOrgID = imageBuilderOrgID
					ctrl = gomock.NewController(GinkgoT())
					router = chi.NewRouter()
					router.Use(func(next http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							// set identity orgID to imageBuilderOrgID
							ctx := testHelpers.WithCustomIdentity(r.Context(), imageBuilderOrgID)
							ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
							next.ServeHTTP(w, r.WithContext(ctx))
						})
					})
					router.Route("/storage", MakeStorageRouter)
				})

				AfterEach(func() {
					// restore config values
					conf.Auth = originalAuth
					conf.ImageBuilderOrgID = originalImageBuilderOrgID
					ctrl.Finish()
				})

				It("image builder org user should return the requested resource content", func() {
					targetRepoFile := "summary.sig"
					req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/%s", image.ID, targetRepoFile), nil)
					Expect(err).ToNot(HaveOccurred())

					fileContent := "this is a simple file content"

					url, err := url2.Parse(image.Commit.Repo.URL)
					Expect(err).ToNot(HaveOccurred())
					targetPath := fmt.Sprintf("%s/%s", url.Path, targetRepoFile)

					fileContentReader := strings.NewReader(fileContent)
					fileContentReadCloser := io.NopCloser(fileContentReader)
					mockFilesService.EXPECT().GetFile(targetPath).Return(fileContentReadCloser, nil)

					httpTestRecorder := httptest.NewRecorder()
					router.ServeHTTP(httpTestRecorder, req)

					Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
					respBody, err := io.ReadAll(httpTestRecorder.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(respBody)).To(Equal(fileContent))
				})
			})

			Context("any other org access", func() {
				// by other org we mean the org accessing the image repo is not the one that created/own it,
				// and it is not image builder org
				var originalAuth bool
				var originalImageBuilderOrgID string
				conf := config.Get()
				imageBuilderOrgID := faker.UUIDHyphenated()
				otherOrgID := faker.UUIDHyphenated()

				BeforeEach(func() {
					// save original config auth and imageBuilderOrgID values
					originalAuth = conf.Auth
					originalImageBuilderOrgID = conf.ImageBuilderOrgID
					// set auth to True to force use identity
					conf.Auth = true
					// set ImageBuilderOrgID to the current defined one
					conf.ImageBuilderOrgID = imageBuilderOrgID
					ctrl = gomock.NewController(GinkgoT())
					router = chi.NewRouter()
					router.Use(func(next http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							// set identity orgID to otherOrgID
							ctx := testHelpers.WithCustomIdentity(r.Context(), otherOrgID)
							ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
							next.ServeHTTP(w, r.WithContext(ctx))
						})
					})
					router.Route("/storage", MakeStorageRouter)
				})

				AfterEach(func() {
					// restore config values
					conf.Auth = originalAuth
					conf.ImageBuilderOrgID = originalImageBuilderOrgID
					ctrl.Finish()
				})

				It("image is not found when other org try to access image repo", func() {
					targetRepoFile := "summary.sig"
					req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/%s", image.ID, targetRepoFile), nil)
					Expect(err).ToNot(HaveOccurred())

					httpTestRecorder := httptest.NewRecorder()
					router.ServeHTTP(httpTestRecorder, req)

					Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
					respBody, err := io.ReadAll(httpTestRecorder.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(respBody)).To(ContainSubstring("storage image not found"))
				})
			})
		})

		Context("GetImageRepoFileContent", func() {
			It("Should redirect to the requested resource content file", func() {
				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/content/%s", image.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				url, err := url2.Parse(image.Commit.Repo.URL)
				Expect(err).ToNot(HaveOccurred())
				targetPath := fmt.Sprintf("%s/%s", url.Path, targetRepoFile)
				expectedURL := fmt.Sprintf("%s/%s?signature", url, targetRepoFile)
				mockFilesService.EXPECT().GetSignedURL(targetPath).Return(expectedURL, nil)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusSeeOther))
				Expect(httpTestRecorder.Header()["Location"][0]).To(Equal(expectedURL))
			})
			It("should return error when the image does not exists", func() {

				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/content/summary.sig", 9999), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("storage image not found"))
			})

			It("should return error when requested image id is not a number", func() {
				req, err := http.NewRequest("GET", "/storage/images-repos/not-a-number/content/summary.sig", nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("storage image ID must be an integer"))
			})

			It("should return error when requested image id is empty", func() {
				req, err := http.NewRequest("GET", "/storage/images-repos//summary.sig", nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("storage image ID required"))
			})

			It("should return error when target file path is missing", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/content/", image.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("target repository file path is missing"))
			})

			It("should return error when image commit has empty repo", func() {
				image := models.Image{
					OrgID: orgID,
					Name:  faker.UUIDHyphenated(),
					Commit: &models.Commit{
						OrgID: orgID,
						Repo: &models.Repo{
							URL:    "",
							Status: models.ImageStatusSuccess,
						},
					},
				}
				db.DB.Create(&image)

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/content/%s", image.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("image repository does not exist"))
			})

			It("should return error when image commit is without repo", func() {
				image := models.Image{
					OrgID: orgID,
					Name:  faker.UUIDHyphenated(),
					Commit: &models.Commit{
						OrgID: orgID,
						Repo:  nil,
					},
				}
				result := db.DB.Create(&image)
				Expect(result.Error).ToNot(HaveOccurred())

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/content/%s", image.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("image repository does not exist"))
			})

			It("should return error when image commit repo has an un-parseable url", func() {
				image := models.Image{
					OrgID: orgID,
					Name:  faker.UUIDHyphenated(),
					Commit: &models.Commit{
						OrgID: orgID,
						Repo: &models.Repo{
							URL:    "https:\t//repo-storage.org\n/path/to/bucket",
							Status: models.ImageStatusSuccess,
						},
					},
				}
				result := db.DB.Create(&image)
				Expect(result.Error).ToNot(HaveOccurred())

				targetRepoFile := "summary.sig"
				req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/content/%s", image.ID, targetRepoFile), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("bad image repository url"))
			})

			Context("org access", func() {
				// by org access we mean the org that created/own the image and repo
				var originalAuth bool
				var originalImageBuilderOrgID string
				conf := config.Get()
				imageBuilderOrgID := faker.UUIDHyphenated()

				BeforeEach(func() {
					// save original config auth and imageBuilderOrgID values
					originalAuth = conf.Auth
					originalImageBuilderOrgID = conf.ImageBuilderOrgID
					// set auth to True to force use identity
					conf.Auth = true
					// set ImageBuilderOrgID to the current defined one
					conf.ImageBuilderOrgID = imageBuilderOrgID
					ctrl = gomock.NewController(GinkgoT())
					router = chi.NewRouter()
					router.Use(func(next http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							// set identity orgID to orgID the creator/owner of the image
							ctx := testHelpers.WithCustomIdentity(r.Context(), orgID)
							ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
							next.ServeHTTP(w, r.WithContext(ctx))
						})
					})
					router.Route("/storage", MakeStorageRouter)
				})

				AfterEach(func() {
					// restore config values
					conf.Auth = originalAuth
					conf.ImageBuilderOrgID = originalImageBuilderOrgID
					ctrl.Finish()
				})

				It("org user should redirect to the requested resource content", func() {
					targetRepoFile := "summary.sig"
					req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/content/%s", image.ID, targetRepoFile), nil)
					Expect(err).ToNot(HaveOccurred())

					url, err := url2.Parse(image.Commit.Repo.URL)
					Expect(err).ToNot(HaveOccurred())
					targetPath := fmt.Sprintf("%s/%s", url.Path, targetRepoFile)
					expectedURL := fmt.Sprintf("%s/%s?signature", url, targetRepoFile)
					mockFilesService.EXPECT().GetSignedURL(targetPath).Return(expectedURL, nil)

					httpTestRecorder := httptest.NewRecorder()
					router.ServeHTTP(httpTestRecorder, req)

					Expect(httpTestRecorder.Code).To(Equal(http.StatusSeeOther))
					Expect(httpTestRecorder.Header()["Location"][0]).To(Equal(expectedURL))
				})
			})
			Context("image builder org access", func() {
				var originalAuth bool
				var originalImageBuilderOrgID string
				conf := config.Get()
				imageBuilderOrgID := faker.UUIDHyphenated()

				BeforeEach(func() {
					// save original config auth and imageBuilderOrgID values
					originalAuth = conf.Auth
					originalImageBuilderOrgID = conf.ImageBuilderOrgID
					// set auth to True to force use identity
					conf.Auth = true
					// set ImageBuilderOrgID to the current defined one
					conf.ImageBuilderOrgID = imageBuilderOrgID
					ctrl = gomock.NewController(GinkgoT())
					router = chi.NewRouter()
					router.Use(func(next http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							// set identity orgID to imageBuilderOrgID
							ctx := testHelpers.WithCustomIdentity(r.Context(), imageBuilderOrgID)
							ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
							next.ServeHTTP(w, r.WithContext(ctx))
						})
					})
					router.Route("/storage", MakeStorageRouter)
				})

				AfterEach(func() {
					// restore config values
					conf.Auth = originalAuth
					conf.ImageBuilderOrgID = originalImageBuilderOrgID
					ctrl.Finish()
				})

				It("image builder org user should redirect to the requested resource content", func() {
					targetRepoFile := "summary.sig"
					req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/content/%s", image.ID, targetRepoFile), nil)
					Expect(err).ToNot(HaveOccurred())

					url, err := url2.Parse(image.Commit.Repo.URL)
					Expect(err).ToNot(HaveOccurred())
					targetPath := fmt.Sprintf("%s/%s", url.Path, targetRepoFile)
					expectedURL := fmt.Sprintf("%s/%s?signature", url, targetRepoFile)
					mockFilesService.EXPECT().GetSignedURL(targetPath).Return(expectedURL, nil)

					httpTestRecorder := httptest.NewRecorder()
					router.ServeHTTP(httpTestRecorder, req)

					Expect(httpTestRecorder.Code).To(Equal(http.StatusSeeOther))
					Expect(httpTestRecorder.Header()["Location"][0]).To(Equal(expectedURL))
				})
			})

			Context("any other org access", func() {
				// by other org we mean the org accessing the image repo is not the one that created/own it,
				// and it is not image builder org
				var originalAuth bool
				var originalImageBuilderOrgID string
				conf := config.Get()
				imageBuilderOrgID := faker.UUIDHyphenated()
				otherOrgID := faker.UUIDHyphenated()

				BeforeEach(func() {
					// save original config auth and imageBuilderOrgID values
					originalAuth = conf.Auth
					originalImageBuilderOrgID = conf.ImageBuilderOrgID
					// set auth to True to force use identity
					conf.Auth = true
					// set ImageBuilderOrgID to the current defined one
					conf.ImageBuilderOrgID = imageBuilderOrgID
					ctrl = gomock.NewController(GinkgoT())
					router = chi.NewRouter()
					router.Use(func(next http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							// set identity orgID to otherOrgID
							ctx := testHelpers.WithCustomIdentity(r.Context(), otherOrgID)
							ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
							next.ServeHTTP(w, r.WithContext(ctx))
						})
					})
					router.Route("/storage", MakeStorageRouter)
				})

				AfterEach(func() {
					// restore config values
					conf.Auth = originalAuth
					conf.ImageBuilderOrgID = originalImageBuilderOrgID
					ctrl.Finish()
				})

				It("image is not found when other org try to access image repo", func() {
					targetRepoFile := "summary.sig"
					req, err := http.NewRequest("GET", fmt.Sprintf("/storage/images-repos/%d/%s", image.ID, targetRepoFile), nil)
					Expect(err).ToNot(HaveOccurred())

					httpTestRecorder := httptest.NewRecorder()
					router.ServeHTTP(httpTestRecorder, req)

					Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
					respBody, err := io.ReadAll(httpTestRecorder.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(respBody)).To(ContainSubstring("storage image not found"))
				})
			})
		})
	})
})
