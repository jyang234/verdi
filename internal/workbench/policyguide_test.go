package workbench

import (
	stdhtml "html"
	"regexp"
	"strings"
	"testing"
)

// The policy setup guide: when a wall reports policy-forbidden ("project
// has not adopted policy authority"), the board's context/policy notice
// must lead somewhere usable — an inline, read-only guide on the same
// board naming the one verb that performs initial setup (verdi policy
// adopt --starter), the four files it writes — which a project may
// instead author by hand, the fallback rather than the route
// (constitution, profile, policy, consumer inventory) — and the existing
// read-only CLI inspection requests with their real operand shape and
// result fields. It adopts nothing, mutates nothing, and never presents
// proposed/validated as accepted.

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

func policyForbiddenInput() asdShellInput {
	return asdShellInput{
		ProblemPresent: true,
		OutcomePresent: true,
		ACs:            []asdACFact{{ID: "ac-1", EvidenceCount: 1}},
		Mode:           "authoring",
		Branch:         "design/x",
		StateFormal:    "proposed",
		DesignWired:    true,
		// designapp forwards draftmutation's own BARE Detail here, never its
		// Error() form ("<code>: <detail>") — Code already carries
		// "policy-forbidden" separately, so embedding it again in Detail
		// would double the prefix once a caller renders "Code: Detail"
		// (ac-5, spec/uat-round-1; internal/designapp/outcome.go's
		// translateDraftmutationError).
		CapsFailure: &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: "project has not adopted policy authority"},
	}
}

// noDesignAssistanceDetail is ResolvePolicyGrant's OTHER policy-forbidden
// refusal (internal/draftmutation/policy.go): the effective policy
// resolved — adopted and sealed — but carries no design_assistance
// payload. Same code, materially different meaning. Bare, like
// policyForbiddenInput's Detail above — never the "policy-forbidden: "-
// prefixed Error() form.
const noDesignAssistanceDetail = "effective policy has no design_assistance authority"

func noDesignAssistanceView() *asdView {
	v := testASDView()
	in := policyForbiddenInput()
	in.CapsFailure = &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: noDesignAssistanceDetail}
	v.Shell = deriveASDShell(in)
	return v
}

// TestPolicyConcern_WitnessCarriesSinglePrefix is the exact regression for
// the UAT-observed board readiness panel defect (ac-5, spec/uat-round-1,
// co-1): the context/policy concern's Witnesses entry is Code + ": " +
// Detail, rendered verbatim into the board's readiness panel
// (boardshellrender.go's "Witnesses" list). Before the fix, designapp
// forwarded draftmutation's own Error() form as Detail, so this
// concatenation doubled the prefix to "policy-forbidden: policy-forbidden:
// ...". Both policy-forbidden discriminants (genuine non-adoption and
// adopted-but-no-design_assistance) are pinned to their single-prefix
// exact string, not merely a substring match.
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
			in.CapsFailure = &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: tc.detail}
			shell := deriveASDShell(in)
			var row *asdConcern
			for i := range shell.All {
				if shell.All[i].ID == "context/policy" {
					row = &shell.All[i]
				}
			}
			if row == nil {
				t.Fatal("no context/policy concern")
			}
			if len(row.Witnesses) != 1 || row.Witnesses[0] != tc.want {
				t.Fatalf("Witnesses = %v, want exactly [%q] (single package prefix)", row.Witnesses, tc.want)
			}
		})
	}
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
// summary first (what is missing, why review is blocked), then the one
// setup verb and the files it writes — hand-authoring named only as the
// fallback — the four read-only CLI inspection requests with their real
// operand shape and result fields under expandable technical detail, and
// the proposed/validated-is-not-accepted line — with no control that
// could mutate anything.
func TestPolicyGuide_RendersReadOnlyGuidance(t *testing.T) {
	html := renderBoardRegion(badgeRenderProjection(modeAuthoring), &boardGitState{Branch: "design/x", DefaultBranch: "main"}, policyForbiddenView())
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
	// The concern row's destination link resolves to the guide.
	if !strings.Contains(html, `<a class="asd-dest-link" href="#`+policyGuideID+`">`) {
		t.Fatalf("context/policy row has no destination link to #%s; got: %s", policyGuideID, html)
	}
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
		view  *asdView
		paths []string
	}{
		{"not-adopted", policyForbiddenView(), []string{".verdi/policy/profiles/<profile-id>.md", ".verdi/policy/policies/<name>.md"}},
		{"no-design-assistance", noDesignAssistanceView(), []string{".verdi/policy/policies/<name>.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			html := renderBoardRegion(badgeRenderProjection(modeAuthoring), &boardGitState{Branch: "design/x", DefaultBranch: "main"}, tc.view)
			guide := policyGuideSection(t, html)
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
// refusal proves absence in the SERVING CHECKOUT only. The guide and the
// concern row must say so, direct the reader to inspect the accepted and
// proposed snapshots FIRST, keep the initial-setup files conditional on
// confirmed absence, and never assert absence on the default branch or
// prescribe an auto pull/merge.
func TestPolicyGuide_PolicyLessCheckoutIsInspectFirst(t *testing.T) {
	shell := deriveASDShell(policyForbiddenInput())
	var row *asdConcern
	for i := range shell.All {
		if shell.All[i].ID == "context/policy" {
			row = &shell.All[i]
		}
	}
	if row == nil {
		t.Fatal("no context/policy concern")
	}
	for _, s := range []string{row.Summary, row.Guidance} {
		if m := policyGuideDefaultBranchAbsence.FindString(s); m != "" {
			t.Errorf("context/policy asserts default-branch absence (%q) in %q", m, s)
		}
		if strings.Contains(s, "No policy authority is adopted") || strings.Contains(s, "project has not adopted") {
			t.Errorf("context/policy states project-wide non-adoption the checkout refusal cannot prove: %q", s)
		}
		if policyGuideSyncRitual.MatchString(s) {
			t.Errorf("context/policy prescribes a synchronization command: %q", s)
		}
	}
	if !strings.Contains(row.Summary, "checkout") {
		t.Errorf("context/policy summary %q is not scoped to the serving checkout", row.Summary)
	}
	inspect, adopt := strings.Index(row.Guidance, "Inspect"), strings.Index(row.Guidance, "adopt")
	if inspect < 0 || (adopt >= 0 && adopt < inspect) {
		t.Errorf("context/policy guidance %q must direct inspection before any adoption step", row.Guidance)
	}
	// accepted.adopted alone proves neither branch lag nor deletion: the
	// row asks why the checkout lacks the policy and infers no cause.
	if !strings.Contains(row.Guidance, "inspect why this checkout lacks") || strings.Contains(row.Guidance, "branch lags") || strings.Contains(row.Guidance, "branch may lag") {
		t.Errorf("context/policy guidance %q must ask why the checkout lacks accepted policy without inferring a cause", row.Guidance)
	}

	for _, mode := range []boardModeKind{modeAuthoring, modeReadOnly} {
		t.Run(string(mode), func(t *testing.T) {
			html := renderBoardRegion(badgeRenderProjection(mode), &boardGitState{Branch: "design/x", DefaultBranch: "main"}, policyForbiddenView())
			guide := policyGuideSection(t, html)
			text := visibleText(guide)
			if m := policyGuideDefaultBranchAbsence.FindString(text); m != "" {
				t.Errorf("%s: guide asserts default-branch absence: %q", mode, m)
			}
			if strings.Contains(text, "This project has not adopted policy authority") {
				t.Errorf("%s: guide states project-wide non-adoption as its own fact", mode)
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
			// Lagging-branch case: existing project process, never a ritual,
			// and never an inferred cause.
			if !strings.Contains(text, "project's own process") {
				t.Errorf("%s: guide does not defer branch synchronization to the project's own process", mode)
			}
			if !strings.Contains(text, "inspect why this checkout lacks the accepted policy") || strings.Contains(text, "this branch lags") {
				t.Errorf("%s: guide must ask why the checkout lacks accepted policy without inferring a cause; got: %s", mode, text)
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

// TestPolicyConcern_RowsHonorBoardMode (F3): BOTH policy-forbidden concern
// summaries must scope their editing claim to the board's mode. On a
// read-only or review board no browser edit proceeds, so the row must not
// say editing "proceeds"; on an authoring board the row names what a write
// records. The no-design-assistance row keeps the refusal detail verbatim
// and never describes the adopted policy as absent, in every mode.
func TestPolicyConcern_RowsHonorBoardMode(t *testing.T) {
	kinds := []struct {
		name   string
		detail string
		keep   string
	}{
		{"not-adopted", "project has not adopted policy authority", "no adopted policy authority"},
		{"no-design-assistance", noDesignAssistanceDetail, "effective policy has no design_assistance authority"},
	}
	for _, k := range kinds {
		for _, mode := range []boardModeKind{modeAuthoring, modeReview, modeReadOnly} {
			t.Run(k.name+"/"+string(mode), func(t *testing.T) {
				in := policyForbiddenInput()
				in.Mode = string(mode)
				in.CapsFailure = &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: k.detail}
				shell := deriveASDShell(in)
				var row *asdConcern
				for i := range shell.All {
					if shell.All[i].ID == "context/policy" {
						row = &shell.All[i]
					}
				}
				if row == nil {
					t.Fatal("no context/policy concern")
				}
				if !strings.Contains(row.Summary, k.keep) {
					t.Errorf("summary %q lost its discriminating detail %q", row.Summary, k.keep)
				}
				if k.name == "no-design-assistance" {
					for _, bad := range []string{"No policy authority is adopted", "has not adopted", "not-applicable"} {
						if strings.Contains(row.Summary, bad) {
							t.Errorf("summary %q describes an adopted policy as absent (%q)", row.Summary, bad)
						}
					}
				}
				proceeds := strings.Contains(row.Summary, "policy, so browser editing proceeds")
				refuses := strings.Contains(row.Summary, "policy, but this ") && strings.Contains(row.Summary, "board refuses browser writes")
				if mode == modeAuthoring {
					if !proceeds || refuses {
						t.Errorf("authoring summary %q must state that browser editing proceeds here", row.Summary)
					}
				} else if proceeds || !refuses {
					t.Errorf("%s summary %q claims editing proceeds on a board that refuses writes", mode, row.Summary)
				}
				// The rendered row, not only the struct.
				v := testASDView()
				v.Shell = shell
				html := renderBoardRegion(badgeRenderProjection(mode), &boardGitState{Branch: "design/x", DefaultBranch: "main"}, v)
				start := strings.Index(html, `data-concern-id="context/policy"`)
				if start < 0 {
					t.Fatalf("no rendered context/policy row; got: %s", html)
				}
				rowHTML := html[start:]
				rowHTML = rowHTML[:strings.Index(rowHTML, "</article>")]
				if mode != modeAuthoring && strings.Contains(rowHTML, "editing proceeds") {
					t.Errorf("%s: rendered row still claims editing proceeds: %s", mode, rowHTML)
				}
				if !strings.Contains(rowHTML, stdhtml.EscapeString(k.keep)) {
					t.Errorf("%s: rendered row lost detail %q: %s", mode, k.keep, rowHTML)
				}
			})
		}
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

// TestPolicyGuide_NamesTheAdoptVerb (spec/spec-documents ac-10, R-W4-8):
// the not-adopted guide's "no setup wizard" disclaimer becomes a pointer
// at the real verb, verdi policy adopt --starter, while the guide stays
// read-only markup with its four read-only check blocks; the verb is a
// pointer, not a fifth here-document.
func TestPolicyGuide_NamesTheAdoptVerb(t *testing.T) {
	html := renderBoardRegion(badgeRenderProjection(modeAuthoring), &boardGitState{Branch: "design/x", DefaultBranch: "main"}, policyForbiddenView())
	guide := policyGuideSection(t, html)
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
