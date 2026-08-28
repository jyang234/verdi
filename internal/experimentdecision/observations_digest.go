package experimentdecision

import "github.com/jyang234/verdi/internal/experiment"

// ObservationRecordProjection is the digest projection, now owned by
// internal/experiment so the capsule binding kernel can prove retained
// observation bytes without an import cycle. This alias and the delegate
// below keep every existing caller on the single algorithm owner.
type ObservationRecordProjection = experiment.ObservationRecordProjection

// ObservationsDigest delegates to the relocated single owner.
func ObservationsDigest(def experiment.Definition, obs []experiment.Observation) (string, error) {
	return experiment.ObservationsDigest(def, obs)
}
