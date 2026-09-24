# Process hardening — orchestration plan (four features)

**Goal.** Build the four remaining process-hardening features — `spec/ritual-write-scope-v2` (RWS), `spec/strict-lint-target`
(SLT), `spec/self-governance` (SG), and `spec/verification-rules` (VR) — in parallel where it is safe, through the story
process, until every acceptance criterion is evidenced by CI records and every feature's work is merged. `spec/mutation-ratchet`
is not in scope (closed as not-doing, SI-217).

**What "complete" means here.** No story or feature can close in this repository today (backlog BL-44): unadopted, the countersign
is unproven; adopted, the review-phase conflict gate is blocked by three sealed-provenance inputs. This plan therefore stops at
"implemented, evidenced, merged" and does not claim closure. Building through the story process (D-PH-1) keeps each feature
from the permanent no-implementing-stories state (BL-4), but these stories will not be built under sealed execution, so closing
them later still needs a close-path decision (the sealed review path, or a governed decision on unsealed work). That limit is
recorded, not solved, by this plan.

**Base.** origin/main `3bc854b6`. Worktree `verdi-wt/ph-plan`, branch `design/process-hardening-orchestration`.

**Binding authority.** The four feature specs as accepted on main; the spike report
`docs/superpowers/reports/2026-09-20-process-hardening-spikes.md` and `docs/spikes/<slug>/README.md`; the owner decisions of
2026-09-21 recorded in that report; the workspace CLAUDE.md and `verdi/CLAUDE.md`; the invention ledger. Where this plan and a
feature spec differ, the spec governs until its v2 (step 1) supersedes it.

## Owner decisions carried in

From 2026-09-21 (spike wave report): mutation-ratchet not-doing; RWS adopts the four-state `index_carry`; SLT drops `dupl`, keeps
`gochecknoglobals` behind a CLAUDE.md sentence, uses a committed baseline; SG restates ac-4 against the policy-conflict path, with
lint and journey wiring later; VR takes T=18 on the deduplicated basis, Seam B, and a 02 ratification.

From 2026-09-23 (this plan; the owner accepted the controller's defaults):

- **D-PH-1 — story process (closes BL-3).** Every lane builds part of a story instantiated from its feature's stub
  (`verdi design start --from-stub <feature> <stub>`, with the bare feature name) and built on the branch `verdi build start`
  cuts. A story may be built by several lanes. Stories give each acceptance criterion its own evidence and keep the features out
  of BL-4's trap. Stub instantiation requires an accepted parent feature (merge-signaled acceptance design;
  `internal/stubinstantiate` refuses a proposed parent), so each feature's stories are authored only after its v2 has merged
  (step 1).
- **D-PH-2 — SG takes the payload route.** SG's rules are a typed constitution payload, not a new mandatory `rung` field on
  every policy claim: no kernel schema ratification, no ~480-case fixture migration, reversible.
- **D-PH-3 — the build rules move into the repository.** The Go-style and testing rules move from the workspace-root CLAUDE.md
  (outside git) into one in-repo rules file that the workspace CLAUDE.md points to, so SG's drift witness can run in CI. The
  owner-decided sentences are added there: package globals (SLT), "every commit builds" and "never a bare `git stash`" (SG
  rules 4 and 5).
- **D-PH-4 — VR.** T=18 is recorded in the v2; ac-1's witness is Seam B (a `// verdi:clause` marker and a gate test), and ac-1's
  "store lint" wording is corrected to match; ac-3 is retargeted through a small `spec/readiness-recovery-v3` that amends co-2 to
  carry clauses, because adding clauses to the carried copy in readiness-recovery-v2 would break VL-015's byte identity; that
  successor merges only after VR's decoder support (lane V1) is on main (unit A3c).
- **D-PH-5 — test speed first.** Before these waves, lane T1 makes `cmd/verdi`'s suite cheaper (one shared binary build, trimmed
  heavy fixtures) and then safely parallel (audited `t.Parallel`, `-parallel 4` in the Makefile `test` target). No test is
  deleted or weakened. This replaces raising the package timeout.
- **D-PH-6 — X-1 first.** The shared strict-YAML seam (`artifact.DecodeStrict`) silently drops null sequence elements,
  truncates floats into integers, and accepts `yes`/`no` booleans. VR and SG add YAML fields, so this is fixed before them.

## Serialization rules

The features are independent, but they share surfaces. These rules make parallel lanes safe:

| Shared surface | Touched by | Rule |
|---|---|---|
| Spec v2s, ratifications (02, 05, 01), 08 entries, origin-and-mirror edits | SLT, SG, VR | Controller-authored; one ratification in flight at a time (an origin edit reds every other branch's fidelity check until it merges) |
| Invention ledger and backlog | all | Controller only; re-read origin/main's tail before every push |
| CLI-verb and MCP-tool registries | SG (lane G2) | Only lane G2 changes them, isolated |
| `internal/lint` rule list and VL numbers | VR, SG | Numbers assigned here: VR takes VL-023 and VL-024, SG takes VL-025 if it needs one. VL-026 is taken by closed-spec object supersession (its design of 2026-09-24, SI-264); the next free number is VL-027 |
| Consolidation successor map (`internal/sealedexec/public_consolidation_import_test.go`) | X-1, VR (V1), RWS (R5, if it edits pinned `cmd/verdi/context*.go`) | One editing lane at a time, in the order X-1 → V1 → R5 |
| `cmd/verdi`, broadly | RWS (R5 observer threading, ~43 files) | R5 runs alone in `cmd/verdi`; no other lane edits `cmd/verdi` during it |
| Strict-lint baseline | every later lane | Milestone M-SLT: SLT's whole-feature review, risk gate, and pull request merge before any other feature's pull request merges. Lanes may run beside SLT, but each lane integrated after M-SLT must pass `make lint-strict` against the committed baseline (new findings are fixed, never added to the baseline) |
| The in-repo rules file | SLT, SG | Written once in step 1 (A5); later lanes do not edit it |
| Governance adoption of this repository | SG (G6) | Last. Adoption turns on the build-phase conflict gate for `verdi build start`; G6 first proves that gate still passes for a representative story, or stops |

**Concurrency.** At most three lanes in flight. Full gates run serially with no other agent running. Lane test runs use T1's
`-parallel 4` cap.

## Step 0 — prerequisites

| # | Item | Why |
|---|---|---|
| P1 | Phase B wave 1 lands on main (its own PR); the unsealed-provenance exemption is parked after it | Shared plumbing; stops two programs editing the same pinned files |
| P2 | Lane T1 (test speed) lands (its own PR) | Every lane runs `cmd/verdi` several times; its suite sits at the default package timeout locally |
| P3 | Lane X1: the strict-YAML seam refuses null sequence elements, non-integral numbers in integer fields, and non-boolean scalars in boolean fields; a reviewed successor of the pinned `internal/artifact/decode.go`; a ledger entry first (previously accepted inputs become refusals); `make lint-store` before and after proves no committed artifact relied on the old behavior | D-PH-6 |

## Step 1 — authority (controller-authored; each unit gets one independent cross-model review of its exact head, at most one correction pass, one closure check, then the owner's merge)

Every feature follows the same acceptance order, because stub instantiation needs an accepted parent and a merged specification
must pass strict decoding at its merge gate:

1. the feature's v2 (and any contract ratification it needs) is reviewed and **merged**;
2. its stories are instantiated from the merged v2's stubs (`verdi design start --from-stub <feature> <stub>`), authored as one
   story batch per feature, reviewed once, and **merged**;
3. only then does `verdi build start` cut each story's build branch, and its lanes start.

| Unit | Content | Needed before |
|---|---|---|
| A1 RWS ledger rows and story batch | Ledger rows: the `update-ref` conflict (`internal/specimport`, `internal/stubinstantiate` call `gitx.UpdateRef`, which ac-3 forbids); recovery's seventh token `-f` (SI-224); any test-only environment hook; the readings the census needs. RWS-v2 is already accepted, so its three stories are instantiated at once from `write-scope-registry` (ac-1), `ritual-effect-witness` (ac-2, ac-4), `gitx-recorder-seam` (ac-3) and reviewed with the ledger rows | R1 |
| A2a SLT v2 | Carries the 2026-09-21 decisions; resolves oq-1..oq-4; the retro-witness's "after" commit (the loader fix is visible only at 5f60c76c, not 410db101); the step's wall-clock budget; `exhaustive` re-measured with `default-signifies-exhaustive: true`; stubs aligned to lanes S1, S2 | A2b |
| A2b SLT story batch | Stories from the merged v2's stubs | S1 |
| A3a VR v2 and the 02 ratification | The clause grammar in 02 (08 entry, mirror sync), the three accepted-when-absent exception rows, T=18, Seam B wording for ac-1, ac-3 retargeted to a later readiness-recovery-v3; resolves oq-1..oq-5; stubs aligned to V1–V4. The spike's 46-line 02 draft is input to adjudicate, not a ready patch: it omits a clause's own evidence field while VR ac-1 requires clause evidence kinds, so A3a decides whether a clause declares its evidence kinds or inherits its criterion's, and how the Seam B marker gate detects a kind mismatch | A3b |
| A3b VR story batch | Stories from the merged v2's stubs | V1 |
| A3c readiness-recovery-v3 | The clause-bearing successor: amends co-2 only, carries its clauses and the new witnesses' references. It can merge only after V1's decoder support has landed on main, because its merge gate strictly decodes `clauses:` (today `artifact.Constraint` rejects the field) | V3 |
| A4a SG v2 and the 05 and 01 ratifications | The payload route (D-PH-2); ac-4 restated against the policy-conflict path; the `verdi policy` subcommand and MCP projection (05); the disclosure-index directory (01, VL-007); the journey reason code; the VL rule's ledger row; resolves oq-1..oq-5; stubs aligned to G1–G6 | A4b |
| A4b SG story batch | Stories from the merged v2's stubs | G1 |
| A5 In-repo rules file | The rules move and the owner-decided sentences (D-PH-3); the owner applies the one-line pointer in the workspace CLAUDE.md | S1 (package-globals sentence), G3 |

That is nine reviewed authority units. The one-ratification-at-a-time rule still holds across them: 02 (A3a), 05 and 01 (A4a).

## Step 2 — lanes

Tier 3 lanes are implemented by Opus 5.5; Tier 1 and 2 by Sonnet 5; every review and every fix by Opus 5.5 (workspace
CLAUDE.md). Each lane follows the repository chain: a brief with its frozen interface, write set, and pinned-file grep; RED then
GREEN; controller pre-review with the brief's full GREEN list; independent review; a fresh fixer and a fresh re-reviewer for
Critical or Important findings in Tier 3; integration only when the integrated patch equals the reviewed patch.

| Lane | Feature / story | Scope | Tier | Depends on |
|---|---|---|---|---|
| S1 | SLT | `.golangci.strict.yml`, `make lint-strict` in `VERIFY_STEPS`, committed baseline and its shrink-only check, static witnesses (parity-file digest, exclusion count) | 2 | A2b, A5, T1 |
| S2 | SLT | Retro-witness at the corrected commits, the golangci analysis cache in CI, the budget, `merge-gate.yml` and `verify.yml` kept in step | 2 | S1 |
| R1 | RWS / write-scope-registry | The registry (Go literal, not a package-level variable, so SLT's `gochecknoglobals` is satisfied), the closed seven-field grammar with four-state `index_carry`, the static witness, and the full census of every mutating git entry point (the spike measured 5; at least 14 more exist) | 3 | A1 (strict-lint clean at integration once M-SLT has landed) |
| R2 | RWS / ritual-effect-witness | The seeded-fixture harness with the three sensors (command log, state diff, created-commit file lists), two seeded states, a local bare remote | 2 | R1 |
| R3 | RWS / ritual-effect-witness | Per-ritual cases for every declared ritual on every publication path, including close | 3 | R2 |
| R4 | RWS / ritual-effect-witness | UAT fixes: `build start` cuts from the default branch (UAT-023) and checks collisions (UAT-031); guarded commits in design start and commit-to-design (UAT-036); the owner decides each newly found `carried` ritual | 3 | R1 |
| R5 | RWS / gitx-recorder-seam | The forbidden-token check over every verb's command log; threading the observer through every verb (successor bindings if pinned `context*.go` files change) | 3 | R3, R4, V1 (successor-map order) |
| V1 | VR | Decoder support: the clause type in `internal/artifact` (pinned `spec.go`, `object.go`: reviewed successors), specdoc rendering. Lands on main as its own story pull request, the merge point A3c waits for | 3 | A3b, X1 |
| V2 | VR | The relation lint and the under-enumeration lint (VL-023, VL-024) | 2 | V1 |
| V3 | VR | readiness-recovery-v3 co-2 dogfood and its new witnesses | 2 | V2, A3c |
| V4 | VR | ac-4's required-field gate test with the three exception rows (UAT-008 first) | 2 | V1 |
| G1 | SG | The typed rules payload and its claims for the in-repo rules | 2 | A4b, A5, X1 |
| G2 | SG | The `verdi policy` subcommand and MCP projection per rung (the only registry change in this plan) | 3 | G1 |
| G3 | SG | The drift witness between the in-repo rules file and the policy set | 2 | G1, A5 |
| G4 | SG | ac-4's exemption witness on the policy-conflict path (the golangci parity exception with a window) | 3 | G1 |
| G5 | SG | The deferred-items index (store-layout amendment, journey reason code, VL-025 if needed) | 3 | A4b |
| G6 | SG | Adoption of this repository's governance store (ac-1), last, after proving the build-phase conflict gate still passes | 3 | all other lanes |

**Waves** (at most three lanes in flight; the dependency column governs):

- **Wave A:** S1 → S2, then SLT's whole-feature review, risk gate, and merge (milestone M-SLT); R1 → R2; V1 once X1 and A3b
  have merged, then V1's story pull request merges (the point A3c waits for).
- **Wave B:** R3, R4, V2, V4, G1 → G2 (G1 after A4b).
- **Wave C:** R5 (alone in `cmd/verdi`), V3 (after A3c), G3, G4, G5.
- **Wave D:** G6, then one aggregate recheck of the whole repository.

Each feature closes its own sequence when its last lane integrates: a whole-feature Opus review, the owner's risk gate, and its
own pull request, so value lands feature by feature. Wave D's recheck is an extra full gate after G6, not a replacement for any
feature's own gate. Estimated size: 17–21 lanes including X1, and nine reviewed authority units.

## Owner actions

- Approve the in-repo rules file and apply the pointer in the workspace CLAUDE.md (A5).
- Merge the ratification pull requests (02, 05, 01) and the v2 specs.
- Decide each newly found `carried` ritual as RWS's census reports it.
- Risk gate and merge for each feature.

## Out of scope

Closing any of these features or stories (BL-44); the unsealed-provenance exemption's wave 2 and later (parked); the sealed
review path; mutation-ratchet; moving the 52 backlog rows into SG's index (a later migration).

## Risks

- **Build-phase gate after adoption (G6).** Adoption could block `verdi build start` for all later work. G6 is last and proves the
  gate first; if it blocks, G6 stops and returns to the owner.
- **RWS census growth.** More mutating verbs mean more declarations or fixes; R4 carries a conditional extra lane.
- **Load.** The machine is CPU-saturated during full gates; T1 and the three-lane cap are the mitigation, and timing-sensitive
  tests are re-run in isolation before being treated as failures.
- **The closure limit.** Stories built this way are eligible to close but not closable today (see "What 'complete' means").

## Source coverage

| Source | Destination |
|---|---|
| Owner decisions 2026-09-21 (spike wave report) | "Owner decisions carried in" |
| Owner decisions 2026-09-23 (D-PH-1..6) | same section; A2–A5; lanes |
| Spike findings per feature (sizing, pinned files, registries, gaps) | Serialization rules; lanes table; risks |
| BL-3, BL-25, BL-26, BL-27, BL-28 | D-PH-1; A2; A4; A3; A1 and RWS lanes |
| X-1 (phase B wave 1 finding; backlog row there) | D-PH-6; P3 |
| Plan review of 0c941931, F1 (stories need an accepted parent) | D-PH-1; step 1 acceptance order; A1–A4 split into feature and story units |
| Plan review F2 (clause-bearing successor before its decoder) | A3a/A3b/A3c; V1's merge point; V3 after A3c |
| Plan review recommendations (strict-lint milestone; bare feature name; clause evidence-kind contract) | M-SLT row and waves; D-PH-1 command; A3a |

Coverage: every listed source item is mapped. Intentional omissions: mutation-ratchet (not doing) and closure (BL-44).
