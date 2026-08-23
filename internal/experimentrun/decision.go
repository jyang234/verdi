package experimentrun

import (
	"fmt"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentdecision"
)

func completeRun(storage runStorage, def experiment.Definition, observations []experiment.Observation, receipt experiment.ExecutionReceipt, warmupFailures []experiment.WarmupFailure, environmentRoots []string) (experiment.Result, string, error) {
	if err := experiment.ValidateComplete(def, observations); err != nil {
		return experiment.Result{}, "", fmt.Errorf("experimentrun: complete run observations: %w", err)
	}
	if err := cleanupEnvironmentRoots(storage.root, environmentRoots); err != nil {
		return experiment.Result{}, "", err
	}
	result, digest, err := buildCompleteResult(def, observations, receipt, warmupFailures)
	if err != nil {
		return experiment.Result{}, "", err
	}
	data, err := experiment.EncodeResult(result)
	if err != nil {
		return experiment.Result{}, "", fmt.Errorf("experimentrun: encode V2 result: %w", err)
	}
	if err := storage.publishResult(data); err != nil {
		return experiment.Result{}, "", err
	}
	return result, digest, nil
}

// buildCompleteResult invokes the sole recommendation engine, projects its
// recomputable V2 decision, binds the exact receipt-owned execution annex, and
// returns the canonical whole-result digest required by later ratification.
func buildCompleteResult(def experiment.Definition, observations []experiment.Observation, receipt experiment.ExecutionReceipt, warmupFailures []experiment.WarmupFailure) (experiment.Result, string, error) {
	core, err := experimentdecision.Evaluate(def, observations, experimentdecision.EnvironmentAttestation{PolicyID: receipt.EnvironmentPolicy})
	if err != nil {
		return experiment.Result{}, "", fmt.Errorf("experimentrun: evaluate complete run: %w", err)
	}
	decision, err := experiment.DecisionFromResult(core, observations)
	if err != nil {
		return experiment.Result{}, "", fmt.Errorf("experimentrun: project V2 decision: %w", err)
	}
	receiptDigest, err := experiment.ExecutionReceiptDigest(receipt)
	if err != nil {
		return experiment.Result{}, "", fmt.Errorf("experimentrun: digest execution receipt: %w", err)
	}
	disclosures := []experiment.IsolationDisclosure{}
	if receipt.Network.Mode == experiment.NetworkAllow {
		disclosures = []experiment.IsolationDisclosure{experiment.IsolationWeaker}
	}
	failures := append([]experiment.WarmupFailure(nil), warmupFailures...)
	if failures == nil {
		failures = []experiment.WarmupFailure{}
	}
	result, err := experiment.NewResultV2(decision, experiment.ResultExecution{
		ExecutionDigest: receiptDigest,
		Isolation: experiment.ResultIsolation{
			Network:     receipt.Network,
			Disclosures: disclosures,
		},
		WarmupDiagnostics: experiment.WarmupDiagnostics{
			Authority: experiment.WarmupAuthorityNonDecisionDiagnostic,
			Scope:     experiment.WarmupScopeFinalInvocation,
			Failures:  failures,
		},
	})
	if err != nil {
		return experiment.Result{}, "", fmt.Errorf("experimentrun: construct V2 result: %w", err)
	}
	if err := experimentdecision.VerifyResult(def, observations, &receipt, result); err != nil {
		return experiment.Result{}, "", fmt.Errorf("experimentrun: verify V2 result: %w", err)
	}
	digest, err := experiment.ResultDigest(result)
	if err != nil {
		return experiment.Result{}, "", fmt.Errorf("experimentrun: digest V2 result: %w", err)
	}
	return result, digest, nil
}
