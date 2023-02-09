package kafkacommon_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/stretchr/testify/assert"
)

type TestEvent struct{ Output string }

func (event *TestEvent) String() string {
	return event.Output
}

func TestWaitProduceDeliveryEvent(t *testing.T) {
	topic := "a_dummy_topic_name"
	expectedTopicPartitionError := errors.New("an expected topicPartition error")

	testCases := []struct {
		Name          string
		Event         kafka.Event
		Timeout       int
		ExpectedError error
	}{
		{
			Name: "should return nil when no error occurred",
			Event: &kafka.Message{
				TopicPartition: kafka.TopicPartition{
					Topic: &topic, Partition: kafka.PartitionAny,
					Error: nil,
				},
			},
			ExpectedError: nil,
		},
		{
			Name: "should return the occurred error",
			Event: &kafka.Message{
				TopicPartition: kafka.TopicPartition{
					Topic: &topic, Partition: kafka.PartitionAny,
					Error: expectedTopicPartitionError,
				},
			},
			ExpectedError: expectedTopicPartitionError,
		},
		{
			Name:          "should return error when the event type is not kafka.Message",
			Event:         &TestEvent{Output: "a dummy event output, this is not a kafka.Message type"},
			ExpectedError: kafkacommon.ErrUnknownProduceDeliveryEvent,
		},
		{
			Name: "should return timeout error when the producer produce message did not finish in time",
			Event: &kafka.Message{
				TopicPartition: kafka.TopicPartition{
					Topic: &topic, Partition: kafka.PartitionAny,
					Error: nil,
				},
			},
			Timeout:       500,
			ExpectedError: kafkacommon.ErrProduceDeliveryTimeout,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			deliveryChan := make(chan kafka.Event)
			defer close(deliveryChan)
			go func(channel chan kafka.Event, event kafka.Event, timeout int) {
				if timeout > 0 {
					timeoutDuration, _ := time.ParseDuration(fmt.Sprintf("%dms", timeout))
					time.Sleep(timeoutDuration)
					return
				}
				channel <- event
			}(deliveryChan, testCase.Event, testCase.Timeout)

			err := kafkacommon.WaitProduceDeliveryEvent(deliveryChan, testCase.Timeout)
			assert.Equal(t, testCase.ExpectedError, err)
		})
	}
}
