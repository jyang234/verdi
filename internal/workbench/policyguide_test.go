package workbench

import (
	"regexp"
	"strings"
	"testing"
)

// The policy setup guide: when a wall reports policy-forbidden ("project
// has not adopted policy authority"), the board's context/policy notice
// must lead somewhere usable — an inline, read-only guide on the same
// board naming the manual initial setup (constitution, profile, policy,
// consumer inventory) and the existing read-only CLI inspection requests
// with their real operand shape and result fields. It adopts nothing,
// mutates nothing, and never presents proposed/validated as accepted.

const policyGuideID = "asd-policy-guide"

// policyGuideScopedWording is the truthful, mode-independent line the
// guide must carry; policyGuideFalseWording is the unconditional claim it
// must never make (false on a frozen/read-only wall).
const (
	policyGuideScopedWording = "Ordinary draft editing does not require policy; this board&#39;s read-only restrictions still apply."
	policyGuideFalseWording  = "Human editing on this wall continues"
)

// policyGuideHereDoc is the complete copyable shell block the guide must
// render for one request, as it appears HTML-escaped inside <pre><code>:
// command, quoted here-document opener, the request JSON, and the
// matching closing delimiter.
func policyGuideHereDoc(command, escapedJSON string) string {
	return command + " &lt;&lt;&#39;JSON&#39;\n" + escapedJSON + "\nJSON</code></pre>"
}

func policyForbiddenInput() asdShellInput {
	return asdShellInput{
		ProblemPresent: true,
		OutcomePresent: true,
		ACs:            []asdACFact{{ID: "ac-1", EvidenceCount: 1}},
		Mode:           "authoring",
		Branch:         "design/x",
		StateFormal:    "proposed",
		DesignWired:    true,
		// designapp forwards draftmutation's Error() form: "<code>: <detail>".
		CapsFailure: &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: "policy-forbidden: project has not adopted policy authority"},
	}
}

// noDesignAssistanceDetail is ResolvePolicyGrant's OTHER policy-forbidden
// refusal (internal/draftmutation/policy.go): the effective policy
// resolved — adopted and sealed — but carries no design_assistance
// payload. Same code, materially different meaning.
const noDesignAssistanceDetail = "policy-forbidden: effective policy has no design_assistance authority"

func noDesignAssistanceView() *asdView {
	v := testASDView()
	in := policyForbiddenInput()
	in.CapsFailure = &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: noDesignAssistanceDetail}
	v.Shell = deriveASDShell(in)
	return v
}

// TestPolicyGuide_AdoptedPolicyWithoutDesignAssistance_NeverDescribedAsAbsent
// is the negative regression: policy-forbidden with the
// no-design_assistance detail must NOT be rendered as "no policy is
// adopted" — neither in the concern nor in the guide. The refusal detail
// is carried verbatim; the guide is the general variant (no initial-setup
// file list, no "no accepted .verdi/policy" claim), still read-only, still
// carrying the complete here-document checks, in draft and read-only
// modes alike.
func TestPolicyGuide_AdoptedPolicyWithoutDesignAssistance_NeverDescribedAsAbsent(t *testing.T) {
	shell := deriveASDShell(func() asdShellInput {
		in := policyForbiddenInput()
		in.CapsFailure = &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: noDesignAssistanceDetail}
		return in
	}())
	var found bool
	for _, c := range shell.All {
		if c.ID != "context/policy" {
			continue
		}
		found = true
		if c.State != asdStateUnproven || c.Blocking {
			t.Fatalf("context/policy = %+v, want nonblocking unproven", c)
		}
		for _, bad := range []string{"No policy authority is adopted", "not-applicable", "has not adopted"} {
			if strings.Contains(c.Summary, bad) || strings.Contains(c.Guidance, bad) {
				t.Fatalf("context/policy describes an adopted policy as absent (%q): %+v", bad, c)
			}
		}
		if !strings.Contains(c.Summary, "effective policy has no design_assistance authority") {
			t.Fatalf("context/policy summary %q does not carry the refusal detail verbatim", c.Summary)
		}
		if c.Dest != "#"+policyGuideID {
			t.Fatalf("context/policy Dest = %q, want %q", c.Dest, "#"+policyGuideID)
		}
	}
	if !found {
		t.Fatal("no context/policy concern")
	}
	if shell.PolicySetupGuide != policyGuideNoDesignAssistance {
		t.Fatalf("PolicySetupGuide = %q, want %q", shell.PolicySetupGuide, policyGuideNoDesignAssistance)
	}

	for _, mode := range []boardModeKind{modeAuthoring, modeReadOnly} {
		t.Run(string(mode), func(t *testing.T) {
			html := renderBoardRegion(badgeRenderProjection(mode), &boardGitState{Branch: "design/x", DefaultBranch: "main"}, noDesignAssistanceView())
			guide := policyGuideSection(t, html)
			if !strings.Contains(guide, `data-policy-guide="no-design-assistance"`) {
				t.Fatalf("%s: guide is not the no-design-assistance variant; got: %s", mode, guide)
			}
			for _, bad := range []string{
				"has not adopted policy authority",
				"there is no accepted <code>.verdi/policy</code>",
				"Files to author",
				".verdi/policy/constitution.md",
				policyGuideFalseWording,
			} {
				if strings.Contains(guide, bad) {
					t.Fatalf("%s: guide falsely describes an adopted policy as absent/unaccepted (%q); got: %s", mode, bad, guide)
				}
			}
			for _, want := range []string{
				"effective policy has no design_assistance authority", // the refusal, verbatim
				"design_assistance",
				policyGuideScopedWording,
				"not acceptance",
				policyGuideHereDoc("verdi context constitution inspect --request -", `{&#34;schema&#34;:&#34;verdi.constitution-inspect-request/v1&#34;}`),
			} {
				if !strings.Contains(guide, want) {
					t.Fatalf("%s: guide missing %q; got: %s", mode, want, guide)
				}
			}
			if n := strings.Count(guide, "\nJSON</code></pre>"); n != 4 {
				t.Fatalf("%s: guide closes %d here-documents, want 4", mode, n)
			}
			if m := regexp.MustCompile(`<(form|button|input|select|textarea)\b|data-asd-panel=`).FindString(guide); m != "" {
				t.Fatalf("%s: guide carries a control %q", mode, m)
			}
		})
	}
}

func policyForbiddenView() *asdView {
	v := testASDView()
	v.Shell = deriveASDShell(policyForbiddenInput())
	return v
}

func adoptedPolicyView() *asdView {
	v := testASDView()
	in := policyForbiddenInput()
	in.CapsFailure = nil
	in.Caps = &DesignCapabilitiesView{PolicyMode: "proposal-only", PolicyDigest: "sha256:abc", RefusalPrecondition: "policy-mode", RefusalDetail: "mode forbids agent writes"}
	v.Shell = deriveASDShell(in)
	return v
}

// TestPolicyGuide_NoticeLinksToInlineGuide: the policy-forbidden
// context/policy concern carries a destination on the board itself.
func TestPolicyGuide_NoticeLinksToInlineGuide(t *testing.T) {
	shell := deriveASDShell(policyForbiddenInput())
	for _, c := range shell.All {
		if c.ID != "context/policy" {
			continue
		}
		if c.Dest != "#"+policyGuideID {
			t.Fatalf("context/policy Dest = %q, want %q (the notice must reach the inline guide)", c.Dest, "#"+policyGuideID)
		}
		if c.State != asdStateUnproven || c.Blocking {
			t.Fatalf("context/policy = %+v, want nonblocking unproven (unchanged posture)", c)
		}
		return
	}
	t.Fatal("no context/policy concern")
}

// policyGuideSection extracts the rendered guide's markup.
func policyGuideSection(t *testing.T, html string) string {
	t.Helper()
	start := strings.Index(html, `id="`+policyGuideID+`"`)
	if start < 0 {
		t.Fatalf("board region has no element with id=%q; got: %s", policyGuideID, html)
	}
	end := strings.Index(html[start:], `data-policy-guide-end`)
	if end < 0 {
		t.Fatalf("policy guide has no end marker; got: %s", html[start:])
	}
	return html[start : start+end]
}

// TestPolicyGuide_RendersReadOnlyGuidance pins the guide's content: plain
// summary first (what is missing, why review is blocked), the manual
// initial files, the four read-only CLI inspection requests with their
// real operand shape and result fields under expandable technical
// detail, and the proposed/validated-is-not-accepted line — with no
// control that could mutate anything.
func TestPolicyGuide_RendersReadOnlyGuidance(t *testing.T) {
	html := renderBoardRegion(badgeRenderProjection(modeAuthoring), &boardGitState{Branch: "design/x", DefaultBranch: "main"}, policyForbiddenView())
	guide := policyGuideSection(t, html)

	for _, want := range []string{
		// what is missing / why blocked, in plain words — scoped truthfully
		// for every mode (a read-only board is not "editing continues")
		"has not adopted policy authority",
		"semantic review",
		policyGuideScopedWording,
		// the manual initial setup, by file
		".verdi/policy/constitution.md",
		".verdi/policy/profiles/",
		".verdi/policy/policies/",
		".verdi/constitution/consumers.json",
		// the real CLI requests as COMPLETE copyable shell blocks: the
		// operation, its real operand shape, the request JSON on a quoted
		// here-document, and the matching closing delimiter — never a bare
		// command that would block waiting on stdin
		policyGuideHereDoc("verdi context constitution inspect --request -", `{&#34;schema&#34;:&#34;verdi.constitution-inspect-request/v1&#34;}`),
		policyGuideHereDoc("verdi context constitution validate --request -", `{&#34;schema&#34;:&#34;verdi.constitution-validate-request/v1&#34;}`),
		policyGuideHereDoc("verdi context constitution impact-review --request -", `{&#34;schema&#34;:&#34;verdi.constitution-impact-review-request/v1&#34;,&#34;targets&#34;:[]}`),
		policyGuideHereDoc("verdi context constitution submit-preparation --request -", `{&#34;schema&#34;:&#34;verdi.constitution-submit-preparation-request/v1&#34;,&#34;targets&#34;:[]}`),
		// the actual readiness result fields
		"accepted.adopted", "proposed.adopted", "snapshot.adopted",
		"coverage.state", "coverage.reasons",
		"ready_for_submission", "blocking_reasons",
		// CLI success is not readiness; proposed/validated is not accepted
		"exit 0",
		"not acceptance",
		// technical detail is expandable, not an always-open wall
		"<details",
	} {
		if !strings.Contains(guide, want) {
			t.Errorf("policy guide missing %q; got: %s", want, guide)
		}
	}

	if strings.Contains(guide, policyGuideFalseWording) {
		t.Fatalf("policy guide claims %q — false on a read-only board; wording must be mode-scoped", policyGuideFalseWording)
	}
	if !strings.Contains(guide, `data-policy-guide="not-adopted"`) {
		t.Fatalf("policy guide for the not-adopted discriminant is not the not-adopted variant; got: %s", guide)
	}
	// Every rendered command block is exactly one complete here-document.
	if n := strings.Count(guide, `<pre class="asd-policy-guide-cmd">`); n != 4 {
		t.Fatalf("policy guide renders %d command blocks, want 4", n)
	}
	if n := strings.Count(guide, "\nJSON</code></pre>"); n != 4 {
		t.Fatalf("policy guide closes %d here-documents with a matching JSON delimiter, want 4", n)
	}

	// Read-only: no form, button, input, or fetch wiring inside the guide.
	if m := regexp.MustCompile(`<(form|button|input|select|textarea)\b|data-asd-panel=`).FindString(guide); m != "" {
		t.Fatalf("policy guide carries a control %q — it must be read-only markup", m)
	}
	// The concern row's destination link resolves to the guide.
	if !strings.Contains(html, `<a class="asd-dest-link" href="#`+policyGuideID+`">`) {
		t.Fatalf("context/policy row has no destination link to #%s; got: %s", policyGuideID, html)
	}
}

// TestPolicyGuide_AbsentWhenPolicyAdopted: a wall with adopted policy
// authority renders no setup guide.
func TestPolicyGuide_AbsentWhenPolicyAdopted(t *testing.T) {
	html := renderBoardRegion(badgeRenderProjection(modeAuthoring), &boardGitState{Branch: "design/x", DefaultBranch: "main"}, adoptedPolicyView())
	if strings.Contains(html, `id="`+policyGuideID+`"`) {
		t.Fatalf("policy guide rendered on an adopted-policy wall; got: %s", html)
	}
}

// TestPolicyGuide_ReadOnlyAndReviewModes_KeepRestrictions: the guide is
// the same read-only markup on a read-only or review board — it adds no
// control there either.
func TestPolicyGuide_ReadOnlyAndReviewModes_KeepRestrictions(t *testing.T) {
	for _, mode := range []boardModeKind{modeReadOnly, modeReview} {
		t.Run(string(mode), func(t *testing.T) {
			html := renderBoardRegion(badgeRenderProjection(mode), &boardGitState{Branch: "design/x", DefaultBranch: "main"}, policyForbiddenView())
			guide := policyGuideSection(t, html)
			if m := regexp.MustCompile(`<(form|button|input|select|textarea)\b`).FindString(guide); m != "" {
				t.Fatalf("%s: policy guide carries a control %q", mode, m)
			}
			if !strings.Contains(guide, ".verdi/policy/constitution.md") {
				t.Fatalf("%s: policy guide lost its setup content", mode)
			}
			// Truthful scoped wording in every mode; never the unconditional
			// "editing continues" claim a frozen or read-only wall would belie.
			if !strings.Contains(guide, policyGuideScopedWording) {
				t.Fatalf("%s: policy guide lacks the mode-scoped wording %q", mode, policyGuideScopedWording)
			}
			if strings.Contains(guide, policyGuideFalseWording) {
				t.Fatalf("%s: policy guide claims %q on a %s board", mode, policyGuideFalseWording, mode)
			}
			if n := strings.Count(guide, "\nJSON</code></pre>"); n != 4 {
				t.Fatalf("%s: policy guide closes %d here-documents, want 4", mode, n)
			}
		})
	}
}
