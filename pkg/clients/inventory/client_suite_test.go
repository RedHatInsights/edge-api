package inventory_test

import (
	"testing"

	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
)

func TestInventoryClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Inventory Client Suite")
}
