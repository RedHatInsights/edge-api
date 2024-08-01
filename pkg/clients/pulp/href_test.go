package pulp

import (
	"testing"

	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/pkg/ptr"
)

func TestScanUUID(t *testing.T) {

	t.Run("scanUUID", func(t *testing.T) {
		var tests = []struct {
			name     string
			href     string
			expected uuid.UUID
		}{
			{
				name:     "valid uuid",
				href:     "/pulp/api/v3/repositories/rpm/rpm/9c8a1c9e-9d0b-4e0d-8f5c-7f4f2f3e8b8a/",
				expected: uuid.MustParse("9c8a1c9e-9d0b-4e0d-8f5c-7f4f2f3e8b8a"),
			},
			{
				name:     "valid uuid",
				href:     "/api/pulp/edge-integration-test-1/api/v3/contentguards/core/header/01902b07-242d-7ee0-9964-6191c8f8d622/",
				expected: uuid.MustParse("01902b07-242d-7ee0-9964-6191c8f8d622"),
			},
			{
				name:     "empty string",
				href:     "",
				expected: uuid.UUID{},
			},
			{
				name:     "invalid uuid",
				href:     "this is not a uuid",
				expected: uuid.UUID{},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// linter G601 workaround
				actual := ScanUUID(ptr.To(tt.href))
				if actual != tt.expected {
					t.Errorf("scanUUID(%s): expected %v, actual %v", tt.href, tt.expected, actual)
				}
			})
		}
	})
}

func TestScanRepoFileVersion(t *testing.T) {

	t.Run("scanRepoFileVersion", func(t *testing.T) {
		var tests = []struct {
			name     string
			href     string
			expected int64
		}{
			{
				name:     "valid version",
				href:     "/api/pulp/edge-integration-test-2/api/v3/repositories/file/file/01910e45-ceb3-7213-bed8-0727e76d0d12/versions/1/",
				expected: 1,
			},
			{
				name:     "valid version",
				href:     "/api/pulp/edge-integration-test-2/api/v3/repositories/file/file/01910e45-ceb3-7213-bed8-0727e76d0d12/versions/2/",
				expected: 2,
			},
			{
				name:     "empty string",
				href:     "",
				expected: 0,
			},
			{
				name:     "invalid version",
				href:     "this is not a version",
				expected: 0,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// linter G601 workaround
				actual := ScanRepoFileVersion(ptr.To(tt.href))
				if actual != tt.expected {
					t.Errorf("scanRepoFileVersion(%s): expected %v, actual %v", tt.href, tt.expected, actual)
				}
			})
		}
	})
}
