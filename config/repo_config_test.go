// FIXME: golangci-lint
// nolint:errcheck,gosec,govet,revive,typecheck
package config_test

import (
	"strings"
	"testing"

	"github.com/redhatinsights/edge-api/config"
)

//Validate distribution ref values should return refs by distribution"
func TestValidateRepo(t *testing.T) {
	supportedDistributions := []string{"rhel-84", "rhel-85", "rhel-86", "rhel-87", "rhel-90"}

	for _, d := range supportedDistributions {
		dist := strings.Split(d, "-")
		distribution := dist[0]
		version := strings.Split(dist[1], "")
		majorVersion := strings.Join(version[:len(version)-1], "")

		if !strings.Contains(config.DistributionsRefs[d], distribution) {
			t.Errorf(" %q not found: %q", distribution, config.DistributionsRefs[d])
		}

		if !strings.Contains(config.DistributionsRefs[d], majorVersion) {
			t.Errorf("%q not found: %q", majorVersion, config.DistributionsRefs[d])
		}

	}

}
func TestValidatePackages(t *testing.T) {
	supportedDistributions := []string{"rhel-84", "rhel-85", "rhel-86", "rhel-87", "rhel-90"}

	for _, d := range supportedDistributions {

		if len(config.DistributionsPackages[d]) == 0 {
			t.Errorf("package not found: %q", config.DistributionsPackages[d])
		}
	}

}
func TestSupportedPackages(t *testing.T) {
	supportedDistributions := []string{"rhel-84", "rhel-85", "rhel-86", "rhel-87", "rhel-90"}

	if len(config.DistributionsPackages) != len(supportedDistributions) {
		t.Errorf("packages supported %d and found %d", len(config.DistributionsPackages), len(supportedDistributions))
	}

}

func TestInvaliddistribution(t *testing.T) {
	if config.DistributionsPackages["invalid"] != nil {
		t.Errorf("expected nil,  found %v", config.DistributionsPackages["invalid"])
	}
	if config.DistributionsRefs["invalid"] != "" {
		t.Errorf("expected nil,  found %v", config.DistributionsPackages["invalid"])
	}
}
