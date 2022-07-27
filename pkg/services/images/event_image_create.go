package images

import (
	"encoding/json"

	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	identity "github.com/redhatinsights/platform-go-middlewares/identity"

	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

/*
	Events are self-contained structures that know how to Produce and Consume themselves.
	Only the primary definition needs to reside in the same file.
	Functional programming patterns should be used and can be in multiple files.

	Steps to create an event
	1. Populate the constants
		- Console event subject (Producer)
		- The topic the event will be sent across (Producer)
			NOTE: this is a convenience feature
				the event does not have to be nailed to a specific topic
		- The record key that is used to determine the event to Consume (Consumer)
	2. Define the event structure
		- ConsoleRedhatComCloudEventSchema
		- The event data struct
	3. Define the event methods
		Required
		- Produce() - the code that creates the event and sends it to the bus
		- Consume() - the code that runs against event data received from the bus

		There can be zero or more parameters in Produce() and Consume().
		The interface just requires the methods exist.
		Any other supporting functions, methods, etc. are allowed.
*/

// create_image event definitions
// TODO: define this as a struct to enable simple re-use
const (
	// SubjectEdgeMgmtImageCreateEvent is the Console Event subject
	SubjectEdgeMgmtImageCreateEvent   string = "redhat:console:fleetmanagement:createimageevent"
	TopicEdgeMgmtImageCreateEvent     string = kafkacommon.TopicFleetmgmtImageBuild
	KeyEdgeManagementImageCreateEvent string = kafkacommon.RecordKeyCreateImage
)

// EdgeMgmtImageCreateEvent defines a new image
type EdgeMgmtImageCreateEvent struct {
	ConsoleSchema models.ConsoleRedhatComCloudEventsSchema `json:"consoleschema"`
	NewImage      models.Image                             `json:"newimage"`
}

// create_image event methods

// Consume executes code against the data in the received event
func (ev EdgeMgmtImageCreateEvent) Consume() error {
	ev.pre()

	// ADD THE CREATE EVENT CODE HERE

	ev.post()

	return nil
}

// run this at the beginning of Handle()
func (ev EdgeMgmtImageCreateEvent) pre() error {
	log.Debug("EdgeMgmtImageCreateEvent running the pre()")
	identity := ev.ConsoleSchema.GetIdentity()
	log.WithField("account", identity.AccountNumber).Debug("Pre EdgeMgmtImageCreateEvent")

	return nil
}

// run this at the end of Handle()
func (ev EdgeMgmtImageCreateEvent) post() error {
	log.Debug("EdgeMgmtImageCreateEvent running the post()")
	identity := ev.ConsoleSchema.GetIdentity()
	log.WithField("account", identity.AccountNumber).Debug("Post EdgeMgmtImageCreateEvent")

	return nil
}

// Produce sets up and sends the event
func (ev EdgeMgmtImageCreateEvent) Produce(image *models.Image, ident identity.XRHID) (EdgeMgmtImageCreateEvent, error) {
	// create the event with standard console struct and body data
	consoleEvent := kafkacommon.CreateConsoleEvent(image.RequestID, image.OrgID, image.Name, SubjectEdgeMgmtImageCreateEvent, ident)
	ev.ConsoleSchema = consoleEvent
	ev.NewImage = *image

	// marshal the event into a string
	edgeEventMessage, err := json.Marshal(ev)
	if err != nil {
		log.Error("Marshal edgeEventMessage failed")
	}
	log.WithField("event", string(edgeEventMessage)).Debug("event text")

	// send the event to the bus
	if err = kafkacommon.ProduceEvent(TopicEdgeMgmtImageCreateEvent, KeyEdgeManagementImageCreateEvent, edgeEventMessage); err != nil {
		log.WithField("request_id", ev.ConsoleSchema.ID).Error("Producing the event failed")

		return ev, err
	}

	return ev, nil
}
