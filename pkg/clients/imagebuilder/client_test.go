package imagebuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
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
	})
	It("test compose image when ostree parent commit is empty", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			b, err := ioutil.ReadAll(r.Body)
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
