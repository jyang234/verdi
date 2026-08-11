package repositoryfacts

import (
	"context"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
)

// GitReader is the read-only Git-plumbing surface Gather depends on (the
// 04 §port pattern, mirroring internal/journey/port.go's own GitReader —
// this package is that interface's SI-85 extraction origin): a
// consumer-owned interface, not internal/gitx's free functions called
// directly, so Gather is unit-testable against an in-process fake with no
// real git process at all.
//
// The method set is narrowed to exactly what Gather itself calls: no
// HasLocalBranch/HasRemoteTrackingBranch (lifecycle-branch lookups are a
// caller's own concern; this leaf extracts only the repository-fact
// surface, not journey's whole GitReader). It contains nothing capable of
// moving HEAD, writing the index, or touching the working tree — no
// Checkout, no generic Run(args ...string) escape hatch — so Gather is
// read-only by construction.
type GitReader interface {
	// RevParse resolves rev to its object id (gitx.RevParse).
	RevParse(ctx context.Context, dir, rev string) (string, error)
	// CurrentBranch returns dir's checked-out branch short name, or ("",
	// nil) for a detached HEAD (gitx.CurrentBranch's own documented
	// contract — not an error).
	CurrentBranch(ctx context.Context, dir string) (string, error)
	// RemoteURL returns the URL configured for remote name (gitx.RemoteURL).
	// A genuinely-absent remote is gitx.ErrNoSuchRemote (errors.Is-able).
	RemoteURL(ctx context.Context, dir, name string) (string, error)
	// StatusDirty reports whether dir's working tree has any uncommitted
	// change (gitx.StatusDirty).
	StatusDirty(ctx context.Context, dir string) (bool, error)
	// StagedPaths returns the repository-relative paths whose index
	// entries differ from HEAD (gitx.StagedPaths).
	StagedPaths(ctx context.Context, dir string) ([]string, error)
	// Show reads path's content as it existed at ref (gitx.Show).
	Show(ctx context.Context, dir, ref, path string) ([]byte, error)
	// IsAncestor reports whether ancestor is ref itself or a real ancestor
	// of ref (gitx.IsAncestor).
	IsAncestor(ctx context.Context, dir, ancestor, ref string) (bool, error)
}

// gitxReader adapts internal/gitx's free functions to GitReader — the
// production implementation.
type gitxReader struct{}

// NewGitReader returns the production GitReader: a thin adapter over
// internal/gitx, execed against the process's system git.
func NewGitReader() GitReader { return gitxReader{} }

func (gitxReader) RevParse(ctx context.Context, dir, rev string) (string, error) {
	return gitx.RevParse(ctx, dir, rev)
}

func (gitxReader) CurrentBranch(ctx context.Context, dir string) (string, error) {
	return gitx.CurrentBranch(ctx, dir)
}

func (gitxReader) RemoteURL(ctx context.Context, dir, name string) (string, error) {
	return gitx.RemoteURL(ctx, dir, name)
}

func (gitxReader) StatusDirty(ctx context.Context, dir string) (bool, error) {
	return gitx.StatusDirty(ctx, dir)
}

func (gitxReader) StagedPaths(ctx context.Context, dir string) ([]string, error) {
	return gitx.StagedPaths(ctx, dir)
}

func (gitxReader) Show(ctx context.Context, dir, ref, path string) ([]byte, error) {
	return gitx.Show(ctx, dir, ref, path)
}

func (gitxReader) IsAncestor(ctx context.Context, dir, ancestor, ref string) (bool, error) {
	return gitx.IsAncestor(ctx, dir, ancestor, ref)
}

// DefaultBranchResolver is the func-value shape of
// specstate.ResolveDefaultBranch: NewGatherer wires this field directly
// to that function; this package's own tests substitute a fake func —
// the "production func value, fakeable" seam, without a named interface
// for a single free function (mirrors internal/journey/port.go's
// identically shaped DefaultBranchResolver).
type DefaultBranchResolver func(ctx context.Context, root string) (specstate.Branch, bool)
