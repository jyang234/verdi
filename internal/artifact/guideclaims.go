package artifact

import (
	"fmt"
	"strings"
)

// guideClaimsSchema is verdi.guideclaims/v1's schema literal
// (spec/guide-claims-gate ac-1).
const guideClaimsSchema = "verdi.guideclaims/v1"

// GuideClaimStatus is the guide's Appendix B three-valued honesty enum
// (2026-07-17-integration-startup-guide.md "Appendix B — Honesty ledger":
// EXISTS = shipped behavior on main; PARTIAL = the mechanism exists in a
// narrower shape; INVENTED = proposed, no code, no spec yet). Unknown
// values fail closed at decode (CLAUDE.md: "unknown enum values fail
// closed").
type GuideClaimStatus string

// The closed GuideClaimStatus enum.
const (
	GuideClaimExists   GuideClaimStatus = "EXISTS"
	GuideClaimPartial  GuideClaimStatus = "PARTIAL"
	GuideClaimInvented GuideClaimStatus = "INVENTED"
)

var guideClaimStatuses = map[GuideClaimStatus]bool{
	GuideClaimExists:   true,
	GuideClaimPartial:  true,
	GuideClaimInvented: true,
}

// GuideClaimWitness is one corpus test name bound to a row — one element
// of ac-1's "one witness set". spec/guide-claims-gate ac-2 binds each
// EXISTS/PARTIAL row's witnesses three independent ways
// (internal/specalign/guideclaims_test.go: name-in-corpus, the
// `// guide-claim: <row-id>` anchor at the witness's own declaration, and
// PASS-coupling in `make verify`); this type carries only the name — the
// other two bindings are checked against the live corpus, not stored
// here.
type GuideClaimWitness struct {
	Name string `yaml:"name"`
}

// GuideClaimRow is one ATOMIC capability row (spec/guide-claims-gate
// ac-1): exactly one capability, one status, one witness set. A row
// shape that tries to describe more than one capability under itself
// (an early draft's "sub_claims:" list, or a "capability:" that is a
// YAML sequence instead of a scalar) is rejected at decode by
// DecodeGuideClaims's KnownFields(true)/type-mismatch failure — never
// silently accepted as one merged claim.
//
// ID is this row's stable identity: the `// guide-claim: <row-id>`
// anchor's target and this manifest's own primary key (DecodeGuideClaims
// rejects a duplicate ID).
type GuideClaimRow struct {
	ID         string              `yaml:"id"`
	Section    string              `yaml:"section"`
	Capability string              `yaml:"capability"`
	Status     GuideClaimStatus    `yaml:"status"`
	Caveat     string              `yaml:"caveat,omitempty"`
	Cite       string              `yaml:"cite,omitempty"`
	Witnesses  []GuideClaimWitness `yaml:"witnesses,omitempty"`
}

// GuideClaimsManifest is verdi.guideclaims/v1's top-level document shape:
// `verdi/docs/guide-claims.yaml`, a flat list of atomic rows transcribing
// the Integration & Startup Guide's Appendix B (spec/guide-claims-gate).
// It is a plain top-level YAML document (no frontmatter delimiters),
// mirroring internal/model's own DecodeModel wrapper over the shared
// internal/artifact strict-decode seam.
type GuideClaimsManifest struct {
	Schema string          `yaml:"schema"`
	Rows   []GuideClaimRow `yaml:"rows"`
}

// DecodeGuideClaims strict-decodes `verdi/docs/guide-claims.yaml` through
// the single internal/artifact seam (DecodeStrict: KnownFields(true) +
// the restricted YAML dialect), then validates the schema literal and
// every row (Validate below). This is the schema's one entry point —
// internal/specalign/guideclaims_test.go never decodes guide-claims.yaml
// any other way.
func DecodeGuideClaims(data []byte) (*GuideClaimsManifest, error) {
	var m GuideClaimsManifest
	if err := DecodeStrict(data, &m); err != nil {
		return nil, fmt.Errorf("artifact: decoding guide-claims.yaml: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks the schema literal and every row: id/section/capability
// non-empty, id unique, status drawn from the closed enum, a PARTIAL
// row's caveat is present (spec/guide-claims-gate ac-3: "a PARTIAL row
// without accompanying caveat text reds"), a non-EXISTS row's cite is
// present (ac-3: "a non-EXISTS row ... carries a cite: field ... a row
// lacking cite: where one is required reds"), and an EXISTS or PARTIAL row
// binds at least one witness (ac-2: a live capability claim with no witness
// at all is the ADJ-50 lying-gate class this story exists to close). That
// last rule lives here at decode, fail-closed, so the decoder and
// internal/specalign's gate agree — the gate's evaluateGuideClaimRows can
// never be handed a witnessed-status row with an empty witness set
// (judged-ac2-zero-witness-red-untested). It does NOT check cite
// RESOLUTION (does the cited entry actually exist) — that is
// internal/specalign's workspace-side-only, loud-skip-on-unavailable
// check (ac-3's own fidelity-precedent split), since the chronicle lives
// outside this repository and this package has no notion of a workspace
// root.
func (m GuideClaimsManifest) Validate() error {
	if m.Schema != guideClaimsSchema {
		return fmt.Errorf("artifact: guide-claims.yaml: schema = %q, want %q", m.Schema, guideClaimsSchema)
	}
	seen := make(map[string]bool, len(m.Rows))
	for i, r := range m.Rows {
		label := r.ID
		if label == "" {
			label = fmt.Sprintf("(row %d)", i)
		}
		if r.ID == "" {
			return fmt.Errorf("artifact: guide-claims.yaml: row %d: id is required", i)
		}
		if seen[r.ID] {
			return fmt.Errorf("artifact: guide-claims.yaml: row %s: duplicate id", label)
		}
		seen[r.ID] = true
		if r.Section == "" {
			return fmt.Errorf("artifact: guide-claims.yaml: row %s: section is required", label)
		}
		if r.Capability == "" {
			return fmt.Errorf("artifact: guide-claims.yaml: row %s: capability is required", label)
		}
		if !guideClaimStatuses[r.Status] {
			return fmt.Errorf("artifact: guide-claims.yaml: row %s: status %q is not one of EXISTS/PARTIAL/INVENTED", label, r.Status)
		}
		if r.Status == GuideClaimPartial && r.Caveat == "" {
			return fmt.Errorf("artifact: guide-claims.yaml: row %s: status PARTIAL requires caveat text (spec/guide-claims-gate ac-3)", label)
		}
		if r.Status != GuideClaimExists && r.Cite == "" {
			return fmt.Errorf("artifact: guide-claims.yaml: row %s: status %s requires a cite: field naming a chronicle/ledger entry (spec/guide-claims-gate ac-3)", label, r.Status)
		}
		if r.Cite != "" {
			if err := validateGuideClaimCite(r.Cite); err != nil {
				return fmt.Errorf("artifact: guide-claims.yaml: row %s: %w", label, err)
			}
		}
		if (r.Status == GuideClaimExists || r.Status == GuideClaimPartial) && len(r.Witnesses) == 0 {
			return fmt.Errorf("artifact: guide-claims.yaml: row %s: status %s requires at least one witness (spec/guide-claims-gate ac-2; a live capability claim with no witness is exactly the ADJ-50 lying-gate class) — judged-ac2-zero-witness-red-untested", label, r.Status)
		}
		for j, w := range r.Witnesses {
			if w.Name == "" {
				return fmt.Errorf("artifact: guide-claims.yaml: row %s: witness %d: name is required", label, j)
			}
		}
	}
	return nil
}

// validateGuideClaimCite enforces the `<path>#<anchor>` cite: SHAPE at DECODE
// time (spec/guide-claims-gate ac-3), fail-closed. Before this, cite: shape
// was validated only workspace-side inside internal/specalign's resolveCite
// (via parseCite), so decode — and therefore CI, which strict-decodes but has
// no access to the out-of-repo chronicle — accepted ANY non-empty string as a
// cite: a free-text placeholder ("TODO"), a bare path with no anchor, or an
// empty path/anchor around the '#' all passed, and CI could not tell them from
// a genuine chronicle/ledger reference (judged-ac3-cite-shape-and-anchor-
// semantics-weaker-than-entry-existence). The shape check has no filesystem
// dependency, so it is identical in CI and workspace and belongs at decode
// beside the other fail-closed rules. It validates SHAPE only — whether the
// cited entry genuinely EXISTS remains resolveCite's workspace-side,
// loud-skip-on-unavailable job, since the chronicle lives outside this repo.
func validateGuideClaimCite(cite string) error {
	i := strings.Index(cite, "#")
	if i < 0 {
		return fmt.Errorf("cite %q is not shaped <path>#<anchor>: no '#' anchor separator — a free-text or bare-path cite cannot be told from a real chronicle/ledger reference (spec/guide-claims-gate ac-3)", cite)
	}
	if strings.TrimSpace(cite[:i]) == "" {
		return fmt.Errorf("cite %q is not shaped <path>#<anchor>: empty <path> before '#' (spec/guide-claims-gate ac-3)", cite)
	}
	if strings.TrimSpace(cite[i+1:]) == "" {
		return fmt.Errorf("cite %q is not shaped <path>#<anchor>: empty <anchor> after '#' (spec/guide-claims-gate ac-3)", cite)
	}
	return nil
}
