// FIXME: golangci-lint
// nolint:govet,revive
package logger

import (
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	lc "github.com/redhatinsights/platform-go-middlewares/v2/logging/cloudwatch"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
)

// Log is an instance of the global logrus.Logger
var logLevel log.Level

// hook is an instance of cloudwatch.hook
var hook *lc.LogrusHook

// InitLogger initializes the API logger
func InitLogger(writer io.Writer) {
	// add context hook before other hooks
	log.AddHook(&ctxHook{})

	cfg := config.Get()

	switch cfg.LogLevel {
	case "DEBUG":
		logLevel = log.DebugLevel
	case "ERROR":
		logLevel = log.ErrorLevel
	default:
		logLevel = log.InfoLevel
	}
	log.SetReportCaller(true)

	if cfg.Logging != nil && cfg.Logging.Region != "" {
		cred := credentials.NewStaticCredentials(cfg.Logging.AccessKeyID, cfg.Logging.SecretAccessKey, "")
		awsconf := aws.NewConfig().WithRegion(cfg.Logging.Region).WithCredentials(cred)
		w, err := lc.NewBatchWriterWithDuration(cfg.Logging.LogGroup, cfg.Hostname, awsconf, 10*time.Second)
		if err != nil {
			log.WithFields(log.Fields{"error": err.Error()}).Error("Error creating AWS hook")
		}
		hook = lc.NewLogrusHook(w)
		log.AddHook(hook)
		log.SetFormatter(&log.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
			FieldMap: log.FieldMap{
				log.FieldKeyTime: "@timestamp",
			},
		})
	}

	log.SetOutput(writer)
	log.SetLevel(logLevel)
}

// FlushLogger Flush batched logging messages
func FlushLogger() {
	if hook != nil {
		err := hook.Flush()
		if err != nil {
			log.WithFields(log.Fields{"error": err.Error()}).Error("Error flushing batched logging messages")
		}
	}
}

// LogErrorAndPanic Records the error, flushes the buffer, then panics the container
func LogErrorAndPanic(msg string, err error) {
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error(msg)
		FlushLogger()
		panic(err)
	}
}
