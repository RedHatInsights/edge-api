package clients

import (
	"context"
	"github.com/pkg/errors"
	"github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/redhatinsights/edge-api/config"
)

// GetOutgoingHeaders returns Red Hat Insights headers from our request to use it
// in other request that will be used when communicating in Red Hat Insights services
func GetOutgoingHeaders(ctx context.Context) map[string]string {
	requestID := request_id.GetReqID(ctx)
	headers := map[string]string{"x-rh-insights-request-id": requestID}
	if config.Get().Auth {
		xrhid := ctx.Value("xRhIdentity")
		if xrhid == nil {
			logger.LogErrorAndPanic("Error getting request ID", errors.New("Error getting x-rh-identity"))
		}
		headers["x-rh-identity"] = xrhid.(string)
	}
	return headers
}
