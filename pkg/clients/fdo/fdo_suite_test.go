//go:build fdo
// +build fdo

package fdo_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFdo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FDO Suite")
}
