package unsealedprovenance

import "fmt"

// CutoffChangedCode is the stable, closed code a governed constitution change
// path reports when a proposal would remove or replace the accepted cutoff
// record (design §7, SI-245; SI-255: "constitution submit-preparation blocks
// any proposal whose cutoff is missing or differs").
const CutoffChangedCode = "unsealed-provenance-cutoff-changed"

// CutoffViolation decides the append-only cutoff rule for one proposed
// constitution change. accepted and proposed are the two sides' effective
// payloads; nil reads as no payload, and therefore no cutoff. It returns ""
// when accepted records no cutoff (adding the first cutoff, or any proposal
// over a constitution without one, is not this rule's concern) or when
// proposed records an identical one; otherwise it returns a witness naming
// the accepted cutoff and what the proposal does to it.
func CutoffViolation(accepted, proposed *Payload) string {
	if accepted == nil || accepted.Cutoff == nil {
		return ""
	}
	if proposed == nil || proposed.Cutoff == nil {
		return fmt.Sprintf("the accepted constitution records cutoff commit %s; the proposal records none", accepted.Cutoff.Commit)
	}
	if *proposed.Cutoff != *accepted.Cutoff {
		return fmt.Sprintf("the accepted constitution records cutoff commit %s; the proposal records %s", accepted.Cutoff.Commit, proposed.Cutoff.Commit)
	}
	return ""
}
