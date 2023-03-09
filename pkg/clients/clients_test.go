package clients_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients"
	"github.com/redhatinsights/edge-api/pkg/routes/common"

	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Clients", func() {

	Context("GetOutgoingHeaders", func() {
		var orgID string
		var requestID string
		var ctx context.Context
		var originalAuth bool

		BeforeEach(func() {
			originalAuth = config.Get().Auth
			orgID = faker.UUIDHyphenated()
			requestID = faker.UUIDHyphenated()
			ctx = context.Background()
			ctx = context.WithValue(ctx, request_id.RequestIDKey, requestID)
			content, err := json.Marshal(&identity.XRHID{Identity: identity.Identity{OrgID: orgID}})
			Expect(err).ToNot(HaveOccurred())
			ctx = common.SetOriginalIdentity(ctx, string(content))
		})

		AfterEach(func() {
			config.Get().Auth = originalAuth
		})

		It("should get requestID", func() {
			headers := clients.GetOutgoingHeaders(ctx)
			headerRequestID, ok := headers["x-rh-insights-request-id"]
			Expect(ok).To(BeTrue())
			Expect(headerRequestID).To(Equal(requestID))
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
		var orgID string
		var requestID string
		var ctx context.Context
		var originalAuth bool
		var client http.Client
		var originalTLScaPATH string

		BeforeEach(func() {
			originalAuth = config.Get().Auth
			orgID = faker.UUIDHyphenated()
			originalTLScaPATH = config.Get().TlsCAPath
			requestID = faker.UUIDHyphenated()
			ctx = context.Background()
			ctx = context.WithValue(ctx, request_id.RequestIDKey, requestID)
			content, err := json.Marshal(&identity.XRHID{Identity: identity.Identity{OrgID: orgID}})
			Expect(err).ToNot(HaveOccurred())
			ctx = common.SetOriginalIdentity(ctx, string(content))

		})

		AfterEach(func() {
			config.Get().Auth = originalAuth
			config.Get().TlsCAPath = originalTLScaPATH
		})

		It("should get client when TLS path is empty", func() {
			config.Get().HTTPClientTimeout = 30
			clientWithTLSPath := clients.ConfigureHttpClient(&client)
			Expect(clientWithTLSPath.Timeout).To(Equal(30 * time.Second))
		})
		It("should get client when TLS path doesnt exist", func() {
			config.Get().TlsCAPath = "/test_TLS"
			clientWithTLSPath := clients.ConfigureHttpClient(&client)
			Expect(clientWithTLSPath.Transport).To(BeNil())

		})
		It("should get client when TLS path that exist", func() {
			file, err := os.CreateTemp("", "*-tls_file.txt")
			Expect(err).To(BeNil())
			config.Get().TlsCAPath = file.Name()
			clientWithTLSPath := clients.ConfigureHttpClient(&client)
			Expect(clientWithTLSPath.Transport).ToNot(BeNil())
			Expect(clientWithTLSPath.Timeout, 30*time.Second)
			transport, ok := client.Transport.(*http.Transport)
			Expect(ok).To(BeTrue())
			Expect(transport.TLSClientConfig).ToNot(BeNil())
			Expect(transport.TLSClientConfig.RootCAs).ToNot(BeNil())
		})
	})
})
