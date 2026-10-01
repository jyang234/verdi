// Package lintratchet is the strict lint gate's baseline check
// (spec/strict-lint-gate dc-1). It parses golangci-lint's JSON report, keys
// and counts its findings, and compares them with the committed baseline,
// .golangci.strict-baseline.json, and that baseline with the one at the merge
// base with the default branch (on the default branch itself, at HEAD's first
// parent). The check alone decides the gate's exit status: 0 when the run's
// findings equal the baseline and the baseline did not grow, 1 for a new
// finding, a stale allowance, or a grown baseline, and 2 for any operational
// failure, including any nonzero golangci-lint exit (golangci-lint runs with
// --issues-exit-code=0, so a reported finding is data).
//
// A finding's key is (linter, package, message, flagged source line):
// counted per key because identical findings repeat within a package, and
// without file or line, so a line shift or a move within a package keeps an
// old finding old, while editing the flagged line or moving it to another
// package makes it new (strict-lint-target-v2 dc-3). Its package is its file's
// repository-relative directory, and its flagged source line is
// golangci-lint's SourceLines joined by newlines (ledger SI-311).
package lintratchet

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// Key identifies a finding for the baseline.
type Key struct {
	Linter  string
	Package string
	Message string
	Source  string
}

// Finding is one reported issue: its key and where the run reported it. The
// position is never part of the key; it only tells a reader where a new
// finding is.
type Finding struct {
	Key  Key
	File string
	Line int
}

// Counts maps each key to how many times it occurs.
type Counts map[Key]int

// typecheckLinter is the name golangci-lint reports a package it could not
// load under. Its "finding" means the gated linters never analyzed that
// package, which is a golangci-lint failure other than reporting findings,
// never data (spec/strict-lint-gate ac-2).
const typecheckLinter = "typecheck"

// The report's shape is golangci-lint v2.5.0's own (the Makefile's pin):
// printers.JSONResult, report.Data, report.Warning, report.LinterData,
// result.Issue, go/token.Position, result.Range, and analysis.SuggestedFix
// with its TextEdit. Every one of these is fixed by that version, so each is
// decoded with unknown fields refused. Pointers mark the fields that must be
// present, not merely zero.
type jsonReport struct {
	Issues *[]jsonIssue
	Report *jsonReportData
}

type jsonReportData struct {
	Warnings []jsonWarning
	Linters  []jsonLinterData
	Error    string
}

type jsonWarning struct {
	Tag  string
	Text string
}

type jsonLinterData struct {
	Name    string
	Enabled bool
}

type jsonIssue struct {
	FromLinter           string
	Text                 string
	Severity             string
	SourceLines          []string
	Pos                  jsonPosition
	LineRange            *jsonRange
	HunkPos              int
	SuggestedFixes       []jsonSuggestedFix
	ExpectNoLint         bool
	ExpectedNoLintLinter string
}

type jsonPosition struct {
	Filename string
	Offset   int
	Line     int
	Column   int
}

type jsonRange struct {
	From int
	To   int
}

type jsonSuggestedFix struct {
	Message   string
	TextEdits []jsonTextEdit
}

type jsonTextEdit struct {
	Pos     int
	End     int
	NewText []byte
}

// ParseReport decodes golangci-lint's JSON report strictly and returns its
// findings in report order. A report that is truncated, carries anything
// after its one JSON value or a field the pinned schema does not write, lacks
// its Issues or Report section, records a run error, names a file outside the
// repository, or reports a package golangci-lint could not load is refused.
func ParseReport(data []byte) ([]Finding, error) {
	var report jsonReport
	if err := artifact.DecodeStrictJSON(data, &report); err != nil {
		return nil, fmt.Errorf("golangci-lint report: %w", err)
	}
	if report.Issues == nil {
		return nil, errors.New("golangci-lint report: no Issues array")
	}
	if report.Report == nil {
		return nil, errors.New("golangci-lint report: no Report section")
	}
	if report.Report.Error != "" {
		return nil, fmt.Errorf("golangci-lint report: the run recorded an error: %s", report.Report.Error)
	}
	var findings []Finding
	for i, issue := range *report.Issues {
		finding, err := issueFinding(issue)
		if err != nil {
			return nil, fmt.Errorf("golangci-lint report: issue %d: %w", i, err)
		}
		findings = append(findings, finding)
	}
	return findings, nil
}

// issueFinding keys one issue.
func issueFinding(issue jsonIssue) (Finding, error) {
	switch {
	case issue.FromLinter == "":
		return Finding{}, errors.New("no linter")
	case issue.FromLinter == typecheckLinter:
		return Finding{}, fmt.Errorf("golangci-lint could not load the code at %s (reported as %q: %s), so the gated linters never analyzed it", issue.Pos.Filename, typecheckLinter, issue.Text)
	case issue.Text == "":
		return Finding{}, errors.New("no message")
	case issue.Pos.Filename == "":
		return Finding{}, errors.New("no file")
	}
	file := path.Clean(issue.Pos.Filename)
	if path.IsAbs(file) || file == ".." || strings.HasPrefix(file, "../") {
		return Finding{}, fmt.Errorf("file %q is not repository-relative: the strict configuration must sit at the repository root, so golangci-lint reports paths relative to it", issue.Pos.Filename)
	}
	return Finding{
		Key: Key{
			Linter:  issue.FromLinter,
			Package: path.Dir(file),
			Message: issue.Text,
			Source:  strings.Join(issue.SourceLines, "\n"),
		},
		File: file,
		Line: issue.Pos.Line,
	}, nil
}

// CountFindings counts findings per key.
func CountFindings(findings []Finding) Counts {
	counts := Counts{}
	for _, f := range findings {
		counts[f.Key]++
	}
	return counts
}
