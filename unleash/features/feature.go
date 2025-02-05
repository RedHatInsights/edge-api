// FIXME: golangci-lint
// nolint:revive,typecheck

// Package feature configures and handles feature flags for use in the application
package feature

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/Unleash/unleash-client-go/v4"
	unleashCTX "github.com/Unleash/unleash-client-go/v4/context"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
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

// LONG_TERM OPERATIONAL AND DEV FLAGS

// GLITCHTIP LOGGING FLAGS

// GlitchtipLogging is the feature flag for reporting errors to GlitchTip
var GlitchtipLogging = &Flag{Name: "edge-management.glitchtip_logging", EnvVar: "GLITCHTIP_LOGGING"}

// KAFKA LOGGING FLAGS

// KafkaLogging is the feature flag for logging kafka messages
var KafkaLogging = &Flag{Name: "edge-management.kafka_logging", EnvVar: "KAFKA_LOGGING"}

// JOB QUEUE FLAGS
var JobQueue = &Flag{Name: "edge-management.job_queue", EnvVar: "FEATURE_JOBQUEUE"}

// DEVICE FEATURE FLAGS

// DeviceSync is the feature flag for routes.CreateImageUpdate() EDA code
var DeviceSync = &Flag{Name: "edge-management.device_sync", EnvVar: "DEVICE_SYNC"}

// DeviceSyncCreate is the feature flag for routes.CreateImageUpdate() EDA code
var DeviceSyncCreate = &Flag{Name: "edge-management.device_sync_create", EnvVar: "DEVICE_SYNC_CREATE"}

// DeviceSyncDelete is the feature flag for routes.CreateImageUpdate() EDA code
var DeviceSyncDelete = &Flag{Name: "edge-management.device_sync_delete", EnvVar: "DEVICE_SYNC_DELETE"}

// StorageImagesRepos is the feature flag to use storage.images-repos when updating images or creating ISO artifacts
var StorageImagesRepos = &Flag{Name: "edge-management.storage_images_repos", EnvVar: "STORAGE_IMAGES_REPOS"}

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

// STATIC DELTA FLAGS

// HideCreateGroup toggles creation of static deltas
var HideCreateGroup = &Flag{Name: "edge-management.hide-create-group", EnvVar: "FEATURE_HIDE_CREATE_GROUP"}

// EdgeParityGroupsMigration toggles edge parity groups migration
var EdgeParityGroupsMigration = &Flag{Name: "edgeParity.groups-migration", EnvVar: "FEATURE_EDGE_PARITY_GROUPS_MIGRATION"}

// EnforceEdgeGroups is a feature flag to query to query unleash whether the org is enforced to use edge groups
var EnforceEdgeGroups = &Flag{Name: "edge-management.enforce_edge_groups", EnvVar: "FEATURE_ENFORCE_EDGE_GROUPS"}

// EdgeParityInventoryGroupsEnabled is a feature flag for inventory groups usage
var EdgeParityInventoryGroupsEnabled = &Flag{Name: "edgeParity.inventory-groups-enabled", EnvVar: "FEATURE_INVENTORY_GROUPS_ENABLED"}

// EdgeParityInventoryRbac is a feature flag for inventory rbac usage
var EdgeParityInventoryRbac = &Flag{Name: "edgeParity.inventory-rbac", EnvVar: "FEATURE_INVENTORY_RBAC"}

// DB LOGGING FLAGS

// SilentGormLogging toggles noisy logging from Gorm (using for tests during development on slow machines/connections
var SilentGormLogging = &Flag{Name: "edge-management.silent_gorm_logging", EnvVar: "FEATURE_SILENT_GORM_LOGGING"}

// PULP INTEGRATION FLAGS

// PulpIntegration covers the overall integration of pulp and deprecation of AWS storage
var PulpIntegration = &Flag{Name: "edge-management.pulp_integration", EnvVar: "FEATURE_PULP_INTEGRATION"}

// PulpIntegrationDisableAWSRepoStore disables the AWS repo store process for development
var PulpIntegrationDisableAWSRepoStore = &Flag{Name: "edge-management.pulp_integration_disable_awsrepostore", EnvVar: "FEATURE_PULP_INTEGRATION_DISABLE_AWSREPOSTORE"}

// PulpIntegrationUpdateViaPulp uses the Pulp Distribution URL for image and system updates
var PulpIntegrationUpdateViaPulp = &Flag{Name: "edge-management.pulp_integration_updateviapulp", EnvVar: "FEATURE_PULP_INTEGRATION_UPDATEVIAPULP"}

// (ADD FEATURE FLAGS ABOVE)
// FEATURE FLAG CHECK CODE

// CheckFeature checks to see if a given feature is available
func CheckFeature(feature string, options ...unleash.FeatureOption) bool {
	if !config.FeatureFlagsConfigured() {
		return false
	}

	if len(options) == 0 {
		options = append(options, unleash.WithContext(unleashCTX.Context{}))
	}
	return unleash.IsEnabled(feature, options...)
}

// CheckFeature checks to see if a given feature is available with context
func CheckFeatureCtx(ctx context.Context, feature string, options ...unleash.FeatureOption) bool {
	if !config.FeatureFlagsConfigured() {
		return false
	}

	uctx := unleashCTX.Context{}
	orgID := identity.GetIdentity(ctx).Identity.OrgID
	if orgID != "" {
		uctx = unleashCTX.Context{
			UserId: orgID,
			Properties: map[string]string{
				"orgId": orgID,
			},
		}
	}

	options = append(options, unleash.WithContext(uctx))
	return unleash.IsEnabled(feature, options...)
}

// IsEnabledLocal returns a bool directly from the environment. Use before Unleash is init'd
func (ff *Flag) IsEnabledLocal() bool {
	envBool, err := strconv.ParseBool(os.Getenv(ff.EnvVar))
	if err != nil {
		fmt.Println("ERROR: ", err.Error())

		return false
	}

	return envBool
}

// IsEnabled checks both the feature flag service and env vars on demand
func (ff *Flag) IsEnabled(options ...unleash.FeatureOption) bool {
	if ff.Name != "" && CheckFeature(ff.Name, options...) {
		return true
	}

	if _, e := os.LookupEnv(ff.EnvVar); e {
		return true
	}

	return false
}

// IsEnabledCtx checks both the feature flag service and env vars on demand.
// Organization ID is passed from the context if present.
func (ff *Flag) IsEnabledCtx(ctx context.Context, options ...unleash.FeatureOption) bool {
	if ff.Name != "" && CheckFeatureCtx(ctx, ff.Name, options...) {
		return true
	}

	if _, e := os.LookupEnv(ff.EnvVar); e {
		return true
	}

	return false
}
