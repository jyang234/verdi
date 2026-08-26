package lifecyclecountersign

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/forge"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

const lifecycleCandidateSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestResolve(t *testing.T) {
	t.Run("proven exact-head fresh non-self story review", func(t *testing.T) {
		resolver, request := lifecycleFixture(t, "story", "101", "900")
		result, err := resolver.Resolve(context.Background(), request)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.Verdict != countersign.VerdictProven || result.Record == nil {
			t.Fatalf("result = %+v, want proven record", result)
		}
		if result.Record.Obligation.Role != "story-review" || result.Record.Obligation.RequiredCount != 1 || result.Record.Obligation.SeparationRule != countersign.SeparationDifferentFromAuthor {
			t.Fatalf("obligation = %+v", result.Record.Obligation)
		}
		if len(result.Record.Approvals) != 1 || result.Record.Approvals[0].ApprovalRef != "review/1" || result.Record.Approvals[0].PrincipalResolution.PrincipalID == "" {
			t.Fatalf("approvals = %+v", result.Record.Approvals)
		}
	})

	t.Run("feature target maps to feature uat and model close count", func(t *testing.T) {
		resolver, request := lifecycleFixture(t, "feature", "201", "900")
		request.Model.Lifecycle["feature"] = model.Lifecycle{Transitions: []model.Transition{{Verb: "close", Obligations: []model.Obligation{{Scheme: "attestation", Kind: "countersign", Count: 2}}}}}
		second := resolver.Forge.(*forgefake.Forge)
		snapshot, err := forge.NewApprovalSnapshot("github", "acme/widgets", "17", lifecycleCandidateSHA, forge.ProviderActor{Scheme: "github-user-id", Subject: "900"}, lifecycleNow().Add(-time.Minute), []forge.Approval{
			lifecycleApproval("1", "201"), lifecycleApproval("2", "202"),
		})
		if err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		second.SeedApprovalSnapshot("17", snapshot)
		result, err := resolver.Resolve(context.Background(), request)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.Verdict != countersign.VerdictProven || result.Record == nil || result.Record.Obligation.Role != "feature-uat" || result.Record.Obligation.RequiredCount != 2 {
			t.Fatalf("result = %+v", result)
		}
	})

	t.Run("self approval is violated", func(t *testing.T) {
		resolver, request := lifecycleFixture(t, "story", "900", "900")
		result, err := resolver.Resolve(context.Background(), request)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.Verdict != countersign.VerdictViolated || result.Record == nil {
			t.Fatalf("result = %+v, want violated record", result)
		}
		if !containsLifecycleWitness(result.Witnesses, "approval-separation:") {
			t.Fatalf("witnesses = %+v, want separation witness", result.Witnesses)
		}
	})

	t.Run("missing operands are blocking unproven", func(t *testing.T) {
		missingProfile := fmt.Errorf("%w: profile unavailable", ErrProfileUnavailable)
		tests := []struct {
			name   string
			mutate func(*Resolver, *Request)
			want   string
		}{
			{"config", func(_ *Resolver, r *Request) { r.Manifest.Countersign = nil }, "config"},
			{"forge", func(r *Resolver, _ *Request) { r.Forge = nil }, "forge"},
			{"source branch", func(_ *Resolver, r *Request) { r.SourceBranch = "" }, "source-branch"},
			{"merge request", func(r *Resolver, _ *Request) { r.Forge = forgefake.New() }, "merge-request"},
			{"profile", func(r *Resolver, _ *Request) {
				r.LoadProfile = func(string) (gp.Profile, error) { return gp.Profile{}, missingProfile }
			}, "profile"},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				resolver, request := lifecycleFixture(t, "story", "101", "900")
				tc.mutate(&resolver, &request)
				result, err := resolver.Resolve(context.Background(), request)
				if err != nil {
					t.Fatalf("Resolve: %v", err)
				}
				if result.Verdict != countersign.VerdictUnproven || result.Record != nil || !containsLifecycleWitness(result.Witnesses, tc.want) {
					t.Fatalf("result = %+v, want unproven %s", result, tc.want)
				}
			})
		}
	})

	t.Run("malformed model contract is operational", func(t *testing.T) {
		resolver, request := lifecycleFixture(t, "story", "101", "900")
		request.Model.Lifecycle["story"] = model.Lifecycle{Transitions: []model.Transition{{Verb: "close", Obligations: []model.Obligation{
			{Scheme: "attestation", Kind: "countersign", Count: 1},
			{Scheme: "attestation", Kind: "countersign", Count: 2},
		}}}}
		if _, err := resolver.Resolve(context.Background(), request); err == nil || !strings.Contains(err.Error(), "exactly one") {
			t.Fatalf("Resolve error = %v, want malformed model error", err)
		}
	})

	t.Run("forge-unavailable reads are blocking unproven", func(t *testing.T) {
		for _, at := range []string{"discovery", "approvals"} {
			t.Run(at, func(t *testing.T) {
				resolver, request := lifecycleFixture(t, "story", "101", "900")
				resolver.Forge = unavailableLifecycleForge{Forge: resolver.Forge, at: at}
				result, err := resolver.Resolve(context.Background(), request)
				if err != nil {
					t.Fatalf("Resolve: %v", err)
				}
				if result.Verdict != countersign.VerdictUnproven || !containsLifecycleWitness(result.Witnesses, "forge") {
					t.Fatalf("result = %+v, want forge-unavailable unproven", result)
				}
			})
		}
	})

	t.Run("malformed provider snapshot remains operational", func(t *testing.T) {
		resolver, request := lifecycleFixture(t, "story", "101", "900")
		f := resolver.Forge.(*forgefake.Forge)
		snapshot, err := forge.NewApprovalSnapshot("github", "acme/widgets", "17", lifecycleCandidateSHA, forge.ProviderActor{Scheme: "github-user-id", Subject: "900"}, lifecycleNow().Add(-time.Minute), []forge.Approval{lifecycleApproval("1", "101")})
		if err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		snapshot.ProviderSnapshotID = "sha256:" + strings.Repeat("0", 64)
		f.SeedApprovalSnapshot("17", snapshot)
		if _, err := resolver.Resolve(context.Background(), request); err == nil || !strings.Contains(err.Error(), "provider snapshot") {
			t.Fatalf("Resolve error = %v, want malformed provider contract error", err)
		}
	})
}

type unavailableLifecycleForge struct {
	forge.Forge
	at string
}

func (f unavailableLifecycleForge) ListOpenMRs(ctx context.Context, target string) ([]forge.OpenMR, error) {
	if f.at == "discovery" {
		return nil, fmt.Errorf("read open merge requests: %w", forge.ErrUnavailable)
	}
	return f.Forge.ListOpenMRs(ctx, target)
}

func (f unavailableLifecycleForge) ListApprovals(ctx context.Context, change string) (forge.ApprovalSnapshot, error) {
	if f.at == "approvals" {
		return forge.ApprovalSnapshot{}, fmt.Errorf("read approvals: %w", forge.ErrUnavailable)
	}
	return f.Forge.ListApprovals(ctx, change)
}

func lifecycleFixture(t *testing.T, class, approver, author string) (Resolver, Request) {
	t.Helper()
	profile := lifecycleProfile(t)
	f := forgefake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "17", SourceBranch: "feature/candidate"})
	snapshot, err := forge.NewApprovalSnapshot("github", "acme/widgets", "17", lifecycleCandidateSHA, forge.ProviderActor{Scheme: "github-user-id", Subject: author}, lifecycleNow().Add(-time.Minute), []forge.Approval{lifecycleApproval("1", approver)})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	f.SeedApprovalSnapshot("17", snapshot)
	mdl := &model.Model{Lifecycle: map[string]model.Lifecycle{
		"story":   {Transitions: []model.Transition{{Verb: "close", Obligations: []model.Obligation{{Scheme: "attestation", Kind: "countersign", Count: 1}}}}},
		"feature": {Transitions: []model.Transition{{Verb: "close", Obligations: []model.Obligation{{Scheme: "attestation", Kind: "countersign", Count: 1}}}}},
	}}
	manifest := &store.Manifest{Countersign: &store.CountersignConfig{
		TrustSource: "forge-live", FreshnessPolicyID: "forge-current",
		MaximumObservationAgeSeconds: 300, MaximumApprovalAgeSeconds: 3600,
	}}
	resolver := Resolver{
		Forge: f,
		LoadProfile: func(string) (gp.Profile, error) {
			return profile, nil
		},
		Clock: lifecycleNow,
	}
	request := Request{
		Root: "/candidate", Manifest: manifest, Model: mdl, TargetClass: class,
		DefaultBranch: "main", SourceBranch: "feature/candidate", LocalCandidateSHA: lifecycleCandidateSHA,
	}
	return resolver, request
}

func lifecycleProfile(t *testing.T) gp.Profile {
	t.Helper()
	raw := []byte(`schema: verdi.governance-profile/v1
id: lifecycle
class: team
applicable_transitions: [close]
identity_trust_sources:
  - { id: forge-live, kind: forge }
role_mappings:
  - { role: story-review, trust_source: forge-live, subjects: ["101", "900"] }
  - { role: feature-uat, trust_source: forge-live, subjects: ["201", "202", "900"] }
ownership_sources: []
signature_requirements: []
required_approvers:
  - { transitions: [close], roles: [story-review, feature-uat], minimum: 1 }
distinctness_rules:
  - { transitions: [close], left_role: story-review, right_role: feature-uat, relation: different-principal }
evidence_source_restrictions: []
escalation_thresholds: []
`)
	profile, err := gp.DecodeProfile(raw, gp.Catalog{Roles: []string{"story-review", "feature-uat"}, Transitions: []string{"close"}})
	if err != nil {
		t.Fatalf("DecodeProfile: %v", err)
	}
	return profile
}

func lifecycleApproval(id, subject string) forge.Approval {
	stamp := lifecycleNow().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano)
	return forge.Approval{
		ApprovalID: id, ApprovalRef: "review/" + id, State: forge.ApprovalActive,
		ApprovedAt: stamp, UpdatedAt: stamp, CandidateSHA: lifecycleCandidateSHA,
		Actor:             forge.ProviderActor{Scheme: "github-user-id", Subject: subject},
		ProviderWitnesses: []forge.ProviderWitness{{Name: "review_id", Value: id}},
	}
}

func lifecycleNow() time.Time { return time.Date(2026, 8, 26, 17, 0, 0, 0, time.UTC) }

func containsLifecycleWitness(witnesses []string, part string) bool {
	for _, witness := range witnesses {
		if strings.Contains(witness, part) {
			return true
		}
	}
	return false
}
