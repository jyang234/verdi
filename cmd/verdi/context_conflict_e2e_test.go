package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/policyconflict"
)

const contextConflictNoConflictJudgeResult = `{"findings":[],"recommendation":"no-conflict","schema":"verdi.policy-conflict-judge-result/v1"}`

func writeContextConflictJudge(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "judge.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write fake judge: %v", err)
	}
	return path
}

func configureContextConflictJudge(t *testing.T, repo *fixturegit.Repo, command string, timeoutSeconds int) {
	t.Helper()
	manifest := "schema: verdi.layout/v1\nalign:\n  judge_cmd: [" + command + "]\n"
	if timeoutSeconds != 0 {
		manifest += "  judge_timeout_seconds: " + strconv.Itoa(timeoutSeconds) + "\n"
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi", "verdi.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write verdi.yaml: %v", err)
	}
	repo.Head = commitAllOnCurrentBranch(t, repo.Dir, "configure conflict judge")
}

func TestContextConflictBuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)

	t.Run("absent constitution is typed exit one", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{
			".verdi/verdi.yaml":                         "schema: verdi.layout/v1\n",
			".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
		}, Message: "legacy store"}})
		reqPath := writeContextRequestFile(t, repo.Dir, "conflict-request.json", contextConflictRequestBytes(t, "spec/feature-alpha"))
		stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "context", "conflict", "--request", reqPath)
		if code != 1 || stdout != "" || stderr == "" {
			t.Fatalf("exit=%d stdout=%q stderr=%q, want typed exit 1 with no report", code, stdout, stderr)
		}
	})

	t.Run("file stdout and stdin produce canonical blocked report", func(t *testing.T) {
		repo := buildContextCompileRepo(t, map[string]string{
			".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
		})
		request := contextConflictRequestBytes(t, "spec/feature-alpha")
		reqPath := writeContextRequestFile(t, repo.Dir, "conflict-request.json", request)
		headBefore := contextE2ECurrentHead(t, repo.Dir)
		statusBefore := contextE2EPorcelainStatus(t, repo.Dir)

		stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "context", "conflict", "--request", reqPath)
		if code != 1 || stderr != "" {
			t.Fatalf("file exit=%d stderr=%q stdout=%q, want completed blocked report", code, stderr, stdout)
		}
		report, err := policyconflict.DecodeReport([]byte(stdout))
		if err != nil {
			t.Fatalf("DecodeReport(stdout): %v\n%s", err, stdout)
		}
		if report.Verdict != policyconflict.VerdictBlockedUnproven {
			t.Fatalf("verdict=%q, want blocked-unproven", report.Verdict)
		}

		cmdOut, cmdErr, cmdCode := runContextConflictBinaryWithStdin(t, bin, repo.Dir, request)
		if cmdCode != 1 || cmdErr != "" {
			t.Fatalf("stdin exit=%d stderr=%q stdout=%q", cmdCode, cmdErr, cmdOut)
		}
		if _, err := policyconflict.DecodeReport([]byte(cmdOut)); err != nil {
			t.Fatalf("DecodeReport(stdin stdout): %v", err)
		}
		if got := contextE2ECurrentHead(t, repo.Dir); got != headBefore {
			t.Fatalf("HEAD changed: %s -> %s", headBefore, got)
		}
		if got := contextE2EPorcelainStatus(t, repo.Dir); got != statusBefore {
			t.Fatalf("status changed: before=%q after=%q", statusBefore, got)
		}
	})

	t.Run("explicit output is the only visible worktree mutation", func(t *testing.T) {
		repo := buildContextCompileRepo(t, map[string]string{
			".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
		})
		reqPath := writeContextRequestFile(t, repo.Dir, "conflict-request.json", contextConflictRequestBytes(t, "spec/feature-alpha"))
		statusBefore := contextE2EPorcelainStatus(t, repo.Dir)
		stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "context", "conflict", "--request", reqPath, "--out", "conflict-report.json")
		if code != 1 || stdout != "" || stderr != "" {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		data, err := os.ReadFile(filepath.Join(repo.Dir, "conflict-report.json"))
		if err != nil {
			t.Fatalf("read report: %v", err)
		}
		if _, err := policyconflict.DecodeReport(data); err != nil {
			t.Fatalf("DecodeReport(file): %v", err)
		}
		statusAfter := contextE2EPorcelainStatus(t, repo.Dir)
		if !strings.Contains(statusAfter, "?? conflict-report.json\n") || !strings.Contains(statusAfter, strings.TrimSpace(statusBefore)) {
			t.Fatalf("status before=%q after=%q, want only named report added", statusBefore, statusAfter)
		}
	})

	t.Run("configured judge failures stay operational and write no report", func(t *testing.T) {
		cases := []struct {
			name    string
			body    string
			timeout int
		}{
			{"nonzero", "exit 7", 0},
			{"malformed", "printf 'not-json'", 0},
			{"timeout", "sleep 5", 1},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				repo := buildContextCompileRepo(t, map[string]string{
					".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
				})
				judge := writeContextConflictJudge(t, tc.body)
				configureContextConflictJudge(t, repo, judge, tc.timeout)
				reqPath := writeContextRequestFile(t, repo.Dir, "conflict-request.json", contextConflictRequestBytes(t, "spec/feature-alpha"))
				outPath := filepath.Join(repo.Dir, "must-not-exist.json")
				start := time.Now()
				stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "context", "conflict", "--request", reqPath, "--out", outPath)
				if code != 2 || stdout != "" || stderr == "" {
					t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
				}
				if tc.timeout != 0 && time.Since(start) > 4*time.Second {
					t.Fatalf("timeout returned after %s, want prompt configured deadline", time.Since(start))
				}
				if _, err := os.Stat(outPath); !os.IsNotExist(err) {
					t.Fatalf("partial report exists: %v", err)
				}
			})
		}
	})

	t.Run("immutable cache hit avoids a second process", func(t *testing.T) {
		repo := buildContextCompileRepo(t, map[string]string{
			".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
		})
		judge := writeContextConflictJudge(t, "printf '%s\\n' '"+contextConflictNoConflictJudgeResult+"'")
		configureContextConflictJudge(t, repo, judge, 0)
		reqPath := writeContextRequestFile(t, repo.Dir, "conflict-request.json", contextConflictRequestBytes(t, "spec/feature-alpha"))
		stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "context", "conflict", "--request", reqPath)
		if code != 1 || stderr != "" {
			t.Fatalf("first exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if err := os.Remove(judge); err != nil {
			t.Fatalf("remove judge after first run: %v", err)
		}
		secondOut, secondErr, secondCode := runVerdiBinary(t, bin, repo.Dir, nil, "context", "conflict", "--request", reqPath)
		if secondCode != 1 || secondErr != "" {
			t.Fatalf("cache-hit exit=%d stdout=%q stderr=%q", secondCode, secondOut, secondErr)
		}
		if secondOut != stdout {
			t.Fatalf("cache-hit report differs\nfirst=%s\nsecond=%s", stdout, secondOut)
		}
	})

	t.Run("checkout root is redacted from judge diagnostics", func(t *testing.T) {
		repo := buildContextCompileRepo(t, map[string]string{
			".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
		})
		missing := filepath.Join(repo.Dir, "missing-judge")
		configureContextConflictJudge(t, repo, missing, 0)
		reqPath := writeContextRequestFile(t, repo.Dir, "conflict-request.json", contextConflictRequestBytes(t, "spec/feature-alpha"))
		stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "context", "conflict", "--request", reqPath)
		if code != 2 || stdout != "" || stderr == "" {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if strings.Contains(stderr, repo.Dir) || !strings.Contains(stderr, contextCheckoutToken) {
			t.Fatalf("stderr leaked checkout root: %q", stderr)
		}
	})
}

func runContextConflictBinaryWithStdin(t *testing.T, bin, dir string, stdin []byte) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, "context", "conflict", "--request", "-")
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(stdin)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err == nil {
		return outBuf.String(), errBuf.String(), 0
	}
	if exitErr, ok := err.(interface{ ExitCode() int }); ok {
		return outBuf.String(), errBuf.String(), exitErr.ExitCode()
	}
	t.Fatalf("run context conflict: %v", err)
	return "", "", 2
}
