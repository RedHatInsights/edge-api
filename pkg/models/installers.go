package models

// Installer defines the model for a ISO installer
type Installer struct {
	Model
	Account          string `json:"Account"`
	ImageBuildISOURL string `json:"ImageBuildISOURL"`
	ComposeJobID     string `json:"ComposeJobID"`
	Status           string `json:"Status"`
	Username         string `json:"Username"`
	SSHKey           string `json:"SshKey"`
	Checksum         string `json:"Checksum"`
}
