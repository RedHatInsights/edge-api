// FIXME: golangci-lint
// nolint:revive
package kafkacommon

import (
	"github.com/redhatinsights/edge-api/config"
	log "github.com/sirupsen/logrus"
)

// TopicServiceInterface is the interface for the service
type TopicServiceInterface interface {
	GetTopic(requested string) (string, error)
}

// TopicService is the service struct
type TopicService struct {
}

const (
	// TopicFleetmgmtImageBuild topic name
	TopicFleetmgmtImageBuild string = "platform.edge.fleetmgmt.image-build"
	// TopicFleetmgmtImageISOBuild topic name
	TopicFleetmgmtImageISOBuild string = "platform.edge.fleetmgmt.image-iso-build"

	// TopicPlaybookDispatcherRuns external topic for playbook dispatcher results
	TopicPlaybookDispatcherRuns string = "platform.playbook-dispatcher.runs"
	// TopicInventoryEvents external topic for hosted inventory events
	TopicInventoryEvents string = "platform.inventory.events"
)

// TopicNotFoundError indicates the account was nil
type TopicNotFoundError struct{}

func (e *TopicNotFoundError) Error() string {
	return "Topic is not found in config"
}

// NewTopicService returns a new service
func NewTopicService() TopicServiceInterface {
	return &TopicService{}
}

// GetTopic takes the requested kafka topic and returns the topic actually created
func (t *TopicService) GetTopic(requested string) (string, error) {
	cfg := config.Get()
	if cfg.KafkaConfig != nil {
		topics := cfg.KafkaConfig.Topics
		for _, topic := range topics {
			log.WithField("requestedName", requested).Debug("looking up actual topic")
			if topic.RequestedName == requested {
				log.WithFields(log.Fields{"requestedName": requested, "Name": topic.Name}).Debug("Found the actual topic name")

				return topic.Name, nil
			}
		}
	}
	err := new(TopicNotFoundError)
	log.WithFields(log.Fields{"requestedName": requested, "error": err}).Error("Actual topic not found. Returning the requested topic name")

	return requested, err
}
