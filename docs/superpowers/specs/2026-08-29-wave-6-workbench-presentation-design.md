# Wave 6 Workbench Presentation Authority Design

**Status:** Owner-approved planning authority with the SI-177 amendment pending
its one independent review and owner merge. Task 1B implementation and Task 2
resumption remain blocked until that amended exact head reaches the configured
default branch.

**Planning base:** `915529f792f7a672e9631f42909995b38ed12655`

**SI-177 amendment base:** `ab7518975b6621aceeef4607cca29d9a87cd75b7`

**Owners:** platform-team

**Delivery shape:** ten serialized predecessor/presentation units followed by
one integrated Wave 6 gate. Each unit is one reviewed pull request. The added
units are the owner-approved ASD browser-human authority correction and
writer-process transaction correction inserted after successive Task 2 stop
gates; both must merge before the ASD workbench resumes.

**Frontend owner:** FABLE. Sonnet workers implement non-frontend predecessors;
Opus workers challenge and repair accepted defects; Codex independently judges
every completed unit after the Claude/FABLE producer chain stops.

## Contents

1. Decision and scope
2. Effective authority and audited entry state
3. Shared workbench architecture
4. Browser authority, identity, and failure semantics
5. Live refresh, accessibility, responsiveness, and performance
6. ASD predecessor and workbench
7. CI predecessor and workbench
8. GLG predecessors and workbench
9. CSE coordinator and workbench
10. Serialized delivery and review protocol
11. Verification and Wave 6 exit gate
12. Lossless source-coverage witness

## 1. Decision and scope

Wave 6 delivers the four ratified workbench presentations in this fixed order:

1. AI-assisted spec design (ASD) application predecessor;
2. ASD browser-human mutation-authority correction;
3. ASD writer-process transaction correction;
4. ASD workbench;
5. Context Integrity (CI) application predecessor;
6. CI workbench;
7. Guided Lifecycle and Governance (GLG) Wave 5 predecessors;
8. GLG workbench;
9. Comparative Spike Experiments (CSE) browser-neutral human-proof
   coordinator;
10. CSE workbench; and
11. integrated Wave 6 review and gates.

The sequence is fully serialized. A later unit starts only from the
owner-merged, independently Codex-approved head of its predecessor. The split
does not create browser-owned cores: every UI unit consumes one typed
application operation whose non-browser behavior is already tested.

The current default branch contains the complete CSE Wave 5 lifecycle, but it
does not contain all Wave 5 predecessors promised for ASD, CI, and GLG. The
missing behavior is therefore delivered as named predecessor units rather than
silently implemented in HTTP handlers or JavaScript.

Those predecessor units retain their original Wave 5 obligation identities;
the owner-approved Wave 6 campaign is their delivery position, not a claim that
the unticked orchestration rows were already complete or a renaming of their
semantics. Each row becomes complete only when its predecessor unit merges.

This design does not add:

- a browser database, local-storage authority, service worker, event-sourced UI
  lifecycle, or mutable status cache;
- a second policy resolver, journey state table, experiment decision engine,
  design mutation grammar, provenance grammar, or human identity system;
- browser-held private keys, browser signing, ambient Git-user authority,
  self-declared principals, or reusable authenticated sessions;
- optimistic success, inferred readiness, automatic ratification, automatic
  acceptance, automatic recovery, or automatic destructive cleanup;
- a frontend framework, package-manager dependency, websocket, or SSE channel;
  or
- any Wave 7 dogfood, release, or whole-program approval claim.

## 2. Effective authority and audited entry state

`verdi spec state` at the planning base proves the effective feature authority:

| Feature | Effective specification | State |
|---|---|---|
| ASD | `spec/ai-assisted-spec-design` | `accepted-pending-build`, exact |
| CI | `spec/context-integrity-v2` | `accepted-pending-build`, exact |
| GLG | `spec/guided-lifecycle-governance-v3` | `accepted-pending-build`, exact |
| CSE | `spec/comparative-spike-experiments-v3` | `accepted-pending-build`, exact |

The superseded CI, GLG, and CSE predecessors are history, not alternative
implementation authority.

### 2.1 Existing presentation foundation

The repository already provides:

- server-rendered `internal/workbench` pages and the shared dex stylesheet;
- branch-aware board routes and typed board projections;
- the Wave 3.5 `/readiness` pilot, its four-area rail, attention queue, exact
  three-item preview, complete inline expansion, and solo-author language;
- `verdi serve`, health/readiness endpoints, hermetic handler tests, and
  Playwright infrastructure; and
- strict application cores and adapter parity for the complete CSE Wave 5
  operation set.

These are reused. The Wave 3.5 cockpit is promoted as a visual and interaction
shell, not as lifecycle authority or proof that GLG is complete.

### 2.2 Missing predecessor proof

The audit found four hard stop gates:

| Area | Current gap | Required owner before UI |
|---|---|---|
| ASD | `internal/workbench/boardspecapi.go` still owns direct artifact mutation while capability, bounded-context, semantic-review, and on-demand-provenance adapter parity is incomplete | one `internal/designapp` application core composed from `draftmutation`, `designprovenance`, artifact, Git, and context owners |
| CI | policy authority, context compilation, and conflict evaluation exist, but no application operation owns propose, validate, impact-review, and submit preparation | one `internal/constitutionapp` consumer over the existing CI packages |
| GLG | `internal/journey` still marks eventual readiness as deferred; feature attestation, lifecycle recovery, and journey metrics lack the complete application/adapter path | complete typed owners plus one `internal/journeyapp` consumer |
| CSE | the application core exists, but canonical human challenge/proof orchestration is private to `cmd/verdi` | one browser-neutral coordinator in `internal/experimentapp` or a narrower shared consumer-owned package |

If a predecessor cannot be built from existing ratified schemas and authority,
its unit stops with `NEEDS_CONTEXT`; the UI unit does not invent a substitute.

## 3. Shared workbench architecture

```text
browser GET/action
      |
      v
internal/workbench route + render adapter
      |
      v
typed feature application operation
      |
      +-- schema/state owner
      +-- accepted Git snapshot
      +-- policy/governance owner
      +-- mutation or execution owner
      `-- immutable/canonical artifact seam
```

The browser adapter may:

- strict-decode one bounded request;
- obtain checkout, branch, accepted-HEAD, and revision facts through existing
  ports;
- invoke one typed application operation;
- render its returned typed facts without reinterpretation; and
- refresh the same projection conditionally.

It may not:

- derive semantic state, policy, readiness, recommendation, authority, or
  provenance independently;
- write feature artifacts directly;
- turn a display label, button state, HTTP status, or JavaScript variable into
  lifecycle truth;
- suppress violated or unproven facts; or
- claim success before the underlying operation returns a clean typed result.

### 3.1 Shared shell

Every Wave 6 page preserves the pilot's primary interaction model:

- the four ordered areas `shape-proposal`, `show-success`, `check-context`, and
  `request-review`;
- one deterministic current focus;
- an attention queue containing every unresolved concern;
- exactly three preview rows before the complete list is expanded inline;
- immediate/current concerns before distant downstream concerns, with timing
  and dependency stated rather than inferred from position;
- source-derived corrective guidance for every violated or unproven row;
- a plain explanation of why the four areas occur in that order and what must
  be true before moving forward;
- human-review work labeled plainly as human review, with the technical
  obligation retained as secondary evidence;
- plain-language primary labels with exact formal identifiers and states as
  secondary text;
- the complete concern set even when the queue changes prominence; and
- explicit `proven`, `violated-with-witness`, and `unproven` language.

The shell is presentation. Each feature's application core supplies the facts
and permissible actions that populate it.

### 3.2 Closed route grammar

The v1 route inventory is:

| Surface | Page route | Conditional projection route |
|---|---|---|
| ASD | existing `/board/spec/{name}` | existing page path plus `/snapshot` |
| CI | `/constitution` | `/constitution/snapshot` |
| GLG | `/journey/{ref}` and existing `/readiness` | page path plus `/snapshot` |
| CSE | `/experiment/{spike}/{experiment}` | page path plus `/snapshot` |

Canonical refs that contain `/` are encoded as one path segment by one shared
workbench route constructor. A handler never concatenates raw refs. Feature
actions use `POST <page-path>/api/<application-operation>` where the final
token is the exact closed application operation name. Unknown routes,
operations, fields, nulls, duplicate keys, oversized bodies, and trailing data
fail before any application call.

The server returns an honest rendered result and a fresh revision token. It
does not create a second canonical JSON artifact or promise that HTML bytes are
machine authority.

## 4. Browser authority, identity, and failure semantics

### 4.1 Actor custody

ASD browser draft mutations carry the kernel's explicit unauthenticated-human
attribution when no real principal evidence exists. The browser, form fields,
cookies, OS user, Git author, and server process identity cannot mint a
principal. This attribution is provenance only.

That state is minted only by a sealed `NewUnauthenticatedHuman` kernel
constructor. Its private sealed basis is distinct from a human actor produced
by a violated or unproven governance-principal resolution even though both
serialize the kernel's unauthenticated attribution. The latter retains the
existing policy-gated behavior; a failed identity proof does not acquire the
browser-human allowance. MCP and request decoders have no route to construct
the browser-human actor.

The `design_assistance` payload governs agent participation, not this explicit
browser-human draft action. The browser-human action therefore does not require
policy-authority adoption and does not consult the assistance mode as an
authorization input. If a valid effective policy exists, the mutation records
its sealed digest as provenance only. If policy authority is genuinely not
adopted, the mutation proceeds with the explicit policy posture
`not-applicable`. Any adopted but malformed or unsealed policy authority still
fails operationally; absence is not allowed to hide corruption.

This requires `verdi.design-provenance/v2`. V1 remains strict decode-only
history. New writers emit V2 with one required, non-null top-level `policy`
object and exactly one closed arm:

- `{"state":"resolved","digest":"sha256:..."}` when a valid effective
  policy identity exists; or
- `{"state":"not-applicable"}` only for the explicit unauthenticated-human
  shape when policy authority is genuinely not adopted.

The `resolved` arm requires a canonical effective-policy digest; the
`not-applicable` arm forbids a digest. Delegated-agent entries always require
`resolved`. V2 forbids the V1 `policy_digest` field, while V1 continues to
require it and forbids `policy`. Unknown, missing, null, duplicate, cross-arm,
and trailing data fail closed. Existing mutation, chain, attribution, context,
operation, change, excerpt, canonical-JSON, and self-digest rules are
unchanged. Mixed V1/V2 logs decode and validate in order, while no current
writer emits V1. No sentinel or hash of absence may be presented as a policy
identity.

The accepted ASD feature specification's sentence that each entry uses
`verdi.design-provenance/v1` describes the original schema at ratification.
SI-176 and this owner-approved Wave 6 amendment explicitly supersede that
historical writer version without editing the frozen accepted artifact, using
the same ledger-plus-wave-authority evolution pattern as SI-161's ratification
V3 transition.

CI and GLG operations use the authority required by their typed core. If an
operation needs authenticated human authority and no accepted proof seam
exists, its predecessor stops; the browser cannot downgrade it to an
unauthenticated action.

CSE human operations retain the Wave 5 offline proof boundary. The workbench:

1. requests the canonical application-owned challenge;
2. renders and downloads those exact bytes;
3. instructs the human to sign them outside Verdi;
4. accepts exactly one raw 64-byte detached Ed25519 signature file; and
5. submits challenge and proof through the shared coordinator.

The browser never reads, stores, generates, requests, or transmits a private
key. It does not invent a proof envelope or browser session.

### 4.2 Proposed versus accepted state

Browser mutation writes only the same proposal artifacts as CLI/MCP. Git and
the configured accepted default-branch HEAD remain the acceptance boundary.
Every authority-bearing page shows:

- checkout and branch;
- current worktree HEAD;
- accepted HEAD;
- clean/dirty and ahead/behind/diverged posture when available;
- whether displayed bytes are proposed or accepted; and
- the exact operation classification and disclosures.

No action button changes accepted state by itself.

### 4.3 Failure classes

Browser responses preserve the repository's three-way operation result:

| Core class | Browser treatment |
|---|---|
| clean / exit 0 | render success only after the operation and fresh projection both verify |
| verdict / exit 1 | HTTP response remains usable and renders the exact verdict, witnesses, and corrective destination |
| operational / exit 2 | render a stable operational panel; never show a favorable state; no partial action effect may be hidden |

HTTP status is transport posture, not proof meaning. Tests assert the typed
classification and code in the body/data attributes, not merely a 2xx/4xx
number.

## 5. Live refresh, accessibility, responsiveness, and performance

### 5.1 Conditional live refresh

Every application projection supplies a deterministic revision token over all
facts rendered by that page. Accepted HEAD alone is insufficient when the
projection also depends on worktree, forge, policy, evaluator, or run facts.

The browser behavior is fixed:

- poll the feature's `/snapshot` route every two seconds only while the page
  is visible;
- send the last exact revision token;
- receive `304 Not Modified` when the token is unchanged;
- replace only the server-rendered projection region when it changes;
- expose a keyboard-reachable manual **Refresh** control using the same route;
- pause timers when `document.hidden` is true and restart with one immediate
  conditional refresh when visible; and
- retain unsaved edits, focus, expanded rows, and the last action result across
  background refresh.

No polling result writes authority. A stale action request is refused by the
application operation's expected-revision/HEAD precondition and returns a fresh
projection.

### 5.2 Accessibility and responsive contract

Every Wave 6 page must prove:

- semantic landmarks and heading order;
- a skip link and visible focus;
- complete keyboard access with no focus trap;
- status changes announced through a bounded live region without repeatedly
  announcing unchanged polling responses;
- labels, descriptions, and error association for every control;
- no meaning conveyed by color alone;
- usable layout at 320 CSS pixels and at 200% zoom;
- horizontal scrolling contained to intrinsically wide evidence/code regions;
  and
- reduced-motion behavior for any transition.

Playwright tests exercise keyboard flows and an automated accessibility scan;
static markup tests alone do not satisfy this contract.

### 5.3 Deterministic performance budgets

Wave 6 uses structural budgets instead of flaky wall-clock thresholds. A
*projection* is one independently requested passive page or on-demand panel;
the initial page does not compose several operations that each re-enumerate the
accepted tree. Heavy CSE capsule, closure, or run-detail panels remain explicit
on-demand projections when no single existing aggregate read owns those facts.

The budgets are:

- one page projection performs one accepted-HEAD resolution and one accepted
  tree enumeration unless a cited predecessor API proves a separate immutable
  source is required;
- a conditional refresh invokes one application projection and no mutation;
- no new JavaScript asset exceeds 64 KiB uncompressed;
- no hermetic maximum-fixture initial HTML response exceeds 512 KiB;
- the page is fully usable from the initial server response before JavaScript
  or polling; and
- tests and production contact no external network.

## 6. ASD predecessor and workbench

### 6.1 ASD application predecessor

`internal/designapp` becomes the sole application consumer for the six ASD
operations already fixed by AC-8:

- `get_board`;
- `get_design_context`;
- `get_design_capabilities`;
- `mutate_draft`;
- `get_design_provenance`; and
- `prepare_design_review`.

It composes the existing artifact, `draftmutation`, `designprovenance`,
context, Git, and board projection owners. In the final Task 2 state CLI, MCP,
and workbench call it. Task 1 routes CLI/MCP through it but intentionally leaves
the pre-existing shipped board splice path unchanged because Task 1 cannot
touch frontend files. Task 2 must atomically rewire every board mutation to
`designapp` and delete the splice path in the same unit. The forbidden state is
a merged workbench that calls `designapp` while its legacy direct mutation path
remains active.

The predecessor proves strict request/response contracts, deep-copy custody,
stale-revision refusal, capability enforcement, direct-edit disclosure,
bounded context, on-demand provenance, deterministic semantic review, and
CLI/MCP conformance. It adds no frontend files.

#### 6.1.1 Browser-human authority correction

The Task 2 stop gate proved that the merged predecessor cannot yet construct
the exact browser actor required by §4.1 and that the current mutation service
requires `design_assistance` even for the AI-free workbench journey. Before
Task 2 resumes, one non-frontend predecessor unit must:

1. add the sealed explicit unauthenticated-human constructor and basis;
2. preserve the existing delegated-agent and unproven/violated-resolution
   authorization matrix byte-for-byte;
3. authorize the explicit browser-human path independently of assistance mode
   and policy adoption while retaining operational refusal for malformed
   adopted authority;
4. implement the V1-decode/V2-write provenance dispatch and exact policy union
   from §4.1; and
5. prove that no current CLI, MCP, or request-decoder path can mint the actor,
   no policy is fabricated, and existing V1 history remains readable. Task 2
   separately proves that its browser mutation adapter becomes the sole
   production caller.

This correction touches no frontend, route, board, JavaScript, CSS, or
Playwright file. It merges as its own independently reviewed predecessor before
the FABLE-owned ASD workbench implementation continues.

#### 6.1.2 Writer-process transaction correction

The second Task 2 stop gate proved that the application operation is still
unreachable from its production hosts. `verdi serve` and standalone
`verdi mcp` acquire `.verdi/data/writer.lock` for their lifetime as I-12's one
writer process, while every `draftmutation.Service.Mutate` attempts to acquire
that same non-reentrant file lock. `filelock.Acquire` correctly refuses the
live holder even when it is the caller's process, so served workbench and live
MCP mutations fail operationally before the transaction begins.

The correction preserves the lock's process boundary and adds the missing
transaction boundary inside that process:

1. `filelock.Acquire` remains non-reentrant. A live existing lock still
   returns `ErrHeld`; no caller receives a second lock handle or implicit
   ownership merely because the lock body names its PID.
2. `filelock` records successful acquisitions in a synchronized process-local
   ownership registry. A read-only query reports current-process ownership
   only when the exact requested lock path still names the same file acquired
   by this process. PID text, liveness, or a caller-authored lock body alone is
   never ownership proof.
3. `draftmutation.WithWriterLock` serializes complete transactions with a
   process-local mutex scoped to the validated checkout writer-lock path's
   cleaned absolute spelling. Existing component-by-component `lstat` refusal
   runs before the key is trusted, so a symlink spelling cannot become an
   alternate ownership or mutex identity. Different checkouts are not globally
   serialized.
4. After entering that mutex, `WithWriterLock` first uses the ordinary
   acquisition path. If it acquires the lock, it releases exactly that handle
   after the callback. If acquisition returns `ErrHeld` and the registry proves
   this process owns the exact still-present lock, it reuses the outer
   exclusion and does not release it. Every other held, malformed, replaced,
   unreadable, or unproven case remains an operational refusal.
5. The mutex covers validation, journal recovery/write, spec and provenance
   writes, fsync, callback completion, and any inner-acquired release. Crash
   safety, journal ordering, stale-lock takeover, symlink refusal, and
   cross-process contention remain unchanged.
6. Until Task 2 deletes the legacy board splice writer, Task 1B routes
   `boardSpecServer.spliceSpec`'s complete read/parse/apply/validate/atomic-write
   callback through `WithWriterLock`. That temporary adapter participates in
   the same per-checkout serialization without gaining a second lock or
   journal algorithm. Task 2 removes the legacy path atomically as already
   required.

SI-177 narrowly supersedes SI-69's phrase “refusing while serve/another writer
holds it” only when “serve” is this caller process and the process-local
registry proves its exact outer lock. “Another writer” continues to mean every
other process and every unproven, forged, or replaced local lock, all of which
remain refused. The registry is ephemeral lock custody, never durable state,
request input, artifact authority, or a substitute for the on-disk exclusion;
it disappears on crash, leaving the existing stale-lock takeover protocol to
recover the checkout.

This is process-bound reuse, not recursive transaction nesting: a mutation
callback must not call `WithWriterLock` again, and the public Go documentation
retains that non-recursive contract. The serialization claim covers
`WithWriterLock` transactions and the temporary legacy `spec.md` splice routed
through that operation; it is not a claim that every file written by the serve
process shares one mutex. In particular, `boardio` owns distinct annotation
JSONL files outside the spec/provenance transaction projection. SI-177 neither
aliases those files to `spec.md` nor claims a transaction write can replace an
annotation append; Task 2 retains the existing boardio owner and its explicit
post-clean-transaction ordering.

The correction creates no actor, request, route, or browser authority and
changes no artifact bytes. It repairs the already-shipped live `mutate_draft`
MCP path and supplies the same kernel behavior to the later workbench adapter.
Task 1B is non-visual, but its temporary legacy-handler participation is owned
by a FABLE frontend worker under the repository's frontend exception; Sonnet
owns the filelock/draftmutation kernel. The unit merges and is independently
Codex-approved before Task 2 resumes.

### 6.2 ASD workbench

The existing board gains:

- a revision/posture header;
- synchronized canonical object rendering;
- slug/path grammar validation before an author invests in the draft, with the
  rejected bytes and corrective grammar shown at the relevant field;
- capability-driven in-place correction of existing stubs and objects after
  creation, through the same typed transaction rather than delete/recreate;
- unsaved-edit protection across refresh and navigation;
- typed mutation forms driven by returned capabilities;
- on-demand provenance kept collapsed by default;
- semantic review with additions, replacements, deletions, reorderings,
  relationship changes, warnings, and unclassified direct edits; and
- a graduation/registration impact preview before the durable mutation,
  including resulting refs, paths, relationships, affected downstream facts,
  and any blocking unknown; and
- explicit proposal/accepted state and next action.

The page never claims that provenance establishes authority. Direct Markdown
remains legal and visibly `unclassified` unless policy forbids it.

## 7. CI predecessor and workbench

### 7.1 Constitution application predecessor

`internal/constitutionapp` owns the browser-neutral application flow:

- inspect the effective constitution and exact source layers;
- create or amend a Git-backed proposal;
- strict-validate the proposal;
- derive its rule ledger and applicability/derivation trail;
- run mechanical and required semantic conflict evaluation through existing
  CI owners;
- derive an impact-review packet over affected contexts, capabilities, and
  governed operations; and
- prepare submission without merging or inventing approval.

It composes `policyartifact`, `policyauthority`, `contextcompile`,
`policyconflict`, Git, and governance-principal seams. It cannot add
precedence, a policy language, a second conflict evaluator, or a UI-only
approval record. CLI and workbench may expose the complete human workflow;
MCP exposes only read, validation, and review projection and structurally
refuses commit, submission, approval, exemption ownership, and semantic
disposition. Record parity means those adapters consume the same application
records, not that an agent receives every human operation.

### 7.2 Constitution workbench

The constitution page presents:

- source layers, owners, scopes, precedence, digests, and accepted posture;
- the effective rule ledger without flattening away provenance;
- a derivation trail for each effective rule;
- proposal editing through typed application operations;
- mechanical, semantic, exemption, and unresolved conflict witnesses;
- impact review before submission; and
- the normal Git proposal/merge boundary.

Unknown applicability, unavailable judges, stale dispositions, and opaque
external authority remain visible and blocking where the core says so.

## 8. GLG predecessors and workbench

### 8.1 GLG Wave 5 predecessors

The GLG predecessor unit completes, in the ratified order:

1. lifecycle-wide current and eventual readiness;
2. feature-outcome attestation scaffolding and review;
3. diagnosis-first lifecycle recovery; and
4. privacy-bounded journey metrics over canonical observational events.

`internal/journey` continues to own journey derivation. New schema owners stay
separate; `internal/journeyapp` composes their typed operations for CLI, MCP,
and workbench parity. The current `deriveEventual` deferral must be removed by
authority-backed logic, not by display heuristics.

Recovery is read-only by default. It emits diagnosis, witness requests,
preconditions, exact reversible local actions when an existing typed executor
owns them, and explicit guidance otherwise. Destructive or external actions
are never executed in Wave 6 unless an existing typed executor already proves
authorization, explicit confirmation, preconditions, and postconditions.

Metrics observe process outcomes, not people. They contain no prompt text,
hidden reasoning, source content, secrets, ambient telemetry, or gate-driving
state.

### 8.2 GLG workbench

The promoted Wave 3.5 cockpit becomes live and lifecycle-complete:

- current and eventual blockers are distinct;
- feature and story journeys retain their formal steps and witnesses;
- feature-outcome attestation is scaffolded and reviewed without agent-authored
  human claims;
- recovery shows diagnosis before any permissible action;
- journey metrics show provenance and definitions and never modify readiness;
- all four areas, the three-row preview, complete expansion, and solo-author
  language stay byte-compatible unless an accessibility correction requires a
  reviewed markup-only change; and
- broader role obligations are presented without implying that one solo author
  satisfied independent-review or countersign requirements.

## 9. CSE coordinator and workbench

### 9.1 Browser-neutral proof coordinator

The CSE predecessor extracts the canonical challenge/proof orchestration from
`cmd/verdi/experiment_human.go` into a consumer-owned shared seam. It accepts
typed operation identity and exact accepted/proposal facts, returns canonical
challenge bytes, and verifies the raw signature through `experimenthuman` and
the existing governance kernel. CLI output and classifications remain
byte-compatible.

The seam cannot read credentials, sign, cache authentication, mint a principal,
or accept caller-created trust facts. Its negative matrix includes missing,
short, long, foreign, stale, unmapped, ambiguous, and historical-policy-tree
proofs with zero mutation.

### 9.2 Experiment workbench

The experiment page exposes the same 15 CLI application operations:

`inspect`, `discover-capabilities`, `validate-draft`, `review-registration`,
`status`, `explain-result`, `draft-definition`, `capture-candidate`,
`reconcile-draft`, `propose-registration`, `start`, `resume`,
`propose-ratification`, `publish-capsule`, and `release-workspaces`.

The MCP inventory remains exactly the ten agent-safe operations `inspect`,
`discover-capabilities`, `validate-draft`, `review-registration`, `status`,
`explain-result`, `draft-definition`, `capture-candidate`, `start`, and
`resume`. Human-only and destructive authority operations remain structurally
absent. The page preserves these boundaries while presenting:

- inspect, capability discovery, draft validation, registration review,
  status, and explanation;
- draft definition, candidate capture, registration reconciliation/proposal;
- start and resume using the shared strict input-bindings document;
- ratification proposal through the offline proof coordinator;
- capsule publication and workspace release as distinct operations; and
- closure evidence and posture without a second closure algorithm.

The page emphasizes registration lock, immutable inputs, execution schedule,
result witnesses, reproduction posture, ratification proof, capsule contents,
and cleanup state. It never runs an evaluator in the browser, merges a
proposal, aliases capsule publication to release, or exposes human-only
operations through MCP.

Execution receipt v2's slot-aware input bindings, receipt v1's decode-only
history, provenance preimage snapshots, ratification v3 retained proof,
accepted-use historical-policy re-verification, exact capsule binding,
publication-only behavior, release retry, and closure verification remain
unchanged application authority.

## 10. Serialized delivery and review protocol

All ten implementation units are Tier 3. Each Claude Code session starts
with `/fable-orchestration`; FABLE remains chief architect and final
producer-side judge.

For each unit:

1. FABLE reads the task brief, effective authority, predecessor report, and
   exact base before editing.
2. FABLE dispatches implementation-heavy non-frontend work to Sonnet. Every UI
   unit and UI fix is implemented by a FABLE frontend worker.
3. The worker captures an honest semantic RED, implements the minimum GREEN,
   runs proportionate gates, and returns exact evidence.
4. FABLE performs a pre-review that proves ancestry, clean status, exact
   write-set containment, focused GREEN, prohibited-artifact absence, and
   report accuracy before spending Opus review effort.
5. One independent Opus reviewer challenges the consolidated exact range.
6. FABLE adjudicates every finding. Accepted defects go to a fresh Opus fixer;
   rejected findings retain an authority-backed written ruling.
7. A fresh Opus re-reviewer, distinct from both finding reviewer and fixer,
   performs the Tier 3 closure check after at most one correction pass. No
   automatic third producer-side review round is permitted.
8. Claude stops with `READY_FOR_CODEX_REVIEW` or
   `READY_FOR_CODEX_CLOSURE_REVIEW`; it does not claim Codex approval and does
   not push, open a PR, merge, amend reviewed history, or start the next unit.
9. Codex independently reads the effective authority, immutable diff package,
   report, and exact head; runs proportional probes; adjudicates; and either
   requests one bounded correction or approves publication.
10. If Codex requests correction, Claude/FABLE routes it under the same
    ownership rules and returns a fixed range; Codex performs one closure
    review and no automatic third Codex review.
11. The owner publishes and merges before the next serialized unit starts.

## 11. Verification and Wave 6 exit gate

Every unit proves:

- TDD RED/GREEN for every new core and browser behavior;
- table-driven happy and negative unit tests;
- strict-decoder and canonical-output tests where a wire contract exists;
- hermetic Git, policy, evaluator, forge, MCP, and locking integration tests;
- built-binary parity for every new CLI operation;
- live wire parity for every new MCP operation;
- Playwright coverage for every browser-visible behavior, including keyboard,
  accessibility, 320-pixel layout, 200% zoom, visible/hidden refresh, stale
  actions, invalid and inconclusive states, and zero-effect refusals;
- fixed-set inventory tests that bite on unauthorized route/operation growth;
- no-network proof; and
- clean focused race, vet, lint, format, and diff checks.

The integrated Wave 6 head must pass, fresh and in the foreground:

```bash
make verify
go test -race ./...
```

The final review additionally proves:

- every browser operation consumes the same application result as CLI/MCP;
- all human-only agent paths remain structurally absent or refused;
- exact repository and authority posture is visible;
- no UI-only authority, hidden cache, shadow database, or second algorithm
  exists;
- all four feature workbenches pass keyboard-accessible Playwright journeys;
- the promoted Wave 3.5 shell remains lossless; and
- Wave 7 remains unstarted.

## 12. Lossless source-coverage witness

The consolidated authority carries 37/37 source groups:

| # | Source authority | Destination | Transformation or intentional omission |
|---:|---|---|---|
| 1 | Workspace `AGENTS.md` authority-first workflow | §§1, 10–11 | Preserved; spec-only authored by Codex, implementation routed by FABLE/Sonnet/Opus, Codex independently gates. |
| 2 | Orchestration Wave 6 concurrency `1` | §§1, 10 | Preserved as ten serialized units and one integration gate. |
| 3 | Orchestration Wave 6 per-unit PR rule | §§1, 10 | Preserved; no long-lived combined implementation branch. |
| 4 | Orchestration Wave 6 exit gate | §§3–5, 11 | Expanded into exact parity, posture, accessibility, and no-shadow-authority witnesses. |
| 5 | Wave 3.5 hybrid rail/queue shell | §§3.1, 8.2 | Promoted unchanged as presentation, not lifecycle state. |
| 6 | Wave 3.5 exact three preview | §§3.1, 8.2 | Preserved exactly; complete list remains inline. |
| 7 | Wave 3.5 solo-author language | §§3.1, 8.2 | Preserved; broader-role duties remain visible and unsatisfied when absent. |
| 8 | Wave 3.5 F02 distant concerns dilute focus | §3.1 | Immediate/current concerns sort before downstream concerns; timing/dependency is explicit and the exact-three preview remains. |
| 9 | Wave 3.5 F03 unproven labels lack corrective guidance | §3.1 | Every violated/unproven row carries source-derived corrective guidance; no generic favorable copy. |
| 10 | Wave 3.5 F04 stage sequencing is unexplained | §3.1 | The shell explains the fixed order and forward preconditions without creating lifecycle state. |
| 11 | Wave 3.5 F05 human review is visible only through technical detail | §§3.1, 8.2 | Human review is a primary plain label; formal obligation and role remain secondary exact evidence. |
| 12 | Wave 3.5 F06 ASD slug constraints appear after authoring | §6.2 | Grammar and rejected bytes are validated before durable mutation. |
| 13 | Wave 3.5 F07 restart-only freshness is not operationally understood | §5.1 | Startup-only posture becomes conditional visible polling plus manual refresh and stale-action refusal. |
| 14 | Wave 3.5 F08 graduation impact is not previewable | §6.2 | Review previews resulting refs/paths/relationships/downstream effects before mutation. |
| 15 | ASD AC-2 provenance/direct edit | §§4.1, 6 | Explicit unauthenticated attribution and unclassified direct-edit disclosure retained. |
| 16 | ASD AC-5 bounded context | §6.1 | Shared application operation; no browser context compiler. |
| 17 | ASD AC-6 semantic review | §6 | Deterministic view before existing Git acceptance. |
| 18 | ASD AC-7/AC-8 parity | §6 | Same six operations across CLI, MCP, and workbench. |
| 19 | CI AC-2 manifest/derivation | §7 | Rule ledger and derivation consume context/policy owners. |
| 20 | CI AC-3 conflict honesty | §7 | Mechanical/semantic/exemption/unknown distinctions retained. |
| 21 | CI AC-6 governance ledger | §§7.1–7.2 | Git-backed proposal and impact review; no UI approval state. |
| 22 | CI AC-6 workbench flow and CO-6 verification | §§3, 5, 7, 11 | One application result feeds every adapter; browser and gate coverage remains explicit. |
| 23 | GLG AC-1 journey/readiness | §8 | Current and eventual posture completed through core. |
| 24 | GLG AC-6 attestation | §8 | Typed scaffold/review and workbench projection retained. |
| 25 | GLG AC-7 recovery | §§4.1, 8 | Exact diagnosis-first and confirmation boundary retained. |
| 26 | GLG AC-8 metrics | §8 | Privacy-bounded observational metrics; never gate state. |
| 27 | CSE AC-5 application parity | §9 | Same 15 operations; human-only MCP refusal unchanged. |
| 28 | CSE AC-5/DC-7/DC-16 human proof and CO-7 verification | §§4.1, 9, 11 | Existing detached-proof authority is extracted, never weakened or reimplemented, and retains its negative proof matrix. |
| 29 | SI-144–SI-148/SI-160–SI-161 | §§4, 9 | Registration, proof, input binding, capsule, and accepted-use authority retained unchanged. |
| 30 | Current workbench/serve/e2e implementation | §§2.1, 3, 5, 11 | Existing server-rendered, dependency-free, hermetic structure reused; no frontend framework introduced. |
| 31 | Wave 6 handoff current-main/worktree boundary | §§1–2 | Remote base is reverified; the stale user-owned checkout stays untouched; isolated worktree use is mandatory. |
| 32 | Wave 6 handoff planning-before-implementation rule | §10 and implementation plan review gate | Codex authors spec-only authority; one Claude review, one Codex correction maximum, same-Claude closure, then owner approval/merge. |
| 33 | Wave 6 handoff FABLE model routing | §10 | Sonnet implementation, Opus finding/fix/re-review, FABLE adjudication, and no producer claim of Codex approval retained exactly. |
| 34 | Wave 6 handoff Codex independent review | §§10–11 | Exact diff/authority/probes and reachable-state findings required; one bounded correction and one Codex closure only. |
| 35 | Wave 6 handoff browser/test/controller gates | §§5 and 11 | Server-rendered/dependency-free/no-network, Playwright keyboard/responsive coverage, recording scan, alternate-port disclosure, full race/verify/spec-align retained. |
| 36 | Task 2 browser-human stop-gate witness and SI-176 | §§1, 4.1, 6.1.1, 10–11 | Preserves unconditional AI-free browser authoring, explicit unauthenticated provenance, failed-resolution non-bypass, agent policy enforcement, honest policy absence, V1 history, and serialized predecessor review; no runtime or frontend implementation is folded into authority. |
| 37 | Task 2 same-process writer-lock stop-gate witness and SI-177 | §§1, 6.1.2, 10–11 | Preserves I-12's one-writer-process exclusion and SI-69's crash-safe transaction while narrowly superseding SI-69's serve-held refusal only for this caller process's registry-proven exact outer lock; `WithWriterLock` transactions and the temporary legacy `spec.md` splice share per-checkout serialization until Task 2 deletes the splice, while distinct boardio annotation files retain their existing owner and ordering; PID text, cross-process bypass, outer-lock release, durable registry authority, and a system-wide all-file mutex claim remain excluded. |

No source group, semantic rule, public effect, deferral, threat-model boundary,
or closure disclosure is intentionally omitted. Wave 7 dogfood is explicitly
excluded rather than silently counted as Wave 6 evidence.
