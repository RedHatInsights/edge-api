package devices

import (
	"context"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/utility"
	log "github.com/sirupsen/logrus"
)

type EventDeviceSyncHandlerDummy struct {
	models.CRCCloudEvent
	Data models.EdgeBasePayload `json:"data,omitempty"`
}

type EventDeviceSyncHandler struct {
	EventDeviceSyncHandlerDummy
}

func (ev EventDeviceSyncHandler) Consume(ctx context.Context) {
	eventlog := utility.GetLoggerFromContext(ctx)
	eventlog.Info("Starting device sync")

	if ev.RedHatOrgID == "" || ev.Data.RequestID == "" {
		eventlog.WithFields(log.Fields{
			"requestId": ev.Data.RequestID,
			"orgID":     ev.RedHatOrgID,
		}).Error("Malformed device sync request, required data missing")
		return
	}

	if ev.RedHatOrgID != ev.Data.Identity.Identity.OrgID {
		eventlog.WithFields(log.Fields{
			"IdentityId": ev.Data.Identity.Identity.OrgID,
			"orgID":      ev.RedHatOrgID,
		}).Error("Malformed device sync request, required data mismatch")
		return
	}
	// get the services from the context
	edgeAPIServices := dependencies.ServicesFromContext(ctx)
	deviceService := edgeAPIServices.DeviceService
	deviceService.SyncDevicesWithInventory(ev.RedHatOrgID)
}
