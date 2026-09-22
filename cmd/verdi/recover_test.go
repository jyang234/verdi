package main

import (
	"bytes"
	"context"
	"encoding/json"
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

// runInDir chdirs to dir for the duration of fn's call (t.Chdir handles
// its own restore/serialization; this just gives cmdRecover's own tests a
// one-line call shape matching the brief's own table-test code).
func runInDir(t *testing.T, dir string, fn func() int) int {
	t.Helper()
	t.Chdir(dir)
	return fn()
}

// hasState reports whether p carries a recognized state with the given
// code, regardless of target.
func hasState(p recovery.Projection, code recovery.StateCode) bool {
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

// cutEmptyBranch mirrors internal/recovery/fixtures_test.go's own helper
// of the same name: cuts name from repo's current checkout and stays
// checked out on it.
func cutEmptyBranch(t *testing.T, repo *fixturegit.Repo, name string) (cutPoint string) {
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

// deadPID mirrors internal/recovery/fixtures_test.go's own helper of the
// same name: starts and waits a `true` subprocess and returns its pid,
// guaranteed reaped.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("running short-lived child: %v", err)
	}
	return cmd.Process.Pid
}

// writeStaleWriterLock mirrors internal/recovery/fixtures_test.go's own
// helper of the same name: writes store.WriterLockPath(root) naming a
// dead pid with an old start time.
func writeStaleWriterLock(t *testing.T, repo *fixturegit.Repo) string {
	t.Helper()
	path := store.WriterLockPath(repo.Dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating lock directory: %v", err)
	}
	info := filelock.Info{PID: deadPID(t), Start: time.Now().Add(-time.Hour).Unix()}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshaling lock info: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

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
			if tc.wantCode != "" && !hasState(p, recovery.StateCode(tc.wantCode)) {
				t.Fatalf("no %s in %+v", tc.wantCode, p.States)
			}
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

// TestRecover_JSONAndBareFormsAreByteIdentical proves the explicit --json
// flag and the legacy no-flag form delegate to the same call
// (recover.go's own package comment: "byte-identical, mirroring
// journey.Canonical's own contract").
func TestRecover_JSONAndBareFormsAreByteIdentical(t *testing.T) {
	repo, _ := recoverFixtureStore(t)

	var jsonOut, bareOut bytes.Buffer
	jsonCode := runInDir(t, repo.Dir, func() int { return cmdRecover([]string{"--json", "spec/checkout"}, &jsonOut, io.Discard) })
	bareCode := runInDir(t, repo.Dir, func() int { return cmdRecover([]string{"spec/checkout"}, &bareOut, io.Discard) })
	if jsonCode != bareCode {
		t.Fatalf("exit codes differ: --json %d, bare %d", jsonCode, bareCode)
	}
	if jsonOut.String() != bareOut.String() {
		t.Fatalf("--json and bare forms differ:\n--json: %s\nbare:   %s", jsonOut.String(), bareOut.String())
	}
}

// TestRecover_ApplyStubRefusesWithExit2 proves the Task 3 --apply stub
// (internal/recovery.ErrNotImplemented) is wired all the way through:
// cmdRecover maps it to exit 2. Task 4 replaces this stub and this test
// alongside it.
func TestRecover_ApplyStubRefusesWithExit2(t *testing.T) {
	repo, _ := recoverFixtureStore(t)
	var stdout, stderr bytes.Buffer
	code := runInDir(t, repo.Dir, func() int {
		return cmdRecover([]string{"spec/checkout", "--apply", "unwind-branch-cut:close/checkout"}, &stdout, &stderr)
	})
	if code != 2 {
		t.Fatalf("exit %d, want 2; stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), recovery.ErrNotImplemented.Error()) {
		t.Fatalf("stderr %q does not mention %q", stderr.String(), recovery.ErrNotImplemented.Error())
	}
}

// TestRecover_UnresolvableRootIsOperational proves a well-formed
// invocation with no resolvable store root is exit 2, "recover: "-
// prefixed (recoverErr's own guarantee).
func TestRecover_UnresolvableRootIsOperational(t *testing.T) {
	var stderr bytes.Buffer
	code := runInDir(t, t.TempDir(), func() int { return cmdRecover([]string{"spec/checkout"}, io.Discard, &stderr) })
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.HasPrefix(stderr.String(), "recover: ") {
		t.Fatalf("stderr = %q, want a \"recover: \"-prefixed line", stderr.String())
	}
}

// TestRecover_GitLogEnvRecordsCommands proves VERDI_RECOVERY_GITLOG
// (R-RR3-10) is honored: a plain read-only run appends its command log,
// and none of the recorded commands carry a forbidden token.
func TestRecover_GitLogEnvRecordsCommands(t *testing.T) {
	repo, _ := recoverFixtureStore(t)
	logPath := filepath.Join(t.TempDir(), "gitlog.txt")
	t.Setenv(recoveryGitLogEnv, logPath)

	var stdout, stderr bytes.Buffer
	code := runInDir(t, repo.Dir, func() int { return cmdRecover([]string{"spec/checkout"}, &stdout, &stderr) })
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
	forbidden := map[string]bool{}
	for _, tok := range recovery.ForbiddenTokens {
		forbidden[tok] = true
	}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			t.Fatalf("gitlog line %q is not <root>\\t<argv...>", line)
		}
		for _, word := range strings.Fields(parts[1]) {
			if forbidden[word] {
				t.Fatalf("gitlog line %q carries forbidden token %q", line, word)
			}
		}
	}
}
