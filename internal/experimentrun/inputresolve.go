package experimentrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/experiment"
)

const environmentRootName = ".verdi-cse-environment"

// InputResolver resolves a locked artifact reference to the exact protected
// base-tree file the evaluator receives. It is read-only by contract.
type InputResolver interface {
	ResolveExperimentInput(ctx context.Context, root string, request ResolveInputRequest) (ResolvedInput, error)
}

// ResolveInputRequest binds one resolver request to its exact closed slot.
type ResolveInputRequest struct {
	Slot InputSlot
	Ref  experiment.ArtifactRef
}

// Validate checks both the closed slot and locked artifact identity.
func (r ResolveInputRequest) Validate() error {
	if err := r.Slot.Validate(); err != nil {
		return err
	}
	if err := r.Ref.Validate("input resolver request"); err != nil {
		return err
	}
	if strings.HasPrefix(string(r.Slot), fixtureSlotPrefix) && r.Slot != FixtureInputSlot(r.Ref.ID) {
		return fmt.Errorf("experimentrun: fixture input slot %q does not match requested id %q", r.Slot, r.Ref.ID)
	}
	return nil
}

// ResolvedInput is an InputResolver's proposed locked artifact identity.
type ResolvedInput struct {
	ID     string
	Path   string
	Digest string
}

// ResolvedInputs is the complete evaluator input set in locked definition
// order. It contains no file bytes and no mutable state shared with a resolver.
type ResolvedInputs struct {
	Workload experiment.ResolvedArtifact
	Fixtures []experiment.ResolvedArtifact
	Contract experiment.ResolvedArtifact
}

// ResolveInputs resolves and proves every workload, fixture, and contract
// reference below root. Missing, changed, duplicate, extra, or unprotected
// inputs are operational errors.
func ResolveInputs(ctx context.Context, resolver InputResolver, root string, def experiment.Definition) (ResolvedInputs, error) {
	if ctx == nil {
		return ResolvedInputs{}, fmt.Errorf("experimentrun: resolve inputs: nil context")
	}
	if resolver == nil {
		return ResolvedInputs{}, fmt.Errorf("experimentrun: resolve inputs: resolver is nil")
	}
	if err := def.Validate(); err != nil {
		return ResolvedInputs{}, fmt.Errorf("experimentrun: resolve inputs definition: %w", err)
	}
	if err := validateExactRoot(root); err != nil {
		return ResolvedInputs{}, fmt.Errorf("experimentrun: resolve inputs root: %w", err)
	}
	protected := make(map[string]bool, len(def.ProtectedPaths))
	for _, path := range def.ProtectedPaths {
		protected[path] = true
	}
	seenPaths := map[string]bool{}
	resolve := func(slot InputSlot, field string, ref experiment.ArtifactRef) (experiment.ResolvedArtifact, error) {
		request := ResolveInputRequest{Slot: slot, Ref: ref}
		if err := request.Validate(); err != nil {
			return experiment.ResolvedArtifact{}, fmt.Errorf("%s: %w", field, err)
		}
		resolved, err := resolver.ResolveExperimentInput(ctx, root, request)
		if err != nil {
			return experiment.ResolvedArtifact{}, fmt.Errorf("%s: %w", field, err)
		}
		if resolved.ID != ref.ID || resolved.Digest != ref.Digest {
			return experiment.ResolvedArtifact{}, fmt.Errorf("%s: resolver identity {%q,%q}, want {%q,%q}", field, resolved.ID, resolved.Digest, ref.ID, ref.Digest)
		}
		artifact := experiment.ResolvedArtifact{ID: resolved.ID, Path: resolved.Path, Digest: resolved.Digest}
		if err := artifact.Validate(field); err != nil {
			return experiment.ResolvedArtifact{}, err
		}
		if !protected[artifact.Path] {
			return experiment.ResolvedArtifact{}, fmt.Errorf("%s: resolved path %q is absent from definition protected_paths", field, artifact.Path)
		}
		if seenPaths[artifact.Path] {
			return experiment.ResolvedArtifact{}, fmt.Errorf("%s: resolved path %q duplicates another locked input", field, artifact.Path)
		}
		if err := verifyRegularInput(root, artifact.Path, artifact.Digest); err != nil {
			return experiment.ResolvedArtifact{}, fmt.Errorf("%s: %w", field, err)
		}
		seenPaths[artifact.Path] = true
		return artifact, nil
	}
	workload, err := resolve(InputSlotWorkload, "workload", def.Workload)
	if err != nil {
		return ResolvedInputs{}, err
	}
	fixtures := make([]experiment.ResolvedArtifact, len(def.Fixtures))
	for i, fixture := range def.Fixtures {
		fixtures[i], err = resolve(FixtureInputSlot(fixture.ID), fmt.Sprintf("fixtures[%d]", i), fixture)
		if err != nil {
			return ResolvedInputs{}, err
		}
	}
	contract, err := resolve(InputSlotContract, "contract", def.Contract)
	if err != nil {
		return ResolvedInputs{}, err
	}
	return ResolvedInputs{Workload: workload, Fixtures: fixtures, Contract: contract}, nil
}

// PreflightEnvironmentRoot returns the reserved candidate-local profile root
// only when it is absent. Any pre-existing path is a collision and no path is
// created by this check.
func PreflightEnvironmentRoot(workspaceRoot string) (string, error) {
	if err := validateExactRoot(workspaceRoot); err != nil {
		return "", fmt.Errorf("experimentrun: preflight environment root: %w", err)
	}
	path := filepath.Join(workspaceRoot, environmentRootName)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return path, nil
	}
	if err != nil {
		return "", fmt.Errorf("experimentrun: preflight environment root %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("experimentrun: reserved environment root %q is a symlink", path)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("experimentrun: reserved environment root %q is not a directory", path)
	}
	entries, readErr := os.ReadDir(path)
	if readErr != nil {
		return "", fmt.Errorf("experimentrun: read reserved environment root %q: %w", path, readErr)
	}
	if len(entries) != 0 {
		return "", fmt.Errorf("experimentrun: reserved environment root %q is nonempty", path)
	}
	return "", fmt.Errorf("experimentrun: reserved environment root %q already exists", path)
}

func validateExactRoot(root string) error {
	if root == "" || !filepath.IsAbs(root) {
		return fmt.Errorf("root %q must be absolute", root)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("lstat root %q: %w", root, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("root %q must be a non-symlink directory", root)
	}
	return nil
}

func verifyRegularInput(root, path, digest string) error {
	current := root
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("lstat %q: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("input %q traverses a symlink", path)
		}
		if i < len(segments)-1 && !info.IsDir() {
			return fmt.Errorf("input %q parent is not a directory", path)
		}
		if i == len(segments)-1 && !info.Mode().IsRegular() {
			return fmt.Errorf("input %q is not a regular file", path)
		}
	}
	file, err := os.Open(current)
	if err != nil {
		return fmt.Errorf("open input %q: %w", path, err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("hash input %q: %w", path, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("finalize input %q: %w", path, closeErr)
	}
	got := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	if got != digest {
		return fmt.Errorf("input %q raw-byte digest %q does not match registered %q", path, got, digest)
	}
	return nil
}

func validateResolvedInputs(def experiment.Definition, inputs ResolvedInputs) error {
	protected := make(map[string]bool, len(def.ProtectedPaths))
	for _, path := range def.ProtectedPaths {
		protected[path] = true
	}
	seen := map[string]bool{}
	validate := func(field string, ref experiment.ArtifactRef, value experiment.ResolvedArtifact) error {
		if value.ID != ref.ID || value.Digest != ref.Digest {
			return fmt.Errorf("%s identity does not match locked reference", field)
		}
		if err := value.Validate(field); err != nil {
			return err
		}
		if !protected[value.Path] {
			return fmt.Errorf("%s path %q is absent from definition protected_paths", field, value.Path)
		}
		if seen[value.Path] {
			return fmt.Errorf("%s path %q duplicates another locked input", field, value.Path)
		}
		seen[value.Path] = true
		return nil
	}
	if err := validate("workload", def.Workload, inputs.Workload); err != nil {
		return err
	}
	if len(inputs.Fixtures) != len(def.Fixtures) {
		return fmt.Errorf("fixture count %d, want %d", len(inputs.Fixtures), len(def.Fixtures))
	}
	for i, ref := range def.Fixtures {
		if err := validate(fmt.Sprintf("fixtures[%d]", i), ref, inputs.Fixtures[i]); err != nil {
			return err
		}
	}
	return validate("contract", def.Contract, inputs.Contract)
}
