# oq-1 hand labels — top 20 by mechanical claim count

Source data: `measure.out` (full command output;
`go run ./docs/spikes/verification-rules/_scratch/measure .`, exit 0). This
file hand-labels each of the top-20 items TRUE (genuine multi-claim,
deserving an enumerated `clauses:` list) or FALSE (mechanical
over-count — one claim over an enumerated domain, or a disjunction with one
shared consequence, or a citation list) using this rule, applied by one
reader (this spike's author) with no independent cross-check — the
labeling is therefore a single-rater judgment, disclosed as a caveat in
`README.md`:

**Rule.** Read the text's *top-level* semicolon/`and`-joined segments only
(ignore commas inside one em-dash- or paren-bounded sub-list — those name
members of one set, not separate obligations). Count a segment as one
claim if it asserts a requirement/prohibition on its own topic,
independently falsifiable of the other segments. A segment whose payload
is a closed set/taxonomy ("X covers/binds/lists these N named things") is
one claim (a completeness assertion over that set), not N. A segment
enumerating N alternative *triggers* that all share one *consequence* is
one disjunctive claim, not N, unless the segments plainly cover unrelated
topics (co-3/co-9/co-5 below: each item is about a different mechanism
entirely, not instances of one class).

| # | ref | claims (proxy) | real top-level claims (by rule) | label | why |
|---|---|---|---|---|---|
| 1 | ai-assisted-spec-design#co-3 | 24 | ~11, all different topics (mutation gate, digest, auto-merge, validate-before-write, transaction, ID-addressing, actor distinctness, provenance non-authority, agent-action ban, stale-overwrite guard, response identity) | TRUE | genuinely 11 independent prohibitions, not a taxonomy |
| 2 | readiness-recovery#ac-8 | 22 | 4 (required per-state fields; the 8-state closed set; ambiguous-state handling; exit code contract) | TRUE | 4 distinct topics even after collapsing the state-list to one closed-set claim |
| 3 | ai-assisted-spec-design#co-9 | 21 | 5, one per test surface (operation coverage, adapter conformance, browser e2e, MCP/CLI e2e, no-live-network) | TRUE | different topics, not instances of one class |
| 4 | ai-assisted-spec-design#co-5 | 19 | 5, one per provenance rule (no-import, excerpt classification, digest-at-attach, markdown-edit origin, generated-provenance non-evidence) | TRUE | different topics |
| 5 | readiness-recovery#ac-9 | 18 | 4 (per-choice required fields; exactly-two-executable-choices named; other states diagnosis-only; forbidden git verbs) | TRUE | 4 distinct topics |
| 6 | sealed-claude-vatc-events#ac-3 | 18 | 2 (the vocabulary is this closed taxonomy; hidden reasoning is excluded) | FALSE | the long list is ONE closed-vocabulary completeness claim, not 18 |
| 7 | sealed-codex-execution#ac-3 | 18 | ~2 (taxonomy normalized+sequenced; any-of-5-conditions blocks authority) | FALSE | both segments are one-claim-over-an-enumerated-domain/disjunction |
| 8 | ai-assisted-spec-design#co-2 | 17 | 4 (schema strict-decode; unknown-X fails closed; extensibility restricted to named ports; fields mutated only via descriptors) | TRUE | 4 distinct topics, closest call in the TRUE set |
| 9 | comparative-spike-experiments#ac-6 | 17 | 4 (policy non-weakening over named domain; candidates can't touch protected inputs; provenance records named fields; provenance is not itself an authority decision) | TRUE | last clause is a separate topic from "records these fields" |
| 10 | comparative-spike-experiments-v2#ac-6 | 17 | (identical text to #9) | TRUE | same text as v1 — see oq-4 (identical, not reworded) |
| 11 | comparative-spike-experiments-v3#ac-6 | 17 | (identical text to #9) | TRUE | same text as v1/v2 — see oq-4 |
| 12 | spec-documents#ac-1 | 17 | 6, one per topic (builds-model; renders md/html in 3 kinds; byte-deterministic; stamps 4 named fields; proposed-not-accepted header; never-omits-discloses-instead) | TRUE | unambiguous, 6 clearly different topics |
| 13 | sealed-claude-vatc-events#ac-2 | 16 | 1 (the envelope binds this exact field list into two named chains) | FALSE | pure schema field-list, one composition claim |
| 14 | context-integrity#ac-2 | 15 | ~2 (compile-determinism over named inputs/contexts; every input classified+ordered+digest-bound) | FALSE | one determinism claim + one closely-tied elaboration |
| 15 | context-integrity-v2#ac-2 | 15 | (identical text to #14) | FALSE | same text as v1 — see oq-4 |
| 16 | context-receipts-review#ac-1 | 15 | 2 (schema binds named field list; the receipt event is outside the self-digest boundary) | FALSE | first segment is a field-list claim; only 2 total |
| 17 | readiness-recovery#ac-1 | 15 | ~3, borderline (closure blockers derive only from these 7 named sources; each Blocker has these 6 named fields; never-a-prediction/never-a-present-violation) | FALSE (borderline) | closest call in the FALSE set — could reasonably be scored 4 |
| 18 | comparative-spike-experiments#ac-1 | 14 | 4 (explore freely; register-with-named-fields restricted to human lock; immutable after lock; any change is a new revision) | TRUE | 4 distinct topics |
| 19 | comparative-spike-experiments-v2#ac-1 | 14 | (identical text to #18) | TRUE | same text as v1 — see oq-4 |
| 20 | comparative-spike-experiments-v3#ac-1 | 14 | (identical text to #18) | TRUE | same text as v1/v2 — see oq-4 |

**Totals: 13 TRUE, 7 FALSE (35% false-positive rate, unconditioned).**

## False-positive rate by candidate threshold (using only these 20 labeled points)

| T (flag claims>T) | labeled sample above T | false among them | FP rate | corpus-wide backlog at T (from `measure.out` sweep) |
|---|---|---|---|---|
| 13 | 20 | 7 | 35.0% | 23 |
| 14 | 17 | 7 | 41.2% | 17 |
| 15 | 13 | 3 | 23.1% | 13 |
| **16** | **12** | **2** | **16.7%** | **12** |
| 17 | 7 | 2 | 28.6% | 7 |
| 18 | 4 | 0 | 0.0% | 4 |
| 19 | 3 | 0 | 0.0% | 3 |

The curve is **not monotonic**: T=17 regresses above T=16 because the
claims=17 band (5 items, all TRUE) drops out while the claims=18 band (3
items, 2 FALSE) is still included one step earlier. This is small-sample
(n=20) noise, disclosed here rather than smoothed over.

**Correction (post-review, lane-verification-rules-review.md VR-1): the
table above double-counts duplicate texts and its threshold does not
survive deduplication.** Rows 10, 11, 15, 19, and 20 above are each
byte-identical text to an earlier-numbered row (10/11 to #9, 15 to #14,
19/20 to #18 — oq-4's own finding that every carried AC/constraint is
byte-identical across a family's version revisions, applied to the
sample itself). Counting each physical copy as an independent judgment
inflates the sample from 15 real distinct texts to 20, and inflates every
threshold's apparent backlog by the same duplication. Deduplicated:

## Deduplicated false-positive rate by candidate threshold (15 distinct judgments)

Full recomputation: `_scratch/dedup/main.go`, `go run
./docs/spikes/verification-rules/_scratch/dedup .` — exit 0, output
`dedup.out` (64 lines). Confirms 206 distinct texts corpus-wide (of 277
scored) and reduces the top-20 sample to exactly 15 distinct judgments
(9 TRUE / 6 FALSE, 40.0% unconditioned FP rate — higher than the raw
table's 35.0%, because four of the five removed duplicates were TRUE
rows, which had been diluting the FALSE share):

| T (flag claims>T) | distinct sample above T | false among them | FP rate | corpus backlog at T (distinct-206 basis) |
|---|---|---|---|---|
| 13 | 15 | 6 | 40.0% | 16 |
| 14 | 14 | 6 | 42.9% | 14 |
| 15 | 11 | 3 | 27.3% | 11 |
| 16 | 10 | 2 | **20.0%** | 10 |
| 17 | 7 | 2 | 28.6% | 7 |
| **18** | **4** | **0** | **0.0%** | **4** |
| 19 | 3 | 0 | 0.0% | 3 |

On the deduplicated basis, T=16 sits **at**, not under, one in five
(20.0% exactly) — the story's rule ("falls under one in five") does not
select it. **T=18 is the first threshold that is genuinely under 20%,
giving backlog 4** (identical to the raw-basis T=18 backlog, since none
of the top-4 raw items are versioned-family duplicates in the first
place). A `live-distinct` population (dropping any object whose spec
directory is itself a frozen predecessor — i.e. the target of some other
spec's `links: {type: supersedes}` — and re-pointing to a live sibling
with identical text where one exists) produces **exactly the same sweep
as distinct-206 at every threshold** (`dedup.out`: 0 objects dropped) —
because oq-4 already proved every carried AC/constraint text propagates
unchanged to its family's live head, so "distinct" and "excluding frozen
predecessors" turn out to be the same filter for this corpus's AC/
constraint population specifically (they would diverge if a future
revision ever removed or amended a carried clause without a live
successor carrying identical text). See `README.md` oq-1 for which basis
the recommendation adopts.
