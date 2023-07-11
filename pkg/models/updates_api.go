package models

type UpdateCommitAPI struct {
	ID               uint   `json:"ID" example:"1056"`                                                                                          // The unique ID of the commit
	ImageBuildTarURL string `json:"ImageBuildTarURL" example:"https://storage-host.example.com/v2/99999999/tar/59794/tmp/repos/59794/repo.tar"` // The commit tar url
	OSTreeCommit     string `json:"OSTreeCommit" example:"9bd8dfe9856aa5bb1683e85f123bfe7785d45fbdb6f10372ff2c80e703400999"`                    // The ostree commit hash
	OSTreeRef        string `json:"OSTreeRef" example:"rhel/9/x86_64/edge"`                                                                     // The commit ostree ref
} // UpdateCommit

type UpdateDeviceAPI struct {
	ID              uint   `json:"ID" example:"1096"`                                                                                // The unique ID of the device
	UUID            string `json:"UUID" example:"54880418-b7c2-402e-93e5-287e168de7a6"`                                              // The device inventory uuid
	RHCClientID     string `json:"RHCClientID" example:"5f9ac7d3-2264-4dad-a5a0-39c91c071c8a"`                                       // The device RHC client ID
	Connected       bool   `json:"Connected" example:"true"`                                                                         // Is the device connected
	Name            string `json:"Name" example:"teat-host.example.com"`                                                             // the device inventory name
	CurrentHash     string `json:"CurrentHash,omitempty" example:"0bd8dfe9856aa5bb1683e85f123bfe7785d45fbdb6f10372ff2c80e703400446"` // the current device loaded commit hash
	ImageID         uint   `json:"ImageID" gorm:"index" example:"10778"`                                                             // The current related image ID
	UpdateAvailable bool   `json:"UpdateAvailable" example:"true"`                                                                   // Whether an update is available
} // @name UpdateDevice

type UpdateDispatchRecordAPI struct {
	ID                   uint   `json:"ID" example:"1089"`                                                                             // The unique ID of the DispatcherRecord
	PlaybookURL          string `json:"PlaybookURL" example:"https://console.redhat.com/api/edge/v1/updates/1026/update-playbook.yml"` // The generated playbook url
	DeviceID             uint   `json:"DeviceID" example:"12789"`                                                                      // The unique ID of the device being updated
	Status               string `json:"Status" example:"BUILDING"`                                                                     // The status of device update
	Reason               string `json:"Reason" example:""`                                                                             // In case of failure the error reason returned by the playbook-dispatcher service
	PlaybookDispatcherID string `json:"PlaybookDispatcherID" example:"c84cfd11-745c-4ee3-b87d-057a96732415"`                           // The playbook dispatcher job id
} // @name UpdateDispatchRecord

type UpdateRepoAPI struct {
	ID         uint   `json:"ID" example:"53218"`                                                      // The unique ID of the update repository
	RepoURL    string `json:"RepoURL" example:"https://storage-host.example.com/53218/upd/53218/repo"` // The url of the update ostree repository
	RepoStatus string `json:"RepoStatus" example:"SUCCESS"`                                            // The status of the device update repository building
} // @name UpdateRepo

// UpdateAPI The structure of a device update
type UpdateAPI struct {
	ID              uint                      `json:"ID" example:"1026"`           // The unique ID of device update
	Commit          UpdateCommitAPI           `json:"Commit"`                      // The device Update target commit
	OldCommits      []UpdateCommitAPI         `json:"OldCommits"`                  // The device alternate commits from current device commit to target commit
	Devices         []UpdateDeviceAPI         `json:"Devices"`                     // The current devices to update
	Status          string                    `json:"Status" example:"BUILDING"`   // the current devices update status
	Repo            *UpdateRepoAPI            `json:"Repo"`                        // The current repository built from this update
	ChangesRefs     bool                      `json:"ChangesRefs" example:"false"` // Whether this update is changing device ostree ref
	DispatchRecords []UpdateDispatchRecordAPI `json:"DispatchRecords"`             // The current update dispatcher records
} // @name Update

// DevicesUpdateAPI the structure for creating device updates
type DevicesUpdateAPI struct {
	CommitID    uint     `json:"CommitID,omitempty" example:"1026"`                                                               // Optional: The unique ID of the target commit
	DevicesUUID []string `json:"DevicesUUID" example:"b579a578-1a6f-48d5-8a45-21f2a656a5d4,1abb288d-6d88-4e2d-bdeb-fcc536be58ec"` // List of devices uuids to update
} // @name DevicesUpdate

// ImageValidationRequestAPI is the structure for validating images for device updates
type ImageValidationRequestAPI struct {
	ID uint `json:"ID" example:"1029"` // the unique ID of the image
} // @name ImageValidationRequest

// ImageValidationResponseAPI is the validation response of images for device updates
type ImageValidationResponseAPI struct {
	UpdateValid bool `json:"UpdateValid" example:"true"`
} // @name ImageValidationResponse

// DeviceNotificationAPI is the implementation of expected notification payload
type DeviceNotificationAPI struct {
	Version     string                     `json:"version" example:"v1.1.0"`                                          // notification version
	Bundle      string                     `json:"bundle" example:"rhel"`                                             // bundle name
	Application string                     `json:"application" example:"edge-management"`                             // application name
	EventType   string                     `json:"event_type" example:"update-devices"`                               // event type
	Timestamp   string                     `json:"timestamp" example:"2023-07-06T11:15:04Z"`                          // notification timestamp
	OrgID       string                     `json:"org_id" example:"11111111"`                                         // notification organization id
	Context     string                     `json:"context" example:"{\"CommitID\":\"31581\",\"UpdateID\":\"34916\"}"` // notification context payload data
	Events      []EventNotificationAPI     `json:"events"`                                                            // notification events
	Recipients  []RecipientNotificationAPI `json:"recipients"`                                                        // notification recipients
} // @name DeviceNotification

// EventNotificationAPI is used to track events to notification
type EventNotificationAPI struct {
	Payload string `json:"payload" example:"{\"ID\":\"\"}"` // notification event payload
} // @name EventNotification

// RecipientNotificationAPI is used to track recipients to notification
type RecipientNotificationAPI struct {
	OnlyAdmins            bool     `json:"only_admins" example:"false"`             // notification recipient for only admins
	IgnoreUserPreferences bool     `json:"ignore_user_preferences" example:"false"` // notification recipient to ignore user preferences
	Users                 []string `json:"users" example:"user-id"`                 // notification recipient users
} // @name RecipientNotification
