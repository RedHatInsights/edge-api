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
	ValidateUserProvidedCommitID(deviceUUID []string, commitID uint) error
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

// ValidateUserProvidedCommitID validates if user provided commitID belong to same ImageSet as of Device Image
func (s *CommitService) ValidateUserProvidedCommitID(deviceUUID []string, commitID uint) error {

	var device models.Device
	var imageForCommitID models.Image
	var imageID models.Image

	resultDevice := db.DB.Where("uuid = ? ", deviceUUID).Find(&device)
	if resultDevice.Error != nil {
		s.log.WithField("error", resultDevice.Error.Error()).Error("Error searching for devices using DeviceUUID")
		return resultDevice.Error
	}

	resultImageSet := db.DB.Where("id = ? ", device.ImageID).Find(&imageID)
	if resultImageSet.Error != nil {
		s.log.WithField("error", resultImageSet.Error.Error()).Error("Error searching for Image using DeviceImageID")
		return resultImageSet.Error
	}

	resultCommit := db.DB.Where("commit_id = ?", commitID).Find(&imageForCommitID)
	if resultCommit.Error != nil {
		s.log.WithField("error", resultCommit.Error.Error()).Error("Error searching for Images using user provided CommitID")
		return resultCommit.Error
	}

	if *imageForCommitID.ImageSetID != *imageID.ImageSetID {
		return new(InvalidCommitID)
	}
	return nil
}
