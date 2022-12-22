// nolint:govet,revive,typecheck
package common

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/magiconair/properties/assert"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

func TestGetIdentity(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	getIdentity, error := GetIdentity(req)
	assert.Equal(t, getIdentity, DefaultAccount)
	assert.Equal(t, error, nil)
}

func TestGetDefaultIdentity(t *testing.T) {
	cfg := config.Get()
	auth := cfg.Auth

	// Reset config.Auth back to its original value
	defer func(auth bool) {
		config.Get().Auth = auth
	}(auth)

	ctx := context.Background()
	orgID := faker.UUIDHyphenated()

	cases := []struct {
		Name             string
		Context          context.Context
		Auth             bool
		ExpectedIdentity string
		ExpectedError    error
	}{
		{
			Name:             "Auth is false",
			Context:          ctx,
			Auth:             false,
			ExpectedIdentity: DefaultUserName,
			ExpectedError:    nil,
		},
		{
			Name:             "Cannot get Identity from Context",
			Context:          context.WithValue(ctx, identity.Key, nil),
			Auth:             true,
			ExpectedIdentity: "",
			ExpectedError:    errors.New("cannot find identity"),
		},
		{
			Name: "Get Identity from Context",
			Context: context.WithValue(ctx, identity.Key, identity.XRHID{Identity: identity.Identity{
				AccountNumber: faker.UUIDHyphenated(),
				OrgID:         orgID,
			}}),
			Auth:             true,
			ExpectedIdentity: identity.GetIdentityHeader(ctx),
			ExpectedError:    nil,
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			// Save current config.Auth
			cfg.Auth = test.Auth
			getIdentity, error := GetIdentityFromContext(test.Context)
			assert.Equal(t, getIdentity.Identity.User.Username, test.ExpectedIdentity)
			assert.Equal(t, error, test.ExpectedError)
		})
	}
}
