package models

import (
	"errors"
	"testing"
)

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
			name: "empty ssh key when image type is installer",
			image: &Image{
				Distribution: "rhel-8",
				Name:         "image_name",
				Commit:       &Commit{Arch: "x86_64"},
				OutputTypes:  []string{ImageTypeInstaller},
				Installer: &Installer{
					Username: "root",
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
					Username: "root",
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
					Username: "root",
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
