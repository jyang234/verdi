package contextowner

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
)

// NewCall constructs one local public owner call from operation and the
// exact public request arm bytes Verdi is about to send. requestArm carries
// no trailing LF, exactly like EncodeCall's own "request" member.
//
// The controller request digest is always computed locally
// (ControllerRequestDigest) -- a caller can never supply one, which is half
// of contract §2.2's "neither side accepts a caller-supplied digest without
// recomputation" (the other half is NewReply's check). NewCall then wraps
// requestArm in the call envelope with that digest and runs the result
// through DecodeCall, the sole public validator, so there is never a second
// registry or field list for what a call may contain.
func NewCall(operation Operation, requestArm []byte) (Call, error) {
	if !validOperation(operation) {
		return Call{}, fmt.Errorf("contextowner: local owner call: unknown owner operation %q", operation)
	}
	digest, err := ControllerRequestDigest(operation, requestArm)
	if err != nil {
		return Call{}, fmt.Errorf("contextowner: local owner call: %w", err)
	}
	document, err := canonjson.Marshal(callWire{
		Schema:                  CallSchemaID,
		Operation:               operation,
		ControllerRequestDigest: digest,
		Request:                 json.RawMessage(requestArm),
	})
	if err != nil {
		return Call{}, fmt.Errorf("contextowner: local owner call: encode: %w", err)
	}
	call, err := DecodeCall(bytes.NewReader(document))
	if err != nil {
		return Call{}, fmt.Errorf("contextowner: local owner call: %w", err)
	}
	return call, nil
}

// RequestArm returns call's exact canonical request-arm bytes, without a
// trailing LF -- the same bytes NewCall's requestArm parameter accepts and
// the exact "request" member EncodeCall would place in call's wire form.
func RequestArm(call Call) ([]byte, error) {
	arm, err := encodeRequestArm(call)
	if err != nil {
		return nil, fmt.Errorf("contextowner: request arm: %w", err)
	}
	return append([]byte(nil), arm...), nil
}

// ResultArm returns reply's exact canonical result-arm bytes, without a
// trailing LF -- the same bytes NewReply's resultArm parameter accepts and
// the exact "result" member EncodeReply would place in reply's wire form.
func ResultArm(reply Reply) ([]byte, error) {
	arm, err := encodeResultArm(reply)
	if err != nil {
		return nil, fmt.Errorf("contextowner: result arm: %w", err)
	}
	return append([]byte(nil), arm...), nil
}

// NewReply constructs one local public owner reply from call and the exact
// public result arm bytes an owner returned for it. resultArm carries no
// trailing LF, exactly like EncodeReply's own "result" member.
//
// NewReply first recomputes call's controller request digest from call's own
// request arm and requires it to equal call.ControllerRequestDigest: the
// other half of contract §2.2's rule that neither side accepts a
// caller-supplied digest without recomputation. resultArm is then proved
// byte-canonical and fully valid in isolation (validPublicResultArm, the
// same pattern LegacyRequestPreimage uses on the request side) before it is
// ever embedded in a larger document -- canonjson.Marshal would otherwise
// silently re-canonicalize a noncanonical resultArm while building that
// document, which would hide exactly the defect this function exists to
// catch. Finally {call, result, schema} is wrapped canonically and run
// through DecodeReply -- the sole public validator -- and then
// ValidateRelations, so a structurally valid but relation-violating reply is
// refused with an error that satisfies errors.Is(err, ErrRelationMismatch).
func NewReply(call Call, resultArm []byte) (Reply, error) {
	requestArm, err := RequestArm(call)
	if err != nil {
		return Reply{}, fmt.Errorf("contextowner: local owner reply: %w", err)
	}
	wantDigest, err := ControllerRequestDigest(call.Operation, requestArm)
	if err != nil {
		return Reply{}, fmt.Errorf("contextowner: local owner reply: %w", err)
	}
	if call.ControllerRequestDigest != wantDigest {
		return Reply{}, fmt.Errorf("contextowner: local owner reply: %s controller request digest does not match its own recomputed request", call.Operation)
	}

	canonicalResultArm, err := validPublicResultArm(call.Operation, resultArm)
	if err != nil {
		return Reply{}, fmt.Errorf("contextowner: local owner reply: %w", err)
	}

	callDocument, err := EncodeCall(call)
	if err != nil {
		return Reply{}, fmt.Errorf("contextowner: local owner reply: %w", err)
	}
	document, err := canonjson.Marshal(replyWire{
		Schema: ReplySchemaID,
		Call:   trimFrame(callDocument),
		Result: json.RawMessage(canonicalResultArm),
	})
	if err != nil {
		return Reply{}, fmt.Errorf("contextowner: local owner reply: encode: %w", err)
	}
	reply, err := DecodeReply(bytes.NewReader(document))
	if err != nil {
		return Reply{}, fmt.Errorf("contextowner: local owner reply: %w", err)
	}
	if err := ValidateRelations(reply); err != nil {
		return Reply{}, err
	}
	return reply, nil
}

// validPublicResultArm proves trimmed is byte-canonical AND a fully valid
// instance of operation's published result arm, returning its exact
// canonical bytes. It mirrors validPublicRequestArm on the result side,
// reusing the exact per-operation decode/encode pair EncodeReply itself
// calls (decodeResultArm, encodeResultArm).
func validPublicResultArm(operation Operation, trimmed []byte) ([]byte, error) {
	if _, err := DecodeResultArm(operation, trimmed); err != nil {
		return nil, err
	}
	return append([]byte(nil), trimmed...), nil
}

// DecodeResultArm strictly decodes an exact canonical public result arm without
// a request. resultArm has no trailing LF. The returned Reply carries only the
// operation in Call; it proves neither request identity nor request/result
// relations. Bind the result to its actual request with NewReply before use as a
// response to that request.
func DecodeResultArm(operation Operation, resultArm []byte) (Reply, error) {
	if !validOperation(operation) {
		return Reply{}, fmt.Errorf("contextowner: unknown owner operation %q", operation)
	}
	reply := Reply{Call: Call{Operation: operation}}
	if err := decodeResultArm(json.RawMessage(resultArm), &reply); err != nil {
		return Reply{}, err
	}
	canonical, err := encodeResultArm(reply)
	if err != nil {
		return Reply{}, err
	}
	if !bytes.Equal(resultArm, canonical) {
		return Reply{}, fmt.Errorf("contextowner: %s result arm is not byte-canonical", operation)
	}
	return reply, nil
}

// ValidateResultArm strictly validates an exact canonical public result arm
// without a request. Framing uses this structural seam before binding the
// response to its outstanding call through NewReply and ValidateRelations.
// resultArm has no trailing LF. This function does not prove any relation.
func ValidateResultArm(operation Operation, resultArm []byte) error {
	_, err := DecodeResultArm(operation, resultArm)
	return err
}
