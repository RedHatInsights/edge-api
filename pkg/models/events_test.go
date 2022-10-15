package models

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
	Identity:       identity.XRHID{Identity: identity.Identity{OrgID: common.DefaultOrgID}},
	LastHandleTime: time.Date(2015, 10, 21, 9, 00, 42, 651387237, time.UTC).Format(time.RFC3339),
	RequestID:      faker.UUIDDigit(),
}

// TestGetIdentity compares the field returned from the payload with the expected result
func TestGetIdentity(t *testing.T) {
	ident := edgeBasePayload.GetIdentity()
	assert.Equal(t, ident.Identity.OrgID, edgeBasePayload.Identity.Identity.OrgID, "OrgID does not match")
}

// TestGetRequestID compares the field returned from the payload with the expected result
func TestGetRequestID(t *testing.T) {
	assert.Equal(t, edgeBasePayload.GetRequestID(), edgeBasePayload.RequestID, "RequestID does not match")
}
