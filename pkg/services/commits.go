package services

import (
	"context"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// CommitServiceInterface defines the interface to handle the business logic of RHEL for Edge Commits
type CommitServiceInterface interface {
	GetCommitByID(commitID uint) (*models.Commit, error)
	GetCommitByOSTreeCommit(ost string) (*models.Commit, error)
}

// NewCommitService gives a instance of the main implementation of CommitServiceInterface
func NewCommitService(ctx context.Context, log *log.Entry) CommitServiceInterface {
	return &CommitService{
		Service: Service{ctx: ctx, log: log.WithField("service", "commit")},
	}
}

// CommitService is the main implementation of a CommitServiceInterface
type CommitService struct {
	Service
}

// GetCommitByID receives CommitID uint and get a *models.Commit back
func (s *CommitService) GetCommitByID(commitID uint) (*models.Commit, error) {
	s.log = s.log.WithField("commitID", commitID)
	s.log.Debug("Getting commit by id")
	var commit models.Commit
	result := db.DB.First(&commit, commitID)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error searching for commit by commitID")
		return nil, result.Error
	}
	s.log.Debug("Commit retrieved")
	return &commit, nil
}

// GetCommitByOSTreeCommit receives an OSTreeCommit string and get a *models.Commit back
func (s *CommitService) GetCommitByOSTreeCommit(ost string) (*models.Commit, error) {
	s.log = s.log.WithField("ostreeCommitHash", ost)
	var commit models.Commit
	result := db.DB.Where("os_tree_commit = ?", ost).First(&commit)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error searching for commit by ostreeCommitHash")
		return nil, result.Error
	}
	s.log.Debug("Commit retrieved")
	return &commit, nil
}
