package models

import (
	"errors"
)

// Device is a record of Edge Devices referenced by their UUID as per the
// cloud.redhat.com Inventory.
//
//	Connected refers to the devices Cloud Connector state, 0 is unavailable
//	and 1 is reachable.
type Device struct {
	Model
	UUID        string `json:"UUID"`
	DesiredHash string `json:"DesiredHash"`
	RHCClientID string `json:"RHCClientID"`
	Connected   bool   `gorm:"default:true" json:"Connected"`
}

//  UpdateTransaction rpresents the combination of an OSTree commit and a set of Inventory
//	hosts that need to have the commit deployed to them

//	This will ultimately kick off a transaction where the old version(s) of
//	OSTree commit that are currently deployed onto those devices are combined
//	with the new commit into a new OSTree repo, static deltas are computed, and
//	then the result is stored in a way that can be served(proxied) by a
//	Server (pkg/repo/server.go).
type UpdateTransaction struct {
	Model
	Commit          *Commit          `json:"Commit"`
	CommitID        uint             `json:"CommitID"`
	Account         string           `json:"Account"`
	OldCommits      []Commit         `gorm:"many2many:updatetransaction_commits;" json:"OldCommits"`
	Devices         []Device         `gorm:"many2many:updatetransaction_devices;" json:"Devices"`
	Tag             string           `json:"Tag"`
	Status          string           `json:"Status"`
	RepoID          uint             `json:"RepoID"`
	Repo            *Repo            `json:"Repo"`
	DispatchRecords []DispatchRecord `gorm:"many2many:updatetransaction_dispatchrecords;" json:"DispatchRecords"`
}

// DispatchRecord represents the combination of a Playbook Dispatcher (https://github.com/RedHatInsights/playbook-dispatcher),
// of a PlaybookURL, a pointer to a Device, and the status.
// This is used within UpdateTransaction for accounting purposes.
type DispatchRecord struct {
	Model
	PlaybookURL          string  `json:"PlaybookURL"`
	DeviceID             uint    `json:"DeviceID"`
	Device               *Device `json:"Device"`
	Status               string  `json:"Status"`
	PlaybookDispatcherID string  `json:"PlaybookDispatcherID"`
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
