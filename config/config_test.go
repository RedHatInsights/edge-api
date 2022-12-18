// FIXME: golangci-lint
// nolint:errcheck,gocritic,gosec,gosimple,govet,revive,typecheck
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitializeApplicationConfig(t *testing.T) {
	currentConfig := Config

	Init()
	assert.NotNil(t, Config)
	assert.NotEqual(t, Config, currentConfig)
}

func TestCreateNewConfig(t *testing.T) {
	localConfig, err := CreateEdgeAPIConfig()
	assert.Nil(t, err)
	assert.NotNil(t, localConfig)
}

func TestRedactPasswordFromURL(t *testing.T) {
	cases := []struct {
		Name   string
		Input  string
		Output string
	}{
		{
			Name:   "should redact password from url",
			Input:  "https://zaphod:password@example.com/?this=that&thisone=theother",
			Output: "https://zaphod:xxxxx@example.com/?this=that&thisone=theother",
		},
		{
			Name:   "should not redact password from url",
			Input:  "https://example.com/?this=that&thisone=theother",
			Output: "https://example.com/?this=that&thisone=theother",
		},
		{
			Name:   "should not redact url with dividers",
			Input:  "the=quick_brown+fox%jumped@over;the:lazy-dog",
			Output: "the=quick_brown+fox%jumped@over;the:lazy-dog",
		},
		{
			Name:   "should not redact url with spaces",
			Input:  "the quick brown fox jumped over the lazy dog",
			Output: "the quick brown fox jumped over the lazy dog",
		},
		{
			Name:   "should not redact url without spaces",
			Input:  "TheQuickBrownFoxJumpedOverTheLazyDog",
			Output: "TheQuickBrownFoxJumpedOverTheLazyDog",
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			got := redactPasswordFromURL(test.Input)
			assert.Equal(t, got, test.Output)
		})
	}
}
