package experiment

import (
	"fmt"

	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// RatificationSchema is the only accepted ratification.yaml schema
// identifier.
const RatificationSchema = "verdi.experiment-ratification/v1"

// Ratification is one verdi.experiment-ratification/v1 record (AC-5,
// DC-16): a human's adapter-authenticated response to one immutable
// result.
type Ratification struct {
	Schema       string      `yaml:"schema"`
	ResultDigest string      `yaml:"result_digest"`
	Actor        string      `yaml:"actor"`
	Disposition  Disposition `yaml:"disposition"`
	Candidate    string      `yaml:"candidate,omitempty"`
	Reason       string      `yaml:"reason,omitempty"`
}

// DecodeRatification strict-decodes raw as a ratification.yaml document
// and fully validates it (decodeStrictYAML: the shared strict seam plus
// this package's trailing-document guard).
func DecodeRatification(raw []byte) (Ratification, error) {
	var r Ratification
	if err := decodeStrictYAML(raw, &r); err != nil {
		return Ratification{}, fmt.Errorf("experiment: decoding ratification: %w", err)
	}
	if err := r.Validate(); err != nil {
		return Ratification{}, err
	}
	return r, nil
}

// Validate checks the schema, result digest, disposition, and the
// candidate/reason conditionals (required only for select-other). The
// actor must resolve as a canonical kernel principal (owner adjudication
// OD-4: ratification actors are principal-resolution class) — a bare name
// or unauthenticated marker is never valid here.
func (r Ratification) Validate() error {
	if r.Schema != RatificationSchema {
		return fmt.Errorf("experiment: unknown ratification schema %q, want %q", r.Schema, RatificationSchema)
	}
	if err := ValidateDigest(r.ResultDigest); err != nil {
		return fmt.Errorf("experiment: ratification.result_digest: %w", err)
	}
	if err := governanceprincipal.PrincipalID(r.Actor).Validate(); err != nil {
		return fmt.Errorf("experiment: ratification.actor: %w", err)
	}
	if err := r.Disposition.Validate(); err != nil {
		return fmt.Errorf("experiment: ratification.disposition: %w", err)
	}

	selectOther := r.Disposition == DispositionSelectOther
	if selectOther {
		if r.Candidate == "" {
			return fmt.Errorf("experiment: ratification.candidate is required when disposition is %q", DispositionSelectOther)
		}
		if r.Reason == "" {
			return fmt.Errorf("experiment: ratification.reason is required when disposition is %q", DispositionSelectOther)
		}
	} else if r.Candidate != "" {
		return fmt.Errorf("experiment: ratification.candidate must be absent when disposition is %q", r.Disposition)
	}
	if r.Candidate != "" {
		if err := ValidateID(r.Candidate); err != nil {
			return fmt.Errorf("experiment: ratification.candidate: %w", err)
		}
	}
	return nil
}

// ValidateRatificationBinding checks the preconditions AC-5's disposition
// list IMPLIES but its grammar cannot express (invention ledger SI-45),
// for one ratification bound to the definition and result it responds to:
//
//   - select-recommended ("select the recommended candidate") requires the
//     bound result to actually recommend one, i.e. a proven-winner
//     verdict. Against a disclosed-unproven or violated-with-witness
//     result there is no recommendation to select, and accepting the
//     record would record a human decision that names nothing.
//   - select-other ("select a different candidate with an explicit
//     reason") requires the named candidate to be one the definition
//     REGISTERED — a candidate outside the locked contract was never
//     compared — and, when the result does name a winner, to be a
//     different one, because "other" is meaningless otherwise.
//
// Every other disposition (reject-all, misframed, request-new-revision)
// responds to the result as a whole and binds no candidate, so this check
// imposes nothing on it.
//
// It LAYERS over the record's own grammar validation the way
// ValidateComplete layers over ValidateObservations: r.Validate runs
// first, and stays free of any definition or result knowledge. def and res
// are the caller's already-validated context — DeriveState has decoded
// both and pinned res to def's digest before it gets here.
//
// Callers bind a ratification to its context in more than one place over
// time (state derivation now, adapter surfaces later); every one of them
// runs this check, because a disposition's meaning does not change with
// the surface that records it.
func ValidateRatificationBinding(def Definition, res Result, r Ratification) error {
	if err := r.Validate(); err != nil {
		return err
	}

	switch r.Disposition {
	case DispositionSelectRecommended:
		if res.Verdict != VerdictProvenWinner {
			return fmt.Errorf("experiment: ratification disposition %q requires a %q result, but the bound result's verdict is %q",
				DispositionSelectRecommended, VerdictProvenWinner, res.Verdict)
		}
	case DispositionSelectOther:
		registered := false
		for _, c := range def.Candidates {
			if c.ID == r.Candidate {
				registered = true
				break
			}
		}
		if !registered {
			return fmt.Errorf("experiment: ratification candidate %q does not name a registered candidate of definition %q",
				r.Candidate, def.ID)
		}
		if res.Winner != "" && r.Candidate == res.Winner {
			return fmt.Errorf("experiment: ratification disposition %q names candidate %q, which IS the result's recommended winner",
				DispositionSelectOther, r.Candidate)
		}
	}
	return nil
}
