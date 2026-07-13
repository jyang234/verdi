package align

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/upstream"
)

const acceptanceBoundaryContractJSON = `{
  "service": "loansvc",
  "schema_version": "flowmap.boundary/v1",
  "entrypoints": { "http": [], "consumers": [] },
  "published": [ { "name": "notification-svc", "kind": "events" } ],
  "consumed": [],
  "external_dependencies": [],
  "blind_spots": []
}
`

const regeneratedBoundaryContractJSON = `{
  "service": "loansvc",
  "schema_version": "flowmap.boundary/v1",
  "entrypoints": { "http": [ { "method": "GET", "route": "/healthz" } ], "consumers": [] },
  "published": [ { "name": "notification-svc", "kind": "events" } ],
  "consumed": [ { "name": "audit-svc", "kind": "http" } ],
  "external_dependencies": [],
  "blind_spots": []
}
`

const loansvcFlowmapYAML = "version: 1\nservice: loansvc\n"

// boundaryWriteRunner wraps a Runner and simulates `flowmap boundary`'s
// real side effect (it writes .flowmap/boundary-contract.json in place —
// spike S1's "no stdout mode" finding) by writing branchContract to disk
// whenever a non-check boundary request passes through, since
// upstream.FakeRunner itself only returns canned Results and performs no
// filesystem I/O. Mirrors cmd/verdi/sync_test.go's own helper of the same
// shape (test-local duplication of a small, package-specific test double,
// not production logic).
type boundaryWriteRunner struct {
	upstream.Runner
	svcDir         string
	branchContract []byte
}

func (r boundaryWriteRunner) Run(ctx context.Context, req upstream.Request) (upstream.Result, error) {
	res, err := r.Runner.Run(ctx, req)
	if err == nil && req.Bin == "flowmap" && req.Subcommand == "boundary" {
		_ = os.MkdirAll(filepath.Join(r.svcDir, ".flowmap"), 0o755)
		_ = os.WriteFile(filepath.Join(r.svcDir, ".flowmap", "boundary-contract.json"), r.branchContract, 0o644)
	}
	return res, err
}

func buildComputeRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				"loansvc/.flowmap.yaml":                   loansvcFlowmapYAML,
				"loansvc/.flowmap/boundary-contract.json": acceptanceBoundaryContractJSON,
			},
			Message: "accept: loansvc boundary contract baseline",
		},
	})
}

func testSpec(frozenCommit string) *artifact.SpecFrontmatter {
	return &artifact.SpecFrontmatter{
		Base: artifact.Base{
			ID:     "spec/stale-decline",
			Kind:   artifact.KindSpec,
			Title:  "t",
			Owners: []string{"platform-team"},
			Frozen: &artifact.Frozen{At: "2024-01-01", Commit: frozenCommit},
		},
		Class:   artifact.ClassFeature,
		Status:  "accepted-pending-build",
		Story:   "jira:LOAN-1482",
		Impacts: []string{"loansvc"},
		Declares: &artifact.Declares{
			Boundaries: []artifact.Boundary{
				{From: "loansvc", To: "notification-svc", Via: "events"},
				{From: "loansvc", To: "payments-svc", Via: "http"},
			},
		},
		AcceptanceCriteria: []artifact.AcceptanceCriterion{
			{ID: "ac-1", Text: "t", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}},
		},
	}
}

func seedComputeRunner(svcDir string) upstream.Runner {
	fr := upstream.NewFakeRunner()
	fr.Enqueue("flowmap", "graph", upstream.Result{Stdout: []byte("{}"), ExitCode: 0})
	fr.Enqueue("flowmap", "boundary", upstream.Result{ExitCode: 0})
	return boundaryWriteRunner{Runner: fr, svcDir: svcDir, branchContract: []byte(regeneratedBoundaryContractJSON)}
}

// TestCompute_ThreeValuedBoundaryDiff exercises all three declares.boundaries
// verdicts (holds/violated/undeclared) plus the acceptance-baseline diff, in
// one fixture: notification-svc(events) holds (present in both baseline and
// regenerated contracts' published:), payments-svc(http) is declared but
// never present (violated), and audit-svc(http) appears in the regenerated
// contract's consumed: with no declaring boundary (undeclared). The
// baseline diff sees the GET /healthz route and the audit-svc consumed
// entry, both additions (non-breaking).
func TestCompute_ThreeValuedBoundaryDiff(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	result, err := Compute(context.Background(), ComputedInput{
		Root:   repo.Dir,
		Runner: seedComputeRunner(svcDir),
		Spec:   spec,
		Covers: repo.Head,
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	if got := result.Impacted; len(got) != 1 || got[0] != "loansvc" {
		t.Fatalf("Impacted = %v, want [loansvc]", got)
	}

	if len(result.Findings) != 3 {
		t.Fatalf("Findings = %+v, want 3", result.Findings)
	}
	byID := make(map[string]artifact.Finding, len(result.Findings))
	for _, f := range result.Findings {
		byID[f.ID] = f
		if f.Kind != artifact.FindingComputed {
			t.Fatalf("finding %s: Kind = %q, want computed", f.ID, f.Kind)
		}
		if f.Dispositioned() {
			t.Fatalf("finding %s: freshly computed finding must be undispositioned, got %q", f.ID, f.Disposition)
		}
	}

	holds, ok := byID["boundary-loansvc-notification-svc-events"]
	if !ok || !strings.Contains(holds.Text, "holds") {
		t.Fatalf("holds finding missing or wrong: %+v (have %v)", holds, byID)
	}
	violated, ok := byID["boundary-loansvc-payments-svc-http"]
	if !ok || !strings.Contains(violated.Text, "VIOLATED") {
		t.Fatalf("violated finding missing or wrong: %+v", violated)
	}
	undeclared, ok := byID["boundary-loansvc-audit-svc-http"]
	if !ok || !strings.Contains(undeclared.Text, "UNDECLARED") {
		t.Fatalf("undeclared finding missing or wrong: %+v", undeclared)
	}

	if len(result.BaselineDiffs) != 1 {
		t.Fatalf("BaselineDiffs = %+v, want 1 entry", result.BaselineDiffs)
	}
	bd := result.BaselineDiffs[0]
	if bd.Skipped {
		t.Fatalf("BaselineDiffs[0] unexpectedly skipped: %+v", bd)
	}
	if bd.BaselineCommit != repo.Head {
		t.Fatalf("BaselineCommit = %q, want %q", bd.BaselineCommit, repo.Head)
	}
	if len(bd.Entries) != 2 {
		t.Fatalf("BaselineDiffs[0].Entries = %+v, want 2 additions", bd.Entries)
	}
	for _, e := range bd.Entries {
		if e.Op != upstream.DiffAdd || e.Breaking {
			t.Fatalf("entry %+v: want a non-breaking addition", e)
		}
	}
}

// TestCompute_BaselineSkippedWhenNoAcceptanceContract proves a service with
// no committed boundary contract at the acceptance commit (a service the
// build itself introduces) is Skipped, not an error ("where present").
func TestCompute_BaselineSkippedWhenNoAcceptanceContract(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{"loansvc/.flowmap.yaml": loansvcFlowmapYAML},
			Message: "no boundary contract committed yet",
		},
	})
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)
	spec.Declares = nil

	result, err := Compute(context.Background(), ComputedInput{
		Root:   repo.Dir,
		Runner: seedComputeRunner(svcDir),
		Spec:   spec,
		Covers: repo.Head,
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(result.BaselineDiffs) != 1 || !result.BaselineDiffs[0].Skipped {
		t.Fatalf("BaselineDiffs = %+v, want one skipped entry", result.BaselineDiffs)
	}
	// No declares.boundaries at all -> the two declared findings are gone;
	// audit-svc/notification-svc still surface as undeclared since the
	// regenerated contract's published/consumed arrays are unchanged from
	// the three-valued fixture's branchContract.
	if len(result.Findings) != 2 {
		t.Fatalf("Findings = %+v, want 2 undeclared-only findings", result.Findings)
	}
	for _, f := range result.Findings {
		if !strings.Contains(f.Text, "UNDECLARED") {
			t.Fatalf("finding %+v: want UNDECLARED (no declares.boundaries at all)", f)
		}
	}
}

// TestCompute_Negative covers the operational-precondition failures: no
// runner injected, no spec.
func TestCompute_Negative(t *testing.T) {
	repo := buildComputeRepo(t)

	t.Run("no runner", func(t *testing.T) {
		_, err := Compute(context.Background(), ComputedInput{Root: repo.Dir, Spec: testSpec(repo.Head), Covers: repo.Head})
		if err == nil {
			t.Fatal("Compute(no runner): want error, got nil")
		}
	})
	t.Run("no spec", func(t *testing.T) {
		_, err := Compute(context.Background(), ComputedInput{Root: repo.Dir, Runner: upstream.NewFakeRunner(), Covers: repo.Head})
		if err == nil {
			t.Fatal("Compute(no spec): want error, got nil")
		}
	})
}
