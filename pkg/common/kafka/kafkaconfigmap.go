// FIXME: golangci-lint
// nolint:errcheck,revive
// Package kafkacommon contains all common kafka functions
package kafkacommon

import (
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/kafka"

	"github.com/redhatinsights/edge-api/config"
)

// KafkaConfigMapServiceInterface is the interface that defines the config map service
type KafkaConfigMapServiceInterface interface {
	GetKafkaProducerConfigMap() kafka.ConfigMap
	GetKafkaConsumerConfigMap(consumerGroup string) kafka.ConfigMap
}

// KafkaConfigMapService is the config map service
type KafkaConfigMapService struct {
}

// NewKafkaConfigMapService returns a new service
func NewKafkaConfigMapService() KafkaConfigMapServiceInterface {
	return &KafkaConfigMapService{}
}

// getKafkaCommonConfigMap returns the kafka configMap common to producer and consumer
func (k *KafkaConfigMapService) getKafkaCommonConfigMap() kafka.ConfigMap {
	cfg := config.Get()
	kafkaConfigMap := kafka.ConfigMap{}

	// use the first kafka broker from config
	if cfg.KafkaBroker != nil {
		kafkaConfigMap.SetKey("bootstrap.servers", fmt.Sprintf("%s:%d", cfg.KafkaBroker.Hostname, *cfg.KafkaBroker.Port))
		var securityProtocol string
		if cfg.KafkaBroker.SecurityProtocol != nil {
			securityProtocol = *cfg.KafkaBroker.SecurityProtocol
		}
		if cfg.KafkaBrokerCaCertPath != "" {
			kafkaConfigMap.SetKey("ssl.ca.location", cfg.KafkaBrokerCaCertPath)
		}
		if cfg.KafkaBroker.Authtype != nil && *cfg.KafkaBroker.Authtype == "sasl" && cfg.KafkaBroker.Sasl != nil {
			if cfg.KafkaBroker.Sasl.SaslMechanism != nil {
				kafkaConfigMap.SetKey("sasl.mechanisms", *cfg.KafkaBroker.Sasl.SaslMechanism)
			}
			if cfg.KafkaBroker.Sasl.Username != nil {
				kafkaConfigMap.SetKey("sasl.username", *cfg.KafkaBroker.Sasl.Username)
			}
			if cfg.KafkaBroker.Sasl.Password != nil {
				kafkaConfigMap.SetKey("sasl.password", *cfg.KafkaBroker.Sasl.Password)
			}
			if securityProtocol == "" && cfg.KafkaBroker.Sasl.SecurityProtocol != nil && *cfg.KafkaBroker.Sasl.SecurityProtocol != "" { // nolint: staticcheck
				// seems we still in transition period and no security protocol was defined in parent
				// set it from sasl config
				securityProtocol = *cfg.KafkaBroker.Sasl.SecurityProtocol // nolint: staticcheck
			}
		}
		if securityProtocol != "" {
			kafkaConfigMap.SetKey("security.protocol", securityProtocol)
		}

	}
	return kafkaConfigMap
}

// GetKafkaProducerConfigMap returns the correct kafka auth based on the environment and given config
func (k *KafkaConfigMapService) GetKafkaProducerConfigMap() kafka.ConfigMap {
	cfg := config.Get()
	kafkaConfigMap := k.getKafkaCommonConfigMap()

	kafkaConfigMap.SetKey("request.required.acks", cfg.KafkaRequestRequiredAcks)
	kafkaConfigMap.SetKey("message.send.max.retries", cfg.KafkaMessageSendMaxRetries)
	kafkaConfigMap.SetKey("retry.backoff.ms", cfg.KafkaRetryBackoffMs)

	return kafkaConfigMap
}

// GetKafkaConsumerConfigMap returns the correct kafka auth based on the environment and given config
func (k *KafkaConfigMapService) GetKafkaConsumerConfigMap(consumerGroup string) kafka.ConfigMap {
	cfg := config.Get()
	kafkaConfigMap := k.getKafkaCommonConfigMap()
	kafkaConfigMap.SetKey("group.id", consumerGroup)

	if cfg.KafkaBrokers != nil {
		kafkaConfigMap.SetKey("broker.address.family", "v4")
		kafkaConfigMap.SetKey("session.timeout.ms", 6000)
		kafkaConfigMap.SetKey("auto.offset.reset", "earliest")
	}
	return kafkaConfigMap
}
