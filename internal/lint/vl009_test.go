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

// vl009ShallowADRTmpl is a bare ADR carrying only the frozen stamp under
// test, authored fresh per test so %s can carry a dynamically-computed
// commit sha.
const vl009ShallowADRTmpl = `---
id: adr/vl-009-shallow
kind: adr
title: "VL-009 overlay: frozen stamp under a shallow horizon"
status: accepted
owners: [platform-team]
decided: 2026-04-01
frozen: { at: 2026-04-01, commit: %s }
---
# VL-009 overlay: frozen stamp under a shallow horizon

frozen.commit is a real ancestor in complete history; whether it is provable
from HEAD depends on whether this checkout is shallow.
`

// TestVL009_FrozenCommitShallowBeyondHorizon_Notices is the P2-10b red-first
// pin at the VL-009 seam: a frozen.commit that is genuinely reachable in
// complete history but sits BEYOND a shallow clone's horizon (its object was
// never fetched) must NOT red as a violation — it reads as a disclosed-
// unproven NOTICE (SeverityDisclosure: printed, never flips the exit),
// because shallow history cannot prove unreachability. This is exactly the
// GitHub-Actions shallow-checkout shape that redded VL-009 content-dependently
// by horizon depth (PRs #186, #192).
func TestVL009_FrozenCommitShallowBeyondHorizon_Notices(t *testing.T) {
	// L1 = store root; the ADR (pinning frozen.commit = L1) is committed at
	// L2, so a --depth 1 clone keeps L2 (with the ADR in its tree) but leaves
	// L1 beyond the horizon and unfetched.
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{".verdi/verdi.yaml": setupManifestYAML, ".gitattributes": setupGitAttributes}, Message: "store root"},
	})
	beyond := repo.Heads[0]

	adrDir := filepath.Join(repo.Dir, ".verdi", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", adrDir, err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "vl-009-shallow.md"), []byte(fmt.Sprintf(vl009ShallowADRTmpl, beyond)), 0o644); err != nil {
		t.Fatalf("writing vl-009-shallow.md: %v", err)
	}
	commitAll(t, repo.Dir, "add ADR whose frozen.commit is L1")

	clone := fixturegit.ShallowClone(t, repo, 1)

	findings := runLint(t, clone, Context{}, Options{})
	onlyRule(t, findings, "VL-009")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want exactly 1 VL-009 disclosure:\n%s", len(findings), findingsString(findings))
	}
	f := findings[0]
	if f.Severity != SeverityDisclosure {
		t.Fatalf("severity = %v, want SeverityDisclosure (a printed notice, not a verdict failure — shallow cannot prove unreachability)", f.Severity)
	}
	if !containsAll(f.Message, beyond, "frozen.commit", "shallow history cannot prove unreachability") {
		t.Fatalf("message = %q, want it to name the stamp (frozen.commit), the commit %q, and \"shallow history cannot prove unreachability\"", f.Message, beyond)
	}
	if s := f.String(); !strings.HasPrefix(s, "disclosed-unproven [lint:VL-009] ") {
		t.Fatalf("String() = %q, want a printed \"disclosed-unproven [lint:VL-009] ...\" disclosure line", s)
	}
	// The disclosure carries NO wall locus: it surfaces through the disclosures
	// channel, never as a board badge (VLBadges keys on Locus alone).
	if f.Locus != nil {
		t.Fatalf("disclosure Locus = %+v, want nil (a disclosure never badges the wall)", f.Locus)
	}
}

// TestVL009_FrozenCommitShallowWithinHorizon_Clean proves the asymmetric
// other half at the VL-009 seam: in the SAME shallow clone, a frozen.commit
// that is a within-horizon ancestor (present, visibly reachable) is proven
// reachable and fires nothing — positive proof is shallow-independent.
func TestVL009_FrozenCommitShallowWithinHorizon_Clean(t *testing.T) {
	// L1 = store root, L2 = filler; the ADR (pinning frozen.commit = L2) is
	// committed at L3, so a --depth 2 clone keeps L3 and L2 (within horizon)
	// while L1 falls beyond it.
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{".verdi/verdi.yaml": setupManifestYAML, ".gitattributes": setupGitAttributes}, Message: "store root"},
		{Files: map[string]string{"notes.txt": "filler\n"}, Message: "filler"},
	})
	within := repo.Heads[1]

	adrDir := filepath.Join(repo.Dir, ".verdi", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", adrDir, err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "vl-009-shallow.md"), []byte(fmt.Sprintf(vl009ShallowADRTmpl, within)), 0o644); err != nil {
		t.Fatalf("writing vl-009-shallow.md: %v", err)
	}
	commitAll(t, repo.Dir, "add ADR whose frozen.commit is L2")

	clone := fixturegit.ShallowClone(t, repo, 2)

	findings := runLint(t, clone, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-009" {
			t.Fatalf("VL-009 fired on a within-horizon (proven-reachable) frozen.commit in a shallow clone: %s", f.String())
		}
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
