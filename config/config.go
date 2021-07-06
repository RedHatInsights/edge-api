package config

import (
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"

	"github.com/spf13/viper"
)

// EdgeConfig represents the runtime configuration
type EdgeConfig struct {
	Hostname           string
	Auth               bool
	WebPort            int
	MetricsPort        int
	Logging            *loggingConfig
	LogLevel           string
	Debug              bool
	Database           *dbConfig
	BucketName         string
	AccessKey          string
	SecretKey          string
	UpdateTempPath     string
	OpenAPIFilePath    string
	ImageBuilderConfig *imageBuilderConfig
}

type dbConfig struct {
	User     string
	Password string
	Hostname string
	Port     uint
	Name     string
}

type imageBuilderConfig struct {
	URL string
}

//
type loggingConfig struct {
	AccessKeyID     string
	SecretAccessKey string
	LogGroup        string
	Region          string
}

var config *EdgeConfig

// Init configuration for service
func Init() {
	options := viper.New()
	options.SetDefault("WebPort", 3000)
	options.SetDefault("MetricsPort", 8080)
	options.SetDefault("LogLevel", "INFO")
	options.SetDefault("Auth", false)
	options.SetDefault("Debug", false)
	options.SetDefault("EdgeTarballsBucket", "rh-edge-tarballs")
	options.SetDefault("ImageBuilderUrl", "http://image-builder:8080")
	options.SetDefault("UpdateTempPath", "/tmp/updates/")
	options.SetDefault("OpenAPIFilePath", "./cmd/spec/openapi.json")
	options.AutomaticEnv()

	kubenv := viper.New()
	kubenv.AutomaticEnv()

	config = &EdgeConfig{
		Hostname:       kubenv.GetString("Hostname"),
		Auth:           options.GetBool("Auth"),
		WebPort:        options.GetInt("WebPort"),
		MetricsPort:    options.GetInt("MetricsPort"),
		Debug:          options.GetBool("Debug"),
		LogLevel:       options.GetString("LogLevel"),
		BucketName:     options.GetString("EdgeTarballsBucket"),
		UpdateTempPath: options.GetString("UpdateTempPath"),
		OpenAPIFilePath: options.GetString("OpenAPIFilePath"),
		ImageBuilderConfig: &imageBuilderConfig{
			URL: options.GetString("ImageBuilderUrl"),
		},
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
		}

		bucket, _ := clowder.ObjectBuckets[config.BucketName]
		config.BucketName = bucket.RequestedName
		config.AccessKey = *bucket.AccessKey
		config.SecretKey = *bucket.SecretKey

		config.Logging = &loggingConfig{
			AccessKeyID:     cfg.Logging.Cloudwatch.AccessKeyId,
			SecretAccessKey: cfg.Logging.Cloudwatch.SecretAccessKey,
			LogGroup:        cfg.Logging.Cloudwatch.LogGroup,
			Region:          cfg.Logging.Cloudwatch.Region,
		}
	}
}

// Get returns an initialized EdgeConfig
func Get() *EdgeConfig {
	return config
}
