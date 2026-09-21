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

// fixtureStore builds a minimal fixturegit repository (the same
// .verdi/verdi.yaml manifest internal/journey/facts_integration_test.go's
// buildFactsRepo uses) carrying one active feature spec, spec/checkout,
// and returns it alongside the resolved store.Config every recognizer
// test gathers facts against.
func fixtureStore(t *testing.T) (*fixturegit.Repo, *store.Config) {
	t.Helper()
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
