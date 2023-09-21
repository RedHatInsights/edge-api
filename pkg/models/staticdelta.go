package models

import (
	"fmt"

	"github.com/redhatinsights/edge-api/pkg/db"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// StaticDelta models the basic data needed to generate a static delta
type StaticDelta struct {
	FromCommit StaticDeltaCommit
	ImagesetID uint64           `json:"imageset_id"` // the ID of the imageset with commits (versions)
	Name       string           `json:"name"`        // the combined commitfrom-committo static delta name
	State      StaticDeltaState `json:"state"`
	ToCommit   StaticDeltaCommit
	Type       string `json:"type"` // delta is stored with "repo" (default) or in external "file"
}

// StaticDeltaCommit models a commit specific to generation of static deltas
type StaticDeltaCommit struct {
	Path   string `json:"path"` // the local path for the from commit repo
	Ref    string `json:"ref"`  // the ref for from commit (e.g., x86_64/edge)
	Rev    string `json:"rev"`  // the commit Rev
	TarURL string `json:"tar_url"`
	URL    string `json:"url"`
}

// StaticDeltaState models the state of a static delta database record
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

	// StaticDeltaStatusNotFound represents when a static delta is not found (not an error)
	StaticDeltaStatusNotFound = "NOTFOUND"

	// StaticDeltaStatusNotFound represents static delta under development (not an error)
	StaticDeltaStatusNotImplemented = "NOTIMPLEMENTED"

	// StaticDeltaStatusReady represents the static delta is ready to be used for an update
	StaticDeltaStatusReady = "READY"

	// StaticDeltaStatusUploading represents the delta is being uploaded to repo storage
	StaticDeltaStatusUploading = "UPLOADING"
)

func GetStaticDeltaName(fromCommitHash string, toCommitHash string) string {
	return fmt.Sprintf("%s-%s", fromCommitHash, toCommitHash)
}

// Delete removes the static delta state record from the database
func (sds *StaticDeltaState) Delete(edgelog *log.Entry) error {
	edgelog.Info("Deleting static delta state record")
	result := db.DB.Where("org_id = ? AND name = ?", sds.OrgID, sds.Name).Delete(&sds)
	if result.Error != nil {
		edgelog.Error(result.Error)

		return result.Error
	}

	return nil
}

// Exists returns a comma ok true if the static delta state record exists in the database
func (sds *StaticDeltaState) Exists(edgelog *log.Entry) (bool, error) {
	dbState, err := sds.Query(edgelog)
	if err != nil {
		return false, err
	}

	if dbState.Status != StaticDeltaStatusNotFound {
		return true, nil
	}

	return false, nil
}

// IsReady returns a comma ok true if the static delta state is set to ready in the database
func (sds *StaticDeltaState) IsReady(edgelog *log.Entry) (bool, error) {
	dbState, err := sds.Query(edgelog)
	if err != nil {
		return false, err
	}

	if dbState.Status == StaticDeltaStatusReady {
		return true, nil
	}

	return false, nil
}

// Query retrieves the state of a static delta commit pair from the database
func (sds *StaticDeltaState) Query(edgelog *log.Entry) (*StaticDeltaState, error) {
	if result := db.DB.Where("org_id = ? AND name = ?", sds.OrgID, sds.Name).First(&sds); result.Error != nil {
		switch result.Error {
		case gorm.ErrRecordNotFound:
			sds.Status = StaticDeltaStatusNotFound

			edgelog.WithFields(log.Fields{
				"org_id": sds.OrgID,
				"name":   sds.Name,
				"status": sds.Status}).Info("Static delta not found")

			return sds, nil
		default:
			edgelog.WithFields(log.Fields{"error": result.Error}).Error("Static delta database query experienced an error")

			return sds, result.Error
		}
	}

	edgelog.WithFields(log.Fields{
		"org_id": sds.OrgID,
		"name":   sds.Name,
		"url":    sds.URL,
		"status": sds.Status}).Debug("Found a static delta")

	return sds, nil
}

// Save writes a static delta state to the database
func (sds *StaticDeltaState) Save(edgelog *log.Entry) error {
	edgelog.WithFields(log.Fields{
		"org_id": sds.OrgID,
		"name":   sds.Name,
		"url":    sds.URL,
		"status": sds.Status}).Debug("Saving static delta state")
	result := db.DB.Save(&sds)
	if result.Error != nil {
		edgelog.Error(result.Error)
		edgelog.WithFields(log.Fields{"error": result.Error}).Error("Error while saving static delta state")

		return result.Error
	}

	return nil
}
