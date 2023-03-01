// FIXME: golangci-lint
// nolint:errcheck,gocritic,gosec,gosimple,govet,revive,typecheck
package config

import (
	"os"
	"testing"

	"github.com/bxcodec/faker/v3"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/stretchr/testify/assert"
)

func TestInitializeApplicationConfig(t *testing.T) {
	currentConfig := Config

	Init()
	assert.NotNil(t, Config)
	assert.NotEqual(t, Config, currentConfig)
}

func TestCreateNewConfig(t *testing.T) {
	localConfig, err := CreateEdgeAPIConfig()
	assert.Nil(t, err)
	assert.NotNil(t, localConfig)
}

func TestRedactPasswordFromURL(t *testing.T) {
	cases := []struct {
		Name   string
		Input  string
		Output string
	}{
		{
			Name:   "should redact password from url",
			Input:  "https://zaphod:password@example.com/?this=that&thisone=theother",
			Output: "https://zaphod:xxxxx@example.com/?this=that&thisone=theother",
		},
		{
			Name:   "should not redact password from url",
			Input:  "https://example.com/?this=that&thisone=theother",
			Output: "https://example.com/?this=that&thisone=theother",
		},
		{
			Name:   "should not redact url with dividers",
			Input:  "the=quick_brown+fox%jumped@over;the:lazy-dog",
			Output: "the=quick_brown+fox%jumped@over;the:lazy-dog",
		},
		{
			Name:   "should not redact url with spaces",
			Input:  "the quick brown fox jumped over the lazy dog",
			Output: "the quick brown fox jumped over the lazy dog",
		},
		{
			Name:   "should not redact url without spaces",
			Input:  "TheQuickBrownFoxJumpedOverTheLazyDog",
			Output: "TheQuickBrownFoxJumpedOverTheLazyDog",
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			got := redactPasswordFromURL(test.Input)
			assert.Equal(t, got, test.Output)
		})
	}
}

func TestKafkaBroker(t *testing.T) {
	originalConfig := Config
	originalClowderEnvConfig := os.Getenv("ACG_CONFIG")
	originalClowderLoadedConfig := clowder.LoadedConfig
	originalClowderObjectBuckets := clowder.ObjectBuckets
	originalEDGETarBallsBucket := os.Getenv("EDGETARBALLSBUCKET")

	// restore configs
	defer func(conf *EdgeConfig, clowderEnvConfig string, clowderLoadedConfig *clowder.AppConfig,
		clowderObjectBuckets map[string]clowder.ObjectStoreBucket, originalBucketName string) {
		Config = conf
		clowder.LoadedConfig = clowderLoadedConfig
		clowder.ObjectBuckets = clowderObjectBuckets
		if clowderEnvConfig == "" {
			err := os.Unsetenv("ACG_CONFIG")
			assert.NoError(t, err)
		}
		if originalBucketName == "" {
			err := os.Unsetenv("EDGETARBALLSBUCKET")
			assert.NoError(t, err)
		}

	}(originalConfig, originalClowderEnvConfig, originalClowderLoadedConfig, originalClowderObjectBuckets, originalEDGETarBallsBucket)

	err := os.Setenv("ACG_CONFIG", "need some value only, as the config path is not needed here")
	assert.NoError(t, err)

	authTypeSasl := clowder.BrokerConfigAuthtype("sasl")
	caCert := faker.UUIDHyphenated()
	publicPort := 3000

	kafkaBroker := clowder.BrokerConfig{
		Authtype: &authTypeSasl,
		Cacert:   &caCert,
	}

	kafkaBrokerWitoutCaCert := clowder.BrokerConfig{
		Authtype: &authTypeSasl,
		Cacert:   nil,
	}

	bucketName := faker.UUIDHyphenated()
	bucketAccessKey := faker.UUIDHyphenated()
	bucketSecretKey := faker.UUIDHyphenated()
	err = os.Setenv("EDGETARBALLSBUCKET", bucketName)
	assert.NoError(t, err)
	clowder.ObjectBuckets = map[string]clowder.ObjectStoreBucket{bucketName: {AccessKey: &bucketAccessKey, SecretKey: &bucketSecretKey}}

	testCases := []struct {
		Name                      string
		clowderConfig             *clowder.AppConfig
		ExpectedKafkaBroker       *clowder.BrokerConfig
		ExpectedKafkaBrokerCaCert string
	}{
		{

			Name: "should set config kafkaBroker and kafkaBrokerCaCertPath",
			clowderConfig: &clowder.AppConfig{
				PublicPort: &publicPort,
				Database:   &clowder.DatabaseConfig{},
				Logging:    clowder.LoggingConfig{Cloudwatch: &clowder.CloudWatchConfig{}},
				Kafka: &clowder.KafkaConfig{
					Brokers: []clowder.BrokerConfig{kafkaBroker},
				},
			},
			ExpectedKafkaBroker: &kafkaBroker,
		},

		{
			Name: "when Broker defined and caCert not defined, should set config kafkaBroker and not set kafkaBrokerCaCertPath",
			clowderConfig: &clowder.AppConfig{
				PublicPort: &publicPort,
				Database:   &clowder.DatabaseConfig{},
				Logging:    clowder.LoggingConfig{Cloudwatch: &clowder.CloudWatchConfig{}},
				Kafka: &clowder.KafkaConfig{
					Brokers: []clowder.BrokerConfig{kafkaBrokerWitoutCaCert},
				},
			},
			ExpectedKafkaBroker: &kafkaBrokerWitoutCaCert,
		},
		{
			Name: "when no Brokers defined, should not set config kafkaBroker and not set kafkaBrokerCaCertPath",
			clowderConfig: &clowder.AppConfig{
				PublicPort: &publicPort,
				Database:   &clowder.DatabaseConfig{},
				Logging:    clowder.LoggingConfig{Cloudwatch: &clowder.CloudWatchConfig{}},
				Kafka:      &clowder.KafkaConfig{Brokers: []clowder.BrokerConfig{}},
			},
			ExpectedKafkaBroker: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			clowder.LoadedConfig = testCase.clowderConfig
			// init the configuration
			Init()
			assert.Equal(t, Config.KafkaBroker, testCase.ExpectedKafkaBroker)
			if testCase.ExpectedKafkaBroker != nil {
				if testCase.ExpectedKafkaBroker.Cacert != nil && *testCase.ExpectedKafkaBroker.Cacert != "" {
					assert.NotEmpty(t, Config.KafkaBrokerCaCertPath)
					caCertByteContent, err := os.ReadFile(Config.KafkaBrokerCaCertPath)
					assert.NoError(t, err)
					assert.Equal(t, string(caCertByteContent), *kafkaBroker.Cacert)
				} else {
					assert.Empty(t, Config.KafkaBrokerCaCertPath)
				}
			} else {
				assert.Nil(t, Config.KafkaBroker)
				assert.Empty(t, Config.KafkaBrokerCaCertPath)
			}
		})
	}
}
