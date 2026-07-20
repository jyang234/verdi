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

// TestDisplayClassPlural covers the display layer's best-effort plural
// (the vocabulary-prose closure): the no-rename fallback MUST reproduce
// today's hand-written plurals byte-for-byte (the parity floor's plural
// half — "stories", "spikes", "features"), and renamed words get the
// regular English form.
func TestDisplayClassPlural(t *testing.T) {
	renamed := &Model{Vocabulary: Vocabulary{Classes: map[string]string{
		"story":   "Change Request",
		"spike":   "Deep Dive",
		"feature": "Epic",
	}}}
	displayOnly := &Model{Classes: map[string]Class{"story": {Display: "Story"}}}
	tests := []struct {
		name string
		m    *Model
		id   string
		want string
	}{
		{"no-rename story keeps today's plural", &Model{}, "story", "stories"},
		{"no-rename spike keeps today's plural", &Model{}, "spike", "spikes"},
		{"no-rename feature keeps today's plural", &Model{}, "feature", "features"},
		{"vocabulary rename pluralizes the renamed word", renamed, "story", "Change Requests"},
		{"single-word rename", renamed, "feature", "Epics"},
		{"multi-word rename", renamed, "spike", "Deep Dives"},
		{"Class.Display consonant-Y pluralizes as -ies", displayOnly, "story", "Stories"},
		{"unknown id pluralizes its own fallback", &Model{}, "box", "boxes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.DisplayClassPlural(tt.id); got != tt.want {
				t.Fatalf("DisplayClassPlural(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}

	var nilModel *Model
	if got := nilModel.DisplayClassPlural("story"); got != "stories" {
		t.Fatalf("nil.DisplayClassPlural(story) = %q, want the id's own plural (nil-receiver fallback)", got)
	}
}

// TestPluralizeDisplay pins the helper's own rules, negative edges
// included: empty stays empty (never a bare "s"), vowel-y words never
// get -ies, sibilant endings get -es.
func TestPluralizeDisplay(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"story", "stories"},
		{"Story", "Stories"},
		{"day", "days"},
		{"key", "keys"},
		{"y", "ys"}, // a single "y" has no preceding consonant to trigger -ies
		{"epic", "epics"},
		{"process", "processes"},
		{"fix", "fixes"},
		{"blitz", "blitzes"},
		{"branch", "branches"},
		{"wish", "wishes"},
		{"path", "paths"}, // plain -th is not a sibilant ending
	}
	for _, tt := range tests {
		if got := pluralizeDisplay(tt.in); got != tt.want {
			t.Fatalf("pluralizeDisplay(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestCapitalize covers the label-position helper: first rune only,
// unicode-aware, and safe on empty input.
func TestArticle(t *testing.T) {
	tests := []struct{ in, want string }{
		{"story", "a"},
		{"feature", "a"},
		{"spike", "a"},
		{"Initiative", "an"},
		{"epic", "an"},
		{"ACR", "an"},
		{"Objective", "an"},
		{"Umbrella", "an"}, // spelling-based: a consonant-sounding 'u' still gets "an" (disclosed limit)
		{"", "a"},          // degenerate; display words are never empty (DisplayClass falls back to the id)
		{"épic", "a"},      // non-ASCII initials fall to "a" (spelling heuristic covers a/e/i/o/u only)
	}
	for _, tt := range tests {
		if got := Article(tt.in); got != tt.want {
			t.Fatalf("Article(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIndefinite(t *testing.T) {
	tests := []struct{ in, want string }{
		{"story", "a story"},
		{"feature", "a feature"},
		{"spike", "a spike"},
		{"Initiative", "an Initiative"},
		{"Change Request", "a Change Request"},
		{"superseded", "a superseded"}, // state words compose the same way
		{"", "a "},                     // degenerate compose; display words are never empty (documented at the definition)
	}
	for _, tt := range tests {
		if got := Indefinite(tt.in); got != tt.want {
			t.Fatalf("Indefinite(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCapitalize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"story", "Story"},
		{"Story", "Story"},
		{"change request", "Change request"},
		{"épic", "Épic"},
	}
	for _, tt := range tests {
		if got := Capitalize(tt.in); got != tt.want {
			t.Fatalf("Capitalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
