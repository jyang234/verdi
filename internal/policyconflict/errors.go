package policyconflict

import (
	"errors"
	"fmt"
)

// OperationalError identifies failures that prevent a verdict report from
// being produced. It intentionally carries no partial Result.
type OperationalError struct {
	Op  string
	Err error
}

func (e *OperationalError) Error() string {
	if e == nil {
		return "policyconflict: operational failure"
	}
	return fmt.Sprintf("policyconflict: %s: %v", e.Op, e.Err)
}

func (e *OperationalError) Unwrap() error { return e.Err }

// NotAdoptedError is the explicit no-constitution refusal. It is separate
// from OperationalError so lifecycle callers can preserve pre-adoption
// behavior without mistaking a malformed or incomplete store for absence.
type NotAdoptedError struct{ Err error }

func (e *NotAdoptedError) Error() string {
	if e == nil || e.Err == nil {
		return "policyconflict: constitution not adopted"
	}
	return fmt.Sprintf("policyconflict: constitution not adopted: %v", e.Err)
}

func (e *NotAdoptedError) Unwrap() error { return e.Err }

func operational(op string, err error) error {
	if err == nil {
		err = errors.New("unknown failure")
	}
	return &OperationalError{Op: op, Err: err}
}

// IsOperational reports whether err prevented the service from producing a
// completed three-valued verdict.
func IsOperational(err error) bool {
	var target *OperationalError
	return errors.As(err, &target)
}

// IsNotAdopted reports whether err is the explicit no-constitution refusal.
func IsNotAdopted(err error) bool {
	var target *NotAdoptedError
	return errors.As(err, &target)
}
