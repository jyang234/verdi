# AI-assisted spec design

Date: 2026-07-30
Status: approved in design session; awaiting document review

## Contents

1. [Purpose](#purpose)
2. [Current-state grounding](#current-state-grounding)
3. [Design principles](#design-principles)
4. [Human and agent authority](#human-and-agent-authority)
5. [Shared draft-mutation architecture](#shared-draft-mutation-architecture)
6. [Mutation contract](#mutation-contract)
7. [Provenance and supporting excerpts](#provenance-and-supporting-excerpts)
8. [External agent experience](#external-agent-experience)
9. [Human review and acceptance](#human-review-and-acceptance)
10. [Context integrity](#context-integrity)
11. [Configuration and extensibility](#configuration-and-extensibility)
12. [Verdi-go and Jira boundaries](#verdi-go-and-jira-boundaries)
13. [Failure behavior and abuse resistance](#failure-behavior-and-abuse-resistance)
14. [Testing and rollout](#testing-and-rollout)
15. [Dogfood observations and success measures](#dogfood-observations-and-success-measures)
16. [Scope boundaries](#scope-boundaries)
17. [Decisions and rejected alternatives](#decisions-and-rejected-alternatives)

## Purpose

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

## Current-state grounding

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

## Human and agent authority

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

## Shared draft-mutation architecture

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

## Mutation contract

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
  principal: johnyang
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

Raw Markdown edits remain legal. On the next Verdi read, they receive origin
`unclassified` because the system cannot honestly reconstruct who or what
produced them. Verdi must not fabricate agent or human attribution. Project
policy may allow, disclose, or block direct Markdown mutation, but the default
preserves it as a first-class workflow and surfaces the weaker provenance.

## Provenance and supporting excerpts

Typed mutations append a committed, content-addressed record to a
non-authoritative sidecar:

```text
.verdi/specs/active/<name>/design-provenance.jsonl
```

Each entry uses `verdi.design-provenance/v1` and contains:

- the spec ref;
- the previous and resulting spec digests;
- actor kind, principal, harness, and optional session identifier;
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

## External agent experience

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

### Branch and worktree identity

Every capability and mutation response names the exact checkout, branch,
HEAD, and spec. If a workbench and agent use the same checkout, changes may
appear live. If the agent works in an isolated worktree, Verdi exposes that
branch through the existing branch-aware board flow; it never silently copies
or merges content between worktrees. Handoff occurs through Git and Verdi
artifacts, not through an assumed shared chat memory.

## Human review and acceptance

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
understand the model's reasoning. A human reviews this concise semantic view,
runs the existing acceptance ritual, and uses the normal PR approval. Those
are sufficient; extra checkboxes would create review theater without adding a
counterweight.

The agent may prepare or explain the packet but cannot mark it approved,
accept the design, approve the PR, or merge it. The human acceptance event
ratifies the refined spec, not the earlier conversation or its excerpts.

## Context integrity

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

## Configuration and extensibility

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

## Verdi-go and Jira boundaries

### Verdi-go

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

## Failure behavior and abuse resistance

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

## Testing and rollout

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

### Rollout sequence

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

## Scope boundaries

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

## Decisions and rejected alternatives

### Require humans to type problem and outcome statements

Rejected. A human who already explored the problem with an agent would simply
copy generated text into those fields. The meaningful counterweight is
semantic human acceptance, not the mechanics of text entry.

### Import the complete chat transcript

Rejected as invasive and counterproductive. It enlarges the prompt-injection
surface, preserves abandoned reasoning, and poisons later build context.
Bounded, classified excerpts provide memory cues without treating the journey
as the decision.

### Keep agent output in a separate proposal layer

Rejected as a second source of truth and unnecessary friction. When the human
asks an agent to draft, the agent may write the canonical draft through the
guarded mutation core. Git, provenance, stale-base refusal, semantic review,
and human acceptance provide the counterweights.

### Require confirmation for every agent mutation

Rejected as approval theater for reversible draft work. Project policy and an
explicit human request authorize ordinary draft edits. Consequential
governance transitions remain human-only.

### Let agents edit through browser automation

Rejected because it couples semantics to UI structure and gives agents a
different, less deterministic path. The board and agent surfaces must call the
same typed core.

### Embed a Verdi AI assistant

Rejected. Codex, Claude Code, and future harnesses already provide
conversation, model selection, and session management. Verdi should expose
high-fidelity context and domain operations rather than compete with them.

### Create separate integrations for each harness or Verdi-go

Rejected because duplicate adapters drift. Harnesses consume one Verdi
contract, and Verdi-go remains behind its existing pinned CLI and
strict-decoding boundary.

### Use provenance as evidence or an acceptance score

Rejected. Provenance can explain origin and jog memory, but it cannot prove
correctness, compliance, comprehension, or human judgment. Treating it as a
score would invite gamification.

### Include Jira creation in this implementation

Rejected for this scope. Jira provisioning can and should be deterministic,
but its identity and reconciliation semantics require a dedicated design.
