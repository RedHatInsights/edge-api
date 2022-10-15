package models

import (
	"testing"

	"github.com/magiconair/properties/assert"
	identity "github.com/redhatinsights/platform-go-middlewares/identity"
)

var edgeBasePayload = &EdgeBasePayload{
	RequestID: "1234567890",
	Identity: identity.XRHID{
		Identity: identity.Identity{
			AccountNumber: "1234567890"},
	},
}

func TestGetIdentity(t *testing.T) {
	ident := edgeBasePayload.GetIdentity()
	assert.Equal(t, ident.Identity.AccountNumber, "1234567890", "Identity is not 1234567890")
}

func TestGetRequestID(t *testing.T) {
	assert.Equal(t, edgeBasePayload.GetRequestID(), "1234567890", "requestID is not 1234567890")
}
