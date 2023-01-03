// nolint:govet,revive,typecheck
package common

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/magiconair/properties/assert"
	"github.com/redhatinsights/platform-go-middlewares/identity"

	"github.com/redhatinsights/edge-api/config"
)

func TestGetAccount(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	getAccount, err := GetAccount(req)
	assert.Equal(t, getAccount, DefaultAccount)
	assert.Equal(t, err, nil)
}

func TestGetDefaultAccount(t *testing.T) {
	cfg := config.Get()
	auth := cfg.Auth
	ctx := context.Background()
	account := faker.UUIDHyphenated()

	defer func(auth bool) {
		config.Get().Auth = auth
	}(auth)

	cases := []struct {
		Name            string
		Context         context.Context
		Auth            bool
		ExpectedAccount string
		ExpectedError   error
	}{
		{
			Name:            "Auth is false",
			Context:         ctx,
			Auth:            false,
			ExpectedAccount: DefaultAccount,
			ExpectedError:   nil,
		},
		{
			Name:            "Cannot get account from Context",
			Context:         context.WithValue(ctx, identity.Key, nil),
			Auth:            true,
			ExpectedAccount: "",
			ExpectedError:   errors.New("cannot find account number"),
		},
		{
			Name: "Get account from Context",
			Context: context.WithValue(ctx, identity.Key, identity.XRHID{Identity: identity.Identity{
				OrgID:         faker.UUIDHyphenated(),
				AccountNumber: account,
			}}),
			Auth:            true,
			ExpectedAccount: account,
			ExpectedError:   nil,
		},
		{
			Name: "Blank account from Context",
			Context: context.WithValue(ctx, identity.Key, identity.XRHID{Identity: identity.Identity{
				OrgID:         faker.UUIDHyphenated(),
				AccountNumber: "",
			}}),
			Auth:            true,
			ExpectedAccount: "",
			ExpectedError:   errors.New("cannot find account number"),
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			cfg.Auth = test.Auth
			getAccount, err := GetAccountFromContext(test.Context)
			assert.Equal(t, getAccount, test.ExpectedAccount)
			assert.Equal(t, err, test.ExpectedError)
		})
	}
}
