// FIXME: golangci-lint
// nolint:gosimple,revive
package image

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"

	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// EventImageRequestedBuildHandlerDummy is a dummy placeholder to workaround the Golang struct vs json fun
type EventImageRequestedBuildHandlerDummy struct {
	models.CRCCloudEvent
	// shadow the CRCCloudEvent Data with the actual payload type
	Data models.EdgeImageRequestedEventPayload `json:"data,omitempty"`
}

// EventImageRequestedBuildHandler is the local image requested event struct that consumer methods can be declared against
type EventImageRequestedBuildHandler struct {
	EventImageRequestedBuildHandlerDummy
}

// Consume executes code against the data in the received event
func (ev EventImageRequestedBuildHandler) Consume(ctx context.Context) {
	eventlog := GetLoggerFromContext(ctx)

	// rebuilding the context and identity here
	edgeAPIServices := dependencies.Init(ctx)
	ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)

	eventlog.Info("Starting image build")

	payload := ev.Data
	identity := payload.GetIdentity()
	identityBytes, err := json.Marshal(identity)
	if err != nil {
		eventlog.Error("Error Marshaling the identity into a string")
		return
	}

	base64Identity := base64.StdEncoding.EncodeToString(identityBytes)

	// add the new identity to the context and create ctxServices with that context
	ctx = common.SetOriginalIdentity(ctx, base64Identity)

	// temporarily using some Marshal/Unmarshal conjuring to move our future EDA image back to models.Image world
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
	err = imageService.ProcessImage(image)
	if err != nil {
		log.Error("Error processing the image")
	}

	return
}
