package constitutionapp

import (
	"github.com/jyang234/verdi/internal/unsealedprovenance"
)

// unsealedProvenanceCutoffReason applies the append-only cutoff rule of the
// unsealed-provenance exemption design (§7; SI-245, SI-255) to one
// submission's exact accepted and proposed snapshots. It returns "" when the
// rule does not block, or a blocking reason that begins with the closed code
// unsealedprovenance.CutoffChangedCode when the accepted constitution records
// a cutoff and the proposal's is missing or differs. This package never
// reads the payload's fields itself; unsealedprovenance decides the rule.
//
// An exact tree whose bytes were unavailable proves nothing about its
// cutoff, so the rule stays silent rather than claim a change it could not
// read: impact coverage already discloses that side as
// accepted-/proposed-tree-unavailable and blocks readiness, exactly as it did
// before this rule existed.
func unsealedProvenanceCutoffReason(accepted, proposed Snapshot) (string, *Error) {
	if accepted.unavailable || proposed.unavailable {
		return "", nil
	}
	acceptedPayload, typed := snapshotUnsealedProvenance(accepted)
	if typed != nil {
		return "", typed
	}
	proposedPayload, typed := snapshotUnsealedProvenance(proposed)
	if typed != nil {
		return "", typed
	}
	witness := unsealedprovenance.CutoffViolation(acceptedPayload, proposedPayload)
	if witness == "" {
		return "", nil
	}
	return unsealedprovenance.CutoffChangedCode + ": " + witness, nil
}

// snapshotUnsealedProvenance returns one snapshot's effective
// unsealed-provenance payload. A store that adopted no constitution carries
// no payload (nil). An adopted snapshot must carry its effective policy, and
// a payload that cannot be read is corrupted authority, never "no cutoff".
func snapshotUnsealedProvenance(snapshot Snapshot) (*unsealedprovenance.Payload, *Error) {
	if !snapshot.Adopted {
		return nil, nil
	}
	if snapshot.Effective == nil {
		return nil, verdict("corrupted-policy", "the adopted constitution at "+snapshot.Ref+" carries no effective policy")
	}
	payload, _, err := unsealedprovenance.Effective(snapshot.Effective)
	if err != nil {
		return nil, verdictWithCause("corrupted-policy", "reading the unsealed-provenance payload at "+snapshot.Ref, err)
	}
	return payload, nil
}
