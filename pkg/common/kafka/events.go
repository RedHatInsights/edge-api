// FIXME: golangci-lint
// nolint:revive
package kafkacommon

import (
	"time"

	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/pkg/models"
)

// CreateEdgeEvent creates an event with standard CRC fields and edge payload
func CreateEdgeEvent(orgID string, source string, reqID string,
	eventType string, subject string, payload interface{}) models.CRCCloudEvent {

	cloudEvent := models.CRCCloudEvent{
		Data:        payload,
		DataSchema:  "v1",
		ID:          uuid.New().String(),
		RedHatOrgID: orgID,
		Source:      source,
		SpecVersion: "v1",
		Subject:     subject,
		Time:        time.Now().Format(time.RFC3339),
		Type:        eventType,
	}

	return cloudEvent
}
