# Mechanical Spec Import Implementation Plan

> **For agentic workers:** Use the existing `/fable-orchestration` workflow to execute these bounded tasks with TDD and fixed-range reviews. The owner has already selected genuine Claude Code orchestration; no execution-choice question is needed.

**Goal:** Import an existing native/Markdown feature or story into a valid proposed spec and its ordinary board, preserving source and exposing missing content truthfully.

**Architecture:** One `internal/specimport` application core owns normalization, candidate preparation, creation and import-record inspection. Adapters consume it; existing artifact, splice, lint, actor-policy, Git and board seams keep their ownership. Source sidecars are non-authoritative and excluded from default context.

**Tech Stack:** Existing Go 1.25 module, goldmark, yaml.v3 through artifact seams, canonical JSON, Git plumbing, server-rendered workbench and Playwright. No new model or network dependency.

**Spec:** `docs/superpowers/specs/2026-09-14-existing-spec-import-design.md` (owner-adopted) and `docs/superpowers/specs/2026-09-14-spec-import-contract.md` (SI-200 implementation bindings; review before execution).

## Global constraints

- 1–32 UTF-8 sources; ≤2 MiB each, ≤8 MiB total supplied bytes; 12 MiB JSON envelope. Explicit selections only. No network, archives, traversal or source execution.
- Four closed formats: native, markdown-v1, f13-reference-v1, manual-v1. No AI calls or fallback inference.
- Existing field, model, parent/tracker, evidence, actor and lifecycle requirements remain; no fabricated approval/proof.
- External text copied or explicitly user-edited with exact retained origin; positive Problem/Outcome labels prepopulate automatically. F13 missing labels remain source findings.
- One new design branch, one commit containing canonical draft and all required provenance. Never alter caller checkout/index or an existing target.
- Every mutation uses the contract's adapter identity. No request-decoded human actor or new CLI human bypass.
- Main alone owns authority/design/plan/ledger edits. Backend implementation is genuine Claude Code Sonnet; UI implementation and UI fixes are genuine FABLE 5.1; Opus 5 reviews/assigned backend defects. Accurate model provenance is required; do not use stale model labels in commit attribution.
- Every worker is not alone: preserve others' edits, stage only the declared write set and report unexpected changes. No push, PR, merge, installation replacement, hosted tests, user ATC writes or worktree deletion.
- Screenshots, video and traces stay disabled. Fixtures live only under testdata/; JSON fixtures are deterministic. No `.verdi/data` commits.
- Focused RED/GREEN per task; main full `make verify` and race once on the integrated candidate. Significant new failure or correction justifies relevant reruns. Required source coverage and review cannot be replaced by passing tests.

## Execution order and review ownership

Tasks 1–4 are sequential: freeze each returned interface before its consumer starts.
Each substantive implementation diff receives one bounded independent Opus 5
review. Tiers below apply the invoked risk protocol; critical/important backend
fixes in Tier 3 go to a fresh Opus fixer then fresh Opus re-reviewer. UI fixes
remain FABLE by the owner's exception. Main adjudicates authority and inspects
containment, verification and actual model records. No Claude worker authors new
authority to resolve a gap; return the exact conflict to main. Specification-only
contract review is separately one cross-model review plus at most one correction
and same-reviewer closure, not a runtime reviewer/fixer chain.

### Task 1: Pure source normalization and mechanical profiles — Sonnet, Tier 3

**Files:** create `internal/specimport/doc.go`, `schema.go`, `codec.go`,
`normalize.go`, `markdown.go`, `mapping.go`, `source.go`, corresponding focused
`*_test.go`; fixtures under `internal/specimport/testdata/` only. Split oversized
files by responsibility. No handlers, CLI dispatch or existing authority edits.

**Consumes:** artifact strict JSON decoding, current goldmark and canonical JSON.
Pinned proposal source snapshots/field map as fixture input, never runtime file
reads from docs. **Produces:** contract Request/Source/Mapping/Target/Link and
Plan/Snapshot/Field/Span/Coverage/Finding types; `Request.Validate`, `DecodeRequest`, `Normalize` and
`ReadSource` with exact signatures in the contract.

- [ ] Add a minimal positive labeled fixture, multiline Problem/Outcome and flat criteria; pin source spans and text, not just count. Include leading YAML frontmatter, setext title/fields and retained pre-title prose as positive cases. Add the missing/duplicate/fenced/other-target cases before code.
- [ ] Capture RED with `go test ./internal/specimport -run 'TestNormalize|TestDecodeRequest|TestReadSource' -count=1`. New behavior must fail before implementation.
- [ ] Implement strict limits, normalized selections, real Markdown structure, deterministic profiles/explicit mappings and byte partitions. No template rendering or repository writes in this task.
- [ ] Add exact F13 fixtures and compare 4 snapshots/8 criteria/2409 primary bytes against pinned originals. Add altered-profile-byte refusal, missing evidence, unassigned support, overlap accounting and invalid source-span cases.
- [ ] Drive Normalize directly with malformed struct values (invalid/duplicate source IDs, enum/size/target violations), not just decoder input. Assert the same refusal. Test evidence-only mappings retain copied origin, all coverage totals balance, and source-ID-looking object labels demand explicit resolution.
- [ ] Verify both the extracted F13 snapshot and the pinned stage-plan lines 602–671 normalize to the identical selected source digest. Pin 17 byte intervals, of which 8 are mapped; the eight line-accounting units are a different measure.
- [ ] Exercise safe file reads with hermetic regular files, traversal/symlink rejection and cancellation; no symlink fallback or recursive walk.
- [ ] Run focused GREEN and `go vet ./internal/specimport`; format touched Go files. Record commands/output, changed paths and exported interface in the lane report before fixed-range review.

Core positive test shape (fixture helpers belong in the same test package):

```go
req := Request{Schema: "verdi.spec-import-request/v1", Format: "markdown-v1",
    Target: Target{Slug: "sample", Class: "feature", Title: "Sample"},
    Primary: "source", Sources: []Source{{ID: "source", Label: "sample.md", Data: raw}},
    RetainUnmapped: true}
plan, err := Normalize(req)
if err != nil { t.Fatal(err) }
if plan.Fields[0].Target != "problem" || plan.Fields[0].Text != "First line.\nSecond line." {
    t.Fatalf("problem lost source content: %+v", plan.Fields)
}
```

### Task 2: Candidate composition and shared validation — Sonnet, Tier 3

**Files:** create `internal/specimport/compose.go`, `compose_test.go`, candidate
fixtures; create `internal/lint/candidate.go`, `candidate_test.go`; narrowly edit
`internal/lint/walk.go` only if extracting its existing byte decode helper is
needed. Reuse `internal/artifact/splice`, `internal/designscaffold`, `internal/store`
and `internal/model`; a narrow creation-only body helper in `internal/artifact/splice` with tests is allowed if existing operations do not mirror body text. No independent canonical parser or rule copies.

**Consumes:** frozen Task 1 Plan/Request. **Produces:** contract `Compose`, plus
`lint.CheckCandidate(ctx context.Context, root, relPath string, content []byte)
([]lint.Finding, error)` as an additive read-only shared seam. Candidate findings
reuse existing VL-002/003/005/006 implementations over a Snapshot with the new
in-memory Document inserted and indexed; filter target-specific findings and
surface corrupt/unresolvable dependencies explicitly. Existing rules unchanged.

- [ ] Add RED tests showing a labeled native result has matching frontmatter/body, no scaffold placeholder AC/stub or orphaned body placeholders, absent stubs (zero invented decomposition), valid resolving anchors and retained-only commands absent from the spec.
- [ ] Implement through existing class template/scaffold and typed splice operations. Preserve custom template fields or report an explicit unsupported candidate. Never silently drop an operation to make validation pass.
- [ ] Add tests for missing evidence, feature attestation floor, missing parent/tracker, duplicate existing identity, incompatible model/template and invalid links. Native tests compare exact bytes and IDs; refuse frozen/closed/old incomplete native input.
- [ ] Test explicit pair deferral uses current constants/anchors and marks generated origin. An incomplete preview has fields/findings but cannot claim a valid candidate until all non-deferrable requirements pass.
- [ ] Capture RED/GREEN with `go test ./internal/specimport ./internal/lint -run 'TestCompose|TestCheckCandidate|TestNormalize' -count=1`; run package suites and vet once after integration of the shared helper. Review only the fixed Task 2 diff.

```go
candidate, findings, err := Compose(ctx, repo.Dir, req, plan)
if err != nil { t.Fatal(err) }
if len(findings) != 0 { t.Fatalf("unexpected findings: %+v", findings) }
fm, body, err := artifact.SplitFrontmatter(candidate)
if err != nil { t.Fatal(err) }
spec, err := artifact.DecodeSpec(fm)
if err != nil { t.Fatal(err) }
if err := spec.ResolveObjectAnchors(body); err != nil { t.Fatal(err) }
if bytes.Contains(candidate, []byte("git commit -m")) { t.Fatal("retained command promoted") }
```

### Task 3: Preview, atomic creation and verified import records — Sonnet, Tier 3

**Files:** create `internal/specimport/service.go`, `publish.go`, `record.go` and
focused tests; add trusted path helpers in `internal/store/paths.go` with tests;
add `imports` only to `internal/lint/walk.go` top-level admission; narrowly export
and test the existing actor-policy dispatch in `internal/draftmutation/service.go`
without changing decisions. Shared Git helper changes stay in `internal/gitx` with
hermetic tests. Context exclusion assertions use the existing index/designapp/
contextcompile test seams; do not create a second context builder.

**Consumes:** Task 1/2 APIs and existing Git plumbing, actor constructors/policy,
canonical JSON and writer/file-lock seams. **Produces:** NewService, Service.Preview,
Service.Apply, ReadRecord, typed PreviewResult/Result/RecordView/Record/errors in
the contract. Consumer-owned injectable I/O ports may be internal; production
must retain the one shared validation/mutation/authority algorithm.

- [ ] Write RED integration tests with fixturegit, snapshotting HEAD/branch refs/index/files before preview and refused apply. Include clean/stale/dirty/index-staged contexts and exact deterministic preview recomputation.
- [ ] Implement preview digest binding, source sidecar record, strict provenance validation and create-only Git publication in sorted path order. Never stage/checkout or invoke source commands.
- [ ] Test invalid Request structs at Preview and Apply directly, engine-digest changes, and malformed source IDs before any path construction.
- [ ] Drive faults before publication and after successful publication but before response through a hermetic repository port. Same-request retry must return the existing exact commit even after the checkout becomes dirty; a losing identical CAS must reconcile before collision; mismatched request/actor/moved branch must refuse. Concurrent attempts create at most one branch.
- [ ] Verify source record after later supported spec edits: historical import remains verifiable, current bytes are disclosed as changed. Corrupt/missing record or snapshot is never a pass. Record/source files cannot enter default context, artifact index or lint Snapshot/ByRef even if they contain valid-looking frontmatter. A retained byte-identical native candidate must cause zero VL-002 duplicate findings.
- [ ] Exercise browser-human versus delegated-agent policy outcomes without broadening allowed actor constructors. No adopted policy is allowed only on the existing explicit browser-human path; malformed policy fails.
- [ ] Run `go test ./internal/specimport ./internal/draftmutation ./internal/store ./internal/gitx ./internal/lint -count=1` and relevant context exclusion tests; include race on the new service tests. Record focused command outputs for review.

```go
before := snapshotRepository(t, repo.Dir)
preview, err := svc.Preview(ctx, repo.Dir, req)
if err != nil { t.Fatal(err) }
assertRepositoryEqual(t, before, snapshotRepository(t, repo.Dir))
created, err := svc.Apply(ctx, repo.Dir, req, preview.Digest, actor)
if err != nil { t.Fatal(err) }
again, err := svc.Apply(ctx, repo.Dir, req, preview.Digest, actor)
if err != nil || again.Status != "already-created" || again.Commit != created.Commit {
    t.Fatalf("retry: %+v, %v", again, err)
}
```

### Task 4: CLI and browser adoption path — Sonnet CLI, FABLE 5.1 UI, Tier 3

Separate write sets, sequential inside this task to avoid dispatch conflicts.

**CLI files (Sonnet):** `cmd/verdi/designimport.go`, `designimport_test.go`, built-
binary tests and fixtures under `cmd/verdi/testdata/`; minimal `design.go` dispatch/
usage update. **Consumes:** frozen Task 3 API. **Produces:** contract import source,
preview and apply commands; canonical JSON and 0/1/2 handling, read-only `record` inspection and explicit deferral disclosure on apply, no human CLI bypass.

- [ ] Add RED built-binary source/preview/apply tests. Preview must work without assistance policy; delegated apply must preserve existing refusal/correction guidance.
- [ ] Implement thin dispatch and strict request reading. CLI may compose JSON but must not reproduce parsing, publication, evidence or policy logic.
- [ ] Run `go test ./cmd/verdi -run 'TestDesignImport|TestImportBinary' -count=1` and required existing design dispatch/usage tests. Obtain bounded review before UI starts.

**UI files (FABLE):** new `internal/workbench/specimport.go`, `specimportrender.go`,
`specimport_test.go`, `assets/specimport.js`; minimal handler.go/index.go/branch-board
source-record affordance changes; `e2e/tests/72-spec-import.spec.ts`, fixtures under testdata and only necessary harness
registration. Existing recording-off config remains. Reuse mintBrowserActor from boardspecdesign.go; preserve the exactly-one-constructor structural test unchanged. All visible HTML/CSS/JS,
UI handler behavior and UI fixes belong to genuine FABLE 5.1.

**Consumes:** the same Task 3 service/API; existing shell, model vocabulary,
branchBoard route helper and origin protections. **Produces:** declared page/API/
record routes and discoverable home action. No parallel card model, client-side
acceptance judgment or direct filesystem draft writer.

- [ ] Write handler and browser RED cases for importing an existing labeled spec, editing a mapping, selecting evidence, acknowledging retained source and requiring a fresh preview after any edit.
- [ ] Implement accessible labeled file/range/target selection and readable fields/gaps/source coverage. Show F13's missing labels separately from eight unset evidence declarations; provide explicit pair deferral. Escape all source text and error content.
- [ ] Create the draft, follow the ordinary branch-board link, make a supported edit, reload and inspect persisted spec plus truthful original source record. Include its link next to review preparation, explaining ASD unclassified creation versus separately recorded import origin. Test corrupted record, ambiguous headings and correction paths.
- [ ] Verify one complete human browser import with no model/assistance policy available and zero model/provider calls. No coaching-only hidden command may be required by the happy path.
- [ ] Run focused Go handler tests and the new Playwright test file with trace/video/screenshot disabled; scan recording artifacts after each invocation. Main checks screenshots were not used and actual FABLE model records before acceptance.

### Task 5: Integrated gates, documentation and local rehearsal — main plus assigned fixes

**Files:** README/docs-guide claims relevant to new capability (main documentation),
new concise release report and deterministic test fixtures where required. Runtime
fixes route to Opus/FABLE per ownership. No installation replacement or user ATC
checkout modification.

- [ ] Review integrated fixed range independently with Opus 5 for cross-task
  source/provenance/authority/context/CLI-UI compatibility. Main adjudicates each
  finding against adopted design and SI-200. Correct through assigned models.
- [ ] Run `make verify` and `go test -race ./...` on the integrated head; preserve
  command output and identify failures without weakening gates. Scan recording
  artifacts after the browser suite.
- [ ] Build a separately identified local candidate binary and record SHA-256.
  Verify engine_digest equals that binary hash. Run F13 rehearsal in a disposable real-project pilot checkout, never the user's
  independent checkout. Exercise import, field correction/deferral, supported edit,
  reload, board/evidence inspection and truthful source-record inspection.
- [ ] Update onboarding with actual supported syntax, Markdown grammar, CLI actor
  limitation and explicit unsupported formats. Verify commands against that binary.
- [ ] Record what passed, missing external proof and exact release identity. This
  new rehearsal is assisted. Local MVP acceptance still requires two complete
  journeys on the same release with the second independent; do not count the
  paused/assisted prior session or hosted work as completed evidence.

## Plan coverage and preflight

Task 1 covers source/input/format/field/coverage contracts; Task 2 covers candidate/
anchors/native/model/reference/requiredness; Task 3 covers identity/policy/provenance/
publication/retry/context exclusion; Task 4 covers both adapters and usable editing;
Task 5 covers docs, whole-change review, gates and F13 rehearsal. All parent design
acceptance witnesses have an assigned task. The adopted design's tracker, AI-free,
create-only, context and release boundaries are global constraints on each task.
No new source authority is inferred by runtime workers. Save full local command
logs separately, with concise committed lane/release evidence. A task is not
accepted from a producer's summary alone.
