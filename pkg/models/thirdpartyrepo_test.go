// nolint:govet,revive,typecheck
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

func TestCreateCSThirdparty(t *testing.T) {

	csRepo := ThirdPartyRepo{
		Name:                "csRepo",
		URL:                 faker.URL(),
		UUID:                faker.UUIDHyphenated(),
		DistributionVersion: &[]string{"Any"},
		DistributionArch:    "any",
		GpgKey:              "any",
		PackageCount:        1,
		OrgID:               faker.ID,
	}

	err := db.DB.Create(&csRepo)
	assert.Equal(t, err.Error, nil)
	assert.NotEmpty(t, csRepo.UUID)
	assert.NotEmpty(t, csRepo.DistributionArch)
	assert.NotEmpty(t, csRepo.GpgKey)
	assert.NotEmpty(t, csRepo.PackageCount)

	assert.NotEmpty(t, csRepo.DistributionVersion)

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

func TestAddSlashToURL(t *testing.T) {
	testCases := []struct {
		Name        string
		URL         string
		ExpectedURL string
	}{
		{
			Name:        "should remove trailing white spaces and add slash",
			URL:         " http://example.com.repo  ",
			ExpectedURL: "http://example.com.repo/",
		},
		{
			Name:        "should remove only trailing white spaces",
			URL:         " http://example.com.repo/  ",
			ExpectedURL: "http://example.com.repo/",
		},
		{
			Name:        "should not change the URL",
			URL:         "http://example.com.repo/",
			ExpectedURL: "http://example.com.repo/",
		},
		{
			Name:        "should not change the URL when empty",
			URL:         "",
			ExpectedURL: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			url := AddSlashToURL(testCase.URL)
			assert.Equal(t, testCase.ExpectedURL, url)
		})
	}
}

func TestCreateThirdPartyRepoWithURL(t *testing.T) {
	// should clean up url and add a slash "/" when creating repo
	url := " http://example.com.repo  \t"
	wantedURL := "http://example.com.repo/"
	repo := ThirdPartyRepo{
		Name:  faker.Name(),
		OrgID: faker.UUIDHyphenated(),
		URL:   url,
	}
	err := db.DB.Create(&repo).Error
	assert.NoError(t, err)
	assert.Equal(t, wantedURL, repo.URL)
}

func TestUpdateThirdPartyRepoWithURL(t *testing.T) {
	// should clean up url and add a slash "/" when updating repo
	repo := ThirdPartyRepo{
		Name:  faker.Name(),
		OrgID: faker.UUIDHyphenated(),
		URL:   faker.URL(),
	}
	err := db.DB.Create(&repo).Error
	assert.NoError(t, err)

	url := " http://example.com.repo  \t"
	wantedURL := "http://example.com.repo/"
	repo.URL = url
	err = db.DB.Save(&repo).Error
	assert.NoError(t, err)
	assert.Equal(t, wantedURL, repo.URL)
}
