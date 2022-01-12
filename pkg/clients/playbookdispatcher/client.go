package playbookdispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients"
	log "github.com/sirupsen/logrus"
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
type DispatcherPayload struct {
	Recipient   string `json:"recipient"`
	PlaybookURL string `json:"url"`
	Account     string `json:"account"`
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
	url := c.url + "/internal/dispatch"
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

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		c.log.WithFields(log.Fields{
			"statusCode": res.StatusCode,
			"error":      err,
		}).Error("PlaybookDispatcher ExecuteDispatcher Request Error")
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
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
		c.log.Error("Error while trying to unmarshal ", &playbookResponse)
	}
	return playbookResponse, nil
}
