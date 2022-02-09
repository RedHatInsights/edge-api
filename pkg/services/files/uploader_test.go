package files_test

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/services/files"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Uploader Test", func() {
	var logEntry *log.Entry
	var account string
	Describe("local uploader", func() {
		var uploader files.Uploader
		BeforeEach(func() {
			logEntry = log.NewEntry(log.StandardLogger())
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
				uploadPath, err := uploader.UploadRepo(src, account)
				Expect(err).ToNot(HaveOccurred())
				Expect(uploadPath).To(Equal(src))
			})
		})
		When("upload file is called", func() {
			It("returns src and does nothing", func() {
				src := "/tmp/tmp-repo"
				uploadPath, err := uploader.UploadRepo(src, account)
				Expect(err).ToNot(HaveOccurred())
				Expect(uploadPath).To(Equal(src))
			})
		})
		When("base folder is invalid", func() {
			It("returns error", func() {
				src := "/invalid-base-folder/tmp-repo"
				uploadPath, err := uploader.UploadRepo(src, account)
				Expect(err).To(HaveOccurred())
				Expect(uploadPath).To(Equal(""))
			})
		})
		When("upload file", func() {
			var path, filename, data string
			BeforeEach(func() {
				data = "i am a file data"
				filename = "random-filename.txt"
				path = fmt.Sprintf("/tmp/random-folder/%s", filename)
				f, err := os.Create(path)
				if err != nil {
					log.Fatal(err)
				}
				defer f.Close()
				ioutil.WriteFile(path, []byte(data), fs.ModeAppend)
			})
			AfterEach(func() {
				os.Remove(path)
			})
			It("returns error", func() {
				_, filename := filepath.Split(path)
				newFilePath, err := uploader.UploadFile(path, filename)

				Expect(err).ToNot(HaveOccurred())
				Expect(newFilePath).To(Equal(fmt.Sprintf("/tmp/%s", filename)))
			})
		})
	})
})
