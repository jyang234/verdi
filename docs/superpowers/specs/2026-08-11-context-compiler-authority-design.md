# Context Compiler Authority Design

**Status:** proposed authority; effective only after owner merge

**Planning base:** `ab9186bb44759cbd8f6d686a4fe9813f0a5517ff`

**Owners:** platform-team

**Consumers:** policy-conflict-gate and sealed execution; `context compile`
is the first direct inspection surface

## 1. Decision

The Context Integrity Wave-3 compiler is one deterministic, read-only
composition service. It loads the adopted constitution through
`policyauthority`, computes repository facts from the checkout, resolves the
accepted specification and its governed dependencies, discovers a closed
candidate universe, classifies every candidate exactly once, evaluates narrow
phase applicability, renders the applicable instruction projection through the
one projection renderer, and returns:

1. canonical `verdi.context-manifest/v1` bytes;
2. canonical `verdi.context-data-item/v1` payload items for every included
   non-authoritative data input; and
3. the generated projection files that alone occupy the authority channel.

The result is in memory. `verdi context compile` writes the manifest to stdout
or one caller-selected `--out` file, but this increment creates no store path,
payload tree, receipt, or execution workspace. The later sealed-execution unit
consumes the same service result in process; it does not rediscover or
reclassify context.

This unit is a compiler, not the conflict gate or the sealed runner. A
successful local compilation is advisory evidence that the declared inputs
were composed deterministically. It does not prove policy conflicts clean,
capabilities enforced, a model obeyed instructions, actors authenticated, or a
review capsule complete.

## 2. Invocation and result boundary

The public CLI surface is:

```text
verdi context compile --request <path|-> [--out <path>]
```

`context` is registered at verb phase 23. The request is exact canonical JSON
and carries no actor, constitution, policy, repository-fact, or evidence
claims. Those facts are resolved by trusted in-process ports. The CLI supplies
no principal-resolution port in v1, so its actor posture is always explicitly
unproven.

There is no MCP tool in v1. A future MCP surface must reuse the same service
and receive its own serialized-registry review.

Exit classes are:

| Exit | Meaning |
|---|---|
| 0 | Compilation completed. The manifest may remain advisory and may contain explicit unproven or unknown facts. |
| 1 | Typed state refusal: no adopted constitution, adapter mismatch, accepted-spec or expected-repository mismatch, declared phase/scope refusal, or instruction-projection drift. |
| 2 | Malformed or noncanonical input, invalid authority, I/O failure, Git failure that prevents required classification, or another operational failure. |

Invoking the compiler against a legacy store with no constitution returns the
exit-1 refusal. This does not change that store's existing lifecycle or make
other verbs constitution-dependent.

## 3. Exact request grammar

The request schema is `verdi.context-compile-request/v1`:

```json
{
  "adapter": {"id": "codex", "version": "1"},
  "expected": {"branch": "main", "head": "0123456789abcdef0123456789abcdef01234567"},
  "grants": {"grants": [], "schema": "verdi.execution-grants/v1"},
  "phase": "build",
  "schema": "verdi.context-compile-request/v1",
  "scope": {"environments": [], "paths": [], "phases": ["build"], "refs": []},
  "spec": "spec/example-story"
}
```

The fields are exact:

- `phase` is `design`, `build`, or `review` and must occur in
  `scope.phases` unless that dimension is the explicit universal `[]`;
- `adapter` must byte-exactly match one `(id, version)` row in the resolved
  constitution;
- `spec` is one unpinned whole `spec/<name>` ref. The compiler resolves its
  accepted merge-signaled baseline and exact bytes; a caller cannot select a
  commit or provide replacement bytes;
- `expected` is optional as a whole. When present, both `branch` and `head`
  are mandatory and must match computed checkout facts. It is omitted, never
  `null`, when unused;
- `scope` is the existing `policyartifact.Scope` grammar: all four dimensions
  are present, members are unique, and an explicit empty dimension is
  universal;
- `grants` is the complete existing `verdi.execution-grants/v1` document and
  is decoded by the one `execworkspace` grant seam. The compiler records it
  but never applies it.

Unknown fields, duplicate keys, trailing data, explicit `null`, invalid UTF-8,
unknown enums, and any byte form that differs from canonical re-encoding fail
closed. Canonical JSON means sorted keys, no HTML escaping, and one trailing
newline.

## 4. Repository facts and trusted inputs

Repository identity, branch, HEAD, configured default branch and head, their
relationship, dirty/staged posture, and managed-worktree identity are computed
once in a new shared `internal/repositoryfacts` leaf package. Journey and the
context compiler consume that package. Context Integrity never imports the
journey projection, and the two consumers cannot silently diverge in the fact
shape or canonical remote-identity rule.

The compiler additionally receives trusted in-process ports for:

- the current checkout and Git object reads;
- `policyauthority.Load` and `policyauthority.Resolve` results;
- accepted-baseline resolution through `specstate`;
- optional sealed `governanceprincipal.PrincipalResolution` values; and
- pure instruction-projection rendering plus read-only drift verification.

No request field can stand in for one of those results. Principal resolutions
are accepted only as already-sealed kernel values from the injected port. The
manifest records a canonical projection of their identity, state, role and
trust witness; it does not turn that projection back into kernel authority.
When no resolution exists, `actors.posture` is `unproven`, `resolutions` is
empty, and disclosure `actor-resolution-unproven` is mandatory.

## 5. Candidate universe and total classification

Candidate identity is `(source, logical-id)`, so the same repository path may
have a committed candidate and a distinct worktree-overlay candidate. The
universe is the deterministic union of:

| Source | Candidates |
|---|---|
| `head-tree` | Every Git-tracked leaf at the computed HEAD that is not lifted into `store-authority` or `declared-context`, addressed by repo-relative path and blob object ID. Managed generated outputs remain explicit head-tree candidates so their exclusion is visible. |
| `worktree-overlay` | Every staged, modified, deleted, or untracked path relative to HEAD, recorded without reading or digesting uncommitted content. |
| `store-authority` | Resolved constitution, profile, applicable policies/overlays/exemptions, accepted spec, parent feature fragments, and obligations. Decisions are indexed in the manifest from their containing accepted-spec or fragment payload; a tracked path lifted here is not duplicated as a repository-file candidate. |
| `declared-context` | Every exact `context:` ref declared by the target feature and resolved through the artifact/store seams. A tracked path lifted here is not duplicated as a repository-file candidate. |
| `projection` | Every file returned by the pure renderer for the selected adapter. |
| `opaque` | One fixed adapter-owned `harness-vendor-base` candidate. |

`.git/` is repository metadata and never enters the candidate universe.
`.verdi/data/` is represented by one excluded subtree-boundary candidate; its
descendants are neither enumerated nor named. This prevents disposable,
potentially secret paths from leaking into a proof that already states the
whole subtree was outside inspection.

Tracked files under `testdata/`, examples, experiments, and other ordinary
directories remain candidates. No directory is excluded merely because its
name resembles a fixture or service convention. Untracked dependency caches
are covered by the worktree-overlay rule and therefore excluded without their
bytes being read.

Every candidate appears in exactly one of `included`, `excluded`, or `opaque`.
The three ledgers are sorted by source then logical ID and have unique IDs.
Source assignment precedence is `store-authority`, then `declared-context`, then
`head-tree`; the same path cannot enter two of those three sources. A generated
managed output deliberately has one excluded `head-tree` candidate and one
included `projection` candidate because the former is checkout state and the
latter is the freshly compiled authority payload.
Generated projection outputs are excluded from the repository-data view with
`generated-projection-output` and included once through the `projection`
source. The ASD sidecar is always excluded with the exact reason
`design-provenance-sidecar`; its JSONL chain is not inspected by this unit.

Logical IDs are canonical and source-independent: repository-backed entries
use `path:<repo-relative-path>`, artifact-backed entries use
`ref:<canonical-ref>`, and the opaque base uses
`opaque:harness-vendor-base/<adapter-id>/<adapter-version>`. Source remains a
separate field, so `head-tree/path:x` and `worktree-overlay/path:x` are distinct
candidates without inventing two spellings for path `x`.

### 5.1 Closed included kinds

The v1 included-kind enum is:

```text
accepted-spec
parent-feature-fragment
obligation
policy-artifact
repository-file
declared-context-ref
instruction-projection
```

### 5.2 Closed exclusion reasons

The v1 exclusion-reason enum is:

```text
design-provenance-sidecar
data-zone-disposable
uncommitted-content
out-of-declared-scope
phase-inapplicable
superseded-spec
archived-record
generated-projection-output
non-text-data
non-regular-file
```

An unknown kind or reason fails decode. `service-noise`, blanket `testdata`,
and blanket experiment-tree exclusions do not exist.

## 6. Applicability and phase capsules

Applicability has the closed values `applicable`, `inapplicable`, and
`unknown`. It is computed per dimension and then conjoined:

- an explicit empty dimension is universal;
- phase, environment, and ref members compare by exact equality;
- a path without a trailing slash matches only that exact path;
- a path ending in `/` matches descendants on segment boundaries and never a
  same-prefix sibling;
- a candidate is applicable when every dimension intersects;
- a known empty intersection is inapplicable; and
- an unavailable or unresolvable comparison is unknown.

Unknown never silently excludes. The candidate is included, its applicability
is marked `unknown`, and a disclosure names the missing operand. Proof-grade
scope subset/overlap conflict semantics remain owned by the later
policy-conflict-gate.

The phase policies are:

### Design

The design capsule is broad and advisory: accepted target spec, its declared
context refs, related feature fragments, obligations, applicable constitution,
scoped tracked repository material, projection, grants, and the fixed opaque
base. Scope still bounds repository material; "broad" is not permission to
ignore declared scope.

### Build

The build capsule requires an accepted story or spike. A non-spike story may
carry multiple `implements` edges; each target feature appears in sorted
`parent_features`. Its fragment contains the targeted ACs plus that feature's
problem, outcome, all constraints, and all decisions. A spike uses each
`resolves` target open-question fragment plus the same governing feature
problem, outcome, constraints, and decisions. No single-parent restriction is
invented. Other ACs/open questions from those features are excluded as
`out-of-declared-scope`.

The capsule also includes all obligations bound to the target pairs, the
applicable constitution, approved grants, scoped repository material, and the
generated projection.

### Review

The review capsule names the accepted spec, result diff, evidence bundle,
builder receipt, and review policy as `required_inputs`. Wave 3 can prove the
accepted spec and applicable review policy, but it has no ratified request or
receipt grammar for the other three. They are therefore emitted as
`unproven`, with their fixed disclosure codes, and the compile remains advisory
and exits 0. No unsigned placeholder is consumed as a report or receipt. The
Wave-4 owner may widen the request and proof grammar through a later ledgered
schema revision. `required_inputs` is `[]` for design and build; for review it
contains exactly those five rows in the closed grammar of §8.2.

## 7. Instruction projection and channel separation

Only generated instruction-projection bytes occupy payload channel
`authority`. Accepted specs, policy artifacts, obligations, repository files,
and declared context remain semantically governed inputs, but their bytes are
wrapped as channel `data`. Imperative prose inside them therefore cannot become
adapter instructions by retrieval or classification.

`internal/instructionprojection` gains a pure rendering entry point that
accepts the already-resolved policy store, the sealed effective policy, the
selected adapter, and the compiler-filtered applicable policy entries. It
returns the same file bytes and manifest facts that `Generate` and `Verify`
use, without writing. `Generate` delegates to that pure seam before its
existing writes; `Verify` delegates before its existing read-only comparisons.
There is one renderer and one projection-manifest builder.

Filtering is phase-sensitive. The compiler's one applicability engine selects
the policy entries whose scopes apply to the request, then the renderer formats
that selected view. The renderer does not independently interpret scope, and
the compiler does not duplicate projection formatting. Existing managed files
are verified against the same phase-neutral stored projection before compile;
any drift is an exit-1 refusal. The phase-filtered compile result exists only in
memory and cannot overwrite the managed projection files.

## 8. Payload and manifest grammar

### 8.1 Data item

Each included non-authority payload is canonical
`verdi.context-data-item/v1`:

| Field | Rule |
|---|---|
| `schema` | exactly `verdi.context-data-item/v1` |
| `id` | stable logical candidate ID |
| `source` | one of the six source values in §5 |
| `kind` | one of the included kinds in §5.1 except `instruction-projection` |
| `path` | optional canonical repo-relative path; omitted when inapplicable |
| `ref` | optional canonical artifact ref; omitted when inapplicable |
| `classification` | exactly `non-authoritative-data` |
| `content_digest` | SHA-256 of the exact bytes carried in `content` |
| `content` | exact valid-UTF-8 source text, or the canonical fragment bytes defined below |
| `digest` | SHA-256 of the digestless canonical data-item object |

Binary or invalid-UTF-8 tracked content is not copied into a payload and is
classified `excluded/non-text-data`. Secrets are never read from uncommitted
content or `.verdi/data/`. A tracked symlink or Gitlink is never followed and
is classified `excluded/non-regular-file`.

A `parent-feature-fragment` item's `content` is canonical JSON, including its
trailing newline, for one nested object with exactly `feature`, `problem`,
`outcome`, `targets`, `constraints`, and `decisions`. `feature` carries the
whole feature ref, repository path and exact source-file digest. `problem` and
`outcome` carry their complete normalized `{text,anchor}` objects. `targets`
carries the complete normalized AC objects named by `implements`, or the
complete normalized open-question objects named by `resolves`, sorted by
canonical fragment ref. `constraints` and `decisions` carry every complete
normalized object of those kinds in declaration order. No untargeted AC,
untargeted open question, stub, unrelated link, or free body prose enters this
fragment payload.

### 8.2 Manifest

`verdi.context-manifest/v1` has these mandatory top-level sections, in addition
to its self digest:

```text
schema, phase, adapter, revisions, accepted_spec, parent_features, decisions,
obligations, repository, policy, dispositions, owners, scope,
governance_profile, actors, included, excluded, opaque, capabilities,
projection_files, required_inputs, evidence, disclosures, expansions, digest
```

Empty collections are explicit `[]`; optional scalar/object fields are omitted,
never `null`. `dispositions` and `expansions` are empty in v1, and strict decode
rejects either when nonempty.

The section shapes are exact:

| Section | Shape and invariant |
|---|---|
| `schema` | exactly `verdi.context-manifest/v1` |
| `phase` | exactly the normalized request phase |
| `adapter` | `{id,version}` copied from the exact constitution row, never a free-standing caller claim |
| `revisions` | `{authority,context,parent?}`; authority is a digest, context is `1`, and parent is omitted in v1 |
| `accepted_spec` | `{ref,path,blob,commit,content_digest}` using the merge-signaled baseline and exact evaluated bytes |
| `parent_features` | sorted `[{ref,path,source_digest,fragment_digest,payload_digest}]`; source digest binds the feature file, fragment digest binds §8.1's canonical selected object, and payload digest binds its data-item wrapper |
| `decisions` | sorted `[{ref,content_digest}]` for every governing decision already carried in accepted-spec or parent-fragment payloads; digest is over the complete normalized decision object |
| `obligations` | sorted `[{ref,path,ac,kind,content_digest}]`; `ac` and `kind` are the exact declared pair |
| `repository` | shared repository facts: known/value remote, branch and HEAD; known name/ref/head default branch; relationship `equal\|ahead\|behind\|diverged\|unknown`; known/value dirty and staged; managed/name worktree; source `head\|working-tree\|remote-ref\|receipt-bound`; disclosures |
| `policy` | `{effective_digest,constitution_digest,profile_id,profile_digest,entries}`; entries are sorted `{kind,id,digest,applicability}` rows for policy, overlay and exemption operands |
| `dispositions` | `[]` exactly |
| `owners` | sorted unique strings from the target and governing parents; declared owners, never authenticated actors |
| `scope` | the normalized four-dimension `policyartifact.Scope` request value |
| `governance_profile` | `{id,class,digest}` from the selected sealed profile |
| `actors` | `{posture,resolutions,disclosures}`; posture is `proven`, `violated-with-witness`, or `unproven`; resolutions are sorted canonical exported `governanceprincipal.PrincipalResolution` fields and never include the private seal |
| `included` | sorted `[{id,source,kind,applicability,payload_channel,path?,ref?,content_digest,payload_digest,disclosures}]` |
| `excluded` | sorted `[{id,source,reason,applicability,path?,ref?,disclosures}]`; no content or content-digest field exists |
| `opaque` | sorted `[{id,kind,adapter,disclosures}]`; kind is `harness-vendor-base` and no digest is claimed |
| `capabilities` | the complete normalized existing `verdi.execution-grants/v1` document |
| `projection_files` | sorted `[{path,digest}]` for the selected adapter's pure-rendered files |
| `required_inputs` | sorted `[{kind,resolution,digest?,witnesses}]`; resolution is `proven`, `violated-with-witness`, or `unproven` |
| `evidence` | `{authority,freshness,consumed_reports,disclosures}` with the v1 restrictions below |
| `disclosures` | sorted unique closed disclosure codes used by the enclosing manifest |
| `expansions` | `[]` exactly |
| `digest` | SHA-256 of the digestless canonical manifest object |

Unless a row states declaration order, a set-like list sorts bytewise by the
identity fields shown in that row: refs by ref; candidates by `(source,id)`;
policy entries by `(kind,id)`; projection files by path; actors by
`(claim.trust_source,claim.subject)`; and required inputs by kind. Duplicate
identity tuples fail closed rather than collapse.

Every included entry names source, kind, identity, applicability, payload
channel, exact content digest, and the digest of its data item or projection
file. Excluded entries name source, identity, reason, and applicability but
never content bytes. Opaque entries name adapter, kind, identity and a fixed
disclosure, but claim no content digest.

`repository` uses the shared fact shape and carries disclosures for every
unknown fact. `policy` names the effective-policy identity/digest and every
applicable authority operand. `actors` and `required_inputs` use the closed
resolution values `proven`, `violated-with-witness`, and `unproven`; an
unproven or violated row must name a disclosure or witness.

The remaining closed vocabularies are:

- payload channel: `authority`, `data`;
- policy entry kind: `policy`, `overlay`, `exemption`;
- required-input kind: `accepted-spec`, `result-diff`, `evidence-bundle`,
  `builder-receipt`, `review-policy`;
- evidence authority: `advisory`; and
- disclosure code: `actor-resolution-unproven`,
  `repository-remote-unknown`, `repository-branch-unknown`,
  `repository-head-unknown`, `default-branch-unknown`,
  `default-relationship-unknown`, `dirty-state-unknown`,
  `staged-state-unknown`, `freshness-unknown`, `applicability-unknown`,
  `review-result-diff-unproven`, `review-evidence-bundle-unproven`,
  `review-builder-receipt-unproven`, and `opaque-harness-vendor-base`.

Unknown values fail decode. Actor posture is `violated-with-witness` when any
sealed resolution is violated, otherwise `unproven` when any is unproven or the
set is empty, otherwise `proven` when every resolution is authenticated.

`evidence` contains:

- `authority`: `advisory` for every v1 local compile;
- `freshness`: `fresh`, `stale`, or `unknown` against the computed HEAD;
- `consumed_reports`: `[]` in v1; specs, policies, obligations, projections,
  and data items are inputs, not reports; and
- disclosures for every unknown or unproven evidence fact.

Canonical outputs contain no wall clock, random value, username, absolute
path, conversation/session ID, or raw remote URL.

## 9. Digests and revisions

Every digest string is `sha256:` plus 64 lowercase hexadecimal characters.
Raw files use exact-byte SHA-256. Structured values use canonical-JSON bytes.

The authority revision is the digest of one private canonical preimage that
contains only:

1. effective-policy digest;
2. accepted spec ref, merge-signaled baseline identity, and exact-byte digest;
3. every parent feature fragment identity and exact-byte digest, sorted;
4. every decision identity and digest, sorted; and
5. every obligation identity and exact-byte digest, sorted.

Repository state, grants, actor posture, payload classification, projection
files and opaque disclosures are bound by the manifest self digest, not folded
into authority identity.

The root context revision is integer `1`; its parent is omitted. `expansions`
is `[]`. Nonempty expansions and a parent on revision 1 fail closed until
sealed execution records the item-level expansion grammar. The manifest digest
is computed over its digestless canonical form.

## 10. Failure taxonomy

| State | Result |
|---|---|
| Identical trusted inputs | Byte-identical manifest, data items, projections and digests |
| Legacy store without constitution | Exit-1 typed refusal; no global lifecycle change |
| Adapter id/version not registered | Exit-1 typed refusal naming expected rows |
| Expected branch/HEAD mismatch | Exit-1 typed refusal naming computed values |
| Accepted ref is not an accepted baseline | Exit-1 typed refusal with merge-signaled witness |
| Phase outside declared scope | Exit-1 typed refusal |
| Existing generated projection drift | Exit-1 typed refusal with closed projection reason |
| Applicability operand unavailable | Include as unknown with disclosure when a manifest remains computable |
| Required repository fact unavailable | Exit 2 if total classification cannot be computed; otherwise unknown with disclosure |
| Malformed/noncanonical request or authority | Exit 2 |
| Review receipt/evidence inputs absent in Wave 3 | Exit 0 advisory manifest with mandatory unproven rows |

No state silently upgrades advisory evidence or an unproven actor/input to an
authoritative fact.

## 11. Implementation ownership and tests

The later runtime unit may create `internal/contextcompile` and
`internal/repositoryfacts`, add the pure seam under
`internal/instructionprojection`, and register `cmd/verdi/context.go`. It may
make the minimum journey refactor required to consume `repositoryfacts`. It
must not modify `internal/execworkspace` grant or network semantics, create a
store path, add an MCP tool, write projection files during compile, implement
the conflict gate, or start a process.

The implementation is TDD and must prove at least:

1. strict canonical request, data-item and manifest codecs, including every
   unknown/duplicate/null/trailing/UTF-8/enum/union negative;
2. exact repository-fact parity between journey and compiler;
3. total candidate classification, distinct committed/overlay identities,
   `.verdi/data/` subtree privacy, and no blanket fixture exclusion;
4. exact path-boundary and three-valued applicability tables;
5. multi-feature story and spike fragment composition;
6. phase-specific capsules and explicit Wave-3 review-input disclosures;
7. only projection bytes in the authority channel and every other payload in
   the provenance-wrapped data schema;
8. one pure renderer used by compile, generate and verify, with no compile-time
   write and a drift refusal;
9. byte-identical repeated compilation and digest ratchets;
10. built-binary exit 0/1/2 behavior against hermetic adopted and legacy
    stores; and
11. clean `go test -race ./...`, `make fixture`, `make spec-align`, and
    `make verify` output.

No test uses a network endpoint. Git and repository-state tests use fixturegit
or other hermetic repositories.

## 12. Source coverage and losslessness

Coverage is **51/51** implicated authority elements from the read-only Wave-3
audit. The mapping is lossless by group:

| Source group | Count | Destination |
|---|---:|---|
| Audit items 1–20: AC-2 inputs, compiler operations, manifest fields, phase capsules, revisions and expansion ledger | 20 | §§1–3, 5–9 |
| Audit items 21–33: DC-1; DC-2; DC-15; DC-16; DC-18; DC-19; DC-20–DC-24 as one custody cluster; and CO-1, CO-2, CO-3, CO-4, CO-5, CO-6 | 13 | §§1–11 |
| Audit items 34–41: OD-8 and SI-25, SI-28, SI-33, SI-35/SI-38, SI-60, SI-61 and SI-67–SI-70 | 8 | §§3–11 |
| Audit items 42–46: orchestration provenance, merge-signaled acceptance, store layout, artifact refs/fragments and evidence/obligations | 5 | §§4–9, 11 |
| Audit items 47–51: later conflict, execution, receipt and workbench units plus the forbidden journey dependency | 5 | §§1, 4, 6, 10–11 |

Transformations are explicit: the audit's singular parent becomes the artifact
contract's legal sorted multi-parent set; its duplicated journey fact assembly
becomes one shared leaf package; its manifest-only result becomes the delivery
sequence's manifest-and-payload result; its phase-invariant projection becomes
one renderer over a compiler-filtered applicable view; and its silent optional
actors/review inputs become typed disclosed-unproven facts.

Intentional omissions are:

- no proof-grade scope-conflict verdict, disposition authoring, or exemption
  judgment (policy-conflict-gate);
- no payload persistence, process start, isolation proof, capability
  enforcement, expansion, or resume (sealed execution);
- no result-diff, evidence-bundle or builder-receipt wire invented before its
  owning Wave-4 unit;
- no receipt authentication or independent review (`context-receipts-review`);
- no workbench or browser surface (Wave 6); and
- no mutation of accepted specs, frozen artifacts, policy authority, generated
  projections, or the store.

No source clause, alternative, proof effect, public-interface decision,
deferral, or disclosed residual is silently omitted.
