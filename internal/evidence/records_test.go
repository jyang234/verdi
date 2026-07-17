package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
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

// TestLoadRecordsWithSources_Happy proves the manifest names exactly the
// record files the walk read (existing files under ancestor-or-self
// commit directories), each with the sha256 of the exact bytes read —
// spec/evidence-slot dc-3's receipt inputs, produced by the ONE loader
// walk rather than a second one (co-3).
func TestLoadRecordsWithSources_Happy(t *testing.T) {
	repo := buildRecordsRepo(t)
	derivedRoot := filepath.Join(repo.Dir, "derived", "spec--test")
	body := recordJSON(repo.Heads[0], "ci")
	writeDerivedVerdicts(t, derivedRoot, repo.Heads[0], body)

	recs, sources, err := LoadRecordsWithSources(context.Background(), repo.Dir, derivedRoot, repo.Head)
	if err != nil {
		t.Fatalf("LoadRecordsWithSources: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("records = %d, want 1", len(recs))
	}
	if len(sources) != 1 {
		t.Fatalf("sources = %+v, want exactly one entry (runtime.json does not exist and must not be listed)", sources)
	}
	wantPath := repo.Heads[0] + "/verdicts.json"
	if sources[0].Path != wantPath {
		t.Errorf("sources[0].Path = %q, want %q (slash-separated, derivedRoot-relative)", sources[0].Path, wantPath)
	}
	sum := sha256.Sum256([]byte(body))
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	if sources[0].Digest != wantDigest {
		t.Errorf("sources[0].Digest = %q, want %q (the exact bytes read)", sources[0].Digest, wantDigest)
	}
}

// TestExcludedCommitDirs_Happy proves a non-ancestor sibling commit
// directory is named (spec/close-preflight dc-4's "found but excluded"
// disclosure) while an ancestor-or-self commit directory — the ordinary,
// nothing-to-disclose case — is not.
func TestExcludedCommitDirs_Happy(t *testing.T) {
	repo := buildRecordsRepo(t)
	ctx := context.Background()
	sibling := branchSiblingCommit(t, repo.Dir, repo.Heads[0])

	derivedRoot := filepath.Join(repo.Dir, "derived", "spec--test")
	writeDerivedVerdicts(t, derivedRoot, repo.Heads[0], recordJSON(repo.Heads[0], "ci"))
	writeDerivedVerdicts(t, derivedRoot, sibling, recordJSON(sibling, "ci"))

	got, err := ExcludedCommitDirs(ctx, repo.Dir, derivedRoot, repo.Head)
	if err != nil {
		t.Fatalf("ExcludedCommitDirs: %v", err)
	}
	if len(got) != 1 || got[0] != sibling {
		t.Fatalf("ExcludedCommitDirs = %v, want exactly [%s] (the ancestor commit dir must not be named)", got, sibling)
	}
}

// TestExcludedCommitDirs_NoExclusions proves an all-ancestor derived tree
// (the ordinary case) reports no exclusions at all — nil, not an empty-but-
// non-nil slice a caller might render as an empty bracketed list.
func TestExcludedCommitDirs_NoExclusions(t *testing.T) {
	repo := buildRecordsRepo(t)
	derivedRoot := filepath.Join(repo.Dir, "derived", "spec--test")
	writeDerivedVerdicts(t, derivedRoot, repo.Heads[0], recordJSON(repo.Heads[0], "ci"))

	got, err := ExcludedCommitDirs(context.Background(), repo.Dir, derivedRoot, repo.Head)
	if err != nil {
		t.Fatalf("ExcludedCommitDirs: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ExcludedCommitDirs = %v, want none excluded", got)
	}
}

// TestExcludedCommitDirs_MissingDerivedRoot proves a never-synced story
// (no derived tree on disk at all) reads as "nothing excluded", not an
// error — mirroring LoadRecordsWithSources's own never-synced posture.
func TestExcludedCommitDirs_MissingDerivedRoot(t *testing.T) {
	repo := buildRecordsRepo(t)
	got, err := ExcludedCommitDirs(context.Background(), repo.Dir, filepath.Join(repo.Dir, "derived", "never-synced"), repo.Head)
	if err != nil {
		t.Fatalf("ExcludedCommitDirs(missing derivedRoot): %v", err)
	}
	if got != nil {
		t.Fatalf("ExcludedCommitDirs(missing derivedRoot) = %v, want nil", got)
	}
}

// TestExcludedCommitDirs_SkipsNonCommitShapedEntries proves a stray
// non-commit-shaped directory (an editor/OS artifact) is silently skipped,
// mirroring LoadRecords' own tolerance.
func TestExcludedCommitDirs_SkipsNonCommitShapedEntries(t *testing.T) {
	repo := buildRecordsRepo(t)
	derivedRoot := filepath.Join(repo.Dir, "derived", "spec--test")
	if err := os.MkdirAll(filepath.Join(derivedRoot, "views"), 0o755); err != nil {
		t.Fatalf("mkdir views: %v", err)
	}

	got, err := ExcludedCommitDirs(context.Background(), repo.Dir, derivedRoot, repo.Head)
	if err != nil {
		t.Fatalf("ExcludedCommitDirs: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ExcludedCommitDirs = %v, want none (a non-commit-shaped dir must be skipped, not misread as excluded)", got)
	}
}

// TestExcludedCommitDirs_Negative proves a genuine ancestry-check failure
// (a commit-shaped directory name that resolves to no real commit at all —
// distinct from a real, merely-non-ancestor sibling) is a surfaced
// operational error, not a silent "excluded".
func TestExcludedCommitDirs_Negative(t *testing.T) {
	repo := buildRecordsRepo(t)
	derivedRoot := filepath.Join(repo.Dir, "derived", "spec--test")
	// Commit-shaped (matches commitDirRe) but not a real object in this
	// repo's history at all.
	writeDerivedVerdicts(t, derivedRoot, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", recordJSON("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "ci"))

	if _, err := ExcludedCommitDirs(context.Background(), repo.Dir, derivedRoot, repo.Head); err == nil {
		t.Fatal("ExcludedCommitDirs(unresolvable commit dir name): want error, got nil")
	}
}

// TestLoadRecordsWithSources_Negative proves the manifest never cites
// what the walk did not read: a non-ancestor sibling commit's file is
// excluded, and a missing derivedRoot yields a nil manifest and nil
// error (the never-synced authoring state, not a failure).
func TestLoadRecordsWithSources_Negative(t *testing.T) {
	repo := buildRecordsRepo(t)
	ctx := context.Background()

	sibling := branchSiblingCommit(t, repo.Dir, repo.Heads[0])
	derivedRoot := filepath.Join(repo.Dir, "derived", "spec--test")
	writeDerivedVerdicts(t, derivedRoot, sibling, recordJSON(sibling, "ci"))

	recs, sources, err := LoadRecordsWithSources(ctx, repo.Dir, derivedRoot, repo.Head)
	if err != nil {
		t.Fatalf("LoadRecordsWithSources: %v", err)
	}
	if len(recs) != 0 || len(sources) != 0 {
		t.Fatalf("records = %+v, sources = %+v, want both empty (sibling commit is not an ancestor)", recs, sources)
	}

	recs, sources, err = LoadRecordsWithSources(ctx, repo.Dir, filepath.Join(repo.Dir, "derived", "never-synced"), repo.Head)
	if err != nil {
		t.Fatalf("LoadRecordsWithSources(missing derivedRoot): %v", err)
	}
	if recs != nil || sources != nil {
		t.Fatalf("missing derivedRoot: records = %+v, sources = %+v, want nil/nil", recs, sources)
	}
}
