package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/model"
)

// Config is a store's resolved configuration: its root directory,
// strict-decoded manifest, and resolved operating model. Open is the
// store package's config bottleneck (L-M3, docs/design/plans/2026-07-17-
// extensibility-phase1-plan.md Task 3): the single place verbs load
// verdi.yaml (and, as of Task 6, model.yaml), replacing
// cmd/verdi/forgeboot.go's former loadManifest body verbatim.
//
// Model is always non-nil after a successful Open: an absent
// .verdi/model.yaml resolves to model.Canonical() (the embedded
// default), never a nil pointer or a bare zero value — mirroring this
// same file's own Manifest posture (a store always has SOME resolved
// config) and the load-bearing parity claim spec/model-schema's outcome
// depends on: a store with no model.yaml at all changes nothing about
// how it behaves today.
type Config struct {
	Root     string
	Manifest *Manifest
	Model    *model.Model
}

// Open reads and strict-decodes root's verdi.yaml and (Task 6)
// resolves its model.yaml, returning the resolved Config. verdi.yaml's
// error wrapping is unchanged from the pre-move loadManifest
// (cmd/verdi/forgeboot.go, now a thin delegate to Open): "reading
// verdi.yaml: %w" when the file itself cannot be read (e.g. missing),
// "decoding verdi.yaml: %w" when DecodeManifest rejects its contents
// (YAML syntax, strict-decode, or Validate failures) — behavior-
// preserving, so the ~10 existing loadManifest callers see byte-
// identical errors. Model resolution runs only once verdi.yaml itself
// is valid (see loadModel below for its own error wrapping, which is
// NOT the same shape as this — K5).
func Open(root string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "verdi.yaml"))
	if err != nil {
		return nil, fmt.Errorf("reading verdi.yaml: %w", err)
	}
	m, err := DecodeManifest(data)
	if err != nil {
		return nil, fmt.Errorf("decoding verdi.yaml: %w", err)
	}

	mdl, err := loadModel(root)
	if err != nil {
		return nil, err
	}

	return &Config{Root: root, Manifest: m, Model: mdl}, nil
}

// loadModel resolves root's .verdi/model.yaml: strict-decoded (via
// model.DecodeModel — kernel validation and the stage-1 frontier both
// apply) when present, else the embedded canonical default
// (model.Canonical(), internal/model/embed.go) when absent.
//
// Error wrapping is asymmetric on purpose (K5, unlike Open's own
// verdi.yaml handling above): a read failure still gets this function's
// own "reading model.yaml: %w" prefix, but a model.DecodeModel failure is
// returned UNWRAPPED. DecodeModel is already the authoritative source of
// its own error framing — its strict-decode-failure path self-prefixes
// with "model: decoding model.yaml: %w" (decode.go), while its
// Validate/checkFrontier failure paths return their own already-complete
// text (a kernel rule's own "model: ..." message, or the pinned frontier
// text) with no such prefix at all. Re-wrapping here unconditionally, as
// this function used to, doubled the strict-decode case's prefix
// ("decoding model.yaml: model: decoding model.yaml: ...") while adding
// a misleading "decoding model.yaml:" framing to Validate/checkFrontier
// failures that were never about a decode step. Passing the error
// through keeps every %w chain intact (errors.Is(err, model.ErrFrontier)
// still holds through this now-shorter chain) and gives each failure mode
// exactly the framing it actually has.
func loadModel(root string) (*model.Model, error) {
	path := filepath.Join(root, ".verdi", "model.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.Canonical(), nil
		}
		return nil, fmt.Errorf("reading model.yaml: %w", err)
	}
	m, err := model.DecodeModel(data)
	if err != nil {
		return nil, err
	}
	return m, nil
}
