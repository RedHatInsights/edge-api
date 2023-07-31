package models

// CheckThirdPartyRepoNameDataAPI is the third party repository check name data
type CheckThirdPartyRepoNameDataAPI struct {
	IsValid bool `json:"isValid" example:"false"` // The indicator of third party repository name validity
} // @name CheckThirdPartyRepoNameData

// CheckThirdPartyRepoNameAPI is the third party repository check name result data
type CheckThirdPartyRepoNameAPI struct {
	Data CheckThirdPartyRepoNameDataAPI `json:"data"` // The data of third party repository check name result
} // @name CheckThirdPartyRepoName

// ThirdPartyRepoAPI is the third party repository entity main data
type ThirdPartyRepoAPI struct {
	ID          uint   `json:"ID,omitempty" example:"1028"`                               // The unique ID of the third party repository
	Name        string `json:"Name" example:"my_custom_repo"`                             // The name of the third party repository
	URL         string `json:"URL" example:"https://public.example.com/my_custom_repo"`   // The URL of the third party repository
	Description string `json:"Description,omitempty" example:"a repo for some utilities"` // The description of the third party repository
} // @name ThirdPartyRepo

// ThirdPartyRepoListAPI is the third party repositories list result data
type ThirdPartyRepoListAPI struct {
	Count int                 `json:"count" example:"25"` // The overall count of the stored third party repositories
	Data  []ThirdPartyRepoAPI `json:"data"`               // The data list of the third party repositories
} // @name ThirdPartyRepoList
