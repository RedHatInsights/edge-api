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

	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/pkg/db"
)

var _ = Describe("DeviceGroupsService basic functions", func() {
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
		It("should fail to create a DeviceGroup with duplicated name", func() {
			deviceGroupName := faker.Name()
			deviceGroup, err := deviceGroupsService.CreateDeviceGroup(&models.DeviceGroup{Name: deviceGroupName, Account: account, Type: models.DeviceGroupTypeDefault})
			Expect(err).To(BeNil())
			Expect(deviceGroup).NotTo(BeNil())

			_, err = deviceGroupsService.CreateDeviceGroup(&models.DeviceGroup{Name: deviceGroupName, Account: account, Type: models.DeviceGroupTypeDefault})
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("device group already exists"))
		})
	})
	Context("deletion of DeviceGroup", func() {
		account := common.DefaultAccount
		deviceGroupName := faker.Name()
		devices := []models.Device{
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
			},
		}
		deviceGroup := &models.DeviceGroup{
			Name:    deviceGroupName,
			Type:    models.DeviceGroupTypeDefault,
			Account: account,
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
		deviceGroupName1 := faker.Name()
		deviceGroupName2 := faker.Name()
		devices := []models.Device{
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account1,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account1,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account2,
			},
		}
		deviceGroups := []models.DeviceGroup{
			{Name: deviceGroupName1, Account: account1, Type: models.DeviceGroupTypeDefault},
			{Name: deviceGroupName2, Account: account2, Type: models.DeviceGroupTypeDefault},
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

			addedDevices, err := deviceGroupsService.AddDeviceGroupDevices(account1, deviceGroup1.ID, devices)
			Expect(err).To(BeNil())
			Expect(len(*addedDevices)).To(Equal(2))
		})
		When("re-adding devices", func() {
			It("should not return an error", func() {
				_, err := deviceGroupsService.AddDeviceGroupDevices(account1, deviceGroup1.ID, devices)
				Expect(err).To(BeNil())
			})
		})
		When("adding empty devices", func() {
			It("should fail", func() {
				_, err := deviceGroupsService.AddDeviceGroupDevices(account1, deviceGroup1.ID, []models.Device{})
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
				devices, err := deviceGroupsService.AddDeviceGroupDevices(account1, deviceGroup1.ID, []models.Device{devices[0], fakeDevice})
				Expect(devices).To(BeNil())
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupAccountDevicesNotFound{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
		When("adding not existing device to existing device-group", func() {
			It("should fail", func() {
				var fakeDevice models.Device
				err := faker.FakeData(&fakeDevice)
				Expect(err).To(BeNil())
				devices, err := deviceGroupsService.AddDeviceGroupDevices(account1, deviceGroup1.ID, []models.Device{fakeDevice})
				Expect(devices).To(BeNil())
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupAccountDevicesNotFound{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
		When("adding with empty account", func() {
			It("should fail", func() {
				_, err := deviceGroupsService.AddDeviceGroupDevices("", deviceGroup1.ID, devices)
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupAccountOrIDUndefined{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
		When("adding with empty DeviceGroup ID", func() {
			It("should fail", func() {
				_, err := deviceGroupsService.AddDeviceGroupDevices(account1, 0, devices)
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupAccountOrIDUndefined{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
		When("adding devices with wrong account", func() {
			It("should fail", func() {
				var devicesFromDB []models.Device
				dbResult := db.DB.Where("account in (?)", []string{account1, account2}).Find(&devicesFromDB)
				Expect(dbResult.Error).To(BeNil())

				_, err := deviceGroupsService.AddDeviceGroupDevices(account1, deviceGroup1.ID, devicesFromDB)
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupAccountDevicesNotFound{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
	})

	Context("delete DeviceGroup devices", func() {
		account, err := common.GetAccountFromContext(ctx)
		It("should return account from context without error", func() {
			Expect(err).To(BeNil())
		})
		deviceGroupName := faker.Name()
		devices := []models.Device{
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
			},
			{
				Name:    faker.Name(),
				UUID:    faker.UUIDHyphenated(),
				Account: account,
			},
		}
		deviceGroup := &models.DeviceGroup{
			Name:    deviceGroupName,
			Type:    models.DeviceGroupTypeDefault,
			Account: account,
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
				deletedDevices, delErr := deviceGroupsService.DeleteDeviceGroupDevices(account, deviceGroupID, []models.Device{savedDeviceGroup.Devices[0]})
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

			It("should return error when device does not exist in device-group", func() {
				_, err := deviceGroupsService.DeleteDeviceGroupDevices(account, deviceGroupID, []models.Device{savedDeviceGroup.Devices[0]})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("devices not found in device group"))
			})

			It("should return error when account is undefined", func() {
				_, err := deviceGroupsService.DeleteDeviceGroupDevices("", deviceGroupID, []models.Device{savedDeviceGroup.Devices[0]})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("account or deviceGroupID undefined"))
			})

			It("should return error when deviceGroupId is undefined", func() {
				_, err := deviceGroupsService.DeleteDeviceGroupDevices(account, 0, []models.Device{savedDeviceGroup.Devices[0]})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("account or deviceGroupID undefined"))

			})

			It("should return error when devices not supplied", func() {
				_, err := deviceGroupsService.DeleteDeviceGroupDevices(account, deviceGroupID, []models.Device{})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("devices must be supplied to be added to or removed from device group"))
			})
		})
	})
})
