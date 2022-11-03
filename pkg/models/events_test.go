package models	// nolint:gofmt,goimports,revive

import (
	"testing"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/magiconair/properties/assert"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	identity "github.com/redhatinsights/platform-go-middlewares/identity"
)

// Setup the test payload
var edgeBasePayload = &EdgeBasePayload{
	Identity:       identity.XRHID{Identity: identity.Identity{OrgID: common.DefaultOrgID}},	// nolint:gofmt,goimports,revive
	LastHandleTime: time.Date(2015, 10, 21, 9, 00, 42, 651387237, time.UTC).Format(time.RFC3339),	// nolint:revive
	RequestID:      faker.UUIDDigit(),	// nolint:typecheck
}

// nolint:revive // TestGetIdentity compares the field returned from the payload with the expected result
func TestGetIdentity(t *testing.T) {
	ident := edgeBasePayload.GetIdentity()
	assert.Equal(t, ident.Identity.OrgID, edgeBasePayload.Identity.Identity.OrgID, "OrgID does not match")	// nolint:gofmt,goimports,revive
}

// nolint:revive // TestGetRequestID compares the field returned from the payload with the expected result
func TestGetRequestID(t *testing.T) {
	assert.Equal(t, edgeBasePayload.GetRequestID(), edgeBasePayload.RequestID, "RequestID does not match")	// nolint:gofmt,goimports,revive
}
