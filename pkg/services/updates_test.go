package services

import (
	"context"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
)

func TestGetUpdateTransactionsForDevice(t *testing.T) {

	uuid := faker.UUIDHyphenated()
	updateService := UpdateService{
		ctx: context.Background(),
	}

	device := models.Device{
		UUID: uuid,
	}
	db.DB.Create(&device)
	updates := []models.UpdateTransaction{
		{
			Devices: []models.Device{
				device,
			},
		},
		{
			Devices: []models.Device{
				device,
			},
		},
	}
	db.DB.Create(&updates[0])
	db.DB.Create(&updates[1])
	actual, err := updateService.GetUpdateTransactionsForDevice(&device)
	if actual == nil {
		t.Errorf("Expected not nil updates")
	} else if len(*actual) != 2 {
		t.Errorf("Expected two update transactions, got %d", len(*actual))
	}
	if err != nil {
		t.Errorf("Error not expected, got %s", err.Error())
	}

}
