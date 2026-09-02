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
