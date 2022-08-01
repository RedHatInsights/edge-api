package images

import (
	"context"
	"encoding/base64"
	"encoding/json"

	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
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
	// rebuilding the context and identity here
	// TODO: move this to its own context function
	// TODO: then get rid of the context and service dependencies
	ctx := context.Background()
	// TODO: refactor services out of the mix so the microservices can more easily stand alone
	edgeAPIServices := dependencies.Init(ctx)
	ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)

	//resumeLog := edgeAPIServices.Log.WithField("originalRequestId", image.RequestID)
	log.Info("Starting image build")

	// recreate a stripped down identity header
	//strippedIdentity := `{ "identity": {"account_number": ` + image.Account + `, "type": "User", "internal": {"org_id": ` + image.OrgID + `, }, }, }`
	//resumeLog.WithField("identity_text", strippedIdentity).Debug("Creating a new stripped identity")
	identity := ev.ConsoleSchema.GetIdentity()
	identityBytes, err := json.Marshal(identity)
	log.WithField("marshaled_identity", string(identityBytes)).Debug("Marshaled the identity")
	if err != nil {
		log.Error("Error Marshaling the identity into a string")
	}
	//log.WithField("identity", string(identityString)).Debug("Getting identity to (re)create context")

	base64Identity := base64.StdEncoding.EncodeToString(identityBytes)
	log.WithField("identity_base64", base64Identity).Debug("Using a base64encoded identity")

	// add the new identity to the context and create ctxServices with that context
	ctx = common.SetOriginalIdentity(ctx, base64Identity)
	//ctxServices := dependencies.ServicesFromContext(ctx)
	// TODO: consider a bitwise& param to only add needed ctxServices

	// temporarily using some Marshal/Unmarshal conjuring to move our future EDA image back to models.Image world
	var image *models.Image
	imageString, jsonErr := json.Marshal(ev.NewImage)
	if jsonErr != nil {
		log.Error("Error marshaling the image")
	}
	jsonErr = json.Unmarshal(imageString, &image)

	log := log.WithFields(log.Fields{
		"requestId": image.RequestID,
		"accountId": image.Account,
		"orgID":     image.OrgID,
	})

	// call the service-based CreateImage
	// TODO: port it all to EDA
	imageService := services.NewImageService(ctx, log)
	err = imageService.ProcessImage(image)
	// TODO: send an event with the status here
	//ev.post()

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
