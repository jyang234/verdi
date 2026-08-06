package execworkspace

// GC implements spec/execution-workspace §GC slice: the execution slice of
// `verdi gc` (invention SI-11, disclosed). Rank numbers in comments below
// cite that section's own numbered decision list verbatim. It extends
// wtmanager.decideReclaim's total-ordered-switch shape (§Safe cleanup) with
// the ranks this component's larger state space needs, and follows
// GitReconciler's own two-phase read/mutate discipline: ranks CLASSIFY
// read-only, and MUTATION AT ANY RANK HAPPENS ONLY UNDER THE ACQUIRED UNIT
// LOCK, RE-DERIVED immediately before mutating — "a decision that no longer
// holds under the lock is re-decided, never applied."
import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/gitx"
)

// GCOutcome is the GC slice's TOTAL outcome set (spec §GC slice): the six
// ranked outcomes plus the disclosed PARTIAL outcome of a reclaim step that
// failed. Compile-time-exhaustive, mirroring internal/reclaim.KeptReason's
// own pattern (keptReasonNames): a value added here without a matching
// gcOutcomeNames entry fails the BUILD, never runs with a silently blank
// label.
type GCOutcome int

const (
	// GCOutcomeUnknown is the zero value, never returned alongside a nil
	// error from any exported entry point — the same discipline
	// PathKind/Outcome already use in this package.
	GCOutcomeUnknown GCOutcome = iota
	// ReclaimOrphaned is rank 0's mutating outcome: nothing at all at the
	// unit path (siblings only, or a registry-only unit).
	ReclaimOrphaned
	// KeepMalformed is rank 1's outcome, and also every rank's own
	// unevaluable-predicate keep whose "cannot tell" class is malformed
	// (rank 0's lstat failure at the unit path).
	KeepMalformed
	// KeepNotEligible is rank 2's outcome: nothing at the marker path.
	KeepNotEligible
	// KeepDirty is rank 3's outcome, and its own unevaluable-predicate
	// keep (a gitx.StatusDirty error).
	KeepDirty
	// KeepLocked is rank 4's outcome: the unit's lock cannot be acquired.
	KeepLocked
	// Reclaimed is rank 5's mutating outcome.
	Reclaimed
	// Partial is the disclosed partial outcome of a failed reclaim step
	// (rank 0 or rank 5), never a reported success; the sweep continues.
	Partial
	numGCOutcomes // sentinel: always one past the last real value.
)

// gcOutcomeNames is GCOutcome's own compile-time exhaustiveness check —
// see internal/reclaim.keptReasonNames' own doc comment for the mechanism
// (an ellipsis-sized array literal assigned to the sentinel-sized declared
// type only compiles when the lengths are identical).
var gcOutcomeNames [numGCOutcomes]string = [...]string{
	GCOutcomeUnknown: "unknown",
	ReclaimOrphaned:  "reclaim-orphaned",
	KeepMalformed:    "keep-malformed",
	KeepNotEligible:  "keep-not-eligible",
	KeepDirty:        "keep-dirty",
	KeepLocked:       "keep-locked",
	Reclaimed:        "reclaimed",
	Partial:          "partial",
}

// String renders o's closed-vocabulary label, or a self-naming "unknown"
// fallback for a value outside the closed set (CLAUDE.md: "unknown enum
// values fail closed").
func (o GCOutcome) String() string {
	if o < 0 || int(o) >= len(gcOutcomeNames) {
		return fmt.Sprintf("execworkspace.GCOutcome(%d)", int(o))
	}
	return gcOutcomeNames[o]
}

// GCResult is the execution gc slice's one-line-per-unit report (§Safe
// cleanup: "one disclosed result line per unit"). Detail carries the
// disclosed reason text: which lstat/StatusDirty/lock check failed and how,
// which acquisition case applied, or which reclaim step failed — never a
// generic, undifferentiated message.
type GCResult struct {
	WorkspaceID string
	Outcome     GCOutcome
	Detail      string
}

// Line renders r as gc's disclosed report line, mirroring
// wtmanager.Result.Line's own per-reason style: a distinct message per
// outcome, never one undifferentiated "kept" or "reclaimed" line.
func (r GCResult) Line() string {
	switch r.Outcome {
	case ReclaimOrphaned:
		return fmt.Sprintf("execution: reclaim-orphaned: %s (orphaned metadata cleared, registry reconciled)", r.WorkspaceID)
	case Reclaimed:
		return fmt.Sprintf("execution: reclaimed: %s (registration remains; a later gc resolves it)", r.WorkspaceID)
	case KeepMalformed:
		return fmt.Sprintf("execution: kept: malformed (%s): %s", r.WorkspaceID, r.Detail)
	case KeepNotEligible:
		return fmt.Sprintf("execution: kept: not eligible (%s): not yet released", r.WorkspaceID)
	case KeepDirty:
		if r.Detail == "" {
			return fmt.Sprintf("execution: kept: uncommitted changes (%s)", r.WorkspaceID)
		}
		return fmt.Sprintf("execution: kept: uncommitted changes (%s): %s", r.WorkspaceID, r.Detail)
	case KeepLocked:
		return fmt.Sprintf("execution: kept: in use (%s): %s", r.WorkspaceID, r.Detail)
	case Partial:
		return fmt.Sprintf("execution: partial: %s: %s (sweep continues)", r.WorkspaceID, r.Detail)
	default:
		return fmt.Sprintf("execution: %s (%s): %s", r.Outcome, r.WorkspaceID, r.Detail)
	}
}

func keepResult(workspaceID string, outcome GCOutcome, detail string) GCResult {
	return GCResult{WorkspaceID: workspaceID, Outcome: outcome, Detail: detail}
}

func partialResult(workspaceID, detail string) GCResult {
	return GCResult{WorkspaceID: workspaceID, Outcome: Partial, Detail: detail}
}

// gcHookAfterLockForTests is a TEST-ONLY seam, defaulted to nil (no-op) and
// never set by production code — mirrors GitReconciler.afterClaimForTests.
// It runs immediately after EITHER mutating rank's gate (rank 0's or rank
// 5's filelock.Acquire) succeeds and before that rank's own RE-DERIVATION
// check, so a test can deterministically land a state change in the exact
// window the spec's "RE-DERIVED under the acquired lock immediately before
// mutating" rule exists to catch — a window no external actor could
// otherwise hit on demand.
var gcHookAfterLockForTests func(workspaceID string)

// GC implements the execution slice of `verdi gc` end to end over one store
// root's data/execution/ tree, cutting/reconciling worktrees against
// repoRoot. It returns one GCResult per scanned unit, in deterministic
// (sorted workspace-id) order, plus every scan-level disclosure (also
// deterministically ordered) — grammar-external filesystem entries and
// unclassified/out-of-root-but-under-root administrative entries. The
// returned error is non-nil ONLY for a pre-sweep operational failure (the
// scan itself); every per-unit outcome, including every keep and every
// partial, is folded into its own GCResult and never fails this call
// (controller decision AD-10).
func GC(ctx context.Context, storeRoot, repoRoot string) ([]GCResult, []string, error) {
	gr := NewGitReconciler(storeRoot)
	ids, disclosures, err := scanUnits(ctx, gr, storeRoot, repoRoot)
	if err != nil {
		return nil, nil, err
	}

	sortedIDs := make([]string, 0, len(ids))
	for id := range ids {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Strings(sortedIDs)

	results := make([]GCResult, 0, len(sortedIDs))
	for _, id := range sortedIDs {
		results = append(results, decideUnit(ctx, storeRoot, repoRoot, id, gr))
	}
	return results, disclosures, nil
}

// scanUnits computes the SCAN SET — the union of the filesystem grammar
// half and the administrative-enumeration half (spec §GC slice) — plus
// every scan-level disclosure. gr supplies both the canonicalized execution
// root the administrative half classifies against and the SAME
// resolution/canonicalization helpers (resolveAdminDir, resolveAdminEntry)
// GitReconciler's own reconciliation uses, reused here rather than
// copy-pasted (spec: "Reuse the reconciler's resolution/canonicalization
// helpers").
func scanUnits(ctx context.Context, gr *GitReconciler, storeRoot, repoRoot string) (map[string]bool, []string, error) {
	ids := map[string]bool{}
	var disclosures []string

	// (a) filesystem grammar half: every entry under data/execution/. A
	// missing execution root is an empty filesystem half, not an error.
	entries, err := os.ReadDir(ExecutionRoot(storeRoot))
	switch {
	case err == nil:
		for _, e := range entries {
			ce, ok := ClassifyEntry(e.Name())
			if !ok {
				disclosures = append(disclosures, fmt.Sprintf(
					"execution: unclassified entry %q under data/execution/, kept for human attention", e.Name()))
				continue
			}
			ids[ce.WorkspaceID] = true
		}
	case errors.Is(err, os.ErrNotExist):
		// Empty filesystem half.
	default:
		return nil, nil, fmt.Errorf("execworkspace: gc: scan execution root: %w", err)
	}

	// (b) administrative enumeration half: the SAME enumeration the
	// reconciler uses (gr.resolveAdminDir / resolveAdminEntry, both reused
	// unexported helpers from reconcile.go).
	adminDir, err := gr.resolveAdminDir(ctx, repoRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("execworkspace: gc: resolve administrative directory: %w", err)
	}
	adminEntries, err := os.ReadDir(adminDir)
	switch {
	case err == nil:
		for _, e := range adminEntries {
			if !e.IsDir() {
				continue
			}
			entryDir := filepath.Join(adminDir, e.Name())
			resolved, _, ok := resolveAdminEntry(entryDir)
			if !ok {
				disclosures = append(disclosures, fmt.Sprintf(
					"execution: unclassified administrative entry %s, kept for human attention", entryDir))
				continue
			}
			id, isUnit, underRoot := classifyResolvedAdminPath(resolved, gr.canonicalExecutionRoot)
			switch {
			case isUnit:
				ids[id] = true
			case underRoot:
				disclosures = append(disclosures, fmt.Sprintf(
					"execution: unclassified administrative entry %s (resolves to %s, under data/execution/ but names no unit), kept for human attention",
					entryDir, resolved))
			default:
				// Resolved OUTSIDE data/execution/: another slice's, never
				// touched, no disclosure needed (spec: "never touched and
				// not a unit here").
			}
		}
	case errors.Is(err, os.ErrNotExist):
		// No administrative entries registered at all.
	default:
		return nil, nil, fmt.Errorf("execworkspace: gc: enumerate administrative directory: %w", err)
	}

	sort.Strings(disclosures)
	return ids, disclosures, nil
}

// classifyResolvedAdminPath implements the administrative-entry
// classification rule (spec §GC slice): resolved's relationship to
// canonicalRoot decides whether it names a unit, names no unit but is
// provably under the root (a scan-level disclosure), or resolves outside
// the root entirely (silently not a unit here, never touched).
func classifyResolvedAdminPath(resolved, canonicalRoot string) (workspaceID string, isUnit, underRoot bool) {
	if canonicalRoot == "" || resolved == "" {
		return "", false, false
	}
	sep := string(filepath.Separator)
	if resolved == canonicalRoot {
		// Degenerate: resolves to the execution root itself, which is
		// never a unit path (a unit path is always exactly one level
		// under the root).
		return "", false, true
	}
	if !strings.HasPrefix(resolved, canonicalRoot+sep) {
		return "", false, false
	}
	if filepath.Dir(resolved) == canonicalRoot {
		base := filepath.Base(resolved)
		if ValidWorkspaceID(base) {
			return base, true, true
		}
	}
	return "", false, true
}

// decideUnit is the per-unit TOTAL, ORDERED decision (spec §GC slice, ranks
// 0-5), exactly one disclosed reason per unit. MALFORMATION IS TESTED
// BEFORE ELIGIBILITY: a malformed unit path is never disclosed as the
// ordinary not-yet-released case. Unexported and reconciler-parameterized
// so tests can exercise every rank against a hermetic fake reconciler; GC
// (above) always supplies a real *GitReconciler.
func decideUnit(ctx context.Context, storeRoot, repoRoot, workspaceID string, reconciler Reconciler) GCResult {
	unitPath := UnitPath(storeRoot, workspaceID)

	unitKind, err := LstatType(unitPath)
	if err != nil {
		// "An LSTAT FAILURE at the unit path is likewise an OPERATIONAL
		// ERROR ... never read as absence": rank 0's own carve-out, whose
		// keep kind is KEEP-MALFORMED, never reclaim-orphaned.
		return keepResult(workspaceID, KeepMalformed, fmt.Sprintf("lstat unit path failed: %v", err))
	}
	if unitKind == PathAbsent {
		return decideRank0(ctx, storeRoot, repoRoot, workspaceID, reconciler)
	}
	if unitKind != PathDir {
		// Rank 1: a non-directory object at the unit path.
		return keepResult(workspaceID, KeepMalformed, fmt.Sprintf("unit path is %s, not a directory", unitKind))
	}

	markerPath := ReleasedPath(storeRoot, workspaceID)
	markerKind, err := LstatType(markerPath)
	if err != nil {
		return keepResult(workspaceID, KeepMalformed, fmt.Sprintf("lstat marker path failed: %v", err))
	}
	switch markerKind {
	case PathAbsent:
		// Rank 2: nothing at all at the marker path.
		return keepResult(workspaceID, KeepNotEligible, "")
	case PathRegular:
		// Proceeds to rank 3.
	default:
		// Rank 1: a non-regular object at the marker path.
		return keepResult(workspaceID, KeepMalformed, fmt.Sprintf("marker path is %s, not a regular file", markerKind))
	}

	dirty, derr := gitx.StatusDirty(ctx, unitPath)
	if derr != nil {
		// The unevaluable-predicate keep mints no new reason kind: a
		// gitx.StatusDirty error keeps at rank 3's own kind (keep-dirty),
		// fail-closed, naming the failed check.
		return keepResult(workspaceID, KeepDirty, fmt.Sprintf("StatusDirty check failed: %v", derr))
	}
	if dirty {
		// Rank 3: uncommitted changes.
		return keepResult(workspaceID, KeepDirty, "")
	}

	return decideRank5(ctx, storeRoot, repoRoot, workspaceID, reconciler)
}

// tryAcquireLock attempts filelock.Acquire for the shared rank-0/rank-4 gate
// ("attempt filelock.Acquire ... failure — a live holder, or any other
// acquisition failure such as an undecodable lock body — keep-locked, the
// disclosure naming which case, fail-closed"). It returns a non-empty
// detail string naming the failure case on failure.
func tryAcquireLock(lockPath string) (*os.File, string, error) {
	f, err := filelock.Acquire(lockPath)
	if err != nil {
		return nil, describeAcquireFailure(err), err
	}
	return f, "", nil
}

// describeAcquireFailure names WHICH acquisition-failure case occurred, per
// the spec's fail-closed disclosure requirement: a live holder (naming its
// pid) is distinguished from every other acquisition failure (an
// undecodable lock body, a permission error, etc.).
func describeAcquireFailure(err error) string {
	var held *filelock.ErrHeld
	if errors.As(err, &held) {
		return fmt.Sprintf("live holder pid %d", held.Info.PID)
	}
	return fmt.Sprintf("lock acquisition failed: %v", err)
}

// decideRank0 implements rank 0: nothing at all at the unit path (siblings
// only, or nothing on disk at all for a registry-only unit). The gate is
// the shared acquisition rule above; a lone live .lock (materialization in
// flight) fails acquisition and keeps-locked without touching anything. A
// lone stale .lock is taken over by filelock.Acquire's own stale-lock
// detection (no error), and its deletion below (via the final
// filelock.Release call) is that holder's own release — the same single
// fused operation this rank names.
func decideRank0(ctx context.Context, storeRoot, repoRoot, workspaceID string, reconciler Reconciler) GCResult {
	lockPath := LockPath(storeRoot, workspaceID)
	lockFile, detail, aerr := tryAcquireLock(lockPath)
	if aerr != nil {
		return keepResult(workspaceID, KeepLocked, "rank 0 (lone .lock — materialization in flight, or other acquisition failure): "+detail)
	}

	if gcHookAfterLockForTests != nil {
		gcHookAfterLockForTests(workspaceID)
	}

	// RE-DERIVE under the acquired lock, immediately before mutating: is
	// the unit path STILL absent? A decision that no longer holds is
	// re-decided, never applied.
	unitPath := UnitPath(storeRoot, workspaceID)
	unitKind, err := LstatType(unitPath)
	if err != nil || unitKind != PathAbsent {
		_ = filelock.Release(lockFile, lockPath)
		return decideUnit(ctx, storeRoot, repoRoot, workspaceID, reconciler)
	}

	for _, sib := range orphanSiblings(storeRoot, workspaceID) {
		kind, lerr := LstatType(sib.path)
		if lerr != nil {
			_ = filelock.Release(lockFile, lockPath)
			return partialResult(workspaceID, fmt.Sprintf("rank 0: lstat orphaned sibling (%s) failed: %v", sib.label, lerr))
		}
		switch kind {
		case PathAbsent:
			continue
		case PathRegular:
			if rerr := os.Remove(sib.path); rerr != nil {
				_ = filelock.Release(lockFile, lockPath)
				return partialResult(workspaceID, fmt.Sprintf("rank 0: unlink orphaned sibling (%s) failed: %v", sib.label, rerr))
			}
		default:
			_ = filelock.Release(lockFile, lockPath)
			return partialResult(workspaceID, fmt.Sprintf("rank 0: unexpected object kind %s at orphaned sibling (%s)", kind, sib.label))
		}
	}

	if rerr := reconciler.ReconcileUnit(ctx, repoRoot, unitPath); rerr != nil {
		_ = filelock.Release(lockFile, lockPath)
		return partialResult(workspaceID, fmt.Sprintf("rank 0: registry reconciliation failed: %v", rerr))
	}

	// The .lock deletion IS this holder's release of it — ONE fused
	// operation (filelock.Release closes then removes), never an unlink
	// followed by a second release.
	if relErr := filelock.Release(lockFile, lockPath); relErr != nil {
		return partialResult(workspaceID, fmt.Sprintf("rank 0: lock deletion (release) failed: %v", relErr))
	}
	return keepResult(workspaceID, ReclaimOrphaned, "")
}

// decideRank5 implements ranks 4 and 5 together: rank 4's acquisition IS
// the gate for rank 5's reclaim, held across the whole reclaim. On success,
// the decision is RE-DERIVED under the lock (unit path still a real
// directory, marker still a regular file, still clean) before the FIXED
// ORDER of deletions runs: .request.staging, .request, unit path
// (os.RemoveAll), .released, .lock — the last deletion fused as this
// holder's own release, exactly as rank 0's. The spec's own fixed order
// names exactly these five deletions and no in-rank registry
// reconciliation: a reclaim's surviving registration (a `git worktree`
// registration outlives a directory deletion) is rank 0's job on a LATER
// invocation, disclosed here rather than "fixed" by inventing an extra
// reconciliation step this rank's own text does not name.
func decideRank5(ctx context.Context, storeRoot, repoRoot, workspaceID string, reconciler Reconciler) GCResult {
	lockPath := LockPath(storeRoot, workspaceID)
	lockFile, detail, aerr := tryAcquireLock(lockPath)
	if aerr != nil {
		return keepResult(workspaceID, KeepLocked, "rank 4: "+detail)
	}

	if gcHookAfterLockForTests != nil {
		gcHookAfterLockForTests(workspaceID)
	}

	unitPath := UnitPath(storeRoot, workspaceID)
	unitKind, err := LstatType(unitPath)
	if err != nil || unitKind != PathDir {
		_ = filelock.Release(lockFile, lockPath)
		return decideUnit(ctx, storeRoot, repoRoot, workspaceID, reconciler)
	}
	markerPath := ReleasedPath(storeRoot, workspaceID)
	markerKind, err := LstatType(markerPath)
	if err != nil || markerKind != PathRegular {
		_ = filelock.Release(lockFile, lockPath)
		return decideUnit(ctx, storeRoot, repoRoot, workspaceID, reconciler)
	}
	dirty, derr := gitx.StatusDirty(ctx, unitPath)
	if derr != nil || dirty {
		_ = filelock.Release(lockFile, lockPath)
		return decideUnit(ctx, storeRoot, repoRoot, workspaceID, reconciler)
	}

	stagingPath := RequestStagingPath(storeRoot, workspaceID)
	if err := removeRegularIfPresent(stagingPath); err != nil {
		_ = filelock.Release(lockFile, lockPath)
		return partialResult(workspaceID, fmt.Sprintf("rank 5: delete request-staging step failed: %v", err))
	}
	requestPath := RequestPath(storeRoot, workspaceID)
	if err := removeRegularIfPresent(requestPath); err != nil {
		_ = filelock.Release(lockFile, lockPath)
		return partialResult(workspaceID, fmt.Sprintf("rank 5: delete request step failed: %v", err))
	}
	if err := os.RemoveAll(unitPath); err != nil {
		_ = filelock.Release(lockFile, lockPath)
		return partialResult(workspaceID, fmt.Sprintf("rank 5: delete unit-path step failed: %v", err))
	}
	if err := removeRegularIfPresent(markerPath); err != nil {
		_ = filelock.Release(lockFile, lockPath)
		return partialResult(workspaceID, fmt.Sprintf("rank 5: delete released-marker step failed: %v", err))
	}
	// The .lock deletion IS this holder's release — one fused operation.
	if relErr := filelock.Release(lockFile, lockPath); relErr != nil {
		return partialResult(workspaceID, fmt.Sprintf("rank 5: delete lock step (release) failed: %v", relErr))
	}
	_ = detail // acquisition succeeded; detail is only meaningful on failure.
	return keepResult(workspaceID, Reclaimed, "")
}

// removeRegularIfPresent lstat-types path (never following a symlink) and
// removes it if it is a regular file; an absent path is a no-op; any other
// object kind, or the lstat itself, is reported as an error — used by rank
// 5's fixed-order sibling-deletion steps, each of which is "delete if
// present" against a form that has always been lstat-typed uniformly
// (never a following stat) throughout this package.
func removeRegularIfPresent(path string) error {
	kind, err := LstatType(path)
	if err != nil {
		return err
	}
	switch kind {
	case PathAbsent:
		return nil
	case PathRegular:
		return os.Remove(path)
	default:
		return fmt.Errorf("unexpected object kind %s at %s", kind, path)
	}
}
