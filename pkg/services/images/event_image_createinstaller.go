package images

import (
	log "github.com/sirupsen/logrus"
)

// create_image event definitions

// EdgeMgmtImageCreateInstallerEvent defines an existing image to be updated
type EdgeMgmtImageCreateInstallerEvent struct {
	EdgeMgmtImageCreateEvent
}

// create_installer event methods

// Consume executes code against the data in the event
func (ev EdgeMgmtImageCreateInstallerEvent) Consume() error {
	ev.pre()

	log.WithField("image_name", ev.NewImage.Name).Debug("Handle EdgeMgmtImageCreateInstallerEvent")

	ev.post()

	return nil
}

// run this at the beginning of Handle()
func (ev EdgeMgmtImageCreateInstallerEvent) pre() error {
	log.Debug("EdgeMgmtImageCreateInstallerEvent running the pre()")
	identity := ev.ConsoleSchema.GetIdentity()
	log.WithField("account", identity.AccountNumber).Debug("Pre EdgeMgmtImageCreateInstallerEvent")

	return nil
}

// run this at the end of Handle()
func (ev EdgeMgmtImageCreateInstallerEvent) post() error {
	log.Debug("EdgeMgmtImageCreateInstallerEvent running the post()")
	identity := ev.ConsoleSchema.GetIdentity()
	log.WithField("account", identity.AccountNumber).Debug("Post EdgeMgmtImageCreateInstallerEvent")

	return nil
}
