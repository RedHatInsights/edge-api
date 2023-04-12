// FIXME: golangci-lint
// nolint:errcheck,govet,revive
package playbookdispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/redhatinsights/edge-api/pkg/clients"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
)

// ClientInterface is an Interface to make requests to PlaybookDispatcher
type ClientInterface interface {
	ExecuteDispatcher(payload DispatcherPayload) ([]Response, error)
}

// Client is the implementation of an ClientInterface
type Client struct {
	ctx context.Context
	log *log.Entry
	url string
	psk string
}

// InitClient initializes the client for Image Builder
func InitClient(ctx context.Context, log *log.Entry) *Client {
	cfg := config.Get()
	return &Client{ctx: ctx, log: log, url: cfg.PlaybookDispatcherConfig.URL, psk: cfg.PlaybookDispatcherConfig.PSK}
}

// DispatcherPayload represents the payload sent to playbook dispatcher
// as per https://github.com/RedHatInsights/playbook-dispatcher/blob/master/schema/private.openapi.yaml
// RunInputV2
type DispatcherPayload struct {
	Recipient   string `json:"recipient"`
	PlaybookURL string `json:"url"`
	OrgID       string `json:"org_id"`
	// Principal is the Username of the user interacting with the service
	Principal string `json:"principal"`
	// Human readable name of the playbook run. Used to present the given playbook run in external systems (Satellite)
	PlaybookName string `json:"name"`
}

// Response represents the response retrieved by playbook dispatcher
type Response struct {
	StatusCode           int    `json:"code"`
	PlaybookDispatcherID string `json:"id"`
}

// ExecuteDispatcher executes a DispatcherPayload, sending it to playbook dispatcher
func (c *Client) ExecuteDispatcher(payload DispatcherPayload) ([]Response, error) {
	payloadBuf := new(bytes.Buffer)
	if err := json.NewEncoder(payloadBuf).Encode([1]DispatcherPayload{payload}); err != nil {
		return nil, err
	}
	// as per https://github.com/RedHatInsights/playbook-dispatcher/blob/master/schema/private.openapi.yaml
	url := c.url + "/internal/v2/dispatch"
	c.log.WithFields(log.Fields{
		"url":     url,
		"payload": payloadBuf.String(),
	}).Info("PlaybookDispatcher ExecuteDispatcher Request Started")
	req, _ := http.NewRequest("POST", url, payloadBuf)
	req.Header.Add("Content-Type", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	req.Header.Add("Authorization", fmt.Sprintf("PSK %s", c.psk))

	client := clients.ConfigureClientWithTLS(&http.Client{})
	res, err := client.Do(req)
	if err != nil {
		code := 500
		if res != nil {
			code = res.StatusCode
		}

		c.log.WithFields(log.Fields{
			"statusCode": code,
			"error":      err,
		}).Error("PlaybookDispatcher ExecuteDispatcher Request Error")
		return nil, err
	}

	body, err := io.ReadAll(res.Body)
	c.log.WithFields(log.Fields{
		"statusCode":   res.StatusCode,
		"responseBody": string(body),
		"error":        err,
	}).Info("PlaybookDispatcher ExecuteDispatcher Response")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMultiStatus {
		return nil, fmt.Errorf("error calling playbook dispatcher, got status code %d and body %s", res.StatusCode, body)
	}

	var playbookResponse []Response
	if err := json.Unmarshal([]byte(body), &playbookResponse); err != nil {
		c.log.WithField("response", &playbookResponse).Error("Error while trying to unmarshal PlaybookDispatcher ExecuteDispatcher Response")
		return nil, err
	}
	return playbookResponse, nil
}
