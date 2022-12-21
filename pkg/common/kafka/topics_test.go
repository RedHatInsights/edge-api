package kafkacommon_test

import (
	"context"
	"errors"
	"testing"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	v1 "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/redhatinsights/edge-api/config"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/stretchr/testify/assert"
)

func TestGetTopic(t *testing.T) {
	service := &kafkacommon.TopicService{}
	cfg := config.Get()
	topicConfig := clowder.TopicConfig{
		Name:          kafkacommon.TopicFleetmgmtImageISOBuild,
		RequestedName: kafkacommon.TopicFleetmgmtImageISOBuild,
	}
	conf := cfg.KafkaConfig

	// Reset config.kafkaconfig back to its original value
	defer func(conf *v1.KafkaConfig) {
		config.Get().KafkaConfig = conf
	}(conf)

	ctx := context.Background()
	cfg.KafkaConfig = &v1.KafkaConfig{}
	cfg.KafkaConfig.Topics = []clowder.TopicConfig{}
	cfg.KafkaConfig.Topics = append(cfg.KafkaConfig.Topics, topicConfig)

	cases := []struct {
		Name            string
		Context         context.Context
		conf            *v1.KafkaConfig
		ExpectedRequest string
		ExpectedError   error
	}{
		{
			Name:            "image-iso-build",
			Context:         ctx,
			ExpectedRequest: kafkacommon.TopicFleetmgmtImageISOBuild,
			conf:            cfg.KafkaConfig,
			ExpectedError:   nil,
		},
		{
			Name:            "kafka topic is not found",
			Context:         ctx,
			ExpectedRequest: kafkacommon.TopicFleetmgmtImageISOBuild,
			conf:            conf,
			ExpectedError:   errors.New("topic is not found in config"),
		},
	}
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			cfg.KafkaConfig = test.conf
			topic, error := service.GetTopic(topicConfig.RequestedName)
			assert.Equal(t, topic, test.ExpectedRequest)
			if test.ExpectedError != nil {
				assert.Error(t, error, "expected error but no error occurred")
				assert.Equal(t, error.Error(), test.ExpectedError.Error())

			} else {
				assert.Equal(t, error, test.ExpectedError)
			}
		})
	}
}
