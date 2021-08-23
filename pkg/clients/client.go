package clients

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/platform-go-middlewares/request_id"
)

// GetOutgoingHeaders returns Red Hat Insights headers from our request to use it
// in other request that will be used when communicating in Red Hat Insights services
func GetOutgoingHeaders(ctx context.Context) map[string]string {
	requestId := request_id.GetReqID(ctx)
	headers := map[string]string{"x-rh-insights-request-id": requestId}
	if config.Get().Auth {
		xhrid := identity.Get(ctx)
		identityHeaders, err := json.Marshal(xhrid)
		if err != nil {
			log.Fatal(err)
			panic(err)
		}
		headers["x-rh-identity"] = base64.StdEncoding.EncodeToString(identityHeaders)
	}
	return headers
}
