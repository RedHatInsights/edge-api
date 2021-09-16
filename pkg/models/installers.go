package models

import "gorm.io/gorm"

// Installer defines the model for a ISO installer
type Installer struct {
	gorm.Model
	Account          string `json:"account"`
	ImageBuildISOURL string `json:"image_build_iso_url"`
	ComposeJobID     string `json:"compose_job_id"`
	Status           string `json:"status"`
	Username         string `json:"username"`
	SSHKey           string `json:"sshkey"`
	Checksum         string `json:"checksum"`
}
