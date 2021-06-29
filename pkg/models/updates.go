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
	ConnectionState int `gorm:"default:1"`
}

/*
UpdateRecord

	Represents the combination of an OSTree commit and a set of Inventory
	hosts that need to have the commit deployed to them

	This will ultimately kick off a transaction where the old version(s) of
	OSTree commit that are currently deployed onto those devices are combined
	with the new commit into a new OSTree repo, static deltas are computed, and
	then the result is stored in a way that can be served(proxied) by a
	Server (pkg/repo/server.go).
*/
type UpdateRecord struct {
	gorm.Model
	UpdateCommitID uint
	Account        string
	OldCommitIDs   string
	InventoryHosts []Device `gorm:"many2many:updaterecord_devices;"`
	Status         string
	UpdateRepoURL  string
}

const (
	// Errors
	UpdateCommitIDCantBeNilMessage   = "update commit id can't be empty"
	AccountCantBeEmptyMessage        = "account can't be empty"
	InventoryHostsCantBeEmptyMessage = "inventory hosts can not be empty"

	// Status
	UpdateStatusCreated  = "CREATED"
	UpdateStatusBuilding = "BUILDING"
	UpdateStatusError    = "ERROR"
	UpdateStatusSuccess  = "SUCCESS"
)

func (ur *UpdateRecord) ValidateRequest() error {
	if ur.UpdateCommitID == 0 {
		return errors.New(UpdateCommitIDCantBeNilMessage)
	}
	if ur.InventoryHosts == nil || len(ur.InventoryHosts) == 0 {
		return errors.New(InventoryHostsCantBeEmptyMessage)
	}
	return nil
}
