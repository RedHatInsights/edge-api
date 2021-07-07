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
func TestValidateRequestWithEdgeInstallerOutputType(t *testing.T) {
	img := &Image{
		Distribution: "rhel-8",
		Commit:       &Commit{Arch: "x86_64"},
	}

	err := img.ValidateRequest()
	if err != nil {
		t.Errorf("Error not expected")
	}
}
func TestValidateRequestWithEdgeCommitImageType(t *testing.T) {
	img := &Image{
		Distribution: "rhel-8",
		Commit:       &Commit{Arch: "x86_64"},
	}

	err := img.ValidateRequest()
	if err != nil {
		t.Errorf("Error not expected")
	}
}

func TestValidateRequest(t *testing.T) {
	img := &Image{
		Distribution: "rhel-8",
		Commit:       &Commit{Arch: "x86_64"},
	}

	err := img.ValidateRequest()
	if err != nil {
		t.Errorf("Image object should be valid")
	}
}
