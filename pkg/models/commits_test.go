// nolint:govet,revive,typecheck
package models

import (
	"os"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/stretchr/testify/assert"
)

func TestCommitsBeforeCreate(t *testing.T) {
	cases := []struct {
		Name     string
		Input    Commit
		Expected error
	}{
		{
			"Missing orgID",
			Commit{},
			ErrOrgIDIsMandatory,
		},
		{
			"Can be created",
			Commit{OrgID: faker.UUIDHyphenated()},
			nil,
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			got := test.Input.BeforeCreate(db.DB)
			assert.Equal(t, test.Expected, got)
		})
	}
}

func TestDistributionURL(t *testing.T) {
	var awsURL = "https://aws.repo.example.com/repo/is/here"
	var pulpURL = "https://pulp.distribution.example.com/api/pulp-content/pulp/repo/is/here"
	repo := Repo{
		URL:        awsURL,
		Status:     RepoStatusSuccess,
		PulpURL:    pulpURL,
		PulpStatus: RepoStatusSuccess,
	}

	t.Run("return AWS URL", func(t *testing.T) {
		defer config.Cleanup()

		os.Unsetenv("FEATURE_PULP_INTEGRATION")
		os.Unsetenv("PULP_CONTENT_URL")

		assert.Equal(t, awsURL, repo.DistributionURL())
	})

	t.Run("return pulp distribution url", func(t *testing.T) {
		defer config.Cleanup()

		os.Setenv("FEATURE_PULP_INTEGRATION", "true")
		os.Setenv("PULP_CONTENT_URL", "http://internal.repo.example.com:8080")

		assert.Equal(t, pulpURL, repo.DistributionURL())
	})
}

func TestContentURL(t *testing.T) {
	var awsURL = "https://aws.repo.example.com/repo/is/here"
	var pulpURL = "https://pulp.distribution.example.com:3030/api/pulp-content/pulp/repo/is/here"
	repo := Repo{
		URL:        awsURL,
		Status:     RepoStatusSuccess,
		PulpURL:    pulpURL,
		PulpStatus: RepoStatusSuccess,
	}

	t.Run("return pulp content url", func(t *testing.T) {
		defer config.Cleanup()

		os.Setenv("FEATURE_PULP_INTEGRATION", "true")
		os.Setenv("PULP_CONTENT_URL", "http://internal.repo.example.com:8080")

		var expectedURL = "https://internal.repo.example.com:8080/api/pulp-content/pulp/repo/is/here"

		assert.Equal(t, expectedURL, repo.ContentURL())
	})

	t.Run("return aws content url", func(t *testing.T) {
		defer config.Cleanup()

		os.Unsetenv("FEATURE_PULP_INTEGRATION")
		os.Unsetenv("PULP_CONTENT_URL")

		var expectedURL = "https://aws.repo.example.com/repo/is/here"

		assert.Equal(t, expectedURL, repo.ContentURL())
	})
}
