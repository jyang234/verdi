package artifact

import "fmt"

// DiagramClassProposal is the only non-empty value DiagramFrontmatter.Class
// carries (02 §Diagram proposals: "class ... discriminator; absent = the
// incumbent authored-living diagram").
const DiagramClassProposal = "proposal"

// DiagramFrontmatter is the frontmatter schema for kind "diagram"
// (02 §Kind registry: class absent -> active -> superseded,
// authored-living, never frozen; 02 §Diagram proposals,
// spec/proposal-artifact dc-1: class: proposal -> proposed -> accepted,
// frozen at acceptance).
type DiagramFrontmatter struct {
	Base   `yaml:",inline"`
	Status Status `yaml:"status"`

	// Class discriminates the incumbent, authored-living diagram (absent,
	// the only shape this kind had before spec/proposal-artifact) from a
	// future-state proposal (DiagramClassProposal). Any other non-empty
	// value fails closed in Validate.
	Class string `yaml:"class,omitempty"`
	// Scope pins the flowmap --root selector truth is regenerated under
	// (02 §Diagram proposals, diagram-proposals dc-6); absent means the
	// whole graph, hairball cap disclosed. Orthogonal to DerivedFrom — a
	// from-scratch proposal may still carry Scope. Opaque to this story:
	// verification-extractor (not yet built) owns what it means.
	Scope string `yaml:"scope,omitempty"`
	// DerivedFrom is present iff this proposal was forked from a
	// generated base (02 §Diagram proposals); nil for a from-scratch
	// proposal and for every incumbent diagram.
	DerivedFrom *DiagramDerivedFrom `yaml:"derived_from,omitempty"`
}

// DiagramDerivedFrom names the pinned generated base a derived proposal
// forked from (02 §Diagram proposals): Ref is the base diagram ref (a
// diagram/<name> ref, optionally @commit-pinned to the fork point);
// Digest is the sha256 of that base's canonical graph JSON at that
// commit — the truth generator's own graph and the stale-base detector's
// reference point (internal/diagramverify's StaleBase recomputes it by
// re-running flowmap at current HEAD).
//
// SourceDigest is the OPTIONAL round-6 (ADJ-16) companion: sha256 of the
// canonical JSON of the node/edge graph the verification extractor's own
// one-way grammar extracts from the base's COMMITTED mermaid body at the
// pinned commit — recomputable from git history alone, unlike Digest's
// flowmap graph JSON, which is never committed. The mechanical
// before-peek and reset (spec/diagram-proposals ac-3), pure functions of
// provenance, gate on SourceDigest (internal/diagrambase); Digest stays
// the truth-movement comparand. A derived proposal without SourceDigest
// renders peek/reset disclosed-unavailable — never guessed, never gated
// on the wrong digest.
//
// Validate here checks only presence and ref-SHAPE, deliberately NOT
// corpus resolution or the sha256:<64-hex> digest FORMAT: spec/
// proposal-artifact ac-5 assigns both of those checks to the new VL-021
// lint rule instead (internal/lint/vl021.go), which needs a dangling ref
// or a malformed digest to decode cleanly in the first place in order to
// distinguish "present but wrong" (its own finding) from "absent or
// structurally malformed" (a decode failure) — see vl021.go's doc comment
// for the fixture that exercises exactly this split. SourceDigest is
// optional and, like Digest, format-checked by VL-021 (only when present),
// not here.
type DiagramDerivedFrom struct {
	Ref          string `yaml:"ref" json:"ref"`
	Digest       string `yaml:"digest" json:"digest"`
	SourceDigest string `yaml:"source_digest,omitempty" json:"source_digest,omitempty"`
}

// Validate checks Ref is present and parses as a ref (pinned or
// unpinned — a derived proposal's base is typically pinned to its fork
// commit, but this package does not require it) and Digest is present.
// SourceDigest is optional (ADJ-16): its absence is legal here and renders
// peek/reset disclosed-unavailable at the editor seam, never an error.
func (d DiagramDerivedFrom) Validate() error {
	if d.Ref == "" {
		return fmt.Errorf("artifact: derived_from.ref is required")
	}
	if _, err := ParseRef(d.Ref); err != nil {
		return fmt.Errorf("artifact: derived_from.ref: %w", err)
	}
	if d.Digest == "" {
		return fmt.Errorf("artifact: derived_from.digest is required")
	}
	return nil
}

// DecodeDiagram strict-decodes and validates diagram frontmatter.
func DecodeDiagram(data []byte) (*DiagramFrontmatter, error) {
	var fm DiagramFrontmatter
	if err := DecodeStrict(data, &fm); err != nil {
		return nil, err
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Validate checks the common fields, DerivedFrom's shape (if present),
// and branches on Class exactly the way SpecFrontmatter.Validate branches
// on a spec's Class (spec/proposal-artifact dc-1, an established pattern
// in this codebase, not a new one): class: proposal uses the distinct
// proposalStatuses enum {proposed, accepted} and requires Frozen iff
// status: accepted; Class absent keeps the original diagramStatuses
// {active, superseded} enum and requireFrozen(..., false, ...) exactly as
// it decoded before this story, so every pre-existing incumbent-diagram
// fixture keeps decoding byte-identically.
func (fm DiagramFrontmatter) Validate() error {
	if err := fm.validateBase(KindDiagram); err != nil {
		return err
	}
	if fm.DerivedFrom != nil {
		if err := fm.DerivedFrom.Validate(); err != nil {
			return err
		}
	}

	switch fm.Class {
	case "":
		if !diagramStatuses[fm.Status] {
			return fmt.Errorf("artifact: diagram status %q is not a known status", fm.Status)
		}
		return requireFrozen(fm.Frozen, false, "diagram", string(fm.Status))
	case DiagramClassProposal:
		if !proposalStatuses[fm.Status] {
			return fmt.Errorf("artifact: proposal diagram status %q is not a known status (proposed/accepted only, spec/proposal-artifact ac-1)", fm.Status)
		}
		frozenRequired := fm.Status == "accepted"
		return requireFrozen(fm.Frozen, frozenRequired, "proposal diagram", string(fm.Status))
	default:
		return fmt.Errorf("artifact: diagram class %q is not a known class (only %q, or absent)", fm.Class, DiagramClassProposal)
	}
}

// The four-value DISCLOSED status vocabulary (02 §Diagram proposals):
// proposed and accepted are AUTHORED (proposalStatuses); realized and
// stale are COMPUTED ONLY — absent from proposalStatuses by construction,
// which is what makes strict decode refuse them as authored input
// (spec/proposal-artifact ac-4/dc-3: "the enforcement mechanism ... is the
// decode boundary itself, not a separate runtime guard").
const (
	DiagramStatusRealized Status = "realized"
	DiagramStatusStale    Status = "stale"
)

// ResidualDiff is the residual outcome of regenerating truth for a derived
// proposal and structurally diffing it against the proposal's own
// content — verification-extractor's own three-way diff result
// (spec/diagram-proposals ac-1), consumed here by reference and never
// recomputed (spec/proposal-artifact dc-3: "does NOT compute the residual
// itself ... consumed here through its own return type, not
// reimplemented"). verification-extractor is a separate, not-yet-built
// story; this minimal stand-in carries only what DiagramDisclosedStatus
// needs — whether the residual is empty — so a later swap to the real
// type changes no caller of this function.
type ResidualDiff struct {
	// Elements names each residual discrepancy the diff surfaced (e.g. a
	// contradicted or stale-base node/edge identity). Its own vocabulary
	// is verification-extractor's to define; only emptiness matters here.
	Elements []string
}

// DiagramDisclosedStatus computes the four-value disclosed status
// (spec/proposal-artifact ac-4, dc-3) as a pure function of fm's authored
// status plus an externally supplied residual-diff outcome: no I/O, no
// clock, no global state. residual == nil means no verification has run
// yet, so the authored status (proposed or accepted) passes through
// unchanged. Once a residual is supplied for an accepted proposal, an
// empty residual discloses DiagramStatusRealized (regeneration diff
// leaves nothing outstanding) and a non-empty one discloses
// DiagramStatusStale (truth has diverged). Neither computed value is ever
// written back to the artifact by this function or by anything else in
// this story — see the decode-boundary note on the two constants above.
func DiagramDisclosedStatus(fm DiagramFrontmatter, residual *ResidualDiff) Status {
	if residual == nil || fm.Status != "accepted" {
		return fm.Status
	}
	if len(residual.Elements) == 0 {
		return DiagramStatusRealized
	}
	return DiagramStatusStale
}
