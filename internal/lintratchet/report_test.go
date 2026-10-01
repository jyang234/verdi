package lintratchet

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// reportJSON wraps issues (a JSON array body) in golangci-lint v2.5.0's
// report envelope, with a Report section shaped like the real one.
func reportJSON(issues string) string {
	return `{"Issues":[` + issues + `],"Report":{"Linters":[{"Name":"containedctx","Enabled":true},{"Name":"dupl"}]}}` + "\n"
}

// issueJSON renders one issue the way golangci-lint v2.5.0 writes it.
func issueJSON(linter, text, file, source string, line int) string {
	return `{"FromLinter":` + jsonString(linter) + `,"Text":` + jsonString(text) + `,"Severity":"","SourceLines":[` + jsonString(source) + `],` +
		`"Pos":{"Filename":` + jsonString(file) + `,"Offset":10,"Line":` + strconv.Itoa(line) + `,"Column":2},"ExpectNoLint":false,"ExpectedNoLintLinter":""}`
}

// jsonString renders s as a JSON string literal.
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// TestParseReport covers the strict decode of golangci-lint's JSON report:
// each finding's key is (linter, package, message, flagged source line),
// where the package is the file's repository-relative directory (SI-311),
// and anything the pinned schema does not fix, or any sign the run did not
// analyze the code, is refused.
func TestParseReport(t *testing.T) {
	fix := `{"FromLinter":"errorlint","Text":"non-wrapping format verb","Severity":"","SourceLines":["\treturn fmt.Errorf(\"x: %v\", err)"],` +
		`"Pos":{"Filename":"internal/a/a.go","Offset":146,"Line":7,"Column":35},` +
		`"SuggestedFixes":[{"Message":"Use %w","TextEdits":[{"Pos":142,"End":143,"NewText":"dw=="}]}],` +
		`"LineRange":{"From":7,"To":7},"HunkPos":0,"ExpectNoLint":false,"ExpectedNoLintLinter":""}`
	cases := []struct {
		name    string
		report  string
		want    []Finding
		wantErr string
	}{
		{
			name:   "no findings",
			report: reportJSON(""),
			want:   nil,
		},
		{
			name:   "one finding keyed by its directory, message, and source line",
			report: reportJSON(issueJSON("gochecknoglobals", "counter is a global variable", "internal/a/a.go", "var counter int", 4)),
			want: []Finding{{
				Key:  Key{Linter: "gochecknoglobals", Package: "internal/a", Message: "counter is a global variable", Source: "var counter int"},
				File: "internal/a/a.go", Line: 4,
			}},
		},
		{
			name:   "a file at the repository root is in package .",
			report: reportJSON(issueJSON("noctx", "net/http.Get must not be called", "main.go", "\thttp.Get(u)", 9)),
			want: []Finding{{
				Key:  Key{Linter: "noctx", Package: ".", Message: "net/http.Get must not be called", Source: "\thttp.Get(u)"},
				File: "main.go", Line: 9,
			}},
		},
		{
			name: "several source lines are joined by newlines",
			report: reportJSON(`{"FromLinter":"containedctx","Text":"found a struct that contains a context.Context field","Severity":"","SourceLines":["\tctx context.Context","\tn int"],` +
				`"Pos":{"Filename":"internal/b/b.go","Offset":1,"Line":3,"Column":2},"ExpectNoLint":false,"ExpectedNoLintLinter":""}`),
			want: []Finding{{
				Key:  Key{Linter: "containedctx", Package: "internal/b", Message: "found a struct that contains a context.Context field", Source: "\tctx context.Context\n\tn int"},
				File: "internal/b/b.go", Line: 3,
			}},
		},
		{
			name:   "every field the pinned schema writes is accepted",
			report: reportJSON(fix),
			want: []Finding{{
				Key:  Key{Linter: "errorlint", Package: "internal/a", Message: "non-wrapping format verb", Source: "\treturn fmt.Errorf(\"x: %v\", err)"},
				File: "internal/a/a.go", Line: 7,
			}},
		},
		{
			name:   "report warnings are accepted",
			report: `{"Issues":[],"Report":{"Warnings":[{"Tag":"runner","Text":"a warning"}],"Linters":[{"Name":"noctx","Enabled":true}]}}`,
			want:   nil,
		},
		{
			name: "two findings on one line stay two findings",
			report: reportJSON(issueJSON("errorlint", "non-wrapping format verb", "internal/a/a.go", "\treturn fmt.Errorf(\"%v\", f())", 5) + "," +
				issueJSON("contextcheck", "Function `f` should pass the context parameter", "internal/a/a.go", "\treturn fmt.Errorf(\"%v\", f())", 5)),
			want: []Finding{
				{Key: Key{Linter: "errorlint", Package: "internal/a", Message: "non-wrapping format verb", Source: "\treturn fmt.Errorf(\"%v\", f())"}, File: "internal/a/a.go", Line: 5},
				{Key: Key{Linter: "contextcheck", Package: "internal/a", Message: "Function `f` should pass the context parameter", Source: "\treturn fmt.Errorf(\"%v\", f())"}, File: "internal/a/a.go", Line: 5},
			},
		},

		{name: "empty input", report: "", wantErr: "report"},
		{name: "a truncated report", report: reportJSON(issueJSON("noctx", "m", "a/a.go", "s", 1))[:60], wantErr: "report"},
		{name: "trailing data", report: reportJSON("") + `{"Issues":[]}`, wantErr: "trailing data"},
		{name: "not an object", report: `[]`, wantErr: "report"},
		{name: "no Issues", report: `{"Report":{}}`, wantErr: "Issues"},
		{name: "null Issues", report: `{"Issues":null,"Report":{}}`, wantErr: "Issues"},
		{name: "no Report", report: `{"Issues":[]}`, wantErr: "Report"},
		{name: "an unknown top-level field", report: `{"Issues":[],"Report":{},"Extra":1}`, wantErr: "Extra"},
		{name: "an unknown issue field", report: reportJSON(`{"FromLinter":"noctx","Text":"m","Severity":"","SourceLines":["s"],"Pos":{"Filename":"a/a.go","Offset":0,"Line":1,"Column":1},"ExpectNoLint":false,"ExpectedNoLintLinter":"","Fixed":true}`), wantErr: "Fixed"},
		{name: "an unknown position field", report: reportJSON(`{"FromLinter":"noctx","Text":"m","Severity":"","SourceLines":["s"],"Pos":{"Filename":"a/a.go","Offset":0,"Line":1,"Column":1,"Path":"x"},"ExpectNoLint":false,"ExpectedNoLintLinter":""}`), wantErr: "Path"},
		{name: "an unknown report field", report: `{"Issues":[],"Report":{"Linters":[],"Mode":"x"}}`, wantErr: "Mode"},
		{name: "an unknown linter-data field", report: `{"Issues":[],"Report":{"Linters":[{"Name":"noctx","Enabled":true,"Default":true}]}}`, wantErr: "Default"},
		{name: "an unknown suggested-fix field", report: reportJSON(strings.Replace(fix, `"Message":"Use %w"`, `"Message":"Use %w","Kind":1`, 1)), wantErr: "Kind"},
		{name: "a run error", report: `{"Issues":[],"Report":{"Error":"context loading failed"}}`, wantErr: "context loading failed"},
		{name: "no linter", report: reportJSON(issueJSON("", "m", "a/a.go", "s", 1)), wantErr: "linter"},
		{name: "no message", report: reportJSON(issueJSON("noctx", "", "a/a.go", "s", 1)), wantErr: "message"},
		{name: "no file", report: reportJSON(issueJSON("noctx", "m", "", "s", 1)), wantErr: "file"},
		{name: "an absolute file", report: reportJSON(issueJSON("noctx", "m", "/home/u/verdi/a/a.go", "s", 1)), wantErr: "repository-relative"},
		{name: "a file outside the repository", report: reportJSON(issueJSON("noctx", "m", "../other/a.go", "s", 1)), wantErr: "repository-relative"},
		{name: "a typecheck failure", report: reportJSON(issueJSON("typecheck", ": # example.com/x\n./b.go:3:23: undefined: y", "b.go", "package x", 1)), wantErr: "could not load"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseReport([]byte(tc.report))
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseReport = %+v, nil error; want an error containing %q", got, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ParseReport error %q does not contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseReport: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("ParseReport =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}

// TestCountFindings counts findings per key: identical findings repeat
// within a package, so a key's count, not its presence, is what the
// baseline allows (strict-lint-target-v2 dc-3).
func TestCountFindings(t *testing.T) {
	a := Key{Linter: "errorlint", Package: "p", Message: "m", Source: "s"}
	b := Key{Linter: "noctx", Package: "p", Message: "m", Source: "s"}
	cases := []struct {
		name     string
		findings []Finding
		want     Counts
	}{
		{name: "none", findings: nil, want: Counts{}},
		{name: "one", findings: []Finding{{Key: a, File: "p/x.go", Line: 1}}, want: Counts{a: 1}},
		{name: "a repeated key on other lines and files", findings: []Finding{{Key: a, File: "p/x.go", Line: 1}, {Key: a, File: "p/y.go", Line: 9}, {Key: b, File: "p/x.go", Line: 1}}, want: Counts{a: 2, b: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CountFindings(tc.findings)
			if len(got) != len(tc.want) {
				t.Fatalf("CountFindings = %v, want %v", got, tc.want)
			}
			for k, n := range tc.want {
				if got[k] != n {
					t.Fatalf("CountFindings[%+v] = %d, want %d (all: %v)", k, got[k], n, got)
				}
			}
		})
	}
}
