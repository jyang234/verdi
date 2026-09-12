package contextowner

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// The legacy (private controller) request-arm schema derivation, contract
// §2.2. It mirrors internal/sealedexec's controllerRequestSchema literal for
// literal: this package cannot import that one without cycling back through
// the bridge it owns (sealedexec imports contextowner), so the derivation is
// restated here from the same fixed rule -- exactly as this file's sibling
// schema.go already restates the public-side derivation instead of aliasing
// a private type. public_contract_fixtures_test.go independently pins both
// derivations against the frozen fixtures.
const (
	legacyRequestSchemaPrefix     = "verdi.context-controller/"
	legacyRequestSchemaTag        = "-request/v1"
	legacyInstallRequestSchemaTag = "-request/v2"
)

// LegacyRequestSchema derives the private controller request-arm schema
// literal that operation's legacy identity rule L(P) (contract §2.2)
// replaces the published schema with. It returns "" for any operation
// outside the closed 22-operation registry, including resolve-claim-mcp,
// which this package deliberately never handles.
func LegacyRequestSchema(operation Operation) string {
	if !validOperation(operation) {
		return ""
	}
	if operation == OperationInstallExpansion {
		return legacyRequestSchemaPrefix + string(operation) + legacyInstallRequestSchemaTag
	}
	return legacyRequestSchemaPrefix + string(operation) + legacyRequestSchemaTag
}

// LegacyRequestPreimage computes contract §2.2's L(P): requestArm must be
// the exact canonical bytes of operation's published request arm, exactly as
// it appears as a call's "request" member -- without a trailing LF. L(P) is
// that same strictly valid document with only its top-level schema replaced
// by LegacyRequestSchema(operation), canonically re-encoded as a standalone
// document with exactly one trailing LF.
//
// requestArm is proved byte-canonical and a fully valid instance of the
// published request arm by the same decode/encode pair EncodeCall itself
// uses (decodeRequestArm then encodeRequestArm, see validPublicRequestArm):
// a noncanonical or otherwise invalid arm is refused here rather than
// silently repaired into L(P).
func LegacyRequestPreimage(operation Operation, requestArm []byte) ([]byte, error) {
	if !validOperation(operation) {
		return nil, fmt.Errorf("contextowner: legacy request preimage: unknown owner operation %q", operation)
	}
	canonicalArm, err := validPublicRequestArm(operation, requestArm)
	if err != nil {
		return nil, fmt.Errorf("contextowner: legacy request preimage: %w", err)
	}
	preimage, err := republishArmSchema(canonicalArm, RequestSchema(operation), LegacyRequestSchema(operation))
	if err != nil {
		return nil, fmt.Errorf("contextowner: legacy request preimage: %w", err)
	}
	return append(preimage, '\n'), nil
}

// ControllerRequestDigest computes contract §2.2's controller request
// digest: "sha256:" followed by the lowercase hex SHA-256 of
// LegacyRequestPreimage's exact bytes. It is a cross-process identity
// binding recomputed locally by both the sender (NewCall) and the receiver
// (NewReply), never accepted as a caller-supplied value.
func ControllerRequestDigest(operation Operation, requestArm []byte) (string, error) {
	preimage, err := LegacyRequestPreimage(operation, requestArm)
	if err != nil {
		return "", err
	}
	return digestBytes(preimage), nil
}

// validPublicRequestArm proves trimmed is byte-canonical AND a fully valid
// instance of operation's published request arm, returning its exact
// canonical bytes (identical to trimmed when it already was one). It reuses
// the exact per-operation decode/encode pair EncodeCall itself calls to
// derive and validate a request arm (decodeRequestArm, encodeRequestArm), so
// there is no second field list or validation path for the request arm
// alone -- the same pattern codec.go's canonicalNestedObject uses for a
// nested document, applied here to the top-level request arm.
func validPublicRequestArm(operation Operation, trimmed []byte) ([]byte, error) {
	call := Call{Operation: operation}
	if err := decodeRequestArm(json.RawMessage(trimmed), &call); err != nil {
		return nil, err
	}
	canonical, err := encodeRequestArm(call)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(trimmed, canonical) {
		return nil, fmt.Errorf("contextowner: %s request arm is not byte-canonical", operation)
	}
	return canonical, nil
}

// republishArmSchema returns document (an exact canonical JSON object with
// no trailing LF) with only its top-level "schema" member replaced by to.
// The document must already declare from, or it is refused rather than
// silently relabeled -- the mechanical publication rule (schema.go's package
// doc, correction §3.3) applied here in the legacy direction, restated
// rather than imported from sealedexec's own republishSchema for the same
// one-way-dependency reason LegacyRequestSchema restates its derivation.
func republishArmSchema(document []byte, from, to string) ([]byte, error) {
	var members map[string]json.RawMessage
	if err := artifact.DecodeExactJSON(document, &members); err != nil {
		return nil, fmt.Errorf("republish schema: %w", err)
	}
	declaredRaw, ok := members["schema"]
	if !ok {
		return nil, fmt.Errorf("republish schema: document declares no schema")
	}
	var declared string
	if err := json.Unmarshal(declaredRaw, &declared); err != nil {
		return nil, fmt.Errorf("republish schema: %w", err)
	}
	if declared != from {
		return nil, fmt.Errorf("republish schema: document does not declare the expected schema")
	}
	replacement, err := json.Marshal(to)
	if err != nil {
		return nil, err
	}
	members["schema"] = replacement
	republished, err := canonjson.Marshal(members)
	if err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(republished, []byte("\n")), nil
}
