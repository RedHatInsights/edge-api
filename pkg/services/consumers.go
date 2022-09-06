package services

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"

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
	// to consume messages
	s := &KafkaConsumerService{
		UpdateService: NewUpdateService(context.Background(), log.WithField("service", "update")),
		DeviceService: NewDeviceService(context.Background(), log.WithField("service", "device")),
		ImageService:  NewImageService(context.Background(), log.WithField("service", "image")),
		RetryMinutes:  5,
		config:        config,
		shuttingDown:  false,
		topic:         topic,
	}
	switch topic {
	case "platform.playbook-dispatcher.runs":
		s.consumer = s.ConsumePlaybookDispatcherRuns
	case "platform.inventory.events":
		s.consumer = s.ConsumePlatformInventoryEvents
	case "platform.edge.fleetmgmt.image-build":
		s.consumer = s.ConsumeImageBuildEvents
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
			key := string(e.Key)
			var service string
			if key == "service" {
				service = string(e.Value)
			}
			if service == "edge" {
				err := s.UpdateService.ProcessPlaybookDispatcherRunEvent(e.Value)
				if err != nil {
					log.WithFields(log.Fields{
						"error": err.Error(),
					}).Error("Error treating Kafka message")
					return err
				}
			} else {
				log.Debug("Skipping message - it is not from edge service")
			}
		case kafka.Error:
			// terminate the application if all brokers are down.
			log.WithFields(log.Fields{"code": e.Code(), "error": e}).Error("Exiting ConsumePlaybookDispatcherRuns loop due to Kafka broker issue")
			if e.Code() == kafka.ErrAllBrokersDown {
				run = false
				return new(KafkaBrokerIssue)
			}
		default:
			log.Debug("Event Ignored: ", e)
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
			key := string(e.Key)
			var eventType string
			if key == "event_type" {
				eventType = string(e.Value)
			}

			if eventType != InventoryEventTypeCreated && eventType != InventoryEventTypeUpdated && eventType != InventoryEventTypeDelete {
				continue
			}

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
				return err
			}
		case kafka.Error:
			log.WithFields(log.Fields{"code": e.Code(), "error": e}).Error("Exiting ConsumePlatformInventoryEvents loop due to Kafka broker issue")
			if e.Code() == kafka.ErrAllBrokersDown {
				run = false
				return errors.New("uh oh, caught an error due to kafka broker issue") // create an error in errors.go
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

// ConsumeImageBuildEvents parses create events from platform.edge.fleetmgmt.image-build kafka topic
func (s *KafkaConsumerService) ConsumeImageBuildEvents() error {
	type IBevent struct {
		ImageID uint `json:"image_id"`
	}

	log.Info("Starting to consume image build events")
	run := true
	for run {
		cs := s.Reader.Poll(100)
		if cs == nil {
			continue
		}

		switch e := cs.(type) {
		case *kafka.Message:
			// retrieve the image ID from the message value
			var eventMessage *IBevent
			// currently only logging events while resume is handled via API
			eventErr := json.Unmarshal([]byte(e.Value), &eventMessage)
			if eventErr != nil {
				log.WithField("error", eventErr).Debug("Error unmarshalling event. This is not the event you're looking for")
			} else {
				log.WithFields(log.Fields{"imageID": eventMessage.ImageID, "topic": s.topic}).Debug("Resuming image ID from event")
			}
			if s.isShuttingDown() {
				log.Info("Shutting down, exiting image build events consumer")
				return nil
			}
		case kafka.Error:
			log.WithFields(log.Fields{"code": e.Code(), "error": e}).Error("Exiting ConsumePlatformInventoryEvents loop due to Kafka broker issue")
			if e.Code() == kafka.ErrAllBrokersDown {
				run = false
				return nil // create an error in errors.go
			}
		default:
			log.Debug("Event Ignored: ", e)
		}
		if s.isShuttingDown() {
			log.Info("Shutting down, exiting image build events consumer")
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
