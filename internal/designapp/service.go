package designapp

import (
	"context"

	"github.com/jyang234/verdi/internal/draftmutation"
)

// MutationService is the consumer-owned port over draftmutation's one
// mutation algorithm (AC-1's "Shared draft-mutation architecture" — the
// ONLY mutation algorithm; designapp must never reimplement it).
// draftmutation.Service satisfies this directly.
type MutationService interface {
	Mutate(ctx context.Context, start string, request draftmutation.Request, actor draftmutation.Actor) (draftmutation.Response, *draftmutation.Error)
}

// Service is the sole application consumer for the six ASD operations
// (doc.go). Every port has a production default wired by NewService;
// tests substitute fakes/fixturegit-backed adapters at each named port,
// never by reimplementing an owner's algorithm inside this package.
type Service struct {
	// Identity resolves the canonical checkout/branch/HEAD facts every one
	// of the six operations reports (AC-8 "Branch and worktree identity").
	// draftmutation.GitIdentityReader{} is real production Git; MutateDraft
	// does not use this field directly — draftmutation.Service.Mutate
	// resolves its own identity internally.
	Identity draftmutation.IdentityReader

	// Policy resolves the effective design_assistance grant
	// (internal/policyauthority via draftmutation's own adapter — AC-3).
	Policy draftmutation.PolicySource

	// State projects a spec's Git-derived lifecycle state
	// (internal/specstate — AC-3's "current spec state").
	State draftmutation.StateProjector

	// Mutation is the one mutate_draft algorithm owner (AC-1).
	Mutation MutationService

	// Board loads the deterministic board projection get_board returns
	// (AC-8: "the same deterministic board projection already shared with
	// the human interface"). Callers with a live forge (MCP) and callers
	// without one (CLI) supply different BoardLoader values; designapp
	// itself never learns forge/review-feed concepts (board.go).
	Board BoardLoader

	// Align supplies get_design_context's Verdi-go-derived service and
	// boundary findings (AC-5). nil-safe: absent or unconfigured (no
	// verdi.yaml toolchain: block) degrades to a disclosed, non-fatal
	// omission — the same graceful-skip posture cmd/verdi/baseline.go
	// already uses for the identical "no toolchain configured" case
	// (context.go).
	Align AlignFindings
}

// NewService returns the production Service: real Git identity, real
// constitution-backed policy authority, real Git-derived spec state, the
// real draftmutation core, the real board projection owner (with no forge
// — MCP's Backend overrides Board to add its own live/absent review-feed
// posture), and the real internal/align findings composer.
func NewService() Service {
	return Service{
		Identity: draftmutation.GitIdentityReader{},
		Policy:   draftmutation.ConstitutionPolicySource{},
		State:    draftmutation.NewGitStateProjector(),
		Mutation: draftmutation.NewService(),
		Board:    workbenchBoardLoader{},
		Align:    upstreamAlignFindings{},
	}
}

// resolveIdentity is the shared read-path identity prologue every
// non-mutation operation starts from: the same
// draftmutation.ResolveCanonicalIdentity construction Mutate itself uses
// (AC-8: "every response names the exact checkout, branch, HEAD, and
// spec"), reused rather than re-derived a second time.
func (s Service) resolveIdentity(ctx context.Context, start, spec string) (draftmutation.Identity, *Error) {
	if s.Identity == nil {
		return draftmutation.Identity{}, operational("identity-unavailable", "identity reader is not configured", nil)
	}
	identity, err := draftmutation.ResolveCanonicalIdentity(ctx, start, spec, s.Identity)
	if err != nil {
		return draftmutation.Identity{}, operational("identity-unavailable", "resolving canonical identity", err)
	}
	return identity, nil
}
