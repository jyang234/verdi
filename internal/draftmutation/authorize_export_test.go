package draftmutation

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// TestAuthorizeMutationPolicy_ExportedDispatch proves the narrowly exported
// AuthorizeMutationPolicy (spec-import-contract.md: "Export the existing
// draftmutation actor-policy dispatcher as a shared function, without
// changing its authorization matrix") reaches the identical decisions
// Mutate's own unexported call already produced: explicit browser-human
// routes through AuthorizeBrowserHuman (adopted -> resolved digest,
// not-adopted -> honest not-applicable, never a forged digest), and every
// other actor keeps AuthorizePolicy's existing design_assistance-mode
// matrix unchanged.
func TestAuthorizeMutationPolicy_ExportedDispatch(t *testing.T) {
	identity := testIdentity()

	t.Run("browser-human adopted resolves a digest", func(t *testing.T) {
		human, err := NewUnauthenticatedHuman()
		if err != nil {
			t.Fatal(err)
		}
		policy, typed := AuthorizeMutationPolicy(context.Background(), "/repo", identity, human, staticPolicySource{policy: resolvedPolicy(t, "off", true)})
		if typed != nil {
			t.Fatalf("AuthorizeMutationPolicy: %v", typed)
		}
		if policy.State != designprovenance.PolicyResolved || policy.Digest == "" {
			t.Fatalf("policy = %+v, want resolved with a nonempty digest", policy)
		}
	})

	t.Run("browser-human not adopted is honestly not-applicable", func(t *testing.T) {
		human, err := NewUnauthenticatedHuman()
		if err != nil {
			t.Fatal(err)
		}
		policy, typed := AuthorizeMutationPolicy(context.Background(), "/repo", identity, human, staticPolicySource{err: policyauthority.ErrNotAdopted})
		if typed != nil {
			t.Fatalf("AuthorizeMutationPolicy: %v", typed)
		}
		if policy.State != designprovenance.PolicyNotApplicable || policy.Digest != "" {
			t.Fatalf("policy = %+v, want not-applicable with no digest", policy)
		}
	})

	t.Run("delegated agent under draft-write resolves a digest", func(t *testing.T) {
		agent, err := NewDelegatedAgent("codex", "session-1")
		if err != nil {
			t.Fatal(err)
		}
		policy, typed := AuthorizeMutationPolicy(context.Background(), "/repo", identity, agent, staticPolicySource{policy: resolvedPolicy(t, "draft-write", true)})
		if typed != nil {
			t.Fatalf("AuthorizeMutationPolicy: %v", typed)
		}
		if policy.State != designprovenance.PolicyResolved || policy.Digest == "" {
			t.Fatalf("policy = %+v, want resolved with a nonempty digest", policy)
		}
	})

	t.Run("delegated agent under proposal-only is policy-forbidden", func(t *testing.T) {
		agent, err := NewDelegatedAgent("codex", "session-1")
		if err != nil {
			t.Fatal(err)
		}
		_, typed := AuthorizeMutationPolicy(context.Background(), "/repo", identity, agent, staticPolicySource{policy: resolvedPolicy(t, "proposal-only", true)})
		if typed == nil || typed.Code != CodePolicyForbidden {
			t.Fatalf("AuthorizeMutationPolicy = %v, want CodePolicyForbidden", typed)
		}
	})

	t.Run("unsealed actor is actor-forbidden", func(t *testing.T) {
		_, typed := AuthorizeMutationPolicy(context.Background(), "/repo", identity, Actor{}, staticPolicySource{policy: resolvedPolicy(t, "draft-write", true)})
		if typed == nil || typed.Code != CodeActorForbidden {
			t.Fatalf("AuthorizeMutationPolicy = %v, want CodeActorForbidden", typed)
		}
	})
}
