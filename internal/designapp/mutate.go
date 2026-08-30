package designapp

import (
	"context"

	"github.com/jyang234/verdi/internal/draftmutation"
)

// MutateDraft applies one atomic typed transaction (AC-8's mutate_draft).
// It is a deliberate pass-through onto draftmutation's own Mutate: the
// request, response, and error types are draftmutation's closed union,
// byte-identical to what `verdi design mutate` has always produced,
// because AC-1's mutation contract is the ONE algorithm (doc.go) and
// wrapping or re-encoding it here would be exactly the "parallel
// interpretation of domain mutations" CO-9's adapter-conformance section
// forbids. request must already be the strict-decoded, validated
// draftmutation.Request draftmutation.DecodeRequest produces; actor must
// already be adapter-minted (draftmutation.NewDelegatedAgent or
// draftmutation.NewTrustedHuman) — designapp never mints or authorizes an
// actor itself (CO-4).
func (s Service) MutateDraft(ctx context.Context, start string, request draftmutation.Request, actor draftmutation.Actor) (draftmutation.Response, *draftmutation.Error) {
	if s.Mutation == nil {
		return draftmutation.Response{}, draftmutation.NewIdentityUnavailableError("mutation service is not configured", nil)
	}
	return s.Mutation.Mutate(ctx, start, request, actor)
}
