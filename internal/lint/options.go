package lint

// Options controls engine behavior beyond the rules themselves.
type Options struct {
	// GrandfatherArchive skips VL-001..VL-006 for files under
	// specs/archive/ (02 §Open questions OQ-3: "the lint grandfather flag
	// (skip VL-001..006 under specs/archive/ on import)"). Off by default;
	// implemented for the one-time bulk-migration scenario OQ-3 describes,
	// otherwise dormant — no v0 caller turns it on (PLAN.md: "No importer
	// in v0 ... otherwise dormant").
	GrandfatherArchive bool
}
