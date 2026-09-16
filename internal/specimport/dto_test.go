package specimport

import "testing"

// TestPreviewResultDigest_DeterministicAndExcludesSelf proves
// computePreviewDigest is a pure function of PreviewResult's own content
// EXCLUDING the Digest field itself (spec-import-contract.md: "digests are
// SHA-256 lowercase hex over canonical JSON excluding the digest itself"):
// two structurally equal values (whatever their own stale Digest field
// carries) digest identically, and changing any other field changes the
// digest.
func TestPreviewResultDigest_DeterministicAndExcludesSelf(t *testing.T) {
	base := PreviewResult{
		Schema: PreviewResultSchema, BaseCommit: "a", ModelDigest: "b", ConfigDigest: "c",
		EngineDigest: "d", RequestDigest: "e", SpecRef: "spec/sample",
		Fields: []Field{{Target: "problem", Text: "x", Origin: OriginCopiedSource}},
		Ready:  true,
	}
	first, err := computePreviewDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 64 {
		t.Fatalf("digest %q is not 64 lowercase hex characters", first)
	}

	stale := base
	stale.Digest = "stale-value-must-be-ignored"
	second, err := computePreviewDigest(stale)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("digest changed when only the stale self-field changed: %q != %q", first, second)
	}

	changed := base
	changed.Ready = false
	third, err := computePreviewDigest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatalf("digest did not change when Ready changed")
	}
}

// TestBoardPath pins the existing branch-board route convention
// (spec-import-contract.md: "board_path using the existing
// /b/design%2F<slug>/board/spec/<slug> branch-board route convention").
func TestBoardPath(t *testing.T) {
	if got, want := boardPath("sample-feature"), "/b/design%2Fsample-feature/board/spec/sample-feature"; got != want {
		t.Fatalf("boardPath = %q, want %q", got, want)
	}
}
