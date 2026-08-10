package experimentdecision

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"

	"github.com/jyang234/verdi/internal/experiment"
)

// formatFloat renders v as a json.Number using strconv.FormatFloat(v, 'f',
// -1, 64) — the ONE fixed formatting every writer of a
// verdi.experiment-result/v1 document must use for CO-3's byte-identity
// guarantee to hold (see internal/experiment.DefinitionDigest's numeric
// -normalization doc comment: ResultDigest binds the decoded Result's
// exact json.Number literal, so two writers that agree on every value but
// format it differently still produce different bytes).
func formatFloat(v float64) json.Number {
	return json.Number(strconv.FormatFloat(v, 'f', -1, 64))
}

// percentile returns the nearest-rank percentile (no interpolation) of
// values at percentage p: k = ceil(p/100 * n), 1-based, indexing an
// ascending sort of values. values must be nonempty.
func percentile(values []float64, p float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	n := len(sorted)
	k := int(math.Ceil(p / 100 * float64(n)))
	if k < 1 {
		k = 1
	}
	if k > n {
		k = n
	}
	return sorted[k-1]
}

// maximum returns the largest value in values, which must be nonempty.
func maximum(values []float64) float64 {
	m := values[0]
	for _, v := range values[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// minimum returns the smallest value in values, which must be nonempty.
func minimum(values []float64) float64 {
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// mean returns the arithmetic mean of values, which must be nonempty.
func mean(values []float64) float64 {
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// aggregate reduces values by the registered closed aggregation vocabulary
// (spec/comparative-spike-experiments AC-2 step 3): p50/p95 use the
// nearest-rank percentile, maximum uses maximum, and mean/rate both use
// the arithmetic mean. Boolean measurements reach this function already
// projected onto the same scale (true 1, false 0 — SI-45), so all five
// aggregations are defined over them unchanged. values must be nonempty;
// agg must already have
// passed Aggregation.Validate() (Evaluate's callers only ever reach this
// through an already-validated locked Definition).
func aggregate(agg experiment.Aggregation, values []float64) float64 {
	switch agg {
	case experiment.AggregationP50:
		return percentile(values, 50)
	case experiment.AggregationP95:
		return percentile(values, 95)
	case experiment.AggregationMaximum:
		return maximum(values)
	case experiment.AggregationMean, experiment.AggregationRate:
		return mean(values)
	}
	// Unreachable for an Aggregation that already passed Validate(); a
	// defensive zero avoids a silent NaN propagating into a downstream
	// ordering comparison.
	return 0
}
