package gitx

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type commandRunner func(context.Context, string, ...string) ([]byte, error)

// FastForwardCategory is the closed outcome vocabulary for the guarded merge.
type FastForwardCategory string

const (
	FastForwardSucceeded    FastForwardCategory = "succeeded"
	FastForwardInvalidInput FastForwardCategory = "invalid-input"
	FastForwardStatusFailed FastForwardCategory = "status-failed"
	FastForwardRunwayDirty  FastForwardCategory = "runway-dirty"
	FastForwardMergeFailed  FastForwardCategory = "merge-failed"
)

// FastForwardResult states both the closed outcome and whether
// `git merge --ff-only` was actually invoked.
type FastForwardResult struct {
	Category  FastForwardCategory
	Attempted bool
}

// FastForwardOnly advances a clean runway with exactly
// `git merge --ff-only <outputCommit>`. It never attempts recovery.
func FastForwardOnly(ctx context.Context, runway, outputCommit string) (FastForwardResult, error) {
	return fastForwardOnly(ctx, runway, outputCommit, run)
}

func fastForwardOnly(ctx context.Context, runway, outputCommit string, runner commandRunner) (FastForwardResult, error) {
	if ctx == nil {
		return FastForwardResult{Category: FastForwardInvalidInput}, errors.New("gitx: FastForwardOnly: nil context")
	}
	if runner == nil {
		return FastForwardResult{Category: FastForwardInvalidInput}, errors.New("gitx: FastForwardOnly: nil git runner")
	}
	if runway == "" || strings.TrimSpace(runway) != runway || filepath.Clean(runway) != runway {
		return FastForwardResult{Category: FastForwardInvalidInput}, fmt.Errorf("gitx: FastForwardOnly: runway path %q is not clean", runway)
	}
	if err := validateFullGitOID(outputCommit); err != nil {
		return FastForwardResult{Category: FastForwardInvalidInput}, fmt.Errorf("gitx: FastForwardOnly: output commit: %w", err)
	}
	status, err := runner(ctx, runway, "status", "--porcelain")
	if err != nil {
		return FastForwardResult{Category: FastForwardStatusFailed}, fmt.Errorf("gitx: FastForwardOnly: inspect runway status: %w", err)
	}
	if len(strings.TrimSpace(string(status))) != 0 {
		return FastForwardResult{Category: FastForwardRunwayDirty}, errors.New("gitx: FastForwardOnly: runway is dirty")
	}
	if _, err := runner(ctx, runway, "merge", "--ff-only", outputCommit); err != nil {
		return FastForwardResult{Category: FastForwardMergeFailed, Attempted: true}, fmt.Errorf("gitx: FastForwardOnly(%q): %w", outputCommit, err)
	}
	return FastForwardResult{Category: FastForwardSucceeded, Attempted: true}, nil
}

func validateFullGitOID(value string) error {
	if len(value) != 40 && len(value) != 64 {
		return errors.New("must be a full 40- or 64-character object id")
	}
	if value != strings.ToLower(value) {
		return errors.New("must be lowercase hexadecimal")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return fmt.Errorf("must be hexadecimal: %w", err)
	}
	return nil
}
