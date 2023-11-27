package migrategroups

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/redhatinsights/edge-api/pkg/clients/inventorygroups"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ErrMigrationFeatureNotAvailable error returned when the migration feature flag is disabled
var ErrMigrationFeatureNotAvailable = errors.New("groups migrations is not available")

// ErrOrgIDIsMandatory error returned when the org_id with empty value is passed
var ErrOrgIDIsMandatory = errors.New("org_id is mandatory")

// ErrInventoryGroupAlreadyExist error returned when trying to migrate an already existing group
var ErrInventoryGroupAlreadyExist = errors.New("inventory group already exist")

// DefaultDataLimit the default data limit to use when collecting data
var DefaultDataLimit = 100

// DefaultMaxDataPageNumber the default data pages to handle as preventive way to enter an indefinite loop
var DefaultMaxDataPageNumber = 100

// DefaultIdentityType the default identity type used in header when requesting inventory groups end-point
var DefaultIdentityType = "System"

// AuthTypeBASIC the BASIC identity type used in header when requesting inventory groups end-point
var AuthTypeBASIC = "basic-auth"

// OrgsGroupsFilter the filter added to filter an organization groups (if the org_id is defined in the map as a key)
var OrgsGroupsFilter = map[string][]interface{}{
	"11789772": {"device_groups.name LIKE ?", "%-Store-%"},
}

// DefaultOrgsIDS if the slice is not empty, only the organizations with this ids will be taken into account when migrating
var DefaultOrgsIDS = []string{
	"11789772",
}

// OrgCandidate the org candidate queried from the database
type OrgCandidate struct {
	OrgID        string `json:"org_id"`
	GroupsCount  int    `json:"groups_count"`
	DevicesCount int    `json:"devices_count"`
}

// GetNewInventoryGroupClient the function to get the client to inventory groups end-point, for testing convenience
var GetNewInventoryGroupClient = inventorygroups.InitClient

func newInventoryGroupsOrgClient(orgID string) (inventorygroups.ClientInterface, error) {
	// create a new inventory-groups client and set organization identity in the initialization context
	ident := identity.XRHID{Identity: identity.Identity{
		OrgID:    orgID,
		Type:     DefaultIdentityType,
		AuthType: AuthTypeBASIC,
		Internal: identity.Internal{OrgID: orgID},
	}}
	jsonIdent, err := json.Marshal(&ident)
	if err != nil {
		return nil, err
	}
	base64Identity := base64.StdEncoding.EncodeToString(jsonIdent)

	ctx := context.Background()
	ctx = context.WithValue(ctx, request_id.RequestIDKey, uuid.NewString())
	ctx = common.SetOriginalIdentity(ctx, base64Identity)
	client := GetNewInventoryGroupClient(ctx, log.NewEntry(log.StandardLogger()))

	return client, nil
}

func createInventoryGroup(client inventorygroups.ClientInterface, edgeGroup models.DeviceGroup) error {
	groupsHosts := make([]string, 0, len(edgeGroup.Devices))
	for _, device := range edgeGroup.Devices {
		if device.UUID == "" {
			continue
		}
		groupsHosts = append(groupsHosts, device.UUID)
	}
	logger := log.WithFields(log.Fields{
		"context":    "org-group-migration",
		"org_id":     edgeGroup.OrgID,
		"group_name": edgeGroup.Name,
		"hosts":      groupsHosts,
	})

	logger.Info("inventory group create started")
	inventoryGroup, err := client.CreateGroup(edgeGroup.Name, groupsHosts)
	if err != nil {
		logger.WithField("error", err.Error()).Info("error occurred while creating inventory group")
		return err
	}

	// update edge group with inventory group id
	edgeGroup.UUID = inventoryGroup.ID
	if err := db.DB.Omit("Devices").Save(&edgeGroup).Error; err != nil {
		logger.WithField("error", err.Error()).Info("error occurred saving local edge group")
		return err
	}

	logger.Info("inventory group finished successfully")
	return nil
}

func migrateGroup(client inventorygroups.ClientInterface, edgeGroup models.DeviceGroup) error {
	logger := log.WithFields(log.Fields{
		"context":    "org-group-migration",
		"org_id":     edgeGroup.OrgID,
		"group_name": edgeGroup.Name,
	})
	logger.Info("group migration started")

	// check if group exist in inventory group
	if _, err := client.GetGroupByName(edgeGroup.Name); err != nil && err != inventorygroups.ErrGroupNotFound {
		logger.WithField("error", err.Error()).Error("unknown error occurred while getting inventory group")
		return err
	} else if err == nil {
		// inventory group should not exist to continue
		logger.Error("group already exists")
		return ErrInventoryGroupAlreadyExist
	}

	if err := createInventoryGroup(client, edgeGroup); err != nil {
		logger.WithField("error", err.Error()).Error("error occurred while creating inventory group")
		return err
	}
	logger.Info("group migration finished")
	return nil
}

func migrateOrgGroups(orgID string, gormDB *gorm.DB) error {
	if orgID == "" {
		return ErrOrgIDIsMandatory
	}

	logger := log.WithFields(log.Fields{"context": "org-groups-migration", "org_id": orgID})
	logger.Info("organization groups migration started")

	client, err := newInventoryGroupsOrgClient(orgID)
	if err != nil {
		logger.WithField("error", err.Error()).Error("error occurred while creating organization inventory-groups client")
		return err
	}

	// get all org groups
	var orgGroupsToMigrate []models.DeviceGroup
	baseQuery := db.OrgDB(orgID, gormDB, "device_groups").Debug().Where("device_groups.uuid IS NULL OR device_groups.uuid = ''")
	if orgGroupsFilter, ok := OrgsGroupsFilter[orgID]; ok && len(orgGroupsFilter) > 0 {
		query, args := orgGroupsFilter[0], orgGroupsFilter[1:]
		baseQuery = baseQuery.Where(query, args...)
	}

	if err := baseQuery.Preload("Devices").Order("created_at").Find(&orgGroupsToMigrate).Error; err != nil {
		return err
	}

	logger = log.WithField("groups_count", len(orgGroupsToMigrate))

	for _, group := range orgGroupsToMigrate {
		if err := migrateGroup(client, group); err != nil {
			return err
		}
	}

	logger.Info("organization groups migration finished")
	return nil
}

func getAllOrgs(gormDB *gorm.DB) ([]OrgCandidate, error) {
	var orgsData []OrgCandidate
	baseQuery := gormDB.Debug().Table("device_groups").
		Select("device_groups.org_id as org_id, count(distinct(device_groups.name)) as groups_count, count(distinct(devices.id)) as devices_count").
		Joins("LEFT JOIN device_groups_devices ON device_groups_devices.device_group_id = device_groups.id").
		Joins("LEFT JOIN devices ON devices.id = device_groups_devices.device_id").
		Where("device_groups.uuid IS NULL OR device_groups.uuid = ''"). // consider only orgs with empty inventory group id
		Where("device_groups.deleted_at IS NULL").                      // with non deleted groups
		Where("devices.deleted_at IS NULL").                            // with non deleted devices
		Where("devices.id IS NOT NULL")                                 // we take only groups with hosts

	if len(DefaultOrgsIDS) > 0 {
		baseQuery = baseQuery.Where("device_groups.org_id IN (?)", DefaultOrgsIDS)
	}
	if err := baseQuery.Group("device_groups.org_id").
		Order("device_groups.org_id").
		Limit(DefaultDataLimit).
		Scan(&orgsData).Error; err != nil {
		return nil, err
	}

	return orgsData, nil
}

func MigrateAllGroups(gormDB *gorm.DB) error {
	logger := log.WithField("context", "orgs-groups-migration")
	if !feature.EdgeParityGroupsMigration.IsEnabled() {
		logger.Info("group migration feature is disabled, migration is not available")
		return ErrMigrationFeatureNotAvailable

	}
	if gormDB == nil {
		gormDB = db.DB
	}

	page := 0
	orgsCount := 0
	for page < DefaultMaxDataPageNumber {
		orgsToMigrate, err := getAllOrgs(gormDB)
		if err != nil {
			logger.WithField("error", err.Error()).Error("error occurred while getting orgs to migrate")
			return err
		}
		if len(orgsToMigrate) == 0 {
			break
		}
		for _, orgToMigrate := range orgsToMigrate {
			orgLogger := logger.WithFields(log.Fields{
				"org_id":        orgToMigrate.OrgID,
				"groups_count":  orgToMigrate.GroupsCount,
				"devices_count": orgToMigrate.DevicesCount,
			})
			orgLogger.Info("starting migration of organization groups")
			err := migrateOrgGroups(orgToMigrate.OrgID, gormDB)
			if err != nil {
				orgLogger.WithField("error", err.Error()).Error("error occurred while migrating organization groups")
				return err
			}

		}
		orgsCount += len(orgsToMigrate)
		page++
	}

	logger.WithFields(log.Fields{"orgs_count": orgsCount}).Info("migration of organizations groups finished")
	return nil
}
