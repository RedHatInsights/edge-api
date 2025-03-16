package storage_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2" // nolint: revive
	. "github.com/onsi/gomega"    // nolint: revive
)

func TestMigrate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage Suite")
}
