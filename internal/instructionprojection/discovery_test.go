package instructionprojection

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestDiscovery_CaseVariantInstructionFile_FailsClosed proves discovery
// matches a declared instruction filename CASE-INSENSITIVELY. On a
// case-insensitive filesystem (APFS, NTFS) a planted "agents.md" IS the
// file the harness opens when it asks for "AGENTS.md", so byte-exact
// matching would let a hand-authored instruction file be read by the
// harness while this witness reported nothing — a silent false CLEAN,
// the exact outcome CO-1 forbids.
func TestDiscovery_CaseVariantInstructionFile_FailsClosed(t *testing.T) {
	tests := []struct {
		name string
		rel  string
		code Reason
	}{
		{"lowercased at repo root", "agents.md", ReasonUnmanaged},
		{"mixed case nested", "services/legacy/Agents.MD", ReasonShadowing},
		{"uppercased extension nested", "docs/agents.MD", ReasonShadowing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, unmanagedAdapterStoreFiles())
			writeTree(t, root, map[string]string{tt.rel: "a hand-authored instruction file in a case variant\n"})

			report, err := Verify(root)
			if err != nil {
				t.Fatalf("Verify(): %v", err)
			}
			findOne(t, report, "codex", tt.code, tt.rel)
		})
	}
}

// TestDiscovery_ExactCaseManagedFileStaysClean is the case-folding
// rule's positive arm: folding must not turn an adapter's OWN generated
// files into findings. The fixture adapter manages AGENTS.md and
// docs/AGENTS.md and discovers AGENTS.md, so both files are matched by
// the folded rule and both must be satisfied by the managed set.
func TestDiscovery_ExactCaseManagedFileStaysClean(t *testing.T) {
	root := newFixtureRoot(t)
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	for _, f := range report.Findings {
		if f.Code == ReasonUnmanaged || f.Code == ReasonShadowing {
			t.Fatalf("case-folded discovery flagged a generated managed file: %+v", f)
		}
	}
	if !report.Clean() {
		t.Fatalf("Verify() after Generate() = %+v, want Clean()", report.Findings)
	}
}

// TestReport_DisclosesExcludedSubtrees proves every report — clean or
// not — carries the walk's own exclusion disclosure. The walk never
// descends into .git, node_modules, or .verdi/data, and an unexamined
// subtree that no report mentions is exactly the silence CO-1 forbids:
// a reader must be able to see WHAT was not examined, not only what was.
func TestReport_DisclosesExcludedSubtrees(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, unmanagedAdapterStoreFiles())
	writeTree(t, root, map[string]string{
		"node_modules/pkg/AGENTS.md":  "vendored, never examined\n",
		".git/AGENTS.md":              "git metadata, never examined\n",
		".verdi/data/run/AGENTS.md":   "verdi-owned working data, never examined\n",
		"testdata/store/AGENTS.md":    "testdata IS examined\n",
		"vendor/thirdparty/AGENTS.md": "an ordinary directory IS examined\n",
	})

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}

	want := []string{".git", ".verdi/data", "node_modules"}
	if len(report.ExcludedSubtrees) != len(want) {
		t.Fatalf("ExcludedSubtrees = %v, want %v", report.ExcludedSubtrees, want)
	}
	for i := range want {
		if report.ExcludedSubtrees[i] != want[i] {
			t.Fatalf("ExcludedSubtrees = %v, want exactly %v (sorted)", report.ExcludedSubtrees, want)
		}
	}
	if !sort.StringsAreSorted(report.ExcludedSubtrees) {
		t.Fatalf("ExcludedSubtrees = %v, want sorted", report.ExcludedSubtrees)
	}

	// The excluded subtrees produce no findings ...
	for _, f := range report.Findings {
		for _, ex := range []string{"node_modules/", ".git/", ".verdi/data/"} {
			if len(f.Path) >= len(ex) && f.Path[:len(ex)] == ex {
				t.Fatalf("a finding was reported inside an excluded subtree: %+v", f)
			}
		}
	}
	// ... while testdata and an ordinary vendor directory are examined
	// like any other tree: an instruction file cannot hide under a
	// fixture directory.
	findOne(t, report, "codex", ReasonShadowing, "testdata/store/AGENTS.md")
	findOne(t, report, "codex", ReasonShadowing, "vendor/thirdparty/AGENTS.md")
}

// TestReport_DisclosesExcludedSubtrees_OnACleanReport proves the
// disclosure is unconditional: a report with zero findings still names
// the subtrees the walk never entered.
func TestReport_DisclosesExcludedSubtrees_OnACleanReport(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, zeroAdapterStoreFiles())

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if !report.Clean() {
		t.Fatalf("Verify() = %+v, want Clean()", report.Findings)
	}
	if len(report.ExcludedSubtrees) == 0 {
		t.Fatal("a clean report carries no exclusion disclosure; the walk's excluded subtrees must always be stated")
	}
}

// TestDiscovery_ExcludedSubtreeContentIsReallySkipped is the mechanical
// arm behind the disclosure: a file that WOULD be a finding anywhere
// else produces none inside an excluded subtree.
func TestDiscovery_ExcludedSubtreeContentIsReallySkipped(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, unmanagedAdapterStoreFiles())
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "dep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "dep", "AGENTS.md"), []byte("vendored\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if !report.Clean() {
		t.Fatalf("Verify() = %+v, want Clean(): a vendored dependency tree is never part of the project's own instruction chain", report.Findings)
	}
}
