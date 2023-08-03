package models

import (
	"github.com/redhatinsights/edge-api/pkg/db"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

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

	// StaticDeltaStatusNotFound represents when a static delta is not found (not an error)
	StaticDeltaStatusNotFound = "NOTFOUND"

	// StaticDeltaStatusReady represents the static delta is ready to be used for an update
	StaticDeltaStatusReady = "READY"

	// StaticDeltaStatusUploading represents the delta is being uploaded to repo storage
	StaticDeltaStatusUploading = "UPLOADING"
)

// ReadFromStore retrieves the state of a static delta commit pair
func (sds *StaticDeltaState) ReadFromStore(edgelog *log.Entry, orgID string, deltaName string) (*StaticDeltaState, error) {
	if result := db.DB.Where("org_id = ? AND name = ?", orgID, deltaName).First(&sds); result.Error != nil {
		switch result.Error {
		case gorm.ErrRecordNotFound:
			edgelog.WithFields(log.Fields{"error": result.Error}).Error("Static delta not found")
			sds.Status = StaticDeltaStatusNotFound

			return sds, nil
		default:
			edgelog.WithFields(log.Fields{"error": result.Error}).Error("Static delta database lookup experienced an error")

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

// SaveToStore writes a static delta state to the database
func (sds *StaticDeltaState) SaveToStore(edgelog *log.Entry) error {
	edgelog.Info("Updating static delta state record")
	result := db.DB.Save(&sds)
	if result.Error != nil {
		edgelog.Error(result.Error)

		return result.Error
	}

	return nil
}
