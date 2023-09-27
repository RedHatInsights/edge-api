package migrategroups_test

import (
	"context"
	"errors"
	"os"

	"github.com/redhatinsights/edge-api/cmd/migrategroups/migrategroups"

	"github.com/redhatinsights/edge-api/pkg/clients/inventorygroups"
	"github.com/redhatinsights/edge-api/pkg/clients/inventorygroups/mock_inventorygroups"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"

	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
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

		BeforeEach(func() {
			err := os.Setenv(feature.EdgeParityGroupsMigration.EnvVar, "true")
			Expect(err).ToNot(HaveOccurred())

			ctrl = gomock.NewController(GinkgoT())
			mockInventoryGroupClient = mock_inventorygroups.NewMockClientInterface(ctrl)
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
						OrgID:   orgID,
						Name:    faker.Name(),
						Devices: []models.Device{{OrgID: orgID, Name: faker.Name(), UUID: faker.UUIDHyphenated()}},
					}
					err = db.DB.Create(&otherGroup).Error
					Expect(err).ToNot(HaveOccurred())
				}

				migrategroups.DefaultOrgsIDS = []string{orgID}
				migrategroups.OrgsGroupsFilter = map[string][]any{
					orgID: {"device_groups.name = ?", group.Name},
				}

			})

			AfterEach(func() {
				migrategroups.DefaultOrgsIDS = initialDefaultOrgsIDS
				migrategroups.OrgsGroupsFilter = initialOrgsGroupsFilter
				migrategroups.GetNewInventoryGroupClient = inventorygroups.InitClient
			})

			It("should migrate group successfully", func() {
				migrategroups.GetNewInventoryGroupClient = func(ctx context.Context, log *log.Entry) inventorygroups.ClientInterface {
					rhIndent, err := common.GetIdentityInstanceFromContext(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(rhIndent.Identity.OrgID).To(Equal(orgID))
					Expect(rhIndent.Identity.Type).To(Equal(migrategroups.DefaultIdentityType))
					return mockInventoryGroupClient
				}

				expectedInventoryGroup := inventorygroups.Group{
					ID:    faker.UUIDHyphenated(),
					Name:  group.Name,
					OrgID: orgID,
				}

				mockInventoryGroupClient.EXPECT().GetGroupByName(group.Name).Return(nil, inventorygroups.ErrGroupNotFound).Times(1)
				mockInventoryGroupClient.EXPECT().CreateGroup(group.Name, []string{group.Devices[0].UUID}).Return(&expectedInventoryGroup, nil).Times(1)

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
					migrategroups.GetNewInventoryGroupClient = func(ctx context.Context, log *log.Entry) inventorygroups.ClientInterface {
						return mockInventoryGroupClient
					}
				})

				It("should return error when inventory group exist with same name", func() {
					expectedInventoryGroup := inventorygroups.Group{
						ID:    faker.UUIDHyphenated(),
						Name:  otherGroup.Name,
						OrgID: orgID,
					}
					mockInventoryGroupClient.EXPECT().GetGroupByName(otherGroup.Name).Return(&expectedInventoryGroup, nil).Times(1)
					err := migrategroups.MigrateAllGroups(db.DB)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(migrategroups.ErrInventoryGroupAlreadyExist))
				})

				It("should return error when GetGroupByName fails", func() {
					expectedError := errors.New("expected error when getting inventory group by name")
					mockInventoryGroupClient.EXPECT().GetGroupByName(otherGroup.Name).Return(nil, expectedError).Times(1)
					err := migrategroups.MigrateAllGroups(db.DB)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(expectedError))
				})

				It("should return error when CreateGroup fails", func() {
					expectedError := errors.New("expected error when creating inventory group")
					mockInventoryGroupClient.EXPECT().GetGroupByName(otherGroup.Name).Return(nil, inventorygroups.ErrGroupNotFound).Times(1)
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
					migrategroups.GetNewInventoryGroupClient = func(ctx context.Context, log *log.Entry) inventorygroups.ClientInterface {
						return mockInventoryGroupClient
					}
				})

				It("group should be migrated successfully", func() {
					expectedInventoryGroup := inventorygroups.Group{
						ID:    faker.UUIDHyphenated(),
						Name:  otherGroup.Name,
						OrgID: orgID,
					}

					mockInventoryGroupClient.EXPECT().GetGroupByName(otherGroup.Name).Return(nil, inventorygroups.ErrGroupNotFound).Times(1)
					mockInventoryGroupClient.EXPECT().CreateGroup(otherGroup.Name, []string{otherGroup.Devices[0].UUID}).Return(&expectedInventoryGroup, nil).Times(1)

					err := migrategroups.MigrateAllGroups(db.DB)
					Expect(err).ToNot(HaveOccurred())

					// reload group from db
					err = db.DB.First(&otherGroup).Error
					Expect(err).ToNot(HaveOccurred())
					Expect(otherGroup.UUID).To(Equal(expectedInventoryGroup.ID))
				})
			})
		})
	})
})
