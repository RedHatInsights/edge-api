// FIXME: golangci-lint
// nolint:revive,typecheck
package imagebuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/bxcodec/faker/v3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
)

func TestModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Image Builder Client Suite")
}

// Custom type that allows setting the func that our Mock Do func will run instead
type MockDoType func(req *http.Request) (*http.Response, error)

// MockClient is the mock client
type MockClient struct {
	MockDo MockDoType
}

// Overriding what the Do function should "do" in our MockClient
func (m *MockClient) Do(req *http.Request) (*http.Response, error) {
	return m.MockDo(req)
}

var _ = Describe("Image Builder Client Test", func() {
	var client *Client
	var dbName string
	var originalImageBuilderURL string
	conf := config.Get()
	BeforeEach(func() {
		config.Init()
		config.Get().Debug = true
		dbName = fmt.Sprintf("%d-client.db", time.Now().UnixNano())
		config.Get().Database.Name = dbName
		db.InitDB()

		err := db.DB.AutoMigrate(
			&models.ImageSet{},
			&models.Commit{},
			&models.UpdateTransaction{},
			&models.Package{},
			&models.Image{},
			&models.Repo{},
			&models.Device{},
			&models.DispatchRecord{},
		)
		if err != nil {
			panic(err)
		}
		client = InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
		// save the original image builder url
		originalImageBuilderURL = conf.ImageBuilderConfig.URL
	})
	AfterEach(func() {
		os.Remove(dbName)
		// restore the original image builder url
		conf.ImageBuilderConfig.URL = originalImageBuilderURL
		// disable passing user to image builder feature flag
		err := os.Unsetenv(feature.PassUserToImageBuilder.EnvVar)
		Expect(err).ToNot(HaveOccurred())
	})
	It("should init client", func() {
		Expect(client).ToNot(BeNil())
	})
	It("test validation of correct package name", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"meta":{"count":5}}`)
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL
		res, err := client.SearchPackage("vim", "x86_64", "rhel-85")
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Meta.Count).To(Equal(5))
	})
	It("test validation of wrong package name", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"meta":{"count":0}}`)
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL
		res, err := client.SearchPackage("badrpm", "x86_64", "rhel-85")
		Expect(err).To(BeNil())
		Expect(res.Meta.Count).To(Equal(0))
	})
	It("test web service when package search returns StatusBadRequest", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, `{"error":{"count":0}}`)
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL
		_, err := client.SearchPackage("badpackage", "x86_64", "rhel-85")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("image builder search packages request error"))
	})
	It("test validation of special character package name", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"meta":{"count":1}}`)
			pkgName := r.URL.Query().Get("search")
			Expect(pkgName).To(Equal("gcc-c++"))
			parampkgName := r.URL.RawQuery
			Expect(parampkgName).To(ContainSubstring("search=gcc-c%2B%2B"))

		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL
		res, err := client.SearchPackage("gcc-c++", "x86_64", "rhel-85")

		Expect(err).To(BeNil())
		Expect(res.Meta.Count).To(Equal(1))
	})
	It("test validation of empty package name", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL
		res, err := client.SearchPackage("", "x86_64", "rhel-85")
		Expect(err.Error()).To(Equal("mandatory fields should not be empty"))
		Expect(res).To(BeNil())
	})
	It("test GetComposeStatus with valid parameters", func() {
		jobId := faker.UUIDHyphenated()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"image_status":{"status": "success", "reason":"success"}}`)
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL
		res, err := client.GetComposeStatus(jobId)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.ImageStatus.Status).To(Equal(imageStatusSuccess))
	})
	It("test GetComposeStatus with failed status", func() {
		jobId := faker.UUIDHyphenated()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"image_status":{"status": "failure", "reason":"Worker running this job stopped responding"}}`)
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL
		res, err := client.GetComposeStatus(jobId)
		Expect(res).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("worker running this job stopped responding"))
	})
	It("test GetComposeStatus error on request", func() {
		jobId := faker.UUIDHyphenated()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, `{"image_status":{"status": "success", "reason":"success"}}`)
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL
		res, err := client.GetComposeStatus(jobId)
		Expect(res).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("request for status was not successful"))
	})
	It("test GetComposeStatus error on parser Json to Object", func() {
		jobId := faker.UUIDHyphenated()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Added an invalid char to fail json parser
			fmt.Fprintln(w, `{"image_status":{"status": "success", "reason":"invalid status"}_`)
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL
		res, err := client.GetComposeStatus(jobId)
		Expect(res).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("invalid character '_' after object key:value pair"))
	})
	It("test GetComposeStatus error on empty body response", func() {
		jobId := faker.UUIDHyphenated()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL
		res, err := client.GetComposeStatus(jobId)
		Expect(res).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("unexpected end of JSON input"))
	})
	It("test compose image", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintln(w, `{"id": "compose-job-id-returned-from-image-builder"}`)
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL

		pkgs := []models.Package{
			{
				Name: "vim",
			},
			{
				Name: "ansible",
			},
		}
		img := &models.Image{Distribution: "rhel-8",
			Packages: pkgs,
			Commit: &models.Commit{
				Arch: "x86_64",
				Repo: &models.Repo{},
			}}
		img, err := client.ComposeCommit(img)
		Expect(err).ToNot(HaveOccurred())
		Expect(img).ToNot(BeNil())
		Expect(img.Commit.ComposeJobID).To(Equal("compose-job-id-returned-from-image-builder"))
		Expect(img.Commit.ExternalURL).To(BeFalse())
	})
	It("image actication key is filled", func() {
		composeJobID := faker.UUIDHyphenated()
		orgID := "123"
		repoURL := fmt.Sprintf("%s/api/edge/v1/storage/images-repos/12345", config.Get().EdgeCertAPIBaseURL)
		dist := "rhel-84"
		newDist := "rhel-85"

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ComposeRequest
			body, err := io.ReadAll(r.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(body).ToNot(BeNil())
			err = json.Unmarshal(body, &req)
			Expect(err).ToNot(HaveOccurred())
			Expect(req.Customizations.Subscription.ActivationKey).To(Equal("test-key"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, err = fmt.Fprintf(w, `{"id": "%s"}`, composeJobID)
			Expect(err).ToNot(HaveOccurred())
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL

		img := &models.Image{Distribution: dist,
			Name:  faker.Name(),
			OrgID: orgID,
			Commit: &models.Commit{
				OrgID:              orgID,
				Arch:               "x86_64",
				Repo:               &models.Repo{},
				OSTreeRef:          config.DistributionsRefs[newDist],
				OSTreeParentRef:    config.DistributionsRefs[dist],
				OSTreeParentCommit: repoURL,
			},
			ActivationKey: "test-key",
		}

		img, err := client.ComposeCommit(img)
		Expect(err).ToNot(HaveOccurred())
		Expect(img).ToNot(BeNil())
		Expect(img.Commit.ComposeJobID).To(Equal(composeJobID))
		Expect(img.Commit.ExternalURL).To(BeFalse())
	})

	It("image actication key is not filled", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ComposeRequest
			body, err := io.ReadAll(r.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(body).ToNot(BeNil())
			err = json.Unmarshal(body, &req)
			Expect(err).ToNot(HaveOccurred())
			Expect(req.Customizations.Subscription).To(BeNil())
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintln(w, `{"id": "compose-job-id-returned-from-image-builder"}`)
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL

		pkgs := []models.Package{
			{
				Name: "vim",
			},
			{
				Name: "ansible",
			},
		}
		img := &models.Image{Distribution: "rhel-8",
			Packages: pkgs,
			Commit: &models.Commit{
				Arch: "x86_64",
				Repo: &models.Repo{},
			}}
		img, err := client.ComposeCommit(img)
		Expect(err).ToNot(HaveOccurred())
		Expect(img).ToNot(BeNil())
		Expect(img.Commit.ComposeJobID).To(Equal("compose-job-id-returned-from-image-builder"))
		Expect(img.Commit.ExternalURL).To(BeFalse())
	})

	It("should return error when image has org_id undefined", func() {
		pkgs := []models.Package{
			{
				Name: "vim",
			},
			{
				Name: "ansible",
			},
		}
		img := &models.Image{Distribution: "rhel-8",
			Packages: pkgs,
			Commit: &models.Commit{
				Arch: "x86_64",
				Repo: &models.Repo{},
			},
			ThirdPartyRepositories: []models.ThirdPartyRepo{
				{
					Name: "repo test",
					URL:  "https://repo.com",
				},
				{
					Name: "repo test2",
					URL:  "https://repo2.com",
				},
			},
		}
		result, err := client.GetImageThirdPartyRepos(img)
		Expect(result).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("error retrieving orgID  information, image orgID undefined"))
	})

	It("should retrieve and return third party repos from database and return valid list", func() {
		pkgs := []models.Package{
			{
				Name: "vim",
			},
			{
				Name: "ansible",
			},
		}
		OrgId := faker.UUIDHyphenated()
		thirdPartyRepo := models.ThirdPartyRepo{
			Name:  faker.UUIDHyphenated(),
			URL:   faker.URL(),
			OrgID: OrgId,
		}
		db.DB.Create(&thirdPartyRepo)
		thirdPartyRepo2 := models.ThirdPartyRepo{
			Name:   faker.UUIDHyphenated(),
			URL:    faker.URL(),
			OrgID:  OrgId,
			GpgKey: "some dummy data",
		}
		db.DB.Create(&thirdPartyRepo2)
		img := &models.Image{Distribution: "rhel-8",
			Packages: pkgs,
			OrgID:    OrgId,
			Commit: &models.Commit{
				Arch: "x86_64",
				Repo: &models.Repo{},
			},
			ThirdPartyRepositories: []models.ThirdPartyRepo{
				thirdPartyRepo,
				thirdPartyRepo2,
			},
		}
		result, err := client.GetImageThirdPartyRepos(img)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(result)).To(Equal(2))
		Expect(result[0].BaseURL).To(Equal(thirdPartyRepo.URL))
		Expect(result[0].CheckGPG).To(BeNil())
		Expect(result[0].GPGKey).To(BeNil())
		Expect(result[1].BaseURL).To(Equal(thirdPartyRepo2.URL))
		Expect(result[1].GPGKey).ToNot(BeNil())
		Expect(*result[1].GPGKey).To(Equal(thirdPartyRepo2.GpgKey))
		Expect(result[1].CheckGPG).ToNot(BeNil())
		Expect(*result[1].CheckGPG).To(BeTrue())
	})

	It("should return error when custom repositories id are not valid/not found", func() {
		pkgs := []models.Package{
			{
				Name: "vim",
			},
			{
				Name: "ansible",
			},
		}
		img := &models.Image{Distribution: "rhel-8",
			Packages: pkgs,
			OrgID:    "org_id",
			Commit: &models.Commit{
				Arch: "x86_64",
				Repo: &models.Repo{},
			},
			ThirdPartyRepositories: []models.ThirdPartyRepo{
				{
					Name: "repo test",
					URL:  "https://repo.com",
				},
				{
					Name: "repo test2",
					URL:  "https://repo2.com",
				},
			},
		}
		result, err := client.GetImageThirdPartyRepos(img)
		Expect(result).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("enter valid third party repository id"))
	})

	It("compose image should send thirdparty repos", func() {
		composeJobID := faker.UUIDHyphenated()
		dist := "rhel-91"
		orgID := faker.UUIDHyphenated()
		thirdPartyRepo1 := models.ThirdPartyRepo{
			Name:  faker.UUIDHyphenated(),
			URL:   faker.URL(),
			OrgID: orgID,
		}
		db.DB.Create(&thirdPartyRepo1)
		thirdPartyRepo2 := models.ThirdPartyRepo{
			Name:   faker.UUIDHyphenated(),
			URL:    faker.URL(),
			OrgID:  orgID,
			GpgKey: "some dummy data",
		}
		db.DB.Create(&thirdPartyRepo2)

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var composeRequest ComposeRequest
			body, err := io.ReadAll(r.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(body).ToNot(BeNil())
			Expect(string(body)).To(ContainSubstring(`"gpgkey":"some dummy data"`))
			err = json.Unmarshal(body, &composeRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(composeRequest.Customizations).ToNot(BeNil())
			Expect(composeRequest.Customizations.PayloadRepositories).ToNot(BeNil())
			payloadRepositories := *composeRequest.Customizations.PayloadRepositories

			Expect(len(payloadRepositories)).To(Equal(2))
			Expect(payloadRepositories[0].BaseURL).To(Equal(thirdPartyRepo1.URL))
			Expect(payloadRepositories[0].GPGKey).To(BeNil())
			Expect(payloadRepositories[0].CheckGPG).To(BeNil())

			Expect(payloadRepositories[1].BaseURL).To(Equal(thirdPartyRepo2.URL))
			Expect(payloadRepositories[1].GPGKey).ToNot(BeNil())
			Expect(*payloadRepositories[1].GPGKey).To(Equal(thirdPartyRepo2.GpgKey))
			Expect(payloadRepositories[1].CheckGPG).ToNot(BeNil())
			Expect(*payloadRepositories[1].CheckGPG).To(BeTrue())

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, err = fmt.Fprintf(w, `{"id": "%s"}`, composeJobID)
			Expect(err).ToNot(HaveOccurred())
		}))
		defer ts.Close()

		config.Get().ImageBuilderConfig.URL = ts.URL
		img := &models.Image{
			Distribution: dist,
			OrgID:        orgID,
			Commit: &models.Commit{
				Arch:      "x86_64",
				Repo:      &models.Repo{},
				OSTreeRef: config.DistributionsRefs[dist],
			},
			ThirdPartyRepositories: []models.ThirdPartyRepo{thirdPartyRepo1, thirdPartyRepo2},
		}

		img, err := client.ComposeCommit(img)
		Expect(err).ToNot(HaveOccurred())
		Expect(img).ToNot(BeNil())
		Expect(img.Commit.ComposeJobID).To(Equal(composeJobID))
	})

	Context("compose image commit with ChangesRefs values", func() {
		dist := "rhel-86"
		repoURL := faker.URL()
		var originalImageBuilderURL string
		conf := config.Get()

		BeforeEach(func() {
			// save the original image builder url
			originalImageBuilderURL = conf.ImageBuilderConfig.URL
		})

		AfterEach(func() {
			// restore the original image builder url
			conf.ImageBuilderConfig.URL = originalImageBuilderURL
		})

		When("change ref is true", func() {
			// this happens when upgrading an image to a different major version, example: form 8.6 to 9.0
			newDist := "rhel-90"
			composeJobID := faker.UUIDHyphenated()
			It("parent ref is present in the request ", func() {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var req ComposeRequest
					body, err := io.ReadAll(r.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(body).ToNot(BeNil())
					err = json.Unmarshal(body, &req)
					Expect(err).ToNot(HaveOccurred())
					Expect(req.ImageRequests[0].Ostree).ToNot(BeNil())
					Expect(req.ImageRequests[0].Ostree.URL).To(Equal(repoURL))
					Expect(req.ImageRequests[0].Ostree.Ref).To(Equal(config.DistributionsRefs[newDist]))
					Expect(req.ImageRequests[0].Ostree.ParentRef).To(Equal(config.DistributionsRefs[dist]))
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					_, err = fmt.Fprintf(w, `{"id": "%s"}`, composeJobID)
					Expect(err).ToNot(HaveOccurred())

				}))
				defer ts.Close()

				Expect(dist).ToNot(Equal(newDist))
				Expect(config.DistributionsRefs[dist]).ToNot(BeEmpty())
				Expect(config.DistributionsRefs[newDist]).ToNot(BeEmpty())
				Expect(config.DistributionsRefs[newDist]).ToNot(Equal(config.DistributionsRefs[dist]))

				config.Get().ImageBuilderConfig.URL = ts.URL
				img := &models.Image{Distribution: dist,
					Commit: &models.Commit{
						Arch:               "x86_64",
						Repo:               &models.Repo{},
						OSTreeRef:          config.DistributionsRefs[newDist],
						OSTreeParentRef:    config.DistributionsRefs[dist],
						ChangesRefs:        true,
						OSTreeParentCommit: repoURL,
					}}
				img, err := client.ComposeCommit(img)
				Expect(err).ToNot(HaveOccurred())
				Expect(img).ToNot(BeNil())
				Expect(img.Commit.ComposeJobID).To(Equal(composeJobID))
				Expect(img.Commit.ExternalURL).To(BeFalse())
			})
		})

		When("change ref is false", func() {
			dist := "rhel-85" // nolint:gofmt,goimports,govet
			newDist := "rhel-86"
			composeJobID := faker.UUIDHyphenated()
			type TestOSTree struct { // nolint:gofmt,goimports,govet
				URL       *string `json:"url,omitempty"`
				Ref       string  `json:"ref"`
				ParentRef *string `json:"parent"`
			}

			type TestImageRequest struct { // nolint:gofmt,goimports,govet
				Architecture  string         `json:"architecture"`
				ImageType     string         `json:"image_type"`
				Ostree        *TestOSTree    `json:"ostree,omitempty"`
				UploadRequest *UploadRequest `json:"upload_request"`
			}

			type TestComposeRequest struct {
				Customizations *Customizations    `json:"customizations"`
				Distribution   string             `json:"distribution"`
				ImageRequests  []TestImageRequest `json:"image_requests"`
			}

			It("parent ref is not present in the request when parent ref equal to current ref ", func() {
				// this happens when updating an image within the same major version
				// example: form 8.5 to 8.5 to change the installed packages collections or updating from 8.5 to 8.6
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var req TestComposeRequest
					body, err := io.ReadAll(r.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(body).ToNot(BeNil())
					err = json.Unmarshal(body, &req)
					Expect(err).ToNot(HaveOccurred())
					Expect(req.ImageRequests[0].Ostree).ToNot(BeNil())
					Expect(*req.ImageRequests[0].Ostree.URL).To(Equal(repoURL))
					Expect(req.ImageRequests[0].Ostree.Ref).To(Equal(config.DistributionsRefs[newDist]))
					Expect(req.ImageRequests[0].Ostree.ParentRef).To(BeNil())
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					_, err = fmt.Fprintf(w, `{"id": "%s"}`, composeJobID)
					Expect(err).ToNot(HaveOccurred())

				}))
				defer ts.Close()

				Expect(dist).ToNot(Equal(newDist))
				Expect(config.DistributionsRefs[dist]).ToNot(BeEmpty())
				Expect(config.DistributionsRefs[newDist]).ToNot(BeEmpty())
				Expect(config.DistributionsRefs[newDist]).To(Equal(config.DistributionsRefs[dist]))

				config.Get().ImageBuilderConfig.URL = ts.URL
				img := &models.Image{Distribution: dist,
					Commit: &models.Commit{
						Arch:               "x86_64",
						Repo:               &models.Repo{},
						OSTreeRef:          config.DistributionsRefs[newDist],
						OSTreeParentRef:    config.DistributionsRefs[dist],
						ChangesRefs:        false,
						OSTreeParentCommit: repoURL,
					}}
				img, err := client.ComposeCommit(img)
				Expect(err).ToNot(HaveOccurred())
				Expect(img).ToNot(BeNil())
				Expect(img.Commit.ComposeJobID).To(Equal(composeJobID))
				Expect(img.Commit.ExternalURL).To(BeFalse())
			})

			It("parent ref and url are not present in the request when parent repo url is empty", func() {
				// this happens when creating a new image
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var req TestComposeRequest
					body, err := io.ReadAll(r.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(body).ToNot(BeNil())
					err = json.Unmarshal(body, &req)
					Expect(err).ToNot(HaveOccurred())
					Expect(req.ImageRequests[0].Ostree).ToNot(BeNil())
					Expect(req.ImageRequests[0].Ostree.Ref).To(Equal(config.DistributionsRefs[newDist]))
					Expect(req.ImageRequests[0].Ostree.URL).To(BeNil())
					Expect(req.ImageRequests[0].Ostree.ParentRef).To(BeNil())
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					_, err = fmt.Fprintf(w, `{"id": "%s"}`, composeJobID)
					Expect(err).ToNot(HaveOccurred())
				}))
				defer ts.Close()

				Expect(config.DistributionsRefs[newDist]).ToNot(BeEmpty())

				config.Get().ImageBuilderConfig.URL = ts.URL
				img := &models.Image{Distribution: dist,
					Commit: &models.Commit{
						Arch:               "x86_64",
						Repo:               &models.Repo{},
						OSTreeRef:          config.DistributionsRefs[newDist],
						OSTreeParentRef:    "",
						ChangesRefs:        false,
						OSTreeParentCommit: "",
					}}
				img, err := client.ComposeCommit(img)
				Expect(err).ToNot(HaveOccurred())
				Expect(img).ToNot(BeNil())
				Expect(img.Commit.ComposeJobID).To(Equal(composeJobID))
				Expect(img.Commit.ExternalURL).To(BeFalse())
			})
		})
	})

	It("test compose installer", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintln(w, `{"id": "compose-job-id-returned-from-image-builder"}`)
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL

		pkgs := []models.Package{
			{
				Name: "vim",
			},
			{
				Name: "ansible",
			},
		}
		img := &models.Image{Distribution: "rhel-8",
			Packages: pkgs,
			Commit: &models.Commit{
				Arch: "x86_64",
				Repo: &models.Repo{},
			},
			Installer: &models.Installer{},
		}
		img, err := client.ComposeInstaller(img)
		Expect(err).ToNot(HaveOccurred())
		Expect(img).ToNot(BeNil())
		Expect(img.Installer.ComposeJobID).To(Equal("compose-job-id-returned-from-image-builder"))
		Expect(img.Commit.ExternalURL).To(BeFalse())
	})

	It("test compose installer with username and ssh-key", func() {
		// enable feature flag
		err := os.Setenv(feature.PassUserToImageBuilder.EnvVar, "true")
		Expect(err).ToNot(HaveOccurred())
		installer := models.Installer{Username: faker.Username(), SSHKey: faker.UUIDHyphenated()}
		composeJobID := "compose-job-id-returned-from-image-builder"
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).ToNot(BeNil())
			var req ComposeRequest
			err = json.Unmarshal(b, &req)
			Expect(err).ToNot(HaveOccurred())
			Expect(req.Customizations.Users).ToNot(BeNil())
			Expect(len(*req.Customizations.Users)).To(Equal(1))
			Expect((*req.Customizations.Users)[0].Name).To(Equal(installer.Username))
			Expect((*req.Customizations.Users)[0].SSHKey).To(Equal(installer.SSHKey))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			payload := fmt.Sprintf(`{"id": "%s"}`, composeJobID)
			_, err = fmt.Fprintln(w, payload)
			Expect(err).ToNot(HaveOccurred())
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL

		pkgs := []models.Package{{Name: "vim"}, {Name: "ansible"}}
		img := &models.Image{
			Distribution: "rhel-8",
			Packages:     pkgs,
			Commit:       &models.Commit{Arch: "x86_64", Repo: &models.Repo{}},
			Installer:    &installer,
		}
		img, err = client.ComposeInstaller(img)
		Expect(err).ToNot(HaveOccurred())
		Expect(img).ToNot(BeNil())
		Expect(img.Installer.ComposeJobID).To(Equal(composeJobID))
		Expect(img.Commit.ExternalURL).To(BeFalse())
	})

	It("test compose installer without username and ssh-key", func() {
		// enable feature flag
		err := os.Setenv(feature.PassUserToImageBuilder.EnvVar, "true")
		Expect(err).ToNot(HaveOccurred())
		// install has no username and ssh-key
		installer := models.Installer{}
		composeJobID := "compose-job-id-returned-from-image-builder"
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).ToNot(BeNil())
			var req ComposeRequest
			err = json.Unmarshal(b, &req)
			Expect(err).ToNot(HaveOccurred())
			// when installer username or ssh-key are empty no user is passed to image-builder
			Expect(req.Customizations.Users).To(BeNil())

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			payload := fmt.Sprintf(`{"id": "%s"}`, composeJobID)
			_, err = fmt.Fprintln(w, payload)
			Expect(err).ToNot(HaveOccurred())
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL

		pkgs := []models.Package{{Name: "vim"}, {Name: "ansible"}}
		img := &models.Image{
			Distribution: "rhel-8",
			Packages:     pkgs,
			Commit:       &models.Commit{Arch: "x86_64", Repo: &models.Repo{}},
			Installer:    &installer,
		}
		img, err = client.ComposeInstaller(img)
		Expect(err).ToNot(HaveOccurred())
		Expect(img).ToNot(BeNil())
		Expect(img.Installer.ComposeJobID).To(Equal(composeJobID))
		Expect(img.Commit.ExternalURL).To(BeFalse())
	})

	It("test compose installer should not pass username and ssh-key when feature flag is disabled", func() {
		// ensure feature flag disabled
		err := os.Unsetenv(feature.PassUserToImageBuilder.EnvVar)
		Expect(err).ToNot(HaveOccurred())
		installer := models.Installer{Username: faker.Username(), SSHKey: faker.UUIDHyphenated()}
		composeJobID := "compose-job-id-returned-from-image-builder"
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).ToNot(BeNil())
			var req ComposeRequest
			err = json.Unmarshal(b, &req)
			Expect(err).ToNot(HaveOccurred())
			// when feature flag is disabled the user and ssh key should not not be passed to image builder
			Expect(req.Customizations.Users).To(BeNil())

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			payload := fmt.Sprintf(`{"id": "%s"}`, composeJobID)
			_, err = fmt.Fprintln(w, payload)
			Expect(err).ToNot(HaveOccurred())
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL

		pkgs := []models.Package{{Name: "vim"}, {Name: "ansible"}}
		img := &models.Image{
			Distribution: "rhel-8",
			Packages:     pkgs,
			Commit:       &models.Commit{Arch: "x86_64", Repo: &models.Repo{}},
			Installer:    &installer,
		}
		img, err = client.ComposeInstaller(img)
		Expect(err).ToNot(HaveOccurred())
		Expect(img).ToNot(BeNil())
		Expect(img.Installer.ComposeJobID).To(Equal(composeJobID))
		Expect(img.Commit.ExternalURL).To(BeFalse())
	})

	It("test compose image when ostree parent commit is empty", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			b, err := io.ReadAll(r.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).ToNot(BeNil())
			var req ComposeRequest
			err = json.Unmarshal(b, &req)
			Expect(err).ToNot(HaveOccurred())
			Expect(req.ImageRequests[0].Ostree).To(BeNil())
			fmt.Fprintln(w, `{"id": "compose-job-id-returned-from-image-builder"}`)
		}))
		defer ts.Close()
		config.Get().ImageBuilderConfig.URL = ts.URL

		pkgs := []models.Package{}
		img := &models.Image{Distribution: "rhel-8",
			Packages: pkgs,
			Commit: &models.Commit{
				Arch: "x86_64",
				Repo: &models.Repo{},
			}}
		img, err := client.ComposeCommit(img)
		Expect(err).ToNot(HaveOccurred())
		Expect(img).ToNot(BeNil())
		Expect(img.Commit.ComposeJobID).To(Equal("compose-job-id-returned-from-image-builder"))
		Expect(img.Commit.ExternalURL).To(BeFalse())
	})

	Context("edge-management.storage_images_repos  feature", func() {
		BeforeEach(func() {
			err := os.Setenv("STORAGE_IMAGES_REPOS", "True")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			os.Unsetenv("STORAGE_IMAGES_REPOS")
		})

		It("when repo url use cert endpoint rhsm is true", func() {
			composeJobID := faker.UUIDHyphenated()
			orgID := faker.UUIDHyphenated()
			repoURL := fmt.Sprintf("%s/api/edge/v1/storage/images-repos/12345", config.Get().EdgeCertAPIBaseURL)
			dist := "rhel-84"
			newDist := "rhel-85"

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req ComposeRequest
				body, err := io.ReadAll(r.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(body).ToNot(BeNil())
				err = json.Unmarshal(body, &req)
				Expect(err).ToNot(HaveOccurred())
				Expect(req.ImageRequests[0].Ostree.URL).To(Equal(repoURL))
				Expect(req.ImageRequests[0].Ostree.ContentURL).To(Equal(repoURL))
				Expect(req.ImageRequests[0].Ostree.RHSM).To(BeTrue())
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, err = fmt.Fprintf(w, `{"id": "%s"}`, composeJobID)
				Expect(err).ToNot(HaveOccurred())
			}))
			defer ts.Close()
			config.Get().ImageBuilderConfig.URL = ts.URL

			img := &models.Image{Distribution: dist,
				Name:  faker.Name(),
				OrgID: orgID,
				Commit: &models.Commit{
					OrgID:              orgID,
					Arch:               "x86_64",
					Repo:               &models.Repo{},
					OSTreeRef:          config.DistributionsRefs[newDist],
					OSTreeParentRef:    config.DistributionsRefs[dist],
					OSTreeParentCommit: repoURL,
				},
			}

			img, err := client.ComposeCommit(img)
			Expect(err).ToNot(HaveOccurred())
			Expect(img).ToNot(BeNil())
			Expect(img.Commit.ComposeJobID).To(Equal(composeJobID))
			Expect(img.Commit.ExternalURL).To(BeFalse())
		})

		It("ComposeInstaller use cert endpoint with rhsm true", func() {
			composeJobID := faker.UUIDHyphenated()
			orgID := faker.UUIDHyphenated()
			dist := "rhel-84"
			newDist := "rhel-85"
			img := models.Image{Distribution: dist,
				Name:      faker.Name(),
				OrgID:     orgID,
				Installer: &models.Installer{OrgID: orgID},
				Commit: &models.Commit{
					OrgID:              orgID,
					Arch:               "x86_64",
					Repo:               &models.Repo{},
					OSTreeRef:          config.DistributionsRefs[newDist],
					OSTreeParentRef:    config.DistributionsRefs[dist],
					OSTreeParentCommit: faker.URL(),
				},
			}
			result := db.DB.Create(&img)
			Expect(result.Error).ToNot(HaveOccurred())
			expectedRepoURL := fmt.Sprintf("%s/api/edge/v1/storage/images-repos/%d", config.Get().EdgeCertAPIBaseURL, img.ID)

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req ComposeRequest
				body, err := io.ReadAll(r.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(body).ToNot(BeNil())
				err = json.Unmarshal(body, &req)
				Expect(err).ToNot(HaveOccurred())
				Expect(req.ImageRequests[0].Ostree.URL).To(Equal(expectedRepoURL))
				Expect(req.ImageRequests[0].Ostree.ContentURL).To(Equal(expectedRepoURL))
				Expect(req.ImageRequests[0].Ostree.RHSM).To(BeTrue())
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, err = fmt.Fprintf(w, `{"id": "%s"}`, composeJobID)
				Expect(err).ToNot(HaveOccurred())
			}))
			defer ts.Close()
			config.Get().ImageBuilderConfig.URL = ts.URL

			image, err := client.ComposeInstaller(&img)
			Expect(err).ToNot(HaveOccurred())
			Expect(image).ToNot(BeNil())
			Expect(image.Installer.ComposeJobID).To(Equal(composeJobID))
			Expect(img.Commit.ExternalURL).To(BeFalse())
		})
	})

	Describe("get thirdpartyrepo information", func() {
		Context("when thirdpartyrepo information does exists", func() {
			It("should have third party repository url as payloadrepository baseurl", func() {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					fmt.Fprintln(w, `{"id": "compose-request-id-returned-from-image-builder"}`)
				}))
				defer ts.Close()
				config.Get().ImageBuilderConfig.URL = ts.URL
				thirdpartyrepoURL := "http://www.thirdpartyrepo.com"
				repos := Repository{
					BaseURL: thirdpartyrepoURL,
				}
				req := &ComposeRequest{
					Customizations: &Customizations{
						PayloadRepositories: &[]Repository{repos},
					},
					Distribution: "rhel-8",
					ImageRequests: []ImageRequest{
						{
							Architecture: "x86_64",
							ImageType:    models.ImageTypeCommit,
							UploadRequest: &UploadRequest{
								Options: make(map[string]string),
								Type:    "aws.s3",
							},
						}},
				}
				cr, err := client.compose(req)
				Expect(err).ToNot(HaveOccurred())
				Expect(cr).ToNot(BeNil())
				Expect(cr.ID).To(Equal("compose-request-id-returned-from-image-builder"))

			})
		})
	})

	Describe("get metadata information", func() {

		Context("when pakages exists", func() {
			BeforeEach(func() {
				err := os.Setenv("DEDUP_INSTALLED_PACKAGES", "True")
				Expect(err).ToNot(HaveOccurred())
				pkgs := []models.InstalledPackage{}
				pkgs = append(pkgs, models.InstalledPackage{Name: "rhc",
					Version: "1",
					Epoch:   "1",
					Release: "1",
					Arch:    "x86_64"})
				img := &models.Image{Distribution: "rhel-8",
					Commit: &models.Commit{
						Arch:              "x86_64",
						Repo:              &models.Repo{},
						ComposeJobID:      faker.UUIDHyphenated(),
						InstalledPackages: pkgs,
					}}
				db.DB.Save(img.Commit.InstalledPackages)
				db.DB.Save(img.Commit)
				db.DB.Save(img)

			})
			AfterEach(func() {
				// disable the feature
				os.Unsetenv("DEDUP_INSTALLED_PACKAGES")
			})
			It("should not create new packages RHC into db", func() {
				pkgs := []models.Package{}
				img := &models.Image{Distribution: "rhel-8",
					Packages: pkgs,
					Commit: &models.Commit{
						OrgID:        faker.UUIDHyphenated(),
						Arch:         "x86_64",
						Repo:         &models.Repo{},
						ComposeJobID: faker.UUIDHyphenated(),
					}}

				// build our response JSON
				jsonResponse := `{
				"ostree_commit": "mock-repo",
				"packages": [
					{"arch": "x86_64", "name": "rhc", "release": "1", "sigmd5": "a", "signature": "b", "type": "rpm", "version": "1", "epoch":"1"
					}, 
					{"arch": "x86_64", "name": "NetworkManager", "release": "3.el9", "sigmd5": "c", "signature": "d", "type": "rpm", "version": "1.18.2"
					},
					{"arch": "x86_64", "name": "ModemManager", "release": "3.el9", "sigmd5": "c", "signature": "d", "type": "rpm", "version": "1.18.2"
					}]
				}`

				// create a new reader with that JSON
				r := io.NopCloser(bytes.NewReader([]byte(jsonResponse)))

				ImageBuilderHTTPClient = &MockClient{
					MockDo: func(*http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: 200,
							Body:       r,
						}, nil
					},
				}
				img, a := client.GetMetadata(img)

				Expect(a).ToNot(HaveOccurred())
				Expect(img).ToNot(BeNil())
				Expect(len(img.Commit.InstalledPackages)).To(Equal(3))
			})

			It("should create new packages into db", func() {
				pkgs := []models.Package{}
				img := &models.Image{Distribution: "rhel-8",
					Packages: pkgs,
					Commit: &models.Commit{
						OrgID:        faker.UUIDHyphenated(),
						Arch:         "x86_64",
						Repo:         &models.Repo{},
						ComposeJobID: faker.UUIDHyphenated(),
					}}

				// build our response JSON
				jsonResponse := `{
				"ostree_commit": "mock-repo",
				"packages": [
					{"arch": "x86_64", "name": "NetworkManager", "release": "3.el9", "sigmd5": "c", "signature": "d", "type": "rpm", "version": "1.18.2"
					},
					{"arch": "x86_64", "name": "ModemManager", "release": "3.el9", "sigmd5": "c", "signature": "d", "type": "rpm", "version": "1.18.2"
					}]
				}`

				// create a new reader with that JSON
				r := io.NopCloser(bytes.NewReader([]byte(jsonResponse)))

				ImageBuilderHTTPClient = &MockClient{
					MockDo: func(*http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: 200,
							Body:       r,
						}, nil
					},
				}

				img, a := client.GetMetadata(img)

				Expect(a).ToNot(HaveOccurred())
				Expect(img).ToNot(BeNil())
				Expect(len(img.Commit.InstalledPackages)).To(Equal(2))

			})

			It("should return an error ", func() {
				pkgs := []models.Package{}
				img := &models.Image{Distribution: "rhel-8",
					Packages: pkgs,
					Commit: &models.Commit{
						OrgID:        faker.UUIDHyphenated(),
						Arch:         "x86_64",
						Repo:         &models.Repo{},
						ComposeJobID: faker.UUIDHyphenated(),
					}}

				// build our response JSON
				jsonResponse := `{}`

				// create a new reader with that JSON
				r := io.NopCloser(bytes.NewReader([]byte(jsonResponse)))

				ImageBuilderHTTPClient = &MockClient{
					MockDo: func(*http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: 500,
							Body:       r,
						}, nil
					},
				}

				img, err := client.GetMetadata(img)
				Expect(img).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("image metadata not found"))
			})
		})
	})

	Describe("Validates Package", func() {
		AfterEach(func() {
			db.DB.Exec("ALTER TABLE installed_packages Create COLUMN name")
			os.Unsetenv("DEDUP_INSTALLED_PACKAGES")
		})
		It("should return an existent package ", func() {
			pkgs := []models.InstalledPackage{}
			pkgs = append(pkgs, models.InstalledPackage{Name: "rhc",
				Version: "1", Epoch: "1", Release: "1", Arch: "x86_64"})
			img := &models.Image{Distribution: "rhel-8",
				Commit: &models.Commit{
					Arch:              "x86_64",
					Repo:              &models.Repo{},
					ComposeJobID:      faker.UUIDHyphenated(),
					InstalledPackages: pkgs,
				}}
			db.DB.Save(img.Commit.InstalledPackages)
			db.DB.Save(img.Commit)
			db.DB.Save(img)

			var metadata Metadata
			var installedPackage InstalledPackage
			installedPackage.Name = "rhc"
			installedPackage.Version = "1"
			installedPackage.Release = "1"

			metadata.InstalledPackages = append(metadata.InstalledPackages, installedPackage)
			var metadataPackages []string
			for n := range metadata.InstalledPackages {
				metadataPackages = append(metadataPackages,
					fmt.Sprintf("%s-%s-%s", metadata.InstalledPackages[n].Name, metadata.InstalledPackages[n].Release, metadata.InstalledPackages[n].Version))
			}

			a, err := client.ValidatePackages(metadataPackages)
			Expect(err).ToNot(HaveOccurred())
			Expect(a).NotTo(BeNil())
			Expect(a["rhc"].Name).To(Equal("rhc"))
		})

		It("should return nil ", func() {

			var metadata Metadata
			var installedPackage InstalledPackage
			installedPackage.Name = "rhc"
			installedPackage.Version = "1"
			installedPackage.Release = "1"

			metadata.InstalledPackages = append(metadata.InstalledPackages, installedPackage)
			var metadataPackages []string
			for n := range metadata.InstalledPackages {
				metadataPackages = append(metadataPackages,
					fmt.Sprintf("%s-%s-%s", metadata.InstalledPackages[n].Name, metadata.InstalledPackages[n].Release, metadata.InstalledPackages[n].Version))
			}

			a, err := client.ValidatePackages(metadataPackages)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(a)).To(Equal(0))
		})

		It("should return error ", func() {
			db.DB.Exec("ALTER TABLE installed_packages DROP COLUMN name")
			a, err := client.ValidatePackages([]string{})
			Expect(err).ToNot(BeNil())
			Expect(a).To(BeNil())
		})
	})

	Describe("get metadata if disabled featureinformation", func() {

		Context("when pakages exists", func() {
			BeforeEach(func() {
				os.Unsetenv("DEDUP_INSTALLED_PACKAGES")
				pkgs := []models.InstalledPackage{}
				pkgs = append(pkgs, models.InstalledPackage{Name: "rhc",
					Version: "1",
					Epoch:   "1",
					Release: "1",
					Arch:    "x86_64"})
				img := &models.Image{Distribution: "rhel-8",
					Commit: &models.Commit{
						Arch:              "x86_64",
						Repo:              &models.Repo{},
						ComposeJobID:      faker.UUIDHyphenated(),
						InstalledPackages: pkgs,
					}}
				db.DB.Save(img.Commit.InstalledPackages)
				db.DB.Save(img.Commit)
				db.DB.Save(img)

			})
			AfterEach(func() {
				// disable the feature
				os.Unsetenv("DEDUP_INSTALLED_PACKAGES")
			})
			It("should duplicates packages RHC into db", func() {
				pkgs := []models.Package{}
				img := &models.Image{Distribution: "rhel-8",
					Packages: pkgs,
					Commit: &models.Commit{
						OrgID:        faker.UUIDHyphenated(),
						Arch:         "x86_64",
						Repo:         &models.Repo{},
						ComposeJobID: faker.UUIDHyphenated(),
					}}

				// build our response JSON
				jsonResponse := `{
				"ostree_commit": "mock-repo",
				"packages": [
					{"arch": "x86_64", "name": "rhc", "release": "1", "sigmd5": "a", "signature": "b", "type": "rpm", "version": "1", "epoch":"1"
					}, 
					{"arch": "x86_64", "name": "NetworkManager", "release": "3.el9", "sigmd5": "c", "signature": "d", "type": "rpm", "version": "1.18.2"
					},
					{"arch": "x86_64", "name": "ModemManager", "release": "3.el9", "sigmd5": "c", "signature": "d", "type": "rpm", "version": "1.18.2"
					}]
				}`

				// create a new reader with that JSON
				r := io.NopCloser(bytes.NewReader([]byte(jsonResponse)))

				ImageBuilderHTTPClient = &MockClient{
					MockDo: func(*http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: 200,
							Body:       r,
						}, nil
					},
				}

				img, a := client.GetMetadata(img)

				Expect(a).ToNot(HaveOccurred())
				Expect(img).ToNot(BeNil())
				Expect(len(img.Commit.InstalledPackages)).To(Equal(3))

			})
		})
	})

})
