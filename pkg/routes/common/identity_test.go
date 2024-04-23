// FIXME: golangci-lint
// nolint:govet,revive
package common

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
)

func TestGetIdentityInstanceFromContext(t *testing.T) {
	id := identity.XRHID{Identity: identity.Identity{OrgID: DefaultOrgID}}
	identityBytes, _ := json.Marshal(id) // nolint:errcheck,gofmt,goimports
	base64Identity := base64.StdEncoding.EncodeToString(identityBytes)
	illegalFirstByte := "X="

	cases := []struct {
		Name          string
		Context       context.Context
		ExpectedOrgID string
		ExpectedError error
	}{
		{
			Name:          "No identity found from context",
			Context:       context.Background(),
			ExpectedOrgID: "",
			ExpectedError: errors.New("no identity found"),
		},
		{
			Name:          "Cannot decode identity from context",
			Context:       identity.WithRawIdentity(context.Background(), illegalFirstByte),
			ExpectedOrgID: "",
			ExpectedError: base64.CorruptInputError(1),
		},
		{
			Name:          "Cannot unmarshal identity from context",
			Context:       identity.WithRawIdentity(context.Background(), base64.StdEncoding.EncodeToString([]byte("{\"bb\""))),
			ExpectedOrgID: "",
			ExpectedError: &json.SyntaxError{},
		},

		{
			Name:          "Find identity instance from context",
			Context:       identity.WithRawIdentity(context.Background(), base64Identity),
			ExpectedOrgID: DefaultOrgID,
			ExpectedError: nil,
		},
	}

	var tt *json.SyntaxError
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			ident, err := GetIdentityInstanceFromContext(test.Context)
			assert.Equal(t, ident.Identity.OrgID, test.ExpectedOrgID)
			if err != nil {
				t.Log(err.Error())
			}
			if errors.As(err, &tt) {
				// It's not possible to create json.SyntaxError directly
				// https://stackoverflow.com/questions/71768824/how-to-handle-json-syntax-error-in-a-go-test-case
				assert.Equal(t, err.Error(), "unexpected end of JSON input")
			} else {
				assert.Equal(t, err, test.ExpectedError)
			}

		})
	}
}
