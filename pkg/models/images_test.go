package models

import (
	"testing"
)

func TestValidateRequestWithEmptyDistribution(t *testing.T) {
	img := &Image{}

	err := img.ValidateRequest()

	if err == nil {
		t.Errorf("Error expected")
	}
	if err.Error() != DistributionCantBeNilMessage {
		t.Errorf("expected distribution can't be empty")
	}
}

func TestValidateRequestWithEmptyArchitecture(t *testing.T) {
	img := &Image{
		Distribution: "rhel-8",
	}

	err := img.ValidateRequest()
	if err == nil {
		t.Errorf("Error expected")
	}
	if err.Error() != ArchitectureCantBeEmptyMessage {
		t.Errorf("expected architecture can't be empty")
	}
}
func TestValidateRequestWithIsoOutputType(t *testing.T) {
	img := &Image{
		Distribution: "rhel-8",
		Commit:       &Commit{Arch: "x86_64"},
		OutputType:   "iso",
	}

	err := img.ValidateRequest()
	if err == nil {
		t.Errorf("Error expected")
	}
	if err.Error() != OnlyTarAcceptedMessage {
		t.Errorf("expected only tar accepted error")
	}
}

func TestValidateRequest(t *testing.T) {
	img := &Image{
		Distribution: "rhel-8",
		Commit:       &Commit{Arch: "x86_64"},
		OutputType:   "tar",
	}

	err := img.ValidateRequest()
	if err != nil {
		t.Errorf("Image object should be valid")
	}
}
