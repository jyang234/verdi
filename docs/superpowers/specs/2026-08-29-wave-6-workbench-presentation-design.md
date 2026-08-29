# Wave 6 Workbench Presentation Authority Design

**Status:** Owner-approved planning authority; implementation remains blocked
until this consolidated exact head passes its one independent Claude read-only
review, any one Codex-authored correction pass, and the same Claude reviewer's one
closure check.

**Planning base:** `915529f792f7a672e9631f42909995b38ed12655`

**Owners:** platform-team

**Delivery shape:** eight serialized predecessor/presentation units followed by
one integrated Wave 6 gate. Each unit is one reviewed pull request.

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
2. ASD workbench;
3. Context Integrity (CI) application predecessor;
4. CI workbench;
5. Guided Lifecycle and Governance (GLG) Wave 5 predecessors;
6. GLG workbench;
7. Comparative Spike Experiments (CSE) browser-neutral human-proof
   coordinator;
8. CSE workbench; and
9. integrated Wave 6 review and gates.

The sequence is fully serialized. A later unit starts only from the
owner-merged, independently Codex-approved head of its predecessor. The split
does not create browser-owned cores: every UI unit consumes one typed
application operation whose non-browser behavior is already tested.

The current default branch contains the complete CSE Wave 5 lifecycle, but it
does not contain all Wave 5 predecessors promised for ASD, CI, and GLG. The
missing behavior is therefore delivered as named predecessor units rather than
silently implemented in HTTP handlers or JavaScript.

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

Wave 6 uses structural budgets instead of flaky wall-clock thresholds:

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
context, Git, and board projection owners. CLI, MCP, and workbench call it.
`internal/workbench/boardspecapi.go` stops applying artifact mutations itself.

The predecessor proves strict request/response contracts, deep-copy custody,
stale-revision refusal, capability enforcement, direct-edit disclosure,
bounded context, on-demand provenance, deterministic semantic review, and
CLI/MCP conformance. It adds no frontend files.

### 6.2 ASD workbench

The existing board gains:

- a revision/posture header;
- synchronized canonical object rendering;
- slug/path grammar validation before an author invests in the draft, with the
  rejected bytes and corrective grammar shown at the relevant field;
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

All eight implementation units are Tier 3. Each Claude Code session starts
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

The consolidated authority carries 35/35 source groups:

| # | Source authority | Destination | Transformation or intentional omission |
|---:|---|---|---|
| 1 | Workspace `AGENTS.md` authority-first workflow | §§1, 10–11 | Preserved; spec-only authored by Codex, implementation routed by FABLE/Sonnet/Opus, Codex independently gates. |
| 2 | Orchestration Wave 6 concurrency `1` | §§1, 10 | Preserved as eight serialized units and one integration gate. |
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
| 22 | CI CO-6 browser parity | §§3, 7 | One application result feeds every adapter. |
| 23 | GLG AC-1 journey/readiness | §8 | Current and eventual posture completed through core. |
| 24 | GLG AC-6 attestation | §8 | Typed scaffold/review and workbench projection retained. |
| 25 | GLG AC-7 recovery | §§4.1, 8 | Exact diagnosis-first and confirmation boundary retained. |
| 26 | GLG AC-8 metrics | §8 | Privacy-bounded observational metrics; never gate state. |
| 27 | CSE AC-5 application parity | §9 | Same 15 operations; human-only MCP refusal unchanged. |
| 28 | CSE AC-7/CO-7 human proof | §§4.1, 9 | Existing detached-proof authority extracted, never weakened or reimplemented. |
| 29 | SI-144–SI-148/SI-160–SI-161 | §§4, 9 | Registration, proof, input binding, capsule, and accepted-use authority retained unchanged. |
| 30 | Current workbench/serve/e2e implementation | §§2.1, 3, 5, 11 | Existing server-rendered, dependency-free, hermetic structure reused; no frontend framework introduced. |
| 31 | Wave 6 handoff current-main/worktree boundary | §§1–2 | Remote base is reverified; the stale user-owned checkout stays untouched; isolated worktree use is mandatory. |
| 32 | Wave 6 handoff planning-before-implementation rule | §10 and implementation plan review gate | Codex authors spec-only authority; one Claude review, one Codex correction maximum, same-Claude closure, then owner approval/merge. |
| 33 | Wave 6 handoff FABLE model routing | §10 | Sonnet implementation, Opus finding/fix/re-review, FABLE adjudication, and no producer claim of Codex approval retained exactly. |
| 34 | Wave 6 handoff Codex independent review | §§10–11 | Exact diff/authority/probes and reachable-state findings required; one bounded correction and one Codex closure only. |
| 35 | Wave 6 handoff browser/test/controller gates | §§5 and 11 | Server-rendered/dependency-free/no-network, Playwright keyboard/responsive coverage, recording scan, alternate-port disclosure, full race/verify/spec-align retained. |

No source group, semantic rule, public effect, deferral, threat-model boundary,
or closure disclosure is intentionally omitted. Wave 7 dogfood is explicitly
excluded rather than silently counted as Wave 6 evidence.
