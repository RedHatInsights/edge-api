// FIXME: golangci-lint
// nolint:gocritic,govet,ineffassign,revive
package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	log "github.com/sirupsen/logrus"
)

// ConsumerService is the interface that takes care of our consumer implementation
type ConsumerService interface {
	Start()
	Close()
}

// KafkaConsumerService is the implementation of a consumer service based on Kafka topics
type KafkaConsumerService struct {
	Reader        *kafka.Consumer
	UpdateService UpdateServiceInterface
	DeviceService DeviceServiceInterface
	ImageService  ImageServiceInterface
	RetryMinutes  uint
	config        *clowder.KafkaConfig
	shuttingDown  bool
	mutex         sync.RWMutex
	topic         string
	consumer      func() error
}

// NewKafkaConsumerService gives a instance of the Kafka implementation of ConsumerService
func NewKafkaConsumerService(config *clowder.KafkaConfig, topic string) ConsumerService {
	if config == nil {
		return nil
	}

	actualTopic, err := kafkacommon.GetTopic(topic)
	if err != nil {
		log.WithField("error", err.Error()).Error("Error getting actual topic from requested topic")
	}

	// to consume messages
	s := &KafkaConsumerService{
		UpdateService: NewUpdateService(context.Background(), log.WithField("service", "update")),
		DeviceService: NewDeviceService(context.Background(), log.WithField("service", "device")),
		ImageService:  NewImageService(context.Background(), log.WithField("service", "image")),
		RetryMinutes:  5,
		config:        config,
		shuttingDown:  false,
		topic:         actualTopic,
	}
	switch topic {
	case kafkacommon.TopicPlaybookDispatcherRuns:
		s.consumer = s.ConsumePlaybookDispatcherRuns
	case kafkacommon.TopicInventoryEvents:
		s.consumer = s.ConsumePlatformInventoryEvents
	default:
		log.WithField("topic", topic).Error("No consumer for topic")
		return nil
	}
	s.Reader = s.initReader()
	return s
}

func (s *KafkaConsumerService) initReader() *kafka.Consumer {
	GroupID := "edge-fleet-management-update-playbook"
	kafkaConfigMap := kafkacommon.GetKafkaConsumerConfigMap(GroupID)
	c, err := kafka.NewConsumer(&kafkaConfigMap)

	if err != nil {
		log.WithField("error", err.Error()).Error("Failed to create consumer")
	}

	err = c.SubscribeTopics([]string{s.topic}, nil)
	if err != nil {
		log.Error("Subscribing to topics failed")

	}
	return c
}

// HeaderPlayBookDispatcher is the header for Playbook Dispatcher events
type HeaderPlayBookDispatcher struct {
	Service string `json:"service"`
}

func getHeader(headers []kafka.Header, header string) string {
	// TODO: consider having this return an interface
	// read the service from the headers
	for _, h := range headers {
		if h.Key == header {
			return string(h.Value)
		}
	}
	return ""
}

// ConsumePlaybookDispatcherRuns is the method that consumes from the topic that gives us the execution of playbook from playbook dispatcher service
func (s *KafkaConsumerService) ConsumePlaybookDispatcherRuns() error {
	log.Info("Starting to consume playbook dispatcher's runs")

	run := true
	for run {
		cs := s.Reader.Poll(100)
		if cs == nil {
			continue
		}

		switch e := cs.(type) {
		case *kafka.Message:
			service := getHeader(e.Headers, "service")

			if feature.KafkaLogging.IsEnabled() {
				log.WithFields(log.Fields{
					"event_topic":     *e.TopicPartition.Topic,
					"event_partition": e.TopicPartition.Partition,
					"event_offset":    e.TopicPartition.Offset,
					"event_recordkey": string(e.Key),
					"event_service":   service,
				})
			}

			// if it's an edge service event, process the message body
			if service == "edge" {
				err := s.UpdateService.ProcessPlaybookDispatcherRunEvent(e.Value)
				// if there's a problem with the message body, log the error and commit the offset
				if err != nil {
					log.WithField("error", err.Error()).Error("Continuing without handling edge service event")
				}
			} else {
				log.WithField("headers", fmt.Sprintf("%v", e.Headers)).Debug("Skipping message - it is not from edge service")
			}

			// commit the Kafka offset
			_, err := s.Reader.Commit()
			if err != nil {
				log.WithField("error", err).Error("Error committing offset after message")
			}
		case kafka.Error:
			// terminate the application if all brokers are down.
			log.WithFields(log.Fields{"code": e.Code(), "error": e}).Error("Exiting ConsumePlaybookDispatcherRuns loop due to Kafka broker issue")
			if e.Code() == kafka.ErrAllBrokersDown {
				run = false
				return new(KafkaAllBrokersDown)
			}
		default:
			if feature.KafkaLogging.IsEnabled() {
				log.Debug("Event Ignored: ", e)
			}
		}

		if s.isShuttingDown() {
			log.Info("Shutting down, exiting playbook dispatcher's runs consumer")
			run = false
			return nil
		}
	}
	return nil
}

// ConsumePlatformInventoryEvents parses create events from platform.inventory.events kafka topic and save them as devices in the DB
func (s *KafkaConsumerService) ConsumePlatformInventoryEvents() error {
	log.Info("Starting to consume platform inventory events")
	run := true
	for run {
		cs := s.Reader.Poll(100)
		if cs == nil {
			continue
		}

		switch e := cs.(type) {
		case *kafka.Message:
			eventType := getHeader(e.Headers, "event_type")

			if feature.KafkaLogging.IsEnabled() {
				log.WithFields(log.Fields{
					"event_topic":     *e.TopicPartition.Topic,
					"event_partition": e.TopicPartition.Partition,
					"event_offset":    e.TopicPartition.Offset,
					"event_recordkey": string(e.Key),
					"event_type":      eventType,
				})
			}

			if eventType != InventoryEventTypeCreated && eventType != InventoryEventTypeUpdated && eventType != InventoryEventTypeDelete {
				continue
			}

			log.Debug("Processing an Inventory event")

			var err error

			switch eventType {
			case InventoryEventTypeCreated:
				err = s.DeviceService.ProcessPlatformInventoryCreateEvent(e.Value)
			case InventoryEventTypeUpdated:
				err = s.DeviceService.ProcessPlatformInventoryUpdatedEvent(e.Value)
			case InventoryEventTypeDelete:
				err = s.DeviceService.ProcessPlatformInventoryDeleteEvent(e.Value)
			default:
				err = nil
			}

			if err != nil {
				log.WithField("error", err.Error()).Error("Continuing without handling event_type event: " + eventType)
			}

			// commit the Kafka offset
			_, err = s.Reader.Commit()
			if err != nil {
				log.WithField("error", err).Error("Error committing offset after message")
			}
		case kafka.Error:
			log.WithFields(log.Fields{"code": e.Code(), "error": e}).Error("Exiting ConsumePlatformInventoryEvents loop due to Kafka broker issue")
			if e.Code() == kafka.ErrAllBrokersDown {
				run = false
				return new(KafkaAllBrokersDown)
			}
		default:
			log.Debug("Event Ignored: ", e)
		}

		if s.isShuttingDown() {
			log.Info("Shutting down, exiting platform inventory events consumer")
			return nil
		}
	}
	return nil
}

func (s *KafkaConsumerService) isShuttingDown() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.shuttingDown
}

func (s *KafkaConsumerService) setShuttingDown() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.shuttingDown = true
}

// Close wraps up reader work
func (s *KafkaConsumerService) Close() {
	log.Info("Closing Kafka readers...")

	s.setShuttingDown()

	if err := s.Reader.Close(); err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Error closing Kafka reader")
	}
}

// Start consumers for this application
func (s *KafkaConsumerService) Start() {
	log.Info("Starting consumers...")

	// keeping track of fails for logging reference
	failCounter := 0
	for {
		// The only way to currently exit this for is sending an exit signal to the app
		// Due to this call, this is also a method that can't be unit tested (see comment in the method above)
		err := s.consumer()

		// break out of loop if application is gracefully shutting down with -SIGTERM
		if s.isShuttingDown() {
			if err != nil {
				log.WithFields(log.Fields{
					"error": err.Error(),
				}).Error("There was en error connecting to the broker. Reader was intentionally closed.")
			}
			log.Info("Shutting down, exiting main consumer loop")
			break
		}

		// just logging that we'll retry here
		if err != nil {
			log.WithFields(log.Fields{
				"error":          err.Error(),
				"minutesToRetry": s.RetryMinutes,
			}).Error("There was en error connecting to the broker. Retry in a few minutes.")
		}

		// closing the reader if there was a connection issue
		if err := s.Reader.Close(); err != nil {
			failCounter++
			log.WithFields(log.Fields{
				"topic":        s.topic,
				"fail-counter": failCounter,
				"error":        err.Error(),
			}).Error("Error closing Kafka reader")
		} else {
			failCounter = 0
		}
		time.Sleep(time.Minute * time.Duration(s.RetryMinutes))
		s.Reader = s.initReader()
	}
}
