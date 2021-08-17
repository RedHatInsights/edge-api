package playbooks

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/redhatinsights/edge-api/config"
	log "github.com/sirupsen/logrus"
)

type DispatcherPayload struct {
	Recipient   string `json:"recipient"`
	PlaybookURL string `json:"url"`
	Account     string `json:"account"`
}

type PlaybookDispatcherResponse struct {
	StatusCode           int    `json:"code"`
	PlaybookDispatcherID string `json:"id"`
}

func ExecuteDispatcher(payload DispatcherPayload, headers map[string]string) ([]PlaybookDispatcherResponse, error) {
	payloadAry := [1]DispatcherPayload{payload}

	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(payloadAry)
	cfg := config.Get()
	log.Infof("::executeDispatcher::BEGIN")
	url := cfg.PlaybookDispatcherConfig.URL
	fullURL := url + "/internal/dispatch"
	log.Infof("Requesting url: %s\n", fullURL)
	req, _ := http.NewRequest("POST", fullURL, payloadBuf)

	req.Header.Add("Content-Type", "application/json")
	log.Infof("ExecuteDispatcher:: cfg.PlaybookDispatcherConfig:: %#v", cfg.PlaybookDispatcherConfig)

	req.Header.Add("Authorization", "PSK "+cfg.PlaybookDispatcherConfig.PSK)
	req.Header.Add("x-rh-identity", headers["x-rh-identity"])
	req.Header.Add("x-rh-insights-request-id", headers["x-rh-insights-request-id"])

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

	var playbookResponse []PlaybookDispatcherResponse
	json.Unmarshal([]byte(body), &playbookResponse)
	log.Infof("::executeDispatcher::playbookResponse: %#v", playbookResponse)
	return playbookResponse, nil
}
