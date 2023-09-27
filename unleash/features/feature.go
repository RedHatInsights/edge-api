// FIXME: golangci-lint
// nolint:revive,typecheck

// Package feature configures and handles feature flags for use in the application
package feature

import (
	"os"

	"github.com/Unleash/unleash-client-go/v3"
	unleashCTX "github.com/Unleash/unleash-client-go/v3/context"

	"github.com/redhatinsights/edge-api/config"
)

// FeatureCustomRepos is the const of the custom repo feature flag
const FeatureCustomRepos = "fleet-management.custom-repos"

// FeatureImageBuildMS is the const of the ms build feature flag
const FeatureImageBuildMS = "fleet-management.images_iso"

// Flag defines names for feature flag service and local env
type Flag struct {
	Name   string
	EnvVar string
}

// GLITCHTIP LOGGING FLAGS

// GlitchtipLogging is the feature flag for reporting errors to GlitchTip
var GlitchtipLogging = &Flag{Name: "edge-management.glitchtip_logging", EnvVar: "GLITCHTIP_LOGGING"}

// KAFKA LOGGING FLAGS

// KafkaLogging is the feature flag for logging kafka messages
var KafkaLogging = &Flag{Name: "edge-management.kafka_logging", EnvVar: "KAFKA_LOGGING"}

// IMAGE FEATURE FLAGS

// ImageCreateEDA is the feature flag for routes.CreateImage() EDA code
var ImageCreateEDA = &Flag{Name: "edge-management.image_create", EnvVar: "FEATURE_IMAGECREATE"}

// ImageUpdateEDA is the feature flag for routes.CreateImageUpdate() EDA code
var ImageUpdateEDA = &Flag{Name: "edge-management.image_update", EnvVar: "FEATURE_IMAGEUPDATE"}

// ImageCreateCommitEDA is the feature flag for routes.CreateCommit() EDA code
var ImageCreateCommitEDA = &Flag{Name: "", EnvVar: "FEATURE_IMAGECREATE_COMMIT"}

// ImageCreateInstallerEDA is the feature flag for routes.CreateInstaller() EDA code
var ImageCreateInstallerEDA = &Flag{Name: "", EnvVar: "FEATURE_IMAGECREATE_INSTALLER"}

// ImageCreateKickstartEDA is the feature flag for routes.CreateKickstart() EDA code
var ImageCreateKickstartEDA = &Flag{Name: "", EnvVar: "FEATURE_IMAGECREATE_KICKSTART"}

// ImageCreateRepoEDA is the feature flag for routes.CreateRepo() EDA code
var ImageCreateRepoEDA = &Flag{Name: "", EnvVar: "FEATURE_IMAGECREATE_REPO"}

// ImageCompletionEventsEDA is the feature flag for routes.CreateRepo() EDA code
var ImageCompletionEventsEDA = &Flag{Name: "edge-management.completion_events", EnvVar: "FEATURE_COMPLETION_EVENTS"}

// ImageCreateISOEDA is the feature flag for routes.CreateCommit() EDA code
var ImageCreateISOEDA = &Flag{Name: "edge-management.image_create_iso", EnvVar: "FEATURE_IMAGECREATE_ISO"}

// BuildUpdateRepoWithOldCommits is the feature flag for services.BuildUpdateRepo() to enable oldCommits feature
var BuildUpdateRepoWithOldCommits = &Flag{Name: "edge-management.build_update_repo_with_old_commits", EnvVar: "FEATURE_BUILD_UPDATE_REPO_WITH_OLD_COMMITS"}

// DEVICE FEATURE FLAGS

// DeviceSync is the feature flag for routes.CreateImageUpdate() EDA code
var DeviceSync = &Flag{Name: "edge-management.device_sync", EnvVar: "DEVICE_SYNC"}

// DeviceSyncCreate is the feature flag for routes.CreateImageUpdate() EDA code
var DeviceSyncCreate = &Flag{Name: "edge-management.device_sync_create", EnvVar: "DEVICE_SYNC_CREATE"}

// DeviceSyncDelete is the feature flag for routes.CreateImageUpdate() EDA code
var DeviceSyncDelete = &Flag{Name: "edge-management.device_sync_delete", EnvVar: "DEVICE_SYNC_DELETE"}

// StorageImagesRepos is the feature flag to use storage.images-repos when updating images or creating ISO artifacts
var StorageImagesRepos = &Flag{Name: "edge-management.storage_images_repos", EnvVar: "STORAGE_IMAGES_REPOS"}

// DedupPackage is the feature flag to use edge-management.dedup_installed_packages when creating images
var DedupPackage = &Flag{Name: "edge-management.dedup_installed_packages", EnvVar: "DEDUP_INSTALLED_PACKAGES"}

// UpdateRepoRequested is the feature flag to use for services.UpdateService.CreateUpdate(id) EDA Code
var UpdateRepoRequested = &Flag{Name: "edge-management.update_repo_requested", EnvVar: "FEATURE_UPDATE_REPO_REQUESTED"}

// ContentSources is a feature flag to use for code related to Parity and custom repositories
var ContentSources = &Flag{Name: "edge-management.content_sources", EnvVar: "FEATURE_CONTENT_SOURCES"}

// MigrateCustomRepositories is a feature flag to use for code related to custom repositories migration to content-sources
var MigrateCustomRepositories = &Flag{Name: "edge-management.migrate_custom_repositories", EnvVar: "FEATURE_MIGRATE_CUSTOM_REPOSITORIES"}

// PostMigrateDeleteCustomRepositories is a feature flag to use for deletion of repos after migration
var PostMigrateDeleteCustomRepositories = &Flag{Name: "edge-management.post_migrate_delete_repos", EnvVar: "FEATURE_POST_MIGRATE_DELETE_REPOSITORIES"}

// CleanUPImages is a feature flag to use for cleanup images
var CleanUPImages = &Flag{Name: "edge-management.cleanup_images", EnvVar: "FEATURE_CLEANUP_IMAGES"}

// CleanUPDeleteImages is a feature flag to use for cleanup delete images
var CleanUPDeleteImages = &Flag{Name: "edge-management.cleanup_delete_images", EnvVar: "FEATURE_CLEANUP_DELETE_IMAGES"}

// CleanUPDevices is a feature flag to use for cleanup devices
var CleanUPDevices = &Flag{Name: "edge-management.cleanup_devices", EnvVar: "FEATURE_CLEANUP_DEVICES"}

// CleanUPOrphanCommits is a feature flag to use for cleanup orphan commits
var CleanUPOrphanCommits = &Flag{Name: "edge-management.cleanup_orphan_commits", EnvVar: "FEATURE_CLEANUP_ORPHAN_COMMITS"}

// SkipUpdateRepo is a feature flag to skip the process of download and re-upload of update repositories
var SkipUpdateRepo = &Flag{Name: "edge-management.skip_update_repo", EnvVar: "FEATURE_SKIP_UPDATE_REPO"}

// STATIC DELTA FLAGS

// StaticDeltaDev toggles creation of static deltas
var StaticDeltaDev = &Flag{Name: "edge-management.static_delta_dev", EnvVar: "FEATURE_STATIC_DELTA_DEV"}

// StaticDeltaShortCircuit toggles creation of static deltas
var StaticDeltaShortCircuit = &Flag{Name: "edge-management.static_delta_shortcircuit",
	EnvVar: "FEATURE_STATIC_DELTA_SHORTCIRCUIT"}

// StaticDeltaGenerate toggles creation of static deltas
var StaticDeltaGenerate = &Flag{Name: "edge-management.static_delta_generate", EnvVar: "FEATURE_STATIC_DELTA_GENERATE"}

// CreateGroup toggles creation of static deltas
var HideCreateGroup = &Flag{Name: "edge-management.hide-create-group", EnvVar: "FEATURE_HIDE_CREATE_GROUP"}

// EdgeParityGroupsMigration toggles edge parity groups migration
var EdgeParityGroupsMigration = &Flag{Name: "edgeParity.groups-migration", EnvVar: "FEATURE_EDGE-PARITY-GROUPS-MIGRATION"}

// DB LOGGING FLAGS

// SilentGormLogging toggles noisy logging from Gorm (using for tests during development on slow machines/connections
var SilentGormLogging = &Flag{Name: "edge-management.silent_gorm_logging", EnvVar: "FEATURE_SILENT_GORM_LOGGING"}

// (ADD FEATURE FLAGS ABOVE)
// FEATURE FLAG CHECK CODE

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
