package kafkacommon

import (
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/redhatinsights/edge-api/config"
)

// GetKafkaProducerConfigMap returns the correct kafka auth based on the environment and given config
func GetKafkaProducerConfigMap() kafka.ConfigMap {
	cfg := config.Get()
	kafkaConfigMap := kafka.ConfigMap{}

	if cfg.KafkaBrokers != nil {
		// FIXME: golangci-lint
		kafkaConfigMap.SetKey("bootstrap.servers", fmt.Sprintf("%s:%d", cfg.KafkaBrokers[0].Hostname, *cfg.KafkaBrokers[0].Port)) // nolint:errcheck,revive
		if cfg.KafkaBrokers[0].Sasl != nil {
			// FIXME: golangci-lint
			kafkaConfigMap.SetKey("sasl.mechanisms", *cfg.KafkaBrokers[0].Sasl.SaslMechanism) // nolint:errcheck,revive
			// FIXME: golangci-lint
			kafkaConfigMap.SetKey("security.protocol", *cfg.KafkaBrokers[0].Sasl.SecurityProtocol) // nolint:errcheck,revive
			// FIXME: golangci-lint
			kafkaConfigMap.SetKey("sasl.username", *cfg.KafkaBrokers[0].Sasl.Username) // nolint:errcheck,revive
			// FIXME: golangci-lint
			kafkaConfigMap.SetKey("sasl.password", *cfg.KafkaBrokers[0].Sasl.Password) // nolint:errcheck,revive
		}
	}
	return kafkaConfigMap
}

// GetKafkaConsumerConfigMap returns the correct kafka auth based on the environment and given config
func GetKafkaConsumerConfigMap(consumerGroup string) kafka.ConfigMap {
	cfg := config.Get()
	kafkaConfigMap := GetKafkaProducerConfigMap()
	// FIXME: golangci-lint
	kafkaConfigMap.SetKey("group.id", consumerGroup) // nolint:errcheck,revive

	if cfg.KafkaBrokers != nil {
		kafkaConfigMap.SetKey("broker.address.family", "v4")
		kafkaConfigMap.SetKey("session.timeout.ms", 6000)
		kafkaConfigMap.SetKey("auto.offset.reset", "earliest")
	}
	return kafkaConfigMap
}
