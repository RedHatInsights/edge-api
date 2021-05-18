package logger

import (
	"os"

	"github.com/redhatinsights/edge-api/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Log is an instance of the global logrus.Logger
var logLevel log.Level

// InitLogger initializes the API logger
func InitLogger() {

	cfg := config.Get()

	logconfig := viper.New()
	logconfig.SetEnvPrefix("EDGE")
	logconfig.AutomaticEnv()

	switch cfg.LogLevel {
	case "DEBUG":
		logLevel = log.DebugLevel
	case "ERROR":
		logLevel = log.ErrorLevel
	default:
		logLevel = log.InfoLevel
	}

	log.SetOutput(os.Stdout)
	log.SetLevel(logLevel)
}
