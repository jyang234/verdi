package specimport

import "fmt"

// recomputeCoverage rebuilds every source's Coverage from fields' OWN spans
// — never plan.Coverage, which was computed BEFORE prepareCandidateFields
// may have displaced a mapped statement span with a spanless
// generated-deferral field (Task 2 handoff: "Never reuse stale
// Plan.Coverage after deferral. Displaced spans become retained or
// unresolved under RetainUnmapped, with byte totals reasserted; generated
// fields have no source span"). For a non-deferred request fields' spans
// are byte-identical to plan.Fields' own, so this reproduces plan.Coverage
// exactly — safe to call unconditionally for every format/request.
func recomputeCoverage(request Request, plan Plan, fields []Field) []Coverage {
	mappedBySource := make(map[string][]mappedSpan, len(plan.Sources))
	for _, f := range fields {
		for _, sp := range f.Spans {
			mappedBySource[sp.SourceID] = append(mappedBySource[sp.SourceID], mappedSpan{start: sp.Start, end: sp.End, target: f.Target})
		}
	}
	if request.Format == FormatNative {
		for _, snap := range plan.Sources {
			if snap.ID == request.Primary {
				mappedBySource[request.Primary] = []mappedSpan{{start: 0, end: len(snap.Data), target: nativeCoverageTarget}}
			}
		}
	}

	coverages := make([]Coverage, 0, len(plan.Sources))
	for _, snap := range plan.Sources {
		coverages = append(coverages, buildCoverage(snap.ID, len(snap.Data), mappedBySource[snap.ID], request.RetainUnmapped))
	}
	return coverages
}

// unresolvedCoverageFindings mirrors Normalize's own per-source
// unresolved-coverage check (normalize.go), applied to a — possibly
// recomputed — Coverage list, so a fresh recompute after deferral always
// carries its own accurate blocking finding rather than a stale one.
func unresolvedCoverageFindings(coverage []Coverage, retainUnmapped bool) []Finding {
	if retainUnmapped {
		return nil
	}
	var findings []Finding
	for _, cov := range coverage {
		if cov.UnresolvedBytes > 0 {
			findings = append(findings, Finding{
				Code:     FindingUnresolvedCoverage,
				Target:   cov.SourceID,
				Message:  fmt.Sprintf("source %q has %d unmapped byte(s) and retain_unmapped is false; creation is blocked until every selected byte is mapped or explicitly retained", cov.SourceID, cov.UnresolvedBytes),
				Blocking: true,
			})
		}
	}
	return findings
}

// dropFindingCode returns findings without any entry whose Code is code.
func dropFindingCode(findings []Finding, code string) []Finding {
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if f.Code != code {
			out = append(out, f)
		}
	}
	return out
}

// subtractFindings returns the multiset difference from-minus: each
// element of minus removes at most one structurally-equal entry from a
// working copy of from (Finding has only comparable fields, so exact `==`
// is a well-defined match). Order is preserved. Used to isolate exactly
// Compose's OWN additional findings from its returned set, which — for
// every external format — already contains prepareCandidateFields' own
// findings verbatim (Compose calls it internally too), so a naive
// concatenation would double-report every one of them.
func subtractFindings(from, minus []Finding) []Finding {
	remaining := append([]Finding(nil), minus...)
	out := make([]Finding, 0, len(from))
	for _, f := range from {
		removed := false
		for i, m := range remaining {
			if f == m {
				remaining = append(remaining[:i], remaining[i+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			out = append(out, f)
		}
	}
	return out
}

// assemblePreview combines Normalize's own findings (via
// prepareCandidateFields, which supersedes missing-statement under
// explicit pair deferral), the accepted candidate-ready fields, RECOMPUTED
// coverage over those fields' own spans, and Compose's additional findings
// (spec-import-contract.md's Task 3 handoff: "Preview combines normalized
// findings, accepted candidate-ready fields, RECOMPUTED coverage using
// buildCoverage, and Compose findings"). composeFindings is Compose's own
// returned findings slice (or nil, when the caller has not run Compose —
// e.g. a pure test of this assembly step alone).
func assemblePreview(request Request, plan Plan, composeFindings []Finding) (fields []Field, coverage []Coverage, findings []Finding) {
	fields, fieldFindings := prepareCandidateFields(request, plan)
	coverage = recomputeCoverage(request, plan, fields)

	findings = dropFindingCode(fieldFindings, FindingUnresolvedCoverage)
	findings = append(findings, unresolvedCoverageFindings(coverage, request.RetainUnmapped)...)
	findings = append(findings, subtractFindings(composeFindings, fieldFindings)...)
	return fields, coverage, findings
}
