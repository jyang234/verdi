# Readiness Recovery — Wave 1 Implementation Plan (ac-1 eventual blockers, ac-2/ac-3 readiness loader, ac-4 parity, ac-5 wall gap witness)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make readiness continuous: the journey derives eventual closure blockers from declared requirements; one per-request loader serves readiness for any active spec on any branch to the CLI, the board's readiness page and Document tab, and MCP, byte-identically; no request ever launches the policy-conflict judge; the wall shell's divergence from that derivation is pinned as an explicit gap list.

**Architecture:** `internal/journey` gains real eventual derivation behind two new read-only ports (stub reconciliation and the feature outcome floor, both built on `internal/matrixprojection` + `internal/evidence`, which journey may import) and an optional conflict-report extra; five reason codes join the closed vocabulary. The startup adapter in `cmd/verdi/readiness_snapshot.go` lifts, with its cmd-private helpers, into `internal/readinessload` (`Load(ctx, root, ref, Options)`), losing the design-branch gate and the mandatory context request; `internal/policyconflict` gains a cache-only judge so the loader can take cached semantic judgments without ever executing the judge command. The three snapshot consumers and `cmd/verdi/specdoc.go` call the loader per request through consumer-defined ports; the readiness page's stamp line is the one markup change (Fable slice). A Go test pins the concern-id gap between the loader's snapshot and the wall shell over the claim-wall fixture.

**Tech Stack:** Go 1.25 (`github.com/jyang234/verdi`), `internal/journey`, `internal/readinesspilot`, `internal/policyconflict`, `internal/contextcompile`, `internal/matrixprojection`, `internal/evidence`, `internal/index`, `internal/specdocload`/`internal/specdoc`, `internal/workbench`, `internal/mcpserve`, `internal/fixturegit`, Playwright (`e2e/tests/49-readiness-pilot.spec.ts` only).

**Spec:** `.verdi/specs/active/readiness-recovery/spec.md` on branch `design/readiness-recovery` (PR #334; merge = acceptance) — ac-1..ac-5 under co-1..co-6, dc-1, dc-3, dc-5, dc-6. Parent authority: spec/guided-lifecycle-governance-v3 AC-6, DC-11 ("Readiness looks forward only at already-declared requirements … does not forecast"); dc-2 of spec/spec-documents (Wave 6 presentation design §3.1/§8.2) for the readiness page; spec/spec-documents ac-6 parity contract.

## Global Constraints

- No network and no external process in any test (co-1): git through `internal/fixturegit`; CLI through the built binary (`buildVerdiBinary`/`runVerdiBinary`); MCP through `mcpserve.Backend`; the judge is never executed — a test that needs a semantic judgment seeds the cache file at `store.PolicyConflictCachePath` with a canonical judgment, never runs a command.
- Projections, not authority (co-2): no readiness file, cache, status field, transition, receipt, event log, or artifact kind is added; the journey record and the readiness snapshot remain in-memory derivations; the only on-disk read the loader adds is the existing judge cache.
- Write surface closed (co-3): nothing in this wave writes to the store or the repository.
- co-4: no workbench markup, CSS, or JS changes except the readiness page's derivation stamp line and its `?spec=` query handling; that slice is Fable work (Task 4); the wall shell (`boardspecasd.go`, `boardshellrender.go`) is NOT edited.
- co-5: every new production literal containing a class word (`feature`, `story`, `spike`, `component`) or a lifecycle state word (`draft`, `proposed`, `accepted-pending-build`, `accepted`, `superseded`, `closed`) routes through `*model.Model` or carries `// vocab:identity — <why>`; `go test -count=1 ./internal/specalign/ -run TestVocabProseWitness` passes after every task.
- co-6: an unavailable fact is stated: a journey projected without a conflict report says so in `blockers.eventual.disclosures`; a readiness derivation without a context request renders the check-context area's `context/verdict` as unproven with that reason; a cache miss renders `context/semantic/<id>` unproven with reason `judge-unavailable` and the context-conflict destination.
- DC-11: no eventual blocker depends on the wall clock. Exception expiry is judged by the conflict report's own resolution (which has a date source), never by comparing a date to `time.Now()` in journey or readinessload. `go test` must be deterministic: the same ref at the same HEAD derives identical bytes.
- Journey grammar stays closed and validated: new blocker IDs match `^[a-z][a-z0-9-]*(/[a-z][a-z0-9-]*)*$`; `Transition` is a model verb id or `build:start`; eventual items are strictly ascending by ID and unique against current; a `ReasonCode` has one fixed class. The canonical golden `internal/journey/testdata/canonical-record.json` and its digest are regenerated exactly once (Task 1) with the diff reviewed.
- `internal/journey` may import `internal/index`, `internal/matrixprojection`, `internal/policyconflict`, `internal/policyartifact`; it may NOT import `cmd/verdi`, `internal/readinesspilot`, `internal/specdoc*`, `internal/workbench`, `internal/mcpserve`, `internal/sealedexec`, `internal/dex`, `internal/designapp`, `internal/contextresolve`, `internal/skillpack`, `internal/sealedreview` (they depend on journey). `internal/readinessload` may not import `internal/workbench` (board hrefs are computed by the caller, see R-RR1-6).
- Serialized registries: only Task 3 edits `cmd/verdi/help.go`, `cmd/verdi/specdoc.go`'s usage constants, `internal/showcasealign/coverage_test.go`; only Task 2 edits `cmd/verdi/serve.go`.
- Exit codes unchanged: `verdi journey` always 0 or 2; `verdi spec doc` 0 or 2; `verdi serve` 2 on an unbuildable startup readiness.
- Risk tiers (dc-6): Tasks 1, 2, 3 Tier 2; Task 4 (Fable) Tier 2; Task 5 Tier 1. gofmt/vet/golangci-lint clean; `go test -race` clean. Implementer commits end with `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>` (Fable slice: `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`). Never write a `// path/to/file.go` marker into a file. Never use bare `git stash`.
- Work happens in `/Users/johnyang/code/verdi-system/verdi-wt/readiness-recovery-w1` on branch `agent/readiness-recovery-wave-1` (base main e768cf9c). Port base `VERDI_E2E_PORT_BASE=4390` for the gate; lanes use 4490.

---

## Rulings recorded in this plan

- **R-RR1-1 (eventual sources and their reason codes).** ac-1's seven sources map to six derivations, all in `internal/journey`, and five new reason codes with fixed classes: `stub-unreconciled` (mechanical; one per feature stub whose bucket is `unreconciled`, transition `close`, from `evidence.ReconcileStubs` over `matrixprojection.DiscoverImplementingStories`); `outcome-floor-unsatisfied` (judgmental; one per feature criterion whose `FloorResult.Satisfied` is false, transition `close`, from `evidence.FoldFeature` as `matrixprojection` already invokes it); `question-claimed-by-spike` (mechanical; one per open question a spike stub `resolves`, transition `close`: the spike's resolution is the debt — an UNCLAIMED question is not a journey blocker; it stays the readiness shape concern it is today, so it is never counted twice); obligation blockers bound to a NON-candidate later transition (reusing `obligation-*-unproven` codes and the existing `obligationBlockerID` grammar with the later verb, so IDs never collide with current ones, which use candidate verbs); `principal-resolution-unproven/<later-verb>` for required roles at non-candidate transitions carrying countersign (reusing the existing code; resolution stays "unproven" — v1 wires no resolver, disclosed as today); `conflict-row-unresolved` (class from the row: mechanical rows → mechanical, semantic rows → judgmental; one per report row whose state is not proven, transition `accept`) and `exemption-ineffective` (governance; one per `ExemptionResolution` whose `Resolution.Bound` or `.Freshness` is not proven, transition `accept`) — both ONLY when a conflict report is supplied (R-RR1-2). "Absent authoritative receipts" is covered by the later-transition obligation blockers (receipt-bearing obligations the model declares); receipt kinds the model does not declare as obligations are not derived, disclosed in the ledger. Cost if wrong: reason codes and one derivation each.
- **R-RR1-2 (the conflict report is an optional extra).** `Projector.ProjectWith(ctx, cfg, arg, Extras{Conflict *policyconflict.Report})`; `Project` delegates with empty extras. With a nil report the eventual section is still `Derived: true` and carries the disclosure `"conflict rows and exemption bounds were not evaluated by this projection: no policy-conflict report was supplied"`. `verdi journey` supplies none in this wave; the readiness loader supplies its own evaluation. Cost if wrong: one optional parameter.
- **R-RR1-3 (derived for every lifecycle state).** The eventual section derives for proposed and accepted specs alike and for both classes (feature-only sources apply to features); the parent's "from feature acceptance onward" is the floor, and a debt named earlier is still a debt, never a violation (DC-11). Cost if wrong: one state guard.
- **R-RR1-4 (cache-only judge).** `policyconflict.NewCacheOnlyJudge(adapter JudgeAdapter) Judge` returns the cached `ValidatedExchange` on a hit and `ErrJudgeCacheMiss` on a miss; `runValidatedJudge` treats `ErrJudgeCacheMiss` exactly like a nil judge (no exchange → `deriveSemanticProof` yields unproven with `judge-unavailable`). The cache key is computed by the same `judgeCacheKeyDigest` path a real run uses, so a judgment cached by `verdi context conflict` or by `serve --context-request` is found by a request. Cost if wrong: one wrapper and one sentinel.
- **R-RR1-5 (the context request is optional).** `readinessload.Options.ContextRequestPath` is optional. When set, the request is decoded, validated as today (design phase, whole-spec ref, expected repository), and the conflict evaluation runs with the cache-only judge; `RequestDigest` is the request bytes' digest. When absent, no conflict request is synthesized (inventing adapter, grants, or scope would be authority the store never declared): the check-context area carries exactly `context/verdict` unproven with witness `"no context request supplied for this derivation"` and destination `verdi context conflict --request <path>`, and `RequestDigest` is the digest of the empty request (`sha256` of zero bytes) so the snapshot still validates. Cost if wrong: one branch in the loader.
- **R-RR1-6 (no workbench import from the loader).** The board path a shape concern's destination needs (`workbench.BranchBoardHref`) is supplied by the caller through `Options.BoardHref func(branch, name string) string`; `cmd/verdi` and the workbench pass the real function, the CLI passes it too (the destination text is part of parity), and a nil func yields the CLI fallback vector. Cost if wrong: one option.
- **R-RR1-7 (the stamp).** `Snapshot.StaleNotice` keeps its field name (schema compatibility for every consumer) and its text becomes `Derived at HEAD <sha> for this request.`; the readiness page's chrome says "Derivation stamp." instead of "Startup snapshot." and its purpose sentence says the page is derived for each request. `readinesstest.ValidSnapshot` follows. Cost if wrong: strings.
- **R-RR1-8 (parity keeps the ref gate as a guard).** `specdoc.WithReadiness` keeps refusing a snapshot whose `TargetRef` differs from the document's ref; every consumer now asks the loader for its own ref, so the guard is a tripwire, not a scope rule. The parity test's foreign-snapshot arm stays. Cost if wrong: none.
- **R-RR1-9 (`--no-readiness`).** `verdi spec doc` derives readiness by default for the spec and tasks kinds in the accepted and working-tree modes (never for `--at`); `--no-readiness` omits it, rendering the existing "not supplied" line; the four-way parity test's readiness arm runs all four consumers with the loader and requires byte-equality, and its no-readiness arm runs the CLI with `--no-readiness` against a board and MCP wired without a loader. A loader failure (for example, the journey cannot be projected) is a disclosure on stderr and the section renders "not supplied" — a document is not a verdict (spec-documents ac-2). Cost if wrong: one flag.
- **R-RR1-10 (wall gap witness shape).** `internal/workbench/readinessgap_test.go` derives the loader's snapshot and the wall shell over the same claim-wall fixture, reduces both to concern-id families (the id with `<…>` for variable tails), and compares the two directional differences to `internal/workbench/testdata/readiness-gap.golden` committed with the wave; the test FAILS if the gap shrinks or grows without the golden being updated, and the golden's header says it is a gap list, never parity. Cost if wrong: one golden.

---

## File structure

| File | Responsibility |
|---|---|
| `internal/journey/reason.go` | five new `ReasonCode`s with fixed classes. |
| `internal/journey/port.go` | `StubReconciler`, `FeatureFolder` ports + production adapters; `Extras`. |
| `internal/journey/eventual.go` (new) | `deriveEventual(in eventualInput) EventualBlockers` and its six source derivations. |
| `internal/journey/project.go`, `facts.go` | `ProjectWith`, gathering the two new fact families for features. |
| `internal/journey/*_test.go`, `testdata/canonical-record.json` | table tests per source (positive + cleared), golden regeneration. |
| `internal/policyconflict/cachejudge.go` (new) | `NewCacheOnlyJudge`, `ErrJudgeCacheMiss`; `service.go` treats a miss as no judgment. |
| `internal/readinessload/{doc.go,load.go,conflict.go,facts.go,options.go}` (new) | the lifted adapter: `Load`, request handling, conflict evaluation with the cache-only judge, provenance/mutation/board/claimed-question facts. |
| `internal/readinessload/*_test.go` | moved from `cmd/verdi/readiness_snapshot*_test.go`, plus no-request, cache-hit, cache-miss, any-branch cases. |
| `cmd/verdi/readiness_snapshot.go`, `conflictgate.go:163`, `context_conflict.go` | deleted/thinned: `cmd/verdi` keeps only the serve wiring; `newLocalContextConflictProvider` moves to `readinessload` (or `policyconflict`) and `cmd/verdi/context_conflict.go` calls it there. |
| `cmd/verdi/serve.go` | `--context-request` builds the loader options; the startup pre-run evaluates once with the REAL judge (as today) to warm the cache, then the same loader serves requests. |
| `internal/readinesspilot/derive.go:141` | stamp text; `readinesstest.ValidSnapshot`. |
| `internal/workbench/boardspec.go`, `handler.go`, `readiness.go`, `boarddocument.go` | `Deps.Readiness` becomes `Deps.ReadinessLoader` (port) + `Deps.ReadinessDefaultSpec`; `/readiness?spec=`; Document tab per ref. |
| `internal/workbench/readinessrender.go`, `readiness_test.go`, `e2e/tests/49-readiness-pilot.spec.ts` | Fable slice: stamp chrome and purpose sentence; e2e assertions. |
| `internal/mcpserve/backend.go`, `tool_get_document.go` | `Backend.ReadinessLoader`; per-ref derivation for live modes. |
| `cmd/verdi/specdoc.go`, `specstate.go:58`, `help.go` | `--no-readiness`; usage constants. |
| `cmd/verdi/mcp.go` | standalone `verdi mcp` wires the loader (no request). |
| `cmd/verdi/document_parity_e2e_test.go` | readiness arm over the loader; no-readiness arm. |
| `internal/workbench/readinessgap_test.go`, `testdata/readiness-gap.golden` | ac-5 witness. |
| `internal/showcasealign/coverage_test.go`, `cli_showcase_test.go`, `mcp_showcase_test.go` | `cli:spec` and `mcp:get_document` rows gain the readiness proof. |
| `docs/superpowers/reports/2026-09-19-readiness-recovery-wave-1.md` | wave report (Task 6). |

---

## Task 0 (controller): SDD ledger

Open `.superpowers/sdd/2026-09-19-readiness-recovery-wave-1/progress.md` with the preflight scan (below) before Task 1 dispatches.

---

### Task 1: Eventual closure blockers in the journey (ac-1)

**Files:**
- Create: `internal/journey/eventual.go`, `internal/journey/eventual_test.go`
- Modify: `internal/journey/reason.go` (five codes + classes), `internal/journey/port.go` (two ports, `Extras`, adapters), `internal/journey/project.go` (`ProjectWith`; `Eventual: deriveEventual(...)`), `internal/journey/facts.go` (`Facts.Stubs`, `Facts.FeatureFold` for class feature), `internal/journey/derive.go` (delete the stub `deriveEventual()`), `internal/journey/derive_test.go:374` (rewrite `TestDeriveEventual`), `internal/journey/fixtures_test.go:77`, `internal/journey/testdata/canonical-record.json` (regenerate), `internal/journey/reason_test.go` (vocabulary pins), `cmd/verdi/journey_test.go` (an eventual-populated assertion over a feature fixture)

**Interfaces:**
- Consumes: `evidence.ReconcileStubs(StubReconcileInput) (StubReconciliation, error)` (`internal/evidence/stubreconcile.go:126`), `matrixprojection.DiscoverImplementingStories(ctx, root, commit, featureName, spec, ix, resolver)` (`project.go:181`), `evidence.FoldFeature(FeatureInput) (FeatureResult, error)` (`featurefold.go:150`), `index.Build`, `policyconflict.Report{Mechanical, Semantic}` + `ExemptionResolution{Resolution AuthorityResolution{Bound, Freshness}}`, `model.Transition`, `journey.Blocker`, `obligationBlockerID`, `obligationReason`, `transitionHasCountersign`, `candidateTransitions`.
- Produces:

```go
package journey

const (
	ReasonStubUnreconciled       ReasonCode = "stub-unreconciled"        // ClassMechanical
	ReasonOutcomeFloorUnsatisfied ReasonCode = "outcome-floor-unsatisfied" // ClassJudgmental
	ReasonQuestionClaimedBySpike ReasonCode = "question-claimed-by-spike" // ClassMechanical
	ReasonConflictRowUnresolved  ReasonCode = "conflict-row-unresolved"   // class per row: mechanical|judgmental — see note
	ReasonExemptionIneffective   ReasonCode = "exemption-ineffective"     // ClassGovernance
)

// Extras carries request-time facts a caller may supply to a projection.
type Extras struct {
	Conflict *policyconflict.Report
}

func (p Projector) ProjectWith(ctx context.Context, cfg *store.Config, arg string, extras Extras) (Record, error)

type StubReconciler interface {
	Reconcile(ctx context.Context, root, commit string, spec *artifact.SpecFrontmatter, mdl *model.Model) (evidence.StubReconciliation, error)
}
type FeatureFolder interface {
	Fold(ctx context.Context, root, commit string, spec *artifact.SpecFrontmatter, mdl *model.Model) (evidence.FeatureResult, error)
}
```

Note on `conflict-row-unresolved`: `reasonClasses` binds one class per code. Use TWO codes to keep that invariant: `conflict-mechanical-unresolved` (mechanical) and `conflict-semantic-unresolved` (judgmental). That makes six new codes; update the constant block accordingly.

- [ ] **Step 1: Failing tests (vocabulary and each source)**

`reason_test.go`: extend the existing vocabulary pin so `ReasonCodes()` returns the nine old codes plus the six new ones, each with its fixed class; a code outside the set fails `Class()`.

`eventual_test.go` (package `journey`): one table per source with a positive case and a cleared case, driving `deriveEventual` directly with hand-built inputs (no I/O):

```go
func TestDeriveEventual_StubsAndFloor(t *testing.T) {
	owner := Owner{Declared: "platform-team", Attribution: governanceprincipal.NewUnauthenticatedAttribution()}
	in := eventualInput{
		Class: "feature", Owner: owner, LaterTransitions: []model.Transition{{Verb: "close"}},
		Stubs: &evidence.StubReconciliation{Stubs: []evidence.StubResult{{Slug: "board-tab", Bucket: evidence.StubUnreconciled}, {Slug: "core", Bucket: evidence.StubRealized, RealizedBy: []string{"spec/core"}}}},
		Fold:  &evidence.FeatureResult{ACs: []evidence.FeatureACResult{{ID: "ac-1", Floor: evidence.FloorResult{Satisfied: false, DeclaresAttestation: true, Attestation: evidence.AttestationAbsent}}, {ID: "ac-2", Floor: evidence.FloorResult{Satisfied: true}}}},
	}
	eb := deriveEventual(in)
	if !eb.Derived || len(eb.Disclosures) != 1 || !strings.Contains(eb.Disclosures[0], "no policy-conflict report") {
		t.Fatalf("eventual = %+v", eb)
	}
	ids := blockerIDs(eb.Items)
	want := []string{"outcome-floor/ac-1", "stub-unreconciled/board-tab"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	byID := indexBlockers(eb.Items)
	if b := byID["stub-unreconciled/board-tab"]; b.Reason != ReasonStubUnreconciled || b.Class != ClassMechanical || b.Transition != "close" || !strings.Contains(b.ClearingCondition, "board-tab") || len(b.Witnesses) == 0 {
		t.Fatalf("stub blocker = %+v", b)
	}
	if b := byID["outcome-floor/ac-1"]; b.Reason != ReasonOutcomeFloorUnsatisfied || b.Class != ClassJudgmental || !strings.Contains(b.ClearingCondition, "attestations/") {
		t.Fatalf("floor blocker = %+v", b)
	}
	if err := (Blockers{Current: []Blocker{}, Eventual: eb}).validate(); err != nil {
		t.Fatal(err)
	}
}
```

Add, in the same style: `TestDeriveEventual_ClaimedQuestions` (a spec with `oq-1` claimed by spike stub `s` → `question-claimed/oq-1`, transition `close`, witnesses name the stub; an unclaimed `oq-2` produces NO item); `TestDeriveEventual_LaterTransitionObligations` (candidates `[accept]`, later `[close]` whose obligations carry `attestation/countersign` and `fold/green` → `obligation-countersign-unproven/close` and `obligation-fold-green-unproven/close`, and NO duplicate of a current blocker for the candidate verb; plus `principal-resolution-unproven/close`); `TestDeriveEventual_ConflictReport` (a report with one mechanical row `State: violated-with-witness`, one semantic row `unproven`, one exemption resolution with `Bound: unproven` → `conflict-mechanical/<row>`, `conflict-semantic/<row>`, `exemption-ineffective/<id>`, all transition `accept`; with a nil report → none of them and the disclosure); `TestDeriveEventual_StoryClassSkipsFeatureSources` (class story with a non-nil `Stubs` → no stub or floor items); `TestDeriveEventual_OrderingAndUniqueness` (items sorted ascending; an item whose ID equals a current blocker's ID is refused by `Blockers.validate`).

`project_test.go` (or `facts_test.go`, following the file's existing fixturegit recipe): `TestProject_FeatureEventualFromStore` — a feature spec with two stubs, one implemented by a closed story and one unreconciled, one AC with no attestation → the record's `Blockers.Eventual.Items` names both; the record round-trips through `Canonical`/`Decode`; a story spec in the same store gets only later-transition items.

Run: `go test ./internal/journey/ -run 'TestDeriveEventual|TestProject_FeatureEventual|TestReasonCodes' -count=1` — Expected: FAIL (undefined).

- [ ] **Step 2: Implement**

`reason.go`: add the six codes to the const block and `reasonClasses`; the doc comment lists each code's meaning in one line. `port.go`: the two ports; production adapters `stubReconciler{}` (builds `index.Build` over root at commit, `matrixprojection.DiscoverImplementingStories`, then `evidence.ReconcileStubs` with `StubStory{SpecRef, ACIDs, Closed}` from the discovered edges — read `cmd/verdi/closefeature.go:340 reconcileFeatureStubs` and replicate it in the adapter, then make `closefeature.go` call the adapter so the logic has one home) and `featureFolder{}` (as `matrixprojection/project.go:136` invokes `FoldFeature`; `Preview: true`; `FeatureSlug` = the feature's name); `Projector` gains the two fields, `NewProjector` wires them, the test constructor accepts fakes. `facts.go`: for class feature, `GatherFacts` fills `Facts.Stubs *evidence.StubReconciliation` and `Facts.FeatureFold *evidence.FeatureResult`; a reconciliation or fold error is a disclosure in `Facts.RepositoryDisclosures`-style (add `Facts.EventualDisclosures []string`) and the corresponding source yields no items — never a projection failure (co-6). `eventual.go`: `eventualInput{Class string; Owner Owner; Candidates, LaterTransitions []model.Transition; Spec *artifact.SpecFrontmatter; Stubs *evidence.StubReconciliation; Fold *evidence.FeatureResult; Conflict *policyconflict.Report; Disclosures []string}` and `deriveEventual(in) EventualBlockers` producing sorted, deduplicated items with the ID grammars: `stub-unreconciled/<slug>`, `outcome-floor/<ac-id>`, `question-claimed/<oq-id>`, `obligation-*-unproven/<verb>` (via `obligationBlockerID`), `principal-resolution-unproven/<verb>`, `conflict-mechanical/<row-id>`, `conflict-semantic/<row-id>`, `exemption-ineffective/<exemption-id>` (row and exemption ids are lowercased and any character outside `[a-z0-9-]` mapped to `-`, then validated). Later transitions = every transition of the class's lifecycle that is not a candidate (compute from `cfg.Model.Lifecycle[class].Transitions` minus `candidateTransitions`). Clearing conditions are sentences naming the exact artifact or command (`"reconcile stub <slug>: instantiate a story that claims it, or withdraw it with a note"`, `"author attestations/<feature>/<ac>.md or land a passing outcome record for <ac>"`, `"spike stub <s> resolves <oq> before close"`, the existing obligation wording, `"resolve conflict row <id> or record a disposition"`, `"renew or replace exemption <id>: its bound or freshness is not proven"`). `project.go`: `ProjectWith` passes extras into `eventualInput.Conflict`; `Project` calls `ProjectWith(..., Extras{})`; delete `derive.go:300-309`.

Regenerate the golden: `go test ./internal/journey/ -run TestGolden -update` if the package has an update flag (read `golden_test.go`); otherwise write the new canonical bytes from the test's own producer and review the diff — the eventual section must now read `derived: true` with the fixture's items and the no-report disclosure, and the record digest changes exactly once.

Run: `go test -race -count=1 ./internal/journey/ ./cmd/verdi/ -run 'Journey|Eventual|Golden|Codec|Parity|Reason' && go vet ./internal/journey/ && golangci-lint run ./internal/journey/ && go test -count=1 ./internal/specalign/ -run TestVocabProseWitness && go test -count=1 ./internal/readinesspilot/` (readinesspilot's `review/action` now flips to non-unproven when the other conditions hold — update `derive_test.go:520-527` accordingly: the "eventual blockers unavailable" case must now construct `Derived: false` explicitly rather than rely on the stub).

Commit series: `Add the six eventual-blocker reason codes to the journey vocabulary`; `Derive eventual closure blockers from declared requirements`; `Project stub reconciliation and the feature outcome floor into the journey`.

---

### Task 2: The readiness loader and the cache-only judge (ac-2, ac-3)

**Files:**
- Create: `internal/policyconflict/cachejudge.go`, `cachejudge_test.go`; `internal/readinessload/doc.go`, `load.go`, `options.go`, `conflict.go`, `facts.go`, `load_test.go`, `conflict_test.go`, `testdata/` (moved fixtures)
- Modify: `internal/policyconflict/service.go:354` (`runValidatedJudge`: `ErrJudgeCacheMiss` → no exchange), `internal/readinesspilot/derive.go:141` (stamp text), `internal/readinesspilot/readinesstest/readinesstest.go:64`, `internal/readinesspilot/schema_test.go:411`, `internal/specdoc/readiness_test.go` (stamp strings), `internal/mcpserve/tool_get_document_test.go:362`, `cmd/verdi/serve.go` (loader wiring), `cmd/verdi/context_conflict.go` (provider construction moves; the verb calls the shared constructor), `cmd/verdi/conflictgate.go:163` (`validatedConflictRequestPath` moves), `cmd/verdi/serve_integration_test.go:69,253`
- Delete: `cmd/verdi/readiness_snapshot.go`, `readiness_snapshot_test.go`, `readiness_snapshot_integration_test.go` (their tests move to `internal/readinessload`, re-homed on `fixturegit` recipes — keep every pinned property listed in the survey: exactly-one request read and its digest, `..`/symlink refusal before any read, design-phase requirement, identity mismatches, the shape-destination rewrite, exact CLI vectors, no persistence under `.verdi/data` except the judge cache).

**Interfaces:**
- Consumes: `readinesspilot.Derive/Input/Snapshot`, `journey.Projector.ProjectWith` (Task 1), `policyconflict.NewService/ServiceDeps/Request/Judge/JudgeAdapter/CachedJudge`, `contextcompile.DecodeRequest`, `store.Open/PolicyConflictCachePath`, `boardio`, `designprovenance`, `artifact`.
- Produces:

```go
package policyconflict

var ErrJudgeCacheMiss = errors.New("policyconflict: no cached judgment for this input")

// NewCacheOnlyJudge answers only from the judge cache adapter.Root holds: a hit returns the
// validated exchange exactly as CachedJudge would; a miss returns ErrJudgeCacheMiss and never
// runs adapter.Argv. The service treats a miss as "no judgment" (unproven, judge-unavailable).
func NewCacheOnlyJudge(adapter JudgeAdapter) Judge

package readinessload

type Options struct {
	ContextRequestPath string                          // optional (R-RR1-5)
	BoardHref          func(branch, name string) string // optional (R-RR1-6)
	Judge              JudgeMode                        // JudgeCacheOnly (default) | JudgeRun (serve's startup pre-run only)
}
type JudgeMode string
const ( JudgeCacheOnly JudgeMode = "cache-only"; JudgeRun JudgeMode = "run" )

// Load derives readiness for ref (an unpinned whole spec ref) at the checkout's HEAD.
func Load(ctx context.Context, root, ref string, opts Options) (readinesspilot.Snapshot, error)

// Loader is the consumer-side port every surface defines for itself; this is the production value.
type Loader struct{ Root string; Opts Options }
func (l Loader) Load(ctx context.Context, ref string) (readinesspilot.Snapshot, error)
```

- [ ] **Step 1: Failing tests**

`cachejudge_test.go`: seed a canonical judgment at `store.PolicyConflictCachePath(root, treeHash, keyDigest)` for a known `SemanticInput` (build the key with the package's own `judgeCacheKeyDigest` — same package, so reachable) → `NewCacheOnlyJudge(adapter).Judge(ctx, prompt, input)` returns the exchange with `Runner` a fake that fails the test if called; with no cache file → `ErrJudgeCacheMiss` and the runner still never called; a corrupt cache file → an operational error, not a miss. `service_test.go`: `runValidatedJudge` with a cache-only judge on a miss yields `(nil, nil)` and the semantic row resolves `unproven` with reason `judge-unavailable`.

`internal/readinessload/load_test.go` (moved + new), key new cases:
```go
func TestLoad_AnyBranchNoRequest(t *testing.T) {
	repo, ref := readinessRepo(t, "feature") // the moved recipe, but checked out on main, not design/<name>
	snap, err := Load(context.Background(), repo.Dir, ref, Options{})
	if err != nil { t.Fatal(err) }
	if snap.Branch != "main" || snap.Head != repo.Head || snap.StaleNotice != "Derived at HEAD "+repo.Head+" for this request." {
		t.Fatalf("snapshot identity = %+v", snap)
	}
	ctxArea := areaByID(snap, readinesspilot.AreaContext)
	if ctxArea.State != readinesspilot.StateUnproven { t.Fatalf("context area = %+v", ctxArea) }
	v := concernByID(snap, "context/verdict")
	if v.State != readinesspilot.StateUnproven || !contains(v.Witnesses, "no context request supplied for this derivation") || v.Destination.CLI[0] != "verdi" || v.Destination.CLI[2] != "conflict" {
		t.Fatalf("context/verdict = %+v", v)
	}
	if snap.RequestDigest != readinessDigest(nil) { t.Fatalf("request digest = %s", snap.RequestDigest) }
	// Determinism: a second derivation is byte-identical.
	again, _ := Load(context.Background(), repo.Dir, ref, Options{})
	if !reflect.DeepEqual(snap, again) { t.Fatal("derivation is not deterministic") }
}
```
plus `TestLoad_WithRequestCacheMissNeverRunsJudge` (a manifest with `align.judge_cmd` set to a script that fails the test if executed; a request file; `Options{ContextRequestPath}` → `context/semantic/<id>` unproven with witness containing `judge-unavailable`, and `.verdi/data` gains no file), `TestLoad_WithRequestCacheHit` (seed the cache as in the policyconflict test → the semantic row's state comes from the cached judgment), `TestLoad_RefusesPinnedOrFragmentRef`, `TestLoad_ArchivedSpecIsOperational`, and the moved identity/shape/CLI-vector pins.

Run: `go test ./internal/policyconflict/ -run 'CacheOnly|CacheMiss' -count=1; go test ./internal/readinessload/ -count=1` — Expected: FAIL.

- [ ] **Step 2: Implement**

`cachejudge.go` as specified (reuse `judgeCacheKeyDigest`, `store.PolicyConflictCachePath`, `refuseManagedCacheSymlinks`, `loadCachedJudgment`, `ValidateJudgeResult`; never touch the writer lock). `service.go:354`: in the `default:` arm, `if errors.Is(err, ErrJudgeCacheMiss) { return nil, nil }`.

`internal/readinessload`: move the builder body into `Load`; delete the `branch == "design/"+name` check; when `opts.ContextRequestPath == ""` skip request decode and conflict evaluation and synthesize the single unproven `context/verdict` concern (readinesspilot's `Input.Conflict` must accept an empty report — read `deriveContext` and add an `Input.ConflictUnavailable string` witness path if a zero `Report` fails `Validate`; the package is pure and importable); when set, construct the provider with `policyconflict.NewService` exactly as `newLocalContextConflictProvider` does (move that function here, exported as `NewConflictProvider(ctx, root, request, mode JudgeMode)`, and make `cmd/verdi/context_conflict.go` call it with `JudgeRun`) but with `Primary: NewCacheOnlyJudge(adapter)` under `JudgeCacheOnly`. `journey.NewProjector().ProjectWith(ctx, cfg, ref, journey.Extras{Conflict: report})` when a report exists. Stamp per R-RR1-7. `Loader.Load` is the port value.

`cmd/verdi/serve.go`: `--context-request` → at startup, `readinessload.Load(ctx, root, request.Spec, Options{ContextRequestPath, JudgeRun, BoardHref: workbench.BranchBoardHref})` ONCE (warms the cache; exit 2 on failure as today), then pass `readinessload.Loader{Root, Opts: {ContextRequestPath, JudgeCacheOnly, BoardHref}}` and `ReadinessDefaultSpec: request.Spec` into the workbench and MCP deps (Task 3 defines those fields; in this task, keep the existing `*Snapshot` wiring compiling by passing the startup snapshot as before AND adding the loader field — Task 3 removes the snapshot field).

Run: `go test -race -count=1 ./internal/policyconflict/ ./internal/readinessload/ ./internal/readinesspilot/... ./internal/specdoc/ ./internal/mcpserve/ && go test -count=1 ./cmd/verdi/ -run 'Serve|Conflict' && go vet ./... && golangci-lint run ./internal/policyconflict/ ./internal/readinessload/ ./cmd/verdi/ && go test -count=1 ./internal/specalign/ -run TestVocabProseWitness`.

Commit series: `Add a cache-only policy-conflict judge`; `Lift the readiness adapter into internal/readinessload and derive per request`; `Warm the judge cache at serve startup and hand the loader to the server`.

---

### Task 3: Consumers and parity (ac-4, Go side)

**Files:**
- Modify: `internal/workbench/boardspec.go:101-139` (`Deps.ReadinessLoader ReadinessLoader`, `Deps.ReadinessDefaultSpec string`; drop `Deps.Readiness`), `internal/workbench/handler.go:130,149`, `internal/workbench/readiness.go` (`readinessHandler(loader, defaultSpec)`: `?spec=<name>` → `loader.Load(ctx, "spec/"+name)`; no query → default spec; neither → the existing 503 disclosure; a loader error → 503 with the error's text), `internal/workbench/boarddocument.go:52-60` (`loader.Load(ctx, "spec/"+name)` for the working-tree document; a load error becomes a disclosure and no readiness), `internal/mcpserve/backend.go` (`ReadinessLoader`), `internal/mcpserve/tool_get_document.go:101-115` (live modes call the loader for `ref`; `ModeAt` never), `cmd/verdi/specdoc.go` (`--no-readiness`; loader for accepted/working-tree modes; loader errors → `spec doc: readiness: <err>` on stderr, section "not supplied"), `cmd/verdi/specstate.go:58` + `help.go` (usage), `cmd/verdi/mcp.go` (standalone mcp wires `readinessload.Loader{Root, Opts{JudgeCacheOnly}}`), `cmd/verdi/document_parity_e2e_test.go` (readiness arm over the loader for all four; the no-readiness arm), `internal/showcasealign/coverage_test.go` + `cli_showcase_test.go` + `mcp_showcase_test.go` (the `cli:spec` proof runs `spec doc` on the showcase and asserts a populated Readiness section; `mcp:get_document` likewise), `internal/workbench/readiness_test.go` (route tests: `?spec=`, default, none), `internal/mcpserve/tool_get_document_test.go`, `cmd/e2eharness/readinessallproven.go` (its hand-built snapshot now feeds a fake loader), `cmd/e2eharness/main.go` (no change to the serve command line).
- Not modified: `internal/workbench/readinessrender.go` (Task 4), `boardspecasd.go`, `boardshellrender.go`, `internal/dex` (the docs site stays without readiness; spec/spec-documents ac-4's static site is a build-time artifact with no request).

**Interfaces:**
- Consumes: `readinessload.Loader` (Task 2); `specdocload.Request.Readiness` (unchanged); `specdoc.WithReadiness` (unchanged, R-RR1-8).
- Produces: `workbench.ReadinessLoader interface { Load(ctx context.Context, ref string) (readinesspilot.Snapshot, error) }` (consumer-defined in workbench; an identical one in mcpserve — the `-er` pattern, defined at each consumer), `Deps.ReadinessDefaultSpec`, `Backend.ReadinessLoader`, `verdi spec doc … [--no-readiness]`.

- [ ] **Step 1: Failing tests** — `readiness_test.go`: `TestReadinessRoute_QuerySpecDerivesPerRequest` (a fake loader counting calls; two GETs → two loads; `?spec=x` passes `spec/x`; unknown spec → 503 with the loader's error text; no loader → 503 disclosure unchanged); `boarddocument_test.go`: the Document tab of spec A carries A's readiness and spec B's carries B's (two-spec fixture, real loader); `tool_get_document_test.go`: same per-ref property over MCP, and `commit` mode never calls the loader; `cmd/verdi/specdoc_test.go`: default render has a populated `## Readiness` for a fixture where the journey projects, `--no-readiness` renders "not supplied", `--at` never calls the loader; `document_parity_e2e_test.go`: `TestDocumentParity_FourConsumersWithReadiness` — CLI (default), MCP (`Backend{ReadinessLoader: real}`), board (`Deps{ReadinessLoader: real}`) byte-equal with a populated Readiness section; dex excluded (documented); `TestDocumentParity_NoReadiness` — CLI `--no-readiness` == board/MCP without a loader == dex. Keep the foreign-snapshot arm (R-RR1-8) by feeding a fake loader that returns a snapshot for another ref.

Run: the named tests — Expected: FAIL.

- [ ] **Step 2: Implement** per the Files list. `--no-readiness` joins `specDocForm`; the `TestHelp_SpecShowsEveryForm` pin updates. Showcase rows: `cli:spec`'s proof asserts `## Readiness` followed by `Source: readiness snapshot for` on the showcase's accepted feature; `mcp:get_document` likewise.

Run: `go test -race -count=1 ./internal/workbench/ ./internal/mcpserve/ && go test -count=1 ./cmd/verdi/ -run 'SpecDoc|DocumentParity|Help|Serve|Mcp' && go test -count=1 ./internal/showcasealign/ ./internal/specalign/ -run 'Coverage|Showcase|TestVocabProseWitness|Instruction' && go vet ./... && golangci-lint run ./internal/workbench/ ./internal/mcpserve/ ./cmd/verdi/`.

Commit series: `Serve readiness per request through a loader port on the workbench and MCP`; `Supply readiness from verdi spec doc by default with --no-readiness`; `Extend the four-way document parity to readiness`.

---

### Task 4: The derivation stamp on the readiness page (ac-2/ac-4, Fable lane)

**Files:**
- Modify: `internal/workbench/readinessrender.go:140,163-167` (purpose sentence "This page derives readiness for the current design work on every request."; chrome `<strong>Derivation stamp.</strong>`; `aria-label="Derivation stamp"`; class names unchanged), `internal/workbench/readiness_test.go:48,153,207,601` (pins), `e2e/tests/49-readiness-pilot.spec.ts:577-579,599,644` (assert `Derived at HEAD` and the stamp chrome; the instrumentation vocabulary `stale-notice-inspected` stays — it is an event name, not copy), `cmd/e2eharness/readinessallproven.go:70`.

- [ ] Steps: failing Go pins first, then the strings, then `cd e2e && VERDI_E2E_PORT_BASE=4490 npx playwright test tests/49-readiness-pilot.spec.ts tests/81-guidance-first-cards.spec.ts`, then the recording-artifact scan (`find e2e -name '*.png' -o -name '*.webm' -o -name 'trace.zip' | grep -v node_modules` prints nothing). Commit: `Stamp the readiness page with the derivation HEAD instead of a startup notice`.

---

### Task 5: The wall gap witness (ac-5)

**Files:**
- Create: `internal/workbench/readinessgap_test.go`, `internal/workbench/testdata/readiness-gap.golden`

- [ ] **Step 1:** Write the test: build the claim-wall fixture (`newClaimWallFixture`, `boardspecasd_test.go:507`), derive the wall shell via `(&boardSpecServer{root}).loadASD(ctx, claimWallName)` and the loader snapshot via `readinessload.Load(ctx, root, "spec/"+claimWallName, Options{})`; reduce each concern id to its family (`shape/question/oq-1` → `shape/question/<id>`, `success/blocker/x/y` → `success/blocker/<id>`, etc. — a small `family(id string) string` that keeps the first two segments and replaces the rest with `<id>`, with `shape/board/<kind>/<id>` keeping three); compute `wallOnly` and `loaderOnly` sorted; also compute, for the shared families, any blocking-flag disagreement (`shape/board`). Render as:
```
# readiness gap — the wall shell versus the continuous derivation (spec/readiness-recovery ac-5)
# This is a GAP LIST, not parity. The post-design workbench lane consumes it; a change here must be deliberate.
wall-only:
  context/agent-writes
  …
loader-only:
  context/disclosure/<id>
  …
blocking-disagreement:
  shape/board
```
and compare byte-for-byte with the golden; on mismatch print both and fail. Run with the golden absent → FAIL. **Step 2:** commit the golden the test produces after reading it for sanity (it must list the 10 wall-only and 12 loader-only families the survey enumerated, or explain any difference in the report). Commit: `Pin the wall shell's readiness gap as an explicit list`.

---

### Task 6: Wave gate and report

- [ ] `go build ./... && gofmt -l . && go vet ./... && golangci-lint run ./...` clean; `VERDI_E2E_PORT_BASE=4390 make verify` → `verify OK`; artifact scan empty; tree clean.
- [ ] Report `docs/superpowers/reports/2026-09-19-readiness-recovery-wave-1.md` in the evidence format; commit `Report readiness-recovery wave 1: continuous readiness`.

---

## Self-review

**Spec coverage.** ac-1: Task 1 (six derivations, six codes, `Derived: true`, disclosure when no report; DC-11 by construction — no forecast, no wall clock). ac-2: Task 2 (loader, any spec/any branch, gate removed, stamp, determinism test). ac-3: Task 2 (cache-only judge; test with a judge command that would fail the run; serve pre-run stays). ac-4: Task 3 (four consumers, `--no-readiness`, parity arms, journey eventual on the CLI) + Task 4 (page). ac-5: Task 5. co-4: Task 4 is the only markup slice and it is Fable's.

**Placeholders.** Task 1's clearing-condition sentences are given; Task 2's request/no-request branches are specified; Task 5's family reducer is described precisely. The golden files are produced by the tests and reviewed, as the journey's canonical record already is.

**Type consistency.** `journey.Extras`/`ProjectWith` (T1) ← `readinessload` (T2); `readinessload.Loader` (T2) ← workbench/mcpserve/specdoc ports (T3) and the gap test (T5); `Options.BoardHref` (T2) ← `workbench.BranchBoardHref` passed by `cmd/verdi` (T2/T3); `StaleNotice` text (T2) ← page chrome (T4) and `readinesstest.ValidSnapshot` (T2).

**Known limits carried.** `verdi journey` supplies no conflict report (disclosed); `internal/dex` stays without readiness (static build); principal resolution stays "unproven" (v1); receipt kinds not modeled as obligations are not derived (ledger note).

## Amendments during execution

Recorded here by the controller as they happen; where a code block here disagrees with HEAD, HEAD is right.

- Task 1 (R-RR1-11): "later transitions" are the transitions reachable forward from the spec's current lifecycle state (walk the class lifecycle's from→to graph from the current state), minus the immediate candidates — never a transition already behind the state, so an accepted feature never carries `merge` as an eventual blocker. Task 1's resolution (a) ("lifecycle transitions minus candidates") is superseded.
- Task 1 → Task 4: the harness readiness page now carries the eventual `review/blocker/*` concerns, so `e2e/tests/49-readiness-pilot.spec.ts`'s pinned `ATTENTION_QUEUE`/`COMPLETED_CHECKS` arrays are updated in Task 4 (Fable) together with the stamp; `make verify` is expected red at e2e between Task 1 and Task 4.
- Task 1 (R-RR1-12, SI-209): each eventual source names the transition whose gate consumes the requirement, never a shared "first later verb" and never the literal `unknown`: stub reconciliation, the outcome floor, and spike-claimed questions name `close` (the closure gate reads them); conflict rows and ineffective exemptions name the earliest forward-reachable transition whose gate evaluates policy — the acceptance transition (`merge` in the canonical model, resolved from the class lifecycle as the transition whose target is the accepted state) before acceptance, `close` after; obligation and principal items name each forward-reachable non-candidate transition (R-RR1-11). A class with no declared lifecycle emits no feature or conflict items and discloses why. The parent's AC-6 makes stub reconciliation and the outcome floor eventual even when `close` is the next legal transition: they are gated at closure time, not now.
- Task 1 (R-RR1-13, deferred to the final fix wave): once the closure transition is behind the state (closed, superseded), the closure-gated feature sources (stubs, outcome floor, claimed questions) derive nothing — a closed feature carries no closure debt.
- Task 2 (R-RR1-14): `readinessload.Options.ConflictProvider` is the hermetic seam every consumer and test injects a provider through; co-1 forbids network and the real judge, and exactly one local fake-judge launch test per package is permitted where the launch itself is the subject. Task 3 exit obligations: delete `cmd/verdi/serve.go`'s package-level `serveReadinessLoader`/`serveReadinessDefaultSpec` by threading the loader through `runServe`; re-home the CLI-dispatch round trip of the old `TestReadinessSnapshotEmittedCLIVectorsReachRegisteredCommands` in `cmd/verdi`.
- Task 3 (R-RR1-16, final fix wave): no request-specific witness is written into context/verdict — it would carry the request's target into document bytes and re-create the ac-4 parity divergence R-RR1-15 closed. co-6 is carried by the fixed no-request witness and destination; the one place the startup request's target is named is a single `verdi serve` stdout line at startup ("readiness: the startup context request targets spec/<name>; every other spec derives without a request").
- Task 3 (R-RR1-17, final fix wave): serve's per-request loader carries the startup-validated `PredecodedRequest` bundle, superseding Task 2's "re-read the request file fresh on every request" comment — a request file deleted or edited mid-run must not fail every spec's derivation; `ContextRequestPath` stays set so the context/verdict destination vector still names it.
- Task 4 (R-RR1-18, SI-210, final fix wave): a cached conflict report whose target content digest does not match the decoded spec bytes is the same epistemic state as a cache miss — the loader sets the ac-3 `ConflictUnavailable` witness (the context-conflict verb as destination) instead of returning an error, so an edited spec cannot blank the readiness page. Witnesses: a loader unit test (stale cached report → ConflictUnavailable, no error) and the Playwright order-independence pin (suite 50 edits the store, suite 81 still renders the readiness page with the unproven context concern).
- Task 1 (R-RR1-13) is implemented in the final fix wave: `resolveEventualScope` skips the three closure-gated feature sources when the closure transition's from-state is not forward-reachable from the current state (closed, superseded).
- Task 4b (R-RR1-19, final fix wave): R-RR1-13 extends to the policy sources — once no policy-evaluating transition is forward-reachable from the current state (closed, superseded), the conflict-mechanical, conflict-semantic, and exemption-ineffective sources derive nothing, and the section carries one disclosure naming why ("no policy-evaluating transition lies ahead of state <s>: policy-conflict findings were not derived as eventual debt"); a debt names the gate that consumes it (R-RR1-12) and a terminal state has none, while the report's continued existence makes the absence disclosed rather than silent (co-6).
- Task 5 (R-RR1-20): the gap witness derives the wall side with the design bridge wired (a scripted capabilities bridge returning a mutable draft-write view), so every hermetically reachable wall source appears; the golden header says the list is fixture-derived, not a vocabulary audit.
- Task 5 (R-RR1-21, SI-211): ac-5's "e2e harness fixture" is stood in for by the claim-wall Go fixture because the harness provisioner is `cmd/e2eharness`'s package main and unimportable from a hermetic Go test; the golden header records the substitution and the fixture-gated omissions (for example `shape/board` needs an open sticky); the harness fixture's own gap is disclosed-as-unproven and inherited by the post-design workbench lane.
- Task 2b (R-RR1-22): gate #1 was red only in `internal/sealedexec`, whose immutable public-consolidation witness pins by digest four sources Task 2 changed (`internal/policyconflict/service.go`, `cmd/verdi/context.go`, `cmd/verdi/context_conflict.go`, `cmd/verdi/context_constitution.go`). Per the SI-200 precedent (commit 4decd376) and spec-documents R-W4-11, the four reviewed successors are bound in `consolidationVerdiSuccessors` with historical and successor digests and byte-exact inverse edits; `checks.json` is untouched and no path is skipped. Any later lane whose write set includes a witness-pinned path runs the sealedexec consolidation tests in its GREEN set.
- Task 4c (R-RR1-23): gate #2 was red only in `49-readiness-pilot.spec.ts` under the full alphabetical run — its exact-array oracles read the shared harness store, which the board suites mutate before it runs, and the per-request derivation (ac-2) now shows those mutations where the deleted startup snapshot froze them. Suite 49 reads an isolated readiness-pilot fixture store: a control endpoint (`/readiness-pilot-fixture`, the unproven-board and spec-import fixture pattern) lazily provisions a second store through the same sequence the shared store uses and serves it with its own `verdi serve --context-request`; the oracles stay exact and unchanged; the shared serve keeps its request for suites 50 and 81.
- Correction wave (2026-09-21, after the independent review `docs/superpowers/reports/2026-09-21-readiness-recovery-independent-review.md`; fixes land on the wave-2 branch, which contains wave 1):
  - R-RRF-1 (SI-215, review R1): the eventual section gains an additive `unavailable` array for sources that could not be computed, and readiness derives a dedicated `review/eventual-derivation` concern from it, independent of the safe-action fallback.
  - R-RRF-2 (review R2): Task 1's `Preview: true` on the journey's feature-fold adapter is reversed to `false`. The eventual outcome-floor blocker names the debt the closure gate will read, and closure folds `source: ci` only (`cmd/verdi/closefeature.go`, verdi-evidence-model §Evidence records "Provenance classes": local is advisory); a local-only pass therefore never clears an eventual blocker. Advisory progress stays where it already is (the matrix preview); no separate advisory line is added in this wave.
  - R-RRF-3 (SI-216, review R3): an `expected` mismatch on a per-request load is the ConflictUnavailable posture, not an error; the warm-up keeps refusing it through an explicit option.
  - R-RRF-4 (SI-217, review R4): the startup-spec CLI/served divergence is a recorded spec conflict (ac-3/ac-4 vs verdi-surfaces §CLI), disclosed-as-unproven and owner-routed; the parity test's comment cites SI-217; no code change.
  - R-RRF-5 (review R5): an outcome-floor blocker's clearing condition offers the attestation path only when the criterion declares the attestation evidence kind (the fold reads that path only then); otherwise it names the passing-outcome-record route alone. ac-7's "names the attestation path or a passing outcome record" is read as "the effective route(s)", never a route the fold ignores.
