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

// DeriveState computes an experiment's derived lifecycle state from the
// presence and validity of its artifacts under dir (AC-1's state table,
// DC-2): exploratory, registered, measured, recommended, inconclusive, or
// ratified. States form a ladder — reaching a state requires every lower
// state's requirement to still hold — and DeriveState walks the ladder in
// order, stopping at the first artifact that is genuinely ABSENT.
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
func DeriveState(dir string) (State, error) {
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

	if err := checkCandidatePatchesExist(dir, def); err != nil {
		return "", err
	}

	// Commit 3 strengthens this rung with ValidateObservations/
	// ValidateComplete; commit 2 checks presence, decode, and per-record
	// envelope digest linkage only (readObservations).
	_, ok, err = readObservations(dir, defDigest)
	if err != nil {
		return "", err
	}
	if !ok {
		return StateRegistered, nil
	}

	res, ok, err := readResult(dir, defDigest)
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

// checkCandidatePatchesExist requires every candidate's registered patch
// path to exist on disk (commit 2's "complete" test for the registered
// rung: existence only). Commit 3 strengthens this to full digest and
// protected-path verification via ValidateCandidatePatch.
func checkCandidatePatchesExist(dir string, def Definition) error {
	for _, c := range def.Candidates {
		path := filepath.Join(dir, c.Patch)
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("experiment: locked definition %q: candidate %q patch %s is missing", def.ID, c.ID, path)
			}
			return fmt.Errorf("experiment: statting %s: %w", path, err)
		}
	}
	return nil
}

// readObservations reads and decodes observations.jsonl, and checks the
// commit-2 "envelope digest match" rule: every record's experiment_digest
// equals defDigest. ok is false only when the file itself is absent.
// Commit 3 strengthens this rung with the full ValidateObservations and
// ValidateComplete cross-record integrity checks.
func readObservations(dir, defDigest string) (obs []Observation, ok bool, err error) {
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
	for i, o := range obs {
		if o.ExperimentDigest != defDigest {
			return nil, false, fmt.Errorf("experiment: %s: record %d: experiment_digest %q does not match the locked definition digest %q", path, i, o.ExperimentDigest, defDigest)
		}
	}
	return obs, true, nil
}

// readResult reads and decodes result.json, and checks that its
// definition_digest and algorithm match the locked definition. ok is
// false only when the file itself is absent.
func readResult(dir, defDigest string) (res Result, ok bool, err error) {
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
