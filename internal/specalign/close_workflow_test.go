// Lane L3 (docs/superpowers/plans/2026-09-22-closing-machinery.md R-CM-4):
// source-level assertions over the close/close-evidence workflow files,
// mirroring workflow_test.go's own no-network, decode-through-
// internal/artifact.DecodeYAMLLoose discipline (see that file's package
// comment for why: this file shares decodeWorkflow/asMap/... rather than
// importing yaml.v3 a second time). This file additionally re-checks
// verify.yml's close/** carve-out, since that edit is this lane's own.
//
// Scope: R-CM-4's contract only (03 §Closure ritual, spec/remote-and-ci
// DC-1). No approval-reading/countersign assertions — that machinery
// belongs to lane L2 (R-CM-1); this file only proves the close job
// declares `environment: close` so L2 has a run to read, never that any
// approval was actually granted.
package specalign

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func closeEvidencePath(root string) string {
	return workflowPath(root, "close-evidence.yml")
}

func closeDispatchPath(root string) string {
	return workflowPath(root, "close.yml")
}

// TestVerifyWorkflowExcludesCloseBranchesFromItsOwnPushTrigger proves
// verify.yml's push trigger no longer fires on close/** branches (the
// close-evidence workflow owns that branch namespace now, so one push to a
// close/** branch runs the evidence job once, not twice), while leaving
// every other branch's path-filtered behaviour untouched: branches-ignore
// is EXACTLY ["close/**"] (no other branch is carved out), and the paths:
// list and the absence of a `branches:` allow-list are unchanged.
func TestVerifyWorkflowExcludesCloseBranchesFromItsOwnPushTrigger(t *testing.T) {
	doc := decodeWorkflow(t, workflowPath(verdiRepoRoot, "verify.yml"))
	if doc.On.Push == nil {
		t.Fatalf("verify.yml: expected a push trigger, found none")
	}
	if !slices.Equal(doc.On.Push.BranchesIgnore, []string{"close/**"}) {
		t.Errorf(`verify.yml: push trigger branches-ignore: must be exactly ["close/**"], got %v — without close/**, a push to close/** touching a code path would ALSO fire this job and upload a second "verdi-evidence" bundle for the push close-evidence.yml already evidences; any further entry (e.g. "main") silently stops evidence on a branch whose behaviour this carve-out must leave unchanged`, doc.On.Push.BranchesIgnore)
	}
	if doc.On.Push.Branches != nil {
		t.Errorf("verify.yml: push trigger must not gain a branches: allow-list (that would narrow it beyond the close/** exclusion this lane makes), got Branches=%v", doc.On.Push.Branches)
	}
	// Tag pushes (review m-2). Before the carve-out verify.yml defined no
	// branch or tag filter, so every tag push ran it (GitHub does not
	// evaluate path filters for tag pushes). Defining only branches-ignore
	// would stop every tag push ("If you define only tags/tags-ignore or
	// only branches/branches-ignore, the workflow won't run for events
	// affecting the undefined Git ref"), so tags: ["**"], which matches
	// every tag name, keeps the old behaviour.
	if !slices.Equal(doc.On.Push.Tags, []string{"**"}) {
		t.Errorf(`verify.yml: push trigger must declare tags: ["**"] so tag pushes keep running this workflow exactly as before the close/** carve-out, got %v`, doc.On.Push.Tags)
	}
	if want := []string{"branches-ignore", "paths", "tags"}; !slices.Equal(doc.On.Push.Keys, want) {
		t.Errorf("verify.yml: push trigger body must declare exactly %v, got %v (tags-ignore:, paths-ignore:, or any other filter changes which pushes produce evidence)", want, doc.On.Push.Keys)
	}
	wantPaths := []string{
		"**.go", "go.mod", "go.sum", "Makefile", "e2e/**",
		".github/workflows/**", "testdata/**", "scripts/**", "verdi.bindings.yaml",
	}
	got := append([]string(nil), doc.On.Push.Paths...)
	want := append([]string(nil), wantPaths...)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("verify.yml: push trigger paths: must stay byte-identical to before this lane's edit, got %v, want %v", doc.On.Push.Paths, wantPaths)
	}
}

// TestVerifyWorkflowIsCallableAsAReusableWorkflow proves verify.yml now
// also declares on.workflow_call (so close-evidence.yml's `uses:
// ./.github/workflows/verify.yml` resolves), and that this addition is the
// ONLY thing added to `on:` — no pull_request, no workflow_dispatch.
func TestVerifyWorkflowIsCallableAsAReusableWorkflow(t *testing.T) {
	doc := decodeWorkflow(t, workflowPath(verdiRepoRoot, "verify.yml"))
	if doc.On.WorkflowCall == nil {
		t.Fatalf("verify.yml: expected an on.workflow_call trigger (so close-evidence.yml can call this job through workflow_call), found none")
	}
	if doc.On.PullRequest != nil {
		t.Errorf("verify.yml: must still declare no pull_request trigger (Task 8: PR gating lives in merge-gate.yml alone), found one: %+v", doc.On.PullRequest)
	}
	if doc.On.WorkflowDispatch != nil {
		t.Errorf("verify.yml: must not gain a workflow_dispatch trigger — that is the separate close.yml file's job alone")
	}
}

// TestCloseEvidenceWorkflowExists is the first, most basic red-phase
// assertion for the close/** evidence workflow.
func TestCloseEvidenceWorkflowExists(t *testing.T) {
	if _, err := os.Stat(closeEvidencePath(verdiRepoRoot)); err != nil {
		t.Fatalf("expected %s to exist (R-CM-4: close/** branches get exact-commit evidence with no path filter): %v", closeEvidencePath(verdiRepoRoot), err)
	}
}

// TestCloseEvidenceWorkflowTriggersOnCloseBranchesUnfiltered proves the
// push trigger is close/**-only and carries NO paths: filter at all (03
// §Closure ritual's evidence must exist for the exact commit being closed
// regardless of what changed in it — spec/remote-and-ci DC-1).
func TestCloseEvidenceWorkflowTriggersOnCloseBranchesUnfiltered(t *testing.T) {
	doc := decodeWorkflow(t, closeEvidencePath(verdiRepoRoot))
	if doc.On.PullRequest != nil {
		t.Errorf("close-evidence.yml: must declare no pull_request trigger, found one: %+v", doc.On.PullRequest)
	}
	if doc.On.WorkflowDispatch != nil {
		t.Errorf("close-evidence.yml: must declare no workflow_dispatch trigger (push-only — the dispatch-only close ritual lives in the separate close.yml file)")
	}
	if doc.On.Push == nil {
		t.Fatalf("close-evidence.yml: expected a push trigger, found none")
	}
	if !slices.Equal(doc.On.Push.Branches, []string{"close/**"}) {
		t.Errorf(`close-evidence.yml: push trigger branches: must be exactly ["close/**"], got %v`, doc.On.Push.Branches)
	}
	if len(doc.On.Push.Paths) != 0 {
		t.Errorf("close-evidence.yml: push trigger must carry NO paths: filter (every close/** push gets evidence, regardless of what changed), got Paths=%v", doc.On.Push.Paths)
	}
	// The whitelist net (triggerFilter.Keys): the push body may carry
	// `branches:` and nothing else. paths-ignore:, tags:, branches-ignore:
	// and any future narrowing keyword would each let some close/** push go
	// unevidenced while the targeted paths: check above stays green.
	if want := []string{"branches"}; !slices.Equal(doc.On.Push.Keys, want) {
		t.Errorf("close-evidence.yml: push trigger body must declare exactly %v, got %v (a paths-ignore: or any other filter would leave some close/** commit with no exact-commit evidence)", want, doc.On.Push.Keys)
	}
	if want := []string{"push"}; !slices.Equal(doc.On.Keys, want) {
		t.Errorf("close-evidence.yml: on: must declare exactly %v, got %v", want, doc.On.Keys)
	}
}

// TestCloseEvidenceWorkflowCallsVerifyThroughWorkflowCall proves the one
// job reuses verify.yml's own `verify` job body via jobs.<id>.uses, rather
// than duplicating its steps: the self-hosted evidence producer
// (cmd/verdi/selfevidence.go) is only honest when invoked strictly after
// the SAME make-verify run, in the SAME job — never a second, independent
// run (verify.yml's own head comment). A jobs.<id>.uses caller job may only
// carry the small whitelist of keywords GitHub documents for it (name,
// uses, with, secrets, strategy, needs, if, permissions — NOT
// environment:); this test also proves the caller job declares no
// environment: (that lives on close.yml's job alone, and jobs.<job_id>.uses
// + jobs.<job_id>.environment together is not a keyword combination GitHub
// permits on a reusable-workflow caller job).
//
// The caller job's id is pinned to `verify` as well. GitHub documents that
// "the github context is always associated with the caller workflow" when a
// reusable workflow runs, which would make GITHUB_JOB the CALLER's job id;
// community reports show the called job's id instead. Because the evidence
// records' provenance.job_name comes from GITHUB_JOB (SI-229) and elaborated
// obligations name CI job `verify`, both ids must be `verify` for the
// construction to hold under either reading.
func TestCloseEvidenceWorkflowCallsVerifyThroughWorkflowCall(t *testing.T) {
	doc := decodeWorkflow(t, closeEvidencePath(verdiRepoRoot))
	if got, want := jobKeys(doc.Jobs), []string{"verify"}; !slices.Equal(got, want) {
		t.Fatalf("close-evidence.yml: jobs must be exactly %v (the caller job id must be `verify`: GITHUB_JOB may report the caller's job id, and it feeds provenance.job_name, SI-229), got %v", want, got)
	}
	job := doc.Jobs["verify"]
	if job.Uses != "./.github/workflows/verify.yml" {
		t.Errorf("close-evidence.yml: the one job must declare uses: ./.github/workflows/verify.yml (a local reusable-workflow call), got %q", job.Uses)
	}
	if job.Environment != "" {
		t.Errorf("close-evidence.yml: the caller job must not declare environment: %q — GitHub does not permit environment: alongside a job-level uses: reusable-workflow call, and this job needs no approval gate (it only produces evidence)", job.Environment)
	}
	wantKeys := []string{"permissions", "uses"}
	if !slices.Equal(job.Keys, wantKeys) {
		t.Errorf("close-evidence.yml: the caller job must declare exactly the key(s) %v and nothing else (GitHub's reusable-workflow-caller keyword whitelist is name/uses/with/secrets/strategy/needs/if/concurrency/permissions/cache-mode — environment: is NOT among them), got %v", wantKeys, job.Keys)
	}
	// Least privilege for the called job (review m-7). GitHub documents
	// that when jobs.<job_id>.permissions is not specified in the calling
	// job, the called workflow gets the default GITHUB_TOKEN permissions,
	// and actions/checkout persists that token in .git/config for every
	// later step. The called verify job needs only contents: read (the
	// checkout): verdi sync --produce never dials the forge, verify.yml
	// passes no token to any step, and actions/upload-artifact
	// authenticates with the runner's ACTIONS_RUNTIME_TOKEN, not
	// GITHUB_TOKEN. Every scope left unlisted is set to none.
	if want := map[string]string{"contents": "read"}; !maps.Equal(job.Permissions, want) {
		t.Errorf("close-evidence.yml: the caller job's permissions must be exactly %v, got %v", want, job.Permissions)
	}
}

// TestCloseDispatchWorkflowExists is the first, most basic red-phase
// assertion for the dispatch-only close workflow.
func TestCloseDispatchWorkflowExists(t *testing.T) {
	if _, err := os.Stat(closeDispatchPath(verdiRepoRoot)); err != nil {
		t.Fatalf("expected %s to exist (R-CM-4: a dispatch-only close workflow): %v", closeDispatchPath(verdiRepoRoot), err)
	}
}

// TestCloseDispatchWorkflowIsDispatchOnly proves workflow_dispatch is the
// ONLY trigger (never push, never pull_request — the contract's own
// wording), and that it declares exactly one required string input naming
// the spec ref to close.
func TestCloseDispatchWorkflowIsDispatchOnly(t *testing.T) {
	doc := decodeWorkflow(t, closeDispatchPath(verdiRepoRoot))
	if doc.On.Push != nil {
		t.Errorf("close.yml: must declare no push trigger, found one: %+v", doc.On.Push)
	}
	if doc.On.PullRequest != nil {
		t.Errorf("close.yml: must declare no pull_request trigger, found one: %+v", doc.On.PullRequest)
	}
	if doc.On.WorkflowCall != nil {
		t.Errorf("close.yml: must declare no workflow_call trigger, found one: %+v", doc.On.WorkflowCall)
	}
	if doc.On.WorkflowDispatch == nil {
		t.Fatalf("close.yml: expected a workflow_dispatch trigger, found none")
	}
	// The whitelist net (workflowTriggers.Keys): the three named negatives
	// above cannot see a trigger this package does not model, such as
	// schedule: or pull_request_target:, either of which would run the
	// archive-and-publish job without an operator's dispatch.
	if want := []string{"workflow_dispatch"}; !slices.Equal(doc.On.Keys, want) {
		t.Errorf("close.yml: on: must declare exactly %v and no other trigger, got %v", want, doc.On.Keys)
	}
	input, ok := doc.On.WorkflowDispatch.Inputs["spec_ref"]
	if !ok {
		t.Fatalf("close.yml: expected a workflow_dispatch input named spec_ref, got inputs: %v", doc.On.WorkflowDispatch.Inputs)
	}
	if !input.Required {
		t.Errorf("close.yml: spec_ref input must be required: true, got false")
	}
	if input.Type != "string" && input.Type != "" {
		t.Errorf(`close.yml: spec_ref input type must be "string" (or omitted, which defaults to string), got %q`, input.Type)
	}
}

// closeJob returns close.yml's one job, failing the test if there is not
// exactly one.
func closeJob(t *testing.T) workflowJob {
	t.Helper()
	doc := decodeWorkflow(t, closeDispatchPath(verdiRepoRoot))
	if len(doc.Jobs) != 1 {
		t.Fatalf("close.yml: expected exactly one job, found %d: %v", len(doc.Jobs), doc.Jobs)
	}
	for _, j := range doc.Jobs {
		return j
	}
	panic("unreachable")
}

// TestCloseDispatchJobDeclaresProtectedEnvironment proves the close job
// declares environment: close (R-W1-4: this is the ONE gated job, so lane
// L2 can read this run's environment review as the solo approval — this
// lane implements no approval-reading logic itself).
func TestCloseDispatchJobDeclaresProtectedEnvironment(t *testing.T) {
	job := closeJob(t)
	if job.Environment != "close" {
		t.Errorf(`close.yml: the close job must declare environment: close, got %q`, job.Environment)
	}
}

// TestCloseDispatchJobPermissionsAreLeastPrivilege proves the close job's
// permissions: map declares exactly the three scopes the close sequence
// needs (derived from the code this lane read, not guessed):
//   - contents: write   — actions/checkout, and the final `git push` of the
//     close/<name> branch verdi close committed to locally (close.go itself
//     never pushes: dc-3, "this verb stops at the branch").
//   - actions: read     — verdi sync's forge round trip (GET .../actions/runs,
//     .../actions/runs/{id}/artifacts, .../actions/artifacts/{id}/zip —
//     internal/forge/github/github.go), the DC-1 "fetched from the forge
//     artifact store by (ref, commit)" boundary.
//   - pull-requests: read — cmd/verdi/closuregate.go's
//     checkPendingSupersessionCondition (forge.FindOpenMR over GET
//     .../pulls), called unconditionally inside runClosureGate; without
//     this scope the forge degrades to disclosed-unproven rather than
//     proving the condition.
//
// Every other scope must be absent (defaults to "none" once permissions:
// is specified at all — least privilege, not merely "enough").
func TestCloseDispatchJobPermissionsAreLeastPrivilege(t *testing.T) {
	job := closeJob(t)
	want := map[string]string{
		"contents":      "write",
		"actions":       "read",
		"pull-requests": "read",
	}
	if len(job.Permissions) != len(want) {
		t.Fatalf("close.yml: close job permissions = %v, want exactly %v", job.Permissions, want)
	}
	for scope, level := range want {
		if got := job.Permissions[scope]; got != level {
			t.Errorf("close.yml: close job permissions[%q] = %q, want %q", scope, got, level)
		}
	}
}

// TestCloseDispatchJobNeverUsesForbiddenFlags proves no step's run: text
// contains verdi's own local-testing escape hatch or a git history-
// mutating flag/subcommand the brief forbids: --force-local, --force, a
// bare -f flag, reset, restore, clean, stash, update-ref. A CI-run `verdi
// close` must reach PublishRollup as a genuine, authoritative (source: ci)
// run (04 §Semantics), never the non-authoritative --force-local escape
// hatch; and the archive commit verdi close creates locally must survive
// intact to the final push, never rewritten out from under it.
func TestCloseDispatchJobNeverUsesForbiddenFlags(t *testing.T) {
	job := closeJob(t)
	forbidden := []string{
		"--force-local", "--force", " -f ", " -f\n", "git reset", "git restore",
		"git clean", "git stash", "update-ref",
	}
	for _, step := range job.Steps {
		run := " " + step.Run + " " // pad so " -f " token-boundary checks see edge occurrences too
		for _, f := range forbidden {
			if strings.Contains(run, f) {
				t.Errorf("close.yml: step %q run text contains forbidden %q: %q", step.Name, f, step.Run)
			}
		}
	}
}

// TestCloseDispatchStepsProvenSequence proves the close job's steps run in
// the order the ritual requires: the spec_ref input is validated before
// ANY verdi verb runs, `verdi sync` (pulling evidence for the exact commit
// being closed) runs before `verdi close` (which folds it), and the push of
// the resulting close/<name> branch is the FINAL step (nothing runs after
// it that could still abort with the archive commit stranded, unpushed, in
// the ephemeral runner).
//
// Lane E4b (SI-258) widens this by exactly one step: the jq step that fills
// the committed context-request template must sit after the sync and before
// the close, so the close reads a request filled from the validated input
// in this same run. No other ordering constraint changes.
func TestCloseDispatchStepsProvenSequence(t *testing.T) {
	job := closeJob(t)
	steps := job.Steps

	validateIdx := -1
	checkoutIdx := -1
	syncIdx := -1
	instantiateIdx := -1
	closeIdx := -1
	pushIdx := -1
	for i, s := range steps {
		switch {
		case strings.HasPrefix(s.Uses, "actions/checkout@"):
			checkoutIdx = i
		case strings.Contains(s.Run, "$SPEC_REF") && strings.Contains(s.Run, "=~"):
			validateIdx = i
		case strings.Contains(s.Run, "verdi sync") && !strings.Contains(s.Run, "--produce"):
			syncIdx = i
		case strings.HasPrefix(strings.TrimSpace(s.Run), "jq "):
			instantiateIdx = i
		case strings.Contains(s.Run, "verdi close"):
			closeIdx = i
		case strings.Contains(s.Run, "git push"):
			pushIdx = i
		}
	}

	if validateIdx == -1 {
		t.Fatalf("close.yml: no step validating the spec_ref input (a shell regex check against $SPEC_REF) found; decoded run steps: %v", runCommands(steps))
	}
	if validateIdx != 0 {
		t.Errorf("close.yml: spec_ref validation must be the FIRST step (index 0), got index %d", validateIdx)
	}
	if checkoutIdx == -1 {
		t.Fatalf("close.yml: no actions/checkout step found")
	}
	if syncIdx == -1 {
		t.Fatalf("close.yml: no bare `verdi sync` step (pulling evidence, never --produce — this job only ever PULLS, the close-evidence.yml job is the sole producer) found; decoded run steps: %v", runCommands(steps))
	}
	if closeIdx == -1 {
		t.Fatalf("close.yml: no `verdi close` step found; decoded run steps: %v", runCommands(steps))
	}
	if pushIdx == -1 {
		t.Fatalf("close.yml: no `git push` step found (verdi close itself never pushes — dc-3 — so this job must push the archive commit itself for a closure MR to be possible)")
	}

	if instantiateIdx == -1 {
		t.Fatalf("close.yml: no jq step filling the context-request template found (SI-258); decoded run steps: %v", runCommands(steps))
	}

	if validateIdx > checkoutIdx {
		t.Errorf("close.yml: spec_ref validation (step %d) must come before actions/checkout (step %d) — reject before running any verb, including checkout", validateIdx, checkoutIdx)
	}
	if validateIdx >= syncIdx || syncIdx >= instantiateIdx || instantiateIdx >= closeIdx || closeIdx >= pushIdx {
		t.Errorf("close.yml: steps must run in order validate(%d) < sync(%d) < instantiate request(%d) < close(%d) < push(%d)", validateIdx, syncIdx, instantiateIdx, closeIdx, pushIdx)
	}
	if pushIdx != len(steps)-1 {
		t.Errorf("close.yml: the git push step must be the FINAL step (index %d of %d), got index %d", len(steps)-1, len(steps), pushIdx)
	}
}

// TestCloseDispatchChecksOutExactCommitDetached proves the checkout step
// pins ref: to the triggering commit SHA (github.sha), never a bare branch
// name. This is load-bearing, not cosmetic: actions/checkout's own
// input-helper.ts routes a 40/64-hex `ref:` value into `settings.commit`
// with `settings.ref` cleared, and its git-command-manager.ts `checkout()`
// then runs a plain `git checkout --progress --force <sha>` (detached HEAD,
// no local branch created) — whereas the DEFAULT (ref: omitted) would
// resolve `github.ref` = "refs/heads/close/<name>" and run `git checkout
// --progress --force -B close/<name> refs/remotes/origin/close/<name>",
// creating a LOCAL branch named close/<name>. verdi close's own branch cut
// (gitx.CheckoutNewBranch, `git checkout -b close/<name>`) uses -b, which
// refuses if a branch of that name already exists — so the default
// (non-detached) checkout would make every real close job fail at its own
// branch cut. Pinning ref: to github.sha sidesteps that collision
// entirely, and also happens to be the more exact-commit-honest form (DC-1).
func TestCloseDispatchChecksOutExactCommitDetached(t *testing.T) {
	job := closeJob(t)
	step := findStep(job.Steps, "actions/checkout@")
	if step == nil {
		t.Fatalf("close.yml: no actions/checkout step found")
	}
	if got := step.With["ref"]; got != "${{ github.sha }}" {
		t.Errorf(`close.yml: actions/checkout ref: must be "${{ github.sha }}" (detached-HEAD-at-exact-commit; see this test's doc comment for why the default branch-name form collides with verdi close's own branch cut), got %q`, got)
	}
}

// TestCloseEvidenceCalledJobGatesThenProducesThenUploads proves, in the job
// close-evidence.yml actually calls (resolved from its `uses:` path, not
// assumed), that `make verify` runs before `verdi sync --produce`, which runs
// before the upload of the "verdi-evidence" artifact from
// .verdi/data/derived/. The self-hosted evidence producer is honest only when
// it runs strictly after a `make verify` that already passed in the same job
// (verify.yml's head comment), and close's `verdi sync` fetches the bundle by
// that exact artifact name. Each command is matched by exact text and must
// occur once, so `make verify || true` or a duplicate step does not count.
func TestCloseEvidenceCalledJobGatesThenProducesThenUploads(t *testing.T) {
	caller := decodeWorkflow(t, closeEvidencePath(verdiRepoRoot))
	job, ok := caller.Jobs["verify"]
	if !ok {
		t.Fatalf("close-evidence.yml: no `verify` caller job found, got jobs %v", jobKeys(caller.Jobs))
	}
	rel, ok := strings.CutPrefix(job.Uses, "./")
	if !ok || rel == "" {
		t.Fatalf("close-evidence.yml: the caller job must call a local reusable workflow (uses: ./.github/workflows/<file>), got %q", job.Uses)
	}
	called := decodeWorkflow(t, filepath.Join(verdiRepoRoot, filepath.FromSlash(rel)))
	if got, want := jobKeys(called.Jobs), []string{"verify"}; !slices.Equal(got, want) {
		t.Fatalf("%s: the called workflow must declare exactly the jobs %v (its job id is GITHUB_JOB, which feeds provenance.job_name), got %v", rel, want, got)
	}
	steps := called.Jobs["verify"].Steps

	exactlyOnce := func(cmd string) int {
		t.Helper()
		matches := findExactRunSteps(steps, cmd)
		if len(matches) != 1 {
			t.Fatalf("%s: expected exactly one run step whose command is exactly %q, found %d; decoded run steps: %v", rel, cmd, len(matches), runCommands(steps))
		}
		return matches[0]
	}
	gate := exactlyOnce("make verify")
	produce := exactlyOnce("./.build/verdi sync --produce")

	upload := -1
	for i, step := range steps {
		if !strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
			continue
		}
		if upload != -1 {
			t.Fatalf("%s: more than one actions/upload-artifact step (indexes %d and %d)", rel, upload, i)
		}
		upload = i
		if got := step.With["name"]; got != "verdi-evidence" {
			t.Errorf(`%s: the upload step's artifact name must be "verdi-evidence" (the name close's verdi sync fetches by), got %q`, rel, got)
		}
		if got := step.With["path"]; got != ".verdi/data/derived/" {
			t.Errorf(`%s: the upload step must upload ".verdi/data/derived/" (where verdi sync --produce writes the bundle), got %q`, rel, got)
		}
	}
	if upload == -1 {
		t.Fatalf("%s: no actions/upload-artifact step found", rel)
	}
	if gate >= produce || produce >= upload {
		t.Errorf("%s: steps must run in the order make verify (%d) < verdi sync --produce (%d) < upload verdi-evidence (%d)", rel, gate, produce, upload)
	}
}

// TestCloseDispatchJobNeverProducesEvidence proves the close job only PULLS
// evidence: no step runs `verdi sync --produce` (or --produce-runtime). DC-1
// makes evidence authoritative only because it is fetched from the forge
// artifact store by (ref, commit); a record this same run produced would be
// folded without ever taking that round trip.
func TestCloseDispatchJobNeverProducesEvidence(t *testing.T) {
	job := closeJob(t)
	for i, step := range job.Steps {
		if strings.Contains(step.Run, "--produce") {
			t.Errorf("close.yml: step %d runs evidence production, which this job must never do (DC-1: close folds only bundles fetched from the forge artifact store): %q", i, strings.TrimSpace(step.Run))
		}
	}
}

// TestCloseDispatchSyncIsAPlainPull proves the evidence step is exactly
// `./.build/verdi sync`, once, and that no step passes --or-regen, which
// would regenerate records locally (source: local) when no CI bundle exists
// instead of failing.
func TestCloseDispatchSyncIsAPlainPull(t *testing.T) {
	job := closeJob(t)
	if matches := findExactRunSteps(job.Steps, "./.build/verdi sync"); len(matches) != 1 {
		t.Errorf("close.yml: expected exactly one run step whose command is exactly %q, found %d; decoded run steps: %v", "./.build/verdi sync", len(matches), runCommands(job.Steps))
	}
	for i, step := range job.Steps {
		if strings.Contains(step.Run, "--or-regen") {
			t.Errorf("close.yml: step %d passes --or-regen, which falls back to local regeneration instead of the forge round trip: %q", i, strings.TrimSpace(step.Run))
		}
	}
}

// gitPushRE finds a git push invocation anywhere in a run: script,
// tolerating any run of whitespace between the two words.
var gitPushRE = regexp.MustCompile(`\bgit\s+push\b`)

// TestCloseDispatchPushIsAPlainFastForwardOfTheCloseBranch proves the job
// pushes exactly once, with exactly `git push origin HEAD`. After `verdi
// close` exits 0 HEAD is the local close/<name> branch it cut, and `git push
// origin HEAD` pushes that branch to the same name on origin. Exact equality
// refuses a `+` force refspec (`+HEAD`), an explicit destination
// (`HEAD:main`), and any flag, none of which a substring check would see.
func TestCloseDispatchPushIsAPlainFastForwardOfTheCloseBranch(t *testing.T) {
	job := closeJob(t)
	var pushes []int
	for i, step := range job.Steps {
		if gitPushRE.MatchString(step.Run) {
			pushes = append(pushes, i)
		}
	}
	if len(pushes) != 1 {
		t.Fatalf("close.yml: expected exactly one step running git push, found %d (indexes %v)", len(pushes), pushes)
	}
	if got, want := strings.TrimSpace(job.Steps[pushes[0]].Run), "git push origin HEAD"; got != want {
		t.Errorf("close.yml: the push step must be exactly %q (the current close/<name> branch to the same name, fast-forward only), got %q", want, got)
	}
}

// specRefFromInput is the only form in which close.yml may hand its
// dispatch input to a step: as the value of an env: entry, which the runner
// passes to the shell as data.
const specRefFromInput = "${{ inputs.spec_ref }}"

// specRefValidationPattern is the only spec_ref shape close.yml accepts
// (SI-258): a whole spec/<name> ref. A scheme-prefixed tracker ref is
// refused, because the context request the close step passes binds a spec
// ref and a tracker ref cannot fill its spec field.
const specRefValidationPattern = `^spec/[a-z0-9]+(-[a-z0-9]+)*$`

// closeRequestFilledPath is where close.yml writes the filled request:
// under the checkout root, in the git-ignored .build/ (.gitignore) that
// already holds the built binary, so readinessload.ValidatedContextRequestPath
// stops its walk at the store root and never meets a host symlink above it
// (ruling R-PBW1-7).
const closeRequestFilledPath = ".build/close-context-request.json"

// closeRequestInstantiateCommand is the one step that fills the template
// (SI-258): it sets spec from the validated input and touches no other
// field, and -c -S reproduce the canonical encoding (see
// TestCloseRequestTemplateIsTheCanonicalEncodingWithSpecEmpty).
const closeRequestInstantiateCommand = `jq -c -S --arg spec "$SPEC_REF" '.spec = $spec' ` + closeRequestTemplateRel + ` > ` + closeRequestFilledPath

// closeStepCommand is the close step's exact command: the validated spec ref
// and the filled request.
const closeStepCommand = `./.build/verdi close "$SPEC_REF" --context-request ` + closeRequestFilledPath

// TestCloseDispatchRunScriptsNeverInterpolateExpressions proves no run:
// script in close.yml contains a `${{ ... }}` expression, and in particular
// never `${{ inputs.spec_ref }}`. The runner substitutes an expression into
// the script text before the shell starts, so a crafted input would run as
// code inside the approved run, before any validation (review m-4). The
// input reaches the shell only through env: (SPEC_REF).
func TestCloseDispatchRunScriptsNeverInterpolateExpressions(t *testing.T) {
	job := closeJob(t)
	for i, step := range job.Steps {
		if strings.Contains(step.Run, "${{") {
			t.Errorf("close.yml: step %d (name %q) interpolates an expression into its run: script; pass the value through env: instead (a `${{ inputs.spec_ref }}` in run: executes a crafted input before validation): %q", i, step.Name, step.Run)
		}
	}
}

// TestCloseDispatchSpecRefReachesTheShellOnlyThroughEnvAfterValidation
// proves the validation step (index 0), the request-instantiation step, and
// the close step read the input as env SPEC_REF, that the close step's
// command is exactly closeStepCommand, and that no other step receives the
// input at all.
//
// Lane E4b (SI-258) widens the allowed receivers from two steps to three:
// the jq step that fills the template's spec needs the validated input, and
// nothing else it needs comes from a dispatch input.
func TestCloseDispatchSpecRefReachesTheShellOnlyThroughEnvAfterValidation(t *testing.T) {
	job := closeJob(t)
	if len(job.Steps) == 0 {
		t.Fatalf("close.yml: the close job has no steps")
	}
	if got := job.Steps[0].Env["SPEC_REF"]; got != specRefFromInput {
		t.Errorf("close.yml: the first (validation) step must receive the input as env SPEC_REF: %q, got %q", specRefFromInput, got)
	}
	receivers := map[int]bool{0: true}
	for _, cmd := range []string{closeRequestInstantiateCommand, closeStepCommand} {
		matches := findExactRunSteps(job.Steps, cmd)
		if len(matches) != 1 {
			t.Fatalf("close.yml: expected exactly one run step whose command is exactly %q, found %d; decoded run steps: %v", cmd, len(matches), runCommands(job.Steps))
		}
		if got := job.Steps[matches[0]].Env["SPEC_REF"]; got != specRefFromInput {
			t.Errorf("close.yml: the step running %q must receive the input as env SPEC_REF: %q, got %q", cmd, specRefFromInput, got)
		}
		receivers[matches[0]] = true
	}
	for i, step := range job.Steps {
		if receivers[i] {
			continue
		}
		for name, value := range step.Env {
			if strings.Contains(value, "inputs.") {
				t.Errorf("close.yml: step %d (name %q) receives a dispatch input through env %s=%q; only the validation, request-instantiation, and close steps may", i, step.Name, name, value)
			}
		}
		for name, value := range step.With {
			if strings.Contains(value, "inputs.") {
				t.Errorf("close.yml: step %d (name %q) receives a dispatch input through with %s=%q; only the validation, request-instantiation, and close steps may", i, step.Name, name, value)
			}
		}
	}
}

// TestCloseDispatchValidationAcceptsOnlySpecRefs proves the validation step
// (SI-258) accepts exactly the spec/<name> shape: it declares one pattern,
// specRefValidationPattern, tests $SPEC_REF against it once, and on refusal
// prints the value with printf %q (a newline in the input cannot start a
// workflow command) and exits 1. The pattern read from the workflow is then
// evaluated in Go against accepted and refused inputs. For this pattern
// (anchors, ASCII bracket ranges, one group, + and *) bash's POSIX ERE and
// Go's RE2 accept the same strings, and both anchor $ at the end of the
// input, not at an embedded newline.
func TestCloseDispatchValidationAcceptsOnlySpecRefs(t *testing.T) {
	job := closeJob(t)
	if len(job.Steps) == 0 {
		t.Fatalf("close.yml: the close job has no steps")
	}
	run := job.Steps[0].Run
	var patterns []string
	for _, line := range strings.Split(run, "\n") {
		line = strings.TrimSpace(line)
		if name, value, ok := strings.Cut(line, "="); ok && strings.HasSuffix(name, "_re") {
			patterns = append(patterns, name+"="+value)
		}
	}
	if want := []string{"spec_re='" + specRefValidationPattern + "'"}; !slices.Equal(patterns, want) {
		t.Fatalf("close.yml: the validation step must declare exactly the one pattern %v (tracker refs are no longer accepted, SI-258), got %v", want, patterns)
	}
	if n := strings.Count(run, "=~"); n != 1 {
		t.Errorf("close.yml: the validation step must test $SPEC_REF against one pattern, found %d =~ tests", n)
	}
	if !strings.Contains(run, `if [[ "$SPEC_REF" =~ $spec_re ]]; then`) {
		t.Errorf("close.yml: the validation step must test exactly `[[ \"$SPEC_REF\" =~ $spec_re ]]`, got %q", run)
	}
	refusal := ""
	for _, line := range strings.Split(run, "\n") {
		if strings.Contains(line, "::error::") {
			refusal = strings.TrimSpace(line)
		}
	}
	if !strings.HasPrefix(refusal, "printf '::error::") || !strings.Contains(refusal, "%q") || !strings.HasSuffix(refusal, `"$SPEC_REF"`) {
		t.Errorf("close.yml: the refusal must print the value with printf %%q, got %q", refusal)
	}
	for _, reason := range []string{"tracker ref", "binds a spec ref"} {
		if !strings.Contains(refusal, reason) {
			t.Errorf("close.yml: the refusal must say why a tracker ref is refused (missing %q), got %q", reason, refusal)
		}
	}
	if !strings.HasSuffix(strings.TrimSpace(run), "exit 1") {
		t.Errorf("close.yml: the validation step must end with exit 1 after the refusal, got %q", run)
	}

	re := regexp.MustCompile(specRefValidationPattern)
	tests := []struct {
		input string
		want  bool
	}{
		{"spec/a", true},
		{"spec/vatc-machine-projections", true},
		{"spec/a1-2b-c3", true},
		{"jira:LOAN-1482", false},
		{"jira:KEY", false},
		{"feature/a", false},
		{"spec/", false},
		{"spec/A", false},
		{"spec/-a", false},
		{"spec/a-", false},
		{"spec/a--b", false},
		{"spec/a/b", false},
		{"spec/a@0123456789abcdef0123456789abcdef01234567", false},
		{"spec/a#ac-1", false},
		{" spec/a", false},
		{"spec/a\n", false},
		{"spec/a\nspec/b", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			if got := re.MatchString(tt.input); got != tt.want {
				t.Errorf("spec_ref pattern on %q = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestCloseDispatchInstantiatesTheRequestTemplate proves the one step that
// fills the committed template (SI-258): it runs exactly
// closeRequestInstantiateCommand, after the sync and before the close,
// receives only SPEC_REF (from the dispatch input, through env), and
// declares no key but name, env and run, so no shell:, working-directory:
// (which would move where .build/ resolves), if:, or continue-on-error: can
// change what it writes. The template it reads exists at that path.
func TestCloseDispatchInstantiatesTheRequestTemplate(t *testing.T) {
	job := closeJob(t)
	matches := findExactRunSteps(job.Steps, closeRequestInstantiateCommand)
	if len(matches) != 1 {
		t.Fatalf("close.yml: expected exactly one run step whose command is exactly %q, found %d; decoded run steps: %v", closeRequestInstantiateCommand, len(matches), runCommands(job.Steps))
	}
	idx := matches[0]
	syncs := findExactRunSteps(job.Steps, "./.build/verdi sync")
	closes := findExactRunSteps(job.Steps, closeStepCommand)
	if len(syncs) != 1 || len(closes) != 1 {
		t.Fatalf("close.yml: expected one sync step and one close step, found %d and %d", len(syncs), len(closes))
	}
	if idx <= syncs[0] || idx >= closes[0] {
		t.Errorf("close.yml: the request-instantiation step (index %d) must run after the sync (index %d) and before the close (index %d)", idx, syncs[0], closes[0])
	}
	step := job.Steps[idx]
	if want := map[string]string{"SPEC_REF": specRefFromInput}; !maps.Equal(step.Env, want) {
		t.Errorf("close.yml: the request-instantiation step's env must be exactly %v, got %v", want, step.Env)
	}
	if want := []string{"env", "name", "run"}; !slices.Equal(step.Keys, want) {
		t.Errorf("close.yml: the request-instantiation step must declare exactly the keys %v, got %v", want, step.Keys)
	}
	if _, err := os.Stat(closeRequestTemplatePath(verdiRepoRoot)); err != nil {
		t.Errorf("close.yml: the template the instantiation step reads must be committed at %s: %v", closeRequestTemplateRel, err)
	}
}

// redirectTargetRE finds a shell output redirection (`> target`) that
// follows whitespace, so the '>' of a `<name>` placeholder is not read as
// one.
var redirectTargetRE = regexp.MustCompile(`(?:^|\s)>\s*(\S+)`)

// contextRequestOperandRE finds the operand of every --context-request flag.
var contextRequestOperandRE = regexp.MustCompile(`--context-request\s+(\S+)`)

// TestCloseDispatchWritesTheRequestOnlyUnderBuild pins ruling R-PBW1-7: the
// filled request lives at closeRequestFilledPath, relative to the checkout
// root and inside the git-ignored .build/, and nowhere else. No step names
// RUNNER_TEMP or /tmp, every redirect that writes a context request targets
// closeRequestFilledPath, every --context-request operand is that path, and
// the committed template is only ever read.
func TestCloseDispatchWritesTheRequestOnlyUnderBuild(t *testing.T) {
	job := closeJob(t)
	operands := 0
	for i, step := range job.Steps {
		for _, forbidden := range []string{"RUNNER_TEMP", "/tmp"} {
			if strings.Contains(step.Run, forbidden) {
				t.Errorf("close.yml: step %d (name %q) names %s; the filled request lives only at %s", i, step.Name, forbidden, closeRequestFilledPath)
			}
		}
		for _, m := range redirectTargetRE.FindAllStringSubmatch(step.Run, -1) {
			if strings.Contains(m[1], "context-request") && m[1] != closeRequestFilledPath {
				t.Errorf("close.yml: step %d (name %q) writes a context request to %q, want only %q", i, step.Name, m[1], closeRequestFilledPath)
			}
		}
		for _, m := range contextRequestOperandRE.FindAllStringSubmatch(step.Run, -1) {
			operands++
			if m[1] != closeRequestFilledPath {
				t.Errorf("close.yml: step %d (name %q) passes --context-request %q, want %q", i, step.Name, m[1], closeRequestFilledPath)
			}
		}
	}
	if operands != 1 {
		t.Errorf("close.yml: expected exactly one --context-request operand across all steps, found %d", operands)
	}
	if !strings.HasPrefix(closeRequestFilledPath, ".build/") || strings.Contains(closeRequestFilledPath, "..") {
		t.Errorf("closeRequestFilledPath %q must be a relative path inside .build/", closeRequestFilledPath)
	}
}

// TestCloseDispatchCloseStepHasACommitterIdentity proves the step that runs
// `verdi close` gives git the GitHub Actions bot identity for both author
// and committer (review I-2). `verdi close` commits the archive move with a
// bare `git commit` (gitx.CreateCommit), and a runner has no global git
// identity (internal/gitx/commitidentity.go records the resulting "empty
// ident name ... not allowed" failure), so without these the job would die
// at the commit, after the freeze and the archive move. The values are the
// ones actions/checkout's README gives for pushing with the built-in token;
// they are scoped to this one step's environment.
//
// Lane E4b (SI-258) changes only the command this test looks up, to
// closeStepCommand (the close step now also passes the filled request); the
// identity it pins is unchanged.
func TestCloseDispatchCloseStepHasACommitterIdentity(t *testing.T) {
	job := closeJob(t)
	matches := findExactRunSteps(job.Steps, closeStepCommand)
	if len(matches) != 1 {
		t.Fatalf("close.yml: expected exactly one `verdi close` step, found %d; decoded run steps: %v", len(matches), runCommands(job.Steps))
	}
	env := job.Steps[matches[0]].Env
	const (
		botName  = "github-actions[bot]"
		botEmail = "41898282+github-actions[bot]@users.noreply.github.com"
	)
	want := map[string]string{
		"GIT_AUTHOR_NAME":     botName,
		"GIT_AUTHOR_EMAIL":    botEmail,
		"GIT_COMMITTER_NAME":  botName,
		"GIT_COMMITTER_EMAIL": botEmail,
	}
	for _, name := range slices.Sorted(maps.Keys(want)) {
		if got := env[name]; got != want[name] {
			t.Errorf("close.yml: the verdi close step must set env %s=%q (git needs an identity for close's archive commit on a runner that has none), got %q", name, want[name], got)
		}
	}
}

// TestCloseDispatchRunsSerializePerSpecRef proves close.yml declares a
// document-level concurrency group keyed on the dispatched spec ref, with
// cancel-in-progress explicitly false (review m-5). Two runs for the same
// ref would otherwise race: the loser's push is rejected after its rollup
// is already published. With the group, a second dispatch waits as pending
// until the first finishes, and false keeps GitHub from cancelling a run
// that may already have published but not yet pushed.
func TestCloseDispatchRunsSerializePerSpecRef(t *testing.T) {
	doc := decodeWorkflow(t, closeDispatchPath(verdiRepoRoot))
	if doc.Concurrency == nil {
		t.Fatalf("close.yml: expected a document-level concurrency: block, found none")
	}
	if got, want := doc.Concurrency.Group, "close-"+specRefFromInput; got != want {
		t.Errorf("close.yml: concurrency group = %q, want %q (one group per spec ref)", got, want)
	}
	if doc.Concurrency.CancelInProgress == nil || *doc.Concurrency.CancelInProgress {
		t.Errorf("close.yml: concurrency must declare cancel-in-progress: false explicitly (a cancelled run can strand a published rollup with no pushed archive commit), got keys %v", doc.Concurrency.Keys)
	}
}
