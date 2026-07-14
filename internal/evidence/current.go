package evidence

import (
	"strconv"

	"github.com/jyang234/verdi/internal/artifact"
)

// Current reduces records to the latest per (kind, producer) — falling
// back to (kind, witness) when Producer is empty, see artifact.Evidence's
// Producer field doc — ordered by (pipeline id, job id) monotonic (I-25):
// the latest record in each group wins, including two records sharing a
// commit (the flake case: a same-commit retry with a higher (pipeline,
// job) always wins, pass-after-fail included).
//
// Output order is deterministic (first-seen group order over records,
// which LoadRecords already sorts), never map iteration order.
func Current(records []artifact.Evidence) []artifact.Evidence {
	type slot struct {
		rec        artifact.Evidence
		firstIndex int
	}
	latest := make(map[string]slot, len(records))
	var groupOrder []string

	for i, r := range records {
		key := groupKey(r)
		cur, ok := latest[key]
		if !ok {
			latest[key] = slot{rec: r, firstIndex: i}
			groupOrder = append(groupOrder, key)
			continue
		}
		if laterProvenance(cur.rec.Provenance, r.Provenance) {
			latest[key] = slot{rec: r, firstIndex: cur.firstIndex}
		}
	}

	out := make([]artifact.Evidence, 0, len(groupOrder))
	for _, k := range groupOrder {
		out = append(out, latest[k].rec)
	}
	return out
}

// groupKey is the "(kind, producer)" grouping key the fold reduces
// candidate records by. When Producer is absent (see artifact.Evidence's
// doc: not every record's producer identity is recoverable), witness text
// is the best-effort substitute the task's "join through the
// bindings/witness" reading allows — records with genuinely different
// witness text are never silently collapsed together, at the cost of not
// deduping a retry whose witness text happens to differ from its
// predecessor's.
func groupKey(r artifact.Evidence) string {
	if r.Producer != "" {
		return string(r.Kind) + "\x00p:" + r.Producer
	}
	return string(r.Kind) + "\x00w:" + r.Witness
}

// laterProvenance reports whether b sorts strictly after a under
// (pipeline id, job id) monotonic ordering (I-25): pipeline id compared
// first, job id breaking ties within the same pipeline, both compared
// numerically when they parse as integers (the common case: CI-assigned
// pipeline/job ids) and lexicographically otherwise. An absent job (empty
// string) sorts before any present job within the same pipeline — I-25's
// "fold treats absent as zero".
func laterProvenance(a, b artifact.EvidenceProvenance) bool {
	if c := compareOrdinal(a.Pipeline, b.Pipeline); c != 0 {
		return c < 0
	}
	return compareOrdinal(a.Job, b.Job) < 0
}

// compareOrdinal compares two ordering tokens (a pipeline or job id),
// returning -1/0/1. Both are parsed as integers and compared numerically
// when they can be (the common CI case); otherwise the comparison falls
// back to plain string ordering, which is deterministic but not
// necessarily numerically monotonic for non-numeric ids — a disclosed
// v0 limitation, since every CI system verdi targets assigns numeric
// pipeline/job ids.
func compareOrdinal(a, b string) int {
	if a == b {
		return 0
	}
	ai, aerr := strconv.Atoi(a)
	bi, berr := strconv.Atoi(b)
	if aerr == nil && berr == nil {
		switch {
		case ai < bi:
			return -1
		case ai > bi:
			return 1
		default:
			return 0
		}
	}
	if a < b {
		return -1
	}
	return 1
}
