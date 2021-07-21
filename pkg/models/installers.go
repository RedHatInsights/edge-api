package models

import "gorm.io/gorm"

// Installer defines the model for a ISO installer
type Installer struct {
	gorm.Model
	Account          string `json:"Account"`
	ImageBuildISOURL string `json:"ImageBuildISOURL"`
	ComposeJobID     string `json:"ComposeJobID"`
	Status           string `json:"Status"`
	Username         string `json:"User name"`
	Sshkey           string `json:"Ssh Key"`
}
