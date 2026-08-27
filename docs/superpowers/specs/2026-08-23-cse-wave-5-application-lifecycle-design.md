# CSE Wave 5 Application and Lifecycle Authority Design

**Status:** Owner-approved design; repository authority becomes effective when
the reviewed commit carrying this document and SI-139 through SI-149 reaches
the configured default branch.

**Planning base:** `8cbb97aa738e34e4703f6d8d57892357b8cf2bd8`

**Owners:** platform-team

**Delivery units:** CSE Wave 5A policy/application foundation, Wave 5B
registration and adapters, and Wave 5C ratification/release integration

**Frontend boundary:** none. The CSE workbench remains the FABLE-owned Wave 6
consumer of the application core defined here.

## Contents

1. Decision and scope
2. Architecture and ownership
3. Closed operation set
4. Experiment definition v2 and reproduction
5. Typed CSE policy
6. Mutation provenance
7. Proposed versus accepted authority
8. CLI and MCP adapters
9. Ratification, capsule, release, and closure
10. Failure and exit semantics
11. Verification and adversarial coverage
12. Delivery gates
13. Lossless source-coverage witness

## 1. Decision and scope

Wave 5 delivers the non-UI CSE lifecycle through one typed application core.
The core composes the already-landed experiment schemas, decision engine,
strict evaluator adapter, isolated execution service, Context Integrity
effective-policy system, governance-principal kernel, Git-derived acceptance
state, and existing spike closure flow. CLI and MCP are adapters over that
core; neither owns policy, authority, state, or experiment semantics.

The work is split into three serialized units:

1. **Wave 5A — policy and application foundation:** experiment-definition v2,
   registered reproduction rules, the generic layered-payload and accepted-tree
   predecessor seams, typed CSE policy, mutation provenance, and
   read/validate/review application operations.
2. **Wave 5B — registration and adapters:** draft/candidate mutation,
   registration lock, run start/resume/status/explanation, and parity across
   the CLI and agent-safe MCP surface.
3. **Wave 5C — ratification and release:** authenticated ratification,
   reproduction-status integration, selected capsule construction, workspace
   release, the remaining human CLI operations, and existing spike-close
   integration.

The split does not create three cores. Every unit extends one
`internal/experimentapp` consumer. Package boundaries keep schemas, policy,
execution, and application orchestration separate.

This design does not add:

- a workflow engine, mutable lifecycle status, cursor, current-run pointer, or
  preferred-run pointer;
- a second policy store, hierarchy, resolver, or execution-grant vocabulary;
- automatic design selection, ratification, open-question resolution, or
  prototype promotion;
- a new spec kind, link type, evidence kind, or closure path;
- a feature-local identity provider or self-declared human actor;
- a second agent protocol merely to avoid the existing MCP inventory; or
- any browser markup, CSS, JavaScript, HTTP action, or workbench behavior.

## 2. Architecture and ownership

```text
CLI adapter ---------+
                     |
MCP agent adapter ---+--> experimentapp
                           |-- experiment schemas and state derivation
                           |-- experimentpolicy effective resolver
                           |-- governance-principal authority
                           |-- experimentrun execution service
                           |-- Git accepted/proposed identity
                           `-- existing spike closure consumer
```

`internal/experiment` remains the sole schema, state-table, and
cross-record-validation owner. Wave 5A extracts its reads behind a byte-source
seam so the same algorithm can consume either the working filesystem or one
exact Git tree; `internal/experimentapp` may resolve that tree and supply its
bytes, but may not reproduce the state table.

`policyartifact`, `policyauthority`, and `contextcompile` continue to own typed
payload registration, effective-policy resolution, and scope applicability.
Wave 5A adds one generic registered layered-payload mode and one sealed
applicable-payload selection to those owners. `internal/experimentpolicy`
owns only the CSE grammar and the commutative monotone reduction of that
already-selected sealed ledger; it never chooses applicability or precedence.
`internal/experimentrun` remains the sole runner and receipt/result publisher.
`internal/experimentapp` owns lifecycle preconditions, actor custody,
provenance, immutable experiment-directory writes, and composition of those
predecessors.

Adapters may:

- strict-decode one typed request;
- obtain adapter-controlled actor evidence;
- invoke one application operation; and
- render the returned typed result and exit classification.

Adapters may not:

- reinterpret policy or a disclosure;
- construct or modify sealed authority operands;
- synthesize a human principal from request text;
- change recommendation, reproduction, retention, or closure meaning;
- write experiment authority directly; or
- expose an operation the adapter class is forbidden to invoke.

## 3. Closed operation set

The application core exposes a closed operation vocabulary. Operations share
one identity envelope: checkout root, spike ref, experiment id, expected
accepted HEAD when authority-bearing, and adapter-controlled actor.

### 3.1 Read and planning operations

- `inspect` derives the experiment aggregate state, every visible run, fixed
  disclosures, effective-policy posture, reproduction posture, and permitted
  next actions. It never selects a run.
- `discover-capabilities` runs the existing strict `describe` adapter, returns
  canonical capabilities bytes and digest, and writes nothing.
- `validate-draft` validates definition, candidate patches, protected inputs,
  evaluator identity, policy membership, and path ownership without locking.
- `review-registration` derives a deterministic human review packet containing
  question, class, candidates and patch digests, contract, workload, fixtures,
  evaluator and capabilities, metric, guards, schedule, environment policy,
  reproduction rule, and retention effect. The packet is a projection, never
  lifecycle authority.
- `explain-result` returns a deterministic explanation of one exact result,
  its scope, witnesses, run identity, recommendation, and reproduction status.

### 3.2 Agent-permitted mutations

An adapter-controlled agent may create or amend an unlocked draft, capture or
replace candidate patches while exploratory, and start or resume an already
accepted locked run. It may never lock, ratify, close, weaken policy, change a
locked definition, or release evidence.

Every agent operation carries an explicit unauthenticated attribution with
harness identity and optional session identity. Attribution is not authority.

### 3.3 Human-only mutations

`reconcile-draft` is a local-human bookkeeping mutation: it records an
otherwise-unattributable direct draft change with explicit unauthenticated
attribution, changes no draft content, and grants no authority. It is omitted
from MCP and must complete before a read-only registration review can pass.

Registration lock and ratification require a sealed authenticated principal
resolution produced by the adapter's existing trust boundary. A payload actor
string never satisfies the operation.

The registration lock binds the canonical definition digest after the human
reviews the registration packet. The resulting bytes are only a proposal
until merge-signaled accepted HEAD proves the human transport event.

Ratification binds one exact valid result digest and one of the existing
closed dispositions. Its actor is copied from the authenticated principal
resolution, never from request data. The resulting bytes are likewise a
proposal until accepted HEAD proves the transport event.

### 3.4 Post-ratification operations

Only accepted ratification bytes may trigger release processing. A selecting
disposition (`select-recommended` or `select-other`) first creates one
immutable selected capsule, verifies it, and only then releases disposable
workspaces. A non-selecting disposition (`reject-all`, `misframed`, or
`request-new-revision`) creates no selected capsule and may release all bulky
workspaces after the ratification is durable.

Cleanup failure is operational. It never changes the ratification, removes
the minimal durable record, or rewrites decision state. All workspaces remain
eligible for a later idempotent retry. No selecting disposition can enter
spike closure without the valid capsule; non-selecting dispositions cannot
close the linked question and instead leave the spike open for reframing or a
new experiment revision.

## 4. Experiment definition v2 and reproduction

`verdi.experiment/v2` is the Wave 5 emitted and registrable definition schema.
V1 remains strict decode-only compatibility and can still derive historical
state, but it cannot be newly locked through Wave 5.

V2 carries all v1 fields unchanged and adds:

```yaml
class: request-path-performance
reproduction:
  minimum_valid_runs: 2
```

`class` is a required canonical ID. It selects no behavior by itself; it is
only an exact policy matching operand for allowed classes and mandatory
guards. `reproduction` is optional. When present,
`minimum_valid_runs` is an integer of at least two. No environment diversity,
statistical score, confidence level, retry suppression, or favorable run
filter is implied.

A result set is reproduced exactly when:

1. the locked v2 definition declares a reproduction rule;
2. at least `minimum_valid_runs` complete valid result-bearing runs exist;
3. every visible result-bearing run is valid and binds that definition;
4. every result-bearing run proves the same winner; and
5. no malformed or ambiguous run state exists.

Incomplete runs remain visible and do not count. They neither improve nor
erase complete evidence. A recommended or ratified result without the rule,
or below its minimum, remains valid but is never described as reproduced.
Ratifying a candidate other than the proven winner does not make that human
selection reproduced.

## 5. Typed CSE policy

`internal/experimentpolicy` registers `experiment_execution` as a **layered**
payload kind with `policyartifact`. Singleton remains the default for every
existing payload kind. Context Integrity continues to own storage,
inheritance, applicability, effective resolution, identity, and digest.

The generic predecessor amendment is deliberately narrow:

- payload-kind registration records `singleton` or `layered` cardinality;
- `policyauthority` continues rejecting duplicate singleton payload owners,
  but permits a layered kind in several policies and seals every typed layer
  into the one effective-policy identity;
- `contextcompile` applies its existing three-valued scope algorithm to those
  layers for the exact operation target and returns one sealed, sorted,
  lossless selection; unknown applicability stays blocking; and
- feature code receives that selection and cannot add, remove, reorder, or
  independently reselect policy entries.

This is a shared Context Integrity transport, not a CSE hierarchy or second
resolver. The CSE reducer has no organization/project precedence algorithm;
its intersection/minimum/union/denial operations are commutative across every
already-applicable selected layer.

The v1 payload contains:

```yaml
experiment_execution:
  experiment_paths: [".verdi/specs/active/**/experiments/**"]
  candidate_paths: ["spikes/**"]
  classes: [request-path-performance]
  evaluators:
    - argv0: ./tools/cache-evaluator
      protocols: [verdi.experiment-evaluator/v1]
  environments:
    - id: default-deny
      grants: <exact spec/execution-workspace grant-set shape>
      declared_environment:
        GOMAXPROCS: "1"
  limits:
    observation_bytes: 524288
    retained_artifact_bytes: 16777216
  trusted_measurement_sources: [harness-measured, evaluator-measured]
  mandatory_guards:
    - class: request-path-performance
      guards: [contract-correct]
```

Every list is present, sorted, and duplicate-free. Every environment name and
exact value is present in a sorted map; values are policy bytes, never copied
from the ambient process. Both byte ceilings are positive. The grant field
strict-decodes through the one shared execution-workspace grammar and remains
the only timeout/CPU/memory/network/filesystem/process authority. The two CSE
byte ceilings govern CSE-owned evidence boundaries only; they do not restate a
shared grant.

The CSE reducer receives every applicable entry already selected and sealed by
Context Integrity, then combines the payloads monotonically:

- allowlists intersect;
- maximum byte ceilings take the minimum;
- named environment ids intersect; a surviving id must carry byte-identical
  canonical grant bytes and an identical declared-environment map in every
  layer that names it, otherwise refinement is malformed and refuses;
- required guards union;
- any denial dominates;
- a lower layer cannot restore an excluded path, class, evaluator, protocol,
  measurement source, or named environment.

Wave 5 does not invent grant-set intersection. A project can narrow execution
by removing a named environment, while changing the grants or values of a
surviving id is an explicit conflict. Finer grant refinement remains deferred
until the shared execution-grant owner defines it.

`observation_bytes` limits both the raw evaluator run-response bytes and the
canonical measured-observation bytes published for one attempt. Wave 5A adds
one explicit limit operand to `experimentevaluator.ObserveInput`, extends
`experimentrun.ExecutionAuthorization` with the effective observation cap,
and has the run service enforce the smaller of policy and the existing hard
transport ceiling before append. The fixed hard ceiling remains defense in
depth; a larger policy value never weakens it, and a smaller organization or
project value is mechanically enforced.

`retained_artifact_bytes` is the maximum raw-byte length of each artifact
eligible for the selected capsule inventory. It is carried by the sealed CSE
policy decision and enforced by the Wave 5C capsule builder before immutable
publication. It does not apply to the capsule manifest itself and does not
delete an oversized artifact; the operation refuses with the exact artifact
identity and observed size. Applying it earlier to disposable candidate
workspaces would conflate retention policy with execution inputs, so Wave 5
does not do that.

The selected definition environment-policy id must resolve to one exact
effective environment entry. Its canonical grant bytes, exact declared
environment map, and effective-policy digest become the existing
`experimentrun.ExecutionAuthorization`. Missing applicable CSE policy,
duplicate environment ownership, empty intersections, malformed refinement,
or an input outside the final allowance refuses fail-closed.

Policy is resolved once per application operation. The same sealed result and
digest drive validation, execution authorization, and mutation provenance;
an adapter cannot ask two resolvers and choose a favorable answer.

## 6. Mutation provenance

Each experiment directory may carry `mutation-provenance.jsonl`, a strict,
canonical, append-only sequence. It is not lifecycle state and never grants
authority. Each record binds:

```text
schema            verdi.experiment-mutation-provenance/v1
experiment        spike ref + experiment id
operation         closed CSE operation id
previous_digest   exact prior artifact-set digest
result_digest     exact resulting artifact-set digest
policy_digest     exact sealed effective-policy digest
  policy_decision   allowed, with closed reason ids
attribution       canonical principal or explicit unauthenticated marker
harness/session    present only for delegated-agent attribution
paths             sorted canonical affected paths
digest            record self-digest
```

The closed successful mutation ids are `draft-definition`,
`capture-candidate`, `reconcile-direct-draft`, `propose-registration`, and
`propose-ratification`.
Machine evidence production already carries its own receipt, observation,
result, capsule, and release identities and is not duplicated into a second
provenance algorithm. Refused attempts are returned to the caller and are not
written into committed authority merely to create an audit side effect.

Direct Markdown/Git edits cannot truthfully identify every editing event.
`review-registration` remains strictly read-only: it compares the accepted
base with proposed bytes and reports a blocking `direct-draft-unreconciled`
posture when changes lack an exact typed-operation chain. Wave 5B adds the
explicitly mutating, local-human CLI operation `reconcile-draft`. That
operation appends exactly one `reconcile-direct-draft` record with explicit
unauthenticated attribution and the exact prior/result digests. It never
invents a person, agent, session, or intermediate operation. A later read-only
registration review may then judge the reconciled bytes normally. The MCP
agent union omits reconciliation.

Typed proposal writes serialize the artifact and provenance append under the
checkout writer lock. They remain non-authoritative working-tree bytes until
Git commits them together and the accepted resolver proves their complete
pair at default-branch HEAD. A crash between local files therefore leaves a
dirty, incomplete proposal that later validation refuses or reconciles; it
never creates a partially authoritative state and does not justify a second
transaction journal. Existing accepted canonical artifact bytes are never
rewritten to repair a missing provenance record.

## 7. Proposed versus accepted authority

Working-tree or review-branch lock and ratification bytes are proposals. The
application core has separate proposed and accepted resolvers:

- proposed operations validate exact caller-selected bytes and expected HEAD;
- authority-bearing inspection and lifecycle operations resolve one exact
  default-branch HEAD tree, enumerate the experiment directory through Git
  plumbing, and feed those bytes through the new source-backed entry point of
  the existing `internal/experiment` state algorithm.

The accepted resolver does not call `specstate.Projector`: that projector's
closed path grammar owns only spec artifacts. It also does not call the
filesystem-only `DeriveStateDetails`. Wave 5A adds one reader-backed
`experiment.DeriveStateDetailsFromSource` (name illustrative, signature fixed
by its TDD task) and retains `DeriveStateDetails` as the filesystem adapter.
Git enumeration, blob reads, default-branch HEAD identity, and state
derivation are each executed once; a mixed worktree/default-tree snapshot is
operationally impossible through the sealed result.

Registration is proven only when the locked definition's exact bytes and
digest are present in the accepted tree. Ratification is proven only when the
exact ratification bytes, bound result, definition, receipt, observations, and
candidate patches resolve from one accepted tree. An unmerged proposal never
enables execution, cleanup, or closure.

Wave 5 emits `verdi.experiment-ratification/v2`. Its actor block persists the
adapter-resolved `governanceprincipal.PrincipalClaim` (`trust_source` plus
stable `subject`) and the kernel-derived `principal_id`; it does not persist a
forgeable `PrincipalResolution` seal. V1 remains strict decode/state-history
compatibility but cannot be newly proposed or authorize release/closure.

```yaml
schema: verdi.experiment-ratification/v2
result_digest: sha256:...
actor:
  trust_source: github
  subject: stable-adapter-subject
  principal_id: principal/github/...
disposition: select-recommended
```

The actor keys are explicit ratification-v2 schema fields; the record does not
serialize the Go structure or inherit its incidental JSON/YAML field names.

At the accepted revision the human adapter resolves the persisted claim
through the configured governance profile and trust-fact reader, requires an
authenticated sealed result, and requires exact principal-id equality with
the record. Unproven authentication is a blocking verdict. A malformed claim,
principal mismatch, missing configured trust source, or internally
inconsistent identity evidence is operational. This uses the existing kernel
API; no adapter supplies a prebuilt principal ID and no feature-local identity
parser exists.

## 8. CLI and MCP adapters

Wave 5B adds one top-level CLI namespace:

```text
verdi experiment <operation> ...
```

Wave 5B registers the operations implemented through 5B, including the
explicit local-human `reconcile-draft` and registration proposal. Wave 5C
serially extends the same dispatcher with ratification proposal, capsule
publication, and workspace release/retry after their core methods exist;
comparison-backed closure remains an additive check in the existing
`verdi close` path.
Subcommands map one-to-one to the application operations available at that
unit. Machine output is
canonical JSON when `--json` is selected; human output is a rendering of the
same typed result. Standard streams and the repository exit contract remain
unchanged: `0` clean/proven, `1` completed refusal or unproven verdict, `2`
operational.

### Offline human authorization challenge

`reconcile-draft`, `propose-registration`, and Wave 5C
`propose-ratification` do not borrow an ambient forge session, Git author
string, operating-system user, request field, Git signature configuration, or
credential store. They use one offline, action-bound Ed25519 adapter. Verdi
never reads a private key, invokes a signing command, opens a network
connection, or asks an agent process to approve a human operation.

The first invocation without `--human-proof` is read-only and returns verdict
`human-authorization-required` plus canonical JSON for exactly one challenge:

```json
{
  "schema": "verdi.experiment-human-challenge/v1",
  "operation": "propose-registration",
  "spike": "spec/example",
  "experiment_id": "comparison",
  "accepted_head": "0123456789abcdef0123456789abcdef01234567",
  "proposal_head": "89abcdef0123456789abcdef0123456789abcdef",
  "trust_source": "offline-human",
  "input_digest": "sha256:...",
  "proposal_digest": "sha256:..."
}
```

The closed operation vocabulary is `reconcile-draft`,
`propose-registration`, and `propose-ratification`. `input_digest` binds the
canonical operation input: an empty object for reconciliation, the exact
registration-review packet digest for registration, and the complete typed
ratification input for ratification. `proposal_digest` binds the complete
current human-authored experiment artifact set; machine evidence remains
excluded by the same projection that owns mutation provenance.
`proposal_head` is the current proposal-branch HEAD at challenge creation and
proof use, while `accepted_head` remains the separately resolved
default-branch authority coordinate.

The selected accepted governance profile must contain an
`identity-provider` trust source and at least one role-mapping subject for it
with exact grammar `ed25519:<base64.RawURLEncoding of 32 public-key bytes>`.
The subject is both the stable kernel subject and the self-contained public
verification key; no keyring, Git configuration, certificate service, or
second signer registry is consulted. The human saves the exact canonical
challenge bytes and manually signs that file with the matching private key.
The documented OpenSSL example is a prompt, not a Verdi subprocess:

```bash
openssl pkeyutl -sign -rawin -inkey <private-key.pem> \
  -in <challenge-file> -out <signature-file>
```

Verdi does not run that command. The second identical invocation supplies
`--human-proof <signature-file>`. The file must be exactly one raw 64-byte
Ed25519 signature over the canonical challenge bytes. The adapter loads the
constitution store from the same exact accepted Git tree as `accepted_head`
through a shared source-backed `policyauthority` loader, reuses that store's
cross-validation, obtains a deep-cloned sealed selected profile through one
owned accessor, and tries only the mapped Ed25519 subjects of the challenge's
identity-provider source. Exactly one key must verify. It then supplies only
the normalized trust fact to the existing
`governanceprincipal.Resolver`; only the kernel may mint the sealed
`PrincipalResolution` consumed by `experimentapp.NewAuthenticatedHuman`.

The canonical evidence-digest preimage is the exact byte concatenation
`"verdi.experiment-human-proof/v1\x00" || challenge-bytes || signature-bytes`;
the trust fact carries lowercase `sha256:` over that preimage. Display names,
email addresses, commit authors, ambient usernames, and caller-provided
subjects never become identity. An unknown source, unmapped key, invalid
signature, missing proof, or challenge that no longer matches current inputs
is a verdict with stable witnesses. A malformed/noncanonical challenge,
wrong-length proof, unsafe or ambiguous accepted-tree state, failed exact-tree
policy loading, or broken fact-port contract is operational. A successful
proof is single-state: either the human operation changes the bound proposal
digest, so replay no longer matches, or a no-op/refused operation confers no
durable actor token or serialized resolver seal.

The JSON and human renderings include the challenge and manual signing prompt
as data. MCP structurally omits all three human operations and never accepts a
proof. This is an authentication adapter only: the application core
still owns review, policy, writer locking, provenance, and mutation semantics.

### Explicit execution input bindings

`start` and `resume` require one canonical
`verdi.experiment-input-bindings/v1` document. Its wire is UTF-8 canonical JSON
with exactly the required top-level fields `schema` and `inputs`; `schema` is
the literal `verdi.experiment-input-bindings/v1`, and `inputs` is an explicitly
present, non-null array whose entries have exactly `slot`, `id`, `digest`, and
`path`. The strict decoder rejects invalid JSON or UTF-8, missing, null,
unknown, or duplicate fields at either level, trailing data, and bytes unequal
to the shared canonical JSON re-encoding. The closed slot grammar is
`workload`, `contract`, or
`fixture:<fixture-id>`; the suffix of a fixture slot must equal that fixture's
locked definition id. Entries are sorted lexically by slot in canonical bytes.

The document must contain exactly one entry for the workload, exactly one for
the contract, and exactly one for every fixture, with no other or duplicate
slot or path. Each id and digest must equal the locked definition
reference for that slot. Each path uses the existing canonical repository-path
grammar, must be named by `protected_paths`, and is resolved below the explicit
execution root. The existing `experimentrun` input proof remains authoritative
for non-symlink regular-file custody and exact raw-byte SHA-256 parity.

One shared strict codec and bound resolver in `internal/experimentrun` own this
grammar and validation. The application operation transports the typed binding
document for that invocation. CLI `start` and `resume` require
`--inputs <path|->`; the MCP adapter accepts the same typed document. No adapter
may infer a path from an artifact id, scan protected paths by digest, use an
ambient mapping, or implement a second binding grammar. This explicit document
is operation input, not accepted experiment authority, durable run evidence, or
a new mapping store; the immutable execution receipt continues to bind the
resolved protected paths and their exact digests.

Wave 5B also adds one MCP tool named `experiment`. Its request is a strict
closed union over the agent-permitted operation subset. This is one typed tool,
not a free-form command tunnel. It never accepts `propose-registration`,
`propose-ratification`, `publish-capsule`, `release-workspaces`, or closure.
Unknown operations and human-only operation names fail explicitly.

The CLI verb and MCP tool inventories are serialized shared registries. Wave
5B cannot overlap any ASD, CI, or GLG task that changes either registry, and
Wave 5C cannot overlap another CLI-registry owner while it extends the
namespace. Spec-alignment and showcase inventories change in the same commit
as their live registrations.

CLI and MCP conformance tests feed semantically identical requests through
both adapters and require byte-identical core result projections. The MCP
wrapper retains the existing data-never-instructions safety note; free text
from definitions, witnesses, or provenance is always returned as untrusted
data.

## 9. Ratification, capsule, release, and closure

The human adapter obtains one sealed authenticated principal resolution before
building a ratification proposal. It copies the resolution's exact claim and
kernel-derived principal id into the v2 actor block and runs the record and
binding validators. The core cannot accept any actor field supplied by the
request. Accepted use repeats the kernel resolution from that persisted claim
and compares the derived id; the original in-memory seal is neither serialized
nor treated as reconstructable proof.

For a selecting ratification, the selected candidate is:

- the bound result's winner for `select-recommended`; or
- the explicitly named registered candidate for `select-other`.

The capsule builder deterministically produces the existing
`verdi.experiment-capsule/v1` manifest. Its complete required artifact set is:

- locked definition;
- selected candidate patch;
- evaluator capabilities;
- behavioral contract;
- workload;
- every fixture;
- selected run execution receipt;
- the selected run's complete observations;
- selected result;
- ratification; and
- recommendation explanation when present.

Artifact ids are the following exact closed vocabulary and mapping:

| Retained member | Capsule artifact id |
|---|---|
| locked definition | `definition` |
| selected candidate patch | `candidate-patch` |
| evaluator capabilities | `evaluator-capabilities` |
| behavioral contract | `contract` |
| workload | `workload` |
| each registered fixture | `fixture-<registered-fixture-id>` |
| selected run execution receipt | `execution-receipt` |
| selected run observations | `observations` |
| selected result | `result` |
| ratification | `ratification` |
| recommendation explanation, when present | `recommendation` |

`<registered-fixture-id>` is the definition's exact registered fixture id and
must already satisfy the capsule artifact-id grammar when prefixed by
`fixture-`. The capsule manifest describes this inventory and is not itself a
retained member. No builder or adapter may derive an artifact id from a path,
display label, media type, or local naming convention. Digests are recomputed
from exact bytes.
The builder rejects missing, extra, duplicate, mutable, symlinked,
non-regular, or digest-mismatched inputs. The manifest is published immutably
under the writer lock and re-decoded before release begins.

Release calls the existing execution-workspace release operation for each
known experiment-scoped candidate workspace. Selecting ratification releases
all disposable workspaces after the capsule is safe; the capsule, not a dirty
prototype checkout, is the retained reproduction set. Non-selecting
ratification releases every disposable workspace without producing a capsule.
Minimal experiment artifacts are never release targets.

The existing spike-close service receives an additive CSE evidence provider.
For a comparison-backed spike it requires accepted ratification. A selecting
disposition additionally requires a valid capsule whose definition, result,
candidate, and ratification identities match. It then uses the ratified answer
through the spike's existing `resolves` edge. It adds no edge and never edits
the parent feature. Non-selecting dispositions remain honest terminal human
responses to an experiment but do not satisfy open-question closure.

## 10. Failure and exit semantics

Application results classify exactly three outcomes:

- `clean`: the operation completed; for a comparison, a proven winner exists;
- `verdict`: a well-formed operation completed with an unsatisfied policy,
  authority, lifecycle, or evidence condition; or
- `operational`: required bytes or external facts could not be safely
  interpreted or changed.

CLI maps them to exits 0, 1, and 2. MCP returns a typed tool failure carrying
the same classification and witness; JSON-RPC framing errors remain protocol
errors.

Examples of verdict outcomes include policy refusal, unauthenticated human
authority, an agent request for a human-only operation, unreproduced posture,
an inconclusive comparison, and unsatisfied closure evidence. Examples of
operational outcomes include malformed or noncanonical bytes, ambiguous Git
state, unsafe filesystem shape, lock contention, evaluator/harness failure,
and cleanup failure.

No operational error becomes a comparison verdict. No cleanup error rewrites
ratification. No missing fact is interpreted favorably.

## 11. Verification and adversarial coverage

Every unit uses TDD and receives an independent task review before the next
unit crosses its authority boundary. Required evidence includes:

- strict decode, canonical encode, digest, clone-safety, and version migration
  tests for every new record;
- singleton-versus-layered payload registration, duplicate layered owners,
  canonical scope selection, unknown applicability, and mutation-safe sealed
  layer transport;
- policy refinement tables proving intersection/union/deny semantics and
  lower-layer non-weakening;
- observation raw/canonical boundary tests below, at, and above the effective
  policy cap and hard transport cap, plus per-artifact capsule retention
  boundary tests with lower-layer minimum selection;
- application tests proving one effective-policy resolution per operation;
- hermetic Git histories distinguishing proposed from accepted bytes and
  filesystem-versus-exact-tree state parity under divergent worktrees;
- authenticated-principal, persisted-claim re-resolution, and
  unauthenticated-agent custody tests;
- CLI built-binary and live in-process MCP conformance tests, including the
  serialized Wave 5C CLI extension and unchanged agent exclusion;
- explicit MCP refusal of every human-only operation;
- offline signed-challenge tests covering exact accepted/proposal HEAD and
  operation/input/proposal binding, source-backed accepted-profile loading,
  mapped Ed25519 keys, malformed/invalid signatures, stale inputs, replay, and
  proof-free agent refusal;
- registration immutability, read-only unreconciled review, and explicit
  direct-edit reconciliation tests;
- reproduction tables over zero, incomplete, agreeing, disagreeing,
  malformed, and extra visible runs;
- capsule exact-inventory/digest tests;
- cleanup tests for symlink, nonregular, missing, partial, contended, retried,
  selecting, and non-selecting states;
- closure tests proving no new edge or automatic question resolution;
- `go test -race ./...`; and
- `make verify`, including spec-align, showcase, and existing Playwright gates.

Wave 5 adds no browser behavior, so it adds no CSE Playwright suite. Wave 6
must exercise the same core through FABLE-owned workbench tests.

## 12. Delivery gates

Wave 5A may begin only after this authority and its ledger entries merge.
Wave 5B begins only after Wave 5A's exact head receives independent review and
the CLI/MCP registries are free. Wave 5C begins only after Wave 5B adapter
conformance is green. No unit may absorb another Wave 5 feature lane merely
because up to three orchestration slots exist.

Each unit lands in its own pull request. Each PR is based on current main,
contains only its owned packages and authority-aligned inventory changes, and
passes the full gate at its final reviewed head. Wave 6 remains blocked until
Wave 5C lands.

## 13. Lossless source-coverage witness

This design maps every deferred CSE Wave 5 obligation from the original
orchestration and the Wave 3B handoff:

| Source obligation | Destination | Transformation or omission |
|---|---|---|
| Wave 5 CSE CLI and agent adapters | §§2, 3, 8, 11 | One CLI namespace extended in 5B then 5C, plus one unchanged agent-safe MCP union over one core |
| AC-5 same typed operations | §§2–3, 8 | Adapter parsing/rendering separated from core semantics |
| AC-5 human registration lock | §§3, 7 | Proposal bytes separated from merge-signaled accepted authority |
| AC-5 authenticated ratification | §§3, 7, 9 | Ratification v2 persists the kernel claim/id operands; accepted use re-resolves the claim and compares the id |
| AC-5 spike closure | §9 | Additive evidence provider into the existing closure path; no new edge |
| AC-4 reproduction rule deferred by Wave 3B | §4 | Minimal registered run-count rule over existing unanimous aggregate |
| AC-4 cleanup deferred by Wave 3B | §§3, 9 | Release only after durable ratification; minimal evidence excluded |
| AC-4 selected capsule deferred by Wave 3B | §9 | Exact closed inventory over existing capsule v1 manifest |
| Exact capsule artifact-id vocabulary left unnamed by the inventory | §9, SI-149 | Owner-approved one-to-one member mapping, including `fixture-<registered-fixture-id>`; manifest intentionally excluded as a member |
| AC-6 concrete policy resolver deferred by Wave 3B | §5 | Generic Context Integrity layered-payload selection plus typed commutative CSE reduction; no feature-local hierarchy |
| AC-6 policy constraints | §§5, 9 | Paths, classes, evaluators, exact environment values, shared grants, observation/retained-artifact byte ceilings, sources, and mandatory guards; each limit has one owned enforcement boundary |
| AC-6 mutation provenance | §6 | CSE-specific strict append-only record; read-only detection plus explicit direct-edit reconciliation mutation |
| DC-2 derived state | §§3–4, 7 | One experiment state algorithm over filesystem or exact-tree byte sources; no new state artifact or preferred-run pointer |
| DC-7 human-only decisions | §§3, 7–9 | Agent surface structurally omits authority operations |
| DC-7 human authentication evidence | §8 | Manual detached Ed25519 signature binds the exact operation and proposal bytes; the public key comes from the exact accepted profile and the existing kernel mints the actor, while Verdi never handles private credentials |
| AC-3/AC-4 execution input path custody | §8 | One strict per-operation slot/id/digest/path document feeds the shared runner resolver; raw bytes, protected-path membership, and receipt binding remain with the landed execution proof |
| DC-8 durable reasoning/disposable prototypes | §9 | Capsule first, then workspace release |
| DC-9 no patch promotion | §§1, 9 | Capsule is evidence only; product source untouched |
| DC-11 no privileged adapter | §§2, 8, 11 | CLI/MCP parity against one core |
| DC-15 visible reruns and reproduction | §4 | Every visible result participates; no favorable selector |
| DC-16 ratification dispositions | §§3, 7, 9 | Selecting and non-selecting consequences made explicit |
| CO-1 three-valued honesty | §§4, 10 | Missing/ambiguous facts never pass |
| CO-2 strict schemas | §§4–6, 8–9 | Versioned closed records and unions |
| CO-3 deterministic artifacts | §§4, 6, 9 | Canonical bytes, digests, sorted inventories, immutable writes |
| CO-4 process exit mapping | §10 | Exact 0/1/2 adapter mapping |
| CO-5 scoped claims | §§4, 8–9 | Explanation and reproduction remain definition/run scoped |
| CO-7 test requirements | §11 | Core, Git, binary CLI, MCP, adversarial filesystem, full gates |
| Wave 6 CSE workbench | Explicitly omitted in §§1, 8, 11–12 | Deferred unchanged to FABLE-owned Wave 6 |
| Wave 7 genuine comparison | Explicitly omitted in §12 | Deferred unchanged until Wave 5C lands |

Coverage: **29 of 29 source obligations mapped**. No source capability is
silently removed. Candidate-reported corroboration remains the canonical
unresolved OQ-2 and is intentionally omitted from decision eligibility.
