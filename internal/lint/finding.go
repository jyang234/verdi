package lint

import (
	"fmt"

	"github.com/jyang234/verdi/internal/disclosure"
)

// Severity distinguishes a verdict failure from a printed disclosure.
type Severity int

const (
	// SeverityViolation is a verdict failure: it flips `verdi lint` to
	// exit 1. It is the zero value, so every finding is a violation unless
	// the rule that raised it says otherwise — no rule can accidentally
	// downgrade a real violation to a notice by forgetting to set a field.
	SeverityViolation Severity = iota
	// SeverityDisclosure is a printed notice that is NOT a verdict failure:
	// it is surfaced on every run (never silent — CLAUDE.md constitution 2,
	// three-valued honesty) but never flips the exit code on its own. A
	// clean run carrying only disclosures still exits 0. VL-017's
	// disclosed-unproven report when the mutable zone is absent (a CI clone)
	// uses this: disclosure is not failure, so CI stays green once a
	// new-class spec exists (adjudicated at the W2 wave close).
	SeverityDisclosure
)

// Finding is one lint result: which rule fired, on what path, why, and at
// what severity. The engine reports every finding from every rule — lint
// never stops at the first failure (CLAUDE.md constitution 2: "silence is
// never a pass").
type Finding struct {
	// Rule is the VL-xxx id that fired.
	Rule string
	// Path is the store-root-relative, slash-separated path the finding is
	// about (e.g. ".verdi/adr/0001-outbox-events.md"), or a store-relative
	// non-file locus (e.g. ".gitattributes") for repository-wide rules.
	Path string
	// Message is a human-readable explanation, naming the offending value.
	Message string
	// Severity is SeverityViolation (the zero value — a verdict failure) or
	// SeverityDisclosure (a printed notice that does not flip the exit code).
	Severity Severity
	// Locus is this finding's optional, SELF-DECLARED wall badge placement
	// (spec/badge-computes dc-3, spec/wall-receipts dc-3): populated ONLY
	// by the rule that raises the finding, at the exact point it builds
	// the Finding — never inferred afterward from Rule or Path by any
	// consumer (a wall-side allowlist of rule ids is exactly what dc-3
	// forbids). nil — the zero value, so a rule that says nothing about
	// placement gets it automatically, with no per-rule opt-out to forget
	// — means the finding declares no wall locus at all and never reaches
	// a board projection, fail-closed, regardless of Path: this is how
	// store-structural/plumbing findings (gitattributes, data-tracking,
	// status-in-path, a dangling layout.json key) and decode failures
	// (unparsed-island territory) stay off the wall without the wall ever
	// naming their rule ids.
	Locus *WallLocus
}

// WallLocus is a Finding's self-declared badge placement: either an
// OBJECT ANCHOR (Object non-empty — an acceptance criterion, constraint,
// decision, or open-question id, or a declared stub's "stub:<slug>" key —
// naming the rendered card this finding badges) or a SPEC-LEVEL marker
// (Object empty — the finding badges the case file, not any single
// object's card). Both forms are non-nil *WallLocus values; only a nil
// Locus on the Finding itself means "declares nothing" (off the wall).
type WallLocus struct {
	// Object is the rendered board object id this finding anchors to, or
	// "" for a spec-level finding.
	Object string
}

// ObjectLocus declares a finding as object-anchored: it badges the
// rendered card of id (an acceptance criterion, constraint, decision, or
// open-question id, or a declared stub's "stub:<slug>" key — every id
// shape internal/workbench's board projection can render a card for).
func ObjectLocus(id string) *WallLocus { return &WallLocus{Object: id} }

// SpecLocus declares a finding as spec-level: it badges the case file
// (the spec as a whole), never a single object's card.
func SpecLocus() *WallLocus { return &WallLocus{} }

// locusAll stamps every VIOLATION finding in fs with locus and returns fs —
// a small shared helper for the common case of a rule sub-check whose entire
// return value shares one wall placement (e.g. every finding from one
// spec-level sub-check), so call sites read as a one-line wrap rather
// than a manual loop repeated per rule.
//
// A disclosure (SeverityDisclosure) is deliberately LEFT with a nil Locus: a
// disclosed-unproven notice surfaces through the disclosures channel, never as
// a board badge, and internal/wallbadge's VLBadges keys on Locus ALONE with no
// severity filter — so a disclosure that inherited a locus here would wrongly
// render as a wall chip. Leaving it nil is the same fail-closed posture
// VL-017's own disclosure relies on (a rule that says nothing about placement
// gets none). This matters for VL-003's checkPin, whose context[] pins are
// wrapped with SpecLocus here: its shallow-unprovable disclosure (P2-10b) must
// stay off the wall.
func locusAll(fs []Finding, locus *WallLocus) []Finding {
	for i := range fs {
		if fs[i].Severity == SeverityDisclosure {
			continue
		}
		fs[i].Locus = locus
	}
	return fs
}

// String formats f as "VL-xxx path: message" — the CLI's one-line-per-
// finding output format. A disclosure renders through the shared
// internal/disclosure seam (spec/disclosure-seam-v2, ac-1) instead of a
// locally-authored prefix, so it shares its exact phrasing with the gate's
// disclosed conditions and the mcp/workbench review_unavailable field —
// never coincidentally-matching hand-aligned strings (see
// conflict/disclosure-seam-rename-insufficient for why the earlier
// rename-only attempt was insufficient).
func (f Finding) String() string {
	if f.Severity == SeverityDisclosure {
		return disclosure.Render(f.Disclosure())
	}
	return fmt.Sprintf("%s %s: %s", f.Rule, f.Path, f.Message)
}

// Disclosure maps f onto the shared seam value: rule id as source
// ("lint:VL-xxx"), path as scope, message as text. This is the ONE
// Finding->Disclosure mapping in the codebase — String()'s disclosure
// branch renders it, and the disclosures-view enumeration
// (spec/disclosures-panel ac-1) collects it — so the CLI's printed line
// and an enumerated panel item are the same value by construction, never
// two hand-aligned copies.
func (f Finding) Disclosure() disclosure.Disclosure {
	return disclosure.New("lint:"+f.Rule, f.Path, f.Message)
}
