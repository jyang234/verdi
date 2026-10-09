package workbench

import (
	stdhtml "html"
	"regexp"
	"strings"
	"testing"
)

// The policy setup guide: when a wall reports policy-forbidden ("project
// has not adopted policy authority"), the wall must lead somewhere usable
// — an inline, read-only guide naming the one verb that performs initial
// setup (verdi policy adopt --starter), the four files it writes — which a
// project may instead author by hand, the fallback rather than the route
// (constitution, profile, policy, consumer inventory) — and the existing
// read-only CLI inspection requests with their real operand shape and
// result fields. It adopts nothing, mutates nothing, and never presents
// proposed/validated as accepted.
//
// The guide is the home of the retired wall shell's context/policy row
// (spec/wall-strip-and-drawer-v2 ac-7, dc-4; SI-368 (3), (32) B1): the
// record drawer's Readiness tab carries it under its capabilities label,
// chosen by policyGuideFor from the capabilities consultation alone and
// scoped, in its editing line, to the wall's mode. Every claim the row
// carried is asserted here on the guide.

const policyGuideID = "asd-policy-guide"

// policyGuideScopedWording is the truthful, mode-independent line the
// guide must carry; policyGuideFalseWording is the unconditional claim it
// must never make (false on a frozen/read-only wall).
const (
	policyGuideScopedWording = "Ordinary human editing does not require policy; this board&#39;s read-only restrictions still apply."
	policyGuideFalseWording  = "Human editing on this wall continues"
)

// policyGuideHereDoc is the complete copyable shell block the guide must
// render for one request, as it appears HTML-escaped inside <pre><code>:
// command, quoted here-document opener, the request JSON, and the
// matching closing delimiter.
func policyGuideHereDoc(command, escapedJSON string) string {
	return command + " &lt;&lt;&#39;JSON&#39;\n" + escapedJSON + "\nJSON</code></pre>"
}

// policyCaps is one capabilities consultation's outcome as the wall reads
// it: whether the design application service is wired, the view it
// derived, or its refusal — everything policyGuideFor chooses the guide
// from (SI-368 (3): the guide is capabilities, not readiness).
type policyCaps struct {
	Wired   bool
	Caps    *DesignCapabilitiesView
	Failure *DesignFailure
}

func policyForbiddenInput() policyCaps {
	return policyCaps{
		Wired: true,
		// designapp forwards draftmutation's own BARE Detail here, never its
		// Error() form ("<code>: <detail>") — Code already carries
		// "policy-forbidden" separately, so embedding it again in Detail
		// would double the prefix once a caller renders "Code: Detail"
		// (ac-5, spec/uat-round-1; internal/designapp/outcome.go's
		// translateDraftmutationError).
		Failure: &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: "project has not adopted policy authority"},
	}
}

// noDesignAssistanceDetail is ResolvePolicyGrant's OTHER policy-forbidden
// refusal (internal/draftmutation/policy.go): the effective policy
// resolved — adopted and sealed — but carries no design_assistance
// payload. Same code, materially different meaning. Bare, like
// policyForbiddenInput's Detail above — never the "policy-forbidden: "-
// prefixed Error() form.
const noDesignAssistanceDetail = "effective policy has no design_assistance authority"

func noDesignAssistanceInput() policyCaps {
	in := policyForbiddenInput()
	in.Failure = &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: noDesignAssistanceDetail}
	return in
}

func adoptedPolicyInput() policyCaps {
	return policyCaps{Wired: true, Caps: &DesignCapabilitiesView{PolicyMode: "proposal-only", PolicyDigest: "sha256:abc", RefusalPrecondition: "policy-mode", RefusalDetail: "mode forbids agent writes"}}
}

// wallGuide renders the wall's policy setup guide where it lives: the
// record drawer's Readiness tab, under its capabilities label
// (renderReadinessTabGuide; SI-368 (3), (27)(c)), chosen by
// policyGuideFor from in's capabilities consultation, on a wall in mode.
func wallGuide(in policyCaps, mode boardModeKind) string {
	return renderReadinessTabGuide(policyGuideFor(in.Wired, in.Caps, in.Failure), mode)
}

// guideModes is every wall mode the guide is rendered in.
func guideModes() []boardModeKind {
	return []boardModeKind{modeAuthoring, modeReview, modeReadOnly}
}

// expectNoGuideOnTheWall fails when a wall's region in mode carries the
// policy setup guide or a link to it: the guide is the Readiness tab's,
// never the wall's (SI-368 (27)(c)).
func expectNoGuideOnTheWall(t *testing.T, mode boardModeKind) {
	t.Helper()
	html := renderBoardRegion(badgeRenderProjection(mode), &boardGitState{Branch: "design/x", DefaultBranch: "main"}, testASDView())
	if strings.Contains(html, policyGuideID) {
		t.Errorf("%s: the wall's region carries the policy guide or a link to it", mode)
	}
}

// expectNoReadinessState fails when the guide carries a readiness state
// chip: the policy refusal is capabilities, never a readiness verdict, so
// the guide neither blocks nor reads as violated — the retired
// context/policy row's non-blocking, unproven posture (SI-368 (3)).
func expectNoReadinessState(t *testing.T, label, guide string) {
	t.Helper()
	if strings.Contains(guide, "readiness-state--") || strings.Contains(guide, "readiness-concern--") {
		t.Errorf("%s: the guide carries a readiness state:\n%s", label, guide)
	}
}

// TestPolicyConcern_WitnessCarriesSinglePrefix is the exact regression for
// the UAT-observed readiness panel defect (ac-5, spec/uat-round-1, co-1):
// the refusal is quoted as Code + ": " + Detail. Before the fix, designapp
// forwarded draftmutation's own Error() form as Detail, so the
// concatenation doubled the prefix to "policy-forbidden: policy-forbidden:
// ...". The context/policy row that quoted it is retired; its home is the
// policy guide (SI-368 (3), (32)), whose quote, chosen through
// policyGuideFor from the consultation's own failure, is pinned to its
// single-prefix exact string for both policy-forbidden discriminants
// (genuine non-adoption and adopted-but-no-design_assistance), not merely
// a substring.
func TestPolicyConcern_WitnessCarriesSinglePrefix(t *testing.T) {
	for _, tc := range []struct {
		name   string
		detail string
		want   string
	}{
		{
			name:   "not-adopted",
			detail: "project has not adopted policy authority",
			want:   "policy-forbidden: project has not adopted policy authority",
		},
		{
			name:   "no-design-assistance",
			detail: noDesignAssistanceDetail,
			want:   "policy-forbidden: effective policy has no design_assistance authority",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := policyForbiddenInput()
			in.Failure = &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: tc.detail}
			for _, mode := range guideModes() {
				got, n := testIDElementText(wallGuide(in, mode), "asd-policy-guide-report")
				if n != 1 || got != tc.want {
					t.Fatalf("%s: quoted refusal = %q (%d elements), want exactly [%q] (single package prefix)", mode, got, n, tc.want)
				}
			}
		})
	}
}

// TestPolicyGuide_AdoptedPolicyWithoutDesignAssistance_NeverDescribedAsAbsent
// is the negative regression: policy-forbidden with the
// no-design_assistance detail must NOT be rendered as "no policy is
// adopted". The refusal detail is carried verbatim; the guide is the
// general variant (no initial-setup file list, no "no accepted
// .verdi/policy" claim), still read-only, still carrying the complete
// here-document checks, and never a readiness verdict, in every mode.
func TestPolicyGuide_AdoptedPolicyWithoutDesignAssistance_NeverDescribedAsAbsent(t *testing.T) {
	in := noDesignAssistanceInput()
	if g := policyGuideFor(in.Wired, in.Caps, in.Failure); g.Kind != policyGuideNoDesignAssistance {
		t.Fatalf("policyGuideFor kind = %q, want %q", g.Kind, policyGuideNoDesignAssistance)
	}
	for _, mode := range guideModes() {
		t.Run(string(mode), func(t *testing.T) {
			expectNoGuideOnTheWall(t, mode)
			html := wallGuide(in, mode)
			guide := policyGuideSection(t, html)
			expectNoReadinessState(t, string(mode), html)
			if !strings.Contains(guide, `data-policy-guide="no-design-assistance"`) {
				t.Fatalf("%s: guide is not the no-design-assistance variant; got: %s", mode, guide)
			}
			for _, bad := range []string{
				"has not adopted policy authority",
				"No policy authority is adopted",
				"not-applicable",
				"there is no accepted <code>.verdi/policy</code>",
				"Files to author",
				"Files the starter writes",
				"verdi policy adopt",
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

// TestPolicyGuide_NoticeLinksToInlineGuide: the policy-forbidden notice's
// destination is the inline guide itself. The context/policy row that
// linked to it is retired (SI-368 (3), (32)): the Readiness tab carries
// the guide, under its capabilities label and its own id, and it reads
// as no readiness verdict — the row's non-blocking, unproven posture.
func TestPolicyGuide_NoticeLinksToInlineGuide(t *testing.T) {
	in := policyForbiddenInput()
	if g := policyGuideFor(in.Wired, in.Caps, in.Failure); g.Kind != policyGuideNotAdopted {
		t.Fatalf("policyGuideFor kind = %q, want %q", g.Kind, policyGuideNotAdopted)
	}
	for _, mode := range guideModes() {
		html := wallGuide(in, mode)
		if !strings.HasPrefix(html, `<section class="readiness-tab-capabilities" data-testid="readiness-tab-capabilities" aria-label="Capabilities">`) {
			t.Fatalf("%s: the guide is not under the Readiness tab's capabilities label; got: %s", mode, html)
		}
		if !strings.Contains(html, `id="`+policyGuideID+`"`) {
			t.Fatalf("%s: the guide does not carry its own id %q; got: %s", mode, policyGuideID, html)
		}
		expectNoReadinessState(t, string(mode), html)
	}
}

// policyGuideSection extracts the rendered guide's markup.
func policyGuideSection(t *testing.T, html string) string {
	t.Helper()
	start := strings.Index(html, `id="`+policyGuideID+`"`)
	if start < 0 {
		t.Fatalf("no element with id=%q; got: %s", policyGuideID, html)
	}
	end := strings.Index(html[start:], `data-policy-guide-end`)
	if end < 0 {
		t.Fatalf("policy guide has no end marker; got: %s", html[start:])
	}
	return html[start : start+end]
}

// TestPolicyGuide_RendersReadOnlyGuidance pins the guide's content: plain
// summary first (what is missing, why review is blocked), then the one
// setup verb and the files it writes — hand-authoring named only as the
// fallback — the four read-only CLI inspection requests with their real
// operand shape and result fields under expandable technical detail, and
// the proposed/validated-is-not-accepted line — with no control that
// could mutate anything.
func TestPolicyGuide_RendersReadOnlyGuidance(t *testing.T) {
	html := wallGuide(policyForbiddenInput(), modeAuthoring)
	guide := policyGuideSection(t, html)

	for _, want := range []string{
		// what is missing / why blocked, in plain words — scoped truthfully
		// for every mode (a read-only board is not "editing continues")
		"has not adopted policy authority",
		"semantic review",
		policyGuideScopedWording,
		// the files the setup verb writes — the same four a project may
		// author by hand as the fallback — by path, in their ESCAPED form
		// (a raw-prefix check passed even when the browser swallowed
		// <profile-id> as an element)
		".verdi/policy/constitution.md",
		".verdi/policy/profiles/&lt;profile-id&gt;.md",
		".verdi/policy/policies/&lt;name&gt;.md",
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
	// The context/policy row's home is the guide itself, labelled as
	// capabilities in the Readiness tab (SI-368 (3)); the wall carries no
	// row and no link to a guide it does not render.
	if !strings.HasPrefix(html, `<section class="readiness-tab-capabilities" data-testid="readiness-tab-capabilities" aria-label="Capabilities">`) {
		t.Fatalf("the guide is not under the Readiness tab's capabilities label; got: %s", html)
	}
	expectNoGuideOnTheWall(t, modeAuthoring)
}

// visibleText approximates what a browser shows for a markup fragment:
// tags removed, entities decoded. A label written unescaped into <dt>
// (F1) loses its angle-bracket placeholder here exactly as it does in
// the live DOM, where <profile-id> parses as an unknown element.
func visibleText(html string) string {
	return stdhtml.UnescapeString(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(html, ""))
}

// TestPolicyGuide_PlaceholderPathsSurviveRendering (F1): the two
// placeholder file paths must reach the reader intact in BOTH guide
// variants — escaped in the markup, complete in the visible text — not
// as ".verdi/policy/profiles/.md".
func TestPolicyGuide_PlaceholderPathsSurviveRendering(t *testing.T) {
	cases := []struct {
		name  string
		in    policyCaps
		paths []string
	}{
		{"not-adopted", policyForbiddenInput(), []string{".verdi/policy/profiles/<profile-id>.md", ".verdi/policy/policies/<name>.md"}},
		{"no-design-assistance", noDesignAssistanceInput(), []string{".verdi/policy/policies/<name>.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			guide := policyGuideSection(t, wallGuide(tc.in, modeAuthoring))
			text := visibleText(guide)
			for _, p := range tc.paths {
				if !strings.Contains(guide, "<dt>"+stdhtml.EscapeString(p)+"</dt>") {
					t.Errorf("guide markup lacks the escaped label %q; got: %s", stdhtml.EscapeString(p), guide)
				}
				if strings.Contains(guide, "<dt>"+p+"</dt>") {
					t.Errorf("guide writes the placeholder label %q as raw markup", p)
				}
				if !strings.Contains(text, p) {
					t.Errorf("visible guide text lacks %q; visible text: %s", p, text)
				}
			}
		})
	}
}

// policyGuideDefaultBranchAbsence matches any claim that policy is absent
// or unadopted ON THE DEFAULT BRANCH — a fact the refusal cannot prove
// (F2): production resolves .verdi/policy on the serving checkout's
// filesystem, so an older design branch can lack policy the project has
// already accepted. The clause boundary excludes the truthful
// "not accepted: acceptance is the owner's merge to the default branch".
var policyGuideDefaultBranchAbsence = regexp.MustCompile(`(?i)\b(no|not|never|without)\b[^.:;]*\b(accepted|adopted)\b[^.:;]*default branch|default branch[^.:;]*\b(no|not|never|without)\b[^.:;]*\b(accepted|adopted)\b`)

// policyGuideSyncRitual matches invented or automatic synchronization
// commands the guide must never prescribe: bringing a branch up to date is
// the project's own process, not the workbench's.
var policyGuideSyncRitual = regexp.MustCompile(`git (pull|merge|rebase|fetch)`)

// TestPolicyGuide_PolicyLessCheckoutIsInspectFirst (F2): the not-adopted
// refusal proves absence in the SERVING CHECKOUT only. The guide — the
// home of the retired context/policy row, which said the same (SI-368
// (3), (32)) — must say so, direct the reader to inspect the accepted and
// proposed snapshots FIRST, before any adoption step, keep the
// initial-setup files conditional on confirmed absence, ask why the
// checkout lacks the policy without inferring a cause, and never assert
// absence on the default branch, state project-wide non-adoption as its
// own fact, or prescribe an auto pull/merge — in every mode.
func TestPolicyGuide_PolicyLessCheckoutIsInspectFirst(t *testing.T) {
	for _, mode := range guideModes() {
		t.Run(string(mode), func(t *testing.T) {
			expectNoGuideOnTheWall(t, mode)
			guide := policyGuideSection(t, wallGuide(policyForbiddenInput(), mode))
			text := visibleText(guide)
			if m := policyGuideDefaultBranchAbsence.FindString(text); m != "" {
				t.Errorf("%s: guide asserts default-branch absence: %q", mode, m)
			}
			for _, claim := range []string{"This project has not adopted policy authority", "No policy authority is adopted"} {
				if strings.Contains(text, claim) {
					t.Errorf("%s: guide states project-wide non-adoption the checkout refusal cannot prove: %q", mode, claim)
				}
			}
			if policyGuideSyncRitual.MatchString(text) {
				t.Errorf("%s: guide prescribes a synchronization command: %q", mode, policyGuideSyncRitual.FindString(text))
			}
			// The refusal detail stays visible verbatim, as the discriminant.
			if !strings.Contains(text, "project has not adopted policy authority") {
				t.Errorf("%s: guide lost the verbatim refusal detail", mode)
			}
			// Checkout-scoped fact, inspect-first, conditional setup.
			for _, want := range []string{"checkout", "accepted.adopted", "proposed.adopted", "Inspect first", "Only when"} {
				if !strings.Contains(text, want) {
					t.Errorf("%s: guide text lacks %q; got: %s", mode, want, text)
				}
			}
			inspectAt := strings.Index(text, "Inspect first")
			filesAt := strings.Index(text, "Files the starter writes")
			if filesAt < 0 || inspectAt < 0 || filesAt < inspectAt {
				t.Errorf("%s: inspect-first (%d) must precede the conditional file list (%d)", mode, inspectAt, filesAt)
			}
			if adoptAt := strings.Index(text, "verdi policy adopt"); adoptAt < 0 || adoptAt < inspectAt {
				t.Errorf("%s: inspect-first (%d) must precede any adoption step (%d)", mode, inspectAt, adoptAt)
			}
			// Lagging-branch case: existing project process, never a ritual,
			// and never an inferred cause.
			if !strings.Contains(text, "project's own process") {
				t.Errorf("%s: guide does not defer branch synchronization to the project's own process", mode)
			}
			if !strings.Contains(text, "inspect why this checkout lacks the accepted policy") {
				t.Errorf("%s: guide must ask why the checkout lacks accepted policy; got: %s", mode, text)
			}
			for _, cause := range []string{"this branch lags", "branch lags", "branch may lag"} {
				if strings.Contains(text, cause) {
					t.Errorf("%s: guide infers a cause (%q) for the checkout's missing policy", mode, cause)
				}
			}
			// Review's refusal is tied to this checkout RESOLVING governing
			// policy; loading or editing proposed files is not acceptance.
			if !strings.Contains(text, "until this checkout resolves governing policy") || strings.Contains(text, "carries an accepted one") {
				t.Errorf("%s: guide must tie review's refusal to resolved governing policy, not to a loaded store; got: %s", mode, text)
			}
			if !strings.Contains(text, "Loading or editing proposed policy files is not acceptance") {
				t.Errorf("%s: guide must state that loading/editing proposed files is not acceptance", mode)
			}
		})
	}
}

// TestPolicyConcern_RowsHonorBoardMode (F3; SI-368 (32) B1): BOTH
// policy-forbidden variants scope their editing claim to the board's
// mode, now in the policy guide, the retired context/policy row's home.
// On a read-only or review board no browser edit proceeds, so the guide
// never says editing "proceeds" there and says the board refuses browser
// writes; on an authoring board it says browser editing proceeds and
// names what a write records — the explicit not-applicable posture, or
// the resolved policy's sealed digest. The discriminating detail stays,
// and the no-design-assistance guide never describes the adopted policy
// as absent, in every mode.
func TestPolicyConcern_RowsHonorBoardMode(t *testing.T) {
	kinds := []struct {
		name      string
		detail    string
		keep      string
		authoring string
	}{
		{"not-adopted", "project has not adopted policy authority", "no adopted policy authority",
			"On this authoring board, browser editing proceeds and records the explicit not-applicable policy posture."},
		{"no-design-assistance", noDesignAssistanceDetail, "effective policy has no design_assistance authority",
			"On this authoring board, browser editing proceeds under that policy's sealed digest."},
	}
	refusals := map[boardModeKind]string{
		modeReview:   "This review board refuses browser writes regardless.",
		modeReadOnly: "This read-only board refuses browser writes regardless.",
	}
	for _, k := range kinds {
		for _, mode := range guideModes() {
			t.Run(k.name+"/"+string(mode), func(t *testing.T) {
				in := policyForbiddenInput()
				in.Failure = &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: k.detail}
				guide := policyGuideSection(t, wallGuide(in, mode))
				text := visibleText(guide)
				if !strings.Contains(text, k.keep) {
					t.Errorf("guide lost its discriminating detail %q: %s", k.keep, text)
				}
				if k.name == "no-design-assistance" {
					for _, bad := range []string{"No policy authority is adopted", "has not adopted", "not-applicable"} {
						if strings.Contains(text, bad) {
							t.Errorf("guide describes an adopted policy as absent (%q): %s", bad, text)
						}
					}
				}
				line, n := testIDElementText(guide, "asd-policy-guide-editing")
				if n != 1 {
					t.Fatalf("guide carries %d editing lines, want exactly 1: %s", n, guide)
				}
				if !strings.Contains(guide, `data-testid="asd-policy-guide-editing" data-board-mode="`+string(mode)+`"`) {
					t.Errorf("the editing line does not name the board's mode %s: %s", mode, guide)
				}
				if mode == modeAuthoring {
					if line != k.authoring {
						t.Errorf("authoring editing line = %q, want %q", line, k.authoring)
					}
					return
				}
				if line != refusals[mode] {
					t.Errorf("%s editing line = %q, want %q", mode, line, refusals[mode])
				}
				if strings.Contains(text, "proceeds") {
					t.Errorf("%s: the guide claims editing proceeds on a board that refuses writes: %s", mode, text)
				}
			})
		}
	}
}

// TestPolicyGuide_AbsentWhenPolicyAdopted: a wall with adopted policy
// authority renders no setup guide, on the wall or in its Readiness tab,
// in any mode.
func TestPolicyGuide_AbsentWhenPolicyAdopted(t *testing.T) {
	for _, mode := range guideModes() {
		if html := wallGuide(adoptedPolicyInput(), mode); html != "" {
			t.Fatalf("%s: policy guide rendered on an adopted-policy wall; got: %s", mode, html)
		}
		expectNoGuideOnTheWall(t, mode)
	}
}

// TestPolicyGuide_ReadOnlyAndReviewModes_KeepRestrictions: the guide is
// read-only markup on a read-only or review board — it adds no control
// there either, and never says editing proceeds.
func TestPolicyGuide_ReadOnlyAndReviewModes_KeepRestrictions(t *testing.T) {
	for _, mode := range []boardModeKind{modeReadOnly, modeReview} {
		t.Run(string(mode), func(t *testing.T) {
			expectNoGuideOnTheWall(t, mode)
			guide := policyGuideSection(t, wallGuide(policyForbiddenInput(), mode))
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
			if strings.Contains(guide, policyGuideFalseWording) || strings.Contains(visibleText(guide), "proceeds") {
				t.Fatalf("%s: policy guide claims editing continues on a %s board", mode, mode)
			}
			if n := strings.Count(guide, "\nJSON</code></pre>"); n != 4 {
				t.Fatalf("%s: policy guide closes %d here-documents, want 4", mode, n)
			}
		})
	}
}

// TestPolicyGuide_NamesTheAdoptVerb (spec/spec-documents ac-10, R-W4-8):
// the not-adopted guide's "no setup wizard" disclaimer becomes a pointer
// at the real verb, verdi policy adopt --starter, while the guide stays
// read-only markup with its four read-only check blocks; the verb is a
// pointer, not a fifth here-document.
func TestPolicyGuide_NamesTheAdoptVerb(t *testing.T) {
	guide := policyGuideSection(t, wallGuide(policyForbiddenInput(), modeAuthoring))
	for _, want := range []string{"verdi policy adopt --starter [--profile solo|team]", "policy/adopt", "no adoption control", "not accepted"} {
		if !strings.Contains(guide, want) {
			t.Fatalf("guide missing %q", want)
		}
	}
	if strings.Contains(guide, "no setup wizard") {
		t.Fatal("the no-wizard disclaimer survived")
	}
	if n := strings.Count(guide, `<pre class="asd-policy-guide-cmd">`); n != 4 {
		t.Fatalf("read-only check blocks = %d, want 4 (adopt is a pointer, not a here-doc)", n)
	}
}

// TestPolicyGuideFor (SI-368 (3); F3 survey §2 (d)): the policy setup
// guide's variant is a function of the capabilities consultation alone —
// none when the design service is unwired or capabilities were derived,
// not-adopted only for draftmutation's own not-adopted discriminant,
// no-design-assistance for every other policy-forbidden refusal, and
// none for any other failure — quoting the refusal's code and bare
// detail; and the guide the Readiness tab renders is that function's,
// the variant and the quote, in every mode.
func TestPolicyGuideFor(t *testing.T) {
	notAdopted := &DesignFailure{Code: "policy-forbidden", Detail: "the " + policyNotAdoptedDetail + " for this checkout"}
	noDA := &DesignFailure{Code: "policy-forbidden", Detail: "the sealed effective policy carries no design_assistance payload"}
	other := &DesignFailure{Code: "operational", Detail: "capabilities unavailable"}
	for _, tc := range []struct {
		name    string
		wired   bool
		caps    *DesignCapabilitiesView
		failure *DesignFailure
		want    policyGuide
	}{
		{"unwired", false, nil, notAdopted, policyGuide{}},
		{"capabilities derived", true, &DesignCapabilitiesView{PolicyMode: "assist"}, nil, policyGuide{}},
		{"not adopted", true, nil, notAdopted, policyGuide{Kind: policyGuideNotAdopted, Code: notAdopted.Code, Detail: notAdopted.Detail}},
		{"no design assistance", true, nil, noDA, policyGuide{Kind: policyGuideNoDesignAssistance, Code: noDA.Code, Detail: noDA.Detail}},
		{"another failure", true, nil, other, policyGuide{}},
		{"no consultation at all", true, nil, nil, policyGuide{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := policyGuideFor(tc.wired, tc.caps, tc.failure)
			if got != tc.want {
				t.Fatalf("policyGuideFor = %+v, want %+v", got, tc.want)
			}
			for _, mode := range guideModes() {
				html := wallGuide(policyCaps{Wired: tc.wired, Caps: tc.caps, Failure: tc.failure}, mode)
				if tc.want.Kind == policyGuideNone {
					if html != "" {
						t.Fatalf("%s: the tab renders a guide the function did not choose: %s", mode, html)
					}
					continue
				}
				if !strings.Contains(html, `data-policy-guide="`+string(tc.want.Kind)+`"`) {
					t.Fatalf("%s: the tab's guide is not the function's %s variant: %s", mode, tc.want.Kind, html)
				}
				if quote, n := testIDElementText(html, "asd-policy-guide-report"); n != 1 || quote != tc.want.Code+": "+tc.want.Detail {
					t.Fatalf("%s: the tab's guide quotes %q (%d), want the function's %q", mode, quote, n, tc.want.Code+": "+tc.want.Detail)
				}
			}
		})
	}
}
