package evidence

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OWNER/verdi/internal/artifact"
)

// DefaultDeviationsStaleThreshold is the spec-stale flag's threshold-count
// trigger's default (03 §The amendment ladder: "more than a configured
// count of accepted-deviation dispositions accumulated on one story
// (verdi.yaml: audit.deviations_stale_threshold, default 3, tunable —
// a watch item)").
const DefaultDeviationsStaleThreshold = 3

// DeviationsStaleThreshold reads verdi.yaml's audit.deviations_stale_threshold
// (R4-I-10, PLAN-V1.md §7), returning DefaultDeviationsStaleThreshold when
// the manifest, the audit: block, or the key itself is absent.
//
// This is a deliberately narrow, temporary seam, not a permanent
// alternative to internal/store's manifest decode. V1-P3 (this phase) and
// V1-P2 (which is adding the audit: block, along with spike_paths:, to
// internal/store's Manifest type) run in the same wave and share nothing
// — per this phase's brief, neither may touch the other's packages. Using
// internal/store.DecodeManifest here would strict-decode the *whole*
// verdi.yaml against a Manifest type that — on this phase's branch point —
// does not yet declare audit:, so any real verdi.yaml carrying that block
// would fail to decode at all. This function instead reads only the one
// key it needs through artifact.DecodeYAMLLoose (already exported by the
// module's single YAML import seam, internal/artifact) — a loose,
// schema-agnostic decode that tolerates every other top-level key it does
// not care about.
//
// At the V1-P2/V1-P3 merge, the merge reviewer should delete this file and
// point every caller (currently: SpecStale's callers) at
// internal/store.Manifest.Audit.DeviationsStaleThreshold directly — the
// value has exactly one intended long-term home, and this is not it.
func DeviationsStaleThreshold(root string) (int, error) {
	path := filepath.Join(root, ".verdi", "verdi.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultDeviationsStaleThreshold, nil
		}
		return 0, fmt.Errorf("evidence: reading %s: %w", path, err)
	}

	generic, err := artifact.DecodeYAMLLoose(data)
	if err != nil {
		return 0, fmt.Errorf("evidence: parsing %s: %w", path, err)
	}
	top, ok := generic.(map[string]interface{})
	if !ok {
		return DefaultDeviationsStaleThreshold, nil
	}
	auditRaw, ok := top["audit"]
	if !ok {
		return DefaultDeviationsStaleThreshold, nil
	}
	audit, ok := auditRaw.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("evidence: %s: audit: block is not a mapping", path)
	}
	thRaw, ok := audit["deviations_stale_threshold"]
	if !ok {
		return DefaultDeviationsStaleThreshold, nil
	}
	switch v := thRaw.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case uint64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("evidence: %s: audit.deviations_stale_threshold is not an integer (got %T)", path, thRaw)
	}
}
