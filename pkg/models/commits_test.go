package models

import (
	"testing"
)

func TestGetPackagesList(t *testing.T) {
	pkgs := []Package{
		{
			Name: "vim",
		},
		{
			Name: "wget",
		},
	}
	c := &Commit{
		Packages: pkgs,
	}

	packageList := c.GetPackagesList()
	if len(*packageList) != len(pkgs)+len(requiredPackages) {
		t.Errorf("two packages + required packages expected")
	}
	packages := []string{
		"ansible",
		"rhc",
		"rhc-playbook-worker",
		"subscription-manager",
		"subscription-manager-plugin-ostree",
		"insights-client",
		"vim",
		"wget",
	}
	for i, item := range *packageList {
		if item != packages[i] {
			t.Errorf("expected %s, got %s", packages[i], item)
		}
	}
}
