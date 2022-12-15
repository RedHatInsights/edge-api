// FIXME: golangci-lint
// nolint:gocritic,revive
package common

import (
	"context"
	"errors"
	"net/http"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

const (
	// DefaultOrgID that will return on tests and on debug/local mode
	DefaultOrgID = "00000000"
)

// GetOrgID return org-id from http request identity header
func GetOrgID(r *http.Request) (string, error) {
	return GetOrgIDFromContext(r.Context())
}

// GetOrgIDFromContext return org-id number from supplied context
func GetOrgIDFromContext(ctx context.Context) (string, error) {
	conf := config.Get()
	if conf == nil {
		return "", errors.New("conf not initialized")
	}
	if !conf.Auth {
		return DefaultOrgID, nil
	}
	if ctx.Value(identity.Key) != nil {
		ident := identity.Get(ctx)
		if ident.Identity.OrgID != "" {
			return ident.Identity.OrgID, nil
		}
	}

	return "", errors.New("cannot find org-id")
}
