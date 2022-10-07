// FIXME: golangci-lint
// nolint:govet,revive,staticcheck
package models

import (
	"errors"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/magiconair/properties/assert"
	"github.com/redhatinsights/edge-api/pkg/db"
)

func TestGetPackageListWihoutDistribution(t *testing.T) {
	pkgs := []Package{
		{
			Name: "vim",
		},
		{
			Name: "wget",
		},
	}
	img := &Image{
		Distribution: "",
		Packages:     pkgs,
	}

	packageList := img.GetPackagesList()
	// We're returning nil in the case when Distribution is not provided
	// The assertion needs to compare the interface type and value
	assert.Equal(t, packageList, (*[]string)(nil))

}

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
		Distribution: "rhel-90",
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
			image:    &Image{Distribution: "rhel-84"},
			expected: errors.New(NameCantBeInvalidMessage),
		},
		{
			name:     "invalid characters in name",
			image:    &Image{Distribution: "rhel-85", Name: "image?"},
			expected: errors.New(NameCantBeInvalidMessage),
		},
		{
			name: "no commit in image",
			image: &Image{
				Distribution: "rhel-85",
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
				Distribution: "rhel-85",
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
				Distribution: "rhel-86",
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

func TestGetALLPackagesListWithCustomRepos(t *testing.T) {
	packages := []Package{{Name: "vim"}, {Name: "wget"}}
	customPackages := []Package{{Name: "custom-package"}, {Name: "third-party-package"}}
	image := &Image{
		Distribution:   "rhel-90",
		Packages:       packages,
		CustomPackages: customPackages,
		ThirdPartyRepositories: []ThirdPartyRepo{
			{Name: faker.UUIDHyphenated(), URL: faker.URL()},
		},
	}

	allPackagesList := image.GetALLPackagesList()

	if allPackagesList == nil {
		t.Errorf("error to load required expectedPackages")
	}

	expectedPackages := []string{
		"rhc",
		"rhc-worker-playbook",
		"subscription-manager",
		"subscription-manager-plugin-ostree",
		"insights-client",
		"ansible-core",
		"vim",
		"wget",
		customPackages[0].Name,
		customPackages[1].Name,
	}
	if len(expectedPackages) != len(*allPackagesList) {
		t.Errorf("Expected to have %d expectedPackages, but got %d", len(expectedPackages), len(*allPackagesList))
	}

	for i, packageName := range *allPackagesList {
		if packageName != expectedPackages[i] {
			t.Errorf("expected %s, got %s", expectedPackages[i], packageName)
		}
	}
}

func TestGetALLPackagesListWithoutCustomRepos(t *testing.T) {
	packages := []Package{{Name: "vim"}, {Name: "wget"}}
	customPackages := []Package{{Name: "custom-package"}, {Name: "third-party-package"}}
	image := &Image{
		Distribution:   "rhel-90",
		Packages:       packages,
		CustomPackages: customPackages,
	}

	allPackagesList := image.GetALLPackagesList()

	if allPackagesList == nil {
		t.Errorf("error to load required expectedPackages")
	}

	// we expect that custom packages are ignored
	expectedPackages := []string{
		"rhc",
		"rhc-worker-playbook",
		"subscription-manager",
		"subscription-manager-plugin-ostree",
		"insights-client",
		"ansible-core",
		"vim",
		"wget",
	}

	if len(expectedPackages) != len(*allPackagesList) {
		t.Errorf("Expected to have %d expectedPackages, but got %d", len(expectedPackages), len(*allPackagesList))
	}

	for i, item := range *allPackagesList {
		if item != expectedPackages[i] {
			t.Errorf("expected %s, got %s", expectedPackages[i], item)
		}
	}
}

func TestImageCreateWithOrgID(t *testing.T) {
	orgID := faker.UUIDHyphenated()
	image := &Image{

		Distribution: "rhel-85",
		Name:         "image_name",
		Commit: &Commit{
			Arch:  "x86_64",
			OrgID: orgID,
		},
		OutputTypes: []string{ImageTypeInstaller},
		Installer: &Installer{
			Username: "test",
			SSHKey:   "ssh-rsa dd:00:eeff:10",
			OrgID:    orgID,
		},
		OrgID: orgID,
	}

	// Make sure Image has orgID
	result := db.DB.Create(&image)
	assert.Equal(t, result.Error, nil)
}

func TestImageCreateWithoutOrgID(t *testing.T) {
	orgID := faker.UUIDHyphenated()
	image := &Image{

		Distribution: "rhel-85",
		Name:         "image_name",
		Commit: &Commit{
			Arch:  "x86_64",
			OrgID: orgID,
		},
		OutputTypes: []string{ImageTypeInstaller},
		Installer: &Installer{
			Username: "test",
			SSHKey:   "ssh-rsa dd:00:eeff:10",
			OrgID:    orgID,
		},
	}

	// Make sure Image is not created without an orgID
	result := db.DB.Create(&image)
	assert.Equal(t, result.Error, ErrOrgIDIsMandatory)
}

func TestImageSetCreateWithOrgID(t *testing.T) {
	orgID := faker.UUIDHyphenated()
	imageSet := ImageSet{
		Name:    "image-set-1",
		Version: 1,
		OrgID:   orgID,
	}

	// Make sure ImageSet is created with orgID
	result := db.DB.Create(&imageSet)
	assert.Equal(t, result.Error, nil)
}

func TestImageSetCreateWithoutOrgID(t *testing.T) {
	imageSet := ImageSet{
		Name:    "image-set-1",
		Version: 1,
	}

	// Make sure ImageSet cannot be created without an orgID
	result := db.DB.Create(&imageSet)
	assert.Equal(t, result.Error, ErrOrgIDIsMandatory)
}
