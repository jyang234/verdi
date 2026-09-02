package constitutionapp

import (
	"errors"
	"fmt"
)

// Shared request-shape validation errors reused across every operation's
// own validate() method, so the same malformed-request diagnostic reads
// identically regardless of which operation received it.
var (
	errRootRequired   = errors.New("constitutionapp: root is required")
	errBranchRequired = errors.New("constitutionapp: branch is required")
	errKindInvalid    = errors.New("constitutionapp: kind must be one of policy, policy-overlay, policy-exemption")
	errNameRequired   = errors.New("constitutionapp: name is required")
	errContentEmpty   = errors.New("constitutionapp: content is required")
)

// errTargetSpecRequired names which zero-indexed target in a request's
// Targets slice is missing its required Spec field.
func errTargetSpecRequired(index int) error {
	return fmt.Errorf("constitutionapp: targets[%d].spec is required", index)
}

// validateTargets is the ONE declared-target validation both ImpactReview
// and SubmitPreparation use, so the two requests can carry their own
// distinct envelope versions without either converting into the other's type
// merely to reuse a check.
func validateTargets(targets []ImpactTarget) error {
	for i, t := range targets {
		if t.Spec == "" {
			return errTargetSpecRequired(i)
		}
	}
	return nil
}

// requireRequestSchema enforces one request envelope's EXACT version. There
// is no defaulting and no prefix matching: an absent or unrecognized version
// fails closed (root CLAUDE.md: "unknown enum values fail closed"). The
// diagnostic names both the expected and the received value, so a caller
// pinned to an older envelope learns which version it sent rather than only
// that something was wrong.
func requireRequestSchema(got, want string) error {
	if got == want {
		return nil
	}
	if got == "" {
		return fmt.Errorf("constitutionapp: request is missing its required schema field (want %q)", want)
	}
	return fmt.Errorf("constitutionapp: request schema %q is not %q", got, want)
}
