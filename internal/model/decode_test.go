package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// readTestdata reads a fixture from internal/model/testdata, shared by
// every test file in this package.
func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading testdata/%s: %v", name, err)
	}
	return data
}

// TestDecodeModel_Happy proves DecodeModel accepts both positive fixtures:
// the canonical shape itself, and a structurally identical model varying
// only vocabulary and per-class template filenames (dc-1's frontier).
func TestDecodeModel_Happy(t *testing.T) {
	cases := []string{"canonical.yaml", "vocab-rename.yaml"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			m, err := DecodeModel(readTestdata(t, name))
			if err != nil {
				t.Fatalf("DecodeModel(%s): %v", name, err)
			}
			if m.Schema != modelSchema {
				t.Fatalf("Schema = %q, want %q", m.Schema, modelSchema)
			}
			if len(m.Classes) != 2 {
				t.Fatalf("Classes = %d entries, want 2 (feature, story)", len(m.Classes))
			}
			if len(m.Lifecycle) != 2 {
				t.Fatalf("Lifecycle = %d entries, want 2 (feature, story)", len(m.Lifecycle))
			}
		})
	}
}

// TestDecodeModel_UnknownTopLevelKeyRejected proves DecodeModel routes
// through internal/artifact's shared strict-decode seam: KnownFields(true)
// rejects a top-level key Model does not declare.
func TestDecodeModel_UnknownTopLevelKeyRejected(t *testing.T) {
	_, err := DecodeModel(readTestdata(t, "viol-unknown-top-level-key.yaml"))
	if err == nil {
		t.Fatal("DecodeModel: want error for an unknown top-level key, got nil")
	}
	if !strings.Contains(err.Error(), "strict decode") {
		t.Fatalf("error = %q, want it to mention the strict-decode (KnownFields) wall", err.Error())
	}
}

// TestDecodeModel_DialectAnchorRejected proves the shared seam's restricted
// dialect (PLAN.md I-1) applies to model.yaml exactly as it does to every
// other artifact: an anchor is rejected outright, unaliased or not.
func TestDecodeModel_DialectAnchorRejected(t *testing.T) {
	_, err := DecodeModel(readTestdata(t, "viol-dialect-anchor.yaml"))
	if err == nil {
		t.Fatal("DecodeModel: want error for a YAML anchor, got nil")
	}
	if !strings.Contains(err.Error(), "anchor") {
		t.Fatalf("error = %q, want it to name the anchor dialect violation", err.Error())
	}
}

// TestTransitionObligations_NilVsEmpty proves DecodeModel's decode step
// distinguishes an ABSENT `obligations:` key (nil slice) from an explicit
// `obligations: []` (non-nil, zero-length) — spec/model-schema ac-1's own
// "decode must tell nil from []" requirement — independent of the kernel
// rule (validate.go's Lifecycle.validate) that consumes the distinction.
// Decodes via the bare artifact.DecodeStrict seam (not DecodeModel) so this
// test isolates the decode fact from Validate/checkFrontier, which would
// otherwise reject this fixture's two-state toy shape as off-frontier.
func TestTransitionObligations_NilVsEmpty(t *testing.T) {
	const absent = `schema: verdi.model/v1
classes:
  feature: { display: Feature, template: feature.md }
lifecycle:
  feature:
    states: [draft, accepted-pending-build]
    terminal: [accepted-pending-build]
    transitions:
      - { verb: accept, from: draft, to: accepted-pending-build }
`
	var m Model
	if err := artifact.DecodeStrict([]byte(absent), &m); err != nil {
		t.Fatalf("decoding absent-obligations fixture: %v", err)
	}
	if got := m.Lifecycle["feature"].Transitions[0].Obligations; got != nil {
		t.Fatalf("Obligations = %#v, want nil for an absent `obligations:` key", got)
	}

	const explicitEmpty = `schema: verdi.model/v1
classes:
  feature: { display: Feature, template: feature.md }
lifecycle:
  feature:
    states: [draft, accepted-pending-build]
    terminal: [accepted-pending-build]
    transitions:
      - { verb: accept, from: draft, to: accepted-pending-build, obligations: [] }
`
	var m2 Model
	if err := artifact.DecodeStrict([]byte(explicitEmpty), &m2); err != nil {
		t.Fatalf("decoding explicit-empty-obligations fixture: %v", err)
	}
	got := m2.Lifecycle["feature"].Transitions[0].Obligations
	if got == nil {
		t.Fatal("Obligations = nil, want a non-nil empty slice for an explicit `obligations: []`")
	}
	if len(got) != 0 {
		t.Fatalf("Obligations = %#v, want zero-length", got)
	}
}
