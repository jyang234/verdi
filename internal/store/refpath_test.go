package store

import (
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestNewPerKindAbsolutePaths locks the four previously-missing per-kind
// path assemblers (adr, diagram, conflict, reaffirmation) against
// docs/design/specs/01-store-layout.md's directory table, the same
// explicit-slash-template style TestAbsolutePaths already uses for the
// spec/attestation/waiver/obligation family.
func TestNewPerKindAbsolutePaths(t *testing.T) {
	const root = "/store"
	tests := []struct {
		name      string
		got       string
		wantSlash string
	}{
		{"ADRPath", ADRPath(root, "widget-choice"), "/store/.verdi/adr/widget-choice.md"},
		{"DiagramPath", DiagramPath(root, "widget-flow"), "/store/.verdi/diagrams/widget-flow.mermaid"},
		{"ConflictPath", ConflictPath(root, "widget-challenge"), "/store/.verdi/conflicts/widget-challenge.md"},
		{"ReaffirmationDir", ReaffirmationDir(root, "story-7"), "/store/.verdi/reaffirmations/story-7"},
		{"ReaffirmationPath", ReaffirmationPath(root, "story-7", "ac-2"), "/store/.verdi/reaffirmations/story-7/ac-2.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := filepath.FromSlash(tt.wantSlash)
			if tt.got != want {
				t.Errorf("got %q, want %q", tt.got, want)
			}
		})
	}
}

// TestNewPerKindEmptyRootDisplayForm mirrors TestAttestationPathEmptyRootDisplayForm:
// an empty root drops the leading element, yielding the store-relative
// display/identity form ResolveDeclaredContext's git-tree lookups need.
func TestNewPerKindEmptyRootDisplayForm(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"ADRPath", ADRPath("", "widget-choice"), ".verdi/adr/widget-choice.md"},
		{"DiagramPath", DiagramPath("", "widget-flow"), ".verdi/diagrams/widget-flow.mermaid"},
		{"ConflictPath", ConflictPath("", "widget-challenge"), ".verdi/conflicts/widget-challenge.md"},
		{"ReaffirmationPath", ReaffirmationPath("", "story-7", "ac-2"), ".verdi/reaffirmations/story-7/ac-2.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := filepath.FromSlash(tt.want)
			if tt.got != want {
				t.Errorf("got %q, want %q", tt.got, want)
			}
		})
	}
}

// TestNonSpecArtifactPath proves NonSpecArtifactPath — SI-92's one shared
// canonical store path table for every closed-registry kind except spec —
// dispatches each kind to exactly the path its own single-purpose
// accessor would build, so the table can never silently diverge from the
// per-kind functions it composes.
func TestNonSpecArtifactPath(t *testing.T) {
	tests := []struct {
		name string
		kind artifact.Kind
		ref  string // ref name to resolve
		want string
	}{
		{"adr", artifact.KindADR, "widget-choice", ADRPath("", "widget-choice")},
		{"diagram", artifact.KindDiagram, "widget-flow", DiagramPath("", "widget-flow")},
		{"conflict", artifact.KindConflict, "widget-challenge", ConflictPath("", "widget-challenge")},
		{"attestation", artifact.KindAttestation, "story-7--ac-2", AttestationPath("", "story-7", "ac-2")},
		{"waiver", artifact.KindWaiver, "story-7--ac-2", WaiverPath("", "story-7", "ac-2")},
		{"reaffirmation", artifact.KindReaffirmation, "story-7--ac-2", ReaffirmationPath("", "story-7", "ac-2")},
		{"obligation", artifact.KindObligation, "widget--ac-2--behavioral", ObligationPath("", "widget", "ac-2", "behavioral")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NonSpecArtifactPath(tt.kind, tt.ref)
			if err != nil {
				t.Fatalf("NonSpecArtifactPath(%q, %q): %v", tt.kind, tt.ref, err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNonSpecArtifactPath_RejectsSpecKind proves the table explicitly
// refuses spec — spec resolution is a distinct active/archive zone search
// over the pinned tree, not a single fixed path (SI-92).
func TestNonSpecArtifactPath_RejectsSpecKind(t *testing.T) {
	if _, err := NonSpecArtifactPath(artifact.KindSpec, "widget"); err == nil {
		t.Fatal("NonSpecArtifactPath(KindSpec, ...): want error, got nil")
	}
}

// TestNonSpecArtifactPath_RejectsUnknownKind and malformed compound names
// both fail closed rather than silently building a best-effort path.
func TestNonSpecArtifactPath_RejectsMalformedCompoundName(t *testing.T) {
	tests := []struct {
		name string
		kind artifact.Kind
		ref  string
	}{
		{"attestation missing separator", artifact.KindAttestation, "story-7"},
		{"waiver missing separator", artifact.KindWaiver, "story-7"},
		{"reaffirmation missing separator", artifact.KindReaffirmation, "story-7"},
		{"obligation missing one separator", artifact.KindObligation, "widget--ac-2"},
		{"unknown kind", artifact.Kind("bogus"), "widget"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NonSpecArtifactPath(tt.kind, tt.ref); err == nil {
				t.Fatalf("NonSpecArtifactPath(%q, %q): want error, got nil", tt.kind, tt.ref)
			}
		})
	}
}
