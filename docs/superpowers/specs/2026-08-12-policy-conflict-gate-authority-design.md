# Policy Conflict Gate Authority Design

**Status:** Owner-approved design; repository authority becomes effective when
the reviewed commit carrying this document and SI-93 through SI-103 reaches the
configured default branch.

**Planning base:** `bfaa8e2715678cbe8cd71137f23d743666caf1c4`

**Owners:** platform-team

**Delivery unit:** Context Integrity `policy-conflict-gate`

**Consumers:** specification acceptance, current build start, evidence intake,
closure, and the later sealed-execution adapter

## 1. Decision and scope

The Wave-3 policy conflict gate is one application service over one proof
algebra. It consumes either a compiled accepted context or an exact proposed
acceptance candidate, computes typed mechanical proof, discovers semantic
candidates over normalized authority prose, resolves policy exemptions and
semantic dispositions, and returns one strict canonical report and verdict.
Every lifecycle consumer uses that result unchanged. A consumer may render the
result, but it may not reclassify a finding, ignore a blocking state, or own a
parallel conflict rule.

This unit delivers Context Integrity v2 AC-3 and the Wave-3 orchestration row
for `policy-conflict-gate`. It includes:

- the request, report, judge-record, and semantic-disposition schemas;
- proposed-candidate and accepted-context operand construction;
- proof-grade scope comparison and mechanical satisfiability;
- semantic candidate discovery, strict judge validation, challenger posture,
  and immutable result reuse;
- exemption and disposition matching, staleness, boundedness, and principal
  authorization;
- `verdi context conflict --request <path|-> [--out <path>]`;
- the currently reachable acceptance, build-start, evidence-intake, and
  closure integrations; and
- deterministic, hermetic, and built-binary verification.

It does not deliver sealed process launch, receipt authentication, managed
runner trust, a production forge/signature/identity-provider reader, data
expansion, resume, MCP, or any browser/workbench surface. Those remain owned by
later delivery units. It also does not reinterpret the legacy alignment or
decision-conflict schemas.

The core evaluator is pure over declared operands. The process adapter may
write immutable semantic-judgment cache records under the gitignored data zone.
The only committed authority added by this unit is the human-authored semantic
disposition artifact.

## 2. Public invocation and request

The inspection surface is:

```text
verdi context conflict --request <path|-> [--out <path>]
```

There is no new top-level verb and no MCP v1 tool. `--request -` reads stdin.
With no `--out`, stdout is the canonical report. With `--out`, the command
writes the report atomically to that explicit destination and writes no report
bytes to stdout. The existing context-compiler output fence applies: the
destination must be outside `.verdi/`, outside every managed projection path,
different from the request path, symlink-free at every existing path
component, and resolved against the canonical checkout root before use.

The request is strict canonical JSON with schema
`verdi.policy-conflict-request/v1`. Its exact domain shape is:

```go
type Request struct {
    Schema string
    Target Target
}

type Target struct {
    Kind                TargetKind
    AcceptedContext     *contextcompile.Request
    AcceptanceCandidate *AcceptanceCandidate
}

type AcceptanceCandidate struct {
    Adapter  contextcompile.AdapterRef
    Expected contextcompile.Expected
    Grants   execworkspace.GrantSet
    Scope    policyartifact.Scope
    Spec     string
}
```

`Target.Kind` is exactly `accepted-context` or `acceptance-candidate`.
Exactly the matching payload is present; the other key is omitted. Explicit
`null`, an absent matching arm, both arms, unknown fields, duplicate keys,
trailing data, invalid UTF-8, or noncanonical bytes fail operationally.

`accepted-context` embeds the complete existing
`verdi.context-compile-request/v1` object without changing its grammar.
`acceptance-candidate` fixes phase to `design`, requires both expected branch
and full 40-hex HEAD, requires an unpinned whole `spec/<name>` ref, and uses the
same strict adapter, grant, and scope types as context compilation. The service
derives `.verdi/specs/active/<name>/spec.md`, verifies the actual branch and
HEAD, reads the exact HEAD-tree regular blob, strict-decodes it, requires its
declared ID to match, and records its blob and content digest. Request claims
never replace computed repository or candidate facts.

The candidate arm exists because merge-signaled acceptance evaluates proposed
bytes before they can be an accepted context. It does not emit an accepted
context manifest, accepted-baseline identity, or authoritative context claim.
It emits a candidate identity inside the conflict report and otherwise feeds
the same normalized operand and proof pipeline.

The command exits:

- `0` only for a report whose overall state is `pass`;
- `1` for a completed `blocked-violated` or `blocked-unproven` report, including
  an explicit invocation in a legacy store without a constitution; and
- `2` for malformed input or authority, cache corruption, I/O, timeout,
  cancellation, unavailable required repository facts, or another operational
  failure.

## 3. One service and one operand boundary

`internal/policyconflict` owns the application service. Its consumer-facing
port is:

```go
type VerdictProvider interface {
    Evaluate(context.Context, Request) (Result, error)
}

type Result struct {
    Report      Report
    ReportBytes []byte
}
```

The production service performs one ordered evaluation:

1. Validate and canonicalize the request.
2. Resolve either the accepted context or proposed candidate against one
   repository snapshot.
3. Obtain sealed normalized conflict operands from `internal/contextcompile`.
4. Verify that every operand identity agrees with the context/candidate,
   effective-policy, profile, repository, and source digests in that snapshot.
5. Compute scope relations and mechanical satisfiability.
6. Build semantic inputs for prose relations and mechanically unknown scope.
7. Reuse or run primary and, when supplied and required, challenger judgment.
8. Load and resolve current exemptions and dispositions.
9. Interpret authenticated principal resolutions through
   `internal/governanceprincipal`.
10. Derive and canonicalize the one report and overall verdict.

The context compiler gains a sealed runtime-only `ConflictOperands` projection.
It is not a fourth wire artifact and carries no verdict semantics. Its content
is:

```go
type ConflictOperands struct {
    Snapshot          SnapshotIdentity
    EffectivePolicy   policyauthority.EffectivePolicy
    TypedClaims       []TypedClaim
    ProseClaims       []ProseClaim
    Exemptions        []policyartifact.Exemption
    Profile           governanceprincipal.Profile
    ActorResolutions  []governanceprincipal.PrincipalResolution
}
```

The actual Go representation may use sealed pointers and unexported fields to
preserve the landed mutation guard, but it carries exactly those semantic
groups. `SnapshotIdentity` binds request kind, repository facts, context
manifest or candidate identity, effective-policy digest, constitution digest,
profile identity/digest, adapter, phase, scope, capabilities, and exact source
digests. `TypedClaim` adds its governing policy ID to the complete normalized
`policyartifact.Claim`. `ProseClaim` is defined in §6.

Accepted-context construction reuses the existing compiler result and the
already-resolved sealed policy store; the conflict service never reloads
policy. Candidate construction reuses the compiler's repository, policy,
applicability, fragment, decision, obligation, and prose-normalization
components but substitutes the exact proposed candidate identity for accepted
baseline resolution. It cannot call the accepted-manifest encoder.

Hand-constructed, mutated-after-construction, digest-inconsistent, or
cross-snapshot operands fail operationally. A report is never derived from
manifest summary rows alone.

## 4. Scope comparison

Scope comparison is a four-dimension conjunction whose result is exactly
`overlap`, `disjoint`, or `unknown`. An explicitly empty dimension is
universal. Omission remains invalid at policy decode.

### 4.1 Phase and environment

Two nonempty phase or environment sets overlap when their exact intersection is
nonempty and are disjoint otherwise. A universal side overlaps the other side.
The registered closed phase and environment vocabularies make this proof
complete.

### 4.2 Paths

Paths retain `policyartifact`'s canonical relative grammar. A value without a
trailing slash is one exact entry. A value with a trailing slash is a directory
subtree. Two path values overlap when they are exact-equal, when an exact entry
is inside the other value's directory subtree, or when one directory subtree
contains the other. Otherwise they are disjoint. Path comparison is
segment-aware: `cmd/` contains `cmd/verdi/main.go` but not `cmdline/x`.

Two nonempty path sets overlap when any value pair overlaps and are disjoint
when every pair is disjoint. A universal side overlaps the other side.

### 4.3 Artifact refs

Exact-equal refs overlap. A universal side overlaps the other side. For
different refs, the evaluator uses only the accepted/candidate capsule's exact
validated artifact graph. A feature and an implementing or resolving story or
spike overlap; a whole-spec ref and its own fragment overlap; two fragments of
the same object overlap. A graph-proven unrelated pair is disjoint only when
the graph establishes separate accepted roots with no shared governing edge.
Missing, ambiguous, incomplete, or unavailable graph evidence is `unknown`,
never guessed disjoint.

### 4.4 Product rule and witness

For a pair, if any dimension is proven disjoint, the scopes are disjoint. If
all four are proven overlapping, they overlap. Otherwise the result is unknown.
The same product rule extends directly to an N-way claim witness: each
dimension intersects all participating sets or selectors once, and the product
is overlap only when every dimension has a nonempty proven intersection. Scope
overlap is not assumed transitive and is never used as an equivalence relation.
Every comparison records a per-dimension state and the exact set, path, or
graph-edge witness. Sorting is phase, environment, path, then ref; values inside
each dimension are canonical lexical order.

## 5. Mechanical satisfiability

Claims interact only inside one `(family, subject)` group, except that an
identity group additionally keys on its canonical unordered role pair.
Every group uses exactly one of four Verdi-owned operand domains. A group
containing operators from more than one domain is invalid authority and fails
operationally; Verdi does not choose a meaning per pair.

Each domain solver first evaluates the complete conjunction without using
scope. A satisfiable complete conjunction proves every scoped subset
satisfiable. For an unsatisfiable conjunction, the solver emits deterministic
claim witnesses:

1. every exact-scope subgroup is solved as an N-claim conjunction;
2. every differently-scoped claim pair is solved once; and
3. if the complete group is unsatisfiable but neither step produces a
   proven-overlap contradiction, the remaining higher-order scope case is
   `blocked-unproven` with its complete claim witness.

An unsatisfiable witness becomes `blocked-violated` only when §4.4 proves its
N-way scope intersection nonempty. Proven-disjoint witnesses do not conflict.
Unknown intersections and unresolved higher-order combinations move to the
semantic input and remain blocking. This conservative boundary may withhold a
mechanical conclusion, but it can neither manufacture a conflict from
non-transitive pair overlap nor let an unresolved conjunction pass.

When steps 1–2 find no overlapping contradiction, the evaluator may prove the
complete group harmless by scope only through SI-107's component rule. Claims
are joined when their pair is not proven disjoint (unknown remains joined).
`scope-disjoint` is proven only when at least two components exist and the
same domain solver proves every component satisfiable; otherwise the complete
higher-order witness remains blocking-unproven. A disjoint complete N-way
intersection alone is never sufficient.

### 5.1 Discrete-set domain

The operators are `equals`, `not-equals`, `allowed-values`,
`required-values`, and `forbidden-values`. The subject value is a nonempty set
of strings `X`:

- `equals(v)` means `X = {v}`;
- `not-equals(v)` means `v` is not a member of `X`;
- `allowed-values(A)` means `X` is a subset of `A`;
- `required-values(R)` means `R` is a subset of `X`; and
- `forbidden-values(F)` means `X` has empty intersection with `F`.

Multiple constraints are conjunctive. Allowed sets intersect; required and
forbidden sets union; each `not-equals(v)` contributes `v` to the forbidden
set. Multiple `equals` values must agree. With an equals constraint, its
singleton must satisfy every bound. Without equals, the solver requires all
required values to remain allowed and unforbidden and, when an allowed bound
exists, at least one allowed unforbidden value. If no finite allowed bound
exists, a symbolic other value proves satisfiability unless the other
constraints already force a forbidden exact value. The report records the
finite sets and whether the proof used the open-domain witness.

### 5.2 Integer-interval domain

The operators are `minimum` and `maximum`. All minimum bounds reduce to their
maximum; all maximum bounds reduce to their minimum. The conjunction is
unsatisfiable exactly when effective minimum exceeds effective maximum.
Equality is satisfiable. Bounds use the landed signed Go `int` grammar; decode
already rejects non-integers and overflow.

### 5.3 Principal-relation domain

The operators are `same-principal` and `different-principal`. For these two
operators, the existing claim `values` field contains exactly two distinct role
IDs; `subject` is the governed transition ID. The two roles are a semantic set
and normalize lexically, because both kernel relations are symmetric. This is
a narrow operand rule inside the existing claim schema, not a new identity
type or resolver. It prospectively replaces the policy-artifact validator's
placeholder empty-values rule for these two previously uninterpreted
operators; no committed policy artifact currently uses either operator. Policy
decode rejects a bound or fewer or more roles; conflict-operand construction
validates the transition and both roles through the existing kernel catalog.

The evaluator never compares principal strings. It constructs one kernel
authorization request over the exact authenticated resolutions, transition,
canonical role pair, profile, and separation mode. Requiring both relations for the
same transition and role pair is a mechanical conflict. Requiring one relation
is proven only when the kernel returns that conclusion; violated and unproven
kernel results remain violated-with-witness or unproven respectively.
The kernel identifies each distinctness finding with its exact sorted role
pair. The evaluator consumes only whole-request authority findings and
findings whose role or role pair belongs to the requested relation; unrelated
governance findings cannot change this solver result. Advisory/experimental
kernel posture is unproven for the authoritative consumer, not evidence that
the requested relation is violated, and its row reason is
`profile-experimental`. Kernel-authorized `solo-role-collapse` disclosures are
translated to the report code `solo-principal-collapse`, returned beside the
runtime evaluations, and hoisted once to the report's top-level disclosure
set. These are distinct closed vocabularies; an unknown kernel disclosure is
an operational error rather than a new report label. Each kernel principal/role
membership is retained as one report witness token
`<principal_id>:<role_id>`, sorted and deduplicated. The existing component
grammars both forbid `:`, so the association is lossless without a second wire.

### 5.4 Path-capability domain

The operators are `path-read` and `path-write`. Each names required access,
not executable policy code and not an implicit prohibition. Same-kind path
requirements union; read and write may coexist, so these claims are mutually
satisfiable. The conflict gate records the canonical requirement set but does
not reinterpret a missing execution grant as a policy conflict. Grant
satisfaction and enforcement remain with the execution boundary; as DC-5
requires, missing execution is an unmet requirement rather than a conflict.

### 5.5 Exemption application

The evaluator computes the original mechanical result before exemptions. An
exemption may depart only from exact current claim witnesses it names. It is
eligible only when its scope covers the evaluated conflict scope, its witness
digests are current, its bound is effective under §8, and its approvals are
authorized under §9. Effective exempted claims are removed only inside the
covered scope and the same solver runs again. A mechanical conflict is covered
only when the post-exemption conjunction is satisfiable or disjoint. The report
retains the original proof, exact removed claims, exemption identity/digest,
authorization result, and post-exemption proof. A removed mechanical claim is
identified by `(policy_id, claim_id, claim_digest)`, not by the semantic
prose-witness vocabulary. An effective all-five-proven resolution names at
least one exact current row witness; an ineffective resolution names the
mandatory-present explicit empty removal set (`[]`) because it removed
nothing. Removal witnesses sort and deduplicate by `(policy_id, claim_id)`;
their digest must match that exact current row claim.

A semantic disposition can never erase a mechanical proof. `no-conflict` is
not a legal mechanical resolution.

## 6. Semantic claims and judge protocol

The semantic universe contains every normalized human-authored authority prose
claim included in the target capsule and no non-authoritative repository data.
The closed source categories are:

- `policy-instruction`;
- `spec-problem` and `spec-outcome`;
- `acceptance-criterion`, `open-question`, `constraint`, and `decision` from
  the target spec;
- the same feature-object categories from each governing parent feature;
- `adr-decision`; and
- `obligation-declaration`.

Each `ProseClaim` carries a canonical source ID, category, normalized single
text value, text digest, source artifact ref/path/content digest, inherited
scope, governing authority digest, and the exact object/line identity from
which it came. Normalization validates UTF-8, converts CRLF to LF before
artifact parsing, trims only structural frontmatter/body delimiters, and never
case-folds, rewrites, summarizes, or reorders authored text. Policy
instructions inherit their policy/claim scope. Spec and obligation prose
inherit the request scope intersected with their exact spec/object ref.

The semantic input contains the complete sorted prose claims and one sorted
`UnknownMechanicalWitness { id, claims, scope }` for every mechanical row
whose reason is `scope-unproven` or `higher-order-scope-unproven`. Each such
witness retains the row's complete composite typed-claim records and exact
scope proof; it does not copy solver output or later authority-resolution
state. This includes a conservatively unresolved higher-order row even when
its aggregate scope state is `disjoint`. The input also carries applicable
exemption identities/digests and the canonical prompt. The prompt asks about overlap,
simultaneous satisfiability, refinement, explicit exception, authority, and the
strongest reasonable non-conflict interpretation. Prompt bytes are fixed
repository code, not project configuration.

Semantic prose line identities are not general artifact fragment refs. They
are validated by closed source category: policy instructions use
`<policy-id>#instruction-<positive-n>` and retain the policy claim's declared
scope; spec and feature prose use the whole spec ref plus `problem`, `outcome`,
or the category's declared `ac-*`, `oq-*`, `co-*`, or `dc-*` object and carry
that line identity as their sole scope ref; an ADR body uses its exact pinned
ref plus `decision` as its sole scope ref; and an obligation declaration uses
its whole obligation ref as both line identity and sole scope ref. In every arm
the carried source ref, object, line identity, and claim ID agree exactly. This
does not widen the artifact contract's declared-object fragment grammar.

The primary judge's strict inner result is canonical JSON schema
`verdi.policy-conflict-judge-result/v1`:

```go
type JudgeResult struct {
    Schema         string
    Recommendation Recommendation
    Findings       []JudgeFinding
}

type JudgeFinding struct {
    Claims      []ClaimWitness
    Categories  []string
    Explanation string
}
```

`Recommendation` is exactly `conflict`, `no-conflict`, or `inconclusive`.
The recommendation covers the complete semantic-input digest; there is no
implicit pair ledger. `conflict` requires at least one finding, while
`no-conflict` requires the explicit empty findings set. `inconclusive` may
carry findings that explain the unresolved state. Every finding names at least
two distinct input claim witnesses. Claims and categories are unique and
sorted. Explanation is nonblank single-line UTF-8 and is evidence, never
authority. Verdi mints finding IDs from the validated result and the one
semantic-input ID from the complete normalized witness identity; it never
trusts a model-supplied ID or model identity. An unknown, missing, duplicate,
or digest-mismatched witness invalidates the result.

The recorded exchange contains only a completed, successfully validated
exchange: the adapter-declared transport/model posture, command digest, prompt
digest, semantic-input digest, raw result bytes and digest, and parsed result.
Its presence proves the single successful process/validation state, so the
canonical record does not carry single-value process or validation enums or an
always-empty validation-reasons field. Start, exit, timeout, cancellation, and
validation failures remain typed operational errors with redacted diagnostic
metadata and are never persisted as judgment records. Judge output cannot
override adapter-declared model identity. Raw bytes never enter command
construction or a human artifact. The CLI uses the existing
`align.judge_cmd` argv only as process transport; it does not reuse legacy
align prompts, finding IDs, permissive wrappers, disposition types, or
`judge_required`. Missing transport is a completed blocking-unproven semantic
state whenever semantic evaluation is required.

A well-formed `inconclusive` recommendation is likewise a completed
blocking-unproven state. A configured judge that cannot start, exits nonzero,
times out, or is cancelled is an operational failure. So is output with
invalid UTF-8, noncanonical or malformed JSON, unknown fields or enums,
invalid witnesses, or another failed validation. Those failures return a
typed operational error with the redacted exchange metadata needed for
diagnosis; they do not produce a canonical verdict report and cannot be
converted into a favorable fallback.

The service accepts separate primary and challenger ports. Solo and team v1
require the primary port only. High-assurance requires both ports and treats a
missing challenger as unproven. Primary and challenger receive the identical
canonical prompt and normalized semantic input independently; neither receives
the other's output. A recommendation difference, an inconclusive result, or a
difference between their sorted finding-witness sets is disagreement and
requires human disposition. A challenger cannot turn a primary result into an
automatic pass. The local CLI constructs only the primary port from
`align.judge_cmd`; this unit adds no second project-config command. A managed
caller may inject an independently configured challenger port. Consequently a
local high-assurance semantic evaluation is explicitly `blocked-unproven`
until a later authority defines a trusted challenger transport.

## 7. Semantic judgment reuse

The evaluator is pure; the CLI process adapter owns immutable machine records
under:

```text
.verdi/data/cache/policy-conflict-<layout-version>-<tree-hash>-<input-digest>.json
```

The record schema is `verdi.policy-conflict-judgment/v1`. The existing D4
`layout-version` and `tree-hash` are computed by `internal/store`;
`tree-hash` and the filename form of `input-digest` are exactly 64 lowercase
hexadecimal characters without a `sha256:` prefix, while record fields retain
the full canonical digest form. The logical `input-digest` binds the role
(`primary` or `challenger`), adapter-declared
transport/model posture, argv digest, prompt digest, normalized input digest,
profile/challenger posture, and effective authority digest. The content records
the complete exchange from §6, required `profile_id`, full `profile_digest`,
full `authority_digest`, and a self-digest. Together with the exchange's role,
adapter, model, command, prompt, and normalized-input digests, those fields are
all path-key components. A cache hit recomputes the bare outer `input-digest`
from the carried components and also requires the canonical `raw_result` to
decode to the carried parsed result; neither the outer key nor a
self-consistently rewritten record is trusted by assertion.

On a cache hit the adapter strict-decodes, canonical-reencodes, verifies the
path key and every digest, and returns the recorded result without launching a
process. A changed bound component selects another key. A malformed,
noncanonical, mismatched, symlinked, or post-write-mutated record is an
operational failure, not a cache miss.

On a miss the adapter executes and validates without holding the checkout-wide
writer lock. Cache directory creation and immutable publication then acquire
the existing nonblocking D3 `data/writer.lock`, use a temporary file followed
by an atomic no-clobber publication, and release it before returning. A process
that cannot acquire that existing lock fails operationally; it never creates a
second or narrower write authority. Another process may have published the
same key before this process acquires the lock. That winner is accepted only
after strict decode, canonical re-encoding, path-key verification, and
byte-identity with this validated exchange; a different winner is a collision
and fails operationally. Failed, timed-out, cancelled, or invalid process
attempts return the structured operational error defined in §6 and are not
cached as successful judgments. Failure to persist a validated successful
exchange is also operational; the adapter never silently returns an uncached
result whose later reuse posture differs.

The cache is gitignored, disposable, and never authority. It reuses the
existing D4 cache zone rather than introducing a new data-zone root. Deleting
it may spend another judgment but cannot change which disposition matches. No
new `verdi gc` behavior is created; the existing cache-entry pruning contract
applies.

## 8. Semantic disposition artifact

Semantic rulings live at:

```text
.verdi/policy/dispositions/<name>.md
```

The same authority tranche amends `spec/verdi-store-layout` to admit this path,
name policy-authority ownership, replace its obsolete spec-frontmatter
disposition statement, and extend the existing D4 cache filename grammar with
§7's conflict-judgment record. It creates no new data-zone root or writer
authority. Runtime implementation of disposition storage and judgment caching
remains blocked until the reviewed tranche reaches the default branch.

The human artifact schema is `verdi.policy-disposition/v1`, kind
`policy-disposition`, ID `policy-disposition/<name>`. It uses the shared
human-artifact renderer and immutable kernel. Its normalized content is:

```go
type Disposition struct {
    Schema               string
    ID                   string
    Kind                 string
    Title                string
    Owners               []string
    Scope                policyartifact.Scope
    Witness              SemanticWitness
    Conclusion           Conclusion
    Origin               Origin
    CompensatingControls []string
    Approvals            []policyartifact.Approval
    Expiry               string
    ReviewCondition      string
    Template             *policyartifact.TemplateRecord
    Rationale            string
}
```

`Conclusion` is `conflict` or `no-conflict`. `Origin` is `judge-result` or
`human-fallback`. `SemanticWitness` carries the semantic-input ID, every
claim's identity fields required by AC-3 — claim ID and digest, category,
scope, typed values/bounds when present, governing authority digest — the
context/candidate identity digest, and every applicable exemption ID/digest.
It does not duplicate authored claim text or raw judge output. The
semantic-input ID is the canonical digest of exactly that complete witness
identity. A `judge-result` record additionally cites the immutable primary and
challenger judgment-record digests that informed the human, when present, as
provenance; those citations do not redefine freshness or make the human ruling
depend on a repeatable model response. Cache presence is never required to
load or validate a disposition.

Claims, categories, and exemption witnesses normalize as unique sorted sets.
Compensating controls, when present, remain nonempty author-ordered single-line
prose. Owners and approvals normalize under their existing policy-artifact
rules. The body is the nonblank rationale, and at least one approval is always
required. A `human-fallback` must carry a real calendar-date expiry or nonblank
review condition and at least one compensating control. A judge-result
disposition needs no fallback-only control or time bound: under AC-3 it remains
current only while the complete semantic-input witness remains unchanged.
Unknown fields, conclusions, origins, or witness categories fail closed.

A judge-result disposition must match the current validated semantic input. A
fallback disposition is legal only when the current semantic
input exists but configured judgment is absent, a well-formed result is
inconclusive, or primary and challenger disagree; it binds the same complete
witness and cannot be a generic override. Malformed or failed judge execution
remains operational under §6. `no-conflict` resolves only an exact, current,
authorized semantic input. `conflict` establishes `blocked-violated`. A
stale or unauthorized disposition never partially applies.

`internal/policyauthority` owns loading, path/ID parity, strict decoding,
cross-reference validation, and inclusion in the effective-authority digest.
`internal/policyconflict` alone interprets whether a disposition matches and
governs the current semantic input. Existing legacy `.verdi/conflicts/`,
`decision-conflict-report.md`, deviation findings, and spec-frontmatter
dispositions remain unchanged and never satisfy this schema.

`verdi.effective-policy/v1` gains one always-present `dispositions` array,
sorted by canonical disposition ID. An authority store with no dispositions
encodes it as `[]`, not by omission, so the v1 shape has one deterministic
post-adoption form. Adding that field intentionally changes existing
effective-policy digests and every context authority revision and golden that
binds them; implementation updates those ratchets together. The existing
`verdi.context-manifest/v1` reservation remains an explicit empty
`dispositions: []` and rejects nonempty values. V1 therefore binds the exact
effective-policy digest but does not enumerate disposition identities; direct
manifest enumeration is deferred to a later recorded manifest version.

## 9. Bounds and authenticated principals

The report carries a declared UTC calendar date `evaluated_on`, obtained from
an injected date port. Production uses the current UTC date; tests inject it.
This is CO-3's declared provenance stamp and the exact input for expiry
comparison. An expiry before `evaluated_on` is expired; equality remains
effective through that date.

Review conditions are opaque named governance conditions, not self-proving
text. V1 has no evidence source capable of proving one. A record whose only
live bound is a review condition is therefore `blocked-unproven`; an unexpired
calendar expiry may still keep the record bounded. A later managed integration
may add a condition-evidence port together with its trust authority, but this
unit does not add a speculative resolver.

Approval strings in committed artifacts are claims, not authentication. The
service accepts sealed `governanceprincipal.PrincipalResolution` values and
constructs the kernel authorization request for the applied profile, required
roles, ownership, and separation mode. An authenticated authorized result may
satisfy the approval. Violated and unproven resolution states remain distinct
in the report and cannot authorize.

The local CLI has no trusted forge, signature, or identity-provider fact reader
in this unit. It therefore records approval authentication as unproven and
cannot make an exemption or disposition effective. Managed callers may inject
the existing kernel ports. Solo may collapse author and approver only when the
profile and kernel authorize that collapse and the report carries the kernel's
disclosure. Team and high-assurance retain their distinctness requirements.
Experimental profile results are always `blocked-unproven` for authoritative
consumers even when the conflict set is otherwise clean.

## 10. Canonical report and verdict

The machine report is strict canonical JSON schema
`verdi.policy-conflict-report/v1`. Its top-level shape is:

```go
type Report struct {
    Schema       string
    Input        InputIdentity
    Mechanical   []MechanicalEvaluation
    Semantic     []SemanticEvaluation
    Disclosures  []Disclosure
    Verdict      Verdict
    Digest       string
}
```

`InputIdentity` contains the strict `accepted-context`/`acceptance-candidate`
target union, repository snapshot, constitution and effective-policy digests,
complete sorted policy-entry identities, profile ID/class/digest, and
`evaluated_on`. The accepted target carries the manifest digest. The candidate
target carries ref, path, branch, HEAD, blob, content digest, scope, adapter,
and grant digest. These facts occur once; the report has no second
`context_manifest`, top-level exemption ledger, or top-level disposition
ledger that can drift from the evaluation rows.

Each mechanical row carries its deterministic witness ID, family, subject,
complete typed claims keyed by `(policy_id, claim_id)` with their canonical
base-claim digests, scope proof, domain, pre-exemption solver result,
applicable exemption resolutions, post-exemption solver result, state, and
reason codes. Each semantic row carries its semantic-input ID, normalized
claim identities, the sorted `UnknownMechanicalWitness` rows when applicable,
primary/challenger exchanges, applicable disposition resolution, state, and
reason codes. Each embedded authority resolution carries artifact ID/digest
plus match, freshness, scope, bound, and authorization states. Disclosures use
closed codes and sorted witness strings.

The closed reason-code vocabulary is exactly:

```text
mechanical-satisfiable
scope-disjoint
mechanical-conflict
scope-unproven
higher-order-scope-unproven
principal-relation-violated
principal-relation-unproven
exemption-effective
exemption-ineffective
judge-unavailable
judge-inconclusive
challenger-unavailable
judgment-disagreement
disposition-required
disposition-effective-no-conflict
disposition-effective-conflict
disposition-ineffective
profile-experimental
```

Each code names the row's immediate proof outcome; exact stale, mismatch,
scope, bound, and authorization detail remains in the row's typed authority
resolution rather than multiplying codes for every Cartesian combination.

The disclosure vocabulary reuses all fourteen closed
`contextcompile.DisclosureCode` values unchanged and adds exactly
`solo-principal-collapse`, emitted only when kernel authorization proves and
discloses the solo profile's permitted author/approver collapse. Judge,
challenger, disposition, scope, and experimental-profile outcomes stay row
reasons rather than duplicated top-level disclosures. Unknown reason or
disclosure codes fail closed.

The report self-digest is computed over the digestless canonical form. Arrays
sort by stable witness ID except author-ordered prose arrays explicitly named
in their source schemas. No map with semantic iteration order enters a digest.
No absolute checkout path, credential-bearing remote, secret value, process
environment, wall-clock time beyond `evaluated_on`, or random value appears.

The overall verdict is exactly:

```text
pass
blocked-violated
blocked-unproven
```

`pass` requires every relevant relation to be mechanically satisfiable or
disjoint, or resolved by exact current authorized exemption/disposition, with
no blocking disclosure. `blocked-violated` wins when an uncovered mechanical
conflict exists or an effective human disposition concludes conflict.
`blocked-unproven` applies when no violated state exists but scope, judge,
challenger, disposition, approval, expiry/review condition, profile posture, or
required witness remains unproven. If violated and unproven rows coexist, the
overall state is `blocked-violated` and the unproven rows remain visible.

Judge recommendations never directly yield `pass`. A recommended
`no-conflict` still requires an effective disposition. A recommended conflict
is a candidate until an effective conflict disposition confirms it.

## 11. Lifecycle integration and compatibility

Every integration receives a `VerdictProvider`; none imports solver internals.
The lifecycle commands obtain request operands explicitly rather than guessing
an adapter, capability grant, environment, or scope. Their additive grammar is:

```text
verdi gate [--context-request <path>]
verdi build start <story-spec | story-ref> [--context-request <path>]
verdi close <story-ref> [--force-local] [--context-request <path>]
verdi close --preflight <story-ref> [--force-local] [--context-request <path>]
verdi close --prepare <story-ref> [--force-local] [--context-request <path>]
```

`--context-request` accepts a filesystem path, not `-`, and names an existing
strict canonical `verdi.context-compile-request/v1` document. The explicit
`context conflict` inspection surface remains the only stdin consumer. A
lifecycle adapter strict-decodes the document through `contextcompile`,
requires its phase and spec to match the actual command target, compares an
optional caller `expected` claim to the computed branch and HEAD when present,
and then replaces or fills that optional claim with the computed exact facts.
For a design gate it converts the remaining adapter, grants, scope, and spec
operands into the `acceptance-candidate` arm. For build and review it constructs
the `accepted-context` arm. The caller's request never substitutes for branch,
HEAD, lifecycle phase, or resolved target identity.

Argument parsing admits the optional flag without reading it before adoption
is known. When the constitution is absent, an invocation without the flag
retains the exact legacy grammar, output, and behavior. Supplying the new flag
without adoption is operational misuse, not an ignored input. Once the
constitution exists, the flag is mandatory; an absent, unreadable, malformed,
noncanonical, mismatched, or symlink-resolved request is operational exit 2
before the first effect. This is one shared loader/adapter used by every
lifecycle consumer, not four decoders or four request grammars.

The provider evaluates before the command's first effect:

1. Design-branch `verdi gate` builds the acceptance-candidate request and adds
   one numbered `constitutional conflict verdict` condition to the spec-MR
   condition set.
2. `verdi build start` builds an accepted build request after accepted-state,
   cascade, and obligation-quality validation but before resolving HEAD for a
   new branch, cutting the branch, or regenerating a baseline.
3. Build-branch `verdi gate` builds the accepted build request and adds the
   report as its conflict condition beside the existing evidence fold. It does
   not change evidence semantics.
4. `verdi close`, `verdi close --preflight`, and `verdi close --prepare` build
   the accepted review request before branch creation, alignment-report
   refresh, archive movement, report freezing, staging, commit, or publication.
   All three modes evaluate the same conflict result.

The later sealed-execution adapter consumes the same provider and accepted
request. It does not gain a weaker verdict or a second evaluator.

Consumers render only overall state, report digest, closed reason codes, and
witness IDs. The complete report remains available from `context conflict`.
A consumer cannot suppress a reason or translate `blocked-unproven` into pass.
Provider operational failure stays exit 2; either blocking verdict stays exit
1 before mutation.

Capability adoption is prospective. Only absence of the `.verdi/policy/`
directory — `policyauthority.ErrNotAdopted` — returns a typed `not-adopted`
result before requiring conflict request operands. A present policy directory
whose `constitution.md` is absent is incomplete adoption and fails
operationally, as do malformed or symlinked policy stores. Explicit
`verdi context conflict` reports that refusal at exit 1, matching explicit
context compilation. Existing design gate, build start, build gate, and close
retain their exact pre-adoption behavior and output; they do not add a
synthetic passing condition. Once constitution adoption is present, every
integration above is mandatory.

Missing judge or principal infrastructure blocks only when the actual
evaluation needs semantic judgment, an exemption, or a disposition. Current
accepted specifications contribute problem/outcome prose, so an adopted
repository's local lifecycle commands reach semantic evaluation and cannot
pass without authenticated disposition approvals; this unit supplies no local
identity fact reader. Managed callers with kernel-authenticated principal facts
can pass, and pre-adoption local behavior remains unchanged under the preceding
compatibility rule.
No existing accepted, frozen, archived, alignment, deviation, evidence,
waiver, or conflict artifact is rewritten or reinterpreted.

## 12. Failure taxonomy and threat model

| State | Classification |
|---|---|
| Identical complete declared inputs, including date and reused judge records | Byte-identical report and digest |
| Policy directory absent (`ErrNotAdopted`), explicit conflict invocation | Exit-1 typed refusal |
| Policy directory absent (`ErrNotAdopted`), existing lifecycle consumer | Exact legacy behavior |
| Policy directory present but constitution missing, malformed, or symlinked | Exit-2 operational failure |
| Proven disjoint scopes or satisfiable mechanical group | Proven no-conflict row |
| Unsatisfiable overlapping group without effective exemption | Blocked-violated |
| Unsatisfiable higher-order group whose conflict scope is not proven | Blocking-unproven semantic evaluation |
| Unknown ref/scope relation | Semantic evaluation plus blocked-unproven until disposition |
| Judge transport not configured when semantic evaluation is required | Blocking-unproven |
| Well-formed inconclusive judge result | Blocking-unproven |
| Configured judge start failure, nonzero exit, timeout, cancellation, malformed result, or invalid witness | Exit 2 |
| Primary/challenger disagreement | Blocking-unproven until disposition |
| Favorable judge output without current authorized disposition | Blocking-unproven |
| Stale/mismatched/expired/review-unproven/unauthorized authority artifact | Blocking-unproven, with original conflict preserved |
| Experimental profile on authoritative consumer | Blocking-unproven |
| Malformed request, policy, disposition, report, or cache | Exit 2 |
| Sealed operand mismatch or mutation | Exit 2 |
| Symlink at managed disposition/cache/output path | Exit 2 |
| Cache key/content collision | Exit 2 |
| Cancellation or I/O failure | Exit 2 |

Repository and human artifact bytes, judge output, cache bytes, and caller
claims are untrusted until strict validation. The threat model includes path
substitution through symlinks, stale exact witnesses, alternate JSON/YAML
encodings, forged principal strings, malicious judge witness references,
cache collision/corruption, wrong-checkout requests, and authority changes
between candidate construction and evaluation. The single-snapshot sealed
operand boundary and canonical path checks close those paths.

The threat model does not claim deterministic model behavior, inspect vendor
runtime instructions, authenticate a local username, prove external review
conditions in v1, or make a report digest prove isolation or actor identity.
Those limitations are recorded, never silently upgraded.

## 13. Implementation ownership and verification

The runtime unit may create `internal/policyconflict`, extend
`internal/contextcompile` with sealed operands/candidate normalization, add the
disposition kind to `internal/policyartifact`, `internal/policyauthority`,
`internal/humanartifact`, and `internal/store`, replace the placeholder
identity-claim operand validation described in §5.3, extend the `context` CLI
namespace, and add thin provider calls to the four named lifecycle boundaries.
It must not add UI, MCP, receipt, sealed-execution, forge-network, or generic
policy-language behavior.

Implementation is TDD and proves at least:

1. strict canonical request, report, judge-record, and disposition codecs,
   including unknown, duplicate, null, trailing, UTF-8, enum, union, and
   re-encode negatives;
2. mutation-after-seal and cross-snapshot operand rejection;
3. exact phase/environment/path/ref pair and N-way scope truth tables,
   including non-transitive overlap, segment boundaries, unknown artifact
   relationships, and conservative higher-order refusal;
4. complete operator-pair and multi-claim satisfiability tables for all four
   operand domains, including `not-equals` membership exclusion, symmetric
   identity-role operands, kernel delegation, and mixed-domain rejection;
5. pre- and post-exemption proof with stale, partial-scope, expired,
   review-unproven, and unauthorized cases;
6. normalized prose-source coverage and deterministic prompt/input ratchets;
7. whole-input judge recommendation and finding-witness validation,
   independently-run primary/challenger agreement/disagreement, adapter-owned
   model identity, and the rule that model output never passes by itself;
8. cache hit, miss, changed-key, corruption, symlink, collision, cancellation,
   and concurrent-writer behavior;
9. disposition exact match, stale identity, fallback eligibility, conflict and
   no-conflict conclusions, profile authorization, and local unproven identity;
10. repeated/permuted evaluation byte determinism and report digest ratchets;
11. fixture-Git proposed-candidate versus accepted-context behavior;
12. constitution-absence compatibility at each consumer;
13. pre-effect refusal at design gate, build start, build gate, and close;
14. built-binary exit 0/1/2, stdin, stdout, output-fence, and redacted
    diagnostic behavior; and
15. clean `go test -race ./...`, `make fixture`, `make spec-align`, and
    `make verify` output.

Tests use hermetic fake judge processes, fixture Git repositories, injected
clocks, and principal fact readers. Review-condition-only records are covered
as explicit v1 unproven cases. No test uses a network endpoint. This unit has
no browser behavior and adds no Playwright case.

## 14. Source coverage and losslessness

Coverage is **14/14** implicated authority source groups. The mapping is
lossless; each row names the complete clause range used rather than assigning
inconsistent per-clause counts across differently structured sources:

| Source group | Count | Destination |
|---|---:|---|
| Context Integrity AC-3: shared verdict; mechanical layer; six families; eleven operators; scope trivalence; semantic exchange; challenger; resolution; fallback; staleness/reuse | 1 | §§1–12 |
| Context Integrity decisions DC-3–DC-8, DC-15, DC-17–DC-24 | 1 | §§1–13 |
| Context Integrity constraints CO-1–CO-6 | 1 | §§2–13 |
| Context-compiler decisions SI-78–SI-92 that bind invocation, wire, scope, authority, operands, payload, repository, and candidate sources | 1 | §§2–7, 10–13 |
| Merge-signaled acceptance and current acceptance/build/evidence/closure call sites | 1 | §§2–3, 11 |
| Owner rulings OD-2, OD-5, OD-10, OD-12 as one custody/recording cluster | 1 | §§1, 8–9, 13 |
| Store-layout policy/data custody and human-artifact renderer/kernel | 1 | §§7–9, 13 |
| Wave-3 orchestration unit, exit gate, review cadence, and delivery exclusions | 1 | §§1, 11, 13 |
| Owner-approved lifecycle operand acquisition and current `gate`/`build start`/`close` grammars, including the effectful `close --prepare` path | 1 | §11, SI-102 |
| Closed report reason/disclosure labels required by §10 but previously unnamed | 1 | §10, SI-103 |
| Composite mechanical claim identity and exact removal witness | 1 | §§3, 5.5, 10; SI-105; plan Task 6A |
| Principal-relation kernel evidence, role-membership ownership, experimental posture, and disclosure translation | 1 | §§5.3, 9–10; SI-106; plan Task 6A |
| Proven-disjoint satisfiable-component completion | 1 | §§4.4–5; SI-107; plan Tasks 6, 6A |
| Lossless solo-collapse principal/role witness encoding | 1 | §§5.3, 10; SI-108; plan Task 6A |

The transformations are explicit:

- the context compiler's accepted-only request becomes one arm of a conflict
  request because pre-merge acceptance cannot honestly claim accepted state;
- the compiler's applicability comparison remains selection-only while this
  unit adds proof-grade pair and N-way scope algebra without treating overlap
  as transitive;
- existing policy claims become sealed runtime operands, not reconstructed
  manifest summaries;
- the legacy align process command is reused only as transport while one
  whole-input recommendation, independently-run challenge, identity,
  staleness, and disposition semantics are new and isolated;
- a judge retry becomes an immutable disposable machine record inside the
  existing D4 cache zone, while human authority remains a committed
  disposition;
- existing claim-level exemptions are applied by exact scoped removal and
  solver recomputation rather than treated as generic conflict waivers; and
- the spec's one verdict becomes four current thin consumers and one later
  sealed consumer, never five implementations; and
- lifecycle request operands reuse the existing strict context request document
  through one shared adapter instead of adding manifest defaults, selecting an
  arbitrary constitution adapter, or widening a missing environment/scope.

Intentional omissions are:

- no sealed launch, isolation receipt, capability enforcement proof,
  expansion, or resume;
- no builder/reviewer receipt authentication or managed-runner trust;
- no production identity fact reader or review-condition resolver;
- no automatic disposition authoring, agent-authored human judgment, or
  authority mutation;
- no legacy report migration or reinterpretation;
- no nonempty semantic-disposition enumeration in
  `verdi.context-manifest/v1`: the manifest retains SI-80's explicit empty
  field, while its authority revision binds the disposition-bearing
  effective-policy digest; enumeration requires a later recorded manifest
  version;
- no MCP or browser/workbench surface; and
- no layout-version bump, new data-zone root, new GC mode, or second writer
  authority in the canonical store-layout amendment.

No source clause, owner decision, proof effect, public interface, storage
effect, lifecycle consumer, threat boundary, deferral, or residual disclosure
is silently omitted.

### Store-layout promotion witness

Coverage is **2/2** accepted storage effects, with no omitted source element:

1. §7 maps to the existing `data/cache/` directory row, D4's exact
   `policy-conflict-<layout-version>-<tree-hash>-<input-digest>.json` grammar,
   and the existing GC cache-pruning bullet. Both hash path segments are bare
   64-lowercase-hex filename values while record fields retain canonical
   `sha256:<hex>` digests; no new cache root or GC mode is created.
2. §8 maps to the committed `policy/dispositions/<name>.md` row, the corrected
   GLG human-record location, policy-authority loading/digest ownership, the
   conflict gate's sole interpretation ownership, and authored-living temporal
   classification. Legacy spec-frontmatter dispositions are intentionally
   preserved as historical content but cannot satisfy the new schema.

The transformation is additive except for correcting the obsolete
spec-frontmatter-location statement. It changes no existing artifact bytes,
index-cache semantics, writer authority, layout schema version, or historical
record interpretation.
