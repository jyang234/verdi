// The workbench's class-word display vocabulary (spec/vocabulary-surfaces
// ac-2; the vocabulary-prose category closure). One seam, one rule —
// model.DisplayClass's ENUMERATION RULE, restated for this package's
// surfaces:
//
//   - DISPLAY PROSE resolves. Every visible word that speaks a class —
//     the stub cards' "story stub"/"spike stub" labels, the sealed wall's
//     "Instantiate story"/"Instantiate spike" buttons, the four-move
//     guide's feature-wall note, the yarn key's scoping meanings, the
//     oq multi-claim chip, the obligation-yarn tooltip, a story/spike
//     proto-sticky's visible type word, the edge pickers' consequence
//     labels, the v0 board's commit-to-design copy, the corpus page's
//     Class/Story rows, and the verdict page's snapshot copy — comes
//     from classWords (below) or the payload words boardspec.js reads.
//   - The IDENTITY layer stays bare, always: object ids and refs, URLs
//     and route patterns, branch names, CSS classes (stubcard--spike,
//     sticky--story), data-* attributes (data-annotation-type,
//     data-stub, data-spike), testids, boardClientPayload.Class (the
//     client's gating value), the annotation-type ENUM VALUES on the
//     wire and in API refusal diagnostics ("one of comment, question,
//     decision-needed, agent-task, story, spike"), and the fold's
//     verdict keys (story.violated/story.eligible on the matrix page).
//   - "spike" is a variant marker, not a model class: it resolves
//     through Vocabulary.Classes exactly like a class word and falls
//     back to itself (applyModelVocabulary's established pseudo-class
//     treatment; the L-M8 posture — taxonomy decided and documented).
//
// New prose in this package that speaks feature/story/spike obligates a
// classification against this rule at the prose site.
package workbench

import "github.com/jyang234/verdi/internal/model"

// classWords resolves the class words the workbench's display prose
// speaks, through the identical model chain every other surface uses
// (model.DisplayClass and friends — never a board-private rename table).
// The zero value resolves every id to itself (the model methods are
// nil-receiver-safe), so projections built without a model — every test
// literal, every degraded open — render today's bare words byte-for-byte
// (the parity floor).
type classWords struct {
	m *model.Model
}

// word is the class id's display word (id fallback).
func (w classWords) word(id string) string { return w.m.DisplayClass(id) }

// plural is the class id's display word pluralized (best-effort English;
// reproduces today's hand-written plurals when nothing is renamed).
func (w classWords) plural(id string) string { return w.m.DisplayClassPlural(id) }

// capital is the class id's display word with its first rune upper-cased
// — label positions ("Story ref") only.
func (w classWords) capital(id string) string { return model.Capitalize(w.m.DisplayClass(id)) }

// indefinite is the class id's display word with its agreeing indefinite
// article ("a story" / "an Initiative") — a delegate to model.Indefinite,
// the one composed article-word form (Q1; the a/an rule itself is
// model.Article, judged-article-agreement-approximation-undisclosed,
// L-M13a(4)) — for prose positions where the article immediately precedes
// the class word. An article whose head word is fixed prose ("a draft
// <word> spec") agrees with that fixed word instead and never routes
// through this.
func (w classWords) indefinite(id string) string {
	return model.Indefinite(w.word(id))
}

// verb is a lifecycle verb id's display word (id fallback), through the
// identical model chain (model.DisplayVerb) — the verb-word display
// discipline spec/creation-surfaces ac-5 names: a verb-speaking surface
// this feature creates (the creation form's copy, spec/creation-form
// ac-3) routes its verb words here, never hand-written bare verb prose.
func (w classWords) verb(id string) string { return w.m.DisplayVerb(id) }

// renamed returns only the class words whose display form differs from
// the bare id — the payload map boardspec.js reads for its own dialog
// copy and menu labels, empty (omitted) for a no-rename store so the
// embedded page state stays byte-identical (the ClassLabel posture,
// projection.go).
func (w classWords) renamed() map[string]string {
	var out map[string]string
	for _, id := range []string{"feature", "story", "spike"} {
		if v := w.word(id); v != id {
			if out == nil {
				out = map[string]string{}
			}
			out[id] = v
		}
	}
	return out
}
