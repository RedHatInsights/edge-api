package unleash // nolint:gofmt,goimports,revive

import (
	unleashclient "github.com/Unleash/unleash-client-go/v4"
	log "github.com/sirupsen/logrus"
)

// nolint:revive // EdgeListener is an implementation of all of the listener interfaces that simply logs
// debug info to stdout. It is meant for logging purposes.
type EdgeListener struct{}

// OnError prints out errors.
func (l EdgeListener) OnError(err error) { // nolint:gofmt,goimports,revive
	log.Warnf("Unleash: %+v\n", err.Error())
}

// OnWarning prints out warning.
func (l EdgeListener) OnWarning(warning error) { // nolint:gofmt,goimports,revive
	log.Warnf("Unleash: %+v\n", warning.Error())
}

// OnReady prints to the console when the repository is ready.
func (l EdgeListener) OnReady() { // nolint:gofmt,goimports,revive
	log.Infof("Unleash: READY\n")
}

// OnCount prints to the console when the feature is queried.
// This is done every 5 seconds, too much for edge-api
func (l EdgeListener) OnCount(name string, enabled bool) { // nolint:gofmt,goimports,revive
}

// OnSent prints to the console when the server has uploaded metrics.
// This is done on every request, too much for edge-api
func (l EdgeListener) OnSent(payload unleashclient.MetricsData) { // nolint:gofmt,goimports,revive
}

// OnRegistered prints to the console when the client has registered.
func (l EdgeListener) OnRegistered(payload unleashclient.ClientData) { // nolint:gofmt,goimports,revive
	log.Debugf("Registered: %+v\n", payload)
}
