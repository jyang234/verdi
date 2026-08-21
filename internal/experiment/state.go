package experiment

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	definitionFile   = "experiment.yaml"
	runsDirectory    = "runs"
	executionFile    = "execution.json"
	observationsFile = "observations.jsonl"
	resultFile       = "result.json"
	ratificationFile = "ratification.yaml"
)

// ResultVerifier recomputes a stored result's engine-owned decision and,
// for V2, validates the exact run receipt and harness-owned annex.
type ResultVerifier func(Definition, []Observation, *ExecutionReceipt, Result) error

// RunState is one durable run and its independently derived posture.
type RunState struct {
	Run          string
	State        State
	ResultDigest string
}

// StateDerivation keeps the aggregate label, every enumerable run, and the
// authority disclosures that remain unproven by the stored artifacts.
type StateDerivation struct {
	State       State
	Runs        []RunState
	Disclosures []StateDisclosure
}

type runEvidence struct {
	state  RunState
	result *Result
}

// DeriveState preserves the established aggregate-state API. Callers that
// present runs use DeriveStateDetails to retain the per-run enumeration.
func DeriveState(repoRoot, experimentDir string, verify ResultVerifier) (State, []StateDisclosure, error) {
	derived, err := DeriveStateDetails(repoRoot, experimentDir, verify)
	if err != nil {
		return "", nil, err
	}
	return derived.State, derived.Disclosures, nil
}

// DeriveStateDetails derives the aggregate posture without selecting a
// preferred run and returns all run postures in lexical run-id order.
func DeriveStateDetails(repoRoot, experimentDir string, verify ResultVerifier) (StateDerivation, error) {
	if verify == nil {
		return StateDerivation{}, fmt.Errorf("experiment: DeriveState requires a result verifier (a present result.json is state-bearing only when it recomputes)")
	}
	if err := ValidateRepoRelativePath(experimentDir); err != nil {
		return StateDerivation{}, fmt.Errorf("experiment: experiment directory: %w", err)
	}
	dir := filepath.Join(repoRoot, experimentDir)
	def, ok, err := readDefinition(dir)
	if err != nil {
		return StateDerivation{}, err
	}
	if !ok {
		return StateDerivation{State: StateExploratory, Runs: []RunState{}, Disclosures: []StateDisclosure{}}, nil
	}
	locked, err := Locked(def)
	if err != nil {
		return StateDerivation{}, fmt.Errorf("experiment: %s: %w", filepath.Join(dir, definitionFile), err)
	}
	if !locked {
		return StateDerivation{State: StateExploratory, Runs: []RunState{}, Disclosures: []StateDisclosure{}}, nil
	}
	defDigest, err := DefinitionDigest(def)
	if err != nil {
		return StateDerivation{}, err
	}
	if err := checkCandidatePatchesValid(dir, experimentDir, def); err != nil {
		return StateDerivation{}, err
	}
	if err := rejectRootRunArtifacts(dir); err != nil {
		return StateDerivation{}, err
	}

	evidence, err := readRuns(dir, defDigest, def, verify)
	if err != nil {
		return StateDerivation{}, err
	}
	runs := make([]RunState, len(evidence))
	results := make([]Result, 0, len(evidence))
	completeRuns := 0
	hasV1Result := false
	for i, run := range evidence {
		runs[i] = run.state
		if run.state.State != StateRegistered {
			completeRuns++
		}
		if run.result != nil {
			results = append(results, *run.result)
			hasV1Result = hasV1Result || run.result.Schema == ResultSchema
		}
	}

	disclosures := []StateDisclosure{lockWitnessDisclosure()}
	ratification, ratified, err := readRatificationRecord(dir)
	if err != nil {
		return StateDerivation{}, err
	}
	if ratified {
		matches := make([]Result, 0, 1)
		for _, result := range results {
			digest, err := ResultDigest(result)
			if err != nil {
				return StateDerivation{}, err
			}
			if digest == ratification.ResultDigest {
				matches = append(matches, result)
			}
		}
		if len(matches) != 1 {
			return StateDerivation{}, fmt.Errorf("experiment: %s: result_digest %q matches %d run results, want exactly one", filepath.Join(dir, ratificationFile), ratification.ResultDigest, len(matches))
		}
		if err := ValidateRatificationBinding(def, matches[0], ratification); err != nil {
			return StateDerivation{}, fmt.Errorf("experiment: %s: %w", filepath.Join(dir, ratificationFile), err)
		}
		if matches[0].Schema == ResultSchema {
			disclosures = append(disclosures, environmentReceiptDisclosure())
		}
		disclosures = append(disclosures, actorResolutionDisclosure())
		return StateDerivation{State: StateRatified, Runs: runs, Disclosures: disclosures}, nil
	}

	if len(results) == 0 {
		state := StateRegistered
		if completeRuns > 0 {
			state = StateMeasured
		}
		return StateDerivation{State: state, Runs: runs, Disclosures: disclosures}, nil
	}
	if hasV1Result {
		disclosures = append(disclosures, environmentReceiptDisclosure())
	}
	state := StateRecommended
	winner := ""
	for _, result := range results {
		decision := result.decisionDocument()
		if decision.Verdict != VerdictProvenWinner || decision.Winner == "" {
			state = StateInconclusive
			break
		}
		if winner == "" {
			winner = decision.Winner
		} else if winner != decision.Winner {
			state = StateInconclusive
			break
		}
	}
	return StateDerivation{State: state, Runs: runs, Disclosures: disclosures}, nil
}

func readDefinition(dir string) (Definition, bool, error) {
	path := filepath.Join(dir, definitionFile)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Definition{}, false, nil
	}
	if err != nil {
		return Definition{}, false, fmt.Errorf("experiment: reading %s: %w", path, err)
	}
	def, err := DecodeDefinition(raw)
	if err != nil {
		return Definition{}, false, fmt.Errorf("experiment: %s: %w", path, err)
	}
	return def, true, nil
}

func checkCandidatePatchesValid(dir, experimentDir string, def Definition) error {
	for _, candidate := range def.Candidates {
		path := filepath.Join(dir, candidate.Patch)
		raw, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("experiment: locked definition %q: candidate %q patch %s is missing", def.ID, candidate.ID, path)
			}
			return fmt.Errorf("experiment: reading %s: %w", path, err)
		}
		if err := ValidateCandidatePatch(def, candidate.ID, raw, experimentDir); err != nil {
			return fmt.Errorf("experiment: %s: %w", path, err)
		}
	}
	return nil
}

func rejectRootRunArtifacts(dir string) error {
	for _, name := range []string{executionFile, observationsFile, resultFile} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("experiment: obsolete root-level run artifact %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("experiment: stat %s: %w", path, err)
		}
	}
	return nil
}

func readRuns(dir, defDigest string, def Definition, verify ResultVerifier) ([]runEvidence, error) {
	runsPath := filepath.Join(dir, runsDirectory)
	entries, err := os.ReadDir(runsPath)
	if errors.Is(err, os.ErrNotExist) {
		return []runEvidence{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("experiment: reading %s: %w", runsPath, err)
	}
	runs := make([]runEvidence, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			return nil, fmt.Errorf("experiment: malformed run entry %s is not a directory", filepath.Join(runsPath, entry.Name()))
		}
		if err := ValidateID(entry.Name()); err != nil {
			return nil, fmt.Errorf("experiment: malformed run directory %q: %w", entry.Name(), err)
		}
		run, err := readRun(filepath.Join(runsPath, entry.Name()), entry.Name(), defDigest, def, verify)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func readRun(dir, runID, defDigest string, def Definition, verify ResultVerifier) (runEvidence, error) {
	receipt, hasReceipt, err := readExecutionReceipt(dir)
	if err != nil {
		return runEvidence{}, err
	}
	if hasReceipt && (receipt.ExperimentDigest != defDigest || receipt.Run != runID) {
		return runEvidence{}, fmt.Errorf("experiment: %s: execution receipt identity does not match definition/run", dir)
	}

	resultExists, err := artifactExists(filepath.Join(dir, resultFile))
	if err != nil {
		return runEvidence{}, err
	}
	obsPath := filepath.Join(dir, observationsFile)
	raw, err := os.ReadFile(obsPath)
	if errors.Is(err, os.ErrNotExist) {
		if resultExists {
			return runEvidence{}, fmt.Errorf("experiment: %s exists without observations.jsonl", filepath.Join(dir, resultFile))
		}
		return runEvidence{state: RunState{Run: runID, State: StateRegistered}}, nil
	}
	if err != nil {
		return runEvidence{}, fmt.Errorf("experiment: reading %s: %w", obsPath, err)
	}
	if len(raw) == 0 {
		if resultExists {
			return runEvidence{}, fmt.Errorf("experiment: %s exists with an empty observations.jsonl", filepath.Join(dir, resultFile))
		}
		return runEvidence{state: RunState{Run: runID, State: StateRegistered}}, nil
	}
	observations, err := DecodeObservations(raw)
	if err != nil {
		return runEvidence{}, fmt.Errorf("experiment: %s: %w", obsPath, err)
	}
	if err := ValidateObservations(def, observations); err != nil {
		return runEvidence{}, fmt.Errorf("experiment: %s: %w", obsPath, err)
	}
	if observations[0].Run != runID {
		return runEvidence{}, fmt.Errorf("experiment: %s: observation run %q does not match directory %q", obsPath, observations[0].Run, runID)
	}
	if err := ValidateComplete(def, observations); err != nil {
		if errors.Is(err, ErrObservationIncomplete) && !resultExists {
			return runEvidence{state: RunState{Run: runID, State: StateRegistered}}, nil
		}
		return runEvidence{}, fmt.Errorf("experiment: %s: %w", obsPath, err)
	}
	if !resultExists {
		return runEvidence{state: RunState{Run: runID, State: StateMeasured}}, nil
	}

	resultPath := filepath.Join(dir, resultFile)
	resultRaw, err := os.ReadFile(resultPath)
	if err != nil {
		return runEvidence{}, fmt.Errorf("experiment: reading %s: %w", resultPath, err)
	}
	result, err := DecodeResult(resultRaw)
	if err != nil {
		return runEvidence{}, fmt.Errorf("experiment: %s: %w", resultPath, err)
	}
	decision := result.decisionDocument()
	if decision.DefinitionDigest != defDigest || decision.Run != runID || decision.Algorithm != def.Algorithm {
		return runEvidence{}, fmt.Errorf("experiment: %s: result identity does not match locked definition/run", resultPath)
	}
	var receiptArg *ExecutionReceipt
	if result.Schema == ResultSchemaV2 {
		if observations[0].Schema != ObservationSchemaV2 {
			return runEvidence{}, fmt.Errorf("experiment: %s: result v2 requires observation v2", resultPath)
		}
		if !hasReceipt {
			return runEvidence{}, fmt.Errorf("experiment: %s: result v2 requires %s", resultPath, executionFile)
		}
		receiptArg = &receipt
	} else if observations[0].Schema != ObservationSchema {
		return runEvidence{}, fmt.Errorf("experiment: %s: result v1 requires observation v1", resultPath)
	}
	if err := verify(def, observations, receiptArg, result); err != nil {
		return runEvidence{}, fmt.Errorf("experiment: %s: result does not verify against the locked definition and observations: %w", resultPath, err)
	}
	digest, err := ResultDigest(result)
	if err != nil {
		return runEvidence{}, err
	}
	state := StateInconclusive
	if decision.Verdict == VerdictProvenWinner {
		state = StateRecommended
	}
	return runEvidence{state: RunState{Run: runID, State: state, ResultDigest: digest}, result: &result}, nil
}

func readExecutionReceipt(dir string) (ExecutionReceipt, bool, error) {
	path := filepath.Join(dir, executionFile)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ExecutionReceipt{}, false, nil
	}
	if err != nil {
		return ExecutionReceipt{}, false, fmt.Errorf("experiment: reading %s: %w", path, err)
	}
	receipt, err := DecodeExecutionReceipt(raw)
	if err != nil {
		return ExecutionReceipt{}, false, fmt.Errorf("experiment: %s: %w", path, err)
	}
	return receipt, true, nil
}

func artifactExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("experiment: stat %s: %w", path, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("experiment: artifact %s is a directory", path)
	}
	return true, nil
}

func readRatificationRecord(dir string) (Ratification, bool, error) {
	path := filepath.Join(dir, ratificationFile)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Ratification{}, false, nil
	}
	if err != nil {
		return Ratification{}, false, fmt.Errorf("experiment: reading %s: %w", path, err)
	}
	ratification, err := DecodeRatification(raw)
	if err != nil {
		return Ratification{}, false, fmt.Errorf("experiment: %s: %w", path, err)
	}
	return ratification, true, nil
}
