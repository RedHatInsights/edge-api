package models

import (
	"testing"
)

func TestGetPackagesList(t *testing.T) {
	pkgs := []Package{
		Package{
			Name: "vim",
		},
		Package{
			Name: "ansible",
		},
	}
	c := &Commit{
		Packages: pkgs,
	}

	packageList := c.GetPackagesList()
	if len(*packageList) != 2 {
		t.Errorf("two packages expected")
	}
	packages := []string{"vim", "ansible"}
	for i, item := range *packageList {
		if item != packages[i] {
			t.Errorf("expected %s, got %s", packages[i], item)
		}
	}
}
