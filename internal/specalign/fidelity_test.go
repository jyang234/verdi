// Self-hosted spec fidelity (00-index.md §How these documents are
// maintained: "resident at .verdi/specs/active/ as the first citizens of
// the system they describe. They are drafted as status: draft and
// activate on merge"). This is the check that keeps the self-hosted copy
// and its docs/design/specs/ origin honest with each other until the
// store becomes the sole home (deliverable 1a).
package specalign

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestSelfHostedSpecFidelity asserts every spec under
// .verdi/specs/active/<name>/spec.md is byte-identical to its
// ../docs/design/specs/ origin except the single
// "status: draft" -> "status: active" line. Any other drift — including
// no drift at all (an un-activated copy) — fails with a diff summary.
//
// The docs live OUTSIDE this repo, workspace-relative
// (verdi/../docs/design/specs/): a CI checkout of verdi alone (no
// workspace root, no sibling docs/) cannot run this check at all. That
// case SKIPS, loudly, with an explicit disclosure — it never silently
// passes, because a skip is not a pass (CLAUDE.md's three-valued
// honesty: proven / violated-with-witness / disclosed-as-unproven).
func TestSelfHostedSpecFidelity(t *testing.T) {
	docsDir := workspaceDocsDir(verdiRepoRoot)
	if info, err := os.Stat(docsDir); err != nil || !info.IsDir() {
		t.Skipf("DISCLOSURE: workspace docs dir %s not found (%v) — this looks like a checkout of verdi alone, not the full verdi-system workspace. Self-hosted spec fidelity CANNOT be verified in this layout. This is a SKIP, not a pass: a green run here is NOT proof the self-hosted specs match their origins.", docsDir, err)
	}

	activeDir := filepath.Join(verdiRepoRoot, ".verdi", "specs", "active")
	entries, err := os.ReadDir(activeDir)
	if err != nil {
		t.Fatalf("reading %s: %v", activeDir, err)
	}

	var specDirs []string
	for _, e := range entries {
		if e.IsDir() {
			specDirs = append(specDirs, e.Name())
		}
	}
	sort.Strings(specDirs)

	if len(specDirs) != 6 {
		t.Fatalf("expected exactly 6 self-hosted specs under %s, found %d: %v", activeDir, len(specDirs), specDirs)
	}

	for _, name := range specDirs {
		t.Run(name, func(t *testing.T) {
			hostedPath := filepath.Join(activeDir, name, "spec.md")
			hosted, err := os.ReadFile(hostedPath)
			if err != nil {
				t.Fatalf("reading %s: %v", hostedPath, err)
			}

			// Origin filename discovery is convention-based, not
			// hardcoded to a numeric prefix (00-, 01-, ...): the spec
			// dir's own name minus its "verdi-" prefix must appear as
			// the origin file's "-<suffix>.md" tail exactly once. This
			// keeps the mapping robust to the docs directory's numbering
			// scheme, which is not this test's concern.
			suffix := strings.TrimPrefix(name, "verdi-")
			matches, err := filepath.Glob(filepath.Join(docsDir, "*-"+suffix+".md"))
			if err != nil {
				t.Fatalf("globbing %s for suffix %q: %v", docsDir, suffix, err)
			}
			if len(matches) != 1 {
				t.Fatalf("expected exactly one docs/design/specs file matching suffix %q (for self-hosted spec %q), found %d: %v", suffix, name, len(matches), matches)
			}
			docPath := matches[0]

			original, err := os.ReadFile(docPath)
			if err != nil {
				t.Fatalf("reading %s: %v", docPath, err)
			}

			diff, ok := onlyStatusLineFlipped(string(original), string(hosted))
			if !ok {
				t.Fatalf("self-hosted spec %s (%s) has drifted from its origin (%s) beyond the status: draft -> active line:\n%s",
					name, hostedPath, docPath, strings.Join(diff, "\n"))
			}
		})
	}
}

// onlyStatusLineFlipped reports whether hosted differs from original in
// EXACTLY one line, and that line is precisely the frontmatter activation
// flip ("status: draft" in original, "status: active" in hosted). diff is
// a line-oriented summary of every differing line (for the failure
// message) regardless of ok's value.
func onlyStatusLineFlipped(original, hosted string) (diff []string, ok bool) {
	origLines := strings.Split(original, "\n")
	hostLines := strings.Split(hosted, "\n")

	if len(origLines) != len(hostLines) {
		return []string{fmt.Sprintf("line count differs: origin has %d line(s), hosted has %d", len(origLines), len(hostLines))}, false
	}

	var diffIdx []int
	for i := range origLines {
		if origLines[i] != hostLines[i] {
			diffIdx = append(diffIdx, i)
		}
	}
	for _, i := range diffIdx {
		diff = append(diff, fmt.Sprintf("line %d:\n  origin: %s\n  hosted: %s", i+1, origLines[i], hostLines[i]))
	}

	if len(diffIdx) != 1 {
		return diff, false
	}
	i := diffIdx[0]
	return diff, origLines[i] == "status: draft" && hostLines[i] == "status: active"
}
