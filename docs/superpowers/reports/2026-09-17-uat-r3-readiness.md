# UAT round 1, wave 3 — lane W3-A (readiness pilot), ac-10

Sonnet implementer report. Authority: PLAN.md §7 I-128 (ratified 2026-09-17,
option (a)); spec/uat-round-1 ac-10; co-5; controller rulings R3-2, R3-3
(`docs/superpowers/reports/2026-09-17-uat-round-1-wave3-ledger.md` on
`agent/uat-round-1`).

## Status

Implemented and green. All four contract parts (A-D) landed as four small
TDD commits, each with a genuinely observed RED before GREEN. This report is commit E.

## Risk tier

3

## Base..Head

`74fda84a..a103c1cf`, then this report commit E (final SHA at end of message).

## Commits

- `e173c7f7` readinesspilot: a claimed open question is non-blocking/eventual (A)
- `d29a3221` cmd/verdi: populate readiness ClaimedQuestions from spec.Stubs (B)
- `32d52b12` workbench: wall shell mirrors ac-10 for spike-claimed open questions (C)
- `a103c1cf` e2e: cover ac-10 on the readiness pilot with a claimed second question (D)
- (E) this report

## Files changed

`internal/readinesspilot/{derive.go,schema.go,derive_test.go,schema_test.go}`;
`cmd/verdi/{readiness_snapshot.go,readiness_snapshot_integration_test.go}`;
`internal/workbench/{boardspecasd.go,boardspecasd_test.go}`;
`cmd/e2eharness/provision_board.go`; `e2e/tests/49-readiness-pilot.spec.ts`;
this report — 10 source files, all inside the write set (`git diff --stat
74fda84a..HEAD` confirms no other path touched). `readiness_test.go` (also
in-set) is untouched: it builds `Snapshot` fixtures directly, never through
`Input`/`Derive`, so the new `ShapeFacts` field never reaches it.

## Contract implemented

**A** — `ShapeFacts` gains `ClaimedQuestions []ClaimedQuestion` (`QuestionID`,
`StubSlugs`), validated in `Input.validate` (membership in `OpenQuestionIDs`,
no duplicate ids/slugs, slugs control-free/non-empty, non-nil). `deriveShape`
keeps a claimed question's id/area but sets `Blocking:false`,
`Timing:eventual`, a claimed-and-unresolved summary, witnesses = id + each
claiming slug. `concernIdentity` derives the `shape/question/*` family's
expected `Blocking` from `Timing` (mirroring `blockerConcern`), so both
shapes validate and the two cross-combinations are rejected. Unclaimed
questions, Attention inclusion, area aggregation, ordering: unchanged.

**B** — `readinessClaimedQuestions` maps `spec.Stubs` (`Spike && Resolves`) to
sorted `ClaimedQuestion`s in the one production builder. Integration test adds
a real feature-spec fixture (two questions, one claimed) proving through the
actual builder: claimed → non-blocking/eventual with the slug witness;
unclaimed → blocking/current; shape-proposal area driven only by the
unclaimed one — plus a second fixture (single, claimed question) proving the
area is no longer unproven because of it.

**C** — R3-2: the wall's parallel derivation (`boardspecasd.go`) is bound by
ac-10 too. `asdObjectFact` gains `ClaimedBySlugs`; a claimed question gets
`Blocking:false`, a summary naming every claiming slug, guidance that no wall
edit is needed since the spike answers it after acceptance; witnesses keep
"declared open question `<id>`" plus one per slug. Unclaimed unchanged.
`buildASDView` derives `ClaimedBySlugs` from `fm.Stubs`.

**D** — `provision_board.go`'s shared `spec/refi-decline-flow` fixture gains
`oq-2`, claimed by spike stub `refresh-window-spike` (oq-1 stays unclaimed).
In `49-readiness-pilot.spec.ts`: `ATTENTION_QUEUE` gains `shape/question/oq-2`
at its derived position (after `shape/provenance` — both non-blocking,
current sorts before eventual); "9 more items"→"10"; the violated-concern
index in "plain state labels" shifts `[2]`→`[3]`. New test asserts oq-2's
technical details show Blocking `false`/Timing `eventual` and oq-1 still
shows Blocking `true`/Timing `current`. `RAIL`/`COMPLETED_CHECKS` unchanged
(the new concern is non-blocking, so it drives neither).

## Explicit exclusions

Confirmed untouched: lint rules, the accept verb, `readinessrender.go`,
`boardspecrender.go`, templates, assets, `fixtures.ts`, and every file outside the write set.

## RED command and observed failure

- A (genuine RED: production files reverted to HEAD via a saved patch,
  re-applied only after observing the failure): `go test
  ./internal/readinesspilot/ -run TestDeriveShapeClaimedOpenQuestion` →
  `in.Shape.ClaimedQuestions undefined (type ShapeFacts has no field or
  method ClaimedQuestions)`; `undefined: ClaimedQuestion` `[build failed]`.
- B: `go test ./cmd/verdi/ -run TestReadinessSnapshotClaimedOpenQuestion -v`
  → both subtests failed: `claimed open questions must be non-nil` (field
  not yet wired).
- C: `go test ./internal/workbench/ -run TestDeriveASDShell` → `unknown
  field ClaimedBySlugs in struct literal of type asdObjectFact` (×3)
  `[build failed]`.
- D: first isolated run of `49-readiness-pilot.spec.ts` after the fixture
  edit failed 6/16 (diagnosed and fixed — see Residual risks).

## GREEN commands and results

- `go test -race ./internal/readinesspilot/...` → ok (1.6s)
- `go test -race ./internal/workbench/...` → ok (126.8s)
- `go test -race ./cmd/verdi/ -run 'Readiness'` → ok (13.8s, all Readiness* incl. new)
- `go build ./...`, `gofmt -l .`, `go vet ./...` → clean (repo-wide)
- `golangci-lint run ./internal/readinesspilot/... ./internal/workbench/... ./cmd/...` → 0 issues
- `cd e2e && VERDI_E2E_PORT_BASE=4490 npx playwright test tests/49-readiness-pilot.spec.ts`
  → **16/16 passed**; `git status --porcelain` after: only the 2 intended
  files, no recording artifacts (`test-results/` gitignored, untracked).
- Extra (shared fixture, not contracted): same session, `tests/30-board-
  scoping-canvas.spec.ts tests/49-readiness-pilot.spec.ts` → **21/21**,
  before and after my change.

## Reviewer verdict

(leave blank)

## Fix range and closure verdict

(leave blank)

## Residual risks

- oq-2 shifted attention order: (a) the semantic judge's content-derived
  `SEMANTIC_ID` moved (expected — digest is over candidate content; new
  value captured from a real harness run) and (b) the fixture's one
  CLI-destination concern was pushed out of the visible top three into the
  collapsed disclosure, breaking two tests that relied on it being visible
  unexpanded ("CLI fallback tokens copy…", "keyboard traversal reaches
  every cockpit landmark"). Both now expand the disclosure first (test
  setup only, no markup/CSS/JS change).
- Investigated, found NOT to manifest: `spec/refi-decline-flow` is a
  shared, cumulative-state e2e fixture (`workers:1`) — suite 30
  live-graduates two spike stubs claiming oq-1 via the browser before 49
  runs — so ac-10 might flip oq-1 itself non-blocking in full-suite order,
  contradicting "keep oq-1 unclaimed". Empirically this does not happen
  (21/21, before and after my change). Mechanism not determined; I have the
  negative-result evidence, not a causal explanation.
- Not run: full `make e2e`/`make verify` (60+ files) — out of contracted
  scope. Checked every file referencing `refi-decline-flow`/its open
  questions (33, 47, fixtures.ts: nothing relevant; 30, 32: covered above)
  and grepped all 26 files referencing `SHOWCASE.DESIGN_SPEC` for
  total-card-count assertions; found none. Spot check, not exhaustive.
- No spec ambiguity needed an invention-ledger entry: every choice
  (`ClaimedQuestion` shape, witness order, wording, e2e slug names) was
  mechanical, inside the contract's explicit text.

## Integration prerequisites

None from other lanes — base `74fda84a` is main-equivalent; this lane's
write set does not overlap W3-B (`agent/uat-r3-supersede`) or W3-C.
