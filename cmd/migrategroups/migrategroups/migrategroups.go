// FIXME: golangci-lint
// nolint:revive
package migrategroups

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/inventorygroups"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services/utility"
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

// DefaultDataLimit the default data limit to use when collecting data
var DefaultDataLimit = 1000

// DefaultMaxDataPageNumber the default data pages to handle as preventive way to enter an indefinite loop
// two pages should be sufficient as DefaultDataLimit is large enough
var DefaultMaxDataPageNumber = 2

// DefaultIdentityType the default identity type used in header when requesting inventory groups end-point
var DefaultIdentityType = "User"

// AuthTypeBASIC the BASIC identity type used in header when requesting inventory groups end-point
var AuthTypeBASIC = "basic-auth"

// OrgsGroupsFilter the filter added to filter an organization groups (if the org_id is defined in the map as a key)
// simple map item:  org_id: {"device_groups.name LIKE ?", "%-Store-%"},
// to use a filter by any org, use a wildcard :  "*": {"device_groups.name LIKE ?", "%-Store-%"},
var OrgsGroupsFilter = map[string][]interface{}{}

// DefaultOrgsIDS if the slice is not empty, only the organizations with this ids will be taken into account when migrating
var DefaultOrgsIDS = []string{}

// OrgCandidate the org candidate queried from the database
type OrgCandidate struct {
	OrgID        string `json:"org_id"`
	GroupsCount  int    `json:"groups_count"`
	DevicesCount int    `json:"devices_count"`
}

// GetNewInventoryGroupClient the function to get the client to inventory groups end-point, for testing convenience
var GetNewInventoryGroupClient = inventorygroups.InitClient

// GetNewInventoryClient the function to get the client to inventory end-point
var GetNewInventoryClient = inventory.InitClient

type InventoryOrgClients struct {
	InventoryClient       inventory.ClientInterface
	InventoryGroupsClient inventorygroups.ClientInterface
}

// newInventoryOrgClients create a new inventory groups client
func newInventoryOrgClients(orgID string) (*InventoryOrgClients, error) {
	// create a new organization identity in the initialization context
	ident := identity.XRHID{Identity: identity.Identity{
		OrgID:    orgID,
		Type:     DefaultIdentityType,
		AuthType: AuthTypeBASIC,
		Internal: identity.Internal{OrgID: orgID},
		User:     identity.User{OrgAdmin: true, Email: "edge-groups-migrator@example.com", FirstName: "edge-groups-migrator"},
	}}
	jsonIdent, err := json.Marshal(&ident)
	if err != nil {
		return nil, err
	}
	base64Identity := base64.StdEncoding.EncodeToString(jsonIdent)
	ctx := context.Background()
	ctx = context.WithValue(ctx, request_id.RequestIDKey, uuid.NewString())
	ctx = common.SetOriginalIdentity(ctx, base64Identity)
	clientLog := log.WithFields(log.Fields{
		"org_id":  orgID,
		"context": "org-groups-migration",
	})
	clients := InventoryOrgClients{
		InventoryClient:       GetNewInventoryClient(ctx, clientLog),
		InventoryGroupsClient: GetNewInventoryGroupClient(ctx, clientLog),
	}
	return &clients, nil
}

func getInventoryGroupHostsToAdd(clients *InventoryOrgClients, edgeGroup models.DeviceGroup) ([]string, error) {
	logger := log.WithFields(log.Fields{
		"context":    "org-group-migration",
		"org_id":     edgeGroup.OrgID,
		"group_name": edgeGroup.Name,
	})
	groupsHosts := make([]string, 0, len(edgeGroup.Devices))
	// filter group devices to have only those with uuids
	for _, device := range edgeGroup.Devices {
		if device.UUID == "" {
			continue
		}
		groupsHosts = append(groupsHosts, device.UUID)
	}
	hostIDS := make([]string, 0, len(groupsHosts))
	if len(groupsHosts) > 0 {
		// check all devices in inventory, to filter the ones that does not exist and the ones that are already in other groups
		result, err := clients.InventoryClient.ReturnDeviceListByID(groupsHosts)
		if err != nil {
			logger.WithField("error", err.Error()).Info("error occurred while getting group devices from inventory")
			return hostIDS, err
		}
		for _, inventoryDevice := range result.Result {
			if len(inventoryDevice.Groups) == 0 {
				hostIDS = append(hostIDS, inventoryDevice.ID)
			}
		}
	}

	return hostIDS, nil
}

func createInventoryGroup(clients *InventoryOrgClients, edgeGroup models.DeviceGroup) error {
	hostIDS, err := getInventoryGroupHostsToAdd(clients, edgeGroup)
	if err != nil {
		return err
	}

	logger := log.WithFields(log.Fields{
		"context":    "org-group-migration",
		"org_id":     edgeGroup.OrgID,
		"group_name": edgeGroup.Name,
		"hosts":      hostIDS,
	})

	logger.Info("inventory group create started")
	inventoryGroup, err := clients.InventoryGroupsClient.CreateGroup(edgeGroup.Name, hostIDS)
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

func migrateGroup(clients *InventoryOrgClients, edgeGroup models.DeviceGroup) error {
	logger := log.WithFields(log.Fields{
		"context":    "org-group-migration",
		"org_id":     edgeGroup.OrgID,
		"group_name": edgeGroup.Name,
	})
	logger.Info("group migration started")

	// check if group exist in inventory group
	if _, err := clients.InventoryGroupsClient.GetGroupByName(edgeGroup.Name); err != nil && err != inventorygroups.ErrGroupNotFound {
		logger.WithField("error", err.Error()).Error("unknown error occurred while getting inventory group")
		return err
	} else if err == nil {
		logger.Error("edge group name already exists in inventory groups, migration skipped")
		return nil
	}

	if err := createInventoryGroup(clients, edgeGroup); err != nil {
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

	logger := log.WithFields(log.Fields{"context": "org-group-migration", "org_id": orgID})
	logger.Info("organization groups migration started")

	inventoryOrgClients, err := newInventoryOrgClients(orgID)
	if err != nil {
		logger.WithField("error", err.Error()).Error("error occurred while creating organization inventory clients")
		return err
	}

	// get all org groups
	var orgGroupsToMigrate []models.DeviceGroup
	baseQuery := db.OrgDB(orgID, gormDB, "device_groups").Debug().Where("device_groups.uuid IS NULL OR device_groups.uuid = ''")
	for _, filterKey := range []string{orgID, "*"} {
		if orgGroupsFilter, ok := OrgsGroupsFilter[filterKey]; ok && len(orgGroupsFilter) > 0 {
			query, args := orgGroupsFilter[0], orgGroupsFilter[1:]
			baseQuery = baseQuery.Where(query, args...)
		}
	}

	if err := baseQuery.Preload("Devices").Order("created_at").Find(&orgGroupsToMigrate).Error; err != nil {
		return err
	}

	logger = log.WithField("groups_count", len(orgGroupsToMigrate))

	for _, group := range orgGroupsToMigrate {
		if err := migrateGroup(inventoryOrgClients, group); err != nil {
			return err
		}
	}

	logger.WithField("groups-count", len(orgGroupsToMigrate)).Info("organization groups migration finished")
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
		Where("devices.id IS NOT NULL").                                // we take only groups with hosts
		Group("device_groups.org_id").
		Order("device_groups.org_id")

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
	// create a map to track the orgs already processed
	orgsMap := make(map[string]bool)
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
			if orgsMap[orgToMigrate.OrgID] {
				// migrate org only once, an org may return back in the query because:
				//    a- not all groups has been migrated when an org filter exists in OrgsGroupsFilter
				//    b- when a group with same name exists in inventory groups
				continue
			}
			if utility.EnforceEdgeGroups(orgToMigrate.OrgID) {
				// do not migrate orgs that are enforced to use edge groups
				orgLogger.Error("enforce-edge-groups is enabled for this organization, migration is skipped")
				continue
			}

			orgLogger.Info("starting migration of organization groups")
			err := migrateOrgGroups(orgToMigrate.OrgID, gormDB)
			if err != nil {
				orgLogger.WithField("error", err.Error()).Error("error occurred while migrating organization groups")
				return err
			}
			// register org_id to make sure the org is not processed many times
			orgsMap[orgToMigrate.OrgID] = true
		}
		orgsCount += len(orgsToMigrate)
		page++
	}

	logger.WithFields(log.Fields{"orgs_count": orgsCount}).Info("migration of organizations groups finished")
	return nil
}
