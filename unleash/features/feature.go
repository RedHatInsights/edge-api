package feature

import (
	"github.com/Unleash/unleash-client-go/v3"
	unleashCTX "github.com/Unleash/unleash-client-go/v3/context"
)

//FeatureCustomRepos is the const of the custom repo feature flag
const FeatureCustomRepos = "fleet-management.custom-repos"

//FeatureImageBuildMS is the const of the ms build feature flag
const FeatureImageBuildMS = "fleet-management.images_iso"

// CheckFeature checks to see if a given feature is available
func CheckFeature(feature string) bool {
	unleashCtx := unleashCTX.Context{}
	return unleash.IsEnabled(feature, unleash.WithContext(unleashCtx))
}
