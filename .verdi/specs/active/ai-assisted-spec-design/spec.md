---
id: spec/ai-assisted-spec-design
kind: spec
title: "AI-assisted spec design"
owners: [platform-team]
class: feature
problem:
  text: "Verdi's workbench, CLI, MCP, and direct-Markdown authoring paths do not yet share one guarded semantic mutation contract, so delegated agents cannot perform the same declared draft edits as humans without unequal semantics, stale-write risk, or an unclear boundary between authorship and governance authority"
  anchor: problem
outcome:
  text: "one typed, digest-guarded, atomic draft-mutation core serves workbench, CLI, and MCP identically; delegated agents author drafts only under ratified policy while consequential governance stays human-only; bounded non-authoritative provenance is explicitly excluded from normal design and build context; and direct Markdown plus fully AI-free journeys remain first-class"
  anchor: outcome
acceptance_criteria:
  - id: ac-1
    text: "a human or a delegated agent can construct or modify a draft specification through one typed, digest-guarded, atomic draft-mutation transaction whose complete result is strict-validated before write, whose ordered batch lands entirely or not at all, and whose typed change can never become visible without its matching provenance record, including across crash recovery"
    evidence: [static, behavioral, attestation]
    anchor: ac-1
  - id: ac-2
    text: "workbench, CLI, and MCP adapters expose identical mutation semantics over the shared core — given the same base document, actor authority, policy, and operations, each produces byte-identical resulting spec.md, provenance record, digests, semantic diff, and warnings — and the migrated workbench retains no parallel interpretation of domain mutations"
    evidence: [static, behavioral, attestation]
    anchor: ac-2
  - id: ac-3
    text: "agent participation is governed by ratified project policy across exactly three modes (off, proposal-only, draft-write) with capability discovery declaring the active schema, policy and project digests, checkout identity, spec state, and permitted operations; agent actors remain distinguishable from their delegating human principal; and every governance operation is absent or refused through every agent surface"
    evidence: [static, behavioral, attestation]
    anchor: ac-3
  - id: ac-4
    text: "typed mutations append bounded, classified, content-addressed provenance to the committed non-authoritative design-provenance sidecar that follows the spec into archive, is excluded from normal design and build context, never counts as evidence or acceptance input, and surfaces direct Markdown edits as unclassified origin rather than fabricating attribution"
    evidence: [static, behavioral, attestation]
    anchor: ac-4
  - id: ac-5
    text: "an assisting agent can obtain bounded, inspectable design context — the current draft, explicitly selected parent or children, applicable policies and ratified decisions, declared pinned references, Verdi-go-derived findings, and the context and policy digests — while provenance excerpts stay excluded by default and corpus content remains data, never instructions"
    evidence: [static, behavioral, attestation]
    anchor: ac-5
  - id: ac-6
    text: "before acceptance a human can derive a concise semantic review packet — a view, never a persisted approval artifact — exposing semantic changes since the review base, ai-inferred and unresolved objects, unclassified direct edits, and material warnings; the profile-required review of the exact proposed head authorizes merge, and the owner's merge is the single acceptance decision with no second ceremony"
    evidence: [behavioral, attestation]
    anchor: ac-6
  - id: ac-7
    text: "disabling or declining AI assistance leaves the complete workbench, Markdown, Git, validation, review, and acceptance journey intact; assistance-disabled projects expose no agent write controls; and direct Markdown editing with normal Git review remains a first-class authoring path with honestly weaker provenance"
    evidence: [behavioral, attestation]
    anchor: ac-7
  - id: ac-8
    text: "Codex-shaped and Claude-Code-shaped callers use the identical published capability and mutation contracts with no harness-specific schema, lifecycle, or second MCP server; human and agent edits interleave without silent overwrite of an unsaved human edit or a stale write; and Verdi-go remains one pinned, strict-decoded integration"
    evidence: [behavioral, attestation]
    anchor: ac-8
stubs:
  - { slug: draft-mutation-core, acceptance_criteria: [ac-1, ac-4] }
  - { slug: structured-cli, acceptance_criteria: [ac-1, ac-2] }
  - { slug: workbench-migration, acceptance_criteria: [ac-2, ac-7] }
  - { slug: design-context-capabilities, acceptance_criteria: [ac-3, ac-5] }
  - { slug: proposal-only-dogfood, acceptance_criteria: [ac-3] }
  - { slug: draft-write-enablement, acceptance_criteria: [ac-1, ac-3] }
  - { slug: review-provenance-views, acceptance_criteria: [ac-4, ac-6] }
  - { slug: harness-conformance, acceptance_criteria: [ac-8] }
links:
  - { type: depends-on, ref: spec/context-integrity-v2 }
  - { type: depends-on, ref: spec/guided-lifecycle-governance-v3 }
  - { type: depends-on, ref: spec/verdi-artifact-contract }
  - { type: depends-on, ref: spec/verdi-surfaces }
decisions:
  - id: dc-1
    text: "one canonical draft: spec.md remains the semantic source of truth; board, CLI, MCP, review packet, and provenance views are projections or controlled mutation paths, never competing documents; a separate agent proposal layer is rejected as a second source of truth (D-7, D-31)"
    anchor: dc-1
  - id: dc-2
    text: "equal domain operations: an authorized agent uses the same typed operations and validation rules as the workbench — no browser automation, no privileged file-rewrite path (D-8, D-33)"
    anchor: dc-2
  - id: dc-3
    text: "explicit human delegation: an agent may mutate when the human asks; no per-keystroke or per-mutation confirmation theater for reversible draft edits (D-9, D-32)"
    anchor: dc-3
  - id: dc-4
    text: "human-only governance: acceptance, PR approval, waivers, attestations, conflict/deviation dispositions, and equivalent judgment records cannot be authored or performed by an agent (D-10)"
    anchor: dc-4
  - id: dc-5
    text: "optimistic concurrency, fail closed: every typed mutation names the digest it read; a stale writer is refused and must reload; Verdi never silently merges two semantic edits (D-11)"
    anchor: dc-5
  - id: dc-6
    text: "refined output over conversational residue: normal design and build context contains accepted decisions and declared scope, never the transcript or hidden reasoning; full transcript import is rejected (D-12, D-30)"
    anchor: dc-6
  - id: dc-7
    text: "useful provenance, not provenance theater: excerpts jog memory; they are not evidence, instructions, proof of comprehension, or a second source of truth, and provenance is never an acceptance score (D-13, D-36)"
    anchor: dc-7
  - id: dc-8
    text: "model- and harness-neutral: Verdi publishes typed contracts and hosts no model, embeds no assistant, and maintains no per-harness integrations; duplicate adapters drift (D-14, D-34, D-35)"
    anchor: dc-8
  - id: dc-9
    text: "AI-free remains complete: disabling AI assistance degrades nothing (D-15)"
    anchor: dc-9
  - id: dc-10
    text: "mechanics may be deterministic; quality remains judgment: Verdi validates shape, state, links, concurrency, and policy; it cannot score insight or understanding, and no quality score exists to game (D-16)"
    anchor: dc-10
  - id: dc-11
    text: "there is no global auto-design mode: an agent acts in response to a human request and cannot cross the governance boundary (D-17)"
    anchor: dc-11
  - id: dc-12
    text: "the core is one model-neutral application service inside Verdi; all adapters are thin; a human gesture is normally a one-operation transaction; an agent batch succeeds entirely or not at all (D-18, B-4)"
    anchor: dc-12
  - id: dc-13
    text: "the semantic diff reports object-level meaning, never a raw Markdown patch; deletions, large replacements, reordering, and relationship changes are reversible but receive prominent treatment in result and review packet (D-19, C-49)"
    anchor: dc-13
  - id: dc-14
    text: "prompt-driven behavior mapping is published guidance, not hidden model policy; enforcement remains the capability and authority boundary (D-20, S-9)"
    anchor: dc-14
  - id: dc-15
    text: "the review packet is a derived view, not a persisted approval artifact; the profile-required review of the exact proposed head authorizes merge and the owner's merge is the single acceptance decision; requiring humans to type problem/outcome statements is rejected (D-21, D-22, D-29)"
    anchor: dc-15
  - id: dc-16
    text: "bounded, inspectable design context is the first half of the context-poisoning response; the mutation contract carries policy and context identity so the sealed-manifest and contradiction-detection track can later attach without redesign (D-23, Q-2)"
    anchor: dc-16
  - id: dc-17
    text: "project policy selects exactly one of off, proposal-only, draft-write; `layout` is reserved for a future presentation extension and remains false in v1 (D-24, NG-2)"
    anchor: dc-17
  - id: dc-18
    text: "AI assistance consumes the same Verdi-go-derived information a human designer can inspect — one coherent view of the system (D-25)"
    anchor: dc-18
  - id: dc-19
    text: "Jira epic/story provisioning is a deterministic, separately designed track with a recorded likely flow; no silent reverse synchronization from Jira prose into accepted specs (D-26, D-37, NG-3, NG-4)"
    anchor: dc-19
  - id: dc-20
    text: "dogfood observations measure system behavior (transactions, refusals, recovery, inferred-object churn, excerpt usage, effort-to-review), never agent scores or volume rewards; success measures demonstrate process coherence, not universal usability (D-27, Q-3)"
    anchor: dc-20
constraints:
  - id: co-1
    text: "every refusal is explicit and named: stale base returns the current digest and changed identities; forbidden actor/state/operation names the governing policy; an invalid result returns object-level diagnostics; one invalid operation lands nothing; concurrent writers serialize or refuse, never losing an update; three-valued honesty is preserved throughout (C-34..C-39, C-43)"
    anchor: co-1
  - id: co-2
    text: "strict decode everywhere: the request schema is versioned and strict-decoded; unknown configuration, schemas, fields, operations, and enum values fail closed; extensibility attaches only at the declared internal ports — policy evaluation, mutation application, provenance recording, semantic diffing, change notification, and transport adapters — and project-defined fields are mutated only through declared model descriptors, never a generic YAML escape hatch (S-3, C-26, C-36, C-46, C-28)"
    anchor: co-2
  - id: co-3
    text: "nonconfigurable core safety semantics: only draft specs accept semantic mutations; every mutation requires a base digest; stale mutations never auto-merge; the complete result is validated before write; spec and provenance form one transaction with crash recovery to a complete old or new state; operations address stable object IDs, never positions; agent actors remain distinct from humans; provenance is non-authoritative and build-context-excluded; agents cannot accept, approve, attest, waive, or disposition; a stale agent cannot overwrite intervening human edits; every response names the exact checkout, branch, HEAD, and spec (C-18..C-25, C-11, C-40..C-41, C-50, INV-1, INV-4)"
    anchor: co-3
  - id: co-4
    text: "actor identity is adapter-controlled: the payload cannot promote itself to a human actor via an `author` string; the agent does not self-authorize; no agent can claim to be human or bypass state and policy checks by rewriting through MCP (C-9, C-10, C-44, C-45)"
    anchor: co-4
  - id: co-5
    text: "provenance discipline: no complete transcript import; every excerpt targets a declared object, carries exactly one of human-stated / ai-synthesized / ai-inferred / unresolved, distinguishes verbatim from paraphrase, is bounded, removable, and redactable; excerpts record the target's digest at attach time; direct Markdown edits get `unclassified` origin, never fabricated attribution, and never a retroactive entry; generated provenance never counts as evidence and added volume improves no score (C-12..C-14, C-24, C-42, C-47, C-48, INV-2, P-1..P-3, NG-7)"
    anchor: co-5
  - id: co-6
    text: "policy hierarchy narrows only: a lower-level policy cannot re-enable a forbidden capability or relax a nonconfigurable invariant; policy changes follow normal ratification, produce a new policy digest, and affect future mutations only; existing provenance retains the policy identity under which it was created (C-27)"
    anchor: co-6
  - id: co-7
    text: "the Verdi-go boundary is preserved: exec pinned `flowmap`/`groundwork` CLIs and strict-decode declared outputs; never import packages, add a competing MCP server, duplicate graph or policy semantics, or let a harness bypass the pinned execution and decoding boundary (C-29..C-33)"
    anchor: co-7
  - id: co-8
    text: "corpus and annotation content returned to agents is untrusted data, never instructions, and cannot redefine the mutation or governance contract; harness-global files such as AGENTS.md or CLAUDE.md are never silently represented as Verdi decisions (C-17, C-51)"
    anchor: co-8
  - id: co-9
    text: "every operation carries table-driven happy and negative coverage (wrong state, forbidden actor, stale digest, invalid IDs, illegal links, batch rollback, coordinated-commit crash recovery, direct-Markdown disclosure, archive and exclusion behavior, unknown-anything rejection); the three adapters share a byte-identity conformance suite; browser paths have Playwright coverage including the protected unsaved edit and the AI-free journey; MCP/CLI end-to-end tests cover both harness shapes, all three policy modes, and refused governance operations; no test calls a live model, network service, or harness (C-52, C-53, T-1..T-4, ASD 620–678)"
    anchor: co-9
---
# AI-assisted spec design

## Problem

This canonical feature proposal promotes the ratified design document
`docs/superpowers/specs/2026-07-30-ai-assisted-spec-design.md`, source blob
`8595721911c3c756458ace195a686a871b6410d3`. The source remains the
provenance witness; this artifact is the lifecycle authority once the owner's
merge makes this exact revision reachable from `main`.

Verdi already contains most of the mechanical foundations, but they are
exposed through separate paths:

- The workbench board is a projection of the canonical `spec.md`. Its
  authoring API strict-decodes requests, splices named spec objects, validates
  before write, serializes in-process mutations, and writes through the
  atomic-file seam.
- Workbench authoring is limited to draft specs on design branches. Review
  mode is a mirror, and accepted specs are not general editing surfaces.
- The MCP server exposes the same deterministic board projection through
  `get_board`, but its only write tool is `add_annotation`. An agent cannot
  yet perform the same declared spec edits as a human using the board.
- Direct Markdown and Git remain the underlying portable authoring medium.
- The provider port supports resolving tracker records and publishing
  idempotent rollups. It does not create Jira epics or stories.
- Verdi already consumes Verdi-go capabilities through pinned CLI execution
  and strict JSON decoding. Importing Verdi-go packages or recreating its
  graph and policy surfaces would violate the existing boundary.

The missing piece is therefore not a new agent workspace or chat product. It
is a common, guarded mutation contract that lets human and agent interfaces
exercise the same domain behavior.

## Outcome

Verdi should let a human use an external agentic engineering harness such as
Codex or Claude Code to help explore a problem and author a feature or story
spec without making AI participation mandatory or turning design into an
autonomous generation step.

The key distinction is authorship versus authority. An agent may write any
part of a draft when the human asks it to, including problem and outcome
statements synthesized from a preceding conversation. The human remains the
semantic authority who decides whether the design is accepted. Human-driven
does not mean that a human must type every character; it means the human
cannot delegate the consequential governance decisions that make the draft
authoritative.

This design introduces one typed draft-mutation core shared by the workbench,
CLI, and MCP surfaces. It preserves direct Markdown editing as a first-class
workflow, keeps Verdi independent of any particular model or harness, and
records bounded design provenance without placing chat transcripts or
reasoning noise into normal build context.

The result should support all of these paths without giving any one of them
privileged semantics:

- a person designs entirely in the Verdi workbench without AI;
- a person edits Markdown and reviews normal Git diffs;
- a person brainstorms with an agent and asks it to update the open board;
- a person asks an agent to construct a complete draft from a conversation;
- people and agents interleave edits without silent overwrites;
- Codex and Claude Code use the same Verdi contract rather than separate
  integrations.

### Scope carried from the source

In scope:

- one typed, atomic draft-mutation core;
- workbench, CLI, and MCP adapters over the core;
- capability discovery and project policy;
- bounded supporting excerpts and committed design provenance;
- concise semantic review;
- bounded, inspectable design context;
- Codex and Claude Code interoperability through the same contract;
- preservation of direct Markdown and AI-free workflows;
- hermetic conformance, integration, and browser tests.

## Authority model

The following authority model applies regardless of interface:

| Action | Human | Delegated agent | Verdi |
|---|---:|---:|---:|
| Brainstorm and analyze | yes | yes | supplies bounded context |
| Create a draft | yes | yes, when policy permits | validates and records |
| Edit, delete, or reorder draft objects | yes | yes, when policy permits | validates and records |
| Add bounded supporting excerpts | yes | yes, with classification | stores outside build context |
| Review a semantic diff | yes | yes, advisory | derives the packet |
| Accept a design | yes | no | enforces the gate |
| Approve or merge the PR | yes | no | observes forge state where configured |
| Disposition a conflict or deviation | yes | no | records an explicit human act |
| Author a waiver or attestation | yes | no | validates the artifact |
| Choose whether semantic quality is sufficient | yes | no | cannot score understanding |

An agent actor must remain distinguishable from the human principal who
delegated the work. Adapter-controlled identity records the authenticated or
configured principal, harness, and session metadata. The mutation payload
cannot promote itself to a human actor merely by supplying an `author` string.

There is no global “auto-design” mode. A project may allow agent draft writes,
but an agent still acts in response to a human request and remains unable to
cross the governance boundary. This permits the practical workflow in which a
human talks through a problem and then asks the agent to construct the draft,
without pretending that copying the same generated text by hand creates more
human understanding.

Attribution adopts the shared governance-principal kernel representation from
the first canonical revision: it embeds either a canonical kernel principal
identifier or an explicit unauthenticated marker, never a bare principal
string. Attribution records authorship only. They do not authorize a mutation
or satisfy a human governance role merely by naming a principal.

## Design principles

1. **One canonical draft.** `spec.md` remains the semantic source of truth.
   The board, CLI, MCP, review packet, and provenance views are projections or
   controlled ways to mutate it, never competing documents.
2. **Equal domain operations.** An agent that is authorized to edit a draft
   uses the same typed operations and validation rules as the workbench. It
   does not drive the browser or receive a privileged file-rewrite path.
3. **Explicit human delegation.** An agent may mutate a draft when the human
   asks it to. Verdi does not add per-keystroke confirmation theater for
   reversible draft edits.
4. **Human-only governance.** Acceptance, PR approval, waivers,
   attestations, conflict or deviation dispositions, and equivalent judgment
   records cannot be authored or performed by an agent.
5. **Optimistic concurrency, fail closed.** Every typed mutation names the
   digest it read. A stale writer is refused and must reload; Verdi never
   silently merges two semantic edits.
6. **Refined output over conversational residue.** Normal design and build
   context contains accepted decisions and declared scope, not the transcript
   or hidden reasoning that led to them.
7. **Useful provenance, not provenance theater.** Short excerpts may help a
   later reviewer remember why an object exists. They are not evidence,
   instructions, proof of comprehension, or a second source of truth.
8. **Model- and harness-neutral.** Verdi publishes capabilities and typed
   contracts. Codex, Claude Code, future harnesses, and the workbench all
   consume those contracts without Verdi hosting a model.
9. **AI-free remains complete.** Disabling or declining AI assistance does
   not degrade the workbench, Markdown, Git, validation, review, or acceptance
   paths.
10. **Mechanics may be deterministic; quality remains judgment.** Verdi can
    validate shape, state, links, concurrency, and policy. It cannot
    deterministically prove that a problem statement is insightful or that a
    human understands a generated design.

## AC-1

### Shared draft-mutation architecture

The new core is a model-neutral application service inside Verdi. All
supported authoring adapters call it:

```text
human in workbench ─┐
human or script CLI ├─> typed draft-mutation core ─> spec.md
external agent MCP ─┘             │                  provenance sidecar
                                  └─> semantic diff + resulting digest
```

For each transaction, the core:

1. resolves the exact spec and checkout;
2. loads the canonical draft and calculates its digest;
3. confirms that the spec is in a mutable draft state and that project policy
   permits the actor and requested operations;
4. compares the supplied base digest with the loaded digest;
5. applies the ordered operations to an in-memory document;
6. strict-validates the entire resulting spec, including enums, identities,
   links, and model constraints;
7. prepares the matching provenance entry;
8. transactionally commits the spec and provenance update so a typed spec
   change cannot land without its required provenance record;
9. returns the previous digest, resulting digest, semantic changes, and
   warnings.

The implementation must provide crash recovery or rollback around the
coordinated file replacement. “Atomic” here is an observable contract: after
recovery, callers may see either the complete previous transaction or the
complete new transaction, never a typed spec edit with a missing provenance
record.

The first version owns semantic draft objects only. It does not switch
branches, commit, push, open or merge PRs, accept specs, run alignment,
disposition findings, publish evidence, or close work. Presentation-only
layout remains a separate board concern and is not smuggled into the semantic
mutation schema.

Existing workbench actions migrate to thin adapters over this core. A single
human gesture normally becomes a one-operation transaction. An agent may send
a larger ordered batch so it can construct a coherent draft without exposing
the user to a sequence of partially valid intermediate documents. The entire
batch succeeds or none of it does.

### Mutation contract

The request schema is versioned as `verdi.draftmutation/v1` and strict-decoded.
Unknown fields, operation names, enum values, or operation-specific fields
fail closed.

An illustrative request is:

```yaml
schema: verdi.draftmutation/v1
spec: spec/payment-retry
base_digest: sha256:0123456789abcdef
actor:
  kind: delegated-agent
  attribution: <kernel-attribution-record>
  harness: codex
  session: optional-session-id
operations:
  - op: set-problem
    text: Customers can be charged twice when a timed-out payment is retried.
  - op: add-acceptance-criterion
    id: ac-idempotent-retry
    text: Repeating a payment request with the same key creates one charge.
    evidence:
      - behavioral
      - runtime
sources:
  - target: problem
    classification: human-stated
    excerpt: We need retries, but a retry must never create a second charge.
```

`attribution` denotes the shared kernel attribution record; the placeholder is
illustrative type notation, not a literal value or a feature-local
serialization. The record contains exactly one of a canonical kernel principal
identifier or the kernel's explicit unauthenticated marker. The adapter
supplies it, unknown variants fail closed, and the kernel delivery unit owns
the concrete record grammar.

The adapter injects or verifies actor identity. The agent supplies the desired
semantic operations and may supply source classifications; it does not
self-authorize.

The closed operation vocabulary covers:

- setting the problem and intended outcome;
- adding, editing, removing, and reordering acceptance criteria;
- declaring evidence kinds for acceptance criteria;
- adding, editing, or removing constraints, decisions, and open questions;
- adding or removing legal typed links;
- adding, editing, removing, or reordering feature stubs;
- adding or removing explicit context references.

Each operation addresses a stable semantic object ID, never a Markdown line
number, byte offset, CSS selector, or board coordinate. IDs make operations
portable across the workbench, CLI, MCP, and raw formatting changes.

The result is also strict and deterministic:

```yaml
schema: verdi.draftmutation-result/v1
spec: spec/payment-retry
previous_digest: sha256:0123456789abcdef
result_digest: sha256:fedcba9876543210
changes:
  - target: problem
    change: replaced
  - target: acceptance-criterion/ac-idempotent-retry
    change: added
warnings: []
```

The semantic diff reports object-level meaning rather than a raw Markdown
patch. Deletions, large replacements, reordering, and relationship changes are
ordinary reversible draft operations, but they receive prominent treatment in
the result and later review packet.

## AC-2

### Workbench-first flow

1. The human opens an authoring board.
2. The external agent identifies the same checkout, branch, HEAD, and spec.
3. It calls `get_design_capabilities` and `get_board`.
4. It discusses or mutates according to the human request.
5. A successful mutation returns a new digest.
6. The workbench reloads or receives a change notification and shows the
   changed semantic objects.
7. Provenance stays out of the main board unless the human opens it.

The interface may emphasize newly changed objects, but it should not cover the
board in generic “AI-generated” badges. The review packet carries the useful
semantic and provenance distinctions.

An unsaved human inline edit must never be silently overwritten. The browser
either saves it first, keeps the stale base and forces a visible conflict, or
asks the human to discard it. The agent receives the same stale-base refusal
as any other concurrent writer.

The workbench, CLI, and MCP adapters expose the same application-service
contract. Once the workbench is migrated, it retains no parallel
interpretation of domain mutations.

## AC-3

### Configuration and capability discovery

Core safety semantics are not configurable:

- only draft specs accept semantic mutations;
- every typed mutation requires a base digest;
- stale mutations never auto-merge;
- the complete result is validated before write;
- the spec and required provenance update form one transaction;
- agent actors remain distinct from humans;
- provenance is non-authoritative and excluded from normal build context;
- agents cannot accept, approve, attest, waive, or disposition;
- unknown configuration, schemas, operations, and enum values fail closed.

Project policy controls the permitted assistance posture. An illustrative
configuration is:

```yaml
design_assistance:
  agent_writes: draft-write # off | proposal-only | draft-write
  allowed_operations:
    create: true
    edit: true
    delete: true
    reorder: true
    relationships: true
    layout: false
  direct_markdown:
    origin: disclose # allow | disclose | block
  provenance:
    excerpts: optional # off | optional | required-for-inference
    archive: true
    max_excerpt_length: 600
    max_excerpts_per_object: 3
  review:
    require_semantic_packet: true
    surface_ai_inferences: true
    surface_unclassified_edits: true
```

The modes mean:

- `off`: Verdi does not advertise or enable agent write capability;
- `proposal-only`: an agent may discuss, return proposed operations, and use
  ordinary annotations, but it cannot mutate the canonical draft;
- `draft-write`: an agent may use the permitted typed operations.

`layout` is reserved for a future presentation extension and remains false in
the v1 semantic mutation implementation.

A managed parent policy may restrict a repository, while a lower-level policy
may only narrow those permissions. It cannot re-enable a forbidden capability
or relax a nonconfigurable invariant. Policy changes follow the project's
normal ratification process, produce a new policy digest, and affect future
mutations only. Existing provenance retains the policy identity under which it
was created.

`get_design_capabilities` exposes:

- mutation and result schema versions;
- project and policy digests;
- checkout, branch, HEAD, spec, and current digest;
- current spec state;
- permitted operations;
- provenance and direct-Markdown posture;
- review requirements;
- available read, explanation, and context surfaces.

Extensibility occurs at explicit internal ports: policy evaluation, mutation
application, provenance recording, semantic diffing, change notification, and
transport adapters. Project-defined spec fields may be mutated only through
declared model descriptors. Unknown frontmatter remains invalid; extensions
do not create a generic arbitrary-YAML escape hatch.

The `design_assistance` block is a typed ASD payload inside Context
Integrity's single policy-authority system. Context Integrity owns its storage,
inheritance, effective-policy resolution, identity, and digest. ASD owns the
payload fields and the behavior they govern, but has no feature-local fallback,
competing hierarchy, or second policy interpretation.

## AC-4

### Direct Markdown origin

Raw Markdown edits remain legal. On the next Verdi read, they receive origin
`unclassified` because the system cannot honestly reconstruct who or what
produced them. Verdi must not fabricate agent or human attribution. Project
policy may allow, disclose, or block direct Markdown mutation, but the default
preserves it as a first-class workflow and surfaces the weaker provenance.

### Provenance and supporting excerpts

Typed mutations append a committed, content-addressed record to a
non-authoritative sidecar:

```text
.verdi/specs/active/<name>/design-provenance.jsonl
```

Each entry uses `verdi.design-provenance/v1` and contains:

- the spec ref;
- the previous and resulting spec digests;
- actor kind, kernel attribution record, harness, and optional session
  identifier;
- the ordered typed operations;
- object-level previous and resulting digests where applicable;
- optional bounded supporting excerpts and their classifications;
- the entry's own deterministic content digest.

The sidecar follows the spec into the archive. It is committed so review and
future archaeology do not depend on one harness's inaccessible memory, but it
is excluded from normal design and build context. It does not influence
acceptance, evidence folding, alignment, or execution unless a person or agent
explicitly requests the provenance view.

Verdi never imports a complete conversation transcript. An excerpt exists
only to jog memory or recover a small piece of context that might otherwise be
forgotten over time. It must target a declared spec object and carry one of
these classifications:

- `human-stated`;
- `ai-synthesized`;
- `ai-inferred`;
- `unresolved`.

The record also distinguishes verbatim text from a paraphrase. An agent may
record a concise paraphrase, but it must label it as such. Excerpts are
optional by default, bounded in length and count, removable, and redactable.
They are data, never instructions. They are not acceptance evidence, do not
override the spec, and cannot make an inferred claim authoritative.

An excerpt records the target object's digest at the time it was attached. If
the object changes later, the excerpt remains truthful historical provenance
rather than silently appearing to explain the new text.

Direct Markdown changes create no retroactive provenance entry. Verdi exposes
their origin as `unclassified` in review. A stricter project may forbid this
path, but disclosure is the default because a false audit trail would be
worse than an incomplete one.

The committed sidecar is an explicitly classified context candidate. Context
Integrity's compiler excludes it from normal design and build context using the
fixed reason code `design-provenance-sidecar`; it is never silently omitted
from the candidate universe and never included as authority. The path is the
owner-merged store-layout path, and the exclusion contract is the owner-ratified
OD-8 contract.

## AC-5

### Context integrity

This design also establishes the first half of a response to context
poisoning: agents should receive an explicit, inspectable Verdi context rather
than relying on opaque harness memory.

`get_design_context` returns only material relevant to design assistance:

- the current draft;
- an explicitly selected parent feature or child stories;
- applicable project policies and ratified decisions;
- the spec's declared pinned context references;
- Verdi-go-derived service and boundary findings relevant to the scope;
- the context and policy digests.

Provenance excerpts are excluded by default and available through their own
tool. Annotations and artifact bodies remain untrusted data, never
instructions. Harness-global files such as `AGENTS.md` or `CLAUDE.md` are not
silently represented as Verdi decisions.

Normal build context is narrower still: it contains the accepted spec,
applicable ratified project decisions and policies, explicit pinned context,
and deterministic execution inputs. It excludes design provenance,
brainstorming transcripts, abandoned alternatives, and unresolved chat.

This does not prove that Codex, Claude Code, or another harness has no hidden
memory. Verdi cannot control undocumented provider context. It can make the
Verdi-controlled payload inspectable, digestible, and minimal, and it can
require agents to disclose the exact Verdi context and policy digests they
used.

A sealed execution-context manifest and systematic contradiction detection
between project decisions, harness instruction files, feature specs, and story
specs remain a separate design track. The mutation contract must remain
compatible with that future work: each transaction carries policy and context
identity, and unknown or conflicting authoritative decisions can later become
explicit verdicts rather than prompt-level guesses.

## AC-6

### Human review and acceptance

Acceptance should add clarity, not ceremony. Before a human accepts a draft,
Verdi derives a concise semantic review packet from the spec, Git history, and
provenance sidecar. It is a view, not a persisted approval artifact or second
source of truth.

The packet contains:

- the problem and outcome;
- acceptance criteria and declared evidence kinds;
- constraints, decisions, open questions, links, and stubs;
- semantic additions, replacements, deletions, reorderings, and relationship
  changes since the review base;
- objects classified as `ai-inferred` or `unresolved`;
- direct edits whose origin is `unclassified`;
- material validation warnings or policy disclosures.

The packet does not ask the reviewer to certify that they read every token or
understand the model's reasoning. A human reviews this concise semantic view;
the profile-required review of the exact proposed head authorizes merge, and
the owner's merge of that pull request is the single decision that accepts the
specification. No separate acceptance command, status edit, or confirmation
repeats it, and extra checkboxes would create review theater without adding a
counterweight.

The agent may prepare or explain the packet but cannot mark it approved,
accept the design, approve the PR, or merge it. The human acceptance event
ratifies the refined spec, not the earlier conversation or its excerpts.

## AC-7

### Markdown- and Git-first flow

No workbench is required. A human or agent may:

1. inspect the Markdown and capability response;
2. use MCP or CLI typed mutations, or directly edit Markdown;
3. inspect the normal Git diff;
4. run `verdi design review <spec-ref>` to derive the semantic packet;
5. run the existing deterministic validation and acceptance gates;
6. use the normal PR review process.

The same spec, policies, and semantic review apply. Direct Markdown simply
has less attributable provenance.

## AC-8

### External agent experience

Verdi should expose a small common surface rather than recreate Codex or
Claude Code. The proposed tools are:

- `get_board`: return the deterministic board projection already shared with
  the human interface;
- `get_design_context`: return the bounded, authoritative material needed to
  assist with this draft;
- `get_design_capabilities`: declare the active schema, checkout, policy, and
  permitted operations;
- `mutate_draft`: apply one atomic typed transaction;
- `get_design_provenance`: return provenance only when explicitly requested;
- `prepare_design_review`: derive the semantic review packet without changing
  governance state.

These tools are transport adapters over Verdi behavior. The CLI exposes
equivalent structured commands for harnesses that prefer subprocesses.
Harness-specific setup should therefore be limited to MCP configuration and a
small instruction adapter that teaches the harness when to read, propose, or
mutate. There is no Codex-specific spec schema, Claude-specific lifecycle, or
second MCP server for the same behavior.

### Prompt-driven behavior

Harness instructions should map ordinary human requests to conservative,
legible actions:

| Human request | Expected agent behavior |
|---|---|
| “Brainstorm this with me” | Read bounded context and discuss; do not mutate |
| “Capture these findings” | Mutate only the selected objects or add ordinary annotations |
| “Draft the feature we discussed” | Create and populate an atomic draft transaction |
| “Update the board with this decision” | Apply the corresponding typed operation |
| “Review this design” | Analyze and propose; do not mutate unless separately asked |

This mapping is guidance, not hidden model policy. Verdi's enforcement remains
the capability and authority boundary.

### Branch and worktree identity

Every capability and mutation response names the exact checkout, branch,
HEAD, and spec. If a workbench and agent use the same checkout, changes may
appear live. If the agent works in an isolated worktree, Verdi exposes that
branch through the existing branch-aware board flow; it never silently copies
or merges content between worktrees. Handoff occurs through Git and Verdi
artifacts, not through an assumed shared chat memory.

## Relationship to Context Integrity and Guided Lifecycle and Governance

ASD consumes, but does not redefine, the owner-merged shared contracts:

- Guided Lifecycle and Governance owns lifecycle-wide governance-profile and
  principal requirements; Context Integrity records and enforces resolved
  profiles and principals; the governance-principal kernel implements the one
  shared profile schema, principal resolver, trust-source evaluation, and
  authorization interpretation.
- ASD mutation attribution uses the kernel attribution representation: a
  canonical kernel principal identifier or an explicit unauthenticated marker.
  It remains non-authoritative authorship testimony. Human governance roles
  require the authoritative principal-resolution path and block when identity
  or authorization is unproven.
- ASD policy is a typed feature-specific payload inside Context Integrity's
  single policy-authority system. Context Integrity exclusively owns storage,
  inheritance, effective-policy resolution, identity, and digest; ASD cannot
  add a feature-local fallback or competing interpretation.
- Context Integrity's compiler owns channel classification and excludes the
  ASD provenance sidecar under `design-provenance-sidecar`. ASD owns the
  sidecar schema and explicit provenance-read surfaces, never an alternate
  context compiler.
- The artifact contract owns the semantic object model, strict dialect, and
  typed link rules. Verdi Surfaces owns the board projection, MCP inventory,
  and workbench conventions. ASD adds typed draft operations through those
  seams rather than competing structures.

These are imposed owner-merged authorities (OD-2, OD-3, OD-5, OD-8, and
OD-12), not claims derived from the ASD source document.

## Verdi-go and Jira boundaries

Verdi-go is already integrated at the correct boundary: Verdi executes its
pinned `flowmap` and `groundwork` CLIs and strict-decodes declared outputs.
The design context may expose Verdi-derived service, boundary, and policy
findings to an assisting agent, but Verdi does not:

- import Verdi-go packages;
- add a competing Verdi-go MCP server;
- duplicate its graph or policy semantics;
- let harnesses bypass Verdi's pinned execution and decoding boundary.

AI assistance consumes the same derived information a human designer can
inspect. This keeps the integration coherent and prevents different harnesses
from building conflicting views of the system.

### Jira

The current Jira adapter resolves story records and publishes an idempotent
rollup. Deterministically provisioning feature specs as epics and story specs
as child user stories is valuable, but it is a separate track because it
introduces external identity, hierarchy, idempotency, reconciliation,
partial-failure, and freeze-timing semantics.

A likely future flow is:

```text
feature draft -> preview epic projection -> provision/reconcile epic
              -> bind external ref -> human accepts feature

accepted feature stub -> preview story projection -> provision child story
                      -> scaffold bound story design branch
```

Verdi remains authoritative and Jira receives a deterministic projection.
There is no silent reverse synchronization from edited Jira prose into the
accepted Verdi spec. This document records the need but does not add Jira
creation to the AI-assistance implementation.

## Delivery sequence

1. Implement and prove the core plus a structured CLI adapter.
2. Migrate existing workbench spec actions to the core.
3. Expose read-only design capabilities and bounded context.
4. Dogfood `proposal-only` with external harnesses.
5. Enable `draft-write` for the Verdi repository.
6. Add semantic review and provenance views.
7. Run the same journeys through Codex and Claude Code.
8. Broaden availability only after the chronicles show a coherent human
   journey and the failure modes remain legible.

Each step preserves the AI-free path and can be disabled without changing
spec semantics.

## Dogfood observations and success measures

The chronicles should continue recording process friction, but they should
measure the system's behavior rather than score the agent or reward volume.
Useful observations include:

- number and size of typed transactions;
- stale-write refusals and whether recovery was understandable;
- partial-write or recovery failures;
- AI-inferred objects later changed or removed by a human;
- how often reviewers open supporting excerpts;
- unclassified direct edits;
- commands and elapsed effort needed to reach semantic review;
- conflicts found before acceptance;
- context contradictions found late;
- occasions where a user must return to a full transcript because the refined
  artifacts were insufficient.

This design succeeds when dogfooding demonstrates all of the following:

1. A human can ask an external agent to create a complete draft that appears
   on the Verdi board.
2. A human can perform the same journey through the workbench or Markdown
   without AI.
3. Workbench, CLI, and MCP mutations have identical semantics.
4. Human and agent edits can interleave without silent overwrite.
5. Supporting excerpts are useful without importing transcripts.
6. Accepted design and normal build context exclude provenance and
   conversational noise.
7. The semantic review packet plus normal PR review exposes consequential
   changes without checkbox theater.
8. AI-disabled projects retain the complete existing workflow.
9. Agents cannot perform acceptance or other judgment-bearing governance
   acts.
10. Codex and Claude Code use the same Verdi capability and mutation
    contracts.
11. Verdi-go remains one pinned, strict-decoded integration rather than
    competing adapters.
12. Jira creation remains a deterministic, separately designed provisioning
    concern.

These measures demonstrate process coherence and fidelity to an accepted
spec. They do not prove universal usability or semantic quality by
themselves. That claim still requires continued chronicles across different
users, projects, harnesses, and failure cases.

## DC-1

One canonical draft means `spec.md` remains the semantic source of truth.
The board, CLI, MCP, review packet, and provenance views are projections or
controlled ways to mutate it, never competing documents.

Rejected as a second source of truth and unnecessary friction. When the human
asks an agent to draft, the agent may write the canonical draft through the
guarded mutation core. Git, provenance, stale-base refusal, semantic review,
and human acceptance provide the counterweights.

## DC-2

An authorized agent uses the same typed operations and validation rules as the
workbench. It does not drive the browser or receive a privileged file-rewrite
path.

Rejected because it couples semantics to UI structure and gives agents a
different, less deterministic path. The board and agent surfaces must call the
same typed core.

## DC-3

An agent may mutate a draft when the human asks it to. Verdi does not add
per-keystroke or per-mutation confirmation for reversible draft edits.

Rejected as approval theater for reversible draft work. Project policy and an
explicit human request authorize ordinary draft edits. Consequential
governance transitions remain human-only.

## DC-4

Acceptance, pull-request approval, waivers, attestations, conflict or deviation
dispositions, and equivalent judgment records are human-only. Agents may
prepare or explain material but cannot perform those governance acts.

## DC-5

Every typed mutation names the digest it read. A stale writer is refused and
must reload; Verdi never silently merges two semantic edits.

## DC-6

Normal design and build context contains accepted decisions and declared scope,
not the transcript or hidden reasoning that led to them.

Rejected as invasive and counterproductive. It enlarges the prompt-injection
surface, preserves abandoned reasoning, and poisons later build context.
Bounded, classified excerpts provide memory cues without treating the journey
as the decision.

## DC-7

Supporting excerpts can jog memory, but are not evidence, instructions, proof
of comprehension, a second source of truth, or an acceptance score.

Rejected. Provenance can explain origin and jog memory, but it cannot prove
correctness, compliance, comprehension, or human judgment. Treating it as a
score would invite gamification.

## DC-8

Verdi publishes one model- and harness-neutral capability and mutation
contract. It hosts no model and maintains no per-harness schema or lifecycle.

Rejected. Codex, Claude Code, and future harnesses already provide
conversation, model selection, and session management. Verdi should expose
high-fidelity context and domain operations rather than compete with them.

### Create separate integrations for each harness or Verdi-go

Rejected because duplicate adapters drift. Harnesses consume one Verdi
contract, and Verdi-go remains behind its existing pinned CLI and
strict-decoding boundary.

## DC-9

Disabling or declining AI assistance does not degrade the workbench, Markdown,
Git, validation, review, or acceptance paths.

## DC-10

Verdi validates mechanical facts such as shape, state, links, concurrency, and
policy. Semantic quality and human understanding remain judgment; the system
defines no quality score that provenance or generated volume can game.

## DC-11

There is no global auto-design mode. Even where policy allows draft writes, an
agent acts in response to a human request and remains unable to cross the
governance boundary.

## DC-12

The core is one model-neutral application service inside Verdi, and every
adapter is thin. A human gesture is normally one operation; an agent batch
succeeds entirely or not at all.

## DC-13

The semantic diff reports object-level meaning rather than a raw Markdown
patch. Deletions, large replacements, reordering, and relationship changes are
reversible draft work but remain conspicuous in both semantic and Git review.

## DC-14

Harness instructions should map ordinary human requests to conservative,
legible actions:

| Human request | Expected agent behavior |
|---|---|
| “Brainstorm this with me” | Read bounded context and discuss; do not mutate |
| “Capture these findings” | Mutate only the selected objects or add ordinary annotations |
| “Draft the feature we discussed” | Create and populate an atomic draft transaction |
| “Update the board with this decision” | Apply the corresponding typed operation |
| “Review this design” | Analyze and propose; do not mutate unless separately asked |

This mapping is guidance, not hidden model policy. Verdi's enforcement remains
the capability and authority boundary.

## DC-15

The semantic review packet is a derived view, not a persisted approval
artifact. The profile-required review of the exact proposed head authorizes
merge, and the owner's merge is the single acceptance decision.

Rejected. A human who already explored the problem with an agent would simply
copy generated text into those fields. The meaningful counterweight is
semantic human acceptance, not the mechanics of text entry.

## DC-16

Bounded, inspectable design context is the first half of the
context-poisoning response. Every transaction carries policy and context
identity so the sealed-manifest and contradiction-detection track can attach
later without redesigning the mutation contract.

## DC-17

Project policy selects exactly one assistance mode: `off`,
`proposal-only`, or `draft-write`. Presentation `layout` is reserved and
remains false in the v1 semantic mutation implementation.

## DC-18

AI assistance consumes the same Verdi-go-derived information a human designer
can inspect. The integration remains one pinned CLI-execution and strict-decode
boundary.

## DC-19

Jira epic and story provisioning is a separately designed deterministic track.
Verdi remains authoritative, Jira receives a projection, and edited Jira prose
never silently synchronizes back into an accepted spec.

Rejected for this scope. Jira provisioning can and should be deterministic,
but its identity and reconciliation semantics require a dedicated design.

## DC-20

Dogfood records system behavior—transactions, refusals, recovery, inferred
object churn, excerpt use, and effort to review—rather than scoring agents or
rewarding volume. Its measures demonstrate process coherence, not universal
usability or semantic quality.

## CO-1

### Failure behavior

The core distinguishes verdict failures from operational failures and makes
every refusal explicit:

- stale base digest: no write; return the current digest and changed object
  identities so the caller can reload;
- forbidden actor, state, or operation: no write; name the governing policy;
- malformed or unknown operation: no write;
- invalid resulting document: no write; return object-level validation
  diagnostics;
- one invalid operation in a batch: no operation in the batch lands;
- concurrent writer: serialize or refuse; never lose an update;
- provenance preparation or commit failure: the typed spec change does not
  become visible;
- crash during coordinated replacement: recovery produces the complete old or
  complete new transaction;
- raw Markdown edit: preserve the Git-recoverable change and disclose
  `unclassified` origin;
- accepted or review-mode spec: refuse semantic mutation even if an adapter
  mistakenly advertises it.

Every refusal preserves Verdi's three-valued honesty: proven,
violated-with-witness, or disclosed-as-unproven. Missing or ambiguous facts
are never silently treated as a pass.

### Abuse resistance

These controls resist common forms of abuse and gamification:

- An agent cannot claim to be human or perform governance actions.
- A harness cannot bypass state and policy checks by rewriting through MCP.
- A caller cannot obtain a “pass” by omitting unknown fields or inventing new
  operation names.
- A generated provenance record cannot count as evidence or understanding.
- Adding more excerpts, transactions, or AI-produced text does not improve a
  quality score because no such score exists.
- Deleting difficult constraints or acceptance criteria remains possible in a
  draft but is conspicuous in semantic review and ordinary Git review.
- A stale agent cannot overwrite intervening human edits.
- Data returned from the corpus is marked as untrusted and cannot redefine
  the mutation or governance contract.

## CO-2

The mutation request and result, project policy, sidecar entries, capability
responses, and every typed operation use versioned strict schemas. Unknown
schemas, configuration, fields, operations, enum values, and
operation-specific fields fail closed. Extensibility is limited to the six
declared internal ports and declared model descriptors; it never becomes an
arbitrary-YAML escape hatch.

## CO-3

Core safety semantics are not configurable:

- only draft specs accept semantic mutations;
- every typed mutation requires a base digest;
- stale mutations never auto-merge;
- the complete result is validated before write;
- the spec and required provenance update form one transaction;
- agent actors remain distinct from humans;
- provenance is non-authoritative and excluded from normal build context;
- agents cannot accept, approve, attest, waive, or disposition;
- unknown configuration, schemas, operations, and enum values fail closed.

Operations address stable semantic object identifiers, never Markdown lines,
byte offsets, selectors, or board coordinates. Every response names the exact
checkout, branch, HEAD, and spec. A stale agent cannot overwrite intervening
human edits.

## CO-4

Actor identity is adapter-controlled. The payload cannot promote itself to a
human actor via an `author` string, and the agent does not self-authorize.
For provenance, the adapter emits the shared kernel attribution
representation—canonical principal identifier or explicit unauthenticated
marker—plus ASD-owned harness and optional session metadata. An agent cannot
claim to be human or bypass state and policy checks through MCP.

## CO-5

No complete transcript is imported. Each excerpt targets a declared object,
carries exactly one classification—`human-stated`, `ai-synthesized`,
`ai-inferred`, or `unresolved`—distinguishes verbatim text from
paraphrase, and is bounded, removable, and redactable. It records the target
digest at attachment time. Direct Markdown receives `unclassified` origin
without fabricated or retroactive attribution. Provenance is non-authoritative
and cannot count as evidence or improve a score.

## CO-6

A lower policy layer may only narrow permissions; it cannot re-enable a
forbidden capability or relax a core invariant. Policy changes follow ordinary
ratification, produce a new policy digest, and apply prospectively. Existing
provenance retains the policy identity under which it was created. All storage,
inheritance, effective resolution, identity, and digest semantics come from
Context Integrity's one policy-authority system.

## CO-7

Verdi executes pinned `flowmap` and `groundwork` CLIs and strict-decodes
declared outputs. It never imports Verdi-go packages, adds a competing
Verdi-go MCP server, duplicates graph or policy semantics, or lets a harness
bypass the pinned execution and decoding boundary.

## CO-8

Corpus and annotation content returned to an agent is untrusted data, never
instructions, and cannot redefine the mutation or governance contract.
Harness-global files such as `AGENTS.md` or `CLAUDE.md` are not silently
represented as Verdi decisions.

## CO-9

### Verification strategy

No test calls a live model, network service, or harness. Tests submit
deterministic requests and assert exact artifacts, bytes, digests, semantic
diffs, and failures.

### Core tests

Every operation requires table-driven happy and negative cases, including:

- wrong spec state and forbidden actor;
- stale base digest;
- duplicate, missing, or invalid semantic IDs;
- illegal links and evidence kinds;
- ordered multi-operation batches;
- full rollback when any operation fails;
- coordinated spec/provenance failure and crash recovery;
- direct Markdown detection and disclosure;
- archive behavior and build-context exclusion;
- unknown schema, config, field, operation, and enum rejection.

### Adapter conformance

The workbench, CLI, and MCP adapters share a conformance suite. Given the same
base document, actor authority, policy, and operations, each must produce
byte-identical:

- resulting `spec.md`;
- provenance record;
- previous and resulting digests;
- semantic diff;
- warnings and error classifications.

The workbench must stop containing a parallel interpretation of domain
mutations once migrated.

### Browser behavior

Playwright covers:

- an external mutation appearing on an open board;
- cards, relationships, and review summaries updating correctly;
- an unsaved human edit being protected;
- interleaved human and agent transactions;
- provenance remaining on-demand rather than cluttering the board;
- assistance-disabled projects exposing no agent write controls;
- the existing AI-free authoring journey remaining unchanged.

### MCP and CLI behavior

End-to-end tests cover:

- Codex-shaped and Claude-Code-shaped callers using the identical schema;
- all three policy modes;
- an atomic complete-draft batch;
- stale and concurrent callers;
- unclassified direct Markdown;
- capability discovery and policy-digest changes;
- every attempted governance operation being absent or refused.

## Non-goals

The first version owns semantic draft objects only. It does not switch
branches, commit, push, open or merge pull requests, accept specs, run
alignment, disposition findings, publish evidence, or close work.
Presentation-only layout remains a separate board concern and is not smuggled
into the semantic mutation schema.

Out of scope:

- an embedded chat, model picker, or model-hosting layer;
- selecting or prescribing a particular model;
- full transcript import;
- autonomous design acceptance, PR approval, waiver, attestation, or
  disposition;
- semantic quality or human-understanding scores;
- a second proposal document, agent board, or canonical spec;
- a competing Verdi-go integration or MCP server;
- Jira epic or story creation;
- a sealed build-context manifest and full contradiction detector;
- proving that an external harness has no opaque global memory.
