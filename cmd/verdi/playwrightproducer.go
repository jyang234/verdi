// The Playwright per-test CI evidence producer (SI-292..SI-294; design
// docs/superpowers/specs/2026-09-29-playwright-test-producer-design.md): 03
// §Declarations and binding's per-test exception names Go tests and
// Playwright tests, so an elaborated evidence obligation may name a single
// Playwright test as its producer (`playwright:<file>:<title path>`) and is
// matched per test. This file is that producer. Invoked only from `sync
// --produce` (sync.go's runProduce), after the go-test producer
// (testproducer.go), it selects the obligations the running CI job is
// authoritative for (SI-229) whose producer names a Playwright test, runs
// their named files once, serially, with no retries, through the
// repository's own harness (e2e/playwright.config.ts), reads Playwright's JSON
// report with the strict reader (internal/playwrightjson), and emits one
// record per selected obligation carrying that obligation's own kind and
// acceptance-criterion id.
//
// HONESTY. As with the go-test producer, a renamed or removed test surfaces as
// a missing producer, a closure blocker, never a silent pass: a named file
// that does not exist, or a title path the report does not carry (the test
// did not run, or its file declares no tests, SI-308), emits no record, only
// a disclosure, and withdraws any earlier record for its producer at the same
// commit (SI-238). A malformed ref is
// disclosed and skipped on its own obligation, never an operational error for
// the others (SI-303). Only what makes the whole run untrustworthy is an
// operational error (exit 2) that writes nothing: a run that could not be
// carried out; a report that is missing, malformed, or truncated; an error the
// run reports of its own (the harness or its global setup failed, a global
// timeout, Playwright's own load-time refusal of a file); a report that is not
// the account of the run this producer asked for; or two tests in a named file
// sharing a named title path (SI-294).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/playwrightjson"
)

// --- Grammar (SI-292, SI-303) --------------------------------------------------

// playwrightProducerScheme prefixes every Playwright producer ref.
const playwrightProducerScheme = "playwright:"

// playwrightTestsDir is the one directory a named file lives in, relative to
// the store root; e2e/playwright.config.ts's default project collects it.
const playwrightTestsDir = "e2e/tests/"

// playwrightSpecNameRE is the character set of a named file's <name> (SI-303
// reading (a)): exactly the harness selector's
// ^[A-Za-z0-9][A-Za-z0-9._-]*\.spec\.ts$ (e2e/playwright.config.ts,
// VERDI_E2E_SPECS, SI-268) without its suffix, so every file the grammar
// accepts is one the harness can select.
var playwrightSpecNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// playwrightProducerRef is a producer ref's parsed file and title path.
type playwrightProducerRef struct {
	// File is the test file's repository path, e2e/tests/<name>.spec.ts.
	File string
	// TitlePath is the enclosing describe titles and the test's own title,
	// joined by " › ", exactly as the ref spells it.
	TitlePath string
}

// FileName is the file's name under e2e/tests/: the form VERDI_E2E_SPECS
// takes and the report names the file by.
func (r playwrightProducerRef) FileName() string {
	return strings.TrimPrefix(r.File, playwrightTestsDir)
}

// playwrightParseProducerRef strictly parses a producer ref against SI-292's
// grammar `playwright:<file>:<title path>`, returning why a ref does not
// match. The file contains no ":", so the ref splits at the FIRST ":" after
// the prefix and the title path keeps every later one. The file is
// e2e/tests/<name>.spec.ts with <name> in playwrightSpecNameRE (SI-303). The
// title path is valid UTF-8 and non-empty, begins and ends with no whitespace
// (Unicode White_Space), and holds no line break: CR, LF, U+0085, U+2028, or
// U+2029 (SI-303).
func playwrightParseProducerRef(ref string) (playwrightProducerRef, error) {
	rest, ok := strings.CutPrefix(ref, playwrightProducerScheme)
	if !ok {
		return playwrightProducerRef{}, fmt.Errorf("the scheme is not %q", playwrightProducerScheme)
	}
	file, title, ok := strings.Cut(rest, ":")
	if !ok {
		return playwrightProducerRef{}, errors.New("there is no title path segment")
	}
	name, ok := strings.CutPrefix(file, playwrightTestsDir)
	if !ok {
		return playwrightProducerRef{}, fmt.Errorf("the file %q is not under %s", file, playwrightTestsDir)
	}
	name, ok = strings.CutSuffix(name, ".spec.ts")
	if !ok {
		return playwrightProducerRef{}, fmt.Errorf("the file %q is not a .spec.ts file", file)
	}
	if !playwrightSpecNameRE.MatchString(name) {
		return playwrightProducerRef{}, fmt.Errorf("the file name %q is not a letter or digit followed by letters, digits, '.', '_', or '-' (the harness's spec-file selector)", name+".spec.ts")
	}
	if err := playwrightCheckTitlePath(title); err != nil {
		return playwrightProducerRef{}, err
	}
	return playwrightProducerRef{File: file, TitlePath: title}, nil
}

// playwrightCheckTitlePath applies the title path's own rules.
func playwrightCheckTitlePath(title string) error {
	if title == "" {
		return errors.New("the title path is empty")
	}
	if !utf8.ValidString(title) {
		return errors.New("the title path is not valid UTF-8")
	}
	if i := strings.IndexAny(title, "\r\n\u0085\u2028\u2029"); i >= 0 {
		r, _ := utf8.DecodeRuneInString(title[i:])
		return fmt.Errorf("the title path contains a line break (%U)", r)
	}
	first, _ := utf8.DecodeRuneInString(title)
	last, _ := utf8.DecodeLastRuneInString(title)
	if unicode.IsSpace(first) || unicode.IsSpace(last) {
		return errors.New("the title path has leading or trailing whitespace")
	}
	return nil
}

// --- Selection (SI-293) ----------------------------------------------------------

// playwrightSelectedObligation is one candidate this CI job is authoritative
// for, with its producer ref parsed.
type playwrightSelectedObligation struct {
	testProducerCandidate
	playwrightProducerRef
}

// playwrightProducerMalformedRefSource is the disclosure source for an
// elaborated test-producer obligation whose playwright: ref fails the grammar.
const playwrightProducerMalformedRefSource = "sync:playwright-producer-malformed-ref"

func playwrightProducerMalformedRefDisclosure(c testProducerCandidate, reason error) disclosure.Disclosure {
	text := fmt.Sprintf("declares producer %q, which does not match the playwright:<file>:<title path> grammar (%v); no evidence record was emitted for it", c.ProducerRef, reason)
	return disclosure.New(playwrightProducerMalformedRefSource, c.ObligationID, text)
}

// playwrightProducerRuntimeKindSource is the disclosure source for a
// runtime-kind obligation naming a Playwright test: runtime evidence exists
// only after deployment (03 §Evidence kinds), so this pre-merge job emits no
// record for it, exactly as the go-test producer does not.
const playwrightProducerRuntimeKindSource = "sync:playwright-producer-runtime-kind"

func playwrightProducerRuntimeKindDisclosure(c testProducerCandidate) disclosure.Disclosure {
	text := fmt.Sprintf("declares producer %q for a runtime-kind obligation; runtime evidence exists only after deployment (03 §Evidence kinds), so this CI job emitted no record for it", c.ProducerRef)
	return disclosure.New(playwrightProducerRuntimeKindSource, c.ObligationID, text)
}

// playwrightSelectObligations narrows candidates to this CI job's own
// Playwright obligations: authoritative_source.ref == jobName (SI-229; an
// empty jobName, outside a detected CI job, selects nothing) and a producer
// ref starting "playwright:". Any other ref is the go-test producer's and is
// skipped silently here. A runtime-kind obligation and a ref that fails the
// grammar are each disclosed on their own obligation and excluded. The result
// is sorted (file, title path, spec, criterion).
func playwrightSelectObligations(candidates []testProducerCandidate, jobName string) ([]playwrightSelectedObligation, []disclosure.Disclosure) {
	var selected []playwrightSelectedObligation
	var discl []disclosure.Disclosure
	for _, c := range candidates {
		if jobName == "" || c.JobRef != jobName || !strings.HasPrefix(c.ProducerRef, playwrightProducerScheme) {
			continue
		}
		if c.Kind == artifact.EvidenceRuntime {
			discl = append(discl, playwrightProducerRuntimeKindDisclosure(c))
			continue
		}
		parsed, err := playwrightParseProducerRef(c.ProducerRef)
		if err != nil {
			discl = append(discl, playwrightProducerMalformedRefDisclosure(c, err))
			continue
		}
		selected = append(selected, playwrightSelectedObligation{testProducerCandidate: c, playwrightProducerRef: parsed})
	}
	sort.Slice(selected, func(i, j int) bool {
		a, b := selected[i], selected[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.TitlePath != b.TitlePath {
			return a.TitlePath < b.TitlePath
		}
		if a.SpecName != b.SpecName {
			return a.SpecName < b.SpecName
		}
		return a.ACID < b.ACID
	})
	return selected, discl
}

// --- Execution (SI-293) ------------------------------------------------------------

// playwrightRunner abstracts one run of named Playwright files through the
// repository's harness: the producer's execution seam (CLAUDE.md: hermetic
// fakes in unit tests; the real runner only in production).
type playwrightRunner interface {
	// RunPlaywright runs exactly specs (file names under root's e2e/tests/)
	// once through the harness — one worker, no retries, recording off, and a
	// file that declares no tests no error of the run's own — with
	// Playwright's JSON reporter writing to reportPath. A failing test is not
	// an error here: the report says what ran. Only a run that could not be
	// carried out is.
	RunPlaywright(ctx context.Context, root string, specs []string, reportPath string) error
}

// playwrightRealRunner runs the real harness. Log receives both commands'
// output (nil discards it).
type playwrightRealRunner struct {
	Log io.Writer
}

// playwrightRunArgs is the one Playwright command line, run in e2e/: the
// harness config's own test command with one worker, no retries, the JSON
// reporter (to PLAYWRIGHT_JSON_OUTPUT_FILE), trace off, and outputDir for
// Playwright's own per-test output. It names no file: Playwright's positional
// filters are regular-expression substring matches that could run extra
// files, so the files reach the harness only through VERDI_E2E_SPECS, its
// exact, fail-closed selector (SI-268). Screenshots and video stay off by the
// config's defaults (e2e/playwright.config.ts sets neither).
//
// --pass-with-no-tests (SI-308): without it, a run whose named files all
// exist but declare no tests stops with Playwright's own "No tests found"
// error, a run-level error that would withhold every record of the run.
// Because the selector is exact, a zero-test run can come only from such
// files, so with the flag their named title paths are simply absent from the
// report: each such obligation gets no record and a disclosure (SI-294's
// absent title path), and no other obligation is affected.
func playwrightRunArgs(outputDir string) []string {
	return []string{"playwright", "test", "--workers=1", "--retries=0", "--pass-with-no-tests", "--reporter=json", "--trace=off", "--output=" + outputDir}
}

// playwrightRunEnvOverrides are the variables the run sets or removes: an
// inherited V1_ACCEPTANCE would add the config's second project
// (v1-acceptance, every file under e2e/tests-v1/), and inherited selector or
// report variables would redirect the run.
var playwrightRunEnvOverrides = []string{"V1_ACCEPTANCE", "VERDI_E2E_SPECS", "PLAYWRIGHT_JSON_OUTPUT_FILE", "PLAYWRIGHT_JSON_OUTPUT_DIR", "PLAYWRIGHT_JSON_OUTPUT_NAME", "PWD"}

// playwrightRunEnv is parent without playwrightRunEnvOverrides, plus the
// harness selector, the report path, and PWD for dir (what os/exec sets
// itself when it inherits the environment).
func playwrightRunEnv(parent []string, dir string, specs []string, reportPath string) []string {
	env := make([]string, 0, len(parent)+3)
	for _, kv := range parent {
		name, _, _ := strings.Cut(kv, "=")
		drop := false
		for _, o := range playwrightRunEnvOverrides {
			drop = drop || name == o
		}
		if !drop {
			env = append(env, kv)
		}
	}
	return append(env, "VERDI_E2E_SPECS="+strings.Join(specs, " "), "PLAYWRIGHT_JSON_OUTPUT_FILE="+reportPath, "PWD="+dir)
}

// RunPlaywright installs the harness's packages and browser (`make
// e2e-setup`, the e2e shards' own prerequisite, hard-failing without Node),
// then runs playwrightRunArgs in root/e2e with playwrightRunEnv.
func (r playwrightRealRunner) RunPlaywright(ctx context.Context, root string, specs []string, reportPath string) error {
	log := r.Log
	if log == nil {
		log = io.Discard
	}
	setup := exec.CommandContext(ctx, "make", "e2e-setup")
	setup.Dir = root
	setup.Stdout, setup.Stderr = log, log
	if err := setup.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("make e2e-setup: %w", ctxErr)
		}
		return fmt.Errorf("installing the e2e harness's packages and browser (make e2e-setup): %w", err)
	}
	dir := filepath.Join(root, "e2e")
	run := exec.CommandContext(ctx, "npx", playwrightRunArgs(filepath.Join(filepath.Dir(reportPath), "test-results"))...)
	run.Dir = dir
	run.Env = playwrightRunEnv(os.Environ(), dir, specs, reportPath)
	run.Stdout, run.Stderr = log, log
	err := run.Run()
	// A run killed by its context reports the kill as an *exec.ExitError, so
	// check the context first.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("playwright test: %w", ctxErr)
	}
	// A nonzero exit is how Playwright reports a failing test, or an error of
	// the run's own that the report carries; only a command that could not
	// run at all is an error here.
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return fmt.Errorf("running playwright test: %w", err)
	}
	return nil
}

// playwrightExecuteRun runs specs once through runner and returns the strict
// reading of the report it wrote, after checking that the report is the
// account of that run (playwrightCheckRun). Every failure is operational.
func playwrightExecuteRun(ctx context.Context, root string, runner playwrightRunner, specs []string) (playwrightjson.Report, error) {
	work, err := os.MkdirTemp("", "verdi-playwright-*")
	if err != nil {
		return playwrightjson.Report{}, fmt.Errorf("playwright producer: %w", err)
	}
	defer func() { _ = os.RemoveAll(work) }()
	reportPath := filepath.Join(work, "report.json")
	if err := runner.RunPlaywright(ctx, root, specs, reportPath); err != nil {
		return playwrightjson.Report{}, fmt.Errorf("playwright producer: %w", err)
	}
	f, err := os.Open(reportPath)
	if err != nil {
		if os.IsNotExist(err) {
			return playwrightjson.Report{}, errors.New("playwright producer: the run wrote no report")
		}
		return playwrightjson.Report{}, fmt.Errorf("playwright producer: %w", err)
	}
	defer func() { _ = f.Close() }()
	report, err := playwrightjson.Decode(f)
	if err != nil {
		return playwrightjson.Report{}, fmt.Errorf("playwright producer: %w", err)
	}
	if err := playwrightCheckRun(root, report, specs); err != nil {
		return playwrightjson.Report{}, fmt.Errorf("playwright producer: %w", err)
	}
	return report, nil
}

// playwrightANSIRE matches the color escapes Playwright puts in messages.
var playwrightANSIRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

// playwrightCheckRun refuses a report that is not the account of the run the
// producer asked for: one the run reported an error of its own in; one with
// other than exactly one project (a test appears once per project, and the
// pinned reporter writes each copy as its own spec with the same file and
// title path, so a second project would read as a duplicate); one whose run
// was not one worker, no retries, and one repeat; one whose files are not
// relative to root's e2e/tests/; and one that covers a file the producer did
// not name.
func playwrightCheckRun(root string, report playwrightjson.Report, specs []string) error {
	if len(report.Errors) > 0 {
		return fmt.Errorf("the run reported %d error(s) of its own, first: %s", len(report.Errors), strings.TrimSpace(playwrightANSIRE.ReplaceAllString(report.Errors[0], "")))
	}
	if len(report.Projects) != 1 {
		return fmt.Errorf("the run covered %d projects, want exactly one", len(report.Projects))
	}
	p := report.Projects[0]
	if report.Workers != 1 || p.Retries != 0 || p.RepeatEach != 1 {
		return fmt.Errorf("the run used %d workers, retries %d, and repeat-each %d; the producer runs one worker, no retries, once", report.Workers, p.Retries, p.RepeatEach)
	}
	wantDir := filepath.Join(root, filepath.FromSlash(playwrightTestsDir))
	if !playwrightSameDir(filepath.FromSlash(report.RootDir), wantDir) {
		return fmt.Errorf("the report's rootDir %s is not the harness's test directory %s", report.RootDir, wantDir)
	}
	named := map[string]bool{}
	for _, s := range specs {
		named[s] = true
	}
	for _, f := range report.Files {
		if !named[f.Path] {
			return fmt.Errorf("the run covered %s, which the producer did not name", f.Path)
		}
		for _, tc := range f.Tests {
			if tc.ProjectName != p.Name {
				return fmt.Errorf("a test in %s belongs to project %q, not the run's %q", f.Path, tc.ProjectName, p.Name)
			}
		}
	}
	return nil
}

// playwrightSameDir reports whether a and b are the same directory, symlinks
// resolved where they exist.
func playwrightSameDir(a, b string) bool {
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return filepath.Clean(p)
	}
	return resolve(a) == resolve(b)
}

// --- Outcomes and emission (SI-294) -----------------------------------------------

// playwrightProducerAbsentSource is the disclosure source for a selected
// obligation whose named test did not run: its file does not exist, the run's
// report does not carry its title path, or the report shows it never started.
// No record; the obligation reads producer-missing, never a silent pass; an
// earlier record for its producer at the same commit is withdrawn and the
// disclosure says so (SI-238).
const playwrightProducerAbsentSource = "sync:playwright-producer-absent"

func playwrightAbsentDisclosure(s playwrightSelectedObligation, why string) disclosure.Disclosure {
	text := fmt.Sprintf("named Playwright test %q in %s %s; no evidence record was emitted for it", s.TitlePath, s.File, why)
	return disclosure.New(playwrightProducerAbsentSource, s.ObligationID, text)
}

// playwrightFileState is whether a named file exists as a regular file.
func playwrightFileState(root, file string) (bool, error) {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("playwright producer: %w", err)
	}
	return info.Mode().IsRegular(), nil
}

// playwrightVerdict maps one test's attempts to a verdict (SI-294), reading
// each attempt's own status, never Playwright's outcome, which calls an
// interrupted attempt "skipped": fail when any attempt failed, timed out, or
// was interrupted, or when the test needed another attempt (a test Playwright
// calls flaky is never a pass); otherwise pass when its one attempt passed and
// abstain when it was skipped (declaratively, at run time, or after an earlier
// failure in a serial group). ran is false when there was no attempt: the
// test did not run.
func playwrightVerdict(attempts []playwrightjson.Attempt) (verdict artifact.EvidenceVerdict, ran bool, err error) {
	if len(attempts) == 0 {
		return "", false, nil
	}
	for _, a := range attempts {
		switch a.Status {
		case playwrightjson.StatusFailed, playwrightjson.StatusTimedOut, playwrightjson.StatusInterrupted:
			return artifact.VerdictFail, true, nil
		}
	}
	if len(attempts) > 1 {
		return artifact.VerdictFail, true, nil
	}
	switch attempts[0].Status {
	case playwrightjson.StatusPassed:
		return artifact.VerdictPass, true, nil
	case playwrightjson.StatusSkipped:
		return artifact.VerdictAbstain, true, nil
	default:
		return "", true, fmt.Errorf("playwright producer: unrecognized attempt status %q", attempts[0].Status)
	}
}

// playwrightAbsence is one selected obligation the run recorded nothing for,
// with the disclosure that says why.
type playwrightAbsence struct {
	obligation playwrightSelectedObligation
	disclosed  disclosure.Disclosure
}

// playwrightBuildRecords maps each selected obligation to its record, grouped
// by owning spec ref, or to an absence. present says which named files exist;
// report is the run of those files. Two tests in a named file sharing a named
// title path is an operational error, returned before anything is built.
func playwrightBuildRecords(selected []playwrightSelectedObligation, present map[string]bool, report playwrightjson.Report, prov artifact.EvidenceProvenance) (map[string][]artifact.Evidence, []playwrightAbsence, error) {
	tests := map[string][]playwrightjson.Test{} // keyed by file name + "\x00" + title path
	for _, f := range report.Files {
		for _, tc := range f.Tests {
			key := f.Path + "\x00" + tc.JoinedTitlePath()
			tests[key] = append(tests[key], tc)
		}
	}
	for _, s := range selected {
		if n := len(tests[s.FileName()+"\x00"+s.TitlePath]); n > 1 {
			return nil, nil, fmt.Errorf("playwright producer: %d tests in %s share the title path %q, so it names no single test", n, s.File, s.TitlePath)
		}
	}

	bySpec := map[string][]artifact.Evidence{}
	var absent []playwrightAbsence
	for _, s := range selected {
		if !present[s.File] {
			absent = append(absent, playwrightAbsence{obligation: s, disclosed: playwrightAbsentDisclosure(s, "did not run: the file does not exist")})
			continue
		}
		matches := tests[s.FileName()+"\x00"+s.TitlePath]
		if len(matches) == 0 {
			absent = append(absent, playwrightAbsence{obligation: s, disclosed: playwrightAbsentDisclosure(s, "did not run: its title path is not in the run's report")})
			continue
		}
		verdict, ran, err := playwrightVerdict(matches[0].Attempts)
		if err != nil {
			return nil, nil, err
		}
		if !ran {
			absent = append(absent, playwrightAbsence{obligation: s, disclosed: playwrightAbsentDisclosure(s, "did not run: the report shows no attempt")})
			continue
		}
		statuses := make([]string, len(matches[0].Attempts))
		for i, a := range matches[0].Attempts {
			statuses[i] = a.Status
		}
		rec := artifact.Evidence{
			Schema:      "verdi.evidence/v1",
			EvidenceFor: []string{s.ACID},
			Kind:        s.Kind,
			Verdict:     verdict,
			Witness:     fmt.Sprintf("playwright test --workers=1 --retries=0 %s: %s: %s", s.File, s.TitlePath, strings.Join(statuses, ", ")),
			Producer:    s.ProducerRef,
			Provenance:  prov,
		}
		digest, err := namedTestDigest(rec)
		if err != nil {
			return nil, nil, err
		}
		rec.Digest = digest
		specRef := goTestSpecRef(s.SpecName)
		bySpec[specRef] = append(bySpec[specRef], rec)
	}
	return bySpec, absent, nil
}

// --- Orchestration -------------------------------------------------------------------

// playwrightProduceEvidence is `sync --produce`'s Playwright producer (see
// this file's package doc), called from runProduce after
// produceGoTestEvidence with the same provenance, commit, and job name. With
// no selected obligation naming an existing file it runs nothing at all (no
// Node work); otherwise it runs every such file once, in one run. The
// discovery walk's own disclosures (an undecodable or misfiled obligation)
// are not repeated here: produceGoTestEvidence, which runs first over the same
// walk, already printed them. Every other disclosure is printed to stdout.
//
// REPEAT PRODUCTION (SI-238). Each run replaces its managed subset at this
// commit (namedTestManagedSubset): every selected obligation's producer ref in
// its own spec, recorded or not; a selected producer this run did not record
// loses any earlier record, and its disclosure says so. Nothing is written
// unless the run and its report were trusted.
func playwrightProduceEvidence(ctx context.Context, root, commit, jobName string, runner playwrightRunner, prov artifact.EvidenceProvenance, stdout io.Writer) error {
	candidates, _, err := discoverTestProducerObligations(root)
	if err != nil {
		return err
	}
	selected, discl := playwrightSelectObligations(candidates, jobName)

	if len(selected) > 0 {
		present := map[string]bool{}
		var specs []string
		for _, s := range selected {
			if _, seen := present[s.File]; seen {
				continue
			}
			exists, err := playwrightFileState(root, s.File)
			if err != nil {
				return err
			}
			present[s.File] = exists
			if exists {
				specs = append(specs, s.FileName())
			}
		}
		sort.Strings(specs)

		var report playwrightjson.Report
		if len(specs) > 0 {
			if runner == nil {
				return errors.New("playwright producer: no Playwright runner is configured")
			}
			if report, err = playwrightExecuteRun(ctx, root, runner, specs); err != nil {
				return err
			}
		}
		bySpec, absent, err := playwrightBuildRecords(selected, present, report, prov)
		if err != nil {
			return err
		}
		managedFrom := make([]testProducerCandidate, len(selected))
		for i, s := range selected {
			managedFrom[i] = s.testProducerCandidate
		}
		withdrawn, err := writeManagedEvidence(root, commit, namedTestManagedSubset(managedFrom), bySpec)
		if err != nil {
			return err
		}
		for _, a := range absent {
			discl = append(discl, namedTestWithdrawalDisclosure(a.disclosed, a.obligation.testProducerCandidate, withdrawn))
		}
	}

	for _, d := range discl {
		fmt.Fprintln(stdout, disclosure.Render(d))
	}
	return nil
}
