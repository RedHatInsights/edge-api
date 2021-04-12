package logger

import (
	"os"

	"github.com/redhatinsights/edge-api/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Log is an instance of the global logrus.Logger
var Log *logrus.Logger
var logLevel logrus.Level


// InitLogger initializes the API logger
func InitLogger() *logrus.Logger {

	cfg := config.Get()
	logconfig := viper.New()
	logconfig.SetEnvPrefix("EDGE")
	logconfig.AutomaticEnv()

	switch cfg.LogLevel {
	case "DEBUG":
		logLevel = logrus.DebugLevel
	case "ERROR":
		logLevel = logrus.ErrorLevel
	default:
		logLevel = logrus.InfoLevel
	}

	Log = &logrus.Logger{
		Out:          os.Stdout,
		Level:        logLevel,
		Hooks:        make(logrus.LevelHooks),
		ReportCaller: true,
	}

	return Log
}
