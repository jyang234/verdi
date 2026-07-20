package lint

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

func TestVL003_DanglingLink(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-003", "dangling-link"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

func TestVL003_DanglingPin(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-003", "dangling-pin"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

// TestVL003_DanglingFragment is the R4-I-3 rescope's core new behavior: an
// object-id fragment (#<object-id>) naming a real target spec but an object
// id that spec does not declare fails VL-003, not a silent pass (the
// pre-rescope engine never resolved fragments against ByRef at all).
func TestVL003_DanglingFragment(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-003", "dangling-fragment"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

// TestVL003_ResolvingFragment_Clean proves the positive complement: a
// fragment naming a real object id on a real target resolves cleanly.
func TestVL003_ResolvingFragment_Clean(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-003-resolving-fragment/spec.md", `---
id: spec/vl-003-resolving-fragment
kind: spec
class: story
title: "VL-003: resolving object-id fragment"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0098
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-003: resolving object-id fragment

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-003" {
			t.Fatalf("VL-003 fired on a fragment that resolves cleanly: %s", f.String())
		}
	}
}

// TestVL003_UnknownEdgeTypeOnFragment_FailsClosed proves 02's "their edge
// types are the closed five-value enum ... unknown types fail closed"
// (VL-003's own amended row, R4-I-3): a fragment-targeting link whose type
// (here "verifies", a known link type but outside the closed
// implements/resolves/supersedes/exempts/depends-on set) is not eligible
// to target an object fragment fails VL-003. Note this is *not* caught
// anywhere else: internal/lint's walk deliberately decodes via
// artifact.DecodeStrict only, never the kind's own semantic Validate()
// (doc.go's design note), so VL-003 is the sole enforcement point.
func TestVL003_UnknownEdgeTypeOnFragment_FailsClosed(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-003-bad-edge-type/spec.md", `---
id: spec/vl-003-bad-edge-type
kind: spec
class: story
title: "VL-003: fragment-targeting verifies edge"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0097
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
  - { type: verifies, ref: "spec/stale-decline#ac-2" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-003: fragment-targeting verifies edge

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 VL-003:\n%s", len(findings), findingsString(findings))
	}
}

// vl003DanglingPinSpecTmpl and vl003ReachablePinSpecTmpl mirror
// testdata/violations/VL-003/dangling-pin/...'s own shape (a feature spec
// carrying a single context[] pin under test), authored fresh per test so
// %s can carry a dynamically-computed commit sha rather than a literal baked
// into a committed fixture. The pinned kind/name half is adr/0002-outbox-events
// — a real ADR the committed corpus already carries
// (examples/showcase/.verdi/adr/0002-outbox-events.md, layers.txt layer 2) —
// so the pin's kind/name half resolves in the committed zone and the ONLY
// thing under test is the commit half's reachability (checkPin's git predicate).
const vl003DanglingPinSpecTmpl = `---
id: spec/vl-003-dangling-pin-x11b
kind: spec
class: feature
title: "VL-003 overlay: context pin names a locally-dangling commit"
status: draft
owners: [platform-team]
story: jira:LOAN-0001
context:
  - adr/0002-outbox-events@%s
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-003 overlay: context pin names a locally-dangling commit

context[0]'s kind/name half (adr/0002-outbox-events) resolves in the
committed zone, but its commit half is a locally-dangling object no branch or
ref anywhere reaches (X-11b) — the false green a mere object-existence check
(gitx.CommitExists) accepts, which the reachability-from-HEAD check reds.
`

const vl003ReachablePinSpecTmpl = `---
id: spec/vl-003-reachable-pin
kind: spec
class: feature
title: "VL-003 overlay: context pin names a legitimately reachable commit"
status: draft
owners: [platform-team]
story: jira:LOAN-0001
context:
  - adr/0002-outbox-events@%s
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-003 overlay: context pin names a legitimately reachable commit

context[0] pins adr/0002-outbox-events at an ordinary ancestor of HEAD — the
tightened reachability-from-HEAD check must leave this entirely unaffected.
`

// TestVL003_ContextPinDangling_Reds proves ac-3's tightening applies to
// VL-003's own git predicate too (judged-vl003-pin-check-keeps-x11b-false-green-predicate):
// a context[] pin whose commit half exists as a locally-dangling object
// (created, then stripped of every ref that would keep it reachable —
// fixturegit.Dangle, X-11b's exact false green) reds under the tightened
// reachability-from-HEAD check, where the old "is real git history" predicate
// (mere object existence, gitx.CommitExists) would have silently accepted it.
// The pin's kind/name half resolves cleanly (adr/0002-outbox-events is real),
// so the sole finding is the commit half's unreachability.
func TestVL003_ContextPinDangling_Reds(t *testing.T) {
	repo := buildLintRepo(t)
	dangling := fixturegit.Dangle(t, repo, map[string]string{"orphan.txt": "orphan\n"}, "orphaned commit")
	// Dangle commits on a throwaway branch and checks main back out, which
	// sweeps away buildLintRepo's UNTRACKED discovery + mutable-zone working-tree
	// fixtures; restore them so the corpus lints exactly as TestVL003_DanglingPin's
	// does (loansvc's boundary-contract ref resolves, mutable zone present) and
	// the dangling pin is the only finding.
	writeLoansvcFixture(t, repo.Dir)
	provisionMutableZone(t, repo.Dir)

	specRel := filepath.Join(".verdi", "specs", "active", "vl-003-dangling-pin-x11b", "spec.md")
	writeTestFile(t, filepath.Join(repo.Dir, specRel), fmt.Sprintf(vl003DanglingPinSpecTmpl, dangling))
	commitPaths(t, repo.Dir, "add spec pinning a locally-dangling commit", specRel)

	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, dangling) {
		t.Errorf("finding message = %q, want the dangling commit %q named", findings[0].Message, dangling)
	}
	if !strings.Contains(findings[0].Message, "not reachable from HEAD") {
		t.Errorf("finding message = %q, want it to speak of reachability from HEAD (the tightened check), not mere existence", findings[0].Message)
	}
}

// TestVL003_ContextPinReachable_Unaffected proves the other half: a context[]
// pin whose commit half legitimately IS reachable through ordinary history —
// a plain ancestor of HEAD, nothing dangling about it — is entirely unaffected
// by the tightened check.
func TestVL003_ContextPinReachable_Unaffected(t *testing.T) {
	repo := buildLintRepo(t)
	reachable := repo.Heads[0]

	specRel := filepath.Join(".verdi", "specs", "active", "vl-003-reachable-pin", "spec.md")
	writeTestFile(t, filepath.Join(repo.Dir, specRel), fmt.Sprintf(vl003ReachablePinSpecTmpl, reachable))
	commitPaths(t, repo.Dir, "add spec pinning a reachable ancestor commit", specRel)

	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-003" {
			t.Fatalf("VL-003 fired on a legitimately reachable context pin: %s", f.String())
		}
	}
}
