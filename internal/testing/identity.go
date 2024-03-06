package testing

import (
	"context"
	"encoding/json"

	"github.com/redhatinsights/edge-api/pkg/routes/common"
	rhidentity "github.com/redhatinsights/platform-go-middlewares/v2/identity"
	"go.openly.dev/pointy"
)

func WithIdentity(ctx context.Context) context.Context {
	return rhidentity.WithIdentity(ctx, newIdentity(common.DefaultOrgID, pointy.Pointer(common.DefaultAccount), "User"))
}

func WithCustomIdentity(ctx context.Context, orgID string) context.Context {
	return rhidentity.WithIdentity(ctx, newIdentity(orgID, pointy.Pointer(common.DefaultAccount), "User"))
}

func WithCustomIdentityType(ctx context.Context, orgID string, identityType string) context.Context {
	id := newIdentity(orgID, pointy.Pointer(common.DefaultAccount), identityType)
	return rhidentity.WithIdentity(ctx, id)
}

func WithRawIdentity(ctx context.Context, orgID string) context.Context {
	id := newIdentity(orgID, pointy.Pointer(common.DefaultAccount), "User")
	rawID, err := json.Marshal(id)
	if err != nil {
		rawID = []byte{}
	}
	return rhidentity.WithRawIdentity(ctx, string(rawID))
}

func newIdentity(orgID string, accountNumber *string, identityType string) rhidentity.XRHID {
	id := rhidentity.XRHID{
		Identity: rhidentity.Identity{
			OrgID: orgID,
			Type:  identityType,
			User: &rhidentity.User{
				Username: "TestUser",
			},
		},
	}
	if accountNumber != nil {
		id.Identity.AccountNumber = *accountNumber
	}
	return id
}
