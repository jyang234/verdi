package evidence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func writeObligationFixture(t *testing.T, root, specName, fileName, content string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "obligations", specName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(content), 0o644); err != nil {
		t.Fatalf("writing obligation fixture: %v", err)
	}
}

const testObligationStaticAC1 = `---
id: obligation/widget-story--ac-1--static
kind: obligation
title: "Static analysis obligation for AC-1"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/widget-story" }
frozen: { at: 2026-07-13, commit: 3e91ab2 }
---
# Static analysis obligation for AC-1

A golangci-lint pass over the touched packages must be clean.
`

const testObligationBehavioralAC1 = `---
id: obligation/widget-story--ac-1--behavioral
kind: obligation
title: "Behavioral obligation for AC-1"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/widget-story" }
frozen: { at: 2026-07-13, commit: 3e91ab2 }
---
# Behavioral obligation for AC-1

A Playwright e2e test drives the edit form and asserts persistence.
`

const testObligationStaticAC2 = `---
id: obligation/widget-story--ac-2--static
kind: obligation
title: "Static analysis obligation for AC-2"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/widget-story" }
frozen: { at: 2026-07-13, commit: 3e91ab2 }
---
# Static analysis obligation for AC-2

A golangci-lint pass over the touched packages must be clean.
`

// TestObligations_Present proves a present, well-formed obligation decodes
// and is returned keyed by its own for_kind, with both title and body
// preserved (spec/obligation-wall DC-1/CO-2: the same loader must carry
// enough for the board's own richer title-plus-prose render, ac-2, even
// though matrix, ac-1, renders only the title).
func TestObligations_Present(t *testing.T) {
	root := t.TempDir()
	writeObligationFixture(t, root, "widget-story", "ac-1--static.md", testObligationStaticAC1)

	got, err := Obligations(root, "widget-story", "ac-1")
	if err != nil {
		t.Fatalf("Obligations: %v", err)
	}
	obl, ok := got[artifact.EvidenceStatic]
	if !ok {
		t.Fatalf("Obligations = %+v, want a static entry", got)
	}
	if obl.Title != "Static analysis obligation for AC-1" {
		t.Errorf("Title = %q", obl.Title)
	}
	if !strings.Contains(obl.Body, "golangci-lint") {
		t.Errorf("Body = %q, want the golangci-lint prose preserved", obl.Body)
	}
}

// TestObligations_MultipleKinds proves the returned map is keyed by
// for_kind across more than one file for the same AC, and that a kind with
// no matching file simply has no key in the map (spec/obligation-wall
// DC-2: a missing obligation is the ordinary "none" case).
func TestObligations_MultipleKinds(t *testing.T) {
	root := t.TempDir()
	writeObligationFixture(t, root, "widget-story", "ac-1--static.md", testObligationStaticAC1)
	writeObligationFixture(t, root, "widget-story", "ac-1--behavioral.md", testObligationBehavioralAC1)

	got, err := Obligations(root, "widget-story", "ac-1")
	if err != nil {
		t.Fatalf("Obligations: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(Obligations) = %d, want 2: %+v", len(got), got)
	}
	if got[artifact.EvidenceStatic].Title != "Static analysis obligation for AC-1" {
		t.Errorf("static Title = %q", got[artifact.EvidenceStatic].Title)
	}
	if got[artifact.EvidenceBehavioral].Title != "Behavioral obligation for AC-1" {
		t.Errorf("behavioral Title = %q", got[artifact.EvidenceBehavioral].Title)
	}
	if _, ok := got[artifact.EvidenceRuntime]; ok {
		t.Errorf("Obligations has a runtime entry, want none (no such file was written)")
	}
}

// TestObligations_None proves a missing obligation reads as the ordinary
// "none" case (an empty result, no error) — both when the spec has no
// .verdi/obligations/ directory at all, and when the directory exists (for
// a sibling AC) but this AC has no file of its own.
func TestObligations_None(t *testing.T) {
	t.Run("no obligations directory at all", func(t *testing.T) {
		root := t.TempDir()
		got, err := Obligations(root, "widget-story", "ac-1")
		if err != nil {
			t.Fatalf("Obligations: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("Obligations = %+v, want empty", got)
		}
	})

	t.Run("spec directory exists, this ac has no files", func(t *testing.T) {
		root := t.TempDir()
		writeObligationFixture(t, root, "widget-story", "ac-2--static.md", testObligationStaticAC2)

		got, err := Obligations(root, "widget-story", "ac-1")
		if err != nil {
			t.Fatalf("Obligations: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("Obligations(ac-1) = %+v, want empty (only ac-2 has a file)", got)
		}
	})

	t.Run("no store root at all", func(t *testing.T) {
		got, err := Obligations(filepath.Join(t.TempDir(), "does-not-exist"), "widget-story", "ac-1")
		if err != nil {
			t.Fatalf("Obligations: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("Obligations = %+v, want empty", got)
		}
	})
}

// TestObligations_Broken proves a present-but-malformed obligation file is
// a surfaced error, never silently treated as absent (spec/obligation-wall
// DC-1/DC-2: "a broken obligation is not 'no obligation'" — only genuine
// absence gets the disclosed-none treatment).
func TestObligations_Broken(t *testing.T) {
	root := t.TempDir()
	const broken = `---
id: obligation/widget-story--ac-1--static
kind: obligation
title: "Broken"
owners: [platform-team]
for_kind: bogus-not-a-kind
links:
  - { type: verifies, ref: "spec/widget-story" }
frozen: { at: 2026-07-13, commit: 3e91ab2 }
---
# Broken
`
	writeObligationFixture(t, root, "widget-story", "ac-1--static.md", broken)

	if _, err := Obligations(root, "widget-story", "ac-1"); err == nil {
		t.Fatal("Obligations(malformed for_kind): want error, got nil")
	}
}

// TestObligations_BrokenFrontmatter proves a file that isn't even a valid
// frontmatter document (no closing delimiter) is also a surfaced error,
// not silent absence.
func TestObligations_BrokenFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeObligationFixture(t, root, "widget-story", "ac-1--static.md", "not a frontmatter document at all\n")

	if _, err := Obligations(root, "widget-story", "ac-1"); err == nil {
		t.Fatal("Obligations(no frontmatter delimiters): want error, got nil")
	}
}
