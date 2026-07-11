package evidence

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/fixturegit"
)

// buildRecordsRepo returns a two-layer fixturegit repo (layer 1 is layer
// 2's parent), mirroring internal/gitx's own test helper shape.
func buildRecordsRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "layer 1"},
		{Files: map[string]string{"a.txt": "hello again\n"}, Message: "layer 2"},
	})
}

// writeDerivedVerdicts writes a verdicts.json under derivedRoot's
// <commit>/ subdirectory.
func writeDerivedVerdicts(t *testing.T, derivedRoot, commit, json string) {
	t.Helper()
	dir := filepath.Join(derivedRoot, commit)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "verdicts.json"), []byte(json), 0o644); err != nil {
		t.Fatalf("writing verdicts.json: %v", err)
	}
}

func recordJSON(commit, source string) string {
	return `[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"],"kind":"static","verdict":"pass",` +
		`"witness":"w","provenance":{"source":"` + source + `","pipeline":"1","commit":"` + commit + `"},` +
		`"digest":"sha256:` + hex64 + `"}]`
}

// TestLoadRecords_Happy proves records from an ancestor commit (and from
// C itself) are loaded, both provenance sources alike (Fold, not
// LoadRecords, filters by source).
func TestLoadRecords_Happy(t *testing.T) {
	repo := buildRecordsRepo(t)
	derivedRoot := filepath.Join(repo.Dir, "derived", "spec--test")
	writeDerivedVerdicts(t, derivedRoot, repo.Heads[0], recordJSON(repo.Heads[0], "ci"))
	writeDerivedVerdicts(t, derivedRoot, repo.Heads[1], recordJSON(repo.Heads[1], "local"))

	got, err := LoadRecords(context.Background(), repo.Dir, derivedRoot, repo.Head)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("LoadRecords = %d records, want 2 (one from each ancestor-or-self commit)", len(got))
	}
}

// TestLoadRecords_AncestryFiltering proves a record from a commit that is
// not an ancestor of C (a diverged sibling commit, real in this repo's
// object database but not in C's history) is ignored — 03 §The fold:
// "current ... whose commit is an ancestor of C".
func TestLoadRecords_AncestryFiltering(t *testing.T) {
	repo := buildRecordsRepo(t)
	ctx := context.Background()

	// Branch a sibling commit off layer 1 that layer 2 (repo.Head) does
	// not descend from.
	sibling := branchSiblingCommit(t, repo.Dir, repo.Heads[0])

	derivedRoot := filepath.Join(repo.Dir, "derived", "spec--test")
	writeDerivedVerdicts(t, derivedRoot, repo.Heads[0], recordJSON(repo.Heads[0], "ci"))
	writeDerivedVerdicts(t, derivedRoot, sibling, recordJSON(sibling, "ci"))

	got, err := LoadRecords(ctx, repo.Dir, derivedRoot, repo.Head)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("LoadRecords = %d records, want 1 (the sibling commit's record must be filtered out)", len(got))
	}
	if got[0].Provenance.Commit != repo.Heads[0] {
		t.Fatalf("LoadRecords()[0].Provenance.Commit = %q, want the ancestor commit %q, not the sibling %q", got[0].Provenance.Commit, repo.Heads[0], sibling)
	}
}

// branchSiblingCommit checks out a new branch at parentCommit, commits a
// new file, and returns the new commit's sha, leaving the repo back on
// its original branch (main) afterward — a real commit in dir's object
// database that repo.Head does not descend from.
func branchSiblingCommit(t *testing.T, dir, parentCommit string) string {
	t.Helper()
	runGit(t, dir, "checkout", "--quiet", "-b", "sibling", parentCommit)
	if err := os.WriteFile(filepath.Join(dir, "sibling-only.txt"), []byte("sibling\n"), 0o644); err != nil {
		t.Fatalf("writing sibling-only.txt: %v", err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "-c", "user.name=t", "-c", "user.email=t@t.invalid", "commit", "--quiet", "--no-verify", "-m", "sibling commit")
	sha := strings.TrimSpace(runGitOutput(t, dir, "rev-parse", "HEAD"))
	runGit(t, dir, "checkout", "--quiet", "main")
	return sha
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// TestLoadRecords_MissingDerivedRoot proves a story with no derived data
// yet reads as "no records", not an operational error.
func TestLoadRecords_MissingDerivedRoot(t *testing.T) {
	repo := buildRecordsRepo(t)
	got, err := LoadRecords(context.Background(), repo.Dir, filepath.Join(repo.Dir, "derived", "never-synced"), repo.Head)
	if err != nil {
		t.Fatalf("LoadRecords(missing derivedRoot): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadRecords(missing derivedRoot) = %v, want empty", got)
	}
}

// TestLoadRecords_Negative proves a malformed verdicts.json (on disk, at
// an ancestor commit) is a real, surfaced error — broken derived data is
// worse than absent.
func TestLoadRecords_Negative(t *testing.T) {
	repo := buildRecordsRepo(t)
	derivedRoot := filepath.Join(repo.Dir, "derived", "spec--test")
	writeDerivedVerdicts(t, derivedRoot, repo.Heads[0], "not valid json")

	if _, err := LoadRecords(context.Background(), repo.Dir, derivedRoot, repo.Head); err == nil {
		t.Fatal("LoadRecords(malformed verdicts.json): want error, got nil")
	}
}

// TestLoadRecords_SkipsNonCommitShapedEntries proves a stray non-commit
// directory under the ref-slug tree (e.g. an editor/OS artifact) is
// skipped rather than erroring as an unresolvable ancestry check.
func TestLoadRecords_SkipsNonCommitShapedEntries(t *testing.T) {
	repo := buildRecordsRepo(t)
	derivedRoot := filepath.Join(repo.Dir, "derived", "spec--test")
	if err := os.MkdirAll(filepath.Join(derivedRoot, "views"), 0o755); err != nil {
		t.Fatalf("mkdir views: %v", err)
	}

	got, err := LoadRecords(context.Background(), repo.Dir, derivedRoot, repo.Head)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadRecords = %v, want empty (only a non-commit-shaped dir present)", got)
	}
}
