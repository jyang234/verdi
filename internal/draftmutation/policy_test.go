package draftmutation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

type staticPolicySource struct {
	policy *policyauthority.EffectivePolicy
	err    error
}

func (s staticPolicySource) ResolveEffectivePolicy(context.Context, string) (*policyauthority.EffectivePolicy, error) {
	return s.policy, s.err
}

func copyPolicyFixture(t *testing.T, mode string, includePayload bool) string {
	t.Helper()
	source := filepath.Join("..", "policyauthority", "testdata", "store")
	destination := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if entry.Name() == "go-toolchain.md" {
			if includePayload {
				data = []byte(strings.Replace(string(data), "mode: proposal-only", "mode: "+mode, 1))
			} else {
				data = []byte(strings.Replace(string(data), "payloads:\n  design_assistance: {mode: proposal-only, layout: false}", "payloads: {}", 1))
			}
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return destination
}

func resolvedPolicy(t *testing.T, mode string, includePayload bool) *policyauthority.EffectivePolicy {
	t.Helper()
	store, err := policyauthority.Load(copyPolicyFixture(t, mode, includePayload))
	if err != nil {
		t.Fatalf("policyauthority.Load: %v", err)
	}
	policy, err := policyauthority.Resolve(store)
	if err != nil {
		t.Fatalf("policyauthority.Resolve: %v", err)
	}
	return policy
}

func TestPolicyModeMatrixAndDigest(t *testing.T) {
	agent, err := NewDelegatedAgent("codex", "session-1")
	if err != nil {
		t.Fatal(err)
	}
	identity := testIdentity()
	for _, tt := range []struct {
		mode  string
		allow bool
	}{
		{"off", false},
		{"proposal-only", false},
		{"draft-write", true},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			policy := resolvedPolicy(t, tt.mode, true)
			grant, typed := AuthorizePolicy(context.Background(), "/repo", identity, agent, staticPolicySource{policy: policy})
			if tt.allow {
				if typed != nil || grant.Mode != "draft-write" || grant.Digest == "" {
					t.Fatalf("AuthorizePolicy = %+v, %v", grant, typed)
				}
			} else if typed == nil || typed.Code != CodePolicyForbidden || typed.Identity != identity {
				t.Fatalf("AuthorizePolicy = %+v, %v", grant, typed)
			}
		})
	}
}

func TestPolicyAbsentPayloadLayoutAndForgeryFailClosed(t *testing.T) {
	agent, _ := NewDelegatedAgent("codex", "")
	identity := testIdentity()
	tests := []struct {
		name   string
		source PolicySource
		code   Code
	}{
		{"unadopted", staticPolicySource{err: policyauthority.ErrNotAdopted}, CodePolicyForbidden},
		{"absent payload", staticPolicySource{policy: resolvedPolicy(t, "proposal-only", false)}, CodePolicyForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := AuthorizePolicy(context.Background(), "/repo", identity, agent, tt.source); err == nil || err.Code != tt.code || err.Identity != identity {
				t.Fatalf("AuthorizePolicy error = %v", err)
			}
		})
	}

	for _, mutate := range []func(*policyauthority.EffectivePolicy){
		func(policy *policyauthority.EffectivePolicy) {
			payload := policy.Policies[0].Payloads[policyartifact.DesignAssistancePayloadKind].(*policyartifact.DesignAssistancePayload)
			payload.Layout = true
		},
		func(policy *policyauthority.EffectivePolicy) { policy.ProfileID = "forged" },
	} {
		policy := resolvedPolicy(t, "draft-write", true)
		mutate(policy)
		if _, err := AuthorizePolicy(context.Background(), "/repo", identity, agent, staticPolicySource{policy: policy}); err == nil || err.Code != CodeAuthorityInvalid {
			t.Fatalf("forged policy error = %v", err)
		}
	}
}

type trustFacts struct{}

func (trustFacts) ReadTrustFact(_ context.Context, source governanceprincipal.TrustSource, claim governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error) {
	return governanceprincipal.TrustFact{SourceID: source.ID, SourceKind: source.Kind, Subjects: []string{claim.Subject}, EvidenceDigest: DigestBytes([]byte("fact")), Available: true, Valid: true}, nil
}

func TestPolicyActorsRequireAdapterControlledSealedAttribution(t *testing.T) {
	for _, harness := range []string{"", "   "} {
		if _, err := NewDelegatedAgent(harness, ""); err == nil {
			t.Fatalf("NewDelegatedAgent(%q) accepted", harness)
		}
	}
	if _, err := NewDelegatedAgent("codex", "   "); err == nil {
		t.Fatal("NewDelegatedAgent accepted a blank optional session")
	}
	agent, err := NewDelegatedAgent("codex", "session")
	if err != nil || agent.Kind() != ActorDelegatedAgent || !agent.Attribution().Unauthenticated || agent.Harness() != "codex" {
		t.Fatalf("delegated actor = %+v, %v", agent, err)
	}

	store, err := policyauthority.Load(copyPolicyFixture(t, "draft-write", true))
	if err != nil {
		t.Fatal(err)
	}
	profile := store.Profiles[store.Constitution.SelectedProfile].Profile
	resolver := governanceprincipal.NewResolver(trustFacts{})
	resolution, err := resolver.Resolve(context.Background(), profile, governanceprincipal.PrincipalClaim{TrustSource: "github-org", Subject: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	human, err := NewTrustedHuman(resolution)
	if err != nil || human.Kind() != ActorHuman || human.Attribution().PrincipalID == "" || human.Harness() != "" {
		t.Fatalf("trusted human = %+v, %v", human, err)
	}
	forged := governanceprincipal.PrincipalResolution{State: governanceprincipal.ResolutionAuthenticated, PrincipalID: human.Attribution().PrincipalID}
	if _, err := NewTrustedHuman(forged); err == nil || !strings.Contains(err.Error(), "Resolver.Resolve") {
		t.Fatalf("NewTrustedHuman(forged) error = %v", err)
	}

	policy := resolvedPolicy(t, "off", true)
	if grant, typed := AuthorizePolicy(context.Background(), "/repo", testIdentity(), human, staticPolicySource{policy: policy}); typed != nil || grant.Digest == "" {
		t.Fatalf("human policy = %+v, %v", grant, typed)
	}

	if _, typed := AuthorizePolicy(context.Background(), "/repo", testIdentity(), Actor{}, staticPolicySource{policy: policy}); typed == nil || typed.Code != CodeActorForbidden {
		t.Fatalf("zero actor refusal = %v", typed)
	}
}
