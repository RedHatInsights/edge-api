package commits

import "testing"

func TestPatch(t *testing.T) {
	commitOne := &Commit{
		OSTreeRef: "one",
	}
	commitTwo := &Commit{
		OSTreeRef: "two",
	}

	applyPatch(commitOne, commitTwo)

	if commitOne.OSTreeRef != "two" {
		t.Errorf("expected two got %s", commitOne.OSTreeRef)
	}
}
