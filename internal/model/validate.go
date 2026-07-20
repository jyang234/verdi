package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// schemeCatalog is Obligation.Scheme's closed, kernel-fixed catalog
// (guide §5.2 / table C-2: "one of exactly three, fixed by the
// kernel"). Extending it is a frontier/spec round (guide Appendix C's
// "kernel enum" source class), never a per-store choice.
var schemeCatalog = map[string]bool{
	"static":      true,
	"behavioral":  true,
	"attestation": true,
}

// kindCatalog is Obligation.Kind's closed, kernel-fixed catalog (guide
// table C-3; spec/model-schema ac-1's own six-item list): each kind is
// a check the binary knows how to perform, which is why the catalog is
// closed. `hook` is the sole extension point (behavioral/hook plus a
// declared verdi.yaml gate.check hook) — checking a hook's DECLARED
// EXISTENCE is out of this phase's scope (PLAN.md: "`hooks:` runtime
// ... Phase 3+"); this package only checks Kind itself is one of these
// six strings, and (below) that a "hook" kind carries a non-empty name.
var kindCatalog = map[string]bool{
	"author-vouch":     true,
	"countersign":      true,
	"gate-pass":        true,
	"fold-green":       true,
	"hook":             true,
	"stubs-reconciled": true,
}

// frontierErrorText is Task 5's one pinned error, verbatim (dc-1: v1's
// smallest reversible slice — the manifest DESCRIBES the model; it does
// not yet let the shape move). Printed exactly, never paraphrased,
// whenever a well-formed (Validate-clean) model still describes a
// different state, transition, class, or obligation set than today's
// canonicalModel.
const frontierErrorText = "structural model configuration is behind the frontier (verdi.model/v1 accepts the canonical model with vocabulary/template changes only)"

// ErrFrontier is the sentinel checkFrontier returns. spec/model-schema
// ac-3 (frozen, verbatim in both the story spec and its own obligation
// doc) keys `verdi model check`'s 0/1/2 exit discipline on EXACTLY this
// one condition being distinct from every other DecodeModel failure:
// "exit 1 with the pinned frontier error text ... on a structurally
// deviant manifest; and exit 2 on operational trouble (a missing store,
// an unreadable OR UNDECODABLE manifest)" — a kernel validation rule
// violation (validate.go's Validate) is grouped with "undecodable" and
// so is exit 2, NOT exit 1, despite this plan's own Task 7 prose
// saying "exit 1 on validation/frontier failure" (a plan/spec conflict;
// spec+obligation win per this build's own precedence rule — reported
// in the phase report). cmd/verdi/model.go uses errors.Is against this
// exact value — which survives store.Open's %w wrapping — to tell the
// two apart.
var ErrFrontier = errors.New(frontierErrorText)

// Validate checks the kernel rules that make m well-formed regardless of
// which concrete lifecycle it describes (spec/model-schema ac-1, the
// Outcome section's own rule list): the schema literal; every class
// carries a non-empty template; every class's parent (if any) names a
// declared class; no state id is declared twice within one lifecycle's
// states, and no state id is declared twice within its terminal (K3);
// every transition's obligations list is PRESENT (nil means the
// `obligations:` key itself was absent — distinct from a present, empty
// `[]`); terminal states are drawn from states and admit no outgoing
// transition (terminal ⇒ freeze); every transition's from/to names a
// declared state; no verb is declared by two transitions within one
// lifecycle (a verb is a transition's identity — the frontier compare
// keys on it — judged-frontier-duplicate-verb-bypass); every state is
// reachable; every obligation's scheme and kind are drawn from their
// closed catalogs; count is legal only on kind "countersign"; hook is
// legal only with kind "hook" carrying a non-empty Hook; and every
// vocabulary key names a declared referent (validateVocabulary — the
// rename layer may only rename things the model actually declares).
//
// Fails on the first violation found (mirroring store.Manifest.Validate's
// own fail-fast posture), walking classes and lifecycles in sorted key
// order so which violation is reported is deterministic across runs
// even though Classes/Lifecycle are Go maps.
func (m Model) Validate() error {
	if m.Schema != modelSchema {
		return fmt.Errorf("model: schema %q, want %q", m.Schema, modelSchema)
	}

	for _, name := range sortedKeys(m.Classes) {
		c := m.Classes[name]
		if c.Template == "" {
			return fmt.Errorf("model: class %q: template must not be empty", name)
		}
		// A class's template resolves under .verdi/templates/<class>.md
		// (spec/scaffold-templates outcome / ac-3): a bare filename inside the
		// store's sanctioned override directory. Template is deliberately
		// frontier-EXEMPT (a store may rename it freely, checkFrontier), but
		// that exemption is over the FILENAME, never a license to escape the
		// directory — a separator-carrying, absolute, or . / .. value would
		// resolve a file outside .verdi/templates/ and the store's
		// committed-zone trust boundary, which no frontier admits (the
		// canonical model's own template values are bare filenames). Fail
		// closed here, the house posture (judged-template-filename-escapes-
		// templates-dir); LoadTemplate re-checks the same invariant as
		// defense-in-depth.
		if !artifact.IsBareFilename(c.Template) {
			return fmt.Errorf("model: class %q: template %q must be a bare filename under .verdi/templates/ (no path separator, absolute path, or . / ..)", name, c.Template)
		}
		if c.Parent != "" {
			if _, ok := m.Classes[c.Parent]; !ok {
				return fmt.Errorf("model: class %q: parent %q is not a declared class", name, c.Parent)
			}
		}
	}

	for _, name := range sortedKeys(m.Lifecycle) {
		if err := m.Lifecycle[name].validate(name); err != nil {
			return err
		}
	}
	return m.validateVocabulary()
}

// vocabularySpikePseudoClass is the ONE non-class id legal as a
// Vocabulary.Classes key: "spike" (ledger L-M13, rule 3 — ratified at the
// vocabulary-surfaces closure): the variant marker is vocabulary-
// addressable through Vocabulary.Classes exactly like a class word, "an
// explicit, deliberate widening of the classes rename map, to be carved
// as the sole exception when vocabulary-key validation lands". This is
// that carve, taken deliberately while cheap.
const vocabularySpikePseudoClass = "spike"

// validateVocabulary checks that every vocabulary key names a declared
// referent — the vocabulary keys are load-bearing now that every display
// surface resolves through them (spec/vocabulary-surfaces), so a typo'd
// key must fail closed at decode time, never sit silently inert:
//
//   - every Vocabulary.States key is a declared state in SOME lifecycle
//     (States is a flat map, not nested per class — model.go's
//     DisplayState — so any lifecycle's declaration legitimizes the key);
//   - every Vocabulary.Verbs key is a declared transition verb in some
//     lifecycle (the same flat-map reasoning);
//   - every Vocabulary.Classes key is a declared class OR the literal
//     "spike" (vocabularySpikePseudoClass — the L-M13-ratified
//     pseudo-class carve).
//
// Each violation names the offending key AND the legal set (the same
// operator courtesy the scheme/kind catalog errors extend: learn what IS
// legal in the same breath as learning what is not). Maps are walked in
// sorted key order — classes, then states, then verbs — so which
// violation is reported first is deterministic across runs.
func (m Model) validateVocabulary() error {
	legalClasses := make(map[string]bool, len(m.Classes)+1)
	for name := range m.Classes {
		legalClasses[name] = true
	}
	legalClasses[vocabularySpikePseudoClass] = true

	legalStates := make(map[string]bool)
	legalVerbs := make(map[string]bool)
	for _, lc := range m.Lifecycle {
		for _, s := range lc.States {
			legalStates[s] = true
		}
		for _, tr := range lc.Transitions {
			legalVerbs[tr.Verb] = true
		}
	}

	for _, key := range sortedKeys(m.Vocabulary.Classes) {
		if !legalClasses[key] {
			return fmt.Errorf("model: vocabulary: classes key %q is not a declared class or the spike pseudo-class (legal: %s)", key, catalogList(legalClasses))
		}
	}
	for _, key := range sortedKeys(m.Vocabulary.States) {
		if !legalStates[key] {
			return fmt.Errorf("model: vocabulary: states key %q is not a declared state in any lifecycle (declared states: %s)", key, catalogList(legalStates))
		}
	}
	for _, key := range sortedKeys(m.Vocabulary.Verbs) {
		if !legalVerbs[key] {
			return fmt.Errorf("model: vocabulary: verbs key %q is not a declared transition verb in any lifecycle (declared verbs: %s)", key, catalogList(legalVerbs))
		}
	}
	return nil
}

// validate checks one lifecycle.<name> block: no state id is declared
// twice within `states:`, and no state id is declared twice within
// `terminal:` (K3 — a duplicate is internally contradictory, silently
// inert against the map[string]bool both build, and could otherwise mask
// a genuinely missing id the same way a duplicate transition verb could
// mask a missing transition, judged-frontier-duplicate-verb-bypass's own
// reasoning carried to these two lists); terminal ⊆ states; no verb is
// bound by two transitions within this lifecycle (a verb is a
// transition's identity, judged-frontier-duplicate-verb-bypass — a
// duplicate is internally contradictory AND could otherwise mask a missing
// transition at the frontier compare), every transition's obligations-list
// presence / from / to / obligations, that no transition departs a
// terminal state (terminal ⇒ freeze — guide C-2, judged-terminal-freeze-
// not-kernel: a frozen state admits no exit), and (last, since it depends
// on the from/to mentions gathered while walking transitions) that every
// state is reachable.
func (lc Lifecycle) validate(name string) error {
	states := make(map[string]bool, len(lc.States))
	for _, s := range lc.States {
		if states[s] {
			return fmt.Errorf("model: lifecycle %q: state %q is declared more than once in states", name, s)
		}
		states[s] = true
	}
	terminal := make(map[string]bool, len(lc.Terminal))
	for _, t := range lc.Terminal {
		if !states[t] {
			return fmt.Errorf("model: lifecycle %q: terminal state %q is not in states", name, t)
		}
		if terminal[t] {
			return fmt.Errorf("model: lifecycle %q: terminal state %q is declared more than once in terminal", name, t)
		}
		terminal[t] = true
	}

	seenVerbs := make(map[string]bool, len(lc.Transitions))
	mentioned := make(map[string]bool, len(lc.States))
	for _, tr := range lc.Transitions {
		if seenVerbs[tr.Verb] {
			return fmt.Errorf("model: lifecycle %q: transition verb %q is declared more than once — a verb is a transition's identity, so a lifecycle may declare each verb at most once", name, tr.Verb)
		}
		seenVerbs[tr.Verb] = true
		if tr.Obligations == nil {
			return fmt.Errorf("model: lifecycle %q: transition %q (%s -> %s): obligations list is absent — every transition must state its rigor, even as `obligations: []`", name, tr.Verb, tr.From, tr.To)
		}
		if !states[tr.From] {
			return fmt.Errorf("model: lifecycle %q: transition %q: from %q is not a declared state", name, tr.Verb, tr.From)
		}
		if !states[tr.To] {
			return fmt.Errorf("model: lifecycle %q: transition %q: to %q is not a declared state", name, tr.Verb, tr.To)
		}
		if terminal[tr.From] {
			return fmt.Errorf("model: lifecycle %q: transition %q: from %q is a terminal state and admits no outgoing transition (guide C-2: terminal states freeze)", name, tr.Verb, tr.From)
		}
		mentioned[tr.From] = true
		mentioned[tr.To] = true

		for _, ob := range tr.Obligations {
			if err := ob.validate(name, tr.Verb); err != nil {
				return err
			}
		}
	}

	// Reachability (dc-1's own frontier example, and the guide's §5.2
	// worked epic/task model, both leave `superseded` un-mentioned by
	// any modeled transition): a state is reachable when it is either
	// terminal (freezing is itself the kernel's own way in, guide §8.3
	// — supersession flips status as a side effect of a DIFFERENT
	// spec's accept, never a modeled verb-transition of its own) or
	// mentioned as some transition's from/to.
	for _, s := range lc.States {
		if terminal[s] || mentioned[s] {
			continue
		}
		return fmt.Errorf("model: lifecycle %q: state %q is unreachable: it is not terminal and does not appear as any transition's from/to", name, s)
	}
	return nil
}

// validate checks one obligation: scheme and kind each drawn from their
// own closed catalog, count legal only on kind "countersign", and a
// non-empty Hook required (only) when kind is "hook".
func (ob Obligation) validate(lifecycleName, verb string) error {
	if !schemeCatalog[ob.Scheme] {
		return fmt.Errorf("model: lifecycle %q: transition %q: obligation scheme %q is not one of the kernel schemes (%s)", lifecycleName, verb, ob.Scheme, catalogList(schemeCatalog))
	}
	if !kindCatalog[ob.Kind] {
		return fmt.Errorf("model: lifecycle %q: transition %q: obligation kind %q is not one of the kernel kinds (%s)", lifecycleName, verb, ob.Kind, catalogList(kindCatalog))
	}
	if ob.Count != 0 && ob.Kind != "countersign" {
		return fmt.Errorf("model: lifecycle %q: transition %q: obligation count is legal only on kind \"countersign\", got kind %q", lifecycleName, verb, ob.Kind)
	}
	if ob.Kind == "hook" && ob.Hook == "" {
		return fmt.Errorf("model: lifecycle %q: transition %q: obligation kind \"hook\" requires a non-empty hook name", lifecycleName, verb)
	}
	return nil
}

// checkFrontier rejects a well-formed (Validate-clean) model that still
// describes a different state, transition, class, or obligation set
// than canonicalModel — stage 1's frontier (dc-1): v1 DESCRIBES the
// model; it does not yet let the shape move. Vocabulary is never
// compared (display-only, guide §5.2: "renames are safe at any time");
// a Class's Template filename and its Display label are likewise never
// compared (both presentation, judged-frontier-display-structural) —
// every other Class/Lifecycle field is.
//
// The returned error keeps the ONE pinned frontierErrorText verbatim as
// its PREFIX (dc-1's pinned-text contract) and WRAPS the ErrFrontier
// sentinel with %w — so errors.Is still identifies it through store.Open's
// own wrapping (cmd/verdi/model.go's exit-1 decision) — then names the
// first divergent structural axis as an ADDITIVE suffix
// (judged-dc1-frontier-error-not-specific): an operator staring at a
// rejected manifest learns WHICH of the class/state/terminal/transition/
// obligation sets moved, not merely that something did.
func (m Model) checkFrontier() error {
	if axis := m.frontierAxis(); axis != "" {
		return fmt.Errorf("%w: %s diverges", ErrFrontier, axis)
	}
	return nil
}

// frontierAxis names the first structural axis on which m diverges from
// canonicalModel, or "" when m sits exactly at the frontier. The axis
// vocabulary is dc-1's own — "class set", then (per lifecycle, the
// lifecycle named) "state set", "terminal set", "transition set",
// "obligation set" — walked in a FIXED order so which axis a given deviant
// manifest reports is deterministic across runs (CLAUDE.md: deterministic
// outputs): class set first, then lifecycles in sorted key order over the
// UNION of m's and canonical's keys (so a lifecycle present on only one
// side still diverges against the other's zero value), each lifecycle's
// own fields in state -> terminal -> transition -> obligation order. This
// rejects EXACTLY the set of models classesMatchFrontier plus the former
// lifecyclesMatchFrontier rejected — the suffix is additive, never a change
// to WHAT counts as a frontier violation.
func (m Model) frontierAxis() string {
	if !classesMatchFrontier(m.Classes, canonicalModel.Classes) {
		return "class set"
	}
	for _, name := range sortedKeys(unionLifecycleKeys(m.Lifecycle, canonicalModel.Lifecycle)) {
		if axis := lifecycleAxis(m.Lifecycle[name], canonicalModel.Lifecycle[name]); axis != "" {
			return fmt.Sprintf("lifecycle %q %s", name, axis)
		}
	}
	return ""
}

// classesMatchFrontier reports whether got and want declare the same
// class ids with the same Parent/Decomposes. Two Class fields are
// deliberately excluded from the frontier: Template (the named
// template-filename exception) and Display. Display is presentation — a
// class's human label, exactly like Vocabulary.Classes is the rename
// layer sitting above it — and dc-1 draws the frontier over the "state
// set, transition set, class set, obligation set", none of which a
// display-label change alters (judged-frontier-display-structural: a
// changed label is not a different class set). Only a class's PRESENCE
// and its hierarchy position (Parent/Decomposes) are structural.
func classesMatchFrontier(got, want map[string]Class) bool {
	if len(got) != len(want) {
		return false
	}
	for name, wc := range want {
		gc, ok := got[name]
		if !ok {
			return false
		}
		if gc.Parent != wc.Parent || gc.Decomposes != wc.Decomposes {
			return false
		}
	}
	return true
}

// unionLifecycleKeys returns the set of lifecycle keys declared by either
// a or b — the domain frontierAxis walks, so a lifecycle present on only
// one side is compared against the other's zero Lifecycle (an empty state
// set) and surfaces as that lifecycle's own first-divergent axis rather
// than dropping out of an intersection-only walk.
func unionLifecycleKeys(a, b map[string]Lifecycle) map[string]struct{} {
	keys := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	return keys
}

// lifecycleEqual reports whether two Lifecycle values are structurally
// equal — the boolean the frontier compare is built on, kept as the name
// this package's tests exercise directly. It delegates to lifecycleAxis so
// the compare logic lives in exactly one place: equal precisely when no
// axis diverges.
func lifecycleEqual(a, b Lifecycle) bool {
	return lifecycleAxis(a, b) == ""
}

// lifecycleAxis names the first structural axis on which got diverges from
// want, or "" when they are structurally equal: States then Terminal
// compared as SETS (declaration order is not itself structural — dc-1's own
// text names "state set ... transition set", sets, never order), then
// Transitions as a verb-keyed MULTISET (transitionsAxis) since a verb is a
// transition's identity. Returns the BARE axis name ("state set" and so
// on); the owning lifecycle is named by the caller, frontierAxis.
func lifecycleAxis(got, want Lifecycle) string {
	if !stringsEqualAsSets(got.States, want.States) {
		return "state set"
	}
	if !stringsEqualAsSets(got.Terminal, want.Terminal) {
		return "terminal set"
	}
	return transitionsAxis(got.Transitions, want.Transitions)
}

// transitionsAxis names how got's transitions diverge from want's, or ""
// when they match: "transition set" when the verb MULTISET differs or a
// shared verb's From/To differs, "obligation set" when every verb and
// From/To matches but some shared verb's obligation set differs. A verb is
// a transition's identity, so transitions are keyed by Verb with
// MULTIPLICITY (counts matter, not mere membership): a verb whose got-count
// exceeds its want-count drives the running count negative and diverges, so
// a manifest-side DUPLICATE verb can no longer mask a canonical verb the
// manifest OMITS — [accept, accept] and [accept, close] both stay length 2
// (judged-frontier-duplicate-verb-bypass). The kernel's own duplicate-verb
// rule (Lifecycle.validate) already rejects a manifest-side duplicate at
// Validate time and the canonical side never carries one, so the byVerb
// lookup keying each verb to a single transition loses nothing; it is
// defense-in-depth should a duplicate ever reach here. Obligations are
// compared order-insensitively (obligationsEqualAsSets): dc-1 draws the
// frontier over the "obligation set", the same set language it uses for
// States/Terminal, so a reordered obligation list is the identical set, not
// a deviation (judged-frontier-obligations-positional). A verb/From/To
// difference OUTRANKS an obligation-only one — the more structural axis
// wins regardless of slice position, so the reported axis is a function of
// the sets, not their order.
func transitionsAxis(got, want []Transition) string {
	if len(got) != len(want) {
		return "transition set"
	}
	counts := make(map[string]int, len(want))
	byVerb := make(map[string]Transition, len(want))
	for _, t := range want {
		counts[t.Verb]++
		byVerb[t.Verb] = t
	}
	obligationDiverged := false
	for _, t := range got {
		counts[t.Verb]--
		if counts[t.Verb] < 0 {
			return "transition set"
		}
		wt, ok := byVerb[t.Verb]
		if !ok || wt.From != t.From || wt.To != t.To {
			return "transition set"
		}
		if !obligationsEqualAsSets(wt.Obligations, t.Obligations) {
			obligationDiverged = true
		}
	}
	if obligationDiverged {
		return "obligation set"
	}
	return ""
}

// obligationsEqualAsSets reports whether a and b hold the same obligations
// regardless of order (multiset equality — a dropped duplicate is still a
// real difference, so counts matter, not mere membership).
func obligationsEqualAsSets(a, b []Obligation) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[Obligation]int, len(a))
	for _, ob := range a {
		counts[ob]++
	}
	for _, ob := range b {
		counts[ob]--
		if counts[ob] < 0 {
			return false
		}
	}
	return true
}

// stringsEqualAsSets reports whether a and b contain the same strings,
// ignoring order and duplicates.
func stringsEqualAsSets(a, b []string) bool {
	as := make(map[string]bool, len(a))
	for _, s := range a {
		as[s] = true
	}
	bs := make(map[string]bool, len(b))
	for _, s := range b {
		bs[s] = true
	}
	if len(as) != len(bs) {
		return false
	}
	for s := range as {
		if !bs[s] {
			return false
		}
	}
	return true
}

// catalogList renders a sorted catalog map as a comma-joined string for
// error messages (ac-1: an operator hand-editing a manifest must learn
// what IS legal in the same breath as learning what is not — never a
// bare "invalid value").
func catalogList(catalog map[string]bool) string {
	keys := make([]string, 0, len(catalog))
	for k := range catalog {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// sortedKeys returns m's keys in sorted order, so callers walking a Go
// map for validation report deterministic errors across runs.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
