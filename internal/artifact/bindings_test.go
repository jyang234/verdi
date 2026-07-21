package artifact

import (
	"strings"
	"testing"
)

const validBindingsYAML = `schema: verdi.bindings/v1
spec: spec/stale-decline
bindings:
  - { producer: audit-before-publish, kind: static, acs: [ac-1, ac-2] }
  - { producer: refund-flow, kind: behavioral, acs: [ac-3] }
`

func TestDecodeBindings_Happy(t *testing.T) {
	got, err := DecodeBindings([]byte(validBindingsYAML))
	if err != nil {
		t.Fatalf("DecodeBindings: %v", err)
	}
	if got.Spec != "spec/stale-decline" {
		t.Fatalf("Spec = %q, want %q", got.Spec, "spec/stale-decline")
	}
	if len(got.Bindings) != 2 {
		t.Fatalf("got %d bindings, want 2", len(got.Bindings))
	}
	if got.Bindings[0].Producer != "audit-before-publish" || got.Bindings[0].Kind != EvidenceStatic {
		t.Fatalf("bindings[0] = %+v, unexpected", got.Bindings[0])
	}
	if len(got.Bindings[0].ACs) != 2 || got.Bindings[0].ACs[0] != "ac-1" || got.Bindings[0].ACs[1] != "ac-2" {
		t.Fatalf("bindings[0].ACs = %v, want [ac-1 ac-2]", got.Bindings[0].ACs)
	}
}

// TestDecodeBindings_FragmentQualifiedAC proves a Binding.ACs entry may be a
// spec/<name>#<ac-id> fragment ref naming a DIFFERENT spec than
// Bindings.Spec (round-6, spec/close-verb ac-3/dc-1: 03 §Declarations and
// binding's object-fragment disambiguation, generalized to any other spec,
// not only "a story and its feature") — needed so the self-hosted evidence
// producer's one bindings.yaml can bind several self-hosted stories' ACs.
func TestDecodeBindings_FragmentQualifiedAC(t *testing.T) {
	data := "schema: verdi.bindings/v1\nspec: spec/close-verb\nbindings:\n  - { producer: verdi-verify-behavioral, kind: behavioral, acs: [ac-1, \"spec/remote-and-ci#ac-1\"] }\n"
	got, err := DecodeBindings([]byte(data))
	if err != nil {
		t.Fatalf("DecodeBindings: %v", err)
	}
	if len(got.Bindings) != 1 || len(got.Bindings[0].ACs) != 2 {
		t.Fatalf("got %+v, want one binding with two ACs", got)
	}
	if got.Bindings[0].ACs[1] != "spec/remote-and-ci#ac-1" {
		t.Fatalf("ACs[1] = %q, want the fragment ref preserved verbatim", got.Bindings[0].ACs[1])
	}
}

func TestResolveBindingAC(t *testing.T) {
	t.Run("bare ac resolves against the default spec", func(t *testing.T) {
		specRef, acID, err := ResolveBindingAC("spec/close-verb", "ac-3")
		if err != nil {
			t.Fatalf("ResolveBindingAC: %v", err)
		}
		if specRef != "spec/close-verb" || acID != "ac-3" {
			t.Fatalf("got (%q, %q), want (spec/close-verb, ac-3)", specRef, acID)
		}
	})

	t.Run("fragment ref resolves against its own named spec, ignoring the default", func(t *testing.T) {
		specRef, acID, err := ResolveBindingAC("spec/close-verb", "spec/remote-and-ci#ac-1")
		if err != nil {
			t.Fatalf("ResolveBindingAC: %v", err)
		}
		if specRef != "spec/remote-and-ci" || acID != "ac-1" {
			t.Fatalf("got (%q, %q), want (spec/remote-and-ci, ac-1)", specRef, acID)
		}
	})

	t.Run("malformed entry is rejected", func(t *testing.T) {
		if _, _, err := ResolveBindingAC("spec/close-verb", "not-an-ac-or-fragment"); err == nil {
			t.Fatal("ResolveBindingAC(malformed): want error, got nil")
		}
	})
}

// TestResolveBindingAC_PinnedFragment_FailsClosed is spec/ritual-traps
// judged-ac4-pinned-fragment-entry-silently-unpinned: a fragment entry that
// ALSO pins a revision (spec/<name>@<commit>#<ac-id>) must fail closed here
// rather than silently drop the pin and resolve against the current spec.
// Before this fix ResolveBindingAC returned (spec/<name>, <ac-id>, nil) —
// discarding the commit entirely — so both callers (internal/lint VL-003 and
// cmd/verdi/selfevidence) validated a pinned entry's AC against HEAD, giving a
// verdict about the wrong revision with nothing disclosing the discrepancy.
// The resolver validates against the CURRENT committed spec and cannot honor a
// revision pin; honoring pins is a disclosed future extension.
func TestResolveBindingAC_PinnedFragment_FailsClosed(t *testing.T) {
	const entry = "spec/vl-003-fragment-target@0123456789abcdef0123456789abcdef01234567#ac-1"
	specRef, acID, err := ResolveBindingAC("spec/vl-003-fragment-owner", entry)
	if err == nil {
		t.Fatalf("ResolveBindingAC(%q) = (%q, %q, nil): a pinned+fragment entry must fail closed, not silently drop the @commit pin and resolve against the current committed spec", entry, specRef, acID)
	}
	// The honest reason must name the pin and disclose it as an unsupported
	// future extension — not read as a malformed-entry rejection.
	for _, want := range []string{"pin", "current", "future extension"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ResolveBindingAC(%q) error = %q, want it to contain %q (the honest reason)", entry, err.Error(), want)
		}
	}
	// The entry itself must be named so a caller can point at the offender.
	if !strings.Contains(err.Error(), entry) {
		t.Errorf("ResolveBindingAC error = %q, want it to name the offending entry %q", err.Error(), entry)
	}
}

// TestResolveBindingAC_UnpinnedFragment_Unchanged pins the boundary the fix
// must not cross: an UNPINNED fragment entry still resolves against its own
// named spec exactly as before — only the pinned form newly fails closed.
func TestResolveBindingAC_UnpinnedFragment_Unchanged(t *testing.T) {
	specRef, acID, err := ResolveBindingAC("spec/vl-003-fragment-owner", "spec/vl-003-fragment-target#ac-1")
	if err != nil {
		t.Fatalf("ResolveBindingAC(unpinned fragment): %v, want it to still resolve", err)
	}
	if specRef != "spec/vl-003-fragment-target" || acID != "ac-1" {
		t.Fatalf("got (%q, %q), want (spec/vl-003-fragment-target, ac-1) — unpinned fragment resolution must be untouched", specRef, acID)
	}
}

// TestIsBareACEntry is the shared bare-vs-fragment classifier both
// Binding.Validate and ResolveBindingAC route through, and internal/lint
// VL-003 uses to decide which entries survive a broken owning `spec:` ref
// (spec/ritual-traps judged-ac4-broken-owning-spec-ref-masks-fragment-typos):
// only a bare ac-<slug> id is bare; every fragment-qualified or malformed form
// is not.
func TestIsBareACEntry(t *testing.T) {
	cases := []struct {
		entry string
		want  bool
	}{
		{"ac-1", true},
		{"ac-99", true},
		{"ac-multi-part-slug", true},
		{"spec/vl-003-fragment-target#ac-1", false},
		{"spec/vl-003-fragment-target@0123456789abcdef0123456789abcdef01234567#ac-1", false},
		{"spec/close-verb", false},
		{"not-an-ac", false},
		{"AC-1", false},
		{"ac-", false},
		{"ac-1-", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.entry, func(t *testing.T) {
			if got := IsBareACEntry(tc.entry); got != tc.want {
				t.Fatalf("IsBareACEntry(%q) = %v, want %v", tc.entry, got, tc.want)
			}
		})
	}
}

func TestDecodeBindings_Negative(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"unknown top-level field", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: [{producer: a, kind: static, acs: [ac-1]}]\nextra: true\n"},
		{"wrong schema", "schema: verdi.bindings/v0\nspec: spec/stale-decline\nbindings: [{producer: a, kind: static, acs: [ac-1]}]\n"},
		{"spec not a ref", "schema: verdi.bindings/v1\nspec: not a ref\nbindings: [{producer: a, kind: static, acs: [ac-1]}]\n"},
		{"spec wrong kind", "schema: verdi.bindings/v1\nspec: adr/0001-outbox-events\nbindings: [{producer: a, kind: static, acs: [ac-1]}]\n"},
		{"no bindings", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: []\n"},
		{"empty producer", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: [{producer: \"\", kind: static, acs: [ac-1]}]\n"},
		{"unknown evidence kind", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: [{producer: a, kind: bogus, acs: [ac-1]}]\n"},
		{"empty acs", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: [{producer: a, kind: static, acs: []}]\n"},
		{"malformed ac id", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: [{producer: a, kind: static, acs: [not-an-ac]}]\n"},
		{"duplicate producer", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: [{producer: a, kind: static, acs: [ac-1]}, {producer: a, kind: behavioral, acs: [ac-2]}]\n"},
		{"dialect anchor", "schema: verdi.bindings/v1\nspec: &s spec/stale-decline\nbindings: [{producer: a, kind: static, acs: [ac-1]}]\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeBindings([]byte(tc.data)); err == nil {
				t.Fatalf("DecodeBindings(%s): want error, got nil", tc.name)
			}
		})
	}
}
