package services

import (
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// RepoServiceInterface defines the interface to handle the business logic of RHEL for Edge Devices
type RepoServiceInterface interface {
	GetRepoByID(repoID uint) (*models.Repo, error)
	GetRepoByCommitID(commitID uint) (*models.Repo, error)
}

// NewRepoService gives a instance of the main implementation of RepoServiceInterface
func NewRepoService() RepoServiceInterface {
	return &RepoService{}
}

// RepoService is the main implementation of a RepoServiceInterface
type RepoService struct{}

// GetRepoByID receives RepoID uint and get a *models.Repo back
func (s *RepoService) GetRepoByID(repoID uint) (*models.Repo, error) {
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

// GetRepoByCommitID receives Repo.CommitID uint and get a *models.Repo back
func (s *RepoService) GetRepoByCommitID(commitID uint) (*models.Repo, error) {
	log.Debugf("GetRepoByCommitID::commitID: %#v", commitID)

	var commit models.Commit
	result := db.DB.First(&commit, commitID)
	if result.Error != nil {
		return nil, result.Error
	}
	return &commit.Repo, nil
}
