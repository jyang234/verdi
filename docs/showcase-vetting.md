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
