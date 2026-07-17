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

// TestDisplayClass_ThreeLevelChain proves class display resolution's
// spec/vocabulary-surfaces chain: Vocabulary.Classes[id] when a rename is
// declared, else the class's own Class.Display, else the id itself.
func TestDisplayClass_ThreeLevelChain(t *testing.T) {
	tests := []struct {
		name string
		m    *Model
		id   string
		want string
	}{
		{
			name: "id fallback when the model declares nothing",
			m:    &Model{},
			id:   "feature",
			want: "feature",
		},
		{
			name: "class's own Display when no vocabulary rename exists",
			m:    &Model{Classes: map[string]Class{"feature": {Display: "Epic"}}},
			id:   "feature",
			want: "Epic",
		},
		{
			name: "vocabulary rename wins over the class's own Display",
			m: &Model{
				Classes:    map[string]Class{"feature": {Display: "Epic"}},
				Vocabulary: Vocabulary{Classes: map[string]string{"feature": "Initiative"}},
			},
			id:   "feature",
			want: "Initiative",
		},
		{
			name: "empty vocabulary value falls through to Display",
			m: &Model{
				Classes:    map[string]Class{"feature": {Display: "Epic"}},
				Vocabulary: Vocabulary{Classes: map[string]string{"feature": ""}},
			},
			id:   "feature",
			want: "Epic",
		},
		{
			name: "unknown id falls back to itself (spike is not a model class)",
			m:    &Model{Classes: map[string]Class{"feature": {Display: "Epic"}}},
			id:   "spike",
			want: "spike",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.DisplayClass(tt.id); got != tt.want {
				t.Fatalf("DisplayClass(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

// TestDisplayLookups_NilReceiverSafe proves a nil *Model resolves every id
// to itself — the no-model fallback every consuming surface leans on so a
// store whose model could not be resolved renders ids, never panics.
func TestDisplayLookups_NilReceiverSafe(t *testing.T) {
	var m *Model
	if got := m.DisplayState("feature", "accepted-pending-build"); got != "accepted-pending-build" {
		t.Fatalf("nil.DisplayState = %q, want the id unchanged", got)
	}
	if got := m.DisplayVerb("accept"); got != "accept" {
		t.Fatalf("nil.DisplayVerb = %q, want the id unchanged", got)
	}
	if got := m.DisplayClass("feature"); got != "feature" {
		t.Fatalf("nil.DisplayClass = %q, want the id unchanged", got)
	}
}

// TestCanonicalDisplayLayerEmpty pins spec/vocabulary-surfaces' parity
// floor at its root: the embedded canonical model declares NO display
// overrides — no vocabulary block and no per-class Display labels — so
// every display lookup over a store with no model.yaml falls back to the
// bare id and every surface prints byte-identical output to a pre-model
// binary ("absence changes nothing").
func TestCanonicalDisplayLayerEmpty(t *testing.T) {
	m := Canonical()
	if len(m.Vocabulary.Verbs)+len(m.Vocabulary.States)+len(m.Vocabulary.Classes) != 0 {
		t.Fatalf("canonical vocabulary must be empty, got %+v", m.Vocabulary)
	}
	for name, c := range m.Classes {
		if c.Display != "" {
			t.Fatalf("canonical class %q declares Display %q; the canonical display layer must be empty so DisplayClass falls back to the bare id", name, c.Display)
		}
		if got := m.DisplayClass(name); got != name {
			t.Fatalf("canonical DisplayClass(%q) = %q, want the bare id", name, got)
		}
	}
}
