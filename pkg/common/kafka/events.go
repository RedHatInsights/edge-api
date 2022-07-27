package kafkacommon

import (
	"time"

	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

// CreateConsoleEvent helps to create the default console event
func CreateConsoleEvent(reqID, orgID, name, subject string, ident identity.XRHID) models.ConsoleRedhatComCloudEventsSchema {
	consoleEvent := models.ConsoleRedhatComCloudEventsSchema{
		Dataschema:  "v1",
		ID:          reqID,
		Redhatorgid: orgID,
		Source:      "Edge Management",
		Specversion: "v1",
		Subject:     subject,
		System: &models.RhelSystem{
			DisplayName: name,
		},
		Time:           time.Now().Format(time.RFC3339),
		Identity:       ident,
		Lasthandeltime: time.Now().Format(time.RFC3339),
	}
	return consoleEvent
}
