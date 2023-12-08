// FIXME: golangci-lint
// nolint:gosimple,revive
package image

import (
	"context"
	"encoding/json"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/utility"
	log "github.com/sirupsen/logrus"
)

// EventImageISORequestedBuildHandlerDummy is a dummy placeholder to workaround the Golang struct vs json fun
type EventImageISORequestedBuildHandlerDummy struct {
	models.CRCCloudEvent
	// shadow the CRCCloudEvent Data with the actual payload type
	Data models.EdgeImageISORequestedEventPayload `json:"data,omitempty"`
}

// EventImageISORequestedBuildHandler is the local image requested event struct that consumer methods can be declared against
type EventImageISORequestedBuildHandler struct {
	EventImageISORequestedBuildHandlerDummy
}

// Consume executes code against the data in the received event
func (ev EventImageISORequestedBuildHandler) Consume(ctx context.Context) {
	eventlog := utility.GetLoggerFromContext(ctx)
	eventlog.Info("Starting image iso build")

	payload := ev.Data
	var image *models.Image
	imageString, err := json.Marshal(payload.NewImage)
	if err != nil {
		eventlog.WithField("error", err.Error()).Error("Error marshaling the image")
		return
	}
	err = json.Unmarshal(imageString, &image)
	if err != nil {
		eventlog.WithField("error", err.Error()).Error("Error unmarshaling the image")
		return
	}
	if image.OrgID == "" || image.RequestID == "" {
		eventlog.WithFields(log.Fields{
			"message":   "Malformed image request, vital data missing",
			"requestId": image.RequestID,
			"orgID":     image.OrgID,
		})
		return
	}
	eventlog.WithFields(log.Fields{
		"message":   "Image ISO request received",
		"requestId": image.RequestID,
		"orgID":     image.OrgID,
	})

	// get the services from the context
	edgeAPIServices := dependencies.ServicesFromContext(ctx)
	imageService := edgeAPIServices.ImageService
	err = imageService.AddUserInfo(image)
	if err != nil {
		imageService.SetErrorStatusOnImage(err, image)
		eventlog.WithField("error", err.Error()).Error("Failed creating installer for image")
		return
	}

	eventlog.WithField("status", image.Status).Debug("Processing iso image build is done")
}
