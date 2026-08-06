package execworkspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// --- fakeReconciler: hermetic, call-recording, failure-injectable fake for
// the Reconciler port (controller decision AD-4). The real target-specific
// registry-reconciliation primitive is a later lane's concern; these tests
// exercise only the state machine's use of the port. ---

type reconcileCall struct {
	RepoRoot string
	UnitPath string
}

type fakeReconciler struct {
	mu      sync.Mutex
	calls   []reconcileCall
	failNow bool
	failErr error
}

func (f *fakeReconciler) ReconcileUnit(ctx context.Context, repoRoot, unitPath string) error {
	f.mu.Lock()
	f.calls = append(f.calls, reconcileCall{RepoRoot: repoRoot, UnitPath: unitPath})
	fail, failErr := f.failNow, f.failErr
	f.mu.Unlock()
	if fail {
		if failErr != nil {
			return failErr
		}
		return errors.New("fakeReconciler: injected failure")
	}
	// Minimal real reconciliation for test purposes only: clear any stale
	// administrative entry so a subsequent `git worktree add` at unitPath
	// succeeds after this component has directly removed unitPath's
	// directory (step 2/4c). The real per-entry claim/resolve/delete
	// mechanism the spec describes (§Workspace naming, "RECONCILING THE
	// REGISTRY FOR A UNIT") is target-specific and is a later lane's
	// concern (§Implementation seam); this fake uses a blunt
	// `git worktree prune`, safe here only because every test gives this
	// fake its own private, single-unit-at-a-time repoRoot.
	cmd := exec.CommandContext(ctx, "git", "worktree", "prune")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("fakeReconciler: git worktree prune: %w: %s", err, out)
	}
	return nil
}

func (f *fakeReconciler) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeReconciler) callsSnapshot() []reconcileCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]reconcileCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *fakeReconciler) setFail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failNow = true
	f.failErr = err
}

func (f *fakeReconciler) clearFail() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failNow = false
	f.failErr = nil
}

// --- shared fixtures ---

func buildTestRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n", "dir/b.txt": "world\n"}, Message: "layer 1"},
		{Files: map[string]string{"a.txt": "hello again\n"}, Message: "layer 2"},
	})
}

func newTestMaterializer(t *testing.T) (*Materializer, string, *fixturegit.Repo, *fakeReconciler) {
	t.Helper()
	repo := buildTestRepo(t)
	storeRoot := t.TempDir()
	rec := &fakeReconciler{}
	m, err := NewMaterializer(storeRoot, repo.Dir, rec)
	if err != nil {
		t.Fatalf("NewMaterializer: %v", err)
	}
	return m, storeRoot, repo, rec
}

// buildPatchBytes modifies path inside repoDir to newContent, captures the
// resulting unstaged unified diff, restores the original content, and
// returns the patch bytes — mirroring internal/gitx's own test helper, but
// this package must not import a _test.go file from another package.
func buildPatchBytes(t *testing.T, repoDir, path, newContent string) []byte {
	t.Helper()
	full := filepath.Join(repoDir, path)
	original, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("reading %s before patching: %v", path, err)
	}
	if err := os.WriteFile(full, []byte(newContent), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	out, err := exec.Command("git", "-C", repoDir, "diff", "--", path).Output()
	if err != nil {
		t.Fatalf("git diff -- %s: %v", path, err)
	}
	if err := os.WriteFile(full, original, 0o644); err != nil {
		t.Fatalf("restoring %s: %v", path, err)
	}
	if len(out) == 0 {
		t.Fatalf("git diff -- %s produced no output", path)
	}
	return out
}

func lockAbsent(t *testing.T, storeRoot, workspaceID string) {
	t.Helper()
	kind, err := LstatType(LockPath(storeRoot, workspaceID))
	if err != nil {
		t.Fatalf("lstat lock path: %v", err)
	}
	if kind != PathAbsent {
		t.Fatalf("lock path still present (kind=%s) after operation returned; want released", kind)
	}
}

// --- constructor ---

func TestNewMaterializer_RefusesNilReconciler(t *testing.T) {
	if _, err := NewMaterializer(t.TempDir(), t.TempDir(), nil); err == nil {
		t.Fatal("NewMaterializer(nil reconciler): want error, got nil")
	}
}

// --- happy paths, both shapes ---

func TestMaterialize_Happy_ExactSHA(t *testing.T) {
	m, storeRoot, repo, rec := newTestMaterializer(t)
	ctx := context.Background()

	id, err := NewExactIdentity("run-a", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	res, err := m.Materialize(ctx, Request{Identity: id})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if res.Outcome != OutcomeMaterialized {
		t.Fatalf("Outcome = %v, want OutcomeMaterialized", res.Outcome)
	}
	wantID, err := id.WorkspaceID()
	if err != nil {
		t.Fatalf("WorkspaceID: %v", err)
	}
	if res.WorkspaceID != wantID {
		t.Fatalf("WorkspaceID = %q, want %q", res.WorkspaceID, wantID)
	}
	if res.Path != UnitPath(storeRoot, wantID) {
		t.Fatalf("Path = %q, want %q", res.Path, UnitPath(storeRoot, wantID))
	}

	// Detached HEAD at the exact sha, never a branch.
	head, err := gitx.RevParse(ctx, res.Path, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	if head != repo.Head {
		t.Fatalf("worktree HEAD = %q, want exact sha %q", head, repo.Head)
	}
	branch, err := gitx.CurrentBranch(ctx, res.Path)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "" {
		t.Fatalf("worktree is on branch %q, want detached", branch)
	}

	// Witness present and byte-identical to what EncodeSidecar produces.
	data, err := os.ReadFile(RequestPath(storeRoot, wantID))
	if err != nil {
		t.Fatalf("reading witness: %v", err)
	}
	gotID, err := DecodeSidecar(data)
	if err != nil {
		t.Fatalf("DecodeSidecar(witness): %v", err)
	}
	if !gotID.Equal(id) {
		t.Fatalf("witness identity = %s, want %s", gotID, id)
	}

	// Staging is cleaned up (renamed away).
	if kind, _ := LstatType(RequestStagingPath(storeRoot, wantID)); kind != PathAbsent {
		t.Fatalf("staging path present (kind=%s) after successful materialization", kind)
	}

	// Registry reconciliation ran exactly once, for step 2's absent-unit
	// branch, addressed at this unit's own path under the repo root.
	calls := rec.callsSnapshot()
	if len(calls) != 1 {
		t.Fatalf("reconciler called %d times, want 1", len(calls))
	}
	if calls[0].RepoRoot != repo.Dir || calls[0].UnitPath != res.Path {
		t.Fatalf("reconciler call = %+v, want repoRoot=%q unitPath=%q", calls[0], repo.Dir, res.Path)
	}

	lockAbsent(t, storeRoot, wantID)
}

func TestMaterialize_Happy_BasePlusPatch(t *testing.T) {
	m, storeRoot, repo, rec := newTestMaterializer(t)
	ctx := context.Background()

	patch := buildPatchBytes(t, repo.Dir, "a.txt", "patched by materialize\n")
	id, err := NewPatchIdentity("run-b", repo.Head, patch)
	if err != nil {
		t.Fatalf("NewPatchIdentity: %v", err)
	}
	res, err := m.Materialize(ctx, Request{Identity: id, PatchBytes: patch})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if res.Outcome != OutcomeMaterialized {
		t.Fatalf("Outcome = %v, want OutcomeMaterialized", res.Outcome)
	}

	got, err := os.ReadFile(filepath.Join(res.Path, "a.txt"))
	if err != nil {
		t.Fatalf("reading patched file: %v", err)
	}
	if string(got) != "patched by materialize\n" {
		t.Fatalf("a.txt = %q, want patched content", got)
	}

	// Base sha checked out before the patch, never a branch.
	branch, err := gitx.CurrentBranch(ctx, res.Path)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "" {
		t.Fatalf("worktree is on branch %q, want detached", branch)
	}

	if calls := rec.callCount(); calls != 1 {
		t.Fatalf("reconciler called %d times, want 1", calls)
	}
	wantID, _ := id.WorkspaceID()
	lockAbsent(t, storeRoot, wantID)
}

// --- idempotent reuse / crash-after-witness ---

func TestMaterialize_IdempotentReuse(t *testing.T) {
	m, storeRoot, repo, rec := newTestMaterializer(t)
	ctx := context.Background()

	id, err := NewExactIdentity("run-c", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	req := Request{Identity: id}

	first, err := m.Materialize(ctx, req)
	if err != nil {
		t.Fatalf("first Materialize: %v", err)
	}
	if first.Outcome != OutcomeMaterialized {
		t.Fatalf("first Outcome = %v, want OutcomeMaterialized", first.Outcome)
	}
	firstReconcileCalls := rec.callCount()

	// A sentinel inside the worktree survives ONLY if the second call
	// never re-runs `git worktree add` against this directory (which git
	// would refuse against a non-empty existing path anyway) and never
	// removes/rebuilds it.
	sentinel := filepath.Join(first.Path, "sentinel-untouched")
	if err := os.WriteFile(sentinel, []byte("still here\n"), 0o644); err != nil {
		t.Fatalf("writing sentinel: %v", err)
	}

	second, err := m.Materialize(ctx, req)
	if err != nil {
		t.Fatalf("second (idempotent) Materialize: %v", err)
	}
	if second.Outcome != OutcomeReused {
		t.Fatalf("second Outcome = %v, want OutcomeReused", second.Outcome)
	}
	if second.WorkspaceID != first.WorkspaceID || second.Path != first.Path {
		t.Fatalf("reused result = %+v, want same id/path as first %+v", second, first)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel file gone after idempotent reuse: %v", err)
	}
	if got := rec.callCount(); got != firstReconcileCalls {
		t.Fatalf("reconciler called again on idempotent reuse: %d calls, want unchanged %d", got, firstReconcileCalls)
	}

	wantID, _ := id.WorkspaceID()
	lockAbsent(t, storeRoot, wantID)
}

// --- colliding request: same workspace-id (via a RefSlug collision),
// different full identity ---

func TestMaterialize_CollidingIdentity_SameWorkspaceID(t *testing.T) {
	m, storeRoot, repo, _ := newTestMaterializer(t)
	ctx := context.Background()

	// "Foo/Bar" and "foo--bar" both slug to "foo--bar" (RefSlug lowercases
	// and maps "/" to "--"), so they collide on the same <workspace-id>
	// while carrying different full RunID identity.
	id1, err := NewExactIdentity("Foo/Bar", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity(id1): %v", err)
	}
	id2, err := NewExactIdentity("foo--bar", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity(id2): %v", err)
	}
	wid1, _ := id1.WorkspaceID()
	wid2, _ := id2.WorkspaceID()
	if wid1 != wid2 {
		t.Fatalf("test setup: expected colliding workspace ids, got %q vs %q", wid1, wid2)
	}
	if id1.Equal(id2) {
		t.Fatalf("test setup: expected DIFFERENT full identities, got equal")
	}

	if _, err := m.Materialize(ctx, Request{Identity: id1}); err != nil {
		t.Fatalf("first Materialize: %v", err)
	}

	_, err = m.Materialize(ctx, Request{Identity: id2})
	if err == nil {
		t.Fatal("second (colliding) Materialize: want ErrIdentityMismatch, got nil")
	}
	var mismatch *ErrIdentityMismatch
	if !errors.As(err, &mismatch) {
		t.Fatalf("error = %v, want *ErrIdentityMismatch", err)
	}
	if mismatch.WorkspaceID != wid1 {
		t.Fatalf("mismatch.WorkspaceID = %q, want %q", mismatch.WorkspaceID, wid1)
	}
	if !mismatch.Proposed.Equal(id2) || !mismatch.Recorded.Equal(id1) {
		t.Fatalf("mismatch proposed/recorded = %s / %s, want %s / %s", mismatch.Proposed, mismatch.Recorded, id2, id1)
	}

	lockAbsent(t, storeRoot, wid1)
}

// --- crash before witness: dir present, no .request → step 4c rebuilds ---

func TestMaterialize_CrashBeforeWitness_RebuildsViaStep4c(t *testing.T) {
	m, storeRoot, repo, rec := newTestMaterializer(t)
	ctx := context.Background()

	id, err := NewExactIdentity("run-crash", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	workspaceID, err := id.WorkspaceID()
	if err != nil {
		t.Fatalf("WorkspaceID: %v", err)
	}
	unitPath := UnitPath(storeRoot, workspaceID)
	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("MkdirAll execution root: %v", err)
	}
	// Simulate exactly the crash window the spec names: step 5 completed
	// (the worktree exists) but the process died before step 6 ever wrote
	// the completion witness.
	if err := gitx.WorktreeAddDetached(ctx, repo.Dir, unitPath, repo.Head); err != nil {
		t.Fatalf("pre-populating crash residue: %v", err)
	}
	// A leftover marker of a request that's own sidecar was mid-write.
	if err := os.WriteFile(RequestStagingPath(storeRoot, workspaceID), []byte("half-written\n"), 0o644); err != nil {
		t.Fatalf("writing staging residue: %v", err)
	}

	res, err := m.Materialize(ctx, Request{Identity: id})
	if err != nil {
		t.Fatalf("Materialize (should rebuild via 4c): %v", err)
	}
	if res.Outcome != OutcomeMaterialized {
		t.Fatalf("Outcome = %v, want OutcomeMaterialized (rebuilt, never trusted as reuse)", res.Outcome)
	}

	// Staging residue is gone (unlinked as part of 4c).
	if kind, _ := LstatType(RequestStagingPath(storeRoot, workspaceID)); kind != PathAbsent {
		t.Fatalf("staging residue survived 4c rebuild (kind=%s)", kind)
	}
	// A real witness now exists.
	data, err := os.ReadFile(RequestPath(storeRoot, workspaceID))
	if err != nil {
		t.Fatalf("reading witness after rebuild: %v", err)
	}
	gotID, err := DecodeSidecar(data)
	if err != nil {
		t.Fatalf("DecodeSidecar: %v", err)
	}
	if !gotID.Equal(id) {
		t.Fatalf("rebuilt witness identity = %s, want %s", gotID, id)
	}
	// The registry reconciliation ran for this unit under 4c.
	calls := rec.callsSnapshot()
	if len(calls) != 1 {
		t.Fatalf("reconciler called %d times, want 1 (from step 4c)", len(calls))
	}
	if calls[0].UnitPath != unitPath {
		t.Fatalf("reconciler call unit path = %q, want %q", calls[0].UnitPath, unitPath)
	}

	lockAbsent(t, storeRoot, workspaceID)
}

// --- step-5 failure leaves no witness (ordering proof) ---

func TestMaterialize_StepFiveFailure_PatchApplyFails_LeavesNoWitness(t *testing.T) {
	m, storeRoot, repo, _ := newTestMaterializer(t)
	ctx := context.Background()

	// A patch captured against DIFFERENT content than the base sha's own
	// a.txt (layer 1's "hello\n") — applying it against a fresh checkout
	// of repo.Heads[0] fails as a genuine conflict, strictly AFTER
	// WorktreeAddDetached has already succeeded and left a real directory.
	patch := buildPatchBytes(t, repo.Dir, "a.txt", "this diff assumes layer-2 content\n")
	id, err := NewPatchIdentity("run-fail", repo.Heads[0], patch)
	if err != nil {
		t.Fatalf("NewPatchIdentity: %v", err)
	}
	workspaceID, err := id.WorkspaceID()
	if err != nil {
		t.Fatalf("WorkspaceID: %v", err)
	}

	_, err = m.Materialize(ctx, Request{Identity: id, PatchBytes: patch})
	if err == nil {
		t.Fatal("Materialize with a conflicting patch: want error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error = %v, want *OperationalError", err)
	}

	unitPath := UnitPath(storeRoot, workspaceID)
	kind, lerr := LstatType(unitPath)
	if lerr != nil {
		t.Fatalf("lstat unit path after step-5 failure: %v", lerr)
	}
	if kind != PathDir {
		t.Fatalf("unit path kind = %s after step-5 failure, want PathDir (worktree materialized before the patch failed)", kind)
	}
	if kind, _ := LstatType(RequestPath(storeRoot, workspaceID)); kind != PathAbsent {
		t.Fatalf("witness present despite step-5 failure: kind=%s", kind)
	}

	lockAbsent(t, storeRoot, workspaceID)
}

// --- released-terminal ---

func TestMaterialize_ReleasedTerminal(t *testing.T) {
	m, storeRoot, repo, _ := newTestMaterializer(t)
	ctx := context.Background()

	id, err := NewExactIdentity("run-released", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	workspaceID, _ := id.WorkspaceID()
	unitPath := UnitPath(storeRoot, workspaceID)
	if err := os.MkdirAll(unitPath, 0o755); err != nil {
		t.Fatalf("MkdirAll unit path: %v", err)
	}
	if err := os.WriteFile(ReleasedPath(storeRoot, workspaceID), nil, 0o644); err != nil {
		t.Fatalf("writing released marker: %v", err)
	}

	_, err = m.Materialize(ctx, Request{Identity: id})
	if err == nil {
		t.Fatal("Materialize against a released unit: want error, got nil")
	}
	var released *ErrReleasedTerminal
	if !errors.As(err, &released) {
		t.Fatalf("error = %v, want *ErrReleasedTerminal", err)
	}
	if released.WorkspaceID != workspaceID {
		t.Fatalf("released.WorkspaceID = %q, want %q", released.WorkspaceID, workspaceID)
	}

	// Never rebuilt: the directory is exactly as this test left it (no
	// worktree ever cut inside it).
	if _, err := os.Stat(filepath.Join(unitPath, ".git")); err == nil {
		t.Fatal("released-terminal unit was materialized despite the terminal marker")
	}

	lockAbsent(t, storeRoot, workspaceID)
}

// --- malformed .released marker (non-regular) ---

func TestMaterialize_MalformedReleasedMarker_Symlink(t *testing.T) {
	m, storeRoot, repo, _ := newTestMaterializer(t)
	ctx := context.Background()

	id, err := NewExactIdentity("run-malformed-released", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	workspaceID, _ := id.WorkspaceID()
	unitPath := UnitPath(storeRoot, workspaceID)
	if err := os.MkdirAll(unitPath, 0o755); err != nil {
		t.Fatalf("MkdirAll unit path: %v", err)
	}
	if err := os.Symlink("/nowhere", ReleasedPath(storeRoot, workspaceID)); err != nil {
		t.Fatalf("symlinking released marker: %v", err)
	}

	_, err = m.Materialize(ctx, Request{Identity: id})
	if err == nil {
		t.Fatal("Materialize with a symlinked .released marker: want error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error = %v, want *OperationalError (not ErrReleasedTerminal — never treated as released)", err)
	}
	var released *ErrReleasedTerminal
	if errors.As(err, &released) {
		t.Fatal("symlinked .released marker was treated as a genuine release")
	}

	// Kept: the marker is unchanged.
	kind, lerr := LstatType(ReleasedPath(storeRoot, workspaceID))
	if lerr != nil {
		t.Fatalf("lstat released marker: %v", lerr)
	}
	if kind != PathSymlink {
		t.Fatalf("released marker kind = %s after operational error, want unchanged PathSymlink", kind)
	}

	lockAbsent(t, storeRoot, workspaceID)
}

// --- undecodable sidecar: garbage bytes in a regular .request file ---

func TestMaterialize_UndecodableSidecar_Garbage(t *testing.T) {
	m, storeRoot, repo, _ := newTestMaterializer(t)
	ctx := context.Background()

	id, err := NewExactIdentity("run-garbage-sidecar", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	workspaceID, _ := id.WorkspaceID()
	unitPath := UnitPath(storeRoot, workspaceID)
	if err := os.MkdirAll(unitPath, 0o755); err != nil {
		t.Fatalf("MkdirAll unit path: %v", err)
	}
	if err := os.WriteFile(RequestPath(storeRoot, workspaceID), []byte("not json at all"), 0o644); err != nil {
		t.Fatalf("writing garbage sidecar: %v", err)
	}

	_, err = m.Materialize(ctx, Request{Identity: id})
	if err == nil {
		t.Fatal("Materialize with an undecodable sidecar: want error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error = %v, want *OperationalError", err)
	}

	// Kept: the garbage sidecar is untouched, never silently reused or
	// silently deleted by this path.
	data, rerr := os.ReadFile(RequestPath(storeRoot, workspaceID))
	if rerr != nil {
		t.Fatalf("reading sidecar after operational error: %v", rerr)
	}
	if string(data) != "not json at all" {
		t.Fatalf("sidecar mutated by a failed decode attempt: %q", data)
	}

	lockAbsent(t, storeRoot, workspaceID)
}

// --- undecodable sidecar: non-regular object at .request ---

func TestMaterialize_UndecodableSidecar_NonRegular(t *testing.T) {
	m, storeRoot, repo, _ := newTestMaterializer(t)
	ctx := context.Background()

	id, err := NewExactIdentity("run-symlink-sidecar", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	workspaceID, _ := id.WorkspaceID()
	unitPath := UnitPath(storeRoot, workspaceID)
	if err := os.MkdirAll(unitPath, 0o755); err != nil {
		t.Fatalf("MkdirAll unit path: %v", err)
	}
	if err := os.Symlink("/nowhere", RequestPath(storeRoot, workspaceID)); err != nil {
		t.Fatalf("symlinking sidecar: %v", err)
	}

	_, err = m.Materialize(ctx, Request{Identity: id})
	if err == nil {
		t.Fatal("Materialize with a non-regular .request object: want error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error = %v, want *OperationalError", err)
	}
	kind, lerr := LstatType(RequestPath(storeRoot, workspaceID))
	if lerr != nil {
		t.Fatalf("lstat sidecar: %v", lerr)
	}
	if kind != PathSymlink {
		t.Fatalf("sidecar kind = %s after operational error, want unchanged PathSymlink", kind)
	}

	lockAbsent(t, storeRoot, workspaceID)
}

// --- orphaned siblings, no unit directory (step 2) ---

func TestMaterialize_OrphanedSiblings_NoDir(t *testing.T) {
	m, storeRoot, repo, rec := newTestMaterializer(t)
	ctx := context.Background()

	id, err := NewExactIdentity("run-orphans", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	workspaceID, _ := id.WorkspaceID()
	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("MkdirAll execution root: %v", err)
	}
	for _, p := range []string{
		RequestPath(storeRoot, workspaceID),
		RequestStagingPath(storeRoot, workspaceID),
		ReleasedPath(storeRoot, workspaceID),
	} {
		if err := os.WriteFile(p, []byte("orphaned residue"), 0o644); err != nil {
			t.Fatalf("writing orphan %s: %v", p, err)
		}
	}

	res, err := m.Materialize(ctx, Request{Identity: id})
	if err != nil {
		t.Fatalf("Materialize (should clean orphans and proceed fresh): %v", err)
	}
	if res.Outcome != OutcomeMaterialized {
		t.Fatalf("Outcome = %v, want OutcomeMaterialized", res.Outcome)
	}
	if len(res.Disclosures) < 3 {
		t.Fatalf("Disclosures = %v, want at least 3 lines (one per deleted orphan)", res.Disclosures)
	}
	if calls := rec.callCount(); calls != 1 {
		t.Fatalf("reconciler called %d times, want 1", calls)
	}

	// .released is gone and never resurrected (nothing in a fresh
	// materialization creates it).
	if kind, _ := LstatType(ReleasedPath(storeRoot, workspaceID)); kind != PathAbsent {
		t.Fatalf("released marker survived step 2 + fresh materialization: kind=%s", kind)
	}
	// .request now holds the NEW canonical witness, not the old residue.
	data, err := os.ReadFile(RequestPath(storeRoot, workspaceID))
	if err != nil {
		t.Fatalf("reading fresh witness: %v", err)
	}
	if string(data) == "orphaned residue" {
		t.Fatal(".request still holds the orphaned residue bytes")
	}
	gotID, err := DecodeSidecar(data)
	if err != nil {
		t.Fatalf("DecodeSidecar: %v", err)
	}
	if !gotID.Equal(id) {
		t.Fatalf("fresh witness identity = %s, want %s", gotID, id)
	}

	lockAbsent(t, storeRoot, workspaceID)
}

// --- lstat failure at the unit path: never read as absence ---

func TestMaterialize_LstatFailureAtUnitPath_NotAbsence(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission-based lstat failure cannot be induced")
	}
	m, storeRoot, repo, rec := newTestMaterializer(t)
	ctx := context.Background()

	execRoot := ExecutionRoot(storeRoot)
	if err := os.MkdirAll(execRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll execution root: %v", err)
	}
	if err := os.Chmod(execRoot, 0o000); err != nil {
		t.Fatalf("chmod execution root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(execRoot, 0o755) })

	id, err := NewExactIdentity("run-lstat-fail", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	_, err = m.Materialize(ctx, Request{Identity: id})
	if err == nil {
		t.Fatal("Materialize with an unreadable execution root: want error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error = %v, want *OperationalError", err)
	}
	if calls := rec.callCount(); calls != 0 {
		t.Fatalf("reconciler called %d times, want 0 (an lstat failure must never route into step 2)", calls)
	}
}

// --- lock held by a live holder ---

func TestMaterialize_LockHeldByLiveHolder(t *testing.T) {
	m, storeRoot, repo, _ := newTestMaterializer(t)
	ctx := context.Background()

	id, err := NewExactIdentity("run-locked", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	workspaceID, _ := id.WorkspaceID()
	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("MkdirAll execution root: %v", err)
	}
	lockPath := LockPath(storeRoot, workspaceID)
	f, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("pre-acquiring lock: %v", err)
	}
	defer func() { _ = filelock.Release(f, lockPath) }()

	_, err = m.Materialize(ctx, Request{Identity: id})
	if err == nil {
		t.Fatal("Materialize against a live-held lock: want error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error = %v, want *OperationalError", err)
	}
	var held *filelock.ErrHeld
	if !errors.As(err, &held) {
		t.Fatalf("error = %v, want to surface filelock.ErrHeld", err)
	}

	// Still held by the pre-acquirer (never stolen out from under it).
	kind, lerr := LstatType(lockPath)
	if lerr != nil {
		t.Fatalf("lstat lock path: %v", lerr)
	}
	if kind != PathRegular {
		t.Fatalf("lock path kind = %s, want the pre-acquirer's lock still present", kind)
	}
}

// --- reconciler failure in step 2, then self-heals ---

func TestMaterialize_ReconcilerFailure_Step2_SelfHeals(t *testing.T) {
	m, storeRoot, repo, rec := newTestMaterializer(t)
	ctx := context.Background()

	id, err := NewExactIdentity("run-step2-fail", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	workspaceID, _ := id.WorkspaceID()

	rec.setFail(errors.New("boom: registry unreachable"))
	_, err = m.Materialize(ctx, Request{Identity: id})
	if err == nil {
		t.Fatal("Materialize with a failing step-2 reconciler: want error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error = %v, want *OperationalError", err)
	}
	// Untouched beyond what preceded the failure: no worktree, no witness.
	if kind, _ := LstatType(UnitPath(storeRoot, workspaceID)); kind != PathAbsent {
		t.Fatalf("unit path present after a step-2 reconciler failure, want absent (never reached step 5)")
	}
	lockAbsent(t, storeRoot, workspaceID)
	if calls := rec.callCount(); calls != 1 {
		t.Fatalf("reconciler called %d times, want 1", calls)
	}

	rec.clearFail()
	res, err := m.Materialize(ctx, Request{Identity: id})
	if err != nil {
		t.Fatalf("retry after clearing the injected failure: %v", err)
	}
	if res.Outcome != OutcomeMaterialized {
		t.Fatalf("retry Outcome = %v, want OutcomeMaterialized", res.Outcome)
	}
	if calls := rec.callCount(); calls != 2 {
		t.Fatalf("reconciler called %d times total, want 2 (one failed, one succeeded)", calls)
	}
	lockAbsent(t, storeRoot, workspaceID)
}

// --- reconciler failure in step 4c, then self-heals (lands back at step 2) ---

func TestMaterialize_ReconcilerFailure_Step4c_SelfHeals(t *testing.T) {
	m, storeRoot, repo, rec := newTestMaterializer(t)
	ctx := context.Background()

	id, err := NewExactIdentity("run-step4c-fail", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	workspaceID, _ := id.WorkspaceID()
	unitPath := UnitPath(storeRoot, workspaceID)
	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("MkdirAll execution root: %v", err)
	}
	if err := gitx.WorktreeAddDetached(ctx, repo.Dir, unitPath, repo.Head); err != nil {
		t.Fatalf("pre-populating crash residue: %v", err)
	}

	rec.setFail(errors.New("boom: registry unreachable"))
	_, err = m.Materialize(ctx, Request{Identity: id})
	if err == nil {
		t.Fatal("Materialize with a failing step-4c reconciler: want error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error = %v, want *OperationalError", err)
	}
	// Per spec: 4c's removal precedes its reconciliation, so a failure here
	// leaves no unit path but a (simulated) surviving registration —
	// exactly step 2's absent-unit state for the retry to land in.
	if kind, _ := LstatType(unitPath); kind != PathAbsent {
		t.Fatalf("unit directory survived a step-4c reconciler failure, want removed before reconcile ran")
	}
	lockAbsent(t, storeRoot, workspaceID)
	if calls := rec.callCount(); calls != 1 {
		t.Fatalf("reconciler called %d times, want 1", calls)
	}

	rec.clearFail()
	res, err := m.Materialize(ctx, Request{Identity: id})
	if err != nil {
		t.Fatalf("retry after clearing the injected failure: %v", err)
	}
	if res.Outcome != OutcomeMaterialized {
		t.Fatalf("retry Outcome = %v, want OutcomeMaterialized", res.Outcome)
	}
	if calls := rec.callCount(); calls != 2 {
		t.Fatalf("reconciler called %d times total, want 2 (4c failure, then step-2 success)", calls)
	}
	lockAbsent(t, storeRoot, workspaceID)
}

// --- no deletion outside the exact unit ---

func snapshotOtherEntries(t *testing.T, storeRoot, excludeWorkspaceID string) map[string]string {
	t.Helper()
	root := ExecutionRoot(storeRoot)
	snap := map[string]string{}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return snap
		}
		t.Fatalf("ReadDir execution root: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if ce, ok := ClassifyEntry(name); ok && ce.WorkspaceID == excludeWorkspaceID {
			continue
		}
		full := filepath.Join(root, name)
		info, lerr := os.Lstat(full)
		if lerr != nil {
			t.Fatalf("lstat %s: %v", full, lerr)
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, rerr := os.Readlink(full)
			if rerr != nil {
				t.Fatalf("readlink %s: %v", full, rerr)
			}
			snap[name] = "symlink:" + target
		case info.IsDir():
			h := sha256.New()
			_ = filepath.Walk(full, func(p string, fi os.FileInfo, werr error) error {
				if werr != nil {
					return werr
				}
				rel, _ := filepath.Rel(full, p)
				h.Write([]byte(rel))
				if !fi.IsDir() {
					data, rerr := os.ReadFile(p)
					if rerr == nil {
						h.Write(data)
					}
				}
				return nil
			})
			snap[name] = "dir:" + hex.EncodeToString(h.Sum(nil))
		default:
			data, rerr := os.ReadFile(full)
			if rerr != nil {
				t.Fatalf("reading %s: %v", full, rerr)
			}
			snap[name] = "file:" + string(data)
		}
	}
	return snap
}

func TestMaterialize_NoDeletionOutsideExactUnit(t *testing.T) {
	m, storeRoot, repo, _ := newTestMaterializer(t)
	ctx := context.Background()

	// A sibling unit, fully materialized (dir + witness) — must survive
	// every destructive path run against the OTHER unit below.
	siblingID, err := NewExactIdentity("sibling-run", repo.Heads[0])
	if err != nil {
		t.Fatalf("NewExactIdentity(sibling): %v", err)
	}
	if _, err := m.Materialize(ctx, Request{Identity: siblingID}); err != nil {
		t.Fatalf("materializing sibling unit: %v", err)
	}
	siblingWorkspaceID, _ := siblingID.WorkspaceID()

	// Grammar-external entries at the execution root.
	if err := os.WriteFile(filepath.Join(ExecutionRoot(storeRoot), ".DS_Store"), []byte("finder noise"), 0o644); err != nil {
		t.Fatalf("writing grammar-external entry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ExecutionRoot(storeRoot), "README.txt"), []byte("human note"), 0o644); err != nil {
		t.Fatalf("writing grammar-external entry: %v", err)
	}

	before := snapshotOtherEntries(t, storeRoot, "")

	// Path 1: step 2's orphan-sibling deletion on a DIFFERENT unit.
	otherID, err := NewExactIdentity("other-run", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity(other): %v", err)
	}
	otherWorkspaceID, _ := otherID.WorkspaceID()
	if err := os.WriteFile(RequestStagingPath(storeRoot, otherWorkspaceID), []byte("orphan"), 0o644); err != nil {
		t.Fatalf("writing orphan staging: %v", err)
	}
	if _, err := m.Materialize(ctx, Request{Identity: otherID}); err != nil {
		t.Fatalf("materializing other unit (step 2 path): %v", err)
	}

	afterStep2 := snapshotOtherEntries(t, storeRoot, otherWorkspaceID)
	// Also exclude the sibling unit we deliberately created, but that one
	// must be BYTE IDENTICAL between before/after too — check separately.
	sib := before[siblingWorkspaceID]
	if sib == "" {
		t.Fatalf("test setup: sibling unit %q missing from before-snapshot", siblingWorkspaceID)
	}
	if got := afterStep2[siblingWorkspaceID]; got != sib {
		t.Fatalf("sibling unit changed after step-2 path on a different unit: before=%q after=%q", sib, got)
	}
	if got := afterStep2[".DS_Store"]; got != before[".DS_Store"] {
		t.Fatalf("grammar-external .DS_Store changed after step-2 path: before=%q after=%q", before[".DS_Store"], got)
	}
	if got := afterStep2["README.txt"]; got != before["README.txt"] {
		t.Fatalf("grammar-external README.txt changed after step-2 path: before=%q after=%q", before["README.txt"], got)
	}

	// Path 2: step 4c's residue removal on yet another unit.
	fourCID, err := NewExactIdentity("fourc-run", repo.Heads[0])
	if err != nil {
		t.Fatalf("NewExactIdentity(fourc): %v", err)
	}
	fourCWorkspaceID, _ := fourCID.WorkspaceID()
	fourCUnitPath := UnitPath(storeRoot, fourCWorkspaceID)
	if err := gitx.WorktreeAddDetached(ctx, repo.Dir, fourCUnitPath, repo.Heads[0]); err != nil {
		t.Fatalf("pre-populating 4c residue: %v", err)
	}
	before2 := snapshotOtherEntries(t, storeRoot, fourCWorkspaceID)
	if _, err := m.Materialize(ctx, Request{Identity: fourCID}); err != nil {
		t.Fatalf("materializing fourc unit (step 4c path): %v", err)
	}
	afterStep4c := snapshotOtherEntries(t, storeRoot, fourCWorkspaceID)
	for k, v := range before2 {
		if got := afterStep4c[k]; got != v {
			t.Fatalf("entry %q changed after step-4c path on a different unit: before=%q after=%q", k, v, got)
		}
	}
}

// --- request validation: patch-shape bytes must match the digested identity ---

func TestMaterialize_Negative_PatchBytesMismatch(t *testing.T) {
	m, _, repo, _ := newTestMaterializer(t)
	ctx := context.Background()

	patch := buildPatchBytes(t, repo.Dir, "a.txt", "one version\n")
	id, err := NewPatchIdentity("run-mismatch", repo.Head, patch)
	if err != nil {
		t.Fatalf("NewPatchIdentity: %v", err)
	}
	otherPatch := buildPatchBytes(t, repo.Dir, "a.txt", "a completely different version\n")

	_, err = m.Materialize(ctx, Request{Identity: id, PatchBytes: otherPatch})
	if err == nil {
		t.Fatal("Materialize with patch bytes that don't match the digested identity: want error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error = %v, want *OperationalError", err)
	}
}

func TestMaterialize_Negative_ExactSHAWithPatchBytes(t *testing.T) {
	m, _, repo, _ := newTestMaterializer(t)
	ctx := context.Background()

	id, err := NewExactIdentity("run-extra-bytes", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	_, err = m.Materialize(ctx, Request{Identity: id, PatchBytes: []byte("stray bytes\n")})
	if err == nil {
		t.Fatal("Materialize(exact-sha with stray patch bytes): want error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error = %v, want *OperationalError", err)
	}
}

func TestMaterialize_Negative_PatchShapeMissingBytes(t *testing.T) {
	m, _, repo, _ := newTestMaterializer(t)
	ctx := context.Background()

	patch := buildPatchBytes(t, repo.Dir, "a.txt", "some version\n")
	id, err := NewPatchIdentity("run-missing-bytes", repo.Head, patch)
	if err != nil {
		t.Fatalf("NewPatchIdentity: %v", err)
	}
	_, err = m.Materialize(ctx, Request{Identity: id})
	if err == nil {
		t.Fatal("Materialize(patch-shape with no PatchBytes): want error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error = %v, want *OperationalError", err)
	}
}

// --- writeCompletionWitness: white-box unit tests for step 6's staging
// discipline (O_TRUNC not O_EXCL; non-regular staging object refused). A
// full end-to-end Materialize() path cannot reach step 6 with pre-existing
// staging residue in place, because steps 2 and 4c already dispose of any
// sibling residue before step 5/6 ever run under the same lock — so this
// discipline is exercised directly against the unit it belongs to. ---

func TestWriteCompletionWitness_StagingAbsent(t *testing.T) {
	storeRoot := t.TempDir()
	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	id, err := NewExactIdentity("run-witness", "0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	workspaceID, _ := id.WorkspaceID()

	if err := writeCompletionWitness(storeRoot, workspaceID, id); err != nil {
		t.Fatalf("writeCompletionWitness: %v", err)
	}
	data, err := os.ReadFile(RequestPath(storeRoot, workspaceID))
	if err != nil {
		t.Fatalf("reading witness: %v", err)
	}
	got, err := DecodeSidecar(data)
	if err != nil {
		t.Fatalf("DecodeSidecar: %v", err)
	}
	if !got.Equal(id) {
		t.Fatalf("witness = %s, want %s", got, id)
	}
	if kind, _ := LstatType(RequestStagingPath(storeRoot, workspaceID)); kind != PathAbsent {
		t.Fatalf("staging path left behind: kind=%s", kind)
	}
}

func TestWriteCompletionWitness_StagingStaleRegular_TruncatedAndOverwritten(t *testing.T) {
	storeRoot := t.TempDir()
	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	id, err := NewExactIdentity("run-witness-stale", "0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	workspaceID, _ := id.WorkspaceID()
	stagingPath := RequestStagingPath(storeRoot, workspaceID)
	if err := os.WriteFile(stagingPath, []byte("a much longer stale staging body than the real witness"), 0o644); err != nil {
		t.Fatalf("writing stale staging: %v", err)
	}

	if err := writeCompletionWitness(storeRoot, workspaceID, id); err != nil {
		t.Fatalf("writeCompletionWitness: %v", err)
	}
	data, err := os.ReadFile(RequestPath(storeRoot, workspaceID))
	if err != nil {
		t.Fatalf("reading witness: %v", err)
	}
	got, err := DecodeSidecar(data)
	if err != nil {
		t.Fatalf("DecodeSidecar: %v", err)
	}
	if !got.Equal(id) {
		t.Fatalf("witness = %s, want %s (stale staging body must be fully truncated, not appended to)", got, id)
	}
}

func TestWriteCompletionWitness_StagingSymlink_OperationalError(t *testing.T) {
	storeRoot := t.TempDir()
	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	id, err := NewExactIdentity("run-witness-symlink", "0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	workspaceID, _ := id.WorkspaceID()
	stagingPath := RequestStagingPath(storeRoot, workspaceID)
	if err := os.Symlink("/nowhere", stagingPath); err != nil {
		t.Fatalf("symlinking staging path: %v", err)
	}

	err = writeCompletionWitness(storeRoot, workspaceID, id)
	if err == nil {
		t.Fatal("writeCompletionWitness with a symlinked staging path: want error, got nil")
	}
	// Never followed, never written through.
	kind, lerr := LstatType(stagingPath)
	if lerr != nil {
		t.Fatalf("lstat staging path: %v", lerr)
	}
	if kind != PathSymlink {
		t.Fatalf("staging path kind = %s after refusal, want unchanged PathSymlink", kind)
	}
	if kind, _ := LstatType(RequestPath(storeRoot, workspaceID)); kind != PathAbsent {
		t.Fatalf("request path present despite staging refusal: kind=%s", kind)
	}
}

// --- Outcome/OperationalError diagnostics ---

func TestOutcome_String(t *testing.T) {
	cases := map[Outcome]string{
		OutcomeMaterialized: "materialized",
		OutcomeReused:       "reused",
	}
	for outcome, want := range cases {
		if got := outcome.String(); got != want {
			t.Fatalf("Outcome(%d).String() = %q, want %q", outcome, got, want)
		}
	}
}
