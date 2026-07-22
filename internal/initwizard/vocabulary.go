// Package initwizard is the interview logic behind `verdi init --wizard`
// (spec/init-wizard, ledger L-N5, design doc §12 rules W-1..W-4/W-3b),
// kept separate from cmd/verdi/init.go's verb plumbing (CLAUDE.md: "one
// package = one concern"): this package owns the Q&A state machine, the
// candidate model's live validation preview, and materializing content
// (verdi.yaml, model.yaml, template overrides) into an ALREADY-STAGED
// root. It knows nothing about the staging directory's own lifecycle —
// creating the sibling temp dir, running the promotion gate, the single
// os.Rename, or rollback on error — cmd/verdi/init.go owns that whole
// orchestration and the CLI's exit-code discipline, calling into this
// package only for "what content, and is it valid."
//
// The v1 frontier bounds this interview precisely to what
// internal/model's checkFrontier actually admits on top of the canonical
// model: Vocabulary display renames (classes, states, verbs) and a
// class's template-file choice (here, offered as "copy the canonical
// templates into .verdi/templates/ for local editing"). Nothing else —
// class/state/transition/obligation SETS are kernel-fixed — is ever
// offered as a real choice; a request for anything beyond that is
// answered with the frontier explanation, never silently honored.
package initwizard

import (
	"sort"

	"github.com/jyang234/verdi/internal/model"
)

// vocabularySpikePseudoClass mirrors internal/model/validate.go's own
// unexported constant of the same name: "spike" is not a Model.Classes
// key, but it is the one pseudo-class Vocabulary.Classes legally renames
// (the L-M13-ratified carve) — the interview offers it as a rename
// target for exactly that reason. Kept as its own named constant, rather
// than a bare literal, so the two packages' intent is visibly the same
// even though the identifier itself cannot be shared (unexported in
// internal/model).
const vocabularySpikePseudoClass = "spike"

// RenameableIDs is the finite, closed set of ids the wizard's vocabulary
// interview offers a display rename for — every legal
// Vocabulary.Classes/States/Verbs key internal/model/validate.go's own
// validateVocabulary would accept for model.Canonical(), computed from
// the canonical model's own public Classes/Lifecycle fields rather than
// hand-duplicated as a literal list that could silently drift from it.
// Each slice is sorted (CLAUDE.md: deterministic outputs) — map
// iteration order never leaks into interview prompt order.
type RenameableIDSet struct {
	Classes []string
	States  []string
	Verbs   []string
}

// RenameableIDs computes the current RenameableIDSet from
// model.Canonical(). This is the SAME legal-key computation
// validateVocabulary performs (legalClasses/legalStates/legalVerbs,
// internal/model/validate.go) — re-derived here rather than exported
// from internal/model, since internal/initwizard is its only consumer
// (CLAUDE.md: interfaces defined at the consumer) and the canonical
// model's Classes/Lifecycle fields are already public.
func RenameableIDs() RenameableIDSet {
	m := model.Canonical()

	classes := make([]string, 0, len(m.Classes)+1)
	for id := range m.Classes {
		classes = append(classes, id)
	}
	classes = append(classes, vocabularySpikePseudoClass)
	sort.Strings(classes)

	stateSet := map[string]bool{}
	verbSet := map[string]bool{}
	for _, lc := range m.Lifecycle {
		for _, s := range lc.States {
			stateSet[s] = true
		}
		for _, tr := range lc.Transitions {
			verbSet[tr.Verb] = true
		}
	}
	states := make([]string, 0, len(stateSet))
	for s := range stateSet {
		states = append(states, s)
	}
	sort.Strings(states)

	verbs := make([]string, 0, len(verbSet))
	for v := range verbSet {
		verbs = append(verbs, v)
	}
	sort.Strings(verbs)

	return RenameableIDSet{Classes: classes, States: states, Verbs: verbs}
}
