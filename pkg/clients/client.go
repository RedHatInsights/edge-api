// FIXME: golangci-lint
// nolint:revive
package clients

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	l "github.com/redhatinsights/edge-api/logger"
	"net/http"
	"os"
	"time"

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

func ConfigureHttpClient(cfg config.EdgeConfig) *http.Client {
	if cfg.TlsCAPath != "" {
		rootCAs, _ := x509.SystemCertPool()
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}

		certs, err := os.ReadFile(cfg.TlsCAPath)
		if err != nil {
			l.LogErrorAndPanic("Failed to append CA to RootCAs", err)
		}

		if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
			log.Info("No certs appended, using system certs only")
		}

		// disable "G402 (CWE-295): TLS MinVersion too low. (Confidence: HIGH, Severity: HIGH)"
		// #nosec G402
		httpConfig := &tls.Config{
			InsecureSkipVerify: false,
			RootCAs:            rootCAs,
		}
		httpTransport := &http.Transport{TLSClientConfig: httpConfig}
		client := &http.Client{Transport: httpTransport,
			Timeout: time.Second * cfg.HTTPClientTimeout,
		}

		return client
	} else {
		return &http.Client{Timeout: time.Second * cfg.HTTPClientTimeout}
	}
}
