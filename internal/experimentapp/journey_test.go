package experimentapp

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentdecision"
)

type journeyWorkspaceReleaser struct {
	manifest string
	calls    []string
	fail     map[string]bool
	sawFirst bool
}

// journeyResultVerifier keeps the continuous application journey wired to
// the one closed decision verifier used by production composition. It is an
// adapter only: no decision logic is reproduced in this package.
type journeyResultVerifier struct{}

func (journeyResultVerifier) VerifyResult(def experiment.Definition, observations []experiment.Observation, receipt *experiment.ExecutionReceipt, result experiment.Result) error {
	return experimentdecision.VerifyResult(def, observations, receipt, result)
}

func (r *journeyWorkspaceReleaser) Release(workspaceID string) error {
	r.calls = append(r.calls, workspaceID)
	if len(r.calls) == 1 {
		_, err := os.Stat(r.manifest)
		r.sawFirst = err == nil
	}
	if r.fail[workspaceID] {
		return errors.New("journey cleanup refusal")
	}
	return nil
}

func TestCompleteExperimentApplicationJourney(t *testing.T) {
	ctx := context.Background()

	t.Run("one experiment from unlocked draft through closure evidence", func(t *testing.T) {
		root, service := mutationTestService(t)
		identity := testIdentity(t, root, "request-path-v2")
		definitionPath := mutationDefinitionPath(root)
		workloadBytes := []byte("journey-workload\n")
		definition := releaseDefinitionBytes(t, mustReadFile(t, definitionPath), workloadBytes)
		definition = bytes.Replace(definition, []byte("#oq-cache\n"), []byte("#oq-cache-journey\n"), 1)

		drafted := service.DraftDefinition(ctx, identity, DraftDefinitionInput{DefinitionBytes: definition})
		assertJourneyOutcome(t, "draft", drafted.Outcome, ClassificationClean, "clean")

		patch := []byte("diff --git a/spikes/cache.go b/spikes/cache.go\nindex 1111111..2222222 100644\n--- a/spikes/cache.go\n+++ b/spikes/cache.go\n@@ -1 +1 @@\n-old\n+journey\n")
		definition = bytes.Replace(definition,
			[]byte("sha256:948705e2b8a093896358025d2b75282fbd1c36557c278881add34f4c75cbecc7"),
			[]byte(rawDigest(patch)), 1)
		captured := service.CaptureCandidate(ctx, identity, CaptureCandidateInput{
			CandidateID: "cache", PatchBytes: patch, DefinitionBytes: definition,
		})
		assertJourneyOutcome(t, "capture", captured.Outcome, ClassificationClean, "clean")

		// An organization/project layer may only narrow policy. A newly
		// applicable class denial is visible and cannot write a favorable fact.
		beforePolicy := snapshotWorktree(t, root)
		policy := service.policy.(*fakePolicyResolver)
		policy.err = resolveTestPolicyRefusal(t, "class")
		denied := service.ValidateDraft(ctx, identity)
		assertJourneyOutcome(t, "layered policy", denied.Outcome, ClassificationVerdict, "policy-refused")
		if !mapsEqual(beforePolicy, snapshotWorktree(t, root)) {
			t.Fatal("layered-policy refusal mutated the proposal")
		}
		policy.err = nil

		// A direct Git/editor change is reviewable but never silently admitted.
		notePath := filepath.Join(filepath.Dir(definitionPath), "review-note.md")
		if err := os.WriteFile(notePath, []byte("human review note\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		beforeReview := snapshotWorktree(t, root)
		review := service.ReviewRegistration(ctx, identity)
		assertJourneyOutcome(t, "unreconciled review", review.Outcome, ClassificationVerdict, "direct-draft-unreconciled")
		if !mapsEqual(beforeReview, snapshotWorktree(t, root)) {
			t.Fatal("read-only review mutated an unreconciled proposal")
		}
		agentReconcile := service.ReconcileDraft(ctx, identity)
		assertJourneyOutcome(t, "agent reconcile", agentReconcile.Outcome, ClassificationVerdict, "human-actor-required")
		if !mapsEqual(beforeReview, snapshotWorktree(t, root)) {
			t.Fatal("agent reconciliation refusal mutated the proposal")
		}

		human := identity
		human.Actor = authenticatedHuman(t)
		reconciled := service.ReconcileDraft(ctx, human)
		assertJourneyOutcome(t, "human reconcile", reconciled.Outcome, ClassificationClean, "clean")
		if got := mustReadFile(t, notePath); string(got) != "human review note\n" {
			t.Fatalf("reconciliation rewrote direct content: %q", got)
		}
		review = service.ReviewRegistration(ctx, human)
		assertJourneyOutcome(t, "review", review.Outcome, ClassificationClean, "clean")

		beforeAgentLock := snapshotWorktree(t, root)
		agentLock := service.ProposeRegistration(ctx, identity, RegistrationInput{ReviewPacketDigest: review.PacketDigest})
		assertJourneyOutcome(t, "agent lock", agentLock.Outcome, ClassificationVerdict, "human-actor-required")
		if !mapsEqual(beforeAgentLock, snapshotWorktree(t, root)) {
			t.Fatal("agent lock refusal mutated the proposal")
		}
		proposed := service.ProposeRegistration(ctx, human, RegistrationInput{ReviewPacketDigest: review.PacketDigest})
		assertJourneyOutcome(t, "human lock", proposed.Outcome, ClassificationClean, "clean")
		if proposed.Accepted {
			t.Fatal("a local lock proposal claimed accepted authority")
		}
		acceptedBeforeMerge := service.AcceptedRegistration(ctx, identity)
		if acceptedBeforeMerge.Outcome.Classification == ClassificationClean {
			t.Fatal("unmerged lock was treated as accepted")
		}

		service.git = gitFromExperimentDir(t, root, "request-path-v2")
		accepted := service.AcceptedRegistration(ctx, identity)
		assertJourneyOutcome(t, "accepted lock", accepted.Outcome, ClassificationClean, "clean")
		if !accepted.Accepted {
			t.Fatal("accepted registration did not report accepted=true")
		}

		// The accepted snapshot remains authority even when the accepted HEAD
		// changes and the local worktree diverges; execution must fail closed.
		git := service.git.(*fakeGit)
		git.revision.Head = oldHead
		identity.ExpectedAcceptedHEAD = oldHead
		originalNote := mustReadFile(t, notePath)
		if err := os.WriteFile(notePath, append(originalNote, []byte("divergent worktree\n")...), 0o600); err != nil {
			t.Fatal(err)
		}
		runner := &recordingExecutionRunner{}
		service.runner = runner
		bindings := releaseInputBindings(workloadBytes)
		refused := service.Start(ctx, identity, ExecutionInput{Run: "run-divergent", Bindings: bindings})
		assertJourneyOutcome(t, "divergent execution", refused.Outcome, ClassificationOperational, "locked-input-mismatch")
		if len(runner.starts) != 0 {
			t.Fatalf("runner received a refused execution: %+v", runner.starts)
		}
		if err := os.WriteFile(notePath, originalNote, 0o600); err != nil {
			t.Fatal(err)
		}
		started := service.Start(ctx, identity, ExecutionInput{Run: "run-journey", Bindings: bindings})
		resumed := service.Resume(ctx, identity, ExecutionInput{Run: "run-journey", Bindings: bindings})
		assertJourneyOutcome(t, "start", started.Outcome, ClassificationClean, "clean")
		assertJourneyOutcome(t, "resume", resumed.Outcome, ClassificationClean, "clean")
		if len(runner.starts) != 1 || len(runner.resumes) != 1 {
			t.Fatalf("runner calls start=%d resume=%d, want 1/1", len(runner.starts), len(runner.resumes))
		}

		// Darwin cannot execute the Linux-isolation runner. Install the
		// authoritative receipt/observation/result records into this same
		// registered experiment, then continue every accepted-state operation
		// against that one repository identity.
		identity.ExpectedAcceptedHEAD = testHead
		service.results = journeyResultVerifier{}
		locked, err := experiment.DecodeDefinition(mustReadFile(t, definitionPath))
		if err != nil {
			t.Fatal(err)
		}
		definitionDigest, err := experiment.DefinitionDigest(locked)
		if err != nil {
			t.Fatal(err)
		}
		winnerDigest := writeReleasableRun(t, root, locked, "run-alpha", 50, workloadBytes)
		writeReleasableRun(t, root, locked, "run-zeta", 100, workloadBytes)

		first, second := newRatificationSigner(t), newRatificationSigner(t)
		subjects := []string{first.subject, second.subject}
		experimentDir := filepath.Dir(definitionPath)
		resultPath := filepath.Join(experimentDir, "runs", "run-alpha", "result.json")
		honestResult := mustReadFile(t, resultPath)
		forged, err := experiment.DecodeResult(honestResult)
		if err != nil {
			t.Fatal(err)
		}
		forged.Decision.Winner = "baseline"
		for i := range forged.Decision.Candidates {
			switch forged.Decision.Candidates[i].ID {
			case "baseline":
				forged.Decision.Candidates[i].Baseline = false
				forged.Decision.Candidates[i].Eligible = true
				forged.Decision.Candidates[i].Violations = nil
			case "cache":
				forged.Decision.Candidates[i].Baseline = true
			}
		}
		forgedResult, err := experiment.EncodeResult(forged)
		if err != nil {
			t.Fatalf("schema-valid forged result: %v", err)
		}
		if _, err := experiment.DecodeResult(forgedResult); err != nil {
			t.Fatalf("strict-decode forged result: %v", err)
		}
		if err := os.WriteFile(resultPath, forgedResult, 0o600); err != nil {
			t.Fatal(err)
		}
		forgedDigest, err := experiment.ResultDigest(forged)
		if err != nil {
			t.Fatal(err)
		}
		git = refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
		addReleaseProtectedInputs(t, git, workloadBytes)
		forgedProof := mintRatificationVerification(t, root, identity, first, subjects, forgedDigest, experiment.DispositionSelectRecommended, "", "")
		forgedHuman := humanRatificationIdentity(t, identity, forgedProof.Resolution)
		beforeForgery := snapshotWorktree(t, root)
		forgedProposal := service.ProposeRatification(ctx, forgedHuman, ratificationProposalInputFrom(forgedProof, forgedDigest, experiment.DispositionSelectRecommended, "", ""))
		assertJourneyOutcome(t, "forged ratification", forgedProposal.Outcome, ClassificationOperational, "state-invalid")
		forgedReleaser := &journeyWorkspaceReleaser{manifest: releaseManifestPath(root)}
		for _, refusal := range []struct {
			operation string
			outcome   Outcome
		}{
			{operation: "forged publish", outcome: service.PublishRatifiedCapsule(ctx, identity).Outcome},
			{operation: "forged release", outcome: service.ReleaseRatified(ctx, identity, releaseAuthority(t, forgedReleaser)).Outcome},
			{operation: "forged closure", outcome: service.VerifyAcceptedClosureEvidence(ctx, identity).Outcome},
		} {
			if refusal.outcome.Classification == ClassificationClean {
				t.Fatalf("%s accepted forged evidence: %+v", refusal.operation, refusal.outcome)
			}
		}
		if len(forgedReleaser.calls) != 0 {
			t.Fatalf("forged evidence released workspaces: %v", forgedReleaser.calls)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(definitionPath), "ratification.yaml")); !os.IsNotExist(err) {
			t.Fatalf("forged evidence wrote ratification: %v", err)
		}
		if _, err := os.Stat(releaseManifestPath(root)); !os.IsNotExist(err) {
			t.Fatalf("forged evidence wrote capsule: %v", err)
		}
		if !mapsEqual(beforeForgery, snapshotWorktree(t, root)) {
			t.Fatal("forged evidence refusals mutated the proposal")
		}
		if err := os.WriteFile(resultPath, honestResult, 0o600); err != nil {
			t.Fatal(err)
		}
		git = refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
		addReleaseProtectedInputs(t, git, workloadBytes)
		firstProof := mintRatificationVerification(t, root, identity, first, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		secondProof := mintRatificationVerification(t, root, identity, second, subjects, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		human = humanRatificationIdentity(t, identity, firstProof.Resolution)

		before := snapshotWorktree(t, root)
		substituted := ratificationProposalInputFrom(firstProof, winnerDigest, experiment.DispositionSelectRecommended, "", "")
		substituted.Proof = secondProof.Retained
		mismatch := service.ProposeRatification(ctx, human, substituted)
		assertJourneyOutcome(t, "ratification substitution", mismatch.Outcome, ClassificationOperational, "ratification-proof-mismatch")
		agent := service.ProposeRatification(ctx, identity, ratificationProposalInputFrom(firstProof, winnerDigest, experiment.DispositionSelectRecommended, "", ""))
		assertJourneyOutcome(t, "agent ratification", agent.Outcome, ClassificationVerdict, "human-actor-required")
		if !mapsEqual(before, snapshotWorktree(t, root)) {
			t.Fatal("ratification refusals mutated the proposal")
		}

		proposal := service.ProposeRatification(ctx, human, ratificationProposalInputFrom(firstProof, winnerDigest, experiment.DispositionSelectRecommended, "", ""))
		assertJourneyOutcome(t, "ratification proposal", proposal.Outcome, ClassificationClean, "clean")
		unaccepted := service.AcceptedRatification(ctx, human)
		assertJourneyOutcome(t, "unaccepted ratification", unaccepted.Outcome, ClassificationVerdict, "ratification-not-accepted")
		git = refreshAcceptedGit(t, service, root, "request-path-v2", subjects...)
		addReleaseProtectedInputs(t, git, workloadBytes)
		acceptedRatification := service.AcceptedRatification(ctx, human)
		assertJourneyOutcome(t, "accepted ratification", acceptedRatification.Outcome, ClassificationClean, "clean")

		fixture := releaseFixture{
			root: root, service: service, identity: identity, git: git,
			verification: firstProof, subjects: subjects, locked: locked,
			defDigest: definitionDigest, winnerDigest: winnerDigest,
			targets: releaseFixtureTargets(t, root, locked, definitionDigest, []string{"run-alpha", "run-zeta"}),
		}
		status := fixture.service.Status(ctx, fixture.identity)
		assertJourneyOutcome(t, "ratified status", status.Outcome, ClassificationClean, "clean")
		if status.State != experiment.StateRatified || len(status.Runs) != 2 || status.Reproduction.Reproduced || status.Reproduction.ValidRuns != 2 {
			t.Fatalf("status omitted an inconclusive rerun or overstated reproduction: %+v", status)
		}
		var inconclusive *experiment.RunState
		for index := range status.Runs {
			if status.Runs[index].Run == "run-zeta" {
				inconclusive = &status.Runs[index]
			}
		}
		inconclusiveResult, err := experiment.DecodeResult(mustReadFile(t, filepath.Join(experimentDir, "runs", "run-zeta", "result.json")))
		if err != nil {
			t.Fatal(err)
		}
		if inconclusive == nil || inconclusive.State != experiment.StateInconclusive || inconclusiveResult.Decision.Winner != "" || inconclusiveResult.Decision.Verdict != experiment.VerdictDisclosedUnproven || len(inconclusiveResult.Decision.Reasons) != 1 || inconclusiveResult.Decision.Reasons[0].Code != experiment.ReasonInsufficientBaselineImprovement {
			t.Fatalf("run-zeta did not retain its disclosed-unproven insufficient-improvement result: state=%+v result=%+v", inconclusive, inconclusiveResult.Decision)
		}
		explained := fixture.service.Explain(ctx, fixture.identity, ExplainInput{Run: "run-alpha"})
		assertJourneyOutcome(t, "explain", explained.Outcome, ClassificationClean, "clean")
		if explained.Decision.Winner != "cache" || explained.Reproduction.Reproduced {
			t.Fatalf("explanation changed winner/reproduction facts: %+v", explained)
		}

		published := fixture.service.PublishRatifiedCapsule(ctx, fixture.identity)
		assertJourneyOutcome(t, "publish", published.Outcome, ClassificationClean, "clean")
		firstManifest := mustReadFile(t, releaseManifestPath(fixture.root))
		again := fixture.service.PublishRatifiedCapsule(ctx, fixture.identity)
		assertJourneyOutcome(t, "republish", again.Outcome, ClassificationClean, "clean")
		if published.ManifestDigest != again.ManifestDigest || !bytes.Equal(firstManifest, mustReadFile(t, releaseManifestPath(fixture.root))) {
			t.Fatal("capsule publication was not deterministic")
		}
		manifest, err := experiment.DecodeCapsuleManifest(firstManifest)
		if err != nil || manifest.ResultDigest != fixture.winnerDigest || manifest.Selected != "cache" {
			t.Fatalf("published manifest invalid: manifest=%+v err=%v", manifest, err)
		}

		experimentDir = filepath.Dir(mutationDefinitionPath(fixture.root))
		durableBefore := map[string][]byte{
			"ratification": mustReadFile(t, filepath.Join(experimentDir, "ratification.yaml")),
			"result":       mustReadFile(t, filepath.Join(experimentDir, "runs", "run-alpha", "result.json")),
			"manifest":     firstManifest,
		}
		failing := &journeyWorkspaceReleaser{
			manifest: releaseManifestPath(fixture.root),
			fail:     map[string]bool{fixture.targets[1]: true},
		}
		released := fixture.service.ReleaseRatified(ctx, fixture.identity, releaseAuthority(t, failing))
		if released.Outcome.Classification != ClassificationOperational || len(released.Failed) != 1 || !failing.sawFirst {
			t.Fatalf("partial release did not fail after capsule publication: result=%+v releaser=%+v", released, failing)
		}
		if len(failing.calls) != len(fixture.targets) {
			t.Fatalf("partial release attempted %d targets, want %d", len(failing.calls), len(fixture.targets))
		}
		for name, want := range durableBefore {
			var file string
			switch name {
			case "ratification":
				file = filepath.Join(experimentDir, "ratification.yaml")
			case "result":
				file = filepath.Join(experimentDir, "runs", "run-alpha", "result.json")
			default:
				file = releaseManifestPath(fixture.root)
			}
			if got := mustReadFile(t, file); !bytes.Equal(got, want) {
				t.Fatalf("cleanup failure changed durable %s", name)
			}
		}
		retry := fixture.service.ReleaseRatified(ctx, fixture.identity, releaseAuthority(t, &journeyWorkspaceReleaser{manifest: releaseManifestPath(fixture.root)}))
		assertJourneyOutcome(t, "release retry", retry.Outcome, ClassificationClean, "clean")

		prefix := ".verdi/specs/active/request-path-spike/experiments/request-path-v2/"
		plantGitBlob(fixture.git, prefix+"selected/capsule-manifest.json", firstManifest)
		closure := fixture.service.VerifyAcceptedClosureEvidence(ctx, fixture.identity)
		assertJourneyOutcome(t, "closure evidence", closure.Outcome, ClassificationClean, "clean")
		if closure.Capsule == nil || closure.Capsule.ManifestDigest != published.ManifestDigest || !closure.Selecting {
			t.Fatalf("closure evidence did not bind the published capsule: %+v", closure)
		}
	})

	t.Run("faster incorrect candidate remains ineligible", func(t *testing.T) {
		fixtureRoot := filepath.Join("..", "experimentdecision", "testdata")
		state, _, err := experiment.DeriveState(fixtureRoot, "caching-proven", experimentdecision.VerifyResult)
		if err != nil || state != experiment.StateRecommended {
			t.Fatalf("DeriveState(caching-proven) state=%q err=%v", state, err)
		}
		raw := mustReadFile(t, filepath.Join(fixtureRoot, "caching-proven", "runs", "run-1", "result.json"))
		result, err := experiment.DecodeResult(raw)
		if err != nil {
			t.Fatal(err)
		}
		decision := result.Decision
		if decision.Winner != "facts-cache" || decision.DefinitionDigest != "sha256:e24c6781bee8d3c2f12f98453e93f0a849e0aacb7546b007d4b08aa421c9dfeb" {
			t.Fatalf("decision identity/winner = %+v", decision)
		}
		var fast, correct *experiment.DecisionCandidate
		for i := range decision.Candidates {
			switch decision.Candidates[i].ID {
			case "final-cache":
				fast = &decision.Candidates[i]
			case "facts-cache":
				correct = &decision.Candidates[i]
			}
		}
		if fast == nil || correct == nil || fast.Primary == nil || correct.Primary == nil || fast.Primary.Value >= correct.Primary.Value || fast.Eligible || len(fast.Violations) != 1 || fast.Violations[0].Guard != "behavioral-equivalence" {
			t.Fatalf("faster incorrect candidate became favorable: fast=%+v correct=%+v", fast, correct)
		}
		digest, err := experiment.ResultDigest(result)
		if err != nil || digest != "sha256:9417813890938576f5c92d3131b7a24e468f64065cbe1a9321928a441640228a" {
			t.Fatalf("fixture result digest=%q err=%v", digest, err)
		}
	})
}

func assertJourneyOutcome(t *testing.T, operation string, got Outcome, classification Classification, code string) {
	t.Helper()
	if got.Classification != classification || got.Code != code {
		t.Fatalf("%s outcome=%+v, want %s/%s", operation, got, classification, code)
	}
	if got.ExitCode() != map[Classification]int{ClassificationClean: 0, ClassificationVerdict: 1, ClassificationOperational: 2}[classification] {
		t.Fatalf("%s exit projection=%d for %+v", operation, got.ExitCode(), got)
	}
	if strings.TrimSpace(got.Detail) == "" {
		t.Fatalf("%s outcome omitted its witness", operation)
	}
}
