# Readiness-recovery wave 1 report: continuous readiness (ac-1..ac-5)

Status: COMPLETE — READY_FOR_OWNER_RISK_GATE (gate #4 `verify OK` at code head eefe44fe; not pushed).
Risk tier: Tier 2 (Tasks 1–4), Tier 1 (Task 5); Tasks 1 and 3 escalated to Tier 3 on one Critical finding each; the final fix wave (Task 4b) ran as Tier 3.
Base..Head: e768cf9c..eefe44fe (code head; main merged at 6c3518ff, clean, 0 conflicts), plus this docs-only wave-close commit on top.
Plan: docs/superpowers/plans/2026-09-19-readiness-recovery-wave-1.md (rulings R-RR1-1..21; amendments recorded in place).
Ledger: docs/superpowers/invention-ledger.md SI-207 (R-RR1-11/12), SI-208 (R-RR1-18), SI-209 (R-RR1-21).

## Lanes and accepted ranges

| Task | Range | Reviewer verdict | Fix rounds |
|---|---|---|---|
| 1 eventual closure blockers (ac-1) | b810c302..685311a2 | Needs fixes → clean (C1 Tier 3) | 1 (fix-opus-max) |
| 2 readiness loader + cache-only judge (ac-2, ac-3) | 685311a2..410db101 | spec ✅, quality Needs fixes → clean | 1 (original Sonnet) |
| 3 consumers and parity (ac-4) | 0a6c5137..4a960e36 | Needs fixes → clean (C1 Tier 3) | 1 (fix-opus-max) |
| 4 derivation stamp on the page (ac-2/ac-4, Fable) | 4a960e36..d11cd84e | Approved (I1 + 3 minors) → Closed | 1 (Fable) |
| 4b final fix wave R-RR1-13/16/17/18/19 | d11cd84e..c66a749e | Approved (I1 adjudicated Minor, M2, M3 disproven) → Closed | 1 (fix-opus-max) |
| 5 wall gap witness (ac-5) | dd8bb957..ee60e5c3, 813faeb1 | Approve (I1 controller-level → R-RR1-21) | 1 pre-review return + 1 clerical |
| controller | 29fa3adf | evidence only | — |
| 2b consolidation-witness successor binding (R-RR1-22, Tier 3) | 6c3518ff..bb096a2a | Approved (2 cosmetic minors) | 0 |
| 4c isolated readiness-pilot fixture store (R-RR1-23, Fable, Tier 2) | bb096a2a..5bd9ced3, fix eefe44fe | Approve (I1 + 4 minors) → Closed | 1 (Fable) |

## Gates

Gate #1 (`VERDI_E2E_PORT_BASE=4390 make verify` at 6c3518ff, main merged): RED at step test after 599s. Every package green except `internal/sealedexec` (FAIL 140.5s: TestPublicControllerConsolidationWitness / ImportSuccessor / WitnessSensitivity, "stale witness source"): the immutable public-consolidation witness pins four sources Task 2 changed (internal/policyconflict/service.go, cmd/verdi/context.go, context_conflict.go, context_constitution.go). `cmd/verdi` -race: ok 594.1s. Fixed by Task 2b under R-RR1-22 (successor bindings, checks.json untouched).

Gate #2 (same command at bb096a2a): test, fixture, lint-store (15s), spec-align (208s) OK; RED at step e2e after 825s — 10 failed / 319 passed, all ten in `49-readiness-pilot.spec.ts`, which passes 17/17 alone. Cause: its exact-array oracles read the shared harness store, which suites 02–38 mutate (nine open stickies, a spike claim, an edited spec); the continuous derivation shows what the deleted startup snapshot froze. Fixed by Task 4c under R-RR1-23 (isolated readiness-pilot fixture store).

Gate #3 (same command at 5bd9ced3): `verify OK`, exit 0. Steps: build 1s, fmt-check 1s, vet 1s, lint 2s, test 839s, fixture 0s, lint-store 20s, spec-align 219s, e2e 905s (329 passed, 0 failed; suite 49 17/17 under the full alphabetical run). Recording-artifact scan empty; tree clean.

Gate #4 (wave-close gate at the code head eefe44fe): `verify OK`, exit 0. Steps: build 0s, fmt-check 1s, vet 1s, lint 2s, test 746s, fixture 0s, lint-store 13s, spec-align 196s, e2e 776s. e2e 329 passed, 0 failed (12.9m). Recording-artifact scan empty; tree clean.

## Whole-wave Opus review

Accepted (`.superpowers/sdd/…/wave-review.md`, at 29fa3adf before Tasks 2b/4c): no Critical; I1 R-RR1-13 silence vs R-RR1-19 disclosure asymmetry for a terminal state (both rulings recorded); M1 the parity test names four consumers but witnesses three (the page is joined structurally through the same loader); M2 the derivation-stamp literal is hand-built in the harness and readinesstest rather than derived from the producer. Provenance clean: every commit maps to a reviewed lane range or a controller evidence commit. No semantic conflict with main's PR #334–#337 merges. Tasks 2b and 4c landed after this review; each carried its own Tier 3 / Tier 2 chain (fresh reviewer, closure), touched only the sealed-witness test table and the e2e harness/suite 49, and changed no product behavior.

## Residual risks carried to the owner gate

- closefeature.go keeps its own stub-reconciliation glue (Task 1, implementer-disclosed).
- Non-building intermediate commits 62ac6e5f (Task 2) and two in Task 3 — reviewed history not amended; per-commit build rule enforced from Task 3 on (and by main's staged-tree pre-commit hook after the merge).
- localoperator byte-identical test lost independence (Task 2 M7).
- Every readiness poll derives (Task 3 M7) — latency only.
- Document-tab Disclosures inert (Task 3 M6) — deferred to the post-design workbench lane.
- e2e addSticky commit-before-locator window is suite-wide (26/49/50) — suite-level sweep, post-design lane.
- R-RR1-19 gates on closureAhead; a lifecycle where acceptance is reachable but closure is not would suppress genuinely-ahead policy debt (no such model is declared; Model.checkFrontier rejects divergent models).
- The context/verdict destination may name a request file the server has outlived (4b M4, inside R-RR1-17's accepted trade).
- The ac-5 witness derives on the claim-wall Go fixture, not the e2e harness fixture (SI-209); the harness fixture's own gap is inherited unproven by the post-design lane.
- Sealed consolidation witness: four Task 2 sources are bound as successors (R-RR1-22, SI-200 mechanism); `checks.json` untouched. Any lane editing a pinned path must run the sealedexec consolidation tests.
- Suite 49's isolated fixture: the warm GET still carries the API request context's own 30s default; no fixture scratch dir is removed (peer parity); `stop()` leaves a dead cached URL (unreachable).
- `./cmd/verdi` under `-race` runs ~10 min on this machine against Go's default per-package timeout; `make test` sets no `-timeout` (pre-existing at base; observed value at gate #1: 594s at gate #1, then cached).

## Next authorized action

READY_FOR_OWNER_RISK_GATE — owner review/merge of `agent/readiness-recovery-wave-1`; wave 2 (ac-6, ac-7) stacks on this head per its plan; not pushed by the controller.
