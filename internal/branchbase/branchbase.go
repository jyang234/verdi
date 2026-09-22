// Package branchbase holds the ONE read-only "what base does a fresh
// ritual branch cut from" resolution rule (dc-7/I-130), shared so
// cmd/verdi's design start and policy adopt and internal/recovery's
// fact-gathering (R-RR3-5) can never diverge (CLAUDE.md: "anything used
// by two or more packages lives in a shared internal/ package").
//
// It is a pure move of cmd/verdi's own resolveBranchBase's resolution
// logic (cmd/verdi/design.go), split from that function's own disclosure
// printing: this package returns structured facts only and performs no
// I/O beyond the read-only git/store calls needed to compute them, and
// never prints anything.
package branchbase

import (
	"context"
	"errors"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
)

// Kind is Resolve's closed set of read-only outcomes (dc-7/I-130's own
// three cases).
type Kind int

const (
	// ResolvedDefault: the default branch resolved to a real,
	// git-resolvable ref (specstate.ResolveDefaultBranch succeeded).
	ResolvedDefault Kind = iota
	// HeadFallback: no origin remote is configured at all; the disclosed
	// fallback is the current HEAD, never a default-branch base.
	HeadFallback
	// Unresolvable: origin IS configured but its default branch could not
	// be resolved (no CI_DEFAULT_BRANCH, no origin/HEAD, or an ambiguous
	// local main/master fallback) — an operational refusal in cmd/verdi,
	// and an undecidable return branch (R-RR3-5) in internal/recovery.
	Unresolvable
)

// Resolution is Resolve's read-only answer.
type Resolution struct {
	Kind Kind
	// Ref is the base ref a fresh branch would cut from: the resolved
	// default branch's own ref (ResolvedDefault) or the literal "HEAD"
	// (HeadFallback). Empty for Unresolvable.
	Ref string
	// BranchName is the resolved default branch's short name (e.g.
	// "main"), set only for ResolvedDefault.
	BranchName string
	// Commit is Ref's resolved commit sha, set for ResolvedDefault and
	// HeadFallback.
	Commit string
}

// Resolve implements cmd/verdi's own resolveBranchBase resolution rule
// (dc-7/I-130): first specstate.ResolveDefaultBranch; when that fails,
// distinguish "no origin remote at all" (the disclosed HEAD fallback)
// from "origin exists but the default branch is unresolvable or
// ambiguous" (Unresolvable) by reading whether "origin" itself is
// configured. A read failure that is not gitx.ErrNoSuchRemote is
// returned as an error — a genuine operational problem, never guessed
// either way.
func Resolve(ctx context.Context, root string) (Resolution, error) {
	if defaultBranch, ok := specstate.ResolveDefaultBranch(ctx, root); ok {
		commit, err := gitx.RevParse(ctx, root, defaultBranch.Ref)
		if err != nil {
			return Resolution{}, err
		}
		return Resolution{Kind: ResolvedDefault, Ref: defaultBranch.Ref, BranchName: defaultBranch.Name, Commit: commit}, nil
	}

	_, remoteErr := gitx.RemoteURL(ctx, root, "origin")
	switch {
	case errors.Is(remoteErr, gitx.ErrNoSuchRemote):
		headCommit, err := gitx.RevParse(ctx, root, "HEAD")
		if err != nil {
			return Resolution{}, err
		}
		return Resolution{Kind: HeadFallback, Ref: "HEAD", Commit: headCommit}, nil
	case remoteErr != nil:
		return Resolution{}, remoteErr
	default:
		return Resolution{Kind: Unresolvable}, nil
	}
}
