package lint

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestVL021_DanglingRef proves VL-021 fires, naming the offending ref, when
// a class: proposal diagram's derived_from.ref names a diagram that does
// not exist anywhere in the corpus.
func TestVL021_DanglingRef(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-021", "dangling-ref"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-021")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, "diagram/does-not-exist") {
		t.Errorf("finding does not name the offending ref: %s", findings[0].Message)
	}
}

// TestVL021_MalformedDigest proves VL-021 fires, naming the offending
// value, when a class: proposal diagram's derived_from.digest is not
// sha256:<64-hex>, even though its derived_from.ref names a real diagram.
func TestVL021_MalformedDigest(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-021", "malformed-digest"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-021")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, "not-a-real-digest") {
		t.Errorf("finding does not name the offending digest value: %s", findings[0].Message)
	}
}

// TestVL021_MalformedSourceDigest proves VL-021 fires, naming the
// offending value, when a class: proposal diagram carries a present but
// pattern-invalid derived_from.source_digest (ADJ-16) — even though its
// ref and digest are both well-formed. This is the present-bad half of
// the source_digest table (absence is proven never-a-finding by every
// other VL-021 fixture, which omits source_digest entirely).
func TestVL021_MalformedSourceDigest(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-021", "malformed-source-digest"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-021")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, "not-a-source-digest") {
		t.Errorf("finding does not name the offending source_digest value: %s", findings[0].Message)
	}
	if !strings.Contains(findings[0].Message, "source_digest") {
		t.Errorf("finding does not name the source_digest field: %s", findings[0].Message)
	}
}

// TestVL021_CleanSourceDigest is the present-good half: a class: proposal
// diagram whose OPTIONAL derived_from.source_digest is well-formed
// sha256:<64-hex> trips nothing.
func TestVL021_CleanSourceDigest(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-021", "clean-source-digest"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-021" {
			t.Fatalf("VL-021 fired on a clean source_digest: %s", f.String())
		}
	}
}

// TestVL021_Clean proves VL-021 is silent for a class: proposal diagram
// whose derived_from correctly names a real corpus diagram with a
// well-formed sha256:<64-hex> digest — a test that only exercised the
// clean case would not satisfy this obligation on its own, but paired
// with the two negatives above it proves the rule is neither silent nor
// trigger-happy.
func TestVL021_Clean(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-021", "clean"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-021" {
			t.Fatalf("VL-021 fired on a clean derived_from: %s", f.String())
		}
	}
}

// TestVL021_IncumbentDiagramIgnored proves VL-021 never even looks at an
// incumbent diagram (class absent) — the golden corpus's own
// diagram/loansvc-topology fixture, carrying no derived_from at all,
// must never trip this rule.
func TestVL021_IncumbentDiagramIgnored(t *testing.T) {
	repo := buildLintRepo(t)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-021" {
			t.Fatalf("VL-021 fired on the plain golden corpus (no proposal diagrams at all): %s", f.String())
		}
	}
}
