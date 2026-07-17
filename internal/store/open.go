package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config is a store's resolved configuration: its root directory and
// strict-decoded manifest. Open is the store package's config bottleneck
// (L-M3, docs/design/plans/2026-07-17-extensibility-phase1-plan.md Task
// 3): the single place verbs load verdi.yaml, replacing
// cmd/verdi/forgeboot.go's former loadManifest body verbatim.
//
// A Model *model.Model field is planned (Task 6) but deliberately omitted
// here: internal/model does not exist yet as of this task, so adding the
// field now would either be a stub nobody consumes or force a premature
// dependency. Smallest reversible slice — the later task adds it.
type Config struct {
	Root     string
	Manifest *Manifest
}

// Open reads and strict-decodes root's verdi.yaml, returning the
// resolved Config. Error wrapping is unchanged from the pre-move
// loadManifest (cmd/verdi/forgeboot.go, now a thin delegate to Open):
// "reading verdi.yaml: %w" when the file itself cannot be read (e.g.
// missing), "decoding verdi.yaml: %w" when DecodeManifest rejects its
// contents (YAML syntax, strict-decode, or Validate failures) — behavior-
// preserving, so the ~10 existing loadManifest callers see byte-identical
// errors.
func Open(root string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "verdi.yaml"))
	if err != nil {
		return nil, fmt.Errorf("reading verdi.yaml: %w", err)
	}
	m, err := DecodeManifest(data)
	if err != nil {
		return nil, fmt.Errorf("decoding verdi.yaml: %w", err)
	}
	return &Config{Root: root, Manifest: m}, nil
}
