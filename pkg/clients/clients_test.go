// FIXME: golangci-lint
// nolint:revive
package clients_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/internal/testing"
	"github.com/redhatinsights/edge-api/pkg/clients"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
)

var _ = Describe("Clients", func() {

	Context("GetOutgoingHeaders", func() {
		var orgID string
		var ctx context.Context
		var originalAuth bool

		BeforeEach(func() {
			originalAuth = config.Get().Auth
			orgID = faker.UUIDHyphenated()
			ctx = context.Background()
			ctx = testing.WithRawIdentity(ctx, orgID)
		})

		AfterEach(func() {
			config.Get().Auth = originalAuth
		})

		It("when no auth should not get identity", func() {
			config.Get().Auth = false
			headers := clients.GetOutgoingHeaders(ctx)
			identity, ok := headers["x-rh-identity"]
			Expect(ok).To(BeFalse())
			Expect(identity).To(BeEmpty())
		})

		It("when auth should not get identity if not set", func() {
			ctx = context.Background()
			config.Get().Auth = true
			headers := clients.GetOutgoingHeaders(ctx)
			headerIdentity, ok := headers["x-rh-identity"]
			Expect(ok).To(BeFalse())
			Expect(headerIdentity).To(BeEmpty())
		})

		It("when auth should get identity if set", func() {
			config.Get().Auth = true
			headers := clients.GetOutgoingHeaders(ctx)
			headerIdentity, ok := headers["x-rh-identity"]
			Expect(ok).To(BeTrue())
			Expect(headerIdentity).ToNot(BeEmpty())
			var identity identity.XRHID
			err := json.Unmarshal([]byte(headerIdentity), &identity)
			Expect(err).ToNot(HaveOccurred())
			Expect(identity.Identity.OrgID).To(Equal(orgID))
		})
	})
	Context("ConfigureHttpClient", func() {
		var ctx context.Context
		var originalAuth bool
		var client http.Client
		var originalTLScaPATH string

		BeforeEach(func() {
			originalAuth = config.Get().Auth
			originalTLScaPATH = config.Get().TlsCAPath
			ctx = context.Background()
			ctx = testing.WithRawIdentity(ctx, faker.UUIDHyphenated())
		})

		AfterEach(func() {
			config.Get().Auth = originalAuth
			config.Get().TlsCAPath = originalTLScaPATH
		})

		It("should get client when TLS path is empty", func() {
			config.Get().HTTPClientTimeout = 30
			clientWithTLSPath := clients.ConfigureClientWithTLS(&client)
			Expect(clientWithTLSPath.Timeout).To(Equal(30 * time.Second))
		})
		It("should get client when TLS path doesnt exist", func() {
			config.Get().TlsCAPath = "/test_TLS"
			clientWithTLSPath := clients.ConfigureClientWithTLS(&client)
			Expect(clientWithTLSPath.Transport).To(BeNil())

		})
		It("should get client when TLS path that exist", func() {
			file, err := os.CreateTemp("", "*-tls_file.txt")
			Expect(err).To(BeNil())
			config.Get().TlsCAPath = file.Name()
			clientWithTLSPath := clients.ConfigureClientWithTLS(&client)
			Expect(clientWithTLSPath.Transport).ToNot(BeNil())
			Expect(clientWithTLSPath.Timeout, 30*time.Second)
			transport, ok := client.Transport.(*http.Transport)
			Expect(ok).To(BeTrue())
			Expect(transport.TLSClientConfig).ToNot(BeNil())
			Expect(transport.TLSClientConfig.RootCAs).ToNot(BeNil())
		})
	})
})
