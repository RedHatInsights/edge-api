package feature

import (
	"os"

	"github.com/Unleash/unleash-client-go/v3"
	unleashCTX "github.com/Unleash/unleash-client-go/v3/context"
	"github.com/redhatinsights/edge-api/config"
)

//FeatureCustomRepos is the const of the custom repo feature flag
const FeatureCustomRepos = "fleet-management.custom-repos"

//FeatureImageBuildMS is the const of the ms build feature flag
const FeatureImageBuildMS = "fleet-management.images_iso"

// Flag defines names for feature flag service and local env
type Flag struct {
	Name   string
	EnvVar string
}

// ImageCreateEDA is the feature flag for routes.CreateImage() EDA code
var ImageCreateEDA = &Flag{Name: "fleet-management.images_iso", EnvVar: "FEATURE_IMAGECREATE"}

// ImageCreateCommitEDA is the feature flag for routes.CreateCommit() EDA code
var ImageCreateCommitEDA = &Flag{Name: "", EnvVar: "FEATURE_IMAGECREATE_COMMIT"}

// ImageCreateInstallerEDA is the feature flag for routes.CreateInstaller() EDA code
var ImageCreateInstallerEDA = &Flag{Name: "", EnvVar: "FEATURE_IMAGECREATE_INSTALLER"}

// ImageCreateKickstartEDA is the feature flag for routes.CreateKickstart() EDA code
var ImageCreateKickstartEDA = &Flag{Name: "", EnvVar: "FEATURE_IMAGECREATE_KICKSTART"}

// ImageCreateRepoEDA is the feature flag for routes.CreateRepo() EDA code
var ImageCreateRepoEDA = &Flag{Name: "", EnvVar: "FEATURE_IMAGECREATE_REPO"}

// ImageUpdateEDA is the feature flag for routes.CreateImageUpdate() EDA code
var ImageUpdateEDA = &Flag{Name: "", EnvVar: "FEATURE_IMAGEUPDATE"}

// CheckFeature checks to see if a given feature is available
func CheckFeature(feature string) bool {
	cfg := config.Get()

	if cfg.FeatureFlagsEnvironment != "ephemeral" && cfg.FeatureFlagsURL != "" {
		unleashCtx := unleashCTX.Context{}
		return unleash.IsEnabled(feature, unleash.WithContext(unleashCtx))
	}

	return false
}

// IsEnabled checks both the feature flag service and env vars on demand
func (ff *Flag) IsEnabled() bool {
	ffServiceEnabled := false
	ffEnvEnabled := false
	if ff.Name != "" {
		ffServiceEnabled = CheckFeature(ff.Name)
	}

	// just check if the env variable exists. it can be set to any value.
	_, ffEnvEnabled = os.LookupEnv(ff.EnvVar)

	// if either is enabled, make it so
	if ffServiceEnabled || ffEnvEnabled {
		return true
	}

	return false
}
