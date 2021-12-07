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
func NewThirdPartyRepoService(ctx context.Context) ThirdPartyRepoServiceInterface {
	return &ThirdPartyRepoService{
		ctx: ctx,
	}
}

// ThirdPartyRepoService is the main implementation of a ThirdPartyRepoServiceInterface
type ThirdPartyRepoService struct {
	ctx context.Context
}

// CreateThirdPartyRepo creates the ThirdPartyRepo for an Account on our database
func (s *ThirdPartyRepoService) CreateThirdPartyRepo(thirdPartyRepo *models.ThirdPartyRepo, account string) (*models.ThirdPartyRepo, error) {
	if thirdPartyRepo.URL != "" && thirdPartyRepo.Name != "" {
		thirdPartyRepo = &models.ThirdPartyRepo{
			Name:        thirdPartyRepo.Name,
			URL:         thirdPartyRepo.URL,
			Description: thirdPartyRepo.Description,
			Account:     account,
		}
		result := db.DB.Create(&thirdPartyRepo)
		if result.Error != nil {
			return nil, result.Error
		}
		log.Infof("Getting ThirdPartyRepo info: repo %s, %s", thirdPartyRepo.URL, thirdPartyRepo.Name)

	}
	return thirdPartyRepo, nil
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
		log.Info(err)
	}
	if tprepo.Name != "" {
		repoDetails.Name = tprepo.Name
	}

	if tprepo.URL != "" {
		repoDetails.URL = tprepo.URL
	}

	if tprepo.Description != "" {
		repoDetails.Description = tprepo.Description
	}
	result := db.DB.Save(&repoDetails)
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
		log.Info(err)
	}
	if repoDetails.Name == "" {
		return nil, errors.NewInternalServerError()
	}

	delForm := db.DB.Where("account = ? and id = ?", account, ID).Delete(&tprepo)
	if delForm.Error != nil {
		err := errors.NewInternalServerError()
		return nil, err
	}
	return &tprepo, nil
}
