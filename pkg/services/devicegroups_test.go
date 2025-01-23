// FIXME: golangci-lint
// nolint:errcheck,gosec,govet,revive,typecheck
package services_test

import (
	"context"
	"fmt"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"

	log "github.com/osbuild/logging/pkg/logrus"

	"github.com/redhatinsights/edge-api/pkg/db"
)

var _ = Describe("DeviceGroupsService basic functions", func() {
	faker.SetRandomNumberBoundaries(1000, 100000) // set the boundaries for the random number generator - avoids collisions
	var (
		ctx                 context.Context
		deviceGroupsService services.DeviceGroupsServiceInterface
	)
	BeforeEach(func() {
		ctx = context.Background()
		deviceGroupsService = services.NewDeviceGroupsService(ctx, log.NewEntry(log.StandardLogger()))
	})

	Context("creation of duplicated DeviceGroup name", func() {
		account := common.DefaultAccount
		orgID := common.DefaultOrgID
		It("should fail to create a DeviceGroup with duplicated name", func() {
			deviceGroupName := faker.Name()
			deviceGroup, err := deviceGroupsService.CreateDeviceGroup(&models.DeviceGroup{Name: deviceGroupName, Account: account, OrgID: orgID, Type: models.DeviceGroupTypeDefault})
			Expect(err).To(BeNil())
			Expect(deviceGroup).NotTo(BeNil())

			_, err = deviceGroupsService.CreateDeviceGroup(&models.DeviceGroup{Name: deviceGroupName, Account: account, OrgID: orgID, Type: models.DeviceGroupTypeDefault})
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("device group already exists"))
		})
	})
	Context("deletion of DeviceGroup", func() {
		account := common.DefaultAccount
		orgID := common.DefaultOrgID
		deviceGroupName := faker.Name()
		devices := []models.Device{
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
				OrgID:   orgID,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
				OrgID:   orgID,
			},
		}
		deviceGroup := &models.DeviceGroup{
			Name:    deviceGroupName,
			Type:    models.DeviceGroupTypeDefault,
			Account: account,
			OrgID:   orgID,
			Devices: devices,
		}
		It("should create a DeviceGroup", func() {
			dbResult := db.DB.Create(&deviceGroup).Error
			Expect(dbResult).To(BeNil())
		})
		It("should get the DeviceGroup ID", func() {
			dbResult := db.DB.Where("name = ?", deviceGroupName).First(&deviceGroup)
			Expect(dbResult.Error).To(BeNil())
			Expect(deviceGroup.ID).NotTo(BeZero())
		})
		When("deleting a DeviceGroup", func() {
			It("should delete the DeviceGroup", func() {
				err := deviceGroupsService.DeleteDeviceGroupByID(fmt.Sprintf("%d", deviceGroup.ID))
				Expect(err).To(BeNil())
			})
			It("should not find the DeviceGroup", func() {
				dbResult := db.DB.Where("name = ?", deviceGroupName).First(&deviceGroup)
				Expect(dbResult.Error).NotTo(BeNil())
			})
			It("should find the devices in the DB", func() {
				var devicesFromDB []models.Device
				Expect(db.DB.Where("name in (?)", []string{devices[0].Name, devices[1].Name}).Find(&devicesFromDB).Error).To(BeNil())
				Expect(devicesFromDB).NotTo(BeEmpty())
			})
		})
		It("should fail to delete a DeviceGroup with invalid ID", func() {
			err := deviceGroupsService.DeleteDeviceGroupByID("invalid-id")
			Expect(err).NotTo(BeNil())
			expectedError := services.DeviceGroupNotFound{}
			Expect(err.Error()).To(Equal(expectedError.Error()))
		})
	})
	Context("adding devices to DeviceGroup", func() {
		account1 := faker.UUIDHyphenated()
		account2 := faker.UUIDHyphenated()
		orgID1 := faker.UUIDHyphenated()
		orgID2 := faker.UUIDHyphenated()
		deviceGroupName1 := faker.Name()
		deviceGroupName2 := faker.Name()
		devices := []models.Device{
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account1,
				OrgID:   orgID1,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account1,
				OrgID:   orgID1,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account2,
				OrgID:   orgID2,
			},
		}
		deviceGroups := []models.DeviceGroup{
			{Name: deviceGroupName1, Account: account1, OrgID: orgID1, Type: models.DeviceGroupTypeDefault},
			{Name: deviceGroupName2, Account: account2, OrgID: orgID2, Type: models.DeviceGroupTypeDefault},
		}
		It("should create DeviceGroups", func() {
			for _, device := range devices {
				dbResult := db.DB.Create(&device).Error
				Expect(dbResult).To(BeNil())
			}
			for _, deviceGroup := range deviceGroups {
				dbResult := db.DB.Create(&deviceGroup).Error
				Expect(dbResult).To(BeNil())
			}
		})
		var deviceGroup1 models.DeviceGroup
		It("should add devices to DeviceGroups", func() {
			dbResult := db.DB.Where("name in (?)", []string{devices[0].Name, devices[1].Name}).Find(&devices)
			Expect(dbResult.Error).To(BeNil())

			dbResult = db.DB.Where("name = ?", deviceGroupName1).First(&deviceGroup1)
			Expect(dbResult.Error).To(BeNil())

			addedDevices, err := deviceGroupsService.AddDeviceGroupDevices(orgID1, deviceGroup1.ID, devices)
			Expect(err).To(BeNil())
			Expect(len(*addedDevices)).To(Equal(2))
		})
		When("re-adding devices", func() {
			It("should not return an error", func() {
				_, err := deviceGroupsService.AddDeviceGroupDevices(orgID1, deviceGroup1.ID, devices)
				Expect(err).To(BeNil())
			})
		})
		When("adding empty devices", func() {
			It("should fail", func() {
				_, err := deviceGroupsService.AddDeviceGroupDevices(orgID1, deviceGroup1.ID, []models.Device{})
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupDevicesNotSupplied{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
		When("adding multiple devices; one not exist", func() {
			It("should fail", func() {
				var fakeDevice models.Device
				err := faker.FakeData(&fakeDevice)
				Expect(err).To(BeNil())
				devices, err := deviceGroupsService.AddDeviceGroupDevices(orgID1, deviceGroup1.ID, []models.Device{devices[0], fakeDevice})
				Expect(devices).To(BeNil())
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupOrgIDDevicesNotFound{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
		When("adding not existing device to existing device-group", func() {
			It("should fail", func() {
				var fakeDevice models.Device
				err := faker.FakeData(&fakeDevice)
				Expect(err).To(BeNil())
				devices, err := deviceGroupsService.AddDeviceGroupDevices(orgID1, deviceGroup1.ID, []models.Device{fakeDevice})
				Expect(devices).To(BeNil())
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupOrgIDDevicesNotFound{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
		When("adding with empty account and empty orgID", func() {
			It("should fail", func() {
				_, err := deviceGroupsService.AddDeviceGroupDevices("", deviceGroup1.ID, devices)
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupMandatoryFieldsUndefined{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
		When("adding with empty DeviceGroup ID", func() {
			It("should fail", func() {
				_, err := deviceGroupsService.AddDeviceGroupDevices(orgID1, 0, devices)
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupMandatoryFieldsUndefined{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
		When("adding devices with wrong account", func() {
			It("should fail", func() {
				var devicesFromDB []models.Device
				dbResult := db.DB.Where("account in (?)", []string{account1, account2}).Find(&devicesFromDB)
				Expect(dbResult.Error).To(BeNil())

				_, err := deviceGroupsService.AddDeviceGroupDevices(orgID1, deviceGroup1.ID, devicesFromDB)
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupOrgIDDevicesNotFound{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
		When("adding devices with wrong orgID", func() {
			It("should fail", func() {
				var devicesFromDB []models.Device
				dbResult := db.DB.Where("org_id in (?)", []string{orgID1, orgID2}).Find(&devicesFromDB)
				Expect(dbResult.Error).To(BeNil())

				_, err := deviceGroupsService.AddDeviceGroupDevices(orgID1, deviceGroup1.ID, devicesFromDB)
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupOrgIDDevicesNotFound{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
	})

	Context("delete DeviceGroup devices", func() {
		account, err := common.GetAccountFromContext(ctx)
		It("should return account from context without error", func() {
			Expect(err).To(BeNil())
		})
		orgID, err := common.GetOrgIDFromContext(ctx)
		It("should return orgID from context without error", func() {
			Expect(err).To(BeNil())
		})
		deviceGroupName := faker.Name()
		devices := []models.Device{
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
				OrgID:   orgID,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
				OrgID:   orgID,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
				OrgID:   orgID,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
				OrgID:   orgID,
			},
		}
		deviceGroup := &models.DeviceGroup{
			Name:    deviceGroupName,
			Type:    models.DeviceGroupTypeDefault,
			Account: account,
			OrgID:   orgID,
			Devices: devices,
		}

		It("should create DeviceGroup", func() {
			err := db.DB.Create(&deviceGroup).Error
			Expect(err).To(BeNil())
		})

		When("device group created", func() {
			var deviceGroupID uint
			var savedDeviceGroup models.DeviceGroup
			It("should get the saved DeviceGroup", func() {
				res := db.DB.Where("name = ?", deviceGroupName).Preload("Devices").First(&savedDeviceGroup)
				Expect(res.Error).To(BeNil())
				Expect(savedDeviceGroup.ID).NotTo(BeZero())
				Expect(len(savedDeviceGroup.Devices) > 0).To(BeTrue())
				Expect(len(savedDeviceGroup.Devices)).To(Equal(len(devices)))
				deviceGroupID = savedDeviceGroup.ID
			})
			// delete the first device
			var deletedDeviceID uint
			It("should remove device from device-group", func() {
				deletedDevices, delErr := deviceGroupsService.DeleteDeviceGroupDevices(orgID, deviceGroupID, []models.Device{savedDeviceGroup.Devices[0]})
				Expect(delErr).To(BeNil())
				Expect(deletedDevices).NotTo(BeNil())
				Expect(len(*deletedDevices) > 0).To(BeTrue())
				deletedDeviceID = (*deletedDevices)[0].ID
				Expect(deletedDeviceID).To(Equal(savedDeviceGroup.Devices[0].ID))
			})

			var newSavedDeviceGroup models.DeviceGroup
			It("should get the saved DeviceGroup after device delete", func() {
				res := db.DB.Model(&newSavedDeviceGroup).Preload("Devices").First(&newSavedDeviceGroup, deviceGroupID)
				Expect(res.Error).To(BeNil())
				Expect(newSavedDeviceGroup.ID).NotTo(BeZero())
				Expect(len(newSavedDeviceGroup.Devices) > 0).To(BeTrue())
			})
			It("should not return the deleted device group device any more", func() {
				Expect(len(newSavedDeviceGroup.Devices)).To(Equal(len(devices) - 1))
				var deletedDevicesIDS = make([]uint, 0, len(devices))
				for _, device := range newSavedDeviceGroup.Devices {
					if device.ID == deviceGroupID {
						deletedDevicesIDS = append(deletedDevicesIDS, device.ID)
					}
				}
				Expect(deletedDevicesIDS).To(BeEmpty())
			})

			It("should not delete non existing device from device-group", func() {
				var fakeDevice models.Device
				err := faker.FakeData(&fakeDevice)
				Expect(err).To(BeNil())

				deletedDevices, delErr := deviceGroupsService.DeleteDeviceGroupDevices(orgID, deviceGroupID, []models.Device{fakeDevice})
				Expect(delErr).NotTo(BeNil())
				Expect(delErr.Error()).To(Equal(new(services.DeviceGroupDevicesNotFound).Error()))
				Expect(deletedDevices).To(BeNil())
			})

			// delete the multiple devices at once
			It("should remove multiple devices from device-group", func() {
				deletedDevices, delErr := deviceGroupsService.DeleteDeviceGroupDevices(orgID, deviceGroupID, savedDeviceGroup.Devices[1:3])
				Expect(delErr).To(BeNil())
				Expect(deletedDevices).NotTo(BeNil())
				Expect(len(*deletedDevices) > 0).To(BeTrue())
				Expect(len(*deletedDevices)).To(Equal(len(savedDeviceGroup.Devices[1:3])))
			})

			// delete multiple devices; one of them does not exist
			It("should not remove non existing devices from device-group", func() {
				var fakeDevice models.Device
				err := faker.FakeData(&fakeDevice)
				Expect(err).To(BeNil())

				deletedDevices, delErr := deviceGroupsService.DeleteDeviceGroupDevices(orgID, deviceGroupID, []models.Device{savedDeviceGroup.Devices[3], fakeDevice})
				Expect(delErr).NotTo(BeNil())
				Expect(delErr.Error()).To(Equal(new(services.DeviceGroupDevicesNotFound).Error()))
				Expect(deletedDevices).To(BeNil())
			})

			// delete device from another device-group
			It("should not remove non existing devices from device-group; device from another device-group", func() {
				var fakeDeviceGroup models.DeviceGroup
				err = faker.FakeData(&fakeDeviceGroup)
				Expect(err).To(BeNil())
				fakeDeviceGroup.Devices = []models.Device{savedDeviceGroup.Devices[0]}
				Expect(db.DB.Omit("Devices.*").Create(&fakeDeviceGroup).Error).To(BeNil())

				deletedDevices, delErr := deviceGroupsService.DeleteDeviceGroupDevices(orgID, deviceGroupID, fakeDeviceGroup.Devices)
				Expect(delErr).NotTo(BeNil())
				Expect(delErr.Error()).To(Equal(new(services.DeviceGroupDevicesNotFound).Error()))
				Expect(deletedDevices).To(BeNil())
			})

			// delete multiple devices; one of them does not exist AND one of them does not belong to the device-group
			It("should not remove non existing devices from device-group; one of them does not belong to the device-group", func() {
				var fakeDevice models.Device
				err := faker.FakeData(&fakeDevice)
				Expect(err).To(BeNil())

				var fakeDeviceGroup models.DeviceGroup
				err = faker.FakeData(&fakeDeviceGroup)
				Expect(err).To(BeNil())
				fakeDeviceGroup.Devices = []models.Device{savedDeviceGroup.Devices[0], fakeDevice}
				Expect(db.DB.Omit("Devices.*").Create(&fakeDeviceGroup).Error).To(BeNil())

				deletedDevices, delErr := deviceGroupsService.DeleteDeviceGroupDevices(orgID, deviceGroupID, fakeDeviceGroup.Devices)
				Expect(delErr).NotTo(BeNil())
				Expect(delErr.Error()).To(Equal(new(services.DeviceGroupDevicesNotFound).Error()))
				Expect(deletedDevices).To(BeNil())
			})

			It("should return error when device does not exist in device-group", func() {
				_, err := deviceGroupsService.DeleteDeviceGroupDevices(orgID, deviceGroupID, []models.Device{savedDeviceGroup.Devices[0]})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("devices not found in device group"))
			})

			It("should return error when deviceGroupId is undefined", func() {
				_, err := deviceGroupsService.DeleteDeviceGroupDevices(orgID, 0, []models.Device{savedDeviceGroup.Devices[0]})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("device group mandatory field are undefined"))

			})

			It("should return error when devices not supplied", func() {
				_, err := deviceGroupsService.DeleteDeviceGroupDevices(orgID, deviceGroupID, []models.Device{})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("devices must be supplied to be added to or removed from device group"))
			})
		})
	})
	Context("Create DeviceGroup with 2 Devices that has same Image", func() {
		orgID := faker.UUIDHyphenated()
		deviceGroupName := faker.Name()
		commit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated(), ChangesRefs: true}
		db.DB.Create(&commit)
		imageSet1 := models.ImageSet{OrgID: orgID, Name: faker.Name(), Version: 3}
		db.DB.Create(&imageSet1)
		image := models.Image{Name: faker.Name(), OrgID: orgID, CommitID: commit.ID, ImageSetID: &imageSet1.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&image)

		devices := []models.Device{
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				OrgID:   orgID,
				ImageID: image.ID,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				OrgID:   orgID,
				ImageID: image.ID,
			},
		}
		db.DB.Create(&devices)
		deviceGroup := models.DeviceGroup{
			Name:    deviceGroupName,
			OrgID:   orgID,
			Type:    models.DeviceGroupTypeDefault,
			Devices: devices,
		}
		When("Create DeviceGroup with 2 Devices that has same Image", func() {
			It("check that image appear once under DeviceImageInfo", func() {
				res := db.DB.Omit("Devices.*").Create(&deviceGroup)
				Expect(res.Error).To(BeNil())
				Expect(deviceGroup.ID).NotTo(Equal(0))
				re, err := deviceGroupsService.GetDeviceGroups(orgID, 100, 0, db.DB)
				Expect(err).To(BeNil())
				Expect(re).ToNot(BeNil())
				Expect(*re).ToNot(BeEmpty())
				result := (*re)[0].DeviceImageInfo
				Expect(result).ToNot(BeNil())
				Expect(len(*result)).To(Equal(1))
				Expect((*result)[0].Name).To(Equal(image.Name))
				Expect((*result)[0].Distribution).To(Equal(image.Distribution))
				Expect((*result)[0].Version).To(Equal(image.Version))
				Expect((*result)[0].CreatedAt).ToNot(BeNil())
			})
		})
	})

	Context("rename device group", func() {
		var orgID string
		var deviceGroup models.DeviceGroup
		BeforeEach(func() {
			var err error
			orgID, err = common.GetOrgIDFromContext(ctx)
			Expect(err).ToNot(HaveOccurred())
		})
		When("group has no devices", func() {
			BeforeEach(func() {
				deviceGroup = models.DeviceGroup{
					OrgID: orgID,
					Name:  faker.Name(),
				}
				err := db.DB.Create(&deviceGroup).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("should rename group", func() {
				updateGroup := models.DeviceGroup{Name: faker.Name()}
				err := deviceGroupsService.UpdateDeviceGroup(&updateGroup, orgID, fmt.Sprintf("%d", deviceGroup.ID))
				Expect(err).ToNot(HaveOccurred())

				var group models.DeviceGroup
				err = db.DB.First(&group, deviceGroup.ID).Error
				Expect(err).ToNot(HaveOccurred())
				Expect(group.Name).To(Equal(updateGroup.Name))
			})
		})

		When("group has devices", func() {
			BeforeEach(func() {
				deviceGroup = models.DeviceGroup{
					OrgID: orgID,
					Name:  faker.Name(),
					Devices: []models.Device{
						{OrgID: orgID, UUID: faker.UUIDHyphenated()},
						{OrgID: orgID, UUID: faker.UUIDHyphenated()},
					},
				}
				err := db.DB.Create(&deviceGroup).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("should rename group", func() {
				updateGroup := models.DeviceGroup{Name: faker.Name()}
				err := deviceGroupsService.UpdateDeviceGroup(&updateGroup, orgID, fmt.Sprintf("%d", deviceGroup.ID))
				Expect(err).ToNot(HaveOccurred())

				var group models.DeviceGroup
				err = db.DB.First(&group, deviceGroup.ID).Error
				Expect(err).ToNot(HaveOccurred())
				Expect(group.Name).To(Equal(updateGroup.Name))
			})
		})

		When("group with same name exists", func() {
			var existingGroupName string
			BeforeEach(func() {
				existingGroupName = faker.Name()
				deviceGroup = models.DeviceGroup{OrgID: orgID, Name: faker.Name()}
				err := db.DB.Create(&deviceGroup).Error
				Expect(err).ToNot(HaveOccurred())
				otherDeviceGroup := models.DeviceGroup{OrgID: orgID, Name: existingGroupName}
				err = db.DB.Create(&otherDeviceGroup).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return error", func() {
				updateGroup := models.DeviceGroup{Name: existingGroupName}
				err := deviceGroupsService.UpdateDeviceGroup(&updateGroup, orgID, fmt.Sprintf("%d", deviceGroup.ID))
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.DeviceGroupAlreadyExists)))
			})
		})
	})
})
