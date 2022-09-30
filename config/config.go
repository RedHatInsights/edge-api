// FIXME: golangci-lint
// nolint:errcheck,gocritic,gosimple,govet,revive
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// EdgeConfig represents the runtime configuration
type EdgeConfig struct {
	Hostname                 string                    `json:"hostname,omitempty"`
	Auth                     bool                      `json:"auth,omitempty"`
	WebPort                  int                       `json:"web_port,omitempty"`
	MetricsPort              int                       `json:"metrics_port,omitempty"`
	Logging                  *loggingConfig            `json:"logging,omitempty"`
	LogLevel                 string                    `json:"log_level,omitempty"`
	Debug                    bool                      `json:"debug,omitempty"`
	Database                 *dbConfig                 `json:"database,omitempty"`
	BucketName               string                    `json:"bucket_name,omitempty"`
	BucketRegion             string                    `json:"bucket_region,omitempty"`
	AccessKey                string                    `json:"-" edge:"scrub"`
	SecretKey                string                    `json:"-" edge:"scrub"`
	RepoTempPath             string                    `json:"repo_temp_path,omitempty"`
	OpenAPIFilePath          string                    `json:"openapi_file_path,omitempty"`
	ImageBuilderConfig       *imageBuilderConfig       `json:"image_builder,omitempty"`
	InventoryConfig          *inventoryConfig          `json:"inventory,omitempty"`
	PlaybookDispatcherConfig *playbookDispatcherConfig `json:"playbook_dispatcher,omitempty"`
	TemplatesPath            string                    `json:"templates_path,omitempty"`
	EdgeAPIBaseURL           string                    `json:"edge_api_base_url,omitempty"`
	EdgeAPIServiceHost       string                    `json:"edge_api_service_host,omitempty"`
	EdgeAPIServicePort       int                       `json:"edge_api_service_port,omitempty"`
	UploadWorkers            int                       `json:"upload_workers,omitempty"`
	KafkaConfig              *clowder.KafkaConfig      `json:"kafka,omitempty"`
	KafkaBrokers             []clowder.BrokerConfig    `json:"kafka_brokers,omitempty"`
	KafkaTopics              map[string]string         `json:"kafka_topics,omitempty"`
	FDO                      *fdoConfig                `json:"fdo,omitempty"`
	Local                    bool                      `json:"local,omitempty"`
	Dev                      bool                      `json:"dev,omitempty"`
	UnleashURL               string                    `json:"unleash_url,omitempty"`
	UnleashSecretName        string                    `json:"unleash_secret_name,omitempty"`
	FeatureFlagsEnvironment  string                    `json:"featureflags_environment,omitempty"`
	FeatureFlagsURL          string                    `json:"featureflags_url,omitempty"`
	FeatureFlagsAPIToken     string                    `json:"featureflags_api_token,omitempty"`
	FeatureFlagsService      string                    `json:"featureflags_service,omitempty"`
	FeatureFlagsBearerToken  string                    `json:"featureflags_bearer_token,omitempty"`
	TenantTranslatorHost     string                    `json:"tenant_translator_host,omitempty"`
	TenantTranslatorPort     string                    `json:"tenant_translator_port,omitempty"`
	TenantTranslatorURL      string                    `json:"tenant_translator_url,omitempty"`
}

type dbConfig struct {
	Type     string `json:"type,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"-"`
	Hostname string `json:"hostname,omitempty"`
	Port     uint   `json:"port,omitempty"`
	Name     string `json:"name,omitempty"`
}

type fdoConfig struct {
	URL                 string `json:"url,omitempty"`
	APIVersion          string `json:"api_version,omitempty"`
	AuthorizationBearer string `json:"-"`
}

type imageBuilderConfig struct {
	URL string `json:"url,omitempty"`
}

type inventoryConfig struct {
	URL string `json:"url,omitempty"`
}

type playbookDispatcherConfig struct {
	URL    string `json:"url,omitempty"`
	PSK    string `json:"-"`
	Status string `json:"status,omitempty"`
}

// loggingConfig
type loggingConfig struct {
	AccessKeyID     string `json:"-"`
	SecretAccessKey string `json:"-"`
	LogGroup        string `json:"log_group,omitempty"`
	Region          string `json:"region,omitempty"`
}

var config *EdgeConfig

// DevConfigFile is a wrapper for local dev kafka config
type DevConfigFile struct {
	Kafka clowder.KafkaConfig
}

// Init configuration for service
func Init() {
	options := viper.New()
	options.SetDefault("WebPort", 3000)
	options.SetDefault("MetricsPort", 8080)
	options.SetDefault("LogLevel", "DEBUG")
	options.SetDefault("Auth", false)
	options.SetDefault("Debug", false)
	options.SetDefault("EdgeTarballsBucket", "rh-edge-tarballs")
	options.SetDefault("BucketRegion", "us-east-1")
	options.SetDefault("ImageBuilderUrl", "http://image-builder:8080")
	options.SetDefault("InventoryUrl", "http://host-inventory-service:8080/")
	options.SetDefault("PlaybookDispatcherURL", "http://playbook-dispatcher:8080/")
	options.SetDefault("PlaybookDispatcherStatusURL", "http://playbook-dispatcher:8080/")
	options.SetDefault("PlaybookDispatcherPSK", "xxxxx")
	options.SetDefault("RepoTempPath", "/tmp/repos/")
	options.SetDefault("OpenAPIFilePath", "./cmd/spec/openapi.json")
	options.SetDefault("Database", "sqlite")
	options.SetDefault("DatabaseFile", "test.db")
	options.SetDefault("TemplatesPath", "/usr/local/etc/")
	options.SetDefault("EdgeAPIBaseURL", "http://localhost:3000")
	options.SetDefault("EdgeAPIServiceHost", "localhost")
	options.SetDefault("EdgeAPIServicePort", "3000")
	options.SetDefault("UploadWorkers", 100)
	options.SetDefault("FDOHostURL", "https://fdo.redhat.com")
	options.SetDefault("FDOApiVersion", "v1")
	options.SetDefault("FDOAuthorizationBearer", "lorum-ipsum")
	options.SetDefault("Local", false)
	options.SetDefault("Dev", false)
	options.SetDefault("EDGEMGMT_CONFIGPATH", "/tmp/edgemgmt_config.json")
	options.AutomaticEnv()

	if options.GetBool("Debug") {
		options.Set("LOG_LEVEL", "DEBUG")
	}

	if clowder.IsClowderEnabled() {
		// FUTURE: refactor config to follow common CRC config code
		// 		see https://github.com/RedHatInsights/sources-api-go/blob/main/config/config.go
		cfg := clowder.LoadedConfig

		if cfg.FeatureFlags != nil {
			UnleashURL := ""
			if cfg.FeatureFlags.Hostname != "" && cfg.FeatureFlags.Port != 0 && cfg.FeatureFlags.Scheme != "" {
				UnleashURL = fmt.Sprintf("%s://%s:%d/api", cfg.FeatureFlags.Scheme, cfg.FeatureFlags.Hostname, cfg.FeatureFlags.Port)
			}

			options.SetDefault("FeatureFlagsUrl", UnleashURL)

			clientAccessToken := ""
			if cfg.FeatureFlags.ClientAccessToken != nil {
				clientAccessToken = *cfg.FeatureFlags.ClientAccessToken
			}
			options.SetDefault("FeatureFlagsBearerToken", clientAccessToken)
		}
	} else {
		options.SetDefault("FeatureFlagsUrl", os.Getenv("UNLEASH_URL"))
		options.SetDefault("FeatureFlagsAPIToken", os.Getenv("UNLEASH_TOKEN"))
		options.SetDefault("FeatureFlagsBearerToken", options.GetString("UNLEASH_TOKEN"))
	}
	options.SetDefault("FeatureFlagsService", os.Getenv("FEATURE_FLAGS_SERVICE"))

	if os.Getenv("SOURCES_ENV") == "prod" {
		options.SetDefault("FeatureFlagsEnvironment", "production")
	} else {
		options.SetDefault("FeatureFlagsEnvironment", "development")
	}

	// check to see if you are running in ephemeral, the unleash server in ephemeral is empty
	if strings.Contains(options.GetString("FeatureFlagsUrl"), "ephemeral") {
		options.SetDefault("FeatureFlagsEnvironment", "ephemeral")
	}

	options.SetDefault("TenantTranslatorHost", os.Getenv("TENANT_TRANSLATOR_HOST"))
	options.SetDefault("TenantTranslatorPort", os.Getenv("TENANT_TRANSLATOR_PORT"))

	config = &EdgeConfig{
		Hostname:        options.GetString("Hostname"),
		Auth:            options.GetBool("Auth"),
		WebPort:         options.GetInt("WebPort"),
		MetricsPort:     options.GetInt("MetricsPort"),
		Debug:           options.GetBool("Debug"),
		LogLevel:        options.GetString("LOG_LEVEL"),
		BucketName:      options.GetString("EdgeTarballsBucket"),
		BucketRegion:    options.GetString("BucketRegion"),
		RepoTempPath:    options.GetString("RepoTempPath"),
		OpenAPIFilePath: options.GetString("OpenAPIFilePath"),
		ImageBuilderConfig: &imageBuilderConfig{
			URL: options.GetString("ImageBuilderUrl"),
		},
		InventoryConfig: &inventoryConfig{
			URL: options.GetString("InventoryUrl"),
		},
		PlaybookDispatcherConfig: &playbookDispatcherConfig{
			URL:    options.GetString("PlaybookDispatcherURL"),
			PSK:    options.GetString("PlaybookDispatcherPSK"),
			Status: options.GetString("PlaybookDispatcherStatusURL"),
		},
		TemplatesPath:      options.GetString("TemplatesPath"),
		EdgeAPIBaseURL:     options.GetString("EdgeAPIBaseURL"),
		EdgeAPIServiceHost: options.GetString("EDGE_API_SERVICE_SERVICE_HOST"),
		EdgeAPIServicePort: options.GetInt("EDGE_API_SERVICE_SERVICE_PORT"),
		UploadWorkers:      options.GetInt("UploadWorkers"),
		FDO: &fdoConfig{
			URL:                 options.GetString("FDOHostURL"),
			APIVersion:          options.GetString("FDOApiVersion"),
			AuthorizationBearer: options.GetString("FDOAuthorizationBearer"),
		},
		Local:                   options.GetBool("Local"),
		Dev:                     options.GetBool("Dev"),
		UnleashURL:              options.GetString("FeatureFlagsUrl"),
		UnleashSecretName:       options.GetString("FeatureFlagsBearerToken"),
		FeatureFlagsEnvironment: options.GetString("FeatureFlagsEnvironment"),
		FeatureFlagsURL:         options.GetString("FeatureFlagsUrl"),
		FeatureFlagsAPIToken:    options.GetString("FeatureFlagsAPIToken"),
		FeatureFlagsBearerToken: options.GetString("FeatureFlagsBearerToken"),
		FeatureFlagsService:     options.GetString("FeatureFlagsService"),
		TenantTranslatorHost:    options.GetString("TenantTranslatorHost"),
		TenantTranslatorPort:    options.GetString("TenantTranslatorPort"),
	}
	if config.TenantTranslatorHost != "" && config.TenantTranslatorPort != "" {
		config.TenantTranslatorURL = fmt.Sprintf("http://%s:%s", config.TenantTranslatorHost, config.TenantTranslatorPort)
	}
	database := options.GetString("database")

	if database == "pgsql" {
		config.Database = &dbConfig{
			User:     options.GetString("PGSQL_USER"),
			Password: options.GetString("PGSQL_PASSWORD"),
			Hostname: options.GetString("PGSQL_HOSTNAME"),
			Port:     options.GetUint("PGSQL_PORT"),
			Name:     options.GetString("PGSQL_DATABASE"),
			Type:     "pgsql",
		}
	} else {
		config.Database = &dbConfig{
			Name: options.GetString("DatabaseFile"),
			Type: "sqlite",
		}
	}

	// TODO: consolidate this with the clowder block above and refactor to use default, etc.
	if clowder.IsClowderEnabled() {
		cfg := clowder.LoadedConfig

		config.WebPort = *cfg.PublicPort
		config.MetricsPort = cfg.MetricsPort

		config.Database = &dbConfig{
			User:     cfg.Database.Username,
			Password: cfg.Database.Password,
			Hostname: cfg.Database.Hostname,
			Port:     uint(cfg.Database.Port),
			Name:     cfg.Database.Name,
			Type:     "pgsql",
		}

		bucket := clowder.ObjectBuckets[config.BucketName]

		config.BucketName = bucket.RequestedName
		if bucket.Region != nil {
			config.BucketRegion = *bucket.Region
		}
		config.AccessKey = *bucket.AccessKey
		config.SecretKey = *bucket.SecretKey
		config.Logging = &loggingConfig{
			AccessKeyID:     cfg.Logging.Cloudwatch.AccessKeyId,
			SecretAccessKey: cfg.Logging.Cloudwatch.SecretAccessKey,
			LogGroup:        cfg.Logging.Cloudwatch.LogGroup,
			Region:          cfg.Logging.Cloudwatch.Region,
		}

		config.KafkaConfig = cfg.Kafka
	}

	// get config from file if running in developer mode
	// this is different than Local due to code in services/files.go
	if config.Dev {
		configFile := os.Getenv("EDGEMGMT_CONFIG")

		// SOMETHING CHANGED with upstream SetConfigFile or Unmarshal that caused Unmarshal
		// to freak out on mixedCase being translated to lowercase by SetConfigFile
		devConfigFile, err := os.ReadFile(configFile)
		if err != nil {
			log.WithField("error", err.Error()).Error("Error reading local dev config file")
		}
		devConfig := DevConfigFile{}
		if err := json.Unmarshal(devConfigFile, &devConfig); err != nil {
			log.WithField("error", err.Error()).Error("Dev config unmarshal error")
		}

		config.KafkaConfig = &devConfig.Kafka
	}

	if config.KafkaConfig != nil {
		config.KafkaBrokers = make([]clowder.BrokerConfig, len(config.KafkaConfig.Brokers))
		for i, b := range config.KafkaConfig.Brokers {
			config.KafkaBrokers[i] = b
		}
	}
}

// Get returns an initialized EdgeConfig
func Get() *EdgeConfig {
	if config == nil {
		var lock = &sync.Mutex{}
		lock.Lock()
		defer lock.Unlock()
		Init()
	}
	return config
}

// GetConfigValues return all configuration values that may be used for logging
func GetConfigValues() (map[string]interface{}, error) {
	var configValues map[string]interface{}
	cfgBytes, _ := json.Marshal(Get())
	if err := json.Unmarshal(cfgBytes, &configValues); err != nil {
		return configValues, err
	}
	return configValues, nil
}

// Scrub returns a map without sensitive data
// To use, add edge:"scrub" to the struct field
// e.g.,
// 	AccessKey   string   `json:"-" edge:"scrub"`
// NOTE: It currently only returns the first level as key/value pairs
func Scrub(cfg interface{}) map[string]interface{} {
	newMap := make(map[string]interface{})

	// reflect on the struct passed in
	values := reflect.ValueOf(cfg)
	// loop through the fields and ignore if the edge scrub field tag is defined
	types := values.Type()
	for i := 0; i < values.NumField(); i++ {
		if types.Field(i).Tag.Get("edge") != "scrub" {
			newMap[types.Field(i).Name] = values.Field(i)
		}
	}

	// TODO: walk the struct fields and add to the map

	return newMap
}
