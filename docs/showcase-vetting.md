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
