package kafkacommon

import (
	"time"

	"encoding/json"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// Consumer is an interface for a kafka consumer, it matches the confluent consumer type definition
type Consumer interface {
	Assign(partitions []kafka.TopicPartition) (err error)
	Assignment() (partitions []kafka.TopicPartition, err error)
	AssignmentLost() bool
	Close() (err error)
	Commit() ([]kafka.TopicPartition, error)
	CommitMessage(m *kafka.Message) ([]kafka.TopicPartition, error)
	CommitOffsets(offsets []kafka.TopicPartition) ([]kafka.TopicPartition, error)
	Committed(partitions []kafka.TopicPartition, timeoutMs int) (offsets []kafka.TopicPartition, err error)
	Events() chan kafka.Event
	GetConsumerGroupMetadata() (*kafka.ConsumerGroupMetadata, error)
	GetMetadata(topic *string, allTopics bool, timeoutMs int) (*kafka.Metadata, error)
	GetRebalanceProtocol() string
	GetWatermarkOffsets(topic string, partition int32) (low, high int64, err error)
	IncrementalAssign(partitions []kafka.TopicPartition) (err error)
	IncrementalUnassign(partitions []kafka.TopicPartition) (err error)
	OffsetsForTimes(times []kafka.TopicPartition, timeoutMs int) (offsets []kafka.TopicPartition, err error)
	Pause(partitions []kafka.TopicPartition) (err error)
	Poll(timeoutMs int) (event kafka.Event)
	Position(partitions []kafka.TopicPartition) (offsets []kafka.TopicPartition, err error)
	QueryWatermarkOffsets(topic string, partition int32, timeoutMs int) (low, high int64, err error)
	ReadMessage(timeout time.Duration) (*kafka.Message, error)
	Resume(partitions []kafka.TopicPartition) (err error)
	Seek(partition kafka.TopicPartition, timeoutMs int) error
	SetOAuthBearerToken(oauthBearerToken kafka.OAuthBearerToken) error
	SetOAuthBearerTokenFailure(errstr string) error
	StoreMessage(m *kafka.Message) (storedOffsets []kafka.TopicPartition, err error)
	StoreOffsets(offsets []kafka.TopicPartition) (storedOffsets []kafka.TopicPartition, err error)
	String() string
	Subscription() (topics []string, err error)
	Unassign() (err error)
}

// ConsumerServiceInterface is the interface that defines the consumer service
type ConsumerServiceInterface interface {
	ConsumeMessage(event models.CRCCloudEvent) error
}

// ConsumeService is the consumer service for edge
type ConsumeService struct {
	Topic          TopicServiceInterface
	KafkaConfigMap KafkaConfigMapServiceInterface
}

// ConsumeMessage is a helper for the kafka consumer
func (c *ConsumeService) ConsumeMessage(event models.CRCCloudEvent) error {
	log.Debug("Consume an event")

	cfg := config.Get()
	cfgBytes, _ := json.Marshal(cfg)
	crcEvent := &event.Data

	// unmarshal the event from a string
	err := json.Unmarshal(cfgBytes, crcEvent)
	if err != nil {
		log.Error("Unmarshal CRCCloudEvent failed")
		return err
	}
	return nil
}
