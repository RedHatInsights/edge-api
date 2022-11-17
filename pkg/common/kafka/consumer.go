package kafkacommon

import (
	"context"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
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
	Subscribe(topic string, rebalanceCb kafka.RebalanceCb) error
	SubscribeTopics(topics []string, rebalanceCb kafka.RebalanceCb) (err error)
	Subscription() (topics []string, err error)
	Unsubscribe() (err error)
	Unassign() (err error)
}

// ConsumerServiceInterface is the interface that defines the consumer service
type ConsumerServiceInterface interface {
	GetConsumer(groupID string) (Consumer, error)
}

// ConsumerService is the consumer service for edge
type ConsumerService struct {
	ctx            context.Context
	log            *log.Entry
	Topic          TopicServiceInterface
	KafkaConfigMap KafkaConfigMapServiceInterface
}

// NewConsumerService returns a new service
func NewConsumerService(ctx context.Context, log *log.Entry) ConsumerServiceInterface {
	return &ConsumerService{
		ctx:            ctx,
		log:            log,
		Topic:          NewTopicService(),
		KafkaConfigMap: NewKafkaConfigMapService(),
	}
}

// GetConsumer returns a kafka Consumer
func (s *ConsumerService) GetConsumer(groupID string) (Consumer, error) {
	kafkaConfigMap := s.KafkaConfigMap.GetKafkaConsumerConfigMap(groupID)
	c, err := kafka.NewConsumer(&kafkaConfigMap)

	if err != nil {
		log.WithField("error", err.Error()).Error("Failed to create consumer")
		return nil, err
	}

	return c, nil
}
