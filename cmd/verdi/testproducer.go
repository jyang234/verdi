// The per-test CI evidence producer (SI-228, plan R-CM-2; ledger SI-228,
// SI-229): 03 §Declarations and binding's coarse-by-default rule carries
// one sanctioned exception — an elaborated evidence obligation naming a
// single top-level Go test as its producer (`go-test:<package path
// relative to the module root>:<TopLevelTestName>`) is matched per test,
// not per suite. This file is that exception's one CI-side implementation:
// invoked only from `sync --produce` (sync.go's runProduce), after the
// coarse self-hosted producer (selfevidence.go), it walks every obligation
// on disk, selects the ones this CI job is authoritative for (SI-229's
// authoritative_source.ref == provenance.job_name), runs `go test -json
// -count=1 -run '^(...)$' ./<pkg>` once per named package through an
// injectable seam, and emits one record per selected obligation carrying
// that obligation's own kind and acceptance-criterion id.
//
// HONESTY. A renamed or removed test must surface as a missing producer, a
// closure blocker, never a silent pass — so a named test absent from the
// run emits no record, only a disclosure. But a malformed producer ref, or
// an obligation elsewhere in the store this reader cannot even decode,
// must never turn this shared CI step into an operational failure for
// every OTHER story's obligations (03 §Declarations and binding's own
// framing: "never as a silent pass" is about false positives, not about
// making one story's authoring mistake block CI for everyone else) — both
// are disclosed and skipped, exactly like an absent test. Only a runner
// that produces no output at all, or a go test -json stream that is
// malformed or ends before its package's own terminal event, is an
// operational error (exit 2): at that point nothing read from this
// invocation can be trusted.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/disclosure"
)

// --- Grammar (contract 1) ---------------------------------------------

// goTestProducerRefRe is the SI-228 grammar: "go-test:" then a package path
// relative to the module root (anything but a colon — a real Go package
// path never contains one), then a top-level test name (anything but a
// colon or a slash — a slash would name a subtest path, never top-level).
// Anchored full-match: a producer ref with an extra colon, an empty
// package, or an empty/subtest test segment is rejected as malformed
// rather than silently mis-split.
var goTestProducerRefRe = regexp.MustCompile(`^go-test:([^:]+):([^:/]+)$`)

// goTestProducerRef is a producer ref's parsed (package, top-level test)
// pair.
type goTestProducerRef struct {
	Package string
	Test    string
}

// parseGoTestProducerRef strictly parses a producer ref against the SI-228
// grammar. ok is false for anything not of the exact "go-test:<pkg>:<Test>"
// shape (wrong scheme, missing test, empty package, a subtest path, or an
// extra colon).
func parseGoTestProducerRef(ref string) (goTestProducerRef, bool) {
	m := goTestProducerRefRe.FindStringSubmatch(ref)
	if m == nil {
		return goTestProducerRef{}, false
	}
	return goTestProducerRef{Package: m[1], Test: m[2]}, true
}

// --- Discovery (contract 2, part 1: find every candidate) --------------

// testProducerCandidate is one elaborated, test-kind, ci-job-sourced
// obligation found anywhere in the store — before this job's own
// authoritative_source match (selectGoTestObligations) or producer-ref
// grammar check (parseGoTestProducerRef) have been applied.
type testProducerCandidate struct {
	// SpecName is the obligation's own owning story spec (the id's
	// story-slug segment, spec/obligation-artifact DC-2) — where the
	// emitted record is ultimately written, derived/<RefSlug(spec/
	// SpecName)>/<commit>/verdicts.json.
	SpecName string
	ACID     string
	Kind     artifact.EvidenceKind
	// ProducerRef is the obligation's declared quality.producer.ref —
	// unparsed; grammar validity is checked separately (selection, not
	// discovery), so a malformed ref still reaches selection where its
	// disclosure can name the obligation ID it came from.
	ProducerRef string
	// JobRef is the obligation's declared quality.authoritative_source.ref
	// (SI-229): compared against the running CI job's own declared name.
	JobRef string
	// ObligationID is the decoded obligation's own id (e.g.
	// "obligation/stale-decline--ac-1--behavioral"), used only to name the
	// obligation in a disclosure.
	ObligationID string
}

// testProducerObligationUnreadableSource is the disclosure source for an
// obligation file this discovery walk could not read or decode — a
// pre-existing authoring fault elsewhere in the store, never this job's
// concern to repair, and never allowed to fail sync --produce for every
// other story (see this file's package doc).
const testProducerObligationUnreadableSource = "sync:go-test-producer-obligation-unreadable"

func testProducerObligationUnreadableDisclosure(root, path string, err error) disclosure.Disclosure {
	rel, relErr := filepath.Rel(root, path)
	if relErr != nil {
		rel = path
	}
	text := fmt.Sprintf("could not be read or decoded as an obligation (%v); no per-test evidence was considered for it", err)
	return disclosure.New(testProducerObligationUnreadableSource, filepath.ToSlash(rel), text)
}

// discoverTestProducerObligations walks every spec directory under
// .verdi/obligations/ and returns every elaborated obligation whose
// declared producer kind is "test" and whose authoritative source kind is
// "ci-job" (03 §Declarations: checker and authenticated-human producers
// have no per-test emitter; an unresolved-design-debt or legacy-
// unelaborated obligation is never a candidate). A missing .verdi/
// obligations/ tree is the ordinary "no obligations authored yet" case
// (evidence.Obligations's own absence posture), not an error. An
// individual obligation file that cannot be read or decoded is disclosed
// and skipped, never fails the whole walk (see package doc).
func discoverTestProducerObligations(root string) ([]testProducerCandidate, []disclosure.Disclosure, error) {
	obligationsRoot := filepath.Join(root, ".verdi", "obligations")
	specDirs, err := os.ReadDir(obligationsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("go-test producer: reading %s: %w", obligationsRoot, err)
	}
	sort.Slice(specDirs, func(i, j int) bool { return specDirs[i].Name() < specDirs[j].Name() })

	var candidates []testProducerCandidate
	var discl []disclosure.Disclosure
	for _, sd := range specDirs {
		if !sd.IsDir() {
			continue
		}
		dir := filepath.Join(obligationsRoot, sd.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			return nil, nil, fmt.Errorf("go-test producer: reading %s: %w", dir, err)
		}
		sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
				continue
			}
			path := filepath.Join(dir, f.Name())
			raw, err := os.ReadFile(path)
			if err != nil {
				discl = append(discl, testProducerObligationUnreadableDisclosure(root, path, err))
				continue
			}
			fm, _, err := artifact.SplitFrontmatter(raw)
			if err != nil {
				discl = append(discl, testProducerObligationUnreadableDisclosure(root, path, err))
				continue
			}
			decoded, err := artifact.DecodeObligation(fm)
			if err != nil {
				discl = append(discl, testProducerObligationUnreadableDisclosure(root, path, err))
				continue
			}
			if decoded.Quality == nil || decoded.Quality.State != artifact.ObligationQualityElaborated {
				continue
			}
			if decoded.Quality.Producer.Kind != artifact.ObligationProducerTest {
				continue
			}
			if decoded.Quality.AuthoritativeSource.Kind != artifact.ObligationSourceCIJob {
				continue
			}
			ref, err := artifact.ParseRef(decoded.ID)
			if err != nil {
				discl = append(discl, testProducerObligationUnreadableDisclosure(root, path, err))
				continue
			}
			storySlug, acID, _, ok := artifact.SplitObligationName(ref.Name)
			if !ok {
				discl = append(discl, testProducerObligationUnreadableDisclosure(root, path, fmt.Errorf("id %q does not split into <story-slug>--<ac-id>--<for-kind>", decoded.ID)))
				continue
			}
			candidates = append(candidates, testProducerCandidate{
				SpecName:     storySlug,
				ACID:         acID,
				Kind:         decoded.ForKind,
				ProducerRef:  decoded.Quality.Producer.Ref,
				JobRef:       decoded.Quality.AuthoritativeSource.Ref,
				ObligationID: decoded.ID,
			})
		}
	}
	return candidates, discl, nil
}

// --- Selection (contract 2, part 2: this job's own candidates) ---------

// selectedGoTestObligation is one candidate this CI job is authoritative
// for, with its producer ref already grammar-parsed into (package, test).
type selectedGoTestObligation struct {
	testProducerCandidate
	Package string
	Test    string
}

// goTestProducerMalformedRefSource is the disclosure source for an
// elaborated test-producer obligation whose producer ref fails the SI-228
// grammar (contract 1: "never an operational error that breaks CI for
// every story").
const goTestProducerMalformedRefSource = "sync:go-test-producer-malformed-ref"

func goTestProducerMalformedRefDisclosure(c testProducerCandidate) disclosure.Disclosure {
	text := fmt.Sprintf("declares producer %q, which does not match the go-test:<package>:<TopLevelTestName> grammar; no evidence record was emitted for it", c.ProducerRef)
	return disclosure.New(goTestProducerMalformedRefSource, c.ObligationID, text)
}

// selectGoTestObligations narrows candidates to exactly this CI job's own
// authoritative obligations (SI-229: authoritative_source.ref ==
// jobName), then grammar-parses each survivor's producer ref. jobName ==
// "" (not running in a named CI job at all) naturally selects nothing,
// since an elaborated obligation's authoritative_source.ref is always
// non-blank (internal/artifact/obligation.go's Validate). A grammar
// failure is disclosed and excluded, never returned as an error. The
// returned slice is sorted (package, test, spec, ac) for deterministic
// execution grouping and emission order.
func selectGoTestObligations(candidates []testProducerCandidate, jobName string) ([]selectedGoTestObligation, []disclosure.Disclosure) {
	var selected []selectedGoTestObligation
	var discl []disclosure.Disclosure
	for _, c := range candidates {
		if jobName == "" || c.JobRef != jobName {
			continue
		}
		parsed, ok := parseGoTestProducerRef(c.ProducerRef)
		if !ok {
			discl = append(discl, goTestProducerMalformedRefDisclosure(c))
			continue
		}
		selected = append(selected, selectedGoTestObligation{testProducerCandidate: c, Package: parsed.Package, Test: parsed.Test})
	}
	sort.Slice(selected, func(i, j int) bool {
		a, b := selected[i], selected[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.Test != b.Test {
			return a.Test < b.Test
		}
		if a.SpecName != b.SpecName {
			return a.SpecName < b.SpecName
		}
		return a.ACID < b.ACID
	})
	return selected, discl
}

// --- Execution (contract 3) ---------------------------------------------

// namedGoTestRunner abstracts one `go test -json -count=1 -run <pattern>
// ./<pkg>` invocation, restricted to an explicit package and run pattern —
// the per-test producer's own execution seam, distinct from goTestRunner's
// whole-module `./...` run in sync_regen.go (CLAUDE.md: no exec in any
// test; hermetic fakes only in unit tests, the real runner only in
// production and in this file's own one hermetic integration test).
type namedGoTestRunner interface {
	// RunNamedGoTest runs `go test -json -count=1 -run <runPattern>
	// ./<pkg>` with dir as the module root and returns its stdout. A
	// failing or skipped test is not an error here — that is exactly the
	// signal readNamedTestOutcomes reads; only a truly broken invocation
	// (no output at all) is an error.
	RunNamedGoTest(ctx context.Context, dir, pkg, runPattern string) ([]byte, error)
}

// realNamedGoTestRunner execs the real toolchain. Never used by this
// module's own tests (CLAUDE.md: no exec in any test) except the one
// hermetic integration test this file documents, which runs it against a
// tiny, dependency-free fixture module under testdata/.
type realNamedGoTestRunner struct{}

func (realNamedGoTestRunner) RunNamedGoTest(ctx context.Context, dir, pkg, runPattern string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "go", "test", "-json", "-count=1", "-run", runPattern, "./"+pkg)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run() // a failing/skipped test exits nonzero; the JSON stream is what matters
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("go test -json produced no output for package %s: %s", pkg, stderr.String())
	}
	return stdout.Bytes(), nil
}

// goTestRunPattern builds the `-run` alternation from a package's wanted
// top-level test names: "^(A|B|...)$", sorted for a deterministic,
// reproducible command line. Every name here already passed
// parseGoTestProducerRef, so it is a plain Go identifier — never a regex
// metacharacter that would need escaping.
func goTestRunPattern(tests []string) string {
	sorted := append([]string(nil), tests...)
	sort.Strings(sorted)
	return "^(" + strings.Join(sorted, "|") + ")$"
}

// goModulePath reports the module path root/go.mod declares, and whether
// root is a Go module root at all. The go command reports every test2json
// event's Package as the package's full import path — the module path joined
// with the package directory relative to the module root
// (cmd/go/internal/test: test2json.NewConverter(..., p.ImportPath, ...)) —
// and "the module root directory is the directory that contains the go.mod
// file" (Go Modules Reference, go.mod files). No go.mod at root is
// present == false with no error; a go.mod that cannot be read, or that does
// not declare exactly one module path, is an error.
func goModulePath(root string) (modulePath string, present bool, err error) {
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("go-test producer: reading go.mod: %w", err)
	}
	modulePath, err = parseModuleDirective(raw)
	if err != nil {
		return "", true, fmt.Errorf("go-test producer: %s: %w", filepath.Join(root, "go.mod"), err)
	}
	return modulePath, true, nil
}

// parseModuleDirective extracts the one module path a go.mod declares, per
// the Go Modules Reference grammar
// `ModuleDirective = "module" ( ModulePath | "(" newline ModulePath newline ")" ) newline`,
// where `//` starts a comment and a ModulePath may be a quoted string.
func parseModuleDirective(data []byte) (string, error) {
	fields := func(line string) []string {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		return strings.Fields(line)
	}
	lines := strings.Split(string(data), "\n")
	var paths []string
	for i := 0; i < len(lines); i++ {
		f := fields(lines[i])
		if len(f) == 0 || f[0] != "module" {
			continue
		}
		if len(f) != 2 {
			return "", fmt.Errorf("malformed module directive %q", strings.TrimSpace(lines[i]))
		}
		if f[1] != "(" {
			paths = append(paths, f[1])
			continue
		}
		closed := false
		for i++; i < len(lines); i++ {
			g := fields(lines[i])
			if len(g) == 1 && g[0] == ")" {
				closed = true
				break
			}
			if len(g) > 1 {
				return "", fmt.Errorf("malformed module block line %q", strings.TrimSpace(lines[i]))
			}
			paths = append(paths, g...)
		}
		if !closed {
			return "", errors.New("unterminated module block")
		}
	}
	if len(paths) != 1 {
		return "", fmt.Errorf("want exactly one module path, found %d", len(paths))
	}
	p := paths[0]
	if strings.HasPrefix(p, `"`) || strings.HasPrefix(p, "`") {
		unquoted, err := strconv.Unquote(p)
		if err != nil {
			return "", fmt.Errorf("malformed quoted module path %s: %w", p, err)
		}
		p = unquoted
	}
	if p == "" || strings.ContainsAny(p, " \t\r\n") {
		return "", fmt.Errorf("invalid module path %q", p)
	}
	return p, nil
}

// executeGoTestProducers groups selected by package, runs exactly one `go
// test -json` per distinct package (restricted to the union of that
// package's wanted top-level test names), and strict-decodes each run's
// event stream against that package's full import path (modulePath joined
// with the relative package path, as internal/publicrelease's runner joins
// them). Returns outcomes keyed [package][test]; a wanted test
// absent from its package's map means it never ran (readNamedTestOutcomes
// still succeeded — the package reached its own terminal event — the named
// test's own terminal event just never appeared). A runner or decode
// failure is returned as an operational error, never swallowed.
func executeGoTestProducers(ctx context.Context, root, modulePath string, runner namedGoTestRunner, selected []selectedGoTestObligation) (map[string]map[string]testOutcome, error) {
	byPkg := map[string]map[string]bool{}
	var pkgOrder []string
	for _, s := range selected {
		if byPkg[s.Package] == nil {
			byPkg[s.Package] = map[string]bool{}
			pkgOrder = append(pkgOrder, s.Package)
		}
		byPkg[s.Package][s.Test] = true
	}
	sort.Strings(pkgOrder)

	results := make(map[string]map[string]testOutcome, len(pkgOrder))
	for _, pkg := range pkgOrder {
		tests := make([]string, 0, len(byPkg[pkg]))
		for t := range byPkg[pkg] {
			tests = append(tests, t)
		}
		out, err := runner.RunNamedGoTest(ctx, root, pkg, goTestRunPattern(tests))
		if err != nil {
			return nil, fmt.Errorf("go-test producer: %w", err)
		}
		outcomes, err := readNamedTestOutcomes(bytes.NewReader(out), modulePath+"/"+pkg)
		if err != nil {
			return nil, fmt.Errorf("go-test producer: %w", err)
		}
		results[pkg] = outcomes
	}
	return results, nil
}

// --- Reading (contract 4): strict test2json decoding --------------------

// testOutcome is one top-level (or subtest) Go test's own terminal
// disposition, taken verbatim from its test2json Action.
type testOutcome string

const (
	testOutcomePass testOutcome = "pass"
	testOutcomeFail testOutcome = "fail"
	testOutcomeSkip testOutcome = "skip"
)

// goTestJSONEvent is the complete `go test -json` (test2json) event shape
// this reader accepts. Every field the toolchain emits is modeled so
// DisallowUnknownFields rejects only a genuinely foreign shape, never a
// legitimate event this reader simply does not use — mirroring
// internal/publicrelease/events.go's own strict-decode precedent (this
// file's own reader, not a shared import: publicrelease's is a per-package
// completeness check with a fixed required-test list, never one that
// accepts fail/skip as legitimate outcomes, which is exactly what this
// reader must do — a different concern, not a copy-pasteable one).
type goTestJSONEvent struct {
	Time        string  `json:"Time,omitempty"`
	Action      string  `json:"Action"`
	Package     string  `json:"Package"`
	Test        string  `json:"Test,omitempty"`
	Elapsed     float64 `json:"Elapsed,omitempty"`
	Output      string  `json:"Output,omitempty"`
	FailedBuild string  `json:"FailedBuild,omitempty"`
}

// readNamedTestOutcomes strict-decodes one package's `go test -json`
// stream and returns every test (top-level or subtest) whose own terminal
// pass/fail/skip event appeared, keyed by its exact Test name. Output,
// pause, and cont events are accepted and ignored (never inspected or
// re-emitted — internal/publicrelease/events.go's own "private source text
// must never enter the publishable report" discipline, applied here even
// though this reader emits no prose at all). A run event without a
// following terminal event, a terminal event without a preceding run
// event, a duplicate terminal event for the same test, an unexpected
// package, an unknown action, or a stream that ends before the package's
// own terminal event (whether by malformed JSON or genuine truncation) is
// a returned error — contract 4's "malformed or truncated stream is an
// operational error, never a record".
func readNamedTestOutcomes(r io.Reader, pkg string) (map[string]testOutcome, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	outcomes := map[string]testOutcome{}
	running := map[string]bool{}
	started, finished := false, false
	eventNo := 0

	for {
		var e goTestJSONEvent
		if err := dec.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("go test -json for package %s: malformed event %d: %w", pkg, eventNo+1, err)
		}
		eventNo++

		if finished {
			return nil, fmt.Errorf("go test -json for package %s: event %d (%s) after package completion", pkg, eventNo, e.Action)
		}
		if e.Package != pkg {
			return nil, fmt.Errorf("go test -json for package %s: event %d names unexpected package %q", pkg, eventNo, e.Package)
		}

		switch e.Action {
		case "start":
			if started {
				return nil, fmt.Errorf("go test -json for package %s: duplicate start event (event %d)", pkg, eventNo)
			}
			started = true
		case "run":
			if !started || e.Test == "" || running[e.Test] {
				return nil, fmt.Errorf("go test -json for package %s: invalid run event for %q (event %d)", pkg, e.Test, eventNo)
			}
			if _, done := outcomes[e.Test]; done {
				return nil, fmt.Errorf("go test -json for package %s: test %q ran again after its terminal event (event %d)", pkg, e.Test, eventNo)
			}
			running[e.Test] = true
		case "pass", "fail", "skip":
			if e.Test == "" {
				if !started || len(running) > 0 {
					return nil, fmt.Errorf("go test -json for package %s: package completed with %d test(s) still running (event %d)", pkg, len(running), eventNo)
				}
				finished = true
				continue
			}
			if _, dup := outcomes[e.Test]; dup {
				return nil, fmt.Errorf("go test -json for package %s: duplicate terminal event for %q (event %d)", pkg, e.Test, eventNo)
			}
			if !running[e.Test] {
				return nil, fmt.Errorf("go test -json for package %s: terminal event for %q without a preceding run event (event %d)", pkg, e.Test, eventNo)
			}
			delete(running, e.Test)
			outcomes[e.Test] = testOutcome(e.Action)
		case "output", "pause", "cont":
			if !started {
				return nil, fmt.Errorf("go test -json for package %s: event %d (%s) before package start", pkg, eventNo, e.Action)
			}
		default:
			return nil, fmt.Errorf("go test -json for package %s: unknown action %q (event %d)", pkg, e.Action, eventNo)
		}
	}

	if !finished {
		return nil, fmt.Errorf("go test -json for package %s: stream ended before the package's own terminal event (truncated)", pkg)
	}
	return outcomes, nil
}

// --- Emission (contract 5) ----------------------------------------------

// goTestProducerAbsentSource is the disclosure source for a selected
// obligation's named test that never appeared with its own terminal event
// in its package's run (renamed, removed, or otherwise never executed) —
// contract 1/4: no record, a disclosure naming the obligation, and the
// obligation reads producer-missing at fold time, never a silent pass.
const goTestProducerAbsentSource = "sync:go-test-producer-absent"

func goTestProducerAbsentDisclosure(s selectedGoTestObligation) disclosure.Disclosure {
	text := fmt.Sprintf("named test %s in package %s did not run (no terminal event); no evidence record was emitted for it", s.Test, s.Package)
	return disclosure.New(goTestProducerAbsentSource, s.ObligationID, text)
}

func goTestProducerNoModuleDisclosure(s selectedGoTestObligation) disclosure.Disclosure {
	text := fmt.Sprintf("named test %s in package %s did not run: the store root has no go.mod, so it is not the Go module root the package path is relative to; no evidence record was emitted for it", s.Test, s.Package)
	return disclosure.New(goTestProducerAbsentSource, s.ObligationID, text)
}

// verdictForOutcome maps a named test's own terminal test2json action to
// an evidence verdict (contract 5: "pass only on that test's own terminal
// pass event, fail on its fail event, abstain when it was skipped").
func verdictForOutcome(o testOutcome) (artifact.EvidenceVerdict, error) {
	switch o {
	case testOutcomePass:
		return artifact.VerdictPass, nil
	case testOutcomeFail:
		return artifact.VerdictFail, nil
	case testOutcomeSkip:
		return artifact.VerdictAbstain, nil
	default:
		return "", fmt.Errorf("go-test producer: unrecognized test outcome %q", o)
	}
}

// namedTestDigest hashes rec's declared content — kind, producer, the AC
// set it attests, and (unlike selfHostedDigest's always-pass records) the
// verdict itself, which genuinely varies per run — mirroring
// internal/bundle's recordDigest posture (02 §Generated artifacts and
// digests): a content-address of the upstream fact this record asserts.
func namedTestDigest(rec artifact.Evidence) (string, error) {
	keyed := struct {
		Kind        artifact.EvidenceKind    `json:"kind"`
		Producer    string                   `json:"producer"`
		EvidenceFor []string                 `json:"evidence_for"`
		Verdict     artifact.EvidenceVerdict `json:"verdict"`
	}{Kind: rec.Kind, Producer: rec.Producer, EvidenceFor: rec.EvidenceFor, Verdict: rec.Verdict}
	digest, err := canonjson.Digest(keyed)
	if err != nil {
		return "", fmt.Errorf("go-test producer: computing digest: %w", err)
	}
	return digest, nil
}

// buildGoTestRecords turns each selected obligation's outcome (looked up
// in results[pkg][test]) into its own evidence record, grouped by owning
// spec — the shape writeSelfHostedEvidence already merges and writes
// (contract 5: "merged into the owning spec's own derived/<...>/
// verdicts.json through mergeEvidenceByProducer"). A selected obligation
// whose test never ran is disclosed and excluded, never an error.
func buildGoTestRecords(selected []selectedGoTestObligation, results map[string]map[string]testOutcome, prov artifact.EvidenceProvenance) (map[string][]artifact.Evidence, []disclosure.Disclosure, error) {
	bySpec := map[string][]artifact.Evidence{}
	var discl []disclosure.Disclosure
	for _, s := range selected {
		outcome, ok := results[s.Package][s.Test]
		if !ok {
			discl = append(discl, goTestProducerAbsentDisclosure(s))
			continue
		}
		verdict, err := verdictForOutcome(outcome)
		if err != nil {
			return nil, nil, err
		}
		rec := artifact.Evidence{
			Schema:      "verdi.evidence/v1",
			EvidenceFor: []string{s.ACID},
			Kind:        s.Kind,
			Verdict:     verdict,
			Witness:     fmt.Sprintf("go test -json -count=1 -run ... ./%s: %s %s", s.Package, s.Test, outcome),
			Producer:    s.ProducerRef,
			Provenance:  prov,
		}
		digest, err := namedTestDigest(rec)
		if err != nil {
			return nil, nil, err
		}
		rec.Digest = digest
		// writeSelfHostedEvidence (and its own store.RefSlug keying) expects
		// a full spec ref, not the bare story slug SplitObligationName
		// returns — the same "spec/" + name convention
		// evidence.AssessObligation's own wantVerifies uses.
		specRef := "spec/" + s.SpecName
		bySpec[specRef] = append(bySpec[specRef], rec)
	}
	return bySpec, discl, nil
}

// --- Orchestration --------------------------------------------------------

// produceGoTestEvidence is `sync --produce`'s per-test CI evidence
// producer (see this file's package doc). Called from runProduce
// alongside produceSelfHostedEvidence, sharing the same provenance and
// commit. jobName is the running CI job's own declared name
// (forge.CIInfo.JobName, SI-229) — empty outside a detected CI job, which
// naturally selects zero obligations (an elaborated obligation's
// authoritative_source.ref is never blank) and makes this a silent no-op,
// matching contract 2's "only inside sync --produce in CI" without a
// separate gate. Every disclosure collected along the way (a malformed
// producer ref, an undecodable obligation elsewhere in the store, or a
// named test that never ran) is rendered to stdout; only a runner or
// stream failure returns an error (the caller's exit 2).
func produceGoTestEvidence(ctx context.Context, root, commit, jobName string, runner namedGoTestRunner, prov artifact.EvidenceProvenance, stdout io.Writer) error {
	candidates, discl, err := discoverTestProducerObligations(root)
	if err != nil {
		return err
	}
	selected, selectDiscl := selectGoTestObligations(candidates, jobName)
	discl = append(discl, selectDiscl...)

	if len(selected) > 0 {
		modulePath, isModuleRoot, err := goModulePath(root)
		if err != nil {
			return err
		}
		if !isModuleRoot {
			// A producer ref's package path is relative to the module root; a
			// store root with no go.mod has none, so no named test can run.
			for _, s := range selected {
				discl = append(discl, goTestProducerNoModuleDisclosure(s))
			}
			selected = nil
		}
		results, err := executeGoTestProducers(ctx, root, modulePath, runner, selected)
		if err != nil {
			return err
		}
		bySpec, buildDiscl, err := buildGoTestRecords(selected, results, prov)
		if err != nil {
			return err
		}
		discl = append(discl, buildDiscl...)
		if len(bySpec) > 0 {
			if err := writeSelfHostedEvidence(root, commit, bySpec); err != nil {
				return err
			}
		}
	}

	for _, d := range discl {
		fmt.Fprintln(stdout, disclosure.Render(d))
	}
	return nil
}
