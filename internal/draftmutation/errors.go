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

// Error is a service-typed failure. Once request decode yields a valid spec,
// Identity is always the one service-constructed value.
type Error struct {
	Code     Code
	Identity Identity
	Detail   string
	Cause    error
}

func NewError(code Code, identity Identity, detail string) *Error {
	return &Error{Code: code, Identity: identity, Detail: detail}
}

func WrapError(code Code, identity Identity, detail string, cause error) *Error {
	return &Error{Code: code, Identity: identity, Detail: detail, Cause: cause}
}

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
