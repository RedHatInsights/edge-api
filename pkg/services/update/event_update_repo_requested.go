// nolint:revive,typecheck
package update

import (
	"context"
	"errors"

	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/utility"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var ErrEventHandlerValidationError = errors.New("event handler validation error")
var ErrEventHandlerMissingRequiredData = errors.New("malformed UpdateRepoRequested request, required data missing")
var ErrEventHandlerRequiredDataMismatch = errors.New("malformed UpdateRepoRequested request, required data mismatch")
var ErrEventHandlerUpdateIDRequired = errors.New("update repo requested, update ID is required")
var ErrEventHandlerUpdateOrgIDMismatch = errors.New("malformed UpdateRepoRequested request, event orgID and update orgID mismatch")
var ErrEventHandlerUpdateDoesNotExist = errors.New("event UpdateRepoRequested update-transaction does not exist")
var ErrEventHandlerUpdateBadStatus = errors.New("event UpdateRepoRequested update not in building state")

type EventUpdateRepoRequestedHandlerDummy struct {
	models.CRCCloudEvent
	Data models.EdgeUpdateRepoRequestedEventPayload `json:"data,omitempty"`
}

type EventUpdateRepoRequestedHandler struct {
	EventUpdateRepoRequestedHandlerDummy
}

func (ev EventUpdateRepoRequestedHandler) ValidateEvent() (*models.UpdateTransaction, error) {
	if ev.RedHatOrgID == "" || ev.Data.RequestID == "" {
		return nil, ErrEventHandlerMissingRequiredData
	}
	if ev.RedHatOrgID != ev.Data.Identity.Identity.OrgID {
		return nil, ErrEventHandlerRequiredDataMismatch
	}
	if ev.Data.Update.ID == 0 {
		return nil, ErrEventHandlerUpdateIDRequired
	}
	if ev.RedHatOrgID != ev.Data.Update.OrgID {
		return nil, ErrEventHandlerUpdateOrgIDMismatch
	}

	var updateTransaction models.UpdateTransaction
	if result := db.Org(ev.RedHatOrgID, "").First(&updateTransaction, ev.Data.Update.ID); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, ErrEventHandlerUpdateDoesNotExist
		}
		return nil, result.Error
	}

	if updateTransaction.Status != models.UpdateStatusBuilding {
		return nil, ErrEventHandlerUpdateBadStatus
	}

	return &updateTransaction, nil
}

func (ev EventUpdateRepoRequestedHandler) Consume(ctx context.Context) {
	eventLog := utility.GetLoggerFromContext(ctx)
	eventLog.Info("Starting UpdateRepoRequested consume")

	updateTransaction, err := ev.ValidateEvent()
	if err != nil {
		eventLog.WithFields(log.Fields{
			"requestId":   ev.Data.RequestID,
			"orgID":       ev.RedHatOrgID,
			"updateID":    ev.Data.Update.ID,
			"updateOrgID": ev.Data.Update.OrgID,
			"error":       err.Error(),
		}).Error("event failed Validation")
		return
	}

	// get the services from the context
	edgeAPIServices := dependencies.ServicesFromContext(ctx)
	if _, err = edgeAPIServices.UpdateService.BuildUpdateRepo(updateTransaction.OrgID, updateTransaction.ID); err != nil {
		eventLog.WithFields(log.Fields{
			"requestID": ev.Data.RequestID,
			"orgID":     updateTransaction.OrgID,
			"updateID":  updateTransaction.ID,
			"error":     err.Error(),
		}).Error("Error occurred while building update repo")
		return
	}

	// at this point update repo built successfully
	// send an update WriteTemplate event
	edgeEvent := kafkacommon.CreateEdgeEvent(
		ev.RedHatOrgID,
		models.SourceEdgeEventAPI,
		ev.Data.RequestID,
		models.EventTypeEdgeWriteTemplateRequested,
		ev.Subject,
		ev.Data,
	)
	if err = edgeAPIServices.ProducerService.ProduceEvent(
		kafkacommon.TopicFleetmgmtUpdateWriteTemplateRequested, models.EventTypeEdgeWriteTemplateRequested, edgeEvent,
	); err != nil {
		log.WithField("request_id", ev.Data.RequestID).Error("producing the WriteTemplate event failed")
		// set update status to error
		updateTransaction.Status = models.UpdateStatusError
		if result := db.DB.Save(&updateTransaction); result.Error != nil {
			log.WithFields(log.Fields{
				"requestID": ev.Data.RequestID,
				"orgID":     updateTransaction.OrgID,
				"updateID":  updateTransaction.ID,
				"error":     err.Error(),
			}).Error("failed to save update error status when WriteTemplate event failed")
		}
		return
	}
	eventLog.Info("Finished UpdateRepoRequested consume")
}
