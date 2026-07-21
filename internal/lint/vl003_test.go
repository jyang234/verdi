package lint

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/store"
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

// TestVL003_RootGuard_UnnormalizedRoot_NoDoubleCheck is
// judged-ac3-root-guard-path-equality: checkBindings skips its
// unconditional root-discovery pass only when some discovered Service is
// ALREADY rooted exactly at the module root (guarding a hypothetical store
// that IS a flowmap service of itself from having its one root bindings file
// double-checked under two labels). That guard is `svc.Dir == in.Root`, a
// raw string compare — but svc.Dir is always filepath.Abs-normalized
// (internal/store/discovery.go), while in.Root is whatever the caller passed.
// A caller passing a relative or trailing-slash root defeats the guard: the
// root-rooted service is NOT recognized, so the SAME root bindings file is
// checked twice — once as the service's sidecar, once as RootBindings —
// doubling every finding under two different path labels.
//
// This hand-builds exactly that store (a Service rooted at the module root,
// whose Bindings are the very *artifact.Bindings BuildSnapshot also reads
// into RootBindings) and passes in.Root with a trailing slash. Before
// normalization, the single bad-AC entry reds TWICE; after, once — and the
// surviving finding is the SERVICE-labeled one, proving the root file was
// correctly recognized as the service and its root-discovery copy suppressed.
func TestVL003_RootGuard_UnnormalizedRoot_NoDoubleCheck(t *testing.T) {
	root := t.TempDir() // absolute, already clean

	owner := &Document{
		RelPath: ".verdi/specs/active/test-owner/spec.md",
		Base:    artifact.Base{ID: "spec/test-owner"},
		Spec:    &artifact.SpecFrontmatter{AcceptanceCriteria: []artifact.AcceptanceCriterion{{ID: "ac-1"}}},
	}
	// One bad bare entry (ac-99, undeclared by the owner) is the single
	// finding whose duplication under two labels is the tell.
	badBindings := &artifact.Bindings{
		Schema: "verdi.bindings/v1",
		Spec:   "spec/test-owner",
		Bindings: []artifact.Binding{
			{Producer: "some-producer", Kind: artifact.EvidenceStatic, ACs: []string{"ac-99"}},
		},
	}
	snap := &Snapshot{
		Root:  root,
		ByRef: map[string][]*Document{"spec/test-owner": {owner}},
		// The root-rooted service carries the same file BuildSnapshot also
		// read into RootBindings — the real store's shape when a .flowmap.yaml
		// sits at the module root.
		Services:     []store.Service{{Name: "root-svc", Dir: root, Bindings: badBindings}},
		RootBindings: badBindings,
	}

	// Trailing-slash caller: filepath.Abs normalizes it away, but a raw
	// compare against the already-clean svc.Dir does not.
	in := &RunInput{Ctx: context.Background(), Root: root + string(filepath.Separator), Snapshot: snap, Opts: Options{}}
	findings := vl003{}.checkBindings(in)

	if len(findings) != 1 {
		t.Fatalf("checkBindings produced %d findings for an unnormalized (trailing-slash) root, want 1 — the root-rooted service must be recognized so its root bindings file is not also checked as RootBindings:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Path, "root-svc") {
		t.Errorf("surviving finding path = %q, want the SERVICE-labeled copy (verdi.bindings.yaml (root-svc)) — proving the root file was recognized as the service and its duplicate root-discovery check suppressed", findings[0].Path)
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

// TestVL003_RootBindings_MissingTargetSpec_Reds is
// judged-vl-003-missing-target-spec-untested: checkOneBindingsFile's
// missing-target-spec arm — a fragment-qualified entry spec/<name>#<ac-id>
// whose NAMED target spec does not resolve in the committed zone AT ALL — had
// no negative-path test. Its sibling arm (target resolves but does not declare
// the ac) is pinned by TestVL003_RootBindings_TypoFragmentAC_RedsByName; the
// "target does not resolve" arm was exercised by nothing. This pins it on the
// root bindings file: a fragment entry naming a spec absent from the store
// reds, naming both the offending entry and the unresolved target spec — not
// silently pass, and not blamed on the (perfectly resolving) owning spec.
func TestVL003_RootBindings_MissingTargetSpec_Reds(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi", "specs", "active", "vl-003-fragment-owner", "spec.md"), vl003FragmentOwnerSpecMD)
	writeTestFile(t, filepath.Join(dir, "verdi.bindings.yaml"), `schema: verdi.bindings/v1
spec: spec/vl-003-fragment-owner
bindings:
  - { producer: some-producer, kind: static, acs: ["spec/vl-003-does-not-exist#ac-1"] }
`)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	msg := findings[0].Message
	if !strings.Contains(msg, "spec/vl-003-does-not-exist#ac-1") {
		t.Errorf("finding message = %q, want it to name the offending fragment entry", msg)
	}
	if !strings.Contains(msg, `whose target spec "spec/vl-003-does-not-exist" does not resolve to a spec in the committed zone`) {
		t.Errorf("finding message = %q, want the missing-target-spec clause naming the unresolved target spec (not the owning spec)", msg)
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

// TestVL003_RootBindings_BrokenOwnerSpec_TypoFragmentAC_BothRed is
// spec/ritual-traps judged-ac4-broken-owning-spec-ref-masks-fragment-typos:
// the COMBINED shape a broken owning `spec:` ref + a typo'd fragment-qualified
// entry make together. checkOneBindingsFile used to return immediately after
// the "spec %q does not resolve" finding, skipping EVERY AC entry — but a
// fragment-qualified entry's validation per ac-4 depends only on its NAMED
// target spec, not on the file's own primary `spec:` at all. So a root
// verdi.bindings.yaml whose owner ref is broken (here an unauthored owner
// spec, the same honest false a typo'd or archived owner ref yields) hid every
// typo'd fragment AC id behind the single owning-ref finding: lint reds, but
// not BY NAME on the fragment entry, and fixing the owner would reveal a
// second red the author was never told about. ac-4's promise — "a typo'd AC id
// inside a fragment-qualified entry reds lint by name" — silently narrowed to
// "only while the file's unrelated primary spec ref is healthy". Before the
// fix this combined shape produced ONE finding (owning-ref only); after, it
// produces TWO — the owning-ref AND the fragment entry, named independently.
func TestVL003_RootBindings_BrokenOwnerSpec_TypoFragmentAC_BothRed(t *testing.T) {
	dir := t.TempDir()
	// The fragment's NAMED target IS authored (declares only ac-1). The
	// bindings file's OWN owner spec (spec/vl-003-nonexistent-owner) is NOT
	// authored — a well-formed `spec:` ref that does not resolve, standing in
	// for the typo'd-owner / archived-owner class the finding describes.
	writeTestFile(t, filepath.Join(dir, ".verdi", "specs", "active", "vl-003-fragment-target", "spec.md"), vl003FragmentTargetSpecMD)
	writeTestFile(t, filepath.Join(dir, "verdi.bindings.yaml"), `schema: verdi.bindings/v1
spec: spec/vl-003-nonexistent-owner
bindings:
  - { producer: some-producer, kind: static, acs: ["spec/vl-003-fragment-target#ac-9"] }
`)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2 (the broken owning-ref AND the typo'd fragment entry named independently — the fragment must not hide behind the owning-ref):\n%s", len(findings), findingsString(findings))
	}

	var sawOwnerRef, sawFragmentByName bool
	for _, f := range findings {
		if strings.Contains(f.Message, `spec "spec/vl-003-nonexistent-owner" does not resolve to a spec in the committed zone`) {
			sawOwnerRef = true
		}
		// The fragment must red BY NAME against its own target spec — not the
		// owning spec, which does not even resolve.
		if strings.Contains(f.Message, "spec/vl-003-fragment-target#ac-9") &&
			strings.Contains(f.Message, `"spec/vl-003-fragment-target" does not declare`) {
			sawFragmentByName = true
		}
	}
	if !sawOwnerRef {
		t.Errorf("want a finding naming the broken owning spec ref; got:\n%s", findingsString(findings))
	}
	if !sawFragmentByName {
		t.Errorf("want the typo'd fragment entry to red BY NAME against its target spec (unnarrowed by the broken owner); got:\n%s", findingsString(findings))
	}
}

// TestVL003_RootBindings_BrokenOwnerSpec_BareAC_StaysMasked is the companion
// discipline for judged-ac4-broken-owning-spec-ref-masks-fragment-typos:
// validating fragment entries independently must NOT also un-mask bare ones. A
// bare ac-<slug> entry genuinely resolves against bindings.Spec, so when that
// owner is broken it stays masked behind the single owning-ref finding rather
// than spawning a redundant per-entry "target spec does not resolve" copy —
// exactly one finding, never two. (This guards against the naive over-fix that
// simply deletes the early return without keeping bare entries skipped.)
func TestVL003_RootBindings_BrokenOwnerSpec_BareAC_StaysMasked(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "verdi.bindings.yaml"), `schema: verdi.bindings/v1
spec: spec/vl-003-nonexistent-owner
bindings:
  - { producer: some-producer, kind: static, acs: [ac-99] }
`)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want exactly 1 (only the broken owning-ref; a bare entry against a broken owner must not spawn a redundant per-entry finding):\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, `spec "spec/vl-003-nonexistent-owner" does not resolve`) {
		t.Errorf("finding = %q, want the single finding to be the broken owning-ref", findings[0].Message)
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

// vl003ShallowPinSpecTmpl mirrors vl003ReachablePinSpecTmpl (a feature spec
// carrying a single context[] pin under test) with a fresh id — the pinned
// kind/name half is adr/0002-outbox-events (a real committed-corpus ADR, so
// it resolves in the committed zone) and the ONLY thing under test is the
// commit half's reachability under a shallow horizon.
const vl003ShallowPinSpecTmpl = `---
id: spec/vl-003-shallow-pin
kind: spec
class: feature
title: "VL-003 overlay: context pin under a shallow horizon"
status: draft
owners: [platform-team]
story: jira:LOAN-0001
context:
  - adr/0002-outbox-events@%s
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-003 overlay: context pin under a shallow horizon

context[0] pins adr/0002-outbox-events at a real ancestor of HEAD that, in a
shallow clone, sits beyond the horizon and is unfetched.
`

// TestVL003_ContextPinShallowBeyondHorizon_Notices is the P2-10b red-first
// pin at VL-003's checkPin seam: a context[] pin whose commit half is a real
// ancestor but sits BEYOND a shallow clone's horizon reads as a disclosed-
// unproven NOTICE (SeverityDisclosure), never a violation — shallow history
// cannot prove unreachability. The pin's kind/name half resolves cleanly, so
// the ONLY VL-003 finding is the commit half's shallow-unprovable disclosure.
func TestVL003_ContextPinShallowBeyondHorizon_Notices(t *testing.T) {
	repo := buildLintRepo(t)
	beyond := repo.Heads[0] // first corpus layer — a real, deep ancestor of HEAD

	specRel := filepath.Join(".verdi", "specs", "active", "vl-003-shallow-pin", "spec.md")
	writeTestFile(t, filepath.Join(repo.Dir, specRel), fmt.Sprintf(vl003ShallowPinSpecTmpl, beyond))
	commitPaths(t, repo.Dir, "add spec pinning a beyond-horizon ancestor", specRel)

	// A --depth 1 clone keeps only the tip (the committed corpus tree, with
	// the pinned ADR present, materialized in it) and leaves every ancestor —
	// including the pinned `beyond` commit — beyond the horizon. The untracked
	// discovery + mutable-zone fixtures are re-provisioned in the clone (a
	// clone copies only committed content), matching buildLintRepo's posture.
	clone := fixturegit.ShallowClone(t, repo, 1)
	writeLoansvcFixture(t, clone)
	provisionMutableZone(t, clone)

	findings := runLint(t, clone, Context{}, Options{})

	// The corpus-wide P2-10b outcome: under a shallow horizon EVERY frozen
	// stamp / pin the corpus carries is beyond the horizon, yet no VL-009 or
	// VL-003 reachability check may red as a violation — each discloses.
	// (The corpus's frozen.commit stamps are fixturegit's own deterministic
	// layer SHAs, so they are real ancestors in a full clone and beyond the
	// horizon here.)
	for _, f := range findings {
		if (f.Rule == "VL-003" || f.Rule == "VL-009") && f.Severity != SeverityDisclosure {
			t.Fatalf("reachability rule %s redded as a violation under a shallow horizon (want disclosed-unproven): %s", f.Rule, f.String())
		}
	}

	// This spec's own context[] pin is the disclosure under test.
	wantPath := filepath.ToSlash(specRel)
	var got *Finding
	for i := range findings {
		if findings[i].Rule == "VL-003" && findings[i].Path == wantPath {
			got = &findings[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("no VL-003 finding for %s; findings:\n%s", wantPath, findingsString(findings))
	}
	if got.Severity != SeverityDisclosure {
		t.Fatalf("severity = %v, want SeverityDisclosure (shallow cannot prove unreachability)", got.Severity)
	}
	if !containsAll(got.Message, beyond, "shallow history cannot prove unreachability") {
		t.Fatalf("message = %q, want it to name the commit %q and \"shallow history cannot prove unreachability\"", got.Message, beyond)
	}
	if s := got.String(); !strings.HasPrefix(s, "disclosed-unproven [lint:VL-003] ") {
		t.Fatalf("String() = %q, want a printed \"disclosed-unproven [lint:VL-003] ...\" disclosure line", s)
	}
	// A disclosure never carries a wall locus (even reached through the
	// context[] call site's locusAll(SpecLocus()) wrap) — it surfaces through
	// the disclosures channel, not the board's VL badges (which key on Locus).
	if got.Locus != nil {
		t.Fatalf("disclosure Locus = %+v, want nil (a disclosure never badges the wall)", got.Locus)
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
