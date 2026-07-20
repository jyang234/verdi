package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

func TestVL009_FrozenCommitNotRealHistory(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-009"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-009")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

// vl009DanglingADRTmpl and vl009ReachableADRTmpl mirror
// testdata/violations/VL-009/.verdi/adr/vl-009-bad-frozen.md's own shape
// (a bare ADR carrying only the frozen stamp under test), authored fresh
// per test so %s can carry a dynamically-computed commit sha rather than
// a literal baked into a committed fixture.
const vl009DanglingADRTmpl = `---
id: adr/vl-009-dangling
kind: adr
title: "VL-009 overlay: frozen stamp names a locally-dangling commit"
status: accepted
owners: [platform-team]
decided: 2026-04-01
frozen: { at: 2026-04-01, commit: %s }
---
# VL-009 overlay: frozen stamp names a locally-dangling commit

frozen.commit is well-formed and names a real, locally-present git object,
but no branch or ref anywhere reaches it (X-11b) — the false green a mere
object-existence check accepts.
`

const vl009ReachableADRTmpl = `---
id: adr/vl-009-reachable
kind: adr
title: "VL-009 overlay: frozen stamp names a legitimately reachable commit"
status: accepted
owners: [platform-team]
decided: 2026-04-01
frozen: { at: 2026-04-01, commit: %s }
---
# VL-009 overlay: frozen stamp names a legitimately reachable commit

frozen.commit names a real, ordinary ancestor of HEAD — the tightened
reachability-from-HEAD check must leave this entirely unaffected.
`

// TestVL009_FrozenCommitDangling_Reds proves ac-3's core: a frozen.commit
// that exists as a locally-dangling object (created, then stripped of
// every ref that would keep it reachable — fixturegit.Dangle, X-11b's
// exact false green) reds under the tightened reachability-from-HEAD
// check, where the old "is a real commit" predicate (mere object
// existence, gitx.CommitExists) would have silently accepted it.
func TestVL009_FrozenCommitDangling_Reds(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{".verdi/verdi.yaml": setupManifestYAML, ".gitattributes": setupGitAttributes},
			Message: "store root",
		},
	})
	dangling := fixturegit.Dangle(t, repo, map[string]string{"orphan.txt": "orphan\n"}, "orphaned commit")

	adrDir := filepath.Join(repo.Dir, ".verdi", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", adrDir, err)
	}
	adr := fmt.Sprintf(vl009DanglingADRTmpl, dangling)
	if err := os.WriteFile(filepath.Join(adrDir, "vl-009-dangling.md"), []byte(adr), 0o644); err != nil {
		t.Fatalf("writing vl-009-dangling.md: %v", err)
	}
	commitAll(t, repo.Dir, "add ADR with a locally-dangling frozen.commit")

	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-009")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, dangling) {
		t.Errorf("finding message = %q, want the dangling commit %q named", findings[0].Message, dangling)
	}
	if !strings.Contains(findings[0].Message, "reachable") {
		t.Errorf("finding message = %q, want it to speak of reachability (the tightened check), not mere existence", findings[0].Message)
	}
}

// TestVL009_FrozenCommitReachable_Unaffected proves ac-3's other half: a
// frozen.commit that legitimately IS reachable through ordinary history —
// a plain ancestor of HEAD, nothing dangling about it — is entirely
// unaffected by the tightened check.
func TestVL009_FrozenCommitReachable_Unaffected(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{".verdi/verdi.yaml": setupManifestYAML, ".gitattributes": setupGitAttributes},
			Message: "store root",
		},
	})
	reachable := repo.Heads[0]

	adrDir := filepath.Join(repo.Dir, ".verdi", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", adrDir, err)
	}
	adr := fmt.Sprintf(vl009ReachableADRTmpl, reachable)
	if err := os.WriteFile(filepath.Join(adrDir, "vl-009-reachable.md"), []byte(adr), 0o644); err != nil {
		t.Fatalf("writing vl-009-reachable.md: %v", err)
	}
	commitAll(t, repo.Dir, "add ADR with a reachable frozen.commit")

	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-009" {
			t.Fatalf("VL-009 fired on a legitimately reachable frozen.commit: %s", f.String())
		}
	}
}
