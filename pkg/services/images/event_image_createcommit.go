package images

import (
	log "github.com/sirupsen/logrus"
)

// create_image event definitions

// EdgeMgmtImageCreateCommitEvent defines an existing image to be updated
type EdgeMgmtImageCreateCommitEvent struct {
	EdgeMgmtImageCreateEvent
}

// create_image event methods

// Consume executes code against the data in the event
func (ev EdgeMgmtImageCreateCommitEvent) Consume() error {
	ev.pre()

	log.WithField("image_name", ev.NewImage.Name).Debug("Handle EdgeMgmtImageCreateCommitEvent")

	ev.post()

	return nil
}

// run this at the beginning of Handle()
func (ev EdgeMgmtImageCreateCommitEvent) pre() error {
	log.Debug("EdgeMgmtImageCreateCommitEvent running the pre()")
	identity := ev.ConsoleSchema.GetIdentity()
	log.WithField("account", identity.AccountNumber).Debug("Pre EdgeMgmtImageCreateCommitEvent")

	return nil
}

// run this at the end of Handle()
func (ev EdgeMgmtImageCreateCommitEvent) post() error {
	log.Debug("EdgeMgmtImageCreateCommitEvent running the post()")
	identity := ev.ConsoleSchema.GetIdentity()
	log.WithField("account", identity.AccountNumber).Debug("Post EdgeMgmtImageCreateCommitEvent")

	return nil
}
