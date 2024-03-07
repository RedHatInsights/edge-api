// FIXME: golangci-lint
// nolint:revive
package common

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
)

// IdentityTypeUser represent the user identity type
const IdentityTypeUser = "User"

// GetOriginalIdentity get the original identity data from context
func GetOriginalIdentity(ctx context.Context) (string, error) {
	ident := identity.GetRawIdentity(ctx)
	if ident == "" {
		return "", errors.New("no identity found")
	}
	return ident, nil
}

// SetOriginalIdentity set the original identity data to the context
func SetOriginalIdentity(ctx context.Context, value string) context.Context {
	return identity.WithRawIdentity(ctx, value)
}

// GetIdentityInstanceFromContext returns an instances of identity.XRHID from Base64 encoded ident in context
func GetIdentityInstanceFromContext(ctx context.Context) (identity.XRHID, error) {
	ident64, err := GetOriginalIdentity(ctx)
	if err != nil {
		return identity.XRHID{}, err
	}

	identBytes, err := base64.StdEncoding.DecodeString(ident64)
	if err != nil {
		return identity.XRHID{}, err
	}

	var ident identity.XRHID
	err = json.Unmarshal(identBytes, &ident)
	if err != nil {
		return identity.XRHID{}, err
	}

	return ident, nil
}

func GetParsedIdentityPrincipal(ctx context.Context) string {
	if config.Get() == nil {
		return ""
	}
	if !config.Get().Auth {
		return DefaultPrincipal
	}
	id, err := GetIdentityInstanceFromContext(ctx)
	if err != nil {
		return ""
	}
	switch id.Identity.Type {
	case "User":
		return id.Identity.User.Username
	case "ServiceAccount":
		return id.Identity.ServiceAccount.Username
	case "System":
		return id.Identity.System.CommonName
	}
	return ""
}
