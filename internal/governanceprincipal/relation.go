package governanceprincipal

import "fmt"

// EvaluateRelation compares two sealed principal resolutions under one
// closed distinctness relation. It deliberately evaluates no other profile
// authorization rule.
func EvaluateRelation(left, right PrincipalResolution, relation DistinctnessRelation) (AuthorizationState, error) {
	if err := relation.Validate(); err != nil {
		return "", err
	}
	if err := validateRelationOperand("left", left); err != nil {
		return "", err
	}
	if err := validateRelationOperand("right", right); err != nil {
		return "", err
	}
	if left.State == ResolutionViolated || right.State == ResolutionViolated {
		return AuthorizationViolated, nil
	}
	if left.State == ResolutionUnproven || right.State == ResolutionUnproven {
		return AuthorizationUnproven, nil
	}

	same := left.PrincipalID == right.PrincipalID
	if (relation == RelationSamePrincipal && same) || (relation == RelationDifferentPrincipal && !same) {
		return AuthorizationAuthorized, nil
	}
	return AuthorizationViolated, nil
}

func validateRelationOperand(name string, resolution PrincipalResolution) error {
	if err := resolution.State.Validate(); err != nil {
		return fmt.Errorf("governanceprincipal: relation %s resolution: %w", name, err)
	}
	if err := resolution.Claim.Validate(); err != nil {
		return fmt.Errorf("governanceprincipal: relation %s resolution: %w", name, err)
	}
	derived, err := CanonicalPrincipalID(resolution.Claim.TrustSource, resolution.Claim.Subject)
	if err != nil {
		return fmt.Errorf("governanceprincipal: relation %s resolution: %w", name, err)
	}
	if resolution.State == ResolutionAuthenticated {
		if resolution.PrincipalID != derived {
			return fmt.Errorf("governanceprincipal: relation %s authenticated resolution principal id %q does not match claim-derived id %q", name, resolution.PrincipalID, derived)
		}
	} else if resolution.PrincipalID != "" {
		return fmt.Errorf("governanceprincipal: relation %s %s resolution must not carry a principal id", name, resolution.State)
	}
	if err := resolution.checkSeal(); err != nil {
		return fmt.Errorf("governanceprincipal: relation %s resolution: %w", name, err)
	}
	return nil
}
