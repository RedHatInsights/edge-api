package models

import (
	"errors"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/stretchr/testify/assert"
)

func TestValidateRepoURL(t *testing.T) {
	cases := []struct {
		Name     string
		Input    string
		Expected bool
	}{
		{"Valid URL", faker.URL(), true},
		{"Invalid URL", faker.UUIDHyphenated(), false},
	}
	for _, test := range cases {
		assert.Equal(t, test.Expected, ValidateRepoURL(test.Input))
	}
}

func TestValidateRequestFunction(t *testing.T) {
	cases := []struct {
		Name     string
		Input    ThirdPartyRepo
		Expected error
	}{
		{
			"Missing name",
			ThirdPartyRepo{
				Name: "",
				URL:  "http://localhost",
			},
			errors.New(RepoNameCantBeNilMessage),
		},
		{
			"Invalid name",
			ThirdPartyRepo{
				Name: " Invalid Name",
				URL:  "http://localhost",
			},
			errors.New(RepoNameCantBeInvalidMessage),
		},
		{
			"Missing URL",
			ThirdPartyRepo{
				Name: faker.UUIDHyphenated(),
				URL:  "",
			},
			errors.New(RepoURLCantBeNilMessage),
		},
		{
			"Invalid URL",
			ThirdPartyRepo{
				Name: faker.UUIDHyphenated(),
				URL:  faker.UUIDHyphenated(),
			},
			errors.New(InvalidURL),
		},
		{
			"Valid Repository",
			ThirdPartyRepo{
				Name: faker.UUIDHyphenated(),
				URL:  faker.URL(),
			},
			nil,
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			got := test.Input.ValidateRequest()
			assert.Equal(t, test.Expected, got)
		})
	}
}

func TestThirdPartyRepoBeforeCreate(t *testing.T) {
	cases := []struct {
		Name     string
		Input    ThirdPartyRepo
		Expected error
	}{
		{
			"Missing orgID",
			ThirdPartyRepo{
				Name: faker.UUIDHyphenated(),
				URL:  faker.URL(),
			},
			ErrOrgIDIsMandatory,
		},
		{
			"Has orgID",
			ThirdPartyRepo{
				Name:  faker.UUIDHyphenated(),
				URL:   faker.URL(),
				OrgID: faker.UUIDHyphenated(),
			},
			nil,
		},
	}

	for _, test := range cases {
		got := test.Input.BeforeCreate(db.DB)
		assert.Equal(t, test.Expected, got)
	}

}
