package lifecyclecountersign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/forge"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

const lifecycleCandidateSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// lifecycleAcceptedCommit is the ONE pinned accepted default-branch
// commit every fixture below authenticates against.
const lifecycleAcceptedCommit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

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
		tests := []struct {
			name   string
			mutate func(*Resolver, *Request)
			want   string
		}{
			{"config", func(_ *Resolver, r *Request) { r.Manifest.Countersign = nil }, "config"},
			{"forge", func(r *Resolver, _ *Request) { r.Forge = nil }, "forge"},
			{"source branch", func(_ *Resolver, r *Request) { r.SourceBranch = "" }, "source-branch"},
			{"merge request", func(r *Resolver, _ *Request) { r.Forge = forgefake.New() }, "merge-request"},
			{"profile", func(_ *Resolver, r *Request) {
				r.AcceptedProfileSource = fstest.MapFS{}
			}, "profile"},
			{"accepted tree", func(_ *Resolver, r *Request) {
				r.AcceptedProfileSource, r.AcceptedCommit, r.AcceptedBranch = nil, "", ""
			}, "accepted-tree"},
			{"accepted branch is not the forge target", func(_ *Resolver, r *Request) {
				r.AcceptedBranch = "release/2"
			}, "accepted-tree"},
			{"configured trust source", func(_ *Resolver, r *Request) {
				r.Manifest.Countersign.TrustSource = "forge-unselected"
			}, "principal-authentication"},
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
	resolver := Resolver{Forge: f, Clock: lifecycleNow}
	request := Request{
		Root: "/candidate", Manifest: manifest, Model: mdl, TargetClass: class,
		DefaultBranch: "main", SourceBranch: "feature/candidate", LocalCandidateSHA: lifecycleCandidateSHA,
		AcceptedBranch: "main", AcceptedCommit: lifecycleAcceptedCommit,
		AcceptedProfileSource: lifecycleAcceptedSource(`"101", "900"`),
	}
	return resolver, request
}

// lifecycleAcceptedSource is the read-only accepted default-branch tree
// view a caller pins for the invocation: exactly the constitution store
// blobs, never a writable path.
func lifecycleAcceptedSource(storyReviewSubjects string) fstest.MapFS {
	source := fstest.MapFS{}
	for rel, content := range lifecyclePolicyFiles(storyReviewSubjects) {
		source[rel] = &fstest.MapFile{Data: []byte(content), Mode: 0o444}
	}
	return source
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

// lifecycleStoreConstitution is the smallest sealed constitution store
// fixture policyauthority accepts: one constitution selecting one
// governance profile whose catalog carries exactly this package's two
// lifecycle roles and the close transition.
const lifecycleStoreConstitution = `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Lifecycle countersign fixture constitution"
owners: [platform-team]
selected_profile: lifecycle
environments: [local, production]
catalog:
  roles: [feature-uat, story-review]
  transitions: [close]
  evidence_sources: []
  escalation_metrics: []
subjects:
  action: []
  configuration: []
  capability: []
  resource: []
  identity: []
  evidence: []
adapters:
  - id: codex
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
# Lifecycle countersign fixture
`

// lifecycleStoreProfileFormat is the stored selected profile; %s is the
// story-review subject list, the ONE field the accepted-tree and
// working-tree fixtures below differ in.
const lifecycleStoreProfileFormat = `---
schema: verdi.governance-profile/v1
id: lifecycle
class: team
applicable_transitions: [close]
identity_trust_sources:
  - {id: forge-live, kind: forge}
role_mappings:
  - {role: feature-uat, trust_source: forge-live, subjects: ["201", "202", "900"]}
  - {role: story-review, trust_source: forge-live, subjects: [%s]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [close], roles: [feature-uat, story-review], minimum: 1}
distinctness_rules:
  - {transitions: [close], left_role: feature-uat, right_role: story-review, relation: different-principal}
evidence_source_restrictions: []
escalation_thresholds: []
---
Hermetic lifecycle governance profile.
`

func lifecyclePolicyFiles(storyReviewSubjects string) map[string]string {
	return map[string]string{
		".verdi/policy/constitution.md":       lifecycleStoreConstitution,
		".verdi/policy/profiles/lifecycle.md": fmt.Sprintf(lifecycleStoreProfileFormat, storyReviewSubjects),
	}
}

// lifecycleWorkingTreeRoot writes a fully valid, adopted constitution
// store to a MUTABLE checkout root — the bytes an operator can edit at
// will, which this package must never read as governance authority.
func lifecycleWorkingTreeRoot(t *testing.T, storyReviewSubjects string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range lifecyclePolicyFiles(storyReviewSubjects) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

// TestResolveNeverReadsMutableCheckoutProfile is I-121's component
// falsifier: a checkout whose OWN .verdi/policy/ would prove countersign
// must not prove it when no accepted default-branch tree was pinned for
// the invocation. Governance authority is acceptance truth, never
// mutable-checkout state.
func TestResolveNeverReadsMutableCheckoutProfile(t *testing.T) {
	resolver, request := lifecycleFixture(t, "story", "101", "900")
	request.AcceptedProfileSource, request.AcceptedCommit, request.AcceptedBranch = nil, "", ""
	request.Root = lifecycleWorkingTreeRoot(t, `"101", "900"`)

	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Verdict == countersign.VerdictProven || result.Record != nil {
		t.Fatalf("result = %+v, want a non-proven verdict: working-tree profile bytes are not governance authority", result)
	}
}

// TestResolveAcceptedTreeProfileOverridesWorkingTree is I-121's second
// component falsifier: with an accepted tree pinned, a MUTATED checkout
// whose own .verdi/policy/ would refuse this approval cannot override the
// accepted profile, and a checkout carrying no policy at all cannot
// weaken it either.
func TestResolveAcceptedTreeProfileOverridesWorkingTree(t *testing.T) {
	for _, tc := range []struct {
		name string
		root string
	}{
		{"checkout profile refuses the approver", ""},
		{"checkout is not adopted at all", "unadopted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver, request := lifecycleFixture(t, "story", "101", "900")
			if tc.root == "" {
				request.Root = lifecycleWorkingTreeRoot(t, `"999", "900"`)
			} else {
				request.Root = t.TempDir()
			}
			result, err := resolver.Resolve(context.Background(), request)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if result.Verdict != countersign.VerdictProven || result.Record == nil {
				t.Fatalf("result = %+v, want the pinned accepted-tree profile to prove countersign", result)
			}
			if result.Record.Obligation.GovernanceProfileID != "lifecycle" {
				t.Fatalf("governance profile id = %q, want the accepted tree's own profile", result.Record.Obligation.GovernanceProfileID)
			}
		})
	}
}

// TestResolveMalformedAcceptedTreeIsOperational keeps the three-valued
// boundary honest: a MISSING accepted store is unproven (above), while a
// structurally malformed one is an operational failure, never a silent
// pass and never a countersign verdict.
func TestResolveMalformedAcceptedTreeIsOperational(t *testing.T) {
	resolver, request := lifecycleFixture(t, "story", "101", "900")
	source := lifecycleAcceptedSource(`"101", "900"`)
	source[".verdi/policy/constitution.md"] = &fstest.MapFile{Data: []byte("not a constitution\n"), Mode: 0o444}
	request.AcceptedProfileSource = source

	if _, err := resolver.Resolve(context.Background(), request); err == nil || !strings.Contains(err.Error(), lifecycleAcceptedCommit) {
		t.Fatalf("Resolve error = %v, want an operational failure naming the pinned accepted commit", err)
	}
}
