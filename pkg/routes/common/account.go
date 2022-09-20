// FIXME: golangci-lint
// nolint:revive
package common

import (
	"context"
	"fmt"
	"net/http"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

const (
	// DefaultAccount that will return on tests and on debug/local mode
	DefaultAccount = "0000000"
)

// GetAccount from http request header
func GetAccount(r *http.Request) (string, error) {
	return GetAccountFromContext(r.Context())
}

// GetAccountFromContext determines account number from supplied context
func GetAccountFromContext(ctx context.Context) (string, error) {
	if config.Get() != nil {
		if !config.Get().Auth {
			return DefaultAccount, nil
		}
		if ctx.Value(identity.Key) != nil {
			ident := identity.Get(ctx)
			if ident.Identity.AccountNumber != "" {
				return ident.Identity.AccountNumber, nil
			}
		}
	}
	return "", fmt.Errorf("cannot find account number")
}
