package initwizard

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/model"
)

// TestVerdiYAMLContent_Minimal pins the bare/wizard-shared verdi.yaml
// content: schema only (R4-I-56's conservative scope — no invented forge
// or tracker defaults), decoding cleanly through store.DecodeManifest's
// own strict decode (proven indirectly here via a schema-literal check;
// the store package itself is exercised by cmd/verdi/init_test.go's
// built-binary cases, which is where "does verdi model check accept the
// staged root" actually gets proven end to end).
func TestVerdiYAMLContent_Minimal(t *testing.T) {
	if VerdiYAMLContent != "schema: verdi.layout/v1\n" {
		t.Fatalf("VerdiYAMLContent = %q, want exactly %q", VerdiYAMLContent, "schema: verdi.layout/v1\n")
	}
}

// TestVocabularyEmpty_Table proves the divergence predicate the "model.yaml
// only on divergence from canonical" contract depends on (spec/init-wizard
// outcome; L-N5).
func TestVocabularyEmpty_Table(t *testing.T) {
	cases := []struct {
		name  string
		vocab model.Vocabulary
		want  bool
	}{
		{"all nil", model.Vocabulary{}, true},
		{"empty maps", model.Vocabulary{Classes: map[string]string{}, States: map[string]string{}, Verbs: map[string]string{}}, true},
		{"one class rename", model.Vocabulary{Classes: map[string]string{"story": "Task"}}, false},
		{"one state rename", model.Vocabulary{States: map[string]string{"draft": "Idea"}}, false},
		{"one verb rename", model.Vocabulary{Verbs: map[string]string{"accept": "Sign off"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := VocabularyEmpty(tc.vocab); got != tc.want {
				t.Fatalf("VocabularyEmpty(%+v) = %v, want %v", tc.vocab, got, tc.want)
			}
		})
	}
}

// TestCandidateModel_NoRenames_EqualsCanonical proves the zero-divergence
// case: an empty Vocabulary produces a candidate model.DeepEqual to
// model.Canonical() itself — the property that makes "wizard with every
// prompt skipped produces the same store as bare init" true.
func TestCandidateModel_NoRenames_EqualsCanonical(t *testing.T) {
	cand := CandidateModel(model.Vocabulary{})
	want := model.Canonical()
	if !reflect.DeepEqual(cand, want) {
		t.Fatalf("CandidateModel(empty) != model.Canonical():\ngot:  %+v\nwant: %+v", cand, want)
	}
}

// TestCandidateModel_AppliesVocabulary proves a non-empty Vocabulary
// lands on the candidate's own Vocabulary field with every other field
// left exactly as canonical declares it (classes/lifecycle untouched —
// the frontier's own "vocabulary and template filenames excepted" rule).
func TestCandidateModel_AppliesVocabulary(t *testing.T) {
	vocab := model.Vocabulary{Classes: map[string]string{"story": "Task"}}
	cand := CandidateModel(vocab)
	if !reflect.DeepEqual(cand.Vocabulary, vocab) {
		t.Fatalf("CandidateModel(%+v).Vocabulary = %+v, want %+v", vocab, cand.Vocabulary, vocab)
	}
	canonical := model.Canonical()
	if !reflect.DeepEqual(cand.Classes, canonical.Classes) {
		t.Fatalf("CandidateModel must leave Classes exactly as canonical declares them")
	}
	if !reflect.DeepEqual(cand.Lifecycle, canonical.Lifecycle) {
		t.Fatalf("CandidateModel must leave Lifecycle exactly as canonical declares it")
	}
}

// TestRenderModelYAML_RoundTrips is the render/decode-compare pin
// (design doc §12 W-4) at the unit level: for a representative set of
// vocabulary combinations, RenderModelYAML's bytes must decode via
// model.DecodeModel with no error, AND decode to a Model
// reflect.DeepEqual to CandidateModel(vocab) — exactly the equality
// cmd/verdi/init.go's staged-store gate re-proves against the real
// filesystem in the built-binary suite.
func TestRenderModelYAML_RoundTrips(t *testing.T) {
	cases := []struct {
		name  string
		vocab model.Vocabulary
	}{
		{"classes only", model.Vocabulary{Classes: map[string]string{"story": "Task", "spike": "Spike"}}},
		{"states only", model.Vocabulary{States: map[string]string{"draft": "Idea", "closed": "Done"}}},
		{"verbs only", model.Vocabulary{Verbs: map[string]string{"accept": "Sign off", "close": "Ship"}}},
		{"all three", model.Vocabulary{
			Classes: map[string]string{"feature": "Epic"},
			States:  map[string]string{"accepted-pending-build": "In progress"},
			Verbs:   map[string]string{"accept": "Sign off"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendered := RenderModelYAML(tc.vocab)
			decoded, err := model.DecodeModel(rendered)
			if err != nil {
				t.Fatalf("RenderModelYAML(%+v) produced undecodable YAML: %v\n---\n%s", tc.vocab, err, rendered)
			}
			want := CandidateModel(tc.vocab)
			if !reflect.DeepEqual(decoded, want) {
				t.Fatalf("RenderModelYAML(%+v) decode-compare mismatch:\ngot:  %+v\nwant: %+v", tc.vocab, decoded, want)
			}
		})
	}
}

// TestRenderModelYAML_EscapesUserInput proves a display value carrying
// YAML-hostile bytes (an embedded colon-space, a double quote, a
// newline-shaped smuggle attempt) is safely quoted (artifact.
// YAMLDoubleQuote, mirroring internal/workbench/obligationauthor.go's
// renderObligation's own precedent) rather than corrupting the
// surrounding document — the K4 class of defect this codebase already
// closed once at designscaffold's safeScalar.
func TestRenderModelYAML_EscapesUserInput(t *testing.T) {
	hostile := `Ready: "to ship"` + "\nsecond-line-should-not-become-a-key: evil"
	vocab := model.Vocabulary{States: map[string]string{"closed": hostile}}
	rendered := RenderModelYAML(vocab)
	decoded, err := model.DecodeModel(rendered)
	if err != nil {
		t.Fatalf("RenderModelYAML with a hostile display value produced undecodable YAML: %v\n---\n%s", err, rendered)
	}
	if decoded.Vocabulary.States["closed"] != hostile {
		t.Fatalf("hostile value round-tripped incorrectly: got %q, want %q", decoded.Vocabulary.States["closed"], hostile)
	}
	// The escaped form must not introduce a second top-level key (the
	// exact smuggle RenderModelYAML must close) or a second `closed:`
	// vocabulary entry.
	if strings.Count(string(rendered), "\nsecond-line-should-not-become-a-key:") != 0 {
		t.Fatalf("hostile newline was not escaped — rendered content:\n%s", rendered)
	}
}

// TestRenderModelYAML_EmptyVocabulary_NoVocabularyBlock proves the
// caller-visible contract RenderModelYAML itself upholds when handed an
// empty Vocabulary: the rendered bytes decode to a Vocabulary that is
// itself empty (mirroring model.Canonical()'s own empty display layer),
// never a spuriously-present-but-empty block.
func TestRenderModelYAML_EmptyVocabulary_NoVocabularyBlock(t *testing.T) {
	rendered := RenderModelYAML(model.Vocabulary{})
	decoded, err := model.DecodeModel(rendered)
	if err != nil {
		t.Fatalf("RenderModelYAML(empty): %v", err)
	}
	if len(decoded.Vocabulary.Classes) != 0 || len(decoded.Vocabulary.States) != 0 || len(decoded.Vocabulary.Verbs) != 0 {
		t.Fatalf("RenderModelYAML(empty) decoded a non-empty Vocabulary: %+v", decoded.Vocabulary)
	}
}
