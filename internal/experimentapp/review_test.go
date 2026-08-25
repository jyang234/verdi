package experimentapp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

func TestReviewRegistrationIsDeterministicReadOnlyAndDisclosesDirectEdits(t *testing.T) {
	root := t.TempDir()
	copyExperimentFixture(t, root, "experiment-v2", "request-path-v2")
	capabilityBytes, err := os.ReadFile(filepath.Join("testdata", "capabilities.json"))
	if err != nil {
		t.Fatal(err)
	}
	before := snapshotWorktree(t, root)

	run := func() (ReviewRegistrationResult, *fakeGit, *fakePolicyResolver) {
		git := gitFixture(t, "experiment-v2", "request-path-v2")
		policy := &fakePolicyResolver{decision: resolveTestPolicy(t)}
		service, err := NewService(policy, git, &fakeCapabilities{bytes: capabilityBytes}, &acceptingVerifier{})
		if err != nil {
			t.Fatal(err)
		}
		return service.ReviewRegistration(context.Background(), testIdentity(t, root, "request-path-v2")), git, policy
	}

	first, git, policy := run()
	second, _, _ := run()
	if first.Outcome.Classification != ClassificationClean || first.Outcome.ExitCode() != 0 {
		t.Fatalf("ReviewRegistration() outcome = %+v", first.Outcome)
	}
	if first.PacketDigest == "" || !bytes.Equal(first.PacketBytes, second.PacketBytes) || first.PacketDigest != second.PacketDigest {
		t.Fatalf("review packet is not deterministic: first=%+v second=%+v", first, second)
	}
	if first.Packet.Capabilities.Schema == "" || first.Packet.RetainedArtifactBytes <= 0 {
		t.Fatalf("review packet omitted capabilities or retention effect: %+v", first.Packet)
	}
	if git.headCalls != 1 || !reflect.DeepEqual(git.treeCalls, []string{testHead}) || policy.calls != 1 {
		t.Fatalf("calls head=%d tree=%v policy=%d", git.headCalls, git.treeCalls, policy.calls)
	}
	if after := snapshotWorktree(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("ReviewRegistration wrote worktree\nbefore=%#v\nafter=%#v", before, after)
	}

	marker := filepath.Join(root, ".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v2", "unregistered.txt")
	if err := os.WriteFile(marker, []byte("direct edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	direct, _, directPolicy := run()
	if direct.Outcome.Classification != ClassificationVerdict || direct.Outcome.ExitCode() != 1 || direct.Outcome.Code != "direct-draft-unreconciled" {
		t.Fatalf("direct ReviewRegistration() outcome = %+v", direct.Outcome)
	}
	if direct.AcceptedArtifactDigest == direct.ProposedArtifactDigest || directPolicy.calls != 1 {
		t.Fatalf("direct review digests/policy = %q %q / %d", direct.AcceptedArtifactDigest, direct.ProposedArtifactDigest, directPolicy.calls)
	}
	effectivePolicyDigest, err := directPolicy.decision.EffectivePolicyDigest()
	if err != nil {
		t.Fatal(err)
	}
	decisionDigest, err := directPolicy.decision.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if direct.Packet.PolicyDigest != effectivePolicyDigest || direct.Packet.PolicyDigest == decisionDigest {
		t.Fatalf("review policy digest = %q, want effective %q and not decision %q", direct.Packet.PolicyDigest, effectivePolicyDigest, decisionDigest)
	}

	actor := testActor(t)
	record := experiment.ProvenanceRecord{
		Schema:         experiment.ProvenanceSchema,
		Experiment:     experiment.ProvenanceExperiment{Spike: "spec/request-path-spike", ID: "request-path-v2"},
		Operation:      experiment.MutationReconcileDirect,
		PreviousDigest: direct.AcceptedArtifactDigest, ResultDigest: direct.ProposedArtifactDigest,
		PolicyDigest:   decisionDigest,
		PolicyDecision: experiment.ProvenancePolicyDecision{State: experiment.PolicyAllowed, Reasons: []experiment.ProvenancePolicyReason{}},
		Attribution:    actor.Attribution(), Harness: actor.Harness(), Session: actor.Session(),
		Paths: []string{".verdi/specs/active/request-path-spike/experiments/request-path-v2/unregistered.txt"},
	}
	if err := record.Seal(); err != nil {
		t.Fatal(err)
	}
	provenanceBytes, err := experiment.EncodeProvenanceRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	provenancePath := filepath.Join(filepath.Dir(marker), experiment.ProvenanceFile)
	if err := os.WriteFile(provenancePath, provenanceBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	historicalPolicy, _, _ := run()
	if historicalPolicy.Outcome.Classification != ClassificationClean {
		t.Fatalf("historical-policy provenance outcome = %+v", historicalPolicy.Outcome)
	}

	record.PolicyDigest = effectivePolicyDigest
	if err := record.Seal(); err != nil {
		t.Fatal(err)
	}
	provenanceBytes, err = experiment.EncodeProvenanceRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(provenancePath, provenanceBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	reconciled, _, _ := run()
	if reconciled.Outcome.Classification != ClassificationClean || reconciled.Outcome.ExitCode() != 0 {
		t.Fatalf("reconciled ReviewRegistration() outcome = %+v", reconciled.Outcome)
	}
}

func TestReviewRegistrationEnumeratesAbsentAcceptedExperimentAsEmptyBase(t *testing.T) {
	root := t.TempDir()
	copyExperimentFixture(t, root, "experiment-v2", "request-path-v2")
	capabilityBytes, err := os.ReadFile(filepath.Join("testdata", "capabilities.json"))
	if err != nil {
		t.Fatal(err)
	}
	git := &fakeGit{revision: DefaultBranch{Name: "main", Ref: "refs/remotes/origin/main", Head: testHead}}
	policy := &fakePolicyResolver{decision: resolveTestPolicy(t)}
	service, err := NewService(policy, git, &fakeCapabilities{bytes: capabilityBytes}, &acceptingVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	result := service.ReviewRegistration(context.Background(), testIdentity(t, root, "request-path-v2"))
	if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "direct-draft-unreconciled" {
		t.Fatalf("ReviewRegistration() outcome = %+v", result.Outcome)
	}
	if git.headCalls != 1 || !reflect.DeepEqual(git.treeCalls, []string{testHead}) || len(git.blobCalls) != 0 || policy.calls != 1 {
		t.Fatalf("calls head=%d tree=%v blobs=%v policy=%d", git.headCalls, git.treeCalls, git.blobCalls, policy.calls)
	}
}
