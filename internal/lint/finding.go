package lint

import "fmt"

// Finding is one lint violation: which rule fired, on what path, and why.
// The engine reports every finding from every rule — lint never stops at
// the first failure (CLAUDE.md constitution 2: "silence is never a pass").
type Finding struct {
	// Rule is the VL-xxx id that fired.
	Rule string
	// Path is the store-root-relative, slash-separated path the finding is
	// about (e.g. ".verdi/adr/0001-outbox-events.md"), or a store-relative
	// non-file locus (e.g. ".gitattributes") for repository-wide rules.
	Path string
	// Message is a human-readable explanation, naming the offending value.
	Message string
}

// String formats f as "VL-xxx path: message" — the CLI's one-line-per-
// finding output format.
func (f Finding) String() string {
	return fmt.Sprintf("%s %s: %s", f.Rule, f.Path, f.Message)
}
