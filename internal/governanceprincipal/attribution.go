package governanceprincipal

import "fmt"

// Attribution is the permanently advisory record of who authored bytes
// (GLG DC-19). It is an exclusive union: either a canonical kernel
// principal identifier or an explicit unauthenticated marker — never a
// bare string presented as identity, never a second resolution algorithm,
// and never an input Authorize accepts. It carries no role, resolution
// state, or authorization result, and naming a principal grants nothing.
type Attribution struct {
	PrincipalID     PrincipalID `json:"principal_id,omitempty"`
	Unauthenticated bool        `json:"unauthenticated,omitempty"`
}

// Validate enforces the exclusive union: exactly one arm set, and a
// well-formed canonical principal ID when that arm is chosen.
func (a Attribution) Validate() error {
	hasPrincipal := a.PrincipalID != ""
	if hasPrincipal == a.Unauthenticated {
		return fmt.Errorf("governanceprincipal: attribution must set exactly one of principal_id and unauthenticated")
	}
	if hasPrincipal {
		if err := a.PrincipalID.Validate(); err != nil {
			return fmt.Errorf("governanceprincipal: attribution: %w", err)
		}
	}
	return nil
}

// NewPrincipalAttribution returns the advisory attribution naming a
// canonical kernel principal.
func NewPrincipalAttribution(id PrincipalID) (Attribution, error) {
	if err := id.Validate(); err != nil {
		return Attribution{}, fmt.Errorf("governanceprincipal: attribution: %w", err)
	}
	return Attribution{PrincipalID: id}, nil
}

// NewUnauthenticatedAttribution returns the explicit unauthenticated
// marker.
func NewUnauthenticatedAttribution() Attribution {
	return Attribution{Unauthenticated: true}
}

// AttributionFromResolution derives the only attribution a resolution
// permits: an authenticated resolution may name its principal; a violated
// or unproven resolution may only carry the unauthenticated marker. An
// internally inconsistent resolution record is an operational error.
func AttributionFromResolution(res PrincipalResolution) (Attribution, error) {
	if err := res.State.Validate(); err != nil {
		return Attribution{}, err
	}
	if res.State == ResolutionAuthenticated {
		return NewPrincipalAttribution(res.PrincipalID)
	}
	if res.PrincipalID != "" {
		return Attribution{}, fmt.Errorf("governanceprincipal: %s resolution must not carry a principal id", res.State)
	}
	return NewUnauthenticatedAttribution(), nil
}
