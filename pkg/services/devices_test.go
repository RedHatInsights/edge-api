// FIXME: golangci-lint
// nolint:govet,revive,typecheck
package services_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gorm.io/gorm/clause"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory/mock_inventory"
	"github.com/redhatinsights/edge-api/pkg/common/test"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	log "github.com/sirupsen/logrus"
)

var seeder = test.NewSeeder()

var _ = Describe("DfseviceService", func() {
	var ctrl *gomock.Controller
	var mockInventoryClient *mock_inventory.MockClientInterface
	var deviceService services.DeviceService
	var mockImageService *mock_services.MockImageServiceInterface
	var uuid string
	orgID := faker.UUIDHyphenated()
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		uuid = faker.UUIDHyphenated()
		mockInventoryClient = mock_inventory.NewMockClientInterface(ctrl)
		mockImageService = mock_services.NewMockImageServiceInterface(ctrl)

		deviceService = services.DeviceService{
			Service:      services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
			Inventory:    mockInventoryClient,
			ImageService: mockImageService,
		}
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Context("get last deployment", func() {
		var device inventory.Device
		BeforeEach(func() {
			device = inventory.Device{}
		})
		When("list is empty", func() {
			It("should return nil for default values", func() {
				lastDeployment := deviceService.GetDeviceLastDeployment(device)
				Expect(lastDeployment).To(BeNil())
			})
			It("should return nil for empty list", func() {
				device.Ostree.RpmOstreeDeployments = make([]inventory.OSTree, 0)
				lastDeployment := deviceService.GetDeviceLastDeployment(device)
				Expect(lastDeployment).To(BeNil())
			})
		})
		When("deployment exists", func() {
			It("should return first if only one", func() {
				device.Ostree.RpmOstreeDeployments = make([]inventory.OSTree, 1)
				device.Ostree.RpmOstreeDeployments[0].Booted = false
				lastDeployment := deviceService.GetDeviceLastDeployment(device)
				Expect(lastDeployment).ToNot(BeNil())
				Expect(lastDeployment.Booted).To(BeFalse())
			})
			It("should return last if more than one", func() {
				device.Ostree.RpmOstreeDeployments = make([]inventory.OSTree, 2)
				device.Ostree.RpmOstreeDeployments[0].Booted = false
				device.Ostree.RpmOstreeDeployments[1].Booted = true
				lastDeployment := deviceService.GetDeviceLastDeployment(device)
				Expect(lastDeployment).ToNot(BeNil())
				Expect(lastDeployment.Booted).To(BeFalse())
			})
		})
	})
	Context("get last booted deployment", func() {
		var device inventory.Device
		BeforeEach(func() {
			device = inventory.Device{}
		})
		When("list is empty", func() {
			It("should return nil for default values", func() {
				lastDeployment := deviceService.GetDeviceLastDeployment(device)
				Expect(lastDeployment).To(BeNil())
			})
			It("should return nil for empty list", func() {
				device.Ostree.RpmOstreeDeployments = make([]inventory.OSTree, 0)
				lastDeployment := deviceService.GetDeviceLastDeployment(device)
				Expect(lastDeployment).To(BeNil())
			})
		})
		When("deployment exists", func() {
			It("should return nil if only one and not booted", func() {
				device.Ostree.RpmOstreeDeployments = make([]inventory.OSTree, 1)
				device.Ostree.RpmOstreeDeployments[0].Booted = false
				lastDeployment := deviceService.GetDeviceLastBootedDeployment(device)
				Expect(lastDeployment).To(BeNil())
			})
			It("should return nil if only one and booted", func() {
				device.Ostree.RpmOstreeDeployments = make([]inventory.OSTree, 1)
				device.Ostree.RpmOstreeDeployments[0].Booted = true
				lastDeployment := deviceService.GetDeviceLastBootedDeployment(device)
				Expect(lastDeployment).ToNot(BeNil())
				Expect(lastDeployment.Booted).To(BeTrue())
			})
			It("should return last if more than one and last is booted", func() {
				device.Ostree.RpmOstreeDeployments = make([]inventory.OSTree, 2)
				device.Ostree.RpmOstreeDeployments[0].Booted = false
				device.Ostree.RpmOstreeDeployments[1].Booted = true
				lastDeployment := deviceService.GetDeviceLastBootedDeployment(device)
				Expect(lastDeployment).ToNot(BeNil())
				Expect(lastDeployment.Booted).To(BeTrue())
			})
			It("should return first if more than one and last is not booted", func() {
				device.Ostree.RpmOstreeDeployments = make([]inventory.OSTree, 2)
				device.Ostree.RpmOstreeDeployments[0].Booted = true
				device.Ostree.RpmOstreeDeployments[1].Booted = false
				lastDeployment := deviceService.GetDeviceLastBootedDeployment(device)
				Expect(lastDeployment).ToNot(BeNil())
				Expect(lastDeployment.Booted).To(BeTrue())
			})
		})
	})
	Context("GetUpdateAvailableForDeviceByUUID", func() {
		When("error on InventoryAPI", func() {
			It("should return error and no updates available - for all updates", func() {
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(inventory.Response{}, errors.New("error on inventory api"))

				updatesAvailable, countUpdatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, false, 10, 0)
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
				Expect(updatesAvailable).To(BeNil())
				Expect(countUpdatesAvailable).To(Equal(int64(0)))
			})
			It("should return error and no updates available - for latest update", func() {
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(inventory.Response{}, errors.New("error on inventory api"))

				updatesAvailable, countUpdatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, true, 10, 0)
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
				Expect(updatesAvailable).To(BeNil())
				Expect(countUpdatesAvailable).To(Equal(int64(0)))
			})
		})
		When("device is not found on InventoryAPI", func() {
			It("should return error and nil updates available", func() {
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(inventory.Response{}, nil)

				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				updatesAvailable, countUpdatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, false, 10, 0)
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
				Expect(updatesAvailable).To(BeNil())
				Expect(countUpdatesAvailable).To(Equal(int64(0)))
			})
			It("should return error and nil on latest update available", func() {
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(inventory.Response{}, nil)

				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				updatesAvailable, countUpdatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, true, 10, 0)
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
				Expect(updatesAvailable).To(BeNil())
				Expect(countUpdatesAvailable).To(Equal(int64(0)))
			})
		})
		When("there are no booted deployments", func() {
			It("should return error and nil updates available", func() {
				checksum := "fake-checksum"
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: uuid, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: checksum, Booted: false},
						},
					},
						OrgID: orgID,
					},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				updatesAvailable, countUpdatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, false, 10, 0)
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
				Expect(updatesAvailable).To(BeNil())
				Expect(countUpdatesAvailable).To(Equal(int64(0)))
			})
			It("should return error and nil on latest update available", func() {
				checksum := "fake-checksum"
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: uuid, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: checksum, Booted: false},
						},
					},
						OrgID: orgID,
					},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				updatesAvailable, countUpdatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, true, 10, 0)
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
				Expect(updatesAvailable).To(BeNil())
				Expect(countUpdatesAvailable).To(Equal(int64(0)))
			})
		})
		When("everything is okay", func() {
			It("should return updates", func() {
				checksum := "fake-checksum"
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: uuid, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: checksum, Booted: true},
						},
					},
						OrgID: orgID},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

				_, imageSet := seeder.WithOstreeCommit(checksum).CreateImage()
				newImage, _ := seeder.WithInstalledPackages([]models.InstalledPackage{
					{
						Name:    "yum",
						Version: "3:6.0-1",
					},
					{
						Name:    "vim",
						Version: "2.0.0",
					},
					{
						Name:    "git",
						Version: "2.0.0",
					},
				}).WithImageSetID(imageSet.ID).CreateImage()

				updatesAvailable, countUpdatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, false, 10, 0)

				Expect(err).To(BeNil())
				Expect(updatesAvailable).To(HaveLen(1))
				newUpdate := updatesAvailable[0]
				Expect(newUpdate.Image.ID).To(Equal(newImage.ID))
				Expect(newUpdate.PackageDiff.Upgraded).To(HaveLen(1))
				Expect(newUpdate.PackageDiff.Added).To(HaveLen(2))
				Expect(newUpdate.PackageDiff.Removed).To(HaveLen(1))
				Expect(countUpdatesAvailable).To(Equal(int64(1)))
			})
			It("should return updates", func() {
				checksum := faker.UUIDHyphenated()
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: uuid, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: checksum, Booted: true},
						},
					},
						OrgID: orgID},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

				_, imageSet := seeder.WithOstreeCommit(checksum).CreateImage()

				seeder.WithInstalledPackages([]models.InstalledPackage{
					{
						Name:    "yum",
						Version: "3:6.0-1",
					},
					{
						Name:    "vim",
						Version: "2.0.0",
					},
				}).WithImageSetID(imageSet.ID).WithVersion(2).CreateImage()

				thirdImage, _ := seeder.WithInstalledPackages([]models.InstalledPackage{
					{
						Name:    "yum",
						Version: "3:6.0-1",
					},
					{
						Name:    "puppet",
						Version: "2.0.0",
					},
				}).WithImageSetID(imageSet.ID).WithVersion(3).CreateImage()

				updatesAvailable, countUpdatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, true, 10, 0)

				Expect(err).To(BeNil())
				Expect(updatesAvailable).To(HaveLen(1))
				newUpdate := updatesAvailable[0]
				Expect(newUpdate.Image.ID).To(Equal(thirdImage.ID))
				Expect(newUpdate.PackageDiff.Upgraded).To(HaveLen(1))
				Expect(newUpdate.PackageDiff.Added).To(HaveLen(1))
				Expect(newUpdate.PackageDiff.Removed).To(HaveLen(1))
				Expect(countUpdatesAvailable).To(Equal(int64(2)))
			})
		})
		When("no update is available", func() {
			It("should not return updates", func() {
				uuid := faker.UUIDHyphenated()
				checksum := "fake-checksum-2"
				resp := inventory.Response{
					Total: 1,
					Count: 1,
					Result: []inventory.Device{
						{
							ID: uuid,
							Ostree: inventory.SystemProfile{
								RHCClientID: faker.UUIDHyphenated(),
								RpmOstreeDeployments: []inventory.OSTree{
									{
										Checksum: checksum,
										Booted:   true,
									},
								},
							},
							OrgID: orgID},
					},
				}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

				seeder.WithOstreeCommit(checksum).CreateImage()

				updatesAvailable, countUpdatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, false, 10, 0)
				Expect(err).To(BeNil())
				Expect(updatesAvailable).To(BeNil())
				Expect(countUpdatesAvailable).To(Equal(int64(0)))
			})
		})
		When("no checksum is found", func() {
			It("should return device not found", func() {
				checksum := "fake-checksum-3"
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: uuid, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: checksum, Booted: true},
						},
					},
						OrgID: orgID,
					},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

				updatesAvailable, countUpdatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, false, 10, 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
				Expect(updatesAvailable).To(BeNil())
				Expect(countUpdatesAvailable).To(Equal(int64(0)))
			})
		})
	})
	Context("GetDiffOnUpdate", func() {
		oldImage := models.Image{
			Commit: &models.Commit{
				InstalledPackages: []models.InstalledPackage{
					{
						Name:    "vim",
						Version: "2.2",
					},
					{
						Name:    "ansible",
						Version: "1",
					},
					{
						Name:    "yum",
						Version: "2:6.0-1",
					},
					{
						Name:    "dnf",
						Version: "2:6.0-1",
					},
				},
				OrgID: orgID,
			},
			OrgID: orgID,
		}
		newImage := models.Image{
			Commit: &models.Commit{
				InstalledPackages: []models.InstalledPackage{
					{
						Name:    "zsh",
						Version: "1",
					},
					{
						Name:    "yum",
						Version: "2:6.0-2.el6",
					},
					{
						Name:    "dnf",
						Version: "2:6.0-1",
					},
				},
				OrgID: orgID,
			},
			OrgID: orgID,
		}
		newImageWithoutChanges := models.Image{
			Commit: &models.Commit{
				InstalledPackages: []models.InstalledPackage{
					{
						Name:    "vim",
						Version: "2.2",
					},
					{
						Name:    "ansible",
						Version: "1",
					},
					{
						Name:    "yum",
						Version: "2:6.0-1",
					},
					{
						Name:    "dnf",
						Version: "2:6.0-1",
					},
				},
				OrgID: orgID,
			},
			OrgID: orgID,
		}
		When("check package different between version", func() {
			It("should return different in case there are changes between version", func() {
				deltaDiff := services.GetDiffOnUpdate(oldImage, newImage)
				Expect(deltaDiff.Added).To(HaveLen(1))
				Expect(deltaDiff.Removed).To(HaveLen(2))
				Expect(deltaDiff.Upgraded).To(HaveLen(1))
			})

			It("should not return different in case there is no changes between version", func() {
				deltaDiff := services.GetDiffOnUpdate(oldImage, newImageWithoutChanges)
				Expect(deltaDiff.Added).To(HaveLen(0))
				Expect(deltaDiff.Removed).To(HaveLen(0))
				Expect(deltaDiff.Upgraded).To(HaveLen(0))
			})
		})
	})
	Context("GetImageForDeviceByUUID", func() {
		When("Image is found", func() {
			It("should return image", func() {
				checksum := "fake-checksum"
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: uuid, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: checksum, Booted: true},
						},
					},
						OrgID: orgID,
					},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil).Times(2)

				oldImage, imageSet := seeder.WithOstreeCommit(checksum).CreateImage()
				newImage, _ := seeder.WithImageSetID(imageSet.ID).WithVersion(2).
					WithInstalledPackages([]models.InstalledPackage{
						{Name: "vim"},
					}).CreateImage()

				mockImageService.EXPECT().GetImageByOSTreeCommitHash(gomock.Eq(checksum)).Return(newImage, nil)
				mockImageService.EXPECT().GetRollbackImage(gomock.Eq(newImage)).Return(oldImage, nil)

				imageInfo, err := deviceService.GetDeviceImageInfoByUUID(uuid, 10, 0)
				Expect(err).To(BeNil())
				Expect(oldImage.Commit.OSTreeCommit).To(Equal(imageInfo.Rollback.Commit.OSTreeCommit))
				Expect(newImage.Commit.OSTreeCommit).To(Equal(imageInfo.Image.Commit.OSTreeCommit))
				Expect(newImage.Commit.OSTreeCommit).To(Equal(imageInfo.Image.Commit.OSTreeCommit))
				Expect(imageInfo.Image.TotalPackages).To(Equal(1))
			})

		})
		When("Image is not found", func() {
			It("should return image not found", func() {
				checksum := "123"
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: uuid, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: checksum, Booted: true},
						},
					},
						OrgID: orgID,
					},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)
				mockImageService.EXPECT().GetImageByOSTreeCommitHash(gomock.Eq(checksum)).Return(nil, errors.New("Not found"))

				_, err := deviceService.GetDeviceImageInfoByUUID(uuid, 10, 0)
				Expect(err).To(MatchError(new(services.ImageNotFoundError)))
			})
		})
	})
	Context("GetDevices", func() {
		When("no devices are returned from InventoryAPI", func() {
			It("should return zero devices", func() {
				params := new(inventory.Params)
				resp := inventory.Response{
					Total: 0,
					Count: 0,
				}
				mockInventoryClient.EXPECT().ReturnDevices(gomock.Any()).Return(resp, nil)
				devices, err := deviceService.GetDevices(params)
				Expect(err).To(BeNil())
				Expect(devices).ToNot(BeNil())
				Expect(devices.Devices).To(HaveLen(0))
				Expect(devices.Count).To(Equal(0))
				Expect(devices.Total).To(Equal(0))
			})
		})
		When("devices are returned from InventoryAPI", func() {
			It("should return devices", func() {
				params := new(inventory.Params)
				deviceWithImage := models.Device{}
				deviceWithNoImage := models.Device{}
				mockInventoryClient.EXPECT().ReturnDevices(gomock.Eq(params)).Return(inventory.Response{
					Total: 2,
					Count: 2,
					Result: []inventory.Device{{
						ID:          deviceWithImage.UUID,
						DisplayName: "oi",
						LastSeen:    "b",
						OrgID:       orgID,
						Ostree:      inventory.SystemProfile{RHCClientID: "", RpmOstreeDeployments: []inventory.OSTree{}},
					}, {
						ID:          deviceWithNoImage.UUID,
						DisplayName: "oi",
						LastSeen:    "b",
						OrgID:       orgID,
						Ostree:      inventory.SystemProfile{RHCClientID: "", RpmOstreeDeployments: []inventory.OSTree{}},
					}},
				}, nil)
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Any()).AnyTimes().Return(inventory.Response{}, new(services.DeviceNotFoundError))
				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				devices, err := deviceService.GetDevices(params)
				Expect(err).To(BeNil())
				Expect(devices).ToNot(BeNil())
				Expect(devices.Devices).To(HaveLen(2))
				Expect(devices.Count).To(Equal(2))
				Expect(devices.Total).To(Equal(2))
			})
		})
	})
	Context("ProcessPlatformInventoryCreateEvent", func() {

		commit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated()}
		result := db.DB.Create(&commit)
		Expect(result.Error).To(BeNil())
		image := models.Image{OrgID: orgID, CommitID: commit.ID, Status: models.ImageStatusSuccess}
		result = db.DB.Create(&image)
		Expect(result.Error).To(BeNil())
		var message []byte
		var err error
		var event services.PlatformInsightsCreateUpdateEventPayload
		BeforeEach(func() {

			event.Type = services.InventoryEventTypeCreated
			event.Host.SystemProfile.HostType = services.InventoryHostTypeEdge
			event.Host.ID = faker.UUIDHyphenated()
			event.Host.OrgID = orgID
			event.Host.Name = faker.UUIDHyphenated()
			event.Host.Updated = models.EdgeAPITime(sql.NullTime{Time: time.Now().UTC(), Valid: true})
			event.Host.SystemProfile.RpmOSTreeDeployments = []services.RpmOSTreeDeployment{{Booted: true, Checksum: commit.OSTreeCommit}}
			event.Host.SystemProfile.RHCClientID = faker.UUIDHyphenated()
			message, err = json.Marshal(event)
			Expect(err).To(BeNil())
		})
		It("should create devices when no record is found", func() {
			err = deviceService.ProcessPlatformInventoryCreateEvent(message)
			Expect(err).To(BeNil())
			var savedDevice models.Device
			result := db.DB.Where(models.Device{UUID: event.Host.ID, OrgID: event.Host.OrgID}).First(&savedDevice).Unscoped()
			Expect(result.Error).To(BeNil())
			Expect(savedDevice.UUID).To(Equal(event.Host.ID))
			Expect(savedDevice.OrgID).To(Equal(orgID))
			Expect(savedDevice.ImageID).To(Equal(image.ID))
			Expect(savedDevice.LastSeen.Time).To(Equal(event.Host.Updated.Time))
			Expect(savedDevice.Name).To(Equal(event.Host.Name))
			Expect(savedDevice.RHCClientID).To(Equal(event.Host.SystemProfile.RHCClientID))
		})

		It("should NOT create devices when record is found", func() {
			var newDevice = models.Device{
				UUID:        event.Host.ID,
				RHCClientID: event.Host.SystemProfile.RHCClientID,
				OrgID:       event.Host.OrgID,
				Name:        event.Host.Name,
				LastSeen:    event.Host.Updated,
			}
			result = db.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&newDevice)
			err := deviceService.ProcessPlatformInventoryCreateEvent(message)
			Expect(err).To(BeNil())
			var savedDevice models.Device
			result := db.DB.Where(models.Device{UUID: event.Host.ID, OrgID: event.Host.OrgID}).First(&savedDevice).Unscoped()
			Expect(result.Error).To(BeNil())
			Expect(&savedDevice.ID).To(Equal(&newDevice.ID))
			Expect(savedDevice.UUID).To(Equal(newDevice.UUID))

			var total []models.Device
			db.DB.Where(models.Device{UUID: event.Host.ID, OrgID: event.Host.OrgID}).Find(&total).Debug()
			Expect(len(total)).To(Equal(1))
		})
	})
	Context("ProcessPlatformInventoryUpdatedEvent", func() {
		commit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated()}
		result := db.DB.Create(&commit)
		Expect(result.Error).To(BeNil())
		imageSet := models.ImageSet{Name: faker.UUIDHyphenated(), OrgID: orgID}
		result = db.DB.Create(&imageSet)
		Expect(result.Error).To(BeNil())
		image := models.Image{OrgID: orgID, CommitID: commit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
		result = db.DB.Create(&image)
		Expect(result.Error).To(BeNil())

		It("should create a device when device does not exist", func() {
			event := new(services.PlatformInsightsCreateUpdateEventPayload)
			event.Type = services.InventoryEventTypeUpdated
			event.Host.ID = faker.UUIDHyphenated()
			event.Host.InsightsID = faker.UUIDHyphenated()
			event.Host.OrgID = orgID
			event.Host.Name = faker.UUIDHyphenated()
			event.Host.Updated = models.EdgeAPITime(sql.NullTime{Time: time.Now().UTC(), Valid: true})
			event.Host.SystemProfile.HostType = services.InventoryHostTypeEdge
			event.Host.SystemProfile.RpmOSTreeDeployments = []services.RpmOSTreeDeployment{{Booted: true, Checksum: commit.OSTreeCommit}}
			event.Host.SystemProfile.RHCClientID = faker.UUIDHyphenated()
			message, err := json.Marshal(event)
			Expect(err).To(BeNil())

			err = deviceService.ProcessPlatformInventoryUpdatedEvent(message)
			Expect(err).To(BeNil())

			var device models.Device
			res := db.DB.Where("uuid = ?", event.Host.ID).First(&device)
			Expect(res.Error).To(BeNil())
			Expect(device.OrgID).To(Equal(orgID))
			Expect(device.RHCClientID).To(Equal(event.Host.SystemProfile.RHCClientID))
			Expect(device.ImageID).To(Equal(image.ID))
			Expect(device.UpdateAvailable).To(Equal(false))
			Expect(device.LastSeen.Time).To(Equal(event.Host.Updated.Time))
			Expect(device.Name).To(Equal(event.Host.Name))
		})

		It("should update device OrgID, name, lastSeen, image_id and update availability when device already exists", func() {
			// Creating a devices needs to have org_id because of BeforeCreate method applied to Devices model
			device := models.Device{
				UUID:            faker.UUIDHyphenated(),
				UpdateAvailable: true,
				OrgID:           orgID,
			}
			res := db.DB.Create(&device)
			Expect(res.Error).To(BeNil())

			event := new(services.PlatformInsightsCreateUpdateEventPayload)
			event.Type = services.InventoryEventTypeUpdated
			event.Host.ID = device.UUID
			event.Host.InsightsID = faker.UUIDHyphenated()
			event.Host.OrgID = orgID
			event.Host.Name = faker.UUIDHyphenated()
			event.Host.Updated = models.EdgeAPITime(sql.NullTime{Time: time.Now().UTC(), Valid: true})
			event.Host.SystemProfile.HostType = services.InventoryHostTypeEdge
			event.Host.SystemProfile.RpmOSTreeDeployments = []services.RpmOSTreeDeployment{{Booted: true, Checksum: commit.OSTreeCommit}}
			event.Host.SystemProfile.RHCClientID = faker.UUIDHyphenated()
			message, err := json.Marshal(event)
			Expect(err).To(BeNil())

			err = deviceService.ProcessPlatformInventoryUpdatedEvent(message)
			Expect(err).To(BeNil())

			var savedDevice models.Device
			res = db.DB.Where("uuid = ?", device.UUID).First(&savedDevice)
			Expect(res.Error).To(BeNil())
			Expect(savedDevice.OrgID).To(Equal(orgID))
			Expect(savedDevice.ImageID).To(Equal(image.ID))
			Expect(savedDevice.UpdateAvailable).To(Equal(false))
			Expect(savedDevice.RHCClientID).To(Equal(event.Host.SystemProfile.RHCClientID))
			Expect(savedDevice.LastSeen.Time).To(Equal(event.Host.Updated.Time))
			Expect(savedDevice.Name).To(Equal(event.Host.Name))
		})

		Context("device update availability", func() {
			device := models.Device{
				UUID:            faker.UUIDHyphenated(),
				RHCClientID:     faker.UUIDHyphenated(),
				OrgID:           orgID,
				ImageID:         image.ID,
				UpdateAvailable: false,
			}
			res := db.DB.Create(&device)
			Expect(res.Error).To(BeNil())

			event := new(services.PlatformInsightsCreateUpdateEventPayload)
			event.Type = services.InventoryEventTypeUpdated
			event.Host.ID = device.UUID
			event.Host.InsightsID = device.RHCClientID
			event.Host.OrgID = orgID
			event.Host.Name = faker.UUIDHyphenated()
			event.Host.Updated = models.EdgeAPITime(sql.NullTime{Time: time.Now().UTC(), Valid: true})
			event.Host.SystemProfile.HostType = services.InventoryHostTypeEdge
			event.Host.SystemProfile.RpmOSTreeDeployments = []services.RpmOSTreeDeployment{{Booted: true, Checksum: commit.OSTreeCommit}}
			message, err := json.Marshal(event)
			Expect(err).To(BeNil())

			It("should not set update available when an image update failed", func() {
				newImage := models.Image{OrgID: orgID, ImageSetID: &imageSet.ID, Status: models.ImageStatusError}
				result = db.DB.Create(&newImage)
				Expect(result.Error).To(BeNil())

				err = deviceService.ProcessPlatformInventoryUpdatedEvent(message)
				Expect(err).To(BeNil())

				var savedDevice models.Device
				res = db.DB.Where("uuid = ?", device.UUID).First(&savedDevice)
				Expect(res.Error).To(BeNil())
				Expect(savedDevice.OrgID).To(Equal(orgID))
				Expect(savedDevice.ImageID).To(Equal(image.ID))
				Expect(savedDevice.UpdateAvailable).To(Equal(false))
			})

			It("should set update available when an image is updated successfully", func() {
				newImage := models.Image{OrgID: orgID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
				result = db.DB.Create(&newImage)
				Expect(result.Error).To(BeNil())

				err = deviceService.ProcessPlatformInventoryUpdatedEvent(message)
				Expect(err).To(BeNil())

				var savedDevice models.Device
				res = db.DB.Where("uuid = ?", device.UUID).First(&savedDevice)
				Expect(res.Error).To(BeNil())
				Expect(savedDevice.OrgID).To(Equal(orgID))
				Expect(savedDevice.ImageID).To(Equal(image.ID))
				Expect(savedDevice.UpdateAvailable).To(Equal(true))
			})
		})
	})

	Context("ProcessPlatformInventoryDeleteEvent", func() {

		It("device should be deleted", func() {
			// create a platform inventory delete event message
			event := new(services.PlatformInsightsDeleteEventPayload)
			event.Type = services.InventoryEventTypeDelete
			event.ID = faker.UUIDHyphenated()
			event.OrgID = faker.UUIDHyphenated()
			message, err := json.Marshal(event)
			Expect(err).To(BeNil())

			// create a device
			device := models.Device{UUID: event.ID, OrgID: event.OrgID}
			result := db.DB.Create(&device)
			Expect(result.Error).To(BeNil())

			// ensure device created
			var deviceCount int64
			db.Org(event.OrgID, "").Model(&models.Device{}).Where("uuid = ?", event.ID).Count(&deviceCount)
			Expect(deviceCount == 1).To(BeTrue())

			// call the platform inventory delete event processor
			err = deviceService.ProcessPlatformInventoryDeleteEvent(message)
			Expect(err).To(BeNil())

			// ensure device does not exits
			db.Org(event.OrgID, "").Model(&models.Device{}).Where("uuid = ?", event.ID).Count(&deviceCount)
			Expect(deviceCount == 0).To(BeTrue())
		})

		It("device in device-groups should be removed", func() {
			// create a platform inventory delete event message
			event := new(services.PlatformInsightsDeleteEventPayload)
			event.Type = services.InventoryEventTypeDelete
			event.ID = faker.UUIDHyphenated()
			event.OrgID = faker.UUIDHyphenated()
			message, err := json.Marshal(event)
			Expect(err).To(BeNil())

			// create a device
			device := models.Device{UUID: event.ID, OrgID: event.OrgID}
			result := db.DB.Create(&device)
			Expect(result.Error).To(BeNil())

			// ensure the device exists
			var deviceCount int64
			result = db.Org(event.OrgID, "").Model(&models.Device{}).Where("uuid = ?", event.ID).Count(&deviceCount)
			Expect(result.Error).To(BeNil())
			Expect(deviceCount == 1).To(BeTrue())

			// create a device group with device
			deviceGroup := models.DeviceGroup{
				Type: models.DeviceGroupTypeDefault, OrgID: event.OrgID, Name: faker.UUIDHyphenated(),
				Devices: []models.Device{device},
			}
			result = db.DB.Omit("Devices.*").Create(&deviceGroup)
			Expect(result.Error).To(BeNil())

			// ensure device group created with device included
			var savedDeviceGroup models.DeviceGroup
			result = db.Org(deviceGroup.OrgID, "").Preload("Devices").First(&savedDeviceGroup, deviceGroup.ID)
			Expect(result.Error).To(BeNil())
			Expect(savedDeviceGroup.Devices).NotTo(BeEmpty())
			Expect(savedDeviceGroup.Devices[0].ID == device.ID).To(BeTrue())

			// call the platform inventory delete event processor
			err = deviceService.ProcessPlatformInventoryDeleteEvent(message)
			Expect(err).To(BeNil())

			// ensure device does not exits
			result = db.Org(event.OrgID, "").Model(&models.Device{}).Where("uuid = ?", event.ID).Count(&deviceCount)
			Expect(result.Error).To(BeNil())
			Expect(deviceCount == 0).To(BeTrue())

			// ensure device does not exists in device group
			result = db.Org(event.OrgID, "").Preload("Devices").First(&savedDeviceGroup, deviceGroup.ID)
			Expect(result.Error).To(BeNil())
			Expect(savedDeviceGroup.Devices).To(BeEmpty())
		})
	})
	Context("GetDeviceDetailsByUUID", func() {
		var orgID string
		var imageV1 *models.Image
		var deviceWithImage models.Device
		var dispatchRecord *models.DispatchRecord
		var update models.UpdateTransaction

		var deviceService services.DeviceService

		BeforeEach(func() {
			defer GinkgoRecover()
			orgID = common.DefaultOrgID

			imageV1, _ = seeder.CreateImage()
			deviceWithImage = *seeder.WithImageID(imageV1.ID).CreateDevice()

			dispatchRecord = &models.DispatchRecord{
				PlaybookDispatcherID: faker.UUIDHyphenated(),
				Status:               models.DispatchRecordStatusComplete,
				DeviceID:             deviceWithImage.ID,
			}
			db.DB.Omit("Devices.*").Create(dispatchRecord)

			update = models.UpdateTransaction{
				DispatchRecords: []models.DispatchRecord{*dispatchRecord},
				Devices: []models.Device{
					deviceWithImage,
				},
				OrgID:  orgID,
				Status: models.UpdateStatusSuccess,
			}
			db.DB.Omit("Devices.*").Create(&update)

			deviceService = services.DeviceService{
				Service:       services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
				ImageService:  services.NewImageService(context.Background(), log.NewEntry(log.StandardLogger())),
				UpdateService: services.NewUpdateService(context.Background(), log.NewEntry(log.StandardLogger())),
				Inventory:     mockInventoryClient,
			}
		})

		When("device exists", func() {
			It("should return the device", func() {
				invDevice := inventory.Device{
					ID:    deviceWithImage.UUID,
					OrgID: orgID,
					Ostree: inventory.SystemProfile{
						RpmOstreeDeployments: []inventory.OSTree{
							{
								Booted:   true,
								Checksum: imageV1.Commit.OSTreeCommit,
							},
						},
					},
				}
				invResult := []inventory.Device{invDevice}
				resp := inventory.Response{
					Total:  1,
					Count:  1,
					Result: invResult,
				}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Any()).Return(resp, nil)

				deviceDetails, err := deviceService.GetDeviceDetailsByUUID(deviceWithImage.UUID, 10, 0)
				Expect(err).To(BeNil())
				Expect(deviceDetails).ToNot(BeNil())
				Expect(deviceDetails.Device.ID).To(Equal(deviceWithImage.ID))
				Expect(deviceDetails.Device.UUID).To(Equal(deviceWithImage.UUID))
				Expect(deviceDetails.Device.Name).To(Equal(deviceWithImage.Name))
				Expect(deviceDetails.Device.UpdateAvailable).To(BeFalse())
				Expect(deviceDetails.Device.ImageID).To(Equal(imageV1.ID))
				Expect(deviceDetails.Device.OrgID).To(Equal(orgID))

				imageInfo := deviceDetails.Image
				Expect(imageInfo.Image.ID).To(Equal(imageV1.ID))
				Expect(imageInfo.Image.Name).To(Equal(imageV1.Name))
				Expect(imageInfo.Image.OrgID).To(Equal(orgID))
				Expect(imageInfo.UpdatesAvailable).To(BeNil())
				Expect(imageInfo.Rollback).To(BeNil())

				deviceUpdate := (*deviceDetails.UpdateTransactions)[0]

				Expect(deviceUpdate.ID).To(Equal(update.ID))
				Expect(deviceUpdate.Status).To(Equal(models.UpdateStatusSuccess))

				Expect(len(deviceUpdate.DispatchRecords)).To(Equal(1))
				Expect(deviceUpdate.DispatchRecords[0].DeviceID).To(Equal(deviceWithImage.ID))
				Expect(deviceUpdate.DispatchRecords[0].PlaybookDispatcherID).To(Equal(dispatchRecord.PlaybookDispatcherID))
				Expect(deviceUpdate.DispatchRecords[0].Status).To(Equal(models.DispatchRecordStatusComplete))
			})
		})
	})

	Context("GetDeviceView", func() {
		When("devices are returned from the db", func() {
			It("should return devices", func() {
				defer GinkgoRecover()
				orgID := common.DefaultOrgID

				imageV1, _ := seeder.CreateImage()

				deviceUnresponsive := models.Device{OrgID: orgID, ImageID: imageV1.ID, UUID: faker.UUIDHyphenated()}
				result := db.DB.Create(&deviceUnresponsive)
				Expect(result.Error).To(BeNil())

				deviceSuccess := models.Device{OrgID: orgID, ImageID: imageV1.ID, UUID: faker.UUIDHyphenated()}
				result = db.DB.Create(&deviceSuccess)
				Expect(result.Error).To(BeNil())

				deviceErrorFailure := models.Device{OrgID: orgID, ImageID: imageV1.ID, UUID: faker.UUIDHyphenated()}
				result = db.DB.Create(&deviceErrorFailure)
				Expect(result.Error).To(BeNil())

				deviceErrorTimeout := models.Device{OrgID: orgID, ImageID: imageV1.ID, UUID: faker.UUIDHyphenated()}
				result = db.DB.Create(&deviceErrorTimeout)
				Expect(result.Error).To(BeNil())

				deviceBuilding := models.Device{OrgID: orgID, ImageID: imageV1.ID, UUID: faker.UUIDHyphenated()}
				result = db.DB.Create(&deviceBuilding)
				Expect(result.Error).To(BeNil())

				deviceRunning := models.Device{OrgID: orgID, ImageID: imageV1.ID, UUID: faker.UUIDHyphenated()}
				result = db.DB.Create(&deviceRunning)
				Expect(result.Error).To(BeNil())

				dispatchRecord := &models.DispatchRecord{
					PlaybookDispatcherID: faker.UUIDHyphenated(),
					Status:               models.DispatchRecordStatusError,
					DeviceID:             deviceUnresponsive.ID,
				}
				db.DB.Omit("Devices.*").Create(dispatchRecord)

				update := models.UpdateTransaction{
					DispatchRecords: []models.DispatchRecord{*dispatchRecord},
					Devices: []models.Device{
						deviceUnresponsive,
					},
					OrgID:  orgID,
					Status: models.UpdateStatusDeviceDisconnected,
				}
				db.DB.Omit("Devices.*").Create(&update)

				dispatchRecord2 := &models.DispatchRecord{
					PlaybookDispatcherID: faker.UUIDHyphenated(),
					Status:               models.DispatchRecordStatusComplete,
					DeviceID:             deviceSuccess.ID,
				}
				db.DB.Omit("Devices.*").Create(dispatchRecord2)

				dispatchRecord3 := &models.DispatchRecord{
					PlaybookDispatcherID: faker.UUIDHyphenated(),
					Status:               models.DispatchRecordStatusError,
					Reason:               models.UpdateReasonFailure,
					DeviceID:             deviceErrorFailure.ID,
				}
				db.DB.Omit("Devices.*").Create(dispatchRecord3)

				update2 := models.UpdateTransaction{
					DispatchRecords: []models.DispatchRecord{*dispatchRecord2, *dispatchRecord3},
					Devices: []models.Device{
						deviceSuccess,
						deviceErrorFailure,
					},
					OrgID:  orgID,
					Status: models.UpdateStatusSuccess,
				}
				db.DB.Omit("Devices.*").Create(&update2)

				dispatchRecord4 := &models.DispatchRecord{
					PlaybookDispatcherID: faker.UUIDHyphenated(),
					Status:               models.DispatchRecordStatusRunning,
					DeviceID:             deviceBuilding.ID,
				}
				db.DB.Omit("Devices.*").Create(dispatchRecord4)

				update4 := models.UpdateTransaction{
					DispatchRecords: []models.DispatchRecord{*dispatchRecord4},
					Devices: []models.Device{
						deviceBuilding,
					},
					OrgID:  orgID,
					Status: models.UpdateStatusBuilding,
				}
				db.DB.Omit("Devices.*").Create(&update4)

				dispatchRecord5 := &models.DispatchRecord{
					PlaybookDispatcherID: faker.UUIDHyphenated(),
					Status:               models.DispatchRecordStatusError,
					Reason:               models.UpdateReasonTimeout,
					DeviceID:             deviceErrorTimeout.ID,
				}
				db.DB.Omit("Devices.*").Create(dispatchRecord5)

				update5 := models.UpdateTransaction{
					DispatchRecords: []models.DispatchRecord{*dispatchRecord5},
					Devices: []models.Device{
						deviceErrorTimeout,
					},
					OrgID:  orgID,
					Status: models.UpdateStatusError,
				}
				db.DB.Omit("Devices.*").Create(&update5)

				invResult := []inventory.Device{
					{
						ID:    deviceUnresponsive.UUID,
						OrgID: orgID,
					},
					{
						ID:    deviceSuccess.UUID,
						OrgID: orgID,
					},
					{
						ID:    deviceErrorFailure.UUID,
						OrgID: orgID,
					},
					{
						ID:    deviceErrorTimeout.UUID,
						OrgID: orgID,
					},
					{
						ID:    deviceBuilding.UUID,
						OrgID: orgID,
					},
					{
						ID:    deviceRunning.UUID,
						OrgID: orgID,
					},
				}
				resp := inventory.Response{
					Total:  6,
					Count:  6,
					Result: invResult,
				}
				// calls to inventory are now in go routines.
				// in order for the mocks to stay active for the duration of the tests, wait groups were added
				// these calls to inventory can happen more than once so the `AnyTimes()` param was added
				// each call was added to a unique wait group in the `Do` wrapper
				wg := sync.WaitGroup{}
				mockInventoryClient.EXPECT().ReturnDevices(gomock.Any()).Return(resp, nil).AnyTimes().Do(func(arg interface{}) {
					wg.Add(1)
					defer wg.Done()
				})
				wg2 := sync.WaitGroup{}
				mockInventoryClient.EXPECT().ReturnDeviceListByID(gomock.Any()).Return(resp, nil).AnyTimes().Do(func(arg interface{}) {
					wg2.Add(1)
					defer wg2.Done()
				})

				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				dbFilter := db.DB.Model(models.Device{}).Where("devices.image_id = ?", imageV1.ID).Order("devices.created_at ASC")
				devices, err := deviceService.GetDevicesView(0, 0, dbFilter)
				wg.Wait()
				wg2.Wait()
				Expect(err).To(BeNil())
				Expect(devices).ToNot(BeNil())
				Expect(len(devices.Devices)).To(Equal(6))

				Expect(devices.Devices[0].DispatcherStatus).To(Equal(models.UpdateStatusDeviceUnresponsive))
				Expect(devices.Devices[0].Status).To(Equal(models.DeviceViewStatusRunning))
				Expect(devices.Devices[0].DispatcherReason).To(BeEmpty())

				Expect(devices.Devices[1].DispatcherStatus).To(Equal(models.UpdateStatusSuccess))
				Expect(devices.Devices[1].Status).To(Equal(models.DeviceViewStatusRunning))
				Expect(devices.Devices[1].DispatcherReason).To(BeEmpty())

				Expect(devices.Devices[2].DispatcherStatus).To(Equal(models.DispatchRecordStatusError))
				Expect(devices.Devices[2].Status).To(Equal(models.DeviceViewStatusRunning))
				Expect(devices.Devices[2].DispatcherReason).To(Equal(models.UpdateReasonFailure))

				Expect(devices.Devices[3].DispatcherStatus).To(Equal(models.DispatchRecordStatusError))
				Expect(devices.Devices[3].Status).To(Equal(models.DeviceViewStatusRunning))
				Expect(devices.Devices[3].DispatcherReason).To(Equal(models.UpdateReasonTimeout))

				Expect(devices.Devices[4].DispatcherStatus).To(Equal(models.DispatchRecordStatusRunning))
				Expect(devices.Devices[4].Status).To(Equal(models.DeviceViewStatusUpdating))
				Expect(devices.Devices[4].DispatcherReason).To(BeEmpty())

				Expect(devices.Devices[5].DispatcherStatus).To(BeEmpty())
				Expect(devices.Devices[5].Status).To(Equal(models.DeviceViewStatusRunning))
				Expect(devices.Devices[5].DispatcherReason).To(BeEmpty())
			})

			It("should sync devices with inventory", func() {
				defer GinkgoRecover()
				seeder.CreateDevice()

				invResult := []inventory.Device{}
				resp := inventory.Response{
					Total:  0,
					Count:  0,
					Result: invResult,
				}
				wg := sync.WaitGroup{}
				mockInventoryClient.EXPECT().ReturnDevices(gomock.Any()).Return(resp, nil).AnyTimes().Do(func(arg interface{}) {
					wg.Add(1)
					defer wg.Done()
				})
				wg2 := sync.WaitGroup{}
				mockInventoryClient.EXPECT().ReturnDeviceListByID(gomock.Any()).Return(resp, nil).AnyTimes().Do(func(arg interface{}) {
					wg2.Add(1)
					defer wg2.Done()
				})

				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				devices, err := deviceService.GetDevicesView(0, 0, nil)
				wg.Wait()
				wg2.Wait()
				Expect(err).To(BeNil())
				Expect(devices).ToNot(BeNil())
			})

			It("should sync inventory with devices", func() {
				defer GinkgoRecover()
				orgID := common.DefaultOrgID
				deviceWithImage := seeder.CreateDevice()

				invDevice := inventory.Device{
					ID:    deviceWithImage.UUID,
					OrgID: orgID,
				}
				invDevice2 := inventory.Device{
					ID:    faker.UUIDHyphenated(),
					OrgID: orgID,
				}
				invResult := []inventory.Device{invDevice, invDevice2}
				resp := inventory.Response{
					Total:  2,
					Count:  2,
					Result: invResult,
				}
				// calls to inventory are now in go routines.
				// in order for the mocks to stay active for the duration of the tests, wait groups were added
				// these calls to inventory can happen more than once so the `AnyTimes()` param was added
				// each call was added to a unique wait group in the `Do` wrapper
				wg := sync.WaitGroup{}
				mockInventoryClient.EXPECT().ReturnDevices(gomock.Any()).Return(resp, nil).AnyTimes().Do(func(arg interface{}) {
					wg.Add(1)
					defer wg.Done()
				})
				wg2 := sync.WaitGroup{}
				mockInventoryClient.EXPECT().ReturnDeviceListByID(gomock.Any()).Return(resp, nil).AnyTimes().Do(func(arg interface{}) {
					wg2.Add(1)
					defer wg2.Done()
				})

				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				devices, err := deviceService.GetDevicesView(0, 0, nil)
				wg.Wait()
				wg2.Wait()
				Expect(err).To(BeNil())
				Expect(devices).ToNot(BeNil())
			})
		})
	})
	Context("Get CommitID from Device Image", func() {
		It("should return zero images", func() {
			device := models.Device{
				OrgID: orgID,
				UUID:  faker.UUIDHyphenated(),
			}
			db.DB.Create(&device)
			devicesUUID := []string{device.UUID}
			updateImageCommitID, err := deviceService.GetLatestCommitFromDevices(orgID, devicesUUID)
			Expect(updateImageCommitID == 0).To(BeTrue())
			Expect(err).To(MatchError(new(services.DeviceHasImageUndefined)))
			Expect(err).ToNot(BeNil())
		})
	})
	When("device Image does not have update", func() {
		It("should return no image updates", func() {
			orgID := faker.HyphenatedID
			imageSet := &models.ImageSet{
				OrgID:   orgID,
				Name:    "test",
				Version: 1,
			}
			updateImage := models.Image{
				OrgID:      orgID,
				ImageSetID: &imageSet.ID,
				Status:     models.ImageStatusSuccess,
			}
			device := models.Device{
				UUID: faker.UUIDHyphenated(),
			}
			devicesUUID := []string{device.UUID}
			updateImageCommitID, err := deviceService.GetLatestCommitFromDevices(updateImage.OrgID, devicesUUID)
			Expect(updateImageCommitID == 0).To(BeTrue())
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError(new(services.DeviceHasNoImageUpdate)))
		})
	})
	When("device Image have update", func() {
		It("should return commitID", func() {
			orgID := faker.HyphenatedID
			imageSet := models.ImageSet{
				OrgID: orgID,
			}
			db.DB.Create(&imageSet)

			firstCommit := models.Commit{
				OrgID: orgID,
			}
			db.DB.Create(&firstCommit)

			firstImage := models.Image{
				OrgID:      orgID,
				CommitID:   firstCommit.ID,
				Status:     models.ImageStatusSuccess,
				Version:    1,
				ImageSetID: &imageSet.ID,
			}
			db.DB.Create(&firstImage)
			device := models.Device{
				OrgID:   orgID,
				ImageID: firstImage.ID,
			}
			db.DB.Create(&device)
			secondCommit := models.Commit{
				OrgID: orgID,
			}
			db.DB.Create(&secondCommit)

			secondImage := models.Image{
				OrgID:      orgID,
				CommitID:   secondCommit.ID,
				Status:     models.ImageStatusSuccess,
				Version:    2,
				ImageSetID: &imageSet.ID,
			}
			devicesUUID := []string{device.UUID}

			db.DB.Create(&secondImage)
			updateImageCommitID, err := deviceService.GetLatestCommitFromDevices(device.OrgID, devicesUUID)
			Expect(err).To(BeNil())
			Expect(updateImageCommitID).To(Equal(secondCommit.ID))
		})
	})

	Context("Get Device Image Info", func() {
		It("should return full image info", func() {
			checksum := "fake-checksum"
			resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
				{ID: uuid, Ostree: inventory.SystemProfile{
					RHCClientID: faker.UUIDHyphenated(),
					RpmOstreeDeployments: []inventory.OSTree{
						{Checksum: checksum, Booted: true},
					},
				},
					OrgID: orgID,
				},
			}}
			mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).
				Return(resp, nil).Times(1)

			oldImage, imageSet := seeder.CreateImage()
			newImage, _ := seeder.WithImageSetID(imageSet.ID).WithVersion(2).CreateImage()
			seeder.WithImageSetID(imageSet.ID).WithVersion(3).CreateImage()

			device := models.Device{
				OrgID:   "00000000",
				UUID:    faker.UUIDHyphenated(),
				ImageID: newImage.ID,
			}

			db.DB.Create(&device)

			mockImageService.EXPECT().GetImageByOSTreeCommitHash(gomock.Eq(checksum)).Return(newImage, nil)
			mockImageService.EXPECT().GetRollbackImage(gomock.Eq(newImage)).Return(oldImage, nil)
			imageInfoUpd, countUpdateAvailable, err := deviceService.GetUpdateAvailableForDevice(resp.Result[0], false, 10, 0)
			Expect(err).To(BeNil())
			Expect(imageInfoUpd).ToNot(BeNil())
			Expect(countUpdateAvailable).To(Equal(int64(1)))

			imageInfo, err := deviceService.GetDeviceImageInfo(resp.Result[0], 10, 0)
			Expect(err).To(BeNil())
			Expect(imageInfo.Image).ToNot(BeNil())
			Expect(imageInfo.Rollback).ToNot(BeNil())
			Expect(imageInfo.UpdatesAvailable).ToNot(BeNil())
			Expect(imageInfo.Image.TotalPackages).To(Equal(len(newImage.Commit.InstalledPackages)))

			Expect(imageInfo.Image.TotalDevicesWithImage).To(Equal(int64(1)))

		})
		It("should return no packages", func() {
			checksum := "fake-checksum-2"
			resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
				{ID: uuid, Ostree: inventory.SystemProfile{
					RHCClientID: faker.UUIDHyphenated(),
					RpmOstreeDeployments: []inventory.OSTree{
						{Checksum: checksum, Booted: true},
					},
				},
					OrgID: orgID,
				},
			}}
			mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).
				Return(resp, nil).Times(1)
			image, _ := seeder.WithOstreeCommit(checksum).WithInstalledPackages([]models.InstalledPackage{}).CreateImage()
			seeder.WithImageID(image.ID).CreateDevice()

			mockImageService.EXPECT().GetImageByOSTreeCommitHash(gomock.Eq(checksum)).Return(image, nil)

			imageInfo, err := deviceService.GetDeviceImageInfo(resp.Result[0], 10, 0)
			Expect(err).To(BeNil())
			Expect(imageInfo.Image).ToNot(BeNil())
			Expect(imageInfo.Rollback).To(BeNil())
			Expect(imageInfo.Image.TotalPackages).To(Equal(0))
			Expect(imageInfo.Image.TotalDevicesWithImage).To(Equal(int64(1)))

		})

		Context("GetDeviceImageInfo with first image status", func() {
			var ctrl *gomock.Controller
			var mockInventoryClient *mock_inventory.MockClientInterface
			var deviceService services.DeviceService
			var orgID string

			BeforeEach(func() {
				orgID = common.DefaultOrgID
				ctrl = gomock.NewController(GinkgoT())
				uuid = faker.UUIDHyphenated()
				mockInventoryClient = mock_inventory.NewMockClientInterface(ctrl)
				mockImageService = mock_services.NewMockImageServiceInterface(ctrl)
				ctx := context.Background()
				logger := log.NewEntry(log.StandardLogger())
				deviceService = services.DeviceService{
					Service:   services.NewService(ctx, logger),
					Inventory: mockInventoryClient,
					ImageService: &services.ImageService{
						Service: services.NewService(ctx, logger),
					},
				}
			})

			AfterEach(func() {
				ctrl.Finish()
			})

			It("should GetDeviceImageInfo successfully when first version succeed", func() {
				checksum := faker.UUIDHyphenated()
				imageName := faker.Name()
				deviceUUID := faker.UUIDHyphenated()
				inventoryDevice := inventory.Device{
					ID: deviceUUID, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: checksum, Booted: true},
						},
					},
					OrgID: orgID,
				}

				imageSet := &models.ImageSet{
					Name:    imageName,
					Version: 2,
					OrgID:   orgID,
				}
				err := db.DB.Create(imageSet).Error
				Expect(err).ToNot(HaveOccurred())

				oldImage := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
						OrgID:        orgID,
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    1,
					OrgID:      orgID,
				}
				err = db.DB.Create(oldImage).Error
				Expect(err).ToNot(HaveOccurred())

				image := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: checksum,
						OrgID:        orgID,
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    2,
					OrgID:      orgID,
				}
				err = db.DB.Create(image).Error
				Expect(err).ToNot(HaveOccurred())

				device := models.Device{
					OrgID:   orgID,
					UUID:    deviceUUID,
					ImageID: image.ID,
				}
				err = db.DB.Create(&device).Error
				Expect(err).ToNot(HaveOccurred())

				imageInfo, err := deviceService.GetDeviceImageInfo(inventoryDevice, 10, 0)
				Expect(err).ToNot(HaveOccurred())
				Expect(imageInfo.Image).ToNot(BeNil())
				Expect(imageInfo.Image.ID).To(Equal(image.ID))
				Expect(imageInfo.Rollback).ToNot(BeNil())
				Expect(imageInfo.Rollback.ID).To(Equal(oldImage.ID))
			})

			It("should GetDeviceImageInfo successfully when first version failed", func() {
				checksum := faker.UUIDHyphenated()
				imageName := faker.Name()
				deviceUUID := faker.UUIDHyphenated()
				inventoryDevice := inventory.Device{
					ID: deviceUUID, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: checksum, Booted: true},
						},
					},
					OrgID: orgID,
				}

				imageSet := &models.ImageSet{
					Name:    imageName,
					Version: 2,
					OrgID:   orgID,
				}
				err := db.DB.Create(imageSet).Error
				Expect(err).ToNot(HaveOccurred())

				oldImage := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
						OrgID:        orgID,
					},
					Status:     models.ImageStatusError,
					ImageSetID: &imageSet.ID,
					Version:    1,
					OrgID:      orgID,
				}
				err = db.DB.Create(oldImage).Error
				Expect(err).ToNot(HaveOccurred())

				image := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: checksum,
						OrgID:        orgID,
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    2,
					OrgID:      orgID,
				}
				err = db.DB.Create(image).Error
				Expect(err).ToNot(HaveOccurred())

				device := models.Device{
					OrgID:   orgID,
					UUID:    deviceUUID,
					ImageID: image.ID,
				}
				err = db.DB.Create(&device).Error
				Expect(err).ToNot(HaveOccurred())

				imageInfo, err := deviceService.GetDeviceImageInfo(inventoryDevice, 10, 0)
				Expect(err).ToNot(HaveOccurred())
				Expect(imageInfo.Image).ToNot(BeNil())
				Expect(imageInfo.Image.ID).To(Equal(image.ID))
				Expect(imageInfo.Rollback).To(BeNil())
			})

			It("should return error when GetRollbackImage fail", func() {
				deviceService.ImageService = mockImageService

				expectedError := errors.New("error when getting roll back image")

				checksum := faker.UUIDHyphenated()
				imageName := faker.Name()
				deviceUUID := faker.UUIDHyphenated()
				inventoryDevice := inventory.Device{
					ID: deviceUUID, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: checksum, Booted: true},
						},
					},
					OrgID: orgID,
				}

				imageSet := &models.ImageSet{
					Name:    imageName,
					Version: 2,
					OrgID:   orgID,
				}
				err := db.DB.Create(imageSet).Error
				Expect(err).ToNot(HaveOccurred())

				oldImage := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
						OrgID:        orgID,
					},
					Status:     models.ImageStatusError,
					ImageSetID: &imageSet.ID,
					Version:    1,
					OrgID:      orgID,
				}
				err = db.DB.Create(oldImage).Error
				Expect(err).ToNot(HaveOccurred())

				image := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: checksum,
						OrgID:        orgID,
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    2,
					OrgID:      orgID,
				}
				err = db.DB.Create(image).Error
				Expect(err).ToNot(HaveOccurred())

				device := models.Device{
					OrgID:   orgID,
					UUID:    deviceUUID,
					ImageID: image.ID,
				}
				err = db.DB.Create(&device).Error
				Expect(err).ToNot(HaveOccurred())

				mockImageService.EXPECT().GetImageByOSTreeCommitHash(checksum).Return(image, nil)
				mockImageService.EXPECT().GetRollbackImage(image).Return(nil, expectedError)

				_, err = deviceService.GetDeviceImageInfo(inventoryDevice, 10, 0)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			})
		})
	})
	Context("Validate if can update device", func() {
		It("should return true when same refs", func() {
			canUpdate := deviceService.CanUpdate("rhel-85", "rhel-86")
			Expect(canUpdate).To(BeTrue())

		})
		It("should return true when same required package", func() {
			canUpdate := deviceService.CanUpdate("rhel-86", "rhel-90")
			Expect(canUpdate).To(BeTrue())
		})

		It("should return false when diff required package", func() {
			canUpdate := deviceService.CanUpdate("rhel-85", "rhel-90")
			Expect(canUpdate).To(BeFalse())
		})
	})

	Context("Get Device count by image ", func() {
		var imageSet *models.ImageSet
		var img *models.Image
		var img2 *models.Image
		var img3 *models.Image
		var device []models.Device
		BeforeEach(func() {
			img, imageSet = seeder.CreateImage()
			img2, _ = seeder.WithImageSetID(imageSet.ID).CreateImage()
			img3, _ = seeder.WithImageSetID(imageSet.ID).CreateImage()
			device = []models.Device{
				{OrgID: "00000000", UUID: faker.UUIDHyphenated(), ImageID: img.ID},
				{OrgID: "00000000", UUID: faker.UUIDHyphenated(), ImageID: img.ID},
				{OrgID: "00000000", UUID: faker.UUIDHyphenated(), ImageID: img.ID},
				{OrgID: "00000000", UUID: faker.UUIDHyphenated(), ImageID: img2.ID},
				{OrgID: "00000000", UUID: faker.UUIDHyphenated(), ImageID: img2.ID},
			}
		})
		It("should return devices", func() {
			db.DB.Create(&device)
			count, err := deviceService.GetDevicesCountByImage(img.ID)
			Expect(err).To(BeNil())
			Expect(count).To(Equal(int64(3)))

		})
		It("should return 2", func() {
			db.DB.Create(&device)
			count, err := deviceService.GetDevicesCountByImage(img2.ID)
			Expect(err).To(BeNil())
			Expect(count).To(Equal(int64(2)))
		})
		It("should return 0", func() {
			db.DB.Create(&device)
			count, err := deviceService.GetDevicesCountByImage(img3.ID)
			Expect(err).To(BeNil())
			Expect(count).To(Equal(int64(0)))
		})

	})

	Context("Get Device image info paginated ", func() {
		var checksum string
		var resp inventory.Response
		var imageSet *models.ImageSet
		var oldImage *models.Image
		var newImage *models.Image
		var newImage2 *models.Image
		BeforeEach(func() {
			checksum = "fake-checksum-pag"
			resp = inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
				{ID: uuid, Ostree: inventory.SystemProfile{
					RHCClientID: faker.UUIDHyphenated(),
					RpmOstreeDeployments: []inventory.OSTree{
						{Checksum: checksum, Booted: true},
					},
				},
					OrgID: orgID,
				},
			}}
			mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil).Times(2)

			oldImage, imageSet = seeder.WithOstreeCommit(checksum).CreateImage()

			newImage, _ = seeder.WithVersion(2).WithImageSetID(imageSet.ID).CreateImage()
			newImage2, _ = seeder.WithVersion(3).WithImageSetID(imageSet.ID).CreateImage()
		})

		It("should return first result", func() {
			mockImageService.EXPECT().GetImageByOSTreeCommitHash(gomock.Eq(checksum)).Return(oldImage, nil)

			imageInfo, err := deviceService.GetDeviceImageInfoByUUID(uuid, 1, 0)

			Expect(err).To(BeNil())
			Expect(len((*imageInfo.UpdatesAvailable))).To(Equal(1))
			Expect(newImage2.Version).To(Equal((*imageInfo.UpdatesAvailable)[0].Image.Version))

		})
		It("should return last result", func() {
			mockImageService.EXPECT().GetImageByOSTreeCommitHash(gomock.Eq(checksum)).Return(oldImage, nil)

			imageInfo, err := deviceService.GetDeviceImageInfoByUUID(uuid, 1, 1)
			Expect(err).To(BeNil())
			Expect(len((*imageInfo.UpdatesAvailable))).To(Equal(1))
			Expect(newImage.Version).To(Equal((*imageInfo.UpdatesAvailable)[0].Image.Version))
		})
		It("should return all result", func() {
			mockImageService.EXPECT().GetImageByOSTreeCommitHash(gomock.Eq(checksum)).Return(oldImage, nil)

			imageInfo, err := deviceService.GetDeviceImageInfoByUUID(uuid, 10, 0)
			Expect(err).To(BeNil())
			Expect(len((*imageInfo.UpdatesAvailable))).To(Equal(2))
		})
		It("should return last result", func() {
			mockImageService.EXPECT().GetImageByOSTreeCommitHash(gomock.Eq(checksum)).Return(oldImage, nil)

			imageInfo, err := deviceService.GetDeviceImageInfoByUUID(uuid, -1, 1)
			Expect(err).To(BeNil())
			fmt.Printf("\n *imageInfo.UpdatesAvailable %v\n", *imageInfo.UpdatesAvailable)
			Expect(len((*imageInfo.UpdatesAvailable))).To(Equal(1))
		})
		It("should return last result", func() {
			mockImageService.EXPECT().GetImageByOSTreeCommitHash(gomock.Eq(checksum)).Return(oldImage, nil)

			imageInfo, err := deviceService.GetDeviceImageInfoByUUID(uuid, -1, 0)
			Expect(err).To(BeNil())
			fmt.Printf("\n *imageInfo.UpdatesAvailable %v\n", *imageInfo.UpdatesAvailable)
			Expect(len((*imageInfo.UpdatesAvailable))).To(Equal(2))
		})
	})
})
