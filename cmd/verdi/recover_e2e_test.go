// Built-binary end-to-end tests for `verdi recover` (spec/readiness-
// recovery-v2 ac-8..ac-10, Task 3's read path and Task 4's --apply
// protocol): the empty-branch-cut fixture drives the REAL compiled verdi
// binary as a real OS process (buildVerdiBinary/runVerdiBinary, mirroring
// document_parity_e2e_test.go and obligationseam_e2e_test.go's own
// convention) — never a package-internal call standing in for it —
// proving argument parsing, store resolution, the recovery.CommandLog
// observer wiring, and VERDI_RECOVERY_GITLOG's test-only observability
// end to end. Task 3's own --apply stub case (exit 2, mentioning "Task
// 4") is replaced here by the real apply-protocol proof, per the plan's
// own Step 1 note.
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/recovery"
	"github.com/jyang234/verdi/internal/store"
)

// recoverE2ERepo builds the same minimal spec/checkout fixture
// recover_test.go's own recoverFixtureStore does, for the real-binary
// tests below.
func recoverE2ERepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return recoverE2ERepoSpec(t, recoverE2ESpecMD)
}

// recoverE2ESpecMD is the fixture spec every built-binary case below
// reads, and recoverE2ERepoSpec builds the same repository over any
// spec.md body (R-RR3-22's own built-binary regression needs one whose
// BODY is not YAML).
const recoverE2ESpecMD = `---
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

// recoverE2ESpecHostileBodyMD mirrors internal/recovery/fixtures_test.go's
// own hostileBodySpecMD: identical front matter, a Markdown body the YAML
// scanner refuses (a prose paragraph whose continuation line carries a
// ": ", plus a fenced code block). 4 of this repository's own 20 active
// feature specs have exactly this shape.
const recoverE2ESpecHostileBodyMD = recoverE2ESpecMD + `
An interrupted ritual left this note, wrapped the way every spec in this
corpus wraps its prose: the colon on this continuation line is what the
YAML scanner refuses.

` + "```yaml\nkey: value: extra\n```\n"

func recoverE2ERepoSpec(t *testing.T, specMD string) *fixturegit.Repo {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "")
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                    "schema: verdi.layout/v1\nforge: gitlab\n",
				".verdi/specs/active/checkout/spec.md": specMD,
			},
			Message: "scaffold",
		},
	})
}

// TestRecoverE2E_SpecBodyIsNotYAML is R-RR3-22 through the REAL binary:
// a healthy feature spec whose Markdown body the YAML scanner refuses is
// read, not refused. Exit 0 (nothing recognized) or 1 (a state
// recognized) are both projections; exit 2 is the fabricated operational
// error this case exists to forbid, and stdout must still carry exactly
// one canonical, strict-decodable projection line.
func TestRecoverE2E_SpecBodyIsNotYAML(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := recoverE2ERepoSpec(t, recoverE2ESpecHostileBodyMD)
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "close/checkout"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "recover", "--json", "spec/checkout")
	if code == 2 {
		t.Fatalf("verdi recover: exit 2 on a spec whose front matter is fine and whose body is not YAML\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if code != 0 && code != 1 {
		t.Fatalf("verdi recover: exit %d, want 0 or 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	line := strings.TrimRight(stdout, "\n")
	proj, err := recovery.Decode([]byte(line))
	if err != nil {
		t.Fatalf("recovery.Decode(stdout): %v\nstdout: %s", err, stdout)
	}
	if proj.Ref != "spec/checkout" {
		t.Fatalf("projection Ref = %q, want spec/checkout", proj.Ref)
	}
}

func TestRecoverE2E_EmptyBranchCut(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := recoverE2ERepo(t)
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "close/checkout"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "gitlog.txt")
	env := []string{recoveryGitLogEnv + "=" + logPath}
	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "recover", "--json", "spec/checkout")
	if code != 1 {
		t.Fatalf("verdi recover: exit %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	line := strings.TrimRight(stdout, "\n")
	if strings.Contains(line, "\n") {
		t.Fatalf("stdout = %q, want exactly one canonical JSON line", stdout)
	}
	proj, err := recovery.Decode([]byte(line))
	if err != nil {
		t.Fatalf("recovery.Decode(stdout): %v\nstdout: %s", err, stdout)
	}
	if !recoverHasState(proj, recovery.StateEmptyBranchCut) {
		t.Fatalf("no empty-branch-cut state in %+v", proj.States)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading gitlog %s: %v", logPath, err)
	}
	if len(data) == 0 {
		t.Fatal("gitlog file is empty; want at least one recorded command")
	}
	for _, ln := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		parts := strings.SplitN(ln, "\t", 2)
		if len(parts) != 2 {
			t.Fatalf("gitlog line %q is not <root>\\t<argv...>", ln)
		}
		if recovery.IsForbiddenArgv(strings.Fields(parts[1])) {
			t.Fatalf("gitlog line %q carries a forbidden token", ln)
		}
	}
}

// gitlogArgvs parses a VERDI_RECOVERY_GITLOG file's own "<root>\t<argv...>"
// lines into their argv slices, in call order.
func gitlogArgvs(t *testing.T, path string) [][]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading gitlog %s: %v", path, err)
	}
	if len(data) == 0 {
		t.Fatal("gitlog file is empty; want at least one recorded command")
	}
	var out [][]string
	for _, ln := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		parts := strings.SplitN(ln, "\t", 2)
		if len(parts) != 2 {
			t.Fatalf("gitlog line %q is not <root>\\t<argv...>", ln)
		}
		argv := strings.Fields(parts[1])
		if recovery.IsForbiddenArgv(argv) {
			t.Fatalf("gitlog line %q carries a forbidden token", ln)
		}
		out = append(out, argv)
	}
	return out
}

// TestRecoverE2E_ApplyUnwindHappyPath is Task 4's own real apply-protocol
// proof through the built binary (replacing Task 3's stub e2e case,
// which asserted exit 2 and named the Task 4 stub — the plan's own
// text: "The e2e case for the stub asserts exit 2 and is deleted by Task
// 4"): a clean empty branch-cut unwinds via branchcut.Unwind, exit 0,
// every postcondition held, the VERDI_RECOVERY_GITLOG argv sequence is
// exactly reads (rev-parse/status/diff) then a checkout then a
// branch -d, and carries no forbidden token.
func TestRecoverE2E_ApplyUnwindHappyPath(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := recoverE2ERepo(t)
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "close/checkout"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "gitlog.txt")
	env := []string{recoveryGitLogEnv + "=" + logPath}
	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "recover", "spec/checkout", "--apply", "unwind-branch-cut:close/checkout")
	if code != 0 {
		t.Fatalf("verdi recover --apply: exit %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("stdout = %q, want at least one postcondition line and the canonical JSON line", stdout)
	}
	for _, ln := range lines[:len(lines)-1] {
		if !strings.HasPrefix(ln, "postcondition: ") || !strings.Contains(ln, ": held (") {
			t.Fatalf("postcondition line %q not held", ln)
		}
	}
	proj, err := recovery.Decode([]byte(lines[len(lines)-1]))
	if err != nil {
		t.Fatalf("recovery.Decode(stdout's last line): %v\nstdout: %s", err, stdout)
	}
	if recoverHasState(proj, recovery.StateEmptyBranchCut) {
		t.Fatalf("empty-branch-cut still recognized after a successful unwind: %+v", proj.States)
	}

	// Review M2: the whole run's log spans three internal Gathers plus
	// the post-execution journey re-derivation, so reads are interleaved
	// freely — but the run's WRITES are exactly branchcut.Unwind's own
	// two commands, in that order and nothing else (dc-4/ac-9: "no new
	// git primitive"). An extra checkout, a second `branch -d <other>`,
	// or any unrecognized subcommand fails here rather than passing as an
	// unnoticed subsequence.
	argvs := gitlogArgvs(t, logPath)
	if len(argvs) == 0 {
		t.Fatal("no commands recorded")
	}
	wantWrites := [][]string{{"checkout", "main"}, {"branch", "-d", "close/checkout"}}
	if got := gitWriteArgvs(argvs); !reflect.DeepEqual(got, wantWrites) {
		t.Fatalf("the run's write commands were %v, want exactly %v (whole log: %v)", got, wantWrites, argvs)
	}
	firstWrite := -1
	for i, argv := range argvs {
		if len(gitWriteArgvs([][]string{argv})) == 1 {
			firstWrite = i
			break
		}
	}
	// Nothing is written before the executor: the discovery Gather, the
	// re-prove Gather and branchcut.Unwind's own tip re-read all precede
	// the first write.
	if firstWrite <= 0 {
		t.Fatalf("the first recorded command is a write (%v): reads must precede any write; whole log: %v", argvs[0], argvs)
	}
}

// gitReadOnlyVerbs is the closed set of git subcommands a `verdi recover`
// run may issue WITHOUT writing anything — every gitx read this
// projection's three Gathers and the journey re-derivation reach for.
// Anything outside it counts as a write for gitWriteArgvs, so an
// unrecognized subcommand fails the assertion (closed, not open).
var gitReadOnlyVerbs = map[string]bool{
	"rev-parse": true, "status": true, "diff": true, "diff-index": true,
	"for-each-ref": true, "show-ref": true, "symbolic-ref": true,
	"merge-base": true, "ls-tree": true, "ls-files": true, "show": true,
	"cat-file": true, "config": true, "log": true, "rev-list": true,
	"name-rev": true, "describe": true, "var": true, "check-ignore": true,
}

// gitWriteArgvs returns, in call order, every recorded argv whose
// subcommand is not on gitReadOnlyVerbs — the run's own write surface.
func gitWriteArgvs(argvs [][]string) [][]string {
	var writes [][]string
	for _, argv := range argvs {
		if len(argv) == 0 || gitReadOnlyVerbs[argv[0]] {
			continue
		}
		// Subcommands that read only in one exact shape: bare `git
		// remote` lists remotes (`git remote add|set-url|…` does not),
		// `git worktree list` lists worktrees.
		if argv[0] == "remote" && len(argv) == 1 {
			continue
		}
		if argv[0] == "worktree" && len(argv) == 2 && argv[1] == "list" {
			continue
		}
		writes = append(writes, argv)
	}
	return writes
}

// TestRecoverE2E_ApplyUnwindReturnBranchAhead is C1/R-RR3-19's own
// built-binary case: main advanced past close/checkout's cut point before
// the recovery ran, so a correct unwind leaves HEAD at main's tip — NOT
// at the cut. Exit 0, every postcondition held.
func TestRecoverE2E_ApplyUnwindReturnBranchAhead(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := recoverE2ERepo(t)
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "close/checkout"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := gitx.CheckoutExisting(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main): %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "ahead.txt"), []byte("ahead\n"), 0o644); err != nil {
		t.Fatalf("writing ahead.txt: %v", err)
	}
	if err := gitx.AddPaths(ctx, repo.Dir, "ahead.txt"); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, repo.Dir, "advance main past the cut"); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
	if err := gitx.CheckoutExisting(ctx, repo.Dir, "close/checkout"); err != nil {
		t.Fatalf("CheckoutExisting(close/checkout): %v", err)
	}
	mainTip, err := gitx.RevParse(ctx, repo.Dir, "main")
	if err != nil {
		t.Fatalf("RevParse(main): %v", err)
	}

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "recover", "spec/checkout", "--apply", "unwind-branch-cut:close/checkout")
	if code != 0 {
		t.Fatalf("verdi recover --apply: exit %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	for _, ln := range lines[:len(lines)-1] {
		if !strings.HasPrefix(ln, "postcondition: ") || !strings.Contains(ln, ": held (") {
			t.Fatalf("postcondition line %q not held", ln)
		}
	}
	if !strings.Contains(stdout, "postcondition: HEAD is the tip of main: held") {
		t.Fatalf("stdout %q, want the return-branch-tip postcondition (R-RR3-19)", stdout)
	}
	head, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	if head != mainTip {
		t.Fatalf("HEAD = %s, want main's tip %s", head, mainTip)
	}
}

// TestRecoverE2E_UncleanTreeWithholdsTheUnwind proves R-RR3-21 through
// the REAL binary, over the single static repository state a subprocess
// test can set up directly (no in-process hook reaches across a real OS
// process boundary): an empty close/checkout cut with one uncommitted
// file beside it. The state is still recognized and still printed, but
// no executable choice is offered — before R-RR3-21 the operator was
// handed `--apply unwind-branch-cut:close/checkout` for a choice that
// was certain to refuse. The withheld choice's own uncertainty carries
// the `git status --porcelain` witness, and asking for the choice
// anyway is exit 1 with nothing executed: the gitlog carries no checkout
// or branch command at all. The refusal also shows m1's own rendering —
// with no choices in the projection the error says so instead of
// trailing an empty "known choices:" list.
func TestRecoverE2E_UncleanTreeWithholdsTheUnwind(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := recoverE2ERepo(t)
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "close/checkout"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "leftover.txt"), []byte("uncommitted\n"), 0o644); err != nil {
		t.Fatalf("writing leftover.txt: %v", err)
	}

	readOut, readErr, readCode := runVerdiBinary(t, bin, repo.Dir, nil, "recover", "--json", "spec/checkout")
	if readCode != 1 {
		t.Fatalf("verdi recover: exit %d, want 1\nstdout:\n%s\nstderr:\n%s", readCode, readOut, readErr)
	}
	proj, err := recovery.Decode([]byte(strings.TrimRight(readOut, "\n")))
	if err != nil {
		t.Fatalf("recovery.Decode(stdout): %v\nstdout: %s", err, readOut)
	}
	var cut recovery.RecognizedState
	for _, st := range proj.States {
		if st.Code == recovery.StateEmptyBranchCut && st.Target == "close/checkout" {
			cut = st
		}
	}
	if cut.Code == "" {
		t.Fatalf("no empty-branch-cut state for close/checkout: %+v", proj.States)
	}
	if len(cut.Choices) != 0 {
		t.Fatalf("Choices = %+v, want none: the working tree is not clean", cut.Choices)
	}
	witnessed := false
	for _, u := range cut.Uncertainties {
		if strings.Contains(u.Witness, "git status --porcelain") && strings.Contains(u.Witness, "--untracked-files=all") {
			witnessed = true
		}
	}
	if !witnessed {
		t.Fatalf("Uncertainties = %+v, want one with the configuration-independent `git status` witness", cut.Uncertainties)
	}

	logPath := filepath.Join(t.TempDir(), "gitlog.txt")
	env := []string{recoveryGitLogEnv + "=" + logPath}
	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "recover", "spec/checkout", "--apply", "unwind-branch-cut:close/checkout")
	if code != 1 {
		t.Fatalf("verdi recover --apply: exit %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "unknown choice") {
		t.Fatalf("stderr = %q, want it to refuse the withheld choice id", stderr)
	}
	if !strings.Contains(stderr, "no choices are offered for spec/checkout") {
		t.Fatalf("stderr = %q, want m1's own no-choices rendering", stderr)
	}
	if strings.Contains(stderr, "known choices:") {
		t.Fatalf("stderr = %q, want no empty \"known choices:\" tail", stderr)
	}

	for _, argv := range gitlogArgvs(t, logPath) {
		if argv[0] == "checkout" || argv[0] == "branch" {
			t.Fatalf("a withheld-choice run issued a checkout/branch command: %v", argv)
		}
	}
	if cur, err := gitx.CurrentBranch(context.Background(), repo.Dir); err != nil || cur != "close/checkout" {
		t.Fatalf("current branch = %q, err = %v, want close/checkout untouched", cur, err)
	}
}

// TestRecoverE2E_UntrackedFileHiddenByStatusConfigWithholdsTheUnwind is
// the owner risk review's F1 through the REAL binary: the same empty cut
// beside one uncommitted file, with the ordinary display setting
// status.showUntrackedFiles=no in the repository's own config. `git
// status --porcelain` — what the old clean-tree gate read — then reports
// nothing, while the changed-path listing (--untracked-files=all
// overrides the setting) still names the file. The verb must refuse:
// exit 1, no checkout or branch command issued, close/checkout still
// checked out and still existing, and the operator's file still on disk.
func TestRecoverE2E_UntrackedFileHiddenByStatusConfigWithholdsTheUnwind(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := recoverE2ERepo(t)
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "close/checkout"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	cfgCmd := exec.Command("git", "config", "status.showUntrackedFiles", "no")
	cfgCmd.Dir = repo.Dir
	if out, err := cfgCmd.CombinedOutput(); err != nil {
		t.Fatalf("git config status.showUntrackedFiles no: %v\n%s", err, out)
	}
	unfinished := filepath.Join(repo.Dir, "unfinished.txt")
	if err := os.WriteFile(unfinished, []byte("operator work\n"), 0o644); err != nil {
		t.Fatalf("writing unfinished.txt: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "gitlog.txt")
	env := []string{recoveryGitLogEnv + "=" + logPath}
	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "recover", "spec/checkout", "--apply", "unwind-branch-cut:close/checkout")
	if code != 1 {
		t.Fatalf("verdi recover --apply over a tree whose untracked work a display setting hides: exit %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	for _, argv := range gitlogArgvs(t, logPath) {
		if argv[0] == "checkout" || argv[0] == "branch" {
			t.Fatalf("a refused run issued a checkout/branch command: %v", argv)
		}
	}
	if cur, err := gitx.CurrentBranch(context.Background(), repo.Dir); err != nil || cur != "close/checkout" {
		t.Fatalf("current branch = %q, err = %v, want close/checkout untouched", cur, err)
	}
	if ok, err := gitx.HasLocalBranch(context.Background(), repo.Dir, "close/checkout"); err != nil || !ok {
		t.Fatalf("HasLocalBranch(close/checkout) = %v, err = %v: the branch cut was unwound over uncommitted work", ok, err)
	}
	if _, err := os.Stat(unfinished); err != nil {
		t.Fatalf("the operator's own uncommitted file is gone: %v", err)
	}
}

// TestRecoverE2E_ApplyNoExecutor proves a manual-only choice (the stale
// writer lock's own "rm <path>") refuses through the real binary: exit
// 1, the manual command named on stderr, and the lock file itself
// untouched.
func TestRecoverE2E_ApplyNoExecutor(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := recoverE2ERepo(t)
	lockPath := store.WriterLockPath(repo.Dir)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("creating lock dir: %v", err)
	}
	deadCmd := exec.Command("true")
	if err := deadCmd.Run(); err != nil {
		t.Fatalf("running short-lived child: %v", err)
	}
	lockBody, err := json.Marshal(filelock.Info{PID: deadCmd.Process.Pid, Start: time.Now().Add(-time.Hour).Unix()})
	if err != nil {
		t.Fatalf("marshaling lock info: %v", err)
	}
	if err := os.WriteFile(lockPath, lockBody, 0o644); err != nil {
		t.Fatalf("writing %s: %v", lockPath, err)
	}

	// The choice id names the lock path AS THE STORE ITSELF RESOLVED IT
	// (store.FindRoot(".") inside the subprocess), which need not be
	// byte-identical to lockPath here (e.g. macOS's /var -> /private/var
	// symlink) — discovered via a read pass rather than assumed.
	readOut, _, readCode := runVerdiBinary(t, bin, repo.Dir, nil, "recover", "--json", "spec/checkout")
	if readCode != 1 {
		t.Fatalf("verdi recover (read): exit %d, want 1\nstdout:\n%s", readCode, readOut)
	}
	proj, err := recovery.Decode([]byte(strings.TrimRight(readOut, "\n")))
	if err != nil {
		t.Fatalf("recovery.Decode: %v\nstdout: %s", err, readOut)
	}
	if len(proj.States) == 0 || len(proj.States[0].Choices) == 0 {
		t.Fatalf("no choice recognized: %+v", proj.States)
	}
	id := proj.States[0].Choices[0].ID

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "recover", "spec/checkout", "--apply", id)
	if code != 1 {
		t.Fatalf("verdi recover --apply: exit %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "rm ") {
		t.Fatalf("stderr = %q, want the manual command", stderr)
	}
	if _, statErr := os.Stat(lockPath); statErr != nil {
		t.Fatalf("lock removed by a choice with no executor: %v", statErr)
	}
}

// TestRecoverE2E_ArchiveMoveWithUnreadableIndexIsDiagnosed is R-RR3-26
// through the REAL binary, over the owner risk review's own F2 fixture:
// an interrupted close that moved spec/checkout's directory into the
// archive zone on disk and never staged it, in a repository whose index
// git cannot read, with NO close/checkout branch to carry the diagnosis
// instead. Every fact the state is recognized from is available, so the
// operator must be TOLD (exit 1, the state on stdout, the index named as
// the unavailable fact and no command guessed) rather than handed
// exit 0, nothing recognized.
func TestRecoverE2E_ArchiveMoveWithUnreadableIndexIsDiagnosed(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := recoverE2ERepo(t)

	activeDir := store.ActiveSpecDir(repo.Dir, "checkout")
	archiveDir := store.ArchiveSpecDir(repo.Dir, "checkout")
	if err := os.MkdirAll(filepath.Dir(archiveDir), 0o755); err != nil {
		t.Fatalf("creating the archive zone: %v", err)
	}
	if err := os.Rename(activeDir, archiveDir); err != nil {
		t.Fatalf("moving %s to %s: %v", activeDir, archiveDir, err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, ".git", "index"), []byte("broken index\n"), 0o644); err != nil {
		t.Fatalf("truncating .git/index: %v", err)
	}

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "recover", "--json", "spec/checkout")
	if code != 1 {
		t.Fatalf("verdi recover over a fully observed archive-move residue: exit %d, want 1 (a state recognized)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	proj, err := recovery.Decode([]byte(strings.TrimRight(stdout, "\n")))
	if err != nil {
		t.Fatalf("recovery.Decode(stdout): %v\nstdout: %s", err, stdout)
	}
	var states []recovery.RecognizedState
	for _, s := range proj.States {
		if s.Code == recovery.StateArchiveMoveUncommitted {
			states = append(states, s)
		}
	}
	if len(states) != 1 {
		t.Fatalf("%d archive-move-uncommitted states, want exactly one: %+v", len(states), proj.States)
	}
	if len(states[0].Choices) != 0 {
		t.Fatalf("Choices = %+v, want none: the index is exactly what would say which command is right", states[0].Choices)
	}
	var named bool
	for _, u := range states[0].Uncertainties {
		if strings.Contains(u.Text, "staged-path listing could not be observed") && u.Witness != "" {
			named = true
		}
	}
	if !named {
		t.Fatalf("Uncertainties = %+v, want the unobserved index named with the witness that would settle it", states[0].Uncertainties)
	}
}
