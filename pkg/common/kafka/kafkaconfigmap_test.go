package kafkacommon_test

import (
	"context"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	v1 "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/redhatinsights/edge-api/config"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"

	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"
)

func TestGetKafkaProducerConfigMap(t *testing.T) {
	var service *kafkacommon.KafkaConfigMapService
	cfg := config.Get()

	conf := cfg.KafkaConfig
	originalKafkaBroker := cfg.KafkaBroker

	authType := clowder.BrokerConfigAuthtype("sasl")
	dummyString := "something"
	mech := "PLAIN"
	proto := "SASL_SSL"
	port := 80
	brokerConfig := clowder.BrokerConfig{
		Authtype:         &authType,
		Cacert:           &dummyString,
		Hostname:         "192.168.1.7",
		Port:             &port,
		SecurityProtocol: &proto,
		Sasl: &clowder.KafkaSASLConfig{
			SaslMechanism: &mech,
			Username:      &dummyString,
			Password:      &dummyString,
		},
	}
	kafkaConfigMap := kafka.ConfigMap{
		"bootstrap.servers":        "192.168.1.7:80",
		"sasl.mechanisms":          "PLAIN",
		"security.protocol":        "SASL_SSL",
		"sasl.username":            "something",
		"sasl.password":            "something",
		"request.required.acks":    -1,
		"message.send.max.retries": 15,
		"retry.backoff.ms":         100,
	}
	cfg.KafkaBroker = &brokerConfig

	// Reset config.kafkaconfig back to its original value
	defer func(conf *v1.KafkaConfig, kafkaBroker *clowder.BrokerConfig) {
		config.Get().KafkaConfig = conf
		config.Get().KafkaBroker = kafkaBroker
	}(conf, originalKafkaBroker)

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
			Name:            "kafka-producer-config",
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
func TestGetKafkaConsumerConfigMap(t *testing.T) {
	var service *kafkacommon.KafkaConfigMapService
	cfg := config.Get()

	conf := cfg.KafkaConfig
	originalKafkaBroker := cfg.KafkaBroker

	authType := clowder.BrokerConfigAuthtype("sasl")
	dummyString := "something"
	mech := "PLAIN"
	proto := "SASL_SSL"
	port := 80
	brokerConfig := clowder.BrokerConfig{
		Authtype:         &authType,
		Cacert:           &dummyString,
		Hostname:         "192.168.1.7",
		Port:             &port,
		SecurityProtocol: &proto,
		Sasl: &clowder.KafkaSASLConfig{
			SaslMechanism: &mech,
			Username:      &dummyString,
			Password:      &dummyString,
		},
	}
	kafkaConfigMap := kafka.ConfigMap{
		"bootstrap.servers":     "192.168.1.7:80",
		"sasl.mechanisms":       "PLAIN",
		"security.protocol":     "SASL_SSL",
		"sasl.username":         "something",
		"sasl.password":         "something",
		"broker.address.family": "v4",
		"session.timeout.ms":    6000,
		"auto.offset.reset":     "earliest",
		"group.id":              "imagesisobuild",
	}
	brokerSlice := []clowder.BrokerConfig{brokerConfig}
	cfg.KafkaBrokers = brokerSlice
	cfg.KafkaBroker = &brokerConfig

	// Reset config.kafkaconfig back to its original value
	defer func(conf *v1.KafkaConfig, kafkaBroker *clowder.BrokerConfig) {
		config.Get().KafkaConfig = conf
		config.Get().KafkaBroker = kafkaBroker
	}(conf, originalKafkaBroker)

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
			Name:            "kafka-consumer-config",
			Context:         ctx,
			ExpectedRequest: kafkaConfigMap,
			conf:            cfg.KafkaConfig,
			ExpectedError:   nil,
		},
	}
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			cfg.KafkaConfig = test.conf
			consumerGroupID := "imagesisobuild"
			configMap := service.GetKafkaConsumerConfigMap(consumerGroupID)
			assert.Equal(t, configMap, test.ExpectedRequest)
		})
	}
}

func TestGetKafkaProducerConfigMapSecurityProtocol(t *testing.T) {
	var service kafkacommon.KafkaConfigMapServiceInterface
	cfg := config.Get()
	originalKafkaBrokerConf := cfg.KafkaBroker
	defer func(KafkaBrokerConf *clowder.BrokerConfig) {
		config.Get().KafkaBroker = KafkaBrokerConf
	}(originalKafkaBrokerConf)

	authTypeSasl := clowder.BrokerConfigAuthtype("sasl")
	authTypeMtls := clowder.BrokerConfigAuthtype("mtls")
	dummyString := faker.UUIDHyphenated()
	mech := "PLAIN"
	SecurityProtocols := []string{faker.UUIDHyphenated(), faker.UUIDHyphenated()}
	port := 80

	testCases := []struct {
		Name                     string
		BrokerConfig             clowder.BrokerConfig
		ExpectedSecurityProtocol string
	}{
		{
			Name: "should get security protocol from broker config",
			BrokerConfig: clowder.BrokerConfig{
				Authtype:         &authTypeSasl,
				Cacert:           &dummyString,
				Hostname:         "192.168.1.7",
				Port:             &port,
				SecurityProtocol: &SecurityProtocols[0],
				Sasl: &clowder.KafkaSASLConfig{
					SaslMechanism: &mech,
					Username:      &dummyString,
					Password:      &dummyString,
				},
			},
			ExpectedSecurityProtocol: SecurityProtocols[0],
		},
		{
			Name: "should get security protocol from broker sasl config",
			BrokerConfig: clowder.BrokerConfig{
				Authtype: &authTypeSasl,
				Cacert:   &dummyString,
				Hostname: "192.168.1.7",
				Port:     &port,
				Sasl: &clowder.KafkaSASLConfig{
					SaslMechanism:    &mech,
					Username:         &dummyString,
					Password:         &dummyString,
					SecurityProtocol: &SecurityProtocols[1], // nolint: staticcheck
				},
			},
			ExpectedSecurityProtocol: SecurityProtocols[1],
		},
		{
			Name: "should not get security protocol if no defined in broker or sasl config",
			BrokerConfig: clowder.BrokerConfig{
				Authtype: &authTypeSasl,
				Cacert:   &dummyString,
				Hostname: "192.168.1.7",
				Port:     &port,
				Sasl: &clowder.KafkaSASLConfig{
					SaslMechanism: &mech,
					Username:      &dummyString,
					Password:      &dummyString,
				},
			},
			ExpectedSecurityProtocol: "",
		},
		{
			Name: "should not get security protocol when defined in sasl config and auth type mtls",
			BrokerConfig: clowder.BrokerConfig{
				Authtype: &authTypeMtls,
				Cacert:   &dummyString,
				Hostname: "192.168.1.7",
				Port:     &port,
				Sasl: &clowder.KafkaSASLConfig{
					SaslMechanism:    &mech,
					Username:         &dummyString,
					Password:         &dummyString,
					SecurityProtocol: &SecurityProtocols[1], // nolint: staticcheck
				},
			},
			ExpectedSecurityProtocol: "",
		},
		{
			Name: "should get security protocol when defined in broker config and auth type mtls",
			BrokerConfig: clowder.BrokerConfig{
				Authtype:         &authTypeMtls,
				Cacert:           &dummyString,
				Hostname:         "192.168.1.7",
				Port:             &port,
				SecurityProtocol: &SecurityProtocols[0],
				Sasl: &clowder.KafkaSASLConfig{
					SaslMechanism: &mech,
					Username:      &dummyString,
					Password:      &dummyString,
				},
			},
			ExpectedSecurityProtocol: SecurityProtocols[0],
		},
	}

	service = kafkacommon.NewKafkaConfigMapService()
	for _, testCase := range testCases {
		// avoid Implicit memory aliasing
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			config.Get().KafkaBroker = &testCase.BrokerConfig

			configMap := service.GetKafkaProducerConfigMap()
			securityProtocol, err := configMap.Get("security.protocol", "")
			assert.NoError(t, err, "cannot get security protocol from configMap, occur when as type mismatch")
			assert.Equal(t, securityProtocol, testCase.ExpectedSecurityProtocol)
		})
	}
}

func TestGetKafkaProducerConfigMapCaCert(t *testing.T) {
	var service kafkacommon.KafkaConfigMapServiceInterface
	cfg := config.Get()
	originalKafkaBrokerConf := cfg.KafkaBroker
	originalKafkaBrokerConfCaCertPath := cfg.KafkaBrokerCaCertPath
	defer func(conf *clowder.BrokerConfig, caCertPath string) {
		config.Get().KafkaBroker = conf
		config.Get().KafkaBrokerCaCertPath = caCertPath
	}(originalKafkaBrokerConf, originalKafkaBrokerConfCaCertPath)

	authTypeSasl := clowder.BrokerConfigAuthtype("sasl")
	dummyString := faker.UUIDHyphenated()
	port := 80
	caCertPath := faker.UUIDHyphenated()

	testCases := []struct {
		Name                   string
		BrokerConfig           clowder.BrokerConfig
		BrokerConfigCaCertPath string
		ExpectedCaLocation     string
	}{
		{
			Name: "should get security protocol from broker config",
			BrokerConfig: clowder.BrokerConfig{
				Authtype: &authTypeSasl,
				Cacert:   &dummyString,
				Hostname: "192.168.1.7",
				Port:     &port,
			},
			BrokerConfigCaCertPath: caCertPath,
			ExpectedCaLocation:     caCertPath,
		},
		{
			Name: "should get security protocol from broker sasl config",
			BrokerConfig: clowder.BrokerConfig{
				Hostname: "192.168.1.7",
				Port:     &port,
			},
			ExpectedCaLocation: "",
		},
	}

	service = kafkacommon.NewKafkaConfigMapService()
	for _, testCase := range testCases {
		// avoid Implicit memory aliasing
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			config.Get().KafkaBroker = &testCase.BrokerConfig
			config.Get().KafkaBrokerCaCertPath = testCase.BrokerConfigCaCertPath

			configMap := service.GetKafkaProducerConfigMap()
			caLocation, err := configMap.Get("ssl.ca.location", "")
			assert.NoError(t, err, "cannot get ssl ca location from configMap, occurs when as type mismatch")
			assert.Equal(t, caLocation, testCase.ExpectedCaLocation)
		})
	}
}
