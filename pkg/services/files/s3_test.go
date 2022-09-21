// FIXME: golangci-lint
// nolint:revive
package files_test

import (
	"os"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/services/files"
)

var _ = Describe("Uploader Test", func() {

	Describe("Debug True", func() {
		var initialDebug bool
		var initialAccessKey string
		var initialSecretKey string
		var cfg *config.EdgeConfig
		BeforeEach(func() {
			cfg = config.Get()
			initialDebug = cfg.Debug
			cfg.Debug = true
			initialAccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
			initialSecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
		})
		AfterEach(func() {
			cfg.Local = initialDebug
			os.Setenv("AWS_ACCESS_KEY_ID", initialAccessKey)
			os.Setenv("AWS_SECRET_ACCESS_KEY", initialSecretKey)
		})
		It("Get new s3 session successfully", func() {
			accessKey := faker.UUIDHyphenated()
			accessID := faker.UUIDHyphenated()

			os.Setenv("AWS_ACCESS_KEY_ID", accessID)
			os.Setenv("AWS_SECRET_ACCESS_KEY", accessKey)

			session := files.GetNewS3Session()
			Expect(session).ToNot(BeNil())
			credentials, err := session.Config.Credentials.Get()
			Expect(err).To(BeNil())
			Expect(credentials.AccessKeyID).To(Equal(accessID))
			Expect(credentials.SecretAccessKey).To(Equal(accessKey))
			Expect(credentials.ProviderName).To(Equal("EnvConfigCredentials"))
		})
	})

	Describe("Debug False", func() {
		var initialDebug bool
		var initialAccessKey string
		var initialSecretKey string

		var cfg *config.EdgeConfig

		BeforeEach(func() {
			cfg = config.Get()
			initialDebug = cfg.Debug
			initialAccessKey = cfg.AccessKey
			initialSecretKey = cfg.SecretKey
			cfg.Debug = false
			cfg.AccessKey = faker.UUIDHyphenated()
			cfg.SecretKey = faker.UUIDHyphenated()
		})
		AfterEach(func() {
			cfg.Local = initialDebug
			cfg.AccessKey = initialAccessKey
			cfg.SecretKey = initialSecretKey
		})

		It("Get new s3 session successfully", func() {
			session := files.GetNewS3Session()
			Expect(session).ToNot(BeNil())
			credentials, err := session.Config.Credentials.Get()
			Expect(err).To(BeNil())
			Expect(credentials.AccessKeyID).To(Equal(cfg.AccessKey))
			Expect(credentials.SecretAccessKey).To(Equal(cfg.SecretKey))
			Expect(credentials.ProviderName).To(Equal("StaticProvider"))
		})
	})
})
