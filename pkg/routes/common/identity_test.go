// FIXME: golangci-lint
// nolint:govet,revive
package common

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

func TestGetIdentityInstanceFromContext(t *testing.T) {
	identity := identity.XRHID{Identity: identity.Identity{OrgID: DefaultOrgID}}
	identityBytes, _ := json.Marshal(identity)
	base64Identity := base64.StdEncoding.EncodeToString(identityBytes)
	ctx := SetOriginalIdentity(context.Background(), base64Identity)

	ident, _ := GetIdentityInstanceFromContext(ctx)
	assert.Equal(t, ident.Identity.OrgID, DefaultOrgID, "OrgIDs do not match. Identity not decoded correctly")
}
