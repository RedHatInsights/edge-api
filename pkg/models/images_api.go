package models

// CommitAPI is  a struct ...
type CommitAPI struct {
	Arch string `json:"arch" example:"x86_64"` // The commit architecture
} // @name Commit

// CustomPackagesAPI is a struct for auto-generation of openapi.json
type CustomPackagesAPI struct {
	Name string `json:"name" example:"cat"`
} // @name CustomPackages

// InstallerAPI ...
type InstallerAPI struct {
	SSHKey   string `json:"sshkey" example:"ssh-rsa lksjdkjkjsdlkjjds"`
	Username string `json:"username" example:"myuser"`
} // @name Installer

// PackagesAPI ...
type PackagesAPI struct {
	Name string `json:"name" example:"php"`
} // @name Packages

// ThirdPartyReposAPI ...
type ThirdPartyReposAPI struct {
	ID   int    `json:"ID" example:"1234"`                              // The unique ID of the repository
	Name string `json:"Name" example:"my_custom_repo"`                  // The name of the repository
	URL  string `json:"URL" example:"https://example.com/repos/myrepo"` // The base URL of the repository
} // @name ThirdPartyRepos

// CreateImageAPI is the /images POST endpoint struct for openapi.json auto-gen
type CreateImageAPI struct {
	Commit         CommitAPI           `json:"commit"`
	CustomPackages []CustomPackagesAPI `json:"customPackages"`                                 // An optional list of custom repositories
	Description    string              `json:"description" example:"This is an example image"` // A short description of the image
	Distribution   string              `json:"distribution" example:"rhel-92"`                 // The RHEL for Edge OS version
	// Available image types:
	// * rhel-edge-installer - Installer ISO
	// * rhel-edge-commit - Commit only
	ImageType              string               `json:"imageType" example:"rhel-edge-installer"` // The image builder assigned image type
	Installer              InstallerAPI         `json:"installer"`
	Name                   string               `json:"name"  example:"my-edge-image"`
	Packages               []PackagesAPI        `json:"packages"`
	OutputTypes            []string             `json:"outputTypes" example:"rhel-edge-installer,rhel-edge-commit"`
	ThirdPartyRepositories []ThirdPartyReposAPI `json:"thirdPartyRepositories"`
	Version                int                  `json:"version" example:"0"`
} // @name CreateImage

// ImageResponseAPI is a struct for auto-generation of openapi.json
type ImageResponseAPI struct {
	// Currently returns all of the available image data
	Image
} // @name ImageResponse

// SuccessPlaceholderResponse is a placeholder
type SuccessPlaceholderResponse struct {
}

// ImageDetailAPI return the structure to inform package info to images
type ImageDetailAPI struct {
	Image              *Image `json:"image"`
	AdditionalPackages int    `json:"additional_packages" example:"3"`
	Packages           int    `json:"packages" example:"3"`
	UpdateAdded        int    `json:"update_added" example:"3"`
	UpdateRemoved      int    `json:"update_removed" example:"2"`
	UpdateUpdated      int    `json:"update_updated" example:"3"`
}

// ImagesViewDataAPI is the images view data return for images view with filters , limit, offSet
type ImagesViewDataAPI struct {
	Count int64       `json:"count" example:"100"`
	Data  []ImageView `json:"data"`
}
