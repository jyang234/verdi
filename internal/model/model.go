// Package model implements verdi.model/v1 (ledger L-M1,
// docs/design/plans/2026-07-17-extensibility-phase1-plan.md Task 5;
// reference shape: docs/design/concepts/2026-07-17-integration-startup-
// guide.md §5.2, Appendix C tables C-2/C-3): the operating model's one
// declared source. A store's lifecycle states, transitions, the
// obligations that gate them, the class hierarchy, and display
// vocabulary all decode through this package into typed values via the
// shared internal/artifact strict-decode seam (DecodeModel, decode.go),
// rather than living as Go literals scattered across the codebase
// (spec/model-schema's problem statement, extensibility audit @
// 24214fd: roughly 45 files, seam map S1-S4/S7-S8).
//
// Stage 1 (this package, this phase) DESCRIBES the operating model; it
// does not yet drive transition/obligation ENFORCEMENT — the closure
// gate, accept, and close verbs keep their hardcoded condition slices
// this phase (guide Appendix B sequencing). A structurally deviant
// model.yaml (a different state, transition, class, or obligation set
// than today's hardcoded model) is rejected fail-closed by the frontier
// check in validate.go, with the one pinned error checkFrontier
// documents — v1 accepts only the canonical model, vocabulary and
// per-class template filenames excepted.
package model

import (
	"unicode"

	"github.com/jyang234/verdi/internal/canonjson"
)

// Model is the top-level verdi.model/v1 document (guide §5.2, table
// C-2). Schema, Classes, and Lifecycle are always meaningfully present
// in a well-formed model (Validate rejects a missing/wrong Schema);
// Vocabulary is optional — its zero value (all three maps nil) means
// "no renames," and DisplayState/DisplayVerb fall back to the id
// unchanged in that case.
type Model struct {
	Schema     string               `yaml:"schema" json:"schema"`
	Classes    map[string]Class     `yaml:"classes" json:"classes"`
	Lifecycle  map[string]Lifecycle `yaml:"lifecycle" json:"lifecycle"`
	Vocabulary Vocabulary           `yaml:"vocabulary,omitempty" json:"vocabulary,omitempty"`
}

// Class is one `classes.<id>` entry (table C-2): a spec class's display
// label, its place in the hierarchy (Parent — max depth 2, a later
// frontier per the guide, not enforced this phase), whether it
// decomposes into stubs (guide §6.2), and which template file
// instantiates it (Template — must be non-empty per the kernel rule
// validate.go enforces; the FILE itself is only required to exist from
// Task 8 onward, out of this phase's scope).
//
// Display is the class's own declared label — presentation, and so
// frontier-EXEMPT, exactly like Vocabulary.Classes (the rename layer
// sitting on top of it) and a class's own Template FILENAME. dc-1 draws
// the frontier over the state/transition/class/obligation SETS; a
// display-label change alters none of them, so checkFrontier does not
// compare Display (judged-frontier-display-structural — the controller's
// adjudication of the align finding). Only a class's PRESENCE and its
// hierarchy position (Parent/Decomposes) are structural.
type Class struct {
	Display    string `yaml:"display,omitempty" json:"display,omitempty"`
	Parent     string `yaml:"parent,omitempty" json:"parent,omitempty"`
	Template   string `yaml:"template" json:"template"`
	Decomposes string `yaml:"decomposes,omitempty" json:"decomposes,omitempty"`
}

// Lifecycle is one `lifecycle.<class>` block (table C-2: "one block per
// class declared above"): the state machine a class's instances move
// through, and the terminal (frozen, 01 §Temporal classes) subset of
// States.
type Lifecycle struct {
	States      []string     `yaml:"states" json:"states"`
	Terminal    []string     `yaml:"terminal" json:"terminal"`
	Transitions []Transition `yaml:"transitions" json:"transitions"`
}

// Transition is one `lifecycle.*.transitions[]` entry: a named verb
// (becomes `verdi <verb>`, table C-2) moving an item from a declared
// state to a declared state, gated by zero or more Obligations.
//
// Obligations is deliberately never `omitempty` on the YAML tag: decode
// must distinguish an ABSENT `obligations:` key (nil slice — a kernel
// violation; every transition must state its rigor, even as zero, guide
// §5.2: "rigor may be zero but never unstated") from an explicit
// `obligations: []` (non-nil, zero-length — a legal, rigor-zero
// transition). gopkg.in/yaml.v3 already makes this distinction on plain
// (non-pointer) slice fields — confirmed empirically before relying on
// it here — so no pointer or separate presence flag is needed.
type Transition struct {
	Verb        string       `yaml:"verb" json:"verb"`
	From        string       `yaml:"from" json:"from"`
	To          string       `yaml:"to" json:"to"`
	Obligations []Obligation `yaml:"obligations" json:"obligations"`
}

// Obligation is one `transitions[].obligations[]` entry (table C-3, the
// obligation-kind catalog): a (Scheme, Kind) pair drawn from the
// kernel-closed catalogs (schemeCatalog/kindCatalog, validate.go), plus
// the two kind-specific parameters — Count (countersign only) and Hook
// (kind: hook only, the name of a declared `gate.check` hook, guide
// §11). Hook DECLARATION and existence-checking against verdi.yaml is
// out of this phase's scope (PLAN.md: "`hooks:` runtime ... Phase 3+");
// this package only enforces that Hook is non-empty when Kind is
// "hook", per the kernel rule.
type Obligation struct {
	Scheme string `yaml:"scheme" json:"scheme"`
	Kind   string `yaml:"kind" json:"kind"`
	Count  int    `yaml:"count,omitempty" json:"count,omitempty"`
	Hook   string `yaml:"hook,omitempty" json:"hook,omitempty"`
}

// Vocabulary is the optional `vocabulary:` block (table C-2): free-text
// display overrides for verb, state, and class ids. Display only —
// changing a value here never changes the id it renames (refs,
// branches, and commands are unaffected) — and, alongside a class's
// Template filename, the frontier's one deliberate escape hatch: a
// Vocabulary change alone never trips checkFrontier.
type Vocabulary struct {
	Verbs   map[string]string `yaml:"verbs,omitempty" json:"verbs,omitempty"`
	States  map[string]string `yaml:"states,omitempty" json:"states,omitempty"`
	Classes map[string]string `yaml:"classes,omitempty" json:"classes,omitempty"`
}

// DisplayState returns the display label for state id (Vocabulary.States
// — a flat map, not nested per class), or id unchanged when no rename
// exists. class is accepted to match the fixed signature Task 6 depends
// on and to leave room for a future per-class override, but v1's own
// vocabulary is flat: only id is consulted this phase.
//
// CALLER CONVENTION (Q2, final fix wave): class is forward-compat — pass
// the artifact's own declared class id whenever the surface has one in
// scope (a spec page passes its spec's class; a wall passes the wall's),
// so a future per-class override lands by changing only this function.
// Pass "" ONLY when no class genuinely exists at the call site — a
// class-less knowledge artifact (ADR, runbook), or a degraded directory
// entry whose content could not be read at all — never as a shortcut
// where the class is sitting in scope.
//
// A nil receiver resolves id to itself (spec/vocabulary-surfaces): every
// consuming surface — CLI verdicts, board, dex, MCP — falls back to bare
// ids when no model was resolved, never panics and never invents a
// second fallback table of its own.
func (m *Model) DisplayState(class, id string) string {
	if m == nil {
		return id
	}
	if v, ok := m.Vocabulary.States[id]; ok && v != "" {
		return v
	}
	return id
}

// DisplayVerb returns the display label for verb id (Vocabulary.Verbs),
// or id unchanged when no rename exists. Nil-receiver-safe exactly like
// DisplayState.
func (m *Model) DisplayVerb(id string) string {
	if m == nil {
		return id
	}
	if v, ok := m.Vocabulary.Verbs[id]; ok && v != "" {
		return v
	}
	return id
}

// DisplayClass returns the display label for class id
// (spec/vocabulary-surfaces' one class-word chain, used verbatim by every
// surface that renders a class word): Vocabulary.Classes[id] when the
// model declares a rename there, else the class's own declared
// Class.Display, else id itself — the same fallback-to-id shape
// DisplayState/DisplayVerb established, one level deeper because a class
// carries two independent sources of a display word instead of one.
// Nil-receiver-safe exactly like both of them.
//
// ENUMERATION RULE (the vocabulary-prose category closure,
// spec/vocabulary-surfaces Outcome: "a rename can never leak partially"):
// every place a surface prints a class word — feature/story, plus the
// spike variant marker — as DISPLAY PROSE (card and stub labels, buttons,
// guide and dialog copy, axis titles and nav labels, metadata-row labels,
// MCP tool descriptions) resolves through this chain, or through
// DisplayClassPlural/Capitalize beside it. The IDENTITY layer never
// resolves: ids in refs, URLs, branch names, JSON/YAML schema fields and
// tool-argument names, enum VALUES (annotation types on the wire, fold
// verdict keys like story.violated/story.eligible), CSS classes, data-*
// attributes, and testids stay bare. "spike" is a variant marker, not a
// model class: it resolves through Vocabulary.Classes exactly like a
// class word and otherwise falls back to itself (it has no Class.Display
// to fall through to) — the pseudo-class treatment
// workbench.applyModelVocabulary established. New display prose that
// speaks a class word obligates a classification against this rule at
// the prose site (the L-M8 posture: taxonomy decided and documented,
// never implicit).
func (m *Model) DisplayClass(id string) string {
	if m == nil {
		return id
	}
	if v, ok := m.Vocabulary.Classes[id]; ok && v != "" {
		return v
	}
	if c, ok := m.Classes[id]; ok && c.Display != "" {
		return c.Display
	}
	return id
}

// DisplayClassPlural returns DisplayClass(id) pluralized, for display
// prose that speaks of the class in the plural ("Implementing stories",
// "a feature never lists its stories", "claimed by 2 spikes").
// Best-effort English pluralization — see pluralizeDisplay — chosen so
// the no-rename fallback reproduces today's hand-written plurals exactly
// ("story" -> "stories", "spike" -> "spikes") and a renamed word gets the
// common regular form ("Change Request" -> "Change Requests"). Display
// only, exactly like DisplayClass; nil-receiver-safe.
func (m *Model) DisplayClassPlural(id string) string {
	return pluralizeDisplay(m.DisplayClass(id))
}

// pluralizeDisplay is the display layer's best-effort English plural:
// consonant-"y" flips to "ies" ("story" -> "stories"), a sibilant ending
// (s/x/z/ch/sh) appends "es", anything else appends "s". Irregular
// plurals in a team's renamed vocabulary get the regular form — a
// disclosed display approximation, never worth a grammar engine (the
// smallest reversible option; the identity layer never pluralizes at
// all).
func pluralizeDisplay(word string) string {
	if word == "" {
		return ""
	}
	r := []rune(word)
	last := r[len(r)-1]
	if (last == 'y' || last == 'Y') && len(r) >= 2 && !isDisplayVowel(r[len(r)-2]) {
		return string(r[:len(r)-1]) + "ies"
	}
	switch last {
	case 's', 'S', 'x', 'X', 'z', 'Z':
		return word + "es"
	case 'h', 'H':
		if len(r) >= 2 {
			switch r[len(r)-2] {
			case 'c', 'C', 's', 'S':
				return word + "es"
			}
		}
	}
	return word + "s"
}

func isDisplayVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return true
	}
	return false
}

// Capitalize returns word with its first rune upper-cased — the one
// display helper for label positions that capitalize the class word
// ("Story" as a metadata-row label, "Stories" as a section heading), kept
// beside the display lookups so every surface capitalizes the same way
// instead of hand-rolling variants. Identity ids are never capitalized —
// this is display plumbing only.
func Capitalize(word string) string {
	if word == "" {
		return ""
	}
	r := []rune(word)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// Article returns the indefinite article ("a" or "an") for display word —
// "an" when the word's first letter is a vowel, "a" otherwise — kept
// beside Capitalize as the one a/an rule every surface composes with a
// display word ("a story", "an Initiative"), replacing the silently-pinned
// "a" that disagreed with vowel-initial renames
// (judged-article-agreement-approximation-undisclosed; cured by code per
// ledger L-M13a(4)). Best-effort English by SPELLING, the same disclosed
// display-approximation posture as pluralizeDisplay: sound-driven
// exceptions — a silent 'h' ("hour"), a consonant-sounding 'u'
// ("unicorn"-shaped renames) — get the spelling-based article. Display
// plumbing only; the identity layer never composes articles.
func Article(word string) string {
	if word == "" {
		return "a"
	}
	if isDisplayVowel([]rune(word)[0]) {
		return "an"
	}
	return "a"
}

// Indefinite returns Article(word) + " " + word — the composed
// "a story" / "an Initiative" form, kept beside Article as the ONE place
// the article-word pair is assembled (Q1, final fix wave: the pair was
// hand-composed at eleven refusal/description sites across cmd/verdi and
// internal/mcpserve; a twelfth hand-compose would eventually disagree
// with the a/an rule). Only for prose positions where the article
// immediately precedes the display word; an article whose head word is
// fixed prose ("a draft <word> spec", "a scheme-prefixed <word> ref")
// agrees with that fixed word and never routes through this. Display
// plumbing only, exactly like Article; Indefinite("") is "a " — a
// degenerate compose no validated model produces (display words are
// never empty), documented rather than special-cased.
func Indefinite(word string) string {
	return Article(word) + " " + word
}

// Digest returns the resolved model's content-address: canonical-JSON
// sha256 (internal/canonjson.Digest — PLAN.md I-18's one home for the
// canonjson-then-sha256-then-"sha256:"+hex tail, spec/shared-homes
// ac-2/dc-2), stamped into every artifact this model produces (ledger
// L-M5, Task 10 — not this task's own consumer) so an archived spec is
// forever interpretable under the model it lived in (guide §5.2).
// Deterministic: canonjson.Marshal sorts object keys and disables HTML
// escaping, so two calls over an equal Model always agree regardless of
// Go's randomized map iteration order.
func (m *Model) Digest() (string, error) {
	return canonjson.Digest(m)
}
