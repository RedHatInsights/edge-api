package playbooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/common"
	log "github.com/sirupsen/logrus"
)

type DispatcherPayload struct {
	Recipient   string
	PlaybookURL string
	Account     string
}

func ExecuteDispatcher(r *http.Request, payload DispatcherPayload) {
	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(payload)
	cfg := config.Get()
	log.Debugf("::executeDispatcher")
	url := cfg.PlaybookDispatcherConfig.URL

	log.Infof("Requesting url: %s\n", url)
	req, _ := http.NewRequest("POST", url, payloadBuf)

	req.Header.Add("Content-Type", "application/json")

	headers := common.GetOutgoingHeaders(r)
	for key, value := range headers {
		req.Header.Add(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error(fmt.Printf("Playbook dispatcher: %s", err))
		return
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Errorf("error requesting inventory, got status code %d and body %s", resp.StatusCode, body)
		return
	}
}
