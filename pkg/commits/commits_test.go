package commits

import (
	"testing"

	"github.com/redhatinsights/edge-api/pkg/models"
)

func TestPatch(t *testing.T) {
	commitOne := &models.Commit{
		OSTreeRef: "one",
	}
	commitTwo := &models.Commit{
		OSTreeRef: "two",
	}

	applyPatch(commitOne, commitTwo)

	if commitOne.OSTreeRef != "two" {
		t.Errorf("expected two got %s", commitOne.OSTreeRef)
	}
}
