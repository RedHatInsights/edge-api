// FIXME: golangci-lint
// nolint:errcheck,gosec,govet,revive,typecheck
package config_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/config"
)

var _ = Describe("Validates repo config", func() {

	distri8 := []string{"rhel-84", "rhel-85"}
	distri8X := []string{"rhel-86", "rhel-87"}
	distri9 := []string{"rhel-90"}

	Context("Validate distribution ref values", func() {
		It("should return RHEL8 distribution refs", func() {
			for _, d := range distri8 {
				Expect(config.DistributionsRefs[d]).ToNot(BeNil())
				Expect(config.DistributionsRefs[d]).To(ContainSubstring("rhel/8/x86_64/edge"))
			}
		})
		It("should return RHEL8X distribution refs", func() {
			for _, d := range distri8X {
				Expect(config.DistributionsRefs[d]).ToNot(BeNil())
				Expect(config.DistributionsRefs[d]).To(ContainSubstring("rhel/8/x86_64/edge"))

			}
		})
		It("should return RHEL9 distribution refs", func() {
			for _, d := range distri9 {
				Expect(config.DistributionsRefs[d]).ToNot(BeNil())
				Expect(config.DistributionsRefs[d]).To(ContainSubstring("rhel/9/x86_64/edge"))

			}
		})

	})

	Context("Validate package distribution packages", func() {
		It("should return RHEL8 packages", func() {
			for _, d := range distri8 {
				Expect(config.RHEL8).To(Equal(config.DistributionsPackages[d]))
			}
		})
		It("should return RHEL8X packages", func() {
			for _, d := range distri8X {
				Expect(config.RHEL8X).To(Equal(config.DistributionsPackages[d]))
			}
		})
		It("should return RHEL9 packages", func() {
			for _, d := range distri9 {
				Expect(config.RHEL90).To(Equal(config.DistributionsPackages[d]))
			}
		})

	})

	Context("Validate all supported versions", func() {
		It("should return same size of versions", func() {
			Expect(len(config.DistributionsPackages)).To(Equal(len(distri8) + len(distri8X) + len(distri9)))
		})
	})

	Context("Invalid distribution", func() {
		It("should return null packages", func() {
			Expect(config.DistributionsPackages["invalid"]).To(BeNil())
		})
		It("should return empty for ref", func() {
			Expect(config.DistributionsRefs["invalid"]).To(BeEmpty())
		})
	})
})
