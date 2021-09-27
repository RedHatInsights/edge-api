package services

import (
	"context"
	"fmt"
	"testing"
)

func TestGetImageSetByID(t *testing.T) {
	imageSetService := ImageSetsService{
		ctx: context.Background(),
	}
	imageSet, err := imageSetService.GetImageSetsByID(1)
	if err != nil {
		t.Errorf("Expected nil image set available, got %#v", err)
	}
	fmt.Print(imageSet)

}
