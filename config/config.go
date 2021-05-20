package config

import (
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"

	"github.com/spf13/viper"
)

// EdgeConfig represents the runtime configuration
type EdgeConfig struct {
	Hostname    string
	Auth        bool
	WebPort     int
	MetricsPort int
	LogGroup    string
	LogLevel    string
	Debug       bool
	Database    *dbConfig
}

type dbConfig struct {
	User     string
	Password string
	Hostname string
	Port     uint
	Name     string
}

var config *EdgeConfig

func Init() {
	options := viper.New()
	options.SetDefault("WebPort", 3000)
	options.SetDefault("MetricsPort", 8080)
	options.SetDefault("LogGroup", "platform-dev")
	options.SetDefault("LogLevel", "INFO")
	options.SetDefault("Auth", false)
	options.SetDefault("Debug", false)
	options.AutomaticEnv()

	kubenv := viper.New()
	kubenv.AutomaticEnv()

	config = &EdgeConfig{
		Hostname:    kubenv.GetString("Hostname"),
		Auth:        options.GetBool("Auth"),
		WebPort:     options.GetInt("WebPort"),
		MetricsPort: options.GetInt("MetricsPort"),
		Debug:       options.GetBool("Debug"),
		LogGroup:    options.GetString("LogGroup"),
		LogLevel:    options.GetString("LogLevel"),
	}

	if clowder.IsClowderEnabled() {
		cfg := clowder.LoadedConfig

		config.WebPort = *cfg.PublicPort
		config.MetricsPort = cfg.MetricsPort
		config.LogGroup = cfg.Logging.Cloudwatch.LogGroup

		config.Database = &dbConfig{
			User:     cfg.Database.Username,
			Password: cfg.Database.Password,
			Hostname: cfg.Database.Hostname,
			Port:     uint(cfg.Database.Port),
			Name:     cfg.Database.Name,
		}
	}
}

// Get returns an initialized EdgeConfig
func Get() *EdgeConfig {
	return config
}
