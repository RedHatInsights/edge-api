package models

import (
	"errors"

	"gorm.io/gorm"
)

/*
Device

	A record of Edge Devices referenced by their UUID as per the
	cloud.redhat.com Inventory.

	ConnectionState refers to the devices Cloud Connector state, 0 is unavailable
	and 1 is reachable.
*/
type Device struct {
	gorm.Model
	UUID            string
	DesiredHash     string
	ConnectionState bool `gorm:"default:true"`
}

/*
UpdateTransaction

	Represents the combination of an OSTree commit and a set of Inventory
	hosts that need to have the commit deployed to them

	This will ultimately kick off a transaction where the old version(s) of
	OSTree commit that are currently deployed onto those devices are combined
	with the new commit into a new OSTree repo, static deltas are computed, and
	then the result is stored in a way that can be served(proxied) by a
	Server (pkg/repo/server.go).
*/
type UpdateTransaction struct {
	gorm.Model
	Commit         *Commit
	CommitID       uint
	Account        string
	OldCommits     []Commit `gorm:"many2many:updatetransaction_commits;"`
	InventoryHosts []Device `gorm:"many2many:updatetransaction_devices;"`
	Tag            string
	Status         string
}

const (
	// InventoryHostsCantBeEmptyMessage is the error message when the hosts are empty
	InventoryHostsCantBeEmptyMessage = "inventory hosts can not be empty"

	// UpdateStatusCreated is for when a update is created
	UpdateStatusCreated = "CREATED"
	// UpdateStatusBuilding is for when a update is building
	UpdateStatusBuilding = "BUILDING"
	// UpdateStatusError is for when a update is on a error state
	UpdateStatusError = "ERROR"
	// UpdateStatusSuccess is for when a update is available to the user
	UpdateStatusSuccess = "SUCCESS"
)

// ValidateRequest validates a Update Record Request
func (ur *UpdateTransaction) ValidateRequest() error {
	if ur.InventoryHosts == nil || len(ur.InventoryHosts) == 0 {
		return errors.New(InventoryHostsCantBeEmptyMessage)
	}
	return nil
}
