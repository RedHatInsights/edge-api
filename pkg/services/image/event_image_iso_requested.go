// FIXME: golangci-lint
// nolint:gosimple,revive
package image

import (
	"context"

	"github.com/redhatinsights/edge-api/pkg/models"
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
	return
}
