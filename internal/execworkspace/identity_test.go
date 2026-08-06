package execworkspace

import (
	"strings"
	"testing"
)

const validSHA = "abcdef0123456789abcdef0123456789abcdef01"

// validPatchSHA is a canonical lowercase 64-hex sha256 digest, the
// patch-shape counterpart to validSHA.
const validPatchSHA = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

func TestNewExactIdentity_HappyPath(t *testing.T) {
	id, err := NewExactIdentity("feature/my-run", validSHA)
	if err != nil {
		t.Fatalf("NewExactIdentity: unexpected error: %v", err)
	}
	if id.Shape != ExactSHA {
		t.Fatalf("Shape = %v, want ExactSHA", id.Shape)
	}
	if id.RunID != "feature/my-run" {
		t.Fatalf("RunID = %q, want %q", id.RunID, "feature/my-run")
	}
	if id.CommitSHA != validSHA {
		t.Fatalf("CommitSHA = %q, want %q", id.CommitSHA, validSHA)
	}
	if id.PatchSHA256 != "" {
		t.Fatalf("PatchSHA256 = %q, want empty for exact-SHA shape", id.PatchSHA256)
	}
}

func TestNewExactIdentity_RejectsNonCanonicalSHA(t *testing.T) {
	cases := map[string]string{
		"uppercase":  "ABCDEF0123456789ABCDEF0123456789ABCDEF01",
		"short":      "abcdef0123456789",
		"long":       validSHA + "ab",
		"non-hex":    "zzcdef0123456789abcdef0123456789abcdef01",
		"empty":      "",
		"mixed-case": "Abcdef0123456789abcdef0123456789abcdef01",
	}
	for name, sha := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewExactIdentity("run", sha); err == nil {
				t.Fatalf("NewExactIdentity(%q): want error, got nil", sha)
			}
		})
	}
}

func TestNewPatchIdentity_HappyPath(t *testing.T) {
	id, err := NewPatchIdentity("feature/my-run", validSHA, []byte("--- a\n+++ b\n"))
	if err != nil {
		t.Fatalf("NewPatchIdentity: unexpected error: %v", err)
	}
	if id.Shape != BasePlusPatch {
		t.Fatalf("Shape = %v, want BasePlusPatch", id.Shape)
	}
	if len(id.PatchSHA256) != 64 {
		t.Fatalf("PatchSHA256 = %q, want 64 hex chars", id.PatchSHA256)
	}
	if strings.ToLower(id.PatchSHA256) != id.PatchSHA256 {
		t.Fatalf("PatchSHA256 = %q, want lowercase", id.PatchSHA256)
	}
}

func TestNewPatchIdentity_Deterministic(t *testing.T) {
	id1, err := NewPatchIdentity("run", validSHA, []byte("patch-bytes"))
	if err != nil {
		t.Fatalf("NewPatchIdentity: %v", err)
	}
	id2, err := NewPatchIdentity("run", validSHA, []byte("patch-bytes"))
	if err != nil {
		t.Fatalf("NewPatchIdentity: %v", err)
	}
	if id1.PatchSHA256 != id2.PatchSHA256 {
		t.Fatalf("PatchSHA256 not deterministic: %q vs %q", id1.PatchSHA256, id2.PatchSHA256)
	}
	wid1, err := id1.WorkspaceID()
	if err != nil {
		t.Fatalf("WorkspaceID: %v", err)
	}
	wid2, err := id2.WorkspaceID()
	if err != nil {
		t.Fatalf("WorkspaceID: %v", err)
	}
	if wid1 != wid2 {
		t.Fatalf("WorkspaceID not deterministic: %q vs %q", wid1, wid2)
	}
}

func TestNewPatchIdentity_RejectsEmptyPatch(t *testing.T) {
	if _, err := NewPatchIdentity("run", validSHA, nil); err == nil {
		t.Fatalf("NewPatchIdentity(nil patch): want error, got nil")
	}
	if _, err := NewPatchIdentity("run", validSHA, []byte{}); err == nil {
		t.Fatalf("NewPatchIdentity(empty patch): want error, got nil")
	}
}

func TestNewPatchIdentity_RejectsNonCanonicalSHA(t *testing.T) {
	if _, err := NewPatchIdentity("run", "not-a-sha", []byte("x")); err == nil {
		t.Fatalf("NewPatchIdentity: want error for bad sha, got nil")
	}
}

func TestWorkspaceID_ExactShape(t *testing.T) {
	id, err := NewExactIdentity("feature/My-Run", validSHA)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	got, err := id.WorkspaceID()
	if err != nil {
		t.Fatalf("WorkspaceID: %v", err)
	}
	want := "feature--my-run--" + validSHA[:12]
	if got != want {
		t.Fatalf("WorkspaceID() = %q, want %q", got, want)
	}
}

func TestWorkspaceID_PatchShape(t *testing.T) {
	id, err := NewPatchIdentity("feature/My-Run", validSHA, []byte("x"))
	if err != nil {
		t.Fatalf("NewPatchIdentity: %v", err)
	}
	got, err := id.WorkspaceID()
	if err != nil {
		t.Fatalf("WorkspaceID: %v", err)
	}
	want := "feature--my-run--" + validSHA[:12] + "-p" + id.PatchSHA256[:12]
	if got != want {
		t.Fatalf("WorkspaceID() = %q, want %q", got, want)
	}
}

func TestWorkspaceID_UsesStoreRefSlug(t *testing.T) {
	// RefSlug maps '/' to '--' and lowercases; verify execworkspace reuses
	// store.RefSlug rather than a second scheme, by checking an underscore
	// (excluded from the slug alphabet) maps to '-'.
	id, err := NewExactIdentity("weird_run/name", validSHA)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	got, err := id.WorkspaceID()
	if err != nil {
		t.Fatalf("WorkspaceID: %v", err)
	}
	want := "weird-run--name--" + validSHA[:12]
	if got != want {
		t.Fatalf("WorkspaceID() = %q, want %q", got, want)
	}
}

// TestWorkspaceID_ShortDigestIsAnErrorNotAPanic proves a hand-built Identity
// whose digests are too short to slice yields a fail-closed error, never a
// panic and never a truncated path naming the wrong directory.
func TestWorkspaceID_ShortDigestIsAnErrorNotAPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("WorkspaceID() panicked on a short commit sha: %v", r)
		}
	}()
	cases := map[string]Identity{
		"short commit sha":  {Shape: ExactSHA, RunID: "run", CommitSHA: "abc"},
		"short patch sha":   {Shape: BasePlusPatch, RunID: "run", CommitSHA: validSHA, PatchSHA256: "abc"},
		"empty commit sha":  {Shape: ExactSHA, RunID: "run"},
		"unknown shape":     {Shape: Shape(42), RunID: "run", CommitSHA: validSHA},
		"empty run id":      {Shape: ExactSHA, CommitSHA: validSHA},
		"exact with patch":  {Shape: ExactSHA, RunID: "run", CommitSHA: validSHA, PatchSHA256: validPatchSHA},
		"patch without sum": {Shape: BasePlusPatch, RunID: "run", CommitSHA: validSHA},
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := id.WorkspaceID()
			if err == nil {
				t.Fatalf("WorkspaceID() = %q, want error for %+v", got, id)
			}
			if got != "" {
				t.Fatalf("WorkspaceID() = %q alongside an error, want empty string", got)
			}
			if verr := id.Validate(); verr == nil {
				t.Fatalf("Validate() = nil for %+v, want error", id)
			}
		})
	}
}

func TestIdentity_Validate_AcceptsConstructedIdentities(t *testing.T) {
	exact, err := NewExactIdentity("feature/my-run", validSHA)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	if err := exact.Validate(); err != nil {
		t.Fatalf("Validate(exact): unexpected error: %v", err)
	}
	patch, err := NewPatchIdentity("feature/my-run", validSHA, []byte("diff"))
	if err != nil {
		t.Fatalf("NewPatchIdentity: %v", err)
	}
	if err := patch.Validate(); err != nil {
		t.Fatalf("Validate(patch): unexpected error: %v", err)
	}
}

func TestNewIdentity_RejectsEmptyRunID(t *testing.T) {
	if _, err := NewExactIdentity("", validSHA); err == nil {
		t.Fatalf("NewExactIdentity(empty run id): want error, got nil")
	}
	if _, err := NewPatchIdentity("", validSHA, []byte("x")); err == nil {
		t.Fatalf("NewPatchIdentity(empty run id): want error, got nil")
	}
}

// TestNewIdentity_RejectsInvalidUTF8RunID proves a RunID that is not valid
// UTF-8 fails closed at construction and at Validate. Canonical JSON is
// LOSSY for invalid UTF-8 — encoding substitutes U+FFFD — so an identity
// carrying such a RunID could be written to a sidecar and read back as a
// DIFFERENT identity; the guard belongs upstream of the encoder.
func TestNewIdentity_RejectsInvalidUTF8RunID(t *testing.T) {
	cases := map[string]string{
		"lone continuation bytes": "run-\xff\xfe-x",
		"truncated 2-byte start":  "run-\xc3",
		"surrogate half":          "run-\xed\xa0\x80",
		"overlong encoding":       "run-\xc0\xaf",
	}
	for name, runID := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewExactIdentity(runID, validSHA); err == nil {
				t.Fatalf("NewExactIdentity(%q): want error, got nil", runID)
			}
			if _, err := NewPatchIdentity(runID, validSHA, []byte("x")); err == nil {
				t.Fatalf("NewPatchIdentity(%q): want error, got nil", runID)
			}
			id := Identity{Shape: ExactSHA, RunID: runID, CommitSHA: validSHA}
			if err := id.Validate(); err == nil {
				t.Fatalf("Validate(): want error for run id %q, got nil", runID)
			}
			if got, err := id.WorkspaceID(); err == nil {
				t.Fatalf("WorkspaceID() = %q, want error for run id %q", got, runID)
			}
		})
	}
}

// TestEncodeDecodeSidecar_RoundTripsToAnEqualIdentity is the completion
// witness invariant: EVERY identity the constructors emit must survive
// EncodeSidecar/DecodeSidecar byte-for-byte and compare Equal to itself.
// Without it, a legitimate repeat of an identical request compares unequal
// to its own recorded sidecar and takes the ErrIdentityMismatch branch — a
// hard error naming both requests — forever, permanently wedging that
// workspace id.
func TestEncodeDecodeSidecar_RoundTripsToAnEqualIdentity(t *testing.T) {
	cases := map[string]string{
		"plain":         "run",
		"slashed":       "feature/My-Run",
		"underscored":   "weird_run/name",
		"invalid utf-8": "run-\xff\xfe-x",
	}
	for name, runID := range cases {
		t.Run(name, func(t *testing.T) {
			id, err := NewExactIdentity(runID, validSHA)
			if err != nil {
				// Rejected at construction: it can never reach a sidecar,
				// so the round-trip invariant is vacuously preserved.
				t.Skipf("NewExactIdentity(%q) rejected the run id: %v", runID, err)
			}
			data, err := EncodeSidecar(id)
			if err != nil {
				t.Fatalf("EncodeSidecar: %v", err)
			}
			back, err := DecodeSidecar(data)
			if err != nil {
				t.Fatalf("DecodeSidecar: %v", err)
			}
			if !back.Equal(id) {
				t.Fatalf("sidecar round trip is not identity-preserving: encoded %s, decoded %s (bytes %q)", id, back, data)
			}
		})
	}
}

// TestWorkspaceID_SatisfiesValidWorkspaceID closes the loop between the two
// halves of the naming scheme: every id the producer emits is one the
// grammar classifier accepts.
func TestWorkspaceID_SatisfiesValidWorkspaceID(t *testing.T) {
	runIDs := []string{"run", "feature/My-Run", "weird_run/name", "a/b/c", "release/v1.2.3", "..", "--"}
	for _, runID := range runIDs {
		t.Run(runID, func(t *testing.T) {
			exact, err := NewExactIdentity(runID, validSHA)
			if err != nil {
				t.Fatalf("NewExactIdentity(%q): %v", runID, err)
			}
			wid, err := exact.WorkspaceID()
			if err != nil {
				t.Fatalf("WorkspaceID: %v", err)
			}
			if !ValidWorkspaceID(wid) {
				t.Fatalf("ValidWorkspaceID(%q) = false for an exact-shape id the package itself produced", wid)
			}
			patch, err := NewPatchIdentity(runID, validSHA, []byte("diff"))
			if err != nil {
				t.Fatalf("NewPatchIdentity(%q): %v", runID, err)
			}
			pwid, err := patch.WorkspaceID()
			if err != nil {
				t.Fatalf("WorkspaceID: %v", err)
			}
			if !ValidWorkspaceID(pwid) {
				t.Fatalf("ValidWorkspaceID(%q) = false for a patch-shape id the package itself produced", pwid)
			}
		})
	}
}

func TestIdentity_Equal(t *testing.T) {
	a, _ := NewExactIdentity("run", validSHA)
	b, _ := NewExactIdentity("run", validSHA)
	if !a.Equal(b) {
		t.Fatalf("Equal: identical exact identities should be equal")
	}

	c, _ := NewExactIdentity("run", "0000000000000000000000000000000000000000")
	if a.Equal(c) {
		t.Fatalf("Equal: different commit sha should not be equal")
	}

	p1, _ := NewPatchIdentity("run", validSHA, []byte("one"))
	p2, _ := NewPatchIdentity("run", validSHA, []byte("two"))
	if p1.Equal(p2) {
		t.Fatalf("Equal: different patch bytes should not be equal")
	}

	// Same run/sha, but one exact and one patch shape must never compare equal.
	if a.Equal(p1) {
		t.Fatalf("Equal: exact shape must never equal patch shape")
	}
}

func TestIdentity_NoWallClockNoRandomness(t *testing.T) {
	// Deterministic across many calls, proving no hidden time/random input.
	first, _ := NewPatchIdentity("run", validSHA, []byte("stable"))
	for i := 0; i < 25; i++ {
		next, err := NewPatchIdentity("run", validSHA, []byte("stable"))
		if err != nil {
			t.Fatalf("NewPatchIdentity: %v", err)
		}
		nextID, err := next.WorkspaceID()
		if err != nil {
			t.Fatalf("WorkspaceID: %v", err)
		}
		firstID, err := first.WorkspaceID()
		if err != nil {
			t.Fatalf("WorkspaceID: %v", err)
		}
		if nextID != firstID || next.PatchSHA256 != first.PatchSHA256 {
			t.Fatalf("identity not deterministic across repeated calls")
		}
	}
}
