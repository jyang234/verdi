package evidence

import (
	"path/filepath"
	"testing"
)

// corpusStoreRoot is testdata/corpus relative to this package — the same
// committed fixture internal/corpus, internal/index, and internal/dex build
// on. Its committed zone carries the story's real attestation and waivers
// under I-31's canonical <story> path segment,
// RefSlug("jira:LOAN-1482") = "jira-loan-1482".
const corpusStoreRoot = "../../testdata/corpus"

// canonicalStorySlug is what cmd/verdi/matrix.go passes as StorySlug for the
// corpus's feature story: store.RefSlug("jira:LOAN-1482"). Spelled literally
// here (this package stays free of store's ref-resolution policy) so the test
// pins the exact segment the fold consults.
const canonicalStorySlug = "jira-loan-1482"

// TestAttestationExists_CorpusSegment proves the corpus attestation file
// resolves under I-31's canonical <story> segment — the fold's
// attestation-existence path is live end-to-end against the real fixture,
// and is NOT reachable under the old bare-tracker-key segment (the orphan
// the coherence fix removed).
func TestAttestationExists_CorpusSegment(t *testing.T) {
	root, err := filepath.Abs(corpusStoreRoot)
	if err != nil {
		t.Fatalf("resolving corpus root: %v", err)
	}

	exists, err := AttestationExists(root, canonicalStorySlug, "ac-2")
	if err != nil {
		t.Fatalf("AttestationExists(%q, ac-2): %v", canonicalStorySlug, err)
	}
	if !exists {
		t.Fatalf("AttestationExists(%q, ac-2) = false, want true: the corpus attestation must resolve under the canonical segment", canonicalStorySlug)
	}

	// The old bare-tracker-key segment must no longer resolve — proving the
	// orphaned path the incoherence created is gone.
	orphan, err := AttestationExists(root, "story-1482", "ac-2")
	if err != nil {
		t.Fatalf("AttestationExists(story-1482, ac-2): %v", err)
	}
	if orphan {
		t.Fatal("AttestationExists(story-1482, ac-2) = true, want false: the old bare-tracker-key segment must be gone after the I-31 rename")
	}
}

// TestWaiverActive_CorpusSegment proves the corpus waivers resolve under the
// canonical segment: ac-4's active waiver waives (the matrix golden's
// now-live waived path), ac-3's expired waiver does not, and the old bare
// segment resolves neither.
func TestWaiverActive_CorpusSegment(t *testing.T) {
	root, err := filepath.Abs(corpusStoreRoot)
	if err != nil {
		t.Fatalf("resolving corpus root: %v", err)
	}

	active, err := WaiverActive(root, canonicalStorySlug, "ac-4")
	if err != nil {
		t.Fatalf("WaiverActive(%q, ac-4): %v", canonicalStorySlug, err)
	}
	if !active {
		t.Fatalf("WaiverActive(%q, ac-4) = false, want true: the corpus active waiver must waive under the canonical segment", canonicalStorySlug)
	}

	expired, err := WaiverActive(root, canonicalStorySlug, "ac-3")
	if err != nil {
		t.Fatalf("WaiverActive(%q, ac-3): %v", canonicalStorySlug, err)
	}
	if expired {
		t.Fatalf("WaiverActive(%q, ac-3) = true, want false: the corpus ac-3 waiver is expired and must not waive", canonicalStorySlug)
	}

	orphan, err := WaiverActive(root, "story-1482", "ac-4")
	if err != nil {
		t.Fatalf("WaiverActive(story-1482, ac-4): %v", err)
	}
	if orphan {
		t.Fatal("WaiverActive(story-1482, ac-4) = true, want false: the old bare-tracker-key segment must be gone after the I-31 rename")
	}
}
