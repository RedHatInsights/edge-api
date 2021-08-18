package services

import (
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

type CommitServiceInterface interface {
	GetCommitByID(commitID uint) (*models.Commit, error)
	GetCommitByOSTreeCommit(ost string) (*models.Commit, error)
}

func NewCommitService() CommitServiceInterface {
	return &CommitService{}
}

type CommitService struct{}

// GetCommitByID receives CommitID uint and get a *models.Commit back
func (cs *CommitService) GetCommitByID(commitID uint) (*models.Commit, error) {
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

// GetCommitByOSTreeCommit receives an OSTreeCommit string and get a *models.Commit back
func (cs *CommitService) GetCommitByOSTreeCommit(ost string) (*models.Commit, error) {
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
