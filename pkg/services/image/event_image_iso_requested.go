package image

import (
	"context"
	"encoding/json"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
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
	eventlog := GetLoggerFromContext(ctx)
	eventlog.Info("Starting image iso build")

	// rebuilding the context and identity here
	edgeAPIServices := dependencies.Init(ctx)
	ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)

	payload := ev.Data
	var image *models.Image
	imageString, err := json.Marshal(payload.NewImage)
	if err != nil {
		eventlog.Error("Error marshaling the image")
		return
	}
	err = json.Unmarshal(imageString, &image)
	if err != nil {
		eventlog.Error("Error unmarshaling the image")
		return
	}
	log := eventlog.WithFields(log.Fields{
		"requestId": image.RequestID,
		"orgID":     image.OrgID,
	})

	// call the service-based CreateImage
	imageService := services.NewImageService(ctx, log)
	err = imageService.AddUserInfo(image)
	if err != nil {
		imageService.SetErrorStatusOnImage(err, image)
		eventlog.WithField("error", err.Error()).Error("Failed creating installer for image")
		return
	}

	eventlog.WithField("status", image.Status).Debug("Processing iso image build is done")
}
