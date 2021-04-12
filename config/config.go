package config

import (
	"os"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"

	"github.com/spf13/viper"
)

// EdgeConfig represents the runtime configuration
type EdgeConfig struct {
	Hostname             string
	Auth                 bool
	WebPort              int
	MetricsPort          int
	LogGroup             string
	LogLevel             string
	Debug                bool
	UseClowder           bool
}

// Get returns an initialized EdgeConfig
func Get() *EdgeConfig {

	options := viper.New()

	if os.Getenv("CLOWDER_ENABLED") == "true" {
		cfg := clowder.LoadedConfig
		options.SetDefault("WebPort", cfg.WebPort)
		options.SetDefault("MetricsPort", cfg.MetricsPort)
		options.SetDefault("LogGroup", cfg.Logging.Cloudwatch.LogGroup)
	} else {
		options.SetDefault("WebPort", 3000)
		options.SetDefault("MetricsPort", 8080)
		options.SetDefault("LogGroup", "platform-dev")
	}

	options.SetDefault("LogLevel", "INFO")
	options.SetDefault("Auth", true)
	options.SetDefault("Debug", false)
	options.AutomaticEnv()
	kubenv := viper.New()
	kubenv.AutomaticEnv()

	return &EdgeConfig{
		Hostname:             kubenv.GetString("Hostname"),
		Auth:                 options.GetBool("Auth"),
		WebPort:              options.GetInt("WebPort"),
		MetricsPort:          options.GetInt("MetricsPort"),
		Debug:                options.GetBool("Debug"),
		LogGroup:             options.GetString("LogGroup"),
		LogLevel:             options.GetString("LogLevel"),
		UseClowder:           os.Getenv("CLOWDER_ENABLED") == "true",
	}
}
