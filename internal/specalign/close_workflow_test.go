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
	"os"
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
// close-evidence workflow owns that branch namespace now — see
// close-evidence.yml's own head comment for why the SAME commit must never
// be evidenced by two workflows), while leaving every other branch's
// path-filtered behaviour untouched (the paths: list itself, and the
// absence of a `branches:` allow-list, are unchanged).
func TestVerifyWorkflowExcludesCloseBranchesFromItsOwnPushTrigger(t *testing.T) {
	doc := decodeWorkflow(t, workflowPath(verdiRepoRoot, "verify.yml"))
	if doc.On.Push == nil {
		t.Fatalf("verify.yml: expected a push trigger, found none")
	}
	if !slices.Contains(doc.On.Push.BranchesIgnore, "close/**") {
		t.Errorf(`verify.yml: push trigger must carry branches-ignore: ["close/**", ...], got BranchesIgnore=%v — without it, a push to close/** touching a code path would ALSO fire this job, uploading a second "verdi-evidence" artifact for the same commit close-evidence.yml already produced one for (forge.FetchEvidenceBundle picks whichever the API lists first, ambiguously)`, doc.On.Push.BranchesIgnore)
	}
	if doc.On.Push.Branches != nil {
		t.Errorf("verify.yml: push trigger must not gain a branches: allow-list (that would narrow it beyond the close/** exclusion this lane makes), got Branches=%v", doc.On.Push.Branches)
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
func TestCloseEvidenceWorkflowCallsVerifyThroughWorkflowCall(t *testing.T) {
	doc := decodeWorkflow(t, closeEvidencePath(verdiRepoRoot))
	if len(doc.Jobs) != 1 {
		t.Fatalf("close-evidence.yml: expected exactly one job, found %d: %v", len(doc.Jobs), doc.Jobs)
	}
	var job workflowJob
	for _, j := range doc.Jobs {
		job = j
	}
	if job.Uses != "./.github/workflows/verify.yml" {
		t.Errorf("close-evidence.yml: the one job must declare uses: ./.github/workflows/verify.yml (a local reusable-workflow call), got %q", job.Uses)
	}
	if job.Environment != "" {
		t.Errorf("close-evidence.yml: the caller job must not declare environment: %q — GitHub does not permit environment: alongside a job-level uses: reusable-workflow call, and this job needs no approval gate (it only produces evidence)", job.Environment)
	}
	wantKeys := []string{"uses"}
	if !slices.Equal(job.Keys, wantKeys) {
		t.Errorf("close-evidence.yml: the caller job must declare exactly the key(s) %v and nothing else (GitHub's reusable-workflow-caller keyword whitelist is name/uses/with/secrets/strategy/needs/if/concurrency/permissions/cache-mode — environment: is NOT among them), got %v", wantKeys, job.Keys)
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
func TestCloseDispatchStepsProvenSequence(t *testing.T) {
	job := closeJob(t)
	steps := job.Steps

	validateIdx := -1
	checkoutIdx := -1
	syncIdx := -1
	closeIdx := -1
	pushIdx := -1
	for i, s := range steps {
		switch {
		case strings.HasPrefix(s.Uses, "actions/checkout@"):
			checkoutIdx = i
		case strings.Contains(s.Run, "spec_ref") && (strings.Contains(s.Run, "=~") || strings.Contains(s.Run, "regex")):
			validateIdx = i
		case strings.Contains(s.Run, "verdi sync") && !strings.Contains(s.Run, "--produce"):
			syncIdx = i
		case strings.Contains(s.Run, "verdi close"):
			closeIdx = i
		case strings.Contains(s.Run, "git push"):
			pushIdx = i
		}
	}

	if validateIdx == -1 {
		t.Fatalf("close.yml: no step validating the spec_ref input (a shell regex check against inputs.spec_ref) found; decoded run steps: %v", runCommands(steps))
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

	if validateIdx > checkoutIdx {
		t.Errorf("close.yml: spec_ref validation (step %d) must come before actions/checkout (step %d) — reject before running any verb, including checkout", validateIdx, checkoutIdx)
	}
	if validateIdx >= syncIdx || syncIdx >= closeIdx || closeIdx >= pushIdx {
		t.Errorf("close.yml: steps must run in order validate(%d) < sync(%d) < close(%d) < push(%d)", validateIdx, syncIdx, closeIdx, pushIdx)
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
