package recovery

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// mustGather is Gather's own test-only "must succeed" wrapper, mirroring
// this file's other must-style helpers.
func mustGather(t *testing.T, cfg *store.Config, ref string) Facts {
	t.Helper()
	f, err := NewGatherer().Gather(context.Background(), cfg, ref)
	if err != nil {
		t.Fatalf("Gather(%s): %v", ref, err)
	}
	return f
}

// hasState reports whether p carries at least one recognized state of
// code, regardless of target.
func hasState(p Projection, code StateCode) bool {
	for _, s := range p.States {
		if s.Code == code {
			return true
		}
	}
	return false
}

// gitOutput runs a plain git command against dir and returns its
// combined output, failing the test on a non-zero exit — apply_test.go's
// own witness for a branch ref's raw tip value, independent of gitx.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// runGit runs a plain git command against dir, failing the test on a
// non-zero exit and discarding output — apply_test.go's own fixture
// mutator for operations (merge) internal/gitx does not expose.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// cutMergedRitualWorktree cuts branch (one of this ref's own ritual
// branches) from repo's current checkout with its own commit, merges it
// (--no-ff) into main, and gives it an UNMANAGED worktree (outside
// .verdi/data/worktrees, so internal/reclaim's own managed-worktree
// exclusion never fires) — the merged, clean, unmanaged worktree+branch
// unit internal/reclaim's own AC-1 predicate classifies eligible.
// Returns the worktree's own path. Leaves repo checked out on main.
func cutMergedRitualWorktree(t *testing.T, repo *fixturegit.Repo, branch string) (worktreePath string) {
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
	runGit(t, repo.Dir, "merge", "--quiet", "--no-ff", "-m", "merge "+branch, branch)

	path := filepath.Join(t.TempDir(), "unit-wt")
	if err := gitx.WorktreeAdd(ctx, repo.Dir, path, branch); err != nil {
		t.Fatalf("WorktreeAdd(%s): %v", branch, err)
	}
	return path
}

// deadPID starts and waits a `true` subprocess and returns its pid,
// guaranteed reaped and never confusable with a live process (mirrors
// internal/filelock/inspect_test.go's own helper of the same name;
// deliberately duplicated, not shared — a two-line test idiom, not
// production logic CLAUDE.md's shared-package rule targets).
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("running short-lived child: %v", err)
	}
	return cmd.Process.Pid
}

// checkoutSpecMD mirrors internal/journey/facts_test.go's testFeatureSpecMD
// shape, for the one spec name every fixture helper below targets:
// spec/checkout, a feature spec (facts.go's Gather only accepts a
// feature-class ref).
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

// hostileBodySpecMD is checkoutSpecMD with a Markdown BODY the YAML
// scanner refuses (R-RR3-22). Two ordinary authoring shapes, both taken
// from this repository's own active corpus (9 of its 41 active specs
// carry one or the other): a prose paragraph wrapped at 80 columns whose
// CONTINUATION line carries a ": " — a multi-line plain scalar followed
// by a mapping value, "yaml: mapping values are not allowed in this
// context", the exact error 4 of this store's 20 active feature specs
// produce — and a fenced code block, whose leading backtick is a
// reserved YAML indicator. Every reader that splits the front matter off
// first reads the class without noticing either. A reader that hands the
// WHOLE document to the strict YAML decoder fails on both, which is the
// defect this fixture exists to catch.
const hostileBodySpecMD = checkoutSpecMD + `
An interrupted ritual left this note, wrapped the way every spec in this
corpus wraps its prose: the colon on this continuation line is what the
YAML scanner refuses.

` + "```yaml\nkey: value: extra\n```\n"

// fixtureStore builds a minimal fixturegit repository (the same
// .verdi/verdi.yaml manifest internal/journey/facts_integration_test.go's
// buildFactsRepo uses) carrying one active feature spec, spec/checkout,
// and returns it alongside the resolved store.Config every recognizer
// test gathers facts against.
func fixtureStore(t *testing.T) (*fixturegit.Repo, *store.Config) {
	t.Helper()
	return fixtureStoreSpec(t, checkoutSpecMD)
}

// fixtureStoreSpec is fixtureStore over an arbitrary spec.md body for
// spec/checkout — the seam R-RR3-22's own regression fixture needs (a
// spec whose front matter is identical and whose BODY is not YAML).
func fixtureStoreSpec(t *testing.T, specMD string) (*fixturegit.Repo, *store.Config) {
	t.Helper()
	// Pinned empty (mirrors internal/branchbase's own buildRepo, 2A-I1):
	// several tests below rely on the default branch NOT resolving via
	// this env var, and it leaks from CI/e2e-invoking environments (this
	// repo's own e2e helpers, and GitLab CI's predefined variable, both
	// set it). A test that needs it set calls t.Setenv again afterward,
	// which wins.
	t.Setenv("CI_DEFAULT_BRANCH", "")
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                    "schema: verdi.layout/v1\nforge: gitlab\n",
				".verdi/specs/active/checkout/spec.md": specMD,
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

// fixtureStoreNoSpec is fixtureStore without spec/checkout at all — for
// R-RR3-15's own fallback-chain tests, which need a checkout where the
// target is absent from disk (and from main's own tree) so a positive
// case can plant it on exactly one fallback location and a negative case
// can leave every location empty.
func fixtureStoreNoSpec(t *testing.T) (*fixturegit.Repo, *store.Config) {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "") // see fixtureStore's own comment
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n"}, Message: "scaffold"},
	})
	cfg, err := store.Open(repo.Dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	cfg.Root = repo.Dir
	return repo, cfg
}

// cutEmptyBranch cuts name from repo's current checkout (gitx.
// CheckoutNewBranch — the cut-from-current mechanism build start and
// close both use) and stays checked out on it, mirroring an interrupted
// ritual that got as far as the branch cut and no further. Returns the
// branch's cut point (its tip immediately after the cut, before any
// further commits).
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

// writeUnstagedScaffold appends a line to spec/checkout's active spec.md
// on disk without staging it — an interrupted design start that scaffolded
// but never committed (caller must already be on design/checkout).
func writeUnstagedScaffold(t *testing.T, repo *fixturegit.Repo) {
	t.Helper()
	path := store.ActiveSpecPath(repo.Dir, "checkout")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	data = append(data, []byte("\n<!-- scaffold in progress -->\n")...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// stageClosurePaths moves spec/checkout's active zone to the archive zone
// on disk and stages both paths (the exact shape close.go's
// closureResidueName recognizes: an interrupted close that staged its
// move and never committed — caller must already be on close/checkout).
func stageClosurePaths(t *testing.T, repo *fixturegit.Repo) {
	t.Helper()
	moveArchiveUncommitted(t, repo)
	active := store.SpecDirRelPath(store.ZoneActive, "checkout")
	archive := store.SpecDirRelPath(store.ZoneArchive, "checkout")
	if err := gitx.AddPaths(context.Background(), repo.Dir, active, archive); err != nil {
		t.Fatalf("staging closure paths: %v", err)
	}
}

// moveArchiveUncommitted renames spec/checkout's spec directory from the
// active zone to the archive zone on disk only — no staging at all (an
// interrupted close that moved the directory but never even staged it).
func moveArchiveUncommitted(t *testing.T, repo *fixturegit.Repo) {
	t.Helper()
	activeDir := store.ActiveSpecDir(repo.Dir, "checkout")
	archiveDir := store.ArchiveSpecDir(repo.Dir, "checkout")
	if err := os.MkdirAll(filepath.Dir(archiveDir), 0o755); err != nil {
		t.Fatalf("creating archive zone: %v", err)
	}
	if err := os.Rename(activeDir, archiveDir); err != nil {
		t.Fatalf("moving %s to %s: %v", activeDir, archiveDir, err)
	}
}

// commitClosureNoRemote stages and commits spec/checkout's active-to-
// archive move on close/checkout — a completed-but-unpublished closure
// commit; the fixture repository carries no origin remote at all, so no
// remote-tracking branch for close/checkout exists either.
func commitClosureNoRemote(t *testing.T, repo *fixturegit.Repo) {
	t.Helper()
	stageClosurePaths(t, repo)
	if _, err := gitx.CreateCommit(context.Background(), repo.Dir, "close: archive spec/checkout"); err != nil {
		t.Fatalf("committing closure: %v", err)
	}
}

// writeStaleWriterLock writes store.WriterLockPath(root) naming a dead
// pid with an old start time, returning the lock path.
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

// writeOldEmptyWriterLock writes an EMPTY store.WriterLockPath(root) and
// backdates its mtime past filelock's own 2s mid-flush window — the
// writer that died between create and body flush (wave-review I2), the
// only LockStale branch that records no holder at all. Returns the lock
// path.
func writeOldEmptyWriterLock(t *testing.T, repo *fixturegit.Repo) string {
	t.Helper()
	path := store.WriterLockPath(repo.Dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating lock directory: %v", err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("backdating %s: %v", path, err)
	}
	return path
}

// writePreparedJournal writes store.DraftMutationJournalPath(root,
// "checkout") with a minimal {schema, spec, phase} document — the exact
// fields facts.go's permissive journalPeek decodes (internal/draftmutation/
// transaction.go:296-302's journal carries more fields than this
// projection needs).
func writePreparedJournal(t *testing.T, repo *fixturegit.Repo) {
	t.Helper()
	dir := store.DraftMutationDir(repo.Dir, "checkout")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating draft-mutation dir: %v", err)
	}
	doc := journalPeek{Schema: "verdi.draftmutation-journal/v1", Spec: "spec/checkout", Phase: "prepared"}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshaling journal: %v", err)
	}
	path := store.DraftMutationJournalPath(repo.Dir, "checkout")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// writeOrphanWorkspaceStaging creates a "<id>.request.staging" file under
// execworkspace.ExecutionRoot with no "<id>" unit directory — a
// governed-action interrupted before materialization ever produced its
// unit. Returns the workspace id used.
func writeOrphanWorkspaceStaging(t *testing.T, repo *fixturegit.Repo) string {
	t.Helper()
	const id = "checkout--0123456789ab"
	if !execworkspace.ValidWorkspaceID(id) {
		t.Fatalf("fixture id %q is not a valid workspace id", id)
	}
	dir := execworkspace.ExecutionRoot(repo.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating execution root: %v", err)
	}
	path := execworkspace.RequestStagingPath(repo.Dir, id)
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return id
}

// advanceBranch commits one new file on branch (creating it only if it
// already exists — the caller names an existing branch) and returns
// branch's new tip, leaving repo checked out on whatever branch it was on
// before. It is the fixture for R-RR3-5's ordinary "the return branch sat
// ahead of the cut" case: anyone merging into the default branch after a
// ritual branch was cut moves that branch on without touching the cut.
func advanceBranch(t *testing.T, repo *fixturegit.Repo, branch, filename string) (tip string) {
	t.Helper()
	ctx := context.Background()
	was, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if was != branch {
		if err := gitx.CheckoutExisting(ctx, repo.Dir, branch); err != nil {
			t.Fatalf("CheckoutExisting(%s): %v", branch, err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, filename), []byte(filename+"\n"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", filename, err)
	}
	if err := gitx.AddPaths(ctx, repo.Dir, filename); err != nil {
		t.Fatalf("AddPaths(%s): %v", filename, err)
	}
	if _, err := gitx.CreateCommit(ctx, repo.Dir, "advance "+branch+" with "+filename); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
	tip, err = gitx.RevParse(ctx, repo.Dir, branch)
	if err != nil {
		t.Fatalf("RevParse(%s): %v", branch, err)
	}
	if was != branch {
		if err := gitx.CheckoutExisting(ctx, repo.Dir, was); err != nil {
			t.Fatalf("CheckoutExisting(%s): %v", was, err)
		}
	}
	return tip
}

// holdBranchInLinkedWorktree checks branch out in a linked worktree
// outside the store, so git's own `branch -d` safely REFUSES to delete it
// ("checked out at ..."): the reachable state in which branchcut.Unwind
// switches back correctly but gives up on the delete, which a
// postcondition must report as VIOLATED rather than swallow.
func holdBranchInLinkedWorktree(t *testing.T, repo *fixturegit.Repo, branch string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "held-wt")
	if err := gitx.WorktreeAdd(context.Background(), repo.Dir, path, branch); err != nil {
		t.Fatalf("WorktreeAdd(%s): %v", branch, err)
	}
	return path
}
