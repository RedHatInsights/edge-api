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
		err := kafkaConfigMap.SetKey("bootstrap.servers", fmt.Sprintf("%s:%d", cfg.KafkaBrokers[0].Hostname, *cfg.KafkaBrokers[0].Port))
		if err != nil {
			log.WithField("bootstrap.servers", fmt.Sprintf("%s:%d", cfg.KafkaBrokers[0].Hostname, *cfg.KafkaBrokers[0].Port)).Error("Error setting Kafka key")
		}

		if cfg.KafkaBrokers[0].Sasl != nil {
			err = kafkaConfigMap.SetKey("sasl.mechanisms", cfg.KafkaBrokers[0].Sasl.SaslMechanism)
			if err != nil {
				log.WithField("sasl.mechanisms", cfg.KafkaBrokers[0].Sasl.SaslMechanism).Error("Error setting Kafka key")
			}

			err = kafkaConfigMap.SetKey("security.protocol", cfg.KafkaBrokers[0].Sasl.SecurityProtocol)
			if err != nil {
				log.WithField("security.protocol", cfg.KafkaBrokers[0].Sasl.SecurityProtocol).Error("Error setting Kafka key")
			}

			err = kafkaConfigMap.SetKey("sasl.username", cfg.KafkaBrokers[0].Sasl.Username)
			if err != nil {
				log.WithField("sasl.username", cfg.KafkaBrokers[0].Sasl.Username).Error("Error setting Kafka key")
			}

			err = kafkaConfigMap.SetKey("sasl.password", cfg.KafkaBrokers[0].Sasl.Password)
			if err != nil {
				log.WithField("sasl.password", cfg.KafkaBrokers[0].Sasl.Password).Error("Error setting Kafka key")
			}
		}
	}
	return kafkaConfigMap
}

// GetKafkaConsumerConfigMap returns the correct kafka auth based on the environment and given config
func GetKafkaConsumerConfigMap(consumerGroup string) kafka.ConfigMap {
	cfg := config.Get()
	kafkaConfigMap := GetKafkaProducerConfigMap()

	if cfg.KafkaBrokers != nil {
		err = kafkaConfigMap.SetKey("broker.address.family", "v4")
		if err != nil {
			log.WithField("broker.address.family", "v4").Error("Error setting Kafka key")
		}

		err = kafkaConfigMap.SetKey("group.id", consumerGroup)
		if err != nil {
			log.WithField("group.id", consumerGroup).Error("Error setting Kafka key")
		}

		err = kafkaConfigMap.SetKey("session.timeout.ms", 6000)
		if err != nil {
			log.WithField("session.timeout.ms", 6000).Error("Error setting Kafka key")
		}

		err = kafkaConfigMap.SetKey("auto.offset.reset", "earliest")
		if err != nil {
			log.WithField("auto.offset.reset", "earliest").Error("Error setting Kafka key")
		}
	}
	return kafkaConfigMap
}
