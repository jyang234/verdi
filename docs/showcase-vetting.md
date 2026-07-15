# Showcase vetting ledger

Per `docs/design/plans/2026-07-14-public-rollout-design.md` §4.2: every
artifact under `examples/showcase/` earns a row recording the three-column
vetting bar — **lint-clean** (`verdi lint` finds nothing on it),
**exemplary** (no dead prose links, no filler, production-quality writing),
**coherent+justified** (consistent with the LoanServ canon, earns its
place or is cut) — or an explicit, reasoned cut. This file grows across
the renovation tasks (Task 1.4 onward); it does not yet cover every file
in the tree — later tasks (1.5–1.8) add their own families' rows and close
out the full sweep at Task 1.8.

Legend: `Y` = holds; `Y*` = holds, with a note; `n/a` = column does not
apply to this artifact kind.

## Task 1.4 — `stale-decline` family (breadth feature)

| artifact | lint-clean | exemplary | coherent+justified | notes |
|---|---|---|---|---|
| `.verdi/specs/active/stale-decline/spec.md` | Y | Y | Y* | Rewritten body (Design notes, Boundaries, AC-1..4, AC rationale, OQ-1); added `problem`/`outcome`/`open_questions` (oq-1, decision owner platform-team, tied to adr/0003's live retry-policy debate) and per-AC `anchor`s. Adding `open_questions` flips `isNewClassSpec` (vl006.go), which flips `verdi matrix`'s and dex's fold discriminator (`class==feature && Problem!=nil`) from the grandfathered story-fold to the round-four feature-fold — see task-1.4-report.md §matrix-fold-change for the full trade-off writeup; disclosed as the single largest judgment call of this task. |
| `.verdi/specs/active/borrower-update-api/spec.md` | Y | Y | Y | Explainer paragraph moved under `## Provenance` per brief; `problem`/`outcome`/`ac-1` object-model text left byte-identical (golden-pinned by `cmd/verdi/matrix_test.go`'s `TestCmdMatrix_RoundFourStory_RendersStoryFold`); body enriched with a `## Problem`/`## AC-1` cross-reference to the mobile spike's PATCH-vs-PUT finding. |
| `.verdi/specs/active/borrower-update-mobile/spec.md` | Y | Y | Y | `problem`/`outcome` text rewritten to real prose (no external golden dependency, verified by repo-wide grep); AC-1 body explains the offline direct-write exemption's rationale; AC-2 body explains the optimistic-update contract. AC text values (ac-1/ac-2) left unchanged. |
| `.verdi/specs/active/borrower-update-mobile-spike/spec.md` | Y | Y | Y | Added `## Method`/`## Findings` sections: PUT-vs-PATCH tradeoff traced against the mobile client's actual offline-staleness failure mode; recommendation (PATCH) matches `spec/escrow-autopay#oq-1`'s real text, which the story's `resolves` edge targets. |
| `.verdi/specs/active/borrower-update-mobile/deviation-report.md` | Y | Y | Y* | Finding text rewritten from "direct writes for offline support" (redundant with the story's own `exempts` edge, which already documents that theme) to "mobile client retries diverged from spec" per the brief's explicit canon requirement; disposition `accepted-deviation`, note names the accepter (`platform-team`) and date (`2026-07-12`, matching the family's frozen stamps). This file is a plain, non-`layers.txt`-tracked file per the Task 1.2 orphan-avoidance precedent (its sibling `spec.md` is likewise untracked) — see `layers.txt`'s own comment. |
| `.verdi/attestations/jira-loan-1482/ac-2.md` | Y | Y | Y | Title and body rewritten from a one-line placeholder to a concrete staging-repro narrative matching ac-2's static+behavioral pairing. |
| `.verdi/obligations/borrower-update-api/ac-1--static.md` | Y | Y | Y | New. Static claim: the PUT route is registered, full-replace-shaped, direct-write (no outbox indirection on the canonical API path). |
| `.verdi/obligations/borrower-update-api/ac-1--behavioral.md` | Y | Y | Y | New. Behavioral claim: an end-to-end update lands and reads back durable, not just a 2xx status. |
| `.verdi/obligations/borrower-update-mobile/ac-1--static.md` | Y | Y | Y | New. Static claim: the mobile route's direct write stays scoped to the declared `exempts` edge; downstream consequences still enter the outbox. |
| `.verdi/obligations/borrower-update-mobile/ac-1--behavioral.md` | Y | Y | Y | New. Behavioral claim covers both the clean-request path and the accepted client-retry-loop deviation, cross-referencing the deviation report. |
| `.verdi/obligations/borrower-update-mobile/ac-2--behavioral.md` | Y | Y | Y | New. Behavioral claim: the client's own rendered view reflects the change, not merely server-side state. |
| `mutable/annotations/spec--stale-decline.jsonl` | n/a (mutable zone) | Y | Y | Sticky `a-...CCC` retyped `agent-task` → `question`, body changed to match `oq-1`'s text exactly (exceeds VL-017's requirement, since a grandfathered-turned-new-class spec's mutable-zone questions are still checked once new-class); sticky `a-...AAA` retyped `comment` → `agent-task` (its body rephrased as an instruction) to preserve the corpus's required annotation-type variety (`internal/corpus`'s `TestFixtureCorpus_MutableAndDerivedFilesDecode` requires ≥1 targeted, ≥1 board-only, ≥1 `agent-task` record) once the original `agent-task` sticky's role was reassigned to the open question. Target ref SHA re-pinned. |
| `mutable/boards/STORY-1482.json` | n/a (mutable zone) | Y | Y | Pin ref SHA re-pinned only; stickies/yarn content unchanged (matches the annotations file 1:1). |
| `derived/spec--stale-decline/<layer-2-head>/verdicts.json` | n/a (derived zone) | n/a | Y | Directory renamed to the new layer-2 head (`git mv`); `provenance.commit` field inside updated to match. Content (verdicts) unchanged. |
| `derived/spec--stale-decline/<layer-3-head>/verdicts.json` | n/a (derived zone) | n/a | Y | Directory renamed to the new layer-3 head (`git mv`); content unchanged. |
| `layers.txt` | n/a (manifest) | Y | Y | Comment-only change: documents why the five new obligation files are deliberately NOT `layers.txt`-tracked (would orphan them in the narrower `layers.txt`-only builds, since neither story's `spec.md` is itself tracked — same root cause and same fix as the pre-existing `borrower-update-mobile/deviation-report.md` exception this comment sits beside). |
| `.verdi/specs/archive/loan-refi-2023/{spec.md,board.json,rollup.json,deviation-report.md}` | Y | n/a (frozen, out of this task's content scope) | Y* | **Mechanical re-pin only** — these frozen, archived files cite the layer-2 head as their own `frozen`/`covers` pin (layer 3 is frozen at layer 2's commit); editing `stale-decline/spec.md` changed that commit, cascading here exactly as it cascaded through Tasks 1.2/1.3's own re-pins. No prose or structural content touched. Full vetting of this quartet's own prose is Task 1.5's job (its file list owns `.verdi/specs/archive/loan-refi-2023/*`). |

## Task 1.4 — mechanical re-pin fallout (not this task's content scope)

Editing `stale-decline/spec.md` (layer 2) cascaded its own commit head
through layer 3 (the `loan-refi-2023` archive quartet, frozen at layer 2's
head) and layer 4 (the folded dexoverlay content, frozen at layer 3's
head), exactly as Tasks 1.2/1.3's own re-pins did. Every file below had
only its `frozen`/`covers`/context 40-hex pin literal updated to the new
head — zero prose or structural changes. Content vetting for these
families belongs to their owning tasks (1.5 for `escrow-autopay` and the
supersession chains and the archived quartets; see each family's own
future row).

| artifact | lint-clean | exemplary | coherent+justified | notes |
|---|---|---|---|---|
| `.verdi/specs/active/escrow-autopay/spec.md` | Y | n/a (Task 1.5) | Y | Pin literal only. |
| `.verdi/specs/active/escrow-notify/spec.md` | Y | n/a (Task 1.5) | Y | Pin literal only. |
| `.verdi/specs/active/escrow-notify-v2/spec.md` | Y | n/a (Task 1.5) | Y | Pin literal only. |
| `.verdi/specs/active/rate-lock/spec.md` | Y | n/a (Task 1.5) | Y | Pin literal only. |
| `.verdi/specs/active/rate-lock-v2/spec.md` | Y | n/a (Task 1.5) | Y | Pin literal only. |
| `.verdi/attestations/escrow-autopay/ac-1.md` | Y | n/a (Task 1.5) | Y | Pin literal only. |
| `.verdi/specs/archive/refi-rate-check-2024/{spec.md,rollup.json,deviation-report.md}` | Y | n/a (Task 1.5) | Y | Pin literal only (frozen archive quartet). |
| `e2e/tests/04-verdict.spec.ts` | n/a (test, not a store artifact) | n/a | n/a | Hardcoded snapshot SHA literals re-pinned (same pattern as Task 1.3's own follow-up commit). |

## Coverage note

This ledger covers only the files this task (public-rollout-plan Task 1.4)
touched or created. `examples/showcase` carries many more files (the ADR
roster, `escrow-autopay` and its supersession chains, diagrams, the store
README, etc.) that later tasks (1.5–1.8) own and will add rows for here.
Task 1.8 ("lint-clean + full vetting closure for the committed tree")
is responsible for the final sweep proving every file in the tree has a
row.

## Task 1.5 — `escrow-autopay`, supersession chains, both archived quartets

Step 0 (a)–(d) controller adjudications, `escrow-autopay` renovation, the
`rate-lock`/`rate-lock-v2` and `escrow-notify`/`escrow-notify-v2`
supersession chains, and both archived quartets' prose. Full mapping/
adjudication writeup: `.superpowers/sdd/task-1.5-report.md`.

| artifact | lint-clean | exemplary | coherent+justified | notes |
|---|---|---|---|---|
| `.verdi/specs/active/escrow-autopay/spec.md` | Y | Y | Y* | Step 0(a): H1 fixed to match title. Full rewrite: real "autopay mandate + retry policy" narrative (was a leftover "borrower self-service update" placeholder theme). 3 ACs redesigned to a static+behavioral(+attestation) spread; `stubs:` replaced with `autopay-mandate-api`/`autopay-retry-policy`; `frozen.at` corrected to `2026-06-30` (canon). `co-1`/`dc-1`/`dc-2`/`oq-1` ids and text kept byte-unchanged (dc-2 is `borrower-update-mobile`'s `exempts` target; oq-1 is the spike's `resolves` target). `impacts`/`declares.boundaries` grown to include `payments-gw`, mirroring `stale-decline`'s own outbox-boundary pattern. Judgment call (Y*): `borrower-update-mobile` keeps one residual `implements` edge into this feature's `ac-2` — deliberately NOT rewired away — to preserve the pre-existing pending-supersession/`spec-stale` ladder-badge fixture (`e2e/tests/16-dex-v2.spec.ts`, `internal/dex/dexv2_test.go`'s `TestBuildV2_LadderBadges`), which has no other real carrier once escrow-autopay's stories move to stale-decline; `ac-2`'s new text ("sees a mandate change reflected in-session") was written to stay semantically compatible with that edge. |
| `.verdi/specs/active/borrower-update-api/spec.md` | Y | Y | Y | Step 0(b): `implements` edge rewired `spec/escrow-autopay#ac-1` → `spec/stale-decline#ac-2` (the "charge-API AC" — literal text match "charge API", and evidence-kind match static+behavioral). `## Provenance` section updated: still stub-matched (title-slug AND AC-set both match `stale-decline`'s new `borrower-update-api` stub). Own AC-1 text/evidence left byte-identical (golden-pinned elsewhere). |
| `.verdi/specs/active/borrower-update-mobile/spec.md` | Y | Y | Y* | Step 0(b): two `implements` edges rewired `spec/escrow-autopay#ac-1`/`#ac-2` → `spec/stale-decline#ac-1` and `#ac-3`; a THIRD edge, `spec/escrow-autopay#ac-2`, added back (judgment call, see escrow-autopay's own row) — net: 3 implements edges instead of 2. `exempts`/`loan-workflow#ac-1` edges unchanged. Opening blurb rewritten: still a "deviating fixture" (AC-set matches stale-decline's `borrower-update-mobile` stub `{ac-1,ac-3}`, title-slug doesn't — the title's comma produces a double-dash slug), now also explains the residual escrow-autopay edge and its pending-supersession role. |
| `.verdi/specs/active/borrower-update-mobile-spike/spec.md` | Y | Y | Y | Step 0(b) adjudication: `resolves` edge to `spec/escrow-autopay#oq-1` left UNCHANGED — considered and rejected moving it to `spec/stale-decline#oq-1`: `resolves` may only target an open-question fragment (`internal/artifact/spec.go`), and stale-decline's own `oq-1` body prose (Task 1.4) explicitly narrates it as unresolved, blocked on adr/0003 — claiming resolution would contradict that established narrative. escrow-autopay's `oq-1` is verbatim the PUT-vs-PATCH question this spike answers; no edit needed. |
| `.verdi/specs/active/stale-decline/spec.md` | Y | Y | Y | Step 0(c): all four AC `text` fields lifted to real team-written prose mirroring each AC's own body section (ids/`evidence` byte-unchanged — svcfix bindings key on them). Step 0(b): added `stubs:` (`borrower-update-api` → `{ac-2}`, `borrower-update-mobile` → `{ac-1,ac-3}`) mirroring exactly what the rewired stories implement — nothing invented beyond the fold's real inputs. Acceptance check PROVEN: see below. |
| `.verdi/specs/active/escrow-autopay-v2` (`mr/escrow-autopay-v2.spec.md`) | n/a (forge seed, never linted) | Y | Y | Rewritten to match escrow-autopay's new narrative/ACs; `supersession:` `carried`/`amended` structure kept (still amends `ac-2` only, preserving the pending-supersession fixture), content refreshed to the mandate-reflection theme. |
| `.verdi/attestations/escrow-autopay/ac-1.md` | Y | Y | Y | Title/body rewritten from "borrower can update their application" to the new ac-1 theme (autopay mandate creation); explains why it stays "present" even though the fold reads no-signal (no implementing story exists — an attestation alone was never sufficient). |
| `.verdi/specs/active/rate-lock/spec.md` | Y | Y | Y* | Step 2: added a "Why superseded" section (real reason: a fixed 30-day lock didn't survive a 45-day underwriting cycle). AC/CO text/evidence byte-unchanged (carried objects for VL-015). **Moved out of `layers.txt`** into its own dedicated, unchained fixturegit history (`internal/artifact/v2fixture_test.go`, alongside `loan-workflow`) — see layers.txt's own note and the report's VL-015 mechanics section. `frozen.commit` now cites that history's own real head. |
| `.verdi/specs/active/rate-lock-v2/spec.md` | Y | Y | Y | Step 2: real `supersession:` block added (`carried: [co-1]`, `ac-1` amended, `ac-2` removed) — VL-015-PROVEN, not eyeballed (see below). Body explains the amendment reasoning, cross-referencing v1's "Why superseded". Moved alongside v1 to the dedicated history. |
| `.verdi/specs/active/escrow-notify/spec.md` | Y | Y | Y* | Step 2: added a "Why superseded" section (real reason: a 24h window left support answering calls the system already had data for). Step 0(b) fallout: its structurally-required `implements` edge was `spec/rate-lock#ac-1` (dangled once rate-lock moved out of `layers.txt`) — retargeted to `spec/stale-decline#ac-4`, disclosed in-body as a structural (not narrative) link. |
| `.verdi/specs/active/escrow-notify-v2/spec.md` | Y | Y | Y | Step 2 adjudication: does NOT get a `supersession:` block — `class: story` specs may not carry one (`artifact.SpecFrontmatter.Validate`: feature-only field); a story-rung supersession is fully expressed by `supersedes` + the predecessor's terminal status flip (03 §rung 3). Same `implements` retarget as v1 (→ `spec/stale-decline#ac-4`). |
| `.verdi/specs/active/loan-workflow/spec.md` | Y | Y | Y* | Light prose enrichment (Problem/Outcome real narrative detail); added a note explaining why this pair's predecessor stays `status: accepted-pending-build` rather than `superseded` like `rate-lock` (deliberately covers a different amendment-ladder lifecycle point — a `supersession:` manifest existing ahead of the status flip — not touched, to avoid regressing tests asserting its current status). Frontmatter/AC/CO objects untouched. |
| `.verdi/specs/active/loan-workflow-v2/spec.md` | Y | Y | Y | Light prose enrichment; fixed a stale cross-reference ("a story on spec/escrow-autopay" → correctly names stale-decline as `borrower-update-mobile`'s primary feature, escrow-autopay as its residual one). |
| `.verdi/specs/active/legacy-cache-policy/spec.md` (layer 1) | Y | Y | Y | Thin one-line filler replaced with a real narrative (a 15-minute uncached-invalidation cache staleness gap escrow-autopay's `ac-2` guarantee can't tolerate), cross-referencing escrow-autopay. Still `class: component`, `status: superseded`, no frontmatter object fields — unchanged. |
| `.verdi/specs/active/store-layout-notes/spec.md` (layer 1) | Y | Y | Y | Same treatment: real narrative for the cache-invalidation design that supersedes `legacy-cache-policy`. |
| `.verdi/specs/archive/loan-refi-2023/spec.md` | Y | Y | Y | Body prose rewritten (real rollout narrative: manual rate-table re-keying was slow and error-prone; cross-references `refi-rate-check-2024` as its round-four successor). Frontmatter/AC-1 identity fields (id/text/evidence) byte-unchanged — no `implements` edge back into it depends on prose, but `refi-rate-check-2024#implements` depends on `ac-1`'s id existing, preserved. |
| `.verdi/specs/archive/loan-refi-2023/{board.json,rollup.json,deviation-report.md}` | Y | n/a (frozen, untouched structurally) | Y | Untouched except the mechanical SHA re-pin cascading from `stale-decline`'s Step 0(c) edit (layer 2 → layer 3). No structural or content change — respects Step 3's frozen-file boundary. |
| `.verdi/specs/archive/refi-rate-check-2024/spec.md` | Y | Y | Y | Body prose rewritten (real narrative: a published-table format change broke column-position parsing a year after `loan-refi-2023` shipped, letting two stale promotional rates through) — also fixes a stale `testdata/corpus` path reference left over from before the Task 1.1 relocation. AC-1 id/text/evidence byte-unchanged. |
| `.verdi/specs/archive/refi-rate-check-2024/{layout.json,rollup.json,deviation-report.md}` | Y | n/a (frozen, untouched structurally) | Y | Untouched except the mechanical SHA re-pin. |
| `.verdi/obligations/escrow-notify/ac-1--behavioral.md` | Y | Y | Y | New (Step 0(d)). Behavioral claim: a real injected escrow-shortfall event produces an actual borrower notification inside the 24h window. |
| `.verdi/obligations/escrow-notify-v2/ac-1--behavioral.md` | Y | Y | Y | New (Step 0(d)). Behavioral claim, tightened to the 1h window specifically (not merely "faster than 24h"). |
| `.verdi/obligations/refi-rate-check-2024/ac-1--static.md` | Y | Y | Y | New (Step 0(d)). Adjudicated: `internal/lint/vl020.go`'s `GrandfatherArchive` option is never set `true` by the real `verdi lint` CLI (`cmd/verdi/lint.go`) or by `internal/lint/harness_test.go`'s `buildLintRepo`, so archived-zone specs are NOT grandfathered for VL-020 — it genuinely fires on this closed archived story. Static claim: every priced field is resolved by column name against the current published-table schema, never a cached/hardcoded value. |
| `.verdi/obligations/refi-rate-check-2024/ac-1--behavioral.md` | Y | Y | Y | New (Step 0(d)). Behavioral claim: a quote re-priced after the table changes matches the new table, not a stale cached rate. |
| `internal/lint/harness_test.go` (`knownCorpusBaselineFindings`) | n/a (test infra) | n/a | n/a | Step 0(d): map emptied to `map[[3]string]bool{}` — the corpus now genuinely produces zero VL-020 findings on its own (proven: `internal/lint` package green with the empty map). Mechanism (map + `filterKnownBaseline`) kept per the brief ("delete the mechanism only if nothing else uses it; else leave an empty list") since it remains the shared entry point every `buildLintRepo`/`buildV2FixtureCorpusRepo`-based test routes through. |
| `layers.txt` | n/a (manifest) | Y | Y | Restructured: `rate-lock`/`rate-lock-v2` removed (moved to the dedicated `v2fixture_test.go` history); the four new obligations added to layer 4 (same commit as their `verifies` targets — no orphaning, VL-019). Extensive new comments documenting the VL-015 mechanics and the rationale for each placement. |
| `internal/artifact/v2fixture_test.go` | n/a (test infra) | n/a | n/a | Extended the loan-workflow pattern with two more layers (`goldenShaC`/`goldenShaD`) for `rate-lock`'s own draft→frozen supersession history, mirroring the exact mechanism already proven for `loan-workflow`/`loan-workflow-v2`. `TestV2Corpus_SpecsDecode` grown to decode `rate-lock`/`rate-lock-v2` too. |
| `internal/corpus/corpus_test.go` | n/a (test infra) | n/a | n/a | `goldenHeads` re-pinned (layers 1-4, procedure below); `goldenHeadsV2` grown with `goldenShaC`/`goldenShaD`; `derivedDirs` table and `decodeCommittedFile`'s dispatch switch grown with a `.verdi/obligations/` case (previously unexercised by any layered obligation). |

### Acceptance check — PROVEN, not eyeballed

`verdi matrix spec/stale-decline` (`cmd/verdi/matrix_test.go`'s
`TestCmdMatrix_Golden`, which drives the real `cmdMatrix` code path
against a fixturegit-real corpus) now renders:

```
feature: spec/stale-decline
status: accepted-pending-build

AC    STATUS   EVIDENCE                             IMPLEMENTING STORIES                                    TEXT
ac-1  pending  attestation:absent; static:pass      spec/borrower-update-mobile                             every branch that classifies a decline as stale routes its consequence through the outbox — no direct call to notification-svc or payments-gw
ac-2  pending  attestation:absent; behavioral:pass  spec/borrower-update-api                                loansvc retries the charge through the outbox exactly once per stale decline
ac-3  pending  attestation:absent                   spec/borrower-update-mobile                             a partial refund against a stale-declined loan still reconciles correctly before any retried charge is issued
ac-4  pending  attestation:absent                   spec/escrow-notify-v2, spec/escrow-notify [superseded]  the stale-decline rate for the affected cohort is checked against the pre-change baseline seven days post-deploy

stubs: acceptance-time plan; current mapping computed below
STUB                    DECLARED ACS  LIVE STORIES                 RECONCILIATION
borrower-update-api     ac-2          spec/borrower-update-api     unreconciled
borrower-update-mobile  ac-1, ac-3    spec/borrower-update-mobile  unreconciled

feature.violated: false
stub_reconciliation.blocked: true
```

— NOT all-no-signal (the acceptance check), and richer than the BEFORE
state (Task 1.4 landed it as uniform `no-signal` / `attestation:absent`
with an empty stub table). VL-015 is similarly proven, not eyeballed:
`go test ./internal/lint/... -run TestClean_CorpusLintsGreen` and the
whole `internal/lint` package (which chains `rate-lock-v2`'s real
`supersession:` block through the engine) are green.

Not this task's file scope, disclosed as fallout requiring adjudication:
`cmd/verdi/featurematrix_test.go`'s `TestCmdMatrix_FeatureRef_Golden`/
`TestCmdMatrix_FeatureRef_SupersededStoryRendersTerminalMarker` (escrow-
autopay's own stub-table/fold goldens), `internal/dex/dexv2_test.go`'s
`TestBuildV2_FeatureLens`, `internal/workbench/scopingcanvasrender_test.go`'s
`TestScopingCanvas_CommittedFixturesRenderStubCards` (stub slugs), and the
full SHA re-pin propagation across `internal/corpus`, `internal/dex`,
`internal/workbench`, `internal/index`, and `internal/lint`'s many
synthetic fixtures that reuse the corpus's real golden heads as
plausible-looking commit values — all updated; see
`.superpowers/sdd/task-1.5-report.md` for the full inventory.

## Coverage note (Task 1.5)

This section covers the files Task 1.5 touched or created. `examples/showcase`
still carries files Task 1.5 did not touch (the ADR roster, diagrams, the
store README, `payoff-quote-portal`, etc.) that later tasks (1.6–1.8) own.
Task 1.8 remains responsible for the final sweep proving every file in the
tree has a row.

## Task 1.6 — ADR roster + decision scars

| artifact | lint-clean | exemplary | coherent+justified | notes |
|---|---|---|---|---|
| `.verdi/adr/0001-outbox-events.md` | Y | Y | Y | Rewritten from a one-line fixture stub to real Context/Decision/Consequences prose: the synchronous dual-write pattern, its two structural failure modes, superseded 2025-11-05 by `adr/0002` citing the 2025-10 dual-write incident. `decided`/`frozen.at` moved to 2025-08-20 (canon acceptance date; previously an arbitrary 2025-03-01). |
| `.verdi/adr/0002-outbox-events.md` | Y | Y | Y | Rewritten with the same causal mechanics `spec/stale-decline`'s own Design-notes section already narrates (mid-request failover, ambiguous delivered state, retry-driven duplicate notices) — cross-checked against that spec's prose for agreement, not independently invented. Consequences section forward-references both scars 0002 now carries: the shared-retry-budget dispute (`adr/0003`) and the unfiltered-payload exposure (`adr/0004`). `decided`/`frozen.at` moved to 2025-11-05 (canon supersession date). |
| `.verdi/adr/0003-retry-policy.md` | Y | Y | Y | Rewritten as a live, unresolved debate (still `status: proposed`, no `decided`/`frozen` stamp): two named shapes (per-class budget vs. elastic shared budget), explicitly the same debate `spec/stale-decline#oq-1` defers to and `conflict/stale-decline-incident` is filed against — all three artifacts now cross-reference each other and agree. |
| `.verdi/adr/0004-pii-redaction-at-ingest.md` | Y | Y | Y | New. Accepted 2026-03-12 (canon). Cites `conflict/pii-outbox-leak` as the incident that motivated it and `spec/escrow-autopay#dc-3`/`#dc-4` as its one active, audited exemption. Sized so `verdi audit`'s exemption count for this ADR is 2 (< default threshold 3) — proven by a real `verdi audit` run against a provisioned scratch store (see task-1.6-report.md). |
| `.verdi/adr/0005-event-schema-registry.md` | Y | Y | Y | New. Accepted 2026-04-02 (canon). Names all 7 canon services, motivating the topology diagram Task 1.7 expands to match; `depends-on adr/0002` and folds `adr/0004`'s redacted-field set into the registered schema. |
| `.verdi/conflicts/pii-outbox-leak.md` (renamed from `legacy-cache-dispute.md`) | Y | Y | Y | Repurposed: was "legacy cache policy disputed / superseded by store-layout-notes" (unrelated to PII, and store-layout-notes's own supersession of legacy-cache-policy needs no conflict record to be coherent — 03 §Challenging closed decisions doesn't require one per supersession). Now: filed 2026-02-18 against `adr/0002-outbox-events` over unredacted PII in the outbox log, superseded 2026-03-12 the same day `adr/0004` was accepted — the "one superseded conflict, settled by 0004" scar (ledger R4-I-50 / plan ledger L-C). Same layer (2), same `frozen.commit` convention (cites layer 1's head); rename propagated to `layers.txt`, `internal/index/golden_test.go` (`wantCommittedRefs` + the `challenged-by` backlink case), `internal/specalign`'s repo-hygiene tracked-file check. |
| `.verdi/conflicts/stale-decline-incident.md` | Y | Y | Y | Enriched, kept `open` (no frozen stamp — unresolved). Now explicitly the retry-budget-exhaustion incident (2026-02-14) that `adr/0003-retry-policy` exists to settle: added a second `challenges` link (`adr/0002-outbox-events`, preserving the original `challenges: spec/stale-decline` so the existing `challenged-by` golden backlink still holds unchanged) and an `annotates` link to `adr/0003-retry-policy`. Cross-checked against `spec/stale-decline#oq-1`'s own prose, which already named this exact ADR as the resolution path — agreement, not invention. |
| `.verdi/conflicts/false-alarm.md` | Y | Y | Y | Enriched with a concrete dismissal reason (a coincidental same-day unrelated charge misread as a duplicate; outbox log showed exactly one retry) replacing the previous one-line "the evidence held." Links/status/frozen stamp unchanged. |
| `.verdi/waivers/jira-loan-1482/ac-3.md` | Y | Y | Y | Prose only — `status: expired`, `expiry: 2026-06-01`, `reason` byte-identical to Task 1.4's grant (kept consistent with `spec/stale-decline`'s AC-rationale, which names this waiver by id). Added product-lead sign-off narration and the fixture-landing date that let it expire. |
| `.verdi/waivers/jira-loan-1482/ac-4.md` | Y | Y | Y | Prose only — `status: active`, no expiry, `reason` byte-identical to Task 1.4's grant. Explains why no expiry is set (mechanism-blocked on OQ-2, not effort-blocked) and names product-lead's sign-off. |
| `.verdi/reaffirmations/jira-loan-1483/ac-1.md` | Y | Y | Y | Re-dated `frozen.at` 2026-07-14 → 2026-07-12 (the family's own frozen date — 2026-07-14 was a stray non-canon date, likely leaked from the authoring session's wall-clock rather than a LoanServ narrative date). `object`/`hash` (structural, tied to the real `loan-workflow-v2` supersession) left untouched. Added a paragraph tying the tightened 30-second visibility threshold to `adr/0002`'s own publisher poll/flush interval — "tied to 0002" per the brief. |
| `.verdi/specs/active/escrow-autopay/spec.md` (decisions dc-3/dc-4 only; ac/co/dc-1/dc-2 untouched) | Y | Y | Y | Added `dc-3`/`dc-4`, each an `exempts` edge against `adr/0004-pii-redaction-at-ingest`, naming the legacy loan-import bridge job CO-1 already references and its Q3 2026 remediation, product-lead sign-off 2026-03-12. Hangs off CO-1's pre-existing "must not touch the legacy schema" constraint rather than being bolted on — CO-1's own body section was expanded first to explain what the legacy schema/bridge job is, then DC-3/DC-4 build on that explanation. Frozen re-pinned (layer-3 head changed upstream). |
| `layers.txt` | n/a (manifest) | Y | Y | Renamed the `legacy-cache-dispute.md` → `pii-outbox-leak.md` entry (same layer 2); added layer 5 (`adr/0004-pii-redaction-at-ingest.md`, `adr/0005-event-schema-registry.md`), both citing layer 4's head per the established "cite the preceding layer's resulting head" convention (a file cannot self-reference its own about-to-be-built commit). |

### Re-pin — old → new heads (Task 1.6)

Editing layer 1 (`adr/0003-retry-policy.md` rewrite) and layer 2 (`adr/0001`, `adr/0002`, all three conflicts, both waivers) cascaded the entire chain, plus a new layer 5:

| layer | old head | new head |
|---|---|---|
| 1 | `66588948af8b36c02c8fb8f423645afa0a58dbe4` | `9f5621543d6e5158ad3230a7febc83754f2be3dd` |
| 2 | `d70cb19fa17ced67d27b8f9a63b47b3bf280b7d1` | `2350631724b1e69ccdd84da40686a8f079955dc4` |
| 3 | `faf8d8c412c9df35b5a445146a5fe0e8309caa71` | `74c957aed504671bd4fc4ceb30907d2f4813e9b7` |
| 4 | `a02dd7dd74cf087aa5ce91a2b49447147dc2132e` | `09ed3760a09cc1ec9b0c5ccf78cebc3b1ca93fa5` |
| 5 (new; final) | — | `82d1e540854dbafe0322fc3a4ea61de53ff54c83` |

Procedure: iterative `TestFixtureRepo_MatchesGoldenSHAs` re-runs (via a throwaway head-printing test), one layer at a time low→high, blanket-propagating each stabilized old→new head repo-wide (mechanical, verified via `TestFixtureCorpus_PinsNameGoldenCommits`'s whole-tree pin scan) before rebuilding for the next layer — same discipline as Task 1.5's own re-pin. `derived/spec--stale-decline/<sha>/` directories `git mv`'d to the new layer-2/layer-3 head names. Full inventory, golden adjudications, and the `verdi audit` scratch-store proof are in `.superpowers/sdd/task-1.6-report.md`.

### Coverage note (Task 1.6)

Also fixed as fallout, not this task's content scope: `cmd/verdi/featurematrix_test.go`'s `autopay-retry-policy` stub golden (`ac-3` → `ac-2, ac-3`) was already stale at the Task 1.5 baseline (reproduced against clean `48bcf31`, unrelated to any Task 1.6 edit — the frontmatter `stubs:` block and AC-3's own "also plans against ac-2" prose already declared `[ac-2, ac-3]`); corrected as a pre-existing-defect fix since it blocked this task's own `make verify` exit criterion. `internal/dex/build_test.go`'s short-SHA literal (`6658894` → `89f9926`) re-pinned alongside the rest of the cascade.

`examples/showcase` still carries files Task 1.6 did not touch (diagrams, the store README, `payoff-quote-portal`) that Task 1.7/1.8 own. Task 1.8 remains responsible for the final sweep.

## Task 1.7 — diagrams, link-coverage audit, corpus README

| artifact | lint-clean | exemplary | coherent+justified | notes |
|---|---|---|---|---|
| `.verdi/diagrams/loansvc-topology.mermaid` | Y | Y | Y | Expanded from a 2-edge, 3-service sketch to the full 7 canon services (`loansvc`, `notification-svc`, `escrow-svc`, `rate-engine`, `doc-vault`, `borrower-portal`, `payments-gw`), retitled from "LoanSvc topology (fixture)" to "LoanServ service topology" (production-quality title per the vetting bar, matching the register Task 1.4-1.6 already applied elsewhere). Edges: `loansvc->notification-svc`/`loansvc->payments-gw` agree byte-for-byte with the `declares.boundaries` blocks in `spec/stale-decline` and `spec/escrow-autopay`; `loansvc->escrow-svc` is grounded in `spec/store-layout-notes`'s own prose ("every outbox event that changes an escrow account's state also carries an invalidation key"); the three remaining edges (`rate-engine`, `doc-vault`, `borrower-portal`) follow `adr/0005-event-schema-registry`'s Context section, which names all six pre-`payments-gw` services as outbox publishers/consumers without itemizing individual edges — the topology, an authored-living (class-absent) diagram, is not machine-verified against `declares.boundaries` (VL-021/the extractor apply only to `class: proposal`), so drawing the remaining three as `loansvc -> X` hub edges is this task's own informed completion of what `adr/0005` already asserts exists, not an invention of new behavior. Each edge carries an `events: <class> v1` label, honoring `adr/0005`'s own claim ("each edge annotated with the event class and schema version it depends on") — event class names (`decline-notice`, `charge-retry`, `escrow-state-changed`, `rate-lock-status`, `document-generated`, `account-view-updated`) are newly coined for this task, chosen to agree with each edge's already-documented narrative (decline-notice/charge-retry from `spec/stale-decline`'s own Boundaries section) rather than invented independently. Added a `depends-on` link to `adr/0005-event-schema-registry` alongside the pre-existing `derived-from` link to `spec/store-layout-notes` (kept unchanged). |
| `.verdi/diagrams/borrower-journey.mermaid` | Y | Y | Y | New. `sequenceDiagram` (not flowchart/graph vocabulary flowmap's extractor could ever claim) walking a borrower's escrow-autopay enrollment and a scheduled charge that fails and retries, across `borrower-portal`, `loansvc`, `escrow-svc`, `payments-gw`, `notification-svc`. Illustrative BY CLASS per `spec/illustrative-class` dc-2 (no `class: proposal`, so the shared render seam badges it "illustrative · not deterministically verifiable" automatically — verified against the real dex build, see below) — deliberately carries no `links:` block: an earlier draft added a `depends-on: spec/escrow-autopay` link, which is real in the full committed tree but dangling inside `internal/lint`'s narrower `buildLintRepo` harness (that harness builds strictly from `layers.txt`, and `spec/escrow-autopay` is v2-fixture-overlay content outside `layers.txt` — see `layers.txt`'s own note on that split); dropped rather than widening the harness, since the link was supplementary, not load-bearing for the narrative. Joins `layers.txt` layer 1 (never-frozen, alongside `loansvc-topology.mermaid`). |
| `examples/showcase/README.md` | n/a (not lint-walked; lives outside `.verdi/`) | Y | Y | Full rewrite from the old `testdata/corpus` README (coverage-inventory register) to the public store guide: LoanServ narrative in three paragraphs, the two zones (committed tree vs. VL-004-driven design branches — described mechanically, without naming the not-yet-provisioned draft feature per this task's own instruction), how `layers.txt` builds deterministic history (five layers, described accurately against the real file), the corrected eleven-type link map (see below), a diagram-tiers section, and "trace these threads" (AC->obligation->attestation; the ADR-0001->0002 supersession; the two deliberate scars — `spec/borrower-update-mobile`'s spec-stale badge and `conflict/pii-outbox-leak`'s L-C supersession). Absorbed `OVERLAY-NOTES.md`'s substantive content (the dexoverlay fold's own rationale is now implicit in the "how layers.txt builds this history" section's layer-4 description; the file-by-file inventory OVERLAY-NOTES carried is superseded by this store's own vetting ledger, so it was not reproduced verbatim) and that parking file was deleted (`git rm`). |
| `OVERLAY-NOTES.md` | — | — | — | Deleted (`git rm`) per this task's brief — its substance is absorbed into the new README (see above row) and it was a scaffolding file since Task 1.2 introduced it. |
| `.verdi/adr/0002-outbox-events.md` (one sentence only) | Y | Y | Y | Folded-in cosmetic per this task's brief: "This stopped being theoretical on 2025-10-xx" -> "This stopped being theoretical in October 2025" (a stray placeholder-shaped date reads worse than the plain month it always meant). No other content on this file touched. |
| `layers.txt` | n/a (manifest) | Y | Y | Header comment fixed: "relative to this directory (testdata/corpus/)" -> "relative to this directory (examples/showcase/)" (stale since the Task 1.1 relocation). Added `1 .verdi/diagrams/borrower-journey.mermaid` to layer 1. |
| `derived/spec--stale-decline/<layer-2-head>/verdicts.json` | n/a (derived zone) | n/a | Y | Directory `git mv`'d twice across this task's two content iterations (topology expansion, then the borrower-journey links removal) to track layer 2's cascading head; `provenance.commit` fields inside untouched (content unchanged, only the containing directory's name, matching Task 1.4/1.6 precedent). |
| `derived/spec--stale-decline/<layer-3-head>/verdicts.json` | n/a (derived zone) | n/a | Y | Same `git mv` treatment for layer 3's cascading head. |

### Nine-link-type audit — corrected to eleven

The brief's own framing ("nine link types... `implements`, `story`, `impacts`, `supersedes`, `resolves`, `verifies`, `evidence-for` + the remaining two") does not match 02 §Link taxonomy's actual table, which the spec itself titles the closed vocabulary: it lists **eleven** distinct `Type` values (`implements`, `resolves`, `supersedes`, `exempts`, `verifies`, `derived-from`, `annotates`, `depends-on`, `story`, `impacts`, `challenges`). `internal/artifact/common.go`'s `LinkType` enum agrees exactly — its own doc comment on `Valid()` reads "one of the eleven known link types" — and `evidence-for` is not among them: it is a `verdi.bindings.yaml` field name (a producer->AC join documented in 03 §evidence-model and consumed by VL-003's binding-resolution check), never a `links:` edge type. The corpus's own pre-existing `README.md` (now rewritten) independently carried the same "nine" undercount in its coverage-inventory comment, so this was a standing, silent error in two places, not one this task invented — corrected in both by this task, per three-valued honesty (a wrong claim on the record is worse than an absent one). A grep of every `type: <x>` occurrence plus the two scalar-field forms (`story:`, `impacts:`) across `examples/showcase/` found at least one natural, non-degenerate exemplar for **all eleven** types already committed in the tree (table in the new README, reproduced in the report) — no type required a bolt-on addition.

### Re-pin — old -> new heads (Task 1.7)

Editing layer 1 (`loansvc-topology.mermaid`'s content, `borrower-journey.mermaid`'s addition) cascaded the entire chain across two iterations (the second triggered by dropping `borrower-journey.mermaid`'s `links:` block after the lint-harness finding above):

| layer | Task 1.6 head | intermediate head (topology expansion only) | final head (Task 1.7) |
|---|---|---|---|
| 1 | `89f9926e9739b97e23eb52efb16206d0ff10ff4f` | `a3d1fd798c9a28416781a499c0b538a394b8f708` | `9f5621543d6e5158ad3230a7febc83754f2be3dd` |
| 2 | `4e5ef0b6b00f23c9faf7a9e4857255b7be5bea03` | `29daa2fa6a25bc5435047a6ab1874b146e12fcaa` | `2350631724b1e69ccdd84da40686a8f079955dc4` |
| 3 | `30c5ff945413930879823be6db0ccc07d5abd6b9` | `acc28f13b90f17d52c4bbcdb7eec448c98d1ce98` | `74c957aed504671bd4fc4ceb30907d2f4813e9b7` |
| 4 | `024b516641e9e229b0a156c636af59cda7c297d9` | `2943d6dfd34237413a2b9bc18099720f086b6bf1` | `09ed3760a09cc1ec9b0c5ccf78cebc3b1ca93fa5` |
| 5 (final) | `82d1e540854dbafe0322fc3a4ea61de53ff54c83` | `4af55c69be10835d3cd560498b8f484bc03c21c2` | `fd98ad21bfd79ccc2566f5b1f1cd2e48a77eca5e` |

Procedure: identical to Tasks 1.5/1.6's iterative `TestFixtureRepo_MatchesGoldenSHAs` low->high re-run, repeated for a THIRD pass beyond the "intermediate" column shown (the second pass, dropping `borrower-journey.mermaid`'s `links:` block per the lint-harness finding above; a third pass rewrote one confusing sequence-diagram line in the same file for narrative clarity — both content edits, not golden-SHA drift, folded into the single "final head" column above since only the end state matters to a reader). Every pass mechanical, blanket-propagated repo-wide via the old->new SHA pairs — verified zero remaining occurrences of every superseded SHA before moving to the next. `derived/spec--stale-decline/` directories `git mv`'d to the final heads only (intermediate heads were never committed).

### Fallout adjudicated (not this task's content scope, required for `make verify`)

- `internal/corpus/corpus_test.go`: `goldenHeads` (all 5 layers) and the two `derived/spec--stale-decline/<sha>/` subtest path literals re-pinned.
- `internal/index/golden_test.go`: added `diagram/borrower-journey` to `wantCommittedRefs` (`TestGolden_EveryCorpusArtifactIndexed`'s exact-`Len()` check would otherwise fail on the new artifact).
- `internal/dex/build_test.go`: the mermaid-body substring assertion (`"loansvc --&gt; notification-svc"`) updated to the new labeled-edge form; the frozen-temporal-banner short-SHA literal (`89f9926` -> `9f56215`, `spec/stale-decline`'s own `frozen.commit` short form) re-pinned — both real content changes, not golden-SHA drift.
- `internal/dex/testfixture_test.go`, `internal/workbench/testfixture_test.go`: `corpusGoldenHeads` (layers 1-4) re-pinned to the final heads.
- `e2e/tests/05-dex.spec.ts`: comment-only accuracy fix describing the topology's new body (no assertion changed; the test only checks SVG visibility).
- Global mechanical SHA propagation across every other file in the tree that reused a corpus golden head as a same-looking value in a self-contained fixture (`internal/lint/*_test.go`, `internal/workbench/*_test.go` unit fixtures) — done via the same repo-wide sed pass as the functionally-required updates above, matching Task 1.5/1.6's own precedent of keeping these in sync for consistency even where not strictly required for the fixture's own test to pass.
- `go build ./...`, `go vet ./...`, `go test ./...` (whole module), `make spec-align`, `make fixture`, `make lint-store` all re-run clean after the cascade; full `make verify` output is in `.superpowers/sdd/task-1.7-report.md`.

`examples/showcase` now has a vetting row for every file Tasks 1.4-1.7 touched or created. Task 1.8 (out of this task's scope) remains responsible for the final whole-tree sweep confirming every remaining untouched file (mostly `mutable/`/`derived/` fixtures and the two v2-supersession-chain pairs already vetted incidentally by Task 1.5) also has a row.

## Task 2.1 — `payoff-quote-portal` live-draft branch (provisioned, not committed)

Per design §4.1, the showcase design branches are "named, documented, and
vetted as part of the showcase." These artifacts are authored by
`cmd/e2eharness/provision_showcase_draft.go` onto `design/payoff-quote-portal`
(a draft never lands on main, VL-004), so they live in the harness's scratch
store, not the committed tree — but they render publicly on the workbench's
`/b/` draft-boards surface and carry the same three-column bar. Lint-clean is
proven by running `verdi lint` from the provisioned managed worktree: zero
findings whose path is a `payoff-quote-portal` artifact (the only findings on
that single-commit scratch store are the pre-existing corpus-wide VL-009/
VL-003/VL-015 pin-resolution noise the golden fixturegit history resolves —
the Task 3.1 single-commit caveat, unrelated to this content).

| artifact | lint-clean | exemplary | coherent+justified | notes |
|---|---|---|---|---|
| `spec/payoff-quote-portal` (draft feature spec) | Y | Y | Y | Class feature, status draft, `story: jira:LOAN-1533`, owner `servicing-experience`; 2 ACs across behavioral/attestation/static (VL-006/VL-020); `impacts`/`declares.boundaries` over canon services borrower-portal/loansvc/doc-vault; `open_questions: [oq-1]` carries the rate-lock policy question — VL-017's carried path, byte-identical to the open question sticky. Production-length problem/outcome + body sections. |
| `diagram/payoff-quote-flow` (proposal tier) | Y | Y | Y | `class: proposal`, `derived_from` pins the real corpus base `diagram/loansvc-topology@<commit>` (VL-021 resolves it); `source_digest` is the REAL `diagrambase.CanonicalGraphDigest` of that base, `digest` the base body's content sha256 — both well-formed sha256 (VL-021 format check). Flowchart of the borrower-portal→loansvc→doc-vault payoff-quote flow. |
| `data/mutable/annotations/spec--payoff-quote-portal.jsonl` (3 stickies) | Y | Y | Y | Seeded into the branch's pre-cut managed worktree so the authoring board renders its stickies. One `question`/`resolved` (VL-017's resolved-in-place path), one `question`/`open` whose body is oq-1's carried text (VL-017's carried path — the twin the spec object formalizes), one `agent-task` working note. Target refs pinned to the fixture commit; author handles follow the corpus's first-name convention. |

### VL-017 both-paths proof

VL-017 is exemplary on the payoff draft precisely because both legal
dispositions appear on one wall: the **resolved** annotation
(`status: resolved`) is settled in place and skipped by the rule; the
**carried** annotation (`status: open`, body byte-identical to the declared
`open_questions` entry oq-1) is matched by `carriedAsOpenQuestion` and passes
clean. Neither fires a finding — the two ways a new-class spec may honestly
leave a question is what the fixture shows.
