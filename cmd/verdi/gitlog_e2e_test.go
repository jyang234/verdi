// Built-binary end-to-end test of VERDI_GITLOG (spec/gitx-recorder-seam
// ac-2, dc-2; parent spec/ritual-write-scope-v3 dc-10; ledger SI-359 (1),
// (3)): obligation/gitx-recorder-seam--ac-2--behavioral's clause "the
// built binary writes its log to VERDI_GITLOG only when it is set", driven
// through the REAL compiled verdi binary as a real OS process.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

// gitLogE2ERun is one run of the built binary that exited 1: its standard
// output, pid, and every path it left under the repository's working tree
// (.git excluded) and its private TMPDIR.
type gitLogE2ERun struct {
	stdout string
	pid    int
	files  []string
}

// runForGitLog runs `verdi recover --json spec/checkout`, a read-only verb
// that runs git, over a fresh empty-branch-cut repository, with the test
// process's environment minus both git-log hooks, a private empty TMPDIR,
// and extra.
func runForGitLog(t *testing.T, bin string, extra ...string) gitLogE2ERun {
	t.Helper()
	repo := recoverE2ERepo(t)
	gitTestOutput(t, repo.Dir, "checkout", "-q", "-b", "close/checkout")
	tmp := t.TempDir()
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if k != gitx.GitLogEnv && k != recoveryGitLogEnv && k != "TMPDIR" {
			env = append(env, kv)
		}
	}
	cmd := exec.CommandContext(t.Context(), bin, "recover", "--json", "spec/checkout")
	cmd.Dir = repo.Dir
	cmd.Env = append(append(env, "TMPDIR="+tmp), extra...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting verdi: %v", err)
	}
	pid := cmd.Process.Pid
	code := 0
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("verdi recover: %v\nstderr: %s", err, stderr.String())
		}
		code = exitErr.ExitCode()
	}
	if code != 1 {
		t.Fatalf("verdi recover: exit %d, want 1 (the empty branch cut)\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	return gitLogE2ERun{stdout: stdout.String(), pid: pid, files: append(treeFiles(t, repo.Dir, "repo"), treeFiles(t, tmp, "tmp")...)}
}

// treeFiles lists every path under dir, .git excluded, prefixed by label.
func treeFiles(t *testing.T, dir, label string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		out = append(out, label+"/"+filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("listing %s: %v", dir, err)
	}
	return out
}

// decodeGitLogE2E strictly decodes every line of a VERDI_GITLOG file:
// unknown fields and trailing data are refused, and every line must end
// in a newline.
func decodeGitLogE2E(t *testing.T, path string) []gitx.GitLogRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading VERDI_GITLOG: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("VERDI_GITLOG = %q, want newline-terminated records", data)
	}
	var out []gitx.GitLogRecord
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		dec := json.NewDecoder(strings.NewReader(line))
		dec.DisallowUnknownFields()
		var rec gitx.GitLogRecord
		if err := dec.Decode(&rec); err != nil {
			t.Fatalf("VERDI_GITLOG line %q: %v", line, err)
		}
		if _, err := dec.Token(); !errors.Is(err, io.EOF) {
			t.Fatalf("VERDI_GITLOG line %q: trailing data", line)
		}
		out = append(out, rec)
	}
	return out
}

// TestGitLogE2E_RecordsOnlyWhenSet: with VERDI_GITLOG set, the built
// binary's file holds the verb's git calls, every one under the binary's
// own pid in an absolute directory, including in order every call recover's
// own observer saw (VERDI_RECOVERY_GITLOG, SI-359 (2)); unset or empty,
// the verb answers byte-identically and leaves the same files, so nothing
// is recorded and no file is created.
func TestGitLogE2E_RecordsOnlyWhenSet(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	logs := t.TempDir()
	gitLog := filepath.Join(logs, "git.jsonl")
	recoveryLog := filepath.Join(logs, "recovery.log")

	set := runForGitLog(t, bin, gitx.GitLogEnv+"="+gitLog, recoveryGitLogEnv+"="+recoveryLog)
	recs := decodeGitLogE2E(t, gitLog)
	var joined []string
	for _, rec := range recs {
		if rec.PID != set.pid || !filepath.IsAbs(rec.Dir) || len(rec.Args) == 0 {
			t.Fatalf("record %+v: want pid %d, an absolute directory, and an argv", rec, set.pid)
		}
		joined = append(joined, strings.Join(rec.Args, " "))
	}
	observed := 0
	for _, argv := range gitlogArgvs(t, recoveryLog) {
		i := slices.Index(joined, strings.Join(argv, " "))
		if i < 0 {
			t.Fatalf("recover's observer saw `git %s`, which VERDI_GITLOG does not hold in order after the previous one:\n%s", strings.Join(argv, " "), strings.Join(joined, "\n"))
		}
		joined = joined[i+1:]
		observed++
	}
	if observed == 0 {
		t.Fatal("recover's own log is empty, so the comparison proves nothing")
	}
	t.Logf("VERDI_GITLOG held %d record(s) under pid %d, including in order the %d call(s) recover's observer saw", len(recs), set.pid, observed)

	for _, tt := range []struct {
		name  string
		extra []string
	}{
		{"unset", nil},
		{"set to the empty string", []string{gitx.GitLogEnv + "="}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := runForGitLog(t, bin, tt.extra...)
			if got.stdout != set.stdout {
				t.Errorf("stdout = %q, want the same answer as with VERDI_GITLOG set: %q", got.stdout, set.stdout)
			}
			if !slices.Equal(got.files, set.files) {
				t.Errorf("the run left %v, want exactly what the logged run left: %v", got.files, set.files)
			}
		})
	}
	if entries, err := os.ReadDir(logs); err != nil || len(entries) != 2 {
		t.Fatalf("the log directory holds %v (err %v), want only the logged run's two logs", entries, err)
	}
}
