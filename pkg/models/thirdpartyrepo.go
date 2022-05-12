package models

import (
	"errors"
	"regexp"
)

/*
ThirdPartyRepo is a record of Third Party Repository or we can call it as Custom Repository provided by customers per account.

	Here, URL refers to the url of the third party repository, Account refers to the account attached to the third party
	repository.

*/
type ThirdPartyRepo struct {
	Model
	Name        string `json:"Name"`
	URL         string `json:"URL"`
	Description string `json:"Description,omitempty"`
	Account     string
	OrgID       string `json:"org_id"`
}

const (
	// RepoNameCantBeInvalidMessage is the error message when the name is invalid
	RepoNameCantBeInvalidMessage = "name must start with alphanumeric characters and can contain underscore and hyphen characters"
	// RepoURLCantBeNilMessage is the error message when Repository url is nil
	RepoURLCantBeNilMessage = "repository URL can't be empty"
	// RepoNameCantBeNilMessage is the error when Repository name is nil
	RepoNameCantBeNilMessage = "repository name can't be empty"
)

var (
	validRepoName = regexp.MustCompile(`^[A-Za-z0-9]+[A-Za-z0-9\s_-]*$`)
)

// ValidateRequest validates the Repository Request
func (t *ThirdPartyRepo) ValidateRequest() error {
	if t.Name == "" {
		return errors.New(RepoNameCantBeNilMessage)
	}
	if t.URL == "" {
		return errors.New(RepoURLCantBeNilMessage)
	}
	if !validRepoName.MatchString(t.Name) {
		return errors.New(RepoNameCantBeInvalidMessage)
	}
	return nil
}
