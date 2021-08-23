package common

import (
	"fmt"
	"net/http"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

// GetAccount from http request header
func GetAccount(r *http.Request) (string, error) {
	if config.Get() != nil {
		if !config.Get().Auth {
			return "0000000", nil
		}
		if r.Context().Value(identity.Key) != nil {
			ident := identity.Get(r.Context())
			if ident.Identity.AccountNumber != "" {
				return ident.Identity.AccountNumber, nil
			}
		}
	}
	return "", fmt.Errorf("cannot find account number")
}
