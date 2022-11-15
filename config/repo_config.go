// FIXME: golangci-lint
// nolint:revive
package config

// DefaultDistribution set the default image distribution in case miss it
const DefaultDistribution = "rhel-90"

// ostree ref for supported distributions
const OstreeRefRHEL8 = "rhel/8/x86_64/edge"
const OstreeRefRHEL9 = "rhel/9/x86_64/edge"

// RequiredPackages contains minimun list of packages to build an image
var RequiredPackages = []string{"rhc",
	"rhc-worker-playbook",
	"subscription-manager",
	"subscription-manager-plugin-ostree",
	"insights-client"}

// RHEL8 contains additional list of packages to build an image to >= RHEL85
var RHEL8 = []string{"ansible"}

// RHEL8X contains additional list of packages to build an image to = RHEL8X
var RHEL8X = []string{"ansible-core"}

// RHEL90 contains additional list of packages to build an image to = RHEL90
var RHEL90 = []string{"ansible-core"}

// DistributionsPackages add packages by image
var DistributionsPackages = map[string][]string{
	"rhel-84":           RHEL8,
	"rhel-85":           RHEL8,
	"rhel-86":           RHEL8X,
	"rhel-87":           RHEL8X,
	DefaultDistribution: RHEL90,
}

// DistributionsRefs set the ref to Images
var DistributionsRefs = map[string]string{
	"rhel-84":           OstreeRefRHEL8,
	"rhel-85":           OstreeRefRHEL8,
	"rhel-86":           OstreeRefRHEL8,
	"rhel-87":           OstreeRefRHEL8,
	DefaultDistribution: OstreeRefRHEL9,
}
