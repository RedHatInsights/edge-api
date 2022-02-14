package services_test

import (
	"context"
	"strconv"

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
		account, err := common.GetAccountFromContext(ctx)
		It("should return account from conext without error", func() {
			Expect(err).To(BeNil())
		})
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
		account, err := common.GetAccountFromContext(ctx)
		It("should return account from conext without error", func() {
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
		}
		deviceGroup := &models.DeviceGroup{
			Name:    deviceGroupName,
			Type:    models.DeviceGroupTypeDefault,
			Account: account,
			Devices: devices,
		}
		var deviceGroupDB models.DeviceGroup
		It("should create a DeviceGroup", func() {
			for _, device := range devices {
				err := db.DB.Create(&device).Error
				Expect(err).To(BeNil())
			}
			dg, err := deviceGroupsService.CreateDeviceGroup(deviceGroup)
			Expect(err).To(BeNil())
			Expect(dg).NotTo(BeNil())
		})
		It("should get the DeviceGroup ID", func() {
			dbResult := db.DB.Where("name = ?", deviceGroupName).First(&deviceGroupDB)
			Expect(dbResult.Error).To(BeNil())
			Expect(deviceGroupDB.ID).NotTo(BeZero())
		})
		When("deleting a DeviceGroup", func() {
			It("should delete the DeviceGroup", func() {
				err := deviceGroupsService.DeleteDeviceGroupByID(strconv.Itoa(int(deviceGroupDB.ID)))
				Expect(err).To(BeNil())
			})
			It("should not find the DeviceGroup", func() {
				dbResult := db.DB.Where("name = ?", deviceGroupName).First(&deviceGroupDB)
				Expect(dbResult.Error).NotTo(BeNil())
			})
			It("should not find the devices in the DeviceGroup", func() {
				var devicesFromDB []models.Device
				db.DB.Where("name in (?)", []string{devices[0].Name, devices[1].Name}).Find(&devicesFromDB)
				Expect(devicesFromDB).To(BeEmpty())
			})
		})
		It("should fail to delete a DeviceGroup with invalid ID", func() {
			err := deviceGroupsService.DeleteDeviceGroupByID("invalid-id")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("device group was not found"))
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
				err := db.DB.Create(&device).Error
				Expect(err).To(BeNil())
			}
			for _, deviceGroup := range deviceGroups {
				_, err := deviceGroupsService.CreateDeviceGroup(&deviceGroup)
				Expect(err).To(BeNil())
			}
		})
		var devicesFromDB1 []models.Device
		var deviceGroup1 models.DeviceGroup
		It("should add devices to DeviceGroups", func() {
			dbResult := db.DB.Where("name in (?)", []string{devices[0].Name, devices[1].Name}).Find(&devicesFromDB1)
			Expect(dbResult.Error).To(BeNil())

			dbResult = db.DB.Where("name = ?", deviceGroupName1).First(&deviceGroup1)
			Expect(dbResult.Error).To(BeNil())

			addedDevices, err := deviceGroupsService.AddDeviceGroupDevices(account1, deviceGroup1.ID, devicesFromDB1)
			Expect(err).To(BeNil())
			Expect(len(*addedDevices)).To(Equal(2))
		})
		When("re-adding devices", func() {
			It("should not return an error", func() {
				_, err := deviceGroupsService.AddDeviceGroupDevices(account1, deviceGroup1.ID, devicesFromDB1)
				Expect(err).To(BeNil())
			})
		})
		When("adding emtpy devices", func() {
			It("should fail", func() {
				_, err := deviceGroupsService.AddDeviceGroupDevices(account1, deviceGroup1.ID, []models.Device{})
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupDevicesNotSupplied{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
		When("adding with empty account", func() {
			It("should fail", func() {
				_, err := deviceGroupsService.AddDeviceGroupDevices("", deviceGroup1.ID, devicesFromDB1)
				Expect(err).NotTo(BeNil())
				expectedErr := services.DeviceGroupAccountOrIDUndefined{}
				Expect(err.Error()).To(Equal(expectedErr.Error()))
			})
		})
		When("adding with empty DeviceGroup ID", func() {
			It("should fail", func() {
				_, err := deviceGroupsService.AddDeviceGroupDevices(account1, 0, devicesFromDB1)
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
})
