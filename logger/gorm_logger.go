package logger

import (
	"context"
	"time"

	"github.com/osbuild/logging/pkg/logrus"
	"github.com/redhatinsights/platform-go-middlewares/v2/request_id"
	glogger "gorm.io/gorm/logger"
)

// GormLogger is a custom logger for GORM
type GormLogger struct {
	logger logrus.Logger
}

// NewGormLogger creates a new instance of GormLogger
func NewGormLogger(logger logrus.Logger) *GormLogger {
	return &GormLogger{
		logger: logger,
	}
}

// LogMode sets the log mode for GORM
func (l *GormLogger) LogMode(_ glogger.LogLevel) glogger.Interface {
	return l
}

// Info logs an info message
func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	rid := request_id.GetReqID(ctx)
	l.logger.WithFields(logrus.Fields{
		"request_id": rid,
	}).Debugf(msg, data...)
}

// Warn logs a warning message
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	rid := request_id.GetReqID(ctx)
	l.logger.WithFields(logrus.Fields{
		"request_id": rid,
	}).Warnf(msg, data...)
}

// Error logs an error message
func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	rid := request_id.GetReqID(ctx)
	l.logger.WithFields(logrus.Fields{
		"request_id": rid,
	}).Errorf(msg, data...)
}

// Trace logs a SQL query
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	rid := request_id.GetReqID(ctx)
	elapsed := time.Since(begin)
	sql, rows := fc()
	if err != nil {
		l.logger.WithContext(ctx).WithError(err).WithFields(logrus.Fields{
			"latency_ms": elapsed.Milliseconds(),
			"rows":       rows,
			"sql":        sql,
			"request_id": rid,
		}).Warn("SQL failed")
	} else {
		latency := elapsed.Milliseconds()
		lg := l.logger.WithContext(ctx).WithFields(logrus.Fields{
			"latency_ms": latency,
			"rows":       rows,
			"sql":        sql,
			"request_id": rid,
		})
		if latency > 2000 {
			lg.Warn("Slow SQL query")
		} else {
			lg.Trace("SQL query")
		}

	}
}
