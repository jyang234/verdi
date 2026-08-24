package experimentapp

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

type trustFactReaderFunc func(context.Context, governanceprincipal.TrustSource, governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error)

func (f trustFactReaderFunc) ReadTrustFact(ctx context.Context, source governanceprincipal.TrustSource, claim governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error) {
	return f(ctx, source, claim)
}

func TestDirectEditReconciliationIsExplicitHumanOnlyAndContentPreserving(t *testing.T) {
	root, service := mutationTestService(t)
	marker := filepath.Join(filepath.Dir(mutationDefinitionPath(root)), "direct-note.txt")
	if err := os.WriteFile(marker, []byte("direct edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotWorktree(t, root)
	review := service.ReviewRegistration(context.Background(), testIdentity(t, root, "request-path-v2"))
	if review.Outcome.Classification != ClassificationVerdict || review.Outcome.Code != "direct-draft-unreconciled" {
		t.Fatalf("ReviewRegistration(direct edit) outcome = %+v", review.Outcome)
	}

	agent := service.ReconcileDraft(context.Background(), testIdentity(t, root, "request-path-v2"))
	if agent.Outcome.Classification != ClassificationVerdict || agent.Outcome.Code != "human-actor-required" {
		t.Fatalf("ReconcileDraft(agent) outcome = %+v", agent.Outcome)
	}

	identity := testIdentity(t, root, "request-path-v2")
	identity.Actor = authenticatedHuman(t)
	reconciled := service.ReconcileDraft(context.Background(), identity)
	if reconciled.Outcome.Classification != ClassificationClean {
		t.Fatalf("ReconcileDraft(human) outcome = %+v", reconciled.Outcome)
	}
	after := snapshotWorktree(t, root)
	delete(after, filepath.ToSlash(filepath.Join(".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v2", experiment.ProvenanceFile)))
	if !mapsEqual(after, before) {
		t.Fatalf("reconciliation mutated draft content\nbefore=%#v\nafter=%#v", before, after)
	}
	records := mutationProvenance(t, root)
	if len(records) != 1 || records[0].Operation != experiment.MutationReconcileDirect || !records[0].Attribution.Unauthenticated || records[0].Harness != "" || records[0].Session != "" {
		t.Fatalf("reconciliation provenance = %+v", records)
	}
	review = service.ReviewRegistration(context.Background(), identity)
	if review.Outcome.Classification != ClassificationClean {
		t.Fatalf("ReviewRegistration(reconciled) outcome = %+v", review.Outcome)
	}
}

func TestRegistrationRequiresSealedHumanExactPacketAndAcceptedPair(t *testing.T) {
	root, service := mutationTestService(t)
	agentIdentity := testIdentity(t, root, "request-path-v2")
	review := service.ReviewRegistration(context.Background(), agentIdentity)
	if review.Outcome.Classification != ClassificationClean {
		t.Fatalf("ReviewRegistration() outcome = %+v", review.Outcome)
	}

	agent := service.ProposeRegistration(context.Background(), agentIdentity, RegistrationInput{ReviewPacketDigest: review.PacketDigest})
	if agent.Outcome.Classification != ClassificationVerdict || agent.Outcome.Code != "human-actor-required" {
		t.Fatalf("ProposeRegistration(agent) outcome = %+v", agent.Outcome)
	}
	humanIdentity := agentIdentity
	humanIdentity.Actor = authenticatedHuman(t)
	wrong := service.ProposeRegistration(context.Background(), humanIdentity, RegistrationInput{ReviewPacketDigest: rawDigest([]byte("wrong"))})
	if wrong.Outcome.Classification != ClassificationVerdict || wrong.Outcome.Code != "review-packet-mismatch" {
		t.Fatalf("ProposeRegistration(wrong packet) outcome = %+v", wrong.Outcome)
	}

	proposal := service.ProposeRegistration(context.Background(), humanIdentity, RegistrationInput{ReviewPacketDigest: review.PacketDigest})
	if proposal.Outcome.Classification != ClassificationClean || proposal.Accepted {
		t.Fatalf("ProposeRegistration() = %+v", proposal)
	}
	definition, err := experiment.DecodeDefinition(mustReadFile(t, mutationDefinitionPath(root)))
	if err != nil {
		t.Fatal(err)
	}
	locked, err := experiment.Locked(definition)
	if err != nil || !locked {
		t.Fatalf("proposed definition locked=%v err=%v", locked, err)
	}
	records := mutationProvenance(t, root)
	if len(records) != 1 || records[0].Operation != experiment.MutationProposeRegistration || records[0].Attribution.PrincipalID == "" {
		t.Fatalf("registration provenance = %+v", records)
	}

	accepted := service.AcceptedRegistration(context.Background(), humanIdentity)
	if accepted.Outcome.Classification != ClassificationVerdict || accepted.Outcome.Code != "registration-not-accepted" {
		t.Fatalf("AcceptedRegistration(proposal only) = %+v", accepted)
	}
	completeGit := gitFromExperimentDir(t, root, "request-path-v2")
	incompleteGit := gitFromExperimentDir(t, root, "request-path-v2")
	provenancePath := acceptedExperimentFilePath("request-path-v2", experiment.ProvenanceFile)
	delete(incompleteGit.blobs, provenancePath)
	for index, entry := range incompleteGit.entries {
		if entry.Path == provenancePath {
			incompleteGit.entries = append(incompleteGit.entries[:index], incompleteGit.entries[index+1:]...)
			break
		}
	}
	service.git = incompleteGit
	accepted = service.AcceptedRegistration(context.Background(), humanIdentity)
	if accepted.Outcome.Classification != ClassificationVerdict || accepted.Outcome.Code != "registration-not-accepted" {
		t.Fatalf("AcceptedRegistration(incomplete pair) = %+v", accepted)
	}
	service.git = completeGit
	accepted = service.AcceptedRegistration(context.Background(), humanIdentity)
	if accepted.Outcome.Classification != ClassificationClean || !accepted.Accepted || accepted.DefinitionDigest != proposal.DefinitionDigest {
		t.Fatalf("AcceptedRegistration(accepted pair) = %+v", accepted)
	}
}

func TestRegistrationLockedImmutabilityAndExpectedHead(t *testing.T) {
	root, service := mutationTestService(t)
	identity := testIdentity(t, root, "request-path-v2")
	stale := identity
	stale.ExpectedAcceptedHEAD = oldHead
	definitionBytes := mustReadFile(t, mutationDefinitionPath(root))
	result := service.DraftDefinition(context.Background(), stale, DraftDefinitionInput{DefinitionBytes: definitionBytes})
	if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "accepted-head-stale" {
		t.Fatalf("DraftDefinition(stale HEAD) outcome = %+v", result.Outcome)
	}

	review := service.ReviewRegistration(context.Background(), identity)
	human := identity
	human.Actor = authenticatedHuman(t)
	locked := service.ProposeRegistration(context.Background(), human, RegistrationInput{ReviewPacketDigest: review.PacketDigest})
	if locked.Outcome.Classification != ClassificationClean {
		t.Fatalf("ProposeRegistration() outcome = %+v", locked.Outcome)
	}
	definitionBytes = mustReadFile(t, mutationDefinitionPath(root))
	draft := service.DraftDefinition(context.Background(), identity, DraftDefinitionInput{DefinitionBytes: definitionBytes})
	if draft.Outcome.Classification != ClassificationVerdict || draft.Outcome.Code != "definition-locked" {
		t.Fatalf("DraftDefinition(locked) outcome = %+v", draft.Outcome)
	}
	candidate := service.CaptureCandidate(context.Background(), identity, CaptureCandidateInput{CandidateID: "cache", PatchBytes: []byte("x"), DefinitionBytes: definitionBytes})
	if candidate.Outcome.Classification != ClassificationVerdict || candidate.Outcome.Code != "definition-locked" {
		t.Fatalf("CaptureCandidate(locked) outcome = %+v", candidate.Outcome)
	}
}

func authenticatedHuman(t *testing.T) Actor {
	t.Helper()
	profile, err := governanceprincipal.DecodeProfile([]byte(`schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - { id: github, kind: forge }
role_mappings: []
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
`), governanceprincipal.Catalog{Transitions: []string{"accept"}})
	if err != nil {
		t.Fatal(err)
	}
	resolver := governanceprincipal.NewResolver(trustFactReaderFunc(func(context.Context, governanceprincipal.TrustSource, governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error) {
		return governanceprincipal.TrustFact{SourceID: "github", SourceKind: governanceprincipal.TrustSourceForge, Subjects: []string{"user-123"}, EvidenceDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Available: true, Valid: true}, nil
	}))
	resolution, err := resolver.Resolve(context.Background(), profile, governanceprincipal.PrincipalClaim{TrustSource: "github", Subject: "user-123"})
	if err != nil {
		t.Fatal(err)
	}
	actor, err := NewAuthenticatedHuman(resolution)
	if err != nil {
		t.Fatal(err)
	}
	return actor
}

func gitFromExperimentDir(t *testing.T, root, experimentID string) *fakeGit {
	t.Helper()
	base := filepath.Dir(mutationDefinitionPath(root))
	prefix := filepath.ToSlash(filepath.Join(".verdi", "specs", "active", "request-path-spike", "experiments", experimentID))
	git := &fakeGit{revision: DefaultBranch{Name: "main", Ref: "refs/remotes/origin/main", Head: testHead}, blobs: map[string][]byte{}}
	err := filepath.WalkDir(base, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, name)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		treePath := prefix + "/" + filepath.ToSlash(rel)
		object := fmt.Sprintf("object-%03d", len(git.entries)+1)
		git.entries = append(git.entries, GitTreeEntry{Mode: "100644", Type: "blob", Object: object, Path: treePath})
		git.blobs[treePath] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(git.entries, func(i, j int) bool { return git.entries[i].Path < git.entries[j].Path })
	return git
}
