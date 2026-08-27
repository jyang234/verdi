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

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentdecision"
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
	observations := ratifiableObservations(t, def, run, cacheValue, experiment.ObservationSchemaV2)
	core, err := experimentdecision.Evaluate(def, observations, experimentdecision.EnvironmentAttestation{PolicyID: def.Execution.EnvironmentPolicy})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := experiment.DecisionFromResult(core, observations)
	if err != nil {
		t.Fatal(err)
	}
	receipt := ratifiableReceipt(t, def, run)
	receipt.Fingerprint.InputDigests = map[string]string{
		"evaluator:" + def.Evaluator.Argv[0]: strings.TrimPrefix(def.Evaluator.Digest, "sha256:"),
		releaseWorkloadPath:                  strings.TrimPrefix(def.Workload.Digest, "sha256:"),
		releaseContractPath:                  strings.TrimPrefix(def.Contract.Digest, "sha256:"),
		releaseFixturePath:                   strings.TrimPrefix(def.Fixtures[0].Digest, "sha256:"),
	}
	receiptDigest, err := experiment.ExecutionReceiptDigest(receipt)
	if err != nil {
		t.Fatal(err)
	}
	result, err := experiment.NewResultV2(decision, experiment.ResultExecution{
		ExecutionDigest: receiptDigest,
		Isolation: experiment.ResultIsolation{
			Network:     receipt.Network,
			Disclosures: []experiment.IsolationDisclosure{},
		},
		WarmupDiagnostics: experiment.WarmupDiagnostics{
			Authority: experiment.WarmupAuthorityNonDecisionDiagnostic,
			Scope:     experiment.WarmupScopeFinalInvocation, Failures: []experiment.WarmupFailure{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	receiptBytes, err := experiment.EncodeExecutionReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	resultBytes, err := experiment.EncodeResult(result)
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(filepath.Dir(mutationDefinitionPath(root)), "runs", run)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "execution.json"), receiptBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "observations.jsonl"), encodeRatifiableObservations(t, observations), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "result.json"), resultBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := experiment.ResultDigest(result)
	if err != nil {
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

// plantReceiptOnlyRun writes an incomplete run carrying ONLY the given
// encoded execution receipt, regenerates the accepted tree, and restores
// the protected-input blobs.
func plantReceiptOnlyRun(t *testing.T, fixture releaseFixture, run string, encoded []byte, workloadBytes []byte) {
	t.Helper()
	runDir := filepath.Join(filepath.Dir(mutationDefinitionPath(fixture.root)), "runs", run)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "execution.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	fixture.service.git = gitFromExperimentDir(t, fixture.root, "request-path-v2")
	git := fixture.service.git.(*fakeGit)
	addReleaseBlob(t, git, releaseWorkloadPath, workloadBytes, "100644")
	addReleaseBlob(t, git, releaseContractPath, releaseContractBytes(), "100644")
	addReleaseBlob(t, git, releaseFixturePath, releaseFixtureBytes(), "100644")
}

// TestReleaseRatifiedRefusesUnsafeCapsulePath is the symlinked-parent
// correction matrix: publication must refuse before any external write or
// workspace release, on a real filesystem.
func TestReleaseRatifiedRefusesUnsafeCapsulePath(t *testing.T) {
	t.Run("symlinked selected parent", func(t *testing.T) {
		fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionSelectRecommended, "")
		outside := t.TempDir()
		experimentDir := filepath.Dir(mutationDefinitionPath(fixture.root))
		if err := os.Symlink(outside, filepath.Join(experimentDir, "selected")); err != nil {
			t.Fatal(err)
		}
		releaser := &fakeWorkspaceReleaser{}
		result := fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, releaser))
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("symlinked selected parent outcome = %+v, want operational", result.Outcome)
		}
		entries, err := os.ReadDir(outside)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("capsule publication escaped through the symlinked parent: %v", entries)
		}
		if len(releaser.calls) != 0 {
			t.Fatalf("workspaces released despite unsafe capsule path: %v", releaser.calls)
		}
	})

	t.Run("directory collision at the manifest path", func(t *testing.T) {
		fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionSelectRecommended, "")
		if err := os.MkdirAll(releaseManifestPath(fixture.root), 0o755); err != nil {
			t.Fatal(err)
		}
		releaser := &fakeWorkspaceReleaser{}
		result := fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, releaser))
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("directory collision outcome = %+v, want operational", result.Outcome)
		}
		if len(releaser.calls) != 0 {
			t.Fatalf("workspaces released despite manifest collision: %v", releaser.calls)
		}
	})

	t.Run("symlinked final component", func(t *testing.T) {
		fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionSelectRecommended, "")
		experimentDir := filepath.Dir(mutationDefinitionPath(fixture.root))
		if err := os.MkdirAll(filepath.Join(experimentDir, "selected"), 0o755); err != nil {
			t.Fatal(err)
		}
		outsideFile := filepath.Join(t.TempDir(), "target.json")
		if err := os.WriteFile(outsideFile, []byte("outside\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outsideFile, releaseManifestPath(fixture.root)); err != nil {
			t.Fatal(err)
		}
		releaser := &fakeWorkspaceReleaser{}
		result := fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, releaser))
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("symlinked final component outcome = %+v, want operational", result.Outcome)
		}
		after, err := os.ReadFile(outsideFile)
		if err != nil || string(after) != "outside\n" {
			t.Fatalf("capsule publication followed the final-component symlink: %q/%v", after, err)
		}
		if len(releaser.calls) != 0 {
			t.Fatalf("workspaces released despite unsafe manifest path: %v", releaser.calls)
		}
	})
}

// TestReleaseRatifiedValidatesReceiptCandidateAuthority is the
// candidate-parity correction matrix: every receipt's complete candidate
// authority must match the locked definition before any target derives.
func TestReleaseRatifiedValidatesReceiptCandidateAuthority(t *testing.T) {
	baseReceipt := func(t *testing.T, fixture releaseFixture) experiment.ExecutionReceipt {
		t.Helper()
		return ratifiableReceipt(t, fixture.locked, "run-omega")
	}
	fullInputs := func(def experiment.Definition) map[string]string {
		return map[string]string{
			"evaluator:" + def.Evaluator.Argv[0]: strings.TrimPrefix(def.Evaluator.Digest, "sha256:"),
			releaseWorkloadPath:                  strings.TrimPrefix(def.Workload.Digest, "sha256:"),
			releaseContractPath:                  strings.TrimPrefix(def.Contract.Digest, "sha256:"),
			releaseFixturePath:                   strings.TrimPrefix(def.Fixtures[0].Digest, "sha256:"),
		}
	}
	run := func(t *testing.T, fixture releaseFixture) (ReleaseResult, *fakeWorkspaceReleaser) {
		t.Helper()
		releaser := &fakeWorkspaceReleaser{}
		return fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, releaser)), releaser
	}

	t.Run("forged candidate base commit is refused", func(t *testing.T) {
		fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionRejectAll, "")
		receipt := baseReceipt(t, fixture)
		receipt.Fingerprint.InputDigests = fullInputs(fixture.locked)
		forgedBase := strings.Repeat("b", 40)
		for i := range receipt.Candidates {
			receipt.Candidates[i].BaseCommit = forgedBase
			receipt.Candidates[i].Materialization.CommitSHA = forgedBase
		}
		encoded, err := experiment.EncodeExecutionReceipt(receipt)
		if err != nil {
			t.Fatalf("forged-base receipt must remain internally consistent: %v", err)
		}
		plantReceiptOnlyRun(t, fixture, "run-omega", encoded, []byte("workload-bytes\n"))
		result, releaser := run(t, fixture)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("forged base commit outcome = %+v, want operational refusal", result.Outcome)
		}
		if len(releaser.calls) != 0 {
			t.Fatalf("forged base commit still released workspaces: %v", releaser.calls)
		}
	})

	t.Run("missing candidate row is refused", func(t *testing.T) {
		fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionRejectAll, "")
		receipt := baseReceipt(t, fixture)
		receipt.Fingerprint.InputDigests = fullInputs(fixture.locked)
		receipt.Candidates = receipt.Candidates[:1]
		encoded, err := experiment.EncodeExecutionReceipt(receipt)
		if err != nil {
			t.Fatalf("single-row receipt must remain internally consistent: %v", err)
		}
		plantReceiptOnlyRun(t, fixture, "run-omega", encoded, []byte("workload-bytes\n"))
		result, releaser := run(t, fixture)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("missing candidate row outcome = %+v, want operational refusal", result.Outcome)
		}
		if len(releaser.calls) != 0 {
			t.Fatalf("missing candidate row still released workspaces: %v", releaser.calls)
		}
	})

	t.Run("extra candidate row is refused", func(t *testing.T) {
		fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionRejectAll, "")
		receipt := baseReceipt(t, fixture)
		receipt.Fingerprint.InputDigests = fullInputs(fixture.locked)
		defDigest, err := experiment.DefinitionDigest(fixture.locked)
		if err != nil {
			t.Fatal(err)
		}
		extraRunID, err := experiment.WorkspaceRunID(defDigest, "run-omega", "zzz-extra")
		if err != nil {
			t.Fatal(err)
		}
		patchDigest := "sha256:" + strings.Repeat("e", 64)
		receipt.Candidates = append(receipt.Candidates, experiment.ReceiptCandidate{
			ID: "zzz-extra", BaseCommit: strings.Repeat("a", 40), PatchDigest: patchDigest, WorkspaceRunID: extraRunID,
			Materialization: experiment.WorkspaceIdentity{
				Shape: experiment.WorkspaceBasePlusPatch, RunID: extraRunID,
				CommitSHA: strings.Repeat("a", 40), PatchSHA256: strings.TrimPrefix(patchDigest, "sha256:"),
			},
		})
		encoded, err := experiment.EncodeExecutionReceipt(receipt)
		if err != nil {
			t.Fatalf("extra-row receipt must remain internally consistent: %v", err)
		}
		plantReceiptOnlyRun(t, fixture, "run-omega", encoded, []byte("workload-bytes\n"))
		result, releaser := run(t, fixture)
		if result.Outcome.Classification != ClassificationOperational {
			t.Fatalf("extra candidate row outcome = %+v, want operational refusal", result.Outcome)
		}
		if len(releaser.calls) != 0 {
			t.Fatalf("extra candidate row still released workspaces: %v", releaser.calls)
		}
	})

	t.Run("duplicate and reordered candidate rows are refused at decode", func(t *testing.T) {
		for name, mutate := range map[string]func([]experiment.ReceiptCandidate) []experiment.ReceiptCandidate{
			"duplicate": func(rows []experiment.ReceiptCandidate) []experiment.ReceiptCandidate {
				return append([]experiment.ReceiptCandidate{rows[0]}, rows...)
			},
			"reordered": func(rows []experiment.ReceiptCandidate) []experiment.ReceiptCandidate {
				reversed := append([]experiment.ReceiptCandidate(nil), rows...)
				for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
					reversed[i], reversed[j] = reversed[j], reversed[i]
				}
				return reversed
			},
		} {
			t.Run(name, func(t *testing.T) {
				fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionRejectAll, "")
				receipt := baseReceipt(t, fixture)
				receipt.Fingerprint.InputDigests = fullInputs(fixture.locked)
				receipt.Candidates = mutate(receipt.Candidates)
				encoded, err := canonjson.Marshal(receipt)
				if err != nil {
					t.Fatal(err)
				}
				plantReceiptOnlyRun(t, fixture, "run-omega", encoded, []byte("workload-bytes\n"))
				result, releaser := run(t, fixture)
				if result.Outcome.Classification != ClassificationOperational {
					t.Fatalf("%s rows outcome = %+v, want operational refusal", name, result.Outcome)
				}
				if len(releaser.calls) != 0 {
					t.Fatalf("%s rows still released workspaces: %v", name, releaser.calls)
				}
			})
		}
	})
}

// TestReleaseRatifiedSingleAcceptedEnumeration proves a complete
// successful release performs exactly one HEAD resolution, one recursive
// tree enumeration, and no duplicate accepted-blob reads (design §7).
func TestReleaseRatifiedSingleAcceptedEnumeration(t *testing.T) {
	fixture := buildReleaseFixture(t, []byte("workload-bytes\n"), experiment.DispositionSelectRecommended, "")
	fixture.git.headCalls = 0
	fixture.git.treeCalls = nil
	fixture.git.blobCalls = nil
	result := fixture.service.ReleaseRatified(context.Background(), fixture.identity, releaseAuthority(t, &fakeWorkspaceReleaser{}))
	if result.Outcome.Classification != ClassificationClean {
		t.Fatalf("ReleaseRatified() outcome = %+v", result.Outcome)
	}
	if fixture.git.headCalls != 1 {
		t.Fatalf("accepted HEAD resolved %d times, want exactly once", fixture.git.headCalls)
	}
	if len(fixture.git.treeCalls) != 1 {
		t.Fatalf("accepted tree enumerated %d times, want exactly once: %v", len(fixture.git.treeCalls), fixture.git.treeCalls)
	}
	seenBlob := map[string]bool{}
	for _, call := range fixture.git.blobCalls {
		if seenBlob[call] {
			t.Fatalf("duplicate accepted blob read %q", call)
		}
		seenBlob[call] = true
	}
}
