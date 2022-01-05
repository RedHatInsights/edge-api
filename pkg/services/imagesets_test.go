package services

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestGetImageSetByID(t *testing.T) {
	imageSetService := ImageSetsService{
		Service{ctx: context.Background(), log: log.NewEntry(log.StandardLogger())},
	}
	_, err := imageSetService.GetImageSetsByID(1)
	if err != nil {
		t.Errorf("Expected nil image set available, got %#v", err)
	}

}
