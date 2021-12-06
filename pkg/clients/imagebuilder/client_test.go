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
})
