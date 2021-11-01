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

type ConsumerServiceInterface interface {
	Start()
}

type ConsumerService struct {
	config *clowder.KafkaConfig
}

// NewConsumerService gives a instance of the main implementation of ConsumerServiceInterface
func NewConsumerService(config *clowder.KafkaConfig) ConsumerServiceInterface {
	return &ConsumerService{config: config}
}

func (s *ConsumerService) consumePlaybookDispatcherRuns() {
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

func (s *ConsumerService) Start() {
	log.Info("Starting consumers...")

	go s.consumePlaybookDispatcherRuns()

	log.Info("Consumers started...")
}
