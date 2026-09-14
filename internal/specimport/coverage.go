package specimport

import "sort"

// mappedSpan is one (possibly overlapping) contribution to a source's
// byte-coverage partition: target's mapped content occupies [Start,End)
// of that source's selected bytes.
type mappedSpan struct {
	start, end int
	target     string
}

// buildCoverage partitions [0,totalBytes) into disjoint, ordered Intervals
// from the (possibly overlapping) mapped spans in mapped, then classifies
// every byte not covered by any mapped span as retained-only (when
// retainUnmapped) or unresolved (otherwise) — spec-import-contract.md,
// "RetainUnmapped is the user's explicit disposition of all remaining
// source...": "the coverage record partitions every selected source byte
// exactly once into the union of mapped spans and the retained or
// unresolved complement; overlapping field references can share a mapped
// interval, with all destinations listed rather than double-counted."
//
// mapped must already be in the caller's declared, deterministic order
// (statements, then objects in source/explicit order — never map-iteration
// order) so that when two mapped spans overlap, Targets lists them in a
// stable, reproducible sequence.
func buildCoverage(sourceID string, totalBytes int, mapped []mappedSpan, retainUnmapped bool) Coverage {
	boundarySet := map[int]bool{0: true, totalBytes: true}
	for _, m := range mapped {
		boundarySet[m.start] = true
		boundarySet[m.end] = true
	}
	boundaries := make([]int, 0, len(boundarySet))
	for b := range boundarySet {
		if b >= 0 && b <= totalBytes {
			boundaries = append(boundaries, b)
		}
	}
	sort.Ints(boundaries)

	cov := Coverage{SourceID: sourceID, TotalBytes: totalBytes}
	for i := 0; i+1 < len(boundaries); i++ {
		s, e := boundaries[i], boundaries[i+1]
		if s >= e {
			continue
		}
		var targets []string
		for _, m := range mapped {
			if m.start <= s && e <= m.end {
				targets = append(targets, m.target)
			}
		}
		var disposition string
		switch {
		case len(targets) > 0:
			disposition = DispositionMapped
			cov.MappedBytes += e - s
		case retainUnmapped:
			disposition = DispositionRetained
			cov.RetainedBytes += e - s
		default:
			disposition = DispositionUnresolved
			cov.UnresolvedBytes += e - s
		}
		cov.Intervals = append(cov.Intervals, Interval{Start: s, End: e, Disposition: disposition, Targets: targets})
	}
	return cov
}
