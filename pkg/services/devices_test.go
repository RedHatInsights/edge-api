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
			It("should return error and no updates available", func() {
				mockInventoryClient.EXPECT().ReturnDevicesByID(gomock.Eq(uuid)).Return(inventory.Response{}, errors.New("error on inventory api"))

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid)
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

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid)
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

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid)
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

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid)

				Expect(err).To(BeNil())
				Expect(updatesAvailable).To(HaveLen(1))
				newUpdate := updatesAvailable[0]
				Expect(newUpdate.Image.ID).To(Equal(newImage.ID))
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

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid)
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

				updatesAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(uuid)
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
		uuid := faker.UUIDHyphenated()
		device := models.Device{
			UUID: uuid,
		}
		type PlatformInsightsCreateEventPayload struct {
			Type         string `json:"type"`
			PlatformMeta string `json:"platform_metadata"`
			Metadata     struct {
				RequestID string `json:"request_id"`
			} `json:"metadata"`
			Host struct {
				ID             string `json:"id"`
				Account        string `json:"account"`
				DisplayName    string `json:"display_name"`
				AnsibleHost    string `json:"ansible_host"`
				Fqdn           string `json:"fqdn"`
				InsightsID     string `json:"insights_id"`
				StaleTimestamp string `json:"stale_timestamp"`
				Reporter       string `json:"reporter"`
				Tags           string `json:"tags"`
				SystemProfile  string `json:"system_profile"`
			} `json:"host"`
		}
		event := new(PlatformInsightsCreateEventPayload)
		event.Host.ID = uuid
		event.Host.Account = "TESTACCOUNT"
		message, _ := json.Marshal(event)
		It("should update devices when no record is found", func() {
			deviceService.ProcessPlatformInventoryCreateEvent(message)
			db.DB.First(&device, device.UUID)
			Expect(device.UUID).To(Equal(event.Host.ID))
		})
	})
})
