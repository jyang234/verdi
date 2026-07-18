package model

import (
	"strings"
	"testing"
)

// TestDecodeModel_KernelViolations is spec/model-schema ac-1's own proof
// obligation: one committed violation fixture per kernel rule, each
// tripping EXACTLY that rule (asserted by an exact error substring, never
// a generic "got an error").
func TestDecodeModel_KernelViolations(t *testing.T) {
	cases := []struct {
		file       string
		wantSubstr string
	}{
		{"viol-scheme-unknown.yaml", `obligation scheme "vibes" is not one of the kernel schemes`},
		{"viol-kind-unknown.yaml", `obligation kind "bogus-kind" is not one of the kernel kinds`},
		{"viol-obligations-missing.yaml", "obligations list is absent"},
		{"viol-terminal-not-subset.yaml", `terminal state "bogus-state" is not in states`},
		{"viol-terminal-exit.yaml", `from "closed" is a terminal state and admits no outgoing transition`},
		{"viol-state-unreachable.yaml", `state "orphan" is unreachable`},
		{"viol-transition-endpoint-undeclared.yaml", `from "nonexistent-state" is not a declared state`},
		{"viol-parent-unknown.yaml", `parent "nonexistent-class" is not a declared class`},
		{"viol-template-empty.yaml", `class "feature": template must not be empty`},
		{"viol-template-path-escape.yaml", `class "feature": template "../../evil.md" must be a bare filename`},
		{"viol-hook-empty-name.yaml", `kind "hook" requires a non-empty hook name`},
		{"viol-count-non-countersign.yaml", `count is legal only on kind "countersign"`},
		{"viol-duplicate-verb.yaml", `transition verb "accept" is declared more than once`},
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

// TestDecodeModel_Frontier proves dc-1's frontier: a well-formed model
// (every kernel rule holds) that still describes a different transition
// set than canonicalModel is rejected with the ONE pinned error, verbatim —
// whether it ADDS a transition (viol-frontier-structural: an extra `reject`)
// or OMITS one with no masking duplicate (viol-frontier-missing-transition:
// `close` dropped, caught by lifecycleEqual's length arm — the companion to
// judged-frontier-duplicate-verb-bypass's witness, proving the multiset
// refactor kept the plain-omission path a frontier exit 1).
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
			if err.Error() != frontierErrorText {
				t.Fatalf("DecodeModel(%s) error = %q, want the pinned frontier text %q", file, err.Error(), frontierErrorText)
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
// (transitionsEqualAsMultiset) drives the extra accept's count negative and
// fails closed — in either orientation, while canonical still equals itself.
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
