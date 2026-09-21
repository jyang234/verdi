// Package oq5ledger is spike/self-governance oq-5 candidate (c): "leave
// the disclosures in the ledger and parse with a scratch gate test." It
// parses a committed COPY of the wave-1 SDD ledger
// (testdata/progress.md, taken verbatim from the read-only source named
// in this spike's lane brief: verdi-wt/readiness-recovery-w1/
// .superpowers/sdd/2026-09-19-readiness-recovery-wave-1/progress.md, at
// the moment this spike read it) and proves two facts the README's oq-5
// section reports: the ledger carries exactly 9 lines matching
// deferred/residual/parked, and a naive mechanical split of those lines
// into clauses over-counts relative to the 20 distinct items this
// spike's human read produced (docs/spikes/self-governance/
// disclosures.md) — candidate (c)'s central weakness, demonstrated
// rather than talked about. This is evidence only — nothing under
// docs/spikes/self-governance/_scratch/ is part of `go list ./...` or any
// gate; it never touches internal/journey, so its result does NOT mean
// these items are readiness-visible (they are not — see the README).
package oq5ledger

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// deferredLineRe matches a ledger line carrying the disclosure marker
// vocabulary this spike's story names: "deferred", "residual", or
// "parked", case-insensitive — the same test `grep -ni` used by hand.
var deferredLineRe = regexp.MustCompile(`(?i)deferred|residual|parked`)

// clauseSplitRe splits one matched line into semicolon-delimited clauses,
// the ledger's own convention for separating distinct items within one
// entry (see e.g. line 32's five-item "Deferred minors:" list).
var clauseSplitRe = regexp.MustCompile(`;\s*`)

func TestLedger_DeferredResidualParkedLineCount(t *testing.T) {
	data, err := os.ReadFile("testdata/progress.md")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	lines := strings.Split(string(data), "\n")
	var matched []string
	for _, l := range lines {
		if deferredLineRe.MatchString(l) {
			matched = append(matched, l)
		}
	}
	if len(matched) != 9 {
		t.Fatalf("matched %d lines, want 9 (spec/self-governance's own count); lines: %v", len(matched), matched)
	}
}

// TestLedger_NaiveClauseSplitOvercounts is candidate (c)'s central
// weakness, demonstrated rather than asserted away: a matched LINE is not
// one item, and there is no machine-detectable boundary in this free
// prose between a deferred/residual/parked clause and an unrelated
// sentence riding in the same line (e.g. line 29 also reports "M7
// exported constructors ride" — resolved, not deferred — right next to
// the M5 clause that IS). A naive semicolon-split of every matched line
// over-counts relative to this spike's hand-curated 21-raw/20-distinct
// enumeration in docs/spikes/self-governance/disclosures.md, because it
// also splits unrelated sentences in the same paragraph. This test
// asserts only that the naive count is strictly greater than the matched
// LINE count (over-counting genuinely occurs) and logs the exact number,
// rather than hard-coding a "corrected" value a human had to supply —
// hard-coding one would just be disclosures.md's hand curation smuggled
// back in as a magic number, not a scratch gate test parsing the ledger.
func TestLedger_NaiveClauseSplitOvercounts(t *testing.T) {
	data, err := os.ReadFile("testdata/progress.md")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	lines := strings.Split(string(data), "\n")

	var matchedLines, naiveClauses int
	for _, l := range lines {
		if !deferredLineRe.MatchString(l) {
			continue
		}
		matchedLines++
		for _, seg := range clauseSplitRe.Split(l, -1) {
			if strings.TrimSpace(seg) != "" {
				naiveClauses++
			}
		}
	}
	t.Logf("matched lines: %d; naive semicolon-split clauses: %d; hand-curated distinct items: 20 (disclosures.md)", matchedLines, naiveClauses)
	if naiveClauses <= matchedLines {
		t.Fatalf("naive clause split produced %d clauses over %d lines — expected over-counting (>1 clause/line on average); candidate (c)'s fragility did not reproduce, re-check the fixture", naiveClauses, matchedLines)
	}
}

// TestLedger_ItemsAreFreeProseNotStructuredFields proves candidate (c)'s
// structural weakness empirically rather than by assertion: no matched
// clause parses as a "reason:"/"owner:"/"clearing_condition:" structured
// field the way a real journey.Blocker would — every clause is free
// prose. A scratch gate test can COUNT prose clauses; it cannot derive an
// owner or a clearing condition from them without a human writing one,
// which is exactly the authoring step candidate (b)'s record file already
// requires and candidate (c) does not remove.
func TestLedger_ItemsAreFreeProseNotStructuredFields(t *testing.T) {
	data, err := os.ReadFile("testdata/progress.md")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	fieldRe := regexp.MustCompile(`(?i)\b(reason|owner|clearing_condition)\s*:`)
	if fieldRe.MatchString(string(data)) {
		t.Fatalf("fixture unexpectedly contains a structured reason:/owner:/clearing_condition: field — candidate (c)'s prose-only finding no longer holds, re-check the README")
	}
}
