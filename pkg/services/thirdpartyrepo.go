package services

import (
	"context"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
)

// ThirdPartyRepoServiceInterface defines the interface that helps handles
// the business logic of creating Third Party Repository
type ThirdPartyRepoServiceInterface interface {
	CreateThirdPartyRepo(tprepo *models.ThirdPartyRepo, account string) (*models.ThirdPartyRepo, error)
	GetThirdPartyRepoByID(ID string) (*models.ThirdPartyRepo, error)
	UpdateThirdPartyRepo(tprepo *models.ThirdPartyRepo, account string, ID string) error
	DeleteThirdPartyRepoByID(ID string) (*models.ThirdPartyRepo, error)
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

// thirdPartyRepoNameExists check if a repo with the requested name exists
func (s *ThirdPartyRepoService) thirdPartyRepoNameExists(account string, name string) (bool, error) {
	var reposCount int64
	if result := db.DB.Model(&models.ThirdPartyRepo{}).Where("account = ? AND name = ?", account, name).Count(&reposCount); result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error checking third party repository existence")
		return false, result.Error
	}

	return reposCount > 0, nil
}

// CreateThirdPartyRepo creates the ThirdPartyRepo for an Account on our database
func (s *ThirdPartyRepoService) CreateThirdPartyRepo(thirdPartyRepo *models.ThirdPartyRepo, account string) (*models.ThirdPartyRepo, error) {
	if account == "" {
		return nil, new(AccountNotSet)
	}
	if thirdPartyRepo.Name == "" {
		return nil, new(ThirdPartyRepositoryNameIsEmpty)
	}
	if thirdPartyRepo.URL == "" {
		return nil, new(ThirdPartyRepositoryURLIsEmpty)
	}
	repoExists, err := s.thirdPartyRepoNameExists(account, thirdPartyRepo.Name)
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
		Account:     account,
	}
	if result := db.DB.Create(&createdThirdPartyRepo); result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error creating third party repository")
		return nil, result.Error
	}

	return createdThirdPartyRepo, nil
}

// GetThirdPartyRepoByID gets the Third Party Repository by ID from the database
func (s *ThirdPartyRepoService) GetThirdPartyRepoByID(ID string) (*models.ThirdPartyRepo, error) {
	var tprepo models.ThirdPartyRepo
	account, err := common.GetAccountFromContext(s.ctx)
	if err != nil {
		return nil, new(AccountNotSet)
	}
	result := db.DB.Where("account = ? and id = ?", account, ID).First(&tprepo)
	if result.Error != nil {
		return nil, new(ThirdPartyRepositoryNotFound)
	}
	return &tprepo, nil
}

// UpdateThirdPartyRepo updates the existing third party repository
func (s *ThirdPartyRepoService) UpdateThirdPartyRepo(tprepo *models.ThirdPartyRepo, account string, ID string) error {

	tprepo.Account = account
	repoDetails, err := s.GetThirdPartyRepoByID(ID)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error retrieving third party repository")
	}
	if tprepo.Name != "" {
		if tprepo.Name != repoDetails.Name {
			// check if a repository with the new name already exists
			repoExists, err := s.thirdPartyRepoNameExists(account, tprepo.Name)
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
	var tprepo models.ThirdPartyRepo
	account, err := common.GetAccountFromContext(s.ctx)
	result := db.DB.Where("id = ?", ID).First(&tprepo)
	if result.Error != nil {
		return nil, new(ThirdPartyRepositoryNotFound)
	}
	if err != nil {
		return nil, new(AccountNotSet)
	}
	repoDetails, err := s.GetThirdPartyRepoByID(ID)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error retieving third party repository")
	}
	if repoDetails.Name == "" {
		return nil, errors.NewInternalServerError()
	}

	delForm := db.DB.Where("account = ? and id = ?", account, ID).Delete(&tprepo)
	if delForm.Error != nil {
		s.log.WithField("error", delForm.Error.Error()).Error("Error deleting third party repository")
		err := errors.NewInternalServerError()
		return nil, err
	}
	return &tprepo, nil
}
