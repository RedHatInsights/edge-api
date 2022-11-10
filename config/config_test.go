// Package config sets up the application configuration from env, file, etc.
// FIXME: golangci-lint
// nolint:errcheck,gocritic,gosec,gosimple,govet,revive,typecheck
package config

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {

})

func TestRedactPasswordFromURL(t *testing.T) {
	g := NewGomegaWithT(t)

	urlWithPassword := "https://zaphod:password@example.com/?this=that&thisone=theother"
	urlWithoutPassword := "https://example.com/?this=that&thisone=theother"
	stringNotURLWithDividers := "the=quick_brown+fox%jumped@over;the:lazy-dog"
	stringNotURLWithSpaces := "the quick brown fox jumped over the lazy dog"
	stringNotURLWithoutSpaces := "TheQuickBrownFoxJumpedOverTheLazyDog"

	g.Expect(redactPasswordFromURL(urlWithPassword)).To(Equal("https://zaphod:xxxxx@example.com/?this=that&thisone=theother"),
		"URL with password does not match expected output")
	g.Expect(redactPasswordFromURL(urlWithoutPassword)).To(Equal(urlWithoutPassword),
		"URL without password does not match expected output")
	g.Expect(redactPasswordFromURL(stringNotURLWithDividers)).To(Equal(stringNotURLWithDividers),
		"Non URL-formatted string does not match expected output")
	g.Expect(redactPasswordFromURL(stringNotURLWithSpaces)).To(Equal(stringNotURLWithSpaces),
		"Non URL-formatted string does not match expected output")
	g.Expect(redactPasswordFromURL(stringNotURLWithoutSpaces)).To(Equal(stringNotURLWithoutSpaces),
		"Non URL-formatted string does not match expected output")
}
