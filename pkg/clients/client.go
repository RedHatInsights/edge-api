// FIXME: golangci-lint
// nolint:revive
package clients

import (
	"context"

	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/platform-go-middlewares/request_id"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
)

// GetOutgoingHeaders returns Red Hat Insights headers from our request to use it
// in other request that will be used when communicating in Red Hat Insights services
func GetOutgoingHeaders(ctx context.Context) map[string]string {
	requestID := request_id.GetReqID(ctx)
	headers := map[string]string{"x-rh-insights-request-id": requestID}
	if config.Get().Auth {
		xrhid, err := common.GetOriginalIdentity(ctx)

		if err != nil {
			log.Error("Error getting x-rh-identity")
		} else {
			headers["x-rh-identity"] = xrhid
		}
	}
	return headers
}
