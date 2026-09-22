# Readiness Recovery — Wave 3 Implementation Plan (ac-8 recovery projection, ac-9 the two executors, ac-10 apply protocol and MCP)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `verdi recover <ref>` emits a canonical, read-only recovery projection that recognizes every interrupted state the ritual inventory leaves behind, explains each one completely (facts, uncertainties with witnesses, steps completed, invariants held, choices), offers exactly two executable choices behind `--apply <choice-id>` (unwinding an empty branch cut with close's own sequence; delegating stranded residue to the reclaim plan), re-derives the projection and the journey after an applied choice, and is readable over MCP as `get_recovery(ref)` with no apply path.

**Architecture:** One new projection package `internal/recovery` (schema, codec, facts, recognizers, apply) in the two-package shape dc-1 set for readiness; one small shared package `internal/branchcut` extracted byte-for-byte from `cmd/verdi/close.go`'s `unwindClosureBranchCut` so close and recover run one implementation; the gitx recorder seam ritual-write-scope-v2 dc-5 specifies (a one-method observer carried in the context; no signature change; no package state) landed here under both specs' co-4, consumed by ac-9's forbidden-token check; `cmd/verdi/recover.go` and `internal/mcpserve/tool_get_recovery.go` as the two surfaces. Every recognizer is a pure function over one read-only fact gather. The read half is proven read-only by a residue-style AST command-surface gate; the write half is allowed exactly `branchcut.Unwind` and `reclaim.Apply`.

**Tech Stack:** Go 1.25; `internal/gitx`, `internal/filelock`, `internal/residue`, `internal/reclaim`, `internal/journey`, `internal/canonjson`, `internal/disclosure`, `internal/store`, `internal/draftmutation` (paths only), `internal/execworkspace` (grammar only), `internal/wtmanager` (paths only); `cmd/verdi`; `internal/mcpserve`; `internal/specalign` and `internal/showcasealign` inventories; `internal/fixturegit` for every test repository. No Playwright (co-4: recovery presentation is the post-design Fable lane's).

**Spec:** `.verdi/specs/active/readiness-recovery-v2/spec.md` — ac-8, ac-9, ac-10 under co-1..co-6, dc-4 (exactly two executors; staged/archive states are diagnosis; legibility first), dc-6 (Tier 3 with an owner risk gate for ac-9/ac-10; Tier 2 for ac-8; wave 3 = ac-8..ac-10; ends with the full gate), dc-7 (recognized states are the ones the ritual inventory proves the lifecycle leaves behind; anything else is unrecognized, never classified by resemblance). Parent authority: guided-lifecycle-governance-v3 AC-7 ("Recovery is read-only by default … Verdi never invents a recovery lifecycle or guesses which side of an ambiguous partial operation the user intended") and DC-13 ("Each choice proves where it starts and verifies where it ended. If those proofs are unavailable, Verdi provides diagnosis only"). Seam authority: `.verdi/specs/active/ritual-write-scope-v2/spec.md` dc-5 (the observer design) and co-4 ("whichever feature lands first owns the gitx seam and the other consumes it"). Design record: the brainstorming session of 2026-09-21 (approach A; one PR with the owner risk gate at the end; the seam landed here plus the static gate; surfaces rows ratified in-wave). **Base:** origin/main `d307821d` (waves 1 and 2 and the ac-4 supersession merged). Work happens in `/Users/johnyang/code/verdi-system/verdi-wt/readiness-recovery-w3` on branch `agent/readiness-recovery-wave-3`; port base 4590 for the gate, 4690 for lanes.

## Global Constraints

- No network in any test (co-1); every repository is `internal/fixturegit`; CLI paths through the built binary; MCP paths through hermetic transcripts.
- co-2: no readiness or recovery file, cache, status field, transition, receipt, event log, or artifact kind is added; the projection is derived from facts that already exist and is never authority.
- co-3: the only mutation this wave adds is `verdi recover --apply` over the two existing executors; MCP gains a read-only tool only; the projection never authors or suggests a human attestation claim.
- co-4: no workbench markup, CSS, or JS changes.
- co-5: every new production literal that names a spec class or lifecycle status routes through `model.DisplayClass`/`model.Indefinite` or carries `// vocab:identity — <why>` at the producing site; recovery state codes, choice ids, branch names, and paths are identity tokens. `TestVocabProseWitness` and `internal/mcpserve`'s vocabulary tests pass after every task.
- co-6: an unavailable fact is stated as an uncertainty with the witness that would settle it; an undecidable state is disclosed, never guessed; a choice without an executor says so.
- dc-4/ac-9: no new git primitive; the executors call only `branchcut.Unwind` and `reclaim.Apply`; no recovery run's git command log contains `reset`, `restore`, `clean`, `stash`, `--force`, or `update-ref` (the runtime check is scoped to the command log, never to printed manual-command text, which legitimately contains `git restore`).
- Exit codes (ac-8): `verdi recover <ref>` exits 0 when no state is recognized, 1 when at least one is, 2 operational. `--apply` exits 0 when every postcondition holds after execution, 1 for a refused choice (no executor, precondition no longer holds, postcondition violated — nothing changed in the first two), 2 operational. This deliberately diverges from `cmd/verdi/journey.go`'s "exit 1 is unreachable" doctrine; the divergence is documented in `recover.go`'s package comment and recorded as ledger SI-220.
- Serialized registries, each touched by exactly one task: `cmd/verdi/dispatch.go` (`verbPhase`, `usage`, dispatch arm), `cmd/verdi/help.go` (`topLevelUsage`, `verbUsage`), `internal/mcpserve/tooldefs.go`, `internal/mcpserve/server.go`, `internal/specalign/verbs_test.go`, `internal/specalign/mcptools_test.go`, `internal/showcasealign/coverage_test.go` — all in Task 3. `verdi/CLAUDE.md`'s CLI-verbs line, `README.md`, `docs/guide-claims.yaml`, `docs/architecture-and-journeys.md` — Task 3. The verdi-surfaces origin/mirror rows and 08-revision-notes — Task 5 (controller).
- gofmt/vet/golangci-lint clean; `go test -race` clean; every commit builds (the shared pre-commit hook: `make hooks`); implementer commits end with `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>`; Opus fixer commits with `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>`.
- Brief protocol (PA-019): every task brief is generated by `scripts/sdd-brief <this plan> <N> .superpowers/sdd/2026-09-21-readiness-recovery-wave-3/task-<N>-brief.md` and checked with `scripts/sdd-brief --check <this plan> <brief>` before dispatch; the brief is the whole contract.
- Pre-review gate (PA lesson, wave 2): the controller runs each brief's FULL GREEN list before dispatching review, never a subset. Long gates run serially, never concurrently with a reviewer's test runs (cmd/verdi's race run sits at go test's 10-minute default).
- The ledger rows SI-218 (filelock stale-versus-absent query), SI-219 (seam ownership under co-4), SI-220 (recover's exit-1 divergence from journey), and SI-221 (ac-9's "original branch" has two readings by cut mechanism — R-RR3-5) are recorded by the controller in `docs/superpowers/invention-ledger.md` BEFORE Task 1 is dispatched (CLAUDE.md: a proof-meaning or public-interface ambiguity blocks implementation until recorded). Next free number after them: SI-222.

---

## Rulings recorded in this plan

- **R-RR3-1 (exit classification).** ac-8's three exits are verdicts about recognized state, so `recover` renders exit 1 for "something is recognized" although the projection is not authority; the parallel is `verdi close --preflight` ("an absent artifact is a verdict, never operational"), not `verdi journey`. Cost if wrong: one return value.
- **R-RR3-2 (the seam's shape; amended after Task 1 review).** `gitx.Observer` is a one-method interface `Observe(dir string, args []string)`; `gitx.WithObserver(ctx, obs) context.Context` attaches it; EVERY exec site in `internal/gitx` calls `observe` before executing — `run` (exec.go), `ConfigValue` (configvalue.go), and `runStdin` (plumbing.go, the plumbing path behind `WriteBlob`, `BuildTreeWithFile`, `CommitTree`, `UpdateRef`, and `ApplyPatch`, and the only producer of the `update-ref` token). The observer test pins all three, and a test asserts the count of `exec.Command` sites equals the count of `observe` call sites so a fourth site cannot appear unobserved. No signature changes, no package variable, nil observer is a no-op. `internal/recovery.CommandLog` is the one recorder ac-9 needs; ritual-write-scope-v2 consumes the same seam later (co-4). Cost if wrong: one interface.
- **R-RR3-3 (choice identity).** Executable choice ids name their exact target: `unwind-branch-cut:<branch>` and `reclaim:<branch>` (every reclaim unit has a branch; its worktree path, when any, is carried in the choice's facts). `--apply` matches the id byte-for-byte; there is no interactive confirmation — the exact-target id under `--apply` is the confirmation the parent's AC-7 requires, and it is what keeps MCP unable to reach the executors. Cost if wrong: an id grammar.
- **R-RR3-4 (which branches belong to a ref).** A ref's ritual branches are `design/<name>`, `feature/<name>`, `close/<name>` (dc-7: design start, build start, close); `policy/adopt` is a store-wide ritual branch and is reported under every ref's projection with `scope: store`, as are locks, execution-workspace residue, and the draft-mutation journal for the ref's own name. Cost if wrong: one list.
- **R-RR3-5 (what "empty" means, and where to return — from the 2026-09-21 spike, `.superpowers/sdd/2026-09-21-readiness-recovery-wave-3/spike-emptycut-findings.md`).** A ritual branch has no commits of its own iff some OTHER local branch's tip is equal to, or a descendant of, the ritual branch's tip (`gitx.LocalBranches` + `gitx.RevParse` + `gitx.IsAncestor(ritualTip, otherTip)`); this is base-hypothesis-free, survives the base moving on after the cut, and rejects a branch with one own commit. The cut point recorded in the state is the ritual branch's own tip. The return branch depends on the ritual's cut mechanism, which the spike proved are two, not four: build start and close cut from the CURRENT checkout (`gitx.CheckoutNewBranch`), so the return branch is the candidate whose tip equals the ritual tip when exactly one exists, else the unique candidate that contains it, else UNDECIDABLE (deleted source branch or a tie) — no executor, the candidates listed as the uncertainty's witness; design start and policy adopt cut from the RESOLVED DEFAULT-BRANCH BASE (`resolveBranchBase` + `CheckoutNewBranchFrom`) independent of the checkout, so their return branch is the freshly re-resolved default branch (`cmd/verdi`'s `resolveBranchBase` rule, exposed read-only for the projection), UNDECIDABLE when it does not resolve. The branch reflog's "Created from" line is corroboration only (it reads `HEAD` for the current-checkout rituals and vanishes on a fresh clone). ac-9 says "the original branch" once for both mechanisms; the two readings are recorded as ledger SI-221 before Task 2. Because the return branch may sit ahead of the ritual tip, `branchcut.Unwind`'s checkout may move the working tree to a newer commit: the clean-tree and empty-index preconditions make that safe, and the choice's effects say "switch to <branch> at <its tip>". Cost if wrong: one predicate and one two-way rule.
- **R-RR3-6 (stale versus absent locks).** `filelock.Peek` collapses stale into absent by contract (gc's own needs). ac-8 requires stale as a recognized state, so `filelock` gains an additive read-only `Inspect(path) (Inspection, error)` returning one of `LockAbsent`, `LockHeld`, `LockStale`, `LockUndecidable` with the recorded pid/start and a reason sentence; `Peek` is untouched. `LockUndecidable` (an unparseable `ps` under the documented kill-probe-only fallback — it names the pid and the `ps` witness) is reported as an uncertainty, never as stale; a YOUNG empty or partial body is `LockHeld` (Peek's mid-flush charity: a live holder still writing, and there is no pid to name a witness for), an OLD empty body is `LockStale` (review 2A I3 resolved the contradiction between this ruling's first wording and the Interfaces block in favour of the block). Recorded as SI-218. Cost if wrong: one additive function.
- **R-RR3-7 (manual commands are exact, executors are two).** Every non-executable choice carries the exact command line(s) the operator would run and `executor: none`. The stale-lock choice's command is `rm <absolute lock path>` with the liveness evidence in its facts. Staged/uncommitted and archive-move states carry close's own existing advice text verbatim (`closureResidueRefusal`'s commands), which contains `git restore` — allowed in printed text, forbidden in the command log. Cost if wrong: strings.
- **R-RR3-8 (ambiguity withholds executors).** When two recognized states share a branch (an empty cut AND an uncommitted archive move or staged closure paths on disk), both states are emitted, the unwind choice is withheld, and each carries an uncertainty naming the other as the reason (parent AC-7: never guess which side the user intended). Cost if wrong: one guard.
- **R-RR3-9 (the apply protocol).** `--apply` derives the projection fresh, finds the choice by id (unknown id: exit 1 with the known ids listed), refuses a choice with no executor (exit 1, the sentence "choice <id> has no executor; the manual commands are: …"), re-proves the choice's preconditions from a fresh fact gather immediately before execution (any failure: exit 1, nothing changed, the failed precondition named), executes, re-derives the recovery projection and `journey.NewProjector().Project`, prints the postconditions with their observed values, exits 0 only if every postcondition holds. The journey re-derivation failing is operational (exit 2) after the executor ran; the projection still prints what it observed. Cost if wrong: one function.
- **R-RR3-10 (the runtime token check).** `cmdRecover` attaches a `recovery.CommandLog` to its context for the whole run; after the run, if the log contains a forbidden token, the verb prints the offending command and exits 2 — a recovery run that ever issued one is an operational failure of this verb's own contract, not a verdict. The built-binary tests assert the log directly through a `VERDI_RECOVERY_GITLOG=<path>` environment variable that, when set, makes `cmdRecover` append the log to that file (test-only observability; the file is never read by the verb). Cost if wrong: one env var.
- **R-RR3-11 (governed actions).** `governed-action-interrupted` is recognized from exactly two artifacts that exist today: a draft-mutation journal at `store.DraftMutationJournalPath(root, name)` (its `phase` and the steps its directory shows completed) and an execution-workspace entry under `execworkspace.ExecutionRoot(root)` whose `ClassifyEntry` form is a staging or lock sibling without its unit directory. Constitution proposals leave only a branch, so they are covered by the branch-cut recognizer; nothing else is inferred (dc-7). Cost if wrong: two recognizers.
- **R-RR3-13 (a failed push is not observable; the fact is).** No artifact records that a board push failed; git shows only that `design/<name>` is ahead of its remote-tracking branch (or has none). The `board-push-failed` state is emitted on that fact and carries the uncertainty "whether a push was attempted and failed, or was never attempted" with the witness "the board's commit response, or `git push -u origin design/<name>`" — never a claim that a push failed. Cost if wrong: one sentence.
- **R-RR3-14 (reclaim mirrors gc's refusal).** When `residue.Scan` reports any `UnprovenSpecs`, gc refuses to compute or apply a plan over an incomplete scan; the recovery projection does the same: no `stranded-residue` state carries an executable choice, and the projection discloses the unproven specs with gc's own sentence. Cost if wrong: one guard.
- **R-RR3-12 (remote comparison without network).** `closure-unpublished` and `board-push-failed` compare the local branch with its local remote-tracking ref (`gitx.HasRemoteTrackingBranch`, `gitx.AheadBehind`); no fetch is issued. The projection says the comparison is against the last-fetched remote state, as an uncertainty with the witness `git fetch origin`. Cost if wrong: one sentence.

## File structure

- `internal/gitx/observer.go` (new): `Observer`, `WithObserver`, `observe(ctx, dir, args)`; `exec.go` and `configvalue.go` call `observe` first. `observer_test.go`.
- `internal/branchcut/branchcut.go` (new): `Unwind(ctx, root, originalBranch, branch, cutPoint string, stderr io.Writer) Outcome` — the moved body of `unwindClosureBranchCut` with its stderr lines unchanged except the `close:` prefix becoming a parameter; `branchcut_test.go` (the two existing close_test cases moved here plus the recover-facing outcome cases). `cmd/verdi/close.go` and `closefeature.go` call it.
- `internal/filelock/inspect.go` (new): `Inspection`, `LockStatus`, `Inspect`. `inspect_test.go`.
- `internal/recovery/schema.go`: `SchemaID`, `Projection`, `RecognizedState`, `Uncertainty`, `Choice`, `StateCode`, `Scope`, `Reversibility`, `Validate`.
- `internal/recovery/codec.go`: `Canonical`, `Decode` (journey's shape).
- `internal/recovery/facts.go`: `Facts`, `Gatherer`, `NewGatherer`, `Gather(ctx, cfg, ref) (Facts, error)` — the only file in the read half that touches git, locks, or the filesystem.
- `internal/recovery/derive.go`: `Derive(f Facts) Projection` and one recognizer per state.
- `internal/recovery/commandlog.go`: `CommandLog` (an `Observer`), `ForbiddenTokens`, `Forbidden(log) []string`.
- `internal/recovery/apply.go`: `Apply(ctx, cfg, ref, choiceID string, stdout io.Writer) (Outcome, error)` and the two executors behind `Executor` values.
- `internal/recovery/commandsurface_test.go`: the AST gate (read half: read-only gitx names only; `apply.go`: `branchcut.Unwind`, `reclaim.Apply`, plus read-only names).
- `internal/recovery/derive_test.go`, `facts_test.go`, `apply_test.go`, `codec_test.go`, `schema_test.go`, `fixtures_test.go` (fixturegit stores in each interrupted state).
- `cmd/verdi/recover.go`, `recover_test.go`, `recover_e2e_test.go` (built binary).
- `internal/mcpserve/tool_get_recovery.go`, `tool_get_recovery_test.go`; `backend.go` (+`RecoveryLoader` port); `tooldefs.go`; `server.go`; `cmd/verdi/mcp.go` (wire the loader).
- Registries and docs (Task 3): `cmd/verdi/dispatch.go`, `help.go`; `internal/specalign/verbs_test.go`, `mcptools_test.go`; `internal/showcasealign/coverage_test.go`, `cli_showcase_test.go`, `mcp_showcase_test.go`; `verdi/CLAUDE.md`; `README.md`; `docs/guide-claims.yaml`; `docs/architecture-and-journeys.md`.
- Task 5 (controller): `.verdi/specs/active/verdi-surfaces/spec.md` (CLI row, MCP row), workspace-root `docs/design/specs/05-surfaces.md` and `08-revision-notes.md`; `docs/superpowers/invention-ledger.md` (SI-218..220 before Task 1; closure states after).

---

### Task 1: The gitx observer seam and the shared branch-cut unwind (Tier 2)

**Files:**
- Create: `internal/gitx/observer.go`, `internal/gitx/observer_test.go`, `internal/branchcut/branchcut.go`, `internal/branchcut/branchcut_test.go`
- Modify: `internal/gitx/exec.go:14-26` (`run` calls `observe` first), `internal/gitx/configvalue.go:74-76` (`ConfigValue` calls `observe` first), `cmd/verdi/close.go:213-268` (delete `unwindClosureBranchCut`; the six call sites at `close.go:838,844,886` and `closefeature.go:200,206,252` call `branchcut.Unwind(ctx, root, originalBranch, closureBranch, head, "close", stderr)`), `cmd/verdi/close_test.go:1880-1930` (the two direct tests move to `branchcut_test.go`).

**Interfaces:**
- Consumes: `gitx.RevParse`, `gitx.CheckoutExisting`, `gitx.DeleteBranch` (unchanged).
- Produces:

```go
package gitx

// Observer receives every git invocation gitx issues on a context it is
// attached to — dir and the exact argv after "git" — BEFORE the command
// runs (ritual-write-scope-v2 dc-5: a consumer-defined one-method observer
// attached through the context every gitx call already receives; no
// signature change; no package-level state). A nil or absent observer is
// a no-op. Observers must be safe for concurrent use if the caller runs
// gitx calls concurrently; gitx never copies args.
type Observer interface {
	Observe(dir string, args []string)
}

// WithObserver returns a context carrying obs.
func WithObserver(ctx context.Context, obs Observer) context.Context

// observe notifies the context's observer, if any. Called at the top of
// run and of ConfigValue (the one exec site that bypasses run).
func observe(ctx context.Context, dir string, args []string)
```

```go
package branchcut

// Outcome is what Unwind did, for callers that need more than stderr.
type Outcome int

const (
	Unwound          Outcome = iota // switched back and deleted the branch
	LeftUninspectable               // RevParse failed
	LeftAheadOfCut                  // tip != cutPoint: nothing discarded
	LeftSwitchFailed                // CheckoutExisting failed
	LeftDeleteFailed                // switched back, DeleteBranch failed
)

// Unwind is verdi close's branch-cut unwind, moved here verbatim so
// close and recover run ONE sequence (spec/readiness-recovery-v2 ac-9:
// "with the sequence close already uses"): re-prove branch still points
// at cutPoint, switch back to originalBranch (or cutPoint itself when
// originalBranch is "", the detached-HEAD case), then `git branch -d`.
// It NEVER discards committed work and NEVER force-deletes; every
// giving-up path is disclosed on stderr with prefix "<verb>: ".
func Unwind(ctx context.Context, root, originalBranch, branch, cutPoint, verb string, stderr io.Writer) Outcome
```

- [ ] **Step 1: Failing observer tests**

`internal/gitx/observer_test.go`:

```go
type recordingObserver struct{ calls [][]string }

func (r *recordingObserver) Observe(dir string, args []string) {
	r.calls = append(r.calls, append([]string{dir}, args...))
}

func TestObserver_SeesRunAndConfigValue(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	obs := &recordingObserver{}
	ctx := WithObserver(context.Background(), obs)
	if _, err := RevParse(ctx, repo.Dir, "HEAD"); err != nil {
		t.Fatal(err)
	}
	if _, err := ConfigValue(ctx, repo.Dir, "core.bare"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{repo.Dir, "rev-parse", "--verify", "HEAD^{commit}"}, {repo.Dir, "config", "--local", "--get-all", "core.bare"}}
	if !reflect.DeepEqual(obs.calls, want) {
		t.Fatalf("observed %v, want %v", obs.calls, want)
	}
}

func TestObserver_AbsentIsNoop(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	if _, err := RevParse(context.Background(), repo.Dir, "HEAD"); err != nil {
		t.Fatal(err)
	}
	ctx := WithObserver(context.Background(), nil)
	if _, err := RevParse(ctx, repo.Dir, "HEAD"); err != nil {
		t.Fatal(err)
	}
}
```

Copy the exact `rev-parse` argv from `internal/gitx/revparse.go:12-20` into `want` (read it; do not guess).

- [ ] **Step 2: Run to verify RED**

Run: `go test ./internal/gitx -run TestObserver -v`
Expected: FAIL — `WithObserver` undefined.

- [ ] **Step 3: Implement the seam**

`internal/gitx/observer.go`:

```go
package gitx

import "context"

type Observer interface {
	Observe(dir string, args []string)
}

type observerKey struct{}

func WithObserver(ctx context.Context, obs Observer) context.Context {
	if obs == nil {
		return ctx
	}
	return context.WithValue(ctx, observerKey{}, obs)
}

func observe(ctx context.Context, dir string, args []string) {
	if obs, ok := ctx.Value(observerKey{}).(Observer); ok && obs != nil {
		obs.Observe(dir, args)
	}
}
```

In `exec.go`'s `run`, add `observe(ctx, dir, args)` as the first statement. In `configvalue.go`'s `ConfigValue`, add `observe(ctx, dir, []string{"config", "--local", "--get-all", key})` immediately before `exec.CommandContext`. In `plumbing.go`'s `runStdin`, add `observe(ctx, dir, args)` as the first statement. Add a package doc sentence to `exec.go`: "Every exec site calls observe first — run, ConfigValue, and runStdin; observer_test pins all three and counts the exec sites."

- [ ] **Step 4: Run to verify GREEN**

Run: `go test -race ./internal/gitx`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gitx
git commit -m "Add the gitx observer seam (ritual-write-scope-v2 dc-5)"
```

- [ ] **Step 6: Failing branchcut tests (moved and extended)**

Create `internal/branchcut/branchcut_test.go` by moving the two cases at `cmd/verdi/close_test.go:1880-1930` (read them first; keep their fixture construction verbatim, replacing `unwindClosureBranchCut(ctx, repo.Dir, "main", "close/foo", cutPoint, &stderr)` with `Unwind(ctx, repo.Dir, "main", "close/foo", cutPoint, "close", &stderr)`), and add outcome assertions:

```go
func TestUnwind_EmptyCutIsUnwound(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "close/foo"); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if got := Unwind(ctx, repo.Dir, "main", "close/foo", repo.Head, "recover", &stderr); got != Unwound {
		t.Fatalf("outcome = %v, stderr %q", got, stderr.String())
	}
	if branches, _ := gitx.LocalBranches(ctx, repo.Dir); slices.Contains(branches, "close/foo") {
		t.Fatal("close/foo still exists")
	}
	if cur, _ := gitx.CurrentBranch(ctx, repo.Dir); cur != "main" {
		t.Fatalf("current branch = %q", cur)
	}
}

func TestUnwind_AheadOfCutIsLeft(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "close/foo"); err != nil {
		t.Fatal(err)
	}
	fixturegit.Dangle(t, repo, map[string]string{"b.txt": "b\n"}, "own commit") // read Dangle's contract at internal/fixturegit/dangle.go:23 first; if it does not commit on the current branch, write the file and commit with gitx.AddPaths + gitx.CreateCommit instead
	var stderr bytes.Buffer
	if got := Unwind(ctx, repo.Dir, "main", "close/foo", repo.Head, "recover", &stderr); got != LeftAheadOfCut {
		t.Fatalf("outcome = %v", got)
	}
	if !strings.Contains(stderr.String(), "recover: left close/foo in place: it carries commit(s) beyond its cut point") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
```

- [ ] **Step 7: Run to verify RED**

Run: `go test ./internal/branchcut -run TestUnwind -v`
Expected: FAIL — package does not exist.

- [ ] **Step 8: Extract Unwind**

Move the body of `unwindClosureBranchCut` into `internal/branchcut/branchcut.go` as `Unwind`, replacing each `"close: "` prefix with `verb+": "`, returning the matching `Outcome` at each `return`. Keep the doc comment, retargeted: it is now the single implementation close and recover share. In `close.go` and `closefeature.go` replace the six calls with `branchcut.Unwind(ctx, root, originalBranch, closureBranch, head, "close", stderr)` and delete the old function. Delete the two moved tests from `close_test.go`.

- [ ] **Step 9: Run to verify GREEN and close's own pins**

Run: `go test -race ./internal/branchcut && go test -count=1 -run 'TestRunClose|TestCloseFeature|TestUnwind' ./cmd/verdi -timeout 15m`
Expected: PASS; every close stderr pin unchanged (the prefix is still `close:`).

- [ ] **Step 10: Commit**

```bash
git add internal/branchcut cmd/verdi/close.go cmd/verdi/closefeature.go cmd/verdi/close_test.go
git commit -m "Extract close's branch-cut unwind into internal/branchcut"
```

GREEN for Task 1: `gofmt -l ./internal/gitx ./internal/branchcut ./cmd/verdi`; `go vet ./internal/gitx ./internal/branchcut ./cmd/verdi`; `golangci-lint run ./internal/gitx ./internal/branchcut`; `go test -race -count=1 ./internal/gitx ./internal/branchcut ./internal/residue ./internal/reclaim ./internal/wtmanager`; `go test -count=1 -run 'TestRunClose|TestCloseFeature|TestClosePreflight|TestRunPrepare' ./cmd/verdi -timeout 15m`; `go test -count=1 -run TestVocabProseWitness ./internal/specalign`.

---

### Task 2: The recovery projection — schema, codec, facts, recognizers (ac-8, Tier 2)

**Files:**
- Create: `internal/recovery/schema.go`, `codec.go`, `facts.go`, `derive.go`, `commandlog.go`, `schema_test.go`, `codec_test.go`, `facts_test.go`, `derive_test.go`, `commandlog_test.go`, `fixtures_test.go`, `commandsurface_test.go`, `internal/filelock/inspect.go`, `internal/filelock/inspect_test.go`.
- Modify: nothing outside these.

**Interfaces:**
- Consumes: `gitx.CurrentBranch, RevParse, MergeBase, DefaultBranch, LocalBranches, StagedPaths, WorktreeChangedPaths, StatusDirty, LsTree, HasRemoteTrackingBranch, AheadBehind, WorktreeList` (all read-only), `filelock.Inspect` (this task), `store.Open/DraftMutationJournalPath/DraftMutationDir/WriterLockPath/ActiveSpecRelPath/ArchiveSpecDir`, `wtmanager.WorktreePath`, `execworkspace.ExecutionRoot/ClassifyEntry`, `residue.Scan` (read-only), `reclaim.Compute` + `Plan.DryRunRows` (pure), `artifact.ParseRef`, `canonjson.Marshal/Digest`.
- Produces:

```go
package filelock

type LockStatus string

const (
	LockAbsent      LockStatus = "absent"
	LockHeld        LockStatus = "held"
	LockStale       LockStatus = "stale"
	LockUndecidable LockStatus = "undecidable"
)

// Inspection is Inspect's read-only answer. Reason is a sentence for
// LockStale (the liveness evidence) and LockUndecidable (why the probe
// could not decide), "" otherwise.
type Inspection struct {
	Status LockStatus
	Info   Info
	Reason string
}

// Inspect reads path without acquiring or taking over anything. It
// distinguishes the states Peek deliberately collapses (R-RR3-6): a
// complete body whose pid is not alive is LockStale; a young empty or
// partial body is LockHeld (mid-flush, exactly Peek's charity); an old
// empty body is LockStale with reason "empty lock body older than 2s";
// a live pid whose start time cannot be cross-checked (ps unparseable)
// is LockUndecidable with the ps error as reason — never reported stale.
func Inspect(path string) (Inspection, error)
```

```go
package recovery

const SchemaID = "verdi.recovery-projection/v1"

type StateCode string

const (
	StateEmptyBranchCut            StateCode = "empty-branch-cut"
	StateScaffoldUnstaged          StateCode = "scaffold-unstaged"
	StateArtifactsStagedUncommitted StateCode = "artifacts-staged-uncommitted"
	StateArchiveMoveUncommitted    StateCode = "archive-move-uncommitted"
	StateClosureUnpublished        StateCode = "closure-unpublished"
	StateBoardPushFailed           StateCode = "board-push-failed"
	StateStaleLock                 StateCode = "stale-lock"
	StateGovernedActionInterrupted StateCode = "governed-action-interrupted"
	StateStrandedResidue           StateCode = "stranded-residue"
	StateUnrecognized              StateCode = "unrecognized"
)

type Scope string // "ref" | "store"
type Reversibility string // "reversible" | "irreversible" | "none-needed"

type Uncertainty struct {
	Text    string `json:"text"`
	Witness string `json:"witness"` // the command or observation that would settle it
}

type Choice struct {
	ID             string        `json:"id"`
	Summary        string        `json:"summary"`
	Preconditions  []string      `json:"preconditions"`
	Effects        []string      `json:"effects"`
	Reversibility  Reversibility `json:"reversibility"`
	Confirmation   string        `json:"confirmation"`   // "--apply <id>" for executable choices; "none: no executor" otherwise
	Postconditions []string      `json:"postconditions"`
	Executor       string        `json:"executor"`       // "branchcut.Unwind" | "reclaim.Apply" | "none"
	ManualCommands []string      `json:"manual_commands"` // exact command lines; empty for executable choices
}

type RecognizedState struct {
	Code           StateCode     `json:"code"`
	Scope          Scope         `json:"scope"`
	Target         string        `json:"target"` // branch, lock path, journal path, workspace id
	Facts          []string      `json:"facts"`
	Uncertainties  []Uncertainty `json:"uncertainties"`
	StepsCompleted []string      `json:"steps_completed"`
	InvariantsHeld []string      `json:"invariants_held"`
	Choices        []Choice      `json:"choices"`
}

type Projection struct {
	Schema      string            `json:"schema"`
	Ref         string            `json:"ref"`
	Branch      string            `json:"branch"`
	Head        string            `json:"head"`
	States      []RecognizedState `json:"states"`      // sorted by (code, target)
	Disclosures []string          `json:"disclosures"` // facts that could not be gathered (co-6)
	Digest      string            `json:"digest"`
}

func (p Projection) Validate() error
func (p Projection) Recognized() bool // len(States) > 0; an `unrecognized` state counts (ac-8: something is recognized as outside the inventory — exit 1)

func Canonical(p Projection) ([]byte, error)
func Decode(data []byte) (Projection, error)

type Facts struct { /* one field per gathered observation; see facts.go */ }

type Gatherer struct{ /* unexported seams for tests */ }
func NewGatherer() Gatherer
func (g Gatherer) Gather(ctx context.Context, cfg *store.Config, ref string) (Facts, error)

func Derive(f Facts) Projection

// CommandLog records every gitx invocation on a context (an
// gitx.Observer). Forbidden returns the recorded commands that contain
// any of ForbiddenTokens as a whole argv element or an argv prefix
// (`--force` matches `--force` and `--force-with-lease`).
type CommandLog struct{ mu sync.Mutex; entries [][]string }
func (l *CommandLog) Observe(dir string, args []string)
func (l *CommandLog) Entries() [][]string
var ForbiddenTokens = []string{"reset", "restore", "clean", "stash", "--force", "update-ref"}
func (l *CommandLog) Forbidden() [][]string
```

- [ ] **Step 1: Failing filelock.Inspect tests**

`internal/filelock/inspect_test.go` (package `filelock`, so `psLstart` is reachable):

```go
func TestInspect_Table(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name   string
		setup  func(t *testing.T) string
		ps     func(pid int) (time.Time, error)
		want   LockStatus
		reason string
	}{
		{"absent", func(t *testing.T) string { return filepath.Join(dir, "absent.lock") }, nil, LockAbsent, ""},
		{"held by this process", func(t *testing.T) string {
			p := filepath.Join(dir, "held.lock")
			writeLockInfo(t, p, Info{PID: os.Getpid(), Start: time.Now().Unix()})
			return p
		}, nil, LockHeld, ""},
		{"stale dead pid", func(t *testing.T) string {
			p := filepath.Join(dir, "stale.lock")
			writeLockInfo(t, p, Info{PID: deadPID(t), Start: 1})
			return p
		}, nil, LockStale, "pid"},
		{"undecidable ps", func(t *testing.T) string {
			p := filepath.Join(dir, "undecidable.lock")
			writeLockInfo(t, p, Info{PID: os.Getpid(), Start: 1})
			return p
		}, func(int) (time.Time, error) { return time.Time{}, errors.New("ps unavailable") }, LockUndecidable, "ps unavailable"},
		{"old empty body", func(t *testing.T) string {
			p := filepath.Join(dir, "empty.lock")
			if err := os.WriteFile(p, nil, 0o644); err != nil { t.Fatal(err) }
			old := time.Now().Add(-time.Minute)
			if err := os.Chtimes(p, old, old); err != nil { t.Fatal(err) }
			return p
		}, nil, LockStale, "empty lock body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ps != nil {
				orig := psLstart
				psLstart = tc.ps
				t.Cleanup(func() { psLstart = orig })
			}
			got, err := Inspect(tc.setup(t))
			if err != nil { t.Fatal(err) }
			if got.Status != tc.want || !strings.Contains(got.Reason, tc.reason) {
				t.Fatalf("got %+v, want %s/%q", got, tc.want, tc.reason)
			}
		})
	}
}
```

`writeLockInfo` marshals `Info` as JSON to the path; `deadPID` starts and waits a `true` subprocess and returns its pid (the existing filelock tests have a helper for a dead pid — reuse its name if present). Add a malformed-body negative case expecting an error, mirroring `Peek`'s.

- [ ] **Step 2: RED**

Run: `go test ./internal/filelock -run TestInspect -v`
Expected: FAIL — `Inspect` undefined.

- [ ] **Step 3: Implement Inspect** in `internal/filelock/inspect.go`, reusing `decodeLockInfo`, `lockBodyIncomplete`, `lockFileYoung`, and a split of `alive` into `probe(pid, start) (alive bool, decided bool, reason string)` so `alive` keeps its documented fallback while `Inspect` can report undecidable. `Peek`'s behavior and tests must not change.

- [ ] **Step 4: GREEN** `go test -race ./internal/filelock`; commit `Add filelock.Inspect: stale, held, absent, undecidable (SI-218)`.

- [ ] **Step 5: Failing schema/codec tests**

`schema_test.go`: a `validProjection(t)` builder (mirror `internal/journey/record_test.go`'s `validRecord`), a positive `Validate` case, and a negative table: schema not `SchemaID`; states unsorted; duplicate (code,target); a choice with `Executor != "none"` and non-empty `ManualCommands`; a choice with `Executor == "none"` and empty `ManualCommands`; an executable choice whose ID lacks its target after the colon; an uncertainty with empty witness; a control character in any string; empty `Head`; a code outside the closed set. `codec_test.go`: `Canonical` is deterministic and `Decode(Canonical(p))` round-trips; a byte flipped after `Canonical` fails `Decode` on the digest; unknown field rejected; trailing data rejected.

- [ ] **Step 6: RED** `go test ./internal/recovery -run 'TestValidate|TestCanonical|TestDecode'` — package missing.

- [ ] **Step 7: Implement `schema.go` and `codec.go`** exactly on `internal/journey/codec.go`'s shape (`Validate` → zero digest → `canonjson.Digest` → `canonjson.Marshal`; strict decode with digest recomputation). `Validate` sorts nothing: it REJECTS unsorted input (determinism is the producer's job, as `readinesspilot.Snapshot.Validate` does).

- [ ] **Step 8: GREEN**; commit `Add the recovery projection schema and codec`.

- [ ] **Step 9: Failing CommandLog tests**

```go
func TestCommandLog_ForbiddenTokens(t *testing.T) {
	var l CommandLog
	l.Observe("/r", []string{"rev-parse", "HEAD"})
	l.Observe("/r", []string{"branch", "-d", "close/x"})
	l.Observe("/r", []string{"push", "--force-with-lease"})
	l.Observe("/r", []string{"restore", "--staged", "a"})
	got := l.Forbidden()
	want := [][]string{{"push", "--force-with-lease"}, {"restore", "--staged", "a"}}
	if !reflect.DeepEqual(got, want) { t.Fatalf("got %v", got) }
	if len(l.Entries()) != 4 { t.Fatal("entries lost") }
}
```

Also: `commit -m "reset the counter"` is NOT forbidden (the token check is over argv elements, never message text — assert it).

- [ ] **Step 10: RED, implement `commandlog.go`, GREEN, commit** `Add the recovery command log over the gitx observer seam`.

- [ ] **Step 11: Failing fixture + facts tests**

`fixtures_test.go` builds a store repository with `fixturegit.Build` whose layers write `verdi.yaml` (copy the minimal manifest `internal/journey/fixtures_test.go` uses — read it), one active feature spec `spec/checkout` at `.verdi/specs/active/checkout/spec.md` (copy `internal/journey`'s `checkoutSpec()` frontmatter), and provides helpers that put the repo into each interrupted state through gitx and the filesystem only:

```go
func fixtureStore(t *testing.T) (*fixturegit.Repo, *store.Config)
func cutEmptyBranch(t *testing.T, repo *fixturegit.Repo, name string) (cutPoint string)      // gitx.CheckoutNewBranch(name); stays checked out
func writeUnstagedScaffold(t *testing.T, repo *fixturegit.Repo)                                // on design/checkout: append a line to the spec; do not stage
func stageClosurePaths(t *testing.T, repo *fixturegit.Repo)                                    // on close/checkout: move active→archive on disk and git add both paths (the shape close.go's closureResidueName recognizes: read close.go:1016-1058 for the exact two zone prefixes)
func moveArchiveUncommitted(t *testing.T, repo *fixturegit.Repo)                               // rename the spec dir active→archive on disk only
func commitClosureNoRemote(t *testing.T, repo *fixturegit.Repo)                                // on close/checkout: stage and commit the move; no remote-tracking branch exists
func writeStaleWriterLock(t *testing.T, repo *fixturegit.Repo) string                          // store.WriterLockPath with a dead pid
func writePreparedJournal(t *testing.T, repo *fixturegit.Repo)                                 // store.DraftMutationJournalPath(root,"checkout") with {"schema":"verdi.draftmutation-journal/v1","spec":"spec/checkout","phase":"prepared"} — read internal/draftmutation/transaction.go:290-310 for the exact journal document and write the same fields
func writeOrphanWorkspaceStaging(t *testing.T, repo *fixturegit.Repo) string                    // a <id>.request.staging file under execworkspace.ExecutionRoot with no <id> directory
```

Fixture variants the spike showed are load-bearing (each state helper takes a variant): (i) cut from the default branch with `origin/HEAD` set AND a concrete `refs/remotes/origin/<branch>` object (the residue helper sets only the symref, which does not trigger `resolveBranchBase`'s remote-preferred path — set both); (ii) cut from a non-default current branch two commits ahead of `main` (the case that breaks any single-base ahead-count); (iii) no origin remote at all; (iv) `CI_DEFAULT_BRANCH` set with no remote; plus the negatives: the ritual branch with one own commit (must not read empty) and the base moved on after the cut (still empty).

`facts_test.go`: a table where each helper is applied and `Gather` returns the expected fields set (branch names, tips, merge-bases, staged/changed paths, lock inspections, journal presence, workspace entries) and every other field zero; plus: the default branch unresolvable (no `origin/HEAD` and no `main`) → `Facts.DefaultBranch == ""` and `Facts.Disclosures` names it; a symlinked journal path → disclosed, not followed.

- [ ] **Step 12: RED, implement `facts.go`, GREEN**

`Gather` resolves `ref` with `artifact.ParseRef` (spec refs only; a story ref or component is an operational error naming the class through `mdl.DisplayClass`), then gathers, in this order, each guarded so one failure becomes a disclosure and the rest proceeds: current branch + HEAD; every local branch with its tip (`gitx.LocalBranches` + `RevParse`); the resolved default-branch base exactly as `resolveBranchBase` computes it (move that function's read-only resolution into `internal/gitx` or a tiny `internal/branchbase` package so cmd/verdi and recovery share one rule — record the move in the Task 2 report); for each of `design/<name>`, `feature/<name>`, `close/<name>`, `policy/adopt`: exists? tip, the set of other local branches whose tip equals or descends from it (`IsAncestor`), remote-tracking presence, ahead/behind; `gitx.StagedPaths`, `gitx.WorktreeChangedPaths`, `gitx.StatusDirty`; active/archive spec directory presence on disk and in HEAD's tree (`gitx.LsTree`); `filelock.Inspect` over `store.WriterLockPath(root)`, `wtmanager.WorktreePath(root, b)+".lock"` for each ritual branch found, and every `execworkspace.ExecutionRoot` entry whose form is the lock sibling; the draft-mutation journal for the name (read the bytes; decode ONLY `{schema, spec, phase}` with a permissive decoder, since the journal carries more fields than the projection needs, and record the raw phase; list `DraftMutationDir`'s entries as the steps evidence); `execworkspace` entries classified; `residue.Scan(ctx, root, defaultBranchRef)` when the default branch resolved, then `reclaim.Compute(res, root, currentBranch, defaultBranch)` → keep the `Plan` and its `DryRunRows()` for the ref's units only (branch names `design/<name>|feature/<name>|close/<name>`).

Commit `Gather the recovery projection's facts read-only`.

- [ ] **Step 13: Failing recognizer tests**

`derive_test.go`: one table per state driving `Derive` with hand-built `Facts` (no I/O), each with a positive case, a cleared case, and the assertions ac-8 requires — the state's `Code`, `Target`, non-empty `Facts`, every `Uncertainty` with a witness, `StepsCompleted`, `InvariantsHeld`, and the choices:

- `empty-branch-cut`: another local branch's tip equals or descends from the ritual tip (R-RR3-5) ⇒ state with ONE executable choice `unwind-branch-cut:<branch>` whose `Preconditions` are exactly `["<branch> still points at <cutPoint>", "index is empty", "working tree is clean", "<originalBranch> resolves"]`, `Effects` `["switch back to <originalBranch>", "delete <branch> with git branch -d"]`, `Reversibility` `"none-needed"` (the branch carried no commits; re-run the ritual to recut), `Postconditions` `["<branch> does not exist", "current branch is <originalBranch>", "HEAD is <cutPoint>"]`, `Executor` `"branchcut.Unwind"`. When `Facts.StagedPaths` or the on-disk archive move is present for the same name, the choice is withheld (R-RR3-8) and an uncertainty names the other state. When the return branch is undecidable (R-RR3-5): no choice; the uncertainty lists the candidate branches, or for design/policy branches names the witness `git remote set-head origin --auto` / `CI_DEFAULT_BRANCH` (the two inputs `resolveBranchBase` reads — cite the real one from `cmd/verdi/policy.go:255`).
- `scaffold-unstaged`: design branch checked out, changed paths under the active spec dir, none staged ⇒ manual choice with `ManualCommands` `["git add -- .verdi/specs/active/<name>/", "git commit -m \"design: <name>\""]` and `Executor: "none"`.
- `artifacts-staged-uncommitted`: staged paths match `closureResidueName`'s shape ⇒ manual choice carrying close's own advice text (the exact strings from `closureResidueRefusal`; copy them into the recognizer as constants with a comment pointing at `close.go:1059` and a test that pins equality against those constants — do not import package main).
- `archive-move-uncommitted`: archive dir on disk, active dir absent on disk, both unchanged in HEAD's tree, nothing staged ⇒ manual choice with `reportUncommittedArchiveMove`'s commands.
- `closure-unpublished`: `close/<name>` has a commit beyond merge-base AND (no remote-tracking branch OR ahead > 0) ⇒ manual choice `["git push -u origin close/<name>"]`, uncertainty per R-RR3-12.
- `board-push-failed`: `design/<name>` ahead of its remote-tracking branch, or no remote-tracking branch while the branch has commits beyond merge-base ⇒ manual choice `["git push -u origin design/<name>"]`, the R-RR3-12 uncertainty AND the R-RR3-13 uncertainty (a push is not observable).
- `stale-lock`: an inspection `LockStale` ⇒ the state with a manual choice `["rm <path>"]` and facts `pid`, `start`, reason. `LockUndecidable` never becomes a `stale-lock` state: it becomes an entry in `Projection.Disclosures` naming the path, the pid, and the witness `ps -o lstart= -p <pid>` (co-6: an undecidable state is disclosed with the witness that would decide it). Test both.
- `governed-action-interrupted`: journal phase `prepared` ⇒ state with target the journal path, `StepsCompleted` from the directory listing, manual choice `["<the next board draft write: internal/draftmutation's service calls LockedWriter.Recover on open (service.go:59) and completes or rolls back the journal>"]` rendered as the sentence `next board draft write completes or rolls back this journal (internal/draftmutation.LockedWriter.Recover)`, executor none; orphan workspace staging ⇒ state with target the workspace id, manual choice naming `verdi gc` (read `internal/execworkspace`'s GC slice doc for the exact verb form).
- `stranded-residue` (R-RR3-14: withheld entirely when the residue scan reports unproven specs; the projection discloses them): one state per reclaim unit for this ref with `Choices` = one executable `reclaim:<branch>` whose `Effects` are the unit's `DryRunRows()` lines rendered by `Row.Line()`, `Executor` `"reclaim.Apply"`, preconditions `["reclaim plan still lists <branch> as eligible"]`, postconditions `["<branch> does not exist", "<worktree path> does not exist"]` (the second only when the unit has a worktree); a kept (ineligible) unit yields a state with NO executable choice and the kept reason as a fact.
- `unrecognized`: a ritual branch that exists with commits beyond its cut and matches no other recognizer, or a ref-scoped observation the recognizers do not classify ⇒ exactly one `unrecognized` state listing the facts seen and the uncertainty "state is outside the ritual inventory (dc-7)".
- Ordering: `States` sorted by `(Code, Target)`; `Derive` output validates.

- [ ] **Step 14: RED, implement `derive.go`, GREEN**

Each recognizer is `func recognizeX(f Facts) []RecognizedState`; `Derive` concatenates, applies R-RR3-8, sorts, and returns. Every string with a class or status word routes through `f.Model.DisplayClass` or carries the vocab marker.

Commit `Derive the recovery projection's recognized states`.

- [ ] **Step 15: The AST gate**

`commandsurface_test.go`: copy `internal/residue/commandsurface_test.go`'s walker; allow-list for `facts.go`, `derive.go`, `schema.go`, `codec.go`, `commandlog.go`: `CurrentBranch, RevParse, MergeBase, DefaultBranch, LocalBranches, StagedPaths, WorktreeChangedPaths, StatusDirty, LsTree, HasRemoteTrackingBranch, AheadBehind, WorktreeList`; `apply.go` (Task 4) additionally `branchcut.Unwind` and `reclaim.Apply` only. The test fails on any other `gitx.<Ident>` or on any direct `exec.Command`/`os/exec` import in the package. Commit `Gate the recovery package's git command surface`.

GREEN for Task 2: `gofmt -l ./internal/recovery ./internal/filelock`; `go vet ./internal/recovery ./internal/filelock`; `golangci-lint run ./internal/recovery ./internal/filelock`; `go test -race -count=1 ./internal/recovery ./internal/filelock ./internal/wtmanager ./internal/reclaim`; `go test -count=1 -run TestVocabProseWitness ./internal/specalign`.

---

### Task 3: `verdi recover` (read path), MCP `get_recovery`, registries, showcase, docs (ac-8, ac-10's MCP half, Tier 2)

**Files:**
- Create: `cmd/verdi/recover.go`, `cmd/verdi/recover_test.go`, `cmd/verdi/recover_e2e_test.go`, `internal/mcpserve/tool_get_recovery.go`, `internal/mcpserve/tool_get_recovery_test.go`.
- Modify: `cmd/verdi/dispatch.go:18-48` (`"recover": 27` with a comment citing spec/readiness-recovery-v2 ac-8..ac-10), `dispatch.go:44-49` (`usage` gains `recover`), the dispatch arm after `"journey"` (`dispatch.go:210-212`): `if verb == "recover" { return cmdRecover(args[1:], os.Stdout, stderr) }`; `cmd/verdi/help.go:77` (`recover          diagnose an interrupted lifecycle state and offer its safe choices`) and `help.go:153` (`"recover": recoverUsage`); `internal/mcpserve/backend.go:24-45` (`RecoveryLoader` port), `tooldefs.go` (after `get_document`), `server.go:103-104` (`case "get_recovery"`); `cmd/verdi/mcp.go` (wire `RecoveryLoader: recovery.Loader{...}` next to `ReadinessLoader`); `internal/specalign/verbs_test.go:175-179` (`"recover"` appended; bare `verdi recover` fails on argument shape, so no hermeticity special case — assert that in `recover_test.go`); `internal/specalign/mcptools_test.go:112` (`"get_recovery"` appended); `internal/showcasealign/coverage_test.go` (`"cli:recover"` and `"mcp:get_recovery"` rows next to `cli:journey`/`mcp:get_document`), `cli_showcase_test.go` (`TestCLIShowcaseRecover`), `mcp_showcase_test.go` (`TestMCPShowcaseGetRecovery`); `verdi/CLAUDE.md:58` (append `recover` (spec/readiness-recovery-v2 ac-8..ac-10) to the real-verbs sentence); `README.md` CLI table (one row); `docs/guide-claims.yaml` (a `7.6-recover` entry, `status: EXISTS`, witnesses `TestRecover_Table` and `TestRecoverE2E_EmptyBranchCut`); `docs/architecture-and-journeys.md` (one paragraph under the readiness/recovery section naming the projection and the two executors).

**Interfaces:**
- Consumes: `recovery.NewGatherer().Gather`, `recovery.Derive`, `recovery.Canonical`, `recovery.CommandLog`, `gitx.WithObserver`.
- Produces:

```go
package recovery

// Loader is the production RecoveryLoader for MCP and the CLI's read path.
type Loader struct{ Root string }
func (l Loader) Load(ctx context.Context, ref string) (Projection, error) // store.Open → Gather → Derive
```

```go
package mcpserve

type RecoveryLoader interface {
	Load(ctx context.Context, ref string) (recovery.Projection, error)
}
// Backend gains: RecoveryLoader RecoveryLoader (nil = "recovery projection not supplied", a toolError, never a fabricated empty projection)
func (b *Backend) GetRecovery(ctx context.Context, argsRaw json.RawMessage) map[string]any // args: {ref string}; result: the canonical projection object; a request with any other field is a strict-decode refusal
```

```go
package main

// vocab:identity — CLI usage grammar
const recoverUsage = "usage: verdi recover [--json] <spec-ref> [--apply <choice-id>]"
func cmdRecover(args []string, stdout, stderr io.Writer) int
```

- [ ] **Step 1: Failing CLI tests**

`recover_test.go` (unit, `cmdRecover` called directly with a fixture store from Task 2's helpers — export a copy of `fixtureStore` as `recoverFixtureStore` in `recover_test.go`):

```go
func TestRecover_Table(t *testing.T) {
	cases := []struct {
		name     string
		prepare  func(t *testing.T, repo *fixturegit.Repo)
		wantExit int
		wantCode string // a state code expected in stdout, "" for none
	}{
		{"nothing recognized", func(*testing.T, *fixturegit.Repo) {}, 0, ""},
		{"empty branch cut", func(t *testing.T, r *fixturegit.Repo) { cutEmptyBranch(t, r, "close/checkout") }, 1, "empty-branch-cut"},
		{"stale writer lock", func(t *testing.T, r *fixturegit.Repo) { writeStaleWriterLock(t, r) }, 1, "stale-lock"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := recoverFixtureStore(t)
			tc.prepare(t, repo)
			var stdout, stderr bytes.Buffer
			code := runInDir(t, repo.Dir, func() int { return cmdRecover([]string{"--json", "spec/checkout"}, &stdout, &stderr) })
			if code != tc.wantExit { t.Fatalf("exit %d, want %d; stderr %s", code, tc.wantExit, stderr.String()) }
			p, err := recovery.Decode(bytes.TrimRight(stdout.Bytes(), "\n"))
			if err != nil { t.Fatal(err) }
			if tc.wantCode == "" && len(p.States) != 0 { t.Fatalf("states %+v", p.States) }
			if tc.wantCode != "" && !hasState(p, recovery.StateCode(tc.wantCode)) { t.Fatalf("no %s in %+v", tc.wantCode, p.States) }
		})
	}
}

func TestRecover_UsageBeforeStore(t *testing.T) {
	// bare, malformed, and --apply-without-ref all fail on shape with exit 2 and the usage line, in a dir with no store
	for _, args := range [][]string{{}, {"--apply"}, {"--apply", "x"}, {"spec/a", "spec/b"}, {"--json"}} {
		var stderr bytes.Buffer
		if code := runInDir(t, t.TempDir(), func() int { return cmdRecover(args, io.Discard, &stderr) }); code != 2 || !strings.Contains(stderr.String(), recoverUsage) {
			t.Fatalf("args %v: exit %d stderr %q", args, code, stderr.String())
		}
	}
}
```

`runInDir` chdirs for the call (the existing cmd/verdi tests have such a helper — grep `func withWorkingDir\|func runIn` and reuse it). `--apply` in this task: the full grammar is parsed here, and `cmdRecover` calls `recovery.Apply`, which Task 4 implements. To keep Task 3 independently green, Task 3 ships `internal/recovery/apply.go` containing only `var ErrNotImplemented = errors.New("recovery: --apply lands in Task 4 of the wave-3 plan")` and `func Apply(...) (Outcome, error) { return Outcome{}, ErrNotImplemented }` with the Task 4 signature, and `cmdRecover` maps that error to exit 2 with its text; Task 4 replaces the stub. The e2e case for the stub asserts exit 2 and is deleted by Task 4.

- [ ] **Step 2: RED** `go test ./cmd/verdi -run 'TestRecover' -v`.

- [ ] **Step 3: Implement `recover.go`**

Package comment: the exit doctrine (R-RR3-1) contrasted with `journey.go`'s. Body: parse args (shape first, before `store.FindRoot`), `store.FindRoot(".")`, `store.Open`, attach a `recovery.CommandLog` via `gitx.WithObserver`, `recovery.Loader{Root: root}.Load(ctx, ref)` (or `Apply` under `--apply`), print `recovery.Canonical` as one line (the `--json` and no-flag forms are byte-identical, as journey's are), after the run check `log.Forbidden()` (R-RR3-10: non-empty ⇒ print each offending argv on stderr as `recover: forbidden git command issued: git <argv>` and exit 2), honor `VERDI_RECOVERY_GITLOG` by appending `dir\targv...` lines. Exit per R-RR3-1. Errors prefixed `recover: ` exactly once (journey's `journeyErr` pattern).

- [ ] **Step 4: GREEN**, commit `Add verdi recover's read path`.

- [ ] **Step 5: Failing MCP tests**

`tool_get_recovery_test.go`: a fake `RecoveryLoader` returning a fixed projection ⇒ `GetRecovery` returns it as the tool result's JSON (decode with `recovery.Decode` after re-marshaling `content[0].text`); nil loader ⇒ `toolError` containing "recovery projection not supplied"; args with an extra field `{"ref":"spec/x","apply":"y"}` ⇒ strict-decode `toolError`; a story ref ⇒ `toolError`; the `callTool` switch reaches it (`TestServer_CallsGetRecovery` following the existing `get_document` server test's shape).

- [ ] **Step 6: RED, implement, GREEN**, commit `Add the read-only MCP tool get_recovery`.

- [ ] **Step 7: Registries and inventories** — make the edits listed under Files, then:

Run: `go test -count=1 ./internal/specalign -run 'TestV0CLIVerbInventory|TestMCPToolInventory|TestVocabProseWitness' && go test -count=1 ./internal/showcasealign -run 'TestShowcaseCoverage|TestCLIShowcaseRecover|TestMCPShowcaseGetRecovery' && go test -count=1 ./internal/mcpserve`
Expected: PASS. The showcase tests run `recover` against `provisionShowcaseStore` for `spec/stale-decline` (exit 0 or 1 — assert the schema tag and one canonical line, as `TestCLIShowcaseJourney` does) and `get_recovery` through the showcase MCP backend.

- [ ] **Step 8: Built-binary e2e** `recover_e2e_test.go` (`buildVerdiBinary`/`runVerdiBinary` from `document_parity_e2e_test.go`): the empty-branch-cut fixture ⇒ exit 1, one canonical line, `VERDI_RECOVERY_GITLOG` file contains no forbidden token; the stub `--apply` ⇒ exit 2 (deleted in Task 4). Add `./cmd/verdi` is already in the race run; nothing to add to `CROSS_BINARY_PKGS`.

- [ ] **Step 9: Docs** (`README.md`, `guide-claims.yaml`, `architecture-and-journeys.md`, `verdi/CLAUDE.md`) and `go test -count=1 ./internal/specalign` (docsync + instruction-file witnesses). Commit `Register verdi recover and get_recovery in every inventory`.

GREEN for Task 3: `gofmt -l ./cmd/verdi ./internal/mcpserve ./internal/recovery`; `go vet ./cmd/verdi ./internal/mcpserve ./internal/recovery`; `golangci-lint run ./internal/mcpserve ./internal/recovery`; `go test -race -count=1 ./internal/mcpserve ./internal/recovery`; `go test -count=1 -run 'TestRecover|TestDocumentParity|TestCmdJourney' ./cmd/verdi -timeout 15m`; `go test -count=1 ./internal/specalign`; `go test -count=1 ./internal/showcasealign`; `make lint-store` (harness drift: run `verdi harness render` if `harness check` reports drift and commit the rendered skills).

---

### Task 4: The two executors and the apply protocol (ac-9, ac-10, **Tier 3 — owner risk gate**)

**Files:**
- Create: `internal/recovery/apply.go` (replaces Task 3's stub), `apply_test.go`.
- Modify: `cmd/verdi/recover.go` (`--apply` path prints postconditions; the stub mapping removed), `recover_e2e_test.go` (apply cases replace the stub case), `internal/recovery/commandsurface_test.go` (apply.go's allow-list).

**Interfaces:**
- Consumes: `branchcut.Unwind`, `reclaim.Compute`, `reclaim.Apply`, `residue.Scan`, `journey.NewProjector().Project`, `Gather`, `Derive`.
- Produces:

```go
package recovery

var ErrNoExecutor = errors.New("recovery: choice has no executor")
var ErrPreconditionFailed = errors.New("recovery: precondition no longer holds")
var ErrUnknownChoice = errors.New("recovery: unknown choice")

// Outcome is what --apply observed: the choice, each postcondition with
// its observed value, the re-derived projection, and the re-derived
// journey (nil when the journey re-derivation failed; Err carries why).
type Outcome struct {
	ChoiceID       string
	Postconditions []PostconditionResult // {Text, Held bool, Observed string}
	After          Projection
	Journey        *journey.Record
	JourneyErr     error
}

// Apply implements R-RR3-9: derive fresh → find the choice → refuse a
// non-executable one (ErrNoExecutor with the manual commands in the
// error text) → re-prove preconditions from a fresh Gather → execute →
// re-derive → check postconditions. It never executes anything for an
// unknown id or a failed precondition (those errors wrap the sentinel and
// name the id / the failed precondition); nothing is written before the
// executor runs.
func Apply(ctx context.Context, cfg *store.Config, ref, choiceID string, stderr io.Writer) (Outcome, error)
```

- [ ] **Step 1: Failing apply tests** (`apply_test.go`, fixtures from Task 2):

```go
func TestApply_UnwindEmptyBranchCut(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cut := cutEmptyBranch(t, repo, "close/checkout") // leaves close/checkout checked out; cut == repo.Head
	out, err := Apply(context.Background(), cfg, "spec/checkout", "unwind-branch-cut:close/checkout", io.Discard)
	if err != nil { t.Fatal(err) }
	for _, pc := range out.Postconditions { if !pc.Held { t.Fatalf("postcondition failed: %+v", pc) } }
	if hasState(out.After, StateEmptyBranchCut) { t.Fatal("state still recognized after unwind") }
	if out.Journey == nil || out.JourneyErr != nil { t.Fatalf("journey not re-derived: %v", out.JourneyErr) }
	if cur, _ := gitx.CurrentBranch(context.Background(), repo.Dir); cur != "main" { t.Fatalf("branch %q", cur) }
	_ = cut
}

func TestApply_RefusesWhenPreconditionMoved(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")
	// The re-prove gather and the execution are adjacent by construction, so
	// the precondition is broken BEFORE Apply: commit on the branch, then
	// assert Apply refuses and the branch is untouched.
	fixturegit.Dangle(t, repo, map[string]string{"c.txt": "c\n"}, "own commit") // or AddPaths+CreateCommit on close/checkout
	before := gitOutput(t, repo.Dir, "rev-parse", "close/checkout")
	_, err := Apply(context.Background(), cfg, "spec/checkout", "unwind-branch-cut:close/checkout", io.Discard)
	if !errors.Is(err, ErrPreconditionFailed) { t.Fatalf("err = %v", err) }
	if gitOutput(t, repo.Dir, "rev-parse", "close/checkout") != before { t.Fatal("branch changed") }
}

func TestApply_RefusesNoExecutor(t *testing.T) {
	repo, cfg := fixtureStore(t)
	writeStaleWriterLock(t, repo)
	p := Derive(mustGather(t, cfg, "spec/checkout"))
	id := p.States[0].Choices[0].ID
	_, err := Apply(context.Background(), cfg, "spec/checkout", id, io.Discard)
	if !errors.Is(err, ErrNoExecutor) || !strings.Contains(err.Error(), "rm ") { t.Fatalf("err = %v", err) }
	if _, statErr := os.Stat(store.WriterLockPath(repo.Dir)); statErr != nil { t.Fatal("lock removed by a choice with no executor") }
}

func TestApply_UnknownChoiceListsKnown(t *testing.T) { /* ErrUnknownChoice; error text lists every choice id in the projection */ }

func TestApply_ReclaimDelegation(t *testing.T) {
	// a merged feature/checkout branch with a managed worktree (use wtmanager's own test helpers to create one, or plain gitx WorktreeAdd if exported): reclaim:feature/checkout applies; postconditions hold; reclaim's own Row lines appear in stderr verbatim
}

func TestApply_ReclaimRefusalSurfacesVerbatim(t *testing.T) {
	// dirty managed worktree ⇒ reclaim keeps it; Apply returns ErrPreconditionFailed wrapping the Row.Line() text; nothing removed
}

func TestApply_AmbiguityWithholdsUnwind(t *testing.T) {
	// empty cut + archive move on disk ⇒ Derive offers no unwind choice; Apply("unwind-branch-cut:close/checkout") is ErrUnknownChoice; nothing changed
}
```

- [ ] **Step 2: RED**, then implement `apply.go`:

```go
func Apply(ctx context.Context, cfg *store.Config, ref, choiceID string, stderr io.Writer) (Outcome, error) {
	g := NewGatherer()
	facts, err := g.Gather(ctx, cfg, ref)
	if err != nil { return Outcome{}, err }
	before := Derive(facts)
	choice, state, ok := findChoice(before, choiceID)
	if !ok { return Outcome{}, fmt.Errorf("%w: %q; known choices: %v", ErrUnknownChoice, choiceID, choiceIDs(before)) }
	if choice.Executor == "none" { return Outcome{}, fmt.Errorf("%w: %s; the manual commands are: %s", ErrNoExecutor, choiceID, strings.Join(choice.ManualCommands, "; ")) }
	fresh, err := g.Gather(ctx, cfg, ref) // re-prove immediately before execution (parent DC-13)
	if err != nil { return Outcome{}, err }
	if failed := reprove(choice, state, fresh); failed != "" { return Outcome{}, fmt.Errorf("%w: %s", ErrPreconditionFailed, failed) }
	switch choice.Executor {
	case "branchcut.Unwind": executeUnwind(ctx, cfg.Root, state, fresh, stderr) // Outcome != Unwound ⇒ the postcondition check below reports it; nothing else to do
	case "reclaim.Apply": executeReclaim(ctx, cfg.Root, state, fresh, stderr)
	default: return Outcome{}, fmt.Errorf("recovery: executor %q is not one of the two this feature admits (dc-4)", choice.Executor)
	}
	afterFacts, err := g.Gather(ctx, cfg, ref)
	if err != nil { return Outcome{}, err }
	out := Outcome{ChoiceID: choiceID, After: Derive(afterFacts), Postconditions: checkPostconditions(choice, afterFacts)}
	rec, jerr := journey.NewProjector().Project(ctx, cfg, ref)
	if jerr != nil { out.JourneyErr = jerr } else { out.Journey = &rec }
	return out, nil
}
```

`reprove` re-evaluates each precondition string against `fresh` by structured comparison (the precondition strings are generated from a small table so both sides use the same predicate; never string-match the sentence). `executeReclaim` recomputes `reclaim.Compute` on a fresh `residue.Scan`, keeps only the item for the choice's branch, and calls `reclaim.Apply(ctx, root, reclaim.Plan{Items: []reclaim.PlanItem{item}})`; a `KindKept`/`KindRefused` row is a failed postcondition carrying `Row.Line()`.

- [ ] **Step 3: GREEN**; `cmdRecover --apply`: print the postconditions as `postcondition: <text>: held|VIOLATED (<observed>)` lines on stdout before the canonical JSON of `out.After`, exit per R-RR3-1/R-RR3-9. Built-binary cases in `recover_e2e_test.go`: unwind happy path (exit 0, `VERDI_RECOVERY_GITLOG` has exactly the expected argv sequence: rev-parse, status/diff reads, checkout, branch -d, and no forbidden token); precondition moved (exit 1, log has no checkout/branch command); no executor (exit 1); MCP: a transcript proving `get_recovery` has no apply argument (strict-decode refusal).

- [ ] **Step 4: Update the AST gate** allow-list for `apply.go`; run it.

- [ ] **Step 5: Commit** `Execute the two recovery choices behind verdi recover --apply`.

GREEN for Task 4: Task 2's and Task 3's lists plus `go test -race -count=1 ./internal/recovery ./internal/branchcut ./internal/reclaim`; `go test -count=1 -run 'TestRecover|TestApply' ./cmd/verdi -timeout 15m`; `go test -count=1 ./internal/specalign -run TestVocabProseWitness`.

Review chain (dc-6 Tier 3): independent Opus lane review over Task 4's range; any Important/Critical finding ⇒ fresh Opus fixer + fresh Opus re-reviewer; then the whole-wave review and the owner risk gate (Task 6).

---

### Task 5: Surfaces ratification and ledger (controller-authored, spec-only)

**Files:**
- Modify: workspace-root `docs/design/specs/05-surfaces.md` (§CLI: a `verdi recover [--json] <spec-ref> [--apply <choice-id>]` row after `verdi attest`; §MCP: a `get_recovery` row after `get_document`), `08-revision-notes.md` (entry "Readiness-recovery wave 3 — recover verb and get_recovery tool (2026-09-…)", citing ac-8..ac-10 as the authorizing spec and the owner's in-wave ratification decision of 2026-09-21); `.verdi/specs/active/verdi-surfaces/spec.md` (the two rows byte-identical to the origin); `docs/superpowers/invention-ledger.md` (SI-218 filelock stale/absent; SI-219 seam ownership under co-4; SI-220 exit-1 divergence — recorded BEFORE Task 1; their State columns updated at wave close).

- [ ] Row texts are written from the shipped code (Task 3/4 heads), never asserted: grammar, the three exits, the eight state codes, the two executors, the manual-command posture, MCP read-only with no apply argument.
- [ ] `TestSelfHostedSpecFidelity` run through the transient `verdi-wt/docs` symlink (create, run, remove — never leave it): PASS.
- [ ] Commit `Ratify verdi recover and get_recovery into verdi-surfaces`.

---

### Task 6: Wave gate, whole-wave review, owner risk gate, report

- [ ] Controller pre-review of every lane ran its FULL GREEN list (recorded per task in the ledger).
- [ ] `go build ./... && gofmt -l . && go vet ./... && golangci-lint run ./...`; `VERDI_E2E_PORT_BASE=4590 make verify` → `verify OK` (serial; nothing else running); artifact scan empty; tree clean.
- [ ] Whole-wave Opus review over `d307821d..<head>`: cross-lane interfaces (seam ↔ command log ↔ CLI check; branchcut ↔ close pins), enum/state coverage of the eight codes, provenance of every commit, the AST gate's allow-lists against the diff, MCP has no apply path.
- [ ] Owner risk gate (dc-6, Tier 3): the owner reviews Task 4's range and the built-binary apply evidence; merge is not requested before it.
- [ ] Report `docs/superpowers/reports/2026-09-21-readiness-recovery-wave-3.md` (waves 1/2's shape); ledger states updated; return `READY_FOR_OWNER_RISK_GATE`.

---

## Self-review

**Spec coverage.** ac-8: Task 2 (schema with ref/branch/HEAD/states; each state's code, facts, uncertainties with witnesses, steps, invariants, choices; the eight states plus unrecognized under dc-7; ambiguity ⇒ diagnosis and witness request) and Task 3 (exit 0/1/2). ac-9: Task 2 (every choice's preconditions, effects, reversibility, confirmation, postconditions, executor or exact manual command), Task 4 (exactly two executors, re-proof at execution time, close's own sequence via `internal/branchcut`, reclaim under its own dry-run/apply, no new git primitive — the AST gate; the command-log token rule — Task 1's seam + Task 2's log + Task 3/4's runtime check). ac-10: Task 4 (re-derive projection and journey, print postconditions, refuse with nothing changed), Task 3 (`get_recovery` read-only, no apply path). dc-4/dc-7/co-2..co-6 in Global Constraints and R-RR3-7/8/11. ritual-write-scope-v2 dc-5/co-4: Task 1 and SI-219.

**Placeholders.** Every message string, id grammar, allow-list, and exit code is given; the two fixture-helper contracts the implementer must read before use are named with file:line (Dangle, closureResidueName's zone prefixes, the journal document, gitx.DefaultBranch's witness); no "TBD".

**Type consistency.** `recovery.Projection/RecognizedState/Choice` (Task 2) are what `cmdRecover` prints and `GetRecovery` returns (Task 3) and what `Apply` returns in `Outcome.After` (Task 4); `branchcut.Unwind`'s signature (Task 1) is what `executeUnwind` calls (Task 4); `filelock.Inspect`'s `LockStatus` values (Task 2) drive the `stale-lock` recognizer and its undecidable disclosure; `recovery.Loader.Load` (Task 3) is the `RecoveryLoader` port's one method.

**Known limits carried.** Remote comparisons read the last-fetched remote-tracking ref (R-RR3-12). Constitution proposals are recognized only as a branch cut (R-RR3-11). The workbench recovery surface is the post-design Fable lane's (co-4). `internal/gitx` has three exec sites (`run`, `ConfigValue`, `runStdin`); `observer_test` pins all three and counts them, so a fourth must call `observe` or the test fails.

## Amendments during execution

Recorded here by the controller as they happen; where a code block here disagrees with HEAD, HEAD is right.

- Task 1 (R-RR3-2 amended, review Important): the seam covers `runStdin` (plumbing.go) as well — the brief's premise that `ConfigValue` was the only exec site outside `run` was false; SI-219's text corrected in the same commit. The two dropped base assertions (silent clean unwind; branch survives an ahead-of-cut refusal), the detached-HEAD case, and `Outcome.String` ride in the same fix round.
- Task 2 (R-RR3-15, implementer disclosure 2): `gitx.Show` joins the read half's AST allow-list (read-only plumbing, already on `internal/residue`'s list). When the target spec is not on disk in the running checkout, `specClassAt` reads it from the ritual branch's own tree (`design/<name>`, then `close/<name>`) and then from the resolved default-branch base via `Show`; only when none of those hold is the gather an operational error naming every location tried. Cost if wrong: one allow-list entry and one fallback chain.
- Task 2 (R-RR3-16, implementer disclosure 3): Step 13's literal reading of `unrecognized` ("a ritual branch that exists with commits beyond its cut and matches no other recognizer") is withdrawn — such a branch is an ordinary in-progress ritual, not a recovery state, and reporting it would violate ac-8's "never a prediction" by naming healthy work as a problem. `unrecognized` is reserved for observations that contradict the ritual inventory (dc-7): a lock file whose body is malformed (`filelock.Inspect` error), a draft-mutation journal that is present but undecodable or whose phase is outside the known set, an execution-workspace entry the grammar does not classify (grammar-external), and a spec present in BOTH the active and archive zones on disk. Each carries its facts and the uncertainty "outside the ritual inventory; witness: <the file or entry>". A ritual branch with commits of its own and no other recognizer firing produces no state. Cost if wrong: one recognizer's predicate.
- Task 2 (disclosures 1, 4, 5, 7 accepted): `resolveBranchBase` lived at `cmd/verdi/design.go:693` (the brief said policy.go); the R-RR3-14 guard sits in `Gather` (Derive stays a pure function of Facts); a ritual branch's own worktree lock is ref-scoped while writer/workspace locks are store-scoped; close's "delete the leftover directory" prose renders as `rm -rf <absolute dir>` in the manual commands (an exact target, the parent's confirmation rule for destructive cleanup satisfied by disclosure).
- Task 2 (review 2A I3, R-RR3-6 amended): a young empty or partial lock body is `LockHeld`, not undecidable — the Interfaces block's reading; the ruling's first wording contradicted it and is withdrawn. A test covers the young empty and young partial branches.
- Task 2 (review 2A M6, R-RR3-17): `ForbiddenTokens` additionally contains the short force flag `-f` (`git push -f`, `git branch -f`, `git checkout -f` are force operations ac-9 names by intent); the spec's list is a floor, and a recovery run never legitimately passes `-f` to any command. Cost if wrong: one token.
- Task 2 (review 2A M7): `Validate` additionally requires every state's `facts` to be non-empty (ac-8: "identifying facts") and every executable choice id to be unique across the whole projection (R-RR3-3: the id is the byte-for-byte `--apply` key).
- Integration note: Task 1 (f140e500) is merged INTO the Task 2 lane before its fix round so `commandlog.go` can carry `var _ gitx.Observer = (*CommandLog)(nil)` (Task 1 cannot host it: gitx importing recovery would cycle).
- Task 2 (review 2B F4): every recognized state names at least one invariant that still holds, drawn from the gathered facts (for example "no commit exists on <branch> that is not already on <witness>", "the working tree is clean", "HEAD is <sha> and no ritual branch was modified by this run"); `Validate` requires `invariants_held` non-empty alongside `facts` (ac-8: "invariants that still hold").
- Task 2 (review 2B F8, R-RR3-8 scoped): the ambiguity guard withholds the unwind only when the partner state targets the SAME branch name; an empty `design/<name>` cut is not withheld by a staged closure on `close/<name>`.
- Task 2 (review 2B F9): when no other local branch exists at all, `Empty()` cannot decide, so `closure-unpublished`/`board-push-failed` must not assert "carries its own commit"; the own-commit claim becomes an uncertainty with the witness `git log <default base>..<branch>`.
- Task 2 (review 2B F3): a remote-tracking read that failed is an uncertainty, never the fact "has no remote-tracking branch"; `RemoteChecked` gates both remote-shaped recognizers.
- Task 2 (review 2B F6/F7): the copies of close.go's advice commands, `closureResidueName`, and gc's refusal sentence are pinned by tests that READ `cmd/verdi/close.go` and `cmd/verdi/gc.go` as files (relative path from the package), so drift in the source trips the recovery copy.
- Task 2 (review 2B closure R3, R-RR3-5 reading pinned): a tip-equal TIE is undecidable and does not fall through to the containing tier (the spike's own reading); documented at the resolver. Carried Minor residuals for the whole-wave review: 2A-M8 (a filelock test couples to the test binary's start time within the 5-minute tolerance); 2B-R1 (the close.go pin is whole-file `Contains`, so an edit to only the copied function's occurrence of the restore template is not caught); 2B-R2 (eleven of twelve states carry only the generic HEAD invariant; `Facts.Dirty` is gathered but never named as an invariant).
- Task 3 (implementer disclosure 1, R-RR3-18): no `docs/guide-claims.yaml` entry for `recover` in this wave — the workspace-root Integration & Startup Guide's §7 ends at 7.4, and `TestGuideClaimsTranscriptionFidelity_AppendixB` refuses a claim without its guide section; the other post-7.4 verbs carry none for the same reason. A `7.5-recover` claim follows a guide amendment (workspace-root doc, owner-scoped), recorded here as disclosed-as-unproven rather than a red gate. Cost if wrong: one YAML entry later.
- Task 3 (disclosures 2 and 3 accepted): `cmd/verdi/serve.go` wires the RecoveryLoader as well as `mcp.go` (otherwise `get_recovery` is unavailable under `verdi serve`); the MCP tool count prose moves from 21 to 22 in `internal/mcpserve/doc.go`, `server_test.go`, and the architecture doc; two units (`recover.go`, `tool_get_recovery.go`) were written without a captured RED transcript — the reviewer is asked to prove their tests bite by mutation.
