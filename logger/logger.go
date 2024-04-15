// FIXME: golangci-lint
// nolint:govet,revive
package logger

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	lc "github.com/redhatinsights/platform-go-middlewares/logging/cloudwatch"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
)

// Log is an instance of the global logrus.Logger
var logLevel log.Level

// hook is an instance of cloudwatch.hook
var hook *lc.Hook

func prettyfier(f *runtime.Frame) (string, string) {
	s := strings.Split(f.Function, ".")
	funcName := s[len(s)-1]
	return funcName, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
}

type plainFormatter struct{}

// Format formats the log entry for humans (just the message or SQL if present)
func (f *plainFormatter) Format(e *log.Entry) ([]byte, error) {
	var b bytes.Buffer
	if f, ok := e.Data["sql"]; ok {
		b.WriteString(f.(string))
	} else {
		b.WriteString(e.Message)
	}
	b.WriteRune('\n')

	if f, ok := e.Data["error"]; ok {
		b.WriteString("ERROR: ")
		b.WriteString(fmt.Sprintf("%v", f))
		b.WriteRune('\n')
	}

	return b.Bytes(), nil
}

// InitLogger initializes the API logger
func InitLogger(writer io.Writer) {

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
		hook, err := lc.NewBatchingHook(cfg.Logging.LogGroup, cfg.Hostname, awsconf, 10*time.Second)
		if err != nil {
			log.WithFields(log.Fields{"error": err.Error()}).Error("Error creating AWS hook")
		}
		log.AddHook(hook)
		log.SetFormatter(&log.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
			FieldMap: log.FieldMap{
				log.FieldKeyTime: "@timestamp",
			},
			CallerPrettyfier: prettyfier,
		})
	} else {
		log.SetFormatter(&plainFormatter{})
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
