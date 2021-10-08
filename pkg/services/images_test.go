package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/bxcodec/faker/v3"
)

func TestGetImageByWhenImageIsNotFound(t *testing.T) {
	s := ImageService{
		ctx: context.Background(),
	}
	id, _ := faker.RandomInt(10)
	image, err := s.GetImageByID(fmt.Sprint(id[0]))

	switch err.(type) {
	case *ImageNotFoundError:
		fmt.Println("All good")
	default:
		t.Errorf("Expected error, got %#v", image)
	}
}
func TestGetImageByOstreeCommitHashWhenImageIsNotFound(t *testing.T) {
	s := ImageService{
		ctx: context.Background(),
	}
	hash := faker.Word()
	image, err := s.GetImageByOSTreeCommitHash(hash)

	switch err.(type) {
	case *ImageNotFoundError:
		fmt.Println("All good")
	default:
		t.Errorf("Expected error, got %#v", image)
	}
}
