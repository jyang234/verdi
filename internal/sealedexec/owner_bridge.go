// Standalone owner compatibility republishes the legacy top-level schema
// around the same public arm validation used on FD3. It reads no stores and
// makes no owner decisions. Exact canonical standalone request bytes remain
// the legacy request identity preimage.
package sealedexec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextowner"
)

// OwnerFrameCeiling is the exclusive 32 MiB controller-frame ceiling §2 fixes
// for both directions of the bridge. A document AT or above it is refused as
// one operational failure rather than read or written in fragments, so the
// bound is the same whether a payload arrives whole or arrives at all.
const OwnerFrameCeiling = 32 << 20

// The bridge's two closed refusal classes. Callers classify with errors.Is;
// the accompanying reason is drawn from the fixed literals in this file, so a
// bridge diagnostic can name a class but never a rejected value.
var (
	// ErrOwnerRequestRefused marks a private request payload the bridge will
	// not publish as a public owner call.
	ErrOwnerRequestRefused = errors.New("sealedexec: owner request refused")
	// ErrOwnerReplyRefused marks a public owner reply the bridge will not
	// encode as a private controller result.
	ErrOwnerReplyRefused = errors.New("sealedexec: owner reply refused")
)

func ownerRefusal(class error, reason string) error {
	return fmt.Errorf("%w: %s", class, reason)
}

// BridgedOperations is the closed set of operations this bridge covers: the
// controller registry minus the one operation the correction excludes.
//
// Correction §3.4 fixes the exclusion — ATC-owned resolve-claim-mcp remains
// operation 23 in the ATC controller and never enters the bridge — and the
// Global Constraint fixes the size: the bridge covers exactly Verdi operations
// 1–22.
//
// The set is derived from ControllerOperations by exclusion rather than written
// out a second time. That is what makes a future registry change loud: a Verdi
// operation added to the controller enters this set and fails the mapping
// producer, where a set derived by agreeing with the published registry would
// have dropped it without a sound. The returned slice is a fresh copy, so a
// caller cannot rewrite the closed set.
func BridgedOperations() []ControllerOperation {
	registry := ControllerOperations()
	bridged := make([]ControllerOperation, 0, len(registry))
	for _, operation := range registry {
		if isBridgedOperation(operation) {
			bridged = append(bridged, operation)
		}
	}
	return bridged
}

// isBridgedOperation is the one place the exclusion is spelled: an operation is
// bridged when the closed controller registry carries it and it is not the
// ATC-owned one. BridgedOperations enumerates exactly this predicate over the
// registry, and both bridge verbs consult it, so the
// set, the membership test, and the refusals cannot disagree.
func isBridgedOperation(operation ControllerOperation) bool {
	return containsControllerOperation(operation) && operation != ControllerOperationResolveClaimMCP
}

// OwnerBridgeOperation resolves one caller-supplied operand against the closed
// bridged set, returning the controller operation it names.
//
// The bridged set and the published registry must both accept it. They hold the
// same 22 names by construction, and requiring agreement is what turns a future
// divergence into a refusal here rather than into a call no owner can answer.
// The ATC-owned operation fails the first test and never reaches the second.
func OwnerBridgeOperation(name string) (ControllerOperation, bool) {
	operation := ControllerOperation(name)
	if !isBridgedOperation(operation) {
		return "", false
	}
	for _, published := range contextowner.Operations() {
		if published == contextowner.Operation(operation) {
			return operation, true
		}
	}
	return "", false
}

// DecodeOwnerCall validates a canonical standalone legacy request and publishes
// the public call. The exact original bytes, including one LF, must equal the
// public contract's schema-only legacy preimage; inputs are never repaired.
func DecodeOwnerCall(operation ControllerOperation, legacyRequest []byte) (contextowner.Call, error) {
	published, ok := OwnerBridgeOperation(string(operation))
	if !ok {
		return contextowner.Call{}, ownerRefusal(ErrOwnerRequestRefused, "operation is not published on the owner wire")
	}
	if len(legacyRequest) == 0 || len(legacyRequest) >= OwnerFrameCeiling {
		return contextowner.Call{}, ownerRefusal(ErrOwnerRequestRefused, "request payload is empty or reaches the controller frame ceiling")
	}
	if !bytes.HasSuffix(legacyRequest, []byte("\n")) || bytes.HasSuffix(legacyRequest, []byte("\n\n")) {
		return contextowner.Call{}, ownerRefusal(ErrOwnerRequestRefused, "request payload does not carry exactly one trailing LF")
	}
	op := contextowner.Operation(published)
	arm, err := republishSchema(trimFrame(legacyRequest), controllerRequestSchema(published), contextowner.RequestSchema(op))
	if err != nil {
		return contextowner.Call{}, ownerRefusal(ErrOwnerRequestRefused, "request payload does not declare its controller schema")
	}
	call, err := contextowner.NewCall(op, arm)
	if err != nil {
		return contextowner.Call{}, ownerRefusal(ErrOwnerRequestRefused, "published call is not a valid public owner call")
	}
	exact, err := contextowner.LegacyRequestPreimage(op, arm)
	if err != nil || !bytes.Equal(exact, legacyRequest) {
		return contextowner.Call{}, ownerRefusal(ErrOwnerRequestRefused, "request payload is not byte-canonical")
	}
	return call, nil
}

// EncodeOwnerReply validates through the same public arm and relation contract
// as FD3, then republishes only the result's top-level legacy schema.
func EncodeOwnerReply(operation ControllerOperation, reply contextowner.Reply) ([]byte, error) {
	published, ok := OwnerBridgeOperation(string(operation))
	if !ok {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "operation is not published on the owner wire")
	}
	op := contextowner.Operation(published)
	if reply.Call.Operation != op {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "nested call operation contradicts the invoked operation")
	}
	document, err := contextowner.EncodeReply(reply)
	if err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "reply is not a valid public owner reply")
	}
	if len(document) >= OwnerFrameCeiling {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "reply reaches the controller frame ceiling")
	}
	arm, err := contextowner.ResultArm(reply)
	if err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "reply carries no valid result arm")
	}
	if _, err = contextowner.NewReply(reply.Call, arm); err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, err.Error())
	}
	// NewReply recomputes and checks the exact schema-only request preimage.
	legacy, err := republishSchema(arm, contextowner.ResultSchema(op), controllerResultSchema(published))
	if err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "result arm does not declare its published schema")
	}
	result := frameNested(legacy)
	if len(result) >= OwnerFrameCeiling {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "private result reaches the controller frame ceiling")
	}
	return result, nil
}

// republishSchema applies §3.3's publication rule: replace only the top-level
// schema literal, retain every other canonical member, and re-canonicalize.
//
// The declared literal must be exactly the one expected for this direction, so
// an already-published document offered as a private payload — or the reverse
// — is refused rather than translated twice.
func republishSchema(document json.RawMessage, from, to string) (json.RawMessage, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSuffix(document, []byte("\n")), &members); err != nil {
		return nil, err
	}
	declared, ok := members["schema"]
	if !ok {
		return nil, errors.New("document declares no schema")
	}
	var literal string
	if err := json.Unmarshal(declared, &literal); err != nil {
		return nil, err
	}
	if literal != from {
		return nil, errors.New("document declares another schema")
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
	return trimFrame(republished), nil
}
