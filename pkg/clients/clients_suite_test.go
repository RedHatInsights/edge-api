// FIXME: golangci-lint
// nolint:revive
package clients_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestClients(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Clients Suite")
}
