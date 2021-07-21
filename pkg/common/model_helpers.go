package common

import (
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// GetCommitByID
// Pass in CommitID uint and get a *models.Commit back
func GetCommitByID(commitID uint) (*models.Commit, error) {
	log.Debugf("GetCommitByID::commitID: %#v", commitID)
	var commit models.Commit
	result := db.DB.First(&commit, commitID)
	log.Debugf("GetCommitByID::result: %#v", result)
	log.Debugf("GetCommitByID::commit: %#v", commit)
	if result.Error != nil {
		return nil, result.Error
	}
	return &commit, nil
}

// GetCommitByOSTreeCommit
// Pass in an OSTreeCommit string and get a *models.Commit back
func GetCommitByOSTreeCommit(ost string) (*models.Commit, error) {
	log.Debugf("GetCommitByOSTreeCommit::ost: %#v", ost)
	var commit models.Commit
	result := db.DB.Where("os_tree_commit = ?", ost).First(&commit)
	log.Debugf("GetCommitByOSTreeCommit::result: %#v", result)
	log.Debugf("GetCommitByOSTreeCommit::commit: %#v", commit)
	if result.Error != nil {
		return nil, result.Error
	}
	return &commit, nil
}

// GetRepoByID
// Pass in RepoID uint and get a *models.Repo back
func GetRepoByID(repoID uint) (*models.Repo, error) {
	log.Debugf("GetRepoByID::repoID: %#v", repoID)
	var repo models.Repo
	result := db.DB.First(&repo, repoID)
	log.Debugf("GetRepoByID::result: %#v", result)
	log.Debugf("GetRepoByID::repo: %#v", repo)
	if result.Error != nil {
		return nil, result.Error
	}
	return &repo, nil
}

// GetRepoByCommitID
// Pass in RepoID uint and get a *models.Repo back
func GetRepoByCommitID(commitID uint) (*models.Repo, error) {
	log.Debugf("GetRepoByCommitID::commitID: %#v", commitID)
	var repo models.Repo
	result := db.DB.Where("commit_id = ?", commitID).First(&repo)
	log.Debugf("GetRepoByCommitID::result: %#v", result)
	log.Debugf("GetRepoByCommitID::repo: %#v", repo)
	if result.Error != nil {
		return nil, result.Error
	}
	return &repo, nil
}

// GetDeviceByID
// Pass in DeviceID uint and get a *models.Device back
func GetDeviceByID(deviceID uint) (*models.Device, error) {
	log.Debugf("GetDeviceByID::deviceID: %#v", deviceID)
	var device models.Device
	result := db.DB.First(&device, deviceID)
	log.Debugf("GetDeviceByID::result: %#v", result)
	log.Debugf("GetDeviceByID::device: %#v", device)
	if result.Error != nil {
		return nil, result.Error
	}
	return &device, nil
}

// GetDeviceByUUID
// Pass in UUID string and get a *models.Device back
func GetDeviceByUUID(deviceUUID string) (*models.Device, error) {
	log.Debugf("GetDeviceByUUID::deviceUUID: %#v", deviceUUID)
	var device models.Device
	result := db.DB.Where("uuid = ?", deviceUUID).First(&device)
	log.Debugf("GetDeviceByUUID::result: %#v", result)
	log.Debugf("GetDeviceByUUID::device: %#v", device)
	if result.Error != nil {
		return nil, result.Error
	}
	return &device, nil
}
