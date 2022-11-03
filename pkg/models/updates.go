// FIXME: golangci-lint
// nolint:govet,revive,staticcheck
package models

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// UpdateTransaction represents the combination of an OSTree commit and a set of Inventory
// hosts that need to have the commit deployed to them
// This will ultimately kick off a transaction where the old version(s) of
// OSTree commit that are currently deployed onto those devices are combined
// with the new commit into a new OSTree repo, static deltas are computed, and
// then the result is stored in a way that can be served(proxied) by a
// Server (pkg/repo/server.go).
type UpdateTransaction struct {
	Model
	Commit          *Commit          `json:"Commit"`
	CommitID        uint             `json:"CommitID"`
	Account         string           `json:"Account"`
	OrgID           string           `json:"org_id" gorm:"index;<-:create"`
	OldCommits      []Commit         `gorm:"many2many:updatetransaction_commits;" json:"OldCommits"`
	Devices         []Device         `gorm:"many2many:updatetransaction_devices;save_association:false" json:"Devices"`
	Tag             string           `json:"Tag"`
	Status          string           `json:"Status"`
	RepoID          *uint            `json:"RepoID"`
	Repo            *Repo            `json:"Repo"`
	ChangesRefs     bool             `gorm:"default:false" json:"ChangesRefs"`
	DispatchRecords []DispatchRecord `gorm:"many2many:updatetransaction_dispatchrecords;save_association:false" json:"DispatchRecords"`
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
	Reason               string  `json:"Reason"`
	PlaybookDispatcherID string  `json:"PlaybookDispatcherID"`
}

// DevicesUpdate contains the update structure for the device
type DevicesUpdate struct {
	CommitID    uint     `json:"CommitID,omitempty"`
	DevicesUUID []string `json:"DevicesUUID"`
	// TODO: Implement updates by tag
	// Tag        string `json:"Tag"`
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
	// UpdateStatusDeviceDisconnected is for when a update is UpdateStatusDeviceDisconnected
	UpdateStatusDeviceDisconnected = "DISCONNECTED"
	// UpdateStatusDeviceUnresponsive is for when an update is UpdateStatusDeviceUnresponsive
	UpdateStatusDeviceUnresponsive = "UNRESPONSIVE"
)

const (
	// DispatchRecordStatusCreated is for when a the DispatchRecord is created
	DispatchRecordStatusCreated = "CREATED"
	// DispatchRecordStatusPending is for when a UpdateTransaction has started
	//		scheduling PlaybookDispatcher jobs but this one hasn't started yet
	DispatchRecordStatusPending = "PENDING"
	// DispatchRecordStatusRunning is for when a the DispatchRecord is running
	DispatchRecordStatusRunning = "RUNNING"
	// DispatchRecordStatusError is for when a playbook dispatcher job is in a error state
	DispatchRecordStatusError = "ERROR"
	// DispatchRecordStatusComplete is for when a playbook dispatcher job is complete
	DispatchRecordStatusComplete = "COMPLETE"
)

const (
	// UpdateReasonFailure is for when the update failed
	UpdateReasonFailure = "The playbook failed to run."
	// UpdateReasonTimeout is for when the device took more time than expected to update
	UpdateReasonTimeout = "The service timed out during the last update."
)

// ValidateRequest validates a Update Record Request
func (ur *UpdateTransaction) ValidateRequest() error {
	if ur.Devices == nil || len(ur.Devices) == 0 {
		return errors.New(DevicesCantBeEmptyMessage)
	}
	return nil
}

// BeforeCreate method is called before creating any record with update, it make sure org_id is not empty
func (ur *UpdateTransaction) BeforeCreate(tx *gorm.DB) error {
	if ur.OrgID == "" {
		log.Error("update-transaction do not have an org_id")
		return ErrOrgIDIsMandatory
	}

	return nil
}
