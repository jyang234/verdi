package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/OWNER/verdi/internal/gitx"
)

// acceptedRepo builds a Phase 7 repo, runs design start + accept, and
// returns it sitting on the design branch with spec/stale-decline at
// accepted-pending-build — the common starting point for feature start's
// happy-path tests.
func acceptedRepo(t *testing.T) *bytesRepo {
	t.Helper()
	repo, _ := scaffoldAndDesign(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	if got := runAccept(ctx, repo.Dir, "spec/stale-decline", &stdout, &stderr); got != 0 {
		t.Fatalf("runAccept = %d, want 0; stderr=%s", got, stderr.String())
	}
	return &bytesRepo{Dir: repo.Dir}
}

// bytesRepo is a minimal handle (just Dir) for tests that no longer need
// fixturegit.Repo's Head/Heads fields once the repo has moved on from its
// initial fixture state via real design/accept commits.
type bytesRepo struct{ Dir string }

// TestRunFeatureStart_RefusesDraft proves feature start refuses (exit 1) a
// spec still in draft, and never mutates the repo (no branch switch, no
// commit) when it does.
func TestRunFeatureStart_RefusesDraft(t *testing.T) {
	repo, _ := scaffoldAndDesign(t) // draft only, no accept
	ctx := context.Background()

	before, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}

	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	var stdout, stderr bytes.Buffer
	got := runFeatureStart(ctx, repo.Dir, "jira:LOAN-1482", deps, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runFeatureStart(draft spec) = %d, want 1; stderr=%s", got, stderr.String())
	}
	if !contains(stderr.String(), "not accepted-pending-build") {
		t.Fatalf("stderr = %q, want it to name the refusal", stderr.String())
	}

	after, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("a refused feature start must not switch branches: before=%q after=%q", before, after)
	}
}

// TestRunFeatureStart_Succeeds proves feature start cuts feature/<name>
// once the spec is accepted-pending-build, resolving via a story ref
// (I-30's scheme-prefixed form, reusing matrix.go's resolveSpec).
func TestRunFeatureStart_Succeeds(t *testing.T) {
	repo := acceptedRepo(t)
	ctx := context.Background()

	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	var stdout, stderr bytes.Buffer
	got := runFeatureStart(ctx, repo.Dir, "jira:LOAN-1482", deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runFeatureStart = %d, want 0; stderr=%s", got, stderr.String())
	}

	branch, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "feature/stale-decline" {
		t.Fatalf("CurrentBranch = %q, want feature/stale-decline", branch)
	}
}

// TestRunFeatureStart_SpecRefForm proves feature start also accepts the
// spec-ref form (I-30's second accepted form).
func TestRunFeatureStart_SpecRefForm(t *testing.T) {
	repo := acceptedRepo(t)
	ctx := context.Background()

	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	var stdout, stderr bytes.Buffer
	got := runFeatureStart(ctx, repo.Dir, "spec/stale-decline", deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runFeatureStart(spec ref) = %d, want 0; stderr=%s", got, stderr.String())
	}
}

// TestRunFeatureStart_Negative covers runFeatureStart's own
// operational-error path: an unresolvable story/spec ref.
func TestRunFeatureStart_Negative(t *testing.T) {
	repo := acceptedRepo(t)
	ctx := context.Background()
	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}

	t.Run("unknown story ref", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := runFeatureStart(ctx, repo.Dir, "jira:NOPE-1", deps, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runFeatureStart(unknown story) = %d, want 2", got)
		}
	})

	t.Run("bare tracker key", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := runFeatureStart(ctx, repo.Dir, "LOAN-1482", deps, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runFeatureStart(bare key) = %d, want 2", got)
		}
	})
}

// TestCmdFeatureStart_UsageNegative proves cmdFeatureStart's own
// argument-count check.
func TestCmdFeatureStart_UsageNegative(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := cmdFeatureStart(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdFeatureStart(no args) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := cmdFeatureStart([]string{"a", "b"}, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdFeatureStart(two args) = %d, want 2", got)
	}
}

// TestRunFeatureVerb_UnknownSubcommand mirrors design's own subcommand
// dispatch test.
func TestRunFeatureVerb_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := runFeatureVerb([]string{"bogus"}, &stdout, &stderr); got != 2 {
		t.Fatalf("runFeatureVerb(bogus) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := runFeatureVerb(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("runFeatureVerb(no args) = %d, want 2", got)
	}
}

// TestRun_FeatureDispatchesToRealVerb proves dispatch.go routes "feature"
// to the real implementation.
func TestRun_FeatureDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	got := run([]string{"feature", "start", "jira:LOAN-1482"}, &stderr)
	if got != 2 {
		t.Fatalf("run([feature start ...]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "usage") || contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}
