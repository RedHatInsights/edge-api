package kafkacommon_test // nolint:revive

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestKafka(t *testing.T) {
	RegisterFailHandler(Fail) // nolint:gofmt,goimports,typecheck
	RunSpecs(t, "Kafka Suite") // nolint:typecheck
}
