package cleanupdevices_test

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/redhatinsights/edge-api/cmd/cleanup/cleanupdevices"

	"github.com/redhatinsights/edge-api/cmd/cleanup/storage"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/files"
	"github.com/redhatinsights/edge-api/pkg/services/mock_files"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

var _ = Describe("Cleanup devices", func() {
	var ctrl *gomock.Controller
	var s3Client *files.S3Client
	var s3ClientAPI *mock_files.MockS3ClientAPI
	var s3FolderDeleter *mock_files.MockBatchFolderDeleterAPI

	var initialTimeDuration time.Duration
	var configDeleteAttempts int

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		s3ClientAPI = mock_files.NewMockS3ClientAPI(ctrl)
		s3FolderDeleter = mock_files.NewMockBatchFolderDeleterAPI(ctrl)
		s3Client = &files.S3Client{
			Client:        s3ClientAPI,
			FolderDeleter: s3FolderDeleter,
		}

		initialTimeDuration = storage.DefaultTimeDuration
		storage.DefaultTimeDuration = 1 * time.Millisecond
		configDeleteAttempts = int(config.Get().DeleteFilesAttempts)
	})

	AfterEach(func() {
		ctrl.Finish()
		storage.DefaultTimeDuration = initialTimeDuration
	})

	Context("IsDeviceCandidate", func() {

		It("should return error when deleted_at is null ", func() {
			deviceData := cleanupdevices.CandidateDevice{DeviceID: 128}
			deviceDeletedAtValue, err := deviceData.DeviceDeletedAt.Value()
			Expect(err).ToNot(HaveOccurred())
			Expect(deviceDeletedAtValue).To(BeNil())

			err = cleanupdevices.IsDeviceCandidate(&deviceData)
			Expect(err).To(MatchError(cleanupdevices.ErrDeviceNotCleanUpCandidate))
		})
	})

	When("Device not a candidate", func() {
		var deviceData cleanupdevices.CandidateDevice

		BeforeEach(func() {
			deviceData = cleanupdevices.CandidateDevice{DeviceID: 128}
		})

		It("DeleteUpdateTransaction should return error", func() {
			err := cleanupdevices.DeleteUpdateTransaction(&deviceData)
			Expect(err).To(MatchError(cleanupdevices.ErrDeviceNotCleanUpCandidate))
		})
	})

	Context("DeleteCommit", func() {
		var orgID string
		var installedPackages []models.InstalledPackage
		var image models.Image
		var commit models.Commit
		var repo models.Repo
		// orphan commit is a commit without image
		var orphanCommit models.Commit

		BeforeEach(func() {
			// setup only once
			if orgID == "" {
				orgID = faker.UUIDHyphenated()

				installedPackages = []models.InstalledPackage{
					{Name: "git"},
					{Name: "gcc"},
				}
				err := db.DB.Create(&installedPackages).Error
				Expect(err).ToNot(HaveOccurred())
				commit = models.Commit{OrgID: orgID}
				image = models.Image{Name: faker.Name(), OrgID: orgID, Commit: &commit}
				err = db.DB.Create(&image).Error
				Expect(err).ToNot(HaveOccurred())

				repo = models.Repo{URL: faker.URL(), Status: models.ImageStatusPending}
				err = db.DB.Create(&repo).Error
				Expect(err).ToNot(HaveOccurred())

				orphanCommit = models.Commit{OrgID: orgID, InstalledPackages: installedPackages, Repo: &repo}
				err = db.DB.Create(&orphanCommit).Error
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("should return error when device candidate is not valid", func() {
			err := cleanupdevices.DeleteCommit(nil, &cleanupdevices.CandidateDevice{DeviceID: 125})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(cleanupdevices.ErrDeviceNotCleanUpCandidate))
		})

		It("device candidate without commit return no error ", func() {
			err := cleanupdevices.DeleteCommit(nil, &cleanupdevices.CandidateDevice{
				DeviceID: 125, DeviceDeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("commit with image is not deleted and return no error", func() {
			err := cleanupdevices.DeleteCommit(nil, &cleanupdevices.CandidateDevice{
				DeviceID:        125,
				DeviceDeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
				CommitID:        &commit.ID,
				ImageID:         &image.ID,
			})
			Expect(err).ToNot(HaveOccurred())
			// refresh commit from database and ensure still exists
			err = db.DB.First(&commit, commit.ID).Error
			Expect(err).ToNot(HaveOccurred())
		})

		It("commit without image is deleted successfully", func() {
			// ensure commits installed packages exists
			var currentInstalledPackages models.CommitInstalledPackages
			err := db.DB.Where("commit_id", orphanCommit.ID).First(&currentInstalledPackages).Error
			Expect(err).ToNot(HaveOccurred())
			err = cleanupdevices.DeleteCommit(db.DB, &cleanupdevices.CandidateDevice{
				DeviceID:        125,
				DeviceDeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
				CommitID:        &orphanCommit.ID,
				ImageID:         nil,
				CommitRepoID:    &repo.ID,
			})
			Expect(err).ToNot(HaveOccurred())
			orphanCommitID := orphanCommit.ID
			// refresh commit from database and ensure that the commit does not exit
			err = db.DB.First(&orphanCommit, orphanCommitID).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))
			// ensure repo does not exits
			err = db.DB.First(&repo, repo.ID).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))
			// ensure commits installed packages does exists
			err = db.DB.Where("commit_id", orphanCommitID).First(&currentInstalledPackages).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))
		})
	})

	Context("DeleteUpdateTransaction", func() {

		var orgID string
		var updateTransaction models.UpdateTransaction
		var dispatcherRecord models.DispatchRecord
		var device models.Device
		var commit models.Commit
		var oldCommit models.Commit
		var updateRepo models.Repo

		BeforeEach(func() {
			// setup only once
			if orgID == "" {
				orgID = faker.UUIDHyphenated()

				commit = models.Commit{OrgID: orgID}
				err := db.DB.Create(&commit).Error
				Expect(err).ToNot(HaveOccurred())

				oldCommit = models.Commit{OrgID: orgID}
				err = db.DB.Create(&oldCommit).Error
				Expect(err).ToNot(HaveOccurred())

				device = models.Device{OrgID: orgID, UUID: faker.UUIDHyphenated()}
				err = db.DB.Create(&device).Error
				Expect(err).ToNot(HaveOccurred())

				dispatcherRecord = models.DispatchRecord{Device: &device, DeviceID: device.ID, PlaybookDispatcherID: faker.UUIDHyphenated()}
				err = db.DB.Omit("Device").Create(&dispatcherRecord).Error
				Expect(err).ToNot(HaveOccurred())

				updateRepo = models.Repo{URL: faker.URL(), Status: models.ImageStatusPending}
				err = db.DB.Create(&updateRepo).Error
				Expect(err).ToNot(HaveOccurred())

				updateTransaction = models.UpdateTransaction{
					OrgID:           orgID,
					Devices:         []models.Device{device},
					DispatchRecords: []models.DispatchRecord{dispatcherRecord},
					Commit:          &commit,
					OldCommits:      []models.Commit{oldCommit},
					Repo:            &updateRepo,
				}

				err = db.DB.Omit("Devices.*, DispatchRecords.Device").Create(&updateTransaction).Error
				Expect(err).ToNot(HaveOccurred())

				// soft delete the device to make it a candidate
				err = db.DB.Delete(&device, device.ID).Error
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("should return error when device candidate is not valid", func() {
			err := cleanupdevices.DeleteUpdateTransaction(&cleanupdevices.CandidateDevice{DeviceID: 125})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(cleanupdevices.ErrDeviceNotCleanUpCandidate))
		})

		It("should return error when update-transaction is not defined", func() {
			err := cleanupdevices.DeleteUpdateTransaction(&cleanupdevices.CandidateDevice{
				DeviceID: 125, DeviceDeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}, UpdateID: nil,
			})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(cleanupdevices.ErrUpdateTransactionIsNotDefined))
		})

		It("should delete update-transaction successfully", func() {
			updateTransactionID := updateTransaction.ID
			// ensure update transactions commits does exists
			type UpdateTransactionCommit struct {
				UpdateTransactionID uint `json:"update_transaction_id"`
				CommitID            uint `json:"commit_id"`
			}
			var updateTransactionCommits []UpdateTransactionCommit
			err := db.DB.Table("updatetransaction_commits").Select("update_transaction_id, commit_id").
				Where("update_transaction_id = ?", updateTransactionID).Scan(&updateTransactionCommits).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(updateTransactionCommits)).To(Equal(1))
			Expect(updateTransactionCommits[0].CommitID).To(Equal(oldCommit.ID))

			// ensure update-transaction devices does exist
			type UpdateTransactionDevice struct {
				UpdateTransactionID uint `json:"update_transaction_id"`
				DeviceID            uint `json:"device_id"`
			}
			var updateTransactionDevices []UpdateTransactionDevice
			err = db.DB.Table("updatetransaction_devices").Select("update_transaction_id, device_id").
				Where("update_transaction_id = ?", updateTransactionID).Scan(&updateTransactionDevices).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(updateTransactionDevices)).To(Equal(1))
			Expect(updateTransactionDevices[0].DeviceID).To(Equal(device.ID))

			// ensure update-transaction dispatcher-record does exit
			type UpdateTransactionDispatcherRecord struct {
				UpdateTransactionID uint `json:"update_transaction_id"`
				DispatchRecordID    uint `json:"dispatch_record_id"`
			}
			var updateTransactionDispatcherRecords []UpdateTransactionDispatcherRecord
			err = db.DB.Table("updatetransaction_dispatchrecords").Select("update_transaction_id, dispatch_record_id").
				Where("update_transaction_id = ?", updateTransactionID).Scan(&updateTransactionDispatcherRecords).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(updateTransactionDevices)).To(Equal(1))
			Expect(updateTransactionDevices[0].DeviceID).To(Equal(dispatcherRecord.ID))

			err = cleanupdevices.DeleteUpdateTransaction(&cleanupdevices.CandidateDevice{
				DeviceID:        device.ID,
				DeviceDeletedAt: device.DeletedAt,
				UpdateID:        &updateTransaction.ID,
				RepoID:          &updateRepo.ID,
				CommitID:        &commit.ID,
			})
			Expect(err).ToNot(HaveOccurred())

			// ensure update transaction does not exist
			err = db.DB.First(&updateTransaction, updateTransaction.ID).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))

			// ensure update-transaction repo does not exist
			err = db.DB.First(&updateRepo, updateRepo.ID).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))

			// ensure update transactions commits does not exists
			updateTransactionCommits = nil // clear current value
			err = db.DB.Table("updatetransaction_commits").Select("update_transaction_id, commit_id").
				Where("update_transaction_id = ?", updateTransactionID).Scan(&updateTransactionCommits).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(updateTransactionCommits)).To(Equal(0))

			// ensure update-transaction devices does not exist
			updateTransactionDevices = nil // clear current value
			err = db.DB.Table("updatetransaction_devices").Select("update_transaction_id, device_id").
				Where("update_transaction_id = ?", updateTransactionID).Scan(&updateTransactionDevices).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(updateTransactionDevices)).To(Equal(0))

			// ensure update-transaction dispatcher-record does not exit
			updateTransactionDispatcherRecords = nil // clear current value
			err = db.DB.Table("updatetransaction_dispatchrecords").Select("update_transaction_id, dispatch_record_id").
				Where("update_transaction_id = ?", updateTransactionID).Scan(&updateTransactionDispatcherRecords).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(updateTransactionDevices)).To(Equal(0))
		})
	})

	Context("DeleteDevice", func() {
		var orgID string
		var dispatcherRecord models.DispatchRecord
		var deviceGroup models.DeviceGroup
		var device models.Device

		BeforeEach(func() {
			// setup only once
			if orgID == "" {
				orgID = faker.UUIDHyphenated()

				device = models.Device{OrgID: orgID, UUID: faker.UUIDHyphenated()}
				err := db.DB.Create(&device).Error
				Expect(err).ToNot(HaveOccurred())

				deviceGroup = models.DeviceGroup{OrgID: orgID, Name: faker.Name(), Devices: []models.Device{device}}
				err = db.DB.Omit("Devices.*").Create(&deviceGroup).Error
				Expect(err).ToNot(HaveOccurred())

				dispatcherRecord = models.DispatchRecord{Device: &device, DeviceID: device.ID, PlaybookDispatcherID: faker.UUIDHyphenated()}
				err = db.DB.Omit("Device").Create(&dispatcherRecord).Error
				Expect(err).ToNot(HaveOccurred())

				// soft delete the device to make it a candidate
				err = db.DB.Delete(&device, device.ID).Error
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("should return error when device candidate is not valid", func() {
			err := cleanupdevices.DeleteUpdateTransaction(&cleanupdevices.CandidateDevice{DeviceID: 125})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(cleanupdevices.ErrDeviceNotCleanUpCandidate))
		})

		It("should return error when device is linked to update transaction", func() {
			var updateID uint = 123
			err := cleanupdevices.DeleteDevice(&cleanupdevices.CandidateDevice{
				DeviceID: 125, DeviceDeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}, UpdateID: &updateID,
			})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(cleanupdevices.ErrDeleteDeviceWithUpdateTransaction))
		})

		It("should delete device successfully", func() {
			deviceID := device.ID
			// ensure device does exists
			err := db.DB.Unscoped().First(&device, deviceID).Error
			Expect(err).ToNot(HaveOccurred())
			// ensure dispatcher-record does exists
			err = db.DB.Unscoped().First(&dispatcherRecord, dispatcherRecord.ID).Error
			Expect(err).ToNot(HaveOccurred())
			// ensure device-group device does exist
			err = db.DB.Unscoped().Preload("Devices").First(&deviceGroup, deviceGroup.ID).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(deviceGroup.Devices)).To(Equal(1))
			Expect(deviceGroup.Devices[0].ID).To(Equal(deviceID))

			err = cleanupdevices.DeleteDevice(&cleanupdevices.CandidateDevice{DeviceID: deviceID, DeviceDeletedAt: device.DeletedAt})
			Expect(err).ToNot(HaveOccurred())

			// ensure device does not exists
			err = db.DB.Unscoped().First(&device, deviceID).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))

			// ensure dispatcher-record does not exists
			err = db.DB.Unscoped().First(&dispatcherRecord, dispatcherRecord.ID).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))

			// ensure device-group device does not exist
			err = db.DB.Unscoped().Preload("Devices").First(&deviceGroup, deviceGroup.ID).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(deviceGroup.Devices)).To(Equal(0))
		})
	})

	Context("CleanUpUpdateTransaction", func() {

		It("should return error when device candidate is not valid", func() {
			err := cleanupdevices.CleanUpUpdateTransaction(nil, &cleanupdevices.CandidateDevice{DeviceID: 125})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(cleanupdevices.ErrDeviceNotCleanUpCandidate))
		})

		It("should return error when update-transaction is undefined", func() {
			err := cleanupdevices.CleanUpUpdateTransaction(
				nil,
				&cleanupdevices.CandidateDevice{DeviceID: 125, DeviceDeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
			)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(cleanupdevices.ErrUpdateTransactionIsNotDefined))
		})

		When("repo is update repo", func() {
			// a repo is an update repo when it has been built for the update-transaction and has "/upd/" as part of its url path
			var orgID string
			var updateTransaction models.UpdateTransaction
			var device models.Device
			var updateRepo models.Repo
			var updateRepoPath string
			var deviceCandidate *cleanupdevices.CandidateDevice

			BeforeEach(func() {
				// setup only once
				if orgID == "" {
					orgID = faker.UUIDHyphenated()

					device = models.Device{OrgID: orgID, UUID: faker.UUIDHyphenated()}
					err := db.DB.Create(&device).Error
					Expect(err).ToNot(HaveOccurred())

					// repo has "/upd/" as part of the path
					updateRepoPath = "repo/path/upd/" + faker.UUIDHyphenated()
					updateRepo = models.Repo{URL: "https://bucket.example.com/" + updateRepoPath, Status: models.ImageStatusPending}
					err = db.DB.Create(&updateRepo).Error
					Expect(err).ToNot(HaveOccurred())

					updateTransaction = models.UpdateTransaction{
						OrgID:   orgID,
						Devices: []models.Device{device},
						Repo:    &updateRepo,
					}
					err = db.DB.Omit("Devices.*").Create(&updateTransaction).Error
					Expect(err).ToNot(HaveOccurred())

					// soft delete the device to make it a candidate
					err = db.DB.Delete(&device, device.ID).Error
					Expect(err).ToNot(HaveOccurred())

					deviceCandidates, err := cleanupdevices.GetCandidateDevices(db.DB.Where("devices.org_id = ?", orgID))
					Expect(err).ToNot(HaveOccurred())
					Expect(len(deviceCandidates)).To(Equal(1))
					deviceCandidate = &deviceCandidates[0]
				}
			})

			It("deviceCandidate is correct", func() {
				Expect(deviceCandidate.DeviceID).To(Equal(device.ID))
				Expect(deviceCandidate.UpdateID).ToNot(BeNil())
				Expect(*deviceCandidate.UpdateID).To(Equal(updateTransaction.ID))
				Expect(deviceCandidate.RepoID).ToNot(BeNil())
				Expect(*deviceCandidate.RepoID).To(Equal(updateRepo.ID))
			})

			It("should return error when delete folder fails", func() {
				expectedError := errors.New("expected delete folder error")
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, updateRepoPath).Return(expectedError).Times(configDeleteAttempts)
				err := cleanupdevices.CleanUpUpdateTransaction(s3Client, deviceCandidate)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(expectedError))
			})

			It("should cleanup update transaction successfully", func() {
				// will succeed deleting folder on last attempts
				alternateError := errors.New("some aws s3 error")
				Expect(configDeleteAttempts > 1).To(BeTrue())
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, updateRepoPath).Return(alternateError).Times(configDeleteAttempts - 1)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, updateRepoPath).Return(nil).Times(1)
				err := cleanupdevices.CleanUpUpdateTransaction(s3Client, deviceCandidate)
				Expect(err).ToNot(HaveOccurred())
				// update transaction is deleted
				err = db.DB.First(&updateTransaction, updateTransaction.ID).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))

				// update-transaction repo is deleted
				err = db.DB.First(&updateRepo, updateRepo.ID).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))
			})
		})

		When("repo is commit repo", func() {
			// a repo is a commit repo when it has been built for the image commit and has no "/upd/" in path url
			var orgID string
			var updateTransaction models.UpdateTransaction
			var device models.Device
			var updateRepo models.Repo
			var updateRepoPath string
			var deviceCandidate *cleanupdevices.CandidateDevice

			BeforeEach(func() {
				// setup only once
				if orgID == "" {
					orgID = faker.UUIDHyphenated()

					device = models.Device{OrgID: orgID, UUID: faker.UUIDHyphenated()}
					err := db.DB.Create(&device).Error
					Expect(err).ToNot(HaveOccurred())

					// repo has not "/upd/" as part of the path
					updateRepoPath = "repo/path/" + faker.UUIDHyphenated()
					updateRepo = models.Repo{URL: "https://bucket.example.com/" + updateRepoPath, Status: models.ImageStatusPending}
					err = db.DB.Create(&updateRepo).Error
					Expect(err).ToNot(HaveOccurred())

					updateTransaction = models.UpdateTransaction{
						OrgID:   orgID,
						Devices: []models.Device{device},
						Repo:    &updateRepo,
					}
					err = db.DB.Omit("Devices.*").Create(&updateTransaction).Error
					Expect(err).ToNot(HaveOccurred())

					// soft delete the device to make it a candidate
					err = db.DB.Delete(&device, device.ID).Error
					Expect(err).ToNot(HaveOccurred())

					deviceCandidates, err := cleanupdevices.GetCandidateDevices(db.DB.Where("devices.org_id = ?", orgID))
					Expect(err).ToNot(HaveOccurred())
					Expect(len(deviceCandidates)).To(Equal(1))
					deviceCandidate = &deviceCandidates[0]
				}
			})

			It("deviceCandidate is correct", func() {
				Expect(deviceCandidate.DeviceID).To(Equal(device.ID))
				Expect(deviceCandidate.UpdateID).ToNot(BeNil())
				Expect(*deviceCandidate.UpdateID).To(Equal(updateTransaction.ID))
				Expect(deviceCandidate.RepoID).ToNot(BeNil())
				Expect(*deviceCandidate.RepoID).To(Equal(updateRepo.ID))
			})

			It("should cleanup update transaction successfully", func() {
				// delete repo is not called
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, updateRepoPath).Return(nil).Times(0)
				err := cleanupdevices.CleanUpUpdateTransaction(s3Client, deviceCandidate)
				Expect(err).ToNot(HaveOccurred())
				// update transaction is deleted
				err = db.DB.First(&updateTransaction, updateTransaction.ID).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))

				// update-transaction repo is deleted
				err = db.DB.First(&updateRepo, updateRepo.ID).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))
			})
		})
	})

	Context("CleanUpDevice", func() {
		var orgID string
		var updateTransaction models.UpdateTransaction
		var device models.Device
		var device2 models.Device
		var updateRepo models.Repo
		var updateRepoPath string
		var deviceCandidates []cleanupdevices.CandidateDevice

		BeforeEach(func() {
			// setup only once
			if orgID == "" {
				orgID = faker.UUIDHyphenated()

				device = models.Device{OrgID: orgID, UUID: faker.UUIDHyphenated()}
				err := db.DB.Create(&device).Error
				Expect(err).ToNot(HaveOccurred())

				device2 = models.Device{OrgID: orgID, UUID: faker.UUIDHyphenated()}
				err = db.DB.Create(&device2).Error
				Expect(err).ToNot(HaveOccurred())

				// repo has "/upd/" as part of the path
				updateRepoPath = "repo/path/upd/" + faker.UUIDHyphenated()
				updateRepo = models.Repo{URL: "https://bucket.example.com/" + updateRepoPath, Status: models.ImageStatusPending}
				err = db.DB.Create(&updateRepo).Error
				Expect(err).ToNot(HaveOccurred())

				updateTransaction = models.UpdateTransaction{
					OrgID:   orgID,
					Devices: []models.Device{device},
					Repo:    &updateRepo,
				}
				err = db.DB.Omit("Devices.*").Create(&updateTransaction).Error
				Expect(err).ToNot(HaveOccurred())

				// soft delete the device to make it a candidate
				err = db.DB.Delete(&device, device.ID).Error
				Expect(err).ToNot(HaveOccurred())
				// soft delete the device2 to make it a candidate
				err = db.DB.Delete(&device2, device2.ID).Error
				Expect(err).ToNot(HaveOccurred())

				deviceCandidates, err = cleanupdevices.GetCandidateDevices(db.DB.Where("devices.org_id = ?", orgID))
				Expect(err).ToNot(HaveOccurred())
				Expect(len(deviceCandidates)).To(Equal(2))
			}
		})

		It("should return error when device candidate is not valid", func() {
			err := cleanupdevices.CleanUpDevice(nil, &cleanupdevices.CandidateDevice{DeviceID: 125})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(cleanupdevices.ErrDeviceNotCleanUpCandidate))
		})

		It("deviceCandidates data collection is correct", func() {
			Expect(len(deviceCandidates)).To(Equal(2))
			Expect(deviceCandidates[0].DeviceID).To(Equal(device.ID))
			Expect(deviceCandidates[0].UpdateID).ToNot(BeNil())
			Expect(*deviceCandidates[0].UpdateID).To(Equal(updateTransaction.ID))
			Expect(deviceCandidates[0].RepoID).ToNot(BeNil())
			Expect(*deviceCandidates[0].RepoID).To(Equal(updateRepo.ID))
			Expect(deviceCandidates[0].RepoURL).ToNot(BeNil())
			Expect(*deviceCandidates[0].RepoURL).To(Equal(updateRepo.URL))
			Expect(deviceCandidates[1].DeviceID).To(Equal(device2.ID))
			Expect(deviceCandidates[1].UpdateID).To(BeNil())
		})

		It("should return error when delete folder fails", func() {
			expectedError := errors.New("expected delete folder error")
			s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, updateRepoPath).Return(expectedError).Times(configDeleteAttempts)
			err := cleanupdevices.CleanUpDevice(s3Client, &deviceCandidates[0])
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(expectedError))
		})

		It("should cleanup update transaction successfully", func() {
			// delete repo is not called
			s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, updateRepoPath).Return(nil).Times(1)
			err := cleanupdevices.CleanUpDevice(s3Client, &deviceCandidates[0])
			Expect(err).ToNot(HaveOccurred())
			// update transaction is deleted
			err = db.DB.First(&updateTransaction, updateTransaction.ID).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))

			// update-transaction repo is deleted
			err = db.DB.First(&updateRepo, updateRepo.ID).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))

			// at this stage device is not deleted forever
		})

		It("should delete device2 successfully", func() {
			err := cleanupdevices.CleanUpDevice(s3Client, &deviceCandidates[1])
			Expect(err).ToNot(HaveOccurred())
			// ensure device2 transaction is deleted
			err = db.DB.First(&device2, device2.ID).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))
		})

		It("device became a candidate after it's update-transaction was deleted", func() {
			// device2 is not more a candidate as it was deleted forever
			// device still a candidate but this time without update-transaction next call to ClearDevice should delete it forever as device2
			deviceCandidates, err := cleanupdevices.GetCandidateDevices(db.DB.Where("devices.org_id = ?", orgID))
			Expect(err).ToNot(HaveOccurred())
			Expect(len(deviceCandidates)).To(Equal(1))
			Expect(deviceCandidates[0].DeviceID).To(Equal(device.ID))
			Expect(deviceCandidates[0].UpdateID).To(BeNil())
		})
	})

	Context("CleanupOrphanDevicesUpdates", func() {
		var orgID string
		var image models.Image
		var device models.Device
		var updateTransaction models.UpdateTransaction
		var updateTransaction2 models.UpdateTransaction
		var updateTransaction3 models.UpdateTransaction
		var updateRepoPath string
		var updateRepoPath2 string

		BeforeEach(func() {
			err := os.Setenv(feature.CleanUPDevices.EnvVar, "true")
			Expect(err).NotTo(HaveOccurred())

			// setup only once
			if orgID == "" {
				orgID = faker.UUIDHyphenated()
				image = models.Image{OrgID: orgID, Commit: &models.Commit{OrgID: orgID, Repo: &models.Repo{URL: faker.URL()}}}
				err := db.DB.Create(&image).Error
				Expect(err).ToNot(HaveOccurred())

				device = models.Device{OrgID: orgID, UUID: faker.UUIDHyphenated()}
				err = db.DB.Create(&device).Error
				Expect(err).ToNot(HaveOccurred())

				updateRepoPath = "repo/path/upd/" + faker.UUIDHyphenated()

				updateTransaction = models.UpdateTransaction{
					OrgID:    orgID,
					CommitID: image.Commit.ID,
					Commit:   image.Commit,
					// update transaction without device, but with DispatchRecords
					DispatchRecords: []models.DispatchRecord{
						{DeviceID: device.ID},
					},
					Repo: &models.Repo{URL: "https://bucket.example.com/" + updateRepoPath, Status: models.ImageStatusPending},
				}
				err = db.DB.Create(&updateTransaction).Error
				Expect(err).ToNot(HaveOccurred())

				updateTransaction2 = models.UpdateTransaction{
					OrgID:    orgID,
					CommitID: image.Commit.ID,
					Commit:   image.Commit,
					// update transaction without device, but with DispatchRecords
					DispatchRecords: []models.DispatchRecord{
						{DeviceID: device.ID},
					},
					Repo: &models.Repo{Status: models.ImageStatusBuilding},
				}
				err = db.DB.Create(&updateTransaction2).Error
				Expect(err).ToNot(HaveOccurred())

				updateRepoPath2 = fmt.Sprintf("repo/path/%d/", updateTransaction2.Repo.ID) + faker.UUIDHyphenated()
				updateTransaction2.Repo.URL = "https://bucket.example.com/" + updateRepoPath2
				err = db.DB.Save(&updateTransaction2.Repo).Error
				Expect(err).ToNot(HaveOccurred())

				updateTransaction3 = models.UpdateTransaction{
					OrgID:    orgID,
					CommitID: image.Commit.ID,
					Commit:   image.Commit,
					// update transaction with device and with DispatchRecords
					Devices: []models.Device{device},
					DispatchRecords: []models.DispatchRecord{
						{DeviceID: device.ID},
					},
					Repo: &models.Repo{Status: models.ImageStatusBuilding},
				}
				err = db.DB.Omit("Devices.*").Create(&updateTransaction3).Error
				Expect(err).ToNot(HaveOccurred())

				// delete device to allow it to be a candidate
				err = db.DB.Delete(&device).Error
				Expect(err).ToNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			err := os.Unsetenv(feature.CleanUPDevices.EnvVar)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should cleanup updatetransaction repo successfully and delete update-transactions", func() {
			type UpdateTransactionDispatcherRecord struct {
				UpdateTransactionID uint `json:"update_transaction_id"`
				DispatchRecordID    uint `json:"dispatch_record_id"`
			}
			var updateTransactionDispatcherRecord UpdateTransactionDispatcherRecord

			s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, updateRepoPath).Return(nil).Times(1)
			s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, updateRepoPath2).Return(nil).Times(1)
			err := cleanupdevices.CleanupOrphanDevicesUpdates(s3Client, db.DB.Where("devices.org_id = ?", orgID))
			Expect(err).ToNot(HaveOccurred())

			// updateTransaction does not exist
			err = db.DB.Unscoped().First(&updateTransaction, updateTransaction.ID).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))

			// updateTransaction dispatcher record does not exist
			err = db.DB.Table("updatetransaction_dispatchrecords").
				Where("update_transaction_id=?", updateTransaction.ID).
				First(&updateTransactionDispatcherRecord).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))

			// updateTransaction2 does not exist
			err = db.DB.Unscoped().First(&updateTransaction2, updateTransaction2.ID).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))
			// updateTransaction2 dispatcher record does not exist
			err = db.DB.Table("updatetransaction_dispatchrecords").
				Where("update_transaction_id=?", updateTransaction2.ID).
				First(&updateTransactionDispatcherRecord).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))

			// updateTransaction3 still exist
			err = db.DB.Preload("DispatchRecords").First(&updateTransaction3, updateTransaction3.ID).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(updateTransaction3.DispatchRecords)).To(Equal(1))

			// updateTransaction3 dispatcher record still exist
			err = db.DB.Table("updatetransaction_dispatchrecords").
				Where("update_transaction_id=?", updateTransaction3.ID).
				First(&updateTransactionDispatcherRecord).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(updateTransactionDispatcherRecord.UpdateTransactionID).To(Equal(updateTransaction3.ID))
			Expect(updateTransactionDispatcherRecord.DispatchRecordID).To(Equal(updateTransaction3.DispatchRecords[0].ID))
		})
	})

	When("feature flag is disabled", func() {

		BeforeEach(func() {
			err := os.Unsetenv(feature.CleanUPDevices.EnvVar)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not run cleanup devices", func() {
			image := models.Image{Name: faker.UUIDHyphenated()}
			Expect(image.DeletedAt.Value()).To(BeNil())
			err := cleanupdevices.CleanupAllDevices(nil, nil)
			Expect(err).To(MatchError(cleanupdevices.ErrCleanupDevicesNotAvailable))
		})
	})

	When("feature flag is enabled", func() {
		var orgID string
		var image models.Image

		var updateTransaction models.UpdateTransaction
		var updateTransaction2 models.UpdateTransaction
		var orphanUpdateTransaction models.UpdateTransaction
		var device models.Device
		var device2 models.Device
		var device3 models.Device
		var undeletedDevice models.Device
		// var updateRepo models.Repo
		var updateRepoPath string
		var updateRepoPath2 string
		var orphanUpdateRepoPath string

		BeforeEach(func() {
			err := os.Setenv(feature.CleanUPDevices.EnvVar, "true")
			Expect(err).NotTo(HaveOccurred())

			// setup only once
			if orgID == "" {
				orgID = faker.UUIDHyphenated()

				image = models.Image{OrgID: orgID, Commit: &models.Commit{OrgID: orgID, Repo: &models.Repo{URL: faker.URL()}}}
				err := db.DB.Create(&image).Error
				Expect(err).ToNot(HaveOccurred())

				device = models.Device{OrgID: orgID, UUID: faker.UUIDHyphenated()}
				err = db.DB.Create(&device).Error
				Expect(err).ToNot(HaveOccurred())

				undeletedDevice = models.Device{OrgID: orgID, UUID: faker.UUIDHyphenated()}
				err = db.DB.Create(&undeletedDevice).Error
				Expect(err).ToNot(HaveOccurred())

				device2 = models.Device{OrgID: orgID, UUID: faker.UUIDHyphenated()}
				err = db.DB.Create(&device2).Error
				Expect(err).ToNot(HaveOccurred())

				device3 = models.Device{OrgID: orgID, UUID: faker.UUIDHyphenated()}
				err = db.DB.Create(&device3).Error
				Expect(err).ToNot(HaveOccurred())

				updateRepoPath = "repo/path/upd/" + faker.UUIDHyphenated()
				// updateRepo = models.Repo{URL: "https://bucket.example.com/" + updateRepoPath, Status: models.ImageStatusPending}
				// err = db.DB.Create(&updateRepo).Error
				// Expect(err).ToNot(HaveOccurred())

				updateTransaction = models.UpdateTransaction{
					OrgID:    orgID,
					CommitID: image.Commit.ID,
					Commit:   image.Commit,
					Devices:  []models.Device{device},
					Repo:     &models.Repo{URL: "https://bucket.example.com/" + updateRepoPath, Status: models.ImageStatusPending},
				}
				err = db.DB.Omit("Devices.*").Create(&updateTransaction).Error
				Expect(err).ToNot(HaveOccurred())

				updateRepoPath2 = "repo/path/upd/" + faker.UUIDHyphenated()
				updateTransaction2 = models.UpdateTransaction{
					OrgID:   orgID,
					Commit:  &models.Commit{OrgID: orgID, Repo: &models.Repo{URL: faker.URL()}},
					Devices: []models.Device{device2},
					Repo:    &models.Repo{URL: "https://bucket.example.com/" + updateRepoPath2, Status: models.ImageStatusPending},
				}
				err = db.DB.Omit("Devices.*").Create(&updateTransaction2).Error
				Expect(err).ToNot(HaveOccurred())

				orphanUpdateTransaction = models.UpdateTransaction{
					OrgID:  orgID,
					Commit: &models.Commit{OrgID: orgID, Repo: &models.Repo{URL: faker.URL()}},
					// update transaction without device, but with DispatchRecords
					DispatchRecords: []models.DispatchRecord{
						{DeviceID: device.ID},
					},
					Repo: &models.Repo{Status: models.ImageStatusBuilding},
				}
				err = db.DB.Create(&orphanUpdateTransaction).Error
				Expect(err).ToNot(HaveOccurred())
				// update repo url
				orphanUpdateRepoPath = fmt.Sprintf("repo/path/%d/", orphanUpdateTransaction.Repo.ID) + faker.UUIDHyphenated()
				orphanUpdateTransaction.Repo.URL = "https://bucket.example.com/" + orphanUpdateRepoPath
				err = db.DB.Save(&orphanUpdateTransaction.Repo).Error
				Expect(err).ToNot(HaveOccurred())

				// soft delete the devices to make them candidates
				err = db.DB.Delete(&device, device.ID).Error
				Expect(err).ToNot(HaveOccurred())
				err = db.DB.Delete(&device2, device2.ID).Error
				Expect(err).ToNot(HaveOccurred())
				err = db.DB.Delete(&device3, device3.ID).Error
				Expect(err).ToNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			err := os.Unsetenv(feature.CleanUPDevices.EnvVar)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("GetCandidateDevices", func() {
			It("should return the expected devices records", func() {
				deviceCandidates, err := cleanupdevices.GetCandidateDevices(db.DB.Where("devices.org_id = ?", orgID))
				Expect(err).ToNot(HaveOccurred())
				Expect(deviceCandidates).ToNot(BeNil())
				Expect(len(deviceCandidates)).To(Equal(3))
				Expect(deviceCandidates[0].DeviceID).To(Equal(device.ID))
				Expect(deviceCandidates[0].DeviceDeletedAt.Time.Equal(device.DeletedAt.Time)).To(BeTrue())
				Expect(*deviceCandidates[0].UpdateID).To(Equal(updateTransaction.ID))
				Expect(*deviceCandidates[0].RepoID).To(Equal(updateTransaction.Repo.ID))
				Expect(*deviceCandidates[0].RepoURL).To(Equal(updateTransaction.Repo.URL))
				Expect(*deviceCandidates[0].CommitID).To(Equal(updateTransaction.CommitID))
				Expect(*deviceCandidates[0].ImageID).To(Equal(image.ID))
				Expect(*deviceCandidates[0].CommitRepoID).To(Equal(image.Commit.Repo.ID))

				Expect(deviceCandidates[1].DeviceID).To(Equal(device2.ID))
				Expect(deviceCandidates[1].DeviceDeletedAt.Time.Equal(device2.DeletedAt.Time)).To(BeTrue())
				Expect(*deviceCandidates[1].UpdateID).To(Equal(updateTransaction2.ID))
				Expect(*deviceCandidates[1].RepoID).To(Equal(updateTransaction2.Repo.ID))
				Expect(*deviceCandidates[1].RepoURL).To(Equal(updateTransaction2.Repo.URL))
				Expect(*deviceCandidates[1].CommitID).To(Equal(updateTransaction2.CommitID))
				Expect(deviceCandidates[1].ImageID).To(BeNil())
				Expect(*deviceCandidates[1].CommitRepoID).To(Equal(updateTransaction2.Commit.Repo.ID))

				Expect(deviceCandidates[2].DeviceID).To(Equal(device3.ID))
				Expect(deviceCandidates[2].DeviceDeletedAt.Time.Equal(device3.DeletedAt.Time)).To(BeTrue())
				Expect(deviceCandidates[2].UpdateID).To(BeNil())
				Expect(deviceCandidates[2].RepoID).To(BeNil())
				Expect(deviceCandidates[2].RepoURL).To(BeNil())
				Expect(deviceCandidates[2].CommitID).To(BeNil())
				Expect(deviceCandidates[2].ImageID).To(BeNil())
				Expect(deviceCandidates[2].CommitRepoID).To(BeNil())
			})
		})

		Context("CleanupAllDevices", func() {
			var initialDefaultDataLimit int
			BeforeEach(func() {
				initialDefaultDataLimit = cleanupdevices.DefaultDataLimit
				cleanupdevices.DefaultDataLimit = 1
			})

			AfterEach(func() {
				// restore DefaultDataLimit
				cleanupdevices.DefaultDataLimit = initialDefaultDataLimit
			})

			It("should return error when orphan updateTransaction repo folder delete failed", func() {
				expectedError := errors.New("an expected folder delete error")
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, orphanUpdateRepoPath).Return(expectedError).Times(configDeleteAttempts)
				err := cleanupdevices.CleanupAllDevices(s3Client, db.DB.Where("devices.org_id = ?", orgID))
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(cleanupdevices.ErrCleanUpAllDevicesInterrupted))
			})

			It("should return error when folder delete failed", func() {
				expectedError := errors.New("an expected folder delete error")
				// make the orphan device update repo to pass
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, orphanUpdateRepoPath).Return(nil).Times(1)
				// mnke the other update repos folders to fail
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, updateRepoPath).Return(expectedError).Times(configDeleteAttempts)
				err := cleanupdevices.CleanupAllDevices(s3Client, db.DB.Where("devices.org_id = ?", orgID))
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(cleanupdevices.ErrCleanUpAllDevicesInterrupted))
			})

			It("cleanup devices successfully", func() {
				// the orphan update repo was deleted in the previous test and should not be called
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, orphanUpdateRepoPath).Return(nil).Times(0)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, updateRepoPath).Return(nil).Times(1)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, updateRepoPath2).Return(nil).Times(1)

				err := cleanupdevices.CleanupAllDevices(s3Client, db.DB.Where("devices.org_id = ?", orgID))
				Expect(err).ToNot(HaveOccurred())
				// device does not exist
				err = db.DB.First(&device, device.ID).Error
				Expect(err).To(HaveOccurred())
				// device2 does not exist
				err = db.DB.First(&device2, device2.ID).Error
				Expect(err).To(HaveOccurred())
				// device2 does not exist
				err = db.DB.First(&device3, device3.ID).Error
				Expect(err).To(HaveOccurred())

				// updateTransaction does not exist
				err = db.DB.First(&updateTransaction, updateTransaction.ID).Error
				Expect(err).To(HaveOccurred())
				// updateTransaction commit still exist (because has image)
				err = db.DB.First(&updateTransaction.Commit, updateTransaction.Commit.ID).Error
				Expect(err).ToNot(HaveOccurred())

				// updateTransaction2 does not exist
				err = db.DB.First(&updateTransaction2, updateTransaction2.ID).Error
				Expect(err).To(HaveOccurred())

				// undeletedDevice still exist (because not a candidate)
				err = db.DB.First(&undeletedDevice, undeletedDevice.ID).Error
				Expect(err).ToNot(HaveOccurred())

				// orphanUpdateTransaction does not exists
				err = db.DB.First(&orphanUpdateTransaction, orphanUpdateTransaction.ID).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))
			})
		})
	})
})
