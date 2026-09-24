# Verification reliability — program scope

**Status.** A scope for the owner's decision, not a plan. It follows the workbench redesign (owner direction, 2026-09-23) and
precedes any build plan. It records no ruling; each workstream that proceeds gets its own specification and ledger entries.

**Why.** The owner's north star requires highly reliable verification that what is delivered is in alignment with what was
agreed, with deviations forcing conversations (workspace `docs/design/concepts/2026-07-11-verdesign-spec-realignment.md`). A
read-only assessment of origin/main on 2026-09-23 found Verdi strong on the audit trail and weak on the oracle:

- **Proven mechanically today:** the accepted spec bytes landed (gate condition 1); the deviation report is fresh and every
  finding carries a disposition; CI went green; a named Go test passed at the exact commit in the named CI job (SI-228), for the
  obligations elaborated that way.
- **Judged, not proven:** whether the delivered behaviour matches the criteria. The judge (`align.judge_cmd`, an LLM) is optional
  (`judge_required: false`), its model is not pinned, its prompt carries the criteria but not the diff, the constraints, or the
  decisions, it owes no verdict per criterion, and nothing re-verifies its output at gate or close (`internal/align/verify.go` is
  called only from tests).
- **Attested, not authenticated:** dispositions, attestations, and waivers are text anyone can write; the repository has no
  CODEOWNERS file; a waiver outranks a failing test in the fold.
- **Coarse evidence:** the historical closes rest on the self-hosted producer, which "performs NO test execution … emits verdict:
  pass purely because it was INVOKED AT ALL" after `make verify` passed (`cmd/verdi/selfevidence.go`), bound to criteria in
  `verdi.bindings.yaml`. Nothing checks that a named test exercises its criterion, or that it asserts anything (BL-32).
- **Where it is enforced:** alignment is checked at close only; the required merge gate runs `make verify` and `verdi lint`
  (`.github/workflows/merge-gate.yml`), not `verdi align` or `verdi gate`. Nothing has closed since 2026-07-30 (BL-2, BL-44).

## Target

For every accepted criterion of a change, a reader can see one of three honest answers, each backed by something the reader can
re-check:

1. **proven** — a named, reviewed test that fails when the behaviour is absent passed in CI at the delivered commit;
2. **judged** — an independent, bounded judgment with cited witnesses in the delivered diff, re-verifiable, with its reliability
   measured and disclosed;
3. **attested** — an authenticated human claim bound to the exact artifact, from someone other than the author where the profile
   requires it.

A deviation — a criterion not met, a constraint or decision contradicted — stops the change at merge until an authenticated
person dispositions it. "Coarse suite passed" is never presented as evidence for a particular criterion.

## Workstreams

| # | Workstream | What it adds | Depends on |
|---|---|---|---|
| W1 | **Judge contract** | The judge becomes a bounded instrument: a pinned model recorded in the report; input = accepted criteria, constraints, decisions, and the base..head diff; output = a verdict per criterion, constraint, and decision (aligned / deviates with cited witnesses / cannot determine), completeness enforced (every item answered); the raw result re-verified against the recorded findings at gate and close; judge absence blocks where the profile says so. | W0 |
| W0 | **Judge reliability measurement (spike)** | Before W1 claims anything: a fixture corpus of known-aligned and known-deviating changes, the judge run repeatedly, precision and recall and run-to-run agreement measured and published. Decides whether one judgment is enough, whether two must agree, and which verdict classes are trustworthy enough to block. | — |
| W2 | **Per-criterion evidence** | Replace coarse bindings with elaborated obligations that name real tests per criterion (the many legacy obligations without a quality block, BL-10); the rollup and the board show the evidence strength of each criterion (proven by a named test / judged / attested / coarse only). | — |
| W3 | **Test-quality guard** | Evidence that a named test can fail: a falsifier witness per behavioural obligation (the test is shown failing against a recorded mutation of the behaviour it claims), or a targeted mutation check on the named tests only (a narrower revival of BL-32). The owner chooses the mechanism. | W2 |
| W4 | **Authenticated human acts** | Dispositions, attestations, and waivers authenticated the way phase B authenticates exemption approvals (`internal/signedapproval`: a forge-verified signed commit introducing the exact row, bound to the artifact's content); role mappings in the adopted profile; a second person for an accepted deviation under a team profile, with the solo profile's collapse disclosed. | adoption (L4-like) |
| W5 | **Alignment at merge** | `verdi align` and `verdi gate` (or their required subset) run in the merge gate for implementation pull requests, so a deviation forces its conversation before the code lands, not at close. | W1 (a blocking judge needs W0's evidence) |
| W6 | **Verification rules per criterion** | The process-hardening feature `spec/verification-rules`: clauses per criterion and constraint, each clause citing its evidence kind; the under-enumeration lint; required fields enforced. | its own plan (PR #351) |
| W7 | **A close that completes** | The close ritual working end to end in CI for a change built through the story process — the choice between the sealed review path and a governed decision for work built outside sealed execution (BL-44, BL-64). | owner decision |

## Owner decisions this scope needs

1. **Ordering.** Recommended: W0 first (it decides how much to trust any judge), then W2 and W4 in parallel, then W1, W3, W5; W6
   on its own plan; W7 decided separately.
2. **The test-quality mechanism (W3):** falsifier witnesses per obligation, or a targeted mutation check.
3. **Whether the judge may block a merge (W5),** and at which verdict classes — to be informed by W0's measurements.
4. **Which human acts must be authenticated first (W4):** accepted deviations first is the strongest single step.
5. **Whether coarse evidence remains allowed at all** once per-criterion evidence exists, and how the historical closes are
   labelled.

## Sizing

Not estimated yet. W0 is a bounded spike and produces the numbers the rest needs; each other workstream is sized in its own plan
after the owner's decisions. The redesign comes first; W0 can run in parallel with it because it touches no user interface.

## Relation to other work

- **Workbench redesign** (plan PR pending): the per-criterion evidence strength (W2) and the deviation conversation (W5) need a
  legible home on the wall and the index; the redesign's drawer and chips are that home.
- **Process hardening** (PR #351): W6 is its verification-rules feature; ritual write scope protects the repository while
  verification runs.
- **Unsealed-provenance exemption** (parked): its signed-commit approval machinery is W4's foundation; its close path is one
  option for W7.
