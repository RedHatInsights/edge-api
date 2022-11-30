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
func GetLoggerFromContext(ctx context.Context) *log.Entry {
	loggerVal, ok := ctx.Value(loggerKey).(*log.Entry)
	if !ok {
		return nil
	}

	return loggerVal
}

// ContextWithLogger adds a logger to the context
func ContextWithLogger(ctx context.Context, logger *log.Entry) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}
