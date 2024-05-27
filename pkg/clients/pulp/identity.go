package pulp

import (
	"context"
	"net/http"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients"
	"github.com/redhatinsights/platform-go-middlewares/v2/request_id"
)

func addAuthenticationHeader(ctx context.Context, req *http.Request) error {
	c := config.Get()

	// always add correlation id
	req.Header.Add("Correlation-Id", request_id.GetReqID(ctx))

	// add service account header if we are using oauth2 (dev setup)
	if c.PulpOauth2URL != "" && c.PulpOauth2ClientID != "" && c.PulpOauth2ClientSecret != "" {
		err := clients.AddOAuth2IdentityHeader(ctx, req, c.PulpOauth2URL, c.PulpOauth2ClientID, c.PulpOauth2ClientSecret)
		if err != nil {
			return err
		}
	}

	// add service account mock header if we are
	clients.AddBasicCredentialsHeader(ctx, req)

	return nil
}
