package models

import (
	"errors"
	"regexp"
)

type ThirdyPartyRepo struct {
	Model
	Name        string `json:"Name"`
	URL         string `json:"URL"`
	Description string `json:"Description,omitempty"`
	Account     string `json:"Account"`
}

const (
	// RepoNameCantBeInvalidMessage is the error message when the name is invalid
	RepoNameCantBeInvalidMessage = "name must start with alphanumeric characters and can contain underscore and hyphen characters"
	RepoURLCantBeNilMessage      = "repository URL can't be empty "
	RepoNameCantBeNilMessage     = "repository name can't be empty "
)

var (
	validRepoName = regexp.MustCompile(`^[A-Za-z0-9]+[A-Za-z0-9\s_-]*$`)
)

// ValidateRequest validates the Repository Request
func (t *ThirdyPartyRepo) ValidateRequest() error {
	if t.Name == "" {
		return errors.New(RepoNameCantBeNilMessage)
	}
	return nil
}
