package config

import (
	"os"

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
	UseClowder  bool
	Database    *dbConfig
}

type dbConfig struct {
	User     string
	Password string
	Hostname string
	Port     uint
	Name     string
}

// Get returns an initialized EdgeConfig
func Get() *EdgeConfig {

	var dbCfg *dbConfig

	options := viper.New()
	options.SetDefault("WebPort", 3000)
	options.SetDefault("MetricsPort", 8080)
	options.SetDefault("LogGroup", "platform-dev")
	options.SetDefault("LogLevel", "INFO")
	options.SetDefault("Auth", false)
	options.SetDefault("Debug", false)

	if clowder.IsClowderEnabled() {
		cfg := clowder.LoadedConfig
		options.SetDefault("WebPort", cfg.PublicPort)
		options.SetDefault("MetricsPort", cfg.MetricsPort)
		options.SetDefault("LogGroup", cfg.Logging.Cloudwatch.LogGroup)
		dbCfg = &dbConfig{
			User:     cfg.Database.Username,
			Password: cfg.Database.Password,
			Hostname: cfg.Database.Hostname,
			Port:     uint(cfg.Database.Port),
			Name:     cfg.Database.Name,
		}
	}

	options.AutomaticEnv()
	kubenv := viper.New()
	kubenv.AutomaticEnv()

	return &EdgeConfig{
		Hostname:    kubenv.GetString("Hostname"),
		Auth:        options.GetBool("Auth"),
		WebPort:     options.GetInt("WebPort"),
		MetricsPort: options.GetInt("MetricsPort"),
		Debug:       options.GetBool("Debug"),
		LogGroup:    options.GetString("LogGroup"),
		LogLevel:    options.GetString("LogLevel"),
		UseClowder:  os.Getenv("CLOWDER_ENABLED") == "true",
		Database:    dbCfg,
	}
}
