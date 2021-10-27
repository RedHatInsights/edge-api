package services

import (
	"context"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// ThirdPartyRepoServiceInterface defines the interface that helps handles
// the business logic of creating Third Party Repository
type ThirdPartyRepoServiceInterface interface {
	CreateThirdPartyRepo(tprepo *models.ThirdPartyRepo, account string) error
	GetThirdPartyRepoByID(tprepoId int) (*models.ThirdPartyRepo, error)
}

// NewThirdPartyRepoService gives a instance of the main implementation of a ThirdPartyRepoServiceInterface
func NewThirdPartyRepoService(ctx context.Context) ThirdPartyRepoServiceInterface {
	return &ThirdPartyRepoService{}
}

// ThirdPartyRepoService is the main implementation of a ThirdPartyRepoServiceInterface
type ThirdPartyRepoService struct {
	ctx context.Context
}

// CreateThirdPartyRepo creates the ThirdPartyRepo for an Account on our database
func (s *ThirdPartyRepoService) CreateThirdPartyRepo(tprepo *models.ThirdPartyRepo, account string) error {
	if tprepo.URL != "" && tprepo.Name != "" {
		tprepo = &models.ThirdPartyRepo{
			Name:        tprepo.Name,
			URL:         tprepo.URL,
			Description: tprepo.Description,
			Account:     account,
		}
		result := db.DB.Create(&tprepo)
		if result.Error != nil {
			return result.Error
		}
		log.Infof("Getting ThirdPartyRepo info: repo %s, %s", tprepo.URL, tprepo.Name)

	}
	return nil
}

// GetThirdPartyRepoByID gets the Third Party Repository by ID from the database
func (s *ThirdPartyRepoService) GetThirdPartyRepoByID(tprepoId int) (*models.ThirdPartyRepo, error) {
	var tprepo models.ThirdPartyRepo
	result := db.DB.Where("id = ?", tprepoId).Find(&tprepo)
	if result.Error != nil {
		err := errors.NewInternalServerError()
		return nil, err
	}
	return &tprepo, nil
}
