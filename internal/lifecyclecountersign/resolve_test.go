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
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
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

// --- SI-227: kernel-consulted separation of duties -------------------------
//
// The lifecycle resolver no longer hard-wires SeparationDifferentFromAuthor.
// It asks the governance kernel's authorization interpreter, using the
// selected profile's own rules for the close transition, whether the
// obligation's approver role must be filled by a principal different from
// the resolved candidate author. These fixtures exercise the three
// documented outcomes plus the local-operator refusal (SI-227's separately
// narrowed second finding).

// lifecycleSoloConstitution/lifecycleSoloProfile is a solo governance
// profile whose named author role and both close roles (story-review,
// feature-uat) map the SAME forge-authenticated subject ("900") and that
// declares NO distinctness rule at all — solo profiles are not required to
// declare one (validateClassCoverage). SI-233: the author mapping is the
// configuration that permits collapse, and the kernel's own role-collapse
// disclosure is the only source of "permitted with collapse", never
// profile.Class read in isolation.
const lifecycleSoloConstitution = `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Lifecycle countersign solo fixture constitution"
owners: [platform-team]
selected_profile: lifecycle-solo
environments: [local, production]
catalog:
  roles: [author, feature-uat, story-review]
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
# Lifecycle countersign solo fixture
`

const lifecycleSoloProfile = `---
schema: verdi.governance-profile/v1
id: lifecycle-solo
class: solo
applicable_transitions: [close]
identity_trust_sources:
  - {id: forge-live, kind: forge}
role_mappings:
  - {role: author, trust_source: forge-live, subjects: ["900"]}
  - {role: feature-uat, trust_source: forge-live, subjects: ["900"]}
  - {role: story-review, trust_source: forge-live, subjects: ["900"]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [close], roles: [feature-uat, story-review], minimum: 1}
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
Hermetic solo lifecycle governance profile: the owner's principal fills
the author role and both close roles, and the profile declares no
distinctness rule at all.
`

func lifecycleSoloAcceptedSource() fstest.MapFS {
	return fstest.MapFS{
		".verdi/policy/constitution.md":            &fstest.MapFile{Data: []byte(lifecycleSoloConstitution), Mode: 0o444},
		".verdi/policy/profiles/lifecycle-solo.md": &fstest.MapFile{Data: []byte(lifecycleSoloProfile), Mode: 0o444},
	}
}

// lifecycleSoloEscalationProfile is otherwise identical to
// lifecycleSoloProfile but ALSO declares an escalation threshold covering
// close — a rule wholly unrelated to separation of duties, but one the
// kernel-consulted probe cannot supply a metric value for. This is the
// "kernel answer unproven" case: the kernel's decision is not cleanly
// authorized, so the resolver fails closed to SeparationDifferentFromAuthor
// and discloses why, exactly as it would for an unreachable or malformed
// kernel operand.
const lifecycleSoloEscalationConstitution = `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Lifecycle countersign solo escalation fixture constitution"
owners: [platform-team]
selected_profile: lifecycle-solo-escalation
environments: [local, production]
catalog:
  roles: [author, feature-uat, story-review]
  transitions: [close]
  evidence_sources: []
  escalation_metrics: [risk]
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
# Lifecycle countersign solo escalation fixture
`

const lifecycleSoloEscalationProfile = `---
schema: verdi.governance-profile/v1
id: lifecycle-solo-escalation
class: solo
applicable_transitions: [close]
identity_trust_sources:
  - {id: forge-live, kind: forge}
role_mappings:
  - {role: author, trust_source: forge-live, subjects: ["900"]}
  - {role: feature-uat, trust_source: forge-live, subjects: ["900"]}
  - {role: story-review, trust_source: forge-live, subjects: ["900"]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [close], roles: [feature-uat, story-review], minimum: 1}
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds:
  - {transitions: [close], metric: risk, at_least: 0, required_roles: [story-review]}
---
Hermetic solo lifecycle governance profile declaring an unrelated
escalation threshold the kernel-consulted probe cannot satisfy.
`

func lifecycleSoloEscalationAcceptedSource() fstest.MapFS {
	return fstest.MapFS{
		".verdi/policy/constitution.md":                       &fstest.MapFile{Data: []byte(lifecycleSoloEscalationConstitution), Mode: 0o444},
		".verdi/policy/profiles/lifecycle-solo-escalation.md": &fstest.MapFile{Data: []byte(lifecycleSoloEscalationProfile), Mode: 0o444},
	}
}

// lifecycleHighAssuranceProfile is a minimal valid high-assurance profile
// (validateClassCoverage requires approval, distinctness, signature,
// ownership, and evidence-source rules for every applicable transition).
// It is never asked to prove those extra rules here — only that the
// kernel-consulted separation decision still refuses self-approval.
const lifecycleHighAssuranceConstitution = `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Lifecycle countersign high-assurance fixture constitution"
owners: [platform-team]
selected_profile: lifecycle-ha
environments: [local, production]
catalog:
  roles: [feature-uat, story-review]
  transitions: [close]
  evidence_sources: [forge]
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
# Lifecycle countersign high-assurance fixture
`

const lifecycleHighAssuranceProfile = `---
schema: verdi.governance-profile/v1
id: lifecycle-ha
class: high-assurance
applicable_transitions: [close]
identity_trust_sources:
  - {id: forge-live, kind: forge}
  - {id: signed-1, kind: signed-commit}
  - {id: owned-1, kind: ownership}
role_mappings:
  - {role: feature-uat, trust_source: forge-live, subjects: ["201", "202", "900"]}
  - {role: story-review, trust_source: forge-live, subjects: ["101", "900"]}
ownership_sources:
  - {id: own-1, trust_source: owned-1, transitions: [close], roles: [story-review]}
signature_requirements:
  - {transitions: [close], roles: [story-review], trust_sources: [signed-1]}
required_approvers:
  - {transitions: [close], roles: [feature-uat, story-review], minimum: 1}
distinctness_rules:
  - {transitions: [close], left_role: feature-uat, right_role: story-review, relation: different-principal}
evidence_source_restrictions:
  - {transitions: [close], allowed_sources: [forge]}
escalation_thresholds: []
---
Hermetic high-assurance lifecycle governance profile.
`

func lifecycleHighAssuranceAcceptedSource() fstest.MapFS {
	return fstest.MapFS{
		".verdi/policy/constitution.md":          &fstest.MapFile{Data: []byte(lifecycleHighAssuranceConstitution), Mode: 0o444},
		".verdi/policy/profiles/lifecycle-ha.md": &fstest.MapFile{Data: []byte(lifecycleHighAssuranceProfile), Mode: 0o444},
	}
}

// lifecycleLocalOperatorConstitution/lifecycleLocalOperatorProfile is a
// solo profile whose ONLY identity trust source is local-operator — a bare
// self-assertion, never independently verified (2026-09-05 local-operator
// disposition design). SI-227: that alone can never prove a close
// countersign.
const lifecycleLocalOperatorConstitution = `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Lifecycle countersign local-operator fixture constitution"
owners: [platform-team]
selected_profile: lifecycle-local
environments: [local, production]
catalog:
  roles: [story-review]
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
# Lifecycle countersign local-operator fixture
`

const lifecycleLocalOperatorProfile = `---
schema: verdi.governance-profile/v1
id: lifecycle-local
class: solo
applicable_transitions: [close]
identity_trust_sources:
  - {id: local-op, kind: local-operator}
role_mappings:
  - {role: story-review, trust_source: local-op, subjects: ["900"]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [close], roles: [story-review], minimum: 1}
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
Hermetic solo lifecycle governance profile whose only trust source is
local-operator.
`

func lifecycleLocalOperatorFixture(t *testing.T) (Resolver, Request) {
	t.Helper()
	f := forgefake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "17", SourceBranch: "feature/candidate"})
	snapshot, err := forge.NewApprovalSnapshot("github", "acme/widgets", "17", lifecycleCandidateSHA,
		forge.ProviderActor{Scheme: "github-user-id", Subject: "900"}, lifecycleNow().Add(-time.Minute),
		[]forge.Approval{lifecycleApproval("1", "900")})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	f.SeedApprovalSnapshot("17", snapshot)
	mdl := &model.Model{Lifecycle: map[string]model.Lifecycle{
		"story": {Transitions: []model.Transition{{Verb: "close", Obligations: []model.Obligation{{Scheme: "attestation", Kind: "countersign", Count: 1}}}}},
	}}
	manifest := &store.Manifest{Countersign: &store.CountersignConfig{
		TrustSource: "local-op", FreshnessPolicyID: "forge-current",
		MaximumObservationAgeSeconds: 300, MaximumApprovalAgeSeconds: 3600,
	}}
	resolver := Resolver{Forge: f, Clock: lifecycleNow}
	request := Request{
		Root: "/candidate", Manifest: manifest, Model: mdl, TargetClass: "story",
		DefaultBranch: "main", SourceBranch: "feature/candidate", LocalCandidateSHA: lifecycleCandidateSHA,
		AcceptedBranch: "main", AcceptedCommit: lifecycleAcceptedCommit,
		AcceptedProfileSource: fstest.MapFS{
			".verdi/policy/constitution.md":             &fstest.MapFile{Data: []byte(lifecycleLocalOperatorConstitution), Mode: 0o444},
			".verdi/policy/profiles/lifecycle-local.md": &fstest.MapFile{Data: []byte(lifecycleLocalOperatorProfile), Mode: 0o444},
		},
	}
	return resolver, request
}

// TestResolveKernelSeparationSoloCollapse is the "permitted with collapse"
// outcome: a solo profile whose close roles map the author's own forge
// principal — GitLab-shaped self-approval is a legitimate provider fact,
// unlike GitHub's structural refusal. The kernel's solo role-collapse
// disclosure must reach the countersign record's witnesses.
func TestResolveKernelSeparationSoloCollapse(t *testing.T) {
	resolver, request := lifecycleFixture(t, "story", "900", "900")
	request.AcceptedProfileSource = lifecycleSoloAcceptedSource()

	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Verdict != countersign.VerdictProven || result.Record == nil {
		t.Fatalf("result = %+v, want a proven solo-collapse record", result)
	}
	if result.Record.Obligation.SeparationRule != countersign.SeparationNone {
		t.Fatalf("separation rule = %q, want %q", result.Record.Obligation.SeparationRule, countersign.SeparationNone)
	}
	if !containsLifecycleWitness(result.Record.Witnesses, `kernel-separation-probe:author-as-approver:collapse-permitted:kernel_disclosure="solo-role-collapse":`) {
		t.Fatalf("record witnesses = %v, want the kernel's solo role-collapse disclosure on the collapse-permitted probe witness", result.Record.Witnesses)
	}
}

// TestResolveKernelSeparationTeamAndHighAssuranceRefuseSelfApproval is the
// "required" outcome for the two profile classes the ledger names
// explicitly: an author's own approval never counts, and the separation
// verdict is violated with a witness — today's AC-3 behavior, unchanged.
func TestResolveKernelSeparationTeamAndHighAssuranceRefuseSelfApproval(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source fstest.MapFS
	}{
		{"team", lifecycleAcceptedSource(`"101", "900"`)},
		{"high-assurance", lifecycleHighAssuranceAcceptedSource()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver, request := lifecycleFixture(t, "story", "900", "900")
			request.AcceptedProfileSource = tc.source

			result, err := resolver.Resolve(context.Background(), request)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if result.Verdict != countersign.VerdictViolated || result.Record == nil {
				t.Fatalf("result = %+v, want a violated self-approval record", result)
			}
			if result.Record.Obligation.SeparationRule != countersign.SeparationDifferentFromAuthor {
				t.Fatalf("separation rule = %q, want %q", result.Record.Obligation.SeparationRule, countersign.SeparationDifferentFromAuthor)
			}
			if !containsLifecycleWitness(result.Record.Witnesses, "approval-separation:") {
				t.Fatalf("record witnesses = %v, want a self-approval separation witness", result.Record.Witnesses)
			}
		})
	}
}

// TestResolveKernelSeparationUnavailableFailsClosed is the "unavailable or
// unproven kernel answer" outcome: a solo profile whose OWN rules declare
// an escalation threshold the kernel-consulted probe cannot satisfy, so the
// decision is not cleanly authorized. The resolver keeps
// SeparationDifferentFromAuthor and discloses why, and the downstream
// self-approval refusal fires exactly as it does for team/high-assurance.
func TestResolveKernelSeparationUnavailableFailsClosed(t *testing.T) {
	resolver, request := lifecycleFixture(t, "story", "900", "900")
	request.AcceptedProfileSource = lifecycleSoloEscalationAcceptedSource()

	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Verdict != countersign.VerdictViolated || result.Record == nil {
		t.Fatalf("result = %+v, want a fail-closed violated record", result)
	}
	if result.Record.Obligation.SeparationRule != countersign.SeparationDifferentFromAuthor {
		t.Fatalf("separation rule = %q, want the fail-closed %q", result.Record.Obligation.SeparationRule, countersign.SeparationDifferentFromAuthor)
	}
	if !containsLifecycleWitness(result.Record.Witnesses, `kernel-separation-probe:author-as-approver:separation-required:kernel_answer="unproven":`) {
		t.Fatalf("record witnesses = %v, want a separation-required probe witness disclosing the unproven kernel answer", result.Record.Witnesses)
	}
}

// TestResolveLocalOperatorOnlyTrustSourceNeverProvesCountersign is SI-227's
// separately narrowed second finding: a local-operator identity alone
// never proves a close countersign, with its own witness.
func TestResolveLocalOperatorOnlyTrustSourceNeverProvesCountersign(t *testing.T) {
	resolver, request := lifecycleLocalOperatorFixture(t)

	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Verdict != countersign.VerdictUnproven || result.Record != nil {
		t.Fatalf("result = %+v, want an unproven local-operator refusal", result)
	}
	if !containsLifecycleWitness(result.Witnesses, "principal-authentication") {
		t.Fatalf("witnesses = %v, want a principal-authentication witness", result.Witnesses)
	}
	if !containsLifecycleWitness(result.Witnesses, "local-operator") {
		t.Fatalf("witnesses = %v, want it to name local-operator", result.Witnesses)
	}
}

// unprovenTrustFactReader always reports an unavailable trust fact — the
// TestKernelSeparationRuleAuthorNotAuthenticated fixture below needs a
// validly-sealed non-authenticated PrincipalResolution, which only
// gp.Resolver.Resolve can mint.
type unprovenTrustFactReader struct{}

func (unprovenTrustFactReader) ReadTrustFact(_ context.Context, source gp.TrustSource, _ gp.PrincipalClaim) (gp.TrustFact, error) {
	return gp.TrustFact{SourceID: source.ID, SourceKind: source.Kind, Available: false, Reason: "test: evidence unavailable"}, nil
}

// TestKernelSeparationRuleAuthorNotAuthenticated is kernelSeparationRule's
// own unit boundary for an operand no lifecycle fixture's providerFacts
// bridge can produce (the candidate author's claim.Subject is always
// echoed back present by construction): a non-authenticated author makes
// the kernel's answer unavailable outright, so the helper fails closed
// without even calling gp.Authorize.
func TestKernelSeparationRuleAuthorNotAuthenticated(t *testing.T) {
	profile, err := loadSelectedProfile(lifecycleSoloAcceptedSource())
	if err != nil {
		t.Fatalf("loadSelectedProfile: %v", err)
	}
	resolver := gp.NewResolver(unprovenTrustFactReader{})
	author, err := resolver.Resolve(context.Background(), profile, gp.PrincipalClaim{TrustSource: "forge-live", Subject: "900"})
	if err != nil {
		t.Fatalf("resolve author: %v", err)
	}
	if author.State != gp.ResolutionUnproven {
		t.Fatalf("author.State = %q, want unproven", author.State)
	}

	rule, witnesses := kernelSeparationRule(profile, "story-review", author)
	if rule != countersign.SeparationDifferentFromAuthor {
		t.Fatalf("rule = %q, want the fail-closed %q", rule, countersign.SeparationDifferentFromAuthor)
	}
	want := `kernel-separation-probe:author-as-approver:separation-required:kernel_answer="unproven":reason="author-not-authenticated":detail="candidate author principal is not authenticated"`
	if len(witnesses) != 1 || witnesses[0] != want {
		t.Fatalf("witnesses = %v, want exactly [%s]", witnesses, want)
	}
}

// --- SI-233: the kernel is asked exactly one question ----------------------
//
// kernelSeparationRule asks the kernel whether the candidate author's
// principal may fill exactly two roles for the close transition: the named
// author role and the obligation's approver role. The fixtures below vary
// only the selected solo profile, so each case isolates what the profile's
// own rules make the kernel answer.

// kernelProbeCatalogRoles is every role the SI-233 cases name, so one
// constitution serves each case's profile.
const kernelProbeCatalogRoles = "author, feature-uat, policy-owner, story-review"

// kernelProbeSource is the accepted tree for one SI-233 case: a
// constitution whose catalog carries kernelProbeCatalogRoles, and one
// selected profile of class whose role mappings and distinctness rules are
// the YAML fragments the case varies. Every profile trusts only the forge
// source and requires one story-review approver for close.
func kernelProbeSource(class, roleMappings, distinctness string) fstest.MapFS {
	return kernelProbeSourceRequiring(class, roleMappings, distinctness, "story-review")
}

// kernelProbeSourceRequiring is kernelProbeSource with the one role the
// profile's close approver rule requires named by requiredRole.
func kernelProbeSourceRequiring(class, roleMappings, distinctness, requiredRole string) fstest.MapFS {
	constitution := fmt.Sprintf(`---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Lifecycle countersign kernel probe fixture constitution"
owners: [platform-team]
selected_profile: lifecycle-probe
environments: [local, production]
catalog:
  roles: [%s]
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
# Lifecycle countersign kernel probe fixture
`, kernelProbeCatalogRoles)
	profile := fmt.Sprintf(`---
schema: verdi.governance-profile/v1
id: lifecycle-probe
class: %s
applicable_transitions: [close]
identity_trust_sources:
  - {id: forge-live, kind: forge}
role_mappings:
%s
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [close], roles: [%s], minimum: 1}
distinctness_rules: %s
evidence_source_restrictions: []
escalation_thresholds: []
---
Hermetic kernel probe lifecycle governance profile.
`, class, roleMappings, requiredRole, distinctness)
	return fstest.MapFS{
		".verdi/policy/constitution.md":             &fstest.MapFile{Data: []byte(constitution), Mode: 0o444},
		".verdi/policy/profiles/lifecycle-probe.md": &fstest.MapFile{Data: []byte(profile), Mode: 0o444},
	}
}

// kernelProbeMapping is one forge-live role mapping line of a probe
// profile.
func kernelProbeMapping(role string, subjects ...string) string {
	quoted := make([]string, len(subjects))
	for i, subject := range subjects {
		quoted[i] = fmt.Sprintf("%q", subject)
	}
	return fmt.Sprintf("  - {role: %s, trust_source: forge-live, subjects: [%s]}", role, strings.Join(quoted, ", "))
}

// kernelProbeMappings joins mapping lines into a role_mappings block.
func kernelProbeMappings(lines ...string) string { return strings.Join(lines, "\n") }

// resolveKernelProbe resolves the story countersign for one approval by
// approver on a change authored by author, under source's selected
// profile, and requires a canonical record.
func resolveKernelProbe(t *testing.T, approver, author string, source fstest.MapFS) Result {
	t.Helper()
	resolver, request := lifecycleFixture(t, "story", approver, author)
	request.AcceptedProfileSource = source
	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Record == nil {
		t.Fatalf("result = %+v, want a canonical countersign record", result)
	}
	return result
}

// kernelSeparationLines returns every witness the kernel-consulted
// separation decision contributed, in record order.
func kernelSeparationLines(witnesses []string) []string {
	lines := []string{}
	for _, witness := range witnesses {
		if strings.HasPrefix(witness, "kernel-separation") {
			lines = append(lines, witness)
		}
	}
	return lines
}

// TestResolveKernelSeparationAsksOnlyAuthorAndApproverRoles is SI-233's
// falsifier (L2a review I-1, probes P3 and P4): the kernel is asked only
// whether the author may fill the author role and the approver role, so a
// mapping of any unrelated role (policy-owner here) never changes who may
// approve, in either direction, and a solo profile mapping the author role
// and the approver role to the owner permits collapse.
func TestResolveKernelSeparationAsksOnlyAuthorAndApproverRoles(t *testing.T) {
	unrelated := kernelProbeMapping("policy-owner", "900")
	for _, tc := range []struct {
		name        string
		mappings    []string
		wantRule    countersign.SeparationRule
		wantVerdict countersign.Verdict
		wantLine    string
	}{
		{
			name:        "approver role only",
			mappings:    []string{kernelProbeMapping("story-review", "900")},
			wantRule:    countersign.SeparationDifferentFromAuthor,
			wantVerdict: countersign.VerdictViolated,
			wantLine:    `"role-not-authorized":role="author"`,
		},
		{
			name:        "author and approver roles",
			mappings:    []string{kernelProbeMapping("author", "900"), kernelProbeMapping("story-review", "900")},
			wantRule:    countersign.SeparationNone,
			wantVerdict: countersign.VerdictProven,
			wantLine:    `roles=["author" "story-review"]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			alone := resolveKernelProbe(t, "900", "900", kernelProbeSource("solo", kernelProbeMappings(tc.mappings...), "[]"))
			withUnrelated := resolveKernelProbe(t, "900", "900", kernelProbeSource("solo", kernelProbeMappings(append(append([]string{}, tc.mappings...), unrelated)...), "[]"))
			for _, run := range []struct {
				name   string
				result Result
			}{{"alone", alone}, {"with unrelated policy-owner mapping", withUnrelated}} {
				if run.result.Verdict != tc.wantVerdict || run.result.Record.Obligation.SeparationRule != tc.wantRule {
					t.Errorf("%s: verdict=%q rule=%q, want %q %q; witnesses=%v", run.name, run.result.Verdict, run.result.Record.Obligation.SeparationRule, tc.wantVerdict, tc.wantRule, run.result.Record.Witnesses)
				}
				if !containsLifecycleWitness(kernelSeparationLines(run.result.Record.Witnesses), tc.wantLine) {
					t.Errorf("%s: kernel separation witnesses = %v, want one containing %s", run.name, kernelSeparationLines(run.result.Record.Witnesses), tc.wantLine)
				}
			}
			if got, want := strings.Join(kernelSeparationLines(withUnrelated.Record.Witnesses), "\n"), strings.Join(kernelSeparationLines(alone.Record.Witnesses), "\n"); got != want {
				t.Errorf("an unrelated role mapping changed the kernel separation answer:\nwith:\n%s\nwithout:\n%s", got, want)
			}
		})
	}
}

// TestResolveKernelSeparationSoloRulesKeepSelfApprovalRefused is the
// wrongful-permit falsifier (L2a review I-2, probes P1 and P2): a solo
// profile whose own rules do not let the author also approve keeps
// different-from-author, so the author's own approval never proves the
// countersign.
func TestResolveKernelSeparationSoloRulesKeepSelfApprovalRefused(t *testing.T) {
	for _, tc := range []struct {
		name         string
		mappings     []string
		distinctness string
		wantReason   string
	}{
		{
			name:         "different-principal rule between author and approver",
			mappings:     []string{kernelProbeMapping("author", "900"), kernelProbeMapping("story-review", "900")},
			distinctness: "\n  - {transitions: [close], left_role: author, right_role: story-review, relation: different-principal}",
			wantReason:   `"distinctness-violated"`,
		},
		{
			name:         "different-principal rule between the two approver roles",
			mappings:     []string{kernelProbeMapping("author", "900"), kernelProbeMapping("feature-uat", "900"), kernelProbeMapping("story-review", "900")},
			distinctness: "\n  - {transitions: [close], left_role: feature-uat, right_role: story-review, relation: different-principal}",
			wantReason:   `"distinctness-unproven"`,
		},
		{
			name:         "approver role mapped only to another principal",
			mappings:     []string{kernelProbeMapping("author", "900"), kernelProbeMapping("story-review", "101")},
			distinctness: "[]",
			wantReason:   `"role-not-authorized":role="story-review"`,
		},
		{
			name:         "approver role unmapped",
			mappings:     []string{kernelProbeMapping("author", "900")},
			distinctness: "[]",
			wantReason:   `"role-not-authorized":role="story-review"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := resolveKernelProbe(t, "900", "900", kernelProbeSource("solo", kernelProbeMappings(tc.mappings...), tc.distinctness))
			if result.Record.Obligation.SeparationRule != countersign.SeparationDifferentFromAuthor {
				t.Fatalf("separation rule = %q, want %q; witnesses=%v", result.Record.Obligation.SeparationRule, countersign.SeparationDifferentFromAuthor, result.Record.Witnesses)
			}
			if result.Verdict == countersign.VerdictProven {
				t.Fatalf("self-approval proved the countersign: witnesses=%v", result.Record.Witnesses)
			}
			if !containsLifecycleWitness(kernelSeparationLines(result.Record.Witnesses), tc.wantReason) {
				t.Fatalf("kernel separation witnesses = %v, want one naming %s", kernelSeparationLines(result.Record.Witnesses), tc.wantReason)
			}
		})
	}
}

// --- L2a review I-3: witnesses describe the probe, never a verdict ---------

// lifecyclePrincipalID is the forge-live principal ID a subject derives.
func lifecyclePrincipalID(t *testing.T, subject string) gp.PrincipalID {
	t.Helper()
	id, err := gp.CanonicalPrincipalID("forge-live", subject)
	if err != nil {
		t.Fatalf("principal id: %v", err)
	}
	return id
}

// TestResolveKernelSeparationWitnessesDescribeTheProbe pins the exact
// witness lines the separation probe contributes to the canonical record
// (and so to its digest and closure rollups). Each records the kernel's
// answer to the hypothetical author-as-approver request: never led by the
// countersign verdict vocabulary, never asserting a violation by the
// author, and never reading as a collapse that occurred when a different
// principal approved (L2a review I-3, probes P5 and P8). An unproven
// kernel answer says so explicitly (L2a review m-3).
func TestResolveKernelSeparationWitnessesDescribeTheProbe(t *testing.T) {
	author := lifecyclePrincipalID(t, "900")
	for _, tc := range []struct {
		name        string
		approver    string
		source      fstest.MapFS
		wantVerdict countersign.Verdict
		wantRule    countersign.SeparationRule
		wantLines   []string
	}{
		{
			name:        "team record approved by an independent principal",
			approver:    "101",
			source:      lifecycleAcceptedSource(`"101", "900"`),
			wantVerdict: countersign.VerdictProven,
			wantRule:    countersign.SeparationDifferentFromAuthor,
			wantLines: []string{
				`kernel-separation-probe:author-as-approver:separation-required:kernel_answer="refused":reason="distinctness-unproven":role="feature-uat":roles=["feature-uat" "story-review"]:detail="different-principal rule between \"feature-uat\" and \"story-review\": role \"feature-uat\" has no authenticated filler"`,
				`kernel-separation-probe:author-as-approver:separation-required:kernel_answer="refused":reason="role-not-authorized":role="author":roles=[]:detail="no role mapping grants role \"author\" to this principal"`,
			},
		},
		{
			name:        "solo record approved by a different principal",
			approver:    "101",
			source:      kernelProbeSource("solo", kernelProbeMappings(kernelProbeMapping("author", "900"), kernelProbeMapping("story-review", "101", "900")), "[]"),
			wantVerdict: countersign.VerdictProven,
			wantRule:    countersign.SeparationNone,
			wantLines: []string{
				fmt.Sprintf(`kernel-separation-probe:author-as-approver:collapse-permitted:kernel_disclosure="solo-role-collapse":author_principal_id=%q:roles=["author" "story-review"]`, author),
			},
		},
		{
			name:        "solo self-approval under a profile permitting collapse",
			approver:    "900",
			source:      lifecycleSoloAcceptedSource(),
			wantVerdict: countersign.VerdictProven,
			wantRule:    countersign.SeparationNone,
			wantLines: []string{
				fmt.Sprintf(`kernel-separation-probe:author-as-approver:collapse-permitted:kernel_disclosure="solo-role-collapse":author_principal_id=%q:roles=["author" "story-review"]`, author),
			},
		},
		{
			name:        "solo self-approval with an unproven kernel answer",
			approver:    "900",
			source:      lifecycleSoloEscalationAcceptedSource(),
			wantVerdict: countersign.VerdictViolated,
			wantRule:    countersign.SeparationDifferentFromAuthor,
			wantLines: []string{
				`kernel-separation-probe:author-as-approver:separation-required:kernel_answer="unproven":reason="escalation-metric-unavailable":role="":roles=[]:detail="no value supplied for escalation metric \"risk\""`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := resolveKernelProbe(t, tc.approver, "900", tc.source)
			if result.Verdict != tc.wantVerdict || result.Record.Obligation.SeparationRule != tc.wantRule {
				t.Errorf("verdict=%q rule=%q, want %q %q", result.Verdict, result.Record.Obligation.SeparationRule, tc.wantVerdict, tc.wantRule)
			}
			got := kernelSeparationLines(result.Record.Witnesses)
			if strings.Join(got, "\n") != strings.Join(tc.wantLines, "\n") {
				t.Errorf("kernel separation witnesses:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tc.wantLines, "\n"))
			}
			for _, line := range got {
				if strings.Contains(line, string(countersign.VerdictViolated)) || strings.Contains(line, `"`+string(countersign.VerdictProven)+`"`) {
					t.Errorf("probe witness %q speaks the countersign verdict vocabulary", line)
				}
			}
		})
	}
}

// lifecycleAuthenticatedAuthor mints the kernel's sealed, authenticated
// resolution of the forge-live subject under profile, through the same
// provider-fact bridge Resolve uses.
func lifecycleAuthenticatedAuthor(t *testing.T, profile gp.Profile, subject string) gp.PrincipalResolution {
	t.Helper()
	snapshot, err := forge.NewApprovalSnapshot("github", "acme/widgets", "17", lifecycleCandidateSHA, forge.ProviderActor{Scheme: "github-user-id", Subject: subject}, lifecycleNow().Add(-time.Minute), []forge.Approval{})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	author, err := gp.NewResolver(providerFacts{snapshot: snapshot}).Resolve(context.Background(), profile, gp.PrincipalClaim{TrustSource: "forge-live", Subject: subject})
	if err != nil {
		t.Fatalf("resolve author: %v", err)
	}
	if author.State != gp.ResolutionAuthenticated {
		t.Fatalf("author.State = %q, want authenticated", author.State)
	}
	return author
}

// TestKernelSeparationRuleAuthorizedWithoutCollapseDisclosureKeepsSeparation
// is kernelSeparationRule's unit boundary for an authorized kernel decision
// that carries no solo role-collapse disclosure: asked about the author
// role alone (an approver role equal to the author role), the kernel
// authorizes but discloses no collapse of two roles, so the decision alone
// never permits collapse.
func TestKernelSeparationRuleAuthorizedWithoutCollapseDisclosureKeepsSeparation(t *testing.T) {
	profile, err := loadSelectedProfile(kernelProbeSourceRequiring("solo", kernelProbeMapping("author", "900"), "[]", "author"))
	if err != nil {
		t.Fatalf("loadSelectedProfile: %v", err)
	}
	author := lifecycleAuthenticatedAuthor(t, profile, "900")

	rule, witnesses := kernelSeparationRule(profile, kernelAuthorRole, author)
	if rule != countersign.SeparationDifferentFromAuthor {
		t.Fatalf("rule = %q, want %q", rule, countersign.SeparationDifferentFromAuthor)
	}
	want := `kernel-separation-probe:author-as-approver:separation-required:kernel_answer="authorized":reason="collapse-not-disclosed":detail="the kernel disclosed no solo role collapse naming this principal for exactly the author role and the approver role"`
	if len(witnesses) != 1 || witnesses[0] != want {
		t.Fatalf("witnesses = %v, want [%s]", witnesses, want)
	}
}

// TestAuthorApproverCollapse is authorApproverCollapse's own boundary:
// only a solo role-collapse disclosure naming the author's principal for
// exactly the author role and the approver role permits collapse.
func TestAuthorApproverCollapse(t *testing.T) {
	author := lifecyclePrincipalID(t, "900")
	other := lifecyclePrincipalID(t, "101")
	collapse := func(principal gp.PrincipalID, roles ...string) gp.Disclosure {
		return gp.Disclosure{Code: gp.ReasonSoloRoleCollapse, PrincipalID: principal, Roles: roles}
	}
	for _, tc := range []struct {
		name        string
		disclosures []gp.Disclosure
		want        bool
	}{
		{"exactly the author and approver roles", []gp.Disclosure{collapse(author, "author", "story-review")}, true},
		{"the same two roles in another order", []gp.Disclosure{collapse(author, "story-review", "author")}, true},
		{"the matching disclosure beside another principal's", []gp.Disclosure{collapse(other, "author", "story-review"), collapse(author, "author", "story-review")}, true},
		{"no disclosure", nil, false},
		{"an unrelated role pair", []gp.Disclosure{collapse(author, "policy-owner", "story-review")}, false},
		{"the pair plus an unrelated role", []gp.Disclosure{collapse(author, "author", "policy-owner", "story-review")}, false},
		{"another approver role", []gp.Disclosure{collapse(author, "author", "feature-uat")}, false},
		{"another principal", []gp.Disclosure{collapse(other, "author", "story-review")}, false},
		{"another disclosure code", []gp.Disclosure{{Code: gp.ReasonTrustSubjectVerified, PrincipalID: author, Roles: []string{"author", "story-review"}}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decision := gp.AuthorizationDecision{State: gp.AuthorizationAuthorized, Disclosures: tc.disclosures}
			d, ok := authorApproverCollapse(decision, author, "story-review")
			if ok != tc.want {
				t.Fatalf("authorApproverCollapse = %v, want %v", ok, tc.want)
			}
			if ok && d.PrincipalID != author {
				t.Fatalf("disclosure principal = %q, want %q", d.PrincipalID, author)
			}
		})
	}
}

// TestKernelSeparationRuleKernelErrorFailsClosed is kernelSeparationRule's
// unit boundary for a gp.Authorize error (L2a review m-1), which sealed
// Resolve inputs never reach: a profile that did not come unmodified from
// DecodeProfile makes the kernel refuse to interpret it, so the rule stays
// different-from-author and the disclosure names the kernel's error. The
// modified profile would otherwise permit collapse, so only the error
// stands between it and SeparationNone.
func TestKernelSeparationRuleKernelErrorFailsClosed(t *testing.T) {
	sealed, err := loadSelectedProfile(lifecycleSoloAcceptedSource())
	if err != nil {
		t.Fatalf("loadSelectedProfile: %v", err)
	}
	author := lifecycleAuthenticatedAuthor(t, sealed, "900")
	if rule, _ := kernelSeparationRule(sealed, "story-review", author); rule != countersign.SeparationNone {
		t.Fatalf("sealed profile rule = %q, want %q: the fixture must permit collapse", rule, countersign.SeparationNone)
	}
	modified := sealed
	modified.ID = sealed.ID + "-modified"

	for _, tc := range []struct {
		name    string
		profile gp.Profile
	}{
		{"never decoded", gp.Profile{Class: gp.ClassSolo}},
		{"modified after decode", modified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, kernelErr := gp.Authorize(tc.profile, gp.AuthorizationRequest{Transition: kernelCloseTransition, Posture: gp.PostureAuthoritative})
			if kernelErr == nil || !strings.Contains(kernelErr.Error(), "DecodeProfile") {
				t.Fatalf("gp.Authorize error = %v, want the kernel's seal refusal", kernelErr)
			}
			rule, witnesses := kernelSeparationRule(tc.profile, "story-review", author)
			if rule != countersign.SeparationDifferentFromAuthor {
				t.Fatalf("rule = %q, want the fail-closed %q", rule, countersign.SeparationDifferentFromAuthor)
			}
			want := fmt.Sprintf(`kernel-separation-probe:author-as-approver:separation-required:kernel_answer="unproven":reason="kernel-error":detail=%q`, kernelErr.Error())
			if len(witnesses) != 1 || witnesses[0] != want {
				t.Fatalf("witnesses = %v, want exactly [%s]", witnesses, want)
			}
		})
	}
}
