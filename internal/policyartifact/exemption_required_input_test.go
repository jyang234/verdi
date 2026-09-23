package policyartifact

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
)

// Fixture values for the required-input witness family (unsealed-
// provenance exemption design §4; SI-241, SI-255). Object ids are full
// 40-hex (SHA-1 repository) ids unless a case says otherwise.
var (
	riSpecDigest     = "sha256:" + strings.Repeat("a", 64)
	riCommit         = strings.Repeat("1", 40)
	riTree           = strings.Repeat("2", 40)
	riLanded         = strings.Repeat("3", 40)
	riHistoryDigest  = "sha256:" + strings.Repeat("b", 64)
	riCorrective     = "Adopt the sealed review path before any further unsealed use."
	riUnavailable    = "The sealed review capsule was not yet adopted."
	riEscalationYAML = `  escalation:
    use_history_digest: "` + riHistoryDigest + `"
    corrective_action: "` + riCorrective + `"
    sealed_execution_unavailable_reason: "` + riUnavailable + `"
`
)

// requiredInputBlock is the column-zero `required_input:` mapping without
// an escalation block; the escalation block, when used, is appended after
// it (it is the mapping's last key).
func requiredInputBlock() string {
	return `required_input:
  phase: review
  inputs: [builder-receipt, evidence-bundle, result-diff]
  story: spec/vatc-machine-projections
  accepted_spec_digest: "` + riSpecDigest + `"
  implementation:
    commit: "` + riCommit + `"
    tree: "` + riTree + `"
  inventory_entry: vatc-machine-projections
  landed_snapshot: "` + riLanded + `"
`
}

// validRequiredInputDoc is a complete required-input exemption: the
// required_input witness, NO witnesses key, and a mandatory expiry.
func validRequiredInputDoc() string {
	return `---
schema: verdi.policy-exemption/v1
id: policy-exemption/unsealed-vatc-machine-projections
kind: policy-exemption
title: "Unsealed provenance for the vatc-machine-projections implementation"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
` + requiredInputBlock() + `compensating_controls:
  - "Every acceptance criterion folds to evidenced from CI-produced records."
approvals:
  - role: policy-owner
    principal: principal/github-org/YWxpY2U
expiry: "2026-12-31"
` + exemptionTemplateLine + `
---
The implementation was built before a sealed review path existed.
`
}

// withEscalation returns doc with the escalation block appended to its
// required_input mapping.
func withEscalation(doc string) string {
	return strings.Replace(doc, "  landed_snapshot: \""+riLanded+"\"\n", "  landed_snapshot: \""+riLanded+"\"\n"+riEscalationYAML, 1)
}

// withReviewCondition returns doc with a review condition beside its
// expiry.
func withReviewCondition(doc string) string {
	return strings.Replace(doc, "expiry: \"2026-12-31\"\n", "expiry: \"2026-12-31\"\nreview_condition: \"Revisit when the sealed review path is adopted.\"\n", 1)
}

func TestRequiredInputConstants(t *testing.T) {
	if RequiredInputPhase != "review" {
		t.Fatalf("RequiredInputPhase = %q, want review", RequiredInputPhase)
	}
	if EscalationRole != "unsealed-exemption-escalation" {
		t.Fatalf("EscalationRole = %q", EscalationRole)
	}
	want := []string{"builder-receipt", "evidence-bundle", "result-diff"}
	got := RequiredInputNames()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RequiredInputNames() = %v, want %v", got, want)
	}
	if !reflect.DeepEqual([]string{InputBuilderReceipt, InputEvidenceBundle, InputResultDiff}, want) {
		t.Fatalf("input constants = %q %q %q, want %v", InputBuilderReceipt, InputEvidenceBundle, InputResultDiff, want)
	}
	got[0] = "mutated"
	if again := RequiredInputNames(); !reflect.DeepEqual(again, want) {
		t.Fatalf("RequiredInputNames() returned a shared slice: after mutation = %v", again)
	}
}

func TestDecodeExemption_RequiredInputHappy(t *testing.T) {
	sha256Commit := strings.Repeat("c", 64)
	sha256Tree := strings.Repeat("d", 64)
	sha256Landed := strings.Repeat("e", 64)
	sha256IDs := func(doc string) string {
		doc = strings.Replace(doc, riCommit, sha256Commit, 1)
		doc = strings.Replace(doc, riTree, sha256Tree, 1)
		return strings.Replace(doc, riLanded, sha256Landed, 1)
	}
	tests := []struct {
		name           string
		doc            string
		wantEscalation bool
		wantReview     string
		wantCommit     string
	}{
		{"expiry only, no escalation", validRequiredInputDoc(), false, "", riCommit},
		{"with escalation", withEscalation(validRequiredInputDoc()), true, "", riCommit},
		{"review condition beside expiry", withReviewCondition(validRequiredInputDoc()), false, "Revisit when the sealed review path is adopted.", riCommit},
		{"escalation and review condition", withReviewCondition(withEscalation(validRequiredInputDoc())), true, "Revisit when the sealed review path is adopted.", riCommit},
		{"64-hex object ids (one SHA-256 repository)", sha256IDs(validRequiredInputDoc()), false, "", sha256Commit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := DecodeExemption([]byte(tt.doc))
			if err != nil {
				t.Fatalf("DecodeExemption: %v", err)
			}
			if e.Witnesses == nil || len(e.Witnesses) != 0 {
				t.Fatalf("Witnesses = %#v, want the empty non-nil slice", e.Witnesses)
			}
			ri := e.RequiredInput
			if ri == nil {
				t.Fatal("RequiredInput = nil")
			}
			if ri.Phase != "review" || !reflect.DeepEqual(ri.Inputs, RequiredInputNames()) {
				t.Fatalf("phase/inputs = %q %v", ri.Phase, ri.Inputs)
			}
			if ri.Story != "spec/vatc-machine-projections" || ri.AcceptedSpecDigest != riSpecDigest || ri.InventoryEntry != "vatc-machine-projections" {
				t.Fatalf("witness = %+v", ri)
			}
			if ri.Implementation.Commit != tt.wantCommit || len(ri.Implementation.Tree) != len(tt.wantCommit) || len(ri.LandedSnapshot) != len(tt.wantCommit) {
				t.Fatalf("implementation/landed = %+v / %q", ri.Implementation, ri.LandedSnapshot)
			}
			if tt.wantEscalation {
				want := &EscalationRecord{UseHistoryDigest: riHistoryDigest, CorrectiveAction: riCorrective, SealedExecutionUnavailableReason: riUnavailable}
				if !reflect.DeepEqual(ri.Escalation, want) {
					t.Fatalf("Escalation = %+v, want %+v", ri.Escalation, want)
				}
			} else if ri.Escalation != nil {
				t.Fatalf("Escalation = %+v, want nil", ri.Escalation)
			}
			if e.Expiry != "2026-12-31" || e.ReviewCondition != tt.wantReview {
				t.Fatalf("expiry/review = %q/%q", e.Expiry, e.ReviewCondition)
			}
			d1, err := e.Digest()
			if err != nil {
				t.Fatalf("Digest: %v", err)
			}
			again, err := DecodeExemption([]byte(tt.doc))
			if err != nil {
				t.Fatalf("re-decode: %v", err)
			}
			d2, err := again.Digest()
			if err != nil {
				t.Fatalf("re-decode Digest: %v", err)
			}
			if d1 != d2 {
				t.Fatalf("digest unstable across re-decode: %s vs %s", d1, d2)
			}
		})
	}
}

// TestDecodeExemption_RequiredInputSurvivesPinnedClone clones the decoded
// value exactly as internal/contextcompile/conflict.go's cloneExemption
// does for Witnesses (a pinned consolidation source this lane must not
// edit): `append([]Witness{}, in.Witnesses...)` turns a nil slice into
// `[]`, so a nil decoded Witnesses would break the seal. The decoder's
// empty non-nil slice keeps Digest succeeding on the clone.
func TestDecodeExemption_RequiredInputSurvivesPinnedClone(t *testing.T) {
	for _, doc := range []string{validRequiredInputDoc(), withEscalation(validRequiredInputDoc())} {
		in, err := DecodeExemption([]byte(doc))
		if err != nil {
			t.Fatalf("DecodeExemption: %v", err)
		}
		want, err := in.Digest()
		if err != nil {
			t.Fatalf("Digest: %v", err)
		}
		out := *in
		out.Witnesses = append([]Witness{}, in.Witnesses...)
		got, err := out.Digest()
		if err != nil {
			t.Fatalf("clone Digest: %v", err)
		}
		if got != want {
			t.Fatalf("clone digest = %s, want %s", got, want)
		}
	}
}

// TestDecodeExemption_RequiredInputSealCovered proves the canonical digest
// includes required_input: two exemptions differing only inside the
// witness digest differently, and a post-decode edit of any witness field
// fails the seal.
func TestDecodeExemption_RequiredInputSealCovered(t *testing.T) {
	base, err := DecodeExemption([]byte(withEscalation(validRequiredInputDoc())))
	if err != nil {
		t.Fatalf("DecodeExemption: %v", err)
	}
	baseDigest, err := base.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	other, err := DecodeExemption([]byte(strings.Replace(withEscalation(validRequiredInputDoc()), "inventory_entry: vatc-machine-projections", "inventory_entry: other-entry", 1)))
	if err != nil {
		t.Fatalf("DecodeExemption(other): %v", err)
	}
	otherDigest, err := other.Digest()
	if err != nil {
		t.Fatalf("Digest(other): %v", err)
	}
	if otherDigest == baseDigest {
		t.Fatal("digest ignores required_input content")
	}
	canon, err := canonjson.Marshal(base)
	if err != nil {
		t.Fatalf("canonjson.Marshal: %v", err)
	}
	for _, key := range []string{`"required_input":`, `"witnesses":[]`, `"escalation":`, `"landed_snapshot":`} {
		if !strings.Contains(string(canon), key) {
			t.Fatalf("canonical encoding lacks %s: %s", key, canon)
		}
	}

	mutations := []struct {
		name   string
		mutate func(e *Exemption)
	}{
		{"story", func(e *Exemption) { e.RequiredInput.Story = "spec/other" }},
		{"inputs", func(e *Exemption) { e.RequiredInput.Inputs[0] = "accepted-spec" }},
		{"implementation commit", func(e *Exemption) { e.RequiredInput.Implementation.Commit = strings.Repeat("9", 40) }},
		{"landed snapshot", func(e *Exemption) { e.RequiredInput.LandedSnapshot = strings.Repeat("9", 40) }},
		{"escalation", func(e *Exemption) { e.RequiredInput.Escalation.CorrectiveAction = "none" }},
		{"escalation removed", func(e *Exemption) { e.RequiredInput.Escalation = nil }},
		{"witness removed", func(e *Exemption) { e.RequiredInput = nil }},
	}
	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			e, err := DecodeExemption([]byte(withEscalation(validRequiredInputDoc())))
			if err != nil {
				t.Fatalf("DecodeExemption: %v", err)
			}
			m.mutate(e)
			if _, err := e.Digest(); err == nil || !strings.Contains(err.Error(), "modified after decode") {
				t.Fatalf("Digest after %s edit = %v, want a modified-after-decode error", m.name, err)
			}
		})
	}
}

// TestDecodeExemption_ClaimFamilyEncodingUnchanged proves the new field is
// invisible in a claim-family exemption's canonical encoding (omitempty),
// which is why every pinned claim-family digest stays byte-identical.
func TestDecodeExemption_ClaimFamilyEncodingUnchanged(t *testing.T) {
	e, err := DecodeExemption([]byte(validExemptionDoc()))
	if err != nil {
		t.Fatalf("DecodeExemption: %v", err)
	}
	if e.RequiredInput != nil {
		t.Fatalf("claim-family RequiredInput = %+v, want nil", e.RequiredInput)
	}
	canon, err := canonjson.Marshal(e)
	if err != nil {
		t.Fatalf("canonjson.Marshal: %v", err)
	}
	if strings.Contains(string(canon), "required_input") {
		t.Fatalf("claim-family canonical encoding mentions required_input: %s", canon)
	}
}

// TestDecodeExemption_RequiredInputFamilyNegative proves exactly one
// witness family (SI-241, SI-255) and the required-input family's
// mandatory expiry.
func TestDecodeExemption_RequiredInputFamilyNegative(t *testing.T) {
	claimWitnesses := "witnesses:\n  - policy: policy/go-toolchain\n    claim: go-version\n    claim_digest: \"sha256:" + strings.Repeat("1", 64) + "\"\n"
	tests := []struct {
		name    string
		doc     string
		wantSub string
	}{
		{"mixed families", strings.Replace(validRequiredInputDoc(), "required_input:\n", claimWitnesses+"required_input:\n", 1), "both witness families"},
		{"mixed families with an empty witnesses list", strings.Replace(validRequiredInputDoc(), "required_input:\n", "witnesses: []\nrequired_input:\n", 1), "both witness families"},
		{"mixed families, witnesses after required_input", strings.Replace(validRequiredInputDoc(), "compensating_controls:\n", "witnesses: []\ncompensating_controls:\n", 1), "both witness families"},
		{"neither family", strings.Replace(validRequiredInputDoc(), requiredInputBlock(), "", 1), "witnesses is missing"},
		{"required-input without expiry", strings.Replace(validRequiredInputDoc(), "expiry: \"2026-12-31\"\n", "", 1), "required-input exemption must carry an expiry"},
		{"required-input with only a review condition", strings.Replace(validRequiredInputDoc(), "expiry: \"2026-12-31\"", "review_condition: \"Revisit when the sealed review path is adopted.\"", 1), "required-input exemption must carry an expiry"},
		{"required-input with an impossible expiry", strings.Replace(validRequiredInputDoc(), "expiry: \"2026-12-31\"", "expiry: \"2026-02-31\"", 1), "calendar"},
		{"required-input with a blank review condition beside expiry", strings.Replace(validRequiredInputDoc(), "expiry: \"2026-12-31\"\n", "expiry: \"2026-12-31\"\nreview_condition: \"   \"\n", 1), "blank"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeExemption([]byte(tt.doc))
			if err == nil {
				t.Fatalf("DecodeExemption = nil error, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("DecodeExemption error = %v, want containing %q", err, tt.wantSub)
			}
		})
	}
}

// TestDecodeExemption_RequiredInputFieldNegative proves every required-
// input witness field's presence and grammar (SI-255): each refusal names
// the offending field.
func TestDecodeExemption_RequiredInputFieldNegative(t *testing.T) {
	base := validRequiredInputDoc()
	esc := withEscalation(validRequiredInputDoc())
	replace := func(doc, old, new string) string {
		if !strings.Contains(doc, old) {
			t.Fatalf("fixture does not contain %q", old)
		}
		return strings.Replace(doc, old, new, 1)
	}
	drop := func(doc, line string) string { return replace(doc, line+"\n", "") }
	setInputs := func(list string) string {
		return replace(base, "inputs: [builder-receipt, evidence-bundle, result-diff]", "inputs: "+list)
	}
	implementationBlock := "  implementation:\n    commit: \"" + riCommit + "\"\n    tree: \"" + riTree + "\"\n"
	sha256Hex := strings.Repeat("f", 64)

	tests := []struct {
		name    string
		doc     string
		wantSub string
	}{
		// Presence: every field except escalation is mandatory; both
		// implementation fields; every escalation field when present.
		{"missing phase", drop(base, "  phase: review"), "required_input.phase is missing"},
		{"missing inputs", drop(base, "  inputs: [builder-receipt, evidence-bundle, result-diff]"), "required_input.inputs is missing"},
		{"missing story", drop(base, "  story: spec/vatc-machine-projections"), "required_input.story is missing"},
		{"missing accepted_spec_digest", drop(base, "  accepted_spec_digest: \""+riSpecDigest+"\""), "required_input.accepted_spec_digest is missing"},
		{"missing implementation", replace(base, implementationBlock, ""), "required_input.implementation is missing"},
		{"missing implementation.commit", drop(base, "    commit: \""+riCommit+"\""), "required_input.implementation.commit is missing"},
		{"missing implementation.tree", drop(base, "    tree: \""+riTree+"\""), "required_input.implementation.tree is missing"},
		{"missing inventory_entry", drop(base, "  inventory_entry: vatc-machine-projections"), "required_input.inventory_entry is missing"},
		{"missing landed_snapshot", drop(base, "  landed_snapshot: \""+riLanded+"\""), "required_input.landed_snapshot is missing"},
		{"missing escalation.use_history_digest", drop(esc, "    use_history_digest: \""+riHistoryDigest+"\""), "required_input.escalation.use_history_digest is missing"},
		{"missing escalation.corrective_action", drop(esc, "    corrective_action: \""+riCorrective+"\""), "required_input.escalation.corrective_action is missing"},
		{"missing escalation.sealed_execution_unavailable_reason", drop(esc, "    sealed_execution_unavailable_reason: \""+riUnavailable+"\""), "required_input.escalation.sealed_execution_unavailable_reason is missing"},
		{"empty required_input mapping", replace(base, requiredInputBlock(), "required_input: {}\n"), "required_input.phase is missing"},

		// Strict decode: unknown keys at every nesting level.
		{"unknown key in required_input", replace(base, "  phase: review\n", "  phase: review\n  severity: high\n"), "severity"},
		{"unknown key in implementation", replace(base, "    tree: \""+riTree+"\"\n", "    tree: \""+riTree+"\"\n    branch: main\n"), "branch"},
		{"unknown key in escalation", replace(esc, "    corrective_action: ", "    notes: \"x\"\n    corrective_action: "), "notes"},

		// Phase.
		{"phase build", replace(base, "  phase: review", "  phase: build"), "required_input.phase"},
		{"phase design", replace(base, "  phase: review", "  phase: design"), "required_input.phase"},
		{"phase upper-case", replace(base, "  phase: review", "  phase: Review"), "required_input.phase"},
		{"phase blank", replace(base, "  phase: review", "  phase: \"\""), "required_input.phase"},

		// Inputs: exactly the sorted three, never re-sorted.
		{"inputs missing one", setInputs("[builder-receipt, evidence-bundle]"), "required_input.inputs"},
		{"inputs extra one", setInputs("[accepted-spec, builder-receipt, evidence-bundle, result-diff]"), "required_input.inputs"},
		{"inputs duplicate", setInputs("[builder-receipt, builder-receipt, evidence-bundle, result-diff]"), "required_input.inputs"},
		{"inputs duplicate replacing one", setInputs("[builder-receipt, evidence-bundle, evidence-bundle]"), "required_input.inputs"},
		{"inputs unsorted", setInputs("[evidence-bundle, builder-receipt, result-diff]"), "required_input.inputs"},
		{"inputs reversed", setInputs("[result-diff, evidence-bundle, builder-receipt]"), "required_input.inputs"},
		{"inputs empty", setInputs("[]"), "required_input.inputs"},
		{"inputs another set", setInputs("[accepted-spec, current-diff, review-policy]"), "required_input.inputs"},

		// Story: an unpinned, unfragmented spec ref.
		{"story not a ref", replace(base, "story: spec/vatc-machine-projections", "story: vatc-machine-projections"), "required_input.story"},
		{"story an adr ref", replace(base, "story: spec/vatc-machine-projections", "story: adr/vatc-machine-projections"), "required_input.story"},
		{"story an unknown kind", replace(base, "story: spec/vatc-machine-projections", "story: feature/vatc-machine-projections"), "required_input.story"},
		{"story pinned", replace(base, "story: spec/vatc-machine-projections", "story: spec/vatc-machine-projections@abcdef1"), "required_input.story"},
		{"story fragmented", replace(base, "story: spec/vatc-machine-projections", "story: \"spec/vatc-machine-projections#ac-1\""), "required_input.story"},
		{"story blank", replace(base, "story: spec/vatc-machine-projections", "story: \"\""), "required_input.story"},
		{"story non-kebab name", replace(base, "story: spec/vatc-machine-projections", "story: spec/Vatc_Machine"), "required_input.story"},

		// Content digests.
		{"accepted_spec_digest wrong algorithm", replace(base, riSpecDigest, "sha1:"+strings.Repeat("a", 40)), "required_input.accepted_spec_digest"},
		{"accepted_spec_digest upper-case", replace(base, riSpecDigest, "sha256:"+strings.Repeat("A", 64)), "required_input.accepted_spec_digest"},
		{"accepted_spec_digest short", replace(base, riSpecDigest, "sha256:"+strings.Repeat("a", 63)), "required_input.accepted_spec_digest"},
		{"accepted_spec_digest bare hex", replace(base, riSpecDigest, strings.Repeat("a", 64)), "required_input.accepted_spec_digest"},
		{"use_history_digest malformed", replace(esc, riHistoryDigest, "sha256:beef"), "required_input.escalation.use_history_digest"},
		{"use_history_digest non-hex", replace(esc, riHistoryDigest, "sha256:"+strings.Repeat("g", 64)), "required_input.escalation.use_history_digest"},

		// Object ids: full lowercase hex, 40 or 64, one length for all three.
		{"commit short", replace(base, "commit: \""+riCommit+"\"", "commit: \""+riCommit[:39]+"\""), "required_input.implementation.commit"},
		{"commit abbreviated", replace(base, "commit: \""+riCommit+"\"", "commit: \"abcdef1\""), "required_input.implementation.commit"},
		{"commit upper-case", replace(base, "commit: \""+riCommit+"\"", "commit: \""+strings.Repeat("A", 40)+"\""), "required_input.implementation.commit"},
		{"commit non-hex", replace(base, "commit: \""+riCommit+"\"", "commit: \""+strings.Repeat("g", 40)+"\""), "required_input.implementation.commit"},
		{"commit 41 hex", replace(base, "commit: \""+riCommit+"\"", "commit: \""+riCommit+"1\""), "required_input.implementation.commit"},
		{"commit blank", replace(base, "commit: \""+riCommit+"\"", "commit: \"\""), "required_input.implementation.commit"},
		{"tree short", replace(base, "tree: \""+riTree+"\"", "tree: \""+riTree[:20]+"\""), "required_input.implementation.tree"},
		{"tree upper-case", replace(base, "tree: \""+riTree+"\"", "tree: \""+strings.Repeat("B", 40)+"\""), "required_input.implementation.tree"},
		{"tree non-hex", replace(base, "tree: \""+riTree+"\"", "tree: \""+strings.Repeat("z", 40)+"\""), "required_input.implementation.tree"},
		{"landed_snapshot short", replace(base, "landed_snapshot: \""+riLanded+"\"", "landed_snapshot: \"abc\""), "required_input.landed_snapshot"},
		{"landed_snapshot upper-case", replace(base, "landed_snapshot: \""+riLanded+"\"", "landed_snapshot: \""+strings.Repeat("C", 40)+"\""), "required_input.landed_snapshot"},
		{"landed_snapshot non-hex", replace(base, "landed_snapshot: \""+riLanded+"\"", "landed_snapshot: \""+strings.Repeat("x", 64)+"\""), "required_input.landed_snapshot"},
		{"mixed length: tree 64", replace(base, "tree: \""+riTree+"\"", "tree: \""+sha256Hex+"\""), "one repository hash algorithm"},
		{"mixed length: landed 64", replace(base, "landed_snapshot: \""+riLanded+"\"", "landed_snapshot: \""+sha256Hex+"\""), "one repository hash algorithm"},
		{"mixed length: commit 64", replace(base, "commit: \""+riCommit+"\"", "commit: \""+sha256Hex+"\""), "one repository hash algorithm"},

		// Inventory entry id.
		{"inventory_entry non-kebab", replace(base, "inventory_entry: vatc-machine-projections", "inventory_entry: Vatc_Machine"), "required_input.inventory_entry"},
		{"inventory_entry a ref", replace(base, "inventory_entry: vatc-machine-projections", "inventory_entry: spec/vatc-machine-projections"), "required_input.inventory_entry"},
		{"inventory_entry blank", replace(base, "inventory_entry: vatc-machine-projections", "inventory_entry: \"\""), "required_input.inventory_entry"},

		// Escalation text: single non-blank lines.
		{"corrective_action multi-line", replace(esc, "corrective_action: \""+riCorrective+"\"", "corrective_action: \"one\\ntwo\""), "required_input.escalation.corrective_action"},
		{"corrective_action carriage return", replace(esc, "corrective_action: \""+riCorrective+"\"", "corrective_action: \"one\\rtwo\""), "required_input.escalation.corrective_action"},
		{"corrective_action blank", replace(esc, "corrective_action: \""+riCorrective+"\"", "corrective_action: \"   \""), "required_input.escalation.corrective_action"},
		{"corrective_action empty", replace(esc, "corrective_action: \""+riCorrective+"\"", "corrective_action: \"\""), "required_input.escalation.corrective_action"},
		{"unavailable reason multi-line", replace(esc, "sealed_execution_unavailable_reason: \""+riUnavailable+"\"", "sealed_execution_unavailable_reason: \"one\\ntwo\""), "required_input.escalation.sealed_execution_unavailable_reason"},
		{"unavailable reason blank", replace(esc, "sealed_execution_unavailable_reason: \""+riUnavailable+"\"", "sealed_execution_unavailable_reason: \"\\t\""), "required_input.escalation.sealed_execution_unavailable_reason"},
		{"escalation empty mapping", replace(esc, riEscalationYAML, "  escalation: {}\n"), "required_input.escalation.use_history_digest is missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeExemption([]byte(tt.doc))
			if err == nil {
				t.Fatalf("DecodeExemption = nil error, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("DecodeExemption error = %v, want containing %q", err, tt.wantSub)
			}
		})
	}
}

// TestDecodeExemption_RequiredInputNullWitnessesKey pins the strict
// seam's null-is-absent reading for the family rule: a `witnesses` key
// whose value is YAML null decodes to a nil pointer — exactly as `expiry:
// null` reads as no expiry — so it carries no claim witness and the
// decoded value, and therefore its digest, equals the key-absent
// exemption's. Only a non-null witnesses value (even []) mixes families.
func TestDecodeExemption_RequiredInputNullWitnessesKey(t *testing.T) {
	want, err := DecodeExemption([]byte(validRequiredInputDoc()))
	if err != nil {
		t.Fatalf("DecodeExemption: %v", err)
	}
	wantDigest, err := want.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	for _, null := range []string{"witnesses: null\n", "witnesses: ~\n", "witnesses:\n"} {
		t.Run(strings.TrimSpace(null), func(t *testing.T) {
			e, err := DecodeExemption([]byte(strings.Replace(validRequiredInputDoc(), "required_input:\n", null+"required_input:\n", 1)))
			if err != nil {
				t.Fatalf("DecodeExemption: %v", err)
			}
			got, err := e.Digest()
			if err != nil {
				t.Fatalf("Digest: %v", err)
			}
			if got != wantDigest {
				t.Fatalf("digest = %s, want the key-absent digest %s", got, wantDigest)
			}
		})
	}
}
