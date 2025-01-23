// FIXME: golangci-lint
// nolint:revive
package migrategroups_test

import (
	"context"
	"errors"
	"os"

	"github.com/redhatinsights/edge-api/cmd/migrategroups/migrategroups"

	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory/mock_inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/inventorygroups"
	"github.com/redhatinsights/edge-api/pkg/clients/inventorygroups/mock_inventorygroups"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"

	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
	log "github.com/osbuild/logging/pkg/logrus"
)

var _ = Describe("Migrate device groups", func() {

	Context("feature flag disabled", func() {
		BeforeEach(func() {
			// ensure migration feature is disabled, feature should be disabled by default
			err := os.Unsetenv(feature.EdgeParityGroupsMigration.EnvVar)
			Expect(err).ToNot(HaveOccurred())
		})

		It("migrate groups should not be available", func() {
			err := migrategroups.MigrateAllGroups(nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(migrategroups.ErrMigrationFeatureNotAvailable))
		})
	})

	Context("feature flag enabled", func() {
		var ctrl *gomock.Controller
		var mockInventoryGroupClient *mock_inventorygroups.MockClientInterface
		var mockInventoryClient *mock_inventory.MockClientInterface

		BeforeEach(func() {
			err := os.Setenv(feature.EdgeParityGroupsMigration.EnvVar, "true")
			Expect(err).ToNot(HaveOccurred())

			ctrl = gomock.NewController(GinkgoT())
			mockInventoryGroupClient = mock_inventorygroups.NewMockClientInterface(ctrl)
			mockInventoryClient = mock_inventory.NewMockClientInterface(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
			err := os.Unsetenv(feature.EdgeParityGroupsMigration.EnvVar)
			Expect(err).ToNot(HaveOccurred())

		})

		Context("MigrateAllGroups", func() {
			var orgID string
			var group models.DeviceGroup
			var otherGroup models.DeviceGroup
			var otherGroup2 models.DeviceGroup
			var initialDefaultOrgsIDS []string
			var initialOrgsGroupsFilter map[string][]any

			BeforeEach(func() {
				initialDefaultOrgsIDS = migrategroups.DefaultOrgsIDS
				initialOrgsGroupsFilter = migrategroups.OrgsGroupsFilter

				if orgID == "" {
					orgID = faker.UUIDHyphenated()

					group = models.DeviceGroup{
						OrgID: orgID,
						Name:  faker.Name(),
						Devices: []models.Device{
							{
								OrgID: orgID,
								Name:  faker.Name(),
								UUID:  faker.UUIDHyphenated(),
							},
							// device should not be added to inventory group
							{
								OrgID: orgID,
								Name:  faker.Name(),
								UUID:  "",
							},
						},
					}
					err := db.DB.Create(&group).Error
					Expect(err).ToNot(HaveOccurred())

					// create another group
					otherGroup = models.DeviceGroup{
						OrgID: orgID,
						Name:  faker.Name(),
						Devices: []models.Device{
							{OrgID: orgID, Name: faker.Name(), UUID: faker.UUIDHyphenated()},
							// this second device will not be used as it's used in other inventory group
							// and should not be included when creating in group creation
							{OrgID: orgID, Name: faker.Name(), UUID: faker.UUIDHyphenated()},
							// this device will be set as non-existent, and will not be included in create group
							{OrgID: orgID, Name: faker.Name(), UUID: faker.UUIDHyphenated()},
						},
					}
					err = db.DB.Create(&otherGroup).Error
					Expect(err).ToNot(HaveOccurred())

					// create another group without devices
					otherGroup2 = models.DeviceGroup{
						OrgID: orgID,
						Name:  faker.Name(),
					}
					err = db.DB.Create(&otherGroup2).Error
					Expect(err).ToNot(HaveOccurred())
				}

				migrategroups.DefaultOrgsIDS = []string{orgID}
				migrategroups.OrgsGroupsFilter = map[string][]any{
					"*": {"device_groups.name = ?", group.Name},
				}

			})

			AfterEach(func() {
				migrategroups.DefaultOrgsIDS = initialDefaultOrgsIDS
				migrategroups.OrgsGroupsFilter = initialOrgsGroupsFilter
				migrategroups.GetNewInventoryGroupClient = inventorygroups.InitClient
				migrategroups.GetNewInventoryClient = inventory.InitClient
			})

			Context("enforce-edge-groups", func() {

				BeforeEach(func() {
					err := os.Setenv(feature.EnforceEdgeGroups.EnvVar, "true")
					Expect(err).ToNot(HaveOccurred())
				})

				AfterEach(func() {
					err := os.Unsetenv(feature.EnforceEdgeGroups.EnvVar)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should not migrate org when org is enforced to use edge groups", func() {
					// GetGroupByName, ReturnDeviceListByID and CreateGroup should not be called
					mockInventoryGroupClient.EXPECT().GetGroupByName(group.Name).Times(0)
					mockInventoryGroupClient.EXPECT().CreateGroup(group.Name, []string{group.Devices[0].UUID}).Times(0)
					mockInventoryClient.EXPECT().ReturnDeviceListByID([]string{group.Devices[0].UUID}).Times(0)

					err := migrategroups.MigrateAllGroups(nil)
					Expect(err).ToNot(HaveOccurred())

					// reload group from db
					err = db.DB.First(&group).Error
					Expect(err).ToNot(HaveOccurred())
					Expect(group.UUID).To(BeEmpty())
				})
			})

			It("should migrate group successfully", func() {
				migrategroups.GetNewInventoryGroupClient = func(ctx context.Context, log log.FieldLogger) inventorygroups.ClientInterface {
					rhIndent, err := common.GetIdentityInstanceFromContext(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(rhIndent.Identity.OrgID).To(Equal(orgID))
					Expect(rhIndent.Identity.Type).To(Equal(migrategroups.DefaultIdentityType))
					Expect(rhIndent.Identity.AuthType).To(Equal(migrategroups.AuthTypeBASIC))
					Expect(rhIndent.Identity.User.OrgAdmin).To(BeTrue())
					return mockInventoryGroupClient
				}
				migrategroups.GetNewInventoryClient = func(ctx context.Context, log log.FieldLogger) inventory.ClientInterface {
					rhIndent, err := common.GetIdentityInstanceFromContext(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(rhIndent.Identity.OrgID).To(Equal(orgID))
					Expect(rhIndent.Identity.Type).To(Equal(migrategroups.DefaultIdentityType))
					Expect(rhIndent.Identity.AuthType).To(Equal(migrategroups.AuthTypeBASIC))
					Expect(rhIndent.Identity.User.OrgAdmin).To(BeTrue())
					return mockInventoryClient
				}

				expectedInventoryGroup := inventorygroups.Group{
					ID:    faker.UUIDHyphenated(),
					Name:  group.Name,
					OrgID: orgID,
				}

				mockInventoryGroupClient.EXPECT().GetGroupByName(group.Name).Return(nil, inventorygroups.ErrGroupNotFound).Times(1)
				mockInventoryGroupClient.EXPECT().CreateGroup(group.Name, []string{group.Devices[0].UUID}).Return(&expectedInventoryGroup, nil).Times(1)
				mockInventoryClient.EXPECT().ReturnDeviceListByID([]string{group.Devices[0].UUID}).Return(inventory.Response{
					Total:  1,
					Count:  1,
					Result: []inventory.Device{{ID: group.Devices[0].UUID}},
				}, nil).Times(1)

				err := migrategroups.MigrateAllGroups(db.DB)
				Expect(err).ToNot(HaveOccurred())

				// reload group from db
				err = db.DB.First(&group).Error
				Expect(err).ToNot(HaveOccurred())
				Expect(group.UUID).To(Equal(expectedInventoryGroup.ID))
			})

			When("on failure", func() {
				BeforeEach(func() {
					migrategroups.OrgsGroupsFilter = map[string][]any{
						orgID: {"device_groups.name = ?", otherGroup.Name},
					}
					migrategroups.GetNewInventoryGroupClient = func(ctx context.Context, log log.FieldLogger) inventorygroups.ClientInterface {
						return mockInventoryGroupClient
					}
					migrategroups.GetNewInventoryClient = func(ctx context.Context, log log.FieldLogger) inventory.ClientInterface {
						return mockInventoryClient
					}
				})

				It("should ignore edge group migration when inventory group name exists", func() {
					expectedInventoryGroup := inventorygroups.Group{
						ID:    faker.UUIDHyphenated(),
						Name:  otherGroup.Name,
						OrgID: orgID,
					}
					// GetGroupByName is called once
					mockInventoryGroupClient.EXPECT().GetGroupByName(otherGroup.Name).Return(&expectedInventoryGroup, nil).Times(1)

					// CreateGroup and ReturnDeviceListByID are not called
					mockInventoryGroupClient.EXPECT().CreateGroup(otherGroup.Name, []string{otherGroup.Devices[0].UUID}).Return(nil, nil).Times(0)
					mockInventoryClient.EXPECT().ReturnDeviceListByID([]string{otherGroup.Devices[0].UUID}).Return(inventory.Response{
						Total:  1,
						Count:  1,
						Result: []inventory.Device{{ID: otherGroup.Devices[0].UUID}},
					}, nil).Times(0)

					err := migrategroups.MigrateAllGroups(db.DB)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return error when GetGroupByName fails", func() {
					expectedError := errors.New("expected error when getting inventory group by name")
					mockInventoryGroupClient.EXPECT().GetGroupByName(otherGroup.Name).Return(nil, expectedError).Times(1)
					err := migrategroups.MigrateAllGroups(db.DB)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(expectedError))
				})

				It("should return error when ReturnDeviceListByID fails", func() {
					expectedError := errors.New("expected error when retrieving hosts from inventory")
					mockInventoryGroupClient.EXPECT().GetGroupByName(otherGroup.Name).Return(nil, inventorygroups.ErrGroupNotFound).Times(1)
					mockInventoryClient.EXPECT().ReturnDeviceListByID([]string{
						otherGroup.Devices[0].UUID, otherGroup.Devices[1].UUID, otherGroup.Devices[2].UUID,
					}).Return(inventory.Response{}, expectedError).Times(1)

					err := migrategroups.MigrateAllGroups(db.DB)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(expectedError))
				})

				It("should return error when CreateGroup fails", func() {
					expectedError := errors.New("expected error when creating inventory group")
					mockInventoryGroupClient.EXPECT().GetGroupByName(otherGroup.Name).Return(nil, inventorygroups.ErrGroupNotFound).Times(1)
					mockInventoryClient.EXPECT().ReturnDeviceListByID([]string{
						otherGroup.Devices[0].UUID, otherGroup.Devices[1].UUID, otherGroup.Devices[2].UUID,
					}).Return(inventory.Response{
						Total: 1,
						Count: 1,
						Result: []inventory.Device{
							{ID: otherGroup.Devices[0].UUID},
							// because this system already in an inventory group, it will not be included when creating a group
							{ID: otherGroup.Devices[1].UUID, Groups: []inventory.Group{
								{Name: faker.Name(), ID: faker.UUIDHyphenated()},
							}},
						},
					}, nil).Times(1)

					mockInventoryGroupClient.EXPECT().CreateGroup(otherGroup.Name, []string{otherGroup.Devices[0].UUID}).Return(nil, expectedError).Times(1)

					err := migrategroups.MigrateAllGroups(db.DB)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(expectedError))
				})
			})

			When("orgID not in groups filter", func() {
				BeforeEach(func() {
					migrategroups.OrgsGroupsFilter = map[string][]any{
						faker.UUIDHyphenated(): {"device_groups.name = ?", group.Name},
					}
					migrategroups.GetNewInventoryGroupClient = func(ctx context.Context, log log.FieldLogger) inventorygroups.ClientInterface {
						return mockInventoryGroupClient
					}
					migrategroups.GetNewInventoryClient = func(ctx context.Context, log log.FieldLogger) inventory.ClientInterface {
						return mockInventoryClient
					}
				})

				It("group should be migrated successfully", func() {
					expectedInventoryGroup := inventorygroups.Group{
						ID:    faker.UUIDHyphenated(),
						Name:  otherGroup.Name,
						OrgID: orgID,
					}
					expectedInventoryGroup2 := inventorygroups.Group{
						ID:    faker.UUIDHyphenated(),
						Name:  otherGroup2.Name,
						OrgID: orgID,
					}

					mockInventoryGroupClient.EXPECT().GetGroupByName(otherGroup.Name).Return(nil, inventorygroups.ErrGroupNotFound).Times(1)
					mockInventoryGroupClient.EXPECT().CreateGroup(otherGroup.Name, []string{otherGroup.Devices[0].UUID}).Return(&expectedInventoryGroup, nil).Times(1)

					mockInventoryGroupClient.EXPECT().GetGroupByName(otherGroup2.Name).Return(nil, inventorygroups.ErrGroupNotFound).Times(1)
					mockInventoryGroupClient.EXPECT().CreateGroup(otherGroup2.Name, []string{}).Return(&expectedInventoryGroup2, nil).Times(1)

					// ReturnDeviceListByID is called only for otherGroup hosts, because otherGroup2 has no hosts
					mockInventoryClient.EXPECT().ReturnDeviceListByID([]string{
						otherGroup.Devices[0].UUID, otherGroup.Devices[1].UUID,
						// this device is not returned (does not exist in inventory) and should not be included when creating the group
						otherGroup.Devices[2].UUID,
					}).Return(inventory.Response{
						Total: 1,
						Count: 1,
						Result: []inventory.Device{
							{ID: otherGroup.Devices[0].UUID},
							// because this system already in an inventory group, should not be included when creating the group
							{ID: otherGroup.Devices[1].UUID, Groups: []inventory.Group{
								{Name: faker.Name(), ID: faker.UUIDHyphenated()},
							}},
						},
					}, nil).Times(1)

					err := migrategroups.MigrateAllGroups(db.DB)
					Expect(err).ToNot(HaveOccurred())

					// reload other groups from db
					err = db.DB.First(&otherGroup).Error
					Expect(err).ToNot(HaveOccurred())
					Expect(otherGroup.UUID).To(Equal(expectedInventoryGroup.ID))
					err = db.DB.First(&otherGroup2).Error
					Expect(err).ToNot(HaveOccurred())
					Expect(otherGroup2.UUID).To(Equal(expectedInventoryGroup2.ID))
				})
			})
		})
	})
})
