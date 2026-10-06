package specstate

import (
	"context"
	"path/filepath"

	"github.com/jyang234/verdi/internal/gitx"
)

// acceptedKey is the context key a request's pinned accepted HEAD rides
// under.
type acceptedKey struct{}

// acceptedPin is one request's resolution of root's accepted HEAD. outer
// is the pin of another root the context already carried, so a request
// that pins two roots finds each.
type acceptedPin struct {
	root   string
	branch Branch
	ok     bool
	outer  *acceptedPin
}

// WithAcceptedHead resolves root's accepted HEAD once — the default
// branch's discovery (ResolveDefaultBranch's chain), then its ref to an id
// — and returns a context carrying that resolution. Through it every
// ResolveDefaultBranch for root returns the pinned Branch, Tip and Commit
// set, and resolves nothing again, so each consumer of one projection
// reads the default branch at the one commit id the projection resolved
// (Wave 6 §5.3, "one accepted-HEAD resolution" per page projection;
// ledger SI-356). The ref is resolved as `rev-parse --verify <Ref>` (the
// id every consumer that printed the accepted head printed) and that id
// peeled to its commit, so `<Ref>` and `<Ref>^{commit}` are one
// resolution.
//
// The pin rides the context alone: it dies with the request, and nothing
// it resolved outlives it (spec/readiness-recovery-v2 co-2). A context
// already pinned for root is returned unchanged, so a projection that
// composes several loads pins once around all of them. A default branch
// that does not resolve is pinned unresolved; a ref whose id cannot be
// read is pinned without Tip and Commit, and each consumer then reads at
// Ref as it did before.
func WithAcceptedHead(ctx context.Context, root string) context.Context {
	if pinFor(ctx, root) != nil {
		return ctx
	}
	outer, _ := ctx.Value(acceptedKey{}).(*acceptedPin)
	pin := &acceptedPin{root: filepath.Clean(root), outer: outer}
	pin.branch, pin.ok = resolveAcceptedHead(ctx, root)
	return context.WithValue(ctx, acceptedKey{}, pin)
}

// pinFor returns ctx's pin for root, or nil.
func pinFor(ctx context.Context, root string) *acceptedPin {
	clean := filepath.Clean(root)
	for pin, _ := ctx.Value(acceptedKey{}).(*acceptedPin); pin != nil; pin = pin.outer {
		if pin.root == clean {
			return pin
		}
	}
	return nil
}

// resolveAcceptedHead is WithAcceptedHead's one resolution.
func resolveAcceptedHead(ctx context.Context, root string) (Branch, bool) {
	branch, ok := resolveDefaultBranch(ctx, root)
	if !ok {
		return Branch{}, false
	}
	tip, err := gitx.RevParse(ctx, root, branch.Ref)
	if err != nil {
		return branch, true
	}
	commit, err := gitx.RevParse(ctx, root, tip+"^{commit}")
	if err != nil {
		return branch, true
	}
	branch.Tip, branch.Commit = tip, commit
	return branch, true
}
