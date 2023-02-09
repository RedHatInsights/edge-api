package kafkacommon

import (
	"errors"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

// DefaultDeliveryTimeout the default delivery timeout in ms
const DefaultDeliveryTimeout = 10000

var ErrUnknownProduceDeliveryEvent = errors.New("producer produce returned unknown delivery event message")
var ErrProduceDeliveryTimeout = errors.New("producer produce message delivery timeout")

// WaitProduceDeliveryEvent wait for a message to be delivered used with Producer Produce function
// this allows to not block other deliveries when using a global producer instance (ProducerService.GetProducerInstance())
func WaitProduceDeliveryEvent(deliveryChan chan kafka.Event, timeout int) error {
	if timeout == 0 {
		timeout = DefaultDeliveryTimeout
	}
	timeoutDuration, _ := time.ParseDuration(fmt.Sprintf("%dms", timeout))
	select {
	case event := <-deliveryChan:
		message, ok := event.(*kafka.Message)
		if !ok {
			return ErrUnknownProduceDeliveryEvent
		}
		return message.TopicPartition.Error
	case <-time.After(timeoutDuration):
		return ErrProduceDeliveryTimeout
	}
}
