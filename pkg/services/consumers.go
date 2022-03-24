package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"

	"github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
)

// ConsumerService is the interface that takes care of our consumer implementation
type ConsumerService interface {
	Start()
	Close()
}

// KafkaConsumerService is the implementation of a consumer service based on Kafka topics
type KafkaConsumerService struct {
	Reader        *kafka.Reader
	UpdateService UpdateServiceInterface
	DeviceService DeviceServiceInterface
	ImageService  ImageServiceInterface
	RetryMinutes  uint
	config        *clowder.KafkaConfig
	shuttingDown  bool
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
		log.Errorf("No consumer for topic: %s", topic)
		return nil
	}
	s.Reader = s.initReader()
	return s
}

func (s *KafkaConsumerService) initReader() *kafka.Reader {
	brokers := make([]string, len(s.config.Brokers))
	for i, b := range s.config.Brokers {
		brokers[i] = fmt.Sprintf("%s:%d", b.Hostname, *b.Port)
	}
	log.WithFields(log.Fields{
		"brokers": brokers, "topic": s.topic,
	}).Debug("Connecting with Kafka broker")
	// make a new reader that consumes from topic from this consumer group
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   s.topic,
		GroupID: "edge-fleet-management-update-playbook",
	})
	return r
}

// ConsumePlaybookDispatcherRuns is the method that consumes from the topic that gives us the execution of playbook from playbook dispatcher service
func (s *KafkaConsumerService) ConsumePlaybookDispatcherRuns() error {
	log.Info("Starting to consume playbook dispatcher's runs")
	// Keep as much logic out of this is method as the Kafka Reader is not mockable for unit tests, as per
	// https://github.com/segmentio/kafka-go/issues/794
	// Most of the logic needs to be under the ProcessPlaybookDispatcherRunEvent service
	for {
		m, err := s.Reader.ReadMessage(context.Background())
		if err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("Error reading message from Kafka topic")
			return err
		}
		log.WithFields(log.Fields{
			"topic":  m.Topic,
			"offset": m.Offset,
			"key":    string(m.Key),
			"value":  string(m.Value),
		}).Debug("Read message from Kafka topic")
		var service string
		for _, h := range m.Headers {
			if h.Key == "service" {
				service = string(h.Value)
			}
		}
		if service == "edge" {
			err = s.UpdateService.ProcessPlaybookDispatcherRunEvent(m.Value)
			if err != nil {
				log.WithFields(log.Fields{
					"error": err.Error(),
				}).Error("Error treating Kafka message")
			}
		} else {
			log.Debug("Skipping message - it is not from edge service")
		}
	}
}

// ConsumePlatformInventoryEvents parses create events from platform.inventory.events kafka topic and save them as devices in the DB
func (s *KafkaConsumerService) ConsumePlatformInventoryEvents() error {
	log.Info("Starting to consume platform inventory create events")
	for {
		m, err := s.Reader.ReadMessage(context.Background())
		if err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("Error reading message from Kafka topic")
			return err
		}
		var eventType string
		for _, h := range m.Headers {
			if h.Key == "event_type" {
				eventType = string(h.Value)
				break
			}
		}
		if eventType != InventoryEventTypeCreated && eventType != InventoryEventTypeUpdated && eventType != InventoryEventTypeDelete {
			log.Debug("Skipping kafka message - Insights Platform Inventory message is not a created and not an updated event type")
			continue
		}
		/* log.WithFields(log.Fields{
			"topic":  m.Topic,
			"offset": m.Offset,
			"key":    string(m.Key),
			"value":  string(m.Value),
		}).Debug("Read message from Kafka topic") */
		// temporarily removing value to make troubleshooting interrupt easier
		log.WithFields(log.Fields{
			"topic":  m.Topic,
			"offset": m.Offset,
			"key":    string(m.Key),
		}).Debug("Read message from Kafka topic")

		switch eventType {
		case InventoryEventTypeCreated:
			err = s.DeviceService.ProcessPlatformInventoryCreateEvent(m.Value)
		case InventoryEventTypeUpdated:
			err = s.DeviceService.ProcessPlatformInventoryUpdatedEvent(m.Value)
		case InventoryEventTypeDelete:
			err = s.DeviceService.ProcessPlatformInventoryDeleteEvent(m.Value)
		default:
			err = nil
		}
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Error writing Kafka message to DB")
		}
	}
}

// ConsumeImageBuildEvents parses create events from platform.edge.fleetmgmt.image-build kafka topic
func (s *KafkaConsumerService) ConsumeImageBuildEvents() error {
	type IBevent struct {
		ImageID uint `json:"image_id"`
	}

	log.Info("Starting to consume image build events")
	for {
		m, err := s.Reader.ReadMessage(context.Background())
		if err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("Error reading message from Kafka platform.edge.fleetmgmt.image-build topic")
		}

		// temporarily logging all events from the topic
		log.WithFields(log.Fields{
			"topic":  m.Topic,
			"offset": m.Offset,
			"key":    string(m.Key),
			"value":  string(m.Value),
		}).Debug("Read message from Kafka platform.edge.fleetmgmt.image-build topic")

		// retrieve the image ID from the message value
		var eventMessage *IBevent

		eventErr := json.Unmarshal([]byte(m.Value), &eventMessage)
		if eventErr != nil {
			log.WithField("error", eventErr).Debug("Error unmarshaling event. This is not the event you're looking for")
		} else {
			log.WithField("imageID", eventMessage.ImageID).Debug("Resuming image ID from event on " + string(m.Topic))
			//go s.ImageService.ResumeCreateImage(eventMessage.ImageID)
		}
	}
}

// Close wraps up reader work
func (s *KafkaConsumerService) Close() {
	log.Info("Closing Kafka readers...")
	s.shuttingDown = true
	if err := s.Reader.Close(); err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Error closing Kafka reader")
	}
}

// Start consumers for this application
func (s *KafkaConsumerService) Start() {
	log.Info("Starting consumers...")
	for {
		// The only way to actually exit this for is sending an exit signal to the app
		// Due to this call, this is also a method that can't be unit tested (see comment in the method above)
		err := s.consumer()
		if s.shuttingDown {
			log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("There was en error connecting to the broker. Reader was intentionally closed.")
			break
		}
		log.WithFields(log.Fields{
			"error":          err.Error(),
			"minutesToRetry": s.RetryMinutes,
		}).Error("There was en error connecting to the broker. Retry in a few minutes.")
		time.Sleep(time.Minute * time.Duration(s.RetryMinutes))
		s.Reader = s.initReader()
	}
}
