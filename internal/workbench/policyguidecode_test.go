package workbench

import (
	"strings"
	"testing"
)

// TestPolicyGuide_ReportQuotesCodeAndDetail (wave-1 ledger R-5, after the
// ac-5 single-prefix fix): the guide's "The workbench reported:" quote
// carries the refusal's code AND its bare detail as code + ": " + detail
// — the same single-prefix form the context/policy row's witness uses —
// so the reader sees which refusal the workbench raised, not a bare
// sentence. A missing code (negative path) degrades to the bare detail,
// never to a dangling ": " prefix.
func TestPolicyGuide_ReportQuotesCodeAndDetail(t *testing.T) {
	cases := []struct {
		name   string
		kind   policyGuideKind
		code   string
		detail string
		want   string
	}{
		{
			name:   "not-adopted variant quotes code and detail",
			kind:   policyGuideNotAdopted,
			code:   "policy-forbidden",
			detail: policyNotAdoptedDetail,
			want:   "policy-forbidden: project has not adopted policy authority",
		},
		{
			name:   "no-design-assistance variant quotes code and detail",
			kind:   policyGuideNoDesignAssistance,
			code:   "policy-forbidden",
			detail: noDesignAssistanceDetail,
			want:   "policy-forbidden: effective policy has no design_assistance authority",
		},
		{
			name:   "missing code degrades to the bare detail",
			kind:   policyGuideNotAdopted,
			detail: policyNotAdoptedDetail,
			want:   policyNotAdoptedDetail,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writePolicySetupGuide(&b, tc.kind, tc.code, tc.detail)
			html := b.String()
			got, n := testIDElementText(html, "asd-policy-guide-report")
			if n != 1 {
				t.Fatalf("asd-policy-guide-report elements = %d, want exactly 1; guide: %s", n, html)
			}
			if got != tc.want {
				t.Fatalf("reported quote = %q, want %q", got, tc.want)
			}
			if strings.Contains(html, "policy-forbidden: policy-forbidden") {
				t.Fatalf("guide doubles the package prefix (ac-5): %s", html)
			}
			if strings.Contains(got, ": :") || strings.HasPrefix(got, ": ") {
				t.Fatalf("reported quote %q carries a dangling separator", got)
			}
		})
	}
}

// TestDeriveASDShell_PolicyCodeKeepsDetailBare pins the view-model half
// of R-5: the shell carries the refusal's code separately (PolicyCode)
// while PolicyDetail stays the BARE detail — boardspecasd.go's
// strings.Contains(..., policyNotAdoptedDetail) discriminant and every
// verbatim-detail assertion depend on it — for both policy-forbidden
// variants, and carries neither when no policy refusal is present.
func TestDeriveASDShell_PolicyCodeKeepsDetailBare(t *testing.T) {
	cases := []struct {
		name       string
		detail     string
		wantKind   policyGuideKind
		wantCode   string
		wantDetail string
	}{
		{"not-adopted", policyNotAdoptedDetail, policyGuideNotAdopted, "policy-forbidden", policyNotAdoptedDetail},
		{"no-design-assistance", noDesignAssistanceDetail, policyGuideNoDesignAssistance, "policy-forbidden", noDesignAssistanceDetail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := policyForbiddenInput()
			in.CapsFailure = &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: tc.detail}
			shell := deriveASDShell(in)
			if shell.PolicySetupGuide != tc.wantKind {
				t.Fatalf("PolicySetupGuide = %q, want %q", shell.PolicySetupGuide, tc.wantKind)
			}
			if shell.PolicyCode != tc.wantCode {
				t.Errorf("PolicyCode = %q, want %q", shell.PolicyCode, tc.wantCode)
			}
			if shell.PolicyDetail != tc.wantDetail {
				t.Errorf("PolicyDetail = %q, want the bare detail %q", shell.PolicyDetail, tc.wantDetail)
			}
			// The rendered board carries the combined quote exactly once.
			v := testASDView()
			v.Shell = shell
			html := renderBoardRegion(badgeRenderProjection(modeAuthoring), &boardGitState{Branch: "design/x", DefaultBranch: "main"}, v)
			got, n := testIDElementText(html, "asd-policy-guide-report")
			if n != 1 || got != tc.wantCode+": "+tc.wantDetail {
				t.Errorf("rendered report = %q (%d elements), want %q once", got, n, tc.wantCode+": "+tc.wantDetail)
			}
		})
	}
	t.Run("no policy refusal carries no code", func(t *testing.T) {
		in := policyForbiddenInput()
		in.CapsFailure = nil
		in.Caps = &DesignCapabilitiesView{PolicyMode: "proposal-only", PolicyDigest: "sha256:abc", RefusalPrecondition: "policy-mode", RefusalDetail: "mode forbids agent writes"}
		shell := deriveASDShell(in)
		if shell.PolicySetupGuide != "" || shell.PolicyCode != "" || shell.PolicyDetail != "" {
			t.Fatalf("adopted policy shell carries guide state: kind=%q code=%q detail=%q", shell.PolicySetupGuide, shell.PolicyCode, shell.PolicyDetail)
		}
	})
}
