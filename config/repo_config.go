package config

var RequiredPackages = []string{"rhc",
	"rhc-worker-playbook",
	"subscription-manager",
	"subscription-manager-plugin-ostree",
	"insights-client"}
var RHEL8 = []string{"ansible"}
var RHEL86 = []string{"ansible-core"}
var RHEL90 = []string{"ansible-core"}

var DistributionsPackages = map[string][]string{
	"rhel-84": RHEL8,
	"rhel-85": RHEL8,
	"rhel-86": RHEL86,
	"rhel-90": RHEL90,
}

var DistributionsRefs = map[string]string{
	"rhel-84": "rhel/8/x86_64/edge",
	"rhel-85": "rhel/8/x86_64/edge",
	"rhel-90": "rhel/9/x86_64/edge",
}
