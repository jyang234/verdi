package journey

import (
	"context"

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
