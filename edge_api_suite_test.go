// FIXME: golangci-lint
// nolint:revive,typecheck
package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEdgeApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EdgeApi Suite")
}
