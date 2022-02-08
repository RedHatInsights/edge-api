package services

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"

	"github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
)

// ConsumerService is the interface that takes care of our consumer implementation
type ConsumerService interface {
	Start()
}

// KafkaConsumerService is the implementation of a consumer service based on Kafka topics
type KafkaConsumerService struct {
	Reader        *kafka.Reader
	UpdateService UpdateServiceInterface
	DeviceService DeviceServiceInterface
	RetryMinutes  uint
	config        *clowder.KafkaConfig
	shuttingDown  bool
	topic         string
}

// PlatformInsightsCreateEventPayload is the body of the create event found on the platform.inventory.events kafka topic.
type PlatformInsightsCreateEventPayload struct {
	Type         string `json:"type"`
	PlatformMeta string `json:"platform_metadata"`
	Metadata     struct {
		RequestID string `json:"request_id"`
	} `json:"metadata"`
	Host struct {
		ID             string `json:"id"`
		Account        string `json:"account"`
		DisplayName    string `json:"display_name"`
		AnsibleHost    string `json:"ansible_host"`
		Fqdn           string `json:"fqdn"`
		InsightsID     string `json:"insights_id"`
		StaleTimestamp string `json:"stale_timestamp"`
		Reporter       string `json:"reporter"`
		Tags           string `json:"tags"`
		SystemProfile  string `json:"system_profile"`
	} `json:"host"`
}

// NewKafkaConsumerService gives a instance of the Kafka implementation of ConsumerService
func NewKafkaConsumerService(config *clowder.KafkaConfig, topic string) ConsumerService {
	// to consume messages
	s := &KafkaConsumerService{
		UpdateService: NewUpdateService(context.Background(), log.WithField("service", "update")),
		DeviceService: NewDeviceService(context.Background(), log.WithField("service", "device")),
		RetryMinutes:  5,
		config:        config,
		shuttingDown:  false,
		topic:         topic,
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

// ConsumeInventoryCreateEvents parses create events from platform.inventory.events kafka topic and save them as devices in the DB
func (s *KafkaConsumerService) ConsumeInventoryCreateEvents() error {
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
				log.WithFields(log.Fields{
					"topic":  m.Topic,
					"offset": m.Offset,
					"key":    string(m.Key),
					"value":  string(m.Value),
				}).Debug("Read message from Kafka topic")
			}
		}
		if eventType == "created" {
			err = s.DeviceService.ProcessPlatformInventoryCreateEvent(m.Value)
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Error("Error writing Kafka message to DB")
			}
		} else {
			log.Debug("Skipping message - not a create message from platform insights")
		}
	}
}

// RegisterShutdown listens to os signals to wrap up reader work
func (s *KafkaConsumerService) RegisterShutdown() {
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	<-sigint
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

	go s.RegisterShutdown()
	for {
		// The only way to actually exit this for is sending an exit signal to the app
		// Due to this call, this is also a method that can't be unit tested (see comment in the method above)
		var err error
		if s.topic == "platform.playbook-dispatcher.runs" {
			err = s.ConsumePlaybookDispatcherRuns()
		} else if s.topic == "platform.inventory.events" {
			err = s.ConsumeInventoryCreateEvents()
		}
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
