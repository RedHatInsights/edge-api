package devices

import (
	"context"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/utility"
	log "github.com/sirupsen/logrus"
)

type EventInventorySyncHandlerDummy struct {
	models.CRCCloudEvent
	Data models.EdgeBasePayload `json:"data,omitempty"`
}

type EventInventorySyncHandler struct {
	EventInventorySyncHandlerDummy
}

func (ev EventInventorySyncHandler) Consume(ctx context.Context) {
	eventlog := utility.GetLoggerFromContext(ctx)
	eventlog.Info("Starting inventory sync")

	if ev.RedHatOrgID == "" || ev.Data.RequestID == "" {
		eventlog.WithFields(log.Fields{
			"message":   "Malformed inventory sync request, required data missing",
			"requestId": ev.Data.RequestID,
			"orgID":     ev.RedHatOrgID,
		})
		return
	}

	if ev.RedHatOrgID != ev.Data.Identity.Identity.OrgID {
		eventlog.WithFields(log.Fields{
			"message":    "Malformed inventory sync request, required data mis match",
			"IdentityId": ev.Data.Identity.Identity.OrgID,
			"orgID":      ev.RedHatOrgID,
		})
		return
	}
	// get the services from the context
	edgeAPIServices := dependencies.ServicesFromContext(ctx)
	deviceService := edgeAPIServices.DeviceService
	deviceService.SyncInventoryWithDevices(ev.RedHatOrgID)
}
