package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// ensureDataGitignore writes .verdi/.gitignore ("data/\n") into root if
// not already present — buildPhase7Repo's fixture predates the mutable
// zone and doesn't carry one, but `git add -A` (internal/gitx.AddAll,
// commit-to-design's write path) must never sweep up the untracked
// mutable board this file's tests write directly to disk (VL-013).
func ensureDataGitignore(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, ".verdi", ".gitignore")
	if _, err := os.Stat(path); err == nil {
		return
	}
	if err := os.WriteFile(path, []byte("data/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeMutableBoard(t *testing.T, root, key string, board *artifact.Board) {
	t.Helper()
	path := filepath.Join(root, ".verdi", "data", "mutable", "boards", key+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(board)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCmdBoardCommit_Happy proves the CLI entry point end to end against a
// board with zero stickies (the simplest legal board: a spec skeleton with
// no dispositions block at all is still valid per 02's optional
// dispositions: field) pinned to an existing spec.
func TestCmdBoardCommit_Happy(t *testing.T) {
	repo := buildPhase7Repo(t)
	ensureDataGitignore(t, repo.Dir)
	writeMutableBoard(t, repo.Dir, "jira:LOAN-1482", &artifact.Board{Schema: "verdi.board/v1"})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := cmdBoardCommit([]string{"jira:LOAN-1482", "--name", "from-board"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdBoardCommit = %d, want 0; stderr=%s", got, stderr.String())
	}
	spec, _ := readSpec(t, repo.Dir, "from-board")
	if spec.Story != "jira:LOAN-1482" {
		t.Fatalf("spec.Story = %q", spec.Story)
	}
	if !contains(stdout.String(), "committed") {
		t.Fatalf("stdout = %q, want a committed-commit line", stdout.String())
	}
}

// TestCmdBoardCommit_Happy_ExplicitStoryRef proves the --story-ref escape
// hatch for a board keyed by something that isn't itself a scheme:key ref
// (the phase-2 corpus fixture's own "STORY-1482" shape).
func TestCmdBoardCommit_Happy_ExplicitStoryRef(t *testing.T) {
	repo := buildPhase7Repo(t)
	ensureDataGitignore(t, repo.Dir)
	writeMutableBoard(t, repo.Dir, "STORY-1482", &artifact.Board{Schema: "verdi.board/v1"})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := cmdBoardCommit([]string{"STORY-1482", "--name", "from-legacy-board", "--story-ref", "jira:LOAN-1482"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdBoardCommit = %d, want 0; stderr=%s", got, stderr.String())
	}
	spec, _ := readSpec(t, repo.Dir, "from-legacy-board")
	if spec.Story != "jira:LOAN-1482" {
		t.Fatalf("spec.Story = %q, want jira:LOAN-1482", spec.Story)
	}
}

func TestCmdBoardCommit_Negative(t *testing.T) {
	t.Run("missing --name", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		t.Chdir(repo.Dir)
		var stdout, stderr bytes.Buffer
		got := cmdBoardCommit([]string{"jira:LOAN-1482"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdBoardCommit(no --name) = %d, want 2", got)
		}
		if !contains(stderr.String(), "--name") {
			t.Fatalf("stderr = %q, want it to mention --name", stderr.String())
		}
	})
	t.Run("no board-key positional", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		t.Chdir(repo.Dir)
		var stdout, stderr bytes.Buffer
		got := cmdBoardCommit([]string{"--name", "x"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdBoardCommit(no board-key) = %d, want 2", got)
		}
	})
	t.Run("--name given twice", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		t.Chdir(repo.Dir)
		var stdout, stderr bytes.Buffer
		got := cmdBoardCommit([]string{"jira:LOAN-1482", "--name", "a", "--name", "b"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdBoardCommit(dup --name) = %d, want 2", got)
		}
	})
	t.Run("no store root", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var stdout, stderr bytes.Buffer
		got := cmdBoardCommit([]string{"jira:LOAN-1482", "--name", "x"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdBoardCommit(no store root) = %d, want 2", got)
		}
	})
}

// TestRunBoardVerb_UnknownSubcommand mirrors TestRunDesignVerb_UnknownSubcommand.
func TestRunBoardVerb_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := runBoardVerb([]string{"bogus"}, &stdout, &stderr); got != 2 {
		t.Fatalf("runBoardVerb(bogus) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := runBoardVerb(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("runBoardVerb(no args) = %d, want 2", got)
	}
}

// TestRun_BoardDispatchesToRealVerb mirrors the design/sync/matrix dispatch
// smoke tests: dispatch.go routes "board" to the real implementation.
func TestRun_BoardDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	got := run([]string{"board", "commit", "jira:LOAN-1", "--name", "x"}, &stderr)
	if got != 2 {
		t.Fatalf("run([board commit ...]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "usage") || contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}
