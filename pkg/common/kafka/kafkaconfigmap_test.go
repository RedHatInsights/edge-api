// nolint:govet,revive
package kafkacommon_test

import (
	"context"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	v1 "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/redhatinsights/edge-api/config"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/stretchr/testify/assert"
)

func TestGetKafkaProducerConfigMap(t *testing.T) {
	var service *kafkacommon.KafkaConfigMapService
	cfg := config.Get()

	conf := cfg.KafkaConfig

	authType := clowder.BrokerConfigAuthtype("Auth")
	dummyString := "something"
	mech := "PLAIN"
	proto := "SASL_SSL"
	port := 80
	brokerConfig := clowder.BrokerConfig{
		Authtype: &authType,
		Cacert:   &dummyString,
		Hostname: "192.168.1.7",
		Port:     &port,
		Sasl: &clowder.KafkaSASLConfig{
			SaslMechanism:    &mech,
			SecurityProtocol: &proto,
			Username:         &dummyString,
			Password:         &dummyString,
		},
	}
	kafkaConfigMap := kafka.ConfigMap{
		"bootstrap.servers": "192.168.1.7:80",
		"sasl.mechanisms":   "PLAIN",
		"security.protocol": "SASL_SSL",
		"sasl.username":     "something",
		"sasl.password":     "something",
	}
	brokerSlice := []clowder.BrokerConfig{brokerConfig}
	cfg.KafkaBrokers = brokerSlice

	// Reset config.kafkaconfig back to its original value
	defer func(conf *v1.KafkaConfig) {
		config.Get().KafkaConfig = conf
	}(conf)

	ctx := context.Background()
	cfg.KafkaConfig = &v1.KafkaConfig{}
	cfg.KafkaConfig.Brokers = []clowder.BrokerConfig{brokerConfig}

	cases := []struct {
		Name            string
		Context         context.Context
		conf            *v1.KafkaConfig
		ExpectedRequest kafka.ConfigMap
		ExpectedError   error
	}{
		{
			Name:            "image-iso-build",
			Context:         ctx,
			ExpectedRequest: kafkaConfigMap,
			conf:            cfg.KafkaConfig,
			ExpectedError:   nil,
		},
	}
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			cfg.KafkaConfig = test.conf
			configMap := service.GetKafkaProducerConfigMap()
			assert.Equal(t, configMap, test.ExpectedRequest)
		})
	}
}
