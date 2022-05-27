package models

import (
	"errors"
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
	img := &Image{
		Distribution: "rhel-9",
		Packages:     pkgs,
	}

	packageList := img.GetPackagesList()

	if len(*packageList) == 0 {
		t.Errorf("error to load required packages")
	}
	packages := []string{
		"rhc",
		"rhc-worker-playbook",
		"subscription-manager",
		"subscription-manager-plugin-ostree",
		"insights-client",
		"ansible-core",
		"vim",
		"wget",
	}
	for i, item := range *packageList {
		if item != packages[i] {
			t.Errorf("expected %s, got %s", packages[i], item)
		}
	}
}

func TestValidateRequest(t *testing.T) {
	tt := []struct {
		name     string
		image    *Image
		expected error
	}{
		{
			name:     "empty distribution",
			image:    &Image{},
			expected: errors.New(DistributionCantBeNilMessage),
		},
		{
			name:     "empty name",
			image:    &Image{Distribution: "rhel-8"},
			expected: errors.New(NameCantBeInvalidMessage),
		},
		{
			name:     "invalid characters in name",
			image:    &Image{Distribution: "rhel-8", Name: "image?"},
			expected: errors.New(NameCantBeInvalidMessage),
		},
		{
			name: "no commit in image",
			image: &Image{
				Distribution: "rhel-8",
				Name:         "image_name",
			},
			expected: errors.New(ArchitectureCantBeEmptyMessage),
		},
		{
			name: "empty architecture",
			image: &Image{
				Distribution: "rhel-8",
				Name:         "image_name",
				Commit:       &Commit{Arch: ""},
			},
			expected: errors.New(ArchitectureCantBeEmptyMessage),
		},
		{
			name: "empty architecture",
			image: &Image{
				Distribution: "rhel-8",
				Name:         "image_name",
				Commit:       &Commit{Arch: ""},
			},
			expected: errors.New(ArchitectureCantBeEmptyMessage),
		},
		{
			name: "no output type",
			image: &Image{
				Distribution: "rhel-8",
				Name:         "image_name",
				Commit:       &Commit{Arch: "x86_64"},
			},
			expected: errors.New(NoOutputTypes),
		},
		{
			name: "invalid output type",
			image: &Image{
				Distribution: "rhel-8",
				Name:         "image_name",
				Commit:       &Commit{Arch: "x86_64"},
				OutputTypes:  []string{"zip-image-type"},
			},
			expected: errors.New(ImageTypeNotAccepted),
		},
		{
			name: "no installer when image type is installer",
			image: &Image{
				Distribution: "rhel-8",
				Name:         "image_name",
				Commit:       &Commit{Arch: "x86_64"},
				OutputTypes:  []string{ImageTypeInstaller},
			},
			expected: errors.New(MissingInstaller),
		},
		{
			name: "empty username when image type is installer",
			image: &Image{
				Distribution: "rhel-8",
				Name:         "image_name",
				Commit:       &Commit{Arch: "x86_64"},
				OutputTypes:  []string{ImageTypeInstaller},
				Installer: &Installer{
					Username: "",
				},
			},
			expected: errors.New(MissingUsernameError),
		},
		{
			name: "reserved username when image type is installer",
			image: &Image{
				Distribution: "rhel-8",
				Name:         "image_name",
				Commit:       &Commit{Arch: "x86_64"},
				OutputTypes:  []string{ImageTypeInstaller},
				Installer: &Installer{
					Username: "rpcuser",
				},
			},
			expected: errors.New(ReservedUsernameError),
		},
		{
			name: "empty ssh key when image type is installer",
			image: &Image{
				Distribution: "rhel-8",
				Name:         "image_name",
				Commit:       &Commit{Arch: "x86_64"},
				OutputTypes:  []string{ImageTypeInstaller},
				Installer: &Installer{
					Username: "test",
				},
			},
			expected: errors.New(MissingSSHKeyError),
		},
		{
			name: "invalid ssh key",
			image: &Image{
				Distribution: "rhel-8",
				Name:         "image_name",
				Commit:       &Commit{Arch: "x86_64"},
				OutputTypes:  []string{ImageTypeInstaller},
				Installer: &Installer{
					Username: "test",
					SSHKey:   "dd:00:eeff:10",
				},
			},
			expected: errors.New(InvalidSSHKeyError),
		},
		{
			name: "valid image request",
			image: &Image{
				Distribution: "rhel-8",
				Name:         "image_name",
				Commit:       &Commit{Arch: "x86_64"},
				OutputTypes:  []string{ImageTypeInstaller},
				Installer: &Installer{
					Username: "test",
					SSHKey:   "ssh-rsa dd:00:eeff:10",
				},
			},
			expected: nil,
		},
		{
			name: "valid image request for commit",
			image: &Image{
				Distribution: "rhel-8",
				Name:         "image_name",
				Commit:       &Commit{Arch: "x86_64"},
				OutputTypes:  []string{ImageTypeCommit},
			},
			expected: nil,
		},
	}

	for _, te := range tt {
		err := te.image.ValidateRequest()
		if err == nil && te.expected != nil {
			t.Errorf("Test %q was supposed to fail but passed successfully", te.name)
		}
		if err != nil && te.expected == nil {
			t.Errorf("Test %q was supposed to pass but failed: %s", te.name, err)
		}
		if err != nil && te.expected != nil && err.Error() != te.expected.Error() {
			t.Errorf("Test %q: expected to fail on %q but got %q", te.name, te.expected, err)
		}
	}
}

func TestGetALLPackagesList(t *testing.T) {
	pkgs := []Package{
		{
			Name: "vim",
		},
		{
			Name: "wget",
		},
	}
	customPackages := []Package{
		{
			Name: "custompackage",
		},
		{
			Name: "thirdpartypackage",
		},
	}
	img := &Image{
		Distribution:   "rhel-9",
		Packages:       pkgs,
		CustomPackages: customPackages,
	}

	allPackagesList := img.GetALLPackagesList()
	if len(*allPackagesList) == 0 {
		t.Errorf("error to load required packages")
	}

	packages := []string{
		"rhc",
		"rhc-worker-playbook",
		"subscription-manager",
		"subscription-manager-plugin-ostree",
		"insights-client",
		"ansible-core",
		"vim",
		"wget",
		"custompackage",
		"thirdpartypackage",
	}

	for i, item := range *allPackagesList {
		if item != packages[i] {
			t.Errorf("expected %s, got %s", packages[i], item)
		}
	}
}
