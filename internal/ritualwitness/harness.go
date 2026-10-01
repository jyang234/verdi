package ritualwitness

import (
	"context"
	"testing"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// Result is one harness run's outcome: the ritual's exit classification,
// its command log, its before/after Snapshots, the store root Evaluate
// related every worktree path to, and its Verdicts against decl.
type Result struct {
	Exit      int
	Log       CommandLog
	Before    Snapshot
	After     Snapshot
	StoreRoot string
	Verdicts  []Verdict
}

// Run is the harness's one composition point (parent ac-1): it seeds a
// fresh Fixture to state, snapshots it, drives the ritual through d,
// snapshots it again, and evaluates the result against decl. Every git
// invocation involved — the fixture's own setup, d's driving of the
// ritual, and Capture's sensing — stays within the fixture's own temp
// directories and its local bare remote (co-3: hermetic, no network).
func Run(t *testing.T, ctx context.Context, d Driver, decl ws.Declaration, state SeedState) Result {
	t.Helper()
	fx := Build(t, ctx, state)

	before, err := Capture(ctx, fx.Dir, fx.Bare)
	if err != nil {
		t.Fatalf("ritualwitness: Run: capturing the before snapshot: %v", err)
	}

	exit, log, driverErr := d.Run(ctx, fx.Dir)
	if driverErr != nil && exit == 0 {
		t.Fatalf("ritualwitness: Run: driver reported exit 0 alongside an error: %v", driverErr)
	}

	after, err := Capture(ctx, fx.Dir, fx.Bare)
	if err != nil {
		t.Fatalf("ritualwitness: Run: capturing the after snapshot: %v", err)
	}

	return Result{
		Exit:      exit,
		Log:       log,
		Before:    before,
		After:     after,
		StoreRoot: fx.Dir,
		Verdicts:  Evaluate(decl, exit, before, after, fx.Dir, log),
	}
}
