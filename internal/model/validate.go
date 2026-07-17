package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
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
// declared class; every transition's obligations list is PRESENT (nil
// means the `obligations:` key itself was absent — distinct from a
// present, empty `[]`); terminal states are drawn from states; every
// transition's from/to names a declared state; every state is
// reachable; every obligation's scheme and kind are drawn from their
// closed catalogs; count is legal only on kind "countersign"; and hook
// is legal only with kind "hook" carrying a non-empty Hook.
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
	return nil
}

// validate checks one lifecycle.<name> block: terminal ⊆ states, every
// transition's obligations-list presence / from / to / obligations, and
// (last, since it depends on the from/to mentions gathered while
// walking transitions) that every state is reachable.
func (lc Lifecycle) validate(name string) error {
	states := make(map[string]bool, len(lc.States))
	for _, s := range lc.States {
		states[s] = true
	}
	terminal := make(map[string]bool, len(lc.Terminal))
	for _, t := range lc.Terminal {
		if !states[t] {
			return fmt.Errorf("model: lifecycle %q: terminal state %q is not in states", name, t)
		}
		terminal[t] = true
	}

	mentioned := make(map[string]bool, len(lc.States))
	for _, tr := range lc.Transitions {
		if tr.Obligations == nil {
			return fmt.Errorf("model: lifecycle %q: transition %q (%s -> %s): obligations list is absent — every transition must state its rigor, even as `obligations: []`", name, tr.Verb, tr.From, tr.To)
		}
		if !states[tr.From] {
			return fmt.Errorf("model: lifecycle %q: transition %q: from %q is not a declared state", name, tr.Verb, tr.From)
		}
		if !states[tr.To] {
			return fmt.Errorf("model: lifecycle %q: transition %q: to %q is not a declared state", name, tr.Verb, tr.To)
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
func (m Model) checkFrontier() error {
	if !classesMatchFrontier(m.Classes, canonicalModel.Classes) {
		return ErrFrontier
	}
	if !lifecyclesMatchFrontier(m.Lifecycle, canonicalModel.Lifecycle) {
		return ErrFrontier
	}
	return nil
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

// lifecyclesMatchFrontier reports whether got and want declare the same
// lifecycle keys with structurally equal Lifecycle values.
func lifecyclesMatchFrontier(got, want map[string]Lifecycle) bool {
	if len(got) != len(want) {
		return false
	}
	for name, wl := range want {
		gl, ok := got[name]
		if !ok || !lifecycleEqual(gl, wl) {
			return false
		}
	}
	return true
}

// lifecycleEqual compares two Lifecycle values structurally: States and
// Terminal as sets (declaration order is not itself a structural
// property — dc-1's own text names "state set ... transition set",
// sets, never order), Transitions keyed by Verb since a verb is a
// transition's identity.
func lifecycleEqual(a, b Lifecycle) bool {
	if !stringsEqualAsSets(a.States, b.States) {
		return false
	}
	if !stringsEqualAsSets(a.Terminal, b.Terminal) {
		return false
	}
	if len(a.Transitions) != len(b.Transitions) {
		return false
	}
	byVerb := make(map[string]Transition, len(b.Transitions))
	for _, t := range b.Transitions {
		byVerb[t.Verb] = t
	}
	for _, ta := range a.Transitions {
		tb, ok := byVerb[ta.Verb]
		if !ok || !transitionEqual(ta, tb) {
			return false
		}
	}
	return true
}

// transitionEqual compares From/To and the full Obligations list.
// Obligations are compared POSITIONALLY (a stricter-than-required
// posture, disclosed): canonicalModel always emits them in one fixed
// order and every fixture this package authors follows it, so ordering
// carries no legitimate reason to differ this phase.
func transitionEqual(a, b Transition) bool {
	if a.From != b.From || a.To != b.To {
		return false
	}
	if len(a.Obligations) != len(b.Obligations) {
		return false
	}
	for i := range a.Obligations {
		if a.Obligations[i] != b.Obligations[i] {
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
