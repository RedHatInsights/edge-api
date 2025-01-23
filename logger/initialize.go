package logger

import (
	"context"
	"os"

	"github.com/osbuild/logging/pkg/sinit"
	"github.com/osbuild/logging/pkg/strc"
	"github.com/redhatinsights/edge-api/config"
)

var HeadfieldPairs = []strc.HeadfieldPair{
	{HeaderName: "X-Request-Id", FieldName: "request_id"},
}

func InitializeLogging(ctx context.Context, cfg *config.EdgeConfig) error {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	loggingConfig := sinit.LoggingConfig{
		StdoutConfig: sinit.StdoutConfig{
			Enabled: true,
			Level:   "warning",
			Format:  "text",
		},
		CloudWatchConfig: sinit.CloudWatchConfig{
			Enabled:      cfg.Logging.Region != "" && cfg.Logging.AccessKeyID != "" && cfg.Logging.SecretAccessKey != "",
			Level:        cfg.LogLevel,
			AWSRegion:    cfg.Logging.Region,
			AWSSecret:    cfg.Logging.SecretAccessKey,
			AWSKey:       cfg.Logging.AccessKeyID,
			AWSLogGroup:  cfg.Logging.LogGroup,
			AWSLogStream: hostname,
		},
		SentryConfig: sinit.SentryConfig{
			Enabled: cfg.GlitchtipDsn != "",
			DSN:     cfg.GlitchtipDsn,
		},
		TracingConfig: sinit.TracingConfig{
			Enabled:         true,
			ContextCallback: strc.HeadfieldPairCallback(HeadfieldPairs),
		},
	}
	return sinit.InitializeLogging(ctx, loggingConfig)
}

func Flush() {
	sinit.Flush()
}
