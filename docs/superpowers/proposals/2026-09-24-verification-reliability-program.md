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

Two questions stay separate, as in the evidence model (03, Declarations and binding and The fold; `internal/evidence/fold.go`):

- **Result — does the criterion hold?** The governed states are kept: evidenced, violated, pending, no-signal, waived, with
  unknown or unavailable inputs disclosed. Nothing in this program replaces them.
- **Basis — what supports the result?** A named test with its semantic falsifier; a judged finding with cited witnesses; an
  authenticated attestation; suite-level coarse evidence. A criterion may require several kinds together.

The target is that every result rests on a basis a reader can re-check, and that the basis is shown honestly:

1. a criterion declaring behavioral evidence and an attestation stays incomplete while its test passes and its attestation is
   absent — it never becomes "proven" by fitting one basis;
2. a failing test stays visible even where a governed waiver changes eligibility;
3. a judge's "cannot determine" is never alignment, and an "aligned" judgment never clears a failing test, a missing required
   proof, an authentication failure, or incomplete judge coverage;
4. suite-level coarse evidence is labelled as a suite result and never presented as evidence for a particular criterion;
5. a deviation — a criterion not met, a constraint or decision contradicted — requires an authenticated person's disposition.

The evidence model's merge/close distinction is kept: the merge gate does not require all evidence to exist, because runtime
evidence can land after merge; closure does.

## Workstreams

| # | Workstream | What it adds | Depends on |
|---|---|---|---|
| W1 | **Judge contract** | The judge becomes a bounded instrument using W0's measured configuration: a pinned model, prompt, context construction, and tools, recorded in the report; input = accepted criteria, constraints, decisions, and the base..head diff; output = a verdict per criterion, constraint, and decision (aligned / deviates with cited witnesses / cannot determine), completeness enforced (every item answered); the raw result re-verified against the recorded findings at gate and close; judge absence fails closed where the profile requires the judge. Any change to the configuration requires re-measurement. | W0 |
| W0 | **Judge reliability measurement (spike)** | Before W1 claims anything: a pinned configuration (model, prompt, context construction, tools) measured against independently adjudicated labels on a held-out set, including realistic multi-file changes and missing or contradictory context; per-class false-positive and false-negative rates, abstentions, and failures, with the uncertainty of each estimate; acceptance thresholds fixed before the held-out answers are inspected. Run-to-run agreement is reported as a stability observation, never as evidence of correctness. W0 owns its corpus, runner, and results and touches no production user interface or shared gate wiring. | — |
| W2 | **Per-criterion evidence** | Replace coarse bindings with elaborated obligations that name real tests per criterion (the many legacy obligations without a quality block, BL-10); the rollup and the board show the evidence strength of each criterion (proven by a named test / judged / attested / coarse only). | — |
| W3 | **Test-quality guard** | A reviewed semantic falsifier per behavioural obligation: the named test passes on the candidate and fails, for the claimed behavioural reason, against the recorded mutation of the behaviour (a compiler error or a broken fixture is not that witness); the test, implementation and spec identities, the mutation, and both outcomes are recorded. Targeted mutation may generate candidates and supplement it; a mutation score alone does not show the test checks the criterion. W3 establishes test sensitivity, a different property from W0's judge reliability. | W2 |
| W4 | **Authenticated human acts** | Waivers and accepted deviations first (waivers first if they must be ordered, since a waiver can override a failing record in today's fold), then attestations and the other human acts that clear blockers — authenticated the way phase B authenticates exemption approvals (`internal/signedapproval`: a forge-verified signed commit introducing the exact row, bound to the artifact's content, invalidated by any later content change), keeping the distinction between a verified signer and an authorized principal and the profile's separation rules; a second person for an accepted deviation under a team profile, with the solo profile's collapse disclosed. A display name or a CODEOWNERS entry never substitutes for authentication. | adoption (L4-like) |
| W5 | **Alignment at merge** | The alignment check runs in the merge gate for implementation pull requests, so a deviation forces its conversation before the code lands. It needs an explicit protocol — the candidate commit, evidence acquisition (today's merge workflow does not produce the evidence bundle), the CI branch identity, and the report's freshness and commit rule — not just today's local commands invoked in CI. The judge starts advisory (shadow mode); later, measured finding classes may require a human disposition. | W1, W4 |
| W6 | **Verification rules per criterion** | The process-hardening feature `spec/verification-rules`: clauses per criterion and constraint, each clause citing its evidence kind; the under-enumeration lint; required fields enforced. | its own plan (PR #351) |
| W7 | **A close that completes** | The close ritual working end to end in CI for a change built through the story process — the choice between the sealed review path and a governed decision for work built outside sealed execution (BL-44, BL-64). The path and a pilot are chosen early; one successful end-to-end close is an exit criterion of the whole program. | owner decision |

## Owner decisions this scope needs (with the independent review's recommendations)

| Decision | Recommendation |
|---|---|
| Ordering | The redesign and a bounded W0 run first, together (W0 touches no user interface); then W2 and W4; W1 informed by W0; W3 after W2; W5 after W1 and W4; W6 on its own plan; W7's path and pilot chosen early, with one successful close required before the program is called complete |
| Test quality (W3) | Reviewed semantic falsifiers per behavioural obligation; targeted mutation as a supplement |
| The judge and merges (W5) | Advisory first; measured finding classes may later require a human disposition; "aligned" never overrides failing tests or missing required proof |
| Authentication first (W4) | Waivers and accepted deviations together (waivers first if ordered); then attestations and the other blocker-clearing acts |
| Coarse evidence | Kept as labelled suite evidence and historical provenance; never a substitute for the direct evidence new or migrated obligations require; frozen historical records preserved and labelled in current views, not rewritten; the forward migration boundary specified in authority |

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
