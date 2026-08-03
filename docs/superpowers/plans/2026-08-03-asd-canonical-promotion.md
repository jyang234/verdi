# ASD Canonical-Promotion Mapping and Implementation Plan

> **For agentic workers:** This is a planning packet for the future ASD
> canonical-promotion delivery unit (four-feature orchestration, Wave 0,
> lane W2). It authorizes no runtime code, no canonical-spec bytes, and no
> lifecycle mutation. The promotion unit it describes may execute only
> after this plan receives a read-only Codex review and its recorded
> inventions are ratified through that review. Steps use checkbox
> (`- [ ]`) syntax for tracking.

**Goal:** Map every reviewed decision of the ratified ASD design document
into one canonical Verdi feature proposal artifact, losslessly, and define
the exact future pull request that lands it.

**Architecture:** The ASD design document
(`docs/superpowers/specs/2026-07-30-ai-assisted-spec-design.md`, blob
`8595721911c3c756458ace195a686a871b6410d3`) is ratified design authority.
Its canonical form is a statusless, merge-accepted feature-class spec at
`.verdi/specs/active/ai-assisted-spec-design/spec.md`, following the exact
conventions of the two existing canonical feature artifacts
(`spec/context-integrity`, `spec/guided-lifecycle-governance`). Promotion
is one reviewed pull request whose merge is acceptance under the
merge-signaled lifecycle; no `verdi accept`, status field, or frozen stamp
is written.

**Tech Stack:** Verdi filesystem store (`.verdi/specs/active/`), strict
frontmatter dialect (`internal/artifact`), `internal/lint` VL-001..VL-022,
`internal/specstate` Git-derived lifecycle projection, `internal/specalign`
inventories, GitHub `merge-gate` required check.

## Global constraints

Copied from the binding authority; every task below implicitly includes
them.

- Binding semantics remain `docs/design/specs/00..05-*.md`;
  `08-revision-notes.md` is the ratification history. Never edit frozen
  artifacts or binding specs directly.
- Promotion must "preserve their reviewed decisions, constraints,
  non-goals, and rollout ordering in canonical artifact form" and must not
  add new semantics (four-feature orchestration, lines 92 and 236).
- The owner's merge of the promotion pull request is the single acceptance
  ceremony (merge-signaled acceptance design, ratified 2026-08-01). No
  second acceptance action of any kind.
- Never import `verdi-go` packages; exec pinned CLIs and strict-decode
  their JSON only.
- Three-valued honesty: proven, violated with witness, or disclosed as
  unproven. Silence is never a pass.
- Strict decode everywhere; unknown fields, operations, and enum values
  fail closed. Exit contract 0/1/2 preserved.
- No test uses the network. Spec-only changes still pass the full
  `merge-gate` (`make verify`) in CI.
- Claude Code never accepts a specification, approves a pull request,
  merges, or authors a human attestation; the owner alone merges.
- Unresolved semantic ambiguities are recorded (invention discipline) and
  block implementation rather than being silently resolved.

## Contents

1. [Verified authority identities](#1-verified-authority-identities)
2. [Proposed canonical identity and path](#2-proposed-canonical-identity-and-path)
3. [Complete proposed artifact structure](#3-complete-proposed-artifact-structure)
4. [Source-anchored mapping and losslessness table](#4-source-anchored-mapping-and-losslessness-table)
5. [Cross-feature dependencies](#5-cross-feature-dependencies)
6. [Provenance-sidecar identity and exclusion contract](#6-provenance-sidecar-identity-and-exclusion-contract)
7. [Future implementation steps](#7-future-implementation-steps)
8. [Human-ceremony inventory](#8-human-ceremony-inventory)
9. [Three-valued disclosure](#9-three-valued-disclosure)
10. [Semantic gaps and contradictions](#10-semantic-gaps-and-contradictions)
11. [Decision record — inventions requiring ratification](#11-decision-record--inventions-requiring-ratification)

---

## 1. Verified authority identities

Verified at plan time from the isolated worktree (branch
`agent/asd-canonical-promotion-plan`, base commit
`6d71fd7d33beaf8128fa675833ee12595205481d` = `origin/main` at fetch time):

| Authority | Path | Verified identity |
|---|---|---|
| Accepted landing commit | — | `6d71fd7d33beaf8128fa675833ee12595205481d` (merge of PR #258) |
| ASD design (source of every mapped element) | `docs/superpowers/specs/2026-07-30-ai-assisted-spec-design.md` | blob `8595721911c3c756458ace195a686a871b6410d3` |
| Context Integrity canonical spec (structural template, dependency) | `.verdi/specs/active/context-integrity/spec.md` | blob `18a2b92b7c92b5ba807336a13cb38c9e0d9a406c` |
| Guided Lifecycle canonical spec (structural template, dependency) | `.verdi/specs/active/guided-lifecycle-governance/spec.md` | blob `c347668014d26f987d38fbd9dca0082228238694` |
| Orchestration index (sequencing authority) | `docs/superpowers/plans/2026-08-01-four-feature-orchestration.md` | at landing commit |
| Merge-signaled acceptance design (acceptance mechanism) | `docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md` | at landing commit |

Line references to the ASD design below use the blob above; the file is
unchanged at the landing commit.

## 2. Proposed canonical identity and path

**Proposed identity:**

- `id: spec/ai-assisted-spec-design`
- Path: `.verdi/specs/active/ai-assisted-spec-design/spec.md`
- `kind: spec`, `class: feature`, `title: "AI-assisted spec design"`,
  `owners: [platform-team]`
- No persisted `status:` field and no `frozen:` stamp — lifecycle state is
  Git-derived (`internal/specstate`): proposed while the promotion PR is
  open, accepted-pending-build once the owner's merge lands the exact
  revision on `main`.

**Justification against existing conventions:**

- Both existing canonical feature artifacts derive their directory name as
  a kebab-case condensation of their title (`context-integrity`,
  `guided-lifecycle-governance`); the ASD title "AI-assisted spec design"
  condenses to `ai-assisted-spec-design` with no information loss.
- VL-002 requires directory name = ref name and id/path agreement; the
  single-file `spec.md` layout matches both precedent directories (each
  currently holds only `spec.md`).
- Statusless frontmatter matches the normalization precedent: commit
  `ae183b15` deleted exactly the `status: draft` lines from both canonical
  proposals before the acceptance merge; `internal/lint` VL-004 and
  `internal/specstate` already handle statusless feature specs.
- `owners: [platform-team]` matches both precedent artifacts.

Naming is a semantic invention (the design document never names its own
canonical ref) and is **not decided silently**: see [§11 N-1](#11-decision-record--inventions-requiring-ratification)
for the alternatives considered. It binds only when this plan's Codex
review approves it and the owner merges the promotion PR.

## 3. Complete proposed artifact structure

The artifact follows the exact skeleton shared by
`spec/context-integrity` and `spec/guided-lifecycle-governance`:
frontmatter objects with `ac-n`/`dc-n`/`co-n` IDs, each anchored to its
body heading (`## AC-1`, `## DC-1`, `## CO-1` — slug-symmetric anchor
resolution: `anchor: ac-1` resolves `## AC-1`), body sections `# Title`,
`## Problem`, `## Outcome`,
per-AC sections, relationship and delivery-sequence sections, per-DC and
per-CO sections, and `## Non-goals`.

### 3.1 Frontmatter attributes

- `problem`: Verdi's authoring surfaces expose separate, unequal paths —
  the workbench authoring API mutates draft specs, MCP's only write tool
  is `add_annotation`, and direct Markdown carries no provenance — so an
  external agent cannot perform the same declared spec edits as a human,
  and there is no common guarded mutation contract, no bounded design
  provenance, and no policy boundary distinguishing delegated agent
  authorship from human governance authority. (Sources: ASD 58–81.)
- `outcome`: one typed, digest-guarded, atomic draft-mutation core serves
  workbench, CLI, and MCP identically; delegated agents author drafts
  under ratified policy while acceptance, approval, waivers, attestations,
  and dispositions remain human-only; bounded classified provenance lands
  in a committed non-authoritative sidecar excluded from normal build
  context; direct Markdown and fully AI-free journeys remain first-class;
  and Codex and Claude Code consume one published capability and mutation
  contract. (Sources: ASD 26–56, 713–739.)

### 3.2 Acceptance criteria (proposed)

Feature ACs are outcome-level and implementation-blind; every list
includes `attestation` (VL-006 outcome floor). Grouping of eleven of the
design's twelve success measures (ASD 713–734) into eight ACs is
invention N-2 — the twelfth (SM-12, Jira separation) lands in dc-19;
evidence-kind declarations are invention N-3 (§11).

| id | text (condensed; full anchored prose in body) | evidence |
|---|---|---|
| ac-1 | a human or a delegated agent can construct or modify a draft specification through one typed, digest-guarded, atomic draft-mutation transaction whose complete result is strict-validated before write, whose ordered batch lands entirely or not at all, and whose typed change can never become visible without its matching provenance record, including across crash recovery | static, behavioral, attestation |
| ac-2 | workbench, CLI, and MCP adapters expose identical mutation semantics over the shared core — given the same base document, actor authority, policy, and operations, each produces byte-identical resulting spec.md, provenance record, digests, semantic diff, and warnings — and the migrated workbench retains no parallel interpretation of domain mutations | static, behavioral, attestation |
| ac-3 | agent participation is governed by ratified project policy across exactly three modes (off, proposal-only, draft-write) with capability discovery declaring the active schema, policy and project digests, checkout identity, spec state, and permitted operations; agent actors remain distinguishable from their delegating human principal; and every governance operation is absent or refused through every agent surface | static, behavioral, attestation |
| ac-4 | typed mutations append bounded, classified, content-addressed provenance to the committed non-authoritative design-provenance sidecar that follows the spec into archive, is excluded from normal design and build context, never counts as evidence or acceptance input, and surfaces direct Markdown edits as unclassified origin rather than fabricating attribution | static, behavioral, attestation |
| ac-5 | an assisting agent can obtain bounded, inspectable design context — the current draft, explicitly selected parent or children, applicable policies and ratified decisions, declared pinned references, Verdi-go-derived findings, and the context and policy digests — while provenance excerpts stay excluded by default and corpus content remains data, never instructions | static, behavioral, attestation |
| ac-6 | before acceptance a human can derive a concise semantic review packet — a view, never a persisted approval artifact — exposing semantic changes since the review base, ai-inferred and unresolved objects, unclassified direct edits, and material warnings; the profile-required review of the exact proposed head authorizes merge, and the owner's merge is the single acceptance decision with no second ceremony | behavioral, attestation |
| ac-7 | disabling or declining AI assistance leaves the complete workbench, Markdown, Git, validation, review, and acceptance journey intact; assistance-disabled projects expose no agent write controls; and direct Markdown editing with normal Git review remains a first-class authoring path with honestly weaker provenance | behavioral, attestation |
| ac-8 | Codex-shaped and Claude-Code-shaped callers use the identical published capability and mutation contracts with no harness-specific schema, lifecycle, or second MCP server; human and agent edits interleave without silent overwrite of an unsaved human edit or a stale write; and Verdi-go remains one pinned, strict-decoded integration | behavioral, attestation |

### 3.3 Stubs (proposed)

One stub per delivery unit named by the orchestration index's
specification-index row for ASD (line 89), in the design's own rollout
order (ASD 680–693). Granularity is invention N-4 (§11).

```yaml
stubs:
  - { slug: draft-mutation-core,          acceptance_criteria: [ac-1, ac-4] }
  - { slug: structured-cli,               acceptance_criteria: [ac-1, ac-2] }
  - { slug: workbench-migration,          acceptance_criteria: [ac-2, ac-7] }
  - { slug: design-context-capabilities,  acceptance_criteria: [ac-3, ac-5] }
  - { slug: proposal-only-dogfood,        acceptance_criteria: [ac-3] }
  - { slug: draft-write-enablement,       acceptance_criteria: [ac-1, ac-3] }
  - { slug: review-provenance-views,      acceptance_criteria: [ac-4, ac-6] }
  - { slug: harness-conformance,          acceptance_criteria: [ac-8] }
```

Every AC is covered by at least one stub. Stubs 1–2 both derive from
rollout step R-1 (the core plus its structured CLI adapter, split per the
orchestration index's named delivery units); stubs 3–8 correspond to
R-2..R-7 one-for-one; R-8 (broaden only after chronicles) is retained as
delivery-sequence prose, not a stub — it is a gate condition, not a story.

### 3.4 Links (proposed)

```yaml
links:
  - { type: depends-on, ref: spec/context-integrity }
  - { type: depends-on, ref: spec/guided-lifecycle-governance }
  - { type: depends-on, ref: spec/verdi-artifact-contract }
  - { type: depends-on, ref: spec/verdi-surfaces }
```

All four resolve in the active store (VL-003). Justification:
`context-integrity` owns the human-artifact kernel/renderer seam ASD must
consume and the context compiler that classifies ASD's sidecar;
`guided-lifecycle-governance` owns the lifecycle-wide governance-profile
and authenticated-principal contract behind the shared kernel;
`verdi-artifact-contract` owns the object model, strict dialect, and edge
taxonomy the mutation vocabulary operates on; `verdi-surfaces` owns the
board-as-projection, MCP inventory, and workbench conventions the adapters
extend. The link set is invention N-5 (§11).

### 3.5 Decisions (proposed, dc-1..dc-20)

Each carries the source decision's normative text (condensed here; body
sections carry the full prose including the rejected alternative that
motivated it):

| id | decision (source) |
|---|---|
| dc-1 | one canonical draft: spec.md remains the semantic source of truth; board, CLI, MCP, review packet, and provenance views are projections or controlled mutation paths, never competing documents; a separate agent proposal layer is rejected as a second source of truth (D-7, D-31) |
| dc-2 | equal domain operations: an authorized agent uses the same typed operations and validation rules as the workbench — no browser automation, no privileged file-rewrite path (D-8, D-33) |
| dc-3 | explicit human delegation: an agent may mutate when the human asks; no per-keystroke or per-mutation confirmation theater for reversible draft edits (D-9, D-32) |
| dc-4 | human-only governance: acceptance, PR approval, waivers, attestations, conflict/deviation dispositions, and equivalent judgment records cannot be authored or performed by an agent (D-10) |
| dc-5 | optimistic concurrency, fail closed: every typed mutation names the digest it read; a stale writer is refused and must reload; Verdi never silently merges two semantic edits (D-11) |
| dc-6 | refined output over conversational residue: normal design and build context contains accepted decisions and declared scope, never the transcript or hidden reasoning; full transcript import is rejected (D-12, D-30) |
| dc-7 | useful provenance, not provenance theater: excerpts jog memory; they are not evidence, instructions, proof of comprehension, or a second source of truth, and provenance is never an acceptance score (D-13, D-36) |
| dc-8 | model- and harness-neutral: Verdi publishes typed contracts and hosts no model, embeds no assistant, and maintains no per-harness integrations; duplicate adapters drift (D-14, D-34, D-35) |
| dc-9 | AI-free remains complete: disabling AI assistance degrades nothing (D-15) |
| dc-10 | mechanics may be deterministic; quality remains judgment: Verdi validates shape, state, links, concurrency, and policy; it cannot score insight or understanding, and no quality score exists to game (D-16) |
| dc-11 | there is no global auto-design mode: an agent acts in response to a human request and cannot cross the governance boundary (D-17) |
| dc-12 | the core is one model-neutral application service inside Verdi; all adapters are thin; a human gesture is normally a one-operation transaction; an agent batch succeeds entirely or not at all (D-18, B-4) |
| dc-13 | the semantic diff reports object-level meaning, never a raw Markdown patch; deletions, large replacements, reordering, and relationship changes are reversible but receive prominent treatment in result and review packet (D-19, C-49) |
| dc-14 | prompt-driven behavior mapping is published guidance, not hidden model policy; enforcement remains the capability and authority boundary (D-20, S-9) |
| dc-15 | the review packet is a derived view, not a persisted approval artifact; the profile-required review of the exact proposed head authorizes merge and the owner's merge is the single acceptance decision; requiring humans to type problem/outcome statements is rejected (D-21, D-22, D-29) |
| dc-16 | bounded, inspectable design context is the first half of the context-poisoning response; the mutation contract carries policy and context identity so the sealed-manifest and contradiction-detection track can later attach without redesign (D-23, Q-2) |
| dc-17 | project policy selects exactly one of off, proposal-only, draft-write; `layout` is reserved for a future presentation extension and remains false in v1 (D-24, NG-2) |
| dc-18 | AI assistance consumes the same Verdi-go-derived information a human designer can inspect — one coherent view of the system (D-25) |
| dc-19 | Jira epic/story provisioning is a deterministic, separately designed track with a recorded likely flow; no silent reverse synchronization from Jira prose into accepted specs (D-26, D-37, NG-3, NG-4) |
| dc-20 | dogfood observations measure system behavior (transactions, refusals, recovery, inferred-object churn, excerpt usage, effort-to-review), never agent scores or volume rewards; success measures demonstrate process coherence, not universal usability (D-27, Q-3) |

### 3.6 Constraints (proposed, co-1..co-9)

| id | constraint (source) |
|---|---|
| co-1 | every refusal is explicit and named: stale base returns the current digest and changed identities; forbidden actor/state/operation names the governing policy; an invalid result returns object-level diagnostics; one invalid operation lands nothing; concurrent writers serialize or refuse, never losing an update; three-valued honesty is preserved throughout (C-34..C-39, C-43) |
| co-2 | strict decode everywhere: the request schema is versioned and strict-decoded; unknown configuration, schemas, fields, operations, and enum values fail closed; extensibility attaches only at the declared internal ports — policy evaluation, mutation application, provenance recording, semantic diffing, change notification, and transport adapters — and project-defined fields are mutated only through declared model descriptors, never a generic YAML escape hatch (S-3, C-26, C-36, C-46, C-28) |
| co-3 | nonconfigurable core safety semantics: only draft specs accept semantic mutations; every mutation requires a base digest; stale mutations never auto-merge; the complete result is validated before write; spec and provenance form one transaction with crash recovery to a complete old or new state; operations address stable object IDs, never positions; agent actors remain distinct from humans; provenance is non-authoritative and build-context-excluded; agents cannot accept, approve, attest, waive, or disposition; a stale agent cannot overwrite intervening human edits; every response names the exact checkout, branch, HEAD, and spec (C-18..C-25, C-11, C-40..C-41, C-50, INV-1, INV-4) |
| co-4 | actor identity is adapter-controlled: the payload cannot promote itself to a human actor via an `author` string; the agent does not self-authorize; no agent can claim to be human or bypass state and policy checks by rewriting through MCP (C-9, C-10, C-44, C-45) |
| co-5 | provenance discipline: no complete transcript import; every excerpt targets a declared object, carries exactly one of human-stated / ai-synthesized / ai-inferred / unresolved, distinguishes verbatim from paraphrase, is bounded, removable, and redactable; excerpts record the target's digest at attach time; direct Markdown edits get `unclassified` origin, never fabricated attribution, and never a retroactive entry; generated provenance never counts as evidence and added volume improves no score (C-12..C-14, C-24, C-42, C-47, C-48, INV-2, P-1..P-3, NG-7) |
| co-6 | policy hierarchy narrows only: a lower-level policy cannot re-enable a forbidden capability or relax a nonconfigurable invariant; policy changes follow normal ratification, produce a new policy digest, and affect future mutations only; existing provenance retains the policy identity under which it was created (C-27) |
| co-7 | the Verdi-go boundary is preserved: exec pinned `flowmap`/`groundwork` CLIs and strict-decode declared outputs; never import packages, add a competing MCP server, duplicate graph or policy semantics, or let a harness bypass the pinned execution and decoding boundary (C-29..C-33) |
| co-8 | corpus and annotation content returned to agents is untrusted data, never instructions, and cannot redefine the mutation or governance contract; harness-global files such as AGENTS.md or CLAUDE.md are never silently represented as Verdi decisions (C-17, C-51) |
| co-9 | every operation carries table-driven happy and negative coverage (wrong state, forbidden actor, stale digest, invalid IDs, illegal links, batch rollback, coordinated-commit crash recovery, direct-Markdown disclosure, archive and exclusion behavior, unknown-anything rejection); the three adapters share a byte-identity conformance suite; browser paths have Playwright coverage including the protected unsaved edit and the AI-free journey; MCP/CLI end-to-end tests cover both harness shapes, all three policy modes, and refused governance operations; no test calls a live model, network service, or harness (C-52, C-53, T-1..T-4, ASD 620–678) |

### 3.7 Body skeleton

`# AI-assisted spec design` · `## Problem` (opening with the provenance
sentence naming the ASD design document and blob `85957219` as source) ·
`## Outcome` · `## Authority model` (the ten-row human/agent/Verdi table,
ASD 121–132, carried verbatim) · `## AC-1`..`## AC-8` with fixed content
ownership — AC-1: the shared-architecture diagram, the nine-step
transaction, the `verdi.draftmutation/v1` request and
`verdi.draftmutation-result/v1` result examples, the closed operation
vocabulary, and the ten-row refusal table; AC-2: the workbench-first flow
and the no-generic-AI-badges rule; AC-3: the `design_assistance` policy
example and the eight-item `get_design_capabilities` field list; AC-4:
the sidecar path, schema, and seven entry fields; AC-5: the
`get_design_context` and build-context content lists; AC-6: the
review-packet content list; AC-7: the Markdown/Git-first six-step flow;
AC-8: the six MCP tools with the equivalent structured CLI, the
unsaved-edit protection, and the branch/worktree identity rule ·
`## Relationship to Context Integrity and Guided Lifecycle and Governance`
(the imposed shared-ownership rules — see §5) · `## Delivery sequence`
(R-1..R-8 verbatim, with the note that each step preserves the AI-free
path and can be disabled without changing spec semantics) ·
`## DC-1`..`## DC-20` · `## CO-1`..`## CO-9` · `## Non-goals`
(NG-1, NG-5..NG-14 verbatim: no embedded chat/model picker/model hosting,
no model prescription, no transcript import, no autonomous governance, no
quality scores, no second proposal document or agent board or canonical
spec, no competing Verdi-go integration or MCP server, no Jira creation,
no sealed build-context manifest or full contradiction detector, no proof
that an external harness has no opaque global memory; plus the v1 core's
own exclusions: no branch switching, committing, pushing, PR operations,
acceptance, alignment, dispositions, evidence publication, or closure, and
no presentation-layout smuggling).

Ownership: `owners: [platform-team]`, matching both precedent artifacts.

## 4. Source-anchored mapping and losslessness table

Element IDs (D-n decisions, SM-n success measures, T-n test inventories,
C-n constraints, S-n schemas/records, INV-n invariants, R-n rollout steps,
NG-n non-goals, B-n surface behaviors, P-n provenance rules, Q-n open
questions) are defined in Appendix A, the exhaustive line-anchored
inventory of the ASD design committed with this plan; anchors are
section + line numbers in blob `85957219`. Transformation
vocabulary: **verbatim** (text carried unchanged into the body),
**condensed** (frontmatter object text compresses the source; the anchored
body section carries the full source prose, so no normative force is
lost), **grounding** (current-state fact carried into `## Problem` as
context; its normative home remains the binding 00–05 specs),
**dependency-prose** (carried into the relationship section §5).

| Source elements | Anchor (ASD lines) | Canonical destination | Transformation | Proof of preservation |
|---|---|---|---|---|
| D-1 (status line) | 1–4 | consumed by the promotion event itself | none — the design doc is not edited; the canonical body's Problem section cites the doc and blob as provenance | the promotion PR body and the artifact's Problem prose name blob `85957219` |
| D-2, D-3 | Purpose 26–39 | `problem`/`outcome` attributes; dc-4, dc-15 | condensed | authorship-vs-authority sentence carried verbatim in Problem body |
| D-4, D-5 | Purpose 41–56 | `outcome`; ac-7, ac-8; dc-12 | condensed | six supported paths enumerated verbatim in Outcome body |
| B-1, B-2, B-3, C-1, C-2, C-3, D-6 | Current-state grounding 58–81 | `## Problem` body | grounding | facts restated as the gap statement; binding source unchanged (02/05 specs) |
| D-7..D-16 (ten principles) | Design principles 83–115 | dc-1, dc-2, dc-3, dc-4, dc-5, dc-6, dc-7, dc-8, dc-9, dc-10 | condensed; body DC sections verbatim | one dc per principle, same order; each MUST/never clause quoted in its DC body |
| S-1, C-4..C-8 | Authority table 119–132 | `## Authority model` body table; ac-3; co-4 | verbatim | ten-row table carried unchanged; the five agent-"no" rows restated in ac-3 text |
| C-9, C-10 | 134–137, 224–226 | co-4 | condensed | "cannot promote itself to a human actor merely by supplying an `author` string" quoted in CO-4 body |
| D-17 | 139–144 | dc-11 | condensed | "no global auto-design mode" quoted |
| D-18, S-2, INV-1, NG-1, B-4 | Shared architecture 146–190 | ac-1; dc-12; co-3; AC-1 body; Non-goals (v1 core exclusions) | verbatim in AC-1 body | the shared-architecture diagram, nine transaction steps, and the atomicity contract carried unchanged; v1 exclusion list carried into Non-goals |
| S-3, S-4, S-5, S-6, C-11 | Mutation contract 192–255 | AC-1 body (schemas + closed vocabulary verbatim); co-2; co-3 | verbatim | `verdi.draftmutation/v1`, the request/result YAML examples, and all seven operation categories reproduced byte-for-byte |
| D-19, C-49 | 257–260, 614–615 | dc-13 | condensed | "conspicuous in semantic review and ordinary Git review" quoted in DC-13 body |
| C-12, P-2 | 262–266, 313–316 | ac-4; co-5 | condensed | "Verdi must not fabricate agent or human attribution" quoted; allow/disclose/block posture carried in AC-4 body |
| S-7, S-8, P-1 | Provenance 268–291 | ac-4; co-5; §6 of this plan; AC-4 body | verbatim | sidecar path, schema id, all seven entry fields, and the exclusion sentence carried unchanged |
| C-13, C-14, INV-2 | 293–311 | co-5; AC-4 body | verbatim | four classifications enumerated; attach-time digest rule quoted |
| B-5..B-10, C-15 | External agent experience 318–339 | AC-8 body (six-tool list + structured CLI equivalence); dc-8 | verbatim | tool list and "no Codex-specific spec schema, Claude-specific lifecycle, or second MCP server" carried unchanged in AC-8 body |
| S-9, D-20 | Prompt-driven behavior 342–355 | dc-14 + body table | verbatim | five-row request-mapping table carried unchanged |
| B-11, B-12, INV-3 | Workbench-first flow 357–375 | B-11/B-12 → AC-2 body; INV-3 → ac-8 text with full prose in AC-8 body; co-9 covers the Playwright case | verbatim | seven-step flow and no-generic-AI-badges sentence in AC-2 body; "must never be silently overwritten" quoted in AC-8 body |
| B-13, P-3 | Markdown/Git-first flow 377–389 | AC-7 body; ac-7 | verbatim | six-step flow carried; `verdi design review <spec-ref>` named (see §10 G-6) |
| INV-4 | Branch/worktree identity 391–398 | co-3; AC-8 body | verbatim | "never silently copies or merges content between worktrees" quoted |
| D-21, D-22, S-10, C-16 | Human review and acceptance 400–428 | ac-6; dc-15; AC-6 body | verbatim | packet content list carried; "the owner's merge … is the single decision that accepts the specification" quoted unchanged |
| D-23, S-11, S-12, C-17, Q-1 | Context integrity 430–459 | ac-5; dc-16; co-8; AC-5 body | verbatim | `get_design_context` list and build-context content/exclusion lists carried; the honest cannot-prove-hidden-memory disclosure carried into Non-goals (NG-14) and AC-5 body |
| Q-2 | 461–466 | dc-16; §5.3 | condensed + dependency-prose | forward-compatibility clause ("each transaction carries policy and context identity") quoted in DC-16 body |
| C-18..C-26 | Nonconfigurable semantics 470–480 | co-3, co-2 | verbatim | the nine-item list reproduced unchanged in CO-3 body |
| S-13, D-24, NG-2 | Configuration 482–516 | ac-3; dc-17; AC-3 body | verbatim | `design_assistance` YAML example and the three mode definitions carried unchanged |
| C-27, C-28 | 518–523, 536–540 | co-6; co-2 | verbatim | narrowing-only rule quoted; the six internal extensibility ports (policy evaluation, mutation application, provenance recording, semantic diffing, change notification, transport adapters) and the declared-descriptor rule carried into CO-2 body |
| S-14 | 524–534 | ac-3; AC-3 body | verbatim | eight-item capability list carried unchanged |
| C-29..C-33, D-25 | Verdi-go boundaries 542–558 | co-7; dc-18 | verbatim | the four never-clauses reproduced in CO-7 body |
| NG-3, NG-4, D-26, D-37 | Jira boundaries 560–581, 821–824 | dc-19 (separate-track rationale, likely-future-flow diagram, and the no-reverse-synchronization rule in DC-19 body); the Jira exclusion itself stays in Non-goals via NG-12 | verbatim | "no silent reverse synchronization" quoted in DC-19 body |
| C-34..C-43 | Failure behavior 583–603 | co-1; co-3; co-5; AC-1 body failure table | verbatim | ten-row refusal list reproduced unchanged |
| C-44..C-48, C-50, C-51 | Abuse resistance 605–618 | co-2 (C-46), co-3 (C-50), co-4 (C-44, C-45), co-5 (C-47, C-48), co-8 (C-51) | verbatim | resistance list reproduced in the named CO bodies (C-49 is carried by the dc-13 row above) |
| C-52, T-1..T-4 (the four test inventories), C-53 | Testing 620–678 | co-9; ac-2 | verbatim | the four test inventories reproduced in CO-9 body; byte-identity conformance clause restated in ac-2 |
| R-1..R-8, INV-5 | Rollout 680–693 | stubs order (§3.3); `## Delivery sequence` body | verbatim | eight steps reproduced in source order in the body; stubs 1–2 derive from R-1, stubs 3–8 from R-2..R-7, R-8 prose-only; INV-5 quoted beneath them |
| D-27 | Dogfood observations 695–711 | dc-20 | condensed | ten observation classes enumerated in DC-20 body |
| SM-1..SM-12 (twelve success measures) | 713–734 | ac-1..ac-8 texts; dc-19 (SM-12) | condensed (grouping = invention N-2) | §11 N-2 carries the measure→destination matrix; each measure's clause appears verbatim in exactly one AC or DC body |
| Q-3 | 736–739 | dc-20 | verbatim | "do not prove universal usability" caveat quoted |
| D-28 (in-scope list) | 741–753 | ac-1..ac-8; co-9; Outcome body | condensed | the nine in-scope items map 1→ac-1, 2→ac-2, 3→ac-3, 4→ac-4, 5→ac-6, 6→ac-5, 7→ac-8, 8→ac-7, 9→co-9 |
| NG-5..NG-14 | Out of scope 755–767 | `## Non-goals` | verbatim | ten items reproduced unchanged, plus NG-1's v1 core exclusions |
| D-29..D-36 (rejected alternatives) | 769–819 | DC bodies (dc-15, dc-6, dc-1, dc-3, dc-2, dc-8, dc-8, dc-7) | verbatim | each rejection and its rationale carried into the DC body of the decision it motivates |

**Coverage check:** all 166 inventoried elements (37 D, 12 SM, 4 T, 53 C,
14 S, 5 INV, 8 R, 14 NG, 13 B, 3 P, 3 Q — Appendix A is the inventory)
appear in exactly one row above; no element maps to "dropped". The only content in the proposed artifact that
does not originate in the ASD design is: the depends-on link set (N-5),
the relationship section carrying the orchestration index's imposed
ownership rules (§5), the evidence-kind declarations (N-3), and the
`attestation` outcome floor (mechanical VL-006 requirement) — each is
recorded as an invention or an imposed-authority carry-over, never silent.

## 5. Cross-feature dependencies

These bind ASD from the orchestration index (ratified design authority at
the landing commit), not from the ASD document itself — the ASD design
never names them. The canonical artifact carries them in its relationship
section as imposed shared-ownership rules; carrying ratified external
authority is preservation, not new semantics (see §10 G-3).

### 5.1 Shared governance-principal kernel

Orchestration lines 96–103, 238, 247: `governance-principal-kernel` is a
prerequisite delivery unit (Wave 1, solo) before CI or GLG define
consumers; "Neither feature may introduce a second profile enum, actor
type, trust-source resolver, or authorization interpretation." ASD's actor
identity (`actor: {kind, principal, harness, session}`, adapter-verified —
ASD 134–137, 204–208) must resolve its human principal through that shared
kernel once it exists. The canonical artifact
states: ASD introduces no profile or actor schema of its own; the
delegating principal resolves through the shared kernel; the agent-actor
discriminator is ASD-owned.
**Implementation of the ASD core (Wave 2) is blocked until the kernel
(Wave 1) merges.** Promotion itself is not blocked.

### 5.2 CI policy-authority scaffolds

Orchestration line 106: CI `policy-authority` owns the immutable
identity/authority/scope/lifecycle/ownership/provenance kernel plus the
shared renderer for configurable human-authored artifacts; "ASD must
consume this seam for agent-assisted creation and may add typed draft
operations, but it may not create a competing template or policy model."
The canonical artifact states this consumption obligation; ASD's
`design_assistance` policy block is project policy expressed through the
CI-owned policy model once that model lands. **ASD draft-creation
implementation is blocked until `policy-authority` (Wave 1) merges.**
Promotion itself is not blocked.

### 5.3 ASD provenance-sidecar classification

Orchestration lines 120–122 and 240, 258, 275: the context compiler must
classify the ASD sidecar explicitly — never silently include it as
authority, never silently omit it from the candidate universe — and "the
ASD core plan must publish the sidecar identity and exclusion contract
before the context compiler finalizes its input classifier." §6 below
publishes that identity and contract from the ASD side. Ratifying the
shared classification is Wave-0 owner work
(orchestration line 240) and is **not performed here**.

### 5.4 Repository-visible successor invention ledger

Orchestration line 235 requires Wave 0 to "establish a repository-visible
successor invention ledger or explicitly retain `PLAN.md` section 7 with a
portable access path"; line 404 makes an unresolvable ledger a named stop
condition for implementation sessions. At the landing commit no successor
ledger exists in the repository, `PLAN.md` lives at the workspace root
outside the `verdi` repository, and its §7 contains no ASD, provenance, or
promotion entries. **This is an unresolved authority gap that blocks
implementation sessions** (§10 G-1). This plan's §11 is the interim
repository-visible record for the promotion unit's own inventions; whether
§11-style plan sections, a dedicated ledger file, or a retained `PLAN.md`
§7 becomes the successor ledger is an owner decision this plan cannot
make.

## 6. Provenance-sidecar identity and exclusion contract

Published here from the ASD side, as the orchestration index requires of
the ASD core plan. **This section publishes; it does not ratify.** The
shared classification consumed by the CI context compiler is ratified
separately (Wave 0, orchestration line 240), and the context compiler's
reason-code vocabulary belongs to the CI `context-compiler` plan.

**Identity:**

- Path: `.verdi/specs/active/<spec-name>/design-provenance.jsonl`,
  moving with the spec directory to
  `.verdi/specs/archive/<spec-name>/design-provenance.jsonl` at closure
  ("the sidecar follows the spec into the archive", ASD 287).
- Schema: `verdi.design-provenance/v1`; append-only JSONL; each entry
  carries the spec ref, previous and resulting spec digests, actor kind /
  principal / harness / optional session, the ordered typed operations,
  object-level digests where applicable, optional bounded classified
  excerpts, and the entry's own deterministic content digest (ASD
  277–285).
- Committed, content-addressed, non-authoritative.

**Exclusion contract (what the context compiler must honor):**

1. The sidecar is **excluded from normal design and build context** (ASD
   288–291, 478) — it never enters a design, build, or review capsule's
   included set by default.
2. The exclusion is **explicit, never silent**: the sidecar appears in the
   compiled manifest's excluded-candidates ledger with a reason code —
   never silently omitted from the candidate universe, never included as
   authority (orchestration 120–122).
3. The only access path is an **explicit provenance request**
   (`get_design_provenance` or the equivalent CLI/workbench view); content
   so returned is non-authoritative data, never instructions, and never
   enters an instruction projection (ASD 289–291, co-8).
4. The sidecar **cannot influence acceptance, evidence folding, alignment,
   or execution** (ASD 289–291); it is not an evidence source and carries
   no verdicts.
5. The stable build-context **exclusion identifier** (the reason code the
   Wave-2 ASD track must establish — orchestration 258) is proposed as the
   sidecar's schema id `verdi.design-provenance/v1` plus its path shape;
   the final reason-code string is deliberately left to the joint
   ratification with the `context-compiler` plan (§11 N-7).

## 7. Future implementation steps

The promotion delivery unit. Preconditions: this plan Codex-APPROVED
(Gate P) with §11 inventions ratified in that review; base = current
`main` at execution time (revalidate against the actual landing commit if
`main` has moved past `6d71fd7d`, per orchestration Phase C); §10 G-1
(successor ledger) resolved by the owner at least to the extent of naming
where the promotion unit's decision record durably lives.

### Task 1: Author the canonical artifact

**Files:**
- Create: `.verdi/specs/active/ai-assisted-spec-design/spec.md` (the only
  file in the unit)

**Interfaces:**
- Consumes: §3's complete structure and §4's mapping table (this plan).
- Produces: `spec/ai-assisted-spec-design`, resolvable by `verdi spec
  state`, lint-clean, spec-align-clean.

- [ ] **Step 1: Record the red baseline.** From a fresh worktree on the
      unit branch:
      `go build -o .build/verdi ./cmd/verdi && ./.build/verdi spec state spec/ai-assisted-spec-design`
      Expected: operational failure — the ref does not resolve (no such
      spec). Capture output.
- [ ] **Step 2: Write `spec.md`** exactly per §3: statusless frontmatter
      (id, kind, title, owners, class, problem with `anchor: problem`,
      outcome with `anchor: outcome`, eight ACs, eight stubs, four links,
      twenty decisions, nine constraints — every object carrying its
      required `anchor:` resolving to the matching body heading), body
      per §3.7, transferring source prose per §4's transformation column.
- [ ] **Step 3: Lint green.**
      `./.build/verdi lint` → exit 0, no findings for the new directory.
      `./.build/verdi model check` → exit 0.
- [ ] **Step 4: Alignment green.** `make spec-align` → PASS (the
      self-hosted arena accepts a growing store; no inventory changes are
      expected because promotion adds no verb and no MCP tool).
- [ ] **Step 5: State projection.**
      `./.build/verdi spec state spec/ai-assisted-spec-design`
      Expected: `proposed` (not reachable from the default branch), with
      Git-derived baseline fields present. Capture output.
- [ ] **Step 6: Mapping fidelity self-check.** Walk §4 row by row against
      the authored file; every "verbatim" row's body content must match
      the source bytes (frontmatter texts may differ only in YAML
      quoting); every Appendix A inventory ID accounted for. Fix in
      place.
- [ ] **Step 7: Commit** (single commit, imperative subject):
      `git add .verdi/specs/active/ai-assisted-spec-design/spec.md`
      `git commit -m "Promote ASD design into canonical feature proposal"`

### Task 2: Full gates and draft PR

- [ ] **Step 1:** `git diff --check` on the branch (no whitespace
      damage); confirm the changed-file list is exactly the one new file.
- [ ] **Step 2:** `make verify` and `go test -race ./... -count=1` from
      the unit worktree — required by the orchestration handoff contract
      for every delivery-unit PR. Expected: clean (the change is
      store-only; failures indicate an environment or store regression and
      stop the unit).
- [ ] **Step 3:** Push; open a **draft** PR to `main` titled
      `Promote ASD into canonical feature proposal artifact` whose body
      carries the orchestration handoff sections: Authority (design blob
      `85957219`, landing commit, this plan's path and commit, base/head
      SHAs), Requirement coverage (§4's table reference plus the
      per-invention ratification list §11), Verification (captured
      command outputs from Task 1), Disclosures (three-valued, §9 shape),
      Review scope (one file; revert = revert one commit).
- [ ] **Step 4:** Wait for the required `merge-gate` check on the exact
      head. Expected: pass.

### Task 3: Codex checkpoint and owner merge

- [ ] **Step 1:** Request read-only Codex review of the exact head
      (Gate C). Codex verifies §4 losslessness row-by-row, convention
      fidelity against the two precedent artifacts, and the §11 invention
      list.
- [ ] **Step 2:** Repair accepted findings (Opus fixer per the repository
      role split); every push invalidates the prior review; obtain fresh
      exact-head approval.
- [ ] **Step 3:** Mark ready and hand to the owner. **The owner's merge is
      acceptance**; no follow-up command, status flip, or commit occurs.
      Post-merge, `verdi spec state spec/ai-assisted-spec-design` reports
      `accepted-pending-build` — a read-only verification, not a ceremony.

**Rollback posture:** the unit is one commit adding one file; revert the
commit and the store returns byte-identical to its prior state. No runtime
code, schema registry, workflow, or index is touched. If reverted after
merge, the spec's Git-derived state ceases to be reachable-at-HEAD and the
artifact leaves the active store with ordinary review of the revert.

**Explicitly out of the unit:** any edit to the orchestration index, any
runtime/test/workflow change, any obligation scaffolding (VL-020 exempts
feature-class ACs), any `verdi accept` invocation, any edit to the ASD
design document, and any CSE file (Phase C requires disjoint changed-file
inventories; this unit's inventory is one file under
`.verdi/specs/active/ai-assisted-spec-design/`).

## 8. Human-ceremony inventory

Classification per the merge-signaled ceremony-audit rule. Acknowledgements
duplicated by the reviewed owner merge are removed; retained ceremonies
carry distinct judgment, exceptional override, or irreversible-risk
protection.

| # | Ceremony | Class | Disposition |
|---|---|---|---|
| 1 | Acceptance of the canonical ASD artifact | authorization already expressed by PR review + merge | **Retain the merge only.** No `verdi accept`, status edit, ledger flip, or post-merge confirmation (merge-signaled design; precedent `6d71fd7d`) |
| 2 | Codex plan review (this plan) and exact-head implementation review (promotion PR) | substantive judgment (independent review) | **Retain** — Gates P and C; distinct information, not a duplicate acknowledgement |
| 3 | Ratification of §11 inventions (naming, AC grouping, evidence kinds, stubs, links) | substantive judgment | **Retain, folded into ceremony 2** — the plan review is the vehicle; no separate sign-off artifact is created |
| 4 | Future ASD runtime: draft acceptance | authorization already expressed by merge | **Retained as merge-only in the artifact itself** (dc-15/ac-6: "no separate acceptance command, status edit, or confirmation repeats it") — the design already removed the duplicate |
| 5 | Future ASD runtime: per-mutation agent-edit confirmation | informational acknowledgement | **Removed by the design** (dc-3: no confirmation theater for reversible draft edits); the plan adds none back |
| 6 | Future ASD runtime: semantic review packet | deterministic materialization (a derived view) | **No retained ceremony** — the packet is derived, never marked-approved, never persisted (dc-15); reading it is part of ceremony 4's review, not a second act |
| 7 | Future ASD runtime: unsaved-edit conflict resolution (save / keep-stale / discard) | irreversible-risk protection | **Retain** — the one place a human must choose, protecting against silent loss of human work (INV-3) |
| 8 | Future ASD runtime: waivers, attestations, conflict/deviation dispositions | substantive judgment | **Retain, human-only** (dc-4/co-3); agents are refused at every surface |
| 9 | Future ASD runtime: `design_assistance` policy changes | authorization already expressed by normal ratification (PR review/merge) | **Retain the existing flow only**; a policy change is an ordinary reviewed change producing a new policy digest (co-6); no additional acknowledgement |
| 10 | Future ASD runtime: excerpt redaction/removal | exceptional override (data hygiene) | **Retain as an ordinary reviewed edit** — excerpts are removable/redactable by design (co-5); no new ceremony invented |

Net: the promotion unit itself contains exactly one human authorization
act — the owner's merge — plus two independent read-only Codex reviews
(plan and exact head). Every other human interaction in the ASD
lifecycle either already carries distinct judgment in the design or was
already removed by it; this plan introduces zero new ceremonies.

## 9. Three-valued disclosure

**Proven (with witnesses):**

- `origin/main` == `6d71fd7d33beaf8128fa675833ee12595205481d` — verified
  by `git fetch && git rev-parse origin/main` at plan start.
- The three authority blob identities match the pinned OIDs — verified by
  `git rev-parse HEAD:<path>` for all three files (§1).
- Branch `agent/asd-canonical-promotion-plan` did not previously exist
  locally or on the remote — verified before creation.
- Canonical-artifact conventions and statusless acceptance path — verified
  against both precedent artifacts and commits `f2185492`, `ae183b15`,
  `6d71fd7d`.
- No Go test or lint rule indexes `docs/superpowers/plans/` — verified by
  repository-wide search; adding this plan file changes no gate's input
  beyond the files-changed list.
- VL-020 requires obligations for story-class ACs only; the feature-class
  promotion artifact requires none — verified in
  `internal/lint/vl020.go`.

**Violated:** none observed.

**Unproven / blocking:**

- **Successor invention ledger (blocking for implementation sessions):**
  no repository-visible successor ledger exists at the landing commit;
  `PLAN.md §7` is workspace-root, outside this repository, with no ASD
  entries. Until the owner resolves orchestration Wave-0 line 235, every
  implementation session (including the promotion unit under a strict
  reading of stop-condition line 404) lacks a named home for its invention
  records. This plan's §11 is offered as the interim record; its
  sufficiency is an owner call, not proven here.
- **Governance-principal kernel and CI policy-authority do not exist yet**
  (Wave 1 work): the canonical artifact can state the dependencies, but
  their schemas are unproven; ASD runtime implementation is blocked on
  them (§5.1, §5.2).
- **"No new semantics" compliance of the §11 inventions** (AC grouping,
  evidence kinds, stub granularity, link set, imposed-authority
  relationship prose) is asserted with §4 as evidence but is only proven
  by the independent Codex review and owner ratification — until then it
  is a disclosed-unproven claim.
- **GitHub check results for this planning PR** are unproven until the
  required checks complete on the pushed head (reported in the handoff).
- **CSE disjointness at execution time** (Phase C precondition) can only
  be proven against the CSE promotion unit's actual changed-file
  inventory when both exist.

## 10. Semantic gaps and contradictions

Complete list. G-1 blocks implementation; the rest are disclosed readings
or deferred decisions that do not block this plan.

- **G-1 (blocking): missing successor authority.** The repository-visible
  successor invention ledger required by orchestration Wave 0 (line 235)
  does not exist, and the stop condition at line 404 makes an unresolvable
  ledger a session-stopper for implementation. **Missing successor
  authority blocks implementation** of the promotion unit until the owner
  names the ledger location (or ratifies plan-section records like §11 as
  the mechanism).
- **G-2: ASD is silent about its imposed dependencies.** The design
  document never mentions the governance-principal kernel, CI
  policy-authority, or the successor ledger; those bind ASD only through
  the orchestration index (lines 89, 92, 106, 120–122). The canonical
  artifact must therefore carry a relationship section whose content has
  no anchor in the ASD document. Reading adopted: those constraint
  sentences were reviewed and landed with the owner's merge of PR #258
  (which ratified the four indexed documents — orchestration line 14),
  and both precedent canonical artifacts already carry imposed
  "Relationship to …" sections, so carrying them preserves reviewed
  authority rather than adding semantics. Disclosed caveat: the index's
  own ratification as successor orchestration authority (Wave-0 checkbox,
  orchestration line 234) is itself still open; this reading requires
  Codex/owner confirmation at plan review.
- **G-3: evidence kinds and the outcome floor are additions.** The ASD
  design declares no evidence kinds for its success measures; VL-006
  mechanically requires every feature AC to declare kinds including
  `attestation`. The declarations in §3.2 are inventions (N-3), chosen as
  the smallest set consistent with the design's own test plan. They are
  new frontmatter facts, not new behavioral semantics.
- **G-4: binding 02 contract text vs. statusless artifacts.**
  `docs/design/specs/02-artifact-contract.md` still marks `status:` as
  required common frontmatter, while the merge-signaled ratification
  (08-revision-notes, 2026-08-01 entry) permits new active specifications
  to omit it and both precedent canonical artifacts do. The lint engine
  already accepts statusless feature specs. Disclosed as a known
  documentation lag in the binding contract text; not a promotion blocker;
  its eventual 02 amendment belongs to the merge-signaled follow-up work,
  not this unit.
- **G-5: the design's open questions have no canonical object home.**
  Q-1 (cannot prove absence of hidden harness memory) and Q-2 (sealed
  manifest / contradiction detection are a separate track) are honest
  disclosures, not unresolved design forks. Proposed treatment (invention
  N-6): Q-1 lands in Non-goals (NG-14 already states it) and AC-5 body;
  Q-2 lands in DC-16 as the forward-compatibility clause. No
  `open_questions:` objects are declared — neither precedent artifact
  declares any, and neither Q item is an actionable open fork for ASD
  itself.
- **G-6: future surface inventory growth.** The design names six MCP
  tools (one of which, `get_board`, already exists) and a
  `verdi design review <spec-ref>` CLI surface. `internal/specalign`
  inventories pin the current nine MCP tools and current verbs. Promotion
  adds no tool and no verb; the inventory updates belong to the Wave-2/5
  implementation units, which must extend `mcptools_test.go` and (if
  `design review` is ruled a new grammar rather than a subcommand of the
  existing `design` verb) `verbs_test.go`. Disclosed so no later session
  treats the inventories as contradicting the accepted artifact.
- **G-7: sidecar lint coverage is future work.** The committed
  `design-provenance.jsonl` sidecar has no lint rule today (VL-018 covers
  `layout.json`; VL-007 covers only entries directly under `.verdi/`).
  Whether the sidecar needs its own VL rule (schema check,
  append-only/no-rewrite discipline) is a Wave-2 design question for the
  core unit; recorded here so it is not silently resolved at
  implementation time.
- **G-8: exclusion reason-code ownership.** The stable build-context
  exclusion identifier for the sidecar is jointly owned: ASD publishes
  identity (§6), the CI `context-compiler` plan owns the reason-code
  vocabulary. Neither may finalize alone (orchestration 120–122, 258,
  275). Deferred to that joint ratification (N-7).
- **G-9: the binding 00–05 specs live outside this repository.**
  `docs/design/specs/` exists only at the workspace root, not in the
  verdi repository, and the orchestration index forbids web lanes from
  relying on workspace-root files. A repository-contained promotion
  session must use the self-hosted mirrors
  (`.verdi/specs/active/verdi-artifact-contract/spec.md` and siblings),
  which the spec-align fidelity gate keeps byte-faithful to their origins
  where the full workspace layout is present; in a repository-only
  checkout that fidelity check reports a disclosed SKIP, so mirror
  faithfulness is disclosed-unproven there — usable, but never silently
  assumed proven.

No contradiction was found **within** the ASD design itself, and none
between the ASD design and the merge-signaled acceptance design (ASD's own
acceptance section states the identical merge-is-acceptance rule in its
own words).

## 11. Decision record — inventions requiring ratification

Interim repository-visible record for this unit (see G-1). Each entry:
smallest reversible choice, alternatives considered, ratification vehicle
= the Codex review of this plan plus the owner's merge of the promotion
PR. None is decided silently; none is binding until ratified.

- **N-1 — Canonical identity `spec/ai-assisted-spec-design`.**
  Alternatives: `spec/design-assistance` (shorter, but drops the
  document-title correspondence both precedents preserve);
  `spec/asd` (opaque initialism, no precedent). Chosen: the title-derived
  kebab name, matching `context-integrity`/`guided-lifecycle-governance`
  derivation. Reversible until merge; a rename after acceptance would be a
  supersession, so ratify before landing.
- **N-2 — Success measures grouped into eight ACs plus dc-19.**
  Alternatives: twelve 1:1 ACs (maximal fidelity, but splits single
  outcomes across bookkeeping lines and inflates the stub matrix); the
  four test-section headings as ACs (implementation-scoped, violating
  feature-AC outcome-blindness). Chosen: eight outcome-level ACs; the
  matrix is — SM-1→ac-1, SM-2→ac-7, SM-3→ac-2, SM-4→ac-8, SM-5→ac-4,
  SM-6→ac-4, SM-7→ac-6, SM-8→ac-7, SM-9→ac-3, SM-10→ac-8, SM-11→ac-8
  (co-7 carries the boundary constraint), SM-12→dc-19 (a non-goal-shaped
  measure, carried as decision plus the NG-12 non-goal). Every measure's
  clause appears verbatim in exactly one AC or DC body.
- **N-3 — Evidence-kind declarations.** Chosen: `static, behavioral,
  attestation` for ac-1..ac-5 (schema/lint-checkable surfaces plus
  behavioral suites plus human attestation), `behavioral, attestation` for
  ac-6..ac-8 (journey-level outcomes). Smallest sets consistent with the
  design's own test plan (core/conformance tests → static+behavioral;
  browser/e2e journeys → behavioral) and the VL-006 attestation floor.
- **N-4 — Stub granularity: eight stubs matching the orchestration
  index's named delivery units.** Alternative: three stubs matching the
  completion-ledger wave rows (coarser; would erase the reviewed rollout
  ordering R-1..R-7 that promotion must preserve). Chosen: eight, in
  rollout order.
- **N-5 — depends-on link set** (§3.4): the two canonical feature
  dependencies imposed by the orchestration index plus the two component
  specs whose contracts the feature directly extends. Alternative (links
  to only the two features) rejected as under-declaring the reading-order
  dependencies. Disclosed new pattern: neither precedent links to a
  component-class spec, so the two component links are a first — they are
  reading-order `depends-on` edges only, carrying no lifecycle coupling.
- **N-6 — Q-1/Q-2 carried as Non-goals + DC-16 forward-compatibility
  clause,** not `open_questions:` objects (G-5).
- **N-7 — Sidecar exclusion reason-code deferred** to joint ratification
  with the CI `context-compiler` plan; ASD publishes identity and
  contract only (§6, G-8).

---

*Prepared on branch `agent/asd-canonical-promotion-plan` from base
`6d71fd7d33beaf8128fa675833ee12595205481d`. Planning-only: this branch
adds exactly this document.*

## Appendix A — exhaustive line-anchored element inventory

The complete inventory of normative elements in the ASD design (blob
`8595721911c3c756458ace195a686a871b6410d3`), produced for this plan and
independently re-verified by the Opus verification pass. Classes: D
decision · SM success measure · T test inventory · C constraint · S
schema/record · INV invariant · R rollout step · NG non-goal · B
CLI/MCP/workbench behavior · P provenance rule · Q open question.
Counts: 37 D, 12 SM, 4 T, 53 C, 14 S, 5 INV, 8 R, 14 NG, 13 B, 3 P,
3 Q = 166.

| ID | Anchor (lines) | Element |
|---|---|---|
| D-1 | 1–4 | Status line: ratified design authority by owner merge; "not canonical Verdi lifecycle authority until its sequenced canonical-promotion unit merges" |
| D-2 | 28–31 | External-harness assistance "without making AI participation mandatory or turning design into an autonomous generation step" |
| D-3 | 33–39 | Authorship vs. authority: "the human cannot delegate the consequential governance decisions that make the draft authoritative" |
| D-4 | 41–45 | One typed draft-mutation core shared by workbench/CLI/MCP; Markdown first-class; bounded provenance outside normal build context |
| D-5 | 47–56 | Six supported paths, none privileged (workbench-only, Markdown/Git, board updates, complete agent draft, interleaving, one contract for Codex and Claude Code) |
| B-1 | 63–66 | Current: board is a projection of spec.md; authoring API strict-decodes, splices, validates, serializes, atomic-writes |
| C-1 | 67–68 | Current: workbench authoring limited to draft specs on design branches; review mode is a mirror; accepted specs not editing surfaces |
| B-2 | 69–71 | Current: MCP write surface is only `add_annotation` |
| B-3 | 72 | Current: direct Markdown and Git remain the portable authoring medium |
| C-2 | 73–74 | Current: provider port resolves records / publishes rollups; does not create Jira epics or stories |
| C-3 | 75–77 | Importing Verdi-go packages or recreating its surfaces would violate the boundary |
| D-6 | 79–81 | The missing piece is a common guarded mutation contract, not a new workspace or chat product |
| D-7 | 85–87 | One canonical draft; projections never competing documents |
| D-8 | 88–90 | Equal domain operations; no browser driving, no privileged rewrite |
| D-9 | 91–93 | Explicit human delegation; no per-keystroke confirmation theater |
| D-10 | 94–96 | Human-only governance (acceptance, approval, waivers, attestations, dispositions) |
| D-11 | 97–99 | Optimistic concurrency, fail closed; never silent merge |
| D-12 | 100–102 | Refined output over conversational residue |
| D-13 | 103–105 | Useful provenance, not theater; excerpts never evidence/instructions/proof/second truth |
| D-14 | 106–108 | Model- and harness-neutral; Verdi hosts no model |
| D-15 | 109–111 | AI-free remains complete |
| D-16 | 112–115 | Mechanics deterministic; quality remains judgment |
| S-1 | 121–132 | Ten-row authority matrix (action / human / delegated agent / Verdi) |
| C-4 | 128 | Agent cannot accept a design; Verdi enforces the gate |
| C-5 | 129 | Agent cannot approve or merge the PR |
| C-6 | 130 | Agent cannot disposition a conflict or deviation |
| C-7 | 131 | Agent cannot author a waiver or attestation |
| C-8 | 132 | Agent cannot judge semantic sufficiency; Verdi cannot score understanding |
| C-9 | 134–137 | Agent actor distinguishable from delegating human; adapter-controlled identity; payload cannot self-promote via `author` string |
| D-17 | 139–144 | No global auto-design mode |
| D-18 | 148–156 | Core is a model-neutral application service; all adapters call it (architecture diagram) |
| S-2 | 158–172 | Nine-step transaction (resolve, load+digest, state+policy, digest compare, apply, strict-validate, prepare provenance, transactional commit, return) |
| INV-1 | 174–178 | Crash recovery: complete old or complete new transaction, never a typed edit without its provenance record |
| NG-1 | 180–184 | v1 core owns semantic draft objects only; no branch/commit/push/PR/accept/align/disposition/publish/close; no layout smuggling |
| B-4 | 186–190 | Workbench actions become thin adapters; one gesture ≈ one operation; agent batches all-or-nothing |
| S-3 | 194–196 | `verdi.draftmutation/v1`, strict-decoded, unknown anything fails closed |
| S-4 | 198–222 | Request fields: schema, spec, base_digest, actor{kind,principal,harness,session}, operations[], sources[] |
| C-10 | 224–226 | Adapter injects/verifies identity; agent does not self-authorize |
| S-5 | 228–236 | Closed operation vocabulary (seven categories) |
| C-11 | 238–240 | Operations address stable object IDs, never line/byte/CSS/board positions |
| S-6 | 242–255 | `verdi.draftmutation-result/v1`: previous/result digests, object-level changes, warnings |
| D-19 | 257–260 | Semantic diff reports object-level meaning; consequential ops get prominent treatment |
| C-12 | 262–266 | Raw Markdown legal; origin `unclassified`; no fabricated attribution; allow/disclose/block policy, disclose default |
| S-7 | 270–274 | Sidecar path `.verdi/specs/active/<name>/design-provenance.jsonl` |
| S-8 | 276–285 | `verdi.design-provenance/v1` entry: seven fields (spec ref; prev/result digests; actor; ordered ops; object-level digests; optional excerpts+classifications; own content digest) |
| P-1 | 287–291 | Sidecar committed, follows spec to archive, excluded from normal design/build context, influences nothing unless explicitly requested |
| C-13 | 293–301 | No transcript import; excerpt targets a declared object; classifications human-stated / ai-synthesized / ai-inferred / unresolved |
| C-14 | 303–307 | Verbatim vs. paraphrase labeled; excerpts optional, bounded, removable, redactable; data never instructions; never evidence |
| INV-2 | 309–311 | Excerpt records target digest at attach time; stays truthful history |
| P-2 | 313–316 | No retroactive provenance for direct edits; disclosure default ("a false audit trail would be worse than an incomplete one") |
| B-5 | 322–324 | `get_board`: deterministic board projection |
| B-6 | 325–326 | `get_design_context`: bounded authoritative material |
| B-7 | 327–328 | `get_design_capabilities`: schema, checkout, policy, permitted operations |
| B-8 | 329 | `mutate_draft`: one atomic typed transaction |
| B-9 | 330 | `get_design_provenance`: provenance only on explicit request |
| B-10 | 331–332 | `prepare_design_review`: derive packet without changing governance state |
| C-15 | 334–339 | Tools are transport adapters; CLI equivalents; no harness-specific schema/lifecycle/second MCP server |
| S-9 | 346–352 | Five-row prompt→behavior mapping table |
| D-20 | 354–355 | Mapping is guidance, not hidden policy; enforcement is the capability/authority boundary |
| B-11 | 359–366 | Seven-step workbench-first flow |
| B-12 | 368–370 | No generic "AI-generated" badges; packet carries the distinctions |
| INV-3 | 372–375 | Unsaved human inline edit never silently overwritten; agent gets the same stale-base refusal |
| B-13 | 379–386 | Six-step Markdown/Git-first flow incl. `verdi design review <spec-ref>` |
| P-3 | 388–389 | Same spec/policies/review apply; direct Markdown has less attributable provenance |
| INV-4 | 393–398 | Every response names exact checkout/branch/HEAD/spec; no silent cross-worktree copy/merge; handoff through Git and artifacts |
| D-21 | 402–406 | Review packet is a view, not a persisted approval artifact or second truth |
| S-10 | 407–416 | Packet contents (problem/outcome; ACs+evidence; constraints/decisions/questions/links/stubs; semantic changes; ai-inferred/unresolved; unclassified edits; warnings) |
| D-22 | 418–424 | No token-certification; profile-required exact-head review authorizes merge; owner's merge is the single acceptance; no checkbox theater |
| C-16 | 426–428 | Agent cannot mark approved / accept / approve / merge; acceptance ratifies the refined spec, not the conversation |
| D-23 | 432–435 | First half of the context-poisoning response: explicit inspectable context over opaque memory |
| S-11 | 436–443 | `get_design_context` contents (draft; selected parent/children; policies+decisions; pinned refs; Verdi-go findings; context+policy digests) |
| C-17 | 445–448 | Excerpts excluded by default; annotations/artifacts untrusted data; AGENTS.md/CLAUDE.md never silently Verdi decisions |
| S-12 | 450–453 | Normal build context contents and exclusions |
| Q-1 | 455–459 | Cannot prove a harness has no hidden memory; Verdi makes its own payload inspectable and requires digest disclosure |
| Q-2 | 461–466 | Sealed manifest + contradiction detection are a separate track; mutation contract stays compatible (carries policy/context identity) |
| C-18 | 472 | Only draft specs accept semantic mutations |
| C-19 | 473 | Every typed mutation requires a base digest |
| C-20 | 474 | Stale mutations never auto-merge |
| C-21 | 475 | Complete result validated before write |
| C-22 | 476 | Spec + provenance form one transaction |
| C-23 | 477 | Agent actors remain distinct from humans |
| C-24 | 478 | Provenance non-authoritative, excluded from normal build context |
| C-25 | 479 | Agents cannot accept/approve/attest/waive/disposition |
| C-26 | 480 | Unknown configuration/schemas/operations/enums fail closed |
| S-13 | 485–506 | `design_assistance` policy config (agent_writes; allowed_operations; direct_markdown.origin; provenance bounds; review switches) |
| D-24 | 508–513 | Mode meanings: off / proposal-only / draft-write |
| NG-2 | 515–516 | `layout` reserved, false in v1 |
| C-27 | 518–523 | Parent policy restricts; lower policy narrows only; changes prospective with new digest; old provenance keeps old policy identity |
| S-14 | 525–534 | `get_design_capabilities` exposes eight items (schema versions; digests; checkout/branch/HEAD/spec/digest; state; operations; postures; review requirements; surfaces) |
| C-28 | 536–540 | Extensibility at six explicit internal ports (policy evaluation, mutation application, provenance recording, semantic diffing, change notification, transport adapters); declared model descriptors only; no YAML escape hatch |
| C-29 | 546–547 | Verdi-go integration stays exec-pinned-CLIs + strict decode |
| C-30 | 551 | Never import Verdi-go packages |
| C-31 | 552 | No competing Verdi-go MCP server |
| C-32 | 553 | No duplication of its graph/policy semantics |
| C-33 | 554 | No harness bypass of the pinned execution/decoding boundary |
| D-25 | 556–558 | Assistance consumes the same derived information a human can inspect |
| NG-3 | 562–566 | Jira provisioning valuable but a separate track (identity/hierarchy/idempotency/reconciliation/partial-failure/freeze-timing) |
| D-26 | 568–576 | Likely future Jira flow (epic projection → provision → bind → accept; story projection → provision → scaffold) |
| NG-4 | 578–581 | Verdi authoritative; Jira a deterministic projection; no silent reverse synchronization; not in this implementation |
| C-34 | 588–589 | Stale base: no write; return current digest + changed identities |
| C-35 | 590 | Forbidden actor/state/operation: no write; name the governing policy |
| C-36 | 591 | Malformed/unknown operation: no write |
| C-37 | 592–593 | Invalid resulting document: no write; object-level diagnostics |
| C-38 | 594 | One invalid operation → no operation in the batch lands |
| C-39 | 595 | Concurrent writer: serialize or refuse; never lose an update |
| C-40 | 596–597 | Provenance preparation/commit failure → typed change not visible |
| C-41 | 598–599 | Crash during replacement → recovery to complete old or new |
| C-42 | 600–601 | Raw Markdown edit: preserve Git-recoverable change; disclose `unclassified` |
| C-43 | 602–603 | Accepted/review-mode spec: refuse mutation even if an adapter mis-advertises |
| C-44 | 607 | Agent cannot claim to be human or perform governance |
| C-45 | 608 | Harness cannot bypass checks by rewriting through MCP |
| C-46 | 609–610 | No "pass" by omitting unknown fields or inventing operation names |
| C-47 | 611 | Generated provenance never counts as evidence or understanding |
| C-48 | 612–613 | More excerpts/transactions/AI text improve no score; no score exists |
| C-49 | 614–615 | Deleting hard constraints/ACs stays possible in draft but conspicuous in review |
| C-50 | 616 | A stale agent cannot overwrite intervening human edits |
| C-51 | 617–618 | Corpus data marked untrusted; cannot redefine mutation/governance contracts |
| C-52 | 622–624 | No test calls a live model, network service, or harness; tests assert exact artifacts/bytes/digests/diffs/failures |
| T-1 | 628–639 | Core test inventory (state/actor, stale digest, IDs, links/kinds, batches, rollback, crash recovery, Markdown disclosure, archive/exclusion, unknown rejection) |
| T-2 | 643–651 | Adapter conformance: byte-identical spec.md, provenance, digests, diff, warnings/errors across the three adapters |
| C-53 | 653–654 | Workbench must stop containing a parallel interpretation once migrated |
| T-3 | 658–666 | Playwright inventory (external mutation on open board; updates; protected unsaved edit; interleaving; on-demand provenance; disabled projects; AI-free journey) |
| T-4 | 670–678 | MCP/CLI e2e inventory (both harness shapes; three modes; atomic batch; stale/concurrent; unclassified Markdown; capability/policy-digest changes; refused governance) |
| R-1 | 682 | Implement and prove the core plus a structured CLI adapter |
| R-2 | 683 | Migrate existing workbench spec actions to the core |
| R-3 | 684 | Expose read-only design capabilities and bounded context |
| R-4 | 685 | Dogfood proposal-only with external harnesses |
| R-5 | 686 | Enable draft-write for the Verdi repository |
| R-6 | 687 | Add semantic review and provenance views |
| R-7 | 688 | Run the same journeys through Codex and Claude Code |
| R-8 | 689–690 | Broaden only after chronicles show a coherent journey and legible failures |
| INV-5 | 692–693 | Each step preserves the AI-free path and can be disabled without changing spec semantics |
| D-27 | 697–711 | Ten dogfood observation classes; measure the system, never score the agent |
| SM-1 | 715–716 | Agent-created complete draft appears on the board |
| SM-2 | 717–718 | Same journey via workbench or Markdown without AI |
| SM-3 | 719 | Workbench, CLI, MCP mutations identical |
| SM-4 | 720 | Human/agent edits interleave without silent overwrite |
| SM-5 | 721 | Excerpts useful without importing transcripts |
| SM-6 | 722–723 | Accepted design and build context exclude provenance/noise |
| SM-7 | 724–725 | Packet + PR review expose consequential changes without checkbox theater |
| SM-8 | 726 | AI-disabled projects retain the complete workflow |
| SM-9 | 727–728 | Agents cannot perform acceptance or judgment-bearing governance |
| SM-10 | 729–730 | Codex and Claude Code use the same contracts |
| SM-11 | 731–732 | Verdi-go remains one pinned strict-decoded integration |
| SM-12 | 733–734 | Jira creation remains a separately designed provisioning concern |
| Q-3 | 736–739 | Measures show process coherence, not universal usability; that needs continued chronicles |
| D-28 | 743–753 | Nine in-scope items (core; adapters; capability+policy; excerpts+provenance; semantic review; bounded context; interop; Markdown/AI-free preservation; hermetic tests) |
| NG-5 | 757 | No embedded chat, model picker, or model-hosting layer |
| NG-6 | 758 | No selecting or prescribing a model |
| NG-7 | 759 | No full transcript import |
| NG-8 | 760–761 | No autonomous acceptance/approval/waiver/attestation/disposition |
| NG-9 | 762 | No semantic-quality or human-understanding scores |
| NG-10 | 763 | No second proposal document, agent board, or canonical spec |
| NG-11 | 764 | No competing Verdi-go integration or MCP server |
| NG-12 | 765 | No Jira epic or story creation |
| NG-13 | 766 | No sealed build-context manifest / full contradiction detector |
| NG-14 | 767 | No proof that an external harness has no opaque global memory |
| D-29 | 771–775 | Rejected: require humans to type problem/outcome |
| D-30 | 777–782 | Rejected: import the complete chat transcript |
| D-31 | 784–789 | Rejected: separate agent proposal layer |
| D-32 | 791–795 | Rejected: confirmation for every agent mutation |
| D-33 | 797–801 | Rejected: agents editing through browser automation |
| D-34 | 803–807 | Rejected: embedded Verdi AI assistant |
| D-35 | 809–813 | Rejected: per-harness or per-Verdi-go separate integrations |
| D-36 | 815–819 | Rejected: provenance as evidence or acceptance score |
| D-37 | 821–824 | Rejected: Jira creation in this implementation |
