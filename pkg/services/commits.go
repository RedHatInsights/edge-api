package services

import (
	"context"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
)

// CommitServiceInterface defines the interface to handle the business logic of RHEL for Edge Commits
type CommitServiceInterface interface {
	GetCommitByID(commitID uint) (*models.Commit, error)
	GetCommitByOSTreeCommit(ost string) (*models.Commit, error)
	ValidateDevicesImageSetWithCommit(deviceUUID []string, commitID uint) error
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

// ValidateDevicesImageSetWithCommit validates if user provided commitID belong to same ImageSet as of Device Image
func (s *CommitService) ValidateDevicesImageSetWithCommit(deviceUUID []string, commitID uint) error {

	var imageForCommitID []models.Image
	var devicesImageSet []models.Image

	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		return err
	}

	resultImageSet := db.Org(orgID, "devices").Table("devices").
		Select(`images.image_set_id as "image_set_id"`).
		Joins("JOIN images ON devices.image_id = images.id").
		Where("devices.uuid in (?) AND devices.org_id = ?", deviceUUID, orgID).
		Group("images.image_set_id").
		Find(&devicesImageSet)
	if resultImageSet.Error != nil {
		s.log.WithField("error", resultImageSet.Error.Error()).Error("Error searching for ImageSet of Device Images")
		return resultImageSet.Error
	}

	// finding unique ImageSetID for device Image
	devicesImageSetID := make(map[uint]bool, len(devicesImageSet))
	var imageSetID uint
	for _, image := range devicesImageSet {
		if image.ImageSetID == nil {
			return new(ImageHasNoImageSet)
		}
		imageSetID = *image.ImageSetID
		devicesImageSetID[*image.ImageSetID] = true
	}

	if len(devicesImageSetID) > 1 {
		return new(DeviceHasMoreThanOneImageSet)
	}
	resultCommit := db.Org(orgID, "").Where("commit_id = ?", commitID).Find(&imageForCommitID)
	if resultCommit.Error != nil {
		s.log.WithField("error", resultCommit.Error.Error()).Error("Error searching for Images using user provided CommitID")
		return resultCommit.Error
	}

	if *imageForCommitID[0].ImageSetID != imageSetID {
		return new(InvalidCommitID)
	}
	return nil
}
