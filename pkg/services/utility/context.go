// FIXME: golangci-lint
// nolint:revive
package utility

import (
	"context"

	log "github.com/sirupsen/logrus"
)

// these type and consts are to play by the context key rules (no strings)
type loggerKeyType string

const loggerKey = loggerKeyType("logger")

// GetLoggerFromContext grabs the cumulative logger from the context
func GetLoggerFromContext(ctx context.Context) log.FieldLogger {
	loggerVal, ok := ctx.Value(loggerKey).(log.FieldLogger)
	if !ok {
		return nil
	}

	return loggerVal
}

// ContextWithLogger adds a logger to the context
func ContextWithLogger(ctx context.Context, logger log.FieldLogger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}
