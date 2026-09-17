# UAT round 1, wave 3 — lane W3-A (readiness pilot), ac-10

Sonnet implementer report. Authority: PLAN.md §7 I-128 (ratified 2026-09-17,
option (a)); spec/uat-round-1 ac-10; co-5; controller rulings R3-2, R3-3
(`docs/superpowers/reports/2026-09-17-uat-round-1-wave3-ledger.md` on
`agent/uat-round-1`).

## Status

Implemented and green. All four contract parts (A-D) landed as four small
TDD commits (report = commit E). Controller pre-review at 8c5fde5f found one
defect — three new production string literals spoke the bare "spike"
vocabulary word, failing `internal/specalign`'s TestVocabProseWitness
(L-M13a(6)) — fixed in one bounded commit (F, below); everything else in
that pre-review passed. See GREEN commands for the fix's own verification.

## Risk tier

3

## Base..Head

`74fda84a..043d4e41`, then this report update (final SHA at end of message).

## Commits

- `e173c7f7` readinesspilot: a claimed open question is non-blocking/eventual (A)
- `d29a3221` cmd/verdi: populate readiness ClaimedQuestions from spec.Stubs (B)
- `32d52b12` workbench: wall shell mirrors ac-10 for spike-claimed open questions (C)
- `a103c1cf` e2e: cover ac-10 on the readiness pilot with a claimed second question (D)
- `8c5fde5f` docs: W3-A readiness lane report (ac-10) (E)
- `043d4e41` fix: route "spike" through the model display chain (L-M13a(6)) (F, post-pre-review fix)
- (this update) report GREEN section refreshed

## Files changed

`internal/readinesspilot/{derive.go,schema.go,derive_test.go,schema_test.go}`;
`cmd/verdi/{readiness_snapshot.go,readiness_snapshot_integration_test.go}`;
`internal/workbench/{boardspecasd.go,boardspecasd_test.go}`;
`cmd/e2eharness/provision_board.go`; `e2e/tests/49-readiness-pilot.spec.ts`;
this report — 10 source files, all inside the write set, plus the fix
touching 4 of those same files again (`git diff --stat 74fda84a..HEAD`
confirms no other path touched). `readiness_test.go` (also in-set) is
untouched: it builds `Snapshot` fixtures directly, never through
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

**F (pre-review fix)** — routed "spike" through the model display chain at
both sites, following the one existing "spike" precedent
(`boardspecrender.go`'s oq-claims chip, `p.words.word("spike")`) rather than
marking: `asdShellInput`/`boardspecasd.go` gains `SpikeWord`, resolved via
`proj.words.word("spike")` in `buildASDView` (rendered text unchanged — no
rename is configured anywhere in this build). `readinesspilot.Input` gains a
caller-supplied `SpikeWord` (the package stays pure, no `internal/model`
import), populated by `cmd/verdi/readiness_snapshot.go` via the already-open
store's `cfg.Model.DisplayClass("spike")` (zero new imports); the
claimed-question summary reworded "a spike stub"→"spike stubs" (article-free,
still one sentence saying claimed+unresolved, per the original contract's "or
equivalent" allowance). The one new bare "spike" this necessarily introduces
— `SpikeWord`'s own validation error text — is a machinery diagnostic
(mirrors `schema.go`'s existing "closed"-in-a-diagnostic precedent), marked
`// vocab:identity`.

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

Pre-fix (parts A-E):
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

Post-fix (F, 043d4e41 — controller's exact required commands):
- `go test -count=1 ./internal/specalign/` → **ok, 127.0s**, every test
  green; `TestVocabProseWitness` and `TestGuideClaimsManifest_
  RowToWitnessBinding` individually re-confirmed passing (`-run` each, PASS
  both).
- `go test -race ./internal/readinesspilot/... ./internal/workbench/...`
  → both ok (1.6s / 127.9s).
- `gofmt -l .` → empty (repo-wide, after one `gofmt -w` for struct-field
  realignment the new `Input.SpikeWord` field triggered).
- `golangci-lint run ./internal/readinesspilot/... ./internal/workbench/... ./cmd/...` → 0 issues.
- `go build ./...`, `go vet ./...` → clean.
- `go test -race ./cmd/verdi/ -run 'Readiness'` → ok (13.9s) — re-checked
  since the fix also touches `readiness_snapshot.go`.
- Summary text changed (readinesspilot side: "a spike stub"→"spike
  stubs"), so per instruction: workbench race suite re-run (above) and
  `VERDI_E2E_PORT_BASE=4490 npx playwright test tests/49-readiness-pilot.spec.ts`
  → **16/16 passed**; `git status --porcelain` after: only the 5 intended
  fix files, no recording artifacts.

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

## Fix round 2 (Opus lane review)

Fixer: fresh Opus, distinct from the implementer and the reviewer. Scope:
the two controller-adjudicated findings F1 (proof gap on the wall-side
wiring) and F2 (singular grammar over a plural claim). F3 (the stale
`fixtures.ts` comment) and F4 (the readiness-side plural asymmetry) are
out of this lane by the dispatch.

### Fix range

`051925cb..f9a7d890` — three commits, no rebase/squash/amend, nothing
pushed.

- `75a0ee5a` workbench: agree the claimed-question sentence with the claim count (F2)
- `bbd4b7f6` workbench: prove the wall reads spike claims from stored frontmatter (F1a)
- `f9a7d890` e2e: read the wall's spike-claimed question in a browser (ac-10) (F1b)
- (this section) report

Files touched: `internal/workbench/boardspecasd.go`,
`internal/workbench/boardspecasd_test.go`,
`e2e/tests/50-design-workbench.spec.ts`, this report. Nothing else —
`cmd/e2eharness/provision_board.go` needed no fixture adjustment.

### F1(a) — the wiring test, and the proof that it bites

`TestBuildASDView_SpikeClaimedQuestions` and
`TestBuildASDView_NonSpikeStubNeverClaims` (both in
`internal/workbench/boardspecasd_test.go`) start at one real stored
`spec.md` — a draft feature wall on its design branch, two declared open
questions, `oq-2` claimed by two spike stubs declared **zeta-first**, one
plain coverage stub — load it through the production path
(`boardSpecServer.loadASD` → `loadBoard` → `artifact.DecodeSpec` →
`buildASDView` → `deriveASDShell`) and assert the rendered shell: the
claimed row's exact sentences, its `declared open question oq-2` +
sorted-slug witnesses, the unclaimed row's unchanged blocking
edit-or-remove guidance, and that the plain stub reaches no row.

The second case covers the `!st.Spike` guard, which no *valid* spec can
exercise: `artifact.Stub.Validate` refuses `resolves` without
`spike: true` (02 §Kind registry, DC-4) and `loadBoard` decodes through
`artifact.DecodeSpec`, so the guard sits **behind** a refused state. The
test asserts that refusal at the decode seam and then injects the refused
shape past it (`extras.fm.Stubs = append(..., artifact.Stub{... Resolves:
[oq-1]})`) straight into `buildASDView`, pinning the builder's own
fail-closed contract.

Mutation proof — each mutation applied alone to `boardspecasd.go`, run,
then reverted with `git checkout --` (tree verified clean afterwards):

1. Guard deleted (`if !st.Spike { continue }` removed from
   `buildASDView`):

   ```
   --- FAIL: TestBuildASDView_NonSpikeStubNeverClaims (0.34s)
       boardspecasd_test.go:634: oq-1 = {ID:shape/question/oq-1 Area:shape-proposal State:unproven Blocking:false Summary:Open question oq-1 is claimed by spike stub smuggled-plain-stub and remains unresolved: which decline reasons may be shown verbatim? Guidance:No wall edit is required to accept: the claiming spike stub answers it after acceptance. Witnesses:[declared open question oq-1 smuggled-plain-stub] Dest:#obj-oq-1 HumanReview:false}, want the blocking unclaimed row: a non-spike stub claims nothing
   FAIL
   FAIL	github.com/jyang234/verdi/internal/workbench	1.078s
   ```

2. Claim assignment deleted (`fact.ClaimedBySlugs = sorted` replaced by
   `_ = sorted`):

   ```
   --- FAIL: TestBuildASDView_SpikeClaimedQuestions (0.34s)
       boardspecasd_test.go:548: claimed question = {ID:shape/question/oq-2 Area:shape-proposal State:unproven Blocking:true Summary:Open question oq-2 is unresolved: what refresh-window SLA applies? Guidance:Resolve it on the wall: edit or remove oq-2, or graduate a decision that answers it. Witnesses:[declared open question oq-2] Dest:#obj-oq-2 HumanReview:false}, want unproven and non-blocking
   FAIL
   FAIL	github.com/jyang234/verdi/internal/workbench	1.062s
   ```

Both mutations are caught; the restored file is byte-identical to
`75a0ee5a`'s (`git status --porcelain` empty) and the suite is green.

### F1(b) — the browser case, and why it lives in suite 50

`e2e/tests/50-design-workbench.spec.ts`, describe block "shell and
posture": *"a spike-claimed open question reads non-blocking with
claim-aware guidance (ac-10)"*. Suite 50 is the shell-shape oracle and
already renders this wall's shell, so no navigation detour through 49 was
needed. No markup, CSS, or JS changed; `fixtures.ts` is untouched (the
`shape/question/oq-2` literal follows suite 49's own precedent — no
`SHOWCASE.*` marker exists for oq-2 or the fixture's spike slug).

The case expands `[data-testid="asd-more"] > summary` when present and
asserts oq-2's guidance verbatim, `Blocking` `false` in the technical
details, and the `declared open question oq-2` + `refresh-window-spike`
witnesses.

**The oq-1 half is deliberately not asserted**, and this is now measured,
not inferred. A temporary probe (a deliberately failing `toHaveText` on
oq-1's guidance, run as `tests/30 tests/50` and reverted immediately)
reported:

```
Received: "No wall edit is required to accept: the claiming spike stubs answer it after acceptance."
```

So in full-suite order suite 30's two live graduations **do** reach this
wall's frontmatter and oq-1 renders claimed-and-plural, while an isolated
run of suite 50 renders it unclaimed and blocking — the reviewer's
"the readiness page is a frozen startup snapshot but the wall is not"
determination, confirmed on the wall itself, and the mechanism the
implementer's report left undetermined. An oq-1 blocking assertion would
therefore be order-dependent, so the case asserts only that oq-1 still
has a row (SI-125 losslessness); both branches are pinned deterministically
in the Go tests above. Side effect worth recording: `make e2e` now renders
F2's plural sentence on a real wall.

### F2 — grammar, with one disclosed deviation

`deriveASDShell` now agrees the head noun and its verb with
`len(ClaimedBySlugs)`: "claimed by spike **stub** a …" / "the claiming
spike **stub answers** it" for one, "claimed by spike **stubs** a, b …" /
"the claiming spike **stubs answer** it" for two or more. Both sentences
are pinned exactly, for one and for two slugs, in the existing multi-slug
subtest (renamed "multiple claiming stubs are named, witnessed, and spoken
in the plural") and again through the real path in
`TestBuildASDView_SpikeClaimedQuestions`.

**Deviation from the fix contract (disclosed):** the dispatch asked for
"the plural word via the projection's words chain
(`DisplayClassPlural`)". That was not used, because in this sentence the
renameable class word is the *attributive modifier* and "stub" is the head
noun: `plural("spike")` here renders "spike**s** stubs", which is
ungrammatical and diverges from the sibling wording the controller
accepted under F4 (`readinesspilot/derive.go:259` — "claimed by " +
SpikeWord + " stubs") and from the wall's own stub-card label
(`boardspecrender.go:533/537` — `words.word(...) + " stub"`). The
`DisplayClassPlural` precedent at `boardspecrender.go:935-941` pluralizes
that word where it *is* the head noun ("claimed by 2 spikes"). The
rendered plural is "spike stubs", which is what the finding's example text
implies; only the mechanism differs. The class word still resolves through
the chain (`in.SpikeWord` ← `proj.words.word("spike")`), and the reasoning
is recorded in a comment at the site. `TestVocabProseWitness` stays green:
"stub"/"stubs"/"answer"/"answers" are not vocabulary words (the readiness
side already speaks "stubs" bare).

### Commands (fix round 2)

All run from `verdi/` in this worktree, foreground:

- `go test -race -count=1 ./internal/workbench/...` → **ok, 127.275s**, exit 0
- `go test -count=1 ./internal/specalign/ -run TestVocabProseWitness` → **ok, 1.293s**, exit 0
- `go test -count=1 ./cmd/e2eharness/` → **ok, 12.440s**, exit 0
- `cd e2e && VERDI_E2E_PORT_BASE=4490 npx playwright test tests/50-design-workbench.spec.ts`
  → **38 passed (2.6m)** (37 pre-existing + the new case)
- `cd e2e && VERDI_E2E_PORT_BASE=4490 npx playwright test tests/30-board-scoping-canvas.spec.ts tests/50-design-workbench.spec.ts`
  → **43 passed (3.2m)**
- `git status --porcelain` after both e2e runs → empty; `e2e/test-results/`
  holds only `.last-run.json` (gitignored) — no screenshot, trace, or video
- `go build ./...` → exit 0
- `gofmt -l .` → empty
- `go vet ./internal/workbench/...` → exit 0
- `golangci-lint run ./internal/workbench/...` → **0 issues**

### Residuals from this round

- Not run: `make verify` / full `make e2e` (out of the fix contract's
  command set). Suites other than 30 and 50 were not re-run; the only
  production change is the wall's plural branch, and a repo-wide grep
  found no other test, fixture, or golden carrying that prose.
- `e2e/tests/50-design-workbench.spec.ts` does not satisfy `prettier
  --check` — it already did not at `051925cb`, and prettier is in neither
  the Makefile nor `e2e/package.json`, so the new case follows the file's
  existing style rather than reformatting the file.
- F3 (`fixtures.ts`'s "DESIGN_SPEC's one open question" comment, now two)
  remains open for the controller at integration, as dispatched.

## Wave-gate follow-up

Wave 3 gate on integrated head `4d618bf7` (this lane merged at `637b209c`)
`make verify` failed one Playwright case:
`e2e/tests/27-board-legibility.spec.ts:283` "a new sticky lands at the
bottom of its type's lane", line 330 —
`expect(qTop).toBe(oqBottom < 0 ? 40 : oqBottom + 24)` → Expected
`1023.783`, Received `1023.78`. 313 other cases passed; suite 27 run alone
on the same head passed 8/8.

**Root cause (controller's determination, confirmed here):** this lane's
enlarged `DESIGN_SPEC` fixture (`oq-2` card + `refresh-window-spike` stub
card, contract part D) changed the board's obstacle map, moving an
earlier full-suite drop onto a fractional y-coordinate
(`boardlayout.ResolveDrop`'s distance-based collision resolution).
The server persists and renders the full-precision value
(`top:1023.783px`), but Chrome serializes CSS numbers to six significant
figures, so `el.style.top` reads back `"1023.78px"` — a pre-existing
exact-equality fragility on a browser-serialized float, newly exposed by
this lane's own fixture change (not a defect in `boardlayout` or the
fixture itself).

**Fix (test-only, this commit):** in that one test, the two coordinate
assertions (`qTop`/`cTop` against `oqBottom + 24`/`scratchBottom + 24`)
changed from `toBe` to `toBeCloseTo(expected, 1)` (tolerance < 0.05), each
with a one-line comment naming the six-significant-figure cause. No other
`toBe` on a parsed style coordinate exists in that test; `toHaveCSS`
lane-`x` assertions are untouched (fixed band positions, not
collision-derived, and a different assertion mechanism). No product code,
harness fixture, or other test file touched.

**Verified:**
- `cd e2e && VERDI_E2E_PORT_BASE=4490 npx playwright test tests/27-board-legibility.spec.ts`
  → **8/8 passed** (29.7s).
- `cd e2e && VERDI_E2E_PORT_BASE=4490 npx playwright test tests/20-board-drag-robustness.spec.ts tests/27-board-legibility.spec.ts`
  (cumulative-state path: 20 drops/drags real cards on `DESIGN_SPEC`
  before 27 runs) → **13/13 passed** (41.9s).
- `git status --porcelain` → only `e2e/tests/27-board-legibility.spec.ts`;
  no recording artifacts (`e2e/test-results/` gitignored, untracked).
