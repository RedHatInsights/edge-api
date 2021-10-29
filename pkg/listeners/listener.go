package listeners

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
)

func ListenToPlaybookDispatcherRunsTopic() {
	conn, err := kafka.Dial("tcp", "localhost:9092")
	if err != nil {
		panic(err.Error())
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions()
	if err != nil {
		panic(err.Error())
	}

	m := map[string]struct{}{}

	for _, p := range partitions {
		m[p.Topic] = struct{}{}
	}
	for k := range m {
		fmt.Println(k)
	}

	log.Info("Starting listeners...")

	// to consume messages
	topic := "platform.playbook-dispatcher.runs"
	// make a new reader that consumes from topic from this consumer group
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   topic,
		GroupID: "edge-fleet-management",
	})

	for {
		m, err := r.ReadMessage(context.Background())
		if err != nil {
			break
		}
		fmt.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))
	}

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint
		r.Close()
	}()

}

func Start() {
	log.Info("Starting listeners...")

	go ListenToPlaybookDispatcherRunsTopic()

	log.Info("Listeners started...")
}
