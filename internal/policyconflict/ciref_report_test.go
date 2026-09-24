package policyconflict

import (
	"bytes"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/repositoryfacts"
)

// SI-257 (as amended), lane E4c: the report's input identity carries the
// sealed CI ref beside repository as the additive, omit-when-absent field
// ci_ref, so every report without one is byte-identical to before.

const reportCIRefBranch = "close/example"

// reportWithCIRef is the golden report on a detached checkout whose CI ref
// supplied the branch being closed: repository.branch unknown, ci_ref set.
func reportWithCIRef(t *testing.T) Report {
	t.Helper()
	report, err := DecodeReport(mustReadFixture(t, "report.json"))
	if err != nil {
		t.Fatalf("DecodeReport(fixture): %v", err)
	}
	report.Input.Repository.Branch = repositoryfacts.StringFact{}
	report.Input.CIRef = &repositoryfacts.CIRefFact{Known: true, Provider: repositoryfacts.CIProviderGitHub, Name: reportCIRefBranch}
	return report
}

func TestReportInputCIRefRoundTrip(t *testing.T) {
	t.Run("absent: the golden report is unchanged and carries no ci_ref", func(t *testing.T) {
		golden := mustReadFixture(t, "report.json")
		report, err := DecodeReport(golden)
		if err != nil {
			t.Fatalf("DecodeReport: %v", err)
		}
		if report.Input.CIRef != nil {
			t.Fatalf("Input.CIRef = %+v, want nil", *report.Input.CIRef)
		}
		out, err := EncodeReport(report)
		if err != nil {
			t.Fatalf("EncodeReport: %v", err)
		}
		if !bytes.Equal(out, golden) || bytes.Contains(out, []byte(`"ci_ref"`)) {
			t.Fatalf("golden report changed or gained ci_ref:\n%s", out)
		}
	})

	t.Run("present: encoded beside repository and decoded exactly", func(t *testing.T) {
		report := reportWithCIRef(t)
		data, err := EncodeReport(report)
		if err != nil {
			t.Fatalf("EncodeReport: %v", err)
		}
		want := []byte(`"ci_ref":{"known":true,"name":"close/example","provider":"github"}`)
		if !bytes.Contains(data, want) {
			t.Fatalf("encoded report lacks %s:\n%s", want, data)
		}
		decoded, err := DecodeReport(data)
		if err != nil {
			t.Fatalf("DecodeReport: %v", err)
		}
		if decoded.Input.CIRef == nil || *decoded.Input.CIRef != *report.Input.CIRef {
			t.Fatalf("decoded Input.CIRef = %v, want %+v", decoded.Input.CIRef, *report.Input.CIRef)
		}
		again, err := EncodeReport(decoded)
		if err != nil || !bytes.Equal(again, data) {
			t.Fatalf("re-encode = %v, byte-identical=%v", err, bytes.Equal(again, data))
		}
		golden, err := DecodeReport(mustReadFixture(t, "report.json"))
		if err != nil {
			t.Fatalf("DecodeReport(fixture): %v", err)
		}
		if decoded.Digest == golden.Digest {
			t.Fatal("a report carrying ci_ref kept the golden digest; the field must be digest-bound")
		}
	})
}

// TestEncodeReportInputCIRefValidation: a present ci_ref must be the known,
// valid CI ref that supplied the branch being closed — so only on an
// accepted-context report whose repository branch is unknown.
func TestEncodeReportInputCIRefValidation(t *testing.T) {
	cases := []struct {
		name string
		mut  func(r *Report)
	}{
		{"zero value present", func(r *Report) { r.Input.CIRef = &repositoryfacts.CIRefFact{} }},
		{"unknown with a reason", func(r *Report) {
			r.Input.CIRef = &repositoryfacts.CIRefFact{Reason: repositoryfacts.CIRefReasonRemoteTrackingNotHead}
		}},
		{"invalid branch name", func(r *Report) { r.Input.CIRef.Name = "main~1" }},
		{"unknown provider", func(r *Report) { r.Input.CIRef.Provider = "jenkins" }},
		{"known with a reason", func(r *Report) { r.Input.CIRef.Reason = repositoryfacts.CIRefReasonNameInvalid }},
		{"beside a known repository branch", func(r *Report) {
			r.Input.Repository.Branch = repositoryfacts.StringFact{Known: true, Value: "main"}
		}},
		{"on a candidate target", func(r *Report) {
			r.Input.Target = TargetIdentity{Kind: TargetAcceptanceCandidate, Candidate: &CandidateIdentity{
				Ref: "spec/example-story", Path: ".verdi/specs/active/example-story.md", Branch: "feature/example",
				Head: "0123456789abcdef0123456789abcdef01234567", Blob: "89abcdef0123456789abcdef0123456789abcdef",
				ContentDigest: sha('a'),
				Scope:         policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
				Adapter:       contextcompile.AdapterRef{ID: "codex", Version: "1"},
				GrantDigest:   sha('b'),
			}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := reportWithCIRef(t)
			tc.mut(&report)
			_, err := EncodeReport(report)
			requireErrContains(t, err, "input.ci_ref")
		})
	}
}

// TestDecodeReportInputCIRefStrict forges each malformed ci_ref wire value
// with the self-digest recomputed, so only the strict decode (unknown
// fields, canonical bytes) and the field's own validation can refuse it.
func TestDecodeReportInputCIRefStrict(t *testing.T) {
	valid, err := EncodeReport(reportWithCIRef(t))
	if err != nil {
		t.Fatalf("EncodeReport: %v", err)
	}
	if _, err := DecodeReport(valid); err != nil {
		t.Fatalf("test setup: DecodeReport(valid ci_ref report): %v", err)
	}
	knownRef := func(extra map[string]any) map[string]any {
		out := map[string]any{"known": true, "provider": "github", "name": reportCIRefBranch}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	cases := map[string]any{
		"unknown field inside ci_ref":    knownRef(map[string]any{"bogus": "x"}),
		"explicit null":                  nil,
		"zero value present":             map[string]any{"known": false},
		"unknown with a reason":          map[string]any{"known": false, "reason": "ci-ref-name-invalid"},
		"invalid branch name":            knownRef(map[string]any{"name": "main~1"}),
		"unknown provider":               knownRef(map[string]any{"provider": "jenkins"}),
		"case-variant key":               map[string]any{"Known": true, "provider": "github", "name": reportCIRefBranch},
		"not an object":                  reportCIRefBranch,
		"unknown reason code, not known": map[string]any{"known": false, "reason": "bogus"},
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			tree := setAtPath(t, valid, []any{"input", "ci_ref"}, value)
			forged := redigestTopLevel(t, tree)
			_, err := DecodeReport(forged)
			if err == nil {
				t.Fatalf("DecodeReport accepted a forged ci_ref %v", value)
			}
			t.Logf("refused: %v", err)
		})
	}

	t.Run("ci_ref beside a known repository branch", func(t *testing.T) {
		tree := setAtPath(t, valid, []any{"input", "repository", "branch"}, map[string]any{"known": true, "value": "main"})
		_, err := DecodeReport(redigestTopLevel(t, tree))
		requireErrContains(t, err, "input.ci_ref")
	})
}
