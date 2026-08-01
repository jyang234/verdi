# Merge-Signaled Specification Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the exact reviewed specification revision become accepted when it lands on the configured default branch, with no status-flip command or other second human ceremony.

**Architecture:** Add one Git-aware `internal/specstate` projection that compares a working revision with the default-branch blob and derives proposed, accepted, superseded, closed, or unproven state. Preserve legacy status and frozen-stamp decoding, migrate every acceptance consumer to the projection, move mechanical obligation creation before review, and make a single always-present GitHub check the required merge boundary. Git remains authoritative; GitHub supplies the pre-merge enforcement and human merge witness, never lifecycle state.

**Tech Stack:** Go 1.25, Git CLI through `internal/gitx`, strict YAML decoding through `internal/artifact`, hermetic `internal/fixturegit` repositories, GitHub Actions, GitHub rulesets, `make verify`, and `go test -race ./...`.

## Global Constraints

- The owner-ratified authority is `docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md`; binding semantics remain `docs/design/specs/00..05-*.md`, with decision history appended in workspace `docs/design/specs/08-revision-notes.md`.
- One human decision occurs once: merge is acceptance. Do not add `verdi accept`, `/accept`, a status-only commit, a bot, a GitHub App, a personal token, an MCP bridge, or a post-merge mutation.
- Default-branch reachability must resolve to proven, violated with a witness, or disclosed as unproven. Missing branch or ancestry information is never acceptance.
- The accepted baseline identity is the specification path, blob object ID, and first-parent landing commit.
- Merge, squash, and rebase landing strategies must produce the same effective state for the exact landed bytes.
- Existing `accepted-pending-build`, `superseded`, `closed`, and `frozen` data remains valid. This work performs no bulk migration of legacy artifacts.
- New feature and story specifications may omit `status`; component specifications retain their explicit status grammar.
- Direct edits to an accepted feature or story remain forbidden. Supersession or closure must remain a reviewed repository transition.
- No new runtime dependency, service, database, configuration key, environment variable, or networked test is permitted.
- Execute pinned upstream CLIs and strict-decode JSON; never import `verdi-go` packages.
- Every behavior change follows failing test, minimal implementation, green test, refactor, and a focused imperative commit.
- Every task ends with its focused tests. Every delivery pull request ends with fresh `make verify` and `go test -race ./...` output.
- Claude Code authors implementation changes. Codex reviews each exact pull-request head read-only; Claude Code repairs accepted findings; Codex re-reviews the new head. The human owner alone merges.
- Before implementation, record any newly discovered semantic ambiguity in workspace `PLAN.md` section 7 and stop that task. Do not infer lifecycle semantics from another tool.

---

## Delivery and review topology

Land this work as four sequential pull requests so each risky boundary can be reviewed and reverted independently:

1. **Authority:** the ratified design and this plan, extracted from PR #258 onto a clean branch from `origin/main`.
2. **Runtime:** Tasks 2–7, including Git proof, schema compatibility, consumer migration, and ritual retirement.
3. **Enforcement:** Task 8, including the stable required check and ruleset activation.
4. **Dogfood:** Task 9, rebasing and normalizing the four reviewed proposals in PR #258 only after the runtime and enforcement prerequisites land.

Do not activate a required check until the workflow defining that exact check name has landed and completed successfully on a pull request. Do not normalize the four proposal files until the compatible decoder is on `main`.

## File ownership map

- `internal/gitx/blob.go` owns read-only blob identity and first-parent landing discovery.
- `internal/specstate/state.go` owns effective-state vocabulary and result data.
- `internal/specstate/defaultbranch.go` owns default-branch name/ref resolution; `internal/lint/cienv.go` retains compatibility wrappers for existing callers.
- `internal/specstate/resolve.go` owns the single projection from candidate bytes plus Git history to effective state.
- `internal/artifact/spec.go` owns persisted schema compatibility only; it never decides Git-derived state.
- `internal/lint/vl004.go` owns legacy-draft disclosure; `internal/lint/vl010.go` owns accepted-baseline immutability.
- CLI, ref-index, residue, and workbench packages consume effective state through package-local interfaces; none reimplements reachability.
- `cmd/verdi/obligation.go` owns optional pre-review obligation scaffolding; `cmd/verdi/accept.go` becomes a non-mutating compatibility notice.
- `.github/workflows/merge-gate.yml` owns the stable pull-request check; the existing path-filtered workflows retain push duties only.

### Task 1: Land the ratified authority independently

**Files:**
- Copy unchanged: `docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md`
- Copy unchanged: `docs/superpowers/plans/2026-08-01-merge-signaled-spec-acceptance-implementation.md`
- Verify only: workspace `docs/design/specs/08-revision-notes.md`

**Interfaces:**
- Consumes: owner ratification dated 2026-08-01 and PR #258's reviewed documentation head.
- Produces: repository-visible authority on `main`; no runtime behavior.

- [ ] **Step 1: Create the clean authority branch from the current default branch**

```bash
git fetch origin main:refs/remotes/origin/main pull/258/head:refs/remotes/origin/pr-258
git switch -c agent/merge-signaled-acceptance-authority origin/main
git restore --source=origin/pr-258 -- docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md docs/superpowers/plans/2026-08-01-merge-signaled-spec-acceptance-implementation.md
```

Expected: only the two authority documents are changed.

- [ ] **Step 2: Verify the ratification record and scope**

```bash
rg -n 'Ratified by the owner on 2026-08-01|single authoritative acceptance ceremony' docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md
git diff --check
git status --short
```

Expected: both ratification phrases match, `git diff --check` is silent, and no runtime or `.verdi` path is present.

- [ ] **Step 3: Commit, push, and open the authority pull request**

```bash
git add docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md docs/superpowers/plans/2026-08-01-merge-signaled-spec-acceptance-implementation.md
git commit -m "Ratify merge-signaled spec acceptance"
git push -u origin agent/merge-signaled-acceptance-authority
gh pr create --draft --base main --head agent/merge-signaled-acceptance-authority --title "Ratify merge-signaled spec acceptance" --body "Records the owner-ratified lifecycle decision and its reviewed implementation plan. No runtime behavior changes."
```

Expected: the authority PR contains exactly two files. Codex reviews its exact head; the owner merges it before Task 2 starts.

### Task 2: Add deterministic Git baseline plumbing

**Files:**
- Create: `internal/gitx/blob.go`
- Create: `internal/gitx/blob_test.go`

**Interfaces:**
- Consumes: existing `gitx.run`, `gitx.Show`, and `fixturegit.Build` conventions.
- Produces:
  - `func BlobAt(ctx context.Context, dir, ref, path string) (oid string, found bool, err error)`
  - `func FirstParentBlobLanding(ctx context.Context, dir, ref, path, oid string) (commit string, found bool, err error)`

- [ ] **Step 1: Write failing unit tests for blob lookup**

Add table cases that prove a tracked file returns its 40-hex blob OID, an absent path returns `("", false, nil)`, an invalid ref returns an operational error, and an ambiguous/non-file tree entry is refused. The core assertion is:

```go
oid, found, err := BlobAt(ctx, repo, "main", ".verdi/specs/active/payments/spec.md")
if err != nil || !found || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(oid) {
	t.Fatalf("BlobAt() = (%q, %v, %v), want a proven blob", oid, found, err)
}
```

- [ ] **Step 2: Run the focused test and confirm the missing symbol failure**

Run: `go test ./internal/gitx -run 'TestBlobAt' -count=1`

Expected: compilation fails because `BlobAt` is undefined.

- [ ] **Step 3: Implement strict `git ls-tree` parsing**

Use `git ls-tree REF -- PATH`, accept exactly one record matching `^(100644|100755) blob [0-9a-f]{40}\t`, return absent only for empty stdout, and wrap every execution or parse failure with `gitx: BlobAt(REF:PATH)` context. Do not use shell parsing or a generic exported runner.

- [ ] **Step 4: Write failing first-parent landing tests**

Build deterministic histories covering: first add, unchanged later commits, replacement after a revert, regular merge, squash merge, and rebase merge. For each topology, hash the bytes at the default ref and assert:

```go
landing, found, err := FirstParentBlobLanding(ctx, repo, defaultRef, specPath, oid)
if err != nil || !found || landing != wantLanding {
	t.Fatalf("FirstParentBlobLanding() = (%q, %v, %v), want (%q, true, nil)", landing, found, err, wantLanding)
}
```

Also prove an unknown OID returns `("", false, nil)` and an invalid ref returns an error.

- [ ] **Step 5: Implement the minimal first-parent walk**

Read `git rev-list --first-parent --reverse REF`, compare `BlobAt` at each commit, and return the first commit in the final contiguous run whose blob equals `oid`. If a later first-parent commit changes away from `oid`, reset the candidate so revert-and-readd reports the current landing, not the historical first appearance.

- [ ] **Step 6: Run tests, format, and commit**

```bash
gofmt -w internal/gitx/blob.go internal/gitx/blob_test.go
go test -race ./internal/gitx/... ./internal/fixturegit/...
git add internal/gitx/blob.go internal/gitx/blob_test.go
git commit -m "Add Git spec baseline plumbing"
```

Expected: focused packages pass with race detection. Merge-strategy setup remains test-local and uses the fixture repository; it adds no production Git API.

### Task 3: Build the shared effective-state projection

**Files:**
- Create: `internal/specstate/state.go`
- Create: `internal/specstate/defaultbranch.go`
- Create: `internal/specstate/defaultbranch_test.go`
- Create: `internal/specstate/resolve.go`
- Create: `internal/specstate/resolve_test.go`
- Create: `internal/specstate/resolve_integration_test.go`
- Modify: `internal/lint/cienv.go`
- Modify: `internal/lint/cienv_test.go`

**Interfaces:**
- Consumes: `gitx.BlobAt`, `gitx.FirstParentBlobLanding`, `gitx.Show`, `gitx.LsTree`, `artifact.DecodeSpec`.
- Produces:

```go
type State string

const (
	Proposed             State = "proposed"
	AcceptedPendingBuild State = "accepted-pending-build"
	Superseded           State = "superseded"
	Closed               State = "closed"
	Unproven             State = "unproven"
)

type Relation string

const (
	RelationNew       Relation = "new"
	RelationExact     Relation = "exact"
	RelationDiverged  Relation = "diverged"
	RelationUnproven  Relation = "unproven"
)

type Branch struct { Name, Ref string }
type Baseline struct {
	Path string `json:"path"`
	Blob string `json:"blob"`
	LandingCommit string `json:"commit"`
}
type Candidate struct { Path string; Content []byte }
type Result struct {
	State State `json:"state"`
	Relation Relation `json:"relation"`
	Baseline *Baseline `json:"baseline"`
	Disclosures []string `json:"disclosures"`
}

func ResolveDefaultBranch(ctx context.Context, root string) (Branch, bool)
func NewProjector() Projector
func (p Projector) Resolve(ctx context.Context, root string, candidate Candidate) (Result, error)
func (p Projector) ResolveMany(ctx context.Context, root string, candidates []Candidate) ([]Result, error)
func (r Result) ArtifactStatus() artifact.Status
```

`Projector` defines an unexported `gitReader` consumer interface with `Show`, `BlobAt`, `FirstParentBlobLanding`, and `LsTree` methods. Package tests construct it through an unexported `newProjector` with fakes; production callers receive only `NewProjector`. `ResolveMany` reads and decodes the default-branch spec corpus once; `Resolve` delegates to it with one candidate so batch consumers do not create an O(specs²) Git scan.

- [ ] **Step 1: Write failing default-branch resolution tests**

Table-test these exact outcomes: `CI_DEFAULT_BRANCH=main` plus local `main` gives `{main, main}`; remote-only `origin/main` gives `{main, origin/main}`; configured `origin/HEAD` gives its actual target; both `origin/main` and `origin/master` without another signal are ambiguous and return false; no signal returns false; a named branch whose ref cannot resolve returns false.

- [ ] **Step 2: Implement branch name/ref resolution and retain the lint wrapper**

Move the resolution algorithm out of `internal/lint/cienv.go`. Keep this compatibility function so current callers compile until their task migrates:

```go
func ResolveDefaultBranch(ctx context.Context, root string) string {
	branch, ok := specstate.ResolveDefaultBranch(ctx, root)
	if !ok { return "" }
	return branch.Name
}
```

Keep the wrapper's short-name contract for branch-name comparisons. Change `lint.BuildContext` to use `branch.Ref` for `git merge-base` while storing `branch.Name` in `Context.DefaultBranch`.

- [ ] **Step 3: Write the projector's failing table tests**

Cover all rows below with an in-process fake:

| Candidate/default fact | Expected state | Relation |
|---|---|---|
| no resolvable default ref | unproven + disclosure | unproven |
| path absent on default | proposed | new |
| active exact bytes, omitted status | accepted-pending-build | exact |
| active exact bytes, legacy draft | accepted-pending-build + migration disclosure | exact |
| active exact bytes, legacy accepted | accepted-pending-build | exact |
| active exact predecessor named by a valid landed successor | superseded | exact |
| archive exact bytes | closed | exact |
| default path exists but candidate bytes differ | proposed with baseline | diverged |
| blob exists but landing commit cannot be proven | unproven + disclosure | unproven |
| malformed default-branch successor prevents a complete supersession scan | unproven + decode disclosure | unproven |

The exact-byte assertion must compare candidate content with `Show(default.Ref, candidate.Path)`; it must not trust a working-tree status field.

- [ ] **Step 4: Implement minimal resolution**

Derive the zone from `.verdi/specs/active/{name}/spec.md` or `.verdi/specs/archive/{name}/spec.md` and refuse other shapes. For supersession, scan default-branch active specs once per `ResolveMany` call, strict-decode each successor, and require both a `supersedes` link to the predecessor and a matching validated `supersession:` entry before projecting `Superseded`. A malformed scanned spec makes absence of a superseding edge unproven and names the decode witness. Sort disclosures and scanned paths for deterministic output.

- [ ] **Step 5: Add fixture-backed merge-strategy integration tests**

Create the same statusless feature on a design branch, land it by regular merge, squash, and rebase in three hermetic fixture repositories, then assert all three yield `AcceptedPendingBuild`, `RelationExact`, the same blob OID for identical bytes, and the strategy-specific first-parent landing commit. Add an unmerged branch case yielding `Proposed`.

- [ ] **Step 6: Run focused tests and commit**

```bash
gofmt -w internal/specstate internal/lint/cienv.go internal/lint/cienv_test.go
go test -race ./internal/specstate/... ./internal/lint/... ./internal/gitx/...
git add internal/specstate internal/lint/cienv.go internal/lint/cienv_test.go
git commit -m "Derive spec state from Git reachability"
```

### Task 4: Make persisted status optional without weakening immutability

**Files:**
- Modify: `internal/artifact/spec.go`
- Modify: `internal/artifact/spec_test.go`
- Modify: `internal/artifact/status.go`
- Modify: `internal/designscaffold/templates/feature.md`
- Modify: `internal/designscaffold/templates/story.md`
- Modify: `internal/designscaffold/templates/commitdesign.md`
- Modify: `internal/designscaffold/*_test.go`
- Modify: `cmd/verdi/design.go`
- Modify: `cmd/verdi/design_test.go` and scaffold-focused CLI tests
- Modify: `internal/lint/engine.go`
- Modify: `internal/lint/vl004.go`
- Modify: `internal/lint/vl004_test.go`
- Modify: `internal/lint/vl010.go`
- Modify: `internal/lint/vl010_test.go`
- Modify: `internal/lint/vl002.go`
- Modify: `internal/lint/vl002_test.go`
- Modify: `internal/lint/vl020.go`
- Modify: `internal/lint/vl020_test.go`
- Modify: `internal/lint/context.go`
- Modify: `internal/lint/context_test.go`

**Interfaces:**
- Consumes: `specstate.Projector.Resolve` and `specstate.Result` from Task 3.
- Produces: optional persisted status for feature/story specs; new scaffolds without a status line; `VL-004` compatibility disclosures; `VL-010` protection for any accepted feature/story baseline.

- [ ] **Step 1: Add failing schema compatibility tests**

Add positive decode cases for statusless feature and story frontmatter. Keep negative cases proving a component without status fails, an explicit unknown status fails, and legacy accepted/superseded/closed feature or story data without its required frozen stamp still fails.

```go
fm, err := DecodeSpec([]byte("id: spec/payments\nkind: spec\nclass: feature\ntitle: Payments\nowners: [platform]\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\n"))
if err != nil || fm.Status != "" { t.Fatalf("statusless feature = (%+v, %v)", fm, err) }
```

- [ ] **Step 2: Make only feature/story status optional**

Change the YAML tag to `yaml:"status,omitempty"`. In `validateFeature` and `validateStory`, allow the empty value before checking legacy enums; leave component validation unchanged. Preserve frozen-stamp requirements whenever an explicit legacy accepted or terminal value is present.

- [ ] **Step 3: Add failing scaffold tests and remove the persisted draft line**

Update built-in feature/story templates and CLI output expectations so new proposals omit `status:` and print `state: proposed (derived until merge)` instead of `status: draft`. Custom templates containing a legacy valid status continue to render and decode unchanged.

- [ ] **Step 4: Rewrite the VL-004 tests around compatibility disclosure**

Prove: a new statusless proposal in a PR targeting `main` has no VL-004 finding; a legacy draft whose exact bytes are already on default yields one `SeverityDisclosure`; a legacy draft only on the design branch yields no finding; an unresolvable default branch yields a disclosure naming the missing proof rather than a pass claim.

- [ ] **Step 5: Move readiness checks to the PR boundary**

Rename `Context.EnforceDraftGate` to `Context.TargetsDefaultBoundary`. Update VL-020 so a proposed story is editable off the default-branch boundary but missing obligations fail when CI targets the default branch. Add tests for both directions. Update VL-002 so a statusless spec in the archive zone is treated as closed-by-zone while legacy explicit closed behavior remains valid.

- [ ] **Step 6: Extend VL-010's base-side protection test first**

Add red tests showing that modifying or deleting an existing default-branch feature/story with no frozen stamp fails VL-010, while adding a new path succeeds and a component without a frozen stamp retains its existing behavior. Replace `baseFrozen` with `baseProtected`: return true for a stamped artifact or a strict-decoded feature/story at the diff base. Keep the active-to-archive closure exception, but accept a statusless feature/story moving byte-identically to archive.

- [ ] **Step 7: Run schema, scaffold, lint, and CLI tests; commit**

```bash
gofmt -w internal/artifact internal/designscaffold internal/lint cmd/verdi/design.go cmd/verdi/*design*_test.go
go test -race ./internal/artifact/... ./internal/designscaffold/... ./internal/lint/... ./cmd/verdi -run 'Test(DecodeSpec|Design|VL004|VL010)'
git add internal/artifact internal/designscaffold internal/lint cmd/verdi/design.go cmd/verdi/*design*_test.go
git commit -m "Permit Git-derived proposal state"
```

### Task 5: Migrate activation and gate consumers

**Files:**
- Modify: `cmd/verdi/buildstart.go`
- Modify: `cmd/verdi/buildstart_test.go`
- Modify: `cmd/verdi/gate.go`
- Modify: `cmd/verdi/gate_test.go`
- Modify: `cmd/verdi/obligation.go`
- Modify: `cmd/verdi/obligation_test.go`
- Modify: `cmd/verdi/featurearchive.go`
- Modify: `cmd/verdi/featurearchive_test.go`
- Modify: `cmd/verdi/featurematrix.go`
- Modify: `cmd/verdi/featurematrix_test.go`
- Create: `cmd/verdi/specstate.go`
- Create: `cmd/verdi/specstate_test.go`
- Modify: `cmd/verdi/dispatch.go`
- Modify: `internal/specalign/verbs_test.go`

**Interfaces:**
- Consumes: `specstate.Projector.Resolve(ctx, root, specstate.Candidate{Path: relPath, Content: bytes})`.
- Produces: all CLI activation checks require effective `AcceptedPendingBuild`, reject `Proposed`, `Superseded`, and `Closed`, and return operational exit 2 for `Unproven`.
- Produces: `verdi spec state SPEC_REF` emits deterministic canonical JSON containing `state`, `relation`, `baseline`, and `disclosures` without changing repository state.

- [ ] **Step 1: Introduce a package-local resolver seam and failing build-start tests**

Define in `buildstart.go`:

```go
type specStateResolver interface {
	Resolve(context.Context, string, specstate.Candidate) (specstate.Result, error)
}
```

Add tests proving a statusless exact default-branch story starts a build, an unmerged story returns exit 1 with “proposal has not landed”, a superseded story returns exit 1, and missing default-branch proof returns exit 2 with the disclosure.

- [ ] **Step 2: Replace raw build-start status checks**

Read the full spec bytes once, call the resolver, and branch on `Result.State`. Preserve existing story resolution, branch creation, sync, and exit-code behavior after the acceptance precondition.

- [ ] **Step 3: Add failing gate tests and replace `checkAcceptedOnDefaultBranch`**

Prove the gate passes for a statusless exact default-branch feature, fails as a verdict for an absent/diverged proposal, and fails operationally when default ancestry is unproven. The gate message includes `Baseline.Blob` and `Baseline.LandingCommit` on success so the exact accepted revision is auditable.

- [ ] **Step 4: Migrate remaining CLI eligibility probes**

Run the raw-status inventory below, inspect every result, and replace only acceptance/terminal decisions; display-only vocabulary maps and legacy decoder tests remain unchanged.

```bash
rg -n 'Status\s*[!=]=|accepted-pending-build|superseded|closed' cmd/verdi --glob '*.go'
```

For obligation authoring, resolve whether the target is accepted through the projector instead of the current merge-base/path-exists approximation. In `featurearchive.go` and `featurematrix.go`, resolve archived/implementing stories through one `ResolveMany` call so statusless closed stories and unchanged superseded predecessors are classified correctly without repeated corpus scans. Preserve pure fold APIs by passing the derived classification inward.

- [ ] **Step 5: Add the read-only state command test first**

Build a fixture with one statusless exact spec and assert `verdi spec state spec/payments` exits 0, changes neither `HEAD` nor `git status --porcelain`, and emits one canonical JSON line of this shape:

```go
if got.State != "accepted-pending-build" || got.Relation != "exact" || got.Baseline.Path != ".verdi/specs/active/payments/spec.md" {
	t.Fatalf("spec state = %+v", got)
}
if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(got.Baseline.Blob) || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(got.Baseline.LandingCommit) {
	t.Fatalf("baseline is not full Git identity: %+v", got.Baseline)
}
```

Separately assert sorted-key canonical encoding. A proposed spec emits `baseline:null`; unproven ancestry emits `state:"unproven"` and a non-empty disclosure. Known states exit 0; argument, read, or Git failures exit 2.

- [ ] **Step 6: Implement and inventory `verdi spec state`**

Resolve the ref with existing store path helpers, read bytes once, call the projector, and encode through the repository's canonical JSON seam. Add the nested `spec state` dispatch branch and its exact help/inventory entry. Do not add a readiness mutation or forge call.

- [ ] **Step 7: Run CLI tests and commit**

```bash
gofmt -w cmd/verdi
go test -race ./cmd/verdi/... -count=1
git add cmd/verdi
git add internal/specalign/verbs_test.go
git commit -m "Use effective state for lifecycle gates"
```

### Task 6: Migrate indexes, residue, and workbench projections

**Files:**
- Modify: `internal/refindex/port.go`
- Modify: `internal/refindex/refindex.go`
- Modify: `internal/refindex/refindex_test.go` and fake tests
- Modify: `internal/refindex/status.go`
- Modify: `internal/refindex/status_test.go`
- Modify: `internal/workbench/boardspec.go`
- Modify: `internal/workbench/projection.go`
- Modify: `internal/workbench/boardspec_test.go`
- Modify: `internal/workbench/projection_test.go`
- Modify: `internal/workbench/directory.go`
- Modify: `internal/workbench/directory_test.go`
- Modify: `internal/residue/activespecs.go`
- Modify: `internal/residue/activespecs_test.go`
- Modify: `internal/residue/patterna.go`
- Modify: `internal/residue/patterna_test.go`
- Modify: `internal/residue/patternb.go`
- Modify: `internal/residue/patternb_test.go`
- Modify: `internal/residue/closebranches.go`
- Modify: `internal/residue/closebranches_test.go`
- Modify: `internal/residue/scan.go`
- Modify: `internal/residue/scan_test.go`
- Modify: `internal/residue/survey.go`
- Modify: `internal/residue/survey_test.go`

**Interfaces:**
- Consumes: Task 3 `specstate.Result.ArtifactStatus()` for legacy display vocabulary and `Result.State` for decisions.
- Produces: default-branch statusless specs appear accepted/read-only; design-branch specs appear proposed/authoring; superseded predecessors project terminal state without a predecessor edit.

- [ ] **Step 1: Add a consumer-defined state port to refindex and red tests**

```go
type StateResolver interface {
	Resolve(context.Context, string, specstate.Candidate) (specstate.Result, error)
	ResolveMany(context.Context, string, []specstate.Candidate) ([]specstate.Result, error)
}
```

Inject it into `ComputeIndex` with both `Resolve` and `ResolveMany` methods. Prove a statusless default entry maps to `StatusGroupAcceptedPendingBuild` with `SpecStatus: "accepted-pending-build"`; an unmerged design entry maps to drafts-in-progress with `SpecStatus: "draft"`; an unproven entry carries a disclosure instead of entering an accepted group; and a 50-spec fake records one default-corpus scan rather than 50.

- [ ] **Step 2: Implement refindex projection through the shared resolver**

Retain ref-scoped reads and checkout immutability. Production construction uses `specstate.NewProjector` and resolves the collected candidates as one batch; unit tests use the existing fake plus a minimal fake resolver. Do not add state methods to the Git runner port.

- [ ] **Step 3: Add workbench red tests before changing loaders**

Prove the same statusless spec renders `Mode: readonly` when exact on default, `Mode: authoring` on its unmerged design branch, and `Status: accepted-pending-build` for vocabulary display. Pass the effective status into `buildProjection`; keep `buildProjection` pure and do not let it execute Git.

Because board mode and affordances are browser-visible behavior, Claude Code delegates this workbench sub-step and any resulting UI fix to the repository-required FABLE worker, then includes Playwright evidence in the task handoff.

- [ ] **Step 4: Migrate residue and remaining non-CLI decisions**

Inventory all raw lifecycle comparisons outside schema and tests:

```bash
rg -n 'Status\s*[!=]=|StatusDraft|StatusAccepted|accepted-pending-build|superseded|closed' internal --glob '*.go' --glob '!**/*_test.go'
```

For residue/fold code that consumes an already-built status map, change the map producer to supply effective statuses rather than teaching pure folds about Git. For workbench and MCP routes, resolve at the I/O loader and pass a value inward. Keep explicit component status and display-only vocabulary comparisons intact.

- [ ] **Step 5: Add an alignment test guarding decision consumers**

In `internal/specalign`, add a source audit that permits raw feature/story lifecycle decisions only in `internal/artifact` compatibility validation, `internal/specstate`, and named terminal-transition writers. The failure prints every file and line so a future adapter cannot silently reintroduce status-only acceptance.

- [ ] **Step 6: Run affected packages and commit**

```bash
gofmt -w internal/refindex internal/workbench internal/residue internal/specalign
go test -race ./internal/refindex/... ./internal/workbench/... ./internal/residue/... ./internal/specalign/... -count=1
git add internal/refindex internal/workbench internal/residue internal/specalign
git commit -m "Project effective state across read surfaces"
```

### Task 7: Retire the accept mutation and move preparation before review

**Files:**
- Modify: `cmd/verdi/accept.go`
- Modify: `cmd/verdi/accept_test.go`
- Modify: `cmd/verdi/ritual_test.go`
- Modify: `cmd/verdi/acceptobligation.go`
- Modify: `cmd/verdi/acceptobligation_test.go`
- Modify: `cmd/verdi/obligation.go`
- Modify: `cmd/verdi/obligation_test.go`
- Delete: `cmd/verdi/supersede.go`
- Modify: `cmd/verdi/supersedepredecessor_test.go`
- Modify: `cmd/verdi/vocabulary_cli_test.go`
- Modify: `cmd/verdi/dispatch.go` only to add the `obligation scaffold` subcommand branch
- Modify: `internal/specalign/verbs_test.go`
- Create: `docs/superpowers/reports/2026-08-01-lifecycle-ceremony-audit.md`

**Interfaces:**
- Consumes: existing obligation rendering/writing helpers and effective-state supersession from Tasks 3 and 6.
- Produces:
  - `verdi accept SPEC_REF` exits 0, explains merge-signaled acceptance, and performs no filesystem, index, branch, or commit mutation.
  - `verdi obligation scaffold STORY_REF` idempotently creates every missing declared `(ac, evidence-kind)` obligation before review and never overwrites an existing file.

- [ ] **Step 1: Replace mutation-oriented acceptance tests with a non-mutation characterization**

Snapshot `git status --porcelain`, `HEAD`, the spec bytes, predecessor bytes, and obligation directory before and after `verdi accept`. Assert all are byte-identical and stdout contains:

```text
accept is retired: merge the reviewed specification pull request into the configured default branch to accept this exact revision
```

Assert malformed argument count remains exit 2, while a valid proposal invocation is exit 0 because the command is informational, not an acceptance claim.

- [ ] **Step 2: Reduce `runAccept` to the compatibility notice**

Delete status splicing, frozen stamping, staging, committing, predecessor mutation, lint orchestration, and obligation writes from the accept path. Retain no hidden environment switch that restores mutation. Preserve the command during one compatibility window so existing scripts receive actionable output.

- [ ] **Step 3: Add failing batch-scaffold tests**

For a story declaring static and behavioral evidence on `ac-1`, prove `obligation scaffold` creates both convention paths, a second run creates zero and reports both as present, an existing authored file is byte-identical after the run, unknown story/AC/kind fails closed, and an accepted story refuses mutation.

- [ ] **Step 4: Move and rename the reusable creation core**

Move `scaffoldMissingObligations` out of the accept-specific file into the obligation command's responsibility. Remove the acceptance-time `artifact.Frozen` argument: proposal obligations are reviewable content, not falsely frozen at a pre-merge commit. Preserve the existing visible unauthored marker so reviewers can judge the generated demand in the same pull request; do not silently strengthen VL-020's ratified convention-path predicate in this task.

- [ ] **Step 5: Remove predecessor status mutation and prove derived supersession**

Rewrite supersession ritual tests so merging a valid successor leaves predecessor bytes unchanged while `specstate.Projector.Resolve(predecessor)` returns `Superseded`. Keep `VL-015` coverage for dangling, incomplete, or duplicate supersession manifests. Delete status-only supersession exceptions from VL-010 after no production writer depends on them.

- [ ] **Step 6: Write the lifecycle ceremony audit**

Create the report with this complete disposition table and link each retained action to its distinct purpose and test witness:

| Action | Classification | Disposition |
|---|---|---|
| merge specification PR | authoritative specification decision | retain once; merge derives acceptance |
| `verdi accept` | duplicate mechanical acceptance | retire mutation; compatibility notice only |
| obligation scaffold/author | pre-review content preparation | retain as optional idempotent authoring aid; never authority |
| `verdi design start` | reversible workspace/scaffold setup | retain; no lifecycle claim |
| `verdi build start` / `feature start` | reversible implementation workspace setup | retain; requires proven accepted baseline but does not approve it |
| merge implementation PR | authoritative code integration | retain once |
| `verdi close --prepare` and closure PR construction | evidence computation and review preparation | retain before review; closure authority is the reviewed archive merge |
| successor specification merge | authoritative supersession decision | retain once; derive predecessor state without mutation |
| attest, waive, adjudicate, or disposition | distinct human evidence, override, or judgment | retain with identity/provenance checks |
| Codex review | independent technical judgment | retain; never mutates or self-repairs |
| post-merge status edit or completion-ledger commit | deterministic duplicate bookkeeping | prohibit; compute through `verdi spec state` or derived CI data |

- [ ] **Step 7: Verify the ceremony audit and commit**

```bash
rg -n 'CreateCommit|AddAll|status: draft.*accepted-pending-build|scaffoldMissingObligations' cmd/verdi/accept.go cmd/verdi/acceptobligation.go
go test -race ./cmd/verdi/... ./internal/lint/... ./internal/specstate/... -count=1
git add cmd/verdi internal/lint internal/specstate internal/specalign docs/superpowers/reports/2026-08-01-lifecycle-ceremony-audit.md
git commit -m "Retire mutating spec acceptance"
```

Expected: the inventory finds no mutation in `accept.go`; all moved behavior has direct tests in its new pre-review or derived-state home.

- [ ] **Step 8: Run the runtime pull request gate and hand off for review**

```bash
make verify
go test -race ./... -count=1
git status --short
```

Expected: both commands pass and the worktree is clean. Push the runtime branch, open a draft PR, attach exact command output and head SHA, obtain Codex review, return findings to Claude Code, and repeat until Codex approves the exact head. The owner merges before Task 8.

### Task 8: Install the stable required merge gate

**Files:**
- Create: `.github/workflows/merge-gate.yml`
- Modify: `.github/workflows/verify.yml`
- Modify: `.github/workflows/spec-gate.yml`
- Create: `internal/specalign/workflow_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: `make verify`, `./.build/verdi lint`, GitHub pull-request base/head refs.
- Produces: an always-present GitHub check named exactly `merge-gate`; existing `verify` and `spec-gate` continue their push-only evidence duties.

- [ ] **Step 1: Write failing workflow source tests**

Assert `.github/workflows/merge-gate.yml` exists, triggers on every `pull_request` without a `paths` filter, declares one job key `merge-gate`, uses `actions/checkout@v4` with `fetch-depth: 0`, pins Go `1.25`, Node `22`, and golangci-lint `v2.5.0`, runs `make verify`, builds the binary, and runs `./.build/verdi lint`. Assert the old workflows no longer declare `pull_request`.

- [ ] **Step 2: Add the always-present workflow**

Copy the proven setup and commands from `verify.yml` into the new workflow. Keep the first implementation intentionally simple: every PR runs the full gate. Do not add a change classifier, third-party changed-files action, or conditional job whose skip semantics could make the required context ambiguous.

- [ ] **Step 3: Preserve push evidence behavior**

Remove only the `pull_request` triggers from `verify.yml` and `spec-gate.yml`; retain their existing push path filters and evidence upload behavior. Update comments so they no longer claim path-filtered PR coverage.

- [ ] **Step 4: Run local workflow alignment and full verification**

```bash
go test -race ./internal/specalign/... -count=1
make verify
git diff --check
git add .github/workflows/merge-gate.yml .github/workflows/verify.yml .github/workflows/spec-gate.yml internal/specalign/workflow_test.go README.md
git commit -m "Require one stable pull request gate"
```

Expected: workflow source tests pass, `make verify` passes, and the commit includes the new workflow.

- [ ] **Step 5: Prove the check exists before changing the ruleset**

Push the enforcement branch and open its draft PR. Wait for the exact `merge-gate` check to complete successfully, then record:

```bash
enforcement_pr="$(gh pr view --json number --jq .number)"
gh pr checks "$enforcement_pr" --required
gh pr view "$enforcement_pr" --json headRefOid,statusCheckRollup
```

Expected: `merge-gate` is successful for the reported `headRefOid`. Codex reviews the exact head; the owner merges the workflow PR before ruleset mutation.

- [ ] **Step 6: Activate the ruleset after the workflow is on `main`**

Read the current ruleset, preserve every existing rule, add one `required_status_checks` rule for context `merge-gate` with strict freshness enabled, retain required pull requests with zero approving reviews for the solo profile, and enable review-thread resolution:

```bash
gh api repos/jyang234/verdi/rulesets
gh api repos/jyang234/verdi/rulesets/19021982
```

Use GitHub's `PUT /repos/{owner}/{repo}/rulesets/{ruleset_id}` update endpoint. Build its full required body from the just-read response and append this rule without replacing the existing rule array from memory:

```json
{"type":"required_status_checks","parameters":{"required_status_checks":[{"context":"merge-gate"}],"strict_required_status_checks_policy":true,"do_not_enforce_on_create":false}}
```

In the existing `pull_request` rule, set `required_approving_review_count` to `0` and `required_review_thread_resolution` to `true`; preserve all other parameters, bypass actors, conditions, name, target, and enforcement. Create a temporary directory with `mktemp -d`, save the current response there, and use `jq` to emit only the update endpoint's writable fields (`name`, `target`, `enforcement`, `bypass_actors`, `conditions`, `rules`). The transformation updates the existing status-check rule when present or appends the JSON object above when absent, deduplicating checks by context. Then run:

```bash
ruleset_id="$(gh api repos/jyang234/verdi/rulesets --jq '.[] | select(.id == 19021982 and .target == "branch" and .enforcement == "active") | .id')"
test -n "$ruleset_id"
ruleset_payload_dir="$(mktemp -d)"
gh api "repos/jyang234/verdi/rulesets/$ruleset_id" > "$ruleset_payload_dir/current.json"
jq '{name,target,enforcement,bypass_actors,conditions,rules} | .rules |= (map(if .type == "pull_request" then .parameters.required_approving_review_count = 0 | .parameters.required_review_thread_resolution = true else . end) | if any(.[]; .type == "required_status_checks") then map(if .type == "required_status_checks" then .parameters.required_status_checks = ((.parameters.required_status_checks + [{"context":"merge-gate"}]) | unique_by(.context)) | .parameters.strict_required_status_checks_policy = true | .parameters.do_not_enforce_on_create = false else . end) else . + [{"type":"required_status_checks","parameters":{"required_status_checks":[{"context":"merge-gate"}],"strict_required_status_checks_policy":true,"do_not_enforce_on_create":false}}] end)' "$ruleset_payload_dir/current.json" > "$ruleset_payload_dir/update.json"
gh api --method PUT "repos/jyang234/verdi/rulesets/$ruleset_id" --input "$ruleset_payload_dir/update.json"
gh api "repos/jyang234/verdi/rulesets/$ruleset_id"
```

After PUT, verify: target `branch`, enforcement `active`, default-branch condition unchanged, `merge-gate` required, approvals `0`, and review-thread resolution `true`. If ruleset `19021982` no longer exists, stop before creating the temporary directory, inspect the list, and substitute the single active branch ruleset whose conditions include the default branch. Delete the temporary directory after verification; do not commit either JSON body.

- [ ] **Step 7: Exercise both green and red directions**

Open a documentation-only test PR and prove `merge-gate` appears and succeeds. On a disposable branch, introduce a reversible lint violation, push it, and prove the same required context fails and merge is blocked; then revert that violation, push, and prove the refreshed head succeeds. Close the disposable PR without merging.

### Task 9: Normalize and merge the four reviewed proposals

**Files:**
- Modify: `.verdi/specs/active/guided-lifecycle-governance/spec.md`
- Modify: `.verdi/specs/active/context-integrity/spec.md`
- Verify unchanged: `docs/superpowers/specs/2026-07-30-comparative-spike-experiments-design.md`
- Verify unchanged: `docs/superpowers/specs/2026-07-30-ai-assisted-spec-design.md`
- Verify unchanged after merge: `docs/superpowers/plans/2026-08-01-four-feature-orchestration.md`

**Interfaces:**
- Consumes: compatible decoder and stable required gate already on `main`.
- Produces: two canonical Verdi feature specs whose merge creates accepted lifecycle baselines, plus two owner-accepted design authorities that remain inputs to their explicitly sequenced canonical-promotion work; no status or frozen-stamp follow-up commit.

- [ ] **Step 1: Rebase PR #258 and remove only legacy draft fields**

Rebase its branch onto the new `main`. Remove `status: draft` from the two canonical active specs only. Do not add `accepted-pending-build` or `frozen`. Leave the ASD and CSE design documents byte-identical unless independent review finds a substantive design defect; they have no `.verdi` lifecycle frontmatter to normalize.

- [ ] **Step 2: Run proposal readiness and inspect the exact diff**

```bash
make verify
go test -race ./... -count=1
git diff origin/main...HEAD -- .verdi/specs docs/superpowers
rg -n '^status: draft$|^status: accepted-pending-build$|^frozen:' .verdi/specs/active/guided-lifecycle-governance/spec.md .verdi/specs/active/context-integrity/spec.md
```

Expected: both gates pass; the status/frozen inventory is empty for the two new proposals; every other semantic change is explicitly reviewed.

- [ ] **Step 3: Obtain exact-head independent review**

Claude Code attaches the head SHA, spec-to-plan mapping, changed-file list, and fresh command output to PR #258. Codex reviews all four specs, the orchestration index, and the lifecycle proof without modifying the branch. Claude Code repairs accepted findings and requests a fresh review for each new SHA.

- [ ] **Step 4: Merge once and verify derived acceptance without mutation**

After the owner merges PR #258, check out the new `main` and run `verdi spec state` for the two canonical `.verdi` specs. Each must report `accepted-pending-build`, its blob OID, and its first-parent landing commit. Verify the ASD and CSE design-document blobs equal their exact reviewed-head blobs and record them as ratified design inputs, not as canonical lifecycle artifacts. Then assert no later acceptance commit exists.

- [ ] **Step 5: Prove no bookkeeping ceremony remains**

Use `verdi spec state`, the merged PR's immutable head/base metadata, and the merge-gate run as the completion evidence. Do not open a status-only or ledger-only follow-up pull request. If a durable report is later required, generate it deterministically into `.verdi/data/derived/` from Git and forge facts; it must remain a projection, not a lifecycle input.

## Plan review record

### Risk summary

**High risks:**

- Schema transition could make legacy artifacts unreadable or accidentally mutable. Mitigation: additive empty-status decoding only for feature/story classes, explicit legacy tests, baseline-side VL-010 coverage, and no bulk migration.
- A wrong Git proof could misclassify a proposal or a superseded predecessor. Mitigation: one shared projector, full path/blob/landing identity, malformed-corpus unproven behavior, and fixture-backed merge/squash/rebase/revert tests.
- Requiring a nonexistent or unstable check could lock `main`. Mitigation: land and observe `merge-gate` before updating the ruleset, keep the PR rule intact, and document a narrow status-rule rollback.

**Medium risks:**

- Repeated successor scans could grow quadratically. Mitigation: `ResolveMany` scans the default corpus once and batch consumers prove a single scan with 50 candidates.
- Raw status decisions may survive in a secondary adapter. Mitigation: explicit source inventory, package-local resolver ports, and a source-alignment guard that names every forbidden decision site.
- Full verification on documentation-only PRs costs more CI time. Mitigation: begin with the smallest reliable always-present workflow; optimize only after measured run data can justify a tested classifier.

### Complexity assessment

| Item | Classification | Reason |
|---|---|---|
| `internal/specstate` projector | Justified | Acceptance has many existing consumers and must have exactly one Git proof implementation. |
| Git blob/first-parent helpers | Justified | Path/blob/landing identity is a ratified requirement and existing `LastCommit` does not prove it. |
| `ResolveMany` | Justified | Refindex, feature matrix, and residue are current batch consumers; it prevents a demonstrated O(specs²) shape. |
| read-only `verdi spec state` | Justified | Operators and CI need an observable proof without reviving a mutation ceremony. |
| new workflow | Justified | A path-filtered workflow cannot be a stable required check for every PR. |

No new dependency, service, configuration surface, future-only abstraction, or performance optimization is introduced. New files are limited to the shared proof kernel, its tests, the read-only CLI adapter, one workflow, and the required ceremony report.

### Verdict

**Approved.** The plan has explicit migration, test, rollback, performance, and enforcement sequencing. Implementation must stop if Task 3 cannot prove exact landing identity for all three GitHub merge strategies or if Task 8 cannot observe the stable check before ruleset activation.

## Rollback and recovery

- Before ruleset activation, every code change is reverted through an ordinary pull request. Legacy explicit statuses continue to work throughout.
- After ruleset activation, revert the defective runtime/workflow through a pull request that still satisfies `merge-gate`. If the workflow definition itself cannot start, temporarily remove only the `merge-gate` required-status rule using the just-read ruleset document, merge the workflow repair through the still-required PR rule, then restore the status requirement and record the outage.
- Never repair a bad acceptance projection by editing an accepted spec's status. Repair the projector or Git proof, then recompute from unchanged authoritative bytes.
- A post-merge projection, evidence upload, Pages build, or forge synchronization failure is operational. It cannot roll acceptance backward; report it as unproven governance/evidence until repaired.

## Final verification checklist

- [ ] `go test -race ./internal/gitx/... ./internal/specstate/...` passes, including merge/squash/rebase fixtures.
- [ ] `go test -race ./internal/artifact/... ./internal/lint/...` passes, including optional status and accepted-baseline immutability.
- [ ] `go test -race ./cmd/verdi/... ./internal/refindex/... ./internal/workbench/... ./internal/residue/...` passes.
- [ ] `make verify` passes at each pull request's exact head.
- [ ] `go test -race ./... -count=1` passes at each runtime/enforcement head.
- [ ] `verdi accept` changes no file, index entry, ref, or commit.
- [ ] No production acceptance consumer decides from raw feature/story `status` alone.
- [ ] The `merge-gate` context appears on documentation-only, specification-only, code-only, and mixed pull requests.
- [ ] The active default-branch ruleset requires `merge-gate`, zero solo approvals, and resolved review threads.
- [ ] The four proposal merges preserve the exact reviewed blobs and require no follow-up acceptance commit.
