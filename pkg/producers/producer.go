package producers

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
)

func ProduceToPlaybookDispatcherRunsTopic() {
	log.Info("Starting Producers...")

	// to consume messages
	topic := "platform.playbook-dispatcher.runs"
	partition := 0

	conn, err := kafka.DialLeader(context.Background(), "tcp", "localhost:9092", topic, partition)
	if err != nil {
		log.Error("failed to dial leader on producer:", err)
	}

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint
		if err := conn.Close(); err != nil {
			log.Error("failed to close writer:", err)
		}
	}()

	for {
		_, err = conn.WriteMessages(
			kafka.Message{Value: []byte("one!")},
			kafka.Message{Value: []byte("two!")},
			kafka.Message{Value: []byte("three!")},
			kafka.Message{Value: []byte("four!")},
			kafka.Message{Value: []byte("five!")},
			kafka.Message{Value: []byte("six!")},
			kafka.Message{Value: []byte("seven!")},
			kafka.Message{Value: []byte("eight!")},
		)
		if err != nil {
			log.Error("failed to write messages:", err)
		}
		log.Debug("Wrote messages!")
		time.Sleep(1 * time.Second)
	}

}

func Start() {
	log.Info("Starting producers...")

	go ProduceToPlaybookDispatcherRunsTopic()

	log.Info("Producers started...")
}
