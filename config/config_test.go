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
	currentConfig := config

	Init()
	assert.NotNil(t, config)
	assert.NotEqual(t, config, currentConfig)
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
	originalConfig := config
	originalClowderEnvConfig := os.Getenv("ACG_CONFIG")
	originalClowderLoadedConfig := clowder.LoadedConfig
	originalClowderObjectBuckets := clowder.ObjectBuckets
	originalEDGETarBallsBucket := os.Getenv("EDGETARBALLSBUCKET")

	// restore configs
	defer func(conf *EdgeConfig, clowderEnvConfig string, clowderLoadedConfig *clowder.AppConfig,
		clowderObjectBuckets map[string]clowder.ObjectStoreBucket, originalBucketName string) {
		config = conf
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

	defer cleanup()

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
		KafkaServers              []string
		ExpectedKafkaBroker       *clowder.BrokerConfig
		ExpectedKafkaBrokerCaCert string
		ExpectedKafkaServers      string
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

			ExpectedKafkaBroker:  &kafkaBroker,
			KafkaServers:         []string{"kafka-1.example.com:9099", "kafka-2.example.com:9099"},
			ExpectedKafkaServers: "kafka-1.example.com:9099,kafka-2.example.com:9099",
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
			clowder.KafkaServers = testCase.KafkaServers
			// init the configuration
			cleanup()
			Init()
			defer cleanup()
			assert.Equal(t, config.KafkaBroker, testCase.ExpectedKafkaBroker)
			assert.Equal(t, config.KafkaServers, testCase.ExpectedKafkaServers)
			if testCase.ExpectedKafkaBroker != nil {
				if testCase.ExpectedKafkaBroker.Cacert != nil && *testCase.ExpectedKafkaBroker.Cacert != "" {
					assert.NotEmpty(t, config.KafkaBrokerCaCertPath)
					caCertByteContent, err := os.ReadFile(config.KafkaBrokerCaCertPath)
					assert.NoError(t, err)
					assert.Equal(t, string(caCertByteContent), *kafkaBroker.Cacert)
				} else {
					assert.Empty(t, config.KafkaBrokerCaCertPath)
				}
			} else {
				assert.Nil(t, config.KafkaBroker)
				assert.Empty(t, config.KafkaBrokerCaCertPath)
			}
		})
	}
}

func TestContentSourcesURL(t *testing.T) {
	contentSourceURLEnvName := "CONTENT_SOURCES_URL"
	initialContentSourceEnv := os.Getenv(contentSourceURLEnvName)

	// restore initial content source env value
	defer func(envName, envValue string) {
		err := os.Setenv(envName, envValue)
		assert.NoError(t, err)
	}(contentSourceURLEnvName, initialContentSourceEnv)

	expectedContentSourcesURl := faker.URL()
	err := os.Setenv(contentSourceURLEnvName, expectedContentSourcesURl)
	assert.NoError(t, err)

	conf, err := CreateEdgeAPIConfig()
	assert.NoError(t, err)
	assert.Equal(t, expectedContentSourcesURl, conf.ContentSourcesURL)
}

func TestSubscriptionBaseURL(t *testing.T) {
	subscriptionBaseURLEnvName := "SUBSCRIPTION_BASE_URL"
	initialSubscriptionBaseURLEnv := os.Getenv(subscriptionBaseURLEnvName)

	// restore initial content source env value
	defer func(envName, envValue string) {
		err := os.Setenv(envName, envValue)
		assert.NoError(t, err)
	}(subscriptionBaseURLEnvName, initialSubscriptionBaseURLEnv)

	expectedSubscriptionBaseURL := faker.URL()
	err := os.Setenv(subscriptionBaseURLEnvName, expectedSubscriptionBaseURL)
	assert.NoError(t, err)

	conf, err := CreateEdgeAPIConfig()
	assert.NoError(t, err)
	assert.Equal(t, expectedSubscriptionBaseURL, conf.SubscriptionBaseUrl)
}

func TestSubscriptionServerURL(t *testing.T) {
	subscriptionServerURLEnvName := "SUBSCRIPTION_SERVER_URL"
	initialSubscriptionServerURLEnv := os.Getenv(subscriptionServerURLEnvName)

	// restore initial content source env value
	defer func(envName, envValue string) {
		err := os.Setenv(envName, envValue)
		assert.NoError(t, err)
	}(subscriptionServerURLEnvName, initialSubscriptionServerURLEnv)

	expectedSubscriptionServerURL := faker.URL()
	err := os.Setenv(subscriptionServerURLEnvName, expectedSubscriptionServerURL)
	assert.NoError(t, err)

	conf, err := CreateEdgeAPIConfig()
	assert.NoError(t, err)
	assert.Equal(t, expectedSubscriptionServerURL, conf.SubscriptionServerURL)
}

func TestTLSCAPath(t *testing.T) {
	// restore initial clowder config
	defer func(clowderLoadedConfig *clowder.AppConfig) {
		clowder.LoadedConfig = clowderLoadedConfig
		err := os.Unsetenv("ACG_CONFIG")
		assert.NoError(t, err)
	}(clowder.LoadedConfig)

	publicPort := 3000
	bucketName := faker.UUIDHyphenated()
	bucketAccessKey := faker.UUIDHyphenated()
	bucketSecretKey := faker.UUIDHyphenated()
	err := os.Setenv("EDGETARBALLSBUCKET", bucketName)
	assert.NoError(t, err)
	clowder.ObjectBuckets = map[string]clowder.ObjectStoreBucket{bucketName: {AccessKey: &bucketAccessKey, SecretKey: &bucketSecretKey}}

	expectedTlsCAPath := "/tmp/tls_path.txt"

	clowderConfig := &clowder.AppConfig{
		Database:   &clowder.DatabaseConfig{},
		Logging:    clowder.LoggingConfig{Cloudwatch: &clowder.CloudWatchConfig{}},
		TlsCAPath:  &expectedTlsCAPath,
		PublicPort: &publicPort,
	}
	clowder.LoadedConfig = clowderConfig

	err = os.Setenv("ACG_CONFIG", "True")
	assert.NoError(t, err)

	conf, err := CreateEdgeAPIConfig()
	assert.NoError(t, err)
	assert.Equal(t, expectedTlsCAPath, conf.TlsCAPath)
}
