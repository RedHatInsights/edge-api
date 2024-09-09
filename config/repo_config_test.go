// FIXME: golangci-lint
// nolint:errcheck,gosec,govet,revive,typecheck
package config

import (
	"regexp"
	"strings"
	"testing"
)

// Validate distribution ref values should return refs by distribution"
func TestValidateRepo(t *testing.T) {
	// matches rhel-80 format and rhel-8.10 format
	dre := regexp.MustCompile(`(?P<dist>rhel)-((?P<maj>\d)(?P<min>\d)|(?P<maj>\d+)\.(?P<min>\d+))`)
	for key, d := range DistributionsRefs {
		// match the distro string
		match := dre.FindSubmatch([]byte(key))
		if match == nil {
			t.Errorf("%s does not match the requested pattern", key)
			return
		}

		// construct a map of groups: dist, maj, min
		grp := make(map[string]string)
		for i, name := range dre.SubexpNames() {
			if i != 0 && name != "" {
				grp[name] = string(match[i])
			}
		}

		// validate
		t.Run("validate distribution", func(t *testing.T) {
			if !strings.Contains(DistributionsRefs[key], grp["dist"]) {
				t.Errorf(" %q not found: %q", grp["dist"], DistributionsRefs[d])
			}
		})

		t.Run("validate major", func(t *testing.T) {
			if !strings.Contains(DistributionsRefs[key], grp["maj"]) {
				t.Errorf("%q not found: %q", grp["maj"], DistributionsRefs[d])
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
