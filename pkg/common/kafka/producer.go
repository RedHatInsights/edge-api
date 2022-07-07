package common

import (
	"fmt"
	"sync"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	log "github.com/sirupsen/logrus"
)

var lock = &sync.Mutex{}

var singleInstance *kafka.Producer

func getInstance() *kafka.Producer {
	if singleInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		if singleInstance == nil && clowder.IsClowderEnabled() {
			brokers := make([]string, len(clowder.LoadedConfig.Kafka.Brokers))
			for i, b := range clowder.LoadedConfig.Kafka.Brokers {
				brokers[i] = fmt.Sprintf("%s:%d", b.Hostname, *b.Port)
			}
			p, err := kafka.NewProducer(&kafka.ConfigMap{
				"bootstrap.servers": brokers[0]})
			if err != nil {
				log.WithField("error", err).Error("Failed to create producer")
				return nil
			}
			singleInstance = p
		}
	}
	return singleInstance
}
