package journey

import (
	"context"
	"errors"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/repositoryfacts"
	"github.com/jyang234/verdi/internal/specstate"
)

// fakeGitReader is the in-process GitReader double every fact-gathering
// unit test proves its behavior against, with no real git process at all
// — mirroring internal/refindex/fake_test.go's fakeGitRunner convention. A
// nil func field panics if called, so a test that forgets to wire a
// dependency fails loudly.
type fakeGitReader struct {
	revParseFn                func(ctx context.Context, dir, rev string) (string, error)
	currentBranchFn           func(ctx context.Context, dir string) (string, error)
	remoteURLFn               func(ctx context.Context, dir, name string) (string, error)
	statusDirtyFn             func(ctx context.Context, dir string) (bool, error)
	stagedPathsFn             func(ctx context.Context, dir string) ([]string, error)
	showFn                    func(ctx context.Context, dir, ref, path string) ([]byte, error)
	isAncestorFn              func(ctx context.Context, dir, ancestor, ref string) (bool, error)
	hasLocalBranchFn          func(ctx context.Context, dir, name string) (bool, error)
	hasRemoteTrackingBranchFn func(ctx context.Context, dir, remote, branch string) (bool, error)
}

func (f *fakeGitReader) RevParse(ctx context.Context, dir, rev string) (string, error) {
	return f.revParseFn(ctx, dir, rev)
}

func (f *fakeGitReader) CurrentBranch(ctx context.Context, dir string) (string, error) {
	return f.currentBranchFn(ctx, dir)
}

func (f *fakeGitReader) RemoteURL(ctx context.Context, dir, name string) (string, error) {
	return f.remoteURLFn(ctx, dir, name)
}

func (f *fakeGitReader) StatusDirty(ctx context.Context, dir string) (bool, error) {
	return f.statusDirtyFn(ctx, dir)
}

func (f *fakeGitReader) StagedPaths(ctx context.Context, dir string) ([]string, error) {
	return f.stagedPathsFn(ctx, dir)
}

func (f *fakeGitReader) Show(ctx context.Context, dir, ref, path string) ([]byte, error) {
	return f.showFn(ctx, dir, ref, path)
}

func (f *fakeGitReader) IsAncestor(ctx context.Context, dir, ancestor, ref string) (bool, error) {
	return f.isAncestorFn(ctx, dir, ancestor, ref)
}

func (f *fakeGitReader) HasLocalBranch(ctx context.Context, dir, name string) (bool, error) {
	return f.hasLocalBranchFn(ctx, dir, name)
}

func (f *fakeGitReader) HasRemoteTrackingBranch(ctx context.Context, dir, remote, branch string) (bool, error) {
	return f.hasRemoteTrackingBranchFn(ctx, dir, remote, branch)
}

var _ GitReader = (*fakeGitReader)(nil)

// fakeStateResolver is the in-process StateResolver double, mirroring
// internal/refindex/fake_test.go's fakeStateResolver convention.
type fakeStateResolver struct {
	resolveFn func(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error)
}

func (f *fakeStateResolver) Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error) {
	return f.resolveFn(ctx, root, candidate)
}

var _ StateResolver = (*fakeStateResolver)(nil)

// fakeRepositoryFactsGatherer is the in-process RepositoryFactsGatherer
// double journey's own tests substitute for the real
// internal/repositoryfacts leaf, mirroring fakeGitReader/
// fakeStateResolver's identical convention.
type fakeRepositoryFactsGatherer struct {
	gatherFn func(ctx context.Context, in repositoryfacts.GatherInput) (repositoryfacts.Snapshot, error)
}

func (f *fakeRepositoryFactsGatherer) Gather(ctx context.Context, in repositoryfacts.GatherInput) (repositoryfacts.Snapshot, error) {
	return f.gatherFn(ctx, in)
}

var _ RepositoryFactsGatherer = (*fakeRepositoryFactsGatherer)(nil)

// noOpRepositoryFactsGatherer returns a fake that panics if Gather is
// called with no fn wired — the same "fails loudly if forgotten"
// convention as noOpGitReader, for the majority of tests that exercise
// target/lifecycle-fact logic without ever reaching gatherRepositoryFacts.
func noOpRepositoryFactsGatherer() *fakeRepositoryFactsGatherer {
	return &fakeRepositoryFactsGatherer{
		gatherFn: func(context.Context, repositoryfacts.GatherInput) (repositoryfacts.Snapshot, error) {
			panic("journey: fake repository-facts gatherer called with no fn wired")
		},
	}
}

// fakeStubReconciler is the in-process StubReconciler double, mirroring
// fakeGitReader/fakeStateResolver's identical convention.
type fakeStubReconciler struct {
	reconcileFn func(ctx context.Context, root, commit string, spec *artifact.SpecFrontmatter, mdl *model.Model) (evidence.StubReconciliation, error)
}

func (f *fakeStubReconciler) Reconcile(ctx context.Context, root, commit string, spec *artifact.SpecFrontmatter, mdl *model.Model) (evidence.StubReconciliation, error) {
	return f.reconcileFn(ctx, root, commit, spec, mdl)
}

var _ StubReconciler = (*fakeStubReconciler)(nil)

// noOpStubReconciler returns a fake whose Reconcile returns a benign,
// fixed error rather than panicking — unlike noOpGitReader/
// noOpRepositoryFactsGatherer's panic-on-call convention. Those two ports
// are exercised for EVERY target this package projects; StubReconciler is
// exercised only for a feature-class target (GatherFacts's own class
// gate), so most of this file's existing fact-gathering tests reach it
// incidentally, never intentionally. gatherEventualFeatureFacts already
// treats a Reconcile error as an ordinary, disclosed absence — never a
// hard failure — so this fake's error simply becomes one more
// EventualDisclosures entry those tests do not examine. A test that
// specifically exercises stub-reconciliation wiring supplies its own
// reconcileFn instead of this fake.
func noOpStubReconciler() *fakeStubReconciler {
	return &fakeStubReconciler{
		reconcileFn: func(context.Context, string, string, *artifact.SpecFrontmatter, *model.Model) (evidence.StubReconciliation, error) {
			return evidence.StubReconciliation{}, errors.New("journey: fake stub reconciler has no reconcileFn wired for this test")
		},
	}
}

// fakeFeatureFolder is the in-process FeatureFolder double.
type fakeFeatureFolder struct {
	foldFn func(ctx context.Context, root, commit string, spec *artifact.SpecFrontmatter, mdl *model.Model) (evidence.FeatureResult, error)
}

func (f *fakeFeatureFolder) Fold(ctx context.Context, root, commit string, spec *artifact.SpecFrontmatter, mdl *model.Model) (evidence.FeatureResult, error) {
	return f.foldFn(ctx, root, commit, spec, mdl)
}

var _ FeatureFolder = (*fakeFeatureFolder)(nil)

// noOpFeatureFolder returns a fake whose Fold returns a benign, fixed
// error rather than panicking — same reasoning as noOpStubReconciler
// (FeatureFolder is exercised only for a feature-class target, and its
// error becomes an ordinary, disclosed absence, never a hard failure).
func noOpFeatureFolder() *fakeFeatureFolder {
	return &fakeFeatureFolder{
		foldFn: func(context.Context, string, string, *artifact.SpecFrontmatter, *model.Model) (evidence.FeatureResult, error) {
			return evidence.FeatureResult{}, errors.New("journey: fake feature folder has no foldFn wired for this test")
		},
	}
}

// noOpGitReader satisfies GitReader with functions that panic if called —
// a base to override individual fields from in a table-driven test that
// only cares about a subset of behavior.
func noOpGitReader() *fakeGitReader {
	panicMsg := "journey: fake git reader method called with no fn wired"
	return &fakeGitReader{
		revParseFn:                func(context.Context, string, string) (string, error) { panic(panicMsg) },
		currentBranchFn:           func(context.Context, string) (string, error) { panic(panicMsg) },
		remoteURLFn:               func(context.Context, string, string) (string, error) { panic(panicMsg) },
		statusDirtyFn:             func(context.Context, string) (bool, error) { panic(panicMsg) },
		stagedPathsFn:             func(context.Context, string) ([]string, error) { panic(panicMsg) },
		showFn:                    func(context.Context, string, string, string) ([]byte, error) { panic(panicMsg) },
		isAncestorFn:              func(context.Context, string, string, string) (bool, error) { panic(panicMsg) },
		hasLocalBranchFn:          func(context.Context, string, string) (bool, error) { panic(panicMsg) },
		hasRemoteTrackingBranchFn: func(context.Context, string, string, string) (bool, error) { panic(panicMsg) },
	}
}
