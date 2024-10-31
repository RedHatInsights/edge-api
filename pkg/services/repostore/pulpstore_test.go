package repostore

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/magiconair/properties/assert"
	"github.com/redhatinsights/edge-api/config"
)

func TestDistributionURL(t *testing.T) {
	var testURL = "https://example.com:8080"
	var testURLWithPassword = "https://william:tell@example.com:8080"
	var domain = faker.UUIDDigit()
	var repoName = faker.UUIDDigit()

	t.Run("without username", func(t *testing.T) {
		defer config.Cleanup()
		os.Setenv("PULP_CONTENT_URL", testURL)

		var urlTemplate = "%s/api/pulp-content/%s/%s"
		var expectedDistURL = fmt.Sprintf(urlTemplate, testURL, domain, repoName)

		distURL, _ := distributionURL(context.Background(), domain, repoName)
		assert.Equal(t, distURL, expectedDistURL)
	})

	t.Run("with username", func(t *testing.T) {
		defer config.Cleanup()
		os.Setenv("PULP_CONTENT_URL", testURL)
		os.Setenv("PULP_CONTENT_USERNAME", "william")
		os.Setenv("PULP_CONTENT_PASSWORD", "tell")

		var urlTemplate = "%s/api/pulp-content/%s/%s"
		var expectedDistURL = fmt.Sprintf(urlTemplate, testURLWithPassword, domain, repoName)

		distURL, _ := distributionURL(context.Background(), domain, repoName)
		assert.Equal(t, distURL, expectedDistURL)
	})
}
