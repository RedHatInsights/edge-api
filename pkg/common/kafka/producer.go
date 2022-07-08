package kafkacommon

import (
	"fmt"
	"sync"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/redhatinsights/edge-api/config"
	log "github.com/sirupsen/logrus"
)

var lock = &sync.Mutex{}

var singleInstance *kafka.Producer

// GetProducerInstance returns a kafka producer instance
func GetProducerInstance() *kafka.Producer {
	if singleInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		if singleInstance == nil && clowder.IsClowderEnabled() {
			cfg := config.Get()
			brokers := make([]clowder.BrokerConfig, len(cfg.KafkaConfig.Brokers))
			for i, b := range cfg.KafkaConfig.Brokers {
				brokers[i] = b
			}
			p, err := kafka.NewProducer(&kafka.ConfigMap{
				"bootstrap.servers": fmt.Sprintf("%s:%d", brokers[0].Hostname, *brokers[0].Port),
				"sasl.mechanisms":   brokers[0].Sasl.SaslMechanism,
				"security.protocol": brokers[0].Sasl.SecurityProtocol,
				"sasl.username":     brokers[0].Sasl.Username,
				"sasl.password":     brokers[0].Sasl.Password})
			if err != nil {
				log.WithField("error", err).Error("Failed to create producer")
				return nil
			}
			singleInstance = p
		}
	}
	return singleInstance
}

// ProduceEvent is a helper for the kafka producer
func ProduceEvent(requestedTopic, recordKey string, edgeEventMessage []byte) error {
	producer := GetProducerInstance()
	realTopic := GetTopic(requestedTopic)
	err := producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &realTopic, Partition: kafka.PartitionAny},
		Key:            []byte(recordKey),
		Value:          edgeEventMessage,
	}, nil)
	if err != nil {
		return err
	}
	return nil
}
