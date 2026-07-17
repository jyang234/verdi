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

import "github.com/jyang234/verdi/internal/canonjson"

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
// Display is the class's own declared label — distinct from
// Vocabulary.Classes, which is the presentation-layer override sitting
// on top of it. This is a disclosed judgment call (spec+plan do not
// spell out the relationship): Display is treated as STRUCTURAL — part
// of the shape checkFrontier compares — while Vocabulary.Classes is the
// frontier-exempt rename layer, mirroring how Template's own FILENAME is
// frontier-exempt but a class's PRESENCE/hierarchy position is not.
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
func (m *Model) DisplayState(class, id string) string {
	if v, ok := m.Vocabulary.States[id]; ok && v != "" {
		return v
	}
	return id
}

// DisplayVerb returns the display label for verb id (Vocabulary.Verbs),
// or id unchanged when no rename exists.
func (m *Model) DisplayVerb(id string) string {
	if v, ok := m.Vocabulary.Verbs[id]; ok && v != "" {
		return v
	}
	return id
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
