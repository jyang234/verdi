// Package playwrightjson reads the JSON report @playwright/test writes (the
// "json" reporter, report format v2) into per-file, per-test attempts. It is
// the Playwright counterpart of internal/gotestjson: the per-test evidence
// producer (cmd/verdi, SI-292..SI-294) applies its own outcome policy to the
// Report this package returns.
//
// The schema is pinned to PinnedVersion, the @playwright/test version
// e2e/package-lock.json pins (TestPinnedVersionMatchesLockfile), and read
// from that version's own reporter (playwright/lib/runner/index.js,
// "packages/playwright/src/reporters/json.ts": JSONReporter._serializeReport
// and the helpers it calls), not only from its declared types, which omit
// fields the reporter writes (an annotation's location). A report that names
// any other Playwright version is refused.
//
// Each test belongs to the file suite that lists it (the test file the run
// loaded), even when a helper module that file imports declared it: the
// reporter takes a spec's own "file" from the test's direct caller, so such a
// spec names the helper while its file suite names the test file
// (JSONReporter._mergeSuites groups by the file suite). A spec's file is
// required but never read for attribution.
//
// Decoding posture: strict, as CLAUDE.md requires of all JSON. Every object
// whose fields the pinned reporter itself fixes is decoded with
// DisallowUnknownFields: the report, its stats, each project, file and
// describe suite, spec, test, attempt, annotation, attachment, stdio entry,
// test step, source location, and each formatted attempt error
// ({message, location}). Anything after the report is refused, and so is an
// unknown attempt status, test outcome, or expected status. The verdict
// fields must be present, not merely zero: the report's config, suites,
// errors, and stats; a file suite's title, file, and specs; a describe
// suite's title and specs; a spec's title, file, and tests; a test's
// projectName, expectedStatus, status, and results; an attempt's status and
// retry; the config's version, rootDir, workers, and projects; and each
// project's name, retries, and repeatEach.
//
// Three kinds of field stay open, each decoded only as JSON and never read
// for a verdict:
//   - the config beyond version, rootDir, workers, and projects: the reporter
//     spreads the whole resolved config into it (removePrivateFields), so it
//     carries every config option, the webServer and reporter settings, and
//     user metadata, whose keys the report format does not fix;
//   - a project's metadata: user-defined in the config;
//   - a TestError (an attempt's error, a step's error, and each of the run's
//     own errors): the worker serializes it from whatever value the test
//     threw, and adds keys the public TestError type does not list (for
//     example errorContext, from an aria-snapshot matcher); only a run
//     error's message is read, to name it, and must be a string when
//     present.
//
// Open is not shapeless: the config and every TestError must be a JSON
// object (an attempt's or step's error may also be absent), never null or
// another JSON value.
package playwrightjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// PinnedVersion is the @playwright/test version whose report schema this
// package reads.
const PinnedVersion = "1.61.1"

// TitlePathSeparator joins a test's title path, as Playwright joins it in
// its own duplicate-title check and its list output: space, U+203A, space.
const TitlePathSeparator = " \u203a "

// An attempt's own status (Playwright's TestStatus). Any other value is an
// error.
const (
	StatusPassed      = "passed"
	StatusFailed      = "failed"
	StatusTimedOut    = "timedOut"
	StatusSkipped     = "skipped"
	StatusInterrupted = "interrupted"
)

// A test's outcome over all its attempts, as Playwright computes it. Any
// other value is an error.
const (
	OutcomeExpected   = "expected"
	OutcomeUnexpected = "unexpected"
	OutcomeFlaky      = "flaky"
	OutcomeSkipped    = "skipped"
)

// isAttemptStatus reports whether s is one of the attempt statuses above.
func isAttemptStatus(s string) bool {
	switch s {
	case StatusPassed, StatusFailed, StatusTimedOut, StatusSkipped, StatusInterrupted:
		return true
	}
	return false
}

// isOutcome reports whether s is one of the test outcomes above.
func isOutcome(s string) bool {
	switch s {
	case OutcomeExpected, OutcomeUnexpected, OutcomeFlaky, OutcomeSkipped:
		return true
	}
	return false
}

// Report is one decoded JSON report.
type Report struct {
	// Version is the Playwright version that wrote the report: PinnedVersion.
	Version string
	// RootDir is the absolute, slash-separated directory each file's Path is
	// relative to (the config's rootDir).
	RootDir string
	// Workers is the run's worker count.
	Workers int
	// Projects are the run's projects.
	Projects []Project
	// Errors holds the message of each error the run reported of its own
	// (a load error, a failed global setup or web server, a global timeout),
	// "" for one with no message.
	Errors []string
	// Files are the run's test files, in report order, each once.
	Files []File
}

// Project is one project of the run.
type Project struct {
	ID, Name   string
	Retries    int
	RepeatEach int
}

// File is one test file and every test in it.
type File struct {
	// Path is the file relative to Report.RootDir, slash-separated, as the
	// report names it.
	Path string
	// Tests are the file's tests in report order: one per spec per project.
	// A test that a helper module this file imports declares is this file's
	// test, as Playwright attributes it: the report lists it in this file's
	// suite, naming the helper only as the spec's own location (its "file"),
	// and Playwright's own title path and duplicate-title check work per file
	// suite.
	Tests []Test
}

// Test is one test of one project.
type Test struct {
	// TitlePath is the titles of the test's enclosing describe blocks,
	// outermost first, then its own title. An anonymous describe has no
	// title and is omitted, as Playwright omits it from its own title path;
	// the file and the project are never part of it.
	TitlePath []string
	// ProjectName is the test's project.
	ProjectName string
	// ExpectedStatus is the status the test's annotations expect (passed,
	// or skipped, or failed under test.fail()).
	ExpectedStatus string
	// Outcome is Playwright's outcome over all attempts: expected,
	// unexpected, flaky, or skipped.
	Outcome string
	// Attempts are the test's attempts in retry order, one per run of its
	// body; none when it did not run.
	Attempts []Attempt
}

// JoinedTitlePath is the test's title path joined by TitlePathSeparator.
func (t Test) JoinedTitlePath() string { return JoinTitlePath(t.TitlePath) }

// Attempt is one run of a test's body.
type Attempt struct {
	// Status is the attempt's own status: passed, failed, timedOut,
	// skipped, or interrupted.
	Status string
	// Retry is the attempt's index: 0 for the first run.
	Retry int
}

// JoinTitlePath joins title-path parts with TitlePathSeparator.
func JoinTitlePath(parts []string) string { return strings.Join(parts, TitlePathSeparator) }

// --- the wire shapes (1.61.1's JSONReporter) ---------------------------------

type reportJSON struct {
	Config json.RawMessage    `json:"config"`
	Suites *[]suiteJSON       `json:"suites"`
	Errors *[]json.RawMessage `json:"errors"`
	Stats  *statsJSON         `json:"stats"`
}

type statsJSON struct {
	StartTime  string  `json:"startTime"`
	Duration   float64 `json:"duration"`
	Expected   int     `json:"expected"`
	Unexpected int     `json:"unexpected"`
	Flaky      int     `json:"flaky"`
	Skipped    int     `json:"skipped"`
}

type projectJSON struct {
	OutputDir  string          `json:"outputDir"`
	RepeatEach *int            `json:"repeatEach"`
	Retries    *int            `json:"retries"`
	Metadata   json.RawMessage `json:"metadata"`
	ID         string          `json:"id"`
	Name       *string         `json:"name"`
	TestDir    string          `json:"testDir"`
	TestIgnore []string        `json:"testIgnore"`
	TestMatch  []string        `json:"testMatch"`
	Timeout    float64         `json:"timeout"`
}

type suiteJSON struct {
	Title  *string     `json:"title"`
	File   *string     `json:"file"`
	Column int         `json:"column"`
	Line   int         `json:"line"`
	Specs  *[]specJSON `json:"specs"`
	Suites []suiteJSON `json:"suites"`
}

type specJSON struct {
	Tags   []string    `json:"tags"`
	Title  *string     `json:"title"`
	OK     bool        `json:"ok"`
	Tests  *[]testJSON `json:"tests"`
	ID     string      `json:"id"`
	File   *string     `json:"file"`
	Line   int         `json:"line"`
	Column int         `json:"column"`
}

type testJSON struct {
	Timeout        float64          `json:"timeout"`
	Annotations    []annotationJSON `json:"annotations"`
	ExpectedStatus *string          `json:"expectedStatus"`
	ProjectID      string           `json:"projectId"`
	ProjectName    *string          `json:"projectName"`
	Results        *[]resultJSON    `json:"results"`
	Status         *string          `json:"status"`
}

type annotationJSON struct {
	Type        string        `json:"type"`
	Description string        `json:"description"`
	Location    *locationJSON `json:"location"`
}

type locationJSON struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type resultJSON struct {
	WorkerIndex   int                 `json:"workerIndex"`
	ParallelIndex int                 `json:"parallelIndex"`
	ShardIndex    int                 `json:"shardIndex"`
	Status        *string             `json:"status"`
	Duration      float64             `json:"duration"`
	Error         testErrorJSON       `json:"error"`
	Errors        []reportedErrorJSON `json:"errors"`
	Stdout        []stdioJSON         `json:"stdout"`
	Stderr        []stdioJSON         `json:"stderr"`
	Retry         *int                `json:"retry"`
	Steps         []stepJSON          `json:"steps"`
	StartTime     string              `json:"startTime"`
	Annotations   []annotationJSON    `json:"annotations"`
	Attachments   []attachmentJSON    `json:"attachments"`
	ErrorLocation *locationJSON       `json:"errorLocation"`
}

// reportedErrorJSON is the reporter's own formatted attempt error
// (formatError: a message and, when known, a location).
type reportedErrorJSON struct {
	Message  string        `json:"message"`
	Location *locationJSON `json:"location"`
}

type stdioJSON struct {
	Text   *string `json:"text"`
	Buffer *string `json:"buffer"`
}

type stepJSON struct {
	Title    string        `json:"title"`
	Duration float64       `json:"duration"`
	Error    testErrorJSON `json:"error"`
	Steps    []stepJSON    `json:"steps"`
}

// testErrorJSON is an attempt's or a step's TestError: an open object (see
// the package doc), never read for a verdict. The reporter writes an object
// or omits the key (an undefined error), so any other JSON value, null
// included, is refused; an absent error leaves the zero value.
type testErrorJSON struct{}

// UnmarshalJSON accepts a JSON object only. encoding/json calls it for a
// present key, null included, because the field is not a pointer.
func (*testErrorJSON) UnmarshalJSON(raw []byte) error {
	if _, err := decodeObject(raw); err != nil {
		return fmt.Errorf("a test error is %w", err)
	}
	return nil
}

type attachmentJSON struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Path        string `json:"path"`
	Body        string `json:"body"`
}

// --- decoding -----------------------------------------------------------------

// Decode reads one JSON report to its end. It returns an error, and no
// Report, for a report this package cannot trust as the pinned reporter's
// account of a run: empty, truncated, or malformed JSON; anything after the
// report; an unknown field where the schema is fixed; an unknown attempt
// status, test outcome, or expected status; a missing verdict field; another
// Playwright version; a relative rootDir; attempts out of retry order; a file
// listed twice; or stats whose counts disagree with the tests the report
// carries. A spec whose file differs from its file suite's is not refused: it
// is a test a helper module declared, and it is the file suite's test.
func Decode(r io.Reader) (Report, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Report{}, fmt.Errorf("playwrightjson: reading the report: %w", err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return Report{}, errors.New("playwrightjson: the report is empty")
	}
	var wire reportJSON
	if err := strictDecode(raw, &wire); err != nil {
		return Report{}, fmt.Errorf("playwrightjson: %w", err)
	}
	switch {
	case wire.Config == nil:
		return Report{}, errors.New("playwrightjson: the report has no config")
	case wire.Suites == nil:
		return Report{}, errors.New("playwrightjson: the report has no suites")
	case wire.Errors == nil:
		return Report{}, errors.New("playwrightjson: the report has no errors list")
	case wire.Stats == nil:
		return Report{}, errors.New("playwrightjson: the report has no stats")
	}

	report, err := decodeConfig(wire.Config)
	if err != nil {
		return Report{}, fmt.Errorf("playwrightjson: %w", err)
	}
	for i, e := range *wire.Errors {
		msg, err := errorMessage(e)
		if err != nil {
			return Report{}, fmt.Errorf("playwrightjson: run error %d: %w", i, err)
		}
		report.Errors = append(report.Errors, msg)
	}

	counts := map[string]int{}
	seen := map[string]bool{}
	for i, s := range *wire.Suites {
		f, err := decodeFile(s, counts)
		if err != nil {
			return Report{}, fmt.Errorf("playwrightjson: file suite %d: %w", i, err)
		}
		if seen[f.Path] {
			return Report{}, fmt.Errorf("playwrightjson: file %q appears twice", f.Path)
		}
		seen[f.Path] = true
		report.Files = append(report.Files, f)
	}

	st := wire.Stats
	if st.Expected != counts[OutcomeExpected] || st.Unexpected != counts[OutcomeUnexpected] || st.Flaky != counts[OutcomeFlaky] || st.Skipped != counts[OutcomeSkipped] {
		return Report{}, fmt.Errorf("playwrightjson: stats count %d expected, %d unexpected, %d flaky, %d skipped, but the report carries %d, %d, %d, %d",
			st.Expected, st.Unexpected, st.Flaky, st.Skipped,
			counts[OutcomeExpected], counts[OutcomeUnexpected], counts[OutcomeFlaky], counts[OutcomeSkipped])
	}
	return report, nil
}

// strictDecode decodes raw into v with DisallowUnknownFields and refuses
// anything but whitespace after the one JSON value.
func strictDecode(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("trailing data after the report")
	}
	return nil
}

// decodeObject reads raw as a JSON object whose keys stay open. null and any
// other JSON value are refused, wrapping the decoding error when there is one.
func decodeObject(raw []byte) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("not a JSON object: %w", err)
	}
	if obj == nil {
		return nil, errors.New("not a JSON object: null")
	}
	return obj, nil
}

// decodeConfig reads the open config's run shape: version, rootDir, workers,
// and each project, strictly.
func decodeConfig(raw json.RawMessage) (Report, error) {
	config, err := decodeObject(raw)
	if err != nil {
		return Report{}, fmt.Errorf("the config is %w", err)
	}
	var report Report
	for _, field := range []struct {
		name string
		dst  any
	}{{"version", &report.Version}, {"rootDir", &report.RootDir}, {"workers", &report.Workers}} {
		value, ok := config[field.name]
		if !ok {
			return Report{}, fmt.Errorf("the config has no %s", field.name)
		}
		if err := json.Unmarshal(value, field.dst); err != nil {
			return Report{}, fmt.Errorf("the config's %s: %w", field.name, err)
		}
	}
	if report.Version != PinnedVersion {
		return Report{}, fmt.Errorf("the report was written by Playwright %q; this decoder reads the schema of %s only", report.Version, PinnedVersion)
	}
	if !path.IsAbs(report.RootDir) {
		return Report{}, fmt.Errorf("the config's rootDir %q is not absolute", report.RootDir)
	}
	projectsRaw, ok := config["projects"]
	if !ok {
		return Report{}, errors.New("the config has no projects")
	}
	var projects []projectJSON
	if err := strictDecode(projectsRaw, &projects); err != nil {
		return Report{}, fmt.Errorf("the config's projects: %w", err)
	}
	for i, p := range projects {
		if p.Name == nil || p.Retries == nil || p.RepeatEach == nil {
			return Report{}, fmt.Errorf("project %d has no name, retries, or repeatEach", i)
		}
		report.Projects = append(report.Projects, Project{ID: p.ID, Name: *p.Name, Retries: *p.Retries, RepeatEach: *p.RepeatEach})
	}
	return report, nil
}

// errorMessage reads a TestError's message, leaving its other keys open.
func errorMessage(raw json.RawMessage) (string, error) {
	e, err := decodeObject(raw)
	if err != nil {
		return "", err
	}
	msg, ok := e["message"]
	if !ok {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(msg, &s); err != nil {
		return "", fmt.Errorf("message: %w", err)
	}
	return s, nil
}

// decodeFile flattens one file suite's tests, counting each outcome.
func decodeFile(s suiteJSON, counts map[string]int) (File, error) {
	if s.File == nil || s.Title == nil {
		return File{}, errors.New("a file suite has no file or title")
	}
	f := File{Path: *s.File}
	if err := collectTests(&f, s, nil, counts); err != nil {
		return File{}, fmt.Errorf("%s: %w", f.Path, err)
	}
	return f, nil
}

// collectTests appends suite's own tests, then its child describes', with
// titles as the title path so far. The file suite's own title is never part
// of it, and an untitled (anonymous) describe adds nothing.
func collectTests(f *File, suite suiteJSON, titles []string, counts map[string]int) error {
	if suite.Specs == nil {
		return errors.New("a suite has no specs")
	}
	for _, sp := range *suite.Specs {
		if sp.Title == nil {
			return errors.New("a spec has no title")
		}
		if sp.Tests == nil {
			return fmt.Errorf("spec %q has no tests", *sp.Title)
		}
		// A spec's file is where its test was declared, which is a helper
		// module when the file suite's file imports one that declares tests;
		// the test is still the file suite's (see File.Tests).
		if sp.File == nil {
			return fmt.Errorf("spec %q has no file", *sp.Title)
		}
		titlePath := append(append([]string{}, titles...), *sp.Title)
		for _, tj := range *sp.Tests {
			tc, err := decodeTest(tj, titlePath)
			if err != nil {
				return fmt.Errorf("test %q: %w", JoinTitlePath(titlePath), err)
			}
			counts[tc.Outcome]++
			f.Tests = append(f.Tests, tc)
		}
	}
	for _, child := range suite.Suites {
		if child.Title == nil {
			return errors.New("a describe suite has no title")
		}
		childTitles := titles
		if *child.Title != "" {
			childTitles = append(append([]string{}, titles...), *child.Title)
		}
		if err := collectTests(f, child, childTitles, counts); err != nil {
			return err
		}
	}
	return nil
}

func decodeTest(tj testJSON, titlePath []string) (Test, error) {
	switch {
	case tj.ProjectName == nil:
		return Test{}, errors.New("it has no projectName")
	case tj.ExpectedStatus == nil:
		return Test{}, errors.New("it has no expectedStatus")
	case tj.Status == nil:
		return Test{}, errors.New("it has no status")
	case tj.Results == nil:
		return Test{}, errors.New("it has no results")
	}
	if !isAttemptStatus(*tj.ExpectedStatus) {
		return Test{}, fmt.Errorf("unknown expected status %q", *tj.ExpectedStatus)
	}
	if !isOutcome(*tj.Status) {
		return Test{}, fmt.Errorf("unknown test outcome %q", *tj.Status)
	}
	tc := Test{TitlePath: titlePath, ProjectName: *tj.ProjectName, ExpectedStatus: *tj.ExpectedStatus, Outcome: *tj.Status}
	for i, rj := range *tj.Results {
		switch {
		case rj.Status == nil:
			return Test{}, fmt.Errorf("attempt %d has no status", i)
		case rj.Retry == nil:
			return Test{}, fmt.Errorf("attempt %d has no retry index", i)
		case !isAttemptStatus(*rj.Status):
			return Test{}, fmt.Errorf("unknown attempt status %q", *rj.Status)
		case *rj.Retry != i:
			return Test{}, fmt.Errorf("attempt %d has retry %d: attempts are out of retry order", i, *rj.Retry)
		}
		tc.Attempts = append(tc.Attempts, Attempt{Status: *rj.Status, Retry: *rj.Retry})
	}
	return tc, nil
}
