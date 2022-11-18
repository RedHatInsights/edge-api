// FIXME: golangci-lint
// nolint:revive
package image

import (
	"context"
)

// EdgeMgmtImageEventInterface is the interface for the image microservice(s)
type EdgeMgmtImageEventInterface interface {
	Consume(ctx context.Context, imgService imageService) error // handles the execution of code against the data in the event
}
