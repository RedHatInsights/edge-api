// FIXME: golangci-lint
// nolint:revive,typecheck
package files

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/config"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Uploader Test", func() {
	var logEntry *log.Entry

	Describe("local downloader", func() {
		var downloader Downloader
		BeforeEach(func() {
			logEntry = log.NewEntry(log.StandardLogger())
			config.Init()
			cfg := config.Get()
			cfg.Local = true
			downloader = NewDownloader(logEntry)
		})
		When("uploader is created", func() {
			It("return uploader", func() {
				Expect(downloader).To(Not(BeNil()))
				Expect(downloader).To(Equal(&HTTPDownloader{log: logEntry}))
			})
		})

	})

	Describe("s3 downloader", func() {
		var downloader Downloader
		BeforeEach(func() {
			logEntry = log.NewEntry(log.StandardLogger())
			config.Init()
			cfg := config.Get()
			cfg.Local = false
			downloader = NewDownloader(logEntry)
		})
		When("uploader is created", func() {
			It("return uploader", func() {
				Expect(downloader).To(Not(BeNil()))
				Expect(downloader).To(Equal(&S3Downloader{log: logEntry}))
			})
		})
	})
})
