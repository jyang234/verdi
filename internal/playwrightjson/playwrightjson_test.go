package playwrightjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// capturedReport returns a committed report captured from a real run of the
// pinned Playwright (testdata/capture/capture.sh).
func capturedReport(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "reports", name+".json"))
	if err != nil {
		t.Fatalf("reading captured report %s: %v", name, err)
	}
	return raw
}

// mutated decodes a captured report as generic JSON, applies edit, and
// re-encodes it: the hand-derived variants for the malformed and
// unknown-field cases.
func mutated(t *testing.T, name string, edit func(report map[string]any)) []byte {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(capturedReport(t, name)))
	dec.UseNumber()
	var report map[string]any
	if err := dec.Decode(&report); err != nil {
		t.Fatalf("decoding captured report %s: %v", name, err)
	}
	edit(report)
	out, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("re-encoding %s: %v", name, err)
	}
	return out
}

// obj and arr index generic JSON; each fails the test on a shape mismatch.
func obj(t *testing.T, v any, key string) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("not an object: %v", v)
	}
	child, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("%q is not an object in %v", key, m)
	}
	return child
}

func arr(t *testing.T, v any, key string) []any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("not an object: %v", v)
	}
	child, ok := m[key].([]any)
	if !ok {
		t.Fatalf("%q is not an array in %v", key, m)
	}
	return child
}

// fileSuite returns the outcomes report's file suite for file.
func fileSuite(t *testing.T, report map[string]any, file string) map[string]any {
	t.Helper()
	for _, s := range arr(t, report, "suites") {
		if s.(map[string]any)["file"] == file {
			return s.(map[string]any)
		}
	}
	t.Fatalf("no file suite %q", file)
	return nil
}

// firstTest and firstResult reach the first spec of other.spec.ts's
// "outcomes" describe: a plain passing test with one attempt.
func firstSpec(t *testing.T, report map[string]any) map[string]any {
	t.Helper()
	describe := arr(t, fileSuite(t, report, "other.spec.ts"), "suites")[0]
	return arr(t, describe, "specs")[0].(map[string]any)
}

func firstTest(t *testing.T, report map[string]any) map[string]any {
	t.Helper()
	return arr(t, firstSpec(t, report), "tests")[0].(map[string]any)
}

func firstResult(t *testing.T, report map[string]any) map[string]any {
	t.Helper()
	return arr(t, firstTest(t, report), "results")[0].(map[string]any)
}

// failingResult is the outcomes report's "outcomes › fails" attempt, which
// carries an error, errors, an attachment, and an error location.
func failingResult(t *testing.T, report map[string]any) map[string]any {
	t.Helper()
	for _, d := range arr(t, fileSuite(t, report, "outcomes.spec.ts"), "suites") {
		if d.(map[string]any)["title"] != "outcomes" {
			continue
		}
		for _, s := range arr(t, d, "specs") {
			if s.(map[string]any)["title"] == "fails" {
				return arr(t, arr(t, s, "tests")[0], "results")[0].(map[string]any)
			}
		}
	}
	t.Fatal("no outcomes › fails spec")
	return nil
}

// skippedTest is "outcomes › is skipped declaratively", whose annotation
// carries a location.
func skippedTest(t *testing.T, report map[string]any) map[string]any {
	t.Helper()
	for _, d := range arr(t, fileSuite(t, report, "outcomes.spec.ts"), "suites") {
		if d.(map[string]any)["title"] != "outcomes" {
			continue
		}
		for _, s := range arr(t, d, "specs") {
			if s.(map[string]any)["title"] == "is skipped declaratively" {
				return arr(t, s, "tests")[0].(map[string]any)
			}
		}
	}
	t.Fatal("no outcomes › is skipped declaratively spec")
	return nil
}

// want is one decoded test: its joined title path, expected status, outcome,
// attempt statuses in order, and project ("" for the captures' unnamed one).
type want struct {
	title    string
	expected string
	outcome  string
	attempts []string
	project  string
}

func summarize(f File) []want {
	var out []want
	for _, tc := range f.Tests {
		w := want{title: tc.JoinedTitlePath(), expected: tc.ExpectedStatus, outcome: tc.Outcome, project: tc.ProjectName}
		for i, a := range tc.Attempts {
			if a.Retry != i {
				w.attempts = append(w.attempts, "retry-out-of-order")
				continue
			}
			w.attempts = append(w.attempts, a.Status)
		}
		out = append(out, w)
	}
	return out
}

// TestDecode_CapturedReports proves the decoder reads every captured report of
// the pinned Playwright into the shape the producer maps: per file, each test's
// title path (describe titles, anonymous describes omitted, then the test's own
// title, joined by " › ", with ":" kept verbatim), its expected status, its
// Playwright outcome, and every attempt's own status in retry order; and the
// run's own errors.
func TestDecode_CapturedReports(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		files      map[string][]want
		projects   []string // nil: one unnamed project
		errorCount int
		errorHas   string
	}{
		{name: "outcomes", files: map[string][]want{
			"other.spec.ts": {
				{"outcomes › passes", StatusPassed, OutcomeExpected, []string{StatusPassed}, ""},
			},
			"outcomes.spec.ts": {
				{"passes at the top level", StatusPassed, OutcomeExpected, []string{StatusPassed}, ""},
				{"outcomes › passes", StatusPassed, OutcomeExpected, []string{StatusPassed}, ""},
				{"outcomes › fails", StatusPassed, OutcomeUnexpected, []string{StatusFailed}, ""},
				{"outcomes › times out", StatusPassed, OutcomeUnexpected, []string{StatusTimedOut}, ""},
				{"outcomes › is skipped declaratively", StatusSkipped, OutcomeSkipped, []string{StatusSkipped}, ""},
				{"outcomes › skips itself at run time", StatusSkipped, OutcomeSkipped, []string{StatusSkipped}, ""},
				{"outcomes › nested describe › passes in a nested describe", StatusPassed, OutcomeExpected, []string{StatusPassed}, ""},
				{"outcomes › passes inside an anonymous describe", StatusPassed, OutcomeExpected, []string{StatusPassed}, ""},
				{"titles: a colon in the describe › case: a colon in the title", StatusPassed, OutcomeExpected, []string{StatusPassed}, ""},
				{"serial group › first fails", StatusPassed, OutcomeUnexpected, []string{StatusFailed}, ""},
				{"serial group › second is skipped after the failure", StatusPassed, OutcomeSkipped, []string{StatusSkipped}, ""},
				{"retried once › fails first then passes", StatusPassed, OutcomeFlaky, []string{StatusFailed, StatusPassed}, ""},
				{"retried once › fails on every attempt", StatusPassed, OutcomeUnexpected, []string{StatusFailed, StatusFailed}, ""},
				{"expected failures › fails as test.fail() expects", StatusFailed, OutcomeExpected, []string{StatusFailed}, ""},
				{"expected failures › passes despite test.fail()", StatusFailed, OutcomeUnexpected, []string{StatusPassed}, ""},
			},
		}},
		{name: "sigint", files: map[string][]want{
			"sigint.spec.ts": {
				{"interrupted run › passes before the interrupt", StatusPassed, OutcomeExpected, []string{StatusPassed}, ""},
				{"interrupted run › is running when the run is interrupted", StatusPassed, OutcomeSkipped, []string{StatusInterrupted}, ""},
				{"interrupted run › never starts", StatusPassed, OutcomeSkipped, nil, ""},
			},
		}},
		{name: "global-timeout", errorCount: 2, errorHas: "Timed out waiting 3s for the test suite to run", files: map[string][]want{
			"global-timeout.spec.ts": {
				{"global timeout › passes before the stop", StatusPassed, OutcomeExpected, []string{StatusPassed}, ""},
				{"global timeout › is running when the run stops", StatusPassed, OutcomeSkipped, []string{StatusSkipped}, ""},
				{"global timeout › never starts", StatusPassed, OutcomeSkipped, nil, ""},
			},
		}},
		{name: "duplicate", errorCount: 2, errorHas: `duplicate test title "dup › same title"`, files: map[string][]want{}},
		{name: "setup-fails", errorCount: 1, errorHas: "capture fixture: global setup failed", files: map[string][]want{}},
		// Two projects: the reporter writes the one test as two specs with
		// one file and title path, one per project.
		{name: "two-projects", projects: []string{"alpha", "beta"}, files: map[string][]want{
			"other.spec.ts": {
				{"outcomes › passes", StatusPassed, OutcomeExpected, []string{StatusPassed}, "alpha"},
				{"outcomes › passes", StatusPassed, OutcomeExpected, []string{StatusPassed}, "beta"},
			},
		}},
		// A helper module the spec file imports (define-shared.ts) declares
		// a describe and a test: the reporter names the helper as that
		// spec's file but lists it under the importing file's own suite, the
		// unit Playwright's title path and duplicate check work in, so the
		// test is that file's, and the helper is no file of the run.
		{name: "helper-declared", files: map[string][]want{
			"helper-declared.spec.ts": {
				{"local › passes", StatusPassed, OutcomeExpected, []string{StatusPassed}, ""},
				{"shared checks for home › renders", StatusPassed, OutcomeExpected, []string{StatusPassed}, ""},
			},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report, err := Decode(bytes.NewReader(capturedReport(t, tc.name)))
			if err != nil {
				t.Fatalf("Decode(%s) = %v, want a report", tc.name, err)
			}
			if report.Version != PinnedVersion {
				t.Errorf("Version = %q, want %q", report.Version, PinnedVersion)
			}
			if report.RootDir != "/verdi/internal/playwrightjson/testdata/capture/specs" {
				t.Errorf("RootDir = %q", report.RootDir)
			}
			wantProjects := tc.projects
			if wantProjects == nil {
				wantProjects = []string{""}
			}
			var gotProjects []string
			for _, p := range report.Projects {
				gotProjects = append(gotProjects, p.Name)
				if p.Retries != 0 || p.RepeatEach != 1 {
					t.Errorf("project %+v, want no retries and one repeat", p)
				}
			}
			if report.Workers != 1 || !reflect.DeepEqual(gotProjects, wantProjects) {
				t.Errorf("run shape = workers %d, projects %q; want one worker and projects %q", report.Workers, gotProjects, wantProjects)
			}
			if len(report.Errors) != tc.errorCount {
				t.Errorf("Errors = %q, want %d", report.Errors, tc.errorCount)
			}
			if tc.errorHas != "" && (len(report.Errors) == 0 || !strings.Contains(report.Errors[0], tc.errorHas)) {
				t.Errorf("Errors = %q, want the first to contain %q", report.Errors, tc.errorHas)
			}
			got := map[string][]want{}
			for _, f := range report.Files {
				got[f.Path] = summarize(f)
			}
			if !reflect.DeepEqual(got, tc.files) {
				t.Errorf("files =\n%+v\nwant\n%+v", got, tc.files)
			}
		})
	}
}

// TestDecode_RejectsBrokenReports is the strict decoder's negative path over
// hand-derived variants of the captured outcomes report: every truncation,
// malformed document, trailing data, unknown field where the schema is fixed,
// unknown enum value, missing verdict field, and inconsistency is an error,
// never a report.
func TestDecode_RejectsBrokenReports(t *testing.T) {
	t.Parallel()
	full := capturedReport(t, "outcomes")
	cases := []struct {
		name string
		in   func(t *testing.T) []byte
		want string // a substring of the error
	}{
		{"empty input", func(*testing.T) []byte { return nil }, "empty"},
		{"truncated by one byte", func(*testing.T) []byte { return full[:len(full)-1] }, "unexpected EOF"},
		{"truncated at half", func(*testing.T) []byte { return full[:len(full)/2] }, "unexpected EOF"},
		{"not JSON", func(*testing.T) []byte { return []byte("Error: no tests found\n") }, "invalid character"},
		{"an array, not a report", func(*testing.T) []byte { return []byte("[]") }, "cannot unmarshal array"},
		{"trailing object", func(*testing.T) []byte { return append(append([]byte{}, full...), []byte("\n{}")...) }, "trailing data"},
		{"trailing garbage", func(*testing.T) []byte { return append(append([]byte{}, full...), 'x') }, "trailing data"},

		{"unknown top-level field", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { r["summary"] = "x" })
		}, `unknown field "summary"`},
		{"unknown stats field", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { obj(t, r, "stats")["retried"] = 1 })
		}, `unknown field "retried"`},
		{"unknown project field", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				arr(t, obj(t, r, "config"), "projects")[0].(map[string]any)["shard"] = 1
			})
		}, `unknown field "shard"`},
		{"unknown suite field", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { fileSuite(t, r, "other.spec.ts")["mode"] = "serial" })
		}, `unknown field "mode"`},
		{"unknown spec field", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { firstSpec(t, r)["retries"] = 1 })
		}, `unknown field "retries"`},
		{"unknown test field", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { firstTest(t, r)["repeatEachIndex"] = 0 })
		}, `unknown field "repeatEachIndex"`},
		{"unknown result field", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { firstResult(t, r)["attempt"] = 2 })
		}, `unknown field "attempt"`},
		{"unknown annotation field", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				arr(t, skippedTest(t, r), "annotations")[0].(map[string]any)["severity"] = "x"
			})
		}, `unknown field "severity"`},
		{"unknown location field", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { obj(t, failingResult(t, r), "errorLocation")["offset"] = 3 })
		}, `unknown field "offset"`},
		{"unknown attachment field", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				arr(t, failingResult(t, r), "attachments")[0].(map[string]any)["size"] = 1
			})
		}, `unknown field "size"`},
		{"unknown reported-error field", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				arr(t, failingResult(t, r), "errors")[0].(map[string]any)["stack"] = "s"
			})
		}, `unknown field "stack"`},
		{"unknown stdio field", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				firstResult(t, r)["stdout"] = []any{map[string]any{"text": "a", "stream": "out"}}
			})
		}, `unknown field "stream"`},
		{"unknown step field", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				firstResult(t, r)["steps"] = []any{map[string]any{"title": "s", "duration": 1, "category": "test.step"}}
			})
		}, `unknown field "category"`},

		{"an attempt error that is not an object", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { failingResult(t, r)["error"] = 5 })
		}, "a test error is not a JSON object: json: cannot unmarshal number"},
		{"an attempt error that is null", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { failingResult(t, r)["error"] = nil })
		}, "a test error is not a JSON object: null"},
		{"a step error that is not an object", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				firstResult(t, r)["steps"] = []any{map[string]any{"title": "s", "duration": 1, "error": "boom"}}
			})
		}, "a test error is not a JSON object: json: cannot unmarshal string"},
		{"a step error that is null", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				firstResult(t, r)["steps"] = []any{map[string]any{"title": "s", "duration": 1, "error": nil}}
			})
		}, "a test error is not a JSON object: null"},

		{"unknown attempt status", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { firstResult(t, r)["status"] = "flaky" })
		}, `unknown attempt status "flaky"`},
		{"unknown test outcome", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { firstTest(t, r)["status"] = "passed" })
		}, `unknown test outcome "passed"`},
		{"unknown expected status", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { firstTest(t, r)["expectedStatus"] = "expected" })
		}, `unknown expected status "expected"`},
		{"an attempt with no status", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(firstResult(t, r), "status") })
		}, "attempt 0 has no status"},
		{"an attempt with no retry index", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(firstResult(t, r), "retry") })
		}, "attempt 0 has no retry"},
		{"attempts out of retry order", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { firstResult(t, r)["retry"] = 1 })
		}, "attempt 0 has retry 1"},
		{"a test with no results", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(firstTest(t, r), "results") })
		}, "has no results"},
		{"a test with no project name", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(firstTest(t, r), "projectName") })
		}, "has no projectName"},
		{"a test with no expected status", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(firstTest(t, r), "expectedStatus") })
		}, "it has no expectedStatus"},
		{"a test with no status", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(firstTest(t, r), "status") })
		}, "it has no status"},
		{"a spec with no tests", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(firstSpec(t, r), "tests") })
		}, "has no tests"},
		{"a spec with no title", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(firstSpec(t, r), "title") })
		}, "has no title"},
		{"a suite with no specs", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(fileSuite(t, r, "other.spec.ts"), "specs") })
		}, "has no specs"},
		{"a file suite with no file", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(fileSuite(t, r, "other.spec.ts"), "file") })
		}, "a file suite has no file or title"},
		{"a file suite with no title", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(fileSuite(t, r, "other.spec.ts"), "title") })
		}, "a file suite has no file or title"},
		{"a describe suite with no title", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				delete(arr(t, fileSuite(t, r, "other.spec.ts"), "suites")[0].(map[string]any), "title")
			})
		}, "a describe suite has no title"},
		{"a status of another type", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { firstResult(t, r)["status"] = 1 })
		}, "cannot unmarshal number"},
		{"no version", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(obj(t, r, "config"), "version") })
		}, "config has no version"},
		{"another Playwright version", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { obj(t, r, "config")["version"] = "1.62.0" })
		}, `written by Playwright "1.62.0"`},
		{"no rootDir", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(obj(t, r, "config"), "rootDir") })
		}, "config has no rootDir"},
		{"a relative rootDir", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { obj(t, r, "config")["rootDir"] = "tests" })
		}, "rootDir"},
		{"no projects", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(obj(t, r, "config"), "projects") })
		}, "config has no projects"},
		{"no workers", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(obj(t, r, "config"), "workers") })
		}, "config has no workers"},
		{"a project with no name", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				delete(arr(t, obj(t, r, "config"), "projects")[0].(map[string]any), "name")
			})
		}, "project 0 has no name, retries, or repeatEach"},
		{"a project with no retries", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				delete(arr(t, obj(t, r, "config"), "projects")[0].(map[string]any), "retries")
			})
		}, "project 0 has no name, retries, or repeatEach"},
		{"a project with no repeatEach", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				delete(arr(t, obj(t, r, "config"), "projects")[0].(map[string]any), "repeatEach")
			})
		}, "project 0 has no name, retries, or repeatEach"},
		{"a config that is an array", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { r["config"] = []any{} })
		}, "the config is not a JSON object"},
		{"a run error that is not an object", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { r["errors"] = []any{"Error: boom"} })
		}, "run error 0: not a JSON object"},
		{"a run error whose message is not a string", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { r["errors"] = []any{map[string]any{"message": 5}} })
		}, "run error 0: message: json: cannot unmarshal number"},
		{"no stats", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(r, "stats") })
		}, "has no stats"},
		{"no suites", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(r, "suites") })
		}, "has no suites"},
		{"no run errors list", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(r, "errors") })
		}, "has no errors"},
		{"no config", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(r, "config") })
		}, "has no config"},
		{"stats that disagree with the tests", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { obj(t, r, "stats")["expected"] = 99 })
		}, "stats count"},
		{"a dropped test the stats still count", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				other := fileSuite(t, r, "other.spec.ts")
				arr(t, other, "suites")[0].(map[string]any)["specs"] = []any{}
			})
		}, "stats count"},
		{"a spec with no file", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) { delete(firstSpec(t, r), "file") })
		}, `spec "passes" has no file`},
		{"a file suite listed twice", func(t *testing.T) []byte {
			return mutated(t, "outcomes", func(r map[string]any) {
				r["suites"] = append(arr(t, r, "suites"), fileSuite(t, r, "other.spec.ts"))
			})
		}, "appears twice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report, err := Decode(bytes.NewReader(tc.in(t)))
			if err == nil {
				t.Fatalf("Decode = %+v, nil error; want an error containing %q", report, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Decode error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestDecode_NonObjectsNameTheirCause proves the config and a run error, each
// an open object, are refused when they are not a JSON object, with an error
// that says why and wraps the decoding error when there is one, never a
// "<nil>" cause.
func TestDecode_NonObjectsNameTheirCause(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		edit     func(r map[string]any)
		want     string
		wantType bool // the error wraps a *json.UnmarshalTypeError
	}{
		{"a null config", func(r map[string]any) { r["config"] = nil }, "the config is not a JSON object: null", false},
		{"an array config", func(r map[string]any) { r["config"] = []any{} }, "the config is not a JSON object: json: cannot unmarshal array", true},
		{"a null run error", func(r map[string]any) { r["errors"] = []any{nil} }, "run error 0: not a JSON object: null", false},
		{"a number run error", func(r map[string]any) { r["errors"] = []any{5} }, "run error 0: not a JSON object: json: cannot unmarshal number", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode(bytes.NewReader(mutated(t, "outcomes", tc.edit)))
			if err == nil {
				t.Fatalf("Decode = nil error, want one containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "<nil>") {
				t.Errorf("Decode error = %q, want it to contain %q and no <nil>", err, tc.want)
			}
			var typeErr *json.UnmarshalTypeError
			if got := errors.As(err, &typeErr); got != tc.wantType {
				t.Errorf("errors.As(%q, *json.UnmarshalTypeError) = %v, want %v", err, got, tc.wantType)
			}
		})
	}
}

// TestDecode_OpenFieldsStayOpen proves the fields this decoder leaves open —
// the config beyond the run shape, a project's user metadata, and the
// TestError payloads — accept keys the pinned schema does not list, while the
// verdict remains unchanged.
func TestDecode_OpenFieldsStayOpen(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		edit func(t *testing.T, r map[string]any)
	}{
		{"a config key", func(t *testing.T, r map[string]any) { obj(t, r, "config")["futureOption"] = true }},
		{"project metadata", func(t *testing.T, r map[string]any) {
			arr(t, obj(t, r, "config"), "projects")[0].(map[string]any)["metadata"] = map[string]any{"owner": "e2e"}
		}},
		{"a test error's context", func(t *testing.T, r map[string]any) {
			obj(t, failingResult(t, r), "error")["errorContext"] = "- heading"
		}},
		{"a test error's cause", func(t *testing.T, r map[string]any) {
			obj(t, failingResult(t, r), "error")["cause"] = map[string]any{"message": "m", "detail": 1}
		}},
		{"a step error's key", func(t *testing.T, r map[string]any) {
			firstResult(t, r)["steps"] = []any{map[string]any{"title": "s", "duration": 1, "error": map[string]any{"message": "m", "futureKey": 1}}}
		}},
		{"a run error's key", func(t *testing.T, r map[string]any) {
			r["errors"] = []any{map[string]any{"message": "m", "futureKey": 1}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report, err := Decode(bytes.NewReader(mutated(t, "outcomes", func(r map[string]any) { tc.edit(t, r) })))
			if err != nil {
				t.Fatalf("Decode = %v, want the open field accepted", err)
			}
			if len(report.Files) != 2 {
				t.Errorf("Files = %d, want 2", len(report.Files))
			}
		})
	}
}

// TestPinnedVersionMatchesLockfile ties PinnedVersion to the @playwright/test
// version e2e/package-lock.json pins, so a Playwright upgrade fails here until
// the reports are re-captured and the schema re-read.
func TestPinnedVersionMatchesLockfile(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "e2e", "package-lock.json"))
	if err != nil {
		t.Fatalf("reading e2e/package-lock.json: %v", err)
	}
	var lock struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatalf("decoding e2e/package-lock.json: %v", err)
	}
	got := lock.Packages["node_modules/@playwright/test"].Version
	if got != PinnedVersion {
		t.Fatalf("e2e/package-lock.json pins @playwright/test %q, PinnedVersion is %q: re-read the reporter's schema, re-run testdata/capture/capture.sh, and update the pin together", got, PinnedVersion)
	}
}

// TestJoinTitlePath is the title-path join's happy and edge paths.
func TestJoinTitlePath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		parts []string
		want  string
	}{
		{[]string{"a"}, "a"},
		{[]string{"a", "b: c", "d"}, "a › b: c › d"},
		{nil, ""},
	}
	for _, tc := range cases {
		if got := JoinTitlePath(tc.parts); got != tc.want {
			t.Errorf("JoinTitlePath(%q) = %q, want %q", tc.parts, got, tc.want)
		}
	}
}
