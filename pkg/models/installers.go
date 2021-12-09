package models

// Installer defines the model for a ISO installer
type Installer struct {
	Model
	Account          string `json:"account"`
	ImageBuildISOURL string `json:"image_build_isourl"`
	ComposeJobID     string `json:"compose_job_id"`
	Status           string `json:"status"`
	Username         string `json:"username"`
	SSHKey           string `json:"ssh_key"`
	Checksum         string `json:"checksum"`
}
