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

// GetInstance returns a kafka producer instance
func GetInstance() *kafka.Producer {
	if singleInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		if singleInstance == nil && clowder.IsClowderEnabled() {
			brokers := make([]clowder.BrokerConfig, len(clowder.LoadedConfig.Kafka.Brokers))
			for i, b := range clowder.LoadedConfig.Kafka.Brokers {
				brokers[i] = b
			}
			p, err := kafka.NewProducer(&kafka.ConfigMap{
				"bootstrap.servers": fmt.Sprintf("%s:%d", brokers[0].Hostname, *brokers[0].Port),
				"sasl.mechanisms":   brokers[0].Sasl.SaslMechanism,
				"security.protocol": brokers[0].Sasl.SecurityProtocol,
				"sasl.username":     brokers[0].Sasl.Username,
				"sasl.password":     brokers[0].Sasl.Password})
			if err != nil {
				log.WithField("error", err).Error("Failed to create producer")
				return nil
			}
			singleInstance = p
		}
	}
	return singleInstance
}
