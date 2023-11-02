package utility_test

import (
	"os"
	"testing"

	"github.com/redhatinsights/edge-api/pkg/services/utility"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"
)

func TestEnforceEdgeGroups(t *testing.T) {
	orgID := faker.UUIDHyphenated()

	defer func() {
		_ = os.Unsetenv(feature.EnforceEdgeGroups.EnvVar)
	}()

	testCases := []struct {
		Name           string
		EnvValue       string
		OrgID          string
		ExpectedOrg    string
		ExpectedResult bool
	}{
		{
			Name:           "should return true when feature flag is used",
			EnvValue:       "true",
			OrgID:          orgID,
			ExpectedResult: true,
		},
		{
			Name:           "should return false when feature flag is not used",
			EnvValue:       "",
			OrgID:          orgID,
			ExpectedResult: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			if testCase.EnvValue == "" {
				err := os.Unsetenv(feature.EnforceEdgeGroups.EnvVar)
				assert.NoError(t, err)
			} else {
				err := os.Setenv(feature.EnforceEdgeGroups.EnvVar, testCase.EnvValue)
				assert.NoError(t, err)

			}
			assert.Equal(t, utility.EnforceEdgeGroups(testCase.OrgID), testCase.ExpectedResult)
		})
	}

}
