package execworkspace

import (
	"strings"
	"testing"
)

const validSHA = "abcdef0123456789abcdef0123456789abcdef01"

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
	if id1.WorkspaceID() != id2.WorkspaceID() {
		t.Fatalf("WorkspaceID not deterministic: %q vs %q", id1.WorkspaceID(), id2.WorkspaceID())
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
	got := id.WorkspaceID()
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
	got := id.WorkspaceID()
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
	got := id.WorkspaceID()
	want := "weird-run--name--" + validSHA[:12]
	if got != want {
		t.Fatalf("WorkspaceID() = %q, want %q", got, want)
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
		if next.WorkspaceID() != first.WorkspaceID() || next.PatchSHA256 != first.PatchSHA256 {
			t.Fatalf("identity not deterministic across repeated calls")
		}
	}
}
