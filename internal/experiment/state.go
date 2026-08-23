package experiment

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
)

const (
	definitionFile   = "experiment.yaml"
	capabilitiesFile = "evaluator-capabilities.json"
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

// StateDerivation keeps the aggregate label, every enumerable run, the fixed
// disclosures, and the predecessor-derived reproduction posture.
type StateDerivation struct {
	State        State
	Runs         []RunState
	Disclosures  []StateDisclosure
	Reproduction ReproductionStatus
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
	return DeriveStateDetailsFromSource(os.DirFS(repoRoot), experimentDir, verify)
}

// DeriveStateDetailsFromSource runs the sole experiment state algorithm over
// one caller-sealed byte tree. The filesystem API is an adapter to this entry
// point; accepted Git callers can supply bytes from one exact commit.
func DeriveStateDetailsFromSource(source fs.FS, experimentDir string, verify ResultVerifier) (StateDerivation, error) {
	if verify == nil {
		return StateDerivation{}, fmt.Errorf("experiment: DeriveState requires a result verifier (a present result.json is state-bearing only when it recomputes)")
	}
	if source == nil {
		return StateDerivation{}, fmt.Errorf("experiment: state byte source is nil")
	}
	if err := ValidateRepoRelativePath(experimentDir); err != nil {
		return StateDerivation{}, fmt.Errorf("experiment: experiment directory: %w", err)
	}
	dir := experimentDir
	def, ok, err := readDefinition(source, dir)
	if err != nil {
		return StateDerivation{}, err
	}
	if !ok {
		return StateDerivation{State: StateExploratory, Runs: []RunState{}, Disclosures: []StateDisclosure{}}, nil
	}
	locked, err := Locked(def)
	if err != nil {
		return StateDerivation{}, fmt.Errorf("experiment: %s: %w", path.Join(dir, definitionFile), err)
	}
	if !locked {
		reproduction, err := DeriveReproduction(def, []ReproductionRun{}, nil)
		if err != nil {
			return StateDerivation{}, err
		}
		return StateDerivation{State: StateExploratory, Runs: []RunState{}, Disclosures: []StateDisclosure{}, Reproduction: reproduction}, nil
	}
	defDigest, err := DefinitionDigest(def)
	if err != nil {
		return StateDerivation{}, err
	}
	if _, err := ValidateCandidatePatchesFromSource(source, experimentDir, def); err != nil {
		return StateDerivation{}, err
	}
	if err := rejectRootRunArtifacts(source, dir); err != nil {
		return StateDerivation{}, err
	}

	evidence, err := readRuns(source, dir, defDigest, def, verify)
	if err != nil {
		return StateDerivation{}, err
	}
	runs := make([]RunState, len(evidence))
	reproductionRuns := make([]ReproductionRun, len(evidence))
	results := make([]Result, 0, len(evidence))
	completeRuns := 0
	hasV1Result := false
	for i, run := range evidence {
		runs[i] = run.state
		reproductionRuns[i] = ReproductionRun{Run: run.state.Run, Result: run.result}
		if run.state.State != StateRegistered {
			completeRuns++
		}
		if run.result != nil {
			results = append(results, *run.result)
			hasV1Result = hasV1Result || run.result.Schema == ResultSchema
		}
	}

	disclosures := []StateDisclosure{lockWitnessDisclosure()}
	ratification, ratified, err := readRatificationRecord(source, dir)
	if err != nil {
		return StateDerivation{}, err
	}
	var ratificationPtr *Ratification
	if ratified {
		ratificationPtr = &ratification
	}
	reproduction, err := DeriveReproduction(def, reproductionRuns, ratificationPtr)
	if err != nil {
		return StateDerivation{}, fmt.Errorf("experiment: derive reproduction posture: %w", err)
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
			return StateDerivation{}, fmt.Errorf("experiment: %s: result_digest %q matches %d run results, want exactly one", path.Join(dir, ratificationFile), ratification.ResultDigest, len(matches))
		}
		if err := ValidateRatificationBinding(def, matches[0], ratification); err != nil {
			return StateDerivation{}, fmt.Errorf("experiment: %s: %w", path.Join(dir, ratificationFile), err)
		}
		if matches[0].Schema == ResultSchema {
			disclosures = append(disclosures, environmentReceiptDisclosure())
		}
		disclosures = append(disclosures, actorResolutionDisclosure())
		return StateDerivation{State: StateRatified, Runs: runs, Disclosures: disclosures, Reproduction: reproduction}, nil
	}

	if len(results) == 0 {
		state := StateRegistered
		if completeRuns > 0 {
			state = StateMeasured
		}
		return StateDerivation{State: state, Runs: runs, Disclosures: disclosures, Reproduction: reproduction}, nil
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
	return StateDerivation{State: state, Runs: runs, Disclosures: disclosures, Reproduction: reproduction}, nil
}

func readDefinition(source fs.FS, dir string) (Definition, bool, error) {
	filePath := path.Join(dir, definitionFile)
	raw, err := fs.ReadFile(source, filePath)
	if errors.Is(err, fs.ErrNotExist) {
		return Definition{}, false, nil
	}
	if err != nil {
		return Definition{}, false, fmt.Errorf("experiment: reading %s: %w", filePath, err)
	}
	def, err := DecodeDefinition(raw)
	if err != nil {
		return Definition{}, false, fmt.Errorf("experiment: %s: %w", filePath, err)
	}
	return def, true, nil
}

// ValidateCandidatePatchesFromSource validates every registered patch from the
// same source and returns its sorted, unique touched repo paths for policy.
func ValidateCandidatePatchesFromSource(source fs.FS, experimentDir string, def Definition) ([]string, error) {
	if source == nil {
		return nil, fmt.Errorf("experiment: candidate patch byte source is nil")
	}
	if err := ValidateRepoRelativePath(experimentDir); err != nil {
		return nil, fmt.Errorf("experiment: experiment directory: %w", err)
	}
	if err := def.Validate(); err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for _, candidate := range def.Candidates {
		filePath := path.Join(experimentDir, candidate.Patch)
		raw, err := fs.ReadFile(source, filePath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("experiment: definition %q: candidate %q patch %s is missing", def.ID, candidate.ID, filePath)
			}
			return nil, fmt.Errorf("experiment: reading %s: %w", filePath, err)
		}
		if err := ValidateCandidatePatch(def, candidate.ID, raw, experimentDir); err != nil {
			return nil, fmt.Errorf("experiment: %s: %w", filePath, err)
		}
		changed, err := parsePatchPaths(raw)
		if err != nil {
			return nil, fmt.Errorf("experiment: %s: %w", filePath, err)
		}
		for _, changedPath := range changed {
			seen[changedPath] = true
		}
	}
	paths := make([]string, 0, len(seen))
	for changedPath := range seen {
		paths = append(paths, changedPath)
	}
	sort.Strings(paths)
	return paths, nil
}

func rejectRootRunArtifacts(source fs.FS, dir string) error {
	for _, name := range []string{executionFile, observationsFile, resultFile} {
		filePath := path.Join(dir, name)
		if _, err := fs.Stat(source, filePath); err == nil {
			return fmt.Errorf("experiment: obsolete root-level run artifact %s", filePath)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("experiment: stat %s: %w", filePath, err)
		}
	}
	return nil
}

func readRuns(source fs.FS, dir, defDigest string, def Definition, verify ResultVerifier) ([]runEvidence, error) {
	runsPath := path.Join(dir, runsDirectory)
	entries, err := fs.ReadDir(source, runsPath)
	if errors.Is(err, fs.ErrNotExist) {
		return []runEvidence{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("experiment: reading %s: %w", runsPath, err)
	}
	runs := make([]runEvidence, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			return nil, fmt.Errorf("experiment: malformed run entry %s is not a directory", path.Join(runsPath, entry.Name()))
		}
		if err := ValidateID(entry.Name()); err != nil {
			return nil, fmt.Errorf("experiment: malformed run directory %q: %w", entry.Name(), err)
		}
		run, err := readRun(source, dir, path.Join(runsPath, entry.Name()), entry.Name(), defDigest, def, verify)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func readRun(source fs.FS, experimentDir, dir, runID, defDigest string, def Definition, verify ResultVerifier) (runEvidence, error) {
	entries, err := fs.ReadDir(source, dir)
	if err != nil {
		return runEvidence{}, fmt.Errorf("experiment: reading run directory %s: %w", dir, err)
	}
	allowed := map[string]bool{executionFile: true, observationsFile: true, resultFile: true}
	for _, entry := range entries {
		if !allowed[entry.Name()] || entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return runEvidence{}, fmt.Errorf("experiment: malformed run directory %s contains unexpected entry %q", dir, entry.Name())
		}
	}
	receipt, hasReceipt, err := readExecutionReceipt(source, dir)
	if err != nil {
		return runEvidence{}, err
	}
	if hasReceipt && (receipt.ExperimentDigest != defDigest || receipt.Run != runID) {
		return runEvidence{}, fmt.Errorf("experiment: %s: execution receipt identity does not match definition/run", dir)
	}

	resultExists, err := artifactExists(source, path.Join(dir, resultFile))
	if err != nil {
		return runEvidence{}, err
	}
	obsPath := path.Join(dir, observationsFile)
	raw, err := fs.ReadFile(source, obsPath)
	if errors.Is(err, fs.ErrNotExist) {
		if resultExists {
			return runEvidence{}, fmt.Errorf("experiment: %s exists without observations.jsonl", path.Join(dir, resultFile))
		}
		return runEvidence{state: RunState{Run: runID, State: StateRegistered}}, nil
	}
	if err != nil {
		return runEvidence{}, fmt.Errorf("experiment: reading %s: %w", obsPath, err)
	}
	if len(raw) == 0 {
		if resultExists {
			return runEvidence{}, fmt.Errorf("experiment: %s exists with an empty observations.jsonl", path.Join(dir, resultFile))
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
	if observations[0].Schema == ObservationSchemaV2 {
		if err := validateCapabilitiesAuthority(source, experimentDir, def); err != nil {
			return runEvidence{}, err
		}
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

	resultPath := path.Join(dir, resultFile)
	resultRaw, err := fs.ReadFile(source, resultPath)
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

func validateCapabilitiesAuthority(source fs.FS, dir string, def Definition) error {
	filePath := path.Join(dir, capabilitiesFile)
	raw, err := fs.ReadFile(source, filePath)
	if err != nil {
		return fmt.Errorf("experiment: reading V2 capability authority %s: %w", filePath, err)
	}
	capabilities, err := DecodeCapabilities(raw)
	if err != nil {
		return fmt.Errorf("experiment: %s: %w", filePath, err)
	}
	if got := sha256Digest(raw); got != def.Evaluator.CapabilitiesDigest {
		return fmt.Errorf("experiment: %s digest %q does not match evaluator.capabilities_digest %q", filePath, got, def.Evaluator.CapabilitiesDigest)
	}
	if err := ValidateDefinitionCapabilities(def, capabilities); err != nil {
		return fmt.Errorf("experiment: %s does not authorize the locked decision vocabulary: %w", filePath, err)
	}
	return nil
}

func readExecutionReceipt(source fs.FS, dir string) (ExecutionReceipt, bool, error) {
	filePath := path.Join(dir, executionFile)
	raw, err := fs.ReadFile(source, filePath)
	if errors.Is(err, fs.ErrNotExist) {
		return ExecutionReceipt{}, false, nil
	}
	if err != nil {
		return ExecutionReceipt{}, false, fmt.Errorf("experiment: reading %s: %w", filePath, err)
	}
	receipt, err := DecodeExecutionReceipt(raw)
	if err != nil {
		return ExecutionReceipt{}, false, fmt.Errorf("experiment: %s: %w", filePath, err)
	}
	return receipt, true, nil
}

func artifactExists(source fs.FS, filePath string) (bool, error) {
	info, err := fs.Stat(source, filePath)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("experiment: stat %s: %w", filePath, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("experiment: artifact %s is a directory", filePath)
	}
	return true, nil
}

func readRatificationRecord(source fs.FS, dir string) (Ratification, bool, error) {
	filePath := path.Join(dir, ratificationFile)
	raw, err := fs.ReadFile(source, filePath)
	if errors.Is(err, fs.ErrNotExist) {
		return Ratification{}, false, nil
	}
	if err != nil {
		return Ratification{}, false, fmt.Errorf("experiment: reading %s: %w", filePath, err)
	}
	ratification, err := DecodeRatification(raw)
	if err != nil {
		return Ratification{}, false, fmt.Errorf("experiment: %s: %w", filePath, err)
	}
	return ratification, true, nil
}
