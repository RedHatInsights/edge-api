// FIXME: golangci-lint
// nolint:revive
package kafkacommon

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
)

var lock = &sync.Mutex{}

var singleInstance Producer

// Producer is an interface for a kafka producer, it matches the confluent producer type definition
type Producer interface {
	Close()
	Events() chan kafka.Event
	Flush(timeoutMs int) int
	GetFatalError() error
	GetMetadata(topic *string, allTopics bool, timeoutMs int) (*kafka.Metadata, error)
	Len() int
	OffsetsForTimes(times []kafka.TopicPartition, timeoutMs int) (offsets []kafka.TopicPartition, err error)
	Produce(msg *kafka.Message, deliveryChan chan kafka.Event) error
	ProduceChannel() chan *kafka.Message
	QueryWatermarkOffsets(topic string, partition int32, timeoutMs int) (low, high int64, err error)
	SetOAuthBearerToken(oauthBearerToken kafka.OAuthBearerToken) error
	SetOAuthBearerTokenFailure(errstr string) error
	String() string
	TestFatalError(code kafka.ErrorCode, str string) kafka.ErrorCode
}

// ProducerServiceInterface is an interface that defines the producer service
type ProducerServiceInterface interface {
	GetProducerInstance() Producer
	ProduceEvent(requestedTopic, recordKey string, event models.CRCCloudEvent) error
}

// ProducerService is the producer service for edge
type ProducerService struct {
	Topic          TopicServiceInterface
	KafkaConfigMap KafkaConfigMapServiceInterface
}

// NewProducerService returns a new service
func NewProducerService() ProducerServiceInterface {
	return &ProducerService{
		Topic:          NewTopicService(),
		KafkaConfigMap: NewKafkaConfigMapService(),
	}
}

// GetProducerInstance returns a kafka producer instance
func (p *ProducerService) GetProducerInstance() Producer {
	log.Debug("Getting the producer instance")
	if singleInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		cfg := config.Get()
		if cfg.KafkaBrokers != nil {
			log.WithFields(log.Fields{"broker": cfg.KafkaBrokers[0].Hostname,
				"port": *cfg.KafkaBrokers[0].Port}).Debug("Creating a new producer")

			kafkaConfigMap := p.KafkaConfigMap.GetKafkaProducerConfigMap()
			fmt.Println("KafkaConfigMap: ", kafkaConfigMap)
			p, err := kafka.NewProducer(&kafkaConfigMap)
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
func (p *ProducerService) ProduceEvent(requestedTopic, recordKey string, event models.CRCCloudEvent) error {
	log.Debug("Producing an event")
	producer := p.GetProducerInstance()
	if producer == nil {
		log.Error("Failed to get the producer instance")
	}
	realTopic, err := p.Topic.GetTopic(requestedTopic)
	if err != nil {
		log.WithField("error", err).Error("Unable to lookup requested topic name")
	}

	// marshal the event into a string
	edgeEventMessage, err := json.Marshal(event)
	if err != nil {
		log.Error("Marshal CRCCloudEvent failed")
	}
	log.WithField("event", string(edgeEventMessage)).Debug("Debug CRCCloudEvent contents")

	err = producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &realTopic, Partition: kafka.PartitionAny},
		Key:            []byte(recordKey),
		Value:          edgeEventMessage,
	}, nil)
	if err != nil {
		log.WithField("error", err.Error()).Debug("Failed to produce the event")
		return err
	}

	return nil
}

// UnsetProducer sets the producer singleton to nil
func (p *ProducerService) UnsetProducer() {
	singleInstance = nil
}
