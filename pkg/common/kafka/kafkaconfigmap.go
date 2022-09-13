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
		kafkaConfigMap.SetKey("bootstrap.servers", fmt.Sprintf("%s:%d", cfg.KafkaBrokers[0].Hostname, *cfg.KafkaBrokers[0].Port))
		if cfg.KafkaBrokers[0].Sasl != nil {
			kafkaConfigMap.SetKey("sasl.mechanisms", *cfg.KafkaBrokers[0].Sasl.SaslMechanism)
			kafkaConfigMap.SetKey("security.protocol", *cfg.KafkaBrokers[0].Sasl.SecurityProtocol)
			kafkaConfigMap.SetKey("sasl.username", *cfg.KafkaBrokers[0].Sasl.Username)
			kafkaConfigMap.SetKey("sasl.password", *cfg.KafkaBrokers[0].Sasl.Password)
		}
	}
	return kafkaConfigMap
}

// GetKafkaConsumerConfigMap returns the correct kafka auth based on the environment and given config
func GetKafkaConsumerConfigMap(consumerGroup string) kafka.ConfigMap {
	cfg := config.Get()
	kafkaConfigMap := GetKafkaProducerConfigMap()
	kafkaConfigMap.SetKey("group.id", consumerGroup)

	if cfg.KafkaBrokers != nil {
		kafkaConfigMap.SetKey("broker.address.family", "v4")
		kafkaConfigMap.SetKey("session.timeout.ms", 6000)
		kafkaConfigMap.SetKey("auto.offset.reset", "earliest")
	}
	return kafkaConfigMap
}
