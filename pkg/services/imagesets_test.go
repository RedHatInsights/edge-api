package services

import (
	"context"
	"testing"
)

func TestGetImageSetByID(t *testing.T) {
	imageSetService := ImageSetsService{
		ctx: context.Background(),
	}
	_, err := imageSetService.GetImageSetsByID(1)
	if err != nil {
		t.Errorf("Expected nil image set available, got %#v", err)
	}

}
