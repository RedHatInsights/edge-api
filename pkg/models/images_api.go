package models

// CommitAPI is  a struct ...
type CommitAPI struct {
	Arch string `json:"arch" example:"x86_64"` // The commit architecture
} // @name Commit

// CustomPackagesAPI is a struct for auto-generation of openapi.json
type CustomPackagesAPI struct {
	Name string `json:"name" example:"cat"` // Name of custom packages
} // @name CustomPackages

// InstallerAPI ...
type InstallerAPI struct {
	SSHKey   string `json:"sshkey" example:"ssh-rsa lksjdkjkjsdlkjjds"` // SSH key of installer
	Username string `json:"username" example:"myuser"`                  // Username of administrative user
} // @name Installer

// PackagesAPI ...
type PackagesAPI struct {
	Name string `json:"name" example:"php"` // Name of packages
} // @name Packages

// ThirdPartyReposAPI ...
type ThirdPartyReposAPI struct {
	ID   int    `json:"ID" example:"1234"`                              // The unique ID of the repository
	Name string `json:"Name" example:"my_custom_repo"`                  // The name of the repository
	URL  string `json:"URL" example:"https://example.com/repos/myrepo"` // The base URL of the repository
} // @name ThirdPartyRepos

// CreateImageAPI is the /images POST endpoint struct for openapi.json auto-gen
type CreateImageAPI struct {
	Commit         CommitAPI           `json:"commit"`                                           // commit of image
	CustomPackages []CustomPackagesAPI `json:"customPackages" example:"[Name:'customPackage1']"` // An optional list of custom packages
	Description    string              `json:"description" example:"This is an example image"`   // A short description of the image
	Distribution   string              `json:"distribution" example:"rhel-92"`                   // The RHEL for Edge OS version
	// Available image types:
	// * rhel-edge-installer - Installer ISO
	// * rhel-edge-commit - Commit only
	ImageType              string               `json:"imageType" example:"rhel-edge-installer"` // The image builder assigned image type
	Installer              InstallerAPI         `json:"installer"`
	Name                   string               `json:"name"  example:"my-edge-image"`                                // Name of created image
	Packages               []PackagesAPI        `json:"packages"`                                                     // Packages list
	OutputTypes            []string             `json:"outputTypes" example:"[rhel-edge-installer,rhel-edge-commit]"` // Output type of image
	ThirdPartyRepositories []ThirdPartyReposAPI `json:"thirdPartyRepositories"`
	Version                int                  `json:"version" example:"0"` // Version of image
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
	AdditionalPackages int    `json:"additional_packages" example:"3"` // Number of additional packages
	Packages           int    `json:"packages" example:"3"`            // Number of packages
	UpdateAdded        int    `json:"update_added" example:"3"`        // Number of added update
	UpdateRemoved      int    `json:"update_removed" example:"2"`      // Number of removed update
	UpdateUpdated      int    `json:"update_updated" example:"3"`      // Number of updated update
}

// ImagesViewDataAPI is the images view data return for images view with filters , limit, offSet
type ImagesViewDataAPI struct {
	Count int64       `json:"count" example:"100"` // total number of image view data
	Data  []ImageView `json:"data"`
}
