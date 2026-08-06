package execworkspace

// GitReconciler implements the Reconciler port (materialize.go) against the
// real git worktree administrative layout: spec/execution-workspace
// §Workspace naming's "RECONCILING THE REGISTRY FOR A UNIT" algorithm,
// steps 1 through 5, plus its POSTCONDITION re-enumeration and its
// DISCLOSED DEPENDENCY on git's own administrative layout — ledger SI-16,
// SI-17. Step numbers in the comments below cite that section's numbered
// list verbatim.
//
// COMPONENT PROTOCOL: every ReconcileUnit call must run with the unit's
// data/execution/<workspace-id>.lock ALREADY HELD by the caller
// (materialization's steps 2 and 4c already do this — materialize.go —
// and the gc rank-0 mutator must too, per spec §Safe cleanup). ReconcileUnit
// itself deliberately does NOT acquire that lock: the spec's safety
// grounding depends on the lock serializing every component operation
// against this unit, and acquiring it here a second time (or acquiring it
// for the first time inside a caller that forgot to) would either deadlock
// against a reentrant caller or silently break that serialization.
import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
)

// Administrative entry file names inside $GIT_COMMON_DIR/worktrees/<id>/
// (git's own layout, versioned behavior — spec's DISCLOSED DEPENDENCY
// paragraph).
const (
	adminGitdirName            = "gitdir"
	adminGitdirReconcilingName = "gitdir.reconciling"
	adminLockedName            = "locked"
)

// GitReconciler is the production Reconciler. Construct with
// NewGitReconciler; the zero value *GitReconciler{} is also directly usable
// (CLAUDE.md: "design zero values to be useful") — NewGitReconciler exists
// for symmetry with NewMaterializer and as the one documented construction
// site.
type GitReconciler struct {
	// Disclose, if non-nil, receives one diagnostic line per call where the
	// read-only gitx.WorktreeList corroboration (spec §Reused primitives)
	// disagrees with the direct administrative-directory enumeration this
	// type performs. WorktreeList is called exactly once per ReconcileUnit,
	// read-only, and is NEVER the source of scan or postcondition truth —
	// only a corroborating, disclosed cross-check ("it omits entries with
	// missing/unreadable gitdir and reports the main worktree, which has no
	// administrative entry"). Nil is a valid, no-op default: corroboration
	// still runs (the git call still happens, satisfying "call it once"),
	// its result is simply not reported anywhere.
	Disclose func(string)

	// afterClaimForTests is a TEST-ONLY seam, defaulted to nil (no-op) and
	// never set by production code. It runs immediately after step 3's
	// atomic claim and before step 4's re-verify, and serves two adversarial
	// tests:
	//
	//   - planting git's own lock marker in the exact instant step 4 exists
	//     to catch ("RE-VERIFY under the claim ... catching one that landed
	//     in the instant between step 3's check and its rename") — return
	//     false to continue into the real re-verify/RESTORE/delete path;
	//   - simulating a crash between claim and deletion: returning true
	//     stops THIS entry's processing right here, with NO error raised
	//     for it (a real crash raises no error either — the process simply
	//     stops existing) and reconcilePass proceeds as if this entry were
	//     done. The claimed-but-undeleted aside record is then discovered
	//     by ReconcileUnit's own POSTCONDITION re-enumeration exactly as it
	//     would after a genuine crash, producing the real, disclosed
	//     postcondition-failure error — never a false success and never a
	//     fabricated test-only error type.
	//
	// No external timing can otherwise land a marker in that window from
	// outside this process, and no external mechanism can otherwise crash
	// this process at that exact instant, so this hook is the only way to
	// exercise either path.
	afterClaimForTests func(entryDir string) (simulateCrash bool)
}

// NewGitReconciler builds a GitReconciler with no disclosure sink. Set
// Disclose on the returned value to receive corroboration-disagreement
// lines.
func NewGitReconciler() *GitReconciler {
	return &GitReconciler{}
}

// ErrWorktreeLocked is step 3/4's typed refusal: an administrative entry
// resolving to the reconciled unit path carries git's own lock marker
// (worktrees/<id>/locked), so this component refuses to touch it — spec
// §Workspace naming's safety grounding, honoring `git worktree lock`'s
// documented protection. Error() names the human remedy verbatim, as the
// spec's tested sequence rather than alternatives, unlock always first.
type ErrWorktreeLocked struct {
	// EntryDir is the administrative entry's own directory
	// ($GIT_COMMON_DIR/worktrees/<id>), not the worktree path itself — the
	// message below names the worktree path, which is what a human runs
	// `git worktree unlock` against.
	EntryDir     string
	WorktreePath string
}

func (e *ErrWorktreeLocked) Error() string {
	return fmt.Sprintf(
		"execworkspace: reconcile: administrative entry %s is locked by git (worktrees/<id>/locked present) and resolves to %s: refusing to touch it; remedy: `git worktree unlock %s` followed by retrying this operation, or, for git-only cleanup, `git worktree unlock %s` followed by `git worktree prune --expire=now` (unlock always first — prune skips a still-locked entry whatever its expiry, and `--expire` without a value is not a valid invocation)",
		e.EntryDir, e.WorktreePath, e.WorktreePath, e.WorktreePath,
	)
}

func newLockedRefusalError(entryDir, worktreePath string) error {
	return &ErrWorktreeLocked{EntryDir: entryDir, WorktreePath: worktreePath}
}

// ReconcileUnit implements the Reconciler port end to end for exactly one
// execution-workspace unit.
func (r *GitReconciler) ReconcileUnit(ctx context.Context, repoRoot, unitPath string) error {
	canonicalUnit := canonicalPath(unitPath)

	adminDir, err := r.resolveAdminDir(ctx, repoRoot)
	if err != nil {
		return fmt.Errorf("execworkspace: reconcile: resolve admin dir: %w", err)
	}

	// Corroboration (read-only, disclosed only, never authoritative): one
	// gitx.WorktreeList call, compared against the direct enumeration
	// BEFORE any mutation runs, so a disagreement it reveals describes the
	// registrations this reconciliation is about to act on.
	r.corroborate(ctx, repoRoot, adminDir, unitPath, canonicalUnit)

	// Steps 1-5.
	if err := r.reconcilePass(ctx, adminDir, canonicalUnit); err != nil {
		return err
	}

	// POSTCONDITION: re-enumerate through the SAME parent-of-gitdir,
	// canonicalized resolution, aside records included — never a command's
	// exit status.
	survivors, err := scanAdminDir(adminDir, canonicalUnit)
	if err != nil {
		return fmt.Errorf("execworkspace: reconcile: postcondition re-enumeration: %w", err)
	}
	if len(survivors) > 0 {
		return fmt.Errorf("execworkspace: reconcile: postcondition failed for %s: %d administrative entr(ies) still resolve to it: %v", unitPath, len(survivors), survivors)
	}
	return nil
}

// resolveAdminDir resolves $GIT_COMMON_DIR/worktrees/ for repoRoot (step 1's
// target), making a relative git-common-dir result absolute against
// repoRoot.
func (r *GitReconciler) resolveAdminDir(ctx context.Context, repoRoot string) (string, error) {
	commonDir, err := gitx.CommonDir(ctx, repoRoot)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(repoRoot, commonDir)
	}
	return filepath.Join(commonDir, "worktrees"), nil
}

// reconcilePass implements steps 1 through 5: enumerate adminDir directly,
// resolve each entry, and claim+delete every entry resolving to
// canonicalUnit. A missing adminDir means nothing is registered at all —
// the postcondition trivially holds, so this returns nil rather than an
// error.
func (r *GitReconciler) reconcilePass(ctx context.Context, adminDir, canonicalUnit string) error {
	entries, err := os.ReadDir(adminDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // step 1: "a missing worktrees/ dir = nothing registered = postcondition trivially holds"
		}
		return fmt.Errorf("enumerate admin dir %s: %w", adminDir, err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			// Administrative entries are always directories; anything else
			// under worktrees/ is not this algorithm's concern (git never
			// puts a non-directory entry there in normal operation, and
			// this loop only ever acts on entries it can positively resolve
			// to canonicalUnit below).
			continue
		}
		entryDir := filepath.Join(adminDir, e.Name())
		resolved, viaAside, ok := resolveAdminEntry(entryDir)
		if !ok || resolved != canonicalUnit {
			// step 2's "an entry that CANNOT be resolved ... is NOT this
			// unit's — leave it untouched", and every entry resolving
			// elsewhere: also untouched.
			continue
		}
		if err := r.reconcileEntry(entryDir, canonicalUnit, viaAside); err != nil {
			return err
		}
	}
	return nil
}

// reconcileEntry implements steps 3 through 5 for one administrative entry
// already proven (by the caller) to resolve to canonicalUnit.
// alreadyClaimed is true when resolution went through the
// gitdir.reconciling aside record (a crashed claim being re-claimed: "the
// rename source is already gone but the aside exists — a claim already
// held is simply held; proceed").
func (r *GitReconciler) reconcileEntry(entryDir, canonicalUnit string, alreadyClaimed bool) error {
	gitdirPath := filepath.Join(entryDir, adminGitdirName)
	asidePath := filepath.Join(entryDir, adminGitdirReconcilingName)
	lockedPath := filepath.Join(entryDir, adminLockedName)

	if !alreadyClaimed {
		// Step 3: if git's lock marker is present, REFUSE without touching
		// the entry.
		locked, err := lockedMarkerPresent(lockedPath)
		if err != nil {
			return fmt.Errorf("execworkspace: reconcile: check lock marker %s: %w", lockedPath, err)
		}
		if locked {
			return newLockedRefusalError(entryDir, canonicalUnit)
		}
		// Otherwise CLAIM the entry: ONE ATOMIC RENAME of gitdir to
		// gitdir.reconciling.
		if err := os.Rename(gitdirPath, asidePath); err != nil {
			return fmt.Errorf("execworkspace: reconcile: claim entry %s: %w", entryDir, err)
		}
	}

	if r.afterClaimForTests != nil {
		if simulateCrash := r.afterClaimForTests(entryDir); simulateCrash {
			return nil
		}
	}

	// Step 4: RE-VERIFY under the claim — re-read the aside record (the
	// same bytes, renamed) and confirm it still resolves to this unit's
	// path, and re-check the lock marker.
	data, rerr := os.ReadFile(asidePath)
	if rerr != nil {
		return fmt.Errorf("execworkspace: reconcile: re-verify claim %s: aside record unreadable: %w", entryDir, rerr)
	}
	wt, ok := parseGitdirRecord(data)
	if !ok || canonicalPath(wt) != canonicalUnit {
		return fmt.Errorf("execworkspace: reconcile: re-verify claim %s: aside record no longer resolves to %s", entryDir, canonicalUnit)
	}
	locked, err := lockedMarkerPresent(lockedPath)
	if err != nil {
		return fmt.Errorf("execworkspace: reconcile: re-verify lock marker %s: %w", lockedPath, err)
	}
	if locked {
		// A marker landed in the window between step 3's check and its
		// rename: RESTORE — rename the record back, byte-identical — and
		// REFUSE, disclosed.
		if rerr := os.Rename(asidePath, gitdirPath); rerr != nil {
			return fmt.Errorf("execworkspace: reconcile: restore claimed entry %s after late lock marker: %w", entryDir, rerr)
		}
		return newLockedRefusalError(entryDir, canonicalUnit)
	}

	// Step 5: DELETE the claimed entry's whole administrative directory —
	// the ONE deletion site this component's structural NO-REPO-WIDE-
	// MUTATION guarantee rests on, scoped to a single entry PROVEN (above)
	// and held CLAIMED (this function, or a prior crashed invocation).
	if err := os.RemoveAll(entryDir); err != nil {
		return fmt.Errorf("execworkspace: reconcile: delete claimed entry %s: %w", entryDir, err)
	}
	return nil
}

// scanAdminDir enumerates adminDir directly and returns the administrative
// entry NAMES (not full paths) whose resolved worktree path equals
// canonicalUnit — used both by reconcilePass's own scope-check (indirectly,
// via resolveAdminEntry) and by the POSTCONDITION re-enumeration. A missing
// adminDir is not an error (nothing registered).
func scanAdminDir(adminDir, canonicalUnit string) ([]string, error) {
	entries, err := os.ReadDir(adminDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var matches []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		resolved, _, ok := resolveAdminEntry(filepath.Join(adminDir, e.Name()))
		if !ok {
			continue
		}
		if resolved == canonicalUnit {
			matches = append(matches, e.Name())
		}
	}
	return matches, nil
}

// resolveAdminEntry implements step 2's resolution for one administrative
// entry directory: read its own gitdir record (the entry's worktree path is
// that record's PARENT — the record names "<worktree>/.git"), falling back
// to the gitdir.reconciling aside record when gitdir is absent (a crashed
// claim). Returns (canonicalized worktree path, resolved via aside, true)
// on success; ("", false, false) when neither record can be resolved — the
// entry is not this component's to touch.
func resolveAdminEntry(entryDir string) (resolvedCanonicalPath string, viaAside bool, ok bool) {
	if data, err := os.ReadFile(filepath.Join(entryDir, adminGitdirName)); err == nil {
		if wt, parsed := parseGitdirRecord(data); parsed {
			return canonicalPath(wt), false, true
		}
		return "", false, false
	}
	if data, err := os.ReadFile(filepath.Join(entryDir, adminGitdirReconcilingName)); err == nil {
		if wt, parsed := parseGitdirRecord(data); parsed {
			return canonicalPath(wt), true, true
		}
		return "", true, false
	}
	return "", false, false
}

// parseGitdirRecord parses a gitdir (or gitdir.reconciling) record's raw
// bytes into the worktree path it names — the record's PARENT directory,
// since the record itself names "<worktree>/.git". Git always writes an
// ABSOLUTE path here; requiring that is a deliberate extra safety margin
// (never present in the spec's own text, but never contradicting it
// either) against garbage bytes parsing into some accidental relative-path
// collision via canonicalPath's Clean-against-cwd fallback. Returns
// ("", false) for empty content or a non-absolute record — "broken gitdir
// record (garbage bytes resolving nowhere)" in the adversarial test
// vocabulary — never panicking on malformed input.
func parseGitdirRecord(data []byte) (worktreePath string, ok bool) {
	s := strings.TrimSpace(string(data))
	if s == "" || !filepath.IsAbs(s) {
		return "", false
	}
	return filepath.Dir(s), true
}

// lockedMarkerPresent reports whether entry's own worktrees/<id>/locked
// marker exists — LSTAT semantics (presence of ANYTHING, never a following
// stat; git's own lock marker file's content, if any, is irrelevant to this
// component).
func lockedMarkerPresent(lockedPath string) (bool, error) {
	kind, err := LstatType(lockedPath)
	if err != nil {
		return false, err
	}
	return kind != PathAbsent, nil
}

// corroborate performs the ONE read-only gitx.WorktreeList call (spec
// §Reused primitives) and, when Disclose is set, reports a disagreement
// against the direct administrative enumeration. It never affects control
// flow: a WorktreeList failure, or any disagreement it finds, is reported
// (if at all) and otherwise ignored — this component's scan and
// postcondition truth come only from reconcilePass/scanAdminDir.
func (r *GitReconciler) corroborate(ctx context.Context, repoRoot, adminDir, unitPath, canonicalUnit string) {
	listed, lerr := gitx.WorktreeList(ctx, repoRoot)
	if lerr != nil {
		r.disclose(fmt.Sprintf("reconcile: corroboration: gitx.WorktreeList failed for %s (informational only, never authoritative): %v", unitPath, lerr))
		return
	}
	directMatches, derr := scanAdminDir(adminDir, canonicalUnit)
	if derr != nil {
		return // the caller's own subsequent pass will surface this failure.
	}

	listedMatch := false
	for _, wt := range listed {
		if canonicalPath(wt.Path) == canonicalUnit {
			listedMatch = true
			break
		}
	}
	directMatch := len(directMatches) > 0
	if listedMatch != directMatch {
		r.disclose(fmt.Sprintf(
			"reconcile: corroboration disagreement for %s: gitx.WorktreeList reports match=%v, direct administrative enumeration reports match=%v (WorktreeList omits entries with a missing/unreadable gitdir record and never sees the main worktree — never authoritative for scan or postcondition)",
			unitPath, listedMatch, directMatch,
		))
	}
}

func (r *GitReconciler) disclose(line string) {
	if r.Disclose != nil {
		r.Disclose(line)
	}
}

// canonicalPath symlink-resolves p for a stable comparison against git's
// own already-resolved administrative-layout paths — the /tmp-vs-
// /private/tmp class internal/reclaim.canonicalPath (predicate.go) exists
// to survive for git-reported, currently-existing paths.
//
// CHOICE DOCUMENTED (proven by test, not merely asserted): this component
// must ALSO compare correctly when the unit path itself does not exist on
// disk — a stale registration whose directory was removed out-of-band, a
// registry-only unit, or any comparison run after this component's own
// step-5 deletion — and reclaim's plain "EvalSymlinks, else
// filepath.Clean(p) unresolved" fallback is PROVEN INSUFFICIENT for that
// case: when p's leaf is absent, EvalSymlinks(p) fails outright and a bare
// Clean(p) never looks at p's ANCESTOR components, so a symlinked ancestor
// (t.TempDir()'s own /var/folders/ symlinking to /private/var/folders/ on
// macOS being the everyday case, exactly the spec's own "/tmp versus
// /private/tmp" example) is left unresolved on this side of the comparison
// while git's own gitdir record — captured while the directory still
// existed — already names the resolved spelling. Comparing those two
// spellings as unequal would let a stale registration survive undetected,
// silently violating the postcondition this package exists to prove. This
// function therefore resolves the LONGEST EXISTING ANCESTOR via
// EvalSymlinks and rejoins the (possibly absent) remainder — the
// alternative the contract names — recursing toward the root exactly once
// per path component that does not itself exist.
func canonicalPath(p string) string {
	if p == "" {
		return ""
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return filepath.Clean(real)
	}
	clean := filepath.Clean(p)
	dir := filepath.Dir(clean)
	if dir == clean {
		// Reached the root (or p was already "." / relative-degenerate)
		// without finding an existing ancestor: nothing left to resolve.
		return clean
	}
	return filepath.Join(canonicalPath(dir), filepath.Base(clean))
}
