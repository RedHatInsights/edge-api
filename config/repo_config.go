// FIXME: golangci-lint
// nolint:revive
package config

// DefaultDistribution set the default image distribution in case miss it
const DefaultDistribution = "rhel-90"

// RequiredPackages contains minimun list of packages to build an image
var RequiredPackages = []string{"rhc",
	"rhc-worker-playbook",
	"subscription-manager",
	"subscription-manager-plugin-ostree",
	"insights-client"}

// RHEL8 contains aditional list of packages to build an image to >= RHEL85
var RHEL8 = []string{"ansible"}

// RHEL86 contains aditional list of packages to build an image to = RHEL86
var RHEL86 = []string{"ansible-core"}

// RHEL90 contains aditional list of packages to build an image to = RHEL90
var RHEL90 = []string{"ansible-core"}

// DistributionsPackages add packages byi mage
var DistributionsPackages = map[string][]string{
	"rhel-84": RHEL8,
	"rhel-85": RHEL8,
	"rhel-86": RHEL86,
	"rhel-87": RHEL86,
	"rhel-90": RHEL90,
}

// DistributionsRefs set the ref to Images
var DistributionsRefs = map[string]string{
	"rhel-84": "rhel/8/x86_64/edge",
	"rhel-85": "rhel/8/x86_64/edge",
	"rhel-86": "rhel/8/x86_64/edge",
	"rhel-87": "rhel/8/x86_64/edge",
	"rhel-90": "rhel/9/x86_64/edge",
}
