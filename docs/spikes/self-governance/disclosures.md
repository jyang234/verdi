# oq-5 source extraction: wave-1 SDD ledger deferred/residual/parked items

Source (read-only, not modified, not committed by this spike):
`/Users/johnyang/code/verdi-system/verdi-wt/readiness-recovery-w1/.superpowers/sdd/2026-09-19-readiness-recovery-wave-1/progress.md`.

Selection rule: every line matching `deferred|residual|parked` case-insensitive
(`grep -ni 'deferred\|residual\|parked' progress.md`). That command
returns **exactly 9 lines** (line numbers 26, 29, 31, 32, 33, 41, 51, 53,
56), matching spec/self-governance's own count ("wave 1's nine items") and
the spike story's "nine deferred, residual and parked items".

But the 9 LINES carry **more than nine distinct debt clauses** — several
lines enumerate a list. Extracting each semicolon-delimited clause that is
actually marked deferred/residual/parked (as opposed to a clause in the
same line reporting something already resolved, e.g. "M7 exported
constructors ride") gives **21 raw clauses**, of which one (D9) is a
verbatim restatement of an earlier one (D3: "M5 double index walk"
appears in both line 29 and line 32's summary) — so **20 distinct items**.
This is the discrepancy this spike's README records under "Deviations
from the investigation plan": the story's "nine items" names the LEDGER
LINE count correctly; the ITEM count recorded below is 20.

## Raw enumeration (21 clauses, source line cited, D9 marked as a duplicate of D3)

| # | Line | Item |
|---|---|---|
| D1 | 26 | closefeature.go keeps its own stub-reconciliation glue (the journey adapter would re-walk stories) |
| D2 | 26 | fixtures_test.go untouched (golden demonstration lives in goldenRecord()) |
| D3 | 29 | M5 (double index walk) parked → residual — cost if wrong: latency only |
| D4 | 31 | a feature declaring both `2fa` and `s-2fa` stubs loses one blocker with disclosure |
| D5 | 32 | spike word asymmetry between the claimed-question witness (marked identity) and its clearing condition (routed) |
| D6 | 32 | the duplicate-id disclosure names the surviving id, not the lost debt |
| D7 | 32 | TestGatherFacts_HappyPath pins a fake's error prose |
| D8 | 32 | two adapter pass-through errors uncovered |
| D9 | 32 | double index walk (M5) — **duplicate of D3**, not counted again |
| D10 | 33 | closure-gated feature sources (stubs, outcome floor, claimed questions) derive nothing once the closure transition is behind the state — deferred to the final fix wave (cost if wrong: one state guard) |
| D11 | 41 | snapshotDataTree ignores empty dirs and same-length overwrites |
| D12 | 41 | Options.ConflictProvider doc should say a production caller who sets it owns ac-3's no-launch guarantee |
| D13 | 41 | PredecodedRequest doc must say the path check is the caller's too |
| D14 | 41 | Task 2's M7: baseline independence lost |
| D15 | 41 | Task 2's M9: non-building 62ac6e5f |
| D16 | 51 | disclose ONCE by `verdi serve` on stdout at startup that a startup context request targets one spec — deferred to the final fix wave (cost if wrong: one stdout line) |
| D17 | 53 | boarddocument_test pins loader error prose + depends on a constitution-less fixture |
| D18 | 53 | canonical-form ?spec= pin is weak |
| D19 | 53 | Task 3's M7: every poll derives |
| D20 | 53 | Task 3's M8: non-building intermediates |
| D21 | 56 | M6 (Document-tab Disclosures rendering) deferred to the post-design lane (co-4) |

Note: D14/D19 and D15/D20 share an "M7"/"M8"/"M9" label but are DIFFERENT
findings — each review round in this ledger restarts its own M-numbering
per task, so the labels collide without being duplicates. Only D9/D3 is a
genuine duplicate (same debt, same wording, two ledger lines).

## Recorded set: 20 items (D1–D8, D10–D21; D9 dropped as a duplicate of D3)

These are the 20 items this spike recorded into each oq-5 candidate home.
Each is given a stable slug for cross-referencing the three candidate
homes below and in `docs/spikes/self-governance/README.md`:

1. `closefeature-stub-glue` (D1)
2. `fixtures-test-untouched` (D2)
3. `m5-double-index-walk` (D3)
4. `2fa-stub-collision` (D4)
5. `spike-word-asymmetry` (D5)
6. `duplicate-id-names-survivor` (D6)
7. `gatherfacts-test-pins-prose` (D7)
8. `adapter-passthrough-uncovered` (D8)
9. `closure-gated-sources-state-guard` (D10)
10. `snapshot-datatree-empty-dirs` (D11)
11. `conflictprovider-doc-sentence` (D12)
12. `predecodedrequest-doc-sentence` (D13)
13. `task2-m7-baseline-independence` (D14)
14. `task2-m9-non-building-commit` (D15)
15. `startup-context-request-stdout-line` (D16)
16. `boarddocument-test-loader-prose-pin` (D17)
17. `canonical-form-spec-pin-weak` (D18)
18. `task3-m7-every-poll-derives` (D19)
19. `task3-m8-non-building-intermediates` (D20)
20. `m6-document-tab-disclosures-rendering` (D21)
