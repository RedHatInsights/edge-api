// Package config sets up the application configuration from env, file, etc.
// FIXME: golangci-lint
// nolint:errcheck,gocritic,gosimple,govet,revive
package config

import (
	"encoding/json"
	"fmt"
	"net/url"
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
	Hostname                   string                    `json:"hostname,omitempty"`
	Auth                       bool                      `json:"auth,omitempty"`
	WebPort                    int                       `json:"web_port,omitempty"`
	MetricsPort                int                       `json:"metrics_port,omitempty"`
	MetricsBaseURL             string                    `json:"metrics_api_base_url,omitempty"`
	Logging                    *loggingConfig            `json:"logging,omitempty"`
	LogLevel                   string                    `json:"log_level,omitempty"`
	Debug                      bool                      `json:"debug,omitempty"`
	Database                   *dbConfig                 `json:"database,omitempty"`
	BucketName                 string                    `json:"bucket_name,omitempty"`
	BucketRegion               string                    `json:"bucket_region,omitempty"`
	AccessKey                  string                    `json:"-"`
	SecretKey                  string                    `json:"-"`
	AWSToken                   string                    `json:"-"`
	RepoTempPath               string                    `json:"repo_temp_path,omitempty"`
	OpenAPIFilePath            string                    `json:"openapi_file_path,omitempty"`
	ImageBuilderConfig         *imageBuilderConfig       `json:"image_builder,omitempty"`
	InventoryConfig            *inventoryConfig          `json:"inventory,omitempty"`
	PlaybookDispatcherConfig   *playbookDispatcherConfig `json:"playbook_dispatcher,omitempty"`
	TemplatesPath              string                    `json:"templates_path,omitempty"`
	EdgeAPIBaseURL             string                    `json:"edge_api_base_url,omitempty"`
	EdgeCertAPIBaseURL         string                    `json:"edge_cert_api_base_url,omitempty"`
	EdgeAPIServiceHost         string                    `json:"edge_api_service_host,omitempty"`
	EdgeAPIServicePort         int                       `json:"edge_api_service_port,omitempty"`
	UploadWorkers              int                       `json:"upload_workers,omitempty"`
	KafkaConfig                *clowder.KafkaConfig      `json:"kafka,omitempty"`
	KafkaBrokers               []clowder.BrokerConfig    `json:"kafka_brokers,omitempty"`
	KafkaServers               string                    `json:"kafka_servers,omitempty"`
	KafkaBroker                *clowder.BrokerConfig     `json:"kafka_broker,omitempty"`
	KafkaBrokerCaCertPath      string                    `json:"kafka_broker_ca_cert_path,omitempty"`
	KafkaRequestRequiredAcks   int                       `json:"kafka_request_required_acks,omitempty"`
	KafkaMessageSendMaxRetries int                       `json:"kafka_message_send_max_retries,omitempty"`
	KafkaRetryBackoffMs        int                       `json:"kafka_retry_backoff_ms,omitempty"`
	KafkaTopics                map[string]string         `json:"kafka_topics,omitempty"`
	FDO                        *fdoConfig                `json:"fdo,omitempty"`
	Local                      bool                      `json:"local,omitempty"`
	Dev                        bool                      `json:"dev,omitempty"`
	UnleashURL                 string                    `json:"unleash_url,omitempty"`
	UnleashSecretName          string                    `json:"unleash_secret_name,omitempty"`
	FeatureFlagsEnvironment    string                    `json:"featureflags_environment,omitempty"`
	FeatureFlagsURL            string                    `json:"featureflags_url,omitempty"`
	FeatureFlagsAPIToken       string                    `json:"featureflags_api_token,omitempty"`
	FeatureFlagsService        string                    `json:"featureflags_service,omitempty"`
	FeatureFlagsBearerToken    string                    `json:"featureflags_bearer_token,omitempty"`
	ContentSourcesURL          string                    `json:"content_sources_url,omitempty"`
	TenantTranslatorHost       string                    `json:"tenant_translator_host,omitempty"`
	TenantTranslatorPort       string                    `json:"tenant_translator_port,omitempty"`
	TenantTranslatorURL        string                    `json:"tenant_translator_url,omitempty"`
	ImageBuilderOrgID          string                    `json:"image_builder_org_id,omitempty"`
	GpgVerify                  string                    `json:"gpg_verify,omitempty"`
	GlitchtipDsn               string                    `json:"glitchtip_dsn,omitempty"`
	HTTPClientTimeout          int                       `json:"HTTP_client_timeout,omitempty"`
	TlsCAPath                  string                    `json:"Tls_CA_path,omitempty"`
	RepoFileUploadAttempts     uint                      `json:"repo_file_upload_attempts"`
	RepoFileUploadDelay        uint                      `json:"repo_file_upload_delay"`
	DeleteFilesAttempts        uint                      `json:"delete_files_attempts"`
	DeleteFilesRetryDelay      uint                      `json:"delete_files_retry_delay"`
	RbacBaseURL                string                    `json:"rbac_base_url"`
	RbacTimeout                uint                      `mapstructure:"rbac_timeout,omitempty"`
	SubscriptionServerURL      string                    `json:"subscription_server_url"`
	SubscriptionBaseUrl        string                    `json:"subscription_base_url"`
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
	AWSToken        string `json:"-"`
	LogGroup        string `json:"log_group,omitempty"`
	Region          string `json:"region,omitempty"`
}

var Config *EdgeConfig

// DevConfigFile is a wrapper for local dev kafka edgeConfig
type DevConfigFile struct {
	Kafka clowder.KafkaConfig
}

// CreateEdgeAPIConfig create a new configuration for Edge API
func CreateEdgeAPIConfig() (*EdgeConfig, error) {
	options := viper.New()
	options.SetDefault("WebPort", 3000)
	options.SetDefault("MetricsPort", 8080)
	options.SetDefault("MetricsBaseURL", "http://localhost")
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
	options.SetDefault("EdgeCertAPIBaseURL", "http://cert.localhost:3000")
	options.SetDefault("EdgeAPIServiceHost", "localhost")
	options.SetDefault("ContentSourcesURL", "http://content-sources:8000")
	options.SetDefault("EdgeAPIServicePort", "3000")
	options.SetDefault("UploadWorkers", 100)
	options.SetDefault("FDOHostURL", "https://fdo.redhat.com")
	options.SetDefault("FDOApiVersion", "v1")
	options.SetDefault("FDOAuthorizationBearer", "lorum-ipsum")
	options.SetDefault("Local", false)
	options.SetDefault("Dev", false)
	options.SetDefault("EDGEMGMT_CONFIGPATH", "/tmp/edgemgmt_config.json")
	options.SetDefault("KafkaRequestRequiredAcks", -1)
	options.SetDefault("KafkaMessageSendMaxRetries", 15)
	options.SetDefault("KafkaRetryBackoffMs", 100)
	options.SetDefault("HTTPClientTimeout", 30)
	options.SetDefault("TlsCAPath", "/tmp/tls_path.txt")
	options.SetDefault("RepoFileUploadAttempts", 3)
	options.SetDefault("RepoFileUploadDelay", 1)
	options.SetDefault("DeleteFilesAttempts", 10)
	options.SetDefault("DeleteFilesRetryDelay", 5)
	options.SetDefault("RBAC_BASE_URL", "http://rbac-service:8080")
	options.SetDefault("RbacTimeout", 30)
	options.AutomaticEnv()

	if options.GetBool("Debug") {
		options.Set("LOG_LEVEL", "DEBUG")
	}

	if clowder.IsClowderEnabled() {
		// FUTURE: refactor edgeConfig to follow common CRC edgeConfig code
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

	options.SetDefault("GpgVerify", "false")
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

	edgeConfig := &EdgeConfig{
		Hostname:          options.GetString("Hostname"),
		Auth:              options.GetBool("Auth"),
		WebPort:           options.GetInt("WebPort"),
		MetricsPort:       options.GetInt("MetricsPort"),
		MetricsBaseURL:    options.GetString("MetricsBaseURL"),
		Debug:             options.GetBool("Debug"),
		LogLevel:          options.GetString("LOG_LEVEL"),
		BucketName:        options.GetString("EdgeTarballsBucket"),
		BucketRegion:      options.GetString("BucketRegion"),
		RepoTempPath:      options.GetString("RepoTempPath"),
		OpenAPIFilePath:   options.GetString("OpenAPIFilePath"),
		HTTPClientTimeout: options.GetInt("HTTPClientTimeout"),
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
		EdgeCertAPIBaseURL: options.GetString("EdgeCertAPIBaseURL"),
		EdgeAPIServiceHost: options.GetString("EDGE_API_SERVICE_SERVICE_HOST"),
		EdgeAPIServicePort: options.GetInt("EDGE_API_SERVICE_SERVICE_PORT"),
		UploadWorkers:      options.GetInt("UploadWorkers"),
		FDO: &fdoConfig{
			URL:                 options.GetString("FDOHostURL"),
			APIVersion:          options.GetString("FDOApiVersion"),
			AuthorizationBearer: options.GetString("FDOAuthorizationBearer"),
		},
		Local:                      options.GetBool("Local"),
		Dev:                        options.GetBool("Dev"),
		UnleashURL:                 options.GetString("FeatureFlagsUrl"),
		UnleashSecretName:          options.GetString("FeatureFlagsBearerToken"),
		FeatureFlagsEnvironment:    options.GetString("FeatureFlagsEnvironment"),
		FeatureFlagsURL:            options.GetString("FeatureFlagsUrl"),
		FeatureFlagsAPIToken:       options.GetString("FeatureFlagsAPIToken"),
		FeatureFlagsBearerToken:    options.GetString("FeatureFlagsBearerToken"),
		FeatureFlagsService:        options.GetString("FeatureFlagsService"),
		TenantTranslatorHost:       options.GetString("TenantTranslatorHost"),
		ContentSourcesURL:          options.GetString("CONTENT_SOURCES_URL"),
		TenantTranslatorPort:       options.GetString("TenantTranslatorPort"),
		ImageBuilderOrgID:          options.GetString("ImageBuilderOrgID"),
		KafkaRequestRequiredAcks:   options.GetInt("KafkaRequestRequiredAcks"),
		KafkaMessageSendMaxRetries: options.GetInt("KafkaMessageSendMaxRetries"),
		KafkaRetryBackoffMs:        options.GetInt("KafkaRetryBackoffMs"),
		GpgVerify:                  options.GetString("GpgVerify"),
		GlitchtipDsn:               options.GetString("GlitchtipDsn"),
		TlsCAPath:                  options.GetString("/tmp/tls_path.txt"),
		RepoFileUploadAttempts:     options.GetUint("RepoFileUploadAttempts"),
		DeleteFilesAttempts:        options.GetUint("DeleteFilesAttempts"),
		DeleteFilesRetryDelay:      options.GetUint("DeleteFilesRetryDelay"),
		RbacBaseURL:                options.GetString("RBAC_BASE_URL"),
		RbacTimeout:                options.GetUint("RbacTimeout"),
		SubscriptionServerURL:      options.GetString("SUBSCRIPTION_SERVER_URL"),
		SubscriptionBaseUrl:        options.GetString("SUBSCRIPTION_BASE_URL"),
	}
	if edgeConfig.TenantTranslatorHost != "" && edgeConfig.TenantTranslatorPort != "" {
		edgeConfig.TenantTranslatorURL = fmt.Sprintf("http://%s:%s", edgeConfig.TenantTranslatorHost, edgeConfig.TenantTranslatorPort)
	}
	database := options.GetString("database")

	if database == "pgsql" {
		edgeConfig.Database = &dbConfig{
			User:     options.GetString("PGSQL_USER"),
			Password: options.GetString("PGSQL_PASSWORD"),
			Hostname: options.GetString("PGSQL_HOSTNAME"),
			Port:     options.GetUint("PGSQL_PORT"),
			Name:     options.GetString("PGSQL_DATABASE"),
			Type:     "pgsql",
		}
	} else {
		edgeConfig.Database = &dbConfig{
			Name: options.GetString("DatabaseFile"),
			Type: "sqlite",
		}
	}

	// TODO: consolidate this with the clowder block above and refactor to use default, etc.
	if clowder.IsClowderEnabled() {
		cfg := clowder.LoadedConfig
		if cfg.TlsCAPath != nil {
			edgeConfig.TlsCAPath = *cfg.TlsCAPath
		}

		edgeConfig.WebPort = *cfg.PublicPort
		edgeConfig.MetricsPort = cfg.MetricsPort

		edgeConfig.Database = &dbConfig{
			User:     cfg.Database.Username,
			Password: cfg.Database.Password,
			Hostname: cfg.Database.Hostname,
			Port:     uint(cfg.Database.Port),
			Name:     cfg.Database.Name,
			Type:     "pgsql",
		}

		bucket := clowder.ObjectBuckets[edgeConfig.BucketName]

		edgeConfig.BucketName = bucket.RequestedName
		if bucket.Region != nil {
			edgeConfig.BucketRegion = *bucket.Region
		}
		edgeConfig.AccessKey = *bucket.AccessKey
		edgeConfig.SecretKey = *bucket.SecretKey
		edgeConfig.Logging = &loggingConfig{
			AccessKeyID:     cfg.Logging.Cloudwatch.AccessKeyId,
			SecretAccessKey: cfg.Logging.Cloudwatch.SecretAccessKey,
			LogGroup:        cfg.Logging.Cloudwatch.LogGroup,
			Region:          cfg.Logging.Cloudwatch.Region,
		}

		edgeConfig.KafkaConfig = cfg.Kafka
		edgeConfig.KafkaServers = strings.Join(clowder.KafkaServers, ",")
	}

	// get edgeConfig from file if running in developer mode
	// this is different than Local due to code in services/files.go
	if edgeConfig.Dev {
		configFile := os.Getenv("EDGEMGMT_CONFIG")

		// SOMETHING CHANGED with upstream SetConfigFile or Unmarshal that caused Unmarshal
		// to freak out on mixedCase being translated to lowercase by SetConfigFile
		devConfigFile, err := os.ReadFile(configFile)
		if err != nil {
			log.WithField("error", err.Error()).Error("Error reading local dev edgeConfig file")
		}
		devConfig := DevConfigFile{}
		if err := json.Unmarshal(devConfigFile, &devConfig); err != nil {
			log.WithField("error", err.Error()).Error("Dev edgeConfig unmarshal error")
		}

		edgeConfig.KafkaConfig = &devConfig.Kafka
	}

	if edgeConfig.KafkaConfig != nil {
		edgeConfig.KafkaBrokers = make([]clowder.BrokerConfig, len(edgeConfig.KafkaConfig.Brokers))
		for i, b := range edgeConfig.KafkaConfig.Brokers {
			edgeConfig.KafkaBrokers[i] = b
		}

		if len(edgeConfig.KafkaBrokers) > 0 {
			if edgeConfig.KafkaBroker == nil {
				// the config KafkaBroker is the first kafka broker
				edgeConfig.KafkaBroker = &edgeConfig.KafkaBrokers[0]
			}
		}

		// write the first kafka broker caCert if defined
		if edgeConfig.KafkaBroker != nil && edgeConfig.KafkaBroker.Cacert != nil && *edgeConfig.KafkaBroker.Cacert != "" {
			caCertFilePath, err := clowder.LoadedConfig.KafkaCa(*edgeConfig.KafkaBroker)
			if err != nil {
				log.WithField("error", err.Error()).Error("clowder failed to write first broker caCert to file")
				// continue anyway
			} else {
				edgeConfig.KafkaBrokerCaCertPath = caCertFilePath
			}
		}
	}

	return edgeConfig, nil
}

// Init configuration for service
func Init() {
	newConfig, err := CreateEdgeAPIConfig()
	if err != nil {
		return
	}
	Config = newConfig
}

// Get returns an initialized EdgeConfig
func Get() *EdgeConfig {
	if Config == nil {
		var lock = &sync.Mutex{}
		lock.Lock()
		defer lock.Unlock()
		Init()
	}
	return Config
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

// redactPasswordFromURL replaces passwords from URLs.
func redactPasswordFromURL(value string) string {
	parsedUrl, err := url.Parse(value)
	if err == nil && parsedUrl.Host != "" && parsedUrl.Scheme != "" {
		value = parsedUrl.Redacted()
	}

	return value
}

// LogConfigAtStartup logs specific edgeConfig fields at startup
func LogConfigAtStartup(cfg *EdgeConfig) {
	// Add EdgeConfig struct fields we want to log at app startup.
	// This does not walk multi-level types.
	// Instead, list dot format field paths as key.
	allowedConfig := map[string]interface{}{
		"Hostname":                 cfg.Hostname,
		"Auth":                     cfg.Auth,
		"WebPort":                  cfg.WebPort,
		"MetricsPort":              cfg.MetricsPort,
		"MetricsBaseURL":           cfg.MetricsBaseURL,
		"LogLevel":                 cfg.LogLevel,
		"Debug":                    cfg.Debug,
		"BucketName":               cfg.BucketName,
		"BucketRegion":             cfg.BucketRegion,
		"AWSAccessKey":             cfg.AccessKey,
		"AWSSecretKey":             cfg.SecretKey,
		"AWSToken":                 cfg.AWSToken,
		"RepoTempPath ":            cfg.RepoTempPath,
		"OpenAPIFilePath ":         cfg.OpenAPIFilePath,
		"ImageBuilderURL":          cfg.ImageBuilderConfig.URL,
		"InventoryURL":             cfg.InventoryConfig.URL,
		"PlaybookDispatcherConfig": cfg.PlaybookDispatcherConfig.URL,
		"TemplatesPath":            cfg.TemplatesPath,
		"DatabaseType":             cfg.Database.Type,
		"DatabaseName":             cfg.Database.Name,
		"EdgeAPIURL":               cfg.EdgeAPIBaseURL,
		"EdgeAPIServiceHost":       cfg.EdgeAPIServiceHost,
		"EdgeAPIServicePort":       cfg.EdgeAPIServicePort,
		"EdgeCertAPIURL":           cfg.EdgeCertAPIBaseURL,
		"ImageBuilderOrgID":        cfg.ImageBuilderOrgID,
		"GlitchtipDsn":             cfg.GlitchtipDsn,
		"ContentSourcesURL":        cfg.ContentSourcesURL,
		"TlsCAPath":                cfg.TlsCAPath,
		"RepoFileUploadAttempts":   cfg.RepoFileUploadAttempts,
		"RepoFileUploadDelay":      cfg.RepoFileUploadDelay,
		"UploadWorkers":            cfg.UploadWorkers,
	}

	// loop through the key/value pairs
	for k, v := range allowedConfig {
		value := reflect.ValueOf(v)
		if value.Kind() == reflect.String {
			v = redactPasswordFromURL(value.String())
		}

		log.WithField(k, v).Info("Startup configuration values")
	}
}
