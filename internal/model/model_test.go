package model

import (
	"strings"
	"testing"
)

func TestModel_Digest_Deterministic(t *testing.T) {
	d1, err := canonicalModel.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	d2, err := canonicalModel.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if d1 != d2 {
		t.Fatalf("Digest not deterministic: %q vs %q", d1, d2)
	}
	if !strings.HasPrefix(d1, "sha256:") {
		t.Fatalf("Digest = %q, want sha256:<hex> form", d1)
	}
}

// TestModel_Digest_DiffersOnChange proves Digest is a real function of
// content, not a constant — a changed model produces a changed digest.
func TestModel_Digest_DiffersOnChange(t *testing.T) {
	a := canonicalModel
	b := canonicalModel
	b.Schema = "verdi.model/v2"

	da, err := a.Digest()
	if err != nil {
		t.Fatalf("Digest(a): %v", err)
	}
	db, err := b.Digest()
	if err != nil {
		t.Fatalf("Digest(b): %v", err)
	}
	if da == db {
		t.Fatalf("Digest: want different digests for different models, both = %q", da)
	}
}

func TestDisplayState_FallbackAndRename(t *testing.T) {
	m := &Model{}
	if got := m.DisplayState("feature", "accepted-pending-build"); got != "accepted-pending-build" {
		t.Fatalf("DisplayState fallback = %q, want the id unchanged", got)
	}

	m.Vocabulary.States = map[string]string{"accepted-pending-build": "Ready to build"}
	if got := m.DisplayState("feature", "accepted-pending-build"); got != "Ready to build" {
		t.Fatalf("DisplayState rename = %q, want %q", got, "Ready to build")
	}
	// A different id with no rename entry still falls back.
	if got := m.DisplayState("feature", "draft"); got != "draft" {
		t.Fatalf("DisplayState(draft) = %q, want the id unchanged (no rename declared)", got)
	}
}

func TestDisplayVerb_FallbackAndRename(t *testing.T) {
	m := &Model{}
	if got := m.DisplayVerb("accept"); got != "accept" {
		t.Fatalf("DisplayVerb fallback = %q, want the id unchanged", got)
	}

	m.Vocabulary.Verbs = map[string]string{"accept": "Sign off"}
	if got := m.DisplayVerb("accept"); got != "Sign off" {
		t.Fatalf("DisplayVerb rename = %q, want %q", got, "Sign off")
	}
	if got := m.DisplayVerb("close"); got != "close" {
		t.Fatalf("DisplayVerb(close) = %q, want the id unchanged (no rename declared)", got)
	}
}
