// FIXME: golangci-lint
// nolint:gocritic,govet,revive
package services

import (
	"context"
	"strconv"

	"gorm.io/gorm"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
)

// ThirdPartyRepoServiceInterface defines the interface that helps handles
// the business logic of creating Third Party Repository
type ThirdPartyRepoServiceInterface interface {
	CreateThirdPartyRepo(tprepo *models.ThirdPartyRepo, orgID string) (*models.ThirdPartyRepo, error)
	GetThirdPartyRepoByID(ID string) (*models.ThirdPartyRepo, error)
	UpdateThirdPartyRepo(tprepo *models.ThirdPartyRepo, orgID string, ID string) error
	DeleteThirdPartyRepoByID(ID string) (*models.ThirdPartyRepo, error)
	ThirdPartyRepoNameExists(orgID string, name string) (bool, error)
}

// NewThirdPartyRepoService gives a instance of the main implementation of a ThirdPartyRepoServiceInterface
func NewThirdPartyRepoService(ctx context.Context, log *log.Entry) ThirdPartyRepoServiceInterface {
	return &ThirdPartyRepoService{
		Service: Service{ctx: ctx, log: log.WithField("service", "image")},
	}
}

// ThirdPartyRepoService is the main implementation of a ThirdPartyRepoServiceInterface
type ThirdPartyRepoService struct {
	Service
}

// ThirdPartyRepoNameExists check if a repo with the requested name exists
func (s *ThirdPartyRepoService) ThirdPartyRepoNameExists(orgID string, name string) (bool, error) {
	if orgID == "" {
		return false, new(OrgIDNotSet)
	}
	if name == "" {
		return false, new(ThirdPartyRepositoryNameIsEmpty)
	}

	var reposCount int64
	if result := db.Org(orgID, "").Debug().Model(&models.ThirdPartyRepo{}).Where("name = ?", name).Count(&reposCount); result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error checking custom repository existence")
		return false, result.Error
	}

	return reposCount > 0, nil
}

// thirdPartyRepoImagesExists check if a repo with the requested name exists
func (s *ThirdPartyRepoService) thirdPartyRepoImagesExists(id string, imageStatuses []string) (bool, error) {
	repo, err := s.GetThirdPartyRepoByID(id)
	if err != nil {
		return false, err
	}
	var imagesCount int64
	tx := db.DB.Model(&models.Image{}).
		Joins("JOIN images_repos ON images_repos.image_id = images.id").
		Where("images_repos.third_party_repo_id = ?", repo.ID)
	if len(imageStatuses) > 0 {
		tx = tx.Where("images.status IN (?)", imageStatuses)
	}
	if result := tx.Debug().Count(&imagesCount); result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error checking custom repository existence")
		return false, result.Error
	}

	return imagesCount > 0, nil
}

// CreateThirdPartyRepo creates the ThirdPartyRepo for an Org on our database
func (s *ThirdPartyRepoService) CreateThirdPartyRepo(thirdPartyRepo *models.ThirdPartyRepo, orgID string) (*models.ThirdPartyRepo, error) {
	if orgID == "" {
		return nil, new(OrgIDNotSet)
	}
	if thirdPartyRepo.Name == "" {
		return nil, new(ThirdPartyRepositoryNameIsEmpty)
	}
	if thirdPartyRepo.URL == "" {
		return nil, new(ThirdPartyRepositoryURLIsEmpty)
	}
	if !models.ValidateRepoURL(thirdPartyRepo.URL) {
		return nil, new(InvalidURLForCustomRepo)
	}

	repoExists, err := s.ThirdPartyRepoNameExists(orgID, thirdPartyRepo.Name)
	if err != nil {
		return nil, err
	}
	if repoExists {
		return nil, new(ThirdPartyRepositoryAlreadyExists)
	}
	createdThirdPartyRepo := &models.ThirdPartyRepo{
		Name:        thirdPartyRepo.Name,
		URL:         thirdPartyRepo.URL,
		Description: thirdPartyRepo.Description,
		OrgID:       orgID,
	}
	if result := db.DB.Create(&createdThirdPartyRepo); result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error creating custom repository")
		return nil, result.Error
	}

	return createdThirdPartyRepo, nil
}

// GetThirdPartyRepoByID gets the Third Party Repository by ID from the database
func (s *ThirdPartyRepoService) GetThirdPartyRepoByID(ID string) (*models.ThirdPartyRepo, error) {
	var tprepo models.ThirdPartyRepo
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error getting orgID from context")
		return nil, err
	}
	if result := db.Org(orgID, "").Where("id = ?", ID).First(&tprepo); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, new(ThirdPartyRepositoryNotFound)
		}
		return nil, result.Error
	}
	return &tprepo, nil
}

// UpdateThirdPartyRepo updates the existing third party repository
func (s *ThirdPartyRepoService) UpdateThirdPartyRepo(tprepo *models.ThirdPartyRepo, orgID string, ID string) error {

	tprepo.OrgID = orgID
	repoDetails, err := s.GetThirdPartyRepoByID(ID)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error retrieving custom repository")
		return err
	}
	if tprepo.Name != "" {
		if tprepo.Name != repoDetails.Name {
			// check if a repository with the new name already exists
			repoExists, err := s.ThirdPartyRepoNameExists(orgID, tprepo.Name)
			if err != nil {
				return err
			}
			if repoExists {
				return new(ThirdPartyRepositoryAlreadyExists)
			}
		}
		repoDetails.Name = tprepo.Name
	}
	if tprepo.URL != "" {
		if repoDetails.URL != tprepo.URL {
			// prohibit url change if images exists with successful status
			imagesExists, err := s.thirdPartyRepoImagesExists(
				strconv.FormatUint(uint64(repoDetails.ID), 10),
				[]string{models.ImageStatusSuccess, models.ImageStatusBuilding, models.ImageStatusInterrupted},
			)
			if err != nil {
				return err
			}
			if imagesExists {
				return new(ThirdPartyRepositoryImagesExists)
			}
		}
		repoDetails.URL = tprepo.URL
	}

	if tprepo.Description != "" {
		repoDetails.Description = tprepo.Description
	}
	result := db.DB.Save(repoDetails)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// DeleteThirdPartyRepoByID deletes the third party repository using ID
func (s *ThirdPartyRepoService) DeleteThirdPartyRepoByID(ID string) (*models.ThirdPartyRepo, error) {
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error getting orgID from context")
		return nil, err
	}
	repoDetails, err := s.GetThirdPartyRepoByID(ID)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error retrieving custom repository")
		return nil, err
	}
	// fail to delete if any image exists (with any status)
	imagesExists, err := s.thirdPartyRepoImagesExists(strconv.FormatUint(uint64(repoDetails.ID), 10), []string{})
	if err != nil {
		return nil, err
	}
	if imagesExists {
		return nil, new(ThirdPartyRepositoryImagesExists)
	}
	if result := db.Org(orgID, "").Delete(&repoDetails); result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error deleting custom repository")
		return nil, result.Error
	}
	return repoDetails, nil
}
