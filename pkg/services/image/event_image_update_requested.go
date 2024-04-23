// FIXME: golangci-lint
// nolint:gosimple,revive
package image

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/utility"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
	log "github.com/sirupsen/logrus"
)

// EventImageUpdateRequestedBuildHandlerDummy is a dummy placeholder to workaround the Golang struct vs json fun
type EventImageUpdateRequestedBuildHandlerDummy struct {
	models.CRCCloudEvent
	// shadow the CRCCloudEvent Data with the actual payload type
	Data models.EdgeImageUpdateRequestedEventPayload `json:"data,omitempty"`
}

// EventImageUpdateRequestedBuildHandler is the local image update requested event struct that consumer methods can be declared against
type EventImageUpdateRequestedBuildHandler struct {
	EventImageUpdateRequestedBuildHandlerDummy
}

// Consume executes code against the data in the received event
func (ev EventImageUpdateRequestedBuildHandler) Consume(ctx context.Context) {
	eventlog := utility.GetLoggerFromContext(ctx)

	eventlog.Info("Starting image build")

	payload := ev.Data
	id := payload.GetIdentity()
	identityBytes, err := json.Marshal(id)
	if err != nil {
		eventlog.WithField("error", err.Error()).Error("Error Marshaling the identity into a string")
		return
	}

	base64Identity := base64.StdEncoding.EncodeToString(identityBytes)

	// add the new identity to the context and create ctxServices with that context
	ctx = identity.WithIdentity(ctx, id)
	ctx = identity.WithRawIdentity(ctx, base64Identity)

	// temporarily using some Marshal/Unmarshal conjuring to move our future EDA image back to models.Image world
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
	log := eventlog.WithFields(log.Fields{
		"requestId": image.RequestID,
		"orgID":     image.OrgID,
	})

	// process the image
	// get the services from the context
	edgeAPIServices := dependencies.ServicesFromContext(ctx)
	imageService := edgeAPIServices.ImageService
	err = imageService.ProcessImage(ctx, image, true)
	if err != nil {
		log.WithField("error", err.Error()).Error("Error processing the image")
	}
}
