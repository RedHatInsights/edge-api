// FIXME: golangci-lint
// nolint:errcheck,gosec,govet,revive,typecheck
package config_test

import (
	"strings"
	"testing"

	"github.com/redhatinsights/edge-api/config"
)

// Validate distribution ref values should return refs by distribution"
func TestValidateRepo(t *testing.T) {
	for key, d := range config.DistributionsRefs {

		dist := strings.Split(key, "-")
		distribution := dist[0]
		version := strings.Split(dist[1], "")
		majorVersion := strings.Join(version[:len(version)-1], "")

		t.Run("validate distribution", func(t *testing.T) {
			if !strings.Contains(config.DistributionsRefs[key], distribution) {
				t.Errorf(" %q not found: %q", distribution, config.DistributionsRefs[d])
			}
		})

		t.Run("validate major", func(t *testing.T) {
			if !strings.Contains(config.DistributionsRefs[key], majorVersion) {
				t.Errorf("%q not found: %q", majorVersion, config.DistributionsRefs[d])
			}
		})

	}

}
func TestValidatePackages(t *testing.T) {
	for key, value := range config.DistributionsPackages {
		t.Run("validate package", func(t *testing.T) {
			if len(config.DistributionsPackages[key]) == 0 {
				t.Errorf("package not found: %q", value)
			}
		})
	}

}
func TestSupportedPackages(t *testing.T) {
	t.Run("validate size", func(t *testing.T) {
		if len(config.DistributionsPackages) != len(config.DistributionsRefs) {
			t.Errorf("packages supported %d and found %d", len(config.DistributionsPackages), len(config.DistributionsRefs))
		}
	})

}

func TestInvaliddistribution(t *testing.T) {
	if config.DistributionsPackages["invalid"] != nil {
		t.Errorf("expected nil,  found %v", config.DistributionsPackages["invalid"])
	}
	if config.DistributionsRefs["invalid"] != "" {
		t.Errorf("expected nil,  found %v", config.DistributionsPackages["invalid"])
	}
}
