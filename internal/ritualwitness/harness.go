package ritualwitness

import (
	"context"
	"testing"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// Result is one harness run: the ritual's exit classification and error,
// its command log, the before and after Snapshots, and its Verdicts.
type Result struct {
	Exit     int
	Err      error
	Log      CommandLog
	Before   Snapshot
	After    Snapshot
	Verdicts []Verdict
}

// Run validates decl, seeds a fresh Fixture to state, and runs d on it
// (RunOn). Every git invocation involved stays within the fixture's own
// temporary directories and its local bare remote (no network).
func Run(t testing.TB, ctx context.Context, d Driver, decl ws.Declaration, state SeedState) Result {
	t.Helper()
	if err := decl.Validate(); err != nil {
		t.Fatalf("ritualwitness: Run: %v", err)
	}
	return RunOn(t, ctx, Build(t, ctx, state), d, decl)
}

// RunOn snapshots fx, drives one ritual through d, snapshots fx again,
// and evaluates the pair against decl. A test that needs more seeding than
// a SeedState gives adds it to fx before calling RunOn. A driver that
// reports no verb's exit class (verbExit) — a binary that never started, a
// request never answered — fails the test: the run is judged neither a
// clean run nor a refusal.
func RunOn(t testing.TB, ctx context.Context, fx *Fixture, d Driver, decl ws.Declaration) Result {
	t.Helper()
	before, err := Capture(ctx, fx.Dir, fx.Bare)
	if err != nil {
		t.Fatalf("ritualwitness: RunOn: the before snapshot: %v", err)
	}
	exit, log, driverErr := d.Run(ctx, fx.Dir)
	if !verbExit(exit) {
		t.Fatalf("ritualwitness: RunOn: the driver reported exit %d, which is no verb's exit class (0, 1, or 2): %v", exit, driverErr)
	}
	if driverErr != nil && exit == 0 {
		t.Fatalf("ritualwitness: RunOn: the driver reported exit 0 alongside an error: %v", driverErr)
	}
	after, err := Capture(ctx, fx.Dir, fx.Bare)
	if err != nil {
		t.Fatalf("ritualwitness: RunOn: the after snapshot: %v", err)
	}
	verdicts, err := Evaluate(decl, exit, before, after, log)
	if err != nil {
		t.Fatalf("ritualwitness: RunOn: %v", err)
	}
	return Result{Exit: exit, Err: driverErr, Log: log, Before: before, After: after, Verdicts: verdicts}
}

// verbExit reports whether exit is a verb's exit classification: 0 clean,
// 1 verdict failure, 2 operational refusal.
func verbExit(exit int) bool {
	return exit >= 0 && exit <= 2
}
