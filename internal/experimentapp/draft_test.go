package experimentapp

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/experiment"
)

func TestDraftDefinitionAndCandidateCaptureSerializeProvenance(t *testing.T) {
	root, service := mutationTestService(t)
	definitionPath := mutationDefinitionPath(root)
	original := mustReadFile(t, definitionPath)
	updated := bytes.Replace(original, []byte("#oq-cache\n"), []byte("#oq-cache-revised\n"), 1)

	draft := service.DraftDefinition(context.Background(), testIdentity(t, root, "request-path-v2"), DraftDefinitionInput{DefinitionBytes: updated})
	if draft.Outcome.Classification != ClassificationClean {
		t.Fatalf("DraftDefinition() outcome = %+v", draft.Outcome)
	}
	if got := mustReadFile(t, definitionPath); !bytes.Equal(got, updated) {
		t.Fatalf("definition bytes changed unexpectedly\ngot=%q\nwant=%q", got, updated)
	}
	records := mutationProvenance(t, root)
	if len(records) != 1 || records[0].Operation != experiment.MutationDraftDefinition || records[0].Harness != "codex" {
		t.Fatalf("draft provenance = %+v", records)
	}
	if records[0].PreviousDigest == records[0].ResultDigest || draft.ArtifactDigest != records[0].ResultDigest {
		t.Fatalf("draft digests result=%q record=%+v", draft.ArtifactDigest, records[0])
	}

	patchPath := filepath.Join(filepath.Dir(definitionPath), "candidates", "cache.patch")
	newPatch := []byte("diff --git a/spikes/cache.go b/spikes/cache.go\nindex 1111111..2222222 100644\n--- a/spikes/cache.go\n+++ b/spikes/cache.go\n@@ -1 +1 @@\n-old\n+new\n")
	definitionWithPatch := bytes.Replace(updated, []byte("sha256:948705e2b8a093896358025d2b75282fbd1c36557c278881add34f4c75cbecc7"), []byte(rawDigest(newPatch)), 1)
	captured := service.CaptureCandidate(context.Background(), testIdentity(t, root, "request-path-v2"), CaptureCandidateInput{
		CandidateID: "cache", PatchBytes: newPatch, DefinitionBytes: definitionWithPatch,
	})
	if captured.Outcome.Classification != ClassificationClean {
		t.Fatalf("CaptureCandidate() outcome = %+v", captured.Outcome)
	}
	if got := mustReadFile(t, patchPath); !bytes.Equal(got, newPatch) {
		t.Fatalf("captured patch = %q, want %q", got, newPatch)
	}
	records = mutationProvenance(t, root)
	if len(records) != 2 || records[1].Operation != experiment.MutationCaptureCandidate || records[1].PreviousDigest != records[0].ResultDigest {
		t.Fatalf("candidate provenance = %+v", records)
	}
}

func TestDraftDefinitionCreatesCanonicalUnlockedProposal(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "specs", "active", "request-path-spike"), 0o755); err != nil {
		t.Fatal(err)
	}
	capabilities := mustReadFile(t, filepath.Join("testdata", "capabilities.json"))
	service, err := NewService(
		&fakePolicyResolver{decision: resolveTestPolicy(t)},
		&fakeGit{revision: DefaultBranch{Name: "main", Ref: "refs/remotes/origin/main", Head: testHead}},
		&fakeCapabilities{bytes: capabilities},
		&acceptingVerifier{},
	)
	if err != nil {
		t.Fatal(err)
	}
	result := service.DraftDefinition(context.Background(), testIdentity(t, root, "request-path-v2"), DraftDefinitionInput{
		DefinitionBytes: mustReadFile(t, filepath.Join("testdata", "experiment-v2", "experiment.yaml")),
		CandidatePatches: map[string][]byte{
			"baseline": mustReadFile(t, filepath.Join("testdata", "experiment-v2", "candidates", "baseline.patch")),
			"cache":    mustReadFile(t, filepath.Join("testdata", "experiment-v2", "candidates", "cache.patch")),
		},
	})
	if result.Outcome.Classification != ClassificationClean {
		t.Fatalf("DraftDefinition(create) outcome = %+v", result.Outcome)
	}
	definition, err := experiment.DecodeDefinition(mustReadFile(t, mutationDefinitionPath(root)))
	if err != nil {
		t.Fatal(err)
	}
	locked, err := experiment.Locked(definition)
	if err != nil || locked {
		t.Fatalf("created definition locked=%v err=%v", locked, err)
	}
	records := mutationProvenance(t, root)
	if len(records) != 1 || records[0].Operation != experiment.MutationDraftDefinition || records[0].ResultDigest != result.ArtifactDigest {
		t.Fatalf("created draft provenance = %+v result=%+v", records, result)
	}
	review := service.ReviewRegistration(context.Background(), testIdentity(t, root, "request-path-v2"))
	if review.Outcome.Classification != ClassificationClean {
		t.Fatalf("ReviewRegistration(created draft) outcome = %+v", review.Outcome)
	}
}

func TestCandidateProtectedPathRefusalWritesNothing(t *testing.T) {
	root, service := mutationTestService(t)
	before := snapshotWorktree(t, root)
	definition := mustReadFile(t, mutationDefinitionPath(root))
	protectedPatch := []byte("diff --git a/.verdi/specs/active/request-path-spike/experiments/request-path-v2/experiment.yaml b/.verdi/specs/active/request-path-spike/experiments/request-path-v2/experiment.yaml\n")
	definition = bytes.Replace(definition, []byte("sha256:948705e2b8a093896358025d2b75282fbd1c36557c278881add34f4c75cbecc7"), []byte(rawDigest(protectedPatch)), 1)

	result := service.CaptureCandidate(context.Background(), testIdentity(t, root, "request-path-v2"), CaptureCandidateInput{
		CandidateID: "cache", PatchBytes: protectedPatch, DefinitionBytes: definition,
	})
	if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "candidate-refused" {
		t.Fatalf("CaptureCandidate(protected) outcome = %+v", result.Outcome)
	}
	if after := snapshotWorktree(t, root); !mapsEqual(after, before) {
		t.Fatalf("protected candidate changed worktree\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestDraftInterruptionLeavesDirtyProposalRefused(t *testing.T) {
	root, service := mutationTestService(t)
	definitionPath := mutationDefinitionPath(root)
	original := mustReadFile(t, definitionPath)
	updated := bytes.Replace(original, []byte("#oq-cache\n"), []byte("#oq-interrupted\n"), 1)
	coordinator := draftmutation.Coordinator{After: func(step string) error {
		if step == stepProposalArtifactsInstalled {
			return errors.New("simulated interruption")
		}
		return nil
	}}

	result := service.draftDefinition(context.Background(), testIdentity(t, root, "request-path-v2"), DraftDefinitionInput{DefinitionBytes: updated}, coordinator)
	if result.Outcome.Classification != ClassificationOperational || result.Outcome.Code != "proposal-write-failed" {
		t.Fatalf("interrupted DraftDefinition() outcome = %+v", result.Outcome)
	}
	if got := mustReadFile(t, definitionPath); !bytes.Equal(got, updated) {
		t.Fatalf("interrupted definition = %q, want installed proposal", got)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(definitionPath), experiment.ProvenanceFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("interrupted provenance stat error = %v, want absent", err)
	}
	review := service.ReviewRegistration(context.Background(), testIdentity(t, root, "request-path-v2"))
	if review.Outcome.Classification != ClassificationVerdict || review.Outcome.Code != "direct-draft-unreconciled" {
		t.Fatalf("ReviewRegistration(interrupted) outcome = %+v", review.Outcome)
	}
}

func TestDraftMutationProjectionIgnoresMachineEvidenceButNotDirectArtifacts(t *testing.T) {
	root, service := mutationTestService(t)
	definitionPath := mutationDefinitionPath(root)
	original := mustReadFile(t, definitionPath)
	firstBytes := bytes.Replace(original, []byte("#oq-cache\n"), []byte("#oq-first\n"), 1)
	first := service.DraftDefinition(context.Background(), testIdentity(t, root, "request-path-v2"), DraftDefinitionInput{DefinitionBytes: firstBytes})
	if first.Outcome.Classification != ClassificationClean {
		t.Fatalf("DraftDefinition(first) outcome = %+v", first.Outcome)
	}
	experimentDir := filepath.Dir(definitionPath)
	for name, data := range map[string][]byte{
		filepath.Join("runs", "run-1", "execution.json"): []byte("{}\n"),
		"recommendation.md": []byte("# Recommendation\n\nMachine generated.\n"),
		filepath.Join("selected", "capsule-manifest.json"): []byte("{}\n"),
	} {
		full := filepath.Join(experimentDir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	secondBytes := bytes.Replace(firstBytes, []byte("#oq-first\n"), []byte("#oq-second\n"), 1)
	second := service.DraftDefinition(context.Background(), testIdentity(t, root, "request-path-v2"), DraftDefinitionInput{DefinitionBytes: secondBytes})
	if second.Outcome.Classification != ClassificationClean {
		t.Fatalf("DraftDefinition(after machine evidence) outcome = %+v", second.Outcome)
	}
	if err := os.WriteFile(filepath.Join(experimentDir, "unknown-direct.txt"), []byte("direct edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	thirdBytes := bytes.Replace(secondBytes, []byte("#oq-second\n"), []byte("#oq-third\n"), 1)
	third := service.DraftDefinition(context.Background(), testIdentity(t, root, "request-path-v2"), DraftDefinitionInput{DefinitionBytes: thirdBytes})
	if third.Outcome.Classification != ClassificationVerdict || third.Outcome.Code != "direct-draft-unreconciled" {
		t.Fatalf("DraftDefinition(after unknown artifact) outcome = %+v", third.Outcome)
	}
	identity := testIdentity(t, root, "request-path-v2")
	identity.Actor = authenticatedHuman(t)
	reconciled := service.ReconcileDraft(context.Background(), identity)
	if reconciled.Outcome.Classification != ClassificationClean {
		t.Fatalf("ReconcileDraft(unknown artifact) outcome = %+v", reconciled.Outcome)
	}
}

func mutationTestService(t *testing.T) (string, *Service) {
	t.Helper()
	root := t.TempDir()
	copyExperimentFixture(t, root, "experiment-v2", "request-path-v2")
	capabilities := mustReadFile(t, filepath.Join("testdata", "capabilities.json"))
	service, err := NewService(
		&fakePolicyResolver{decision: resolveTestPolicy(t)},
		gitFixture(t, "experiment-v2", "request-path-v2"),
		&fakeCapabilities{bytes: capabilities},
		&acceptingVerifier{},
	)
	if err != nil {
		t.Fatal(err)
	}
	return root, service
}

func mutationDefinitionPath(root string) string {
	return filepath.Join(root, ".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v2", "experiment.yaml")
}

func mutationProvenance(t *testing.T, root string) []experiment.ProvenanceRecord {
	t.Helper()
	data := mustReadFile(t, filepath.Join(filepath.Dir(mutationDefinitionPath(root)), experiment.ProvenanceFile))
	records, err := experiment.DecodeProvenanceLog(data)
	if err != nil {
		t.Fatalf("DecodeProvenanceLog() error = %v", err)
	}
	return records
}

func mustReadFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", name, err)
	}
	return data
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}
