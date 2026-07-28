package evidence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestRenderObligation_Happy pins the exact rendered byte shape — field
// order and the restricted flow-mapping style — the same contract
// internal/workbench/obligationauthor_test.go's board-driven tests already
// pin indirectly (spec/obligation-seam ac-4/O-5: this is now the one seam
// both paths render through, so this direct, byte-exact test is the
// extraction's own proof independent of the board's HTTP surface).
func TestRenderObligation_Happy(t *testing.T) {
	got := RenderObligation(ObligationInput{
		ID:          "obligation/widget-story--ac-1--behavioral",
		Title:       "the retry proves end to end",
		ForKind:     artifact.EvidenceBehavioral,
		VerifiesRef: "spec/widget-story",
		Body:        "The behavioral evidence must show an e2e test.",
		Owners:      []string{"platform-team"},
		Frozen:      artifact.NewFrozen("2026-07-21", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"),
	})
	want := `---
id: obligation/widget-story--ac-1--behavioral
kind: obligation
title: "the retry proves end to end"
owners: ["platform-team"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/widget-story" }
frozen: { at: 2026-07-21, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# the retry proves end to end

The behavioral evidence must show an e2e test.
`
	if got != want {
		t.Errorf("RenderObligation =\n%s\nwant\n%s", got, want)
	}

	// The rendered bytes must themselves round-trip through the exact
	// decode the store's own lint/fold machinery uses — never a form that
	// only happens to look right.
	fm, body, err := artifact.SplitFrontmatter([]byte(got))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	ob, err := artifact.DecodeObligation(fm)
	if err != nil {
		t.Fatalf("DecodeObligation: %v", err)
	}
	if ob.ForKind != artifact.EvidenceBehavioral {
		t.Errorf("for_kind = %q, want behavioral", ob.ForKind)
	}
	if !strings.Contains(string(body), "The behavioral evidence must show an e2e test.") {
		t.Errorf("body missing input prose: %q", body)
	}
}

// TestRenderObligation_MultipleOwners proves owners render as a
// comma-joined, individually-quoted flow sequence — plural, never a single
// hardcoded value (mirroring attest.go's own "owners copied verbatim,
// plural" precedent).
func TestRenderObligation_MultipleOwners(t *testing.T) {
	got := RenderObligation(ObligationInput{
		ID:          "obligation/widget-story--ac-2--static",
		Title:       "t",
		ForKind:     artifact.EvidenceStatic,
		VerifiesRef: "spec/widget-story",
		Body:        "b",
		Owners:      []string{"platform-team", "qa-lead"},
		Frozen:      artifact.NewFrozen("2026-07-21", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"),
	})
	if !strings.Contains(got, `owners: ["platform-team", "qa-lead"]`) {
		t.Errorf("owners line not rendered as expected:\n%s", got)
	}
}

// TestWriteObligationFile_Happy proves the write is atomic (no leftover
// .tmp sibling), creates missing parent directories, and lands exactly the
// bytes given.
func TestWriteObligationFile_Happy(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "obligations", "widget-story", "ac-1--behavioral.md")
	content := RenderObligation(ObligationInput{
		ID:          "obligation/widget-story--ac-1--behavioral",
		Title:       "t",
		ForKind:     artifact.EvidenceBehavioral,
		VerifiesRef: "spec/widget-story",
		Body:        "b",
		Owners:      []string{"platform-team"},
		Frozen:      artifact.NewFrozen("2026-07-21", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"),
	})

	if err := WriteObligationFile(path, content); err != nil {
		t.Fatalf("WriteObligationFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(got) != content {
		t.Errorf("written content = %q, want %q", got, content)
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") || strings.HasPrefix(e.Name(), ".atomicfile-") {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
}

// TestWriteObligationFile_Overwrite proves the write is UNCONDITIONAL — it
// creates or overwrites whatever is at path — the property `verdi
// obligation author`'s regenerate case depends on (spec/obligation-seam
// ac-5): existence/freeze policy is the caller's job, never this
// primitive's.
func TestWriteObligationFile_Overwrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ac-1--behavioral.md")
	first := RenderObligation(ObligationInput{
		ID: "obligation/widget-story--ac-1--behavioral", Title: "first", ForKind: artifact.EvidenceBehavioral,
		VerifiesRef: "spec/widget-story", Body: "first body", Owners: []string{"platform-team"},
		Frozen: artifact.NewFrozen("2026-07-21", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"),
	})
	second := RenderObligation(ObligationInput{
		ID: "obligation/widget-story--ac-1--behavioral", Title: "second", ForKind: artifact.EvidenceBehavioral,
		VerifiesRef: "spec/widget-story", Body: "second body", Owners: []string{"platform-team"},
		Frozen: artifact.NewFrozen("2026-07-21", "cafebabecafebabecafebabecafebabecafebabe"),
	})

	if err := WriteObligationFile(path, first); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteObligationFile(path, second); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(got) != second {
		t.Errorf("overwrite did not take effect: got %q, want %q", got, second)
	}
}

// TestWriteObligationFile_Negative_SelfValidateFailure proves a malformed
// obligation is refused before any disk write happens: no file, and — when
// the parent directory did not previously exist — no directory either.
func TestWriteObligationFile_Negative_SelfValidateFailure(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "does-not-exist-yet")
	path := filepath.Join(dir, "ac-1--behavioral.md")

	// A for_kind that disagrees with the id's own <for-kind> segment fails
	// artifact.ObligationFrontmatter.Validate's DC-2 agreement check.
	malformed := RenderObligation(ObligationInput{
		ID:          "obligation/widget-story--ac-1--behavioral",
		Title:       "t",
		ForKind:     artifact.EvidenceStatic, // disagrees with the id's "--behavioral" segment
		VerifiesRef: "spec/widget-story",
		Body:        "b",
		Owners:      []string{"platform-team"},
		Frozen:      artifact.NewFrozen("2026-07-21", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"),
	})

	err := WriteObligationFile(path, malformed)
	if err == nil {
		t.Fatal("WriteObligationFile(malformed) = nil error, want a self-validation failure")
	}
	if !strings.Contains(err.Error(), "self-validation") {
		t.Errorf("error = %q, want it to name self-validation", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("a failed self-validation must write nothing, but %s exists", path)
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Errorf("a failed self-validation must not even create the parent directory %s", dir)
	}
}

// TestWriteObligationFile_Negative_EmptyOwners proves the shared
// self-validate rejects an obligation with no owners (Base.Validate
// requires non-empty owners) — a caller bug (e.g. accept's backstop
// resolving an empty operator name) is caught here rather than writing an
// artifact that could never itself pass `verdi lint`.
func TestWriteObligationFile_Negative_EmptyOwners(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ac-1--behavioral.md")
	malformed := RenderObligation(ObligationInput{
		ID:          "obligation/widget-story--ac-1--behavioral",
		Title:       "t",
		ForKind:     artifact.EvidenceBehavioral,
		VerifiesRef: "spec/widget-story",
		Body:        "b",
		Owners:      nil,
		Frozen:      artifact.NewFrozen("2026-07-21", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"),
	})
	if err := WriteObligationFile(path, malformed); err == nil {
		t.Fatal("WriteObligationFile(no owners) = nil error, want a self-validation failure")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("a failed self-validation must write nothing, but %s exists", path)
	}
}
