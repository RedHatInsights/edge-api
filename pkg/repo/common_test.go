package repo

import (
	"testing"
)

func TestToGetRepoWhenNameIsContainedOnPrefix(t *testing.T) {
	expectedPrefix := "/api/edge/v1/repos/"

	prefix := getPathPrefix("/api/edge/v1/repos/1/repo", "1")
	if prefix != expectedPrefix {
		t.Errorf("Expected prefix to be %q but got %q", expectedPrefix, prefix)
	}
}

func TestToGetRepoPathPrefix(t *testing.T) {
	expectedPrefix := "/api/edge/v1/repos/"

	prefix := getPathPrefix("/api/edge/v1/repos/8/repo", "8")
	if prefix != expectedPrefix {
		t.Errorf("Expected prefix to be %q but got %q", expectedPrefix, prefix)
	}
}
