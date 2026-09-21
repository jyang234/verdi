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
(n=20) noise, disclosed here rather than smoothed over. T=16 is the
lowest (most inclusive, largest-backlog) threshold at which the labeled
false-positive rate first falls under one in five; it does not stay under
20% at every higher T by construction, only from T=18 up. See
`README.md` oq-1 for the recommendation and caveat.
