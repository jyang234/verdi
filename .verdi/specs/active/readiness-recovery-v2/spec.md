---
id: spec/readiness-recovery-v2
kind: spec
title: "Continuous Readiness and Lifecycle Recovery"
owners: [platform-team]
class: feature
problem: { text: "Verdi tells an operator what blocks the next step but not what they already owe for closure: eventual closure blockers are an undelivered stub in the journey, readiness is a startup photograph of one spec on its own design branch that goes stale until verdi serve restarts and never reaches the CLI, feature outcome attestations have no scaffold, no lint protection, and a board chip that probes the wrong directory, and a ritual interrupted mid-run leaves partial branch, index, archive, publication, or lock state that only verdi close can name and nothing can safely undo.", anchor: problem }
outcome: { text: "From feature acceptance onward every journey carries current and eventual closure blockers as named debts with owners and clearing conditions; readiness is derived per request for any spec on any branch and reads byte-identically on the CLI, the board's readiness page and Document tab, and MCP; a feature outcome attestation is scaffolded, linted, and shown exactly like a story attestation while the human claim stays human-authored; and verdi recover names every recognized interrupted state with its facts, uncertainties, and safe choices, executing only the two existing reversible or confirmed executors and disclosing manual commands for the rest.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "From feature acceptance onward the journey record derives its eventual closure blockers (Derived true, the undelivered-unit disclosure gone) from already-declared requirements only: unreconciled stubs, feature criteria whose outcome floor has neither an authored outcome attestation nor a passing outcome record, unelaborated obligations, open questions and unresolved conflict rows, exceptions past or within their review window, roles with no authenticated owner under the selected profile, and absent authoritative receipts; each is a Blocker with reason, class, witnesses, owner, clearing condition, and the transition it will block, never a prediction that evidence will fail and never a present violation.", evidence: [static, behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "Readiness is derived per request by one loader for any active spec on any branch: the startup adapter moves out of cmd/verdi into a shared package, the design-branch gate is removed, branch and HEAD come from the journey record, the snapshot keeps its type and concern grammar, and its stale notice becomes a derivation stamp naming the HEAD the request looked at; the same ref at the same HEAD derives identical bytes twice.", evidence: [behavioral, attestation], anchor: ac-2 }
  - { id: ac-3, text: "A readiness derivation never launches the policy-conflict judge: the check-context area evaluates mechanically and takes semantic judgments only from the existing judge cache; an uncached semantic candidate is an unproven context/semantic concern whose destination is the context conflict verb; verdi serve --context-request remains an optional startup pre-run of the full evaluation.", evidence: [static, behavioral, attestation], anchor: ac-3 }
  - { id: ac-4, text: "Readiness reads byte-identically for the same ref at the same HEAD and the same context-request input on the CLI (verdi spec doc supplies the Readiness section for the spec and tasks kinds by default, --no-readiness omits it), the board readiness page (the startup spec when --context-request is given, ?spec=<name> otherwise, the existing disclosure page when neither), the board Document tab, and MCP get_document; the CLI takes no request, so for the startup request's own spec a served reading derived through that request (ac-3) and the CLI's reading derived without one answer different questions and may differ in exactly the Readiness section, the request-free side carrying the fixed no-request witness and the served side disclosing a request whose expected-repository claim has gone stale; every other spec derives without a request on every surface; a pinned-commit render still carries none; the four-way parity test gains a readiness arm that also pins that one divergence; verdi journey --json carries the derived eventual section.", evidence: [behavioral, attestation], anchor: ac-4 }
  - { id: ac-5, text: "The wall shell keeps its own derivation in this feature, and a committed parity witness lists, for the e2e harness fixture, every concern the continuous derivation produces that the wall does not and the reverse, pinned as an explicit gap list the post-design workbench lane inherits, never reported as parity.", evidence: [static, attestation], anchor: ac-5 }
  - { id: ac-6, text: "verdi attest <feature-ref> <ac-id> scaffolds attestations/<feature-name>/<ac-id>.md through the existing scaffold renderer with the same unauthored marker, fixed instructional body, self-validation before write, and create-only file posture as the story form; the body may quote the criterion's accepted text, its declared evidence kinds, and the feature's declared owners and never contains a claim-shaped sentence; the marker stays until a human replaces it.", evidence: [behavioral, attestation], anchor: ac-6 }
  - { id: ac-7, text: "Feature outcome attestations get the story form's review ergonomics: the board attestation chip derives its path from the feature's own name instead of the story slug, VL-022's mis-slug protection covers feature-targeting attestations with the same rule shape, the feature-close preflight's unauthored-scaffold disclosure has a producer, and a feature criterion whose outcome floor is unsatisfied is an eventual blocker whose witness is the criterion, whose owner is the feature's owners, and whose clearing condition names the attestation path or a passing outcome record.", evidence: [behavioral, attestation], anchor: ac-7 }
  - { id: ac-8, text: "verdi recover <ref> emits a canonical read-only projection (schema verdi.recovery-projection/v1: ref, branch, HEAD, recognized states) where every recognized state names its code, identifying facts, uncertainties with the witness that would settle each, steps the interrupted ritual completed, invariants that still hold, and choices; the recognized states are an empty branch cut, a scaffold written but not staged, artifacts staged but not committed, an archive move without its commit, a closure committed but unpublished or unpushed, a failed push after a board commit, a stale writer or worktree lock, and an interrupted governed action; ambiguous state yields diagnosis and a witness request, never a guess; exit 0 when nothing is recognized, 1 when something is, 2 operational.", evidence: [static, behavioral, attestation], anchor: ac-8 }
  - { id: ac-9, text: "Every recovery choice states preconditions, effects, reversibility, confirmation requirement, postconditions, and either its executor or the exact manual command when none exists; exactly two choices execute, only through verdi recover <ref> --apply <choice-id> naming the exact target: unwinding an empty branch cut (re-proving at execution time that the branch still points at its cut point, has no commits of its own, and the tree is clean, then returning to the original branch and deleting it with the sequence close already uses) and delegating stranded worktree or branch residue to the existing reclaim plan under its own dry-run and --apply; every other state is diagnosis with manual commands disclosed as having no executor; no new git primitive is added and no recovery run's git command log contains reset, restore, clean, stash, --force, or update-ref.", evidence: [static, behavioral, attestation], anchor: ac-9 }
  - { id: ac-10, text: "After an applied choice, verdi recover re-derives the projection and the journey and prints the postconditions; a choice whose preconditions no longer hold at execution time is refused with nothing changed; the read-only MCP tool get_recovery(ref) returns the projection and no MCP path can apply a choice.", evidence: [behavioral, attestation], anchor: ac-10 }
constraints:
  - { id: co-1, text: "No network in any test; git through internal/fixturegit; CLI paths through the built binary; MCP paths through hermetic transcripts; interruption cases through the existing forced-fault seams, step-named crash hooks, stale-lock simulation, and killed built-binary runs, never a new mechanism.", anchor: co-1 }
  - { id: co-2, text: "Readiness and recovery are projections of facts that already exist: no readiness file, cache, status field, transition, receipt, event log, or artifact kind is added; no recovery lifecycle state is invented; a projection is never authority.", anchor: co-2 }
  - { id: co-3, text: "The write surface stays closed: the only mutation this feature adds is verdi recover --apply over the two existing executors; MCP gains read-only tools only; Verdi and agents never author or suggest a human attestation claim.", anchor: co-3 }
  - { id: co-4, text: "No workbench markup, CSS, or JS changes beyond the readiness page's derivation stamp line and its ?spec query; the wall shell and every recovery and attestation presentation belong to the post-design Fable lane.", anchor: co-4 }
  - { id: co-5, text: "Every new production literal routes class and status words through the model display chain or is classified at the producing site; the vocabulary witness is never weakened.", anchor: co-5 }
  - { id: co-6, text: "Three-valued honesty holds in every projection: an unavailable fact is stated, an undecidable state is disclosed with the witness that would decide it, and an unexecutable choice says it has no executor.", anchor: co-6 }
decisions:
  - { id: dc-1, text: "Two loaders beside the existing packages, not one application layer: internal/readinessload mirrors specdocload and serves every surface; internal/recovery owns the projection schema and executor wiring; journey owns the eventual derivation; evidence owns the feature scaffold. Chosen over an internal/journeyapp composition (four concerns in one package) and over folding readiness and recovery into the journey record (a declared-requirement projection and a residue diagnosis are different things).", anchor: dc-1 }
  - { id: dc-2, text: "Attestation symmetry is CLI-level in this feature (scaffold verb, chip, lint, preflight, readiness); the workbench authoring flow for both classes lands in the Fable lane after the owner's design pass so presentation is built once over the final API.", anchor: dc-2 }
  - { id: dc-3, text: "The three snapshot consumers and the CLI switch to the continuous loader; the wall shell keeps its derivation with a pinned gap witness (ac-5). Replacing the wall's grammar now would pull the workbench lane ahead of the redesign.", anchor: dc-3 }
  - { id: dc-4, text: "Recovery executes exactly two existing executors and nothing else: the branch-cut unwind close already proves, and delegation to the reclaim plan's dry-run and --apply. Staged-but-uncommitted and archive-without-commit states get diagnosis and manual commands because no reversible primitive for them exists and inventing one (git restore) would violate the parent's no-invented-recovery rule. Legibility is the aim of this feature (owner, 2026-09-19): every recognized state must be explained completely before any executor is considered, and a later feature may add executors behind the same projection.", anchor: dc-4 }
  - { id: dc-5, text: "Per-request readiness must not run an external process: the judge cache is the only semantic source at request time; the deliberate judge run stays in verdi context conflict and the optional serve pre-run.", anchor: dc-5 }
  - { id: dc-6, text: "Risk tiers: Tier 3 with an owner risk gate for ac-9 and ac-10's executable choices; Tier 2 for ac-1 through ac-4, ac-6, ac-8; Tier 1 for ac-5 and ac-7's lint and chip fixes. Waves: 1 = ac-1..ac-5 (readiness); 2 = ac-6, ac-7 (attestations); 3 = ac-8..ac-10 (recovery); each ends with the full gate; the integration unit of the orchestration index follows against the merged head.", anchor: dc-6 }
  - { id: dc-7, text: "Recognized recovery states are the ones the ritual inventory proves the lifecycle leaves behind (design start, build start, close, policy adopt, board commit and push, locks, draft-mutation journals, execution workspaces, constitution proposals); a state outside that inventory is unrecognized and reported as such, never classified by resemblance.", anchor: dc-7 }
stubs:
  - { slug: eventual-blockers, acceptance_criteria: [ac-1] }
  - { slug: readiness-loader, acceptance_criteria: [ac-2, ac-3] }
  - { slug: readiness-parity, acceptance_criteria: [ac-4, ac-5] }
  - { slug: feature-attest-scaffold, acceptance_criteria: [ac-6] }
  - { slug: feature-attest-review, acceptance_criteria: [ac-7] }
  - { slug: recovery-projection, acceptance_criteria: [ac-8] }
  - { slug: recovery-executors, acceptance_criteria: [ac-9, ac-10] }
links:
  - { type: depends-on, ref: "spec/guided-lifecycle-governance-v3" }
  - { type: depends-on, ref: "spec/spec-documents" }
  - { type: depends-on, ref: "spec/gc-reclaim" }
  - { type: depends-on, ref: "spec/close-preflight" }
  - { type: supersedes, ref: "spec/readiness-recovery" }
supersession:
  carried: [ac-1, ac-2, ac-3, ac-5, ac-6, ac-7, ac-8, ac-9, ac-10, co-1, co-2, co-3, co-4, co-5, co-6, dc-1, dc-2, dc-3, dc-4, dc-5, dc-6, dc-7]
  amended:
    - { id: ac-4, note: "readiness parity is defined over equivalent context-request inputs: the startup request's own spec may differ between a served surface deriving through the request (ac-3) and the request-free CLI in exactly the Readiness section, disclosed by the no-request and stale-request witnesses; the predecessor promised the four surfaces identical bytes without qualifying the input, which no conforming implementation could meet because spec/spec-documents ac-2 gives verdi spec doc no request flag (independent review 2026-09-21 R4; ledger SI-215; owner chose the amendment 2026-09-21)" }
  amended_advisory: []
  removed: []
  added: []
---
# Continuous Readiness and Lifecycle Recovery

## Problem

Verdi answers "what blocks the next step" and stops there. The journey's
eventual-blocker section is a stub whose disclosure names this feature as its
supplier, so closure debt stays invisible until closing fails. Readiness is a
photograph taken when `verdi serve` starts, for one spec, only on that spec's
design branch, stale until a restart, absent from the CLI. Feature outcome
attestations are hand-authored files with no scaffold, a lint rule that skips
them, and a board chip that probes the story slug. And when a ritual dies
between its branch cut and its commit, only `verdi close` can name the residue,
and nothing can safely undo the most common case.

## Outcome

Every accepted feature's journey carries its current and eventual closure
blockers as named debts. Readiness is a live, per-request derivation that reads
the same on the CLI, the board, and MCP. A feature outcome attestation is
scaffolded, linted, and shown like a story attestation, with the claim still
human-written. `verdi recover` explains every recognized interrupted state and
executes only what already exists to execute.

## ac-1

The eventual derivation reads declared requirements (DC-11 of the parent): each
source is an artifact or gate that exists today; the blocker names the future
transition it will block and the owner who clears it.

## ac-2

One loader, any spec, any branch, per request. The type and grammar stay so the
three existing consumers change by one line: the stamp.

## ac-3

A page render never launches a process. The judge runs where it is asked for.

## ac-4

Parity extends the spec-documents contract to readiness, over equivalent
inputs. `--no-readiness` exists so a render can be pinned without it. The CLI
has no context-request flag (spec/spec-documents ac-2 declares its flag set)
while `verdi serve --context-request` is a served-only startup pre-run (ac-3),
so the startup request's own spec is the one ref whose served and CLI readings
answer different questions; the difference is confined to the Readiness
section and each side says why — the fixed no-request witness on the CLI, or
the stale-request witness once the request's expected-repository claim no
longer matches the checkout. The predecessor's ac-4 promised the four surfaces
identical bytes without qualifying the input; the shipped parity test pinned
the divergence as a recorded conflict (ledger SI-215) until this amendment.

## ac-5

The wall is the one deliberate lag, written down as a gap list.

## ac-6

The scaffold is structure and questions, never the claim (DC-12 of the parent).

## ac-7

Symmetry is ergonomic: the chip, the lint rule, the preflight, and readiness
treat a feature attestation exactly as they treat a story attestation.

## ac-8

Diagnosis first, from observable Git and Verdi facts (DC-13 of the parent).
Unknown or conflicting state is a witness request, not a mutation.

## ac-9

Exactly two executors, both already proven elsewhere; everything else is manual
commands with the executor gap disclosed.

## ac-10

A choice proves where it starts and verifies where it ended. MCP reads only.

## co-1

No network in any test; interruption proofs reuse existing seams.

## co-2

Projections, not authority; no invented state.

## co-3

The write surface stays closed.

## co-4

No presentation work before the design pass.

## co-5

Vocabulary discipline holds for every new literal.

## co-6

Three-valued honesty in every projection.

## dc-1

Two loaders, not one application layer.

## dc-2

CLI-level attestation symmetry now; workbench flow after the design pass.

## dc-3

Snapshot consumers go continuous; the wall waits with a gap witness.

## dc-4

Two executors only. Legibility first: the projection is the deliverable;
executors are conveniences added only where already proven.

## dc-5

No external process from a request.

## dc-6

Risk tiers and waves.

## dc-7

Recognized states come from the ritual inventory.
