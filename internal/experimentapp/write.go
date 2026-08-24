package experimentapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/experiment"
)

const (
	stepProposalArtifactsInstalled = "experiment-proposal-artifacts-installed"
)

type proposalFile struct {
	path      string
	old       []byte
	oldExists bool
	new       []byte
}

// writeProposal serializes exact proposal artifacts and their complete
// provenance append under the checkout-wide writer lock. Artifacts are
// installed before provenance so an interruption is a visibly dirty proposal,
// never an authoritative pair. There is deliberately no second journal.
func writeProposal(ctx context.Context, root string, coordinator draftmutation.Coordinator, artifacts []proposalFile, provenance proposalFile) error {
	files := append([]proposalFile(nil), artifacts...)
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	return draftmutation.WithWriterLock(ctx, root, coordinator, func(_ *draftmutation.LockedWriter) error {
		for _, file := range append(append([]proposalFile(nil), files...), provenance) {
			if err := validateProposalFile(root, file); err != nil {
				return err
			}
		}
		for _, file := range append(append([]proposalFile(nil), files...), provenance) {
			absolute, err := proposalAbsolutePath(root, file.path)
			if err != nil {
				return err
			}
			if err := ensureProposalDirectory(root, filepath.Dir(absolute)); err != nil {
				return err
			}
		}
		for _, file := range files {
			if err := replaceProposalFile(ctx, root, file.path, file.new); err != nil {
				return err
			}
		}
		if coordinator.After != nil {
			if err := coordinator.After(stepProposalArtifactsInstalled); err != nil {
				return fmt.Errorf("experimentapp: after %s: %w", stepProposalArtifactsInstalled, err)
			}
		}
		if err := replaceProposalFile(ctx, root, provenance.path, provenance.new); err != nil {
			return err
		}
		return nil
	})
}

func ensureProposalDirectory(root, directory string) error {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(absoluteRoot, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("experimentapp: proposal directory escapes checkout root")
	}
	current := absoluteRoot
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "." || component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			if err := os.Mkdir(current, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("experimentapp: create proposal directory %q: %w", current, err)
			}
			if err := syncProposalDirectory(filepath.Dir(current)); err != nil {
				return err
			}
			info, statErr = os.Lstat(current)
		}
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("experimentapp: proposal directory %q is missing, a symlink, or not a directory", current)
		}
	}
	return nil
}

func syncProposalDirectory(name string) error {
	directory, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("experimentapp: open proposal directory %q: %w", name, err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return fmt.Errorf("experimentapp: sync proposal directory %q: %w", name, syncErr)
	}
	return closeErr
}

func validateProposalFile(root string, file proposalFile) error {
	if err := experiment.ValidateRepoRelativePath(file.path); err != nil {
		return fmt.Errorf("experimentapp: proposal path: %w", err)
	}
	absolute, err := proposalAbsolutePath(root, file.path)
	if err != nil {
		return err
	}
	current, exists, err := readOptionalProposalFile(absolute)
	if err != nil {
		return err
	}
	if exists != file.oldExists || !bytes.Equal(current, file.old) {
		return fmt.Errorf("experimentapp: proposal file %q changed before writer-lock commit", file.path)
	}
	return nil
}

func replaceProposalFile(ctx context.Context, root, repoPath string, data []byte) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	absolute, err := proposalAbsolutePath(root, repoPath)
	if err != nil {
		return err
	}
	if err := validateProposalDirectory(root, filepath.Dir(absolute)); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(absolute), ".verdi-experiment-write-")
	if err != nil {
		return fmt.Errorf("experimentapp: create proposal stage for %q: %w", repoPath, err)
	}
	temporaryName := temporary.Name()
	temporaryOpen := true
	defer func() {
		if temporaryOpen {
			if closeErr := temporary.Close(); closeErr != nil && resultErr == nil {
				resultErr = closeErr
			}
		}
		if removeErr := os.Remove(temporaryName); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) && resultErr == nil {
			resultErr = removeErr
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("experimentapp: write proposal stage for %q: %w", repoPath, err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("experimentapp: sync proposal stage for %q: %w", repoPath, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("experimentapp: finalize proposal stage for %q: %w", repoPath, err)
	}
	temporaryOpen = false
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, absolute); err != nil {
		return fmt.Errorf("experimentapp: install proposal %q: %w", repoPath, err)
	}
	if err := syncProposalDirectory(filepath.Dir(absolute)); err != nil {
		return fmt.Errorf("experimentapp: sync proposal directory for %q: %w", repoPath, err)
	}
	return nil
}

func proposalAbsolutePath(root, repoPath string) (string, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absolute := filepath.Join(absoluteRoot, filepath.FromSlash(repoPath))
	relative, err := filepath.Rel(absoluteRoot, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("experimentapp: proposal path %q escapes checkout root", repoPath)
	}
	return absolute, nil
}

func validateProposalDirectory(root, directory string) error {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(absoluteRoot, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("experimentapp: proposal directory escapes checkout root")
	}
	current := absoluteRoot
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "." || component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("experimentapp: proposal directory %q is missing, a symlink, or not a directory", current)
		}
	}
	return nil
}

func readOptionalProposalFile(name string) ([]byte, bool, error) {
	info, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return []byte{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("experimentapp: proposal destination %q is a symlink or not a regular file", name)
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, false, err
	}
	data, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return nil, true, readErr
	}
	return data, true, closeErr
}
