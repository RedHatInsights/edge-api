// FIXME: golangci-lint
// nolint:revive
package logger_test

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/redhatinsights/edge-api/logger"

	log "github.com/sirupsen/logrus"
)

var _ = Describe("Logger", func() {
	Context("Flush log messages", func() {
		Specify("Test flushing log messages works without error", func() {
			// Create a sample log message
			log.Trace("Test flushing log messages")

			logger.FlushLogger()
		})
	})
})
