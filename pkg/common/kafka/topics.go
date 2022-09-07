package kafkacommon

import (
	"github.com/redhatinsights/edge-api/config"
	log "github.com/sirupsen/logrus"
)

const (
	// TopicFleetmgmtImageBuild topic name
	TopicFleetmgmtImageBuild string = "platform.edge.fleetmgmt.image-build"
	// TopicFleetmgmtImageISOBuild topic name
	TopicFleetmgmtImageISOBuild string = "platform.edge.fleetmgmt.image-iso-build"
)

// TopicNotFoundError indicates the account was nil
type TopicNotFoundError struct{}

func (e *TopicNotFoundError) Error() string {
	return "Topic is not found in config"
}

// GetTopic takes the requested kafka topic and returns the topic actually created
func GetTopic(requested string) (string, error) {
	cfg := config.Get()
	if cfg.KafkaConfig != nil {
		log.Debug("looking up actual topic")
		topics := cfg.KafkaConfig.Topics
		for _, topic := range topics {
			if topic.RequestedName == requested {
				log.WithField("Name", topic.Name).Debug("Found the actual topic name")

				return topic.Name, nil
			}
		}
	}
	err := new(TopicNotFoundError)
	log.WithFields(log.Fields{"requested_name": requested, "error": err}).Error("Actual topic not found. Returning the requested topic name")

	return requested, err
}
