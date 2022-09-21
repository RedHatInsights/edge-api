// FIXME: golangci-lint
// nolint:revive
package files

import "testing"

func TestSanitizeWithFilePathSameAsDest(t *testing.T) {
	path, err := sanitizePath("/tmp/repos/5535", "./")
	if path != "/tmp/repos/5535" {
		t.Error(path)
	}
	if err != nil {
		t.Error(err)
	}
}
func TestSanitizeWithFilePathWithFolder(t *testing.T) {
	path, err := sanitizePath("/tmp/repos/5535", "abc")
	if path != "/tmp/repos/5535/abc" {
		t.Error(path)
	}
	if err != nil {
		t.Error(err)
	}
}
