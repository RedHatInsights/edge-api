package main

import (
	"fmt"
	"time"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
)

func main() {
	if clowder.IsClowderEnabled() {
		fmt.Printf("Public Port: %d", clowder.LoadedConfig.PublicPort)
	}

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

	fmt.Println("Entering an infinite loop...")
	for {
		fmt.Println("Sleeping 300...")
		time.Sleep(300 * time.Second)
	}
}
