// TestReadinessLoadCLIVectorsReachRegisteredCommands re-homes the deleted
// cmd/verdi/readiness_snapshot_integration_test.go's
// TestReadinessSnapshotEmittedCLIVectorsReachRegisteredCommands (git show
// 685311a2) for internal/readinessload.Load, Task 2's per-request
// successor to the old startup-only adapter: every distinct
// Destination.CLI vector a real derivation emits — over a feature
// fixture, both WITH and WITHOUT a --context-request — is run against the
// real built binary and must dispatch to a registered verb (an
// operational or verdict exit, never dispatch.go's own "unknown verb"
// usage rejection). Task 3 exit obligation (task-2-review.md, Deviation
// 8): internal/readinessload itself cannot build/exec the real verdi
// binary from within its own package test, so this round trip lives here,
// at the one layer that owns both a real Load call and the built binary.
//
// Deviation from the deleted test's own coverage check: the old test used
// a story-class fixture whose spec produced a success/blocker/obligation-
// quality/coverage concern (a CLI-bearing kind); this feature-class
// fixture's unproven success-area rows are all success/contributor/*,
// which readinesspilot routes to the board, never the CLI — so AreaSuccess
// genuinely never emits a vector here (see the coverage check below).
package main

import (
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/readinessload"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/workbench"
)

func TestReadinessLoadCLIVectorsReachRegisteredCommands(t *testing.T) {
	bin := buildVerdiBinary(t)

	// WITH a request: NO judge is configured at all (manifest.Align stays
	// nil — buildContextCompileRepo's own plain manifest declares no
	// align: section), so there is structurally nothing for
	// NewConflictProvider to ever launch (co-1) — the evaluation
	// completes with no judge exchange, which readinesspilot reports as
	// an unproven context/verdict concern with witness
	// "judge-unavailable" (the same outcome internal/readinessload's own
	// TestLoad_WithRequestCacheMissNeverRunsJudge proves for a CONFIGURED
	// but cache-missed judge), giving this test a REAL, runnable CLI
	// vector (`verdi context conflict --request <the real request
	// file>`) without any custom provider or process ever being
	// launchable.
	withRequestRepo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	requestPath := writeContextRequestFile(t, withRequestRepo.Dir, "readiness-request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
	withRequest, err := readinessload.Load(context.Background(), withRequestRepo.Dir, "spec/feature-alpha", readinessload.Options{
		ContextRequestPath: requestPath, BoardHref: workbench.BranchBoardHref,
	})
	if err != nil {
		t.Fatalf("Load (with a request): %v", err)
	}
	if verdict := readinessLoadDispatchConcern(t, withRequest, "context/verdict"); verdict.State == readinesspilot.StateProven {
		t.Fatalf("context/verdict concern = %+v, want unproven (no judge is configured, so this must not accidentally pass)", verdict)
	}

	// WITHOUT a request: ContextRequestPath empty — R-RR1-5's own fixed
	// witness and CLI fallback (verdi context conflict --request <path>,
	// an instructive vector, not a runnable one against any real file).
	withoutRequestRepo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	withoutRequest, err := readinessload.Load(context.Background(), withoutRequestRepo.Dir, "spec/feature-alpha", readinessload.Options{
		BoardHref: workbench.BranchBoardHref,
	})
	if err != nil {
		t.Fatalf("Load (without a request): %v", err)
	}

	env := []string{"CI_DEFAULT_BRANCH=main"}
	executedAreas := map[readinesspilot.AreaID]bool{}
	seen := map[string]bool{}
	for _, tc := range []struct {
		name string
		root string
		snap readinesspilot.Snapshot
	}{
		{"with-request", withRequestRepo.Dir, withRequest},
		{"without-request", withoutRequestRepo.Dir, withoutRequest},
	} {
		for _, concern := range tc.snap.AllConcerns {
			cli := concern.Destination.CLI
			if len(cli) == 0 {
				continue
			}
			key := tc.name + "\x00" + strings.Join(cli, "\x00")
			if seen[key] {
				continue
			}
			if cli[0] != "verdi" {
				// runVerdiBinary is handed cli[1:], so a vector that did
				// not begin with the binary's own name would silently lose
				// a real argument here and could still pass (fix round 1,
				// M4). Every vector readinesspilot emits today starts with
				// "verdi"; this keeps the round trip self-guarding if one
				// ever does not.
				t.Fatalf("emitted CLI %q does not begin with %q — this round trip strips cli[0] as the binary's own name", cli, "verdi")
			}
			seen[key] = true
			executedAreas[concern.Area] = true
			t.Run(tc.name+"/"+concern.ID, func(t *testing.T) {
				// --help-safe: this check reads stderr only and accepts
				// exit 0 unconditionally, so a vector that happened to
				// carry a literal --help/-h token (none of readinesspilot's
				// fixed fallback vectors do today; this stays correct if
				// one ever does) would print its usage text to stdout and
				// exit 0 — never misread here as a dispatch rejection.
				stdout, stderr, code := runVerdiBinary(t, bin, tc.root, env, cli[1:]...)
				if strings.Contains(stderr, "usage:") || strings.Contains(stderr, "not implemented") {
					t.Fatalf("emitted CLI %q was rejected by registered dispatch grammar: exit=%d stdout=%q stderr=%q", cli, code, stdout, stderr)
				}
				if code < 0 || code > 2 {
					t.Fatalf("emitted CLI %q returned exit=%d, want honest clean/verdict/operational exit 0/1/2; stdout=%q stderr=%q", cli, code, stdout, stderr)
				}
			})
		}
	}

	// Unlike the deleted test's story-class fixture (whose spec produced a
	// success/blocker/obligation-quality/coverage row with a CLI fallback),
	// this feature fixture's own unproven success-area rows are all
	// success/contributor/* — readinesspilot routes THAT kind to the
	// board, never the CLI, so AreaSuccess genuinely never emits a vector
	// here. Disclosed rather than forced: checked below only for the two
	// areas this fixture genuinely does exercise, plus a non-vacuous
	// floor so the loop above can never silently run zero vectors.
	if len(seen) == 0 {
		t.Fatal("no CLI vector was exercised at all — this test would vacuously pass with zero coverage")
	}
	for _, area := range []readinesspilot.AreaID{readinesspilot.AreaContext, readinesspilot.AreaReview} {
		if !executedAreas[area] {
			t.Errorf("no emitted CLI vector exercised for area %q across either snapshot", area)
		}
	}
	if executedAreas[readinesspilot.AreaShape] {
		t.Error("shape area emitted a CLI vector instead of the existing branch board destination (R-RR1-6)")
	}
}

// readinessLoadDispatchConcern finds concern id in snap.AllConcerns or
// fails the test — a small local lookup, mirroring the deleted
// readinessSnapshotConcern helper's role for this file alone.
func readinessLoadDispatchConcern(t *testing.T, snap readinesspilot.Snapshot, id string) readinesspilot.Concern {
	t.Helper()
	for _, c := range snap.AllConcerns {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("snapshot has no concern %q among %d concerns", id, len(snap.AllConcerns))
	return readinesspilot.Concern{}
}
