package lintratchet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/disclosure"
)

// The check's exit statuses: every verb exits 0 (clean) / 1 (verdict) / 2
// (operational).
const (
	ExitClean       = 0
	ExitVerdict     = 1
	ExitOperational = 2
)

// growthDisclosureSource names the not-applicable growth comparison in the
// shared disclosure vocabulary (internal/disclosure).
const growthDisclosureSource = "lint-strict:growth-comparison"

// EarlierBaselineReader reads the baseline the growth comparison measures the
// committed one against. GitEarlier is the production reader.
type EarlierBaselineReader interface {
	ReadEarlierBaseline(ctx context.Context) (EarlierBaseline, error)
}

// CheckInput is one baseline check's inputs.
type CheckInput struct {
	// LintExit is golangci-lint's exit status. It runs with
	// --issues-exit-code=0, so anything but 0 is operational.
	LintExit int
	// ReportPath is golangci-lint's JSON report.
	ReportPath string
	// BaselinePath is the committed baseline in the working tree.
	BaselinePath string
	// Earlier reads the baseline at the merge base or HEAD's first parent.
	Earlier EarlierBaselineReader
}

// Check runs the baseline check (spec/strict-lint-gate ac-2) and returns its
// exit status: ExitVerdict when the current run reports a key more times than
// the committed baseline allows, the baseline allows a key more times than the
// run reports, or the baseline allows a key more times than the earlier
// baseline did; ExitOperational for a nonzero golangci-lint exit, a missing or
// malformed report or baseline, or an earlier commit that cannot be resolved
// or holds a malformed baseline; ExitClean otherwise. An earlier commit that
// resolves but holds no baseline makes the growth comparison not applicable,
// which is disclosed (ledger SI-311 (1)). Verdicts go to stdout, operational
// failures to stderr.
func Check(ctx context.Context, in CheckInput, stdout, stderr io.Writer) int {
	findings, baseline, err := readInputs(in.LintExit, in.ReportPath, in.BaselinePath)
	if err != nil {
		fmt.Fprintln(stderr, "lint-strict:", err)
		return ExitOperational
	}
	if in.Earlier == nil {
		fmt.Fprintln(stderr, "lint-strict: no earlier-baseline reader was given, so the growth comparison cannot run")
		return ExitOperational
	}
	earlier, err := in.Earlier.ReadEarlierBaseline(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "lint-strict:", err)
		return ExitOperational
	}

	current := CountFindings(findings)
	violations := Compare(current, baseline)
	if earlier.Present {
		violations = append(violations, CompareGrowth(baseline, earlier.Counts)...)
	} else {
		fmt.Fprintln(stdout, disclosure.Render(disclosure.New(growthDisclosureSource, earlier.Commit, fmt.Sprintf(
			"%s does not exist at %s, so the growth comparison does not apply there (ledger SI-311 (1)); the new-finding and stale-allowance comparisons still hold the baseline equal to the current findings",
			earlier.Path, earlier.Where))))
	}

	positions := positionsByKey(findings)
	for _, v := range violations {
		fmt.Fprintln(stdout, renderViolation(v, positions[v.Key], earlier))
	}
	if len(violations) > 0 {
		fmt.Fprintf(stdout, "lint-strict: FAILED with %d violation(s). Fix a new finding; never add it to the baseline. After a fix, regenerate the baseline (make lint-strict-baseline) so it drops the fixed finding's allowance.\n", len(violations))
		return ExitVerdict
	}
	grew := "the growth comparison did not apply"
	if earlier.Present {
		grew = fmt.Sprintf("it allows nothing more than the baseline at %s %s", earlier.Where, earlier.Commit)
	}
	fmt.Fprintf(stdout, "lint-strict OK: %d finding(s) under %d key(s), exactly what the committed baseline allows; %s.\n", len(findings), len(current), grew)
	return ExitClean
}

// readInputs refuses a nonzero golangci-lint exit, then reads and parses the
// report and the committed baseline.
func readInputs(lintExit int, reportPath, baselinePath string) ([]Finding, Counts, error) {
	if lintExit != 0 {
		return nil, nil, fmt.Errorf("golangci-lint exited %d; it runs with --issues-exit-code=0, so any nonzero exit is a failure other than reporting findings", lintExit)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading the golangci-lint report: %w", err)
	}
	findings, err := ParseReport(data)
	if err != nil {
		return nil, nil, err
	}
	raw, err := os.ReadFile(baselinePath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading the baseline: %w", err)
	}
	baseline, err := ParseBaseline(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", baselinePath, err)
	}
	return findings, baseline, nil
}

// positionsByKey lists where the run reported each key, as file:line.
func positionsByKey(findings []Finding) map[Key][]string {
	out := map[Key][]string{}
	for _, f := range findings {
		out[f.Key] = append(out[f.Key], fmt.Sprintf("%s:%d", f.File, f.Line))
	}
	for k := range out {
		slices.Sort(out[k])
	}
	return out
}

// renderViolation renders one violation for stdout.
func renderViolation(v Violation, at []string, earlier EarlierBaseline) string {
	head := fmt.Sprintf("lint-strict: %s: %s in %s: %s (flagged line %q)", v.Kind, v.Key.Linter, v.Key.Package, v.Key.Message, v.Key.Source)
	switch v.Kind {
	case KindNew:
		return fmt.Sprintf("%s: reported %d time(s), the baseline allows %d; at %s", head, v.Have, v.Allowed, strings.Join(at, ", "))
	case KindStale:
		return fmt.Sprintf("%s: the baseline allows %d, the run reports %d; remove the fixed finding's allowance", head, v.Allowed, v.Have)
	case KindGrowth:
		return fmt.Sprintf("%s: the baseline allows %d, the baseline at %s %s allows %d", head, v.Have, earlier.Where, earlier.Commit, v.Allowed)
	default:
		return fmt.Sprintf("%s: have %d, allowed %d", head, v.Have, v.Allowed)
	}
}

// BaselineInput is one baseline regeneration's inputs.
type BaselineInput struct {
	// LintExit is golangci-lint's exit status; anything but 0 is operational.
	LintExit int
	// ReportPath is golangci-lint's JSON report.
	ReportPath string
	// BaselinePath is where the baseline is written.
	BaselinePath string
}

// WriteBaseline regenerates the committed baseline from a golangci-lint
// report: exactly the report's findings, counted per key, in canonical JSON.
// It returns ExitClean, or ExitOperational for a nonzero golangci-lint exit, a
// missing or malformed report, or a failed write. It is never part of the
// gate: a new finding is fixed, never written into the baseline.
func WriteBaseline(in BaselineInput, stdout, stderr io.Writer) int {
	if in.LintExit != 0 {
		fmt.Fprintf(stderr, "lint-strict-baseline: golangci-lint exited %d; it runs with --issues-exit-code=0, so any nonzero exit is a failure other than reporting findings\n", in.LintExit)
		return ExitOperational
	}
	data, err := os.ReadFile(in.ReportPath)
	if err != nil {
		fmt.Fprintln(stderr, "lint-strict-baseline: reading the golangci-lint report:", err)
		return ExitOperational
	}
	findings, err := ParseReport(data)
	if err != nil {
		fmt.Fprintln(stderr, "lint-strict-baseline:", err)
		return ExitOperational
	}
	counts := CountFindings(findings)
	out, err := EncodeBaseline(counts)
	if err == nil {
		err = atomicfile.Write(in.BaselinePath, out, 0o644)
	}
	if err != nil {
		fmt.Fprintln(stderr, "lint-strict-baseline:", errors.Join(errors.New("writing the baseline"), err))
		return ExitOperational
	}
	fmt.Fprintf(stdout, "lint-strict-baseline: wrote %s: %d finding(s) under %d key(s).\n", in.BaselinePath, len(findings), len(counts))
	return ExitClean
}
