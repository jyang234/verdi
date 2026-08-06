package experimentdecision

import (
	"sort"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/experiment"
)

// ObservationRecordProjection is the minimal per-record projection
// ObservationsDigest hashes. It carries exactly the evidentiary content of
// one observation record — the fields a decision actually depends on —
// omitting the schema identifier, experiment digest, and run identity that
// are already pinned elsewhere in a Result (DefinitionDigest, Run): this
// digest's whole job is to bind a Result to the EXACT evidence set that
// produced it, not to re-assert facts the Result states directly.
type ObservationRecordProjection struct {
	Candidate    string                   `json:"candidate"`
	Round        int                      `json:"round"`
	Guards       []experiment.GuardResult `json:"guards"`
	Measurements []experiment.Measurement `json:"measurements"`
	Disclosures  []string                 `json:"disclosures"`
}

// ObservationsDigest returns the internal/canonjson.Digest of obs's
// ObservationRecordProjection list, sorted by (registered candidate order
// in def.Candidates, round) rather than by the input slice's original
// order. This is the exact value Evaluate writes to a Result's
// ObservationsDigest field: because the projection is sorted by a fixed
// key rather than input order, two callers who supply the same evidence
// set in a different slice order still compute the same digest (CO-3).
//
// def must already be a valid, locked Definition — every candidate named
// by obs must appear in def.Candidates, or ObservationsDigest returns an
// error naming the unregistered candidate. Callers that have already run
// experiment.ValidateObservations (or ValidateComplete) never hit this
// path, since that check already rejects an unregistered candidate.
func ObservationsDigest(def experiment.Definition, obs []experiment.Observation) (string, error) {
	order := make(map[string]int, len(def.Candidates))
	for i, c := range def.Candidates {
		order[c.ID] = i
	}

	projected := make([]ObservationRecordProjection, len(obs))
	for i, o := range obs {
		if _, ok := order[o.Candidate]; !ok {
			return "", errf("computing observations digest: candidate %q is not registered", o.Candidate)
		}
		projected[i] = ObservationRecordProjection{
			Candidate:    o.Candidate,
			Round:        o.Round,
			Guards:       o.Guards,
			Measurements: o.Measurements,
			Disclosures:  o.Disclosures,
		}
	}

	sort.SliceStable(projected, func(i, j int) bool {
		oi, oj := order[projected[i].Candidate], order[projected[j].Candidate]
		if oi != oj {
			return oi < oj
		}
		return projected[i].Round < projected[j].Round
	})

	digest, err := canonjson.Digest(projected)
	if err != nil {
		return "", errfWrap("computing observations digest", err)
	}
	return digest, nil
}
