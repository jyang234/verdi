package gitx

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFastForwardOnly(t *testing.T) {
	t.Run("invokes exactly status then git merge --ff-only", func(t *testing.T) {
		var calls [][]string
		runner := func(_ context.Context, dir string, args ...string) ([]byte, error) {
			if dir != "/fixture/runway" {
				t.Fatalf("runner dir = %q", dir)
			}
			calls = append(calls, append([]string(nil), args...))
			return nil, nil
		}
		commit := strings.Repeat("a", 40)
		result, err := fastForwardOnly(context.Background(), "/fixture/runway", commit, runner)
		if err != nil {
			t.Fatalf("fastForwardOnly: %v", err)
		}
		if result.Category != FastForwardSucceeded || !result.Attempted {
			t.Fatalf("fast-forward result = %#v, want succeeded/attempted", result)
		}
		want := [][]string{{"status", "--porcelain"}, {"merge", "--ff-only", commit}}
		if !reflect.DeepEqual(calls, want) {
			t.Fatalf("git calls = %q, want %q", calls, want)
		}
		assertNoDestructiveGitTokens(t, calls)
	})

	t.Run("real repository fast-forwards to a clean descendant", func(t *testing.T) {
		dir, _, output := ffOnlyDivergedFixture(t, false)
		result, err := FastForwardOnly(context.Background(), dir, output)
		if err != nil {
			t.Fatalf("FastForwardOnly: %v", err)
		}
		if result.Category != FastForwardSucceeded || !result.Attempted {
			t.Fatalf("fast-forward result = %#v, want succeeded/attempted", result)
		}
		got := strings.TrimSpace(string(runGitFixture(t, dir, "rev-parse", "HEAD")))
		if got != output {
			t.Fatalf("HEAD = %q, want %q", got, output)
		}
	})

	t.Run("real repository refuses non-fast-forward without moving HEAD", func(t *testing.T) {
		dir, before, output := ffOnlyDivergedFixture(t, true)
		result, err := FastForwardOnly(context.Background(), dir, output)
		if err == nil {
			t.Fatal("accepted non-fast-forward output")
		}
		if result.Category != FastForwardMergeFailed || !result.Attempted {
			t.Fatalf("non-fast-forward result = %#v, want merge-failed/attempted", result)
		}
		after := strings.TrimSpace(string(runGitFixture(t, dir, "rev-parse", "HEAD")))
		if after != before {
			t.Fatalf("non-fast-forward moved HEAD from %q to %q", before, after)
		}
	})

	t.Run("dirty repository refuses before merge", func(t *testing.T) {
		var calls [][]string
		runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls = append(calls, append([]string(nil), args...))
			return []byte(" M tracked.go\n"), nil
		}
		result, err := fastForwardOnly(context.Background(), "/fixture/runway", strings.Repeat("b", 40), runner)
		if err == nil || !strings.Contains(err.Error(), "dirty") {
			t.Fatalf("dirty refusal = %v", err)
		}
		if result.Category != FastForwardRunwayDirty || result.Attempted {
			t.Fatalf("dirty result = %#v, want runway-dirty/not-attempted", result)
		}
		if got, want := calls, [][]string{{"status", "--porcelain"}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("dirty calls = %q, want %q", got, want)
		}
		assertNoDestructiveGitTokens(t, calls)
	})

	t.Run("malformed output commit refuses before any git command", func(t *testing.T) {
		called := false
		runner := func(context.Context, string, ...string) ([]byte, error) {
			called = true
			return nil, nil
		}
		for _, commit := range []string{"", "HEAD", strings.Repeat("A", 40), strings.Repeat("a", 39), strings.Repeat("g", 40), "--abort"} {
			result, err := fastForwardOnly(context.Background(), "/fixture/runway", commit, runner)
			if err == nil {
				t.Fatalf("accepted malformed output commit %q", commit)
			}
			if result.Category != FastForwardInvalidInput || result.Attempted {
				t.Fatalf("malformed commit %q result = %#v, want invalid-input/not-attempted", commit, result)
			}
		}
		if called {
			t.Fatal("malformed output commit reached git runner")
		}
	})

	t.Run("injected status failure is categorized before merge", func(t *testing.T) {
		injected := errors.New("injected git failure")
		var calls [][]string
		runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls = append(calls, append([]string(nil), args...))
			return nil, injected
		}
		result, err := fastForwardOnly(context.Background(), "/fixture/runway", strings.Repeat("c", 64), runner)
		if !errors.Is(err, injected) {
			t.Fatalf("injected failure = %v", err)
		}
		if result.Category != FastForwardStatusFailed || result.Attempted {
			t.Fatalf("status failure result = %#v, want status-failed/not-attempted", result)
		}
		if got, want := calls, [][]string{{"status", "--porcelain"}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("status failure calls = %q, want %q", got, want)
		}
		assertNoDestructiveGitTokens(t, calls)
	})

	t.Run("injected merge failure is categorized after attempted merge without recovery", func(t *testing.T) {
		injected := errors.New("injected git failure")
		var calls [][]string
		runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls = append(calls, append([]string(nil), args...))
			if args[0] == "merge" {
				return nil, injected
			}
			return nil, nil
		}
		commit := strings.Repeat("c", 64)
		result, err := fastForwardOnly(context.Background(), "/fixture/runway", commit, runner)
		if !errors.Is(err, injected) {
			t.Fatalf("injected failure = %v", err)
		}
		if result.Category != FastForwardMergeFailed || !result.Attempted {
			t.Fatalf("merge failure result = %#v, want merge-failed/attempted", result)
		}
		want := [][]string{{"status", "--porcelain"}, {"merge", "--ff-only", commit}}
		if !reflect.DeepEqual(calls, want) {
			t.Fatalf("merge failure calls = %q, want %q", calls, want)
		}
		assertNoDestructiveGitTokens(t, calls)
	})
}

func TestValidateFullOID(t *testing.T) {
	for _, oid := range []string{strings.Repeat("a", 40), strings.Repeat("b", 64)} {
		if err := ValidateFullOID(oid); err != nil {
			t.Fatalf("ValidateFullOID(%q): %v", oid, err)
		}
	}
	for _, oid := range []string{"", "HEAD", strings.Repeat("A", 40), strings.Repeat("a", 39), strings.Repeat("g", 40)} {
		if err := ValidateFullOID(oid); err == nil {
			t.Fatalf("ValidateFullOID(%q) unexpectedly succeeded", oid)
		}
	}
}

func ffOnlyDivergedFixture(t *testing.T, divergent bool) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	runGitFixture(t, dir, "init", "-b", "main")
	writeGitFixture(t, filepath.Join(dir, "tracked.txt"), "base\n")
	runGitFixture(t, dir, "add", "tracked.txt")
	runGitFixture(t, dir, "-c", "user.name=Verdi Test", "-c", "user.email=verdi@example.invalid", "commit", "-m", "base")
	base := strings.TrimSpace(string(runGitFixture(t, dir, "rev-parse", "HEAD")))
	runGitFixture(t, dir, "checkout", "-b", "output")
	writeGitFixture(t, filepath.Join(dir, "output.txt"), "output\n")
	runGitFixture(t, dir, "add", "output.txt")
	runGitFixture(t, dir, "-c", "user.name=Verdi Test", "-c", "user.email=verdi@example.invalid", "commit", "-m", "output")
	output := strings.TrimSpace(string(runGitFixture(t, dir, "rev-parse", "HEAD")))
	runGitFixture(t, dir, "checkout", "-b", "runway", base)
	if divergent {
		writeGitFixture(t, filepath.Join(dir, "runway.txt"), "runway\n")
		runGitFixture(t, dir, "add", "runway.txt")
		runGitFixture(t, dir, "-c", "user.name=Verdi Test", "-c", "user.email=verdi@example.invalid", "commit", "-m", "runway")
	}
	before := strings.TrimSpace(string(runGitFixture(t, dir, "rev-parse", "HEAD")))
	return dir, before, output
}

func runGitFixture(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return out
}

func writeGitFixture(t *testing.T, target, content string) {
	t.Helper()
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", target, err)
	}
}

func assertNoDestructiveGitTokens(t *testing.T, calls [][]string) {
	t.Helper()
	for _, call := range calls {
		joined := strings.Join(call, " ")
		for _, forbidden := range []string{"reset", "--force", "update-ref", "apply", "am"} {
			if strings.Contains(joined, forbidden) {
				t.Fatalf("git call contains forbidden recovery token %q: %q", forbidden, call)
			}
		}
	}
}
