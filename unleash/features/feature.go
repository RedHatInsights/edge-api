package feature

import (
	"github.com/Unleash/unleash-client-go/v3"
	unleashCTX "github.com/Unleash/unleash-client-go/v3/context"
)

//FeatureCustomRepos is the const of the custom repo feature flag
const FeatureCustomRepos = "fleet-management.custom-repos"

// CheckFeatureWithAccount checks to see if a given feature is available for a given account
func CheckFeatureWithAccount(account string, feature string) bool {
	unleashCtx := unleashCTX.Context{
		UserId: account,
	}
	return unleash.IsEnabled(feature, unleash.WithContext(unleashCtx))
}
