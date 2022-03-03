package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
)

// RecordValue represents the struct of the value in a Kafka message
type RecordValue struct {
	ImageId int
}

func main() {
	/* I am leaving all of this in place for testing.
	It will be replaced in a few days. */
	if clowder.IsClowderEnabled() {
		fmt.Printf("Public Port: %d\n", clowder.LoadedConfig.PublicPort)

		brokers := make([]string, len(clowder.LoadedConfig.Kafka.Brokers))
		for i, b := range clowder.LoadedConfig.Kafka.Brokers {
			brokers[i] = fmt.Sprintf("%s:%d", b.Hostname, *b.Port)
			fmt.Println(brokers[i])
		}

		topics := make([]string, len(clowder.LoadedConfig.Kafka.Topics))
		for i, b := range clowder.LoadedConfig.Kafka.Topics {
			topics[i] = fmt.Sprintf("%s (%s)", b.Name, b.RequestedName)
			fmt.Println(topics[i])
		}

		topic := "platform.edge.fleetmgmt.image-build"

		fmt.Println("Entering an infinite loop...")
		for {
			// Create Producer instance
			p, err := kafka.NewProducer(&kafka.ConfigMap{
				"bootstrap.servers": brokers[0]})
			if err != nil {
				fmt.Printf("Failed to create producer: %s", err)
				os.Exit(1)
			}
			recordKey := "imagebuild"
			data := &RecordValue{
				ImageId: 123456789}
			recordValue, _ := json.Marshal(&data)
			fmt.Printf("Preparing to produce record: %s\t%s\n", recordKey, recordValue)
			perr := p.Produce(&kafka.Message{
				TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
				Key:            []byte(recordKey),
				Value:          []byte(recordValue),
			}, nil)
			if perr != nil {
				fmt.Println("Error sending message")
			}

			// Wait for all messages to be delivered
			p.Flush(15 * 1000)
			p.Close()

			fmt.Printf("Messages was produced to topic %s!", topic)
			fmt.Println("Sleeping 5 minutes...")
			time.Sleep(5 * time.Minute)
		}
	}
}
