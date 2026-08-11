package draftmutation

import "fmt"

// Code is the closed CLI/service refusal and operational diagnostic set.
type Code string

const (
	CodeStaleBase        Code = "stale-base"
	CodeStateForbidden   Code = "state-forbidden"
	CodePolicyForbidden  Code = "policy-forbidden"
	CodeActorForbidden   Code = "actor-forbidden"
	CodeOperationInvalid Code = "operation-invalid"
	CodeResultInvalid    Code = "result-invalid"
	CodeInputInvalid     Code = "input-invalid"
	CodeIdentityInvalid  Code = "identity-invalid"
	CodeAuthorityInvalid Code = "authority-invalid"
	CodeRecoveryInvalid  Code = "recovery-invalid"
	CodeIOFailure        Code = "io-failure"
)

var verdictCodes = map[Code]bool{
	CodeStaleBase: true, CodeStateForbidden: true, CodePolicyForbidden: true,
	CodeActorForbidden: true, CodeOperationInvalid: true, CodeResultInvalid: true,
}

// Error is a closed service diagnostic. Once canonical Git identity is
// constructed, Identity is always that one value. IdentityAvailable is false
// only for an operational failure that prevented construction itself.
type Error struct {
	Code              Code
	Identity          Identity
	Detail            string
	Cause             error
	identityAvailable bool
}

func NewError(code Code, identity Identity, detail string) *Error {
	return &Error{Code: code, Identity: identity, Detail: detail, identityAvailable: identity.Validate() == nil}
}

func WrapError(code Code, identity Identity, detail string, cause error) *Error {
	return &Error{Code: code, Identity: identity, Detail: detail, Cause: cause, identityAvailable: identity.Validate() == nil}
}

// NewIdentityUnavailableError reports a failure that prevented the service
// from constructing canonical Git identity. Its zero Identity is explicitly
// unavailable, never a projection of caller-supplied expected operands.
func NewIdentityUnavailableError(detail string, cause error) *Error {
	return &Error{Code: CodeIdentityInvalid, Detail: detail, Cause: cause}
}

// IdentityAvailable distinguishes a service-constructed canonical identity
// from a pre-construction operational diagnostic.
func (e *Error) IdentityAvailable() bool { return e != nil && e.identityAvailable }

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Detail == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}

func (e *Error) Unwrap() error { return e.Cause }

func (e *Error) Verdict() bool { return verdictCodes[e.Code] }
