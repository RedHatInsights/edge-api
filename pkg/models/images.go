package models

import "errors"

// An Image is what generates a OSTree Commit.
//
// swagger:model image
type Image struct {
	Distribution string // rhel-8
	Architecture string // x86_64
	OSTreeRef    string // "rhel/8/x86_64/edge"
	OSTreeURL    string
	Description  string
	OutputType   string
	Packages     []string
	Status       string
	ComposeJobID string
}

func (i *Image) ValidateRequest() error {
	if i.Distribution == "" {
		return errors.New("distribution can't be empty")
	}
	if i.Architecture == "" {
		return errors.New("architecture can't be empty")
	}
	if i.OutputType != "tar" {
		return errors.New("only tar architecture supported for now")
	}
	return nil
}
