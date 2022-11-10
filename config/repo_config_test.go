// FIXME: golangci-lint
// nolint:errcheck,gosec,govet,revive,typecheck
package config

import (
	"strings"
	"testing"
)

// Validate distribution ref values should return refs by distribution"
func TestValidateRepo(t *testing.T) {
	for key, d := range DistributionsRefs {

		dist := strings.Split(key, "-")
		distribution := dist[0]
		version := strings.Split(dist[1], "")
		majorVersion := strings.Join(version[:len(version)-1], "")

		t.Run("validate distribution", func(t *testing.T) {
			if !strings.Contains(DistributionsRefs[key], distribution) {
				t.Errorf(" %q not found: %q", distribution, DistributionsRefs[d])
			}
		})

		t.Run("validate major", func(t *testing.T) {
			if !strings.Contains(DistributionsRefs[key], majorVersion) {
				t.Errorf("%q not found: %q", majorVersion, DistributionsRefs[d])
			}
		})

	}

}
func TestValidatePackages(t *testing.T) {
	for key, value := range DistributionsPackages {
		t.Run("validate package", func(t *testing.T) {
			if len(DistributionsPackages[key]) == 0 {
				t.Errorf("package not found: %q", value)
			}
		})
	}

}
func TestSupportedPackages(t *testing.T) {
	t.Run("validate size", func(t *testing.T) {
		if len(DistributionsPackages) != len(DistributionsRefs) {
			t.Errorf("packages supported %d and found %d", len(DistributionsPackages), len(DistributionsRefs))
		}
	})

}

func TestInvalidDistribution(t *testing.T) {
	if DistributionsPackages["invalid"] != nil {
		t.Errorf("expected nil,  found %v", DistributionsPackages["invalid"])
	}
	if DistributionsRefs["invalid"] != "" {
		t.Errorf("expected nil,  found %v", DistributionsPackages["invalid"])
	}
}
