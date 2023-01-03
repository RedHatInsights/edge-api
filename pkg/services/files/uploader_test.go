// FIXME: golangci-lint
// nolint:errcheck,revive,typecheck
package files_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/services/files"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
)

var _ = Describe("Uploader Test", func() {
	var logEntry *log.Entry
	var account string
	var acl = "private"
	Describe("local uploader", func() {
		var uploader files.Uploader
		BeforeEach(func() {
			logEntry = log.NewEntry(log.StandardLogger())
			config.Init()
			cfg := config.Get()
			cfg.Local = true
			account = "000000"
			uploader = files.NewUploader(logEntry)
		})
		When("uploader is created", func() {
			It("return uploader", func() {
				Expect(uploader).To(Not(BeNil()))
			})
		})
		When("upload repo is called", func() {
			It("returns src and does nothing", func() {
				src := "/tmp/tmp-repo"
				uploadPath, err := uploader.UploadRepo(src, account, acl)
				Expect(err).ToNot(HaveOccurred())
				Expect(uploadPath).To(Equal(src))
			})
		})
		When("upload file is called", func() {
			It("returns src and does nothing", func() {
				src := "/tmp/tmp-repo"
				uploadPath, err := uploader.UploadRepo(src, account, acl)
				Expect(err).ToNot(HaveOccurred())
				Expect(uploadPath).To(Equal(src))
			})
		})
		When("base folder is invalid", func() {
			It("returns error", func() {
				src := "/invalid-base-folder/tmp-repo"
				uploadPath, err := uploader.UploadRepo(src, account, acl)
				Expect(err).To(HaveOccurred())
				Expect(uploadPath).To(Equal(""))
			})
		})
		When("upload file", func() {
			var path, filename string
			BeforeEach(func() {
				filename = "random-filename.txt"
				path = "/tmp"
				path = fmt.Sprintf("%s/%s", path, filename)
				f, err := os.Create(path)
				if err != nil {
					log.Fatal(err)
				}
				defer f.Close()
				os.Create(path)
			})
			AfterEach(func() {
				os.Remove(path)
			})
			It("doesnt returns error", func() {
				destfile := "random-file.txt"
				newFilePath, err := uploader.UploadFile(path, destfile)

				Expect(newFilePath).To(Equal(fmt.Sprintf("/tmp/%s", destfile)))
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
