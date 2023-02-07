// FIXME: golangci-lint
// nolint:revive,typecheck
package imagebuilder

import (
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

	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/redhatinsights/edge-api/config"
)

func TestModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Image Builder Client Suite")
}

var _ = Describe("Image Builder Client Test", func() {
	var client *Client
	var dbName string
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
	})
	AfterEach(func() {
		os.Remove(dbName)
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
	It("test get thirds party repo without orgId", func() {
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
})
