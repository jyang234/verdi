package repositoryfacts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// GatherInput is Gather's evaluated-target input: the checkout root plus
// the exact bytes and provenance of a target already resolved by the
// caller (journey's own two-form target resolution, or a later
// contextcompile accepted-spec resolution). Gather never resolves a
// target itself; it only computes repository facts AROUND one already
// chosen.
type GatherInput struct {
	Root              string
	TargetPath        string
	TargetContent     []byte
	TargetFoundOnDisk bool
}

// Gatherer computes one Snapshot from a checkout. Construct it with
// NewGatherer (production) or the package-private newGatherer (this
// package's own tests, over fakes) — mirroring internal/journey.Projector's
// own construction discipline. The zero value's ports are nil, so Gather
// fails closed rather than panicking through a nil interface call.
type Gatherer struct {
	git                  GitReader
	resolveDefaultBranch DefaultBranchResolver
}

// NewGatherer returns a Gatherer backed by real git plumbing and
// specstate.ResolveDefaultBranch — the only constructor production
// callers may use.
func NewGatherer() Gatherer {
	return Gatherer{git: NewGitReader(), resolveDefaultBranch: specstate.ResolveDefaultBranch}
}

// newGatherer is the test-only seam: this package's own tests construct a
// Gatherer over in-process fakes (see gather_test.go).
func newGatherer(git GitReader, resolveDefaultBranch DefaultBranchResolver) Gatherer {
	return Gatherer{git: git, resolveDefaultBranch: resolveDefaultBranch}
}

// Gather computes in's repository-identity Snapshot: remote origin,
// branch, HEAD, configured default branch, their relationship,
// dirty/staged posture, managed-worktree identity, and which checkout
// state in.TargetContent was evaluated from.
//
// Every fact that cannot be established from a live checkout becomes
// Known == false plus a deterministic DisclosureCode — never a guessed
// value, and never the underlying git error text (which might itself
// carry a credential or an absolute path). Gather itself returns a
// non-nil error only when g is unusable (the zero value, or a partially
// constructed Gatherer) — the "keep the zero value fail closed" contract
// — never for an ordinary unprovable repository fact.
func (g Gatherer) Gather(ctx context.Context, in GatherInput) (Snapshot, error) {
	if g.git == nil || g.resolveDefaultBranch == nil {
		return Snapshot{}, fmt.Errorf("repositoryfacts: zero-value Gatherer cannot Gather (construct one with NewGatherer)")
	}

	var codes []DisclosureCode
	var f Facts

	switch url, rerr := g.git.RemoteURL(ctx, in.Root, "origin"); {
	case rerr == nil:
		// AC-1's repository section (and this leaf's own Facts shape)
		// reports a CANONICAL repository identity, never the raw origin
		// URL: the raw URL may carry credentials, and its ssh/https
		// spellings of one repository differ, which would make identity
		// and every digest over it checkout-dependent. Canonicalization is
		// gitx's (CanonicalRemoteIdentity); gitx.RemoteURL itself still
		// returns the raw URL for the callers that need it.
		if identity, ok := gitx.CanonicalRemoteIdentity(url); ok {
			f.RemoteOrigin = StringFact{Known: true, Value: identity}
		} else {
			// The raw URL is never routed into the fact OR the disclosure
			// (it is exactly the string that may carry a credential): the
			// disclosure names only the cause class, fixed and
			// machine-independent.
			f.RemoteOrigin = StringFact{Known: false}
			codes = append(codes, DisclosureRemoteOriginUncanonicalizable)
		}
	case errors.Is(rerr, gitx.ErrNoSuchRemote):
		f.RemoteOrigin = StringFact{Known: false}
		codes = append(codes, DisclosureRemoteOriginNotConfigured)
	default:
		// The underlying gitx error (which may itself carry an absolute
		// path or raw git stderr text) is never routed into the fact —
		// Known == false already carries the honesty; the disclosure
		// names only the cause class, fixed and machine-independent.
		f.RemoteOrigin = StringFact{Known: false}
		codes = append(codes, DisclosureRemoteOriginReadFailed)
	}

	branch, berr := g.git.CurrentBranch(ctx, in.Root)
	switch {
	case berr != nil:
		f.Branch = StringFact{Known: false}
		codes = append(codes, DisclosureBranchUnresolved)
	case branch == "":
		f.Branch = StringFact{Known: false}
		codes = append(codes, DisclosureBranchDetached)
	default:
		f.Branch = StringFact{Known: true, Value: branch}
	}

	head, herr := g.git.RevParse(ctx, in.Root, "HEAD")
	if herr != nil {
		f.Head = StringFact{Known: false}
		codes = append(codes, DisclosureHeadUnresolved)
	} else {
		f.Head = StringFact{Known: true, Value: head}
	}

	var defaultHead string
	var defaultKnown bool
	if db, ok := g.resolveDefaultBranch(ctx, in.Root); ok {
		if dh, derr := g.git.RevParse(ctx, in.Root, db.Ref); derr == nil {
			f.DefaultBranch = DefaultBranchFact{Known: true, Name: db.Name, Ref: db.Ref, Head: dh}
			defaultHead, defaultKnown = dh, true
		} else {
			// The default branch NAME resolved, but its ref could not be
			// turned into a commit — a distinct, disclosed failure from "no
			// default branch resolves at all" (never disclosed by this
			// leaf; see DisclosureDefaultBranchRefUnresolved's doc
			// comment), never silently folded into the same unknown.
			codes = append(codes, DisclosureDefaultBranchRefUnresolved)
		}
	}
	if !defaultKnown {
		f.DefaultBranch = DefaultBranchFact{Known: false}
	}

	f.Relationship = relationship(ctx, g.git, in.Root, f.Head, defaultHead, defaultKnown)

	dirty, derr := g.git.StatusDirty(ctx, in.Root)
	if derr != nil {
		f.Dirty = BoolFact{Known: false}
		codes = append(codes, DisclosureDirtyUnknown)
	} else {
		f.Dirty = BoolFact{Known: true, Value: dirty}
	}

	staged, serr := g.git.StagedPaths(ctx, in.Root)
	if serr != nil {
		f.Staged = BoolFact{Known: false}
		codes = append(codes, DisclosureStagedUnknown)
	} else {
		f.Staged = BoolFact{Known: true, Value: len(staged) > 0}
	}

	f.Worktree = worktreeFact(in.Root)

	if !in.TargetFoundOnDisk {
		f.Source = SourceRemoteRef
	} else {
		headBytes, serr := g.git.Show(ctx, in.Root, "HEAD", in.TargetPath)
		if serr == nil && bytes.Equal(headBytes, in.TargetContent) {
			f.Source = SourceHead
		} else {
			f.Source = SourceWorkingTree
		}
	}

	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	if codes == nil {
		codes = []DisclosureCode{}
	}
	return Snapshot{Facts: f, Disclosures: codes}, nil
}

// relationship classifies HEAD against the default branch's HEAD: "equal"
// on identical shas; otherwise IsAncestor(default, HEAD) -> "ahead",
// IsAncestor(HEAD, default) -> "behind", neither -> "diverged"; unknown
// whenever either HEAD is itself unknown or an ancestry check errors.
func relationship(ctx context.Context, git GitReader, root string, head StringFact, defaultHead string, defaultKnown bool) string {
	if !head.Known || !defaultKnown {
		return RelationshipUnknown
	}
	if head.Value == defaultHead {
		return RelationshipEqual
	}
	ahead, aerr := git.IsAncestor(ctx, root, defaultHead, head.Value)
	if aerr != nil {
		return RelationshipUnknown
	}
	if ahead {
		return RelationshipAhead
	}
	behind, berr := git.IsAncestor(ctx, root, head.Value, defaultHead)
	if berr != nil {
		return RelationshipUnknown
	}
	if behind {
		return RelationshipBehind
	}
	return RelationshipDiverged
}

// worktreeMarker is the "/.verdi/data/worktrees/" path segment a store
// root's managed-worktree membership is decided against, derived from
// wtmanager.WorktreesRoot (its own single home for the managed-worktree
// layout — CLAUDE.md: never a second hardcoded copy of that literal)
// rather than a second literal of this package's own.
func worktreeMarker() string {
	return "/" + filepath.ToSlash(wtmanager.WorktreesRoot("")) + "/"
}

// worktreeFact classifies root as a managed worktree iff its
// slash-normalized path contains worktreeMarker's segment; Name is the
// path segment immediately after it (wtmanager's own <name> == a design
// branch's spec name, naming.go's worktreeName).
func worktreeFact(root string) WorktreeFact {
	norm := filepath.ToSlash(root)
	marker := worktreeMarker()
	idx := strings.Index(norm, marker)
	if idx < 0 {
		return WorktreeFact{Managed: false}
	}
	rest := norm[idx+len(marker):]
	name, _, _ := strings.Cut(rest, "/")
	if name == "" {
		return WorktreeFact{Managed: false}
	}
	return WorktreeFact{Managed: true, Name: name}
}
