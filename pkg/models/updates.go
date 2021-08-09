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
	RHCClientID     string
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
	Commit          *Commit
	CommitID        uint
	Account         string
	OldCommits      []Commit `gorm:"many2many:updatetransaction_commits;"`
	Devices         []Device `gorm:"many2many:updatetransaction_devices;"`
	Tag             string
	Status          string
	RepoID          uint
	Repo            *Repo
	DispatchRecords []DispatchRecord `gorm:"many2many:updatetransaction_dispatchrecords;"`
}

/*
DispatchRecord

	Represents the combination of a Playbook Dispatcher (https://github.com/RedHatInsights/playbook-dispatcher)
	PlaybookURL, a pointer to a Device, and the status.

	This is used within UpdateTransaction for accounting purposes.

*/
type DispatchRecord struct {
	gorm.Model
	PlaybookURL          string
	DeviceID             uint
	Device               *Device
	Status               string
	PlaybookDispatcherID string
}

const (
	// DevicesCantBeEmptyMessage is the error message when the hosts are empty
	DevicesCantBeEmptyMessage = "devices can not be empty"

	// UpdateStatusCreated is for when a update is created
	UpdateStatusCreated = "CREATED"
	// UpdateStatusBuilding is for when a update is building
	UpdateStatusBuilding = "BUILDING"
	// UpdateStatusError is for when a update is on a error state
	UpdateStatusError = "ERROR"
	// UpdateStatusSuccess is for when a update is available to the user
	UpdateStatusSuccess = "SUCCESS"
)

const (
	// DispatchRecordStatusCreated is for when a the DispatchRecord is created
	DispatchRecordStatusCreated = "CREATED"
	// DispatchRecordStatusBuilding is for when a UpdateTransaction has started
	//		scheduling PlaybookDispatcher jobs but this one hasn't started yet
	DispatchRecordStatusPending = "PENDING"
	// DispatchRecordStatusRunning is for when a the DispatchRecord is running
	DispatchRecordStatusRunning = "RUNNING"
	// DispatchRecordStatusError is for when a playbook dispatcher job is in a error state
	DispatchRecordStatusError = "ERROR"
	// DispatchRecordStatusSuccess is for when a playbook dispatcher job is complete
	DispatchRecordStatusComplete = "COMPLETE"
)

// ValidateRequest validates a Update Record Request
func (ur *UpdateTransaction) ValidateRequest() error {
	if ur.Devices == nil || len(ur.Devices) == 0 {
		return errors.New(DevicesCantBeEmptyMessage)
	}
	return nil
}
