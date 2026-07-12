package lint

import "fmt"

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
}

// String formats f as "VL-xxx path: message" — the CLI's one-line-per-
// finding output format. A disclosure is prefixed "disclosed-unproven: "
// (disclosure-seam story, R5-2: rename-in-place attempt at ac-1's shared
// disclosure vocabulary — see the gate and review_unavailable call sites
// for the same token) so a reader (and CI logs) can tell a printed
// disclosure apart from a real violation.
func (f Finding) String() string {
	if f.Severity == SeverityDisclosure {
		return fmt.Sprintf("disclosed-unproven: %s %s: %s", f.Rule, f.Path, f.Message)
	}
	return fmt.Sprintf("%s %s: %s", f.Rule, f.Path, f.Message)
}
