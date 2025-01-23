// nolint:revive,typecheck
package devices

import (
	"context"

	log "github.com/osbuild/logging/pkg/logrus"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/utility"
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
			"requestId": ev.Data.RequestID,
			"orgID":     ev.RedHatOrgID,
		}).Error("Malformed device sync request, required data missing")
		return
	}

	if ev.RedHatOrgID != ev.Data.Identity.Identity.OrgID {
		eventlog.WithFields(log.Fields{
			"IdentityId": ev.Data.Identity.Identity.OrgID,
			"orgID":      ev.RedHatOrgID,
		}).Error("Malformed device sync request, required data missing")
		return
	}
	// get the services from the context
	edgeAPIServices := dependencies.ServicesFromContext(ctx)
	deviceService := edgeAPIServices.DeviceService
	deviceService.SyncInventoryWithDevices(ev.RedHatOrgID)
}
