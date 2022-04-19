package config

import (
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"

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
	BucketRegion             *string                   `json:"bucket_region,omitempty"`
	AccessKey                string                    `json:"-"`
	SecretKey                string                    `json:"-"`
	RepoTempPath             string                    `json:"repo_temp_path,omitempty"`
	OpenAPIFilePath          string                    `json:"openapi_file_path,omitempty"`
	ImageBuilderConfig       *imageBuilderConfig       `json:"image_builder,omitempty"`
	InventoryConfig          *inventoryConfig          `json:"inventory,omitempty"`
	DefaultOSTreeRef         string                    `json:"default_ostree_ref,omitempty"`
	PlaybookDispatcherConfig *playbookDispatcherConfig `json:"playbook_dispatcher,omitempty"`
	TemplatesPath            string                    `json:"templates_path,omitempty"`
	EdgeAPIBaseURL           string                    `json:"edge_api_base_url,omitempty"`
	UploadWorkers            int                       `json:"upload_workers,omitempty"`
	KafkaConfig              *clowder.KafkaConfig      `json:"kafka,omitempty"`
	FDO                      *fdoConfig                `json:"fdo,omitempty"`
	Local                    bool                      `json:"local,omitempty"`
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

//
type loggingConfig struct {
	AccessKeyID     string `json:"-"`
	SecretAccessKey string `json:"-"`
	LogGroup        string `json:"log_group,omitempty"`
	Region          string `json:"region,omitempty"`
}

var config *EdgeConfig

// Init configuration for service
func Init() {
	options := viper.New()
	options.SetDefault("WebPort", 3000)
	options.SetDefault("MetricsPort", 8080)
	options.SetDefault("LogLevel", "DEBUG")
	options.SetDefault("Auth", false)
	options.SetDefault("Debug", false)
	options.SetDefault("EdgeTarballsBucket", "rh-edge-tarballs")
	options.SetDefault("ImageBuilderUrl", "http://image-builder:8080")
	options.SetDefault("InventoryUrl", "http://host-inventory-service:8080/")
	options.SetDefault("PlaybookDispatcherURL", "http://playbook-dispatcher:8080/")
	options.SetDefault("PlaybookDispatcherStatusURL", "http://playbook-dispatcher:8080/")
	options.SetDefault("PlaybookDispatcherPSK", "xxxxx")
	options.SetDefault("RepoTempPath", "/tmp/repos/")
	options.SetDefault("OpenAPIFilePath", "./cmd/spec/openapi.json")
	options.SetDefault("Database", "sqlite")
	options.SetDefault("DatabaseFile", "test.db")
	options.SetDefault("DefaultOSTreeRef", "rhel/9/x86_64/edge")
	options.SetDefault("TemplatesPath", "/usr/local/etc/")
	options.SetDefault("EdgeAPIBaseURL", "http://localhost:3000")
	options.SetDefault("UploadWorkers", 100)
	options.SetDefault("FDOHostURL", "https://fdo.redhat.com")
	options.SetDefault("FDOApiVersion", "v1")
	options.SetDefault("FDOAuthorizationBearer", "lorum-ipsum")
	options.SetDefault("Local", false)
	options.AutomaticEnv()

	if options.GetBool("Debug") {
		options.Set("LogLevel", "DEBUG")
	}

	kubenv := viper.New()
	kubenv.AutomaticEnv()

	config = &EdgeConfig{
		Hostname:         kubenv.GetString("Hostname"),
		Auth:             options.GetBool("Auth"),
		WebPort:          options.GetInt("WebPort"),
		MetricsPort:      options.GetInt("MetricsPort"),
		Debug:            options.GetBool("Debug"),
		LogLevel:         options.GetString("LogLevel"),
		BucketName:       options.GetString("EdgeTarballsBucket"),
		RepoTempPath:     options.GetString("RepoTempPath"),
		OpenAPIFilePath:  options.GetString("OpenAPIFilePath"),
		DefaultOSTreeRef: options.GetString("DefaultOSTreeRef"),
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
		TemplatesPath:  options.GetString("TemplatesPath"),
		EdgeAPIBaseURL: options.GetString("EdgeAPIBaseURL"),
		UploadWorkers:  options.GetInt("UploadWorkers"),
		FDO: &fdoConfig{
			URL:                 options.GetString("FDOHostURL"),
			APIVersion:          options.GetString("FDOApiVersion"),
			AuthorizationBearer: options.GetString("FDOAuthorizationBearer"),
		},
		Local: options.GetBool("Local"),
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
		config.BucketRegion = bucket.Region
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

}

// Get returns an initialized EdgeConfig
func Get() *EdgeConfig {
	return config
}
