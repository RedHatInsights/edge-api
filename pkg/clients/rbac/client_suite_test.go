package rbac_test

import (
	"testing"

	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
)

func TestRbacClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rbac Client Suite")
}
