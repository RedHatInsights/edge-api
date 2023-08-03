package models

// StaticDeltaState models the state of an existing static delta
type StaticDeltaState struct {
	Model
	Name   string `json:"name" gorm:"index;<-:create"` // the fromcommit-tocommit static delta name
	OrgID  string `json:"org_id"`                      // the owner of the static delta
	Status string `json:"status"`                      // the status of the generation process
	URL    string `json:"url"`                         // url for the repo where the static delta is stored
}

// values for the StaticDeltaState Status field
const (
	// StaticDeltaStatusError represents there has been an error in the process
	StaticDeltaStatusError = "ERROR"

	// StaticDeltaStatusGenerating represents static delta generation is in process
	StaticDeltaStatusGenerating = "GENERATING"

	// StaticDeltaStatusReady represents the static delta is ready to be used for an update
	StaticDeltaStatusReady = "READY"

	// StaticDeltaStatusUploading represents the delta is being uploaded to repo storage
	StaticDeltaStatusUploading = "UPLOADING"
)
