package kafkacommon_test // nolint:revive

import (
	"testing"

	. "github.com/onsi/ginkgo/v2" // nolint: revive
	. "github.com/onsi/gomega"    // nolint: revive
)

func TestKafka(t *testing.T) {
	RegisterFailHandler(Fail)  // nolint:gofmt,goimports,typecheck
	RunSpecs(t, "Kafka Suite") // nolint:typecheck
}
