package artifact

import "fmt"

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
// without accompanying caveat text reds"), and a non-EXISTS row's cite is
// present (ac-3: "a non-EXISTS row ... carries a cite: field ... a row
// lacking cite: where one is required reds"). It does NOT check cite
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
		for j, w := range r.Witnesses {
			if w.Name == "" {
				return fmt.Errorf("artifact: guide-claims.yaml: row %s: witness %d: name is required", label, j)
			}
		}
	}
	return nil
}
