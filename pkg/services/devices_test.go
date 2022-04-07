package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory/mock_inventory"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("DeviceService", func() {
	var mockInventoryClient *mock_inventory.MockClientInterface
	var deviceService services.DeviceService
	var mockImageService *mock_services.MockImageServiceInterface
	var uuid string
	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		defer ctrl.Finish()
		uuid = faker.UUIDHyphenated()
		mockInventoryClient = mock_inventory.NewMockClientInterface(ctrl)
		mockImageService = mock_services.NewMockImageServiceInterface(ctrl)

		deviceService = services.DeviceService{
			Service:      services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
			Inventory:    mockInventoryClient,
			ImageService: mockImageService,
		}
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

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, false)
				Expect(updatesAvailable).To(BeNil())
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
			})
			It("should return error and no updates available - for latest update", func() {
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(inventory.Response{}, errors.New("error on inventory api"))

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, true)
				Expect(updatesAvailable).To(BeNil())
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
			})
		})
		When("device is not found on InventoryAPI", func() {
			It("should return error and nil updates available", func() {
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(inventory.Response{}, nil)

				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, false)
				Expect(updatesAvailable).To(BeNil())
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
			})
			It("should return error and nil on latest update available", func() {
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(inventory.Response{}, nil)

				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, true)
				Expect(updatesAvailable).To(BeNil())
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
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
					}},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, false)
				Expect(updatesAvailable).To(BeNil())
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
			})
			It("should return error and nil on latest update available", func() {
				checksum := "fake-checksum"
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: uuid, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: checksum, Booted: false},
						},
					}},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, true)
				Expect(updatesAvailable).To(BeNil())
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
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
					}},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

				imageSet := &models.ImageSet{
					Name:    "test",
					Version: 1,
				}
				db.DB.Create(imageSet)
				oldImage := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: checksum,
						InstalledPackages: []models.InstalledPackage{
							{
								Name:    "ansible",
								Version: "1.0.0",
							},
							{
								Name:    "yum",
								Version: "2:6.0-1",
							},
						},
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
				}
				db.DB.Create(oldImage.Commit)
				db.DB.Create(oldImage)
				newImage := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: fmt.Sprintf("a-new-%s", checksum),
						InstalledPackages: []models.InstalledPackage{
							{
								Name:    "yum",
								Version: "3:6.0-1",
							},
							{
								Name:    "vim",
								Version: "2.0.0",
							},
						},
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
				}
				db.DB.Create(newImage.Commit)
				db.DB.Create(newImage)

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, false)

				Expect(err).To(BeNil())
				Expect(updatesAvailable).To(HaveLen(1))
				newUpdate := updatesAvailable[0]
				Expect(newUpdate.Image.ID).To(Equal(newImage.ID))
				Expect(newUpdate.PackageDiff.Upgraded).To(HaveLen(1))
				Expect(newUpdate.PackageDiff.Added).To(HaveLen(1))
				Expect(newUpdate.PackageDiff.Removed).To(HaveLen(1))
			})
			It("should return updates", func() {
				checksum := faker.UUIDHyphenated()
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: uuid, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: checksum, Booted: true},
						},
					}},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

				imageSet := &models.ImageSet{
					Name:    faker.Name(),
					Version: 1,
				}
				db.DB.Create(imageSet)
				oldImage := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: checksum,
						InstalledPackages: []models.InstalledPackage{
							{
								Name:    "ansible",
								Version: "1.0.0",
							},
							{
								Name:    "yum",
								Version: "2:6.0-1",
							},
						},
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
				}
				db.DB.Create(oldImage.Commit)
				db.DB.Create(oldImage)
				newImage := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: fmt.Sprintf("a-new-%s", checksum),
						InstalledPackages: []models.InstalledPackage{
							{
								Name:    "yum",
								Version: "3:6.0-1",
							},
							{
								Name:    "vim",
								Version: "2.0.0",
							},
						},
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
				}
				db.DB.Create(newImage.Commit)
				db.DB.Create(newImage)
				thirdImage := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: fmt.Sprintf("a-third-%s", checksum),
						InstalledPackages: []models.InstalledPackage{
							{
								Name:    "yum",
								Version: "3:6.0-1",
							},
							{
								Name:    "puppet",
								Version: "2.0.0",
							},
						},
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
				}
				db.DB.Create(thirdImage.Commit)
				db.DB.Create(thirdImage)

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, true)

				Expect(err).To(BeNil())
				Expect(updatesAvailable).To(HaveLen(1))
				newUpdate := updatesAvailable[0]
				Expect(newUpdate.Image.ID).To(Equal(thirdImage.ID))
				Expect(newUpdate.PackageDiff.Upgraded).To(HaveLen(1))
				Expect(newUpdate.PackageDiff.Added).To(HaveLen(1))
				Expect(newUpdate.PackageDiff.Removed).To(HaveLen(1))
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
							}},
					},
				}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

				oldImage := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: checksum,
					},
					Status: models.ImageStatusSuccess,
				}
				db.DB.Create(oldImage)

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, false)
				Expect(updatesAvailable).To(BeNil())
				Expect(err).To(BeNil())
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
					}},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid, false)
				Expect(updatesAvailable).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError(new(services.DeviceNotFoundError)))
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
			},
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
			},
		}
		It("should return diff", func() {
			deltaDiff := services.GetDiffOnUpdate(oldImage, newImage)
			Expect(deltaDiff.Added).To(HaveLen(1))
			Expect(deltaDiff.Removed).To(HaveLen(2))
			Expect(deltaDiff.Upgraded).To(HaveLen(1))
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
					}},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil).Times(2)
				imageSet := &models.ImageSet{
					Name:    "test",
					Version: 2,
				}
				db.DB.Create(imageSet)
				oldImage := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: fmt.Sprintf("a-old-%s", checksum),
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    1,
				}
				db.DB.Create(oldImage.Commit)
				db.DB.Create(oldImage)
				fmt.Printf("Old image was created with id %d\n", oldImage.ID)
				newImage := &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: checksum,
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    2,
				}
				db.DB.Create(newImage.Commit)
				db.DB.Create(newImage)
				fmt.Printf("New image was created with id %d\n", newImage.ID)
				fmt.Printf("New image was created with image set id %d\n", *newImage.ImageSetID)

				mockImageService.EXPECT().GetImageByOSTreeCommitHash(gomock.Eq(checksum)).Return(newImage, nil)
				mockImageService.EXPECT().GetRollbackImage(gomock.Eq(newImage)).Return(oldImage, nil)

				imageInfo, err := deviceService.GetDeviceImageInfoByUUID(uuid)
				Expect(err).To(BeNil())
				Expect(oldImage.Commit.OSTreeCommit).To(Equal(imageInfo.Rollback.Commit.OSTreeCommit))
				Expect(newImage.Commit.OSTreeCommit).To(Equal(imageInfo.Image.Commit.OSTreeCommit))
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
					}},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(resp, nil)
				mockImageService.EXPECT().GetImageByOSTreeCommitHash(gomock.Eq(checksum)).Return(nil, errors.New("Not found"))

				_, err := deviceService.GetDeviceImageInfoByUUID(uuid)
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
				Expect(devices).ToNot(BeNil())
				Expect(devices.Devices).To(HaveLen(0))
				Expect(devices.Count).To(Equal(0))
				Expect(devices.Total).To(Equal(0))
				Expect(err).To(BeNil())
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
						Ostree:      inventory.SystemProfile{RHCClientID: "", RpmOstreeDeployments: []inventory.OSTree{}},
					}, {
						ID:          deviceWithNoImage.UUID,
						DisplayName: "oi",
						LastSeen:    "b",
						Ostree:      inventory.SystemProfile{RHCClientID: "", RpmOstreeDeployments: []inventory.OSTree{}},
					}},
				}, nil)
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Any()).AnyTimes().Return(inventory.Response{}, new(services.DeviceNotFoundError))
				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				devices, err := deviceService.GetDevices(params)
				Expect(devices).ToNot(BeNil())
				Expect(devices.Devices).To(HaveLen(2))
				Expect(devices.Count).To(Equal(2))
				Expect(devices.Total).To(Equal(2))
				Expect(err).To(BeNil())
			})
		})
	})
	Context("ProcessPlatformInventoryCreateEvent", func() {
		account := faker.UUIDHyphenated()
		commit := models.Commit{Account: account, OSTreeCommit: faker.UUIDHyphenated()}
		result := db.DB.Create(&commit)
		Expect(result.Error).To(BeNil())
		image := models.Image{Account: account, CommitID: commit.ID, Status: models.ImageStatusSuccess}
		result = db.DB.Create(&image)
		Expect(result.Error).To(BeNil())

		event := new(services.PlatformInsightsCreateUpdateEventPayload)
		event.Type = services.InventoryEventTypeCreated
		event.Host.SystemProfile.HostType = services.InventoryHostTypeEdge
		event.Host.ID = faker.UUIDHyphenated()
		event.Host.Account = account
		event.Host.SystemProfile.RpmOSTreeDeployments = []services.RpmOSTreeDeployment{{Booted: true, Checksum: commit.OSTreeCommit}}
		message, err := json.Marshal(event)
		Expect(err).To(BeNil())

		It("should create devices when no record is found", func() {
			err := deviceService.ProcessPlatformInventoryCreateEvent(message)
			Expect(err).To(BeNil())
			var savedDevice models.Device
			result := db.DB.Where(models.Device{UUID: event.Host.ID}).First(&savedDevice)
			Expect(result.Error).To(BeNil())
			Expect(savedDevice.UUID).To(Equal(event.Host.ID))
			Expect(savedDevice.Account).To(Equal(account))
			Expect(savedDevice.ImageID).To(Equal(image.ID))
		})
	})
	Context("ProcessPlatformInventoryUpdatedEvent", func() {
		account := faker.UUIDHyphenated()
		commit := models.Commit{Account: account, OSTreeCommit: faker.UUIDHyphenated()}
		result := db.DB.Create(&commit)
		Expect(result.Error).To(BeNil())
		imageSet := models.ImageSet{Name: faker.UUIDHyphenated(), Account: account}
		result = db.DB.Create(&imageSet)
		Expect(result.Error).To(BeNil())
		image := models.Image{Account: account, CommitID: commit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
		result = db.DB.Create(&image)
		Expect(result.Error).To(BeNil())

		It("should create a device when device does not exist", func() {
			event := new(services.PlatformInsightsCreateUpdateEventPayload)
			event.Type = services.InventoryEventTypeUpdated
			event.Host.ID = faker.UUIDHyphenated()
			event.Host.InsightsID = faker.UUIDHyphenated()
			event.Host.Account = account
			event.Host.SystemProfile.HostType = services.InventoryHostTypeEdge
			event.Host.SystemProfile.RpmOSTreeDeployments = []services.RpmOSTreeDeployment{{Booted: true, Checksum: commit.OSTreeCommit}}
			message, err := json.Marshal(event)
			Expect(err).To(BeNil())

			err = deviceService.ProcessPlatformInventoryUpdatedEvent(message)
			Expect(err).To(BeNil())

			var device models.Device
			res := db.DB.Where("uuid = ?", event.Host.ID).First(&device)
			Expect(res.Error).To(BeNil())
			Expect(device.Account).To(Equal(account))
			Expect(device.RHCClientID).To(Equal(event.Host.InsightsID))
			Expect(device.ImageID).To(Equal(image.ID))
			Expect(device.UpdateAvailable).To(Equal(false))
		})

		It("should update device account, image_id and update availability when device already exists", func() {
			// create a device without account
			device := models.Device{
				UUID:            faker.UUIDHyphenated(),
				RHCClientID:     faker.UUIDHyphenated(),
				UpdateAvailable: true,
			}
			res := db.DB.Create(&device)
			Expect(res.Error).To(BeNil())

			event := new(services.PlatformInsightsCreateUpdateEventPayload)
			event.Type = services.InventoryEventTypeUpdated
			event.Host.ID = device.UUID
			event.Host.InsightsID = device.RHCClientID
			event.Host.Account = account
			event.Host.SystemProfile.HostType = services.InventoryHostTypeEdge
			event.Host.SystemProfile.RpmOSTreeDeployments = []services.RpmOSTreeDeployment{{Booted: true, Checksum: commit.OSTreeCommit}}
			message, err := json.Marshal(event)
			Expect(err).To(BeNil())

			err = deviceService.ProcessPlatformInventoryUpdatedEvent(message)
			Expect(err).To(BeNil())

			var savedDevice models.Device
			res = db.DB.Where("uuid = ?", device.UUID).First(&savedDevice)
			Expect(res.Error).To(BeNil())
			Expect(savedDevice.Account).To(Equal(account))
			Expect(savedDevice.ImageID).To(Equal(image.ID))
			Expect(savedDevice.UpdateAvailable).To(Equal(false))
		})

		Context("device update availability", func() {
			device := models.Device{
				UUID:            faker.UUIDHyphenated(),
				RHCClientID:     faker.UUIDHyphenated(),
				Account:         account,
				ImageID:         image.ID,
				UpdateAvailable: false,
			}
			res := db.DB.Create(&device)
			Expect(res.Error).To(BeNil())

			event := new(services.PlatformInsightsCreateUpdateEventPayload)
			event.Type = services.InventoryEventTypeUpdated
			event.Host.ID = device.UUID
			event.Host.InsightsID = device.RHCClientID
			event.Host.Account = account
			event.Host.SystemProfile.HostType = services.InventoryHostTypeEdge
			event.Host.SystemProfile.RpmOSTreeDeployments = []services.RpmOSTreeDeployment{{Booted: true, Checksum: commit.OSTreeCommit}}
			message, err := json.Marshal(event)
			Expect(err).To(BeNil())

			It("should not set update available when an image update failed", func() {
				newImage := models.Image{Account: account, ImageSetID: &imageSet.ID, Status: models.ImageStatusError}
				result = db.DB.Create(&newImage)
				Expect(result.Error).To(BeNil())

				err = deviceService.ProcessPlatformInventoryUpdatedEvent(message)
				Expect(err).To(BeNil())

				var savedDevice models.Device
				res = db.DB.Where("uuid = ?", device.UUID).First(&savedDevice)
				Expect(res.Error).To(BeNil())
				Expect(savedDevice.Account).To(Equal(account))
				Expect(savedDevice.ImageID).To(Equal(image.ID))
				Expect(savedDevice.UpdateAvailable).To(Equal(false))
			})

			It("should set update available when an image is updated successfully", func() {
				newImage := models.Image{Account: account, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
				result = db.DB.Create(&newImage)
				Expect(result.Error).To(BeNil())

				err = deviceService.ProcessPlatformInventoryUpdatedEvent(message)
				Expect(err).To(BeNil())

				var savedDevice models.Device
				res = db.DB.Where("uuid = ?", device.UUID).First(&savedDevice)
				Expect(res.Error).To(BeNil())
				Expect(savedDevice.Account).To(Equal(account))
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
			event.Account = faker.UUIDHyphenated()
			message, err := json.Marshal(event)
			Expect(err).To(BeNil())

			// create a device
			device := models.Device{UUID: event.ID, Account: event.Account}
			result := db.DB.Create(&device)
			Expect(result.Error).To(BeNil())

			// ensure device created
			var deviceCount int64
			db.DB.Model(&models.Device{}).Where(models.Device{UUID: event.ID, Account: event.Account}).Count(&deviceCount)
			Expect(deviceCount == 1).To(BeTrue())

			// call the platform inventory delete event processor
			err = deviceService.ProcessPlatformInventoryDeleteEvent(message)
			Expect(err).To(BeNil())

			// ensure device does not exits
			db.DB.Model(&models.Device{}).Where(models.Device{UUID: event.ID, Account: event.Account}).Count(&deviceCount)
			Expect(deviceCount == 0).To(BeTrue())
		})

		It("device in device-groups should be removed", func() {
			// create a platform inventory delete event message
			event := new(services.PlatformInsightsDeleteEventPayload)
			event.Type = services.InventoryEventTypeDelete
			event.ID = faker.UUIDHyphenated()
			event.Account = faker.UUIDHyphenated()
			message, err := json.Marshal(event)
			Expect(err).To(BeNil())

			// create a device
			device := models.Device{UUID: event.ID, Account: event.Account}
			result := db.DB.Create(&device)
			Expect(result.Error).To(BeNil())

			// ensure the device exists
			var deviceCount int64
			result = db.DB.Model(&models.Device{}).Where(models.Device{UUID: event.ID, Account: event.Account}).Count(&deviceCount)
			Expect(result.Error).To(BeNil())
			Expect(deviceCount == 1).To(BeTrue())
			// create a device group with device
			deviceGroup := models.DeviceGroup{
				Type: models.DeviceGroupTypeDefault, Account: event.Account, Name: faker.UUIDHyphenated(),
				Devices: []models.Device{device},
			}
			result = db.DB.Create(&deviceGroup)
			Expect(result.Error).To(BeNil())
			// ensure device group created with device included
			var savedDeviceGroup models.DeviceGroup
			result = db.DB.Where(models.DeviceGroup{Account: deviceGroup.Account}).Preload("Devices").First(&savedDeviceGroup, deviceGroup.ID)
			Expect(result.Error).To(BeNil())
			Expect(savedDeviceGroup.Devices).NotTo(BeEmpty())
			Expect(savedDeviceGroup.Devices[0].ID == device.ID).To(BeTrue())

			// call the platform inventory delete event processor
			err = deviceService.ProcessPlatformInventoryDeleteEvent(message)
			Expect(err).To(BeNil())

			// ensure device does not exits
			result = db.DB.Model(&models.Device{}).Where(models.Device{UUID: event.ID, Account: event.Account}).Count(&deviceCount)
			Expect(result.Error).To(BeNil())
			Expect(deviceCount == 0).To(BeTrue())

			// ensure device does not exists in device group
			result = db.DB.Where(models.DeviceGroup{Account: deviceGroup.Account}).Preload("Devices").First(&savedDeviceGroup, deviceGroup.ID)
			Expect(result.Error).To(BeNil())
			Expect(savedDeviceGroup.Devices).To(BeEmpty())
		})
	})
	Context("Get CommitID from Device Image", func() {
		When("device Image does not exists", func() {
			It("should return zero images", func() {
				account := faker.UUIDHyphenated()
				device := models.Device{
					Account: account,
					UUID:    faker.UUIDHyphenated(),
				}
				updateImageCommitID, err := deviceService.GetUpdateCommitFromDevice(account, device.UUID)
				Expect(updateImageCommitID == 0).To(BeTrue())
				Expect(err).To(MatchError("record not found"))
				Expect(err).ToNot(BeNil())
			})
		})
		When("device Image does not have update", func() {
			It("should return no image updates", func() {
				account := faker.HyphenatedID
				imageSet := &models.ImageSet{
					Name:    "test",
					Version: 1,
				}
				updateImage := models.Image{
					Account:    account,
					ImageSetID: &imageSet.ID,
					Status:     models.ImageStatusSuccess,
				}
				device := models.Device{
					UUID: faker.UUIDHyphenated(),
				}
				updateImageCommitID, err := deviceService.GetUpdateCommitFromDevice(updateImage.Account, device.UUID)
				Expect(updateImageCommitID == 0).To(BeTrue())
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("record not found"))
			})
		})
		When("device Image have update", func() {
			It("should return commitID", func() {
				account := faker.HyphenatedID
				imageSet := models.ImageSet{
					Account: account,
				}
				db.DB.Create(&imageSet)

				firstCommit := models.Commit{
					Account: account,
				}
				db.DB.Create(&firstCommit)

				firstImage := models.Image{
					Account:    account,
					CommitID:   firstCommit.ID,
					Status:     models.ImageStatusSuccess,
					Version:    1,
					ImageSetID: &imageSet.ID,
				}
				db.DB.Create(&firstImage)
				device := models.Device{
					Account: account,
					ImageID: firstImage.ID,
				}
				db.DB.Create(&device)
				secondCommit := models.Commit{
					Account: account,
				}
				db.DB.Create(&secondCommit)

				secondImage := models.Image{
					Account:    account,
					CommitID:   secondCommit.ID,
					Status:     models.ImageStatusSuccess,
					Version:    2,
					ImageSetID: &imageSet.ID,
				}
				db.DB.Create(&secondImage)
				updateImageCommitID, err := deviceService.GetUpdateCommitFromDevice(device.Account, device.UUID)
				Expect(err).To(BeNil())
				Expect(updateImageCommitID).To(Equal(secondCommit.ID))
			})
		})
	})
	Context("GetDeviceView", func() {
		When("devices are returned from the db", func() {
			It("should return devices", func() {
				account := common.DefaultAccount
				var imageV1 *models.Image

				imageSet := &models.ImageSet{
					Name:    "test",
					Version: 2,
					Account: account,
				}
				result := db.DB.Create(imageSet)
				Expect(result.Error).ToNot(HaveOccurred())
				imageV1 = &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    1,
					Account:    common.DefaultAccount,
				}
				result = db.DB.Create(imageV1.Commit)
				Expect(result.Error).ToNot(HaveOccurred())
				result = db.DB.Create(imageV1)
				Expect(result.Error).ToNot(HaveOccurred())

				deviceWithImage := models.Device{Account: account, ImageID: imageV1.ID}

				result = db.DB.Create(&deviceWithImage)
				Expect(result.Error).To(BeNil())

				deviceService := services.DeviceService{
					Service:   services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
					Inventory: mockInventoryClient,
				}

				devices, err := deviceService.GetDevicesView(0, 0, nil)
				Expect(devices).ToNot(BeNil())
				Expect(err).To(BeNil())
			})
		})
	})
})
