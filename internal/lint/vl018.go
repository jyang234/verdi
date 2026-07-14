package lint

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// vl018 enforces "layout.json positions: every key in a spec directory's
// positions map (verdi.boardlayout/v1, §Record schemas) resolves to a real
// object ID declared in that spec's frontmatter, or — as `stub:<slug>` —
// to a declared stub of that spec" (02 §Lint rules; R4-I-5, extended round
// 5.5 dc-6: "Same verbatim-pass-through, prune, and display-resolution
// semantics either way"). It never gates on absence — a spec directory
// with no layout.json at all is not this rule's concern (01 §notes: an
// absent layout.json falls back to the zoned-incremental layout
// algorithm).
type vl018 struct{}

func (vl018) ID() string { return "VL-018" }

func (vl018) Check(in *RunInput) []Finding {
	var findings []Finding

	specBySpecDir := make(map[string]*Document, len(in.Snapshot.Docs))
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Spec == nil {
			continue
		}
		specBySpecDir[specDirOf(d)] = d
	}

	for _, l := range in.Snapshot.Layouts {
		if l.DecodeErr != nil {
			findings = append(findings, Finding{Rule: "VL-018", Path: l.RelPath, Message: fmt.Sprintf("layout.json does not decode: %v", l.DecodeErr)})
			continue
		}
		spec, ok := specBySpecDir[l.SpecDir]
		if !ok {
			// No sibling spec.md decoded in this directory (missing, or
			// itself decode-failed/grandfathered) — nothing to resolve
			// positions keys against; VL-002/VL-001 own a missing/broken
			// spec.md, not this rule.
			continue
		}
		declared := artifact.DeclaredObjectIDs(spec.Spec)
		stubs := artifact.DeclaredStubSlugs(spec.Spec)
		for key := range l.Layout.Positions {
			if declared[key] {
				continue
			}
			if slug, ok := strings.CutPrefix(key, "stub:"); ok && stubs[slug] {
				continue
			}
			findings = append(findings, Finding{Rule: "VL-018", Path: l.RelPath, Message: fmt.Sprintf("positions key %q does not resolve to a declared object id or stub in %s", key, spec.RelPath)})
		}
	}

	return findings
}
