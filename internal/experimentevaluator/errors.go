package experimentevaluator

import (
	"errors"
	"fmt"
)

var (
	// ErrProtocol identifies an invalid evaluator operation or strict response.
	ErrProtocol = errors.New("experiment evaluator protocol failure")
	// ErrEvaluatorDigest identifies an executable identity mismatch or unsafe executable form.
	ErrEvaluatorDigest = errors.New("experiment evaluator executable integrity failure")
	// ErrCapabilitiesDigest identifies describe bytes that do not match the registered digest.
	ErrCapabilitiesDigest = errors.New("experiment evaluator capabilities digest mismatch")
	// ErrLaunch identifies a command-construction, executable-start, or wait failure.
	ErrLaunch = errors.New("experiment evaluator launch failure")
	// ErrEvaluatorExit identifies a child process that started but did not exit successfully.
	ErrEvaluatorExit = errors.New("experiment evaluator exited nonzero")
	// ErrHarnessDeadline identifies expiry of the deadline derived by Profile.Command.
	ErrHarnessDeadline = errors.New("experiment evaluator harness deadline exceeded")
	// ErrContextCancellation identifies cancellation or expiry of the caller's context.
	ErrContextCancellation = errors.New("experiment evaluator caller context canceled")
	// ErrStdoutLimit identifies evaluator stdout above the fixed transport ceiling.
	ErrStdoutLimit = errors.New("experiment evaluator stdout exceeds one MiB")
	// ErrStderrLimit identifies retained evaluator stderr above the fixed transport ceiling.
	ErrStderrLimit = errors.New("experiment evaluator stderr exceeds one MiB")
	// ErrObserver identifies an invalid fixed process observation.
	ErrObserver = errors.New("experiment evaluator process observation failure")
)

// OperationalError is a harness or evaluator failure that produces no valid
// discovery or attempt observation. Stderr is bounded diagnostic data and is
// never evidence.
type OperationalError struct {
	Op     string
	Stderr []byte
	Err    error
}

func (e *OperationalError) Error() string {
	if e == nil {
		return "experimentevaluator: operational failure"
	}
	return fmt.Sprintf("experimentevaluator: %s: %v", e.Op, e.Err)
}

// Unwrap exposes the closed failure classification and underlying cause.
func (e *OperationalError) Unwrap() error { return e.Err }

// IsOperational reports whether err prevented production of valid evaluator
// output rather than expressing candidate evidence.
func IsOperational(err error) bool {
	var target *OperationalError
	return errors.As(err, &target)
}

func operational(op string, kind error, stderr []byte, cause error) error {
	wrapped := kind
	if cause != nil {
		wrapped = fmt.Errorf("%w: %w", kind, cause)
	}
	return &OperationalError{
		Op:     op,
		Stderr: append([]byte(nil), stderr...),
		Err:    wrapped,
	}
}
