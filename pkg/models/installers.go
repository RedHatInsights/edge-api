// FIXME: golangci-lint
// nolint:revive
package models

import (
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Installer defines the model for a ISO installer
type Installer struct {
	Model
	Account          string `json:"Account"`
	OrgID            string `json:"org_id" gorm:"index;<-:create"`
	ImageBuildISOURL string `json:"ImageBuildISOURL"`
	ComposeJobID     string `json:"ComposeJobID"`
	Status           string `json:"Status"`
	Username         string `json:"Username"`
	SSHKey           string `json:"SshKey"`
	Checksum         string `json:"Checksum"`
}

// BeforeCreate method is called before create a record on installer, it make sure org_id is not empty
func (i *Installer) BeforeCreate(tx *gorm.DB) error {
	if i.OrgID == "" {
		log.Error("installer do not have an org_id")
		return ErrOrgIDIsMandatory
	}

	return nil
}
