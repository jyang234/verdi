package experimentapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// fakeWorkspaceReleaser records every release attempt and can refuse named
// targets, mirroring execworkspace.Releaser's idempotent marker contract.
type fakeWorkspaceReleaser struct {
	calls []string
	fail  map[string]bool
}

func (f *fakeWorkspaceReleaser) Release(workspaceID string) error {
	f.calls = append(f.calls, workspaceID)
	if f.fail[workspaceID] {
		return errors.New("release refused by fake")
	}
	return nil
}

type releaseFixture struct {
	root         string
	service      *Service
	identity     Identity
	git          *fakeGit
	resolution   governanceprincipal.PrincipalResolution
	locked       experiment.Definition
	defDigest    string
	winnerDigest string
	targets      []string
}

const (
	releaseWorkloadPath = "inputs/workload.txt"
	releaseContractPath = "inputs/contract.txt"
	releaseFixturePath  = "fixtures/request-log.bin"
)

func releaseContractBytes() []byte { return []byte("contract-bytes\n") }
func releaseFixtureBytes() []byte  { return []byte("fixture-bytes\n") }

// writeReleasableRun mirrors writeRatifiableRun but its receipt carries the
// complete resolved-input custody (workload, contract, fixture) release
// derives retained bytes from.
func writeReleasableRun(t *testing.T, root string, def experiment.Definition, run string, cacheValue int, workloadBytes []byte) string {
	t.Helper()
	digest := writeRatifiableRun(t, root, def, run, cacheValue)
	receipt := ratifiableReceipt(t, def, run)
	receipt.Fingerprint.InputDigests = map[string]string{
		"evaluator:" + def.Evaluator.Argv[0]: strings.TrimPrefix(def.Evaluator.Digest, "sha256:"),
		releaseWorkloadPath:                  strings.TrimPrefix(def.Workload.Digest, "sha256:"),
		releaseContractPath:                  strings.TrimPrefix(def.Contract.Digest, "sha256:"),
		releaseFixturePath:                   strings.TrimPrefix(def.Fixtures[0].Digest, "sha256:"),
	}
	encoded, err := experiment.EncodeExecutionReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(filepath.Dir(mutationDefinitionPath(root)), "runs", run)
	if err := os.WriteFile(filepath.Join(runDir, "execution.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	_ = workloadBytes
	return digest
}

// buildReleaseFixture builds an accepted, locked, executed, RATIFIED
// experiment whose locked workload/contract/fixture digests are the real
// digests of protected-input bytes present in the accepted tree.
func buildReleaseFixture(t *testing.T, workloadBytes []byte, disposition experiment.Disposition, candidate string) releaseFixture {
	t.Helper()
	root, service := mutationTestService(t)

	definitionPath := mutationDefinitionPath(root)
	raw, err := os.ReadFile(definitionPath)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	replace := func(old, new string) {
		if !strings.Contains(doc, old) {
			t.Fatalf("definition fixture does not contain %q", old)
		}
		doc = strings.Replace(doc, old, new, 1)
	}
	replace("digest: sha256:"+strings.Repeat("5", 64), "digest: "+rawDigest(workloadBytes))
	replace("digest: sha256:"+strings.Repeat("6", 64), "digest: "+rawDigest(releaseContractBytes()))
	replace("contract:\n", "fixtures:\n  - id: request-log\n    digest: "+rawDigest(releaseFixtureBytes())+"\ncontract:\n")
	if err := os.WriteFile(definitionPath, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}

	identity := testIdentity(t, root, "request-path-v2")
	human := identity
	human.Actor = authenticatedHuman(t)
	// The digest doctoring above is a direct edit: admit it through the
	// designed reconciliation flow before the registration review.
	reconciled := service.ReconcileDraft(context.Background(), human)
	if reconciled.Outcome.Classification != ClassificationClean {
		t.Fatalf("ReconcileDraft() outcome = %+v", reconciled.Outcome)
	}
	review := service.ReviewRegistration(context.Background(), human)
	if review.Outcome.Classification != ClassificationClean {
		t.Fatalf("ReviewRegistration() outcome = %+v", review.Outcome)
	}
	proposal := service.ProposeRegistration(context.Background(), human, RegistrationInput{ReviewPacketDigest: review.PacketDigest})
	if proposal.Outcome.Classification != ClassificationClean {
		t.Fatalf("ProposeRegistration() outcome = %+v", proposal.Outcome)
	}
	lockedBytes, err := os.ReadFile(definitionPath)
	if err != nil {
		t.Fatal(err)
	}
	locked, err := experiment.DecodeDefinition(lockedBytes)
	if err != nil {
		t.Fatal(err)
	}
	defDigest, err := experiment.DefinitionDigest(locked)
	if err != nil {
		t.Fatal(err)
	}

	winnerDigest := writeReleasableRun(t, root, locked, "run-alpha", 50, workloadBytes)
	writeReleasableRun(t, root, locked, "run-zeta", 100, workloadBytes)

	resolution := ratificationResolve(t, ratificationFacts("user-123"))
	record := experiment.Ratification{
		Schema: experiment.RatificationSchemaV2, ResultDigest: winnerDigest,
		ActorV2: &experiment.RatificationActor{
			TrustSource: resolution.Claim.TrustSource, Subject: resolution.Claim.Subject,
			PrincipalID: string(resolution.PrincipalID),
		},
		Disposition: disposition, Candidate: candidate,
	}
	if candidate != "" {
		record.Reason = "explicit selection for the release fixture"
	}
	encoded, err := experiment.EncodeRatification(record)
	if err != nil {
		t.Fatal(err)
	}
	// plantAcceptedRatification writes ratification.yaml first, so build
	// the pair record against the exact planted layout.
	experimentDir := filepath.Dir(definitionPath)
	if err := os.WriteFile(filepath.Join(experimentDir, "ratification.yaml"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	attribution, err := governanceprincipal.NewPrincipalAttribution(resolution.PrincipalID)
	if err != nil {
		t.Fatal(err)
	}
	pair := ratificationPairRecord(t, root, encoded, attribution)
	plantAcceptedRatification(t, root, service, encoded, &pair)

	git, ok := service.git.(*fakeGit)
	if !ok {
		t.Fatalf("service.git is %T, want *fakeGit", service.git)
	}
	addReleaseBlob(t, git, releaseWorkloadPath, workloadBytes, "100644")
	addReleaseBlob(t, git, releaseContractPath, releaseContractBytes(), "100644")
	addReleaseBlob(t, git, releaseFixturePath, releaseFixtureBytes(), "100644")

	targets := releaseFixtureTargets(t, root, locked, defDigest, []string{"run-alpha", "run-zeta"})
	return releaseFixture{
		root: root, service: service, identity: identity, git: git,
		resolution: resolution, locked: locked, defDigest: defDigest,
		winnerDigest: winnerDigest, targets: targets,
	}
}

func addReleaseBlob(t *testing.T, git *fakeGit, path string, data []byte, mode string) {
	t.Helper()
	git.entries = append(git.entries, GitTreeEntry{Mode: mode, Type: "blob", Object: "blob-" + path, Path: path})
	git.blobs[path] = append([]byte(nil), data...)
}

func releaseFixtureTargets(t *testing.T, root string, def experiment.Definition, defDigest string, runs []string) []string {
	t.Helper()
	seen := map[string]bool{}
	targets := make([]string, 0, len(runs)*len(def.Candidates))
	for _, run := range runs {
		for _, candidate := range def.Candidates {
			patch, err := os.ReadFile(filepath.Join(filepath.Dir(mutationDefinitionPath(root)), filepath.FromSlash(candidate.Patch)))
			if err != nil {
				t.Fatal(err)
			}
			runID, err := experiment.WorkspaceRunID(defDigest, run, candidate.ID)
			if err != nil {
				t.Fatal(err)
			}
			workspaceIdentity, err := execworkspace.NewPatchIdentity(runID, candidate.Base, patch)
			if err != nil {
				t.Fatal(err)
			}
			id, err := workspaceIdentity.WorkspaceID()
			if err != nil {
				t.Fatal(err)
			}
			if !seen[id] {
				seen[id] = true
				targets = append(targets, id)
			}
		}
	}
	sort.Strings(targets)
	return targets
}

func releaseAuthority(t *testing.T, releaser WorkspaceReleaser) ReleaseAuthority {
	t.Helper()
	return ReleaseAuthority{
		Profile: ratificationProfile(t, "github"), Facts: ratificationFacts("user-123"),
		Releaser: releaser,
	}
}

func releaseManifestPath(root string) string {
	return filepath.Join(filepath.Dir(mutationDefinitionPath(root)), "selected", "capsule-manifest.json")
}

func TestReleaseRatifiedSelectingPublishesCapsuleAndReleases(t *testing.T) {
	fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionSelectRecommended, "")
	releaser := &fakeWorkspaceReleaser{}
	result := fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, releaser))
	if result.Outcome.Classification != ClassificationClean {
		t.Fatalf("ReleaseRatified() outcome = %+v", result.Outcome)
	}
	if !result.CapsulePublished || result.Selected != "cache" || result.Disposition != experiment.DispositionSelectRecommended {
		t.Fatalf("ReleaseRatified() = %+v, want published capsule selecting the winner", result)
	}
	raw, err := os.ReadFile(releaseManifestPath(fixture.root))
	if err != nil {
		t.Fatalf("published manifest unreadable: %v", err)
	}
	manifest, err := experiment.DecodeCapsuleManifest(raw)
	if err != nil {
		t.Fatalf("published manifest does not strict-decode: %v", err)
	}
	if manifest.Selected != "cache" || manifest.ResultDigest != fixture.winnerDigest {
		t.Fatalf("published manifest = %+v", manifest)
	}
	ids := make([]string, 0, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		ids = append(ids, artifact.ID)
	}
	for _, want := range []string{"definition", "candidate-patch", "evaluator-capabilities", "contract", "workload", "fixture-request-log", "execution-receipt", "observations", "result", "ratification"} {
		found := false
		for _, id := range ids {
			if id == want {
				found = true
			}
		}
		if !found {
			t.Errorf("manifest omits %q: %v", want, ids)
		}
	}
	if len(result.Released) != len(fixture.targets) || len(result.Failed) != 0 {
		t.Fatalf("Released/Failed = %v/%v, want all %v", result.Released, result.Failed, fixture.targets)
	}
	if fmt.Sprint(releaser.calls) != fmt.Sprint(fixture.targets) {
		t.Fatalf("release calls %v, want deterministic sorted targets %v", releaser.calls, fixture.targets)
	}

	// Idempotent retry: identical manifest bytes and marker-idempotent
	// release succeed again.
	retry := fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, &fakeWorkspaceReleaser{}))
	if retry.Outcome.Classification != ClassificationClean {
		t.Fatalf("retry outcome = %+v", retry.Outcome)
	}
}

func TestReleaseRatifiedNonSelectingReleasesWithoutCapsule(t *testing.T) {
	fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionRejectAll, "")
	releaser := &fakeWorkspaceReleaser{}
	result := fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, releaser))
	if result.Outcome.Classification != ClassificationClean {
		t.Fatalf("ReleaseRatified(reject-all) outcome = %+v", result.Outcome)
	}
	if result.CapsulePublished || result.Selected != "" {
		t.Fatalf("non-selecting result = %+v, want no capsule", result)
	}
	if _, err := os.Lstat(releaseManifestPath(fixture.root)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("non-selecting disposition published a capsule: %v", err)
	}
	if fmt.Sprint(releaser.calls) != fmt.Sprint(fixture.targets) {
		t.Fatalf("release calls %v, want the same complete set %v", releaser.calls, fixture.targets)
	}
}

func TestReleaseRatifiedRequiresAcceptedRatification(t *testing.T) {
	_, service, identity, _, _ := ratifiableService(t)
	releaser := &fakeWorkspaceReleaser{}
	result := service.ReleaseRatified(context.Background(), identity, releaseAuthority(t, releaser))
	if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "ratification-not-accepted" {
		t.Fatalf("ReleaseRatified(no ratification) outcome = %+v", result.Outcome)
	}
	if len(releaser.calls) != 0 {
		t.Fatalf("release ran without accepted ratification: %v", releaser.calls)
	}
}

func TestReleaseRatifiedPartialFailureAttemptsAllAndRetries(t *testing.T) {
	fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionMisframed, "")
	failing := fixture.targets[1]
	releaser := &fakeWorkspaceReleaser{fail: map[string]bool{failing: true}}
	before := fixture.git.blobs[".verdi/specs/active/request-path-spike/experiments/request-path-v2/ratification.yaml"]

	result := fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, releaser))
	if result.Outcome.Classification != ClassificationOperational {
		t.Fatalf("partial failure outcome = %+v, want operational", result.Outcome)
	}
	if fmt.Sprint(releaser.calls) != fmt.Sprint(fixture.targets) {
		t.Fatalf("release attempted %v, want every target %v even after a failure", releaser.calls, fixture.targets)
	}
	if len(result.Failed) != 1 || result.Failed[0] != failing {
		t.Fatalf("Failed = %v, want exactly %q", result.Failed, failing)
	}
	if !strings.Contains(result.Outcome.Detail, failing) {
		t.Fatalf("outcome detail %q does not name the failed target", result.Outcome.Detail)
	}

	// Durable records are untouched by cleanup failure.
	experimentDir := filepath.Dir(mutationDefinitionPath(fixture.root))
	after, err := os.ReadFile(filepath.Join(experimentDir, "ratification.yaml"))
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("cleanup failure disturbed the ratification record: %v", err)
	}
	if _, err := os.Stat(filepath.Join(experimentDir, "runs", "run-alpha", "result.json")); err != nil {
		t.Fatalf("cleanup failure disturbed run evidence: %v", err)
	}

	// Idempotent retry succeeds once the target releases.
	retry := fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, &fakeWorkspaceReleaser{}))
	if retry.Outcome.Classification != ClassificationClean {
		t.Fatalf("retry outcome = %+v", retry.Outcome)
	}
}

func TestReleaseRatifiedOversizedArtifactRefusedBeforeAnyEffect(t *testing.T) {
	oversized := bytes.Repeat([]byte("w"), 8388609) // policy cap 8388608 + 1
	fixture := buildReleaseFixture(t, oversized, experiment.DispositionSelectRecommended, "")
	releaser := &fakeWorkspaceReleaser{}
	result := fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, releaser))
	if result.Outcome.Classification != ClassificationVerdict || result.Outcome.Code != "capsule-retention-refused" {
		t.Fatalf("oversized outcome = %+v, want capsule-retention-refused verdict", result.Outcome)
	}
	if !strings.Contains(result.Outcome.Detail, "workload") || !strings.Contains(result.Outcome.Detail, "8388609") {
		t.Fatalf("oversized witness %q does not name the artifact and observed size", result.Outcome.Detail)
	}
	if _, err := os.Lstat(releaseManifestPath(fixture.root)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("oversized capsule was published")
	}
	if len(releaser.calls) != 0 {
		t.Fatalf("oversized capsule still released workspaces: %v", releaser.calls)
	}
}

func TestReleaseRatifiedSymlinkedRetainedInputRefused(t *testing.T) {
	fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionSelectRecommended, "")
	for i := range fixture.git.entries {
		if fixture.git.entries[i].Path == releaseWorkloadPath {
			fixture.git.entries[i].Mode = "120000"
		}
	}
	releaser := &fakeWorkspaceReleaser{}
	result := fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, releaser))
	if result.Outcome.Classification != ClassificationOperational {
		t.Fatalf("symlinked retained input outcome = %+v, want operational", result.Outcome)
	}
	if _, err := os.Lstat(releaseManifestPath(fixture.root)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("symlinked retained input still published a capsule")
	}
	if len(releaser.calls) != 0 {
		t.Fatalf("symlinked retained input still released workspaces: %v", releaser.calls)
	}
}

func TestReleaseRatifiedConflictingManifestRefusedBeforeRelease(t *testing.T) {
	fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionSelectRecommended, "")
	if err := os.MkdirAll(filepath.Dir(releaseManifestPath(fixture.root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(releaseManifestPath(fixture.root), []byte("{\"schema\":\"conflicting\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	releaser := &fakeWorkspaceReleaser{}
	result := fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, releaser))
	if result.Outcome.Classification != ClassificationOperational {
		t.Fatalf("conflicting manifest outcome = %+v, want operational", result.Outcome)
	}
	if len(releaser.calls) != 0 {
		t.Fatalf("capsule verification did not precede release: %v", releaser.calls)
	}
}

func TestReleaseRatifiedWorkspaceIdentityMismatchRefused(t *testing.T) {
	fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionSelectRecommended, "")
	// Tamper one receipt candidate's materialization patch hash in the
	// accepted run evidence: identity reconstruction must refuse.
	experimentDir := filepath.Dir(mutationDefinitionPath(fixture.root))
	receiptPath := filepath.Join(experimentDir, "runs", "run-zeta", "execution.json")
	raw, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := experiment.DecodeExecutionReceipt(raw)
	if err != nil {
		t.Fatal(err)
	}
	receipt.Candidates[0].PatchDigest = "sha256:" + strings.Repeat("d", 64)
	receipt.Candidates[0].Materialization.PatchSHA256 = strings.Repeat("d", 64)
	tampered, err := experiment.EncodeExecutionReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	fixture.service.git = gitFromExperimentDir(t, fixture.root, "request-path-v2")
	git := fixture.service.git.(*fakeGit)
	addReleaseBlob(t, git, releaseWorkloadPath, []byte("workload-bytes\n"), "100644")
	addReleaseBlob(t, git, releaseContractPath, releaseContractBytes(), "100644")
	addReleaseBlob(t, git, releaseFixturePath, releaseFixtureBytes(), "100644")

	releaser := &fakeWorkspaceReleaser{}
	result := fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, releaser))
	if result.Outcome.Classification != ClassificationOperational {
		t.Fatalf("identity mismatch outcome = %+v, want operational", result.Outcome)
	}
	if len(releaser.calls) != 0 {
		t.Fatalf("identity mismatch still released workspaces: %v", releaser.calls)
	}
}

func TestReleaseRatifiedRequiresReleaser(t *testing.T) {
	fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionRejectAll, "")
	authority := releaseAuthority(t, nil)
	result := fixture.service.ReleaseRatified(context.Background(), fixture.identity, authority)
	if result.Outcome.Classification != ClassificationOperational {
		t.Fatalf("nil releaser outcome = %+v, want operational", result.Outcome)
	}
}
