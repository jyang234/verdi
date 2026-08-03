# Four-Feature Owner Adjudications

**Status:** Owner decisions recorded 2026-08-03; authoritative upon merge of
this pull request.

## What this document is

This is the repository-visible, owner-ratified adjudication of decisions
OD-1 through OD-12 as enumerated in §9 of the merged cross-feature authority
audit packet,
`docs/superpowers/plans/2026-08-03-cross-feature-authority-audit.md`
(landed via PR #264 at first-parent default-branch commit `c99acbf3`; blob
object ID at this branch's base commit `df468c923d2ff9f0c76ae8fab8b2f2b9d58e424e`
is `b4011e2f6f4b3beb03fe76dd5ab4950e140f7197`). That packet's §9 states
plainly that none of the twelve items is an implementation choice and that
the packet "deliberately does not resolve them." This document is where they
are resolved: it records, without alteration, the owner's ruling on each of
the twelve.

The owner made these twelve decisions on 2026-08-03.

## Authority event

Consistent with the ratified merge-signaled acceptance design
(`docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md`,
ratified 2026-08-01), the owner's merge of the pull request carrying this
document into the default branch is the single authority event that makes
these twelve rulings effective repository authority. A Codex review of this
record's pull request is a required technical check — it verifies the
record is a faithful, complete, non-expanding transcription of the owner's
rulings — but that review is not itself the acceptance ceremony. No
follow-up `verdi accept` command, status edit, acknowledgement step, or
confirmation is required or permitted to repeat what the merge already
witnesses.

## What this document does not do

This record settles the twelve open decisions named in the audit packet. It
does not itself supply, draft, or materialize any of the following; each
remains its own owner-merged deliverable, produced under its own review:

- the concrete kernel contract — the joint GLG/CI amendment named by OD-2
  and tracked as the audit packet's recommendation R-5;
- the component-spec amendments those rulings require, including the
  `verdi-store-layout` committed-zone amendment named by OD-12;
- the successor invention ledger named by OD-9, including its import
  content;
- the `CLAUDE.md` instruction amendment named by OD-10;
- any runtime implementation — code, schemas, or generated artifacts — that
  these rulings authorize.

This document decides; it does not implement. It does not define the
`execution-workspace` component's schema, does not enumerate ledger
entries, does not draft `CLAUDE.md` wording, and does not assert that the
governance-principal kernel exists. Each of those is separate, still-future,
owner-merged work.

## Three-valued honesty register

Following the program's proven / violated-with-witness / disclosed-unproven
doctrine, this record's own status must be read precisely:

**Proven, upon this document's merge:** the twelve owner rulings below are
settled. Each of OD-1 through OD-12 has an authoritative answer, recorded
verbatim, attributable to the owner, and dated 2026-08-03. Nothing further
is required to make the *decision* — as distinct from its implementation —
authoritative.

**Disclosed as unproven — concrete authority materialization still
pending:** the R-5 joint GLG/CI kernel contract amendment; the
`verdi-store-layout` component-spec amendment covering the four features'
new artifact paths; the successor invention ledger
`docs/superpowers/invention-ledger.md`; the `CLAUDE.md` instruction
amendment naming the successor orchestration authority and ledger. None of
these exists as landed authority yet. Each requires its own owner-merged
pull request before it can be cited as binding.

**Disclosed as unproven — runtime implementation still pending:** every
Wave 1 and later implementation lane these rulings unblock — the kernel
itself, the `execution-workspace` component, ASD's attribution-schema
adoption, CSE's actor and ratification handling, CI's policy-authority
consumption, and all other code. No ruling in this document is code, and
none of it should be read as evidence that any such code exists.

## The rulings

The twelve rulings below are recorded exactly as the owner supplied them.
Each ruling's normative text is unmodified, unabridged, and unparaphrased.
The "Settles" line under each ruling cites the audit-packet rows it
resolves and is non-normative context; it does not alter, extend, or narrow
the ruling above it.

### OD-1 — Profile artifact home

Governance-profile artifacts live in Context Integrity's constitution store
as typed policy artifacts. The kernel owns their schema and resolution
behavior, not a separate storage silo.

Settles: CX-1 (governance-profile schema duplication); CX-5's storage half
(policy-inheritance mechanism duplication, as it bears on profile
artifacts).

### OD-2 — Kernel contract vehicle and custody

One owner-merged PR jointly amends GLG and CI. GLG owns lifecycle-wide
requirements; CI owns recording and enforcement; the governance-principal
kernel owns implementation. Interim semantic changes require owner-ratified
amendments and cannot be made unilaterally by the kernel implementation
plan.

Settles: CX-2 (kernel contract custody in the interim). This ruling also
fixes the vehicle for audit-packet recommendation R-5.

### OD-3 — ASD actor upgrade

ASD adopts the kernel attribution representation during canonical
promotion. It may not first ship a bare `principal` schema requiring later
migration. Attribution contains either a kernel canonical principal
identity or an explicit unauthenticated marker; it never becomes authority
merely by naming a principal.

Settles: CX-4's ASD half (authoritative principal vs. attribution records).

### OD-4 — CSE actor and provenance treatment

CSE mutation-provenance actors are attribution records. Their scope is
explicitly the CSE experiment mutation surfaces. `ratification.yaml`
actors are principal-resolution class and must resolve to authenticated
kernel principals; unproven authentication blocks ratification.

Settles: CX-4's CSE half (authoritative principal vs. attribution records);
pins the CX-9 experiment-surface scope question (provenance/audit record
shapes) to this explicit reading.

### OD-5 — Policy representation and storage

ASD and CSE use typed feature-specific policy payloads inside Context
Integrity's single policy-authority system. Context Integrity exclusively
owns policy storage, inheritance, effective-policy resolution, and policy
identity/digest. No feature-local fallback, competing hierarchy, or second
policy interpretation is permitted.

Settles: CX-5 (policy-inheritance mechanism duplication), for both ASD's
narrower representation question and CSE's previously open
policy-hierarchy choice.

### OD-6 — Shared isolation boundary

Create a narrowly scoped shared `execution-workspace` component
specification for CI and CSE. It owns exact workspace materialization,
application of isolation controls and execution grants, environment
fingerprint collection, and safe cleanup. It reuses existing
`internal/wtmanager`, `internal/gitx`, and `internal/filelock` primitives
but does not broaden the closed local-design-branch-only
`worktree-manager` story. Policy decisions and feature proof semantics
remain outside this component.

Settles: CX-6 (reusable worktree/isolation boundary); CX-7 (environment-
identity collection duplication).

### OD-7 — Capability terminology

ASD capabilities mean adapter-surface discovery. CI/CSE capabilities mean
execution grants from one shared strict vocabulary. Existing public names
may remain where compatibility requires, but documentation and authority
contracts must preserve the semantic distinction.

Settles: CX-8 (capability vocabulary duplication).

### OD-8 — ASD sidecar classification

The ASD promotion lands the sidecar classification contract and binds the
CI context compiler. The exclusion reason code is
`design-provenance-sidecar`.

Settles: CX-10 (ASD sidecar classification by the CI compiler); CX-11
(context include/exclude vocabulary duplication), for the sidecar-naming
half of that duplication.

### OD-9 — Successor invention ledger

Choose repository-visible Option A. Create
`docs/superpowers/invention-ledger.md` through a separate local-lane PR,
with a complete provenance-preserving import of the existing
PLAN/PLAN-V1 §7 invention history — not a cited-only subset.

Settles: CX-12's ledger branch (repository-visible invention ledger and
successor instructions).

### OD-10 — Instruction amendment

A separate PR amends repository `CLAUDE.md` to name the successor
orchestration authority and invention ledger, preserve three-valued
honesty and the 0/1/2 exit doctrine, and declare CLI/MCP inventories
serialized shared registries. Do not create another hand-authored
`AGENTS.md`.

Settles: CX-12's instructions branch (repository-visible invention ledger
and successor instructions); CX-15 (three-valued honesty and exit codes,
as it bears on the instruction surface); CX-19 (CLI-verb and MCP-tool
inventory arbitration); and audit-packet recommendations R-3 and R-4.

### OD-11 — CSE ratification transport

Merge is the sole transport witness; no post-merge ratification command or
status change exists. In solo mode, the registration lock may ride its
natural registration PR and ratification may ride its natural
results/recommendation PR. Lock and ratification remain temporally
distinct, digest-bound human judgments. Team and high-assurance profiles
may require dedicated PRs or distinct principals.

Settles: audit-packet §6 human-ceremony rows H-12, H-13, and H-14 (CSE
registration lock, CSE ratification of a comparison result, and CSE
ratification transport).

### OD-12 — Store-layout amendment

Land one early shared amendment through the ratified `verdi-store-layout`
component-spec flow, covering all four features' new artifact paths.
Runtime implementation remains divided among the owning feature units.

Settles: CX-17 (committed store-layout ownership for new artifact kinds).

## Dependency statement for downstream promotion authoring

Before authoring either the ASD canonical-promotion plan
(`docs/superpowers/plans/2026-08-03-asd-canonical-promotion.md`) or the CSE
canonical-promotion plan
(`docs/superpowers/plans/2026-08-03-cse-canonical-promotion.md`) may
proceed past authority already landed at the time of this writing, the
following must hold:

- this adjudication record itself must be owner-merged;
- the R-5 joint GLG/CI kernel contract (OD-2) must be owner-merged;
- the shared store-layout/component authority (OD-12, and the
  `execution-workspace` component named by OD-6 where a given promotion
  depends on it) must be owner-merged;
- the successor ledger decision (OD-9) must be materialized as landed
  authority, not merely chosen here;
- each promotion must consume landed authority — the amendments and
  component specs as merged — never the audit packet's proposals directly,
  since the packet itself proposes and ratifies nothing.

A promotion plan that cites the audit packet's §2–§5 proposal text, or this
record's rulings, in place of the corresponding landed amendment is citing
an unproven source and must disclose that gap rather than proceed as if the
amendment already existed.
