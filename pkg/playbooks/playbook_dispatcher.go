package playbooks

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

type DispatcherPayload struct {
	Recipient   string
	PlaybookURL string
	Account     string
}

func ExecuteDispatcher(payload DispatcherPayload) (string, error) {
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

	headers := common.GetOutgoingHeaders(req)
	log.Infof("ExecuteDispatcher:: cfg.PlaybookDispatcherConfig:: %#v", cfg.PlaybookDispatcherConfig)
	req.Header.Add("Authorization", "PSK "+cfg.PlaybookDispatcherConfig.PSK)
	for key, value := range headers {
		log.Infof("Playbook dispatcher headers: %#v, %#v", key, value)
		req.Header.Add(key, value)
	}

	log.Infof("ExecuteDispatcher:: req.Header:: %#v", req.Header)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("Error Code:: Playbook dispatcher: %#v", resp)
		log.Errorf("Error:: Playbook dispatcher: %#v", err)
		log.Errorf("Error:: Playbook dispatcher: %#v", err.Error())
		return models.DispatchRecordStatusError, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Errorf("error calling playbook dispatcher, got status code %d and body %s", resp.StatusCode, body)
		return models.DispatchRecordStatusError, err
	}
	log.Infof("::executeDispatcher::END")
	return models.DispatchRecordStatusCreated, nil
}
