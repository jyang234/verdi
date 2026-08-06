package execworkspace

// Contract item 4: CONCURRENCY COMPOSITION (-race) — one test running,
// against a single store with 3 units in different states, concurrent
// {Materialize(unit A, fresh each round), Release(unit B), GC(whole
// store)} repeatedly, asserting invariants after every round: no unit
// ever half-deleted, no cross-unit interference, race detector clean.
//
// INVARIANTS PROVEN (rather than asserted by fiat): every mutator in this
// package acquires the SAME per-unit filelock.Acquire (non-blocking)
// before mutating, and every exported entry point (Materialize, Release,
// gc's decideUnit inside GC) releases it — via a deferred
// filelock.Release — before RETURNING to its caller. By the time
// sync.WaitGroup.Wait() returns for one round, every one of that round's
// own goroutines has therefore already released whatever lock(s) it
// briefly held, so NO lock file should survive for any of the three
// tracked units at that point — the strong, checkable invariant this test
// asserts every round, rather than a softer "eventually consistent" one.
//
// Fixtures reuse buildTestRepo (materialize_test.go).
import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/filelock"
)

// isAllowedConcurrentReleaseError reports whether err is the ONE
// contention shape this composition can legitimately produce for a
// concurrent Release call: the unit's lock momentarily held by gc's own
// rank-0/rank-5 mutation of the SAME unit (Release's own acquisition is
// non-blocking, per spec §GC slice: "a release whose acquisition fails —
// a live holder ... — is an OPERATIONAL ERROR ... never a wait"). Any
// other error is a real bug, not a benign race.
func isAllowedConcurrentReleaseError(err error) bool {
	if err == nil {
		return true
	}
	var held *filelock.ErrHeld
	return errors.As(err, &held)
}

// assertUnitLockAbsent proves the "no lock ever survives past a round"
// invariant described in the file header.
func assertUnitLockAbsent(t *testing.T, storeRoot, workspaceID string) {
	t.Helper()
	kind, err := LstatType(LockPath(storeRoot, workspaceID))
	if err != nil {
		t.Fatalf("lstat lock path for %s: %v", workspaceID, err)
	}
	if kind != PathAbsent {
		t.Fatalf("unit %s: .lock still present (kind=%s) after every concurrent mutator for this round returned — a lock leaked past its own deferred release", workspaceID, kind)
	}
}

// assertUnitStructurallyConsistent asserts the "never half-deleted"
// invariant for one unit: every present sibling is the SHAPE this
// package alone ever writes there (a real directory at the unit path,
// nothing else; a regular, DECODABLE witness at .request — never
// partial bytes exposed mid-write, proven by the temp-then-rename
// discipline; a regular .request.staging/.released, never a foreign
// object kind).
func assertUnitStructurallyConsistent(t *testing.T, storeRoot, workspaceID string) {
	t.Helper()
	if kind, err := LstatType(UnitPath(storeRoot, workspaceID)); err != nil {
		t.Fatalf("lstat unit path for %s: %v", workspaceID, err)
	} else if kind != PathAbsent && kind != PathDir {
		t.Fatalf("unit %s: unit path is %s, want absent or a real directory (never half-deleted into some other object kind)", workspaceID, kind)
	}
	if kind, err := LstatType(RequestStagingPath(storeRoot, workspaceID)); err != nil {
		t.Fatalf("lstat .request.staging for %s: %v", workspaceID, err)
	} else if kind != PathAbsent && kind != PathRegular {
		t.Fatalf("unit %s: .request.staging is %s, want absent or regular", workspaceID, kind)
	}
	if kind, err := LstatType(ReleasedPath(storeRoot, workspaceID)); err != nil {
		t.Fatalf("lstat .released for %s: %v", workspaceID, err)
	} else if kind != PathAbsent && kind != PathRegular {
		t.Fatalf("unit %s: .released is %s, want absent or regular", workspaceID, kind)
	}

	requestPath := RequestPath(storeRoot, workspaceID)
	kind, err := LstatType(requestPath)
	if err != nil {
		t.Fatalf("lstat .request for %s: %v", workspaceID, err)
	}
	if kind != PathAbsent && kind != PathRegular {
		t.Fatalf("unit %s: .request is %s, want absent or regular", workspaceID, kind)
	}
	if kind == PathRegular {
		data, rerr := os.ReadFile(requestPath)
		if rerr != nil {
			t.Fatalf("reading .request for %s: %v", workspaceID, rerr)
		}
		if _, derr := DecodeSidecar(data); derr != nil {
			t.Fatalf("unit %s: .request present but UNDECODABLE (%v) — a concurrent operation exposed a partial witness, violating temp-then-rename atomicity", workspaceID, derr)
		}
	}
}

func TestConcurrency_MaterializeReleaseGC_Composition(t *testing.T) {
	ctx := context.Background()
	repo := buildTestRepo(t)
	storeRoot := t.TempDir()
	reconciler := NewGitReconciler(storeRoot)
	m, err := NewMaterializer(storeRoot, repo.Dir, reconciler)
	if err != nil {
		t.Fatalf("NewMaterializer: %v", err)
	}
	releaser := NewReleaser(storeRoot)

	// unit B: pre-materialized, never released before the loop starts —
	// the loop's own concurrent Release calls are what release it (and
	// are naturally idempotent from the second successful call on).
	bID, err := NewExactIdentity("conc-b", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity(B): %v", err)
	}
	bRes, err := m.Materialize(ctx, Request{Identity: bID})
	if err != nil {
		t.Fatalf("Materialize(B) setup: %v", err)
	}
	bWorkspaceID := bRes.WorkspaceID

	// unit C: pre-materialized AND pre-released before the loop starts —
	// a clean, immediate reclaim target for the very first concurrent GC.
	cID, err := NewExactIdentity("conc-c", repo.Head)
	if err != nil {
		t.Fatalf("NewExactIdentity(C): %v", err)
	}
	cRes, err := m.Materialize(ctx, Request{Identity: cID})
	if err != nil {
		t.Fatalf("Materialize(C) setup: %v", err)
	}
	cWorkspaceID := cRes.WorkspaceID
	if err := releaser.Release(cWorkspaceID); err != nil {
		t.Fatalf("Release(C) setup: %v", err)
	}

	const iterations = 6 // "5+ iterations" per the contract
	for round := 0; round < iterations; round++ {
		aID, err := NewExactIdentity(fmt.Sprintf("conc-a-%d", round), repo.Head)
		if err != nil {
			t.Fatalf("round %d: NewExactIdentity(A): %v", round, err)
		}
		aWantWorkspaceID, err := aID.WorkspaceID()
		if err != nil {
			t.Fatalf("round %d: WorkspaceID(A): %v", round, err)
		}

		var wg sync.WaitGroup
		var materializeErr, releaseErr, gcErr error
		var materializeRes Result
		var gcResults []GCResult

		wg.Add(3)
		go func() {
			defer wg.Done()
			materializeRes, materializeErr = m.Materialize(ctx, Request{Identity: aID})
		}()
		go func() {
			defer wg.Done()
			releaseErr = releaser.Release(bWorkspaceID)
		}()
		go func() {
			defer wg.Done()
			gcResults, _, gcErr = GC(ctx, storeRoot, repo.Dir)
		}()
		wg.Wait()

		if materializeErr != nil {
			t.Fatalf("round %d: Materialize(A) unexpectedly failed (nothing else in this composition ever targets A's unique per-round id): %v", round, materializeErr)
		}
		if materializeRes.Outcome != OutcomeMaterialized || materializeRes.WorkspaceID != aWantWorkspaceID {
			t.Fatalf("round %d: Materialize(A) result = %+v, want OutcomeMaterialized for %s", round, materializeRes, aWantWorkspaceID)
		}
		if !isAllowedConcurrentReleaseError(releaseErr) {
			t.Fatalf("round %d: Release(B) failed with an unexpected (not lock-contention) error: %v", round, releaseErr)
		}
		if gcErr != nil {
			t.Fatalf("round %d: GC returned a pre-sweep error (AD-10: per-unit conditions must never fail GC itself): %v", round, gcErr)
		}
		t.Logf("round %d: releaseErr=%v gcResultCount=%d", round, releaseErr, len(gcResults))

		for _, id := range []string{aWantWorkspaceID, bWorkspaceID, cWorkspaceID} {
			assertUnitLockAbsent(t, storeRoot, id)
			assertUnitStructurallyConsistent(t, storeRoot, id)
		}

		// A's own witness must be identity-equal to what THIS round
		// requested — proof against cross-unit interference: a
		// concurrently-running Release(B)/GC could not have corrupted or
		// substituted A's own witness.
		data, rerr := os.ReadFile(RequestPath(storeRoot, aWantWorkspaceID))
		if rerr != nil {
			t.Fatalf("round %d: reading A's witness: %v", round, rerr)
		}
		witnessID, derr := DecodeSidecar(data)
		if derr != nil {
			t.Fatalf("round %d: decoding A's witness: %v", round, derr)
		}
		if !witnessID.Equal(aID) {
			t.Fatalf("round %d: A's witness identity = %s, want %s (cross-unit interference)", round, witnessID, aID)
		}
	}
}
