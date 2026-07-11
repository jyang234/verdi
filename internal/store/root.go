package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// manifestRelPath is verdi.yaml's path relative to the store root
// (01 §Directory layout).
const manifestRelPath = ".verdi/verdi.yaml"

// FindRoot walks up from startDir (inclusive) to the nearest ancestor
// directory containing .verdi/verdi.yaml (A6/I-16: "the binary operates on
// the nearest ancestor directory containing .verdi/verdi.yaml"). It
// returns the absolute path to that ancestor — the store root, i.e. the
// directory whose child is .verdi/ — not .verdi itself. startDir must
// exist; an empty startDir is an error rather than silently defaulting to
// the process's cwd, so callers state their intent explicitly.
func FindRoot(startDir string) (string, error) {
	if startDir == "" {
		return "", fmt.Errorf("store: FindRoot: startDir must not be empty")
	}
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("store: FindRoot(%q): %w", startDir, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("store: FindRoot(%q): %w", startDir, err)
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}

	dir := abs
	for {
		if manifestExists(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("store: FindRoot(%q): no ancestor directory contains %s", startDir, manifestRelPath)
		}
		dir = parent
	}
}

// RootAt validates an explicit store-root override (e.g. a --store flag):
// root itself must contain .verdi/verdi.yaml directly, with no ancestor
// walk. It returns root's absolute, cleaned path.
func RootAt(root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("store: RootAt: root must not be empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("store: RootAt(%q): %w", root, err)
	}
	if !manifestExists(abs) {
		return "", fmt.Errorf("store: RootAt(%q): %s does not exist at this path (explicit --store override, no ancestor search)", root, manifestRelPath)
	}
	return abs, nil
}

// manifestExists reports whether dir/.verdi/verdi.yaml exists and is a
// regular file.
func manifestExists(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, manifestRelPath))
	return err == nil && !info.IsDir()
}
