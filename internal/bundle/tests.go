package bundle

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const testsSchema = "verdi.tests/v1"

// PackageResult is one Go package's outcome within a TestSummary.
type PackageResult struct {
	Package  string `json:"package"`
	Status   string `json:"status"` // "pass" | "fail" | "skip"
	Tests    int    `json:"tests"`
	Failures int    `json:"failures"`
}

// TestSummary is schema `verdi.tests/v1` — a small, verdi-owned schema (not
// an upstream shape) summarizing one `go test -json` run: suite-level
// pass/fail plus a per-package breakdown, deliberately coarse (03
// §Declarations: "Unit tests deliberately stay coarse — suite pass/fail
// ... because per-test AC mapping would rot and poison the matrix's
// credibility").
type TestSummary struct {
	Schema   string          `json:"schema"`
	Suite    string          `json:"suite"` // "pass" | "fail"
	Packages []PackageResult `json:"packages"`
}

// goTestEvent is the subset of Go's stdlib `go test -json` event shape
// (the "test2json" format) this package reads. Decoded with a plain,
// non-strict json.Unmarshal per line deliberately: this is first-party Go
// toolchain output, not a verdi-go artifact under this module's
// strict-decode discipline (CLAUDE.md's "never import its packages, strict
// decode only" is about the upstream toolchain verdi does not own; `go
// test -json`'s event format is documented, stable, stdlib-owned, and may
// grow fields verdi does not need — e.g. FailedBuild — without that being
// drift worth failing closed on).
type goTestEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
}

// SummarizeGoTestJSON reads a `go test -json` event stream and produces a
// TestSummary: one PackageResult per package (in first-seen order), with
// Tests/Failures counted from per-test run/fail events and Status taken
// from each package's own terminal pass/fail/skip event (the event with an
// empty Test field). The overall Suite is "fail" if any package's Status
// is "fail" (a "skip" package — no test files — does not fail the suite),
// else "pass".
func SummarizeGoTestJSON(r io.Reader) (*TestSummary, error) {
	type acc struct {
		tests, failures int
		status          string
	}
	byPkg := make(map[string]*acc)
	var order []string

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var ev goTestEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			return nil, fmt.Errorf("bundle: parsing go test -json line %d: %w", lineNo, err)
		}
		if ev.Package == "" {
			return nil, fmt.Errorf("bundle: go test -json line %d: event has no Package", lineNo)
		}
		a, ok := byPkg[ev.Package]
		if !ok {
			a = &acc{}
			byPkg[ev.Package] = a
			order = append(order, ev.Package)
		}
		switch {
		case ev.Test != "" && ev.Action == "run":
			a.tests++
		case ev.Test != "" && ev.Action == "fail":
			a.failures++
		case ev.Test == "" && (ev.Action == "pass" || ev.Action == "fail" || ev.Action == "skip"):
			a.status = ev.Action
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("bundle: reading go test -json stream: %w", err)
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("bundle: go test -json stream contained no package events")
	}

	summary := &TestSummary{Schema: testsSchema, Suite: "pass"}
	for _, pkg := range order {
		a := byPkg[pkg]
		if a.status == "" {
			return nil, fmt.Errorf("bundle: go test -json: package %q has no terminal pass/fail/skip event (truncated stream?)", pkg)
		}
		summary.Packages = append(summary.Packages, PackageResult{
			Package:  pkg,
			Status:   a.status,
			Tests:    a.tests,
			Failures: a.failures,
		})
		if a.status == "fail" {
			summary.Suite = "fail"
		}
	}
	return summary, nil
}
