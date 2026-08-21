package experiment

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/jyang234/verdi/internal/canonjson"
)

type RunPaths struct{ Directory, Execution, Observations, Result string }

func PathsForRun(experimentDir, run string) (RunPaths, error) {
	if err := ValidateRepoRelativePath(experimentDir); err != nil {
		return RunPaths{}, err
	}
	if err := ValidateID(run); err != nil {
		return RunPaths{}, err
	}
	dir := filepath.ToSlash(filepath.Join(experimentDir, "runs", run))
	return RunPaths{Directory: dir, Execution: dir + "/execution.json", Observations: dir + "/observations.jsonl", Result: dir + "/result.json"}, nil
}

func WorkspaceRunID(experimentDigest, run, candidate string) (string, error) {
	if err := ValidateDigest(experimentDigest); err != nil {
		return "", err
	}
	if err := ValidateID(run); err != nil {
		return "", err
	}
	if err := ValidateID(candidate); err != nil {
		return "", err
	}
	b, err := canonjson.Marshal(struct {
		ExperimentDigest string `json:"experiment_digest"`
		Run              string `json:"run"`
		Candidate        string `json:"candidate"`
	}{experimentDigest, run, candidate})
	if err != nil {
		return "", fmt.Errorf("experiment: workspace run id: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
