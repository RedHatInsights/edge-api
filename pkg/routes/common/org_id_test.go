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

func TestGetOrgID(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	gotOrgID, gotError := GetOrgID(req)
	assert.Equal(t, gotOrgID, DefaultOrgID)
	assert.Equal(t, gotError, nil)
}

func TestGetDefaultOrgID(t *testing.T) {
	cfg := config.Get()
	auth := cfg.Auth

	// Reset config.Auth back to its original value
	defer func(auth bool) {
        config.Get().Auth = auth
    }(auth)

	ctx := context.Background()
	orgID := faker.UUIDHyphenated()

	cases := []struct {
		Name          string
		Context       context.Context
		Auth          bool
		ExpectedOrgID string
		ExpectedError error
	}{
		{
			Name:          "Auth is false",
			Context:       ctx,
			Auth:          false,
			ExpectedOrgID: DefaultOrgID,
			ExpectedError: nil,
		},
		{
			Name:          "Cannot get orgID from Context",
			Context:       context.WithValue(ctx, identity.Key, nil),
			Auth:          true,
			ExpectedOrgID: "",
			ExpectedError: errors.New("cannot find org-id"),
		},
		{
			Name: "Get orgID from Context",
			Context: context.WithValue(ctx, identity.Key, identity.XRHID{Identity: identity.Identity{
				AccountNumber: faker.UUIDHyphenated(),
				OrgID:         orgID,
			}}),
			Auth:          true,
			ExpectedOrgID: orgID,
			ExpectedError: nil,
		},
		{
			Name: "Blank orgID from Context",
			Context: context.WithValue(ctx, identity.Key, identity.XRHID{Identity: identity.Identity{
				AccountNumber: faker.UUIDHyphenated(),
				OrgID:         "",
			}}),
			Auth:          true,
			ExpectedOrgID: "",
			ExpectedError: errors.New("cannot find org-id"),
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			// Save current config.Auth
			cfg.Auth = test.Auth
			gotOrgID, gotError := GetOrgIDFromContext(test.Context)
			assert.Equal(t, gotOrgID, test.ExpectedOrgID)
			assert.Equal(t, gotError, test.ExpectedError)
		})
	}
}
