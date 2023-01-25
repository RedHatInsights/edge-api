package clients_test

import (
	"context"
	"encoding/json"

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
})
