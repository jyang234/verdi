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
			// The quote is the refusal's, whatever the wall's mode.
			for _, mode := range guideModes() {
				var b strings.Builder
				writePolicySetupGuide(&b, tc.kind, tc.code, tc.detail, mode)
				html := b.String()
				got, n := testIDElementText(html, "asd-policy-guide-report")
				if n != 1 {
					t.Fatalf("%s: asd-policy-guide-report elements = %d, want exactly 1; guide: %s", mode, n, html)
				}
				if got != tc.want {
					t.Fatalf("%s: reported quote = %q, want %q", mode, got, tc.want)
				}
				if strings.Contains(html, "policy-forbidden: policy-forbidden") {
					t.Fatalf("%s: guide doubles the package prefix (ac-5): %s", mode, html)
				}
				if strings.Contains(got, ": :") || strings.HasPrefix(got, ": ") {
					t.Fatalf("%s: reported quote %q carries a dangling separator", mode, got)
				}
			}
		})
	}
}

// TestDeriveASDShell_PolicyCodeKeepsDetailBare pins the view-model half
// of R-5, now held by policyGuideFor, the function the Readiness tab's
// guide is chosen by (the wall shell that carried it is retired; SI-368
// (3), (32)): the refusal's code is carried separately (Code) while
// Detail stays the BARE detail — policyGuideFor's
// strings.Contains(..., policyNotAdoptedDetail) discriminant and every
// verbatim-detail assertion depend on it — for both policy-forbidden
// variants, and neither is carried when no policy refusal is present.
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
			in.Failure = &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: tc.detail}
			guide := policyGuideFor(in.Wired, in.Caps, in.Failure)
			if guide.Kind != tc.wantKind {
				t.Fatalf("Kind = %q, want %q", guide.Kind, tc.wantKind)
			}
			if guide.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", guide.Code, tc.wantCode)
			}
			if guide.Detail != tc.wantDetail {
				t.Errorf("Detail = %q, want the bare detail %q", guide.Detail, tc.wantDetail)
			}
			// The rendered guide carries the combined quote exactly once,
			// where the wall carries it: the Readiness tab (SI-368 (3)).
			for _, mode := range guideModes() {
				got, n := testIDElementText(wallGuide(in, mode), "asd-policy-guide-report")
				if n != 1 || got != tc.wantCode+": "+tc.wantDetail {
					t.Errorf("%s: rendered report = %q (%d elements), want %q once", mode, got, n, tc.wantCode+": "+tc.wantDetail)
				}
			}
		})
	}
	t.Run("no policy refusal carries no code", func(t *testing.T) {
		in := adoptedPolicyInput()
		if guide := policyGuideFor(in.Wired, in.Caps, in.Failure); guide != (policyGuide{}) {
			t.Fatalf("adopted policy carries guide state: kind=%q code=%q detail=%q", guide.Kind, guide.Code, guide.Detail)
		}
	})
}
