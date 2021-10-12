package services

import (
	"context"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
)

// TPRepoServiceInterface defines
type TPRepoServiceInterface interface {
	CreateThirdyPartyRepo(tprepo *models.ThirdyPartyRepo, account string) error
}

// NewTPRepoService gives a instance of the main implementation of a TPRepoServiceInterface
func NewTPRepoService(ctx context.Context) TPRepoServiceInterface {
	return &TPRepoService{}
}

// TPRepoService is the main implementation of a TPRepoServiceInterface
type TPRepoService struct {
	ctx context.Context
}

// CreateThirdyPartyRepo creates the ThirdyPartyRepo for an Account on our database
func (s *TPRepoService) CreateThirdyPartyRepo(tprepo *models.ThirdyPartyRepo, account string) error {
	var image models.Image
	image.Commit.Account = account
	tx := db.DB.Create(&tprepo)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}
