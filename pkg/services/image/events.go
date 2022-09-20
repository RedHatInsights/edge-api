// FIXME: golangci-lint
// nolint:revive
package image

// EdgeMgmtImageEventInterface is the interface for the image microservice(s)
type EdgeMgmtImageEventInterface interface {
	Consume() error // handles the execution of code against the data in the event
}
