package evidence

import "github.com/jyang234/verdi/internal/artifact"

// filterCandidates applies the authoritative-source filter — keep every
// source:ci record, plus source:local (advisory) records only when
// preview is set — 03 §Evidence records' "Provenance classes", the same
// rule Input.Preview and FeatureInput.Preview each document — and then
// rejects any surviving record whose evidence_for names an AC id absent
// from acSet: a "dangling binding" (03 §Declarations: "a misspelled ac-3
// must never surface as a silent no-signal"). Fold and FoldFeature share
// this exact two-step shape; only their error WORDING differs (Fold's
// full 03 §Declarations quote vs FoldFeature's abbreviated form, "AC" vs
// "feature AC"), so each caller supplies errFn to build its own message
// verbatim rather than this helper guessing at a shared phrasing.
func filterCandidates(records []artifact.Evidence, preview bool, acSet map[string]bool, errFn func(r artifact.Evidence, ac string) error) ([]artifact.Evidence, error) {
	candidates := make([]artifact.Evidence, 0, len(records))
	for _, r := range records {
		switch r.Provenance.Source {
		case artifact.SourceCI:
			candidates = append(candidates, r)
		case artifact.SourceLocal:
			if preview {
				candidates = append(candidates, r)
			}
		}
	}

	for _, r := range candidates {
		for _, ac := range r.EvidenceFor {
			if !acSet[ac] {
				return nil, errFn(r, ac)
			}
		}
	}
	return candidates, nil
}
