// FIXME: golangci-lint
// nolint:revive
package common

import (
	"context"
	"errors"
)

type rhIdentityKeyType string

// rhIdentityKey is the context key for x-rh-identity
const rhIdentityKey = rhIdentityKeyType("xRhIdentity")

// GetOriginalIdentity get the original identity data from context
func GetOriginalIdentity(ctx context.Context) (string, error) {
	ident, ok := ctx.Value(rhIdentityKey).(string)
	if !ok {
		return "", errors.New("no identity found")
	}
	return ident, nil
}

// SetOriginalIdentity set the original identity data to the context
func SetOriginalIdentity(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, rhIdentityKey, value)
}
