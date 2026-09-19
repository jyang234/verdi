package initwizard

import (
	"fmt"

	"github.com/jyang234/verdi/internal/model"
)

// PlainPreset is the `plain` vocabulary preset verdi init writes by default
// (spec/spec-documents ac-11, dc-7; ledger SI-205): the two class renames
// ac-11's own parenthetical pairs with a renameable id — Classes:
// {"story": "planned story", "spike": "research spike"}. ac-11 also names
// "research task" and "revision" in that same parenthetical, but neither
// names a renameable id: RenameableIDs().Classes is exactly {"feature",
// "spike", "story"} (spike being the L-M13 pseudo-class), and neither word
// is a lifecycle state or verb id either. R-W4-4 (SI-205) therefore rules
// that half of ac-11's parenthetical DISCLOSED, not written — the preset
// stays exactly the two class renames above, never a phantom third or
// fourth entry with nowhere to attach.
//
// Returns a fresh model.Vocabulary (fresh maps) on every call: a caller
// mutating the result — RunInterview seeding its own working copy from
// this preset, or a test — can never corrupt another call's value, the
// same "never a shared, cached pointer" contract model.Canonical() keeps
// for the same reason.
func PlainPreset() model.Vocabulary {
	return model.Vocabulary{
		// vocab:identity — the plain preset's display values keyed by class id (spec/spec-documents ac-11, SI-205)
		Classes: map[string]string{"story": "planned story", "spike": "research spike"},
	}
}

// ParseVocabularyPreset maps a `verdi init --vocabulary` value to its
// preset: "plain" -> PlainPreset(), "canonical" -> the empty
// model.Vocabulary (VocabularyEmpty — so the caller's staging step writes
// no model.yaml at all, exactly like a bare, --vocabulary-less init that
// predates this story: the canonical, unrenamed vocabulary is what
// "opting out" of the plain preset means, spec/spec-documents ac-11). Any
// other value is refused with an error naming both legal values — never
// guessed, and never silently treated as canonical.
func ParseVocabularyPreset(name string) (model.Vocabulary, error) {
	switch name {
	case "plain":
		return PlainPreset(), nil
	case "canonical":
		return model.Vocabulary{}, nil
	default:
		return model.Vocabulary{}, fmt.Errorf("initwizard: invalid --vocabulary value %q (must be %q or %q)", name, "plain", "canonical")
	}
}
