package image

import (
	"github.com/redhatinsights/edge-api/pkg/models"
)

// EdgeMgmtImageEventInterface is the interface for the image microservice(s)
type EdgeMgmtImageEventInterface interface {
	Consume() error // handles the execution of code against the data in the event
}

// RegisteredEvents maps the record key to a corresponding event struct for handling
var RegisteredEvents = map[string]interface{}{
	models.EventTypeEdgeImageRequested: &EventImageRequested{},
}
