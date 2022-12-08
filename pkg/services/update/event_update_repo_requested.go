package update

import (
	"context"

	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/utility"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type EventUpdateRepoRequestedHandlerDummy struct {
	models.CRCCloudEvent
	Data models.EdgeUpdateRepoRequestedEventPayload `json:"data,omitempty"`
}

type EventUpdateRepoRequestedHandler struct {
	EventUpdateRepoRequestedHandlerDummy
}

func (ev EventUpdateRepoRequestedHandler) Consume(ctx context.Context) {
	eventLog := utility.GetLoggerFromContext(ctx)
	eventLog.Info("Starting UpdateRepoRequested consume")

	if ev.RedHatOrgID == "" || ev.Data.RequestID == "" {
		eventLog.WithFields(log.Fields{
			"requestId": ev.Data.RequestID,
			"orgID":     ev.RedHatOrgID,
		}).Error("Malformed UpdateRepoRequested request, required data missing")
		return
	}
	if ev.RedHatOrgID != ev.Data.Identity.Identity.OrgID {
		eventLog.WithFields(log.Fields{
			"requestId":  ev.Data.RequestID,
			"IdentityId": ev.Data.Identity.Identity.OrgID,
			"orgID":      ev.RedHatOrgID,
		}).Error("Malformed UpdateRepoRequested request, required data mismatch")
		return
	}
	if ev.Data.Update.ID == 0 {
		eventLog.WithFields(log.Fields{
			"requestId":  ev.Data.RequestID,
			"IdentityId": ev.Data.Identity.Identity.OrgID,
			"orgID":      ev.RedHatOrgID,
		}).Error("update repo requested, update ID is required")
		return
	}
	if ev.RedHatOrgID != ev.Data.Update.OrgID {
		eventLog.WithFields(log.Fields{
			"requestId":   ev.Data.RequestID,
			"orgID":       ev.RedHatOrgID,
			"UpdateOrgID": ev.Data.Update.OrgID,
		}).Error("Malformed UpdateRepoRequested request, event orgID and update orgID mismatch")
		return
	}

	var updateTransaction models.UpdateTransaction
	if result := db.Org(ev.RedHatOrgID, "").First(&updateTransaction, ev.Data.Update.ID); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			eventLog.WithFields(log.Fields{
				"requestId": ev.Data.RequestID,
				"orgID":     ev.RedHatOrgID,
				"updateID":  ev.Data.Update.ID,
			}).Error("event UpdateRepoRequested update does not exist")
			return
		}
		eventLog.WithFields(log.Fields{
			"requestId": ev.Data.RequestID,
			"orgID":     ev.RedHatOrgID,
			"updateID":  ev.Data.Update.ID,
			"error":     result.Error.Error(),
		}).Error("event UpdateRepoRequested update retrieve error")
		return
	}

	if updateTransaction.Status != models.UpdateStatusBuilding {
		eventLog.WithFields(
			log.Fields{"requestId": ev.Data.RequestID, "orgID": updateTransaction.OrgID, "updateID": updateTransaction.ID},
		).Error("event UpdateRepoRequested update not in building state")
		return
	}

	// get the services from the context
	edgeAPIServices := dependencies.ServicesFromContext(ctx)
	_, err := edgeAPIServices.UpdateService.BuildUpdateRepo(updateTransaction.OrgID, updateTransaction.ID)
	if err != nil {
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
	if err = edgeAPIServices.UpdateService.ProduceEvent(
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
