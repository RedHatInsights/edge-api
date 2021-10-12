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
}

const (
	// RepoNameCantBeInvalidMessage is the error message when the name is invalid
	RepoNameCantBeInvalidMessage = "name must start with alphanumeric characters and can contain underscore and hyphen characters"
)

var (
	validRepoName = regexp.MustCompile(`^[A-Za-z0-9]+[A-Za-z0-9\s_-]*$`)
)

// ValidateRequest validates the Repo Request
func (t *ThirdyPartyRepo) ValidateRequest() error {
	if !validRepoName.MatchString(t.Name) {
		return errors.New(RepoNameCantBeInvalidMessage)
	}
	return nil
}
