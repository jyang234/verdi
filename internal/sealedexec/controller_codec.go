package sealedexec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextowner"
)

const (
	redactedSegmentSchemaID = contextowner.RedactedSegmentSchemaID
	storedSegmentSchemaID   = contextowner.StoredSegmentSchemaID
	segmentReferencePrefix  = contextowner.SegmentReferencePrefix
)

type controllerCallWire struct {
	Schema       string              `json:"schema"`
	CallSequence uint64              `json:"call_sequence"`
	Operation    ControllerOperation `json:"operation"`
	Payload      json.RawMessage     `json:"payload"`
}

type controllerResultWire struct {
	Schema       string              `json:"schema"`
	CallSequence uint64              `json:"call_sequence"`
	Operation    ControllerOperation `json:"operation"`
	Payload      json.RawMessage     `json:"payload"`
}

type controllerResultPayloadWire struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

type controllerErrorWire struct {
	Schema    string               `json:"schema"`
	Class     ControllerErrorClass `json:"class"`
	Code      ControllerErrorCode  `json:"code"`
	Witnesses []string             `json:"witnesses"`
}

type claimMCPQueryWire struct {
	RequestDigest string `json:"request_digest"`
	Schema        string `json:"schema"`
}

type claimMCPRegistrationWire struct {
	Name          string   `json:"name"`
	RequestDigest string   `json:"request_digest"`
	Schema        string   `json:"schema"`
	Tools         []string `json:"tools"`
	Type          string   `json:"type"`
	URL           string   `json:"url"`
}

const (
	claimMCPQuerySchemaID        = "verdi.claim-mcp-query/v1"
	claimMCPRegistrationSchemaID = "verdi.claim-mcp-registration/v1"
)

func claimMCPQueryToWire(query ClaimMCPQuery) (claimMCPQueryWire, error) {
	if !scopedMCPDigestRE.MatchString(query.RequestDigest) {
		return claimMCPQueryWire{}, errors.New("sealedexec: claim MCP query requires a canonical request digest")
	}
	return claimMCPQueryWire{RequestDigest: query.RequestDigest, Schema: claimMCPQuerySchemaID}, nil
}

func claimMCPQueryFromWire(wire claimMCPQueryWire) (ClaimMCPQuery, error) {
	if wire.Schema != claimMCPQuerySchemaID || !scopedMCPDigestRE.MatchString(wire.RequestDigest) {
		return ClaimMCPQuery{}, errors.New("sealedexec: claim MCP query is not a canonical query row")
	}
	return ClaimMCPQuery{RequestDigest: wire.RequestDigest}, nil
}

func validateClaimMCPRegistration(registration ClaimMCPRegistration) error {
	if registration.Name != RequiredClaimMCPName {
		return fmt.Errorf("sealedexec: claim MCP registration name %q, want %q", registration.Name, RequiredClaimMCPName)
	}
	if registration.Type != RequiredMCPType {
		return fmt.Errorf("sealedexec: claim MCP registration type %q, want %q", registration.Type, RequiredMCPType)
	}
	if err := ValidateRequiredMCPURL(registration.URL); err != nil {
		return fmt.Errorf("sealedexec: claim MCP registration: %w", err)
	}
	want := requiredClaimTools()
	if len(registration.Tools) != len(want) {
		return fmt.Errorf("sealedexec: claim MCP registration declares %d tools, want %d", len(registration.Tools), len(want))
	}
	for i, tool := range want {
		if registration.Tools[i] != tool {
			return fmt.Errorf("sealedexec: claim MCP registration tool %d = %q, want %q", i, registration.Tools[i], tool)
		}
	}
	if !scopedMCPDigestRE.MatchString(registration.RequestDigest) {
		return errors.New("sealedexec: claim MCP registration requires a canonical request digest")
	}
	return nil
}

func claimMCPRegistrationToWire(registration ClaimMCPRegistration) (claimMCPRegistrationWire, error) {
	if err := validateClaimMCPRegistration(registration); err != nil {
		return claimMCPRegistrationWire{}, err
	}
	return claimMCPRegistrationWire{
		Name: registration.Name, RequestDigest: registration.RequestDigest,
		Schema: claimMCPRegistrationSchemaID, Tools: append([]string(nil), registration.Tools...),
		Type: registration.Type, URL: registration.URL,
	}, nil
}

func claimMCPRegistrationFromWire(wire claimMCPRegistrationWire) (ClaimMCPRegistration, error) {
	if wire.Schema != claimMCPRegistrationSchemaID {
		return ClaimMCPRegistration{}, errors.New("sealedexec: claim MCP registration schema is not canonical")
	}
	registration := ClaimMCPRegistration{
		Name: wire.Name, Type: wire.Type, URL: wire.URL,
		Tools: append([]string(nil), wire.Tools...), RequestDigest: wire.RequestDigest,
	}
	if err := validateClaimMCPRegistration(registration); err != nil {
		return ClaimMCPRegistration{}, err
	}
	return registration, nil
}

func EncodeControllerCall(call ControllerCall) ([]byte, error) {
	if call.Schema != ControllerCallSchemaID {
		return nil, fmt.Errorf("sealedexec: controller call schema must be %q", ControllerCallSchemaID)
	}
	if call.CallSequence == 0 {
		return nil, fmt.Errorf("sealedexec: controller call_sequence must be positive")
	}
	if !validControllerOperation(call.Operation) {
		return nil, fmt.Errorf("sealedexec: unknown controller operation %q", call.Operation)
	}
	payload, err := encodePublicControllerCallPayload(call)
	if err != nil {
		return nil, err
	}
	return boundedControllerFrame(canonjson.Marshal(controllerCallWire{Schema: call.Schema, CallSequence: call.CallSequence, Operation: call.Operation, Payload: payload}))
}

func DecodeControllerCall(reader io.Reader) (ControllerCall, error) {
	var wire controllerCallWire
	raw, err := decodeControllerFrame(reader, &wire)
	if err != nil {
		return ControllerCall{}, fmt.Errorf("sealedexec: decode controller call: %w", err)
	}
	if err := requireFields(raw, "schema", "call_sequence", "operation", "payload"); err != nil {
		return ControllerCall{}, err
	}
	if wire.Schema != ControllerCallSchemaID || wire.CallSequence == 0 || !validControllerOperation(wire.Operation) || rawMissing(wire.Payload) {
		return ControllerCall{}, fmt.Errorf("sealedexec: invalid controller call envelope")
	}
	call := ControllerCall{Schema: wire.Schema, CallSequence: wire.CallSequence, Operation: wire.Operation}
	if err := decodePublicControllerCallPayload(wire.Payload, &call); err != nil {
		return ControllerCall{}, err
	}
	canonical, err := EncodeControllerCall(call)
	if err != nil {
		return ControllerCall{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return ControllerCall{}, fmt.Errorf("sealedexec: controller call is not byte-canonical")
	}
	return call, nil
}

func EncodeControllerResult(result ControllerResult) ([]byte, error) {
	if result.Schema != ControllerResultSchemaID {
		return nil, fmt.Errorf("sealedexec: controller result schema must be %q", ControllerResultSchemaID)
	}
	if result.CallSequence == 0 || !validControllerOperation(result.Operation) {
		return nil, fmt.Errorf("sealedexec: invalid controller result identity")
	}
	var payload controllerResultPayloadWire
	var err error
	if result.Error != nil {
		if !controllerResultArmsZero(result) {
			return nil, fmt.Errorf("sealedexec: controller error reply also carries a result arm")
		}
		payload.Error, err = encodeControllerError(*result.Error)
	} else {
		payload.Result, err = encodePublicControllerSuccessPayload(result)
	}
	if err != nil {
		return nil, err
	}
	payloadBytes, err := marshalControllerPayload(payload)
	if err != nil {
		return nil, err
	}
	return boundedControllerFrame(canonjson.Marshal(controllerResultWire{Schema: result.Schema, CallSequence: result.CallSequence, Operation: result.Operation, Payload: payloadBytes}))
}

func DecodeControllerResult(reader io.Reader) (ControllerResult, error) {
	var wire controllerResultWire
	raw, err := decodeControllerFrame(reader, &wire)
	if err != nil {
		return ControllerResult{}, fmt.Errorf("sealedexec: decode controller result: %w", err)
	}
	if err := requireFields(raw, "schema", "call_sequence", "operation", "payload"); err != nil {
		return ControllerResult{}, err
	}
	if wire.Schema != ControllerResultSchemaID || wire.CallSequence == 0 || !validControllerOperation(wire.Operation) || rawMissing(wire.Payload) {
		return ControllerResult{}, fmt.Errorf("sealedexec: invalid controller result envelope")
	}
	var outcome controllerResultPayloadWire
	if err := unmarshalControllerPayload(wire.Payload, &outcome); err != nil {
		return ControllerResult{}, err
	}
	hasResult, hasError := !rawMissing(outcome.Result), !rawMissing(outcome.Error)
	if hasResult == hasError {
		return ControllerResult{}, fmt.Errorf("sealedexec: controller reply must contain exactly one result or error arm")
	}
	result := ControllerResult{Schema: wire.Schema, CallSequence: wire.CallSequence, Operation: wire.Operation}
	if hasError {
		controllerError, err := decodeControllerError(outcome.Error)
		if err != nil {
			return ControllerResult{}, err
		}
		result.Error = &controllerError
	} else if err := decodePublicControllerSuccessPayload(outcome.Result, &result); err != nil {
		return ControllerResult{}, err
	}
	canonical, err := EncodeControllerResult(result)
	if err != nil {
		return ControllerResult{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return ControllerResult{}, fmt.Errorf("sealedexec: controller result is not byte-canonical")
	}
	return result, nil
}

func encodeControllerError(controllerError ControllerError) (json.RawMessage, error) {
	if controllerError.Schema != ControllerErrorSchemaID || controllerError.Class != ControllerErrorClassOperational {
		return nil, fmt.Errorf("sealedexec: invalid controller operational error schema/class")
	}
	if !validControllerErrorCode(controllerError.Code) {
		return nil, fmt.Errorf("sealedexec: unknown controller error code %q", controllerError.Code)
	}
	if len(controllerError.Witnesses) == 0 {
		return nil, fmt.Errorf("sealedexec: controller error witnesses must be non-null and nonempty")
	}
	if err := validateSortedTexts("controller error witnesses", controllerError.Witnesses); err != nil {
		return nil, err
	}
	return marshalControllerPayload(controllerErrorWire(controllerError))
}

func decodeControllerError(raw json.RawMessage) (ControllerError, error) {
	var wire controllerErrorWire
	if err := unmarshalControllerPayload(raw, &wire); err != nil {
		return ControllerError{}, err
	}
	controllerError := ControllerError(wire)
	if _, err := encodeControllerError(controllerError); err != nil {
		return ControllerError{}, err
	}
	return controllerError, nil
}

func marshalControllerPayload(value any) (json.RawMessage, error) {
	raw, err := canonjson.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("sealedexec: encode controller payload: %w", err)
	}
	return trimFrame(raw), nil
}

func unmarshalControllerPayload(raw json.RawMessage, target any) error {
	if rawMissing(raw) {
		return fmt.Errorf("sealedexec: controller payload is absent or null")
	}
	if _, err := decodeStrict(bytes.NewReader(raw), target); err != nil {
		return fmt.Errorf("sealedexec: decode controller payload: %w", err)
	}
	return nil
}

func frameNested(raw json.RawMessage) []byte {
	framed, err := canonjson.Marshal(raw)
	if err != nil {
		return append(append([]byte(nil), raw...), '\n')
	}
	return framed
}

func trimFrame(raw []byte) json.RawMessage {
	return append(json.RawMessage(nil), bytes.TrimSuffix(raw, []byte("\n"))...)
}

func validControllerOperation(operation ControllerOperation) bool {
	return containsControllerOperation(operation)
}

func containsControllerOperation(operation ControllerOperation) bool {
	for _, candidate := range controllerOperations {
		if candidate == operation {
			return true
		}
	}
	return false
}

func validControllerErrorCode(code ControllerErrorCode) bool {
	switch code {
	case ControllerErrorUnavailable, ControllerErrorMalformedRequest, ControllerErrorIdentityMismatch, ControllerErrorSequenceMismatch, ControllerErrorOperationMismatch, ControllerErrorPersistenceFailed, ControllerErrorConflictingReplay, ControllerErrorInternal:
		return true
	default:
		return false
	}
}

func operationSchemaError(operation ControllerOperation) error {
	return fmt.Errorf("sealedexec: controller %s payload schema mismatch", operation)
}

func requireOnlyCallArm(call ControllerCall, selected any) error {
	want := ControllerCall{Schema: call.Schema, CallSequence: call.CallSequence, Operation: call.Operation}
	switch call.Operation {
	case ControllerOperationVerifyAuthority:
		want.VerifyAuthority = selected.(ControllerVerifyAuthorityRequest)
	case ControllerOperationResolveProfile:
		want.ResolveProfile = selected.(ControllerResolveProfileRequest)
	case ControllerOperationVerifyConflict:
		want.VerifyConflict = selected.(ControllerVerifyConflictRequest)
	case ControllerOperationResolveRecorder:
		want.ResolveRecorder = selected.(ControllerResolveRecorderRequest)
	case ControllerOperationRecorderCheckpoint:
		want.RecorderCheckpoint = selected.(ControllerRecorderCheckpointRequest)
	case ControllerOperationRecorderAppend:
		want.RecorderAppend = selected.(ControllerRecorderAppendRequest)
	case ControllerOperationStoreRedactedSegment:
		want.StoreRedactedSegment = selected.(ControllerStoreRedactedSegmentRequest)
	case ControllerOperationResolveRedactedSegment:
		want.ResolveRedactedSegment = selected.(ControllerResolveRedactedSegmentRequest)
	case ControllerOperationVerifyOpaqueBoundary:
		want.VerifyOpaqueBoundary = selected.(ControllerVerifyOpaqueBoundaryRequest)
	case ControllerOperationVerifyProviderSession:
		want.VerifyProviderSession = selected.(ControllerVerifyProviderSessionRequest)
	case ControllerOperationVerifyExpansion:
		want.VerifyExpansion = selected.(ControllerVerifyExpansionRequest)
	case ControllerOperationStoreAdapterSession:
		want.StoreAdapterSession = selected.(ControllerStoreAdapterSessionRequest)
	case ControllerOperationNextStamp:
		want.NextStamp = selected.(ControllerNextStampRequest)
	case ControllerOperationResolveContext:
		want.ResolveContext = selected.(ControllerResolveContextRequest)
	case ControllerOperationVerifyEpoch:
		want.VerifyEpoch = selected.(ControllerVerifyEpochRequest)
	case ControllerOperationInstallExpansion:
		want.InstallExpansion = selected.(ControllerInstallExpansionRequest)
	case ControllerOperationResolveReceiptInputs:
		want.ResolveReceiptInputs = selected.(ControllerResolveReceiptInputsRequest)
	case ControllerOperationAppendReceipt:
		want.AppendReceipt = selected.(ControllerAppendReceiptRequest)
	case ControllerOperationResolveReceiptVerificationAuthority:
		want.ResolveReceiptVerificationAuthority = selected.(ControllerResolveReceiptVerificationAuthorityRequest)
	case ControllerOperationPersistHandback:
		want.PersistHandback = selected.(ControllerPersistHandbackRequest)
	case ControllerOperationPersistQuarantine:
		want.PersistQuarantine = selected.(ControllerPersistQuarantineRequest)
	case ControllerOperationPersistAbort:
		want.PersistAbort = selected.(ControllerPersistAbortRequest)
	case ControllerOperationResolveClaimMCP:
		want.ResolveClaimMCP = selected.(ControllerResolveClaimMCPRequest)
	}
	if !reflect.DeepEqual(call, want) {
		return fmt.Errorf("sealedexec: controller call carries wrong or multiple operation payloads")
	}
	return nil
}

func controllerResultArmsZero(result ControllerResult) bool {
	want := ControllerResult{Schema: result.Schema, CallSequence: result.CallSequence, Operation: result.Operation, Error: result.Error}
	return reflect.DeepEqual(result, want)
}

func controllerSuccessArmsMatch(result ControllerResult) bool {
	want := ControllerResult{Schema: result.Schema, CallSequence: result.CallSequence, Operation: result.Operation}
	switch result.Operation {
	case ControllerOperationVerifyAuthority:
		want.VerifyAuthority = result.VerifyAuthority
	case ControllerOperationResolveProfile:
		want.ResolveProfile = result.ResolveProfile
	case ControllerOperationVerifyConflict:
		want.VerifyConflict = result.VerifyConflict
	case ControllerOperationResolveRecorder:
		want.ResolveRecorder = result.ResolveRecorder
	case ControllerOperationRecorderCheckpoint:
		want.RecorderCheckpoint = result.RecorderCheckpoint
	case ControllerOperationRecorderAppend:
		want.RecorderAppend = result.RecorderAppend
	case ControllerOperationStoreRedactedSegment:
		want.StoreRedactedSegment = result.StoreRedactedSegment
	case ControllerOperationResolveRedactedSegment:
		want.ResolveRedactedSegment = result.ResolveRedactedSegment
	case ControllerOperationVerifyOpaqueBoundary:
		want.VerifyOpaqueBoundary = result.VerifyOpaqueBoundary
	case ControllerOperationVerifyProviderSession:
		want.VerifyProviderSession = result.VerifyProviderSession
	case ControllerOperationVerifyExpansion:
		want.VerifyExpansion = result.VerifyExpansion
	case ControllerOperationStoreAdapterSession:
		want.StoreAdapterSession = result.StoreAdapterSession
	case ControllerOperationNextStamp:
		want.NextStamp = result.NextStamp
	case ControllerOperationResolveContext:
		want.ResolveContext = result.ResolveContext
	case ControllerOperationVerifyEpoch:
		want.VerifyEpoch = result.VerifyEpoch
	case ControllerOperationInstallExpansion:
		want.InstallExpansion = result.InstallExpansion
	case ControllerOperationResolveReceiptInputs:
		want.ResolveReceiptInputs = result.ResolveReceiptInputs
	case ControllerOperationAppendReceipt:
		want.AppendReceipt = result.AppendReceipt
	case ControllerOperationResolveReceiptVerificationAuthority:
		want.ResolveReceiptVerificationAuthority = result.ResolveReceiptVerificationAuthority
	case ControllerOperationPersistHandback:
		want.PersistHandback = result.PersistHandback
	case ControllerOperationPersistQuarantine:
		want.PersistQuarantine = result.PersistQuarantine
	case ControllerOperationPersistAbort:
		want.PersistAbort = result.PersistAbort
	case ControllerOperationResolveClaimMCP:
		want.ResolveClaimMCP = result.ResolveClaimMCP
	}
	return reflect.DeepEqual(result, want)
}

func canonicalEventAck(ack contextevent.EventAck) (contextevent.EventAck, error) {
	b, e := contextevent.EncodeEventAck(ack)
	if e != nil {
		return contextevent.EventAck{}, e
	}
	return contextevent.DecodeEventAck(bytes.NewReader(b))
}

func canonicalReceiptAck(ack contextevent.ReceiptEventAck) (contextevent.ReceiptEventAck, error) {
	b, e := contextevent.EncodeReceiptEventAck(ack)
	if e != nil {
		return contextevent.ReceiptEventAck{}, e
	}
	return contextevent.DecodeReceiptEventAck(bytes.NewReader(b))
}
