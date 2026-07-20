package model

import (
	"errors"
	"strings"
	"testing"
)

// TestDecodeModel_KernelViolations is spec/model-schema ac-1's own proof
// obligation: one committed violation fixture per kernel rule, each
// tripping EXACTLY that rule (asserted by an exact error substring, never
// a generic "got an error").
//
// guide-claim: 5.2-model-yaml-kernel-digest
// guide-claim: 13-kernel-rules-frontier
func TestDecodeModel_KernelViolations(t *testing.T) {
	cases := []struct {
		file       string
		wantSubstr string
	}{
		{"viol-schema-literal.yaml", `model: schema "verdi.model/v2", want "verdi.model/v1"`},
		{"viol-scheme-unknown.yaml", `obligation scheme "vibes" is not one of the kernel schemes`},
		{"viol-kind-unknown.yaml", `obligation kind "bogus-kind" is not one of the kernel kinds`},
		{"viol-obligations-missing.yaml", "obligations list is absent"},
		{"viol-terminal-not-subset.yaml", `terminal state "bogus-state" is not in states`},
		{"viol-duplicate-state.yaml", `state "draft" is declared more than once in states`},
		{"viol-duplicate-terminal.yaml", `terminal state "accepted-pending-build" is declared more than once in terminal`},
		{"viol-terminal-exit.yaml", `from "closed" is a terminal state and admits no outgoing transition`},
		{"viol-state-unreachable.yaml", `state "orphan" is unreachable`},
		{"viol-transition-endpoint-undeclared.yaml", `from "nonexistent-state" is not a declared state`},
		{"viol-transition-to-undeclared.yaml", `to "nonexistent-state" is not a declared state`},
		{"viol-parent-unknown.yaml", `parent "nonexistent-class" is not a declared class`},
		{"viol-template-empty.yaml", `class "feature": template must not be empty`},
		{"viol-template-path-escape.yaml", `class "feature": template "../../evil.md" must be a bare filename`},
		{"viol-hook-empty-name.yaml", `kind "hook" requires a non-empty hook name`},
		{"viol-count-non-countersign.yaml", `count is legal only on kind "countersign"`},
		{"viol-duplicate-verb.yaml", `transition verb "accept" is declared more than once`},
		{"viol-vocabulary-unknown-key.yaml", `vocabulary: classes key "epic" is not a declared class or the spike pseudo-class`},
		{"viol-vocabulary-empty-value.yaml", `vocabulary: classes key "feature" has an empty rename value`},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			_, err := DecodeModel(readTestdata(t, tc.file))
			if err == nil {
				t.Fatalf("DecodeModel(%s): want error, got nil", tc.file)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("DecodeModel(%s) error = %q, want substring %q", tc.file, err.Error(), tc.wantSubstr)
			}
		})
	}
}

// TestDecodeModel_KernelErrorNamesCatalog proves ac-1's own framing: an
// out-of-catalog scheme/kind names the LEGAL catalog itself, never a bare
// "invalid value" — so an operator learns what IS legal in the same
// breath as learning what is not.
func TestDecodeModel_KernelErrorNamesCatalog(t *testing.T) {
	_, err := DecodeModel(readTestdata(t, "viol-kind-unknown.yaml"))
	if err == nil {
		t.Fatal("DecodeModel: want error, got nil")
	}
	for _, kind := range []string{"author-vouch", "countersign", "gate-pass", "fold-green", "hook", "stubs-reconciled"} {
		if !strings.Contains(err.Error(), kind) {
			t.Fatalf("error = %q, want it to name catalog member %q", err.Error(), kind)
		}
	}
}

// TestModelValidate_VocabularyKeys is judged-vocabulary-keys-unvalidated-
// now-load-bearing's kernel proof: vocabulary keys are load-bearing (every
// display surface resolves through them), so a key naming nothing the
// model declares fails closed — naming the offending key AND the legal
// set, per section — while declared keys and the L-M13 spike pseudo-class
// carve validate clean. Driven over the canonical model with only the
// Vocabulary block varied, so each case isolates exactly the key rule.
func TestModelValidate_VocabularyKeys(t *testing.T) {
	cases := []struct {
		name       string
		vocab      Vocabulary
		wantSubstr string // "" means the model must validate clean
	}{
		{
			"declared keys in every section validate",
			Vocabulary{
				Verbs:   map[string]string{"accept": "Sign off"},
				States:  map[string]string{"accepted-pending-build": "Ready to build"},
				Classes: map[string]string{"feature": "Initiative", "story": "Workstream"},
			},
			"",
		},
		{
			"spike is legal as a classes key (the L-M13 pseudo-class carve)",
			Vocabulary{Classes: map[string]string{"spike": "Timebox"}},
			"",
		},
		{
			"unknown classes key fails closed naming key and legal set",
			Vocabulary{Classes: map[string]string{"epic": "Epic"}},
			`model: vocabulary: classes key "epic" is not a declared class or the spike pseudo-class (legal: feature, spike, story)`,
		},
		{
			"unknown states key fails closed naming key and legal set",
			Vocabulary{States: map[string]string{"ready": "Ready"}},
			`model: vocabulary: states key "ready" is not a declared state in any lifecycle (declared states: accepted-pending-build, closed, draft, superseded)`,
		},
		{
			"unknown verbs key fails closed naming key and legal set",
			Vocabulary{Verbs: map[string]string{"approve": "Approve"}},
			`model: vocabulary: verbs key "approve" is not a declared transition verb in any lifecycle (declared verbs: accept, close)`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := canonicalModel
			m.Vocabulary = tc.vocab
			err := m.Validate()
			if tc.wantSubstr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tc.wantSubstr)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

// TestModelValidate_VocabularyEmptyValueRejected is K6's own proof: an
// empty rename VALUE (as opposed to an unknown KEY, TestModelValidate_
// VocabularyKeys above) is a near-certain typo — DisplayState/DisplayVerb/
// DisplayClass's own fallback chain (model.go) already treats "" as "no
// rename" and silently falls through to the id/Class.Display, so an
// empty value has no visible effect whatsoever, which is precisely why it
// must fail closed at Validate time rather than sit inert. Covers all
// three vocabulary sections; viol-vocabulary-empty-value.yaml (decode_
// test.go's table, via readTestdata above) proves the Classes case
// through the full DecodeModel path, this proves States and Verbs too,
// directly against Model.Validate.
func TestModelValidate_VocabularyEmptyValueRejected(t *testing.T) {
	cases := []struct {
		name       string
		vocab      Vocabulary
		wantSubstr string
	}{
		{
			"empty classes value fails closed naming the key",
			Vocabulary{Classes: map[string]string{"feature": ""}},
			`model: vocabulary: classes key "feature" has an empty rename value`,
		},
		{
			"empty states value fails closed naming the key",
			Vocabulary{States: map[string]string{"accepted-pending-build": ""}},
			`model: vocabulary: states key "accepted-pending-build" has an empty rename value`,
		},
		{
			"empty verbs value fails closed naming the key",
			Vocabulary{Verbs: map[string]string{"accept": ""}},
			`model: vocabulary: verbs key "accept" has an empty rename value`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := canonicalModel
			m.Vocabulary = tc.vocab
			err := m.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tc.wantSubstr)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

// TestDecodeModel_Frontier proves dc-1's frontier: a well-formed model
// (every kernel rule holds) that still describes a different transition
// set than canonicalModel is rejected with the ONE pinned error kept
// VERBATIM as the prefix, plus the additive axis-naming suffix
// (judged-dc1-frontier-error-not-specific). Both fixtures deviate on the
// FEATURE lifecycle's transition set (feature sorts before story) — one
// ADDS a `reject` transition (viol-frontier-structural), the other OMITS
// `close` with no masking duplicate (viol-frontier-missing-transition:
// caught by transitionsAxis's length arm, the companion to judged-frontier-
// duplicate-verb-bypass's witness) — so both name the same axis.
//
// guide-claim: 13-kernel-rules-frontier
func TestDecodeModel_Frontier(t *testing.T) {
	for _, file := range []string{
		"viol-frontier-structural.yaml",
		"viol-frontier-missing-transition.yaml",
	} {
		t.Run(file, func(t *testing.T) {
			_, err := DecodeModel(readTestdata(t, file))
			if err == nil {
				t.Fatalf("DecodeModel(%s): want error, got nil", file)
			}
			// The pinned sentence stays byte-identical as the PREFIX (dc-1's
			// pinned-text contract), honored because the suffix only follows it.
			if !strings.HasPrefix(err.Error(), frontierErrorText) {
				t.Fatalf("DecodeModel(%s) error = %q, want the pinned frontier text as a verbatim prefix %q", file, err.Error(), frontierErrorText)
			}
			wantFull := frontierErrorText + `: lifecycle "feature" transition set diverges`
			if err.Error() != wantFull {
				t.Fatalf("DecodeModel(%s) error = %q, want prefix + axis suffix %q", file, err.Error(), wantFull)
			}
			// The suffix WRAPS the sentinel (%w), so errors.Is still identifies
			// it — cmd/verdi/model.go's exit-1 path relies on this surviving
			// store.Open's own wrapping.
			if !errors.Is(err, ErrFrontier) {
				t.Fatalf("DecodeModel(%s): errors.Is(err, ErrFrontier) = false, want true", file)
			}
		})
	}
}

// TestLifecycleEqual_DuplicateVerbCannotMaskMissing is judged-frontier-
// duplicate-verb-bypass's defense-in-depth proof, driven at the frontier
// compare DIRECTLY: the kernel's own duplicate-verb rule now rejects such a
// manifest at Validate time, so DecodeModel never hands lifecycleEqual a
// duplicate — this exercises the compare on a hand-built value to prove it
// is robust even if one ever slipped that gate. A lifecycle listing `accept`
// twice and omitting `close` must NOT compare equal to canonical's
// [accept, close]: staying length-2 while each accept matched canonical's one
// accept, the pre-fix one-directional verb-map compare returned true and let
// a whole missing transition slip the frontier. The verb-keyed multiset
// (transitionsAxis, reached through lifecycleEqual) drives the extra accept's
// count negative and fails closed — in either orientation, while canonical
// still equals itself.
func TestLifecycleEqual_DuplicateVerbCannotMaskMissing(t *testing.T) {
	canon := canonicalSpecLifecycle()
	accept := canon.Transitions[0] // the `accept` transition, verbatim
	dup := Lifecycle{
		States:      canon.States,
		Terminal:    canon.Terminal,
		Transitions: []Transition{accept, accept}, // accept x2, no close
	}
	if lifecycleEqual(dup, canon) {
		t.Fatal("lifecycleEqual(accept×2-no-close, canonical) = true, want false: a duplicate verb must not mask canonical's missing `close` (judged-frontier-duplicate-verb-bypass)")
	}
	if lifecycleEqual(canon, dup) {
		t.Fatal("lifecycleEqual(canonical, accept×2-no-close) = true, want false (the compare must fail closed in either orientation)")
	}
	if !lifecycleEqual(canon, canonicalSpecLifecycle()) {
		t.Fatal("lifecycleEqual(canonical, canonical) = false, want true (positive control: an unchanged lifecycle still compares equal)")
	}
}

// TestDecodeModel_VocabRenamePassesFrontier proves the frontier's two
// named exceptions actually work: vocabulary renames and per-class
// template filename changes are NOT structural deviations.
func TestDecodeModel_VocabRenamePassesFrontier(t *testing.T) {
	m, err := DecodeModel(readTestdata(t, "vocab-rename.yaml"))
	if err != nil {
		t.Fatalf("DecodeModel(vocab-rename.yaml): %v", err)
	}
	if got := m.Classes["feature"].Template; got != "custom-feature.md" {
		t.Fatalf("Classes[feature].Template = %q, want custom-feature.md", got)
	}
	if got := m.DisplayVerb("accept"); got != "Sign off" {
		t.Fatalf("DisplayVerb(accept) = %q, want %q", got, "Sign off")
	}
}

// TestDecodeModel_ReorderedObligationsPassesFrontier proves the frontier
// compares a transition's obligations as a set, not positionally
// (judged-frontier-obligations-positional): a manifest canonical but for the
// ORDER of the close transition's two obligations decodes clean.
func TestDecodeModel_ReorderedObligationsPassesFrontier(t *testing.T) {
	if _, err := DecodeModel(readTestdata(t, "obligations-reordered.yaml")); err != nil {
		t.Fatalf("DecodeModel(obligations-reordered.yaml): %v", err)
	}
}

// TestDecodeModel_DisplayRenamePassesFrontier proves the frontier exempts a
// class's Display label (judged-frontier-display-structural): a manifest
// canonical in every structural respect but for a changed classes.*.display
// decodes clean — Display is presentation, not part of the class set dc-1's
// frontier is drawn over.
func TestDecodeModel_DisplayRenamePassesFrontier(t *testing.T) {
	m, err := DecodeModel(readTestdata(t, "display-rename.yaml"))
	if err != nil {
		t.Fatalf("DecodeModel(display-rename.yaml): %v", err)
	}
	if got := m.Classes["feature"].Display; got != "Initiative" {
		t.Fatalf("Classes[feature].Display = %q, want Initiative", got)
	}
}

// TestCanonicalModel_SelfValidates proves the Go-literal canonicalModel
// (canonical.go) is itself kernel-well-formed and trivially matches its
// own frontier — a sanity check on checkFrontier's comparison logic
// independent of any YAML round-trip.
func TestCanonicalModel_SelfValidates(t *testing.T) {
	if err := canonicalModel.Validate(); err != nil {
		t.Fatalf("canonicalModel.Validate(): %v", err)
	}
	if err := canonicalModel.checkFrontier(); err != nil {
		t.Fatalf("canonicalModel.checkFrontier(): %v", err)
	}
}

// TestCheckFrontier_DisplayChangeExempt substantiates the adjudicated
// design choice (model.go's Class doc comment, judged-frontier-display-
// structural): a class's Display is presentation, frontier-EXEMPT exactly
// like its Template filename and Vocabulary.Classes — changing it alone
// (with nothing else different) must NOT trip the frontier, since dc-1
// draws the frontier over the state/transition/class/obligation sets and
// a display-label change alters none of them.
func TestCheckFrontier_DisplayChangeExempt(t *testing.T) {
	m := canonicalModel
	feature := m.Classes["feature"]
	feature.Display = "Initiative" // presentation-only change, nothing else
	m.Classes = map[string]Class{
		"feature": feature,
		"story":   m.Classes["story"],
	}

	if err := m.Validate(); err != nil {
		t.Fatalf("Validate(): %v (test setup should stay kernel-well-formed)", err)
	}
	if err := m.checkFrontier(); err != nil {
		t.Fatalf("checkFrontier(): want nil for a display-label-only change (frontier-exempt), got %v", err)
	}
}

// TestCheckFrontier_NamesDivergentAxis is judged-dc1-frontier-error-not-
// specific's proof: checkFrontier's additive suffix names the FIRST divergent
// structural axis — "class set", or a NAMED lifecycle's state/terminal/
// transition/obligation set — so a rejected manifest says WHICH axis moved,
// not merely that one did. Each case is deviant on exactly one axis and drives
// checkFrontier's compare DIRECTLY (the pass DecodeModel runs after Validate,
// so a case need not itself be Validate-well-formed); the pinned
// frontierErrorText stays the verbatim prefix and errors.Is holds throughout.
func TestCheckFrontier_NamesDivergentAxis(t *testing.T) {
	// modelWithFeature swaps lc in as the feature lifecycle inside a FRESH
	// Lifecycle map, so canonicalModel (a package var) is never mutated; the
	// story lifecycle stays canonical, so feature — sorting first — is the
	// lifecycle every case reports.
	modelWithFeature := func(lc Lifecycle) Model {
		m := canonicalModel
		m.Lifecycle = map[string]Lifecycle{"feature": lc, "story": canonicalModel.Lifecycle["story"]}
		return m
	}

	// class set: feature's Decomposes cleared — a hierarchy-position change,
	// the structural half of a Class (Display/Template are frontier-exempt).
	classSet := canonicalModel
	feature := canonicalModel.Classes["feature"]
	feature.Decomposes = ""
	classSet.Classes = map[string]Class{"feature": feature, "story": canonicalModel.Classes["story"]}

	stateSet := canonicalSpecLifecycle()
	stateSet.States = []string{"draft", "accepted-pending-build", "closed", "superseded", "reopened"}

	terminalSet := canonicalSpecLifecycle()
	terminalSet.Terminal = []string{"closed"} // superseded dropped

	transitionSet := canonicalSpecLifecycle()
	transitionSet.Transitions = transitionSet.Transitions[:1] // close dropped

	// obligation set: same verbs, same from/to, but close's obligation set
	// swapped — the axis transitionsAxis distinguishes from a transition-set
	// change.
	obligationSet := canonicalSpecLifecycle()
	obligationSet.Transitions = []Transition{
		obligationSet.Transitions[0], // accept, unchanged
		{Verb: "close", From: "accepted-pending-build", To: "closed",
			Obligations: []Obligation{{Scheme: "attestation", Kind: "author-vouch"}}},
	}

	cases := []struct {
		name     string
		model    Model
		wantAxis string
	}{
		{"class set", classSet, "class set"},
		{"state set", modelWithFeature(stateSet), `lifecycle "feature" state set`},
		{"terminal set", modelWithFeature(terminalSet), `lifecycle "feature" terminal set`},
		{"transition set", modelWithFeature(transitionSet), `lifecycle "feature" transition set`},
		{"obligation set", modelWithFeature(obligationSet), `lifecycle "feature" obligation set`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.model.checkFrontier()
			if err == nil {
				t.Fatalf("checkFrontier() = nil, want a frontier error naming %q", tc.wantAxis)
			}
			want := frontierErrorText + ": " + tc.wantAxis + " diverges"
			if err.Error() != want {
				t.Fatalf("checkFrontier() = %q, want %q", err.Error(), want)
			}
			if !errors.Is(err, ErrFrontier) {
				t.Fatalf("errors.Is(err, ErrFrontier) = false, want true (the suffix must wrap the sentinel)")
			}
		})
	}
}
