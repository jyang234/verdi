# Spec Documents — Wave 2 Implementation Plan (ac-3 board tab, ac-4 docs site, ac-5 MCP, ac-6 parity, readiness seam)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the Wave 1 document core through the board (a Document tab with conditional refresh, copy, and download), the docs site (a Document view plus spec/plan/tasks Markdown files), and MCP (`get_document`), prove the four consumers byte-identical, and close ac-1's readiness clause.

**Architecture:** One new loader package, `internal/specdocload`, assembles a `specdoc.Input` from a store root, a spec name, and a mode (accepted bytes on the default branch, a pinned commit, or the working tree), resolving status through `specstate`, evidence through `matrixprojection`, and readiness from a caller-supplied snapshot when it targets the same spec. The CLI, the board, the docs site, and MCP all call that loader and then `specdoc.Build` + `specdoc.RenderMarkdown`/`RenderHTML`; the parity test pins that the four Markdown outputs for one `(ref, mode, commit)` are the same bytes. `specdoc` gains a Readiness section (engine version 2) so the clause "and the readiness snapshot when present" has a witness.

**Tech Stack:** Go 1.25 (`github.com/jyang234/verdi`), `internal/specdoc` (Wave 1), `internal/specstate`, `internal/matrixprojection`, `internal/readinesspilot`, `internal/workbench` (net/http mux, ETag/If-None-Match snapshot pattern, `boardspecasd.js` polling idiom), `internal/dex` (static site writer), `internal/mcpserve` (strict JSON tool args, `toolJSON`/`toolError`), Playwright (chromium, `workers: 1`), `internal/fixturegit`.

**Spec:** `.verdi/specs/active/spec-documents/spec.md` on main (accepted in PR #327). This plan implements ac-3, ac-4, ac-5, ac-6 and the readiness clause of ac-1, under co-1..co-6 and dc-2, dc-3, dc-8. Wave 1 (`docs/superpowers/plans/2026-09-17-spec-documents-wave-1.md`, report `docs/superpowers/reports/2026-09-18-spec-documents-wave-1.md`) is the base.

## Global Constraints

- No network in any test (co-1). CLI paths through the built binary over `internal/fixturegit` repos with `CI_DEFAULT_BRANCH=main`; browser paths through Playwright under `e2e/`; MCP paths through the in-process `Backend` and the stdio server; forge and git through fixtures.
- The document is never authority (co-2): every render carries the Wave 1 stamp; no consumer writes to the store; the board's `/document` routes are GET-only.
- The write surface stays closed (co-3): `get_document` is read-only; `mutate_draft` and `add_annotation` remain the only MCP write tools.
- Browser-facing markup, CSS, JS, and their Playwright paths (Tasks 5 and 6) are Fable work (co-4). Screenshot, trace, video, and screen recording stay disabled; `e2e/playwright.config.ts` already sets `trace: "off"`.
- co-5: every new production literal containing a class word (`feature`, `story`, `component`, `spike`) or a lifecycle state word (`draft`, `proposed`, `accepted-pending-build`, `accepted`, `superseded`, `closed`) routes through `*model.Model` (`DisplayClass`, `DisplayState`) or carries `// vocab:identity — <why>` on its line or the line above. `go test -count=1 ./internal/specalign/ -run TestVocabProseWitness` must pass after every task.
- co-6: an unavailable fact is stated in fixed wording, never omitted. Readiness that was not supplied says "Readiness was not supplied for this render."
- Wave 6 workbench presentation rules (dc-2): the Document tab is a page route plus a `/snapshot` conditional projection route; poll every two seconds only while visible, send the last exact revision token as `If-None-Match`, `304 Not Modified` on an unchanged token, replace only the document region, keep focus and scroll, expose a keyboard-reachable Refresh control, pause on `document.hidden` and refresh once on return; one accepted-HEAD resolution per page; no new JS asset over 64 KiB uncompressed; the page is usable before JS runs. Canonical refs never appear raw in a path: use the page's own request path to build sibling links.
- Determinism: `dex` output stays byte-identical across rebuilds; `get_document` returns byte-identical JSON for unchanged state; no wall clock, no map iteration in output.
- gofmt-clean, `golangci-lint` clean (`.golangci.yml`), `go vet` clean. Commit subjects imperative. Implementer commits end with `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>` (Sonnet lanes) or `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>` (Fable lanes). Never write a `// path/to/file.go` code-block marker into a file.
- Work happens in `/Users/johnyang/code/verdi-system/verdi-wt/spec-documents-w2` on branch `agent/spec-documents-wave-2` (stacked on `agent/spec-documents-wave-1` at 203e1e14). Never use bare `git stash`. Read `/Users/johnyang/code/verdi-system/CLAUDE.md` first.

---

## Rulings recorded in this plan

- **R-W2-1 (shared loader).** Four consumers can only be byte-identical if they build the same `Input`; the CLI's loading logic moves to `internal/specdocload` and the CLI calls it. Cost if wrong: one package that the CLI alone could have kept.
- **R-W2-2 (readiness is a section).** ac-1 forbids silent omission, so readiness gets `SectionReadiness` in the `spec` and `tasks` kinds, rendered from a `Facts.Readiness` value or stated as not supplied. `engineVersion` becomes 2; every stamp and golden changes once. Cost if wrong: one golden regeneration.
- **R-W2-3 (readiness scope).** The board supplies readiness only when the served snapshot's `TargetRef` equals the document's ref; the CLI, docs site, and MCP do not supply it in this wave. The parity test therefore pins the no-readiness case for all four, and a separate board test pins the rendered Readiness section. Cost if wrong: a later wave adds a readiness request to the CLI and re-pins.
- **R-W2-4 (proposed is derived).** In working-tree mode the loader sets `Stamp.Proposed` from `specstate`: true unless the bytes are the exact accepted bytes on the default branch. The CLI's `--proposed` selects working-tree mode; its header therefore reflects the store's truth rather than the flag. Wave 1's `--proposed` test still passes (a design-branch edit is never exact). Cost if wrong: one flag semantics note.
- **R-W2-5 (board bytes).** The board renders the working tree of the checkout it serves (root mount: the serving checkout; `/b/{branch}` mount: that branch's managed worktree), stamped with that checkout's HEAD. On the sealed wall those bytes are the accepted bytes, so the parity test's board leg reads the root mount on a main checkout. Cost if wrong: parity must be re-scoped to the accepted mode only.
- **R-W2-6 (docs-site bytes).** The docs site renders at its build commit (`stamp.SHA`) via git, not the working tree, so a site is reproducible from a commit. Cost if wrong: dex reads the tree like its other pages and the parity leg re-pins.

---

## File structure

| File | Responsibility |
|---|---|
| `internal/specdoc/readiness.go` | `ReadinessFacts`, `ReadinessArea`, `ReadinessConcern`; `WithReadiness(f Facts, snap readinesspilot.Snapshot, ref string) Facts`. |
| `internal/specdoc/facts.go` | `Facts` gains `Readiness *ReadinessFacts`. |
| `internal/specdoc/kind.go`, `stamp.go`, `model.go`, `build.go`, `markdown.go` | `SectionReadiness`; `engineVersion = 2`; `Document.Readiness`/`ReadinessKnown`; the Readiness section render. |
| `internal/specdocload/load.go` | `Mode`, `Request`, `Result`, `Load(ctx, req) (Result, error)` — the one assembler. |
| `internal/specdocload/load_test.go` | fixturegit tests for the three modes, proposed derivation, disclosures, readiness gating. |
| `cmd/verdi/specdoc.go` | Thin consumer of the loader (no behavior change to the verb's contract). |
| `internal/mcpserve/tool_get_document.go`, `tooldefs.go`, `server.go` | `get_document`. |
| `internal/mcpserve/tool_get_document_test.go` | Backend tests. |
| `internal/specalign/mcptools_test.go`, `internal/mcpserve/server_test.go`, `cmd/verdi/serve_integration_test.go` | Inventory pins 18 → 19. |
| `internal/dex/document.go` | Document view page and the three Markdown siblings per spec. |
| `internal/dex/build.go`, `layout.go`, `artifactpage.go` | Wiring and the "Document" link. |
| `internal/dex/document_test.go` | Build tests. |
| `internal/workbench/boarddocument.go` | `/document` and `/document/snapshot` handlers, `?format=md` download, revision token. |
| `internal/workbench/boarddocumentrender.go` | The Document page template and tab strip; the board page gains a Document link. |
| `internal/workbench/assets/specdocument.js` | Polling, refresh, copy. |
| `internal/workbench/boarddocument_test.go` | Handler tests. |
| `e2e/tests/79-board-document.spec.ts`, `e2e/tests/80-dex-document.spec.ts` | Browser paths. |
| `cmd/verdi/document_parity_e2e_test.go` | ac-6 four-way parity. |

---

### Task 1: Readiness section in `specdoc` (engine version 2)

**Files:**
- Create: `internal/specdoc/readiness.go`
- Modify: `internal/specdoc/facts.go` (add `Readiness *ReadinessFacts` to `Facts`), `internal/specdoc/kind.go` (`SectionReadiness`; spec and tasks kinds), `internal/specdoc/stamp.go` (`engineVersion = 2`), `internal/specdoc/model.go` (`Readiness`, `ReadinessKnown`), `internal/specdoc/build.go` (copy readiness), `internal/specdoc/markdown.go` (render the section)
- Test: `internal/specdoc/readiness_test.go`; regenerate every golden under `internal/specdoc/testdata/`

**Interfaces:**
- Consumes: `readinesspilot.Snapshot{TargetRef, TargetTitle, TargetClass, Branch, Head, RequestDigest string; Areas []Area; CurrentFocus AreaID; Attention []Concern; AllConcerns []Concern; StaleNotice string}` (`internal/readinesspilot/schema.go:71-83`); `Area{ID AreaID; Label string; State State}` (`:63`); `Concern{ID string; Area AreaID; State State; Blocking bool; Timing Timing; WorkClass journey.BlockerClass; Summary string; Witnesses []string; Destination Destination}` (`:49-59`).
- Produces: `type ReadinessFacts struct{ TargetRef, Head, CurrentFocus, StaleNotice string; Areas []ReadinessArea; Attention []ReadinessConcern }`; `type ReadinessArea struct{ ID, Label, State string }`; `type ReadinessConcern struct{ ID, Area, State, Summary string; Blocking bool; Timing string; Witnesses []string }`; `func WithReadiness(f Facts, snap readinesspilot.Snapshot, ref string) Facts`; `SectionReadiness SectionID = "readiness"`; `Document.Readiness *ReadinessFacts`, `Document.ReadinessKnown bool`.

- [ ] **Step 1: Write the failing test**

```go
package specdoc

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/readinesspilot"
)

func readinessFixture() readinesspilot.Snapshot {
	return readinesspilot.Snapshot{
		TargetRef: "spec/lockbox", TargetTitle: "Lockbox", TargetClass: "feature", Branch: "main",
		Head: strings.Repeat("a", 40), RequestDigest: "sha256:" + strings.Repeat("0", 64),
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateUnproven},
			{ID: readinesspilot.AreaSuccess, Label: "Define success", State: readinesspilot.StateProven},
		},
		CurrentFocus: readinesspilot.AreaShape,
		Attention: []readinesspilot.Concern{
			{ID: "shape/question/oq-2", Area: readinesspilot.AreaShape, State: readinesspilot.StateUnproven, Blocking: true, Timing: readinesspilot.TimingCurrent, Summary: "Declared open question remains unresolved", Witnesses: []string{"oq-2"}},
			{ID: "shape/question/oq-1", Area: readinesspilot.AreaShape, State: readinesspilot.StateUnproven, Blocking: false, Timing: readinesspilot.TimingEventual, Summary: "Declared open question is claimed by spike stubs and remains unresolved", Witnesses: []string{"oq-1", "audit-probe"}},
		},
		StaleNotice: "Startup snapshot; restart verdi serve after an edit.",
	}
}

func TestWithReadinessGatesOnTargetRef(t *testing.T) {
	snap := readinessFixture()
	got := WithReadiness(Facts{}, snap, "spec/lockbox")
	if got.Readiness == nil {
		t.Fatal("matching ref must supply readiness")
	}
	if got.Readiness.TargetRef != "spec/lockbox" || got.Readiness.CurrentFocus != "shape-proposal" || len(got.Readiness.Areas) != 2 || len(got.Readiness.Attention) != 2 {
		t.Fatalf("readiness facts = %+v", got.Readiness)
	}
	if got.Readiness.Attention[0].ID != "shape/question/oq-2" || !got.Readiness.Attention[0].Blocking || got.Readiness.Attention[1].Timing != "eventual" {
		t.Fatalf("attention order/fields = %+v", got.Readiness.Attention)
	}
	other := WithReadiness(Facts{}, snap, "spec/other")
	if other.Readiness != nil {
		t.Fatal("a snapshot for another spec must not be supplied")
	}
}

func TestWithReadinessDoesNotTouchOtherFacts(t *testing.T) {
	base := Facts{Coverage: map[string][]string{"ac-1": {"x"}}}
	got := WithReadiness(base, readinessFixture(), "spec/lockbox")
	if len(got.Coverage) != 1 || got.Coverage["ac-1"][0] != "x" {
		t.Fatal("WithReadiness must copy the other facts unchanged")
	}
}

func TestKindSectionsIncludeReadiness(t *testing.T) {
	spec := KindSpec.Sections()
	if spec[len(spec)-1] != SectionReadiness || spec[len(spec)-2] != SectionEvidence {
		t.Fatalf("spec sections = %v", spec)
	}
	tasks := KindTasks.Sections()
	if tasks[len(tasks)-1] != SectionReadiness {
		t.Fatalf("tasks sections = %v", tasks)
	}
	for _, s := range KindPlan.Sections() {
		if s == SectionReadiness {
			t.Fatal("plan kind must not carry readiness")
		}
	}
}

func TestBuildAndRenderReadiness(t *testing.T) {
	fm, body := loadFixture(t)
	commit := strings.Repeat("0", 39) + "1"
	with := WithReadiness(FactsFromSpec(fm), readinessFixture(), "spec/lockbox")
	doc, err := Build(Input{Spec: fm, Body: body, Status: "accepted-pending-build", Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: with, Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	if !doc.ReadinessKnown || doc.Readiness == nil {
		t.Fatal("Build must carry supplied readiness")
	}
	md := RenderMarkdown(doc)
	for _, want := range []string{
		"## Readiness",
		"Source: readiness snapshot for `spec/lockbox` at `aaaaaaaaaaaa`. Current focus: Define the work.",
		"| Define the work | unproven |",
		"| Define success | proven |",
		"1. Declared open question remains unresolved — Define the work; blocking; current; unproven; witnesses: oq-2",
		"2. Declared open question is claimed by spike stubs and remains unresolved — Define the work; advisory; eventual; unproven; witnesses: oq-1, audit-probe",
		"Startup snapshot; restart verdi serve after an edit.",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	without, _ := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: FactsFromSpec(fm), Kind: KindSpec})
	if !strings.Contains(RenderMarkdown(without), "## Readiness\n\nReadiness was not supplied for this render.") {
		t.Errorf("unsupplied readiness must be stated:\n%s", RenderMarkdown(without))
	}
}

func TestEngineDigestChangedForVersion2(t *testing.T) {
	if engineVersion != 2 {
		t.Fatalf("engineVersion = %d, want 2 (readiness section added)", engineVersion)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/specdoc/ -run 'TestWithReadiness|TestKindSectionsIncludeReadiness|TestBuildAndRenderReadiness|TestEngineDigestChangedForVersion2'`
Expected: FAIL with `undefined: WithReadiness` (build failure).

- [ ] **Step 3: Write the readiness facts**

```go
package specdoc

import "github.com/jyang234/verdi/internal/readinesspilot"

// ReadinessFacts is the readiness snapshot as the document reports it:
// the areas in their fixed order, the current focus, and the attention
// queue in the snapshot's own order. Supplied only when the snapshot
// targets this document's spec (spec/spec-documents ac-1: "the readiness
// snapshot when present").
type ReadinessFacts struct {
	TargetRef    string
	Head         string
	CurrentFocus string
	StaleNotice  string
	Areas        []ReadinessArea
	Attention    []ReadinessConcern
}

// ReadinessArea is one of the four ordered areas with its state.
type ReadinessArea struct {
	ID    string
	Label string
	State string
}

// ReadinessConcern is one attention-queue entry.
type ReadinessConcern struct {
	ID        string
	Area      string
	State     string
	Summary   string
	Blocking  bool
	Timing    string
	Witnesses []string
}

// WithReadiness copies f and supplies readiness when snap targets ref.
// A snapshot for another spec leaves Readiness nil, which renders as
// "not supplied" — never as another spec's facts.
func WithReadiness(f Facts, snap readinesspilot.Snapshot, ref string) Facts {
	out := f
	if snap.TargetRef != ref {
		return out
	}
	rf := &ReadinessFacts{
		TargetRef:    snap.TargetRef,
		Head:         snap.Head,
		CurrentFocus: string(snap.CurrentFocus),
		StaleNotice:  snap.StaleNotice,
	}
	for _, a := range snap.Areas {
		rf.Areas = append(rf.Areas, ReadinessArea{ID: string(a.ID), Label: a.Label, State: string(a.State)})
	}
	for _, c := range snap.Attention {
		rf.Attention = append(rf.Attention, ReadinessConcern{
			ID:        c.ID,
			Area:      string(c.Area),
			State:     string(c.State),
			Summary:   c.Summary,
			Blocking:  c.Blocking,
			Timing:    string(c.Timing),
			Witnesses: append([]string(nil), c.Witnesses...),
		})
	}
	out.Readiness = rf
	return out
}
```

Then, in `facts.go`, add the field to `Facts` after `EvidenceSource`:

```go
	// Readiness is the snapshot's facts when a caller supplied one for
	// this spec; nil means "not supplied for this render".
	Readiness *ReadinessFacts
```

In `kind.go`: add `SectionReadiness SectionID = "readiness"` to the const block, append `SectionReadiness` after `SectionEvidence` in the `KindSpec` (default) and `KindTasks` cases of `Sections()`; leave `KindPlan` unchanged.

In `stamp.go`: `engineVersion = 2`.

In `model.go`, add to `Document` after `EvidenceSource`:

```go
	Readiness      *ReadinessFacts
	ReadinessKnown bool
```

In `build.go`, after the evidence copy block:

```go
	if in.Facts.Readiness != nil {
		doc.ReadinessKnown = true
		rf := *in.Facts.Readiness
		rf.Areas = append([]ReadinessArea(nil), in.Facts.Readiness.Areas...)
		rf.Attention = append([]ReadinessConcern(nil), in.Facts.Readiness.Attention...)
		doc.Readiness = &rf
	}
```

In `markdown.go`, add a case to the section switch, after `SectionEvidence`:

```go
		case SectionReadiness:
			w("## Readiness\n\n")
			if !doc.ReadinessKnown || doc.Readiness == nil {
				w("Readiness was not supplied for this render.\n\n")
				break
			}
			rf := doc.Readiness
			w("Source: readiness snapshot for `%s` at `%s`. Current focus: %s.\n\n", rf.TargetRef, shortCommit(rf.Head), areaLabel(rf, rf.CurrentFocus))
			w("| Area | State |\n|---|---|\n")
			for _, a := range rf.Areas {
				w("| %s | %s |\n", escapeCell(a.Label), escapeCell(a.State))
			}
			w("\n")
			if len(rf.Attention) == 0 {
				w("Nothing needs attention.\n\n")
			} else {
				w("Attention:\n\n")
				for i, c := range rf.Attention {
					posture := "advisory"
					if c.Blocking {
						posture = "blocking"
					}
					w("%d. %s — %s; %s; %s; %s; witnesses: %s <a id=\"%s\"></a>\n", i+1, c.Summary, areaLabel(rf, c.Area), posture, c.Timing, c.State, joinOr(c.Witnesses, "none"), c.ID)
				}
				w("\n")
			}
			if rf.StaleNotice != "" {
				w("%s\n\n", rf.StaleNotice)
			}
```

and the two helpers at the bottom of `markdown.go`:

```go
// shortCommit is the 12-hex prefix every stamp line uses; a shorter
// value is printed whole.
func shortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

// areaLabel resolves an area id to its plain label, falling back to the
// id so an unknown id is still visible rather than blank.
func areaLabel(rf *ReadinessFacts, id string) string {
	for _, a := range rf.Areas {
		if a.ID == id {
			return a.Label
		}
	}
	return id
}
```

`readinesspilot.State` values (`proven`, `violated-with-witness`, `unproven`) and `Timing` values (`current`, `eventual`) are rendered as the snapshot supplies them; they are not lifecycle-state vocabulary words, so no marker is needed. If the vocabulary witness objects to `"blocking"`/`"advisory"`, they are not in its word list either; if it objects to anything else, route or mark at the producing site.

- [ ] **Step 4: Regenerate goldens and run**

Run: `go test ./internal/specdoc/ -update && go test -race -count=1 ./internal/specdoc/... && go test -count=1 ./internal/specalign/ -run TestVocabProseWitness`
Expected: PASS. Read `testdata/golden-spec.md` and `golden-tasks.md`: each now ends with a Readiness section stating it was not supplied, before the footer; `golden-plan.md` has no Readiness section; every footer's engine digest changed (version 2). Also run `go test -race -count=1 ./cmd/verdi/ -run TestSpecDoc` — the CLI tests assert substrings, not digests, so they still pass.

- [ ] **Step 5: Commit**

```bash
git add internal/specdoc/
git commit -m "Add the readiness section to specdoc documents (engine version 2)"
```

---

### Task 2: `internal/specdocload` — the one assembler, and the CLI as its consumer

**Files:**
- Create: `internal/specdocload/load.go`, `internal/specdocload/doc.go`
- Test: `internal/specdocload/load_test.go`
- Modify: `cmd/verdi/specdoc.go` (replace `loadSpecDocSource` and the status/facts block with a `specdocload.Load` call; delete `specDocSource`/`loadSpecDocSource`)

**Interfaces:**
- Consumes: `readSpecBytesEitherZone` logic (re-implemented in the package: `store.ActiveSpecPath`, `store.ArchiveSpecPath`, `store.ActiveSpecRelPath`, `store.SpecRelPath(store.ZoneArchive, name)`); `specstate.ResolveDefaultBranch(ctx, root) (specstate.Branch, bool)`; `specstate.NewProjector().Resolve(ctx, root, specstate.Candidate{Path, Content}) (specstate.Result, error)` with `Result.State`, `Result.Relation`, `Result.ArtifactStatus()`, `specstate.RelationExact`; `gitx.RevParse`, `gitx.Show`; `matrixprojection.Project(ctx, root, ref, preview bool, mdl) (Projection, error)`; `specdoc.FactsFromSpec`, `specdoc.WithMatrix`, `specdoc.WithReadiness`.
- Produces:

```go
type Mode int
const (
	ModeAccepted    Mode = iota // the default branch's bytes; commit = that branch's head
	ModeAt                      // a pinned commit's bytes
	ModeWorkingTree             // the checkout's bytes; commit = HEAD; Proposed derived from specstate
)
type Request struct {
	Root      string
	Name      string // bare spec name
	Mode      Mode
	At        string // ModeAt only
	Kind      specdoc.Kind
	Model     *model.Model
	Readiness *readinesspilot.Snapshot // optional; supplied only when it targets spec/<Name>
}
type Result struct {
	Input       specdoc.Input
	RelPath     string   // store-relative path the bytes came from
	Head        string   // the checkout's HEAD used for evidence
	Disclosures []string // status/evidence degradations, in order
}
func Load(ctx context.Context, req Request) (Result, error)
```

- [ ] **Step 1: Write the failing test**

```go
package specdocload

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/specdoc"
)

const manifestYAML = "schema: verdi.config/v1\nforge: none\n"

const lockboxSpec = `---
id: spec/lockbox
kind: spec
title: "Lockbox"
owners: [platform-team]
class: feature
problem: { text: "Keys are shared.", anchor: problem }
outcome: { text: "Each key has one holder.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A key opens one box.", evidence: [behavioral, attestation], anchor: ac-1 }
stubs:
  - { slug: key-holder, acceptance_criteria: [ac-1] }
---
# Lockbox

## Problem

Keys are shared today.

## Outcome

One holder.

## ac-1

Proven by opening.
`

func buildRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return fixturegit.Build(t, []fixturegit.Layer{{
		Message: "adopt store with one accepted spec",
		Files: map[string]string{
			".verdi/verdi.yaml":                   manifestYAML,
			".verdi/specs/active/lockbox/spec.md": lockboxSpec,
		},
	}})
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid", "GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestLoadAcceptedMode(t *testing.T) {
	repo := buildRepo(t)
	res, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAccepted, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	in := res.Input
	if in.Spec == nil || in.Spec.Title != "Lockbox" || in.Stamp.Ref != "spec/lockbox" || in.Stamp.Commit != repo.Head || in.Stamp.Proposed {
		t.Fatalf("input = %+v", in.Stamp)
	}
	if in.Status != "accepted-pending-build" {
		t.Fatalf("status = %q", in.Status)
	}
	if in.Facts.Coverage == nil || in.Facts.Coverage["ac-1"][0] != "key-holder" {
		t.Fatalf("coverage = %v", in.Facts.Coverage)
	}
	if in.Facts.Evidence == nil || !strings.HasPrefix(in.Facts.EvidenceSource, "matrix over the working tree at "+repo.Head[:12]) {
		t.Fatalf("evidence = %v / %q (disclosures %v)", in.Facts.Evidence, in.Facts.EvidenceSource, res.Disclosures)
	}
	if in.Facts.Readiness != nil {
		t.Fatal("no readiness was supplied")
	}
	if res.RelPath != ".verdi/specs/active/lockbox/spec.md" || res.Head != repo.Head {
		t.Fatalf("relpath/head = %q / %q", res.RelPath, res.Head)
	}
}

func TestLoadAtAndWorkingTreeModes(t *testing.T) {
	repo := buildRepo(t)
	git(t, repo.Dir, "checkout", "-q", "-b", "design/lockbox-edit")
	edited := strings.Replace(lockboxSpec, "Keys are shared.", "Keys are shared widely.", 1)
	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi/specs/active/lockbox/spec.md"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo.Dir, "commit", "-qam", "edit")
	head := git(t, repo.Dir, "rev-parse", "HEAD")

	at, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAt, At: head, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	if at.Input.Spec.Problem.Text != "Keys are shared widely." || at.Input.Stamp.Commit != head || at.Input.Stamp.Proposed {
		t.Fatalf("at: %+v", at.Input.Stamp)
	}

	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi/specs/active/lockbox/spec.md"), []byte(strings.Replace(edited, "widely", "UNCOMMITTED", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	wt, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(wt.Input.Spec.Problem.Text, "UNCOMMITTED") || wt.Input.Stamp.Commit != head || !wt.Input.Stamp.Proposed {
		t.Fatalf("working tree: %+v / %q", wt.Input.Stamp, wt.Input.Spec.Problem.Text)
	}

	accepted, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAccepted, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Input.Spec.Problem.Text != "Keys are shared." || accepted.Input.Stamp.Commit != repo.Head {
		t.Fatalf("accepted must read main: %+v", accepted.Input.Stamp)
	}
}

func TestLoadWorkingTreeOnMainIsNotProposed(t *testing.T) {
	repo := buildRepo(t)
	res, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	if res.Input.Stamp.Proposed {
		t.Fatal("the exact accepted bytes on the default branch are not proposed")
	}
}

func TestLoadReadinessGating(t *testing.T) {
	repo := buildRepo(t)
	snap := readinesspilot.Snapshot{TargetRef: "spec/lockbox", Head: repo.Head, Areas: []readinesspilot.Area{{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateProven}}, CurrentFocus: readinesspilot.AreaShape}
	res, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec, Readiness: &snap})
	if err != nil {
		t.Fatal(err)
	}
	if res.Input.Facts.Readiness == nil {
		t.Fatal("matching snapshot must be supplied")
	}
	snap.TargetRef = "spec/other"
	res, _ = Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec, Readiness: &snap})
	if res.Input.Facts.Readiness != nil {
		t.Fatal("a snapshot for another spec must not be supplied")
	}
}

func TestLoadRefusals(t *testing.T) {
	repo := buildRepo(t)
	cases := []struct {
		name string
		req  Request
		want string
	}{
		{"unknown spec", Request{Root: repo.Dir, Name: "nope", Mode: ModeAccepted, Kind: specdoc.KindSpec}, "spec/nope not found"},
		{"bad commit", Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAt, At: "deadbeef", Kind: specdoc.KindSpec}, "deadbeef"},
		{"bad kind", Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAccepted, Kind: "chapter"}, "unknown document kind"},
		{"empty name", Request{Root: repo.Dir, Mode: ModeAccepted, Kind: specdoc.KindSpec}, "spec name"},
		{"no store", Request{Root: t.TempDir(), Name: "lockbox", Mode: ModeAccepted, Kind: specdoc.KindSpec}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load(context.Background(), c.req)
			if err == nil {
				t.Fatal("want error")
			}
			if c.want != "" && !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err %q does not name %q", err, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/specdocload/`
Expected: FAIL (package does not exist / `undefined: Load`).

- [ ] **Step 3: Write the loader**

```go
// Package specdocload assembles the one specdoc.Input every consumer
// renders from: the CLI verb, the board's Document tab, the docs site, and
// the MCP get_document tool. Sharing the assembly is what makes the four
// outputs byte-identical (spec/spec-documents ac-6). It reads the store
// and git; it never writes.
package specdocload
```

```go
package specdocload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/matrixprojection"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// Mode selects which bytes a document is rendered from.
type Mode int

const (
	// ModeAccepted reads the default branch's bytes; the stamp commit is
	// that branch's head. This is the accepted reading.
	ModeAccepted Mode = iota
	// ModeAt reads a pinned commit's bytes.
	ModeAt
	// ModeWorkingTree reads the checkout's bytes, stamped with HEAD; the
	// stamp is marked proposed unless specstate proves the bytes are the
	// exact accepted bytes on the default branch (ruling R-W2-4).
	ModeWorkingTree
)

// Request names the spec, the mode, and the optional facts a caller owns.
type Request struct {
	Root      string
	Name      string
	Mode      Mode
	At        string
	Kind      specdoc.Kind
	Model     *model.Model
	Readiness *readinesspilot.Snapshot
}

// Result carries the assembled Input plus what a consumer may want to
// disclose or reuse.
type Result struct {
	Input       specdoc.Input
	RelPath     string
	Head        string
	Disclosures []string
}

// Load assembles the Input. Errors are operational (unreadable store,
// unknown spec, unresolvable commit or default branch, invalid kind);
// degraded facts (status not resolved, evidence not computed) are
// disclosed in Result.Disclosures and never fail the load.
func Load(ctx context.Context, req Request) (Result, error) {
	if req.Name == "" {
		return Result{}, errors.New("specdocload: spec name is required")
	}
	if _, err := artifact.ParseRef("spec/" + req.Name); err != nil {
		return Result{}, fmt.Errorf("specdocload: spec name %q: %w", req.Name, err)
	}
	if _, err := specdoc.ParseKind(string(req.Kind)); err != nil {
		return Result{}, err
	}
	if _, err := os.Stat(filepath.Join(req.Root, ".verdi", "verdi.yaml")); err != nil {
		return Result{}, fmt.Errorf("specdocload: %s is not a verdi store root: %w", req.Root, err)
	}
	ref := "spec/" + req.Name

	src, err := loadSource(ctx, req)
	if err != nil {
		return Result{}, err
	}
	fm, err := artifact.DecodeSpec(src.content)
	if err != nil {
		return Result{}, fmt.Errorf("specdocload: %s at %s: %w", ref, src.commit, err)
	}
	_, body, err := artifact.SplitFrontmatter(src.content)
	if err != nil {
		return Result{}, fmt.Errorf("specdocload: %w", err)
	}

	var disclosures []string
	status := ""
	proposed := false
	if res, rerr := specstate.NewProjector().Resolve(ctx, req.Root, specstate.Candidate{Path: src.relPath, Content: src.content}); rerr == nil {
		status = string(res.ArtifactStatus())
		if req.Mode == ModeWorkingTree {
			proposed = res.Relation != specstate.RelationExact
		}
	} else {
		disclosures = append(disclosures, "status not resolved: "+rerr.Error())
		if req.Mode == ModeWorkingTree {
			proposed = true
		}
	}

	head, err := gitx.RevParse(ctx, req.Root, "HEAD")
	if err != nil {
		return Result{}, fmt.Errorf("specdocload: resolving HEAD: %w", err)
	}
	facts := specdoc.FactsFromSpec(fm)
	if proj, perr := matrixprojection.Project(ctx, req.Root, ref, req.Mode == ModeWorkingTree, req.Model); perr == nil {
		facts = specdoc.WithMatrix(facts, proj.Record, head)
	} else {
		disclosures = append(disclosures, "evidence not computed: "+perr.Error())
	}
	if req.Readiness != nil {
		facts = specdoc.WithReadiness(facts, *req.Readiness, ref)
	}

	return Result{
		Input: specdoc.Input{
			Spec:   fm,
			Body:   body,
			Status: status,
			Stamp:  specdoc.Stamp{Ref: ref, Commit: src.commit, Proposed: proposed},
			Facts:  facts,
			Model:  req.Model,
			Kind:   req.Kind,
		},
		RelPath:     src.relPath,
		Head:        head,
		Disclosures: disclosures,
	}, nil
}

type source struct {
	relPath string
	content []byte
	commit  string
}

// loadSource picks the bytes for the mode: active zone first, archive
// second, in every mode.
func loadSource(ctx context.Context, req Request) (source, error) {
	name := req.Name
	if req.Mode == ModeWorkingTree {
		for _, p := range []struct{ abs, rel string }{
			{store.ActiveSpecPath(req.Root, name), store.ActiveSpecRelPath(name)},
			{store.ArchiveSpecPath(req.Root, name), store.SpecRelPath(store.ZoneArchive, name)},
		} {
			data, rerr := os.ReadFile(p.abs)
			if rerr == nil {
				head, err := gitx.RevParse(ctx, req.Root, "HEAD")
				if err != nil {
					return source{}, fmt.Errorf("specdocload: resolving HEAD: %w", err)
				}
				return source{relPath: p.rel, content: data, commit: head}, nil
			}
			if !os.IsNotExist(rerr) {
				return source{}, fmt.Errorf("specdocload: reading %s: %w", p.abs, rerr)
			}
		}
		return source{}, fmt.Errorf("specdocload: spec/%s not found in either zone of the working tree", name)
	}
	rev := req.At
	if req.Mode == ModeAccepted {
		branch, ok := specstate.ResolveDefaultBranch(ctx, req.Root)
		if !ok {
			return source{}, errors.New("specdocload: the default branch could not be resolved; use a pinned commit or the working tree")
		}
		rev = branch.Ref
	}
	commit, err := gitx.RevParse(ctx, req.Root, rev)
	if err != nil {
		return source{}, fmt.Errorf("specdocload: resolving %q: %w", rev, err)
	}
	for _, rel := range []string{store.ActiveSpecRelPath(name), store.SpecRelPath(store.ZoneArchive, name)} {
		content, err := gitx.Show(ctx, req.Root, commit, rel)
		if err == nil {
			return source{relPath: rel, content: content, commit: commit}, nil
		}
	}
	return source{}, fmt.Errorf("specdocload: spec/%s not found at %s in either zone", name, commit)
}
```

The store-root check stats `<root>/.verdi/verdi.yaml` directly; `internal/store` has no helper for that path and none is added.

- [ ] **Step 4: Run the loader tests**

Run: `go test -race -count=1 ./internal/specdocload/`
Expected: PASS.

- [ ] **Step 5: Make the CLI a consumer**

In `cmd/verdi/specdoc.go`, delete `specDocSource` and `loadSpecDocSource`, and replace everything from the `store.Open` call through `specdoc.Build` with:

```go
	mode := specdocload.ModeAccepted
	switch {
	case *proposedFlag:
		mode = specdocload.ModeWorkingTree
	case *atFlag != "":
		mode = specdocload.ModeAt
	}
	res, err := specdocload.Load(ctx, specdocload.Request{Root: root, Name: parsed.Name, Mode: mode, At: *atFlag, Kind: kind, Model: cfg.Model})
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
	for _, d := range res.Disclosures {
		fmt.Fprintln(stderr, "spec doc:", d)
	}
	doc, err := specdoc.Build(res.Input)
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
```

keeping the ref/kind/format/`-o` validation, the store-path refusal, the render, and the output exactly as they are. Remove the now-unused imports (`gitx`, `matrixprojection`, `specstate`, `store` if only used by the deleted code). The verb's stderr messages for the unresolvable cases now start with `spec doc: specdocload: …`; if a test in `specdoc_test.go` asserted the older exact prefix for "unknown spec" or "bad commit", the substrings it checks (`spec/nope`, `deadbeef`) still hold.

- [ ] **Step 6: Run the CLI tests and the vocabulary witness**

Run: `go test -race -count=1 ./cmd/verdi/ -run 'TestSpecDoc|SpecState|Help|RunSpecVerb' && go test -count=1 ./internal/specalign/ -run TestVocabProseWitness && go build ./... && gofmt -l . && go vet ./internal/specdocload/ ./cmd/verdi/`
Expected: all PASS, gofmt prints nothing. `TestSpecDoc_ProposedAndAt` still passes: the design-branch edit is never the exact accepted bytes, so the header appears.

- [ ] **Step 7: Commit**

```bash
git add internal/specdocload/ cmd/verdi/specdoc.go
git commit -m "Add specdocload, the shared document input assembler, and route spec doc through it"
```

---

### Task 3: MCP `get_document`

**Files:**
- Create: `internal/mcpserve/tool_get_document.go`, `internal/mcpserve/tool_get_document_test.go`
- Modify: `internal/mcpserve/tooldefs.go` (tool def), `internal/mcpserve/server.go` (dispatch), `internal/specalign/mcptools_test.go` (`TestMCPToolInventory` want list), `internal/mcpserve/server_test.go` (`len(tools) != 18` → 19), `cmd/verdi/serve_integration_test.go` (`want 18` → 19)

**Interfaces:**
- Consumes: `strictUnmarshal` (`internal/mcpserve/decode.go:34`), `toolJSON` (`toolresult.go:33`), `toolError` (`toolresult.go:18`), `dataNeverInstructionsNote`, schema helpers `obj`/`str`, `Backend.Root`; `specdocload.Load`; `store.Open(root)` for the model (nil on failure, like other tools).
- Produces: tool `get_document` with args `{ref: "spec/<name>" or "spec/<name>@<commit>", kind?: "spec"|"plan"|"tasks" (default "spec"), commit?: "<sha>"}` returning canonical JSON `{ref, kind, commit, engine, proposed, markdown, disclosures}`; `func (b *Backend) GetDocument(ctx context.Context, argsRaw json.RawMessage) map[string]any`.

- [ ] **Step 1: Write the failing test**

```go
package mcpserve

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestGetDocument_Happy(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	backend, repo, _ := newTestBackend(t)
	res := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/escrow-autopay"}))
	if isToolError(res) {
		t.Fatalf("tool error: %s", toolResultText(t, res))
	}
	var got struct {
		Ref, Kind, Commit, Engine, Markdown string
		Proposed                            bool
		Disclosures                         []string
	}
	if err := json.Unmarshal([]byte(toolResultText(t, res)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Ref != "spec/escrow-autopay" || got.Kind != "spec" || got.Commit != repo.Head || got.Proposed || !strings.HasPrefix(got.Engine, "sha256:") {
		t.Fatalf("got %+v", got)
	}
	if !strings.HasPrefix(got.Markdown, "# ") || !strings.Contains(got.Markdown, "not authority") || !strings.HasSuffix(got.Markdown, "\n") {
		t.Fatalf("markdown wrong shape:\n%s", got.Markdown)
	}
	again := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/escrow-autopay"}))
	if toolResultText(t, res) != toolResultText(t, again) {
		t.Fatal("two calls over unchanged state must be byte-identical")
	}
}

func TestGetDocument_KindsAndPin(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	backend, repo, _ := newTestBackend(t)
	plan := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/escrow-autopay", "kind": "plan"}))
	if isToolError(plan) || !strings.Contains(toolResultText(t, plan), "## Decisions") || strings.Contains(toolResultText(t, plan), "## Problem") {
		t.Fatalf("plan kind: %s", toolResultText(t, plan))
	}
	pinned := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/escrow-autopay@" + repo.Head}))
	if isToolError(pinned) || !strings.Contains(toolResultText(t, pinned), `"commit":"`+repo.Head+`"`) {
		t.Fatalf("pinned ref: %s", toolResultText(t, pinned))
	}
	viaArg := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/escrow-autopay", "commit": repo.Head}))
	if toolResultText(t, pinned) != toolResultText(t, viaArg) {
		t.Fatal("ref@commit and commit arg must agree")
	}
}

func TestGetDocument_Refusals(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	backend, repo, _ := newTestBackend(t)
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing ref", map[string]any{}, "ref"},
		{"not a spec", map[string]any{"ref": "adr/0001"}, "spec/<name>"},
		{"fragment", map[string]any{"ref": "spec/escrow-autopay#ac-1"}, "spec/<name>"},
		{"unknown field", map[string]any{"ref": "spec/escrow-autopay", "format": "html"}, "unknown field"},
		{"bad kind", map[string]any{"ref": "spec/escrow-autopay", "kind": "chapter"}, "unknown document kind"},
		{"pin and commit disagree", map[string]any{"ref": "spec/escrow-autopay@" + repo.Head, "commit": strings.Repeat("b", 40)}, "disagree"},
		{"unknown spec", map[string]any{"ref": "spec/nope"}, "spec/nope"},
		{"bad commit", map[string]any{"ref": "spec/escrow-autopay", "commit": "deadbeef"}, "deadbeef"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := backend.GetDocument(context.Background(), mustArgs(t, c.args))
			if !isToolError(res) {
				t.Fatalf("want tool error, got %s", toolResultText(t, res))
			}
			if !strings.Contains(toolResultText(t, res), c.want) {
				t.Fatalf("error %q does not name %q", toolResultText(t, res), c.want)
			}
		})
	}
}
```

If the fixture repo built by `newTestBackend` (`fixture_test.go:106`) has no spec named `escrow-autopay`, read `buildFixture` (`fixture_test.go:53`) and use the accepted feature spec it does carry; the assertions only need one accepted feature spec on `main`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/mcpserve/ -run TestGetDocument`
Expected: FAIL with `backend.GetDocument undefined`.

- [ ] **Step 3: Write the tool**

```go
package mcpserve

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
	"github.com/jyang234/verdi/internal/store"
)

type getDocumentArgs struct {
	Ref    string `json:"ref"`
	Kind   string `json:"kind"`
	Commit string `json:"commit"`
}

type getDocumentResult struct {
	Ref         string   `json:"ref"`
	Kind        string   `json:"kind"`
	Commit      string   `json:"commit"`
	Engine      string   `json:"engine"`
	Proposed    bool     `json:"proposed"`
	Markdown    string   `json:"markdown"`
	Disclosures []string `json:"disclosures"`
}

// GetDocument renders a spec as its Markdown document (spec/spec-documents
// ac-5). Read-only: it adds nothing to the write surface. The default
// reading is the accepted bytes on the default branch; a pinned ref or a
// commit argument reads that commit. The result is canonical JSON, so two
// calls over unchanged state are byte-identical.
func (b *Backend) GetDocument(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args getDocumentArgs
	if err := strictUnmarshal(argsRaw, &args); err != nil {
		return toolError("get_document: " + err.Error())
	}
	if args.Ref == "" {
		return toolError("get_document: ref is required")
	}
	ref, err := artifact.ParseRef(args.Ref)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Fragment() {
		return toolError(fmt.Sprintf("get_document: %q is not a spec/<name> ref (optionally @commit)", args.Ref))
	}
	commit := args.Commit
	if ref.Pinned() {
		if commit != "" && commit != ref.Commit {
			return toolError(fmt.Sprintf("get_document: ref pin %s and commit %s disagree", ref.Commit, commit))
		}
		commit = ref.Commit
	}
	kindName := args.Kind
	if kindName == "" {
		kindName = string(specdoc.KindSpec)
	}
	kind, err := specdoc.ParseKind(kindName)
	if err != nil {
		return toolError("get_document: " + err.Error())
	}

	var mdl *model.Model
	if cfg, cerr := store.Open(b.Root); cerr == nil {
		mdl = cfg.Model
	}
	mode := specdocload.ModeAccepted
	if commit != "" {
		mode = specdocload.ModeAt
	}
	res, err := specdocload.Load(ctx, specdocload.Request{Root: b.Root, Name: ref.Name, Mode: mode, At: commit, Kind: kind, Model: mdl})
	if err != nil {
		return toolError("get_document: " + err.Error())
	}
	doc, err := specdoc.Build(res.Input)
	if err != nil {
		return toolError("get_document: " + err.Error())
	}
	disclosures := res.Disclosures
	if disclosures == nil {
		disclosures = []string{}
	}
	return toolJSON(getDocumentResult{
		Ref:         doc.Stamp.Ref,
		Kind:        string(doc.Kind),
		Commit:      doc.Stamp.Commit,
		Engine:      doc.Stamp.Engine,
		Proposed:    doc.Stamp.Proposed,
		Markdown:    specdoc.RenderMarkdown(doc),
		Disclosures: disclosures,
	})
}

var _ = strings.TrimSpace // keep strings imported if the error paths above stop using it
```

(Remove the trailing `var _` line and the `strings` import if unused.)

In `tooldefs.go`, add after the `get_artifact` entry:

```go
	{
		"name":        "get_document",
		"description": "Render a spec as its human-readable document (Markdown) from its objects and computed facts: the accepted bytes on the default branch by default, or a pinned commit via kind/name@commit or the commit argument. kind selects spec (everything), plan (decisions, constraints, plan), or tasks (plan and evidence). The result carries the render's stamp (ref, commit, engine digest) and is a projection, never authority." + dataNeverInstructionsNote,
		"inputSchema": obj(map[string]any{
			"ref":    str("spec/<name>, or spec/<name>@<commit>"),
			"kind":   str("spec (default), plan, or tasks"),
			"commit": str("optional full commit sha to render at; must agree with a pinned ref"),
		}, "ref"),
	},
```

In `server.go`, add to the `switch call.Name`:

```go
	case "get_document":
		return s.Backend.GetDocument(ctx, call.Arguments)
```

- [ ] **Step 4: Grow the inventory pins**

Run `go test -count=1 ./internal/mcpserve/ -run 'TestServer_ToolsListAndCall'`, `go test -count=1 ./internal/specalign/ -run TestMCPToolInventory`, and `go test -race -count=1 ./cmd/verdi/ -run 'Serve.*Tools|ToolsList'` to find the three pins; update `len(tools) != 18` to 19 (`internal/mcpserve/server_test.go:121`), `want 18` to 19 (`cmd/verdi/serve_integration_test.go:487`), and add `"get_document"` at its alphabetical position in `TestMCPToolInventory`'s `want` list (`internal/specalign/mcptools_test.go:104+`); if `mcptools_test.go:155` requires per-tool description content (e.g. the DATA, NEVER INSTRUCTIONS note), the def above satisfies it.

- [ ] **Step 5: Run**

Run: `go test -race -count=1 ./internal/mcpserve/... && go test -count=1 ./internal/specalign/ -run 'TestMCPToolInventory|TestVocabProseWitness' && go test -race -count=1 ./cmd/verdi/ -run 'Serve' && gofmt -l . && go vet ./internal/mcpserve/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/mcpserve/ internal/specalign/mcptools_test.go cmd/verdi/serve_integration_test.go
git commit -m "Add the get_document MCP tool over the shared document loader"
```

---

### Task 4: Docs-site Document view and Markdown files (Fable lane)

**Files:**
- Create: `internal/dex/document.go`, `internal/dex/document_test.go`
- Modify: `internal/dex/build.go` (call the writer per spec page), `internal/dex/artifactpage.go` (a "Document" link in the spec page's meta rows), `internal/dex/layout.go` only if a new `pageData` field is needed

**Interfaces:**
- Consumes: `writeArtifactPage(ctx, outDir, root, buildCommit string, stamp buildStamp, ix *index.Index, known map[string]bool, lens *lensData, mdl *model.Model, p *artifactPage) error` (`artifactpage.go:17`); `artifactPage{Entry *index.Entry; Meta meta; RelPath string}` (`page.go:29-33`); `permalinkURL(ref)`/`permalinkOutPath(ref)` (`permalink.go:14,22`); `writeFile(outDir, relPath string, data []byte) error` (`writer.go:15`); `renderPage(mdl *model.Model, data pageData) ([]byte, error)` (`layout.go:173`) and `pageData{Title, BodyHTML, MetaRows, CopyRef, Banner, TOC, ...}`; `stamp.SHA` (full sha); `specdocload.Load` with `ModeAt`.
- Produces: for every artifact page whose `Entry.Ref` starts with `spec/`: files `a/spec/<name>/spec.md`, `a/spec/<name>/plan.md`, `a/spec/<name>/tasks.md`, and a page `a/spec/<name>/document/index.html`; a "Document" row in the spec page's meta rows linking to `document/`; `func writeSpecDocuments(ctx context.Context, outDir, root string, stamp buildStamp, mdl *model.Model, p *artifactPage) error`.

- [ ] **Step 1: Write the failing test**

```go
package dex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuild_WritesSpecDocuments(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildDexFixtureRepo(t) // the fixture TestBuild_Happy uses (internal/dex/build_test.go:5)
	out := t.TempDir()
	if err := Build(context.Background(), Options{Root: repo.Dir, OutDir: out}); err != nil {
		t.Fatal(err)
	}
	specDir := filepath.Join(out, "a", "spec", "escrow-autopay") // the fixture's accepted feature spec
	for _, name := range []string{"spec.md", "plan.md", "tasks.md"} {
		data, err := os.ReadFile(filepath.Join(specDir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		md := string(data)
		if !strings.HasPrefix(md, "# ") || !strings.Contains(md, "not authority") || !strings.Contains(md, "commit `"+repo.Head+"`") {
			t.Errorf("%s wrong shape:\n%s", name, md)
		}
	}
	page, err := os.ReadFile(filepath.Join(specDir, "document", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "<h2") || !strings.Contains(string(page), "not authority") || !strings.Contains(string(page), `href="spec.md"`) {
		t.Errorf("document page wrong shape")
	}
	specPage, _ := os.ReadFile(filepath.Join(specDir, "index.html"))
	if !strings.Contains(string(specPage), `href="document/"`) {
		t.Errorf("spec page must link to its document view")
	}
	// Non-spec pages get no documents.
	if _, err := os.Stat(filepath.Join(out, "a", "adr")); err == nil {
		entries, _ := filepath.Glob(filepath.Join(out, "a", "adr", "*", "spec.md"))
		if len(entries) != 0 {
			t.Errorf("adr pages must not carry spec documents: %v", entries)
		}
	}
}
```

The fixture is `buildDexFixtureRepo(t)`; its spec directories are: read buildDexFixtureRepo. If `escrow-autopay` is not a feature spec accepted on `main` in that fixture, pick the one that is and use it in the test above. The existing `TestBuild_ByteIdenticalRebuild` and `TestBuildV2_ByteIdenticalRebuild` walk the whole output tree, so the new files are covered for determinism automatically.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/dex/ -run TestBuild_WritesSpecDocuments`
Expected: FAIL (files not found).

- [ ] **Step 3: Write the writer**

```go
package dex

import (
	"context"
	"fmt"
	"html/template"
	"path"
	"strings"

	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
)

// writeSpecDocuments writes the three Markdown documents and the Document
// view beside a spec's page, rendered at the site's build commit through
// the shared loader so the bytes match the CLI, the board, and MCP
// (spec/spec-documents ac-4, ac-6). A page that is not a spec gets
// nothing.
func writeSpecDocuments(ctx context.Context, outDir, root string, stamp buildStamp, mdl *model.Model, p *artifactPage) error {
	if !strings.HasPrefix(p.Entry.Ref, "spec/") {
		return nil
	}
	name := strings.TrimPrefix(p.Entry.Ref, "spec/")
	base := path.Dir(permalinkOutPath(p.Entry.Ref)) // a/spec/<name>
	var specHTML string
	for _, kind := range []specdoc.Kind{specdoc.KindSpec, specdoc.KindPlan, specdoc.KindTasks} {
		res, err := specdocload.Load(ctx, specdocload.Request{Root: root, Name: name, Mode: specdocload.ModeAt, At: stamp.SHA, Kind: kind, Model: mdl})
		if err != nil {
			return fmt.Errorf("dex: document for %s: %w", p.Entry.Ref, err)
		}
		doc, err := specdoc.Build(res.Input)
		if err != nil {
			return fmt.Errorf("dex: document for %s: %w", p.Entry.Ref, err)
		}
		md := specdoc.RenderMarkdown(doc)
		if err := writeFile(outDir, path.Join(base, string(kind)+".md"), []byte(md)); err != nil {
			return err
		}
		if kind == specdoc.KindSpec {
			specHTML, err = specdoc.RenderHTML(doc)
			if err != nil {
				return fmt.Errorf("dex: document html for %s: %w", p.Entry.Ref, err)
			}
		}
	}
	body := documentViewChrome(p.Entry.Ref) + specHTML
	page, err := renderPage(mdl, pageData{
		Title:    p.Entry.Title + " — document",
		BodyHTML: template.HTML(body),
		// copy-ref: fill the same field writeArtifactPage fills (artifactpage.go), value p.Entry.Ref + "@" + stamp.SHA
	})
	if err != nil {
		return err
	}
	return writeFile(outDir, path.Join(base, "document", "index.html"), page)
}

// documentViewChrome is the small header above the rendered document:
// the way back to the spec page and the three Markdown files.
func documentViewChrome(ref string) string {
	return `<nav class="document-files" aria-label="Document files">` +
		`<a href="../">artifact page</a> · ` +
		`<a href="../spec.md" download>spec.md</a> · ` +
		`<a href="../plan.md" download>plan.md</a> · ` +
		`<a href="../tasks.md" download>tasks.md</a>` +
		`</nav>` +
		// vocab:identity — the sentence names the projection's provenance, not a lifecycle state label
		`<p class="document-note">A reading of <code>` + template.HTMLEscapeString(ref) + `</code> rendered from its objects at the site's build commit; not authority.</p>`
}
```

`pageData` (`internal/dex/layout.go:34`) has `Title`, `Status`, `StatusLabel`, `LadderBadges`, `Breadcrumb`, `Banner`, `BannerClass`, `MetaRows`, `BodyHTML`, `Connections`, `TOC`, `CopyRefDisplay`, `DispositionsHTML`, `FeatureLensHTML`, `OpenAPIJSONPath`, `HasMermaid`, `NavByStory`; set `Title`, `BodyHTML`, and the copy-ref field the spec page sets (read how `writeArtifactPage` fills it, `artifactpage.go:17+`), leave the rest zero. Relative hrefs inside `a/spec/<name>/document/index.html`: `../spec.md` resolves to `a/spec/<name>/spec.md`. In the test above the document page assertion is `href="spec.md"`; change the test to `href="../spec.md"` to match this chrome, or emit the links relative to the spec dir — pick the version that resolves in a browser and assert that.

In `build.go`, right after each `writeArtifactPage` call (`build.go:105-109`):

```go
		if err := writeSpecDocuments(ctx, opts.OutDir, opts.Root, stamp, mdl, p); err != nil {
			return err
		}
```

In `artifactpage.go`, in `artifactMetaRows` (`:83`), for spec pages append a row whose value is a link: label `Document`, value `<a href="document/">read as a document</a>` (follow the row type the template uses; if rows are plain strings, add the link to the page's action/nav area next to the copy-ref control instead, and assert on that in the test).

- [ ] **Step 4: Run**

Run: `go test -race -count=1 ./internal/dex/ && go test -count=1 ./internal/specalign/ -run TestVocabProseWitness && gofmt -l . && go vet ./internal/dex/`
Expected: PASS, including both byte-identical-rebuild tests.

- [ ] **Step 5: Playwright for the docs site**

Create `e2e/tests/80-dex-document.spec.ts` following `e2e/tests/05-dex.spec.ts`'s navigation (`dexSpecPath(name)` from fixtures.ts:782, `DEX_BASE`):

```ts
import { test, expect } from "@playwright/test";
import { SHOWCASE, dexSpecPath } from "./fixtures";

test.describe("dex document view", () => {
  test("a spec page links to its document, which renders and offers the three files", async ({ page }) => {
    await page.goto(dexSpecPath(SHOWCASE.FEATURE_SPEC));
    const link = page.getByRole("link", { name: /read as a document/i });
    await expect(link).toBeVisible();
    await link.click();
    await expect(page).toHaveURL(new RegExp(`/a/spec/${SHOWCASE.FEATURE_SPEC}/document/`));
    await expect(page.getByRole("heading", { level: 2, name: "Identity" })).toBeVisible();
    await expect(page.getByText("not authority")).toBeVisible();
    for (const file of ["spec.md", "plan.md", "tasks.md"]) {
      const res = await page.request.get(`${dexSpecPath(SHOWCASE.FEATURE_SPEC)}${file}`);
      expect(res.status()).toBe(200);
      const body = await res.text();
      expect(body.startsWith("# ")).toBe(true);
      expect(body).toContain("not authority");
    }
  });
});
```

Run: `cd e2e && VERDI_E2E_PORT_BASE=4690 npx playwright test tests/80-dex-document.spec.ts`; then `git status --porcelain` must show only intended files and no recording artifacts.

- [ ] **Step 6: Commit**

```bash
git add internal/dex/ e2e/tests/80-dex-document.spec.ts
git commit -m "Add the docs-site Document view and spec/plan/tasks Markdown files per spec"
```

---

### Task 5: Board Document tab (Fable lane)

**Files:**
- Create: `internal/workbench/boarddocument.go`, `internal/workbench/boarddocumentrender.go`, `internal/workbench/assets/specdocument.js`, `internal/workbench/boarddocument_test.go`
- Modify: `internal/workbench/handler.go` (two rows in `boardSpecRoutes()`, asset registration), `internal/workbench/boardspec.go` (`boardSpecServer` gains `readiness *readinesspilot.Snapshot`, set from `Deps.Readiness` where the server is constructed at `handler.go:145`), `internal/workbench/boardspecrender.go:195` (nav gains a Document link)
- Test: `e2e/tests/79-board-document.spec.ts`

**Interfaces:**
- Consumes: `boardSpecRoutes()` rows (`handler.go:33-41`: `{suffix, handler, json}`), `RegisterRoutesWithHome` mounts (root `:141-143`, branch `:152-155`); `boardSpecServer{root, model, ...}`; `writeJSON`; the ETag/`If-None-Match` idiom at `boardspec.go:685-695`; `boardspecasd.js` polling idiom (`refresh(force)`, 2 s interval, `document.hidden`, `visibilitychange`); `specdocload.Load` with `ModeWorkingTree`; `specdoc.Build`, `RenderMarkdown`, `RenderHTML`; `canonjson.Digest`.
- Produces: routes `/board/spec/{name}/document` (page; `?kind=spec|plan|tasks`, `?format=md` → `text/markdown; charset=utf-8` with `Content-Disposition: attachment; filename="<name>-<kind>.md"`) and `/board/spec/{name}/document/snapshot` (JSON `{revision, html, markdown}` with `ETag` and `304`), both mounted under `/b/{branch}` by the route table; `func (s *boardSpecServer) boardDocumentPageHandler(w, r)`, `boardDocumentSnapshotHandler(w, r)`, `func (s *boardSpecServer) loadDocument(ctx, name string, kind specdoc.Kind) (documentSnapshot, error)`; asset `/assets/specdocument.js`.

- [ ] **Step 1: Write the failing handler test**

Read `internal/workbench/reviseaction_test.go` for how a `boardSpecServer`/handler is built over a fixturegit store in this package (`newClaimWallFixture`/`buildAuthoringFixture` in `testfixture_test.go:95` are the fixtures; `getBoard`/`postBoardAPI` the request helpers). Then:

```go
package workbench

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/readinesspilot"
)

func TestBoardDocument_PageSnapshotAndDownload(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	h, repo, name := newAcceptedWallFixture(t) // an http.Handler over a store whose <name> is an accepted feature spec on main; build it from testfixture_test.go's helpers
	page := getStatus(t, h, "/board/spec/"+name+"/document")
	if page.code != http.StatusOK || !strings.Contains(page.body, `id="document-region"`) || !strings.Contains(page.body, "not authority") || !strings.Contains(page.body, `data-testid="document-copy"`) || !strings.Contains(page.body, `data-testid="document-download"`) {
		t.Fatalf("page: %d\n%s", page.code, page.body)
	}
	if strings.Contains(page.body, "Proposed, not accepted") {
		t.Fatalf("the sealed wall's document is the accepted reading")
	}
	snap := getStatus(t, h, "/board/spec/"+name+"/document/snapshot")
	if snap.code != http.StatusOK || snap.etag == "" || !strings.Contains(snap.body, `"revision":"`) || !strings.Contains(snap.body, `"markdown":"# `) {
		t.Fatalf("snapshot: %d etag %q\n%s", snap.code, snap.etag, snap.body)
	}
	req := httptest.NewRequest(http.MethodGet, "/board/spec/"+name+"/document/snapshot", nil)
	req.Header.Set("If-None-Match", snap.etag)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("unchanged token must 304, got %d", rec.Code)
	}
	dl := getStatus(t, h, "/board/spec/"+name+"/document?format=md&kind=tasks")
	if dl.code != http.StatusOK || !strings.HasPrefix(dl.contentType, "text/markdown") || !strings.Contains(dl.disposition, `attachment; filename="`+name+`-tasks.md"`) || !strings.HasPrefix(dl.body, "# ") || strings.Contains(dl.body, "## Problem") {
		t.Fatalf("download: %d %q %q\n%s", dl.code, dl.contentType, dl.disposition, dl.body)
	}
	_ = repo
}

func TestBoardDocument_ReadinessGatedByTarget(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	snap := readinesspilot.Snapshot{TargetRef: "spec/other", Areas: []readinesspilot.Area{{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateProven}}, CurrentFocus: readinesspilot.AreaShape}
	h, _, name := newAcceptedWallFixtureWithReadiness(t, &snap)
	page := getStatus(t, h, "/board/spec/"+name+"/document")
	if !strings.Contains(page.body, "Readiness was not supplied for this render.") {
		t.Fatal("a snapshot for another spec must not render here")
	}
	snap.TargetRef = "spec/" + name
	h, _, name = newAcceptedWallFixtureWithReadiness(t, &snap)
	page = getStatus(t, h, "/board/spec/"+name+"/document")
	if !strings.Contains(page.body, "readiness snapshot for") || !strings.Contains(page.body, "Define the work") {
		t.Fatalf("matching snapshot must render:\n%s", page.body)
	}
}

func TestBoardDocument_Refusals(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	h, _, name := newAcceptedWallFixture(t)
	for _, c := range []struct{ path, want string }{
		{"/board/spec/" + name + "/document?kind=chapter", "unknown document kind"},
		{"/board/spec/" + name + "/document?format=pdf", "format"},
		{"/board/spec/nope/document", "not found"},
	} {
		res := getStatus(t, h, c.path)
		if res.code < 400 || res.code >= 500 || !strings.Contains(res.body, c.want) {
			t.Errorf("%s: %d %q", c.path, res.code, res.body)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/board/spec/"+name+"/document", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST must be refused, got %d", rec.Code)
	}
}

type httpResult struct {
	code                                int
	body, etag, contentType, disposition string
}

func getStatus(t *testing.T, h http.Handler, path string) httpResult {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return httpResult{code: rec.Code, body: rec.Body.String(), etag: rec.Header().Get("ETag"), contentType: rec.Header().Get("Content-Type"), disposition: rec.Header().Get("Content-Disposition")}
}
```

Write `newAcceptedWallFixture` and `newAcceptedWallFixtureWithReadiness` in `boarddocument_test.go` from the package's existing fixture helpers (a fixturegit store with one accepted feature spec on `main`; the handler is `NewHandlerWith(root, Deps{Readiness: snap})`, `internal/workbench/handler.go:53`); they return the handler, the repo, and the spec name.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/workbench/ -run TestBoardDocument`
Expected: FAIL (404 for the page; helpers undefined until written).

- [ ] **Step 3: Write the server side**

```go
package workbench

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
)

// documentSnapshot is the Document tab's conditional projection: the
// rendered HTML fragment, the canonical Markdown it was rendered from,
// and a revision token over both plus the kind and ref. Nothing here is
// authority; the Markdown carries the not-authority stamp itself.
type documentSnapshot struct {
	Revision string `json:"revision"`
	HTML     string `json:"html"`
	Markdown string `json:"markdown"`
	Kind     string `json:"kind"`
	Ref      string `json:"ref"`
	Proposed bool   `json:"proposed"`
}

// loadDocument renders the working tree of the checkout this server
// serves (root mount: the serving checkout; branch mount: that branch's
// worktree), stamped with its HEAD, proposed unless the bytes are the
// accepted bytes on the default branch (ruling R-W2-4/R-W2-5). Readiness
// is supplied only when the served snapshot targets this spec.
func (s *boardSpecServer) loadDocument(ctx context.Context, name string, kind specdoc.Kind) (documentSnapshot, error) {
	res, err := specdocload.Load(ctx, specdocload.Request{Root: s.root, Name: name, Mode: specdocload.ModeWorkingTree, Kind: kind, Model: s.model, Readiness: s.readiness})
	if err != nil {
		return documentSnapshot{}, err
	}
	doc, err := specdoc.Build(res.Input)
	if err != nil {
		return documentSnapshot{}, err
	}
	md := specdoc.RenderMarkdown(doc)
	html, err := specdoc.RenderHTML(doc)
	if err != nil {
		return documentSnapshot{}, err
	}
	snap := documentSnapshot{HTML: html, Markdown: md, Kind: string(kind), Ref: doc.Stamp.Ref, Proposed: doc.Stamp.Proposed}
	rev, err := canonjson.Digest(struct {
		Ref, Kind, Markdown string
	}{snap.Ref, snap.Kind, snap.Markdown})
	if err != nil {
		return documentSnapshot{}, err
	}
	snap.Revision = rev
	return snap, nil
}

func documentKindFromQuery(r *http.Request) (specdoc.Kind, error) {
	k := r.URL.Query().Get("kind")
	if k == "" {
		return specdoc.KindSpec, nil
	}
	return specdoc.ParseKind(k)
}

// boardDocumentPageHandler serves the Document tab, or the raw Markdown
// as a download when ?format=md is given.
func (s *boardSpecServer) boardDocumentPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.PathValue("name")
	kind, err := documentKindFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	format := r.URL.Query().Get("format")
	if format != "" && format != "md" {
		http.Error(w, fmt.Sprintf("format must be md, got %q", format), http.StatusBadRequest)
		return
	}
	snap, err := s.loadDocument(r.Context(), name, kind)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	if format == "md" {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.md"`, name, kind))
		w.Header().Set("ETag", `"`+snap.Revision+`"`)
		_, _ = w.Write([]byte(snap.Markdown))
		return
	}
	page, err := renderBoardDocumentPage(s.model, r.URL.Path, name, snap)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(page)
}

// boardDocumentSnapshotHandler is the conditional projection route:
// ETag = the revision token; If-None-Match on the same token answers 304.
func (s *boardSpecServer) boardDocumentSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	kind, err := documentKindFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	snap, err := s.loadDocument(r.Context(), r.PathValue("name"), kind)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	etag := `"` + snap.Revision + `"`
	w.Header().Set("ETag", etag)
	if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}
```

Route table (`handler.go:33-41`): add two rows in the same shape as the existing ones — `{suffix: "/document", handler: (*boardSpecServer).boardDocumentPageHandler}` and `{suffix: "/document/snapshot", handler: (*boardSpecServer).boardDocumentSnapshotHandler, json: true}` — using the exact struct literal style the file uses (read it); because `RegisterRoutesWithHome` iterates the table for both mounts, the branch mount gets the routes for free. Where `boardSpecServer` is constructed (`handler.go:145`), set its new `readiness` field from `deps.Readiness`; add `readiness *readinesspilot.Snapshot` to the struct at `boardspec.go:120+`. Register the asset next to `boardspecasd.js` (`handler.go:193`) and add it to the embed list (`assets.go:10`).

- [ ] **Step 4: Write the page and the JS**

`boarddocumentrender.go` — a small template in the style of `boardSpecPageTemplate` (`boardspecrender.go:183`): reuse its `<head>` block (stylesheet link, meta) verbatim, then:

```go
package workbench

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/jyang234/verdi/internal/model"
)

var boardDocumentPageTemplate = template.Must(template.New("boarddocument").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<link rel="stylesheet" href="/assets/style.css">
</head>
<body class="document-page">
<nav class="site-nav workbench-nav"><a href="/">index</a> · <a href="{{.BoardHref}}" data-testid="document-tab-board">Board</a> · <span aria-current="page" data-testid="document-tab-document">Document</span></nav>
<header class="document-header">
  <h1>{{.Title}}</h1>
  <nav class="document-kinds" aria-label="Document kind">
    {{range .Kinds}}{{if .Current}}<span aria-current="page" data-testid="document-kind-{{.Kind}}">{{.Label}}</span>{{else}}<a href="{{.Href}}" data-testid="document-kind-{{.Kind}}">{{.Label}}</a>{{end}} {{end}}
  </nav>
  <div class="document-actions">
    <button type="button" id="document-refresh" data-testid="document-refresh">Refresh</button>
    <button type="button" id="document-copy" data-testid="document-copy">Copy Markdown</button>
    <a id="document-download" data-testid="document-download" href="{{.DownloadHref}}" download="{{.DownloadName}}">Download {{.DownloadName}}</a>
    <span id="document-status" role="status" aria-live="polite"></span>
  </div>
</header>
<main id="document-region" data-revision="{{.Revision}}" data-snapshot-href="{{.SnapshotHref}}" data-testid="document-region">{{.HTML}}</main>
<script type="text/plain" id="document-markdown">{{.Markdown}}</script>
<script src="/assets/specdocument.js"></script>
</body>
</html>`))

type documentKindLink struct {
	Kind, Label, Href string
	Current           bool
}

type documentPageData struct {
	Title        string
	BoardHref    string
	Kinds        []documentKindLink
	DownloadHref string
	DownloadName string
	Revision     string
	SnapshotHref string
	HTML         template.HTML
	Markdown     string
}

// renderBoardDocumentPage builds the Document tab. Every sibling link is
// derived from the request path, never from a raw ref, so the page works
// identically under the root and the /b/{branch} mounts.
func renderBoardDocumentPage(mdl *model.Model, requestPath, name string, snap documentSnapshot) ([]byte, error) {
	boardHref := strings.TrimSuffix(requestPath, "/document")
	kinds := []documentKindLink{
		{Kind: "spec", Label: "Spec"},
		{Kind: "plan", Label: "Plan"},
		{Kind: "tasks", Label: "Tasks"},
	}
	for i := range kinds {
		kinds[i].Href = requestPath + "?kind=" + kinds[i].Kind
		kinds[i].Current = kinds[i].Kind == snap.Kind
	}
	data := documentPageData{
		Title:        name + " — document",
		BoardHref:    boardHref,
		Kinds:        kinds,
		DownloadHref: requestPath + "?format=md&kind=" + snap.Kind,
		DownloadName: name + "-" + snap.Kind + ".md",
		Revision:     snap.Revision,
		SnapshotHref: requestPath + "/snapshot?kind=" + snap.Kind,
		HTML:         template.HTML(snap.HTML), //nolint:gosec // the fragment is our own renderer's output over escaped object text
		Markdown:     snap.Markdown,
	}
	var buf bytes.Buffer
	if err := boardDocumentPageTemplate.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
```

`assets/specdocument.js` (well under 64 KiB):

```js
(function () {
  "use strict";
  var region = document.getElementById("document-region");
  if (!region) return;
  var statusEl = document.getElementById("document-status");
  var mdEl = document.getElementById("document-markdown");
  var revision = region.getAttribute("data-revision") || "";
  var snapshotHref = region.getAttribute("data-snapshot-href");
  var busy = false;

  function say(text) { if (statusEl) statusEl.textContent = text; }

  function apply(snap) {
    var y = window.scrollY;
    var active = document.activeElement && document.activeElement.id;
    region.innerHTML = snap.html;
    region.setAttribute("data-revision", snap.revision);
    revision = snap.revision;
    if (mdEl) mdEl.textContent = snap.markdown;
    if (active) { var el = document.getElementById(active); if (el) el.focus(); }
    window.scrollTo(0, y);
    say("Updated");
  }

  function refresh(force) {
    if (busy) return;
    busy = true;
    fetch(snapshotHref, { headers: { "If-None-Match": '"' + revision + '"' } })
      .then(function (res) {
        if (res.status === 304) { if (force) say("Up to date"); return null; }
        if (!res.ok) { say("Refresh failed (" + res.status + ")"); return null; }
        return res.json();
      })
      .then(function (snap) { if (snap) apply(snap); })
      .catch(function () { say("Refresh failed"); })
      .then(function () { busy = false; });
  }

  document.getElementById("document-refresh").addEventListener("click", function () { refresh(true); });
  document.getElementById("document-copy").addEventListener("click", function () {
    var text = mdEl ? mdEl.textContent : "";
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(function () { say("Copied Markdown"); }, function () { say("Copy failed"); });
    } else {
      say("Copy unavailable");
    }
  });

  setInterval(function () { if (document.hidden) return; refresh(false); }, 2000);
  document.addEventListener("visibilitychange", function () { if (!document.hidden) refresh(false); });
})();
```

Board page nav (`boardspecrender.go:195`): the page handler knows its request path; pass `r.URL.Path + "/document"` into the projection or the render call as `DocumentHref` and emit `<a href="{{.DocumentHref}}" data-testid="board-tab-document">Document</a>` in the nav literal next to `index`. Keep the change to the nav string and the one field.

- [ ] **Step 5: Run the Go tests**

Run: `go test -race -count=1 ./internal/workbench/ -run 'TestBoardDocument|TestBoardActionInventory|Revise|Create' && go test -count=1 ./internal/specalign/ -run TestVocabProseWitness && node --check internal/workbench/assets/specdocument.js && gofmt -l . && go vet ./internal/workbench/ && wc -c internal/workbench/assets/specdocument.js`
Expected: PASS; the asset is far below 65536 bytes.

- [ ] **Step 6: Playwright**

Create `e2e/tests/79-board-document.spec.ts` (serial, chromium; copy the `snapshotOf` helper and the two conditional-refresh tests from `e2e/tests/50-design-workbench.spec.ts:26,618,629` and adapt them to the document routes):

```ts
import { test, expect } from "@playwright/test";
import { SHOWCASE, boardPath, branchBoardPath } from "./fixtures";

const docPath = (spec: string) => `${boardPath(spec)}/document`;

test.describe("board document tab", () => {
  test("the sealed wall offers a Document tab that renders the accepted reading", async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.READONLY_SPEC));
    await page.getByTestId("board-tab-document").click();
    await expect(page).toHaveURL(new RegExp(`${docPath(SHOWCASE.READONLY_SPEC)}$`));
    await expect(page.getByTestId("document-region")).toContainText("Identity");
    await expect(page.getByTestId("document-region")).toContainText("not authority");
    await expect(page.getByTestId("document-region")).not.toContainText("Proposed, not accepted");
    await page.getByTestId("document-kind-plan").click();
    await expect(page.getByTestId("document-region")).toContainText("Decisions");
    await expect(page.getByTestId("document-region")).not.toContainText("Problem");
  });

  test("a design-branch draft renders as proposed", async ({ page }) => {
    await page.goto(`${branchBoardPath(SHOWCASE.DESIGN_BRANCH, SHOWCASE.DESIGN_SPEC)}/document`);
    await expect(page.getByTestId("document-region")).toContainText("Proposed, not accepted");
  });

  test("snapshot answers 304 for an unchanged revision token", async ({ page }) => {
    const first = await page.request.get(`${docPath(SHOWCASE.READONLY_SPEC)}/snapshot`);
    expect(first.status()).toBe(200);
    const etag = first.headers()["etag"];
    expect(etag).toBeTruthy();
    const second = await page.request.get(`${docPath(SHOWCASE.READONLY_SPEC)}/snapshot`, { headers: { "If-None-Match": etag } });
    expect(second.status()).toBe(304);
  });

  test("hidden tabs pause polling; visibility resumes with one immediate refresh", async ({ page }) => {
    await page.goto(docPath(SHOWCASE.READONLY_SPEC));
    await page.evaluate(() => {
      (window as any).__snapshotCalls = 0;
      const orig = window.fetch;
      window.fetch = (input: any, init?: any) => {
        if (String(input).includes("/document/snapshot")) (window as any).__snapshotCalls++;
        return orig(input, init);
      };
      Object.defineProperty(document, "hidden", { configurable: true, get: () => true });
    });
    await page.waitForTimeout(4500);
    const whileHidden = await page.evaluate(() => (window as any).__snapshotCalls);
    expect(whileHidden).toBe(0);
    await page.evaluate(() => {
      Object.defineProperty(document, "hidden", { configurable: true, get: () => false });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await page.waitForTimeout(500);
    expect(await page.evaluate(() => (window as any).__snapshotCalls)).toBeGreaterThanOrEqual(1);
  });

  test("copy and download hand over the same Markdown the snapshot carries", async ({ page, context }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    await page.goto(docPath(SHOWCASE.READONLY_SPEC));
    const snap = await (await page.request.get(`${docPath(SHOWCASE.READONLY_SPEC)}/snapshot`)).json();
    await page.getByTestId("document-copy").click();
    await expect(page.getByRole("status")).toHaveText("Copied Markdown");
    const clip = await page.evaluate(() => navigator.clipboard.readText());
    expect(clip).toBe(snap.markdown);
    const dl = await page.request.get(`${docPath(SHOWCASE.READONLY_SPEC)}?format=md`);
    expect(dl.status()).toBe(200);
    expect(dl.headers()["content-disposition"]).toContain(`${SHOWCASE.READONLY_SPEC}-spec.md`);
    expect(await dl.text()).toBe(snap.markdown);
  });

  test("keyboard reaches every control", async ({ page }) => {
    await page.goto(docPath(SHOWCASE.READONLY_SPEC));
    const ids = ["document-tab-board", "document-kind-plan", "document-kind-tasks", "document-refresh", "document-copy", "document-download"];
    const seen = new Set<string>();
    for (let i = 0; i < 20 && seen.size < ids.length; i++) {
      await page.keyboard.press("Tab");
      const id = await page.evaluate(() => document.activeElement?.getAttribute("data-testid"));
      if (id && ids.includes(id)) seen.add(id);
    }
    expect([...seen].sort()).toEqual([...ids].sort());
  });
});
```

Confirm `SHOWCASE.DESIGN_BRANCH` exists in `fixtures.ts` (the survey reports `DESIGN_BRANCH = design/refi-decline-flow` at `:148`); never alias `SHOWCASE`.

Run: `cd e2e && VERDI_E2E_PORT_BASE=4790 npx playwright test tests/79-board-document.spec.ts`; then `git status --porcelain` must show only intended files and no recording artifacts.

- [ ] **Step 7: Commit**

```bash
git add internal/workbench/ e2e/tests/79-board-document.spec.ts
git commit -m "Add the board Document tab with conditional refresh, copy, and download"
```

---

### Task 6: Four-way parity (ac-6)

**Files:**
- Create: `cmd/verdi/document_parity_e2e_test.go`

**Interfaces:**
- Consumes: `buildVerdiBinary(t)` (`cmd/verdi/serve_integration_test.go:284`), `runVerdiBinary(t, bin, dir, extraEnv, args...) (stdout, stderr string, code int)` (`obligationseam_e2e_test.go:31`); `mcpserve.Backend{Root}` + `GetDocument`; `dex.Build(ctx, dex.Options{Root, OutDir})`; `workbench.NewHandlerWith(root string, deps Deps) http.Handler` (`internal/workbench/handler.go:53`); `specDocFixture`/`buildSpecDocRepo` from `cmd/verdi/specdoc_test.go`.

- [ ] **Step 1: Write the test**

```go
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/dex"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/workbench"
)

// TestDocumentParity_FourConsumers is spec/spec-documents ac-6: the CLI,
// the board Document tab, the docs site, and the MCP tool render one ref
// at one commit to byte-identical Markdown. All four run over the same
// fixturegit store on its main checkout, where the working tree, HEAD,
// and the default branch coincide, so every consumer's reading is the
// accepted one.
func TestDocumentParity_FourConsumers(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildSpecDocRepo(t)
	ctx := context.Background()
	env := []string{"CI_DEFAULT_BRANCH=main"}

	// 1. CLI
	bin := buildVerdiBinary(t)
	cliOut, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/lockbox")
	if code != 0 {
		t.Fatalf("cli exit %d: %s", code, stderr)
	}

	// 2. MCP
	backend := &mcpserve.Backend{Root: repo.Dir}
	res := backend.GetDocument(ctx, json.RawMessage(`{"ref":"spec/lockbox"}`))
	var payload struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	raw, _ := json.Marshal(res)
	if err := json.Unmarshal(raw, &payload); err != nil || payload.IsError || len(payload.Content) == 0 {
		t.Fatalf("mcp result: %s", raw)
	}
	var mcpDoc struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(payload.Content[0].Text), &mcpDoc); err != nil {
		t.Fatal(err)
	}

	// 3. Docs site
	out := t.TempDir()
	if err := dex.Build(ctx, dex.Options{Root: repo.Dir, OutDir: out}); err != nil {
		t.Fatal(err)
	}
	dexBytes, err := os.ReadFile(filepath.Join(out, "a", "spec", "lockbox", "spec.md"))
	if err != nil {
		t.Fatal(err)
	}

	// 4. Board (root mount on the main checkout)
	h := workbench.NewHandlerWith(repo.Dir, workbench.Deps{}) // internal/workbench/handler.go:53
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/lockbox/document?format=md", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("board: %d %s", rec.Code, rec.Body.String())
	}

	board := rec.Body.String()
	if cliOut != mcpDoc.Markdown {
		t.Errorf("CLI and MCP differ:\n--- cli ---\n%s\n--- mcp ---\n%s", cliOut, mcpDoc.Markdown)
	}
	if cliOut != string(dexBytes) {
		t.Errorf("CLI and docs site differ:\n--- cli ---\n%s\n--- dex ---\n%s", cliOut, dexBytes)
	}
	if cliOut != board {
		t.Errorf("CLI and board differ:\n--- cli ---\n%s\n--- board ---\n%s", cliOut, board)
	}
	if !strings.HasSuffix(cliOut, "\n") || strings.HasSuffix(cliOut, "\n\n") {
		t.Errorf("exactly one trailing newline")
	}
	// Determinism across a second CLI run.
	again, _, _ := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/lockbox")
	if again != cliOut {
		t.Errorf("two CLI renders differ")
	}
}
```

If the board leg's Evidence source differs from the CLI's (both must resolve the same HEAD and pass the same preview flag; the board passes `preview=true` because it renders working-tree mode while the CLI's accepted mode passes `false`), the fix is in `specdocload`: derive `preview` from whether the bytes are proposed, not from the mode — `preview := proposed` — and re-run all three consumers' tests. Record that decision in the report.

- [ ] **Step 2: Run**

Run: `go test -race -count=1 ./cmd/verdi/ -run TestDocumentParity`
Expected: PASS on the first try only if the four legs already agree; if a leg differs, the diff in the failure names the divergent line — fix the consumer, never the test.

- [ ] **Step 3: Commit**

```bash
git add cmd/verdi/document_parity_e2e_test.go internal/specdocload/
git commit -m "Prove four-way document parity: CLI, board, docs site, and MCP render identical bytes"
```

---

### Task 7: Wave gate

- [ ] **Step 1: Static gates**

Run: `go build ./... && gofmt -l . && go vet ./... && golangci-lint run ./...`
Expected: clean.

- [ ] **Step 2: Full gate**

Run from the worktree root: `VERDI_E2E_PORT_BASE=4390 make verify`
Expected: `verify OK`, exit 0; `git status --porcelain` empty; `find e2e -name '*.png' -o -name '*.webm' -o -name 'trace.zip' | grep -v node_modules` prints nothing.

- [ ] **Step 3: Wave report**

Write `docs/superpowers/reports/2026-09-18-spec-documents-wave-2.md` in the evidence format (Status; Risk tier 3 for ac-3/ac-5 authority surfaces, 2 for the rest; Base..Head; Commits; Files changed; Contract implemented per ac-3, ac-4, ac-5, ac-6 and the ac-1 readiness clause; Explicit exclusions; RED/GREEN; Reviewer verdict blank; Residual risks; Integration prerequisites), then:

```bash
git add docs/superpowers/reports/2026-09-18-spec-documents-wave-2.md
git commit -m "Report spec-documents wave 2: board tab, docs site, MCP, parity, readiness"
```

---

## Self-review

**Spec coverage.** ac-3: Task 5 (page and snapshot routes under the Wave 6 grammar, 2 s poll while visible, ETag/304, region-only replace, focus/scroll kept, Refresh control, copy, download with content-disposition, proposed on a branch board and accepted on the sealed wall, one JS asset). ac-4: Task 4 (Document view beside the verbatim page, three Markdown files). ac-5: Task 3 (read-only tool, write surface unchanged, inventory pins). ac-6: Task 6 (four consumers, byte-identical, determinism, one trailing newline). ac-1 readiness clause: Task 1 (section, facts, gating) and Task 5's readiness test through the board. co-1..co-6 appear in Global Constraints and in each task's checks.

**Placeholders.** None: every step carries its code; the two places where an existing signature must be read before adapting one call (`RegisterRoutesWithHome`, the dex `pageData` fields) name the file and line to read and the adaptation to make.

**Type consistency.** `specdocload.Request{Root, Name, Mode, At, Kind, Model, Readiness}` and `Result{Input, RelPath, Head, Disclosures}` (Task 2) are what Tasks 3, 4, 5 call. `documentSnapshot{Revision, HTML, Markdown, Kind, Ref, Proposed}` (Task 5) is what `specdocument.js` reads (`snap.html`, `snap.revision`, `snap.markdown`) and what the Playwright copy test compares. `specdoc.WithReadiness(f, snap, ref)` (Task 1) is what the loader calls (Task 2). `getDocumentResult{ref, kind, commit, engine, proposed, markdown, disclosures}` (Task 3) is what the parity test decodes (Task 6).

**Known limits carried to Wave 3.** The CLI, docs site, and MCP do not supply readiness (R-W2-3). The board's Document tab has no a11y scanner beyond keyboard reachability; if `e2e` gains an axe helper, add a scan. HTML byte parity is not asserted (only Markdown); the HTML is a pure function of the Markdown through one engine, pinned by Wave 1's `TestRenderHTMLMatchesMarkdownEngine`.
