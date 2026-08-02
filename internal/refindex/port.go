package refindex

import (
	"context"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
)

// GitRunner is the ref-scoped git plumbing ComputeIndex depends on (dc-2,
// the 04 §port pattern) — a consumer-defined interface, not internal/gitx's
// free functions called directly, so ComputeIndex is unit-testable against
// an in-process fake with no real git process at all (see fake_test.go).
//
// Every method is a read against a ref, or a comparison between two refs.
// The method set contains NOTHING capable of moving HEAD or writing a
// working tree or index — no Checkout, no Switch, no generic
// Run(args ...string) escape hatch that could be handed an arbitrary git
// subcommand — so a checkout-mutating call is impossible to express
// through this interface, not merely undocumented or unused (ac-5's static
// guarantee, read directly off this method set).
type GitRunner interface {
	// DefaultBranch resolves dir's configured default branch short name
	// (gitx.DefaultBranch), returning ("", nil) — not an error — when
	// unconfigured.
	DefaultBranch(ctx context.Context, dir string) (string, error)
	// LocalDesignBranches lists dir's local refs/heads/design/* branch
	// short names (the "design/<name>" form), scoped from gitx.LocalBranches's
	// full refs/heads listing.
	LocalDesignBranches(ctx context.Context, dir string) ([]string, error)
	// RemoteDesignBranches lists dir's remote-tracking
	// refs/remotes/origin/design/* branch short names (also "design/<name>"
	// form, "origin/" stripped for direct comparability with
	// LocalDesignBranches — gitx.RemoteDesignBranches).
	RemoteDesignBranches(ctx context.Context, dir string) ([]string, error)
	// Show reads path's content as it existed at ref (gitx.Show).
	Show(ctx context.Context, dir, ref, path string) ([]byte, error)
	// ListTree recursively lists path's tracked files as they existed at
	// ref (gitx.LsTree). A path absent at ref returns an empty, nil-error
	// result; ref failing to resolve at all is a real error.
	ListTree(ctx context.Context, dir, ref, path string) ([]string, error)
	// IsAncestor reports whether ancestor is ref itself or a real ancestor
	// of ref (gitx.IsAncestor) — dc-5's merged-branch filter.
	IsAncestor(ctx context.Context, dir, ancestor, ref string) (bool, error)
}

// gitxRunner is the small adapter (dc-2) satisfying GitRunner over
// internal/gitx's existing free functions — LocalBranches, Show,
// DefaultBranch, IsAncestor (gitx/branch.go, gitx/worktree.go,
// gitx/show.go, gitx/ancestry.go) — plus the two new gitx primitives this
// story adds (RemoteDesignBranches, LsTree), each "more of the same shape"
// rather than invented ad hoc inside this package.
type gitxRunner struct{}

// NewGitRunner returns the production GitRunner: a thin adapter over
// internal/gitx, the only concrete git-executing dependency ComputeIndex's
// real callers (e.g. a future directory-home handler) need to construct.
func NewGitRunner() GitRunner { return gitxRunner{} }

func (gitxRunner) DefaultBranch(ctx context.Context, dir string) (string, error) {
	return gitx.DefaultBranch(ctx, dir)
}

// designPrefix is the branch-namespace prefix `verdi design start` cuts
// every design branch under (cmd/verdi/design.go: `branch := "design/" +
// name`) — the filter LocalDesignBranches applies to gitx.LocalBranches's
// full refs/heads listing, since gitx grows no new local-listing primitive
// for this story (dc-2: only a remote-tracking for-each-ref query and a
// tree-listing are new gitx plumbing).
const designPrefix = "design/"

func (gitxRunner) LocalDesignBranches(ctx context.Context, dir string) ([]string, error) {
	all, err := gitx.LocalBranches(ctx, dir)
	if err != nil {
		return nil, err
	}
	var design []string
	for _, b := range all {
		if strings.HasPrefix(b, designPrefix) {
			design = append(design, b)
		}
	}
	return design, nil
}

func (gitxRunner) RemoteDesignBranches(ctx context.Context, dir string) ([]string, error) {
	return gitx.RemoteDesignBranches(ctx, dir)
}

func (gitxRunner) Show(ctx context.Context, dir, ref, path string) ([]byte, error) {
	return gitx.Show(ctx, dir, ref, path)
}

func (gitxRunner) ListTree(ctx context.Context, dir, ref, path string) ([]string, error) {
	return gitx.LsTree(ctx, dir, ref, path)
}

func (gitxRunner) IsAncestor(ctx context.Context, dir, ancestor, ref string) (bool, error) {
	return gitx.IsAncestor(ctx, dir, ancestor, ref)
}

// StateResolver is the consumer-defined port (dc-2, the 04 §port pattern)
// ComputeIndex resolves each feature/story entry's git-derived effective
// lifecycle state through — internal/specstate.Projector's own exported
// method set, narrowed to exactly what ComputeIndex needs, so refindex
// depends on an interface it defines, never specstate.Projector's concrete
// type directly. "No adapter reimplements reachability" (specstate's own
// package doc): ComputeIndex never re-derives accepted/superseded/closed
// from a raw `status:` field itself — every such decision routes through
// this port. ResolveMany is the batch entry point ComputeIndex actually
// uses (Step 2: "resolves the collected candidates as one batch") so a
// corpus-sized default-branch walk triggers ONE default-corpus scan, never
// one per candidate; Resolve is included for interface completeness and
// direct single-candidate use (e.g. a future caller, or a resolver's own
// tests) but ComputeIndex itself never calls it.
type StateResolver interface {
	Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error)
	ResolveMany(ctx context.Context, root string, candidates []specstate.Candidate) ([]specstate.Result, error)
}

// specstateResolver adapts specstate.Projector to StateResolver — the
// production implementation (dc-2), mirroring gitxRunner's identical role
// for GitRunner.
type specstateResolver struct{ p specstate.Projector }

// NewStateResolver returns the production StateResolver: a thin adapter
// over specstate.NewProjector(), the ONE place lifecycle state is derived
// (specstate's own package doc). ComputeIndex's real callers construct this
// exactly once per call, alongside NewGitRunner().
func NewStateResolver() StateResolver { return specstateResolver{p: specstate.NewProjector()} }

func (r specstateResolver) Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error) {
	return r.p.Resolve(ctx, root, candidate)
}

func (r specstateResolver) ResolveMany(ctx context.Context, root string, candidates []specstate.Candidate) ([]specstate.Result, error) {
	return r.p.ResolveMany(ctx, root, candidates)
}
