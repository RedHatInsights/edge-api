package kafkacommon

import (
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/redhatinsights/edge-api/config"
)

const (
	// FleetmgntImageBuild topic name
	FleetmgntImageBuild string = "platform.edge.fleetmgmt.image-build"
)

// GetTopic takes the requested kafka topic and returns the topic actually created
func GetTopic(requested string) string {
	ret := ""
	if clowder.IsClowderEnabled() {
		cfg := config.Get()
		topics := cfg.KafkaConfig.Topics
		for _, topic := range topics {
			if topic.RequestedName == requested {
				ret = topic.Name
			}
		}
	}
	return ret
}
