package services

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
}

// NewKafkaConsumerService gives a instance of the Kafka implementation of ConsumerService
func NewKafkaConsumerService(config *clowder.KafkaConfig) ConsumerService {
	// to consume messages
	topic := "platform.playbook-dispatcher.runs"
	brokers := make([]string, len(config.Brokers))
	for i, b := range config.Brokers {
		brokers[i] = fmt.Sprintf("%s:%d", b.Hostname, *b.Port)
	}
	log.WithFields(log.Fields{
		"brokers": brokers, "topic": topic,
	}).Debug("Connecting with Kafka broker")
	// make a new reader that consumes from topic from this consumer group
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: "edge-fleet-management-update-playbook",
	})
	return &KafkaConsumerService{Reader: r, UpdateService: NewUpdateService(context.Background())}
}

func (s *KafkaConsumerService) ConsumePlaybookDispatcherRuns() {
	log.Info("Starting listeners...")

	for {
		m, err := s.Reader.ReadMessage(context.Background())
		if err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("Error reading message from Kafka topic")
			break
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

func (s *KafkaConsumerService) RegisterShutdown() {
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	<-sigint
	s.Reader.Close()
}

// Start consumers for this application
func (s *KafkaConsumerService) Start() {
	log.Info("Starting consumers...")

	go s.RegisterShutdown()
	go s.ConsumePlaybookDispatcherRuns()

	log.Info("Consumers started...")
}
