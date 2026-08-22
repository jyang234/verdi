package experimentrun

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/store"
)

// runStorage owns the receipt and observation publication operations for one
// caller-supplied CSE run. It uses the checkout-wide writer lock; it neither
// interprets decision state nor publishes a result.
type runStorage struct {
	root             string
	experimentDir    string
	run              string
	capabilitiesPath string
	candidateDir     string
	executionPath    string
	observationsPath string
	writerLockPath   string
}

func newRunStorage(root, experimentDir, run string) (runStorage, error) {
	if err := validateStorageRoot(root); err != nil {
		return runStorage{}, fmt.Errorf("experimentrun: storage root: %w", err)
	}
	paths, err := experiment.PathsForRun(experimentDir, run)
	if err != nil {
		return runStorage{}, fmt.Errorf("experimentrun: run paths: %w", err)
	}
	experimentPath := filepath.Join(root, filepath.FromSlash(experimentDir))
	return runStorage{
		root:             root,
		experimentDir:    experimentDir,
		run:              run,
		capabilitiesPath: filepath.Join(experimentPath, "evaluator-capabilities.json"),
		candidateDir:     filepath.Join(experimentPath, "candidates"),
		executionPath:    filepath.Join(root, filepath.FromSlash(paths.Execution)),
		observationsPath: filepath.Join(root, filepath.FromSlash(paths.Observations)),
		writerLockPath:   store.WriterLockPath(root),
	}, nil
}

func (s runStorage) preflightStart() error {
	for _, path := range []string{s.executionPath, s.observationsPath, s.writerLockPath} {
		if err := validateStoreFilePath(s.root, path); err != nil {
			return err
		}
	}
	if _, err := readRegularFile(s.root, s.capabilitiesPath); err != nil {
		return fmt.Errorf("read evaluator capabilities: %w", err)
	}
	if _, err := os.Lstat(s.observationsPath); err == nil {
		return fmt.Errorf("observations file %q already exists; start does not resume a run", s.observationsPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("lstat observations file %q: %w", s.observationsPath, err)
	}
	return nil
}

func (s runStorage) loadCapabilities() (experiment.Capabilities, []byte, error) {
	data, err := readRegularFile(s.root, s.capabilitiesPath)
	if err != nil {
		return experiment.Capabilities{}, nil, fmt.Errorf("read evaluator capabilities: %w", err)
	}
	capabilities, err := experiment.DecodeCapabilities(data)
	if err != nil {
		return experiment.Capabilities{}, nil, fmt.Errorf("decode evaluator capabilities: %w", err)
	}
	if capabilities.Schema != experiment.CapabilitiesSchemaV2 {
		return experiment.Capabilities{}, nil, fmt.Errorf("evaluator capabilities schema %q is not %q", capabilities.Schema, experiment.CapabilitiesSchemaV2)
	}
	return capabilities, data, nil
}

func (s runStorage) loadCandidatePatches(def experiment.Definition) (map[string][]byte, error) {
	patches := make(map[string][]byte, len(def.Candidates))
	for _, candidate := range def.Candidates {
		path := filepath.Join(s.candidateDir, candidate.ID+".patch")
		data, err := readRegularFile(s.root, path)
		if err != nil {
			return nil, fmt.Errorf("read candidate %q patch: %w", candidate.ID, err)
		}
		if err := experiment.ValidateCandidatePatch(def, candidate.ID, data, s.experimentDir); err != nil {
			return nil, fmt.Errorf("validate candidate %q patch: %w", candidate.ID, err)
		}
		patches[candidate.ID] = data
	}
	return patches, nil
}

func (s runStorage) createReceipt(receipt experiment.ExecutionReceipt) error {
	data, err := experiment.EncodeExecutionReceipt(receipt)
	if err != nil {
		return fmt.Errorf("encode execution receipt: %w", err)
	}
	return s.withWriterLock(func() error {
		if err := validateStoreFilePath(s.root, s.executionPath); err != nil {
			return err
		}
		if err := ensureParentTree(s.root, filepath.Dir(s.executionPath)); err != nil {
			return err
		}
		created, existing, err := atomicfile.CreateImmutable(s.executionPath, data, 0o600)
		if err != nil {
			return fmt.Errorf("publish execution receipt: %w", err)
		}
		if !created {
			if bytes.Equal(existing, data) {
				return fmt.Errorf("execution receipt %q already exists; start does not resume a run", s.executionPath)
			}
			return fmt.Errorf("execution receipt %q conflicts with the current run identity", s.executionPath)
		}
		return nil
	})
}

func (s runStorage) appendObservation(def experiment.Definition, schedule []ScheduledAttempt, observation experiment.Observation) error {
	line, err := experiment.EncodeObservation(observation)
	if err != nil {
		return fmt.Errorf("encode observation: %w", err)
	}
	return s.withWriterLock(func() error {
		if err := validateStoreFilePath(s.root, s.observationsPath); err != nil {
			return err
		}
		if err := ensureParentTree(s.root, filepath.Dir(s.observationsPath)); err != nil {
			return err
		}
		existing, err := readOptionalRegularFile(s.root, s.observationsPath)
		if err != nil {
			return fmt.Errorf("read observations: %w", err)
		}
		observations, err := experiment.DecodeObservations(existing)
		if err != nil {
			return fmt.Errorf("decode observations: %w", err)
		}
		if len(existing) > 0 && len(observations) == 0 {
			return fmt.Errorf("decode observations: nonempty file contains zero records")
		}
		if err := validateMeasuredPrefix(def, schedule, s.run, observations); err != nil {
			return err
		}
		if err := validateNextObservation(def, schedule, s.run, observations, observation); err != nil {
			return err
		}
		data := append(append([]byte(nil), existing...), line...)
		if err := atomicfile.Write(s.observationsPath, data, 0o600); err != nil {
			return fmt.Errorf("publish observations: %w", err)
		}
		return nil
	})
}

func (s runStorage) withWriterLock(operation func() error) (err error) {
	if err := validateStoreFilePath(s.root, s.writerLockPath); err != nil {
		return err
	}
	if err := ensureParentTree(s.root, filepath.Dir(s.writerLockPath)); err != nil {
		return err
	}
	lock, err := filelock.Acquire(s.writerLockPath)
	if err != nil {
		return fmt.Errorf("acquire checkout writer lock: %w", err)
	}
	defer func() {
		releaseErr := filelock.Release(lock, s.writerLockPath)
		if releaseErr == nil {
			return
		}
		if err == nil {
			err = fmt.Errorf("release checkout writer lock: %w", releaseErr)
			return
		}
		err = errors.Join(err, fmt.Errorf("release checkout writer lock: %w", releaseErr))
	}()
	return operation()
}

func validateMeasuredPrefix(def experiment.Definition, schedule []ScheduledAttempt, run string, observations []experiment.Observation) error {
	measured := measuredSchedule(schedule)
	if len(observations) > len(measured) {
		return fmt.Errorf("observations contain %d records, exceeding measured schedule length %d", len(observations), len(measured))
	}
	if len(observations) == 0 {
		return nil
	}
	if err := experiment.ValidateObservations(def, observations); err != nil {
		return fmt.Errorf("validate observations: %w", err)
	}
	for i, observation := range observations {
		want := measured[i]
		if observation.Run != run {
			return fmt.Errorf("observations record %d run %q does not match durable run %q", i, observation.Run, run)
		}
		if observation.Schema != experiment.ObservationSchemaV2 || observation.Candidate != want.Candidate || observation.Round != want.Cycle.Number {
			return fmt.Errorf("observations record %d is not measured schedule prefix entry %s@%d", i, want.Candidate, want.Cycle.Number)
		}
	}
	return nil
}

func validateNextObservation(def experiment.Definition, schedule []ScheduledAttempt, run string, observations []experiment.Observation, observation experiment.Observation) error {
	measured := measuredSchedule(schedule)
	if len(observations) >= len(measured) {
		return fmt.Errorf("measured schedule is already complete")
	}
	if err := experiment.ValidateObservations(def, append(append([]experiment.Observation(nil), observations...), observation)); err != nil {
		return fmt.Errorf("validate next observation: %w", err)
	}
	if observation.Run != run {
		return fmt.Errorf("next observation run %q does not match durable run %q", observation.Run, run)
	}
	want := measured[len(observations)]
	if observation.Schema != experiment.ObservationSchemaV2 || observation.Candidate != want.Candidate || observation.Round != want.Cycle.Number {
		return fmt.Errorf("next observation is %s@%d, want measured schedule entry %s@%d", observation.Candidate, observation.Round, want.Candidate, want.Cycle.Number)
	}
	return nil
}

func measuredSchedule(schedule []ScheduledAttempt) []ScheduledAttempt {
	measured := make([]ScheduledAttempt, 0, len(schedule))
	for _, attempt := range schedule {
		if attempt.Cycle.Kind == experiment.CycleMeasured {
			measured = append(measured, attempt)
		}
	}
	return measured
}

func validateStorageRoot(root string) error {
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

func validateStoreFilePath(root, path string) error {
	if err := validateStorageRoot(root); err != nil {
		return err
	}
	if err := validateParentTree(root, filepath.Dir(path)); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lstat %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("path %q must be absent or a non-symlink regular file", path)
	}
	return nil
}

func validateParentTree(root, parent string) error {
	relative, err := filepath.Rel(root, parent)
	if err != nil || relative == ".." || filepath.IsAbs(relative) || stringsHasParent(relative) {
		return fmt.Errorf("path %q is outside root %q", parent, root)
	}
	current := root
	if relative == "." {
		return nil
	}
	for _, segment := range stringsSplitPath(relative) {
		current = filepath.Join(current, segment)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		if statErr != nil {
			return fmt.Errorf("lstat parent %q: %w", current, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("parent %q must be a non-symlink directory", current)
		}
	}
	return nil
}

// ensureParentTree creates only missing, validated directory components below
// root. It deliberately does not call MkdirAll so an existing symlink or
// non-directory component is rejected instead of traversed on the way to a
// writer-lock or durable-run artifact.
func ensureParentTree(root, parent string) error {
	if err := validateStorageRoot(root); err != nil {
		return err
	}
	relative, err := filepath.Rel(root, parent)
	if err != nil || relative == ".." || filepath.IsAbs(relative) || stringsHasParent(relative) {
		return fmt.Errorf("path %q is outside root %q", parent, root)
	}
	current := root
	for _, segment := range stringsSplitPath(relative) {
		current = filepath.Join(current, segment)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			if mkdirErr := os.Mkdir(current, 0o755); mkdirErr != nil && !errors.Is(mkdirErr, os.ErrExist) {
				return fmt.Errorf("create parent %q: %w", current, mkdirErr)
			}
			info, statErr = os.Lstat(current)
		}
		if statErr != nil {
			return fmt.Errorf("lstat parent %q: %w", current, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("parent %q must be a non-symlink directory", current)
		}
	}
	return nil
}

func readRegularFile(root, path string) ([]byte, error) {
	if err := validateStoreFilePath(root, path); err != nil {
		return nil, err
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil, os.ErrNotExist
	} else if err != nil {
		return nil, fmt.Errorf("lstat %q: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return data, nil
}

func readOptionalRegularFile(root, path string) ([]byte, error) {
	data, err := readRegularFile(root, path)
	if errors.Is(err, os.ErrNotExist) {
		return []byte{}, nil
	}
	return data, err
}

func stringsHasParent(path string) bool {
	for _, segment := range stringsSplitPath(path) {
		if segment == ".." {
			return true
		}
	}
	return false
}

func stringsSplitPath(path string) []string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" && part != "." {
			result = append(result, part)
		}
	}
	return result
}
