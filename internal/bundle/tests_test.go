package bundle

import (
	"strings"
	"testing"
)

// realGoTestJSON is a trimmed, real `go test -json` capture from
// testdata/svcfix's own suite (internal/app's TestRefundFlow and
// TestGetRefund, both passing) — not hand-authored.
const realGoTestJSON = `
{"Action":"start","Package":"example.com/svcfix/internal/app"}
{"Action":"run","Package":"example.com/svcfix/internal/app","Test":"TestRefundFlow"}
{"Action":"output","Package":"example.com/svcfix/internal/app","Test":"TestRefundFlow","Output":"=== RUN   TestRefundFlow\n"}
{"Action":"pass","Package":"example.com/svcfix/internal/app","Test":"TestRefundFlow","Elapsed":0}
{"Action":"run","Package":"example.com/svcfix/internal/app","Test":"TestGetRefund"}
{"Action":"pass","Package":"example.com/svcfix/internal/app","Test":"TestGetRefund","Elapsed":0}
{"Action":"pass","Package":"example.com/svcfix/internal/app","Elapsed":0.288}
`

func TestSummarizeGoTestJSON_Happy(t *testing.T) {
	s, err := SummarizeGoTestJSON(strings.NewReader(realGoTestJSON))
	if err != nil {
		t.Fatalf("SummarizeGoTestJSON: %v", err)
	}
	if s.Suite != "pass" {
		t.Errorf("Suite = %q, want pass", s.Suite)
	}
	if len(s.Packages) != 1 {
		t.Fatalf("Packages = %+v, want 1 entry", s.Packages)
	}
	pkg := s.Packages[0]
	if pkg.Package != "example.com/svcfix/internal/app" || pkg.Status != "pass" || pkg.Tests != 2 || pkg.Failures != 0 {
		t.Fatalf("Packages[0] = %+v, want {app pass 2 0}", pkg)
	}
}

func TestSummarizeGoTestJSON_Failure(t *testing.T) {
	const stream = `
{"Action":"start","Package":"example.com/svcfix/internal/app"}
{"Action":"run","Package":"example.com/svcfix/internal/app","Test":"TestRefundFlow"}
{"Action":"fail","Package":"example.com/svcfix/internal/app","Test":"TestRefundFlow","Elapsed":0}
{"Action":"fail","Package":"example.com/svcfix/internal/app","Elapsed":0.1}
{"Action":"start","Package":"example.com/svcfix/internal/bus"}
{"Action":"skip","Package":"example.com/svcfix/internal/bus","Elapsed":0}
`
	s, err := SummarizeGoTestJSON(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("SummarizeGoTestJSON: %v", err)
	}
	if s.Suite != "fail" {
		t.Errorf("Suite = %q, want fail", s.Suite)
	}
	if len(s.Packages) != 2 {
		t.Fatalf("Packages = %+v, want 2 entries", s.Packages)
	}
	if s.Packages[0].Status != "fail" || s.Packages[0].Failures != 1 {
		t.Errorf("Packages[0] = %+v, want status fail, 1 failure", s.Packages[0])
	}
	if s.Packages[1].Status != "skip" {
		t.Errorf("Packages[1] = %+v, want status skip", s.Packages[1])
	}
}

func TestSummarizeGoTestJSON_SkipOnlyDoesNotFailSuite(t *testing.T) {
	const stream = `
{"Action":"start","Package":"example.com/svcfix"}
{"Action":"skip","Package":"example.com/svcfix","Elapsed":0}
`
	s, err := SummarizeGoTestJSON(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("SummarizeGoTestJSON: %v", err)
	}
	if s.Suite != "pass" {
		t.Errorf("Suite = %q, want pass (a skip-only package must not fail the suite)", s.Suite)
	}
}

func TestSummarizeGoTestJSON_Negative(t *testing.T) {
	cases := []struct {
		name   string
		stream string
	}{
		{"empty", ""},
		{"not json", "not json at all"},
		{"missing package", `{"Action":"pass"}`},
		{"truncated (no terminal event)", `{"Action":"start","Package":"p"}` + "\n" + `{"Action":"run","Package":"p","Test":"T"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := SummarizeGoTestJSON(strings.NewReader(tc.stream)); err == nil {
				t.Fatalf("SummarizeGoTestJSON(%s): want error, got nil", tc.name)
			}
		})
	}
}
