package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/recovery"
	"github.com/jyang234/verdi/internal/store"
)

// recoverRunInDir chdirs to dir for the duration of fn's call (t.Chdir
// handles its own restore/serialization; this just gives cmdRecover's
// own tests a one-line call shape matching the brief's own table-test
// code). Prefixed `recover` (fix round 1, M5): cmd/verdi is a large test
// package and Task 4 adds more test files here, so a generic name would
// risk a collision.
func recoverRunInDir(t *testing.T, dir string, fn func() int) int {
	t.Helper()
	t.Chdir(dir)
	return fn()
}

// recoverHasState reports whether p carries a recognized state with the
// given code, regardless of target.
func recoverHasState(p recovery.Projection, code recovery.StateCode) bool {
	for _, s := range p.States {
		if s.Code == code {
			return true
		}
	}
	return false
}

// recoverFixtureStore is internal/recovery/fixtures_test.go's own
// fixtureStore, exported into this package's tests (cmd/verdi cannot
// import a sibling package's _test.go file): a minimal fixturegit
// repository carrying one active feature spec, spec/checkout.
func recoverFixtureStore(t *testing.T) (*fixturegit.Repo, *store.Config) {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "")
	const checkoutSpecMD = `---
id: spec/checkout
kind: spec
class: feature
title: "Checkout"
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
---
# body
`
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                    "schema: verdi.layout/v1\nforge: gitlab\n",
				".verdi/specs/active/checkout/spec.md": checkoutSpecMD,
			},
			Message: "scaffold",
		},
	})
	cfg, err := store.Open(repo.Dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	cfg.Root = repo.Dir
	return repo, cfg
}

// recoverCutEmptyBranch mirrors internal/recovery/fixtures_test.go's own
// cutEmptyBranch: cuts name from repo's current checkout and stays
// checked out on it.
func recoverCutEmptyBranch(t *testing.T, repo *fixturegit.Repo, name string) (cutPoint string) {
	t.Helper()
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, name); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", name, err)
	}
	tip, err := gitx.RevParse(context.Background(), repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	return tip
}

// recoverDeadPID mirrors internal/recovery/fixtures_test.go's own
// deadPID: starts and waits a `true` subprocess and returns its pid,
// guaranteed reaped.
func recoverDeadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("running short-lived child: %v", err)
	}
	return cmd.Process.Pid
}

// recoverWriteStaleWriterLock mirrors internal/recovery/fixtures_test.go's
// own writeStaleWriterLock: writes store.WriterLockPath(root) naming a
// dead pid with an old start time.
func recoverWriteStaleWriterLock(t *testing.T, repo *fixturegit.Repo) string {
	t.Helper()
	path := store.WriterLockPath(repo.Dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating lock directory: %v", err)
	}
	info := filelock.Info{PID: recoverDeadPID(t), Start: time.Now().Add(-time.Hour).Unix()}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshaling lock info: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

// TestReportForbiddenCommands_Table is fix round 1's I1 fix: the
// review's mutation probe proved R-RR3-10's runtime reaction (print +
// exit 2) was entirely unproven — deleting the guard in recover.go left
// the whole TestRecover suite green. Extracted so the reaction itself,
// not just recovery.CommandLog.Forbidden()'s own upstream matching, has a
// direct table test: one entry per ForbiddenTokens rule (exact match,
// the "--"-prefix rule, and R-RR3-17's short "-f" flag) plus a clean
// entry that must print nothing and report false.
func TestReportForbiddenCommands_Table(t *testing.T) {
	cases := []struct {
		name    string
		entries [][]string
		want    bool
		wantOut string
	}{
		{"reset", [][]string{{"reset", "--hard"}}, true, "recover: forbidden git command issued: git reset --hard\n"},
		{"force_with_lease_prefix", [][]string{{"push", "--force-with-lease"}}, true, "recover: forbidden git command issued: git push --force-with-lease\n"},
		{"short_force_flag", [][]string{{"checkout", "-f"}}, true, "recover: forbidden git command issued: git checkout -f\n"},
		{"clean", [][]string{{"status", "--porcelain", "--untracked-files=all"}}, false, ""},
		// R-RR3-29: `git -c status.showUntrackedFiles=all worktree
		// remove <path>` is the reclaim executor's real argv. The
		// configuration override must not read as a force flag.
		{"worktree_remove_with_config_override", [][]string{{"-c", "status.showUntrackedFiles=all", "worktree", "remove", "/tmp/wt"}}, false, ""},
		{"mixed_clean_then_forbidden", [][]string{{"rev-parse", "HEAD"}, {"reset", "--hard"}}, true, "recover: forbidden git command issued: git reset --hard\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			got := reportForbiddenCommands(&stderr, tc.entries)
			if got != tc.want {
				t.Fatalf("reportForbiddenCommands() = %v, want %v", got, tc.want)
			}
			if stderr.String() != tc.wantOut {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tc.wantOut)
			}
		})
	}
}

// TestRecoverCmd_ForbiddenCommandInjectedViaHook is I1's cmdRecover-level
// mutation-proof witness: recoverObserverHook (test-only, nil in
// production) injects a forbidden command into the run's own
// recovery.CommandLog after it is attached, proving the WHOLE reaction —
// attach, detect, report, exit 2 — fires for a command an ordinary
// read-only run never issues.
func TestRecoverCmd_ForbiddenCommandInjectedViaHook(t *testing.T) {
	repo, _ := recoverFixtureStore(t)
	recoverObserverHook = func(log *recovery.CommandLog) {
		log.Observe(repo.Dir, []string{"reset", "--hard"})
	}
	defer func() { recoverObserverHook = nil }()

	var stdout, stderr bytes.Buffer
	code := recoverRunInDir(t, repo.Dir, func() int { return cmdRecover([]string{"spec/checkout"}, &stdout, &stderr) })
	if code != 2 {
		t.Fatalf("exit %d, want 2; stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "recover: forbidden git command issued: git reset --hard") {
		t.Fatalf("stderr %q missing the forbidden-command line", stderr.String())
	}
}

func TestRecover_Table(t *testing.T) {
	cases := []struct {
		name     string
		prepare  func(t *testing.T, repo *fixturegit.Repo)
		wantExit int
		wantCode string // a state code expected in stdout, "" for none
	}{
		{"nothing recognized", func(*testing.T, *fixturegit.Repo) {}, 0, ""},
		{"empty branch cut", func(t *testing.T, r *fixturegit.Repo) { recoverCutEmptyBranch(t, r, "close/checkout") }, 1, "empty-branch-cut"},
		{"stale writer lock", func(t *testing.T, r *fixturegit.Repo) { recoverWriteStaleWriterLock(t, r) }, 1, "stale-lock"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := recoverFixtureStore(t)
			tc.prepare(t, repo)
			var stdout, stderr bytes.Buffer
			code := recoverRunInDir(t, repo.Dir, func() int { return cmdRecover([]string{"--json", "spec/checkout"}, &stdout, &stderr) })
			if code != tc.wantExit {
				t.Fatalf("exit %d, want %d; stderr %s", code, tc.wantExit, stderr.String())
			}
			p, err := recovery.Decode(bytes.TrimRight(stdout.Bytes(), "\n"))
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantCode == "" && len(p.States) != 0 {
				t.Fatalf("states %+v", p.States)
			}
			if tc.wantCode != "" && !recoverHasState(p, recovery.StateCode(tc.wantCode)) {
				t.Fatalf("no %s in %+v", tc.wantCode, p.States)
			}
		})
	}
}

func TestRecover_UsageBeforeStore(t *testing.T) {
	// bare, malformed, and --apply-without-ref all fail on shape with exit 2 and the usage line, in a dir with no store
	for _, args := range [][]string{{}, {"--apply"}, {"--apply", "x"}, {"spec/a", "spec/b"}, {"--json"}} {
		var stderr bytes.Buffer
		if code := recoverRunInDir(t, t.TempDir(), func() int { return cmdRecover(args, io.Discard, &stderr) }); code != 2 || !strings.Contains(stderr.String(), recoverUsage) {
			t.Fatalf("args %v: exit %d stderr %q", args, code, stderr.String())
		}
	}
}

// TestRecover_JSONAndBareFormsAreByteIdentical proves the explicit --json
// flag and the legacy no-flag form delegate to the same call
// (recover.go's own package comment: "byte-identical, mirroring
// journey.Canonical's own contract"), over a POPULATED projection (fix
// round 1, M3): the zero-state case a byte-mismatch could trivially
// smuggle through (e.g. two forms disagreeing only inside a non-empty
// states/choices tree) is exercised by cutting an empty branch first,
// rather than only over the vacuous "nothing recognized" projection.
func TestRecover_JSONAndBareFormsAreByteIdentical(t *testing.T) {
	repo, _ := recoverFixtureStore(t)
	recoverCutEmptyBranch(t, repo, "close/checkout")

	var jsonOut, bareOut bytes.Buffer
	jsonCode := recoverRunInDir(t, repo.Dir, func() int { return cmdRecover([]string{"--json", "spec/checkout"}, &jsonOut, io.Discard) })
	bareCode := recoverRunInDir(t, repo.Dir, func() int { return cmdRecover([]string{"spec/checkout"}, &bareOut, io.Discard) })
	if jsonCode != bareCode {
		t.Fatalf("exit codes differ: --json %d, bare %d", jsonCode, bareCode)
	}
	if jsonCode != 1 {
		t.Fatalf("exit %d, want 1 (a populated projection, not the vacuous case)", jsonCode)
	}
	if jsonOut.String() != bareOut.String() {
		t.Fatalf("--json and bare forms differ:\n--json: %s\nbare:   %s", jsonOut.String(), bareOut.String())
	}
	p, err := recovery.Decode(bytes.TrimRight(jsonOut.Bytes(), "\n"))
	if err != nil {
		t.Fatalf("recovery.Decode: %v", err)
	}
	if !recoverHasState(p, recovery.StateEmptyBranchCut) {
		t.Fatalf("no empty-branch-cut state in %+v (the byte-identity check ran over an empty projection after all)", p.States)
	}
}

// TestRecover_ApplyUnknownChoiceRefusesWithExit1 proves R-RR3-9's own
// exit mapping for an unknown choice id: cmdRecover's own --apply path
// (Task 4's real protocol, replacing Task 3's stub and its own now-
// retired exit-2 stub test) exits 1, nothing changed, and stderr lists
// every known choice id.
func TestRecover_ApplyUnknownChoiceRefusesWithExit1(t *testing.T) {
	repo, _ := recoverFixtureStore(t)
	recoverCutEmptyBranch(t, repo, "close/checkout")
	var stdout, stderr bytes.Buffer
	code := recoverRunInDir(t, repo.Dir, func() int {
		return cmdRecover([]string{"spec/checkout", "--apply", "no-such-choice"}, &stdout, &stderr)
	})
	if code != 1 {
		t.Fatalf("exit %d, want 1; stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "unwind-branch-cut:close/checkout") {
		t.Fatalf("stderr %q, want it to list the known choice id", stderr.String())
	}
}

// TestRecover_ApplyNoExecutorRefusesWithExit1 proves R-RR3-9's own exit
// mapping for a choice with no executor: cmdRecover's own --apply path
// never touches the repository and exits 1, stderr naming the manual
// commands.
func TestRecover_ApplyNoExecutorRefusesWithExit1(t *testing.T) {
	repo, _ := recoverFixtureStore(t)
	lockPath := recoverWriteStaleWriterLock(t, repo)

	var readOut bytes.Buffer
	code := recoverRunInDir(t, repo.Dir, func() int {
		return cmdRecover([]string{"--json", "spec/checkout"}, &readOut, io.Discard)
	})
	if code != 1 {
		t.Fatalf("read exit %d, want 1; stdout %q", code, readOut.String())
	}
	p, err := recovery.Decode(bytes.TrimRight(readOut.Bytes(), "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.States) == 0 || len(p.States[0].Choices) == 0 {
		t.Fatalf("no choice recognized: %+v", p.States)
	}
	id := p.States[0].Choices[0].ID

	var stdout, stderr bytes.Buffer
	code = recoverRunInDir(t, repo.Dir, func() int {
		return cmdRecover([]string{"spec/checkout", "--apply", id}, &stdout, &stderr)
	})
	if code != 1 {
		t.Fatalf("apply exit %d, want 1; stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "rm ") {
		t.Fatalf("stderr %q, want the manual command", stderr.String())
	}
	if _, statErr := os.Stat(lockPath); statErr != nil {
		t.Fatal("lock removed by a choice with no executor")
	}
}

// TestRecover_UnresolvableRootIsOperational proves a well-formed
// invocation with no resolvable store root is exit 2, "recover: "-
// prefixed (recoverErr's own guarantee).
func TestRecover_UnresolvableRootIsOperational(t *testing.T) {
	var stderr bytes.Buffer
	code := recoverRunInDir(t, t.TempDir(), func() int { return cmdRecover([]string{"spec/checkout"}, io.Discard, &stderr) })
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.HasPrefix(stderr.String(), "recover: ") {
		t.Fatalf("stderr = %q, want a \"recover: \"-prefixed line", stderr.String())
	}
}

// TestRecover_GitLogEnvRecordsCommands proves VERDI_RECOVERY_GITLOG
// (R-RR3-10) is honored: a plain read-only run appends its command log,
// and none of the recorded commands carry a forbidden token — checked
// through recovery.IsForbiddenArgv itself (fix round 1, M4), not a local
// exact-match-only copy that would silently drift from the "--"-prefix
// rule.
func TestRecover_GitLogEnvRecordsCommands(t *testing.T) {
	repo, _ := recoverFixtureStore(t)
	logPath := filepath.Join(t.TempDir(), "gitlog.txt")
	t.Setenv(recoveryGitLogEnv, logPath)

	var stdout, stderr bytes.Buffer
	code := recoverRunInDir(t, repo.Dir, func() int { return cmdRecover([]string{"spec/checkout"}, &stdout, &stderr) })
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr %s", code, stderr.String())
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading gitlog %s: %v", logPath, err)
	}
	if len(data) == 0 {
		t.Fatal("gitlog file is empty; want at least one recorded command")
	}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			t.Fatalf("gitlog line %q is not <root>\\t<argv...>", line)
		}
		if recovery.IsForbiddenArgv(strings.Fields(parts[1])) {
			t.Fatalf("gitlog line %q carries a forbidden token", line)
		}
	}
}

// TestRecover_ApplyViolatedPostconditionIsExit1 is I2's own end-to-end
// proof of R-RR3-9's VIOLATED branch over a REACHABLE state: close/
// checkout is held by a linked worktree, so branchcut.Unwind switches
// back correctly and git's own safe `branch -d` REFUSES. The verb must
// print the violated postcondition with its observed value and exit 1 —
// never report a giving-up outcome as success.
func TestRecover_ApplyViolatedPostconditionIsExit1(t *testing.T) {
	repo, _ := recoverFixtureStore(t)
	cut := recoverCutEmptyBranch(t, repo, "close/checkout")
	if err := gitx.CheckoutExisting(context.Background(), repo.Dir, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main): %v", err)
	}
	wt := filepath.Join(t.TempDir(), "held-wt")
	if err := gitx.WorktreeAdd(context.Background(), repo.Dir, wt, "close/checkout"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := recoverRunInDir(t, repo.Dir, func() int {
		return cmdRecover([]string{"spec/checkout", "--apply", "unwind-branch-cut:close/checkout"}, &stdout, &stderr)
	})
	if code != 1 {
		t.Fatalf("exit %d, want 1; stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	want := "postcondition: close/checkout does not exist: VIOLATED (exists at " + cut + ")\n"
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout %q, want it to carry %q", stdout.String(), want)
	}
	if strings.Count(stdout.String(), "VIOLATED") != 1 {
		t.Fatalf("stdout %q, want exactly one VIOLATED line (the switch back itself succeeded)", stdout.String())
	}
	if ok, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "close/checkout"); !ok {
		t.Fatal("close/checkout deleted although git refused the delete")
	}
}

// TestReportApplyOutcome_ExitMapping is I2's own direct table over the
// two branches of R-RR3-9's post-execution exit mapping that no test
// reached: a VIOLATED postcondition is exit 1 with the "VIOLATED
// (<observed>)" rendering, and a failed journey re-derivation is exit 2
// with the projection STILL printed ("the projection still prints what
// it observed"), the executor having already run in both.
func TestReportApplyOutcome_ExitMapping(t *testing.T) {
	held := recovery.PostconditionResult{Text: "close/checkout does not exist", Held: true, Observed: "does not exist"}
	violated := recovery.PostconditionResult{Text: "current branch is main", Held: false, Observed: "current branch is close/checkout"}
	// The projection printed after the postcondition lines is the
	// re-derived one; a VALID empty projection (nothing recognized any
	// more) is what a successful unwind actually leaves behind.
	after := recovery.Projection{
		Schema:      recovery.SchemaID,
		Ref:         "spec/checkout",
		Branch:      "main",
		Head:        strings.Repeat("a", 40),
		States:      []recovery.RecognizedState{},
		Disclosures: []string{},
	}

	cases := []struct {
		name        string
		out         recovery.Outcome
		wantExit    int
		wantStdout  []string
		wantStderr  string
		wantNoStdrr bool
	}{
		{
			name:        "every postcondition held",
			out:         recovery.Outcome{ChoiceID: "unwind-branch-cut:close/checkout", After: after, Postconditions: []recovery.PostconditionResult{held}},
			wantExit:    0,
			wantStdout:  []string{"postcondition: close/checkout does not exist: held (does not exist)\n"},
			wantNoStdrr: true,
		},
		{
			name:        "one postcondition violated",
			out:         recovery.Outcome{ChoiceID: "unwind-branch-cut:close/checkout", After: after, Postconditions: []recovery.PostconditionResult{held, violated}},
			wantExit:    1,
			wantStdout:  []string{"postcondition: current branch is main: VIOLATED (current branch is close/checkout)\n"},
			wantNoStdrr: true,
		},
		{
			name: "journey re-derivation failed after the executor ran",
			out: recovery.Outcome{
				ChoiceID:       "unwind-branch-cut:close/checkout",
				After:          after,
				Postconditions: []recovery.PostconditionResult{held},
				JourneyErr:     errors.New("journey: target spec not found"),
			},
			wantExit:   2,
			wantStdout: []string{"postcondition: close/checkout does not exist: held (does not exist)\n", `"schema"`},
			wantStderr: "recover: journey: target spec not found\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := reportApplyOutcome(tc.out, &stdout, &stderr)
			if code != tc.wantExit {
				t.Fatalf("exit %d, want %d; stdout %q stderr %q", code, tc.wantExit, stdout.String(), stderr.String())
			}
			for _, want := range tc.wantStdout {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("stdout %q, want it to carry %q", stdout.String(), want)
				}
			}
			if tc.wantNoStdrr && stderr.String() != "" {
				t.Fatalf("stderr %q, want empty", stderr.String())
			}
			if tc.wantStderr != "" && stderr.String() != tc.wantStderr {
				t.Fatalf("stderr %q, want %q", stderr.String(), tc.wantStderr)
			}
		})
	}
}

// recoverCutMergedRitualWorktree mirrors internal/recovery/
// fixtures_test.go's own cutMergedRitualWorktree: it cuts branch with its
// own commit, merges it --no-ff into main, and gives it an UNMANAGED
// worktree (outside .verdi/data/worktrees), the merged-clean-unmanaged
// unit internal/reclaim classifies eligible. Leaves repo on main and
// returns the worktree's path.
func recoverCutMergedRitualWorktree(t *testing.T, repo *fixturegit.Repo, branch string) string {
	t.Helper()
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, branch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", branch, err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "unit.txt"), []byte(branch+"\n"), 0o644); err != nil {
		t.Fatalf("writing unit.txt: %v", err)
	}
	if err := gitx.AddPaths(ctx, repo.Dir, "unit.txt"); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, repo.Dir, "cut "+branch); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
	if err := gitx.CheckoutExisting(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main): %v", err)
	}
	merge := exec.Command("git", "merge", "--quiet", "--no-ff", "-m", "merge "+branch, branch)
	merge.Dir = repo.Dir
	if out, err := merge.CombinedOutput(); err != nil {
		t.Fatalf("git merge %s: %v\n%s", branch, err, out)
	}
	path := filepath.Join(t.TempDir(), "unit-wt")
	if err := gitx.WorktreeAdd(ctx, repo.Dir, path, branch); err != nil {
		t.Fatalf("WorktreeAdd(%s): %v", branch, err)
	}
	return path
}

// TestRecover_ApplyReclaimRowsCarryTheVerbPrefix is wave-review m4: the
// reclaim delegation's own rows are the only lines this verb writes to
// stderr without going through recoverErr, and they were printed bare.
// Every line on the verb's error stream carries the "recover: " prefix.
func TestRecover_ApplyReclaimRowsCarryTheVerbPrefix(t *testing.T) {
	repo, _ := recoverFixtureStore(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	recoverCutMergedRitualWorktree(t, repo, "feature/checkout")

	var stdout, stderr bytes.Buffer
	code := recoverRunInDir(t, repo.Dir, func() int {
		return cmdRecover([]string{"spec/checkout", "--apply", "reclaim:feature/checkout"}, &stdout, &stderr)
	})
	if code != 0 {
		t.Fatalf("exit %d, want 0; stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	lines := strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n")
	sawRow := false
	for _, ln := range lines {
		if ln == "" {
			continue
		}
		if !strings.HasPrefix(ln, "recover: ") {
			t.Fatalf("stderr line %q does not carry the verb prefix; full stderr %q", ln, stderr.String())
		}
		if strings.Contains(ln, "reclaimed:") && strings.Contains(ln, "feature/checkout") {
			sawRow = true
		}
	}
	if !sawRow {
		t.Fatalf("stderr = %q, want reclaim's own row for feature/checkout", stderr.String())
	}
}
