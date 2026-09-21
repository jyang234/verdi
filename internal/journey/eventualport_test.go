package journey

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// --- co-6: a port error is a disclosure, never a projection failure -------

// failingPortProjector returns a REAL projector (real git, state, profile
// and obligation-quality machinery) whose two eventual-source ports are
// replaced by fakes returning err — the exact shape co-6 names: the
// gathering step fails, the projection does not.
func failingPortProjector(reconcileErr, foldErr func(root string) error) Projector {
	p := NewProjector()
	p.stubs = &fakeStubReconciler{
		reconcileFn: func(_ context.Context, root, _ string, _ *artifact.SpecFrontmatter, _ *model.Model) (evidence.StubReconciliation, error) {
			return evidence.StubReconciliation{}, reconcileErr(root)
		},
	}
	p.folder = &fakeFeatureFolder{
		foldFn: func(_ context.Context, root, _ string, _ *artifact.SpecFrontmatter, _ *model.Model) (evidence.FeatureResult, error) {
			return evidence.FeatureResult{}, foldErr(root)
		},
	}
	return p
}

// TestProject_PortErrorsBecomeDisclosures is co-6's proof: a Reconcile
// error and a Fold error each become a disclosure — in
// Facts.EventualDisclosures and in the projected record's own eventual
// section — and neither is ever a projection failure. The feature-only
// sources simply derive nothing, and the record still validates and
// round-trips.
func TestProject_PortErrorsBecomeDisclosures(t *testing.T) {
	repo := buildEventualFixtureRepo(t)
	cfg := openConfig(t, repo.Dir)
	p := failingPortProjector(
		func(string) error { return fmt.Errorf("reconcile exploded") },
		func(string) error { return fmt.Errorf("fold exploded") },
	)

	facts, err := p.GatherFacts(context.Background(), cfg, "spec/checkout")
	if err != nil {
		t.Fatalf("GatherFacts: %v", err)
	}
	if facts.Stubs != nil || facts.FeatureFold != nil {
		t.Fatalf("Stubs = %v, FeatureFold = %v, want both nil when their ports errored", facts.Stubs, facts.FeatureFold)
	}
	for _, want := range []string{"reconcile exploded", "fold exploded"} {
		if !containsSubstring(facts.EventualDisclosures, want) {
			t.Fatalf("EventualDisclosures = %v, want one naming %q", facts.EventualDisclosures, want)
		}
	}
	if len(facts.EventualDisclosures) != 2 {
		t.Fatalf("EventualDisclosures = %v, want exactly the two port errors", facts.EventualDisclosures)
	}

	rec, err := p.Project(context.Background(), cfg, "spec/checkout")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if !rec.Blockers.Eventual.Derived {
		t.Fatal("Derived = false: an unavailable source is disclosed, not presented as an underived section")
	}
	for _, want := range []string{"reconcile exploded", "fold exploded"} {
		if !containsSubstring(rec.Blockers.Eventual.Disclosures, want) {
			t.Fatalf("record disclosures = %v, want one naming %q", rec.Blockers.Eventual.Disclosures, want)
		}
	}
	for _, b := range rec.Blockers.Eventual.Items {
		for _, prefix := range []string{"stub-unreconciled/", "outcome-floor/"} {
			if strings.HasPrefix(b.ID, prefix) {
				t.Fatalf("items = %v, a source whose port errored must derive nothing", blockerIDs(rec.Blockers.Eventual.Items))
			}
		}
	}
	if _, err := Canonical(rec); err != nil {
		t.Fatalf("Canonical: %v", err)
	}
}

// TestProject_PortFactsReachTheRecord is the positive half of the same
// seam: values supplied THROUGH the ports (not read from the store) reach
// the derived items, so the ports are a real substitution point and not
// dead weight.
func TestProject_PortFactsReachTheRecord(t *testing.T) {
	repo := buildEventualFixtureRepo(t)
	cfg := openConfig(t, repo.Dir)
	p := NewProjector()
	p.stubs = &fakeStubReconciler{
		reconcileFn: func(context.Context, string, string, *artifact.SpecFrontmatter, *model.Model) (evidence.StubReconciliation, error) {
			return evidence.StubReconciliation{Stubs: []evidence.StubResult{{Slug: "port-supplied", Bucket: evidence.StubUnreconciled}}}, nil
		},
	}
	p.folder = &fakeFeatureFolder{
		foldFn: func(context.Context, string, string, *artifact.SpecFrontmatter, *model.Model) (evidence.FeatureResult, error) {
			return evidence.FeatureResult{SpecRef: "spec/checkout", ACs: []evidence.FeatureACResult{{ID: "ac-9", Floor: evidence.FloorResult{Satisfied: false}}}}, nil
		},
	}

	rec, err := p.Project(context.Background(), cfg, "spec/checkout")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	for _, want := range []string{"stub-unreconciled/port-supplied", "outcome-floor/ac-9"} {
		if findBlocker(rec.Blockers.Eventual.Items, want) == nil {
			t.Fatalf("items = %v, want %q", blockerIDs(rec.Blockers.Eventual.Items), want)
		}
	}
	if len(rec.Blockers.Eventual.Disclosures) != 1 || !strings.Contains(rec.Blockers.Eventual.Disclosures[0], "no policy-conflict report") {
		t.Fatalf("disclosures = %v, want only the no-report disclosure", rec.Blockers.Eventual.Disclosures)
	}
}

// TestGatherFacts_EventualDisclosuresAreRootIndependent is I4: a port
// error carries this process's own ABSOLUTE store path (internal/index's
// "index: walking <root>/.verdi", internal/evidence's "evidence: reading
// <derivedRoot>"), and that text flows into the record and its digest.
// Two checkouts of the same store at different paths must still derive
// identical bytes (the plan's line 21, CO-2/CO-4), so every eventual
// disclosure routes through sanitizeDisclosures exactly as the lifecycle
// disclosures already do.
func TestGatherFacts_EventualDisclosuresAreRootIndependent(t *testing.T) {
	gather := func(t *testing.T) (root string, disclosures []string) {
		t.Helper()
		repo := buildEventualFixtureRepo(t)
		cfg := openConfig(t, repo.Dir)
		p := failingPortProjector(
			func(root string) error { return fmt.Errorf("index: walking %s/.verdi: permission denied", root) },
			func(root string) error {
				return fmt.Errorf("evidence: reading %s: permission denied", store.DerivedSpecDir(root, store.RefSlug("spec/checkout")))
			},
		)
		facts, err := p.GatherFacts(context.Background(), cfg, "spec/checkout")
		if err != nil {
			t.Fatalf("GatherFacts: %v", err)
		}
		return cfg.Root, facts.EventualDisclosures
	}

	rootA, first := gather(t)
	rootB, second := gather(t)
	if rootA == rootB {
		t.Fatalf("both evaluations used the same root %q; the test proves nothing", rootA)
	}
	for i, ds := range [][]string{first, second} {
		for _, d := range ds {
			if strings.Contains(d, rootA) || strings.Contains(d, rootB) {
				t.Fatalf("evaluation %d disclosure %q embeds an absolute store root", i, d)
			}
			if !strings.Contains(d, "<store-root>") {
				t.Fatalf("evaluation %d disclosure %q does not carry the sanitized token", i, d)
			}
		}
	}
	if strings.Join(first, "\n") != strings.Join(second, "\n") {
		t.Fatalf("disclosures differ across roots:\n%v\n%v", first, second)
	}
}

// --- the production adapters' error returns -------------------------------

// unparseableRefSpec is a feature frontmatter whose ID is not a ref at
// all — artifact.ParseRef's own refusal, the first error return in both
// adapters.
func unparseableRefSpec() *artifact.SpecFrontmatter {
	return &artifact.SpecFrontmatter{Base: artifact.Base{ID: "nonsense"}, Class: artifact.ClassFeature}
}

func checkoutSpec() *artifact.SpecFrontmatter {
	return &artifact.SpecFrontmatter{
		Base:               artifact.Base{ID: "spec/checkout"},
		Class:              artifact.ClassFeature,
		AcceptanceCriteria: []artifact.AcceptanceCriterion{{ID: "ac-1", Text: "x", Evidence: []artifact.EvidenceKind{"static"}}},
	}
}

// ghostImplementerStoreRoot writes a store whose index carries an
// implements edge from a story spec whose declared ID names a spec no
// zone holds ("spec/missing-story" while the file sits under ghost/), so
// matrixprojection.DiscoverImplementingStories fails resolving the
// implementer — a discovery error with no git and no network.
func ghostImplementerStoreRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeSpec(t, root, store.ZoneActive, "checkout", `---
id: spec/checkout
kind: spec
class: feature
title: "Checkout"
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "the fixture outcome holds", evidence: [static] }
---
# Checkout
`)
	writeSpec(t, root, store.ZoneActive, "ghost", `---
id: spec/missing-story
kind: spec
class: story
title: "Ghost story"
owners: [platform-team]
story: jira:GHOST-1
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/checkout#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the story's own obligation holds", evidence: [static] }
---
# Ghost story
`)
	return root
}

// loneFeatureStoreRoot writes a store holding one feature and nothing
// that implements it: index.Build and discovery both succeed, so the next
// error return in featureFolder.Fold is reachable.
func loneFeatureStoreRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeSpec(t, root, store.ZoneActive, "checkout", `---
id: spec/checkout
kind: spec
class: feature
title: "Checkout"
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "the fixture outcome holds", evidence: [static] }
---
# Checkout
`)
	return root
}

// TestStubReconciler_ErrorPaths covers every error return of the
// production StubReconciler adapter with a hermetic root — no git, no
// network (CLAUDE.md's negative-path rule).
func TestStubReconciler_ErrorPaths(t *testing.T) {
	tests := []struct {
		name string
		root func(t *testing.T) string
		spec *artifact.SpecFrontmatter
		want string
	}{
		{
			name: "unparseable spec id",
			root: func(t *testing.T) string { return t.TempDir() },
			spec: unparseableRefSpec(),
			want: "parsing spec id",
		},
		{
			name: "index build fails on a root with no store at all",
			root: func(t *testing.T) string { return t.TempDir() },
			spec: checkoutSpec(),
			want: "building index",
		},
		{
			name: "implementer discovery fails",
			root: ghostImplementerStoreRoot,
			spec: checkoutSpec(),
			want: "discovering implementing stories",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStubReconciler().Reconcile(context.Background(), tt.root(t), "", tt.spec, model.Canonical())
			if err == nil {
				t.Fatal("Reconcile: want an error")
			}
			if !strings.Contains(err.Error(), tt.want) || !strings.HasPrefix(err.Error(), "journey: ") {
				t.Fatalf("Reconcile error = %q, want a journey-prefixed error naming %q", err, tt.want)
			}
		})
	}
}

// TestFeatureFolder_ErrorPaths covers every error return of the
// production FeatureFolder adapter, including the record-loading failure
// the stub reconciler has no counterpart for.
func TestFeatureFolder_ErrorPaths(t *testing.T) {
	// A regular FILE where the derived-evidence DIRECTORY belongs, so
	// evidence.LoadRecords' own os.ReadDir fails with something other
	// than "does not exist" (which it deliberately tolerates).
	fileInsteadOfDerivedDir := func(t *testing.T) string {
		t.Helper()
		root := loneFeatureStoreRoot(t)
		derived := store.DerivedSpecDir(root, store.RefSlug("spec/checkout"))
		if err := os.MkdirAll(filepath.Dir(derived), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(derived, []byte("not a directory\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		return root
	}

	tests := []struct {
		name string
		root func(t *testing.T) string
		spec *artifact.SpecFrontmatter
		want string
	}{
		{
			name: "unparseable spec id",
			root: func(t *testing.T) string { return t.TempDir() },
			spec: unparseableRefSpec(),
			want: "parsing spec id",
		},
		{
			name: "index build fails on a root with no store at all",
			root: func(t *testing.T) string { return t.TempDir() },
			spec: checkoutSpec(),
			want: "building index",
		},
		{
			name: "implementer discovery fails",
			root: ghostImplementerStoreRoot,
			spec: checkoutSpec(),
			want: "discovering implementing stories",
		},
		{
			name: "loading the feature's evidence records fails",
			root: fileInsteadOfDerivedDir,
			spec: checkoutSpec(),
			want: "loading feature evidence records",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFeatureFolder().Fold(context.Background(), tt.root(t), "", tt.spec, model.Canonical())
			if err == nil {
				t.Fatal("Fold: want an error")
			}
			if !strings.Contains(err.Error(), tt.want) || !strings.HasPrefix(err.Error(), "journey: ") {
				t.Fatalf("Fold error = %q, want a journey-prefixed error naming %q", err, tt.want)
			}
		})
	}
}

// containsSubstring reports whether any element of ss contains want.
func containsSubstring(ss []string, want string) bool {
	for _, s := range ss {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}
