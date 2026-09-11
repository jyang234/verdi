---
id: spec/public-execution-contract
kind: spec
title: "Direct public execution contract"
owners: [platform-team]
class: story
story: jira:VERDI-ATC-7
problem: { text: "Each owner call currently crosses two translation subprocesses and duplicate serialized operation codecs, increasing boundary maintenance cost.", anchor: problem }
outcome: { text: "Direct public FD3 v2 and codec consolidation remove the translation launches and duplicate operation wire while preserving validation, identity, policy, and durable recovery.", anchor: outcome }
acceptance_criteria:
  - id: ac-1
    text: "The matched Verdi and ATC pair exchanges the 22 public owner arms and the fixed local claims exception over strict FD3 v2, with zero production owner decode/encode subprocess launches and unchanged standalone bridge commands."
    evidence: [static, behavioral]
    anchor: ac-1
  - id: ac-2
    text: "Both endpoints preserve structural validation and independently enforce all 16 published request/result relations at their required invocation points; ATC validates a recorder arm before observing its event, and both receivers enforce the exclusive 32 MiB bound while reading with the existing capability-usability distinction."
    evidence: [static, behavioral]
    anchor: ac-2
  - id: ac-3
    text: "The legacy controller request digest preimage for operations 1–22, every durable format, receipt event/control-triple ordering, recovery authority outcome, and copied-store replay in both directions remain as fixed in contract sections 2.2 and 4–5."
    evidence: [static, behavioral]
    anchor: ac-3
  - id: ac-4
    text: "New ATC queries the actual pinned v2 contract on every run before flight effects; the release moves all Verdi FD3 consumers together and supports only the specified quiescent matched-pair upgrade and rollback with exact artifact identities."
    evidence: [static, behavioral]
    anchor: ac-4
  - id: ac-5
    text: "The completed change removes Verdi’s separate private serialized operation representation, preserves required domain conversions and validators, and returns a deletion inventory with each removed check mapped to its new function, invocation point, and rejecting mutation."
    evidence: [static, behavioral]
    anchor: ac-5
links:
  - { type: implements, ref: "spec/verdi-atc-prerequisites#ac-4" }
  - { type: implements, ref: "spec/verdi-atc-prerequisites#ac-5" }
  - { type: implements, ref: "spec/verdi-atc-prerequisites#ac-6" }
decisions:
  - { id: dc-1, text: "Contract sections 1–6 below reproduce the owner-ratified protocol without semantic alteration; the source witness fixes the bounded supersessions and retained authority. No other incumbent clause is replaced.", anchor: dc-1 }
  - { id: dc-2, text: "This story refines the existing execution, receipt, and provider-event prerequisites; it creates no additional feature outcome and does not edit frozen parent or child specification files.", anchor: dc-2 }
  - { id: dc-3, text: "Owner ratification on 2026-09-10 authorizes this amendment and story preparation. Runtime implementation waits for this story’s Git-derived acceptance on the configured default branch; push, PR, and merge require separate owner authorization.", anchor: dc-3 }
  - { id: dc-4, text: "The historical source witness and reviewed design are preserved byte-for-byte with pinned hashes in the promotion record. Their historical proposed status is superseded only by the recorded ratification; ATC invention-ledger section references resolve to section 10.", anchor: dc-4 }
constraints:
  - { id: co-1, text: "Both implementation flights and all contract section 6 gates must pass before release or completion; evidence producers declared here are future obligations, not assertions of passing tests.", anchor: co-1 }
  - { id: co-2, text: "No new provider, principal, claims, observable-write producer, review or landing semantics, persistence transaction, or op-19 production-authority repair is authorized; F12 still ends at a durable candidate handoff.", anchor: co-2 }
  - { id: co-3, text: "The independently implemented public boundary admits no private cross-repository Go imports, no contextowner import of sealedexec, and no durable schema or historical digest rewrite.", anchor: co-3 }
---
# Direct public execution contract

## Problem

Each owner call currently crosses two translation subprocesses and duplicate
serialized operation codecs. The duplication increases the number of places
that must agree on validation and identity.

## Outcome

Direct public FD3 v2 removes the launches; the subsequent codec consolidation
removes the duplicate serialized operation representation. Both are required.

## Acceptance criteria

### AC-1

The matched Verdi and ATC pair exchanges the 22 public owner arms and the fixed local claims exception over strict FD3 v2, with zero production owner decode/encode subprocess launches and unchanged standalone bridge commands.

Scope: FD3 codecs, all openSealedController consumers, ATC routing, standalone bridge compatibility, literal fixtures for all 23 operations, and built-binary launch traces.

### AC-2

Both endpoints preserve structural validation and independently enforce all 16 published request/result relations at their required invocation points; ATC validates a recorder arm before observing its event, and both receivers enforce the exclusive 32 MiB bound while reading with the existing capability-usability distinction.

Scope: All 22 owner arms, six explicitly classified no-extra-relation arms, claims registration, recorder observation, incremental reply reading, cancellation, EOF, partial frames, and the check-to-function/invocation/mutation witness.

### AC-3

The legacy controller request digest preimage for operations 1–22, every durable format, receipt event/control-triple ordering, recovery authority outcome, and copied-store replay in both directions remain as fixed in contract sections 2.2 and 4–5.

Scope: Golden preimages, dispatch and receipt identities, two receipt writes, all five deterministic process-crash cuts under baseline and candidate, actual re-entry classifiers/exits/rows/launches, and two-way copied-store decode/replay.

### AC-4

New ATC queries the actual pinned v2 contract on every run before flight effects; the release moves all Verdi FD3 consumers together and supports only the specified quiescent matched-pair upgrade and rollback with exact artifact identities.

Scope: Actual-run and dry-run version matrix, all Verdi controller hosts, pre-effect refusal evidence, baseline stored-plan limitation, copied stores, exact source commits/binary hashes/contract bytes, and offline rollout instructions.

### AC-5

The completed change removes Verdi’s separate private serialized operation representation, preserves required domain conversions and validators, and returns a deletion inventory with each removed check mapped to its new function, invocation point, and rejecting mutation.

Scope: Verdi contextowner and sealedexec seams, ATC independent public validator, standalone compatibility verbs, retained domain/artifact validators, and the completed two-flight release evidence.

## Decisions

### DC-1

Contract sections 1–6 below reproduce the owner-ratified protocol without semantic alteration; the source witness fixes the bounded supersessions and retained authority. No other incumbent clause is replaced.

### DC-2

This story refines the existing execution, receipt, and provider-event prerequisites; it creates no additional feature outcome and does not edit frozen parent or child specification files.

### DC-3

Owner ratification on 2026-09-10 authorizes this amendment and story preparation. Runtime implementation waits for this story’s Git-derived acceptance on the configured default branch; push, PR, and merge require separate owner authorization.

### DC-4

The historical source witness and reviewed design are preserved byte-for-byte with pinned hashes in the promotion record. Their historical proposed status is superseded only by the recorded ratification; ATC invention-ledger section references resolve to section 10.

## Constraints

### CO-1

Both implementation flights and all contract section 6 gates must pass before release or completion; evidence producers declared here are future obligations, not assertions of passing tests.

### CO-2

No new provider, principal, claims, observable-write producer, review or landing semantics, persistence transaction, or op-19 production-authority repair is authorized; F12 still ends at a durable candidate handoff.

### CO-3

The independently implemented public boundary admits no private cross-repository Go imports, no contextowner import of sealedexec, and no durable schema or historical digest rewrite.

## Provenance and contract

The [promotion record](../../../../docs/superpowers/specs/2026-09-10-public-execution-contract/promotion.md)
records the exact reviewed head, ratification, source hashes, section mapping,
and remaining acceptance gate. Its byte-exact source witness is incorporated
for amendment scope and retained authority; it does not replace those sources.
Source-witness SHA-256: `7f99bcccfdb92c248f8cf4c04c982266d1d7757bc304e3af5f65725f94a8348e`.
The following contract is the reviewed design’s sections 1–6, reproduced
verbatim. Internal section numbers refer to that contract.

## 1. Decision

Use the already published owner request/result arms directly in a version-2
FD3 controller envelope. ATC decodes these arms, invokes the existing owner,
and returns the public result. It no longer invokes `context owner decode`
and `context owner encode` for each of operations 1–22. Operation 23 remains
the locally answered claims-registration operation.

Then consolidate Verdi's serialized operation types and validation around its
public `internal/contextowner` seam. Keep execution-domain models and owner
policy where they belong. This removes a duplicate wire representation; it
does not make the transport responsible for policy or durable state.

| Option | Consequence | Decision |
|---|---|---|
| Direct public FD3, followed by codec consolidation | Removes two process launches per bridged call and one independently maintained wire representation in Verdi | Recommended |
| Generate private/public translations while retaining the bridge | Reduces some editing duplication, but retains translation processes and cannot settle ownership or recovery | Defer; generation is unnecessary for this change |
| Persistent translation helper | Adds service lifetime and shutdown/recovery state while retaining two wires | Reject |

Success means zero translation subprocess launches in production execution,
one public operation-wire implementation inside each repository, and preserved
strict validation and durable artifacts. A transport-only patch is an
intermediate milestone, not completion of this design.

No new operations, provider behavior, claims authority, observable-write
producer, runner identity, review/landing behavior, or persistence transaction
are included. F12 still ends at a durable candidate handoff. Op 19's unavailable
production authority remains unavailable; removing a validator to make it
succeed is forbidden.

## 2. Wire

### 2.1 Version and envelopes

`verdi context contract` keeps its exact no-argument, effect-free CLI grammar.
The new build publishes `verdi.context-controller-contract/v2`, with exactly
the existing five members: `schema`, `controller_call_schema`,
`controller_result_schema`, `controller_error_schema`, and `operations`.
Values name `verdi.context-controller-call/v2`,
`verdi.context-controller-result/v2`, and
`verdi.context-controller-error/v1`, respectively. The ordered 23-operation
registry is unchanged. This specification fixes the complete arm versions and
the SI-194 non-proven rule below. The contract document advertises the version
and registry; it does not carry arm definitions or prove that a build implements
them correctly. The matched-pair conformance tests in §6 establish that evidence.
Further semantic changes require authority and a version advance, even if the
operation names remain unchanged.

Call and result envelopes retain exactly `schema`, `call_sequence`,
`operation`, and `payload`. Sequence starts at 1, advances adjacently, permits
one outstanding call, and is cross-matched on every response. A result payload
contains exactly one of `result` or `error`. The operational error grammar,
codes, witness rules, and empty-stdout-on-command-failure posture are unchanged.
A typed negative or unproven verification remains a successful protocol result
for the caller to classify; it must not become a transport error.

For operations 1–22, call `payload` is the corresponding public request arm;
successful result `payload.result` is the corresponding public result arm.
Request schemas are `verdi.context-owner/<operation>-request/v1`, except
`install-expansion-request/v2`; all result schemas are
`verdi.context-owner/<operation>-result/v1`. Their bodies retain the bridge
correction §3.3 publication, including its two explicit exceptions. A
non-proven context resolution omits `data`; a proven one requires it, also
when embedded in an epoch check.

Operation 23 keeps its current `verdi.context-controller/resolve-claim-mcp-request/v1`
and `.../resolve-claim-mcp-result/v1` payload wrappers and nested public
`verdi.claim-mcp-query/v1` / `verdi.claim-mcp-registration/v1` documents.
Only its outer envelopes advance to v2. This explicitly published exception
does not extend the owner union to a 23rd arm.

Both endpoints reject unknown/duplicate/missing fields, trailing data,
noncanonical encodings, invalid values, wrong schemas, unselected union arms,
and wrong operation/sequence bindings. LF-delimited canonical JSON, sorted
keys, no HTML escaping, and the exclusive 32 MiB frame bound remain unchanged.
Both receivers must enforce the bound while reading, before allocating an
unbounded line. This adds incremental read-bound enforcement at Verdi's current
`ControllerClient.invoke` reply read (`ReadBytes`), while preserving the
accepted frame-size limit; it is not a claim that the baseline already bounds
that allocation. Truncation and real transport failures remain operational failures;
normal child EOF must retain the baseline FD3 shutdown fix.
Preserve the capability-usability distinction in workspace PLAN I-89: transport,
structural decode, sequence, operation, or cancellation failure poisons the
channel; a correctly framed operational refusal or operation-specific relation
mismatch leaves it usable for the existing quarantine attempt. Centralizing
validation must not inadvertently poison that latter path.

The public call/reply wrappers do **not** cross FD3. They are local validation
objects and remain the standalone bridge CLI's public interface. This avoids
repeating the request in a transport reply. Replacing the private arm's schema
with the shorter public schema adds no frame overhead. Existing standalone
bridge wrapper bounds continue to apply to those compatibility commands.

### 2.2 Request identity

Preserve the existing `controller_request_digest` meaning; do not hash the
public arm or the sequenced envelope under that name.

For operations 1–22 only, given a strictly valid public request arm `P` and
operation `op`, derive `L(P)`
by replacing **only** its top-level schema with
`verdi.context-controller/<op>-request/v1`, except installation uses v2.
Canonically encode the resulting object as a standalone document with one LF.
The controller request digest remains `SHA256(L(P))`, with the usual
`sha256:` lowercase-hex spelling. Nested bytes, operands, and array order are
unchanged. This freezes a public compatibility preimage rule; it does not
authorize copying private Go structs or discovering rules from later commits.

ATC constructs the same `verdi.context-owner-call/v1` it previously received
from `decode`, using the operation, `P`, and that digest. Its owner result is
structurally and relationally validated in a local
`verdi.context-owner-reply/v1` containing that exact call, before ATC frames a
success. The relational check is mandatory even for a structurally valid result.
Verdi constructs the same local call from the request it actually sent and
validates the returned result against it. Neither side accepts a caller-supplied
digest without recomputation. The wrappers need not be persisted or logged.

Thus request identity is byte-stable across the transport change. It remains
distinct from the sealed execution-request digest, ATC dispatch-envelope
digest, event digests, receipt digest, and acknowledgment digest. No existing
durable digest is recomputed using the v2 envelope.

## 3. Validation and ownership

| Concern | Owner and required behavior |
|---|---|
| JSON/schema/closed-union/value validation | Strict public codec at both repository boundaries; no schema-only success path |
| Pure request/result relations | One reusable Verdi validator for its runtime client and retained bridge CLI; an independent ATC public-contract validator enforces the same published relations before success framing |
| Canonical embedded artifacts and their digests | Existing owning artifact/domain validators; do not duplicate their algorithms in a new transport package |
| Plan, profile, grants, epoch, workspace, claims, recorder and persisted-state truth | Existing ATC owners over reconciled durable facts and fresh observations |
| Context compilation/resolution and execution/receipt policy | Existing Verdi services; pinned context resolver remains an authorized observation, not an approval |
| Framing, sequence, routing, refusal, cancellation and joining | Existing controller and runner responsibilities |

The public codec is currently not a substitute for every private validator.
Implementation must inventory the checks in Verdi's `controller_codec.go`,
`owner_bridge.go:crossMatchOwnerResult`, `controller_client.go`, and
`contextowner/codec.go` before deleting code. For each removed check, record its
new owning function and a rejecting mutation. Required acceptance is the
intersection of the current relevant validators, not whichever codec is more
permissive. Existing discrepancies block that arm until explicitly adjudicated.

ATC must enforce all 16 published request/result relation branches inventoried
in the source witness before framing success, in addition to its existing
structural and owner checks. A contradictory owner result becomes ATC's
operational refusal, as a rejected `context owner encode` does today; it must
not cross FD3 as success merely because Verdi will reject it later. These are
public-contract relations, not copied private implementation rules. The six
arms with no extra relation remain explicitly classified. The deletion witness
must name both the new function and its invocation point in each repository;
moving a producer-side check solely to the consumer does not preserve coverage.

For example, checkpoint responses must still bind every active acknowledgment
to the request's flight/lane/epoch. Empty initial checkpoints remain valid;
completed revision-array roots are not substituted for live prior-event
digests. Receipt acknowledgment checks still bind the event identity, source
position, event digest, receipt digest, and the required global ordering after
the terminal execution result. The source witness names all operation relations.

ATC validates the public request before any owner action. For recorder-append,
full arm validation now precedes observation: a malformed arm containing an
otherwise valid event produces no observed row. The baseline controller could
observe that event before the bridge rejected the arm; this admission-order
change is explicit. For a valid arm, validate the event, perform the production
observer commit, and only then let
the recorder owner acknowledge the exact committed observation. The changed
nesting must not accidentally bypass the observer or make generic append the
receipt writer. Owners are still unavailable until reconciliation completes.

Inside Verdi, remove the duplicate private operation-wire structs and codecs
once the public seam owns their validations. Retain execution-domain types and
small domain-to-public conversions where required. Compatibility commands
become schema-only republishing around that same validator, not a second
semantic codec. If a dependency cycle prevents reuse, move only the shared
pure representation/validator into the established shared internal seam;
do not import `sealedexec` from `contextowner` or create a second policy owner.
No cross-repository Go imports are permitted.

## 4. Durability and recovery

No dispatch, event, segment, expansion, session, receipt, handback, quarantine,
abort, retry, or candidate schema changes. In particular, ATC's receipt control
fact stores `{ack,event,receipt}`, not the owner wrapper or FD3 envelope.
Transport sequence is per process and is not a durable retry token.

| Receipt cut | Durable evidence | Required treatment |
|---|---|---|
| Before receipt event commit | Closed execution-result only | No receipt acknowledgment or candidate may be inferred |
| Event committed, control fact absent | Receipt event exists after execution-result | Preserve it; do not append a duplicate or claim a durable receipt triple |
| Control fact committed, reply absent/partial | Exact event/receipt/ack triple exists | An identical operation-level repeat may recover the same ack; contradictory bytes refuse |
| Reply delivered, candidate handoff absent | Receipt triple, no candidate handoff | Receipt alone does not authorize candidate replay |
| Candidate handoff committed | Handoff plus required recorder/control/Git facts | Existing re-entry revalidates Git and returns exact handoff without another launch |

These are two durable receipt writes. They are not a cross-store transaction.
The control fact still atomically stores the exact receipt/event/ack triple
before acknowledgment. This preserves I-82's receipt/event binding; it does
not make the separately ingested recorder row part of that transaction.
For the event-only cut, test the actual current operation-level behavior against
reconciled storage; do not assume that a new process calls append-receipt again.
ATC supervisor re-entry compiles/checks the recorded lease and routes through
its existing recovery classifier. F12 has replacement retries, not a newly
enabled sealed resume. Re-entry can also append `vatc.flight.resumed` and
relaunch the same epoch through `fly` using the existing start path. This ATC
re-entry relaunch is distinct from Verdi's sealed `action: resume`; a non-pristine
recorder may cause the subsequent start to refuse. Record whichever actual
classifier and launch path occurs, including suspension, escalation, or an
existing replacement path; this amendment does not promise transparent
continuation of the old execution.

Before shipping v2, kill real built processes at all listed cuts and record
the actual command exit, classifier branch, stored rows, and additional
launches under both the pinned baseline and candidate. Candidate behavior must
preserve the baseline's authority outcome and evidence, including safe refusals.
Any baseline defect found is a separately adjudicated fix, not implicit new
recovery authority. Deterministic test seams must force the named cut; timing
delays alone are not evidence. Neither operation-level idempotency nor a green
unit test establishes end-to-end exactly-once execution.

## 5. Compatibility and rollout

The first release is a matched ATC/Verdi pair. The artifact handoff records both
source commits, executable SHA-256 values, exact contract bytes, and tests.
Hashes are populated from built artifacts, not invented in this design.

| ATC | Verdi contract | Result |
|---|---|---|
| Baseline v1 | Baseline v1 | Existing bridge |
| New v2 | New v2 | Direct public FD3 |
| Baseline v1 | New v2 | Fresh dry-run contract refusal; see the stored-baseline limitation below |
| New v2 | Baseline v1 | Strict contract refusal |

New ATC performs the exact pinned contract query in the `vatc run` assembly
before runway creation, BuildStart, recorder/ledger mutation, claim-listener
startup, or provider launch. A previous plan dry run is insufficient. Pin
verification remains mandatory on every launched Verdi command. An advertised
contract mismatch is exit 2, empty stdout, a closed explanatory diagnostic, and
zero flight effects. A defective build that advertises v2 but implements a
different arm is not detected by that declaration alone; matched-pair release
tests must detect the discrepancy before deployment.
Do not add v1 fallback, environment selectors, a second executable pin, or a
persisted protocol selector. Baseline ATC's existing negotiation behavior is
tested and disclosed; it is not retroactively strengthened by this design.
In particular, baseline ATC negotiates in dry run, not on every actual run.
An old approved baseline plus a changed pin can reach runway/dispatch effects
before its incompatible FD3 call fails. The offline paired rollout forbids
that combination; this proposal does not claim effect-free refusal for the
unmodified old binary's actual-run path.

All Verdi commands using `openSealedController` move together: execution,
context MCP, and receipt verification. CLI argument grammars and their public
request/result documents stay unchanged; their FD3 host must speak v2. An old
host bypassing negotiation receives/refuses an incompatible first frame before
any owner effect. Other FD3 host implementations are incompatible until updated.
The two standalone owner bridge verbs remain supported and effect-free;
they are compatibility/conformance tools, never an automatic runtime fallback.

Upgrade and rollback are offline pair replacement at a quiescent installation.
Stop the old processes and finish or explicitly resolve incomplete flights
using their original pair first. Do not use a pin swap to restart an in-flight
lease. Existing records do not attest a transport version; schema compatibility
does not prove arbitrary mixed-version in-flight recovery. This is a deployment
constraint, not a claimed new durable enforcement field.

Do not rewrite historical stores. Completed records must decode byte-for-byte
under the new pair; replay is subject to the existing fresh-Git checks. New
flights use v2. For rollback, stop the new pair, resolve its incomplete flights
with that pair, and restore the old ATC plus its exact Verdi pin. Read/replay
compatibility in both directions is a release gate, tested on copied fixture
stores; if it fails, rollback is unproven and the candidate cannot ship under
this no-migration design.

## 6. Implementation and verification

Two focused non-frontend implementation flights follow ratification and the
required Verdi story-acceptance gate. Claude Code/Sonnet implements; Opus fixes
and reviews; Codex adjudicates and verifies, under existing repository rules.

1. **Direct transport:** publish/negotiate v2, route public arms, retain all
   validations, preserve identity preimages, remove ATC's production bridge
   dependency and subprocess launches. Pure existing mapping functions may
   temporarily adapt Verdi's runtime; disclose them as remaining duplication.
2. **Codec consolidation:** eliminate the separate private serialized operation
   representation, centralize pure relation checks, and keep only necessary
   domain conversion and standalone CLI compatibility. Return a deleted-code
   inventory and validation-coverage witness; do not delete durable-artifact
   validators because they happened to live beside wire code.

The consolidated release must satisfy:

- Literal request/result fixtures for 22/22 owner operations plus the claims
  exception, in both repositories, with matching canonical bytes and legacy
  identity preimages. Fixtures live under `testdata/`; expected bytes are not
  generated by the implementation being tested.
- Per-arm rejection mutations, including duplicate/unknown fields, null versus
  absence, wrong schema/version/operation, changed request/result identities,
  non-proven context, installation v1, segment digests, and receipt roots/acks.
- For every relation-bearing arm, ATC mutations must prove a structurally valid
  contradictory owner answer becomes an operational error before success
  framing. Verdi mutations must independently prove the consuming check remains;
  the six no-extra-relation arms must also stay explicitly covered.
- A malformed recorder-append arm containing a valid event creates no observed
  row. A valid arm still commits through the observer before acknowledgment.
  Oversized and unterminated reply streams stop at the incremental read bound
  and poison the capability, without unbounded allocation or partial success.
- All controller consumers tested; strict old/new mixed-pair refusal and
  unchanged framing bounds, cancellation, EOF, partial-frame, and error paths.
- A real built-binary launch trace showing **zero** `context owner decode` /
  `context owner encode` calls during execution. Context resolution may still
  invoke its pinned read-only command. This is a structural result; latency
  improvement is measured separately and no numerical speedup is promised.
- The existing built-vatc declared-claim success and replay, runtime-grant
  suspension, and bridge compatibility tests remain; adapt the direct transport
  tests without removing the standalone bridge contract tests.
- The receipt crash-cut and copied-store rollback matrix in §§4–5; report
  refusals, skips, and unproven cases explicitly.
- Both repositories' `make verify` and `go test -race ./...`, plus ATC
  `make boundary-check` with the new built pin. If the full race suite needs
  `-timeout=30m`, record the exact command and reason, as in the baseline.
  Historical fixed-pin skips remain separate from the no-skip boundary target.

No release/completion claim until both steps and their full gates pass. Package
extraction for receipt/checkpoint or expansion owners is subsequent work; F13/
F14 and the repeated two-story journey remain required after that.

## Acceptance status

The design is ratified. This story remains proposed until its exact specification
completes the configured default-branch acceptance flow; no runtime implementation
is authorized before that gate.
