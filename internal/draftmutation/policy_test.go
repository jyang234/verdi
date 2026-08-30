package draftmutation

import (
	"context"
	"errors"
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

type resolutionTrustFacts struct {
	state governanceprincipal.ResolutionState
}

func (f resolutionTrustFacts) ReadTrustFact(_ context.Context, source governanceprincipal.TrustSource, claim governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error) {
	switch f.state {
	case governanceprincipal.ResolutionAuthenticated:
		return governanceprincipal.TrustFact{SourceID: source.ID, SourceKind: source.Kind, Subjects: []string{claim.Subject}, EvidenceDigest: DigestBytes([]byte("authenticated fact")), Available: true, Valid: true}, nil
	case governanceprincipal.ResolutionViolated:
		return governanceprincipal.TrustFact{SourceID: source.ID, SourceKind: source.Kind, Subjects: []string{"other-subject"}, EvidenceDigest: DigestBytes([]byte("violated fact")), Available: true, Valid: true}, nil
	case governanceprincipal.ResolutionUnproven:
		return governanceprincipal.TrustFact{SourceID: source.ID, SourceKind: source.Kind, Reason: "trust evidence unavailable"}, nil
	default:
		return governanceprincipal.TrustFact{}, errors.New("unsupported test resolution state")
	}
}

func resolutionForActor(t *testing.T, state governanceprincipal.ResolutionState) governanceprincipal.PrincipalResolution {
	t.Helper()
	store, err := policyauthority.Load(copyPolicyFixture(t, "draft-write", true))
	if err != nil {
		t.Fatal(err)
	}
	profile := store.Profiles[store.Constitution.SelectedProfile].Profile
	resolution, err := governanceprincipal.NewResolver(resolutionTrustFacts{state: state}).Resolve(context.Background(), profile, governanceprincipal.PrincipalClaim{TrustSource: "github-org", Subject: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if resolution.State != state {
		t.Fatalf("resolution state = %q, want %q", resolution.State, state)
	}
	return resolution
}

func TestResolvedHumanAttributionStateMatrix(t *testing.T) {
	for _, state := range []governanceprincipal.ResolutionState{
		governanceprincipal.ResolutionAuthenticated,
		governanceprincipal.ResolutionViolated,
		governanceprincipal.ResolutionUnproven,
	} {
		t.Run(string(state), func(t *testing.T) {
			actor, err := NewTrustedHuman(resolutionForActor(t, state))
			if err != nil {
				t.Fatalf("NewTrustedHuman: %v", err)
			}
			if actor.Kind() != ActorHuman || actor.Harness() != "" || actor.Session() != "" {
				t.Fatalf("resolved actor = %+v", actor)
			}
			if state == governanceprincipal.ResolutionAuthenticated {
				if actor.Attribution().PrincipalID == "" || actor.Attribution().Unauthenticated {
					t.Fatalf("authenticated attribution = %+v", actor.Attribution())
				}
				if grant, typed := AuthorizePolicy(context.Background(), "/repo", testIdentity(), actor, staticPolicySource{policy: resolvedPolicy(t, "off", true)}); typed != nil || grant.Digest == "" {
					t.Fatalf("authenticated human policy = %+v, %v", grant, typed)
				}
				return
			}
			if !actor.Attribution().Unauthenticated || actor.Attribution().PrincipalID != "" {
				t.Fatalf("%s attribution = %+v, want explicit unauthenticated marker", state, actor.Attribution())
			}
			for _, mode := range []string{"off", "proposal-only"} {
				if _, typed := AuthorizePolicy(context.Background(), "/repo", testIdentity(), actor, staticPolicySource{policy: resolvedPolicy(t, mode, true)}); typed == nil || typed.Code != CodePolicyForbidden {
					t.Fatalf("%s actor bypassed %s policy: %v", state, mode, typed)
				}
			}
			if grant, typed := AuthorizePolicy(context.Background(), "/repo", testIdentity(), actor, staticPolicySource{policy: resolvedPolicy(t, "draft-write", true)}); typed != nil || grant.Digest == "" {
				t.Fatalf("%s draft-write policy = %+v, %v", state, grant, typed)
			}
		})
	}

	forged := resolutionForActor(t, governanceprincipal.ResolutionAuthenticated)
	forged.Witnesses = nil
	if _, err := NewTrustedHuman(forged); err == nil || !strings.Contains(err.Error(), "modified after Resolver.Resolve") {
		t.Fatalf("NewTrustedHuman(forged) error = %v", err)
	}
}

// TestNewUnauthenticatedHumanBasisDistinctFromViolatedUnprovenTrustedHuman
// proves §4.1/SI-176's core distinction: NewUnauthenticatedHuman and a
// violated- or unproven-resolution NewTrustedHuman actor serialize the
// IDENTICAL kernel unauthenticated attribution (Kind/Attribution/Harness/
// Session all equal), yet only the explicit constructor's product is
// accepted by AuthorizeBrowserHuman — a failed identity proof never
// acquires the browser-human allowance.
func TestNewUnauthenticatedHumanBasisDistinctFromViolatedUnprovenTrustedHuman(t *testing.T) {
	browserHuman, err := NewUnauthenticatedHuman()
	if err != nil {
		t.Fatalf("NewUnauthenticatedHuman: %v", err)
	}
	if browserHuman.Kind() != ActorHuman || browserHuman.Harness() != "" || browserHuman.Session() != "" {
		t.Fatalf("browser-human actor shape = %+v", browserHuman)
	}
	if !browserHuman.Attribution().Unauthenticated || browserHuman.Attribution().PrincipalID != "" {
		t.Fatalf("browser-human attribution = %+v, want the explicit unauthenticated marker", browserHuman.Attribution())
	}

	for _, state := range []governanceprincipal.ResolutionState{governanceprincipal.ResolutionViolated, governanceprincipal.ResolutionUnproven} {
		t.Run(string(state), func(t *testing.T) {
			unproven, err := NewTrustedHuman(resolutionForActor(t, state))
			if err != nil {
				t.Fatalf("NewTrustedHuman: %v", err)
			}
			// Identical serialized shape (§4.1: "even though both serialize
			// the kernel's unauthenticated attribution").
			if unproven.Kind() != browserHuman.Kind() || unproven.Attribution() != browserHuman.Attribution() ||
				unproven.Harness() != browserHuman.Harness() || unproven.Session() != browserHuman.Session() {
				t.Fatalf("%s trusted-human shape = %+v, want it identical to the browser-human's serialized shape", state, unproven)
			}

			identity := testIdentity()
			policy := resolvedPolicy(t, "off", true)

			// AuthorizeBrowserHuman refuses the failed-identity-proof actor
			// even under a valid adopted policy — no bypass.
			if _, typed := AuthorizeBrowserHuman(context.Background(), "/repo", identity, unproven, staticPolicySource{policy: policy}); typed == nil || typed.Code != CodeActorForbidden {
				t.Fatalf("AuthorizeBrowserHuman(%s trusted-human) = %v, want actor-forbidden", state, typed)
			}
			// The explicit browser-human actor succeeds under the identical
			// mode-off policy AuthorizePolicy would have refused it under.
			posture, typed := AuthorizeBrowserHuman(context.Background(), "/repo", identity, browserHuman, staticPolicySource{policy: policy})
			if typed != nil || !posture.Adopted || posture.Digest == "" {
				t.Fatalf("AuthorizeBrowserHuman(browser-human) = %+v, %v, want adopted digest", posture, typed)
			}
			// AuthorizePolicy, unchanged, still refuses the failed-identity
			// actor under mode off (the byte-for-byte preserved matrix).
			if _, typed := AuthorizePolicy(context.Background(), "/repo", identity, unproven, staticPolicySource{policy: policy}); typed == nil || typed.Code != CodePolicyForbidden {
				t.Fatalf("AuthorizePolicy(%s trusted-human, mode off) = %v, want policy-forbidden", state, typed)
			}
		})
	}
}

// TestAuthorizeBrowserHumanPolicyPosture is the browser-human authorization
// matrix (Wave 6 design §4.1, SI-176): valid adopted policy resolves
// regardless of mode or design_assistance payload presence; only
// errors.Is(err, policyauthority.ErrNotAdopted) becomes the honest
// not-applicable posture; every other resolution failure (nil source,
// malformed/forged policy) remains an operational refusal; and any actor
// other than the explicit browser-human is refused outright.
func TestAuthorizeBrowserHumanPolicyPosture(t *testing.T) {
	browserHuman, err := NewUnauthenticatedHuman()
	if err != nil {
		t.Fatal(err)
	}
	identity := testIdentity()

	for _, tt := range []struct {
		name    string
		mode    string
		payload bool
	}{
		{"mode off with payload", "off", true},
		{"mode proposal-only with payload", "proposal-only", true},
		{"mode draft-write with payload", "draft-write", true},
		{"mode off with no design_assistance payload at all", "off", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			policy := resolvedPolicy(t, tt.mode, tt.payload)
			wantDigest, err := policy.Digest()
			if err != nil {
				t.Fatal(err)
			}
			posture, typed := AuthorizeBrowserHuman(context.Background(), "/repo", identity, browserHuman, staticPolicySource{policy: policy})
			if typed != nil || !posture.Adopted || posture.Digest != wantDigest {
				t.Fatalf("AuthorizeBrowserHuman = %+v, %v, want adopted %q", posture, typed, wantDigest)
			}
		})
	}

	t.Run("genuine non-adoption is the honest not-applicable posture", func(t *testing.T) {
		posture, typed := AuthorizeBrowserHuman(context.Background(), "/repo", identity, browserHuman, staticPolicySource{err: policyauthority.ErrNotAdopted})
		if typed != nil || posture.Adopted || posture.Digest != "" {
			t.Fatalf("AuthorizeBrowserHuman(not adopted) = %+v, %v, want not-applicable", posture, typed)
		}
	})

	for _, tt := range []struct {
		name   string
		source PolicySource
	}{
		{"nil policy source", nil},
		{"malformed policy source error", staticPolicySource{err: errors.New("policy store is corrupt")}},
		{"unsealed effective policy", staticPolicySource{policy: &policyauthority.EffectivePolicy{}}},
		{"forged effective policy", func() PolicySource {
			policy := resolvedPolicy(t, "draft-write", true)
			policy.ProfileID = "forged"
			return staticPolicySource{policy: policy}
		}()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			posture, typed := AuthorizeBrowserHuman(context.Background(), "/repo", identity, browserHuman, tt.source)
			if typed == nil || typed.Code != CodeAuthorityInvalid || typed.Verdict() || posture.Adopted {
				t.Fatalf("AuthorizeBrowserHuman(%s) = %+v, %v, want operational authority-invalid", tt.name, posture, typed)
			}
		})
	}

	t.Run("delegated agent is refused outright, never reaching policy resolution", func(t *testing.T) {
		agent, err := NewDelegatedAgent("codex", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, typed := AuthorizeBrowserHuman(context.Background(), "/repo", identity, agent, staticPolicySource{policy: resolvedPolicy(t, "draft-write", true)}); typed == nil || typed.Code != CodeActorForbidden {
			t.Fatalf("AuthorizeBrowserHuman(delegated agent) = %v, want actor-forbidden", typed)
		}
	})

	t.Run("resolved trusted-human is refused outright too", func(t *testing.T) {
		human, err := NewTrustedHuman(resolutionForActor(t, governanceprincipal.ResolutionAuthenticated))
		if err != nil {
			t.Fatal(err)
		}
		if _, typed := AuthorizeBrowserHuman(context.Background(), "/repo", identity, human, staticPolicySource{policy: resolvedPolicy(t, "draft-write", true)}); typed == nil || typed.Code != CodeActorForbidden {
			t.Fatalf("AuthorizeBrowserHuman(resolved trusted-human) = %v, want actor-forbidden", typed)
		}
	})

	t.Run("zero actor is refused before basis is ever checked", func(t *testing.T) {
		if _, typed := AuthorizeBrowserHuman(context.Background(), "/repo", identity, Actor{}, staticPolicySource{policy: resolvedPolicy(t, "draft-write", true)}); typed == nil || typed.Code != CodeActorForbidden {
			t.Fatalf("AuthorizeBrowserHuman(zero actor) = %v, want actor-forbidden", typed)
		}
	})
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
