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
	url string
	psk string
}

// InitClient initializes the client for Image Builder
func InitClient(ctx context.Context) *Client {
	cfg := config.Get()
	return &Client{ctx: ctx, url: cfg.PlaybookDispatcherConfig.URL, psk: cfg.PlaybookDispatcherConfig.PSK}
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
	payloadAry := [1]DispatcherPayload{payload}

	payloadBuf := new(bytes.Buffer)
	_ = json.NewEncoder(payloadBuf).Encode(payloadAry)
	log.Infof("::executeDispatcher::BEGIN")
	fullURL := c.url + "/internal/dispatch"
	log.Infof("Requesting url: %s\n", fullURL)
	req, _ := http.NewRequest("POST", fullURL, payloadBuf)

	req.Header.Add("Content-Type", "application/json")

	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		log.Infof("Playbook dispatcher headers: %#v, %#v", key, value)
		req.Header.Add(key, value)
	}
	req.Header.Add("Authorization", fmt.Sprintf("PSK %s", c.psk))

	log.Infof("ExecuteDispatcher:: req.Header:: %#v", req.Header)
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Errorf("Error Code:: Playbook dispatcher: %#v", resp)
		log.Errorf("Error:: Playbook dispatcher: %#v", err)
		log.Errorf("Error:: Playbook dispatcher: %#v", err.Error())
		return nil, err
	}

	if resp.StatusCode != http.StatusMultiStatus {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Errorf("error calling playbook dispatcher, got status code %d and body %s", resp.StatusCode, body)
		return nil, err
	}
	log.Infof("::executeDispatcher::END")
	log.Infof("::executeDispatcher::response: %#v", resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("executeDispatcher: %#v", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	var playbookResponse []Response
	json.Unmarshal([]byte(body), &playbookResponse)
	log.Infof("::executeDispatcher::playbookResponse: %#v", playbookResponse)
	return playbookResponse, nil
}
