package evidence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

func testSupersession() artifact.Supersession {
	return artifact.Supersession{
		Carried:         []string{"ac-1", "co-1"},
		Amended:         []artifact.SupersessionNote{{ID: "ac-2", Note: "tightened wording"}},
		AmendedAdvisory: []artifact.SupersessionNote{{ID: "dc-1", Note: "non-reaffirming clarification"}},
		Removed:         []artifact.SupersessionNote{{ID: "ac-3", Note: "dropped"}},
		Added:           []string{"ac-4"},
	}
}

// TestFoldCascade_PerVerdict is the exit criterion's "a cascade-fold case
// per verdict (unaffected on a carried/amended_advisory edge, stale on an
// amended edge, invalidated on a removed edge)".
func TestFoldCascade_PerVerdict(t *testing.T) {
	tests := []struct {
		name        string
		objectIDs   []string
		wantVerdict CascadeVerdict
	}{
		{name: "unaffected via carried", objectIDs: []string{"ac-1"}, wantVerdict: CascadeUnaffected},
		{name: "unaffected via amended_advisory", objectIDs: []string{"dc-1"}, wantVerdict: CascadeUnaffected},
		{name: "unaffected via both carried and amended_advisory", objectIDs: []string{"ac-1", "dc-1", "co-1"}, wantVerdict: CascadeUnaffected},
		{name: "stale via amended", objectIDs: []string{"ac-2"}, wantVerdict: CascadeStale},
		{name: "invalidated via removed", objectIDs: []string{"ac-3"}, wantVerdict: CascadeInvalidated},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FoldCascade(testSupersession(), []CascadeStory{{SpecRef: "spec/story-x", ObjectIDs: tc.objectIDs}})
			if err != nil {
				t.Fatalf("FoldCascade: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("results = %+v, want exactly 1", got)
			}
			if got[0].Verdict != tc.wantVerdict {
				t.Fatalf("verdict = %s, want %s", got[0].Verdict, tc.wantVerdict)
			}
		})
	}
}

// TestFoldCascade_Precedence proves invalidated beats stale beats
// unaffected when a single story's edges span more than one bucket.
func TestFoldCascade_Precedence(t *testing.T) {
	t.Run("invalidated beats stale and unaffected", func(t *testing.T) {
		got, err := FoldCascade(testSupersession(), []CascadeStory{
			{SpecRef: "spec/story-x", ObjectIDs: []string{"ac-1", "ac-2", "ac-3"}},
		})
		if err != nil {
			t.Fatalf("FoldCascade: %v", err)
		}
		if got[0].Verdict != CascadeInvalidated {
			t.Fatalf("verdict = %s, want invalidated", got[0].Verdict)
		}
	})

	t.Run("stale beats unaffected", func(t *testing.T) {
		got, err := FoldCascade(testSupersession(), []CascadeStory{
			{SpecRef: "spec/story-x", ObjectIDs: []string{"ac-1", "ac-2"}},
		})
		if err != nil {
			t.Fatalf("FoldCascade: %v", err)
		}
		if got[0].Verdict != CascadeStale {
			t.Fatalf("verdict = %s, want stale", got[0].Verdict)
		}
		if len(got[0].Amended) != 1 || got[0].Amended[0] != "ac-2" {
			t.Fatalf("Amended = %v, want [ac-2]", got[0].Amended)
		}
	})
}

// TestFoldCascade_MultipleStoriesIndependent proves each story's verdict is
// computed independently — sibling stories don't affect each other.
func TestFoldCascade_MultipleStoriesIndependent(t *testing.T) {
	got, err := FoldCascade(testSupersession(), []CascadeStory{
		{SpecRef: "spec/story-a", ObjectIDs: []string{"ac-1"}},
		{SpecRef: "spec/story-b", ObjectIDs: []string{"ac-3"}},
	})
	if err != nil {
		t.Fatalf("FoldCascade: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("results = %+v, want 2", got)
	}
	if got[0].Verdict != CascadeUnaffected {
		t.Fatalf("story-a verdict = %s, want unaffected", got[0].Verdict)
	}
	if got[1].Verdict != CascadeInvalidated {
		t.Fatalf("story-b verdict = %s, want invalidated", got[1].Verdict)
	}
}

// TestFoldCascade_Negative_UnclassifiedObject proves a story edge naming an
// object the supersession manifest never classifies fails loudly rather
// than silently defaulting to unaffected.
func TestFoldCascade_Negative_UnclassifiedObject(t *testing.T) {
	_, err := FoldCascade(testSupersession(), []CascadeStory{
		{SpecRef: "spec/story-x", ObjectIDs: []string{"ac-999"}},
	})
	if err == nil {
		t.Fatal("FoldCascade(unclassified object): want error, got nil")
	}
}

// TestFoldCascade_RealFixture runs the cascade fold against the real v2
// fixture's rung-4 supersession pair (loan-workflow -> loan-workflow-v2,
// testdata/corpus) rather than synthetic data — the brief's "the verb runs
// on fixturegit + the v2 fixture's supersession pair" exercised at the
// fold level: spec/borrower-update-mobile's real implements edge into
// spec/loan-workflow#ac-1 (an object loan-workflow-v2's real supersession
// block marks amended) must fold to stale.
func TestFoldCascade_RealFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "corpus", ".verdi", "specs", "active", "loan-workflow-v2", "spec.md"))
	if err != nil {
		t.Fatalf("reading loan-workflow-v2 fixture: %v", err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	v2, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if v2.Supersession == nil {
		t.Fatal("test setup: loan-workflow-v2 fixture carries no supersession: block")
	}

	// spec/borrower-update-mobile's real links block: implements
	// spec/loan-workflow#ac-1 (see testdata/corpus/.verdi/specs/active/
	// borrower-update-mobile/spec.md).
	got, err := FoldCascade(*v2.Supersession, []CascadeStory{
		{SpecRef: "spec/borrower-update-mobile", ObjectIDs: []string{"ac-1"}},
	})
	if err != nil {
		t.Fatalf("FoldCascade: %v", err)
	}
	if got[0].Verdict != CascadeStale {
		t.Fatalf("verdict = %s, want stale (loan-workflow-v2 marks ac-1 amended)", got[0].Verdict)
	}
}
