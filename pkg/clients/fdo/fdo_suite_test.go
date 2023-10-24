//go:build fdo
// +build fdo

package fdo_test

import (
	"testing"

	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
)

func TestFdo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FDO Suite")
}
