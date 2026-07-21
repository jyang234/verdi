package lint

import (
	"context"
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

// vl003RootBindingsTargetSpecMD is the ad hoc spec target root-discovery
// tests bind against: one declared ac-1, nothing else, so a bindings entry
// naming any other ac id is unambiguously wrong.
const vl003RootBindingsTargetSpecMD = `---
id: spec/vl-003-root-bindings-target
kind: spec
class: story
title: "VL-003: root bindings target"
status: draft
owners: [platform-team]
story: jira:LOAN-0099
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-003: root bindings target

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

// TestVL003_RootBindings_BadBareAC_Reds is spec/ritual-traps ac-3's
// red-to-green demonstration: a repository ROOT verdi.bindings.yaml
// (sibling of .verdi/, no .flowmap.yaml anywhere at the module root —
// buildLintRepo's own fixture never puts one there, exactly today's real
// shape D6-4 describes) naming a bare ac id its target spec does not
// declare. Before ac-3's root-discovery path, checkBindings iterates only
// discovered Services and never sees this file at all, so this fixture
// passes lint silently; after, it must red, naming the offending entry.
func TestVL003_RootBindings_BadBareAC_Reds(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi", "specs", "active", "vl-003-root-bindings-target", "spec.md"), vl003RootBindingsTargetSpecMD)
	writeTestFile(t, filepath.Join(dir, "verdi.bindings.yaml"), `schema: verdi.bindings/v1
spec: spec/vl-003-root-bindings-target
bindings:
  - { producer: some-producer, kind: static, acs: [ac-99] }
`)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, "ac-99") {
		t.Errorf("finding message = %q, want it to name the offending ac-99 entry", findings[0].Message)
	}
	if !strings.Contains(findings[0].Path, "verdi.bindings.yaml") {
		t.Errorf("finding path = %q, want it to name verdi.bindings.yaml (root)", findings[0].Path)
	}
}

// TestVL003_RootBindings_BadBareAC_ServicesOnlyLoop_Blind is spec/ritual-traps
// ac-3's *before-leg*, demonstrated-by-pin rather than asserted-in-comment
// (judged-ac3-before-leg-of-red-to-green-only-asserted). ac-3's letter requires
// the pre-fix absence be "not merely asserted but demonstrated red-to-green";
// TestVL003_RootBindings_BadBareAC_Reds above pins only the after-leg, and its
// before condition lived solely in that test's prose. This pins the before
// mechanically, on the SAME bad-bare-AC fixture: with the module root's
// RootBindings WITHHELD from the Snapshot (nil, no decode error — exactly the
// pre-ea868c3 Services-only view, since checkBindings's root-discovery block is
// a no-op precisely when RootBindings and RootBindingsErr are both nil, so the
// current code then behaves identically to the checkBindings that had no
// root-discovery block at all), the Services-only loop yields ZERO findings:
// finding no .flowmap.yaml at the module root (D6-4), it never discovers the
// root file.
//
// The two legs are self-proving and guard the companion against vacuity: the
// SAME checkBindings on the SAME Snapshot WITH RootBindings present must red
// naming ac-99 (proving the fixture is live and the mechanism can fire on this
// exact input), and withholding RootBindings alone must silence it. The delta
// is solely the RootBindings field, so the finding's appearance is controlled
// entirely by the root-discovery path — structurally proving only it can see
// the file.
func TestVL003_RootBindings_BadBareAC_ServicesOnlyLoop_Blind(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi", "specs", "active", "vl-003-root-bindings-target", "spec.md"), vl003RootBindingsTargetSpecMD)
	writeTestFile(t, filepath.Join(dir, "verdi.bindings.yaml"), `schema: verdi.bindings/v1
spec: spec/vl-003-root-bindings-target
bindings:
  - { producer: some-producer, kind: static, acs: [ac-99] }
`)
	repo := buildLintRepo(t, dir)

	snap, err := BuildSnapshot(repo.Dir, Options{})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	in := &RunInput{Ctx: context.Background(), Root: repo.Dir, Snapshot: snap, Opts: Options{}}
	r := vl003{}

	// Non-vacuity leg: the fixture's root bindings file really was decoded, and
	// the SAME checkBindings, with RootBindings present, reds naming ac-99 —
	// without this the zero below could be a dead fixture, not a real absence.
	if snap.RootBindings == nil {
		t.Fatalf("fixture precondition: BuildSnapshot did not decode the root verdi.bindings.yaml (RootBindingsErr=%v); the withheld-vs-present contrast would be vacuous", snap.RootBindingsErr)
	}
	full := r.checkBindings(in)
	if len(full) != 1 {
		t.Fatalf("with RootBindings present, checkBindings produced %d findings, want 1 (the root file's ac-99 entry):\n%s", len(full), findingsString(full))
	}
	if !strings.Contains(full[0].Message, "ac-99") {
		t.Fatalf("with RootBindings present, finding = %q, want it to name the offending ac-99 entry", full[0].Message)
	}

	// Before-leg: withhold RootBindings — the pre-fix Services-only view. The
	// root file must now be invisible, so checkBindings yields zero findings.
	snap.RootBindings = nil
	snap.RootBindingsErr = nil
	if blind := r.checkBindings(in); len(blind) != 0 {
		t.Fatalf("Services-only loop (RootBindings withheld) produced %d findings, want 0 — without the root-discovery path the module-root bindings file must be invisible:\n%s", len(blind), findingsString(blind))
	}
}

// TestVL003_RootBindings_AllCorrect_StaysClean is ac-3's companion case: a
// root verdi.bindings.yaml whose entries are all correct must produce no
// VL-003 findings after the fix — the discovery path itself must not
// introduce a false positive on a clean file.
func TestVL003_RootBindings_AllCorrect_StaysClean(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi", "specs", "active", "vl-003-root-bindings-target", "spec.md"), vl003RootBindingsTargetSpecMD)
	writeTestFile(t, filepath.Join(dir, "verdi.bindings.yaml"), `schema: verdi.bindings/v1
spec: spec/vl-003-root-bindings-target
bindings:
  - { producer: some-producer, kind: static, acs: [ac-1] }
`)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-003" {
			t.Fatalf("VL-003 fired on a clean root bindings file: %s", f.String())
		}
	}
}

// vl003FragmentOwnerSpecMD is the ad hoc bindings file's own primary spec
// (`spec:`) for ac-4's fragment cross-check tests — its own AC ids are
// irrelevant to what's under test; only its existence (so `spec:` itself
// resolves) matters.
const vl003FragmentOwnerSpecMD = `---
id: spec/vl-003-fragment-owner
kind: spec
class: story
title: "VL-003: fragment owner"
status: draft
owners: [platform-team]
story: jira:LOAN-0100
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-003: fragment owner

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

// vl003FragmentTargetSpecMD is the fragment-qualified entry's NAMED spec —
// distinct from the bindings file's own primary spec — declaring exactly
// one AC (ac-1), so a fragment entry naming any other ac id is unambiguously
// a typo.
const vl003FragmentTargetSpecMD = `---
id: spec/vl-003-fragment-target
kind: spec
class: story
title: "VL-003: fragment target"
status: draft
owners: [platform-team]
story: jira:LOAN-0101
links:
  - { type: implements, ref: "spec/stale-decline#ac-2" }
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-003: fragment target

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

// TestVL003_RootBindings_TypoFragmentAC_RedsByName is spec/ritual-traps
// ac-4's core new behavior, proven on the root bindings file ac-3 just made
// a checked target: a fragment-qualified entry (spec/<name>#<ac-id>) naming
// an AC id its NAMED target spec does not declare must red, naming both the
// offending entry and the target spec — not silently pass (today's gap: the
// bare-only lookup checkOneBindingsFile carried into ac-3 never resolves a
// fragment entry against anything but the owning spec).
func TestVL003_RootBindings_TypoFragmentAC_RedsByName(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi", "specs", "active", "vl-003-fragment-owner", "spec.md"), vl003FragmentOwnerSpecMD)
	writeTestFile(t, filepath.Join(dir, ".verdi", "specs", "active", "vl-003-fragment-target", "spec.md"), vl003FragmentTargetSpecMD)
	writeTestFile(t, filepath.Join(dir, "verdi.bindings.yaml"), `schema: verdi.bindings/v1
spec: spec/vl-003-fragment-owner
bindings:
  - { producer: some-producer, kind: static, acs: ["spec/vl-003-fragment-target#ac-9"] }
`)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	msg := findings[0].Message
	if !strings.Contains(msg, "spec/vl-003-fragment-target#ac-9") {
		t.Errorf("finding message = %q, want it to name the offending fragment entry spec/vl-003-fragment-target#ac-9", msg)
	}
	// The discriminating assertion: the check must cross-check against the
	// NAMED (target) spec, not the owning spec — a bare-only lookup that
	// naively fails to match ANY fragment string would also produce a
	// finding here, but it would (wrongly) say the OWNING spec
	// (vl-003-fragment-owner) "does not declare" ac-9, never having looked
	// at the target spec's own ACs at all. Pin the correct clause exactly.
	if !strings.Contains(msg, `"spec/vl-003-fragment-target" does not declare`) {
		t.Errorf("finding message = %q, want the clause to name the TARGET spec spec/vl-003-fragment-target as the one that does not declare ac-9 (not the owning spec vl-003-fragment-owner)", msg)
	}
	if strings.Contains(msg, `"spec/vl-003-fragment-owner" does not declare`) {
		t.Errorf("finding message = %q, wrongly blames the OWNING spec rather than the fragment's named target spec", msg)
	}
}

// TestVL003_RootBindings_CorrectFragmentAC_StaysClean is ac-4's companion
// case: a fragment-qualified entry naming an AC its target spec genuinely
// DOES declare — the exact shape this design series' own bindings additions
// already are (e.g. "spec/judge-ergonomics#ac-1") — must continue to pass,
// proving the check is additive and does not regress real entries.
func TestVL003_RootBindings_CorrectFragmentAC_StaysClean(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi", "specs", "active", "vl-003-fragment-owner", "spec.md"), vl003FragmentOwnerSpecMD)
	writeTestFile(t, filepath.Join(dir, ".verdi", "specs", "active", "vl-003-fragment-target", "spec.md"), vl003FragmentTargetSpecMD)
	writeTestFile(t, filepath.Join(dir, "verdi.bindings.yaml"), `schema: verdi.bindings/v1
spec: spec/vl-003-fragment-owner
bindings:
  - { producer: some-producer, kind: static, acs: ["spec/vl-003-fragment-target#ac-1"] }
`)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-003" {
			t.Fatalf("VL-003 fired on a correct fragment-qualified entry: %s", f.String())
		}
	}
}

// TestVL003_RootBindings_PinnedFragmentAC_RedsFailClosed is spec/ritual-traps
// judged-ac4-pinned-fragment-entry-silently-unpinned: a fragment-qualified
// entry that ALSO pins a revision (spec/<name>@<commit>#<ac-id>) must red —
// this check validates AC ids against the CURRENT committed spec and cannot
// honor a revision pin. The named ac (ac-1) is one the target spec genuinely
// declares, so the dropped-pin path would (wrongly) pass green; the pin itself
// is therefore the sole reason this reds. Before the fix, ResolveBindingAC
// silently discarded the @commit and the entry validated clean against HEAD —
// a fail-open contrary to the rest of VL-003's posture. The finding must name
// the offending entry verbatim and disclose the honest reason.
func TestVL003_RootBindings_PinnedFragmentAC_RedsFailClosed(t *testing.T) {
	const pinnedEntry = "spec/vl-003-fragment-target@0123456789abcdef0123456789abcdef01234567#ac-1"
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi", "specs", "active", "vl-003-fragment-owner", "spec.md"), vl003FragmentOwnerSpecMD)
	writeTestFile(t, filepath.Join(dir, ".verdi", "specs", "active", "vl-003-fragment-target", "spec.md"), vl003FragmentTargetSpecMD)
	writeTestFile(t, filepath.Join(dir, "verdi.bindings.yaml"), `schema: verdi.bindings/v1
spec: spec/vl-003-fragment-owner
bindings:
  - { producer: some-producer, kind: static, acs: ["`+pinnedEntry+`"] }
`)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 (a pinned+fragment entry must fail closed, not silently validate its ac against HEAD):\n%s", len(findings), findingsString(findings))
	}
	msg := findings[0].Message
	if !strings.Contains(msg, pinnedEntry) {
		t.Errorf("finding message = %q, want it to name the offending pinned+fragment entry %q verbatim", msg, pinnedEntry)
	}
	for _, want := range []string{"pin", "current", "future extension"} {
		if !strings.Contains(msg, want) {
			t.Errorf("finding message = %q, want it to disclose the honest reason (contain %q)", msg, want)
		}
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
