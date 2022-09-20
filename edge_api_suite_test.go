// FIXME: golangci-lint
// nolint:revive
package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestEdgeApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EdgeApi Suite")
}
