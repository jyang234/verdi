package journey

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/store"
)

// currentHead and porcelainStatus mirror cmd/verdi/specstate_test.go's
// identically-named helpers (that package is `main`; this one cannot
// import it, so the small idiom is copied, not shared).
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

// buildFactsRepo mirrors cmd/verdi/specstate_test.go's buildSpecStateRepo:
// a minimal .verdi/verdi.yaml scaffold plus caller-supplied files, with
// CI_DEFAULT_BRANCH pinned so default-branch resolution is hermetic (no
// network, no dependence on an "origin" remote or symbolic-ref config).
func buildFactsRepo(t *testing.T, files map[string]string) *fixturegit.Repo {
	t.Helper()
	base := map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n"}
	for k, v := range files {
		base[k] = v
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: base, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	t.Chdir(repo.Dir)
	return repo
}

// TestGatherFacts_Integration_LandedSpec proves the production adapters
// (real git, real internal/specstate, real specstate.ResolveDefaultBranch)
// end to end over a fixturegit-backed repository: a statusless spec whose
// exact bytes are landed on the default branch resolves Source == "head",
// Dirty == false, DefaultBranch facts populated, and lifecycle State ==
// "accepted-pending-build" — the same landed-spec shape
// cmd/verdi/specstate_test.go's TestCmdSpecState_ExactAcceptedPendingBuild
// proves for the CLI's own read-only surface.
func TestGatherFacts_Integration_LandedSpec(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})

	cfg, err := store.Open(repo.Dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	cfg.Root = repo.Dir

	p := NewProjector()
	facts, err := p.GatherFacts(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("GatherFacts: %v", err)
	}

	if facts.Repository.Source != "head" {
		t.Fatalf("Source = %q, want %q", facts.Repository.Source, "head")
	}
	if !facts.Repository.Dirty.Known || facts.Repository.Dirty.Value {
		t.Fatalf("Dirty = %+v, want known/false", facts.Repository.Dirty)
	}
	if !facts.Repository.Staged.Known || facts.Repository.Staged.Value {
		t.Fatalf("Staged = %+v, want known/false", facts.Repository.Staged)
	}
	if !facts.Repository.Head.Known || facts.Repository.Head.Value != repo.Head {
		t.Fatalf("Head = %+v, want known == %s", facts.Repository.Head, repo.Head)
	}
	if !facts.Repository.DefaultBranch.Known || facts.Repository.DefaultBranch.Name != "main" {
		t.Fatalf("DefaultBranch = %+v, want known name=main", facts.Repository.DefaultBranch)
	}
	if facts.Repository.DefaultBranch.Head != repo.Head {
		t.Fatalf("DefaultBranch.Head = %q, want %q", facts.Repository.DefaultBranch.Head, repo.Head)
	}
	if facts.Repository.Relationship != "equal" {
		t.Fatalf("Relationship = %q, want %q (HEAD == default branch HEAD in this fixture)", facts.Repository.Relationship, "equal")
	}
	if facts.Repository.RemoteOrigin.Known {
		t.Fatalf("RemoteOrigin = %+v, want unknown (fixturegit repos carry no origin remote)", facts.Repository.RemoteOrigin)
	}
	if !containsString(facts.RepositoryDisclosures, "no origin remote is configured") {
		t.Fatalf("RepositoryDisclosures = %v, want the no-origin-remote disclosure", facts.RepositoryDisclosures)
	}

	if facts.Lifecycle.State != "accepted-pending-build" {
		t.Fatalf("Lifecycle.State = %q, want accepted-pending-build", facts.Lifecycle.State)
	}
	if facts.Lifecycle.Relation != "exact" {
		t.Fatalf("Lifecycle.Relation = %q, want exact", facts.Lifecycle.Relation)
	}
	if facts.Lifecycle.Posture != "authoritative" {
		t.Fatalf("Lifecycle.Posture = %q, want authoritative", facts.Lifecycle.Posture)
	}
	if facts.Lifecycle.AcceptedBaseline == nil || facts.Lifecycle.AcceptedBaseline.Path != ".verdi/specs/active/payments/spec.md" {
		t.Fatalf("AcceptedBaseline = %+v", facts.Lifecycle.AcceptedBaseline)
	}
}

// TestGatherFacts_Integration_ReadOnly proves GatherFacts changes neither
// HEAD nor the working tree's status (copying the idiom at
// cmd/verdi/specstate_test.go:56-70 — currentHead/porcelainStatus before
// and after) — the read-only-by-construction guarantee GitReader's own doc
// comment claims statically, proven dynamically here against the real git
// adapter.
func TestGatherFacts_Integration_ReadOnly(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})

	cfg, err := store.Open(repo.Dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	cfg.Root = repo.Dir

	headBefore := currentHead(t, repo.Dir)
	statusBefore := porcelainStatus(t, repo.Dir)

	p := NewProjector()
	if _, err := p.GatherFacts(context.Background(), cfg, "spec/payments"); err != nil {
		t.Fatalf("GatherFacts: %v", err)
	}

	if headBefore != currentHead(t, repo.Dir) {
		t.Fatalf("HEAD changed: before=%s after=%s", headBefore, currentHead(t, repo.Dir))
	}
	if statusBefore != porcelainStatus(t, repo.Dir) {
		t.Fatalf("git status --porcelain changed: before=%q after=%q", statusBefore, porcelainStatus(t, repo.Dir))
	}
}

// TestGatherFacts_Integration_Dirty proves a real working-tree edit is
// observed as Dirty == true and Source == "working-tree" (the edited spec
// no longer matches what HEAD holds at the same path).
func TestGatherFacts_Integration_Dirty(t *testing.T) {
	repo := buildFactsRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": testFeatureSpecMD})

	cfg, err := store.Open(repo.Dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	cfg.Root = repo.Dir

	edited := testFeatureSpecMD + "\n<!-- local edit -->\n"
	if err := os.WriteFile(repo.Dir+"/.verdi/specs/active/payments/spec.md", []byte(edited), 0o644); err != nil {
		t.Fatalf("editing spec.md: %v", err)
	}

	p := NewProjector()
	facts, err := p.GatherFacts(context.Background(), cfg, "spec/payments")
	if err != nil {
		t.Fatalf("GatherFacts: %v", err)
	}
	if !facts.Repository.Dirty.Known || !facts.Repository.Dirty.Value {
		t.Fatalf("Dirty = %+v, want known/true", facts.Repository.Dirty)
	}
	if facts.Repository.Source != "working-tree" {
		t.Fatalf("Source = %q, want working-tree", facts.Repository.Source)
	}
}
