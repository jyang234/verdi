package recovery

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/branchbase"
	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// currentHead and porcelainStatus mirror internal/journey/
// facts_integration_test.go's identically-named helpers (deliberately
// duplicated, not shared: a two-line test idiom, not production logic).
func currentHead(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func porcelainStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status --porcelain: %v", err)
	}
	return string(out)
}

func containsString(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func containsSubstring(ss []string, want string) bool {
	for _, s := range ss {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}

func gather(t *testing.T, cfg *store.Config, ref string) Facts {
	t.Helper()
	f, err := NewGatherer().Gather(context.Background(), cfg, ref)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	return f
}

// fabricateRemoteRef seeds refs/remotes/origin/<branch> at commit
// directly — no clone, no fetch, no network (mirrors internal/branchbase's
// own test helper of the same name).
func fabricateRemoteRef(t *testing.T, dir, branch, commit string) {
	t.Helper()
	if err := gitx.UpdateRef(context.Background(), dir, "refs/remotes/origin/"+branch, commit); err != nil {
		t.Fatalf("seeding refs/remotes/origin/%s: %v", branch, err)
	}
}

func setSymbolicRef(t *testing.T, dir, name, target string) {
	t.Helper()
	cmd := exec.Command("git", "symbolic-ref", name, target)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git symbolic-ref %s %s: %v\n%s", name, target, err, out)
	}
}

func addOriginRemote(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "remote", "add", "origin", "https://example.invalid/repo.git")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add origin: %v\n%s", err, out)
	}
}

// TestGather_EmptyBranchCut_CutFromCurrent covers R-RR3-5's cut-from-
// current mechanism (close/build start): a branch cut straight from
// "main" with no commits of its own reads empty, naming "main" as its
// witness.
func TestGather_EmptyBranchCut_CutFromCurrent(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutPoint := cutEmptyBranch(t, repo, "close/checkout")

	f := gather(t, cfg, "spec/checkout")
	if !f.Close.Exists || !f.Close.Empty() {
		t.Fatalf("Close = %+v, want existing and empty", f.Close)
	}
	if f.Close.Tip != cutPoint {
		t.Fatalf("Close.Tip = %q, want %q", f.Close.Tip, cutPoint)
	}
	if !containsString(f.Close.EmptyWitnesses, "main") {
		t.Fatalf("EmptyWitnesses = %v, want to contain main", f.Close.EmptyWitnesses)
	}
	if f.CurrentBranch != "close/checkout" {
		t.Fatalf("CurrentBranch = %q, want close/checkout", f.CurrentBranch)
	}
}

// TestGather_EmptyBranchCut_NonDefaultCurrentBranch is the spike's own
// load-bearing negative: a ritual branch cut from a non-default current
// branch that is itself two commits ahead of main must still read empty
// against ITS OWN base, never falsely non-empty against main.
func TestGather_EmptyBranchCut_NonDefaultCurrentBranch(t *testing.T) {
	repo, cfg := fixtureStore(t)
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "feature/other"); err != nil {
		t.Fatalf("CheckoutNewBranch(feature/other): %v", err)
	}
	for i := 0; i < 2; i++ {
		content := fmt.Sprintf("more %d\n", i)
		if err := os.WriteFile(repo.Dir+"/other.txt", []byte(content), 0o644); err != nil {
			t.Fatalf("writing other.txt: %v", err)
		}
		if err := gitx.AddPaths(ctx, repo.Dir, "other.txt"); err != nil {
			t.Fatalf("AddPaths: %v", err)
		}
		if _, err := gitx.CreateCommit(ctx, repo.Dir, "feature/other work"); err != nil {
			t.Fatalf("CreateCommit: %v", err)
		}
	}
	cutEmptyBranch(t, repo, "close/checkout")

	f := gather(t, cfg, "spec/checkout")
	if !f.Close.Exists || !f.Close.Empty() {
		t.Fatalf("Close = %+v, want existing and empty (cut from feature/other, not main)", f.Close)
	}
	if !containsString(f.Close.EmptyWitnesses, "feature/other") {
		t.Fatalf("EmptyWitnesses = %v, want to contain feature/other", f.Close.EmptyWitnesses)
	}
}

// TestGather_OneOwnCommit_NotEmpty is R-RR3-5's other load-bearing
// negative: a branch with exactly one commit of its own must never read
// empty.
func TestGather_OneOwnCommit_NotEmpty(t *testing.T) {
	repo, cfg := fixtureStore(t)
	ctx := context.Background()
	cutEmptyBranch(t, repo, "close/checkout")
	if err := os.WriteFile(repo.Dir+"/own.txt", []byte("own work\n"), 0o644); err != nil {
		t.Fatalf("writing own.txt: %v", err)
	}
	if err := gitx.AddPaths(ctx, repo.Dir, "own.txt"); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, repo.Dir, "close/checkout's own commit"); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}

	f := gather(t, cfg, "spec/checkout")
	if !f.Close.Exists || f.Close.Empty() {
		t.Fatalf("Close = %+v, want existing and NOT empty (it has its own commit)", f.Close)
	}
}

// TestGather_BaseMovedOn_StillEmpty proves the ancestry predicate survives
// the base advancing after the cut (the spike's other negative): main
// gaining a commit after close/checkout was cut must not flip it to
// non-empty.
func TestGather_BaseMovedOn_StillEmpty(t *testing.T) {
	repo, cfg := fixtureStore(t)
	ctx := context.Background()
	cutEmptyBranch(t, repo, "close/checkout")

	if err := gitx.CheckoutExisting(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main): %v", err)
	}
	if err := os.WriteFile(repo.Dir+"/later.txt", []byte("later\n"), 0o644); err != nil {
		t.Fatalf("writing later.txt: %v", err)
	}
	if err := gitx.AddPaths(ctx, repo.Dir, "later.txt"); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, repo.Dir, "main moved on"); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}

	f := gather(t, cfg, "spec/checkout")
	if !f.Close.Exists || !f.Close.Empty() {
		t.Fatalf("Close = %+v, want existing and still empty after the base moved on", f.Close)
	}
}

// TestGather_NoOriginRemote_HeadFallback covers variant (iii): no origin
// remote at all.
func TestGather_NoOriginRemote_HeadFallback(t *testing.T) {
	_, cfg := fixtureStore(t)
	f := gather(t, cfg, "spec/checkout")
	if f.DefaultBranchResolved {
		t.Fatalf("DefaultBranchResolved = true, want false (no origin remote)")
	}
	if f.DefaultBranch.Kind != branchbase.HeadFallback {
		t.Fatalf("DefaultBranch.Kind = %v, want HeadFallback", f.DefaultBranch.Kind)
	}
	if !containsSubstring(f.Disclosures, "disclosed, not a default-branch base") {
		t.Fatalf("Disclosures = %v, want the HEAD-fallback disclosure", f.Disclosures)
	}
	if !containsSubstring(f.Disclosures, "stranded-residue scan skipped") {
		t.Fatalf("Disclosures = %v, want the residue-scan-skipped disclosure", f.Disclosures)
	}
}

// TestGather_OriginHeadAndRemoteBranch covers variant (i): both
// refs/remotes/origin/HEAD (symref) and a concrete refs/remotes/origin/
// <branch> object set — the resolveBranchRef path the spike proved a
// symref-only fixture never exercises.
func TestGather_OriginHeadAndRemoteBranch(t *testing.T) {
	repo, cfg := fixtureStore(t)
	fabricateRemoteRef(t, repo.Dir, "main", repo.Head)
	setSymbolicRef(t, repo.Dir, "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	f := gather(t, cfg, "spec/checkout")
	if !f.DefaultBranchResolved {
		t.Fatalf("DefaultBranchResolved = false, want true")
	}
	if f.DefaultBranch.BranchName != "main" || f.DefaultBranch.Ref != "origin/main" {
		t.Fatalf("DefaultBranch = %+v, want main/origin/main", f.DefaultBranch)
	}
	if !f.ResidueScanned {
		t.Fatalf("ResidueScanned = false, want true once the default branch resolves")
	}
}

// TestGather_OriginConfiguredButUnresolvable covers the Unresolvable case:
// origin exists but no default branch resolves from it.
func TestGather_OriginConfiguredButUnresolvable(t *testing.T) {
	repo, cfg := fixtureStore(t)
	addOriginRemote(t, repo.Dir)

	f := gather(t, cfg, "spec/checkout")
	if f.DefaultBranchResolved {
		t.Fatalf("DefaultBranchResolved = true, want false")
	}
	if f.DefaultBranch.Kind != branchbase.Unresolvable {
		t.Fatalf("DefaultBranch.Kind = %v, want Unresolvable", f.DefaultBranch.Kind)
	}
	if !containsSubstring(f.Disclosures, "could not be resolved") {
		t.Fatalf("Disclosures = %v, want the unresolvable-default-branch disclosure", f.Disclosures)
	}
}

// TestGather_ScaffoldUnstaged proves an interrupted design start's
// unstaged scaffold edit is observed as a changed, unstaged path under
// the active spec directory.
func TestGather_ScaffoldUnstaged(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "design/checkout")
	writeUnstagedScaffold(t, repo)

	f := gather(t, cfg, "spec/checkout")
	if len(f.StagedPaths) != 0 {
		t.Fatalf("StagedPaths = %v, want none", f.StagedPaths)
	}
	wantPath := store.SpecRelPath(store.ZoneActive, "checkout")
	if !containsString(f.WorktreeChangedPaths, wantPath) {
		t.Fatalf("WorktreeChangedPaths = %v, want to contain %s", f.WorktreeChangedPaths, wantPath)
	}
	if !f.Dirty {
		t.Fatal("Dirty = false, want true")
	}
}

// TestGather_ArtifactsStagedUncommitted proves an interrupted close's
// staged active-to-archive move is observed on StagedPaths.
func TestGather_ArtifactsStagedUncommitted(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")
	stageClosurePaths(t, repo)

	f := gather(t, cfg, "spec/checkout")
	active := store.SpecDirRelPath(store.ZoneActive, "checkout") + "/spec.md"
	archive := store.SpecDirRelPath(store.ZoneArchive, "checkout") + "/spec.md"
	if !containsString(f.StagedPaths, active) {
		t.Fatalf("StagedPaths = %v, want to contain %s", f.StagedPaths, active)
	}
	if !containsString(f.StagedPaths, archive) {
		t.Fatalf("StagedPaths = %v, want to contain %s", f.StagedPaths, archive)
	}
}

// TestGather_ArchiveMoveUncommitted proves an on-disk-only archive move is
// observed as archive-present/active-absent on disk while HEAD's tree
// still shows the pre-move shape.
func TestGather_ArchiveMoveUncommitted(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")
	moveArchiveUncommitted(t, repo)

	f := gather(t, cfg, "spec/checkout")
	if f.ActiveSpecOnDisk {
		t.Fatal("ActiveSpecOnDisk = true, want false")
	}
	if !f.ArchiveSpecOnDisk {
		t.Fatal("ArchiveSpecOnDisk = false, want true")
	}
	if !f.ActiveSpecAtHead {
		t.Fatal("ActiveSpecAtHead = false, want true (the move was never committed)")
	}
	if f.ArchiveSpecAtHead {
		t.Fatal("ArchiveSpecAtHead = true, want false (the move was never committed)")
	}
}

// TestGather_ClosureCommittedNoRemote proves a completed, unpublished
// closure commit is observed as a non-empty close branch with no
// remote-tracking branch.
func TestGather_ClosureCommittedNoRemote(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")
	commitClosureNoRemote(t, repo)

	f := gather(t, cfg, "spec/checkout")
	if !f.Close.Exists || f.Close.Empty() {
		t.Fatalf("Close = %+v, want existing and not empty", f.Close)
	}
	if f.Close.HasRemoteTracking {
		t.Fatal("Close.HasRemoteTracking = true, want false (no origin remote at all)")
	}
}

// TestGather_StaleWriterLock proves a dead-pid writer lock is inspected
// as stale.
func TestGather_StaleWriterLock(t *testing.T) {
	repo, cfg := fixtureStore(t)
	path := writeStaleWriterLock(t, repo)

	f := gather(t, cfg, "spec/checkout")
	if f.WriterLock.Path != path {
		t.Fatalf("WriterLock.Path = %q, want %q", f.WriterLock.Path, path)
	}
	if f.WriterLock.Inspection.Status != filelock.LockStale {
		t.Fatalf("WriterLock.Inspection.Status = %v, want LockStale", f.WriterLock.Inspection.Status)
	}
}

// TestGather_PreparedJournal proves a prepared draft-mutation journal is
// peeked correctly (schema/spec/phase only).
func TestGather_PreparedJournal(t *testing.T) {
	repo, cfg := fixtureStore(t)
	writePreparedJournal(t, repo)

	f := gather(t, cfg, "spec/checkout")
	if !f.Journal.Present {
		t.Fatal("Journal.Present = false, want true")
	}
	if f.Journal.Phase != "prepared" || f.Journal.Spec != "spec/checkout" {
		t.Fatalf("Journal = %+v, want phase=prepared spec=spec/checkout", f.Journal)
	}
}

// TestGather_SymlinkedJournal_Disclosed proves a symlinked journal path is
// disclosed and never followed.
func TestGather_SymlinkedJournal_Disclosed(t *testing.T) {
	repo, cfg := fixtureStore(t)
	dir := store.DraftMutationDir(repo.Dir, "checkout")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating draft-mutation dir: %v", err)
	}
	target := repo.Dir + "/.verdi/verdi.yaml"
	link := store.DraftMutationJournalPath(repo.Dir, "checkout")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlinking journal path: %v", err)
	}

	f := gather(t, cfg, "spec/checkout")
	if f.Journal.Present {
		t.Fatal("Journal.Present = true, want false (a symlink is never followed)")
	}
	if !containsSubstring(f.Disclosures, "symlink") {
		t.Fatalf("Disclosures = %v, want a symlink disclosure", f.Disclosures)
	}
}

// TestGather_OrphanWorkspaceStaging proves a ".request.staging" sibling
// with no unit directory is observed as one WorkspaceUnit with no unit.
func TestGather_OrphanWorkspaceStaging(t *testing.T) {
	repo, cfg := fixtureStore(t)
	id := writeOrphanWorkspaceStaging(t, repo)

	f := gather(t, cfg, "spec/checkout")
	if len(f.WorkspaceUnits) != 1 {
		t.Fatalf("WorkspaceUnits = %v, want exactly one", f.WorkspaceUnits)
	}
	u := f.WorkspaceUnits[0]
	if u.ID != id || u.HasUnit || !u.HasRequestStaging {
		t.Fatalf("WorkspaceUnits[0] = %+v, want id=%s, no unit, request-staging present", u, id)
	}
}

// TestGather_ReadOnly proves Gather changes neither HEAD nor the working
// tree's status (mirrors internal/journey's own facts_integration_test.go
// idiom).
func TestGather_ReadOnly(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")

	headBefore := currentHead(t, repo.Dir)
	statusBefore := porcelainStatus(t, repo.Dir)

	gather(t, cfg, "spec/checkout")

	if headBefore != currentHead(t, repo.Dir) {
		t.Fatalf("HEAD changed: before=%s after=%s", headBefore, currentHead(t, repo.Dir))
	}
	if statusBefore != porcelainStatus(t, repo.Dir) {
		t.Fatalf("git status --porcelain changed: before=%q after=%q", statusBefore, porcelainStatus(t, repo.Dir))
	}
}

// TestGather_RejectsStoryRef proves Gather refuses a story-class spec ref
// with an operational error naming its class.
func TestGather_RejectsStoryRef(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n",
				".verdi/specs/active/onboarding/spec.md": `---
id: spec/onboarding
kind: spec
class: story
title: "Onboarding"
owners: [platform-team]
story: jira:ONB-1
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/checkout#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
---
# body
`,
			},
			Message: "scaffold",
		},
	})
	cfg, err := store.Open(repo.Dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	cfg.Root = repo.Dir

	if _, err := NewGatherer().Gather(context.Background(), cfg, "spec/onboarding"); err == nil {
		t.Fatal("Gather(story ref) = nil error, want an operational error")
	} else if !strings.Contains(err.Error(), "story") {
		t.Fatalf("Gather(story ref) error = %v, want it to name the story class", err)
	}
}

// TestGather_RejectsNonSpecRef proves Gather refuses a non-spec kind ref.
func TestGather_RejectsNonSpecRef(t *testing.T) {
	repo, cfg := fixtureStore(t)
	if _, err := NewGatherer().Gather(context.Background(), cfg, "adr/some-decision"); err == nil {
		t.Fatal("Gather(adr ref) = nil error, want an operational error")
	}
	_ = repo
}

// TestGather_SpecClassFromDesignBranch is R-RR3-15's own positive case:
// the checkout is on "main", spec/checkout does not exist there at all,
// but design/checkout carries its own committed scaffold — Gather must
// still succeed, reading the class from that branch's own tree via
// gitx.Show.
func TestGather_SpecClassFromDesignBranch(t *testing.T) {
	repo, cfg := fixtureStoreNoSpec(t)
	ctx := context.Background()

	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "design/checkout"); err != nil {
		t.Fatalf("CheckoutNewBranch(design/checkout): %v", err)
	}
	specDir := store.ActiveSpecDir(repo.Dir, "checkout")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("creating %s: %v", specDir, err)
	}
	if err := os.WriteFile(store.ActiveSpecPath(repo.Dir, "checkout"), []byte(checkoutSpecMD), 0o644); err != nil {
		t.Fatalf("writing spec.md: %v", err)
	}
	if err := gitx.AddPaths(ctx, repo.Dir, store.SpecDirRelPath(store.ZoneActive, "checkout")); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, repo.Dir, "design: scaffold spec/checkout"); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
	if err := gitx.CheckoutExisting(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main): %v", err)
	}
	if pathExists(store.ActiveSpecPath(repo.Dir, "checkout")) {
		t.Fatal("test setup bug: spec/checkout is visible on disk on main")
	}

	f := gather(t, cfg, "spec/checkout")
	if f.CurrentBranch != "main" {
		t.Fatalf("CurrentBranch = %q, want main", f.CurrentBranch)
	}
	if !f.Design.Exists {
		t.Fatal("Design.Exists = false, want true")
	}
}

// TestGather_SpecClassNotFoundAnywhere is R-RR3-15's own negative case:
// no on-disk zone, no design/<name> or close/<name> branch, and no
// resolvable default branch carries the spec — Gather must refuse with
// an operational error naming every location it tried.
func TestGather_SpecClassNotFoundAnywhere(t *testing.T) {
	_, cfg := fixtureStoreNoSpec(t)
	_, err := NewGatherer().Gather(context.Background(), cfg, "spec/checkout")
	if err == nil {
		t.Fatal("Gather = nil error, want an operational error naming every location tried")
	}
	for _, want := range []string{"on-disk active zone", "on-disk archive zone", "design/checkout", "close/checkout"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %v does not name location %q", err, want)
		}
	}
}

// TestGather_UnprovenResidue_SkipsReclaimAndDiscloses is R-RR3-14's own
// guard, previously disclosed as untested (2B-F7): a malformed spec
// anywhere in the active-zone corpus poisons the successor-corpus scan's
// completeness proof for every OTHER active-zone spec too (the exact
// shape internal/residue/scan_test.go's own TestScan_UnprovenSpec_
// NamedInResult documents), so residue.Scan reports at least one
// UnprovenSpec; Gather must then skip reclaim.Compute entirely and
// disclose gc's own refusal sentence, mirroring gc's own behavior.
func TestGather_UnprovenResidue_SkipsReclaimAndDiscloses(t *testing.T) {
	repo, cfg := fixtureStore(t)
	malformedDir := repo.Dir + "/.verdi/specs/active/malformed"
	if err := os.MkdirAll(malformedDir, 0o755); err != nil {
		t.Fatalf("creating malformed spec dir: %v", err)
	}
	malformed := "---\nid: spec/malformed\nkind: spec\nclass: feature\nunknown_field: true\n---\nbody\n"
	if err := os.WriteFile(malformedDir+"/spec.md", []byte(malformed), 0o644); err != nil {
		t.Fatalf("writing malformed spec: %v", err)
	}
	// specstate's own successor-corpus scan reads via git (LsTree/Show at
	// the default branch ref), never the working tree — the malformed
	// spec must be committed to poison anything.
	ctx := context.Background()
	if err := gitx.AddPaths(ctx, repo.Dir, ".verdi/specs/active/malformed"); err != nil {
		t.Fatalf("staging malformed spec: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, repo.Dir, "seed a malformed spec"); err != nil {
		t.Fatalf("committing malformed spec: %v", err)
	}
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	f := gather(t, cfg, "spec/checkout")
	if !f.ResidueScanned {
		t.Fatal("ResidueScanned = false, want true")
	}
	if len(f.Residue.UnprovenSpecs) == 0 {
		t.Fatal("Residue.UnprovenSpecs is empty, want at least one (the malformed spec poisons the corpus)")
	}
	if f.ReclaimRows != nil {
		t.Fatalf("ReclaimRows = %v, want nil: R-RR3-14 refuses to compute a plan over an incomplete scan", f.ReclaimRows)
	}
	if !containsSubstring(f.Disclosures, gcUnprovenSpecsRefusal) {
		t.Fatalf("Disclosures = %v, want gc's own refusal sentence", f.Disclosures)
	}
}

// TestGather_SpecClassFromCloseBranch is R-RR3-15's close/<name> leg
// (2B-F10: previously untested): the checkout is on "main", spec/checkout
// does not exist there at all, but close/checkout carries its own
// committed copy (the shape a close ritual's own cut-point commit
// leaves, before any archive move).
func TestGather_SpecClassFromCloseBranch(t *testing.T) {
	repo, cfg := fixtureStoreNoSpec(t)
	ctx := context.Background()

	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "close/checkout"); err != nil {
		t.Fatalf("CheckoutNewBranch(close/checkout): %v", err)
	}
	specDir := store.ActiveSpecDir(repo.Dir, "checkout")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("creating %s: %v", specDir, err)
	}
	if err := os.WriteFile(store.ActiveSpecPath(repo.Dir, "checkout"), []byte(checkoutSpecMD), 0o644); err != nil {
		t.Fatalf("writing spec.md: %v", err)
	}
	if err := gitx.AddPaths(ctx, repo.Dir, store.SpecDirRelPath(store.ZoneActive, "checkout")); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, repo.Dir, "close: cut close/checkout"); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
	if err := gitx.CheckoutExisting(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main): %v", err)
	}
	if pathExists(store.ActiveSpecPath(repo.Dir, "checkout")) {
		t.Fatal("test setup bug: spec/checkout is visible on disk on main")
	}

	f := gather(t, cfg, "spec/checkout")
	if f.CurrentBranch != "main" {
		t.Fatalf("CurrentBranch = %q, want main", f.CurrentBranch)
	}
	if !f.Close.Exists {
		t.Fatal("Close.Exists = false, want true")
	}
}

// buildUnrelatedCheckoutRepo builds a fixtureStoreNoSpec repo, lands
// writeToDefault's own commit(s) on main, then switches to a THIRD
// branch ("unrelated", cut from main after landing) with no on-disk
// trace of spec/checkout and neither a design/checkout nor a
// close/checkout branch at all — the shape that can only be resolved via
// R-RR3-15's default-branch-base leg.
func buildUnrelatedCheckoutRepo(t *testing.T, writeToDefault func(t *testing.T, repo *fixturegit.Repo)) (*fixturegit.Repo, *store.Config) {
	t.Helper()
	repo, cfg := fixtureStoreNoSpec(t)
	ctx := context.Background()
	// Cut "unrelated" BEFORE writeToDefault lands its commit(s) on main,
	// so switching back to "unrelated" at the end removes the spec from
	// the working tree again (it was never tracked on that branch at
	// all) rather than leaving main's own commit visible there too.
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "unrelated"); err != nil {
		t.Fatalf("CheckoutNewBranch(unrelated): %v", err)
	}
	if err := gitx.CheckoutExisting(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main): %v", err)
	}
	writeToDefault(t, repo)
	if err := gitx.CheckoutExisting(ctx, repo.Dir, "unrelated"); err != nil {
		t.Fatalf("CheckoutExisting(unrelated): %v", err)
	}
	return repo, cfg
}

// TestGather_SpecClassFromDefaultBranchBase_ActiveZone is R-RR3-15's
// final fallback leg (2B-F10): the spec is absent from disk and from
// every ritual branch, but the resolved default branch's own active
// zone carries it (a merged, never-closed spec).
func TestGather_SpecClassFromDefaultBranchBase_ActiveZone(t *testing.T) {
	repo, cfg := buildUnrelatedCheckoutRepo(t, func(t *testing.T, repo *fixturegit.Repo) {
		ctx := context.Background()
		dir := store.ActiveSpecDir(repo.Dir, "checkout")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating %s: %v", dir, err)
		}
		if err := os.WriteFile(store.ActiveSpecPath(repo.Dir, "checkout"), []byte(checkoutSpecMD), 0o644); err != nil {
			t.Fatalf("writing spec.md: %v", err)
		}
		if err := gitx.AddPaths(ctx, repo.Dir, store.SpecDirRelPath(store.ZoneActive, "checkout")); err != nil {
			t.Fatalf("AddPaths: %v", err)
		}
		if _, err := gitx.CreateCommit(ctx, repo.Dir, "land spec/checkout on main"); err != nil {
			t.Fatalf("CreateCommit: %v", err)
		}
	})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	if pathExists(store.ActiveSpecPath(repo.Dir, "checkout")) {
		t.Fatal("test setup bug: spec/checkout is visible on disk on the unrelated branch")
	}

	f := gather(t, cfg, "spec/checkout")
	if f.CurrentBranch != "unrelated" {
		t.Fatalf("CurrentBranch = %q, want unrelated", f.CurrentBranch)
	}
}

// TestGather_SpecClassFromDefaultBranchBase_ArchiveZone is the archive-
// zone half of the same leg — the disclosed addition beyond R-RR3-15's
// literal wording (2B-F10 flagged this untested): a spec already closed
// and merged lives in the default branch's archive zone only.
func TestGather_SpecClassFromDefaultBranchBase_ArchiveZone(t *testing.T) {
	repo, cfg := buildUnrelatedCheckoutRepo(t, func(t *testing.T, repo *fixturegit.Repo) {
		ctx := context.Background()
		dir := store.ArchiveSpecDir(repo.Dir, "checkout")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating %s: %v", dir, err)
		}
		if err := os.WriteFile(store.ArchiveSpecPath(repo.Dir, "checkout"), []byte(checkoutSpecMD), 0o644); err != nil {
			t.Fatalf("writing spec.md: %v", err)
		}
		if err := gitx.AddPaths(ctx, repo.Dir, store.SpecDirRelPath(store.ZoneArchive, "checkout")); err != nil {
			t.Fatalf("AddPaths: %v", err)
		}
		if _, err := gitx.CreateCommit(ctx, repo.Dir, "land spec/checkout's archive copy on main"); err != nil {
			t.Fatalf("CreateCommit: %v", err)
		}
	})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	if pathExists(store.ArchiveSpecPath(repo.Dir, "checkout")) {
		t.Fatal("test setup bug: spec/checkout's archive copy is visible on disk on the unrelated branch")
	}

	f := gather(t, cfg, "spec/checkout")
	if f.CurrentBranch != "unrelated" {
		t.Fatalf("CurrentBranch = %q, want unrelated", f.CurrentBranch)
	}
}

// TestGather_Determinism (2B-F10) proves three independent Gather+Derive
// runs over the same fixture produce byte-identical canonical output and
// a strictly-ascending state order — no map-iteration order reaches
// facts, disclosures, or choice order.
func TestGather_Determinism(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")
	writeStaleWriterLock(t, repo)
	writePreparedJournal(t, repo)
	writeOrphanWorkspaceStaging(t, repo)

	var canon [][]byte
	for i := 0; i < 3; i++ {
		f := gather(t, cfg, "spec/checkout")
		p := Derive(f)
		if err := p.Validate(); err != nil {
			t.Fatalf("run %d: Validate: %v", i, err)
		}
		for j := 1; j < len(p.States); j++ {
			if stateTargetKey(p.States[j-1]) >= stateTargetKey(p.States[j]) {
				t.Fatalf("run %d: states not strictly ascending at %d: %+v", i, j, p.States)
			}
		}
		data, err := Canonical(p)
		if err != nil {
			t.Fatalf("run %d: Canonical: %v", i, err)
		}
		canon = append(canon, data)
	}
	for i := 1; i < len(canon); i++ {
		if !bytesEqual(canon[0], canon[i]) {
			t.Fatalf("run %d differs from run 0:\n%s\nvs\n%s", i, canon[0], canon[i])
		}
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
