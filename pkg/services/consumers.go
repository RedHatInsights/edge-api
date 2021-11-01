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
	config *clowder.KafkaConfig
}

// NewKafkaConsumerService gives a instance of the Kafka implementation of ConsumerService
func NewKafkaConsumerService(config *clowder.KafkaConfig) ConsumerService {
	return &KafkaConsumerService{config: config}
}

func (s *KafkaConsumerService) consumePlaybookDispatcherRuns() {
	log.Info("Starting listeners...")

	// to consume messages
	topic := "platform.playbook-dispatcher.runs"
	brokers := make([]string, len(s.config.Brokers))
	for i, b := range s.config.Brokers {
		brokers[i] = fmt.Sprintf("%s:%d", b.Hostname, b.Port)
	}
	// make a new reader that consumes from topic from this consumer group
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: "edge-fleet-management",
	})

	for {
		m, err := r.ReadMessage(context.Background())
		if err != nil {
			break
		}
		log.WithFields(log.Fields{
			"topic":  topic,
			"offset": m.Offset,
			"key":    string(m.Key),
			"value":  string(m.Value),
		}).Debug("Read message from Kafka topic")
	}

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint
		r.Close()
	}()

}

// Start consumers for this application
func (s *KafkaConsumerService) Start() {
	log.Info("Starting consumers...")

	go s.consumePlaybookDispatcherRuns()

	log.Info("Consumers started...")
}
