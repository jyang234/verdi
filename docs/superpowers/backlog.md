# Backlog

The durable to-do ledger for the verdi build: ideas, owner decisions, and carried residuals that must not be lost between
sessions. One row per item. This is an interim home: spec/self-governance ac-5 intends a machine-checked record of owned debts,
and when that lands these rows migrate into it.

**Rules.** Add a row the moment an item is deferred, parked, or carried; never delete one — mark it `done` or `dropped` with a
pointer to the commit, PR, or decision that closed it. A row that changes proof meaning, authority, lifecycle state, or a public
interface still needs its own invention-ledger entry before any implementation. IDs are never reused.

**Status values.** `open` (ready to schedule), `decision` (waiting on an owner decision), `blocked` (waiting on another row),
`scheduled` (in a named wave or lane), `done`, `dropped`.

## Process and methodology

| ID | Item | Why it matters | Needs | Status | Source |
|---|---|---|---|---|---|
| BL-1 | Merge-gate check that every implementation pull request names the story it builds | Ten features were built outside the story process and could not close; nothing prevents a repeat | A spec, then a gate change | decision | 2026-09-22 session, risk review |
| BL-2 | Staleness signal when nothing has closed in N weeks | No spec was archived from 2026-07-30 to 2026-09-22 and nobody noticed; unused gates erode silently | A spec; GLG v3 ac-8 (journey metrics) is the natural home and is deferred | decision | 2026-09-22 session, risk review |
| BL-3 | Run each wave lane as a stub-matched story through `verdi build start` | Keeps agent waves inside the lifecycle so their work can close | Owner decision; plan template change | decision | 2026-09-22 session |
| BL-4 | Features built outside the story process: one dated exception naming each feature, or retroactive stories from stubs | readiness-recovery-v2, ai-assisted-spec-design, comparative-spike-experiments-v3, guided-lifecycle-governance-v3, spec-documents, uat-round-1, and part of context-integrity-v2 cannot close | Owner decision; recorded in the invention ledger either way | decision | 2026-09-22 session |
| BL-5 | Author readiness-recovery-v2's ten outcome attestations | Scaffolds sit uncommitted in `verdi-wt/readiness-recovery-attest` (branch `close-prep/readiness-recovery-v2-attestations`); committing an unauthored scaffold would freeze it | Owner writes each claim | decision | 2026-09-22 session |
| BL-6 | Hard-pin subagent models: agent definitions with `model: claude-opus-5-5` and `effort: max` where `verdi/` sessions load them | The `opus` alias resolves to Opus 5.5 today but could drift; the workspace's max-effort definitions are not registered in `verdi/` sessions | Small repo change; takes effect next session | open | 2026-09-22 session |
| BL-29 | Scheduled CI sweep that runs `verdi close --preflight` (and the journey's eventual blockers) across every open accepted spec and publishes the result | Exercises the gates continuously; would have exposed the missing countersign config, the missing evidence producer, and the job-field mismatch in July instead of September | A spec; a scheduled, read-only workflow | decision | 2026-09-22 session, gap review |
| BL-30 | Provider contact: a standing rule that forge fixtures are the providers' published examples verbatim, plus an opt-in live read-only contract check against GitHub kept out of `make verify` (like `make fixture-regen`) | The strict approval decoder shipped in August and could never read a real GitHub response; every test used trimmed fixtures | A CLAUDE.md testing rule and a small opt-in make target | decision | wave 1 L2b review C-1 |
| BL-31 | Git-configuration matrix for ritual tests: run the rituals under ordinary but unusual settings (untracked files hidden, relative diffs, line-ending conversion, relative status paths) | The reclaim data loss came from a normal display setting that no test varied | A fixturegit helper and a matrix over the ritual tests | open | readiness-recovery wave 3 owner closure check (SI-226) |
| BL-32 | Structural guard against tests that prove nothing: reopen mutation-ratchet (dropped under SI-217), or write mutation probes into the Tier 3 review requirements | Review mutation probes caught decorative or tautological tests in almost every wave-1 lane, but only by hand | Owner decision | decision | wave 1 lane reviews |

## Closing machinery

| ID | Item | Why it matters | Needs | Status | Source |
|---|---|---|---|---|---|
| BL-7 | L4: adopt the solo governance profile and extend it (forge trust source; `author`, `story-review`, `feature-uat` mapped to the owner's forge principal; close transition) and add the `countersign:` block | No close can prove a countersign without it (SI-233, R-CM-5) | Controller prepares; owner merges `policy/adopt` | blocked | plan `2026-09-22-closing-machinery.md` |
| BL-8 | Pilot close of `spec/vatc-machine-projections` end to end | First real run of the environment pause, the job name inside a called workflow, the artifact round trip, token scopes, and the approval-required merge-gate run | Wave 1 merged and BL-7 | blocked | plan R-CM-4 |
| BL-9 | Close the remaining verdi-atc-prerequisites stories, then the feature | Their code is on main; they stay open for process and evidence reasons only | BL-8 | blocked | 2026-09-22 implementation-status audit |
| BL-10 | Migrate or retire the 282 legacy obligations without a quality block | Stories carrying them cannot be satisfied under exact producer matching | Owner decision | decision | L1 evidence-path trace |
| BL-11 | Give the countersign reduction an unproven approval-age state | SI-232 withholds an environment-review approval whenever its creation stamp is missing, stricter than v2 | Change to pinned `internal/countersign` with a successor binding | open | SI-232 |
| BL-12 | Merge gate's alignment-report check still requires `covers` = HEAD (`gate.go` condition 3, `gate_decisionconflict.go`) | Consistent with 03 §Gates' gate-then-commit, but differs from closure's one-behind rule | Owner ruling only if it ever bites | open | wave 1 L3b review |

## Carried residuals and defects

| ID | Item | Why it matters | Needs | Status | Source |
|---|---|---|---|---|---|
| BL-13 | Advance `sourceVerdiBaseline` in `internal/publicrelease/sourcecheck.go` | The public-release source pin is stale and will fail the next release run | Owner, before the next public-execution-contract release | open | wave 1 L1a review m-5 |
| BL-14 | `gitx.FastForwardOnly` runs its own configuration-sensitive `git status --porcelain` | Same defect class as the reclaim data-loss fix (SI-226) | A small fix with a config-row test | open | readiness-recovery wave 3 |
| BL-15 | About twenty `gitx.Show` callers pass store-relative paths | Wrong in a nested store (`product/.verdi`) | Re-base through `gitx.RepoPrefix` | open | readiness-recovery wave 3 |
| BL-16 | `close.go` operator prose emits store-relative commands | Commands are wrong from the repository root in a nested store | A small fix | open | readiness-recovery wave 3 |
| BL-17 | The recover e2e write classifier misreads `git worktree list --porcelain` as a write | A test-only false classification | A small fix | open | readiness-recovery wave 3 |
| BL-18 | `verdi recover` and `get_recovery` cost about 840 git subprocesses and 10 seconds per call | Slow for agents | Profile the inherited residue scan | open | readiness-recovery wave 3 |
| BL-19 | The journey advises "withdraw it with a note" for a stub, but no code path accepts a withdrawal | An unreachable clearing condition | A spec decision on where a withdrawal is declared, or remove the advice | decision | 2026-09-22 close attempt |
| BL-20 | `verdi design start --supersedes` handles only features, and its error wrongly cites 02 as forbidding story supersession | Misleads the operator; 02 and 03 rung 3 allow story supersession | Fix the message or support stories | open | vatc-forge-countersign-v2 authoring |
| BL-21 | `internal/align`'s one-second judge deadline flakes under load; `internal/sealedexec` stale-witness failures under the rung-field patch | Unowned defects from the process-hardening spikes review | Triage | open | process-hardening spikes report |
| BL-22 | Fold ordering can hide a sibling job's failure when two jobs in one CI run share a producer | Unreachable today (one producing job per workflow) | Revisit if a workflow gains a second producing job | open | wave 1 L1a re-review O-1 |
| BL-23 | Recovery presentation in the workbench | The recovery projection has no UI | The post-design Fable lane (readiness-recovery-v2 co-4) | open | readiness-recovery wave 3 |
| BL-24 | Closing-machinery wave 1 carried minors | Guards and wording gaps accepted as non-blocking | See the wave 1 report when it lands | open | wave 1 lane reviews |
| BL-33 | `verdi lint` only strict-decodes obligations (VL-001) and never runs `Validate` or the quality-union check | Nine malformed obligation shapes (blank claim, non-normalized ref, missing freshness, unknown invalidator, unknown producer kind, unknown state, missing frozen stamp, and others) pass lint silently; they fail later, and only for their own spec | A lint rule that validates obligations fully | open | wave 1 L1b review m-1 |
| BL-34 | SI-231's one-behind check refuses a genuine one-behind report in a nested store when the repository sets `diff.relative=true` | Fails closed, never wrongly accepts; an operator in that layout cannot use the committed-report path | Pass `--no-relative` inside `gitx.DiffNameStatus` | open | wave 1 L3b fix pass |
| BL-35 | SI-231's byte-identity clause compares the working-tree report with the git blob, so line-ending or smudge filters that rewrite the file cause a refusal | Fails closed; surprising on checkouts with those filters | Decide whether to compare normalized content | open | wave 1 L3b fix pass |
| BL-36 | `verdi close --prepare` over a working-tree report that diverges from a committed one-behind report regenerates it with a disclosure, as before this wave | Uncommitted disposition edits can be regenerated over, with a warning rather than a refusal | Decide whether `--prepare` should refuse there | open | wave 1 L3b fix pass |

## Specs awaiting decisions or plans

| ID | Item | Why it matters | Needs | Status | Source |
|---|---|---|---|---|---|
| BL-25 | strict-lint-target: drop `dupl` or keep it report-only; add a rule sentence for package globals or drop `gochecknoglobals` | The spec's premises were partly falsified by its spike | Owner decision, then a v2 revision | decision | process-hardening spikes report |
| BL-26 | self-governance ac-4: wire lint and journey to exemption windows, or restate ac-4 against the policy-conflict path | The spike found the window honored on one path only | Owner decision, then a v2 revision; the `rung` field breaks about 480 test cases | decision | process-hardening spikes report |
| BL-27 | verification-rules: confirm the deduplicated threshold (18) and ratify three accepted-when-absent contract fields | The under-enumeration lint needs a basis | Owner decision, then a v2 revision and a 02 ratification | decision | process-hardening spikes report |
| BL-28 | ritual-write-scope-v2 build plan: write-scope registry, behavioral ritual witness, forbidden-token witness on every verb, UAT pins | Structural guard against git side effects across every ritual | A plan, then a wave | open | ritual-write-scope-v2 |
