package experiment

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// definitionFile, observationsFile, resultFile, and ratificationFile are
// the fixed artifact filenames DeriveState looks for inside an experiment
// directory (AC-1's layout).
const (
	definitionFile   = "experiment.yaml"
	observationsFile = "observations.jsonl"
	resultFile       = "result.json"
	ratificationFile = "ratification.yaml"
)

// ResultVerifier is the port DeriveState requires to treat a PRESENT
// result.json as state-bearing (invention ledger SI-42): given the locked
// definition, the complete observation set, and the decoded result, it
// answers whether that result IS the closed decision engine's own output
// for that evidence, or an operational error explaining why not.
//
// It is a func port defined HERE, at the consumer, and injected by
// callers, because the only implementation — internal/experimentdecision's
// recompute-equality check — imports this package. A shape-, digest-, and
// algorithm-checked result.json is still hand-writable to name any winner
// (the Git-edit mutation surface AC-5/AC-6 leave open), so shape checking
// alone cannot decide the recommended/inconclusive rungs; recomputation
// can, and it is the caller's job to supply it.
type ResultVerifier func(Definition, []Observation, Result) error

// DeriveState computes an experiment's derived lifecycle state from the
// presence and validity of its artifacts (AC-1's state table, DC-2):
// exploratory, registered, measured, recommended, inconclusive, or
// ratified. States form a ladder — reaching a state requires every lower
// state's requirement to still hold — and DeriveState walks the ladder in
// order, stopping at the first artifact that is genuinely ABSENT.
//
// The experiment is addressed by TWO arguments, because the artifacts are
// read in filesystem coordinates while the protected-input self-check
// (ValidateCandidatePatch) compares in repo coordinates:
//
//   - repoRoot is the filesystem directory to read under, absolute or
//     relative to the process's working directory; it is never compared to
//     anything a patch names.
//   - experimentDir is the experiment directory's canonical REPO-RELATIVE
//     path (ValidateRepoRelativePath). It is joined onto repoRoot to reach
//     the artifacts AND used as the protected prefix naming the
//     experiment's own directory. An absolute or otherwise non-canonical
//     experimentDir is a hard error, never a silently skipped self-check.
//
// Absence and invalidity are NOT the same signal. Once an artifact is
// PRESENT, DeriveState requires it to decode, validate, and digest-link
// correctly; any failure at that point is a hard operational error, never
// a silent downgrade to a lower rung — the artifact is present, and its
// brokenness (a corrupt document, a tampered lock, a mismatched digest, a
// candidate patch whose bytes no longer match its registered digest) is
// itself the fact worth reporting. Only a MISSING next artifact selects
// the rung below it. In particular, a locked definition whose registered
// candidate patch files have gone missing is not "still registered" or
// "back to exploratory" — locking already required those patches to
// exist, so their absence is a store inconsistency and DeriveState
// reports it as an error.
//
// A PARTIALLY WRITTEN observations.jsonl is a hard error here by design at
// this rung: an incomplete observation set is not a lower rung, it is
// evidence that does not yet support any verdict. DC-15's resume behavior
// arrives in a later unit and keys on the ErrObservationIncomplete
// sentinel (observations_validation.go), which is exactly why
// incompleteness carries its own sentinel rather than being folded into
// ErrObservationIntegrity.
//
// verify is REQUIRED (SI-42): a present result.json only bears the
// recommended or inconclusive rung when verify accepts it as the closed
// engine's own output for the locked definition and complete observation
// set. A nil verify is an operational error at entry rather than a
// "checks skipped" mode, and a verify that fails is likewise an
// operational error — never a silent downgrade to measured.
func DeriveState(repoRoot, experimentDir string, verify ResultVerifier) (State, error) {
	if verify == nil {
		return "", fmt.Errorf("experiment: DeriveState requires a result verifier (a present result.json is state-bearing only when it recomputes)")
	}
	if err := ValidateRepoRelativePath(experimentDir); err != nil {
		return "", fmt.Errorf("experiment: experiment directory: %w", err)
	}
	dir := filepath.Join(repoRoot, experimentDir)

	def, ok, err := readDefinition(dir)
	if err != nil {
		return "", err
	}
	if !ok {
		return StateExploratory, nil
	}

	locked, err := Locked(def)
	if err != nil {
		return "", fmt.Errorf("experiment: %s: %w", filepath.Join(dir, definitionFile), err)
	}
	if !locked {
		return StateExploratory, nil
	}
	defDigest, err := DefinitionDigest(def)
	if err != nil {
		return "", err
	}

	if err := checkCandidatePatchesValid(dir, experimentDir, def); err != nil {
		return "", err
	}

	obs, ok, err := readObservations(dir, def)
	if err != nil {
		return "", err
	}
	if !ok {
		return StateRegistered, nil
	}

	res, ok, err := readResult(dir, defDigest, def, obs, verify)
	if err != nil {
		return "", err
	}
	if !ok {
		return StateMeasured, nil
	}

	rung := StateInconclusive
	if res.Verdict == VerdictProvenWinner {
		rung = StateRecommended
	}

	_, ok, err = readRatification(dir, res)
	if err != nil {
		return "", err
	}
	if !ok {
		return rung, nil
	}
	return StateRatified, nil
}

// readDefinition reads and decodes experiment.yaml. ok is false only when
// the file itself is absent; a present-but-undecodable file is an error.
func readDefinition(dir string) (def Definition, ok bool, err error) {
	path := filepath.Join(dir, definitionFile)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Definition{}, false, nil
	}
	if err != nil {
		return Definition{}, false, fmt.Errorf("experiment: reading %s: %w", path, err)
	}
	def, err = DecodeDefinition(raw)
	if err != nil {
		return Definition{}, false, fmt.Errorf("experiment: %s: %w", path, err)
	}
	return def, true, nil
}

// checkCandidatePatchesValid requires every candidate's registered patch
// file to exist on disk, match its registered digest, and touch no
// protected input (ValidateCandidatePatch). dir is where the files are
// READ (repoRoot joined with the experiment directory); experimentDir is
// the repo-relative path the protected-input self-check COMPARES against,
// the same coordinate system protected_paths and the evaluator executable
// use. Keeping the two apart is what lets DeriveState read an experiment
// through an absolute repoRoot without ever comparing an absolute path to
// a patch's repo-relative one.
func checkCandidatePatchesValid(dir, experimentDir string, def Definition) error {
	for _, c := range def.Candidates {
		path := filepath.Join(dir, c.Patch)
		raw, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("experiment: locked definition %q: candidate %q patch %s is missing", def.ID, c.ID, path)
			}
			return fmt.Errorf("experiment: reading %s: %w", path, err)
		}
		if err := ValidateCandidatePatch(def, c.ID, raw, experimentDir); err != nil {
			return fmt.Errorf("experiment: %s: %w", path, err)
		}
	}
	return nil
}

// readObservations reads observations.jsonl and runs the full
// ValidateObservations and ValidateComplete integrity and completeness
// checks against the locked def. ok is false only when the file itself is
// absent.
func readObservations(dir string, def Definition) (obs []Observation, ok bool, err error) {
	path := filepath.Join(dir, observationsFile)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("experiment: reading %s: %w", path, err)
	}
	obs, err = DecodeObservations(raw)
	if err != nil {
		return nil, false, fmt.Errorf("experiment: %s: %w", path, err)
	}
	if err := ValidateComplete(def, obs); err != nil {
		return nil, false, fmt.Errorf("experiment: %s: %w", path, err)
	}
	return obs, true, nil
}

// readResult reads and decodes result.json, checks that its
// definition_digest and algorithm match the locked definition, and then
// requires verify to accept it as the closed engine's own output for
// (def, obs) — SI-42's recompute-equality authority. ok is false only when
// the file itself is absent; every other failure, verification included,
// is an error.
//
// The cheap identity checks run FIRST so a result belonging to a different
// definition or algorithm is reported as exactly that, rather than as a
// recomputation mismatch that names the wrong cause.
func readResult(dir, defDigest string, def Definition, obs []Observation, verify ResultVerifier) (res Result, ok bool, err error) {
	path := filepath.Join(dir, resultFile)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, fmt.Errorf("experiment: reading %s: %w", path, err)
	}
	res, err = DecodeResult(raw)
	if err != nil {
		return Result{}, false, fmt.Errorf("experiment: %s: %w", path, err)
	}
	if res.DefinitionDigest != defDigest {
		return Result{}, false, fmt.Errorf("experiment: %s: definition_digest %q does not match the locked definition digest %q", path, res.DefinitionDigest, defDigest)
	}
	if res.Algorithm != AlgorithmV1 {
		return Result{}, false, fmt.Errorf("experiment: %s: algorithm %q does not match the registered algorithm %q", path, res.Algorithm, AlgorithmV1)
	}
	if err := verify(def, obs, res); err != nil {
		return Result{}, false, fmt.Errorf("experiment: %s: result does not verify against the locked definition and observations: %w", path, err)
	}
	return res, true, nil
}

// readRatification reads and decodes ratification.yaml, and checks that
// its result_digest equals ResultDigest(res). ok is false only when the
// file itself is absent.
func readRatification(dir string, res Result) (r Ratification, ok bool, err error) {
	path := filepath.Join(dir, ratificationFile)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Ratification{}, false, nil
	}
	if err != nil {
		return Ratification{}, false, fmt.Errorf("experiment: reading %s: %w", path, err)
	}
	r, err = DecodeRatification(raw)
	if err != nil {
		return Ratification{}, false, fmt.Errorf("experiment: %s: %w", path, err)
	}
	want, err := ResultDigest(res)
	if err != nil {
		return Ratification{}, false, err
	}
	if r.ResultDigest != want {
		return Ratification{}, false, fmt.Errorf("experiment: %s: result_digest %q does not match the result's own digest %q", path, r.ResultDigest, want)
	}
	return r, true, nil
}
