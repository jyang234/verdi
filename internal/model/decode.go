package model

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// modelSchema is verdi.model/v1's schema literal (guide §5.2, table
// C-2: "kernel enum — the frontier's version axis").
const modelSchema = "verdi.model/v1"

// DecodeModel strict-decodes and validates a `.verdi/model.yaml`
// document: the schema's one entry point (spec/model-schema ac-1),
// mirroring store.DecodeManifest's own wrapper pattern
// (internal/store/manifest.go) over the shared internal/artifact
// strict-decode seam (KnownFields(true) + dialect rejection of
// anchors/aliases/custom tags — this package never imports yaml.v3
// directly, same seam discipline artifact.TestYAMLImportSeam enforces
// module-wide).
//
// Two passes run after decode, in order: Validate (validate.go) checks
// the kernel rules that make ANY model well-formed regardless of which
// concrete lifecycle it describes (obligations-list presence, terminal
// subset, reachability, declared endpoints/parents, non-empty
// templates, and the closed scheme/kind catalogs); checkFrontier then
// checks that THIS particular model matches today's canonical shape
// (stage 1's frontier, dc-1) — vocabulary and per-class template
// filenames excepted. Either failing returns a non-nil error; callers
// (cmd/verdi/model.go) do not need to know which pass produced it, only
// print it.
func DecodeModel(data []byte) (*Model, error) {
	var m Model
	if err := artifact.DecodeStrict(data, &m); err != nil {
		return nil, fmt.Errorf("model: decoding model.yaml: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if err := m.checkFrontier(); err != nil {
		return nil, err
	}
	return &m, nil
}
