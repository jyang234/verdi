# Context Compiler Wave 3 Implementation Plan

> **For the implementing agent:** use `/fable-orchestration` and execute this
> plan task by task with Sonnet implementation workers, TDD, small commits, and
> a fresh Opus review/fix pass. Do not reinterpret authority inside an
> implementation worker; return every ambiguity to the main Codex agent.

**Goal:** implement the deterministic, read-only Wave-3 context compiler and
the built-binary `verdi context compile` inspection surface fixed by
`docs/superpowers/specs/2026-08-11-context-compiler-authority-design.md` and
ledger SI-78 through SI-87.

**Architecture:** a new `internal/contextcompile` package owns strict request,
manifest, payload, applicability, universe, capsule, and compiler behavior. It
consumes existing sealed policy/spec/principal authority through narrow ports,
uses one extracted `internal/repositoryfacts` leaf shared with journey, and
uses a new pure `internal/instructionprojection.Render` seam shared by
Generate and Verify. The compiler returns canonical bytes in memory; only the
CLI may write the caller-selected manifest file. It never persists payloads,
starts a process, applies grants, or emits a receipt.

**Tech stack:** Go, `internal/artifact` exact JSON, `internal/canonjson`,
`internal/gitx`, `internal/specstate`, `internal/policyauthority`,
`internal/policyartifact`, `internal/governanceprincipal`,
`internal/instructionprojection`, table-driven unit tests, fixturegit-style
integration tests, and built-binary Go e2e tests.

## Binding inputs and constraints

- Start only from an exact `origin/main` that contains this plan, the authority
  design, and SI-78 through SI-87. Record that base SHA before editing.
- Read `AGENTS.md`, `CLAUDE.md`, the authority design, SI-78..SI-87, and the
  Context Integrity source spec before the first test.
- Do not edit frozen `.verdi/` artifacts or `docs/design/specs/`. Do not add a
  schema, lifecycle, proof, receipt, persistence, MCP, frontend, or execution
  surface beyond the authority design.
- `context compile` is read-only except for its explicit `--out` destination.
  It never writes `.verdi/`, managed projection files, payload files, Git state,
  or a worktree.
- Never enumerate `.verdi/data/` descendants. Never read or digest staged,
  modified, deleted, or untracked content. Worktree-overlay candidates contain
  path identity and exclusion facts only.
- Never follow symlinks or Gitlinks. Never import `internal/journey` from the
  compiler. Never duplicate policy scope interpretation or projection
  formatting.
- Keep error classes explicit: typed state refusal maps to exit 1; malformed or
  operational error maps to exit 2. Advisory/unproven facts in a completed
  manifest still map to exit 0.
- Every new enum is closed and every set-like slice is sorted and duplicate
  rejected. Optional fields are omitted, never serialized as `null`; mandatory
  empty collections serialize as `[]`.
- Commit after each task with the imperative subject named below and the
  repository-required attribution footer. Run `git diff --check` before every
  commit.

## Task 1: Define strict context wire contracts

**Files:**

- Create: `internal/contextcompile/schema.go`
- Create: `internal/contextcompile/codec.go`
- Create: `internal/contextcompile/validate.go`
- Create: `internal/contextcompile/schema_test.go`
- Create: `internal/contextcompile/testdata/request-build.json`
- Create: `internal/contextcompile/testdata/manifest-build.json`
- Create: `internal/contextcompile/testdata/data-item.json`

### Step 1: Write failing schema and codec tests

Define table-driven tests that require:

- exact canonical request decoding for
  `verdi.context-compile-request/v1`;
- exact canonical manifest and data-item encoding/decoding;
- rejection of unknown fields, duplicate keys, trailing data, invalid UTF-8,
  noncanonical whitespace/key order/missing final newline, explicit null, an
  unknown enum, duplicate identity rows, unsorted set-like rows, invalid
  digests, and nonempty v1 `parent`, `dispositions`, or `expansions`;
- explicit `[]` for every mandatory collection;
- a request phase that occurs in `scope.phases`, unless phases is `[]`;
- exact `spec/<name>` whole refs, an all-or-nothing optional expected
  branch/HEAD pair, and grant bytes accepted only through
  `execworkspace.DecodeGrantSet`;
- a data-item self digest computed over its digestless canonical form and a
  content digest computed over the exact bytes carried in `content`;
- a manifest self digest computed over its digestless canonical form.

The public domain types must include these closed enums:

```go
type Phase string // design, build, review
type Source string // head-tree, worktree-overlay, store-authority,
                   // declared-context, projection, opaque
type IncludedKind string // the seven §5.1 values
type ExclusionReason string // the ten §5.2 values
type Applicability string // applicable, inapplicable, unknown
type Resolution string // proven, violated-with-witness, unproven
type PayloadChannel string // data, authority
```

Use `policyartifact.Scope` and `execworkspace.GrantSet` as decoded domain
fields. Use wire-only private structs where pointer fields are needed to
distinguish omitted from explicit empty/null values.

Run the tests and capture the expected compile failure:

```bash
go test ./internal/contextcompile -run 'Test(Request|Manifest|DataItem)' -count=1
```

### Step 2: Implement the strict codecs and validators

Expose only these constructors/codecs initially:

```go
const RequestSchema = "verdi.context-compile-request/v1"
const ManifestSchema = "verdi.context-manifest/v1"
const DataItemSchema = "verdi.context-data-item/v1"

func DecodeRequest(data []byte) (Request, error)
func EncodeRequest(request Request) ([]byte, error)
func DecodeManifest(data []byte) (Manifest, error)
func EncodeManifest(manifest Manifest) ([]byte, error)
func DecodeDataItem(data []byte) (DataItem, error)
func EncodeDataItem(item DataItem) ([]byte, error)
```

`DecodeRequest` must first use `artifact.DecodeExactJSON`, reject absent/null
mandatory members through pointer-backed wire types, decode the nested grants
by re-encoding that exact nested document canonically and calling
`execworkspace.DecodeGrantSet`, validate the domain object, canonicalize it,
and require byte equality with the original input. Manifest and data-item
decode follow the same exact-byte gate.

Do not hand-roll JSON sorting. Use `canonjson.Marshal`. Do not expose a
constructor that permits a caller-supplied self digest; encoding recomputes it
from a digestless copy and validation checks it.

### Step 3: Ratchet canonical fixtures and run focused race tests

```bash
go test -race ./internal/contextcompile -run 'Test(Request|Manifest|DataItem)' -count=1
git diff --check
git status --short
```

Commit:

```text
Define context compiler wire contracts
```

## Task 2: Extract one shared repository-facts leaf

**Files:**

- Create: `internal/repositoryfacts/facts.go`
- Create: `internal/repositoryfacts/port.go`
- Create: `internal/repositoryfacts/gather.go`
- Create: `internal/repositoryfacts/gather_test.go`
- Modify: `internal/journey/record.go`
- Modify: `internal/journey/port.go`
- Modify: `internal/journey/facts.go`
- Modify: `internal/journey/facts_test.go`
- Modify: `internal/journey/record_test.go`
- Modify: journey canonical golden fixtures only if their bytes change

### Step 1: Pin byte-compatible shared fact types

Write tests proving that the shared fact shape canonically encodes exactly like
the current journey repository section and validates the same known/value
invariants. Define:

```go
type StringFact struct { Known bool; Value string }
type BoolFact struct { Known bool; Value bool }
type DefaultBranchFact struct { Known bool; Name, Ref, Head string }
type WorktreeFact struct { Managed bool; Name string }

type Source string
const (
    SourceHead Source = "head"
    SourceWorkingTree Source = "working-tree"
    SourceRemoteRef Source = "remote-ref"
    SourceReceiptBound Source = "receipt-bound"
)

type Facts struct {
    RemoteOrigin StringFact
    Branch StringFact
    Head StringFact
    DefaultBranch DefaultBranchFact
    Relationship string
    Dirty BoolFact
    Staged BoolFact
    Worktree WorktreeFact
    Source Source
}

type DisclosureCode string
type Snapshot struct {
    Facts Facts
    Disclosures []DisclosureCode
}
```

Use closed, machine-stable disclosure codes in the shared result. Preserve the
current journey prose by mapping those codes in journey; do not make prose the
compiler's protocol.

### Step 2: Move gathering behind a shared consumer-owned port

The package owns a read-only `GitReader` containing the current journey
methods and a `DefaultBranchResolver`. Provide:

```go
type GatherInput struct {
    Root string
    TargetPath string
    TargetContent []byte
    TargetFoundOnDisk bool
}

type Gatherer struct { /* unexported ports */ }
func NewGatherer() Gatherer
func (g Gatherer) Gather(ctx context.Context, in GatherInput) (Snapshot, error)
```

Port the current journey behavior, including credential-free
`gitx.CanonicalRemoteIdentity`, detached HEAD, unresolved default branch,
relationship, dirty/staged, worktree, and source derivation. Keep the zero
value fail closed.

### Step 3: Refactor journey onto the leaf without changing its wire

Use aliases where possible:

```go
type StringFact = repositoryfacts.StringFact
type BoolFact = repositoryfacts.BoolFact
type DefaultBranchFact = repositoryfacts.DefaultBranchFact
type WorktreeFact = repositoryfacts.WorktreeFact
type RepositoryFacts = repositoryfacts.Facts
```

Journey owns only its disclosure-text mapping and record integration.
`internal/contextcompile` must not be introduced yet. Existing journey golden
bytes should remain unchanged; if Go's aliasing forces a change, stop and
return it for adjudication instead of silently revving the journey schema.

### Step 4: Verify both consumers' future seam

```bash
go test -race ./internal/repositoryfacts ./internal/journey -count=1
go test -race ./cmd/verdi -run 'Test.*Journey' -count=1
git diff --check
```

Commit:

```text
Share canonical repository facts
```

## Task 3: Add exact Git candidate discovery

**Files:**

- Create: `internal/gitx/treeentries.go`
- Create: `internal/gitx/treeentries_test.go`
- Create: `internal/gitx/worktreechanges.go`
- Create: `internal/gitx/worktreechanges_test.go`
- Create: `internal/contextcompile/universe.go`
- Create: `internal/contextcompile/universe_test.go`
- Create: `internal/contextcompile/testdata/universe/` fixtures as needed

### Step 1: Add NUL-safe Git primitives with real hermetic repositories

Write failing tests for filenames containing whitespace, tabs, newlines, and
non-ASCII; regular executable/non-executable blobs; symlinks; Gitlinks;
staged changes; unstaged changes; deletions; untracked files; and both sides of
a rename.

Expose:

```go
type TreeEntry struct {
    Mode string
    Type string
    Object string
    Path string
}

func LsTreeEntries(ctx context.Context, dir, ref string) ([]TreeEntry, error)
func WorktreeChangedPaths(ctx context.Context, dir string) ([]string, error)
```

Implement `LsTreeEntries` with `git ls-tree -rz --full-tree <ref>` and parse the
mode/type/object/TAB/path grammar without line splitting. Implement
`WorktreeChangedPaths` with
`git status --porcelain=v1 -z --untracked-files=all`; return unique sorted
repository-root-relative paths and both rename identities. No function reads
path content.

### Step 2: Build the raw closed universe

Define the compiler-local candidate type:

```go
type Candidate struct {
    Source Source
    ID string
    Path string
    Ref string
    Object string
    Mode string
    Type string
}

type UniverseInput struct {
    Head string
    Tree []gitx.TreeEntry
    WorktreePaths []string
    LiftedStorePaths map[string]string
    LiftedContextPaths map[string]string
    ProjectionPaths []string
    Adapter Adapter
}

func BuildUniverse(in UniverseInput) ([]Candidate, error)
```

Tests must prove:

- source precedence `store-authority > declared-context > head-tree`;
- generated projection produces one retained head-tree candidate and one
  separate projection candidate;
- `.git/` never enters the universe;
- `.verdi/data/` produces exactly one boundary candidate and no descendant;
- ordinary tracked `testdata`, examples, and experiments remain candidates;
- worktree-overlay candidates contain no object/content/digest;
- candidate identity is `(source,id)` with `path:`, `ref:`, and the exact
  `opaque:harness-vendor-base/<id>/<version>` forms;
- duplicates and noncanonical paths fail closed.

The universe builder classifies no semantic content yet. It only creates the
complete deterministic candidate set without forbidden reads.

### Step 3: Verify discovery

```bash
go test -race ./internal/gitx -run 'Test(LsTreeEntries|WorktreeChangedPaths)' -count=1
go test -race ./internal/contextcompile -run TestBuildUniverse -count=1
git diff --check
```

Commit:

```text
Discover the context candidate universe
```

## Task 4: Resolve accepted semantic inputs and phase capsules

**Files:**

- Create: `internal/contextcompile/port.go`
- Create: `internal/contextcompile/authority.go`
- Create: `internal/contextcompile/fragments.go`
- Create: `internal/contextcompile/capsule.go`
- Create: `internal/contextcompile/authority_test.go`
- Create: `internal/contextcompile/fragments_test.go`
- Create: `internal/contextcompile/capsule_test.go`
- Create: `internal/contextcompile/testdata/fragments/` fixtures

### Step 1: Define narrow trusted ports and refusals

Use consumer-owned ports, not a generic filesystem or command runner:

```go
type GitReader interface {
    Show(ctx context.Context, root, ref, path string) ([]byte, error)
    LsTreeEntries(ctx context.Context, root, ref string) ([]gitx.TreeEntry, error)
    WorktreeChangedPaths(ctx context.Context, root string) ([]string, error)
}

type StateResolver interface {
    Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error)
}

type AuthorityLoader interface {
    Load(root string) (*policyauthority.Store, error)
    Resolve(store *policyauthority.Store) (*policyauthority.EffectivePolicy, error)
}

type ActorResolver interface {
    Resolutions(ctx context.Context) ([]governanceprincipal.PrincipalResolution, error)
}
```

Define typed sentinel/refusal errors for the exact exit-1 states in authority
§2. Malformed authority and broken port contracts remain ordinary operational
errors. Tests must use `errors.Is`/`errors.As`, not message matching.

### Step 2: Resolve the target only through merge-signaled state

Resolve `spec/<name>` to the active or archive store path, read exact HEAD
bytes, decode with `artifact.DecodeSpec`, and call `specstate.Projector` with
those exact bytes. Continue only when the result names the accepted baseline
path, blob, and landing commit required by the authority design. A caller does
not provide a commit or replacement bytes.

Test proposed, malformed, ambiguous-successor, superseded, archived accepted,
missing, and accepted paths. Prove the recorded content digest binds the exact
accepted bytes and the optional expected branch/HEAD is only compared to
computed repository facts.

### Step 3: Resolve all legal governing parents and fragments

For a non-spike story, accept every legal `implements` edge; for a spike,
accept every legal `resolves` edge. Resolve the referenced feature specs from
accepted authority, fail closed on class/fragment mismatch, group targets by
feature, and sort feature refs and target refs.

Build the exact canonical fragment object from authority §8.1:

```go
type FeatureFragment struct {
    Feature FragmentFeature `json:"feature"`
    Problem artifact.Attribute `json:"problem"`
    Outcome artifact.Attribute `json:"outcome"`
    Targets []FragmentTarget `json:"targets"`
    Constraints []artifact.Constraint `json:"constraints"`
    Decisions []artifact.Decision `json:"decisions"`
}
```

Do not include untargeted ACs/OQs, stubs, links, dispositions, custom fields,
or body prose. Preserve declaration order only for constraints and decisions.
Tests must include one story implementing ACs from two features and one spike
resolving questions from two features.

### Step 4: Compose phase-specific semantic inputs

Create pure `ComposeCapsule` tests for design, build, and review. The build
capsule refuses a feature or an unaccepted target and includes target, parent
fragments, exact pair-bound obligations, policy operands, grants, declared
context, scoped repository candidates, projection, and opaque base. Review
contains exactly five sorted required-input rows; result-diff,
evidence-bundle, and builder-receipt are `unproven` with their fixed disclosure
codes. Design/build required inputs are `[]`.

No report, receipt, or unsigned placeholder is decoded in this task.

### Step 5: Verify authority resolution

```bash
go test -race ./internal/contextcompile -run 'Test(Resolve|FeatureFragment|ComposeCapsule)' -count=1
git diff --check
```

Commit:

```text
Resolve context capsule authority
```

## Task 5: Implement applicability and total classification

**Files:**

- Create: `internal/contextcompile/applicability.go`
- Create: `internal/contextcompile/applicability_test.go`
- Create: `internal/contextcompile/classify.go`
- Create: `internal/contextcompile/classify_test.go`
- Create: `internal/contextcompile/payload.go`
- Create: `internal/contextcompile/payload_test.go`

### Step 1: Pin the narrow applicability algebra

Implement and test:

```go
type ApplicabilityInput struct {
    Policy policyartifact.Scope
    Request policyartifact.Scope
    CandidatePath string
    CandidateRef string
    Phase Phase
    Environment string
}

type ApplicabilityResult struct {
    State Applicability
    Disclosures []DisclosureCode
}

func EvaluateApplicability(in ApplicabilityInput) (ApplicabilityResult, error)
```

An explicit empty dimension is universal. Phase/environment/ref compare by
exact equality. `path` matches one exact path; `path/` matches only descendants
on a segment boundary. A known empty intersection is inapplicable. A missing
operand needed for comparison is unknown with a closed disclosure. Unknown is
never transformed into exclusion by the applicability function.

Include table cases for `cmd/` versus `cmdx/`, exact file paths, both-universal,
one-universal, disjoint sets, missing candidate path/ref/environment, and
unknown enum/path grammar.

### Step 2: Classify every candidate exactly once

Create a pure classifier returning three ledgers and data/projection payloads.
Tests must cover every included kind and every exclusion reason from authority
§5, plus opaque base. Assert:

- the union of included, excluded, and opaque candidate identities equals the
  input universe exactly;
- intersections are empty;
- rows are sorted and duplicate identities fail;
- unknown applicability is included with disclosure;
- inapplicable is excluded as `phase-inapplicable` or
  `out-of-declared-scope` according to the known cause;
- `.verdi/data/` boundary is `data-zone-disposable` and carries no descendant,
  content, or digest;
- worktree overlays are `uncommitted-content` without any byte read;
- ASD JSONL is `design-provenance-sidecar` without decoding;
- symlink/Gitlink is `non-regular-file` without following;
- invalid UTF-8/blob binary is `non-text-data`;
- managed checked-in projection is excluded as
  `generated-projection-output`, while freshly rendered projection is included
  through source `projection` and channel `authority`.

Read committed bytes only through the injected `GitReader.Show(HEAD, path)`
after a regular-blob candidate survives path-level exclusions. Add a fake that
panics if forbidden candidates reach `Show`.

### Step 3: Build provenance-wrapped data items

For each included non-projection candidate, construct a canonical
`verdi.context-data-item/v1` with classification
`non-authoritative-data`, exact content digest, and wrapper digest. Parent
feature fragments use their canonical fragment JSON bytes as `content`.
Projection files remain raw authority-channel bytes and never receive a data
wrapper.

### Step 4: Verify classification

```bash
go test -race ./internal/contextcompile -run 'Test(EvaluateApplicability|Classify|BuildDataItem)' -count=1
git diff --check
```

Commit:

```text
Classify context candidates and payloads
```

## Task 6: Extract the one pure instruction renderer

**Files:**

- Modify: `internal/instructionprojection/render.go`
- Modify: `internal/instructionprojection/manifest.go`
- Modify: `internal/instructionprojection/generate.go`
- Modify: `internal/instructionprojection/verify.go`
- Modify: `internal/instructionprojection/generate_test.go`
- Modify: `internal/instructionprojection/verify_test.go`
- Create: `internal/instructionprojection/render_test.go` if not already
  present

### Step 1: Pin pure rendering and selected-policy behavior

Write tests proving one pure call returns identical bytes and manifest facts to
Generate and Verify, performs no writes, rejects an unsealed/mismatched
effective policy, rejects unknown/duplicate selected policy IDs, sorts the
selection, and omits unselected policies without independently interpreting
scope.

Expose:

```go
type Selection struct {
    PolicyIDs []string
}

type RenderedFile struct {
    Path string
    Content []byte
    Digest string
}

type Rendered struct {
    AdapterID string
    AdapterVersion string
    AuthorityDigest string
    ProfileID string
    ProfileDigest string
    Files []RenderedFile
    Manifest []byte
    ManifestDigest string
}

func Render(
    store *policyauthority.Store,
    effective *policyauthority.EffectivePolicy,
    adapter policyartifact.Adapter,
    selection Selection,
) (Rendered, error)
```

The store/effective pair must be the genuine loaded/resolved pair; the selected
IDs must be a subset of effective policy entries. `Render` formats selected
instructions and builds the same projection manifest, but does no filesystem
I/O.

### Step 2: Route Generate and Verify through Render

Generate invokes `Render` for every adapter with all effective policy IDs, then
performs its existing preflight and atomic writes. Verify invokes the same
full-selection call and compares its returned bytes/facts to disk. The compiler
will later pass only applicable IDs.

Do not change the on-disk projection schema or golden bytes. The existing
Generate/Verify tests must remain exact-byte ratchets.

### Step 3: Verify the shared seam

```bash
go test -race ./internal/instructionprojection -count=1
git diff --check
```

Commit:

```text
Share pure instruction projection rendering
```

## Task 7: Implement the read-only compiler service

**Files:**

- Create: `internal/contextcompile/compiler.go`
- Create: `internal/contextcompile/revisions.go`
- Create: `internal/contextcompile/actors.go`
- Create: `internal/contextcompile/compiler_test.go`
- Create: `internal/contextcompile/integration_test.go`
- Create: `internal/contextcompile/testdata/golden/` canonical results

### Step 1: Define the service result and construction seam

Expose:

```go
type ProjectionFile struct {
    Path string
    Content []byte
    Digest string
}

type Result struct {
    Manifest Manifest
    ManifestBytes []byte
    DataItems []DataItem
    DataItemBytes [][]byte
    ProjectionFiles []ProjectionFile
}

type Compiler struct { /* unexported trusted ports */ }

func NewCompiler() Compiler
func (c Compiler) Compile(ctx context.Context, root string, request Request) (Result, error)
```

The zero value fails closed. A package-private constructor injects fakes.
Return fresh byte slices so a caller cannot mutate shared cached authority.

### Step 2: Write failing end-to-end service tests

Use hermetic Git repositories and an installed policy store to prove:

- identical HEAD, request, and authority yield byte-identical manifests,
  payloads, projections, revisions, and digests across two roots;
- design, multi-parent build story, multi-parent spike, and review capsules;
- no constitution, adapter mismatch, expected branch mismatch, expected HEAD
  mismatch, unaccepted target, wrong target class, and projection drift return
  typed refusals;
- malformed authority, Git failure needed for classification, and broken port
  facts return operational errors;
- missing actors completes with exit-0-capable unproven posture and mandatory
  disclosure; injected sealed resolutions project deterministically without
  exposing/reconstructing the seal;
- included/excluded/opaque partition is total and every included digest binds
  the returned bytes;
- `revisions.authority` binds effective policy, accepted baseline and bytes,
  sorted fragments, decisions, and obligations; `context` is exactly 1 and
  parent is omitted;
- dispositions and expansions remain `[]`;
- evidence is advisory, consumed reports is `[]`, and all stale/unknown facts
  have disclosures;
- compile leaves Git status, index, managed projection bytes, and the `.verdi/`
  tree unchanged.

### Step 3: Implement the ordered compilation pipeline

The production order is fixed:

1. validate request;
2. load/resolve policy authority and exact adapter;
3. gather shared repository facts and compare optional expectations;
4. resolve accepted spec and semantic dependencies;
5. verify the existing full managed projection against disk;
6. discover the candidate universe without forbidden reads;
7. evaluate applicability and select policy IDs;
8. call the pure projection renderer with only those IDs;
9. classify every candidate and build data payloads;
10. project sealed actors or explicit absence;
11. compute authority/context revisions, manifest rows, disclosures, and
    canonical bytes;
12. return the in-memory result.

Do not cache across calls in v1. Do not accept a caller-provided manifest,
digest, actor string, policy fact, repository fact, or evidence claim.

### Step 4: Verify the service

```bash
go test -race ./internal/contextcompile -count=1
go test -race ./internal/repositoryfacts ./internal/instructionprojection -count=1
git diff --check
```

Commit:

```text
Compile deterministic context capsules
```

## Task 8: Add the built-binary inspection surface

**Files:**

- Create: `cmd/verdi/context.go`
- Create: `cmd/verdi/context_test.go`
- Create: `cmd/verdi/context_e2e_test.go`
- Modify: `cmd/verdi/dispatch.go`
- Modify: `internal/specalign/verbs_test.go`

### Step 1: Write failing parser and built-binary tests

Cover:

- exact form `context compile --request <path|-> [--out <path>]`;
- missing/unknown subcommand, missing/duplicate flags, extra positional args,
  `--request` equal to `--out`, and an output path whose parent is unsafe;
- request from a file and stdin;
- canonical manifest to stdout when `--out` is absent;
- one atomic caller-selected manifest write and empty stdout when `--out` is
  present;
- data items and projection bytes are not written anywhere;
- exit 0 for completed manifests carrying advisory/unproven facts;
- exit 1 for each typed refusal family;
- exit 2 for malformed request and operational failure;
- stderr contains deterministic diagnostics without absolute checkout paths,
  raw remote URLs, credentials, payload content, or uncommitted path content;
- the built binary leaves the store and Git worktree unchanged except for the
  explicit output file.

The command accepts no actor flags, evidence flags, persistence flags, or
execution flags.

### Step 2: Implement CLI dispatch and output safety

Register `context` at phase 23, add it to usage, and dispatch before the
fallback. Parse the subcommand and flags before resolving the store root, so a
bare `verdi context` is hermetic and safe in the verb inventory test.

Use `atomicfile.Write` for `--out` after validating it is not inside `.git/`,
`.verdi/`, one of the managed projection paths, or the input request file. A
stdout write failure is operational. No partial file may remain after an
output failure.

Map errors through an explicit helper:

```go
func contextExitCode(err error) int {
    if contextcompile.IsRefusal(err) { return 1 }
    return 2
}
```

### Step 3: Grow the serialized verb inventory

Add `context` to `internal/specalign/verbs_test.go`'s real verb inventory and
document why bare invocation fails on parsing before any store read. Do not
edit frozen surface specs to add the verb; SI-78 and the authority design are
the ratified source for this increment.

### Step 4: Verify the CLI

```bash
go test -race ./cmd/verdi -run 'Test.*Context' -count=1
go test -race ./internal/specalign -run TestV0CLIVerbInventory -count=1
git diff --check
```

Commit:

```text
Expose context compilation through the CLI
```

## Task 9: Audit the complete range and prove gates

**Files:** only tests or implementation files required to correct a
binding defect found by the range audit. Do not alter authority in an
implementation correction.

### Step 1: Run an authority-to-code coverage audit

Build a checklist for every section of the authority design and SI-78..SI-87.
For each item, name the production locus and at least one positive/negative
test. Explicitly challenge:

- all 51 source elements remain represented by the design's 51/51 witness;
- no candidate is absent or double classified;
- uncommitted and `.verdi/data` bytes are never read;
- no non-projection content enters the authority channel;
- multi-parent story/spike fragments are complete but narrow;
- review gaps remain unproven and advisory;
- shared repository facts and renderer have exactly one production seam;
- request claims never replace computed/sealed facts;
- compilation performs no mutation or execution.

Correct only authority-backed defects, TDD first, in small commits.

### Step 2: Run focused and repository-wide verification

From the exact final head, without piping away exit codes:

```bash
go test -race ./internal/contextcompile ./internal/repositoryfacts ./internal/instructionprojection ./internal/journey ./internal/gitx ./cmd/verdi -count=1
go test -race ./... -count=1
make spec-align
make verify
git diff --check
git status --short --branch
```

`make verify` must end with `verify OK`; its existing browser suite must remain
green even though this increment has no frontend work. If a sandbox blocks a
socket or browser bind, preserve the failure evidence and rerun the same gate
unsandboxed rather than weakening a test.

### Step 3: Request one exact-head review

Have a fresh Opus reviewer inspect the complete base-to-head range read-only
against the authority design, SI-78..SI-87, and the 51/51 coverage witness.
Adjudicate every finding. Fix accepted defects with a fresh Opus fixer, rerun
focused plus full gates, and perform the one allowed closure check. Do not
start an automatic third review round.

### Step 4: Prepare the handoff

Report:

- exact base and head SHAs;
- ordered commits;
- changed-file inventory;
- authority-coverage map and any main-agent adjudications;
- exact command outputs and exit codes;
- clean `git diff --check` and `git status --short` evidence;
- explicit statements that no push, PR, store persistence, process execution,
  receipt, MCP surface, frontend, frozen artifact, or frozen spec was added
  unless the main agent separately authorized it.

