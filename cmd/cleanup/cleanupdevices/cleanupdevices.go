package cleanupdevices

import (
	"errors"
	"fmt"
	"strings"

	"github.com/redhatinsights/edge-api/cmd/cleanup/storage"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/files"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ErrCleanupDevicesNotAvailable error returned when the cleanup devices feature flag is disabled
var ErrCleanupDevicesNotAvailable = errors.New("cleanup devices is not available")

// ErrDeviceNotCleanUpCandidate error returned when the device is not a cleanup candidate
var ErrDeviceNotCleanUpCandidate = errors.New("device is not a cleanup candidate")

// ErrCleanUpAllDevicesInterrupted error returned when an error is return from anu Device cleanup function
var ErrCleanUpAllDevicesInterrupted = errors.New("cleanup all devices is interrupted")

// ErrDeleteDeviceWithUpdateTransaction error returned when deleting a device that is linked to an update-transaction
var ErrDeleteDeviceWithUpdateTransaction = errors.New("device with update-transaction cannot be deleted")

// ErrUpdateTransactionIsNotDefined error retuned when update transaction is not defined
var ErrUpdateTransactionIsNotDefined = errors.New("update transaction is not defined")

// DefaultDataLimit the default data limit to use when collecting data
var DefaultDataLimit = 100

// DefaultMaxDataPageNumber the default data pages to handle as preventive way to enter an indefinite loop
var DefaultMaxDataPageNumber = 3000

// CandidateDevice the device cleanup candidate
type CandidateDevice struct {
	DeviceID        uint           `json:"device_id"`
	DeviceDeletedAt gorm.DeletedAt `json:"device_deleted_at"`
	UpdateID        *uint          `json:"update_id,omitempty"`
	RepoID          *uint          `json:"repo_id,omitempty"`
	RepoStatus      *string        `json:"repo_status,omitempty"`
	RepoURL         *string        `json:"repo_url"`
	CommitID        *uint          `json:"commit_id"`
	ImageID         *uint          `json:"image_id"`
	CommitRepoID    *uint          `json:"commit_repo_id"`
}

// IsDeviceCandidate to ensure the device is a candidate
func IsDeviceCandidate(candidateDevice *CandidateDevice) error {
	if deviceDeletedAtValue, err := candidateDevice.DeviceDeletedAt.Value(); err != nil {
		return err
	} else if deviceDeletedAtValue == nil {
		return ErrDeviceNotCleanUpCandidate
	}
	return nil
}

func DeleteCommit(tx *gorm.DB, candidateDevice *CandidateDevice) error {
	if err := IsDeviceCandidate(candidateDevice); err != nil {
		return err
	}

	if candidateDevice.CommitID == nil {
		// the commit does not exist for the candidate device record
		// do nothing and do not return error
		return nil
	}

	// delete commit only if it has no image
	if candidateDevice.ImageID != nil {
		// do nothing and do not return error
		// this will be handled by cleanup-images when update-transaction will be deleted
		return nil
	}

	// assume that the update transaction has already been deleted

	// delete commit from updatetransaction_devices with commit_id
	if err := tx.Exec("DELETE FROM updatetransaction_commits WHERE commit_id=?", *candidateDevice.CommitID).Error; err != nil {
		return err
	}

	// delete commit_installed_packages with commit_id
	if err := tx.Exec("DELETE FROM commit_installed_packages WHERE commit_id=?", *candidateDevice.CommitID).Error; err != nil {
		return err
	}

	// delete commit with commit_id
	if err := tx.Exec("DELETE FROM commits WHERE id=?", *candidateDevice.CommitID).Error; err != nil {
		return err
	}

	if candidateDevice.CommitRepoID != nil {
		// delete commit repo with commit_repo_id
		// assume we are here because image_id is nil, which mean that the commit repo has been cleared by cleanup-images
		if err := tx.Exec("DELETE FROM repos WHERE id=?", *candidateDevice.CommitRepoID).Error; err != nil {
			return err
		}
	}
	return nil
}

func DeleteUpdateTransaction(candidateDevice *CandidateDevice) error {
	if err := IsDeviceCandidate(candidateDevice); err != nil {
		return err
	}

	if candidateDevice.UpdateID == nil {
		return ErrUpdateTransactionIsNotDefined
	}

	logger := log.WithFields(log.Fields{
		"device_id": candidateDevice.DeviceID,
		"update_id": *candidateDevice.UpdateID,
	})

	logger.Debug("deleting update permanently")

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// delete update-transaction from updatetransaction_dispatchrecords with update_transaction_id
		if err := tx.Exec("DELETE FROM updatetransaction_dispatchrecords WHERE update_transaction_id=?", *candidateDevice.UpdateID).Error; err != nil {
			return err
		}

		// delete update-transaction from updatetransaction_devices with update_transaction_id
		if err := tx.Exec("DELETE FROM updatetransaction_devices WHERE update_transaction_id=?", *candidateDevice.UpdateID).Error; err != nil {
			return err
		}

		// delete update-transaction from updatetransaction_commits with update_transaction_id
		if err := tx.Exec("DELETE FROM updatetransaction_commits WHERE update_transaction_id=?", *candidateDevice.UpdateID).Error; err != nil {
			return err
		}

		// delete update-transaction with id
		if err := tx.Unscoped().Where("id", *candidateDevice.UpdateID).Delete(&models.UpdateTransaction{}).Error; err != nil {
			return err
		}

		// delete update-transaction repo
		if candidateDevice.RepoID != nil {
			if err := tx.Unscoped().Where("id", *candidateDevice.RepoID).Delete(&models.Repo{}).Error; err != nil {
				return err
			}
		}

		// NOTE: COMMIT WILL BE DELETED IN IT'S OWN FUNCTION

		return nil
	})

	if err != nil {
		logger.WithField("error", err.Error()).Error("error occurred while deleting update")
	}

	return err
}

func DeleteDevice(candidateDevice *CandidateDevice) error {
	if err := IsDeviceCandidate(candidateDevice); err != nil {
		return err
	}

	// delete device only when it has no update-transaction
	if candidateDevice.UpdateID != nil {
		return ErrDeleteDeviceWithUpdateTransaction
	}

	logger := log.WithField("device_id", candidateDevice.DeviceID)

	logger.Debug("deleting device permanently")

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// delete device dispatch_records
		if err := tx.Exec("DELETE FROM dispatch_records WHERE device_id=?", candidateDevice.DeviceID).Error; err != nil {
			return err
		}

		// delete device from any device_groups_devices
		if err := tx.Exec("DELETE FROM device_groups_devices WHERE device_id=?", candidateDevice.DeviceID).Error; err != nil {
			return err
		}

		// delete device
		err := tx.Unscoped().Where("id", candidateDevice.DeviceID).Delete(&models.Device{}).Error

		return err
	})

	if err != nil {
		logger.WithField("error", err.Error()).Error("error occurred while deleting device")
	}

	return err
}

func CleanUpUpdateTransaction(s3Client *files.S3Client, candidateDevice *CandidateDevice) error {
	if err := IsDeviceCandidate(candidateDevice); err != nil {
		return err
	}

	if candidateDevice.UpdateID == nil {
		return ErrUpdateTransactionIsNotDefined
	}

	logger := log.WithFields(log.Fields{
		"device_id": candidateDevice.DeviceID,
		"update_id": *candidateDevice.UpdateID,
	})

	if candidateDevice.RepoID != nil && candidateDevice.RepoURL != nil &&
		*candidateDevice.RepoURL != "" &&
		(strings.Contains(*candidateDevice.RepoURL, "/upd/") || strings.Contains(*candidateDevice.RepoURL, fmt.Sprintf("/%d/", *candidateDevice.RepoID))) {
		// this is an update repo and was build when updating a device
		// note update repo has //upd/ or update-transaction id as part of the repo path
		logger = logger.WithFields(log.Fields{
			"repo_url": *candidateDevice.RepoURL,
		})

		urlPath, err := storage.GetPathFromURL(*candidateDevice.RepoURL)
		if err != nil {
			logger.WithField("error", err.Error()).Error("error occurred while getting resource path url")
			return err
		}
		logger = logger.WithField("repo_url_path", urlPath)
		logger.Debug("deleting repo directory")
		err = storage.DeleteAWSFolder(s3Client, urlPath)
		if err != nil {
			logger.WithField("error", err.Error()).Error("error occurred while deleting repo directory")
			return err
		}
		// clean url and update cleaned status
		if err := db.DB.Model(&models.Repo{}).Where("id", candidateDevice.RepoID).
			Updates(map[string]interface{}{"status": models.UpdateStatusStorageCleaned, "url": ""}).Error; err != nil {
			logger.WithField("error", err.Error()).Error("error occurred while updating repo status to cleaned")
			return err
		}
	}

	if err := DeleteUpdateTransaction(candidateDevice); err != nil {
		return nil
	}
	return nil
}

func CleanUpDevice(s3Client *files.S3Client, candidateDevice *CandidateDevice) error {
	if err := IsDeviceCandidate(candidateDevice); err != nil {
		return err
	}
	var err error
	if candidateDevice.UpdateID != nil {
		err = CleanUpUpdateTransaction(s3Client, candidateDevice)
	} else {
		// we will receive a device without update-transaction when all update-transactions has been cleaned/deleted
		err = DeleteDevice(candidateDevice)
	}
	return err
}

// GetOrphanDevicesUpdatesCandidates returns the data set of orphan devices cleanup candidates
// orphan devices updates are devices that are linked to dispatcher-records, the dispatcher record is linked to update-transaction
// but the device is not linked to the update-transaction
func GetOrphanDevicesUpdatesCandidates(gormDB *gorm.DB) ([]CandidateDevice, error) {

	var candidateDevices []CandidateDevice
	err := gormDB.Debug().Table("devices").
		Select(`devices.id AS device_id, devices.deleted_at AS device_deleted_at, update_transactions.id AS update_id, repos.id AS repo_id, 
repos.status AS repo_status, repos.URL AS repo_url, update_transactions.commit_id AS commit_id, images.id AS image_id, commits.repo_id as commit_repo_id`).
		Joins("JOIN dispatch_records ON dispatch_records.device_id = devices.id").
		Joins("JOIN updatetransaction_dispatchrecords ON updatetransaction_dispatchrecords.dispatch_record_id = dispatch_records.id").
		Joins("JOIN update_transactions ON update_transactions.id = updatetransaction_dispatchrecords.update_transaction_id").
		Joins("LEFT JOIN updatetransaction_devices ON updatetransaction_devices.update_transaction_id = updatetransaction_dispatchrecords.update_transaction_id").
		Joins("LEFT JOIN repos ON repos.id = update_transactions.repo_id").
		Joins("LEFT JOIN images ON images.commit_id = update_transactions.commit_id").
		Joins("LEFT JOIN commits ON commits.id = update_transactions.commit_id").
		Where("devices.deleted_at IS NOT NULL AND updatetransaction_devices.device_id IS NULL").
		Order("devices.id ASC").
		Order("update_transactions.id ASC").
		Limit(DefaultDataLimit).
		Scan(&candidateDevices).Error

	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Error("error occurred when collecting orphan devices updates candidates")
	}

	return candidateDevices, err
}

// CleanupOrphanDevicesUpdates cleanup all orphan devices updates candidates
// orphan devices updates are devices that are linked to dispatcher-records, the dispatcher record is linked to update-transaction
// but the device is not linked to the update-transaction
func CleanupOrphanDevicesUpdates(s3Client *files.S3Client, gormDB *gorm.DB) error {

	devicesUpdatesCount := 0
	page := 0
	for page < DefaultMaxDataPageNumber && feature.CleanUPDevices.IsEnabled() {
		candidateDevices, err := GetOrphanDevicesUpdatesCandidates(gormDB)
		if err != nil {
			return err
		}

		if len(candidateDevices) == 0 {
			break
		}

		// create a new channel for each iteration
		errChan := make(chan error)

		for _, deviceCandidate := range candidateDevices {
			deviceCandidate := deviceCandidate
			// handle all the page devices candidates at once, by default 100
			go func(resChan chan error) {
				// for orphan device update candidate need to clean up the update-transaction only.
				// the device will be deleted by main function CleanupAllDevices
				resChan <- CleanUpUpdateTransaction(s3Client, &deviceCandidate)
			}(errChan)
		}

		// wait for all results to be returned
		errCount := 0
		for range candidateDevices {
			resError := <-errChan
			if resError != nil {
				// errors are logged in the related functions, at this stage need to know if there is an error, to break the loop
				errCount++
			}
		}

		close(errChan)

		devicesUpdatesCount += len(candidateDevices)

		// break on any error
		if errCount > 0 {
			log.WithFields(log.Fields{"devices_updates_count": devicesUpdatesCount, "errors_count": errCount}).Info("cleanup devices was interrupted because of cleanup errors")
			return ErrCleanUpAllDevicesInterrupted
		}
		page++
	}

	log.WithField("devices_updates_count", devicesUpdatesCount).Info("cleanup devices finished")

	return nil
}

// GetCandidateDevices returns the data set of devices cleanup candidates
func GetCandidateDevices(gormDB *gorm.DB) ([]CandidateDevice, error) {
	var candidateDevices []CandidateDevice

	err := gormDB.Debug().Table("devices").
		Select(`devices.id AS device_id, devices.deleted_at AS device_deleted_at, update_transactions.id AS update_id, repos.id AS	repo_id,
repos.status AS repo_status, repos.URL AS repo_url,	update_transactions.commit_id AS commit_id,	images.id AS image_id, commits.repo_id as commit_repo_id`).
		Joins("LEFT JOIN updatetransaction_devices ON updatetransaction_devices.device_id = devices.id").
		Joins("LEFT JOIN update_transactions ON update_transactions.id = updatetransaction_devices.update_transaction_id").
		Joins("LEFT JOIN repos ON repos.id = update_transactions.repo_id").
		Joins("LEFT JOIN images ON images.commit_id = update_transactions.commit_id").
		Joins("LEFT JOIN commits ON commits.id = update_transactions.commit_id").
		Joins("LEFT JOIN repos as commit_repos ON commit_repos.id = commits.repo_id").
		Where("devices.deleted_at IS NOT NULL").
		Order("devices.id ASC").
		Order("update_transactions.id ASC").
		Limit(DefaultDataLimit).
		Scan(&candidateDevices).Error

	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Error("error occurred when collecting devices candidate")
	}

	return candidateDevices, err
}

func CleanupAllDevices(s3Client *files.S3Client, gormDB *gorm.DB) error {
	if !feature.CleanUPDevices.IsEnabled() {
		log.Warning("cleanup of devices feature flag is disabled")
		return ErrCleanupDevicesNotAvailable
	}
	if gormDB == nil {
		gormDB = db.DB
	}

	// clean up orphan devices updates first
	if err := CleanupOrphanDevicesUpdates(s3Client, gormDB); err != nil {
		return err
	}

	devicesUpdatesCount := 0
	page := 0
	for page < DefaultMaxDataPageNumber && feature.CleanUPDevices.IsEnabled() {
		candidateDevices, err := GetCandidateDevices(gormDB)
		if err != nil {
			return err
		}

		if len(candidateDevices) == 0 {
			break
		}

		// create a new channel for each iteration
		errChan := make(chan error)

		for _, deviceCandidate := range candidateDevices {
			deviceCandidate := deviceCandidate
			// handle all the page devices candidates at once, by default 30
			go func(resChan chan error) {
				resChan <- CleanUpDevice(s3Client, &deviceCandidate)
			}(errChan)
		}

		// wait for all results to be returned
		errCount := 0
		for range candidateDevices {
			resError := <-errChan
			if resError != nil {
				// errors are logged in the related functions, at this stage need to know if there is an error, to break the loop
				errCount++
			}
		}

		close(errChan)

		devicesUpdatesCount += len(candidateDevices)

		// break on any error
		if errCount > 0 {
			log.WithFields(log.Fields{"devices_updates_count": devicesUpdatesCount, "errors_count": errCount}).Info("cleanup devices was interrupted because of cleanup errors")
			return ErrCleanUpAllDevicesInterrupted
		}
		page++
	}

	log.WithField("devices_updates_count", devicesUpdatesCount).Info("cleanup devices finished")
	return nil
}
