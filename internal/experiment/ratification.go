package experiment

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
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
// and fully validates it.
func DecodeRatification(raw []byte) (Ratification, error) {
	var r Ratification
	if err := artifact.DecodeStrict(raw, &r); err != nil {
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
