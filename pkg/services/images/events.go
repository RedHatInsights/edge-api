package images

import (
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/redhatinsights/edge-api/pkg/models"
	identity "github.com/redhatinsights/platform-go-middlewares/identity"
	log "github.com/sirupsen/logrus"
)

// EdgeMgmtImageEventInterface is the interface for the image microservice(s)
type EdgeMgmtImageEventInterface interface {
	Consume() error                                                                      // handles the execution of code against the data in the event
	post() error                                                                         // sends a message or executes code to follow event handling
	Produce(image *models.Image, ident identity.XRHID) (EdgeMgmtImageCreateEvent, error) // produces an event of this type
}

// RegisteredEvents maps the record key to a corresponding event struct for handling
var RegisteredEvents = map[string]interface{}{
	kafkacommon.RecordKeyCreateImage:     &EdgeMgmtImageCreateEvent{},
	kafkacommon.RecordKeyCreateCommit:    &EdgeMgmtImageCreateCommitEvent{},
	kafkacommon.RecordKeyCreateInstaller: &EdgeMgmtImageCreateInstallerEvent{},
}

// ProduceEvent is an interface wrapper to call the Produce method of image events
func ProduceEvent(event EdgeMgmtImageEventInterface, image *models.Image, ident identity.XRHID) (EdgeMgmtImageEventInterface, error) {
	log.Debug("Producing event via the Interface")
	event, err := event.Produce(image, ident)

	return event, err
}
