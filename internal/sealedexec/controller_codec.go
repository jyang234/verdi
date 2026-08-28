package sealedexec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/execworkspace"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyconflict"
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

type verificationWire struct {
	State     contextcompile.Resolution `json:"state"`
	Failure   FailureCode               `json:"failure"`
	Witnesses []string                  `json:"witnesses"`
}

type executionKeyWire struct {
	Flight string `json:"flight"`
	Lane   string `json:"lane"`
	Epoch  string `json:"epoch"`
}

type authorityFactsWire struct {
	State              contextcompile.Resolution `json:"state"`
	Failure            FailureCode               `json:"failure"`
	Witnesses          []string                  `json:"witnesses"`
	ManifestRevision   uint64                    `json:"manifest_revision"`
	ManifestDigest     string                    `json:"manifest_digest"`
	ProjectionDigest   string                    `json:"projection_digest"`
	AuthorityDigest    string                    `json:"authority_digest"`
	AcceptedSpecCommit string                    `json:"accepted_spec_commit"`
}

type profileQueryWire struct {
	Ref           LogicalRef      `json:"ref"`
	WorkspacePath string          `json:"workspace_path"`
	Grants        json.RawMessage `json:"grants"`
}

type profileMaterialWire struct {
	Ref                LogicalRef `json:"ref"`
	Name               string     `json:"name"`
	AbsoluteExecutable string     `json:"absolute_executable"`
	AbsoluteEnvRoot    string     `json:"absolute_env_root"`
	AbsoluteCodexHome  string     `json:"absolute_codex_home"`
	AdapterVersion     string     `json:"adapter_version"`
	DecoderProfile     string     `json:"decoder_profile"`
}

type conflictFactsWire struct {
	State     contextcompile.Resolution `json:"state"`
	Failure   FailureCode               `json:"failure"`
	Witnesses []string                  `json:"witnesses"`
	Report    json.RawMessage           `json:"report"`
}

type recorderFactsWire struct {
	State     contextcompile.Resolution `json:"state"`
	Failure   FailureCode               `json:"failure"`
	Witnesses []string                  `json:"witnesses"`
	Ref       LogicalRef                `json:"ref"`
}

type recorderCheckpointWire struct {
	State                  contextcompile.Resolution `json:"state"`
	Failure                FailureCode               `json:"failure"`
	Witnesses              []string                  `json:"witnesses"`
	Digest                 string                    `json:"digest"`
	Revisions              []contextevent.Revision   `json:"revisions"`
	EventChainRoot         string                    `json:"event_chain_root"`
	TerminalSourceSequence uint64                    `json:"terminal_source_sequence"`
	TerminalGlobalSequence uint64                    `json:"terminal_global_sequence"`
	ActiveRevision         json.RawMessage           `json:"active_revision"`
}

type activeRevisionWire struct {
	Revision           uint64                      `json:"revision"`
	ManifestDigest     string                      `json:"manifest_digest"`
	NextSourceSequence uint64                      `json:"next_source_sequence"`
	PriorEventDigest   string                      `json:"prior_event_digest"`
	PriorRevision      *contextevent.PriorRevision `json:"prior_revision"`
	LastGlobalSequence uint64                      `json:"last_global_sequence"`
	Invalidated        bool                        `json:"invalidated"`
}

type opaqueIdentityWire struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	AdapterID      string `json:"adapter_id"`
	AdapterVersion string `json:"adapter_version"`
}

type opaqueFactsWire struct {
	State     contextcompile.Resolution `json:"state"`
	Failure   FailureCode               `json:"failure"`
	Witnesses []string                  `json:"witnesses"`
	Rows      []opaqueIdentityWire      `json:"rows"`
}

type providerSessionCheckWire struct {
	SessionRef     string `json:"session_ref"`
	AdapterVersion string `json:"adapter_version"`
	ProfileDigest  string `json:"profile_digest"`
	WorkspaceID    string `json:"workspace_id"`
}

type providerSessionFactsWire struct {
	State          contextcompile.Resolution `json:"state"`
	Failure        FailureCode               `json:"failure"`
	Witnesses      []string                  `json:"witnesses"`
	SessionRef     string                    `json:"session_ref"`
	AdapterVersion string                    `json:"adapter_version"`
	ProfileDigest  string                    `json:"profile_digest"`
	WorkspaceID    string                    `json:"workspace_id"`
}

type expansionFactsWire struct {
	State     contextcompile.Resolution `json:"state"`
	Failure   FailureCode               `json:"failure"`
	Witnesses []string                  `json:"witnesses"`
	Root      string                    `json:"root"`
}

type sessionRecordWire struct {
	Key            executionKeyWire      `json:"key"`
	SessionRef     string                `json:"session_ref"`
	AdapterVersion string                `json:"adapter_version"`
	ProfileDigest  string                `json:"profile_digest"`
	WorkspaceID    string                `json:"workspace_id"`
	LifecycleAck   contextevent.EventAck `json:"lifecycle_ack"`
}

type contextQueryWire struct {
	Key executionKeyWire `json:"key"`
	Ref string           `json:"ref"`
}

type contextResolutionWire struct {
	State     contextcompile.Resolution `json:"state"`
	Failure   FailureCode               `json:"failure"`
	Witnesses []string                  `json:"witnesses"`
	Ref       string                    `json:"ref"`
	Data      json.RawMessage           `json:"data"`
}

type flightStateSnapshotWire struct {
	Request            json.RawMessage             `json:"request"`
	Key                executionKeyWire            `json:"key"`
	WorkspaceID        string                      `json:"workspace_id"`
	CandidateCommit    string                      `json:"candidate_commit"`
	CandidateTree      string                      `json:"candidate_tree"`
	Revision           uint64                      `json:"revision"`
	ManifestDigest     string                      `json:"manifest_digest"`
	ProjectionDigest   string                      `json:"projection_digest"`
	ExpansionRoot      string                      `json:"expansion_root"`
	NextSourceSequence uint64                      `json:"next_source_sequence"`
	PriorEventDigest   string                      `json:"prior_event_digest"`
	PriorRevision      *contextevent.PriorRevision `json:"prior_revision,omitempty"`
	LastGlobalSequence uint64                      `json:"last_global_sequence"`
	Invalidated        bool                        `json:"invalidated"`
}

type epochCheckWire struct {
	Snapshot   flightStateSnapshotWire `json:"snapshot"`
	Resolution contextResolutionWire   `json:"resolution"`
}

type expansionInstallWire struct {
	Key                  executionKeyWire      `json:"key"`
	RequestID            string                `json:"request_id"`
	ParentRevision       uint64                `json:"parent_revision"`
	ParentManifestDigest string                `json:"parent_manifest_digest"`
	ChildRevision        uint64                `json:"child_revision"`
	ChildManifestDigest  string                `json:"child_manifest_digest"`
	ExpansionDigest      string                `json:"expansion_digest"`
	ExpansionRoot        string                `json:"expansion_root"`
	TerminalAck          contextevent.EventAck `json:"terminal_ack"`
}

type receiptInputsQueryWire struct {
	Request                json.RawMessage `json:"request"`
	WorkspaceID            string          `json:"workspace_id"`
	DispatchDigest         string          `json:"dispatch_digest"`
	TerminalRevision       uint64          `json:"terminal_revision"`
	TerminalSourceSequence uint64          `json:"terminal_source_sequence"`
	TerminalGlobalSequence uint64          `json:"terminal_global_sequence"`
	EventChainRoot         string          `json:"event_chain_root"`
	ResultFactsDigest      string          `json:"result_facts_digest"`
}

type receiptInputsWire struct {
	Expansions      []contextreceipt.Expansion   `json:"expansions"`
	Obligations     []contextreceipt.Obligation  `json:"obligations"`
	Evidence        []contextreceipt.Evidence    `json:"evidence"`
	ReviewInputs    []contextreceipt.ReviewInput `json:"review_inputs"`
	RunnerPrincipal gp.PrincipalResolution       `json:"runner_principal"`
}

type receiptAppendWire struct {
	Receipt json.RawMessage `json:"receipt"`
	Event   json.RawMessage `json:"event"`
}

// EncodeControllerCall validates and canonically encodes one typed call.
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
	payload, err := encodeControllerCallPayload(call)
	if err != nil {
		return nil, err
	}
	return canonjson.Marshal(controllerCallWire{Schema: call.Schema, CallSequence: call.CallSequence, Operation: call.Operation, Payload: payload})
}

// DecodeControllerCall strictly decodes one canonical typed call frame.
func DecodeControllerCall(reader io.Reader) (ControllerCall, error) {
	var wire controllerCallWire
	raw, err := decodeStrict(reader, &wire)
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
	if err := decodeControllerCallPayload(wire.Payload, &call); err != nil {
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

// EncodeControllerResult validates and canonically encodes exactly one result
// or operational error arm.
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
		payload.Result, err = encodeControllerSuccessPayload(result)
	}
	if err != nil {
		return nil, err
	}
	payloadBytes, err := marshalControllerPayload(payload)
	if err != nil {
		return nil, err
	}
	return canonjson.Marshal(controllerResultWire{Schema: result.Schema, CallSequence: result.CallSequence, Operation: result.Operation, Payload: payloadBytes})
}

// DecodeControllerResult strictly decodes one canonical typed reply frame.
func DecodeControllerResult(reader io.Reader) (ControllerResult, error) {
	var wire controllerResultWire
	raw, err := decodeStrict(reader, &wire)
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
	} else if err := decodeControllerSuccessPayload(outcome.Result, &result); err != nil {
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

func encodeControllerCallPayload(call ControllerCall) (json.RawMessage, error) {
	wantSchema := controllerRequestSchema(call.Operation)
	switch call.Operation {
	case ControllerOperationVerifyAuthority:
		if err := requireOnlyCallArm(call, call.VerifyAuthority); err != nil {
			return nil, err
		}
		if call.VerifyAuthority.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		request, err := EncodeExecutionRequest(call.VerifyAuthority.Request)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema  string          `json:"schema"`
			Request json.RawMessage `json:"request"`
		}{wantSchema, trimFrame(request)})
	case ControllerOperationResolveProfile:
		if err := requireOnlyCallArm(call, call.ResolveProfile); err != nil {
			return nil, err
		}
		if call.ResolveProfile.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		wire, err := profileQueryToWire(call.ResolveProfile.Query)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string           `json:"schema"`
			Query  profileQueryWire `json:"query"`
		}{wantSchema, wire})
	case ControllerOperationVerifyConflict:
		if err := requireOnlyCallArm(call, call.VerifyConflict); err != nil {
			return nil, err
		}
		if call.VerifyConflict.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		report, err := policyconflict.EncodeReport(call.VerifyConflict.Report)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string          `json:"schema"`
			Report json.RawMessage `json:"report"`
		}{wantSchema, trimFrame(report)})
	case ControllerOperationResolveRecorder:
		if err := requireOnlyCallArm(call, call.ResolveRecorder); err != nil {
			return nil, err
		}
		if call.ResolveRecorder.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		if err := validateLogicalRef("recorder ref", call.ResolveRecorder.Ref, RecorderEndpointRefSchemaID); err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string     `json:"schema"`
			Ref    LogicalRef `json:"ref"`
		}{wantSchema, call.ResolveRecorder.Ref})
	case ControllerOperationRecorderCheckpoint:
		if err := requireOnlyCallArm(call, call.RecorderCheckpoint); err != nil {
			return nil, err
		}
		if call.RecorderCheckpoint.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		if err := validateExecutionKey(call.RecorderCheckpoint.Key); err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string           `json:"schema"`
			Key    executionKeyWire `json:"key"`
		}{wantSchema, executionKeyToWire(call.RecorderCheckpoint.Key)})
	case ControllerOperationRecorderAppend:
		if err := requireOnlyCallArm(call, call.RecorderAppend); err != nil {
			return nil, err
		}
		if call.RecorderAppend.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		event, err := contextevent.EncodeEvent(call.RecorderAppend.Event)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string          `json:"schema"`
			Event  json.RawMessage `json:"event"`
		}{wantSchema, trimFrame(event)})
	case ControllerOperationVerifyOpaqueBoundary:
		if err := requireOnlyCallArm(call, call.VerifyOpaqueBoundary); err != nil {
			return nil, err
		}
		if call.VerifyOpaqueBoundary.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		if err := validateOpaqueEntries(call.VerifyOpaqueBoundary.Rows); err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string                       `json:"schema"`
			Rows   []contextcompile.OpaqueEntry `json:"rows"`
		}{wantSchema, call.VerifyOpaqueBoundary.Rows})
	case ControllerOperationVerifyProviderSession:
		if err := requireOnlyCallArm(call, call.VerifyProviderSession); err != nil {
			return nil, err
		}
		if call.VerifyProviderSession.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		if err := validateProviderSessionCheck(call.VerifyProviderSession.Check); err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string                   `json:"schema"`
			Check  providerSessionCheckWire `json:"check"`
		}{wantSchema, providerSessionCheckToWire(call.VerifyProviderSession.Check)})
	case ControllerOperationVerifyExpansion:
		if err := requireOnlyCallArm(call, call.VerifyExpansion); err != nil {
			return nil, err
		}
		if call.VerifyExpansion.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		if err := validateExecutionKey(call.VerifyExpansion.Key); err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string           `json:"schema"`
			Key    executionKeyWire `json:"key"`
		}{wantSchema, executionKeyToWire(call.VerifyExpansion.Key)})
	case ControllerOperationStoreAdapterSession:
		if err := requireOnlyCallArm(call, call.StoreAdapterSession); err != nil {
			return nil, err
		}
		if call.StoreAdapterSession.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		wire, err := sessionRecordToWire(call.StoreAdapterSession.Record)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string            `json:"schema"`
			Record sessionRecordWire `json:"record"`
		}{wantSchema, wire})
	case ControllerOperationNextStamp:
		if err := requireOnlyCallArm(call, call.NextStamp); err != nil {
			return nil, err
		}
		if call.NextStamp.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		return marshalControllerPayload(struct {
			Schema string `json:"schema"`
		}{wantSchema})
	case ControllerOperationResolveContext:
		if err := requireOnlyCallArm(call, call.ResolveContext); err != nil {
			return nil, err
		}
		if call.ResolveContext.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		if err := validateContextQuery(call.ResolveContext.Query); err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string           `json:"schema"`
			Query  contextQueryWire `json:"query"`
		}{wantSchema, contextQueryToWire(call.ResolveContext.Query)})
	case ControllerOperationVerifyEpoch:
		if err := requireOnlyCallArm(call, call.VerifyEpoch); err != nil {
			return nil, err
		}
		if call.VerifyEpoch.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		wire, err := epochCheckToWire(call.VerifyEpoch.Check)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string         `json:"schema"`
			Check  epochCheckWire `json:"check"`
		}{wantSchema, wire})
	case ControllerOperationInstallExpansion:
		if err := requireOnlyCallArm(call, call.InstallExpansion); err != nil {
			return nil, err
		}
		if call.InstallExpansion.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		wire, err := expansionInstallToWire(call.InstallExpansion.Install)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema  string               `json:"schema"`
			Install expansionInstallWire `json:"install"`
		}{wantSchema, wire})
	case ControllerOperationResolveReceiptInputs:
		if err := requireOnlyCallArm(call, call.ResolveReceiptInputs); err != nil {
			return nil, err
		}
		if call.ResolveReceiptInputs.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		wire, err := receiptInputsQueryToWire(call.ResolveReceiptInputs.Query)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string                 `json:"schema"`
			Query  receiptInputsQueryWire `json:"query"`
		}{wantSchema, wire})
	case ControllerOperationAppendReceipt:
		if err := requireOnlyCallArm(call, call.AppendReceipt); err != nil {
			return nil, err
		}
		if call.AppendReceipt.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		wire, err := receiptAppendToWire(call.AppendReceipt.Append)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string            `json:"schema"`
			Append receiptAppendWire `json:"append"`
		}{wantSchema, wire})
	case ControllerOperationPersistHandback:
		if err := requireOnlyCallArm(call, call.PersistHandback); err != nil {
			return nil, err
		}
		if call.PersistHandback.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		record, err := EncodeHandbackRecord(call.PersistHandback.Record)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string          `json:"schema"`
			Record json.RawMessage `json:"record"`
		}{wantSchema, trimFrame(record)})
	case ControllerOperationPersistQuarantine:
		if err := requireOnlyCallArm(call, call.PersistQuarantine); err != nil {
			return nil, err
		}
		if call.PersistQuarantine.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		record, err := EncodeQuarantineRecord(call.PersistQuarantine.Record)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string          `json:"schema"`
			Record json.RawMessage `json:"record"`
		}{wantSchema, trimFrame(record)})
	case ControllerOperationPersistAbort:
		if err := requireOnlyCallArm(call, call.PersistAbort); err != nil {
			return nil, err
		}
		if call.PersistAbort.Schema != wantSchema {
			return nil, operationSchemaError(call.Operation)
		}
		record, err := EncodeAbortRecord(call.PersistAbort.Record)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string          `json:"schema"`
			Record json.RawMessage `json:"record"`
		}{wantSchema, trimFrame(record)})
	default:
		return nil, fmt.Errorf("sealedexec: unknown controller operation %q", call.Operation)
	}
}

func decodeControllerCallPayload(raw json.RawMessage, call *ControllerCall) error {
	schema := controllerRequestSchema(call.Operation)
	switch call.Operation {
	case ControllerOperationVerifyAuthority:
		var wire struct {
			Schema  string          `json:"schema"`
			Request json.RawMessage `json:"request"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema || rawMissing(wire.Request) {
			return operationSchemaError(call.Operation)
		}
		request, err := DecodeExecutionRequest(bytes.NewReader(frameNested(wire.Request)))
		if err != nil {
			return err
		}
		call.VerifyAuthority = ControllerVerifyAuthorityRequest{schema, request}
	case ControllerOperationResolveProfile:
		var wire struct {
			Schema string           `json:"schema"`
			Query  profileQueryWire `json:"query"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		query, err := profileQueryFromWire(wire.Query)
		if err != nil {
			return err
		}
		call.ResolveProfile = ControllerResolveProfileRequest{schema, query}
	case ControllerOperationVerifyConflict:
		var wire struct {
			Schema string          `json:"schema"`
			Report json.RawMessage `json:"report"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema || rawMissing(wire.Report) {
			return operationSchemaError(call.Operation)
		}
		report, err := policyconflict.DecodeReport(frameNested(wire.Report))
		if err != nil {
			return err
		}
		call.VerifyConflict = ControllerVerifyConflictRequest{schema, report}
	case ControllerOperationResolveRecorder:
		var wire struct {
			Schema string     `json:"schema"`
			Ref    LogicalRef `json:"ref"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		call.ResolveRecorder = ControllerResolveRecorderRequest{schema, wire.Ref}
	case ControllerOperationRecorderCheckpoint:
		var wire struct {
			Schema string           `json:"schema"`
			Key    executionKeyWire `json:"key"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		call.RecorderCheckpoint = ControllerRecorderCheckpointRequest{schema, executionKeyFromWire(wire.Key)}
	case ControllerOperationRecorderAppend:
		var wire struct {
			Schema string          `json:"schema"`
			Event  json.RawMessage `json:"event"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema || rawMissing(wire.Event) {
			return operationSchemaError(call.Operation)
		}
		event, err := contextevent.DecodeEvent(bytes.NewReader(frameNested(wire.Event)))
		if err != nil {
			return err
		}
		call.RecorderAppend = ControllerRecorderAppendRequest{schema, event}
	case ControllerOperationVerifyOpaqueBoundary:
		var wire struct {
			Schema string                       `json:"schema"`
			Rows   []contextcompile.OpaqueEntry `json:"rows"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		call.VerifyOpaqueBoundary = ControllerVerifyOpaqueBoundaryRequest{schema, wire.Rows}
	case ControllerOperationVerifyProviderSession:
		var wire struct {
			Schema string                   `json:"schema"`
			Check  providerSessionCheckWire `json:"check"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		call.VerifyProviderSession = ControllerVerifyProviderSessionRequest{schema, providerSessionCheckFromWire(wire.Check)}
	case ControllerOperationVerifyExpansion:
		var wire struct {
			Schema string           `json:"schema"`
			Key    executionKeyWire `json:"key"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		call.VerifyExpansion = ControllerVerifyExpansionRequest{schema, executionKeyFromWire(wire.Key)}
	case ControllerOperationStoreAdapterSession:
		var wire struct {
			Schema string            `json:"schema"`
			Record sessionRecordWire `json:"record"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		record, err := sessionRecordFromWire(wire.Record)
		if err != nil {
			return err
		}
		call.StoreAdapterSession = ControllerStoreAdapterSessionRequest{schema, record}
	case ControllerOperationNextStamp:
		var wire struct {
			Schema string `json:"schema"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		call.NextStamp = ControllerNextStampRequest{schema}
	case ControllerOperationResolveContext:
		var wire struct {
			Schema string           `json:"schema"`
			Query  contextQueryWire `json:"query"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		call.ResolveContext = ControllerResolveContextRequest{schema, contextQueryFromWire(wire.Query)}
	case ControllerOperationVerifyEpoch:
		var wire struct {
			Schema string         `json:"schema"`
			Check  epochCheckWire `json:"check"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		check, err := epochCheckFromWire(wire.Check)
		if err != nil {
			return err
		}
		call.VerifyEpoch = ControllerVerifyEpochRequest{schema, check}
	case ControllerOperationInstallExpansion:
		var wire struct {
			Schema  string               `json:"schema"`
			Install expansionInstallWire `json:"install"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		install, err := expansionInstallFromWire(wire.Install)
		if err != nil {
			return err
		}
		call.InstallExpansion = ControllerInstallExpansionRequest{schema, install}
	case ControllerOperationResolveReceiptInputs:
		var wire struct {
			Schema string                 `json:"schema"`
			Query  receiptInputsQueryWire `json:"query"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		query, err := receiptInputsQueryFromWire(wire.Query)
		if err != nil {
			return err
		}
		call.ResolveReceiptInputs = ControllerResolveReceiptInputsRequest{schema, query}
	case ControllerOperationAppendReceipt:
		var wire struct {
			Schema string            `json:"schema"`
			Append receiptAppendWire `json:"append"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		appendValue, err := receiptAppendFromWire(wire.Append)
		if err != nil {
			return err
		}
		call.AppendReceipt = ControllerAppendReceiptRequest{schema, appendValue}
	case ControllerOperationPersistHandback:
		var wire struct {
			Schema string          `json:"schema"`
			Record json.RawMessage `json:"record"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		record, err := DecodeHandbackRecord(bytes.NewReader(frameNested(wire.Record)))
		if err != nil {
			return err
		}
		call.PersistHandback = ControllerPersistHandbackRequest{schema, record}
	case ControllerOperationPersistQuarantine:
		var wire struct {
			Schema string          `json:"schema"`
			Record json.RawMessage `json:"record"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		record, err := DecodeQuarantineRecord(bytes.NewReader(frameNested(wire.Record)))
		if err != nil {
			return err
		}
		call.PersistQuarantine = ControllerPersistQuarantineRequest{schema, record}
	case ControllerOperationPersistAbort:
		var wire struct {
			Schema string          `json:"schema"`
			Record json.RawMessage `json:"record"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(call.Operation)
		}
		record, err := DecodeAbortRecord(bytes.NewReader(frameNested(wire.Record)))
		if err != nil {
			return err
		}
		call.PersistAbort = ControllerPersistAbortRequest{schema, record}
	default:
		return fmt.Errorf("sealedexec: unknown controller operation %q", call.Operation)
	}
	return nil
}

func encodeControllerSuccessPayload(result ControllerResult) (json.RawMessage, error) {
	wantSchema := controllerResultSchema(result.Operation)
	if !controllerSuccessArmsMatch(result) {
		return nil, fmt.Errorf("sealedexec: controller success carries wrong or multiple result arms")
	}
	switch result.Operation {
	case ControllerOperationVerifyAuthority:
		if result.VerifyAuthority.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		wire, err := authorityFactsToWire(result.VerifyAuthority.Facts)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string             `json:"schema"`
			Facts  authorityFactsWire `json:"facts"`
		}{wantSchema, wire})
	case ControllerOperationResolveProfile:
		if result.ResolveProfile.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		wire, err := profileMaterialToWire(result.ResolveProfile.Material)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema   string              `json:"schema"`
			Material profileMaterialWire `json:"material"`
		}{wantSchema, wire})
	case ControllerOperationVerifyConflict:
		if result.VerifyConflict.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		wire, err := conflictFactsToWire(result.VerifyConflict.Facts)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string            `json:"schema"`
			Facts  conflictFactsWire `json:"facts"`
		}{wantSchema, wire})
	case ControllerOperationResolveRecorder:
		if result.ResolveRecorder.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		wire, err := recorderFactsToWire(result.ResolveRecorder.Facts)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string            `json:"schema"`
			Facts  recorderFactsWire `json:"facts"`
		}{wantSchema, wire})
	case ControllerOperationRecorderCheckpoint:
		if result.RecorderCheckpoint.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		wire, err := recorderCheckpointToWire(result.RecorderCheckpoint.Checkpoint)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema     string                 `json:"schema"`
			Checkpoint recorderCheckpointWire `json:"checkpoint"`
		}{wantSchema, wire})
	case ControllerOperationRecorderAppend:
		if result.RecorderAppend.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		ack, err := canonicalEventAck(result.RecorderAppend.Ack)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string                `json:"schema"`
			Ack    contextevent.EventAck `json:"ack"`
		}{wantSchema, ack})
	case ControllerOperationVerifyOpaqueBoundary:
		if result.VerifyOpaqueBoundary.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		wire, err := opaqueFactsToWire(result.VerifyOpaqueBoundary.Facts)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string          `json:"schema"`
			Facts  opaqueFactsWire `json:"facts"`
		}{wantSchema, wire})
	case ControllerOperationVerifyProviderSession:
		if result.VerifyProviderSession.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		wire, err := providerSessionFactsToWire(result.VerifyProviderSession.Facts)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string                   `json:"schema"`
			Facts  providerSessionFactsWire `json:"facts"`
		}{wantSchema, wire})
	case ControllerOperationVerifyExpansion:
		if result.VerifyExpansion.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		wire, err := expansionFactsToWire(result.VerifyExpansion.Facts)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string             `json:"schema"`
			Facts  expansionFactsWire `json:"facts"`
		}{wantSchema, wire})
	case ControllerOperationStoreAdapterSession:
		if result.StoreAdapterSession.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		return marshalControllerPayload(struct {
			Schema string `json:"schema"`
		}{wantSchema})
	case ControllerOperationNextStamp:
		if result.NextStamp.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		if err := validateControllerStamp(result.NextStamp.Stamp); err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string `json:"schema"`
			Stamp  string `json:"stamp"`
		}{wantSchema, result.NextStamp.Stamp})
	case ControllerOperationResolveContext:
		if result.ResolveContext.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		wire, err := contextResolutionToWire(result.ResolveContext.Resolution)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema     string                `json:"schema"`
			Resolution contextResolutionWire `json:"resolution"`
		}{wantSchema, wire})
	case ControllerOperationVerifyEpoch:
		if result.VerifyEpoch.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		wire, err := verificationToWire(result.VerifyEpoch.Verification)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema       string           `json:"schema"`
			Verification verificationWire `json:"verification"`
		}{wantSchema, wire})
	case ControllerOperationInstallExpansion:
		if result.InstallExpansion.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		return marshalControllerPayload(struct {
			Schema string `json:"schema"`
		}{wantSchema})
	case ControllerOperationResolveReceiptInputs:
		if result.ResolveReceiptInputs.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		wire, err := receiptInputsToWire(result.ResolveReceiptInputs.Inputs)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string            `json:"schema"`
			Inputs receiptInputsWire `json:"inputs"`
		}{wantSchema, wire})
	case ControllerOperationAppendReceipt:
		if result.AppendReceipt.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		ack, err := canonicalReceiptAck(result.AppendReceipt.Ack)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string                       `json:"schema"`
			Ack    contextevent.ReceiptEventAck `json:"ack"`
		}{wantSchema, ack})
	case ControllerOperationPersistHandback:
		if result.PersistHandback.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		ack, err := canonicalControlAck(result.PersistHandback.Ack)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string          `json:"schema"`
			Ack    json.RawMessage `json:"ack"`
		}{wantSchema, ack})
	case ControllerOperationPersistQuarantine:
		if result.PersistQuarantine.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		ack, err := canonicalControlAck(result.PersistQuarantine.Ack)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string          `json:"schema"`
			Ack    json.RawMessage `json:"ack"`
		}{wantSchema, ack})
	case ControllerOperationPersistAbort:
		if result.PersistAbort.Schema != wantSchema {
			return nil, operationSchemaError(result.Operation)
		}
		ack, err := canonicalControlAck(result.PersistAbort.Ack)
		if err != nil {
			return nil, err
		}
		return marshalControllerPayload(struct {
			Schema string          `json:"schema"`
			Ack    json.RawMessage `json:"ack"`
		}{wantSchema, ack})
	default:
		return nil, fmt.Errorf("sealedexec: unknown controller operation %q", result.Operation)
	}
}

func decodeControllerSuccessPayload(raw json.RawMessage, result *ControllerResult) error {
	schema := controllerResultSchema(result.Operation)
	switch result.Operation {
	case ControllerOperationVerifyAuthority:
		var wire struct {
			Schema string             `json:"schema"`
			Facts  authorityFactsWire `json:"facts"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		facts, err := authorityFactsFromWire(wire.Facts)
		if err != nil {
			return err
		}
		result.VerifyAuthority = ControllerVerifyAuthorityResult{schema, facts}
	case ControllerOperationResolveProfile:
		var wire struct {
			Schema   string              `json:"schema"`
			Material profileMaterialWire `json:"material"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		material, err := profileMaterialFromWire(wire.Material)
		if err != nil {
			return err
		}
		result.ResolveProfile = ControllerResolveProfileResult{schema, material}
	case ControllerOperationVerifyConflict:
		var wire struct {
			Schema string            `json:"schema"`
			Facts  conflictFactsWire `json:"facts"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		facts, err := conflictFactsFromWire(wire.Facts)
		if err != nil {
			return err
		}
		result.VerifyConflict = ControllerVerifyConflictResult{schema, facts}
	case ControllerOperationResolveRecorder:
		var wire struct {
			Schema string            `json:"schema"`
			Facts  recorderFactsWire `json:"facts"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		facts, err := recorderFactsFromWire(wire.Facts)
		if err != nil {
			return err
		}
		result.ResolveRecorder = ControllerResolveRecorderResult{schema, facts}
	case ControllerOperationRecorderCheckpoint:
		var wire struct {
			Schema     string                 `json:"schema"`
			Checkpoint recorderCheckpointWire `json:"checkpoint"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		checkpoint, err := recorderCheckpointFromWire(wire.Checkpoint)
		if err != nil {
			return err
		}
		result.RecorderCheckpoint = ControllerRecorderCheckpointResult{schema, checkpoint}
	case ControllerOperationRecorderAppend:
		var wire struct {
			Schema string                `json:"schema"`
			Ack    contextevent.EventAck `json:"ack"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		ack, err := canonicalEventAck(wire.Ack)
		if err != nil {
			return err
		}
		result.RecorderAppend = ControllerRecorderAppendResult{schema, ack}
	case ControllerOperationVerifyOpaqueBoundary:
		var wire struct {
			Schema string          `json:"schema"`
			Facts  opaqueFactsWire `json:"facts"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		facts, err := opaqueFactsFromWire(wire.Facts)
		if err != nil {
			return err
		}
		result.VerifyOpaqueBoundary = ControllerVerifyOpaqueBoundaryResult{schema, facts}
	case ControllerOperationVerifyProviderSession:
		var wire struct {
			Schema string                   `json:"schema"`
			Facts  providerSessionFactsWire `json:"facts"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		facts, err := providerSessionFactsFromWire(wire.Facts)
		if err != nil {
			return err
		}
		result.VerifyProviderSession = ControllerVerifyProviderSessionResult{schema, facts}
	case ControllerOperationVerifyExpansion:
		var wire struct {
			Schema string             `json:"schema"`
			Facts  expansionFactsWire `json:"facts"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		facts, err := expansionFactsFromWire(wire.Facts)
		if err != nil {
			return err
		}
		result.VerifyExpansion = ControllerVerifyExpansionResult{schema, facts}
	case ControllerOperationStoreAdapterSession:
		var wire struct {
			Schema string `json:"schema"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		result.StoreAdapterSession = ControllerStoreAdapterSessionResult{schema}
	case ControllerOperationNextStamp:
		var wire struct {
			Schema string `json:"schema"`
			Stamp  string `json:"stamp"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		result.NextStamp = ControllerNextStampResult{schema, wire.Stamp}
	case ControllerOperationResolveContext:
		var wire struct {
			Schema     string                `json:"schema"`
			Resolution contextResolutionWire `json:"resolution"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		resolution, err := contextResolutionFromWire(wire.Resolution)
		if err != nil {
			return err
		}
		result.ResolveContext = ControllerResolveContextResult{schema, resolution}
	case ControllerOperationVerifyEpoch:
		var wire struct {
			Schema       string           `json:"schema"`
			Verification verificationWire `json:"verification"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		verification, err := verificationFromWire(wire.Verification)
		if err != nil {
			return err
		}
		result.VerifyEpoch = ControllerVerifyEpochResult{schema, verification}
	case ControllerOperationInstallExpansion:
		var wire struct {
			Schema string `json:"schema"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		result.InstallExpansion = ControllerInstallExpansionResult{schema}
	case ControllerOperationResolveReceiptInputs:
		var wire struct {
			Schema string            `json:"schema"`
			Inputs receiptInputsWire `json:"inputs"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		inputs, err := receiptInputsFromWire(wire.Inputs)
		if err != nil {
			return err
		}
		result.ResolveReceiptInputs = ControllerResolveReceiptInputsResult{schema, inputs}
	case ControllerOperationAppendReceipt:
		var wire struct {
			Schema string                       `json:"schema"`
			Ack    contextevent.ReceiptEventAck `json:"ack"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema {
			return operationSchemaError(result.Operation)
		}
		ack, err := canonicalReceiptAck(wire.Ack)
		if err != nil {
			return err
		}
		result.AppendReceipt = ControllerAppendReceiptResult{schema, ack}
	case ControllerOperationPersistHandback, ControllerOperationPersistQuarantine, ControllerOperationPersistAbort:
		var wire struct {
			Schema string          `json:"schema"`
			Ack    json.RawMessage `json:"ack"`
		}
		if err := unmarshalControllerPayload(raw, &wire); err != nil {
			return err
		}
		if wire.Schema != schema || rawMissing(wire.Ack) {
			return operationSchemaError(result.Operation)
		}
		ack, err := DecodeControlAck(bytes.NewReader(frameNested(wire.Ack)))
		if err != nil {
			return err
		}
		switch result.Operation {
		case ControllerOperationPersistHandback:
			result.PersistHandback = ControllerPersistHandbackResult{schema, ack}
		case ControllerOperationPersistQuarantine:
			result.PersistQuarantine = ControllerPersistQuarantineResult{schema, ack}
		case ControllerOperationPersistAbort:
			result.PersistAbort = ControllerPersistAbortResult{schema, ack}
		}
	default:
		return fmt.Errorf("sealedexec: unknown controller operation %q", result.Operation)
	}
	return nil
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
	case ControllerOperationPersistHandback:
		want.PersistHandback = selected.(ControllerPersistHandbackRequest)
	case ControllerOperationPersistQuarantine:
		want.PersistQuarantine = selected.(ControllerPersistQuarantineRequest)
	case ControllerOperationPersistAbort:
		want.PersistAbort = selected.(ControllerPersistAbortRequest)
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
	case ControllerOperationPersistHandback:
		want.PersistHandback = result.PersistHandback
	case ControllerOperationPersistQuarantine:
		want.PersistQuarantine = result.PersistQuarantine
	case ControllerOperationPersistAbort:
		want.PersistAbort = result.PersistAbort
	}
	return reflect.DeepEqual(result, want)
}

func verificationToWire(verification Verification) (verificationWire, error) {
	if err := validateControllerVerification(verification); err != nil {
		return verificationWire{}, err
	}
	return verificationWire(verification), nil
}

func verificationFromWire(wire verificationWire) (Verification, error) {
	verification := Verification(wire)
	if err := validateControllerVerification(verification); err != nil {
		return Verification{}, err
	}
	return verification, nil
}

func validateControllerVerification(verification Verification) error {
	if verification.Witnesses == nil {
		return fmt.Errorf("sealedexec: verification witnesses must be non-null")
	}
	if err := validateSortedTexts("verification witnesses", verification.Witnesses); err != nil {
		return err
	}
	if err := verification.validate("controller verification"); err != nil {
		return err
	}
	if verification.State != contextcompile.ResolutionProven && len(verification.Witnesses) == 0 {
		return fmt.Errorf("sealedexec: non-proven verification requires witnesses")
	}
	return nil
}

func authorityFactsToWire(facts AuthorityFacts) (authorityFactsWire, error) {
	if err := validateControllerVerification(facts.Verification); err != nil {
		return authorityFactsWire{}, err
	}
	for field, value := range map[string]string{"manifest_digest": facts.ManifestDigest, "projection_digest": facts.ProjectionDigest, "authority_digest": facts.AuthorityDigest} {
		if err := validateDigest(field, value); err != nil {
			return authorityFactsWire{}, err
		}
	}
	if err := validateGitOID("accepted_spec_commit", facts.AcceptedSpecCommit, false); err != nil {
		return authorityFactsWire{}, err
	}
	return authorityFactsWire{facts.State, facts.Failure, facts.Witnesses, facts.ManifestRevision, facts.ManifestDigest, facts.ProjectionDigest, facts.AuthorityDigest, facts.AcceptedSpecCommit}, nil
}

func authorityFactsFromWire(w authorityFactsWire) (AuthorityFacts, error) {
	v, e := verificationFromWire(verificationWire{w.State, w.Failure, w.Witnesses})
	if e != nil {
		return AuthorityFacts{}, e
	}
	f := AuthorityFacts{Verification: v, ManifestRevision: w.ManifestRevision, ManifestDigest: w.ManifestDigest, ProjectionDigest: w.ProjectionDigest, AuthorityDigest: w.AuthorityDigest, AcceptedSpecCommit: w.AcceptedSpecCommit}
	_, e = authorityFactsToWire(f)
	return f, e
}

func profileQueryToWire(query ProfileQuery) (profileQueryWire, error) {
	if err := validateLogicalRef("profile query ref", query.Ref, ProjectProfileRefSchemaID); err != nil {
		return profileQueryWire{}, err
	}
	if !filepath.IsAbs(query.WorkspacePath) || filepath.Clean(query.WorkspacePath) != query.WorkspacePath {
		return profileQueryWire{}, fmt.Errorf("sealedexec: profile workspace_path must be absolute and clean")
	}
	grants, err := execworkspace.EncodeGrantSet(query.Grants)
	if err != nil {
		return profileQueryWire{}, err
	}
	return profileQueryWire{query.Ref, query.WorkspacePath, trimFrame(grants)}, nil
}
func profileQueryFromWire(w profileQueryWire) (ProfileQuery, error) {
	grants, err := execworkspace.DecodeGrantSet(frameNested(w.Grants))
	if err != nil {
		return ProfileQuery{}, err
	}
	q := ProfileQuery{w.Ref, w.WorkspacePath, grants}
	_, err = profileQueryToWire(q)
	return q, err
}

func profileMaterialToWire(material ProfileMaterial) (profileMaterialWire, error) {
	if err := validateLogicalRef("profile material ref", material.Ref, ProjectProfileRefSchemaID); err != nil {
		return profileMaterialWire{}, err
	}
	for field, value := range map[string]string{"name": material.Name, "adapter_version": material.AdapterVersion, "decoder_profile": material.DecoderProfile} {
		if err := requireText(field, value); err != nil {
			return profileMaterialWire{}, err
		}
	}
	for field, value := range map[string]string{"absolute_executable": material.AbsoluteExecutable, "absolute_env_root": material.AbsoluteEnvRoot, "absolute_codex_home": material.AbsoluteCodexHome} {
		if !filepath.IsAbs(value) || filepath.Clean(value) != value {
			return profileMaterialWire{}, fmt.Errorf("sealedexec: %s must be absolute and clean", field)
		}
	}
	return profileMaterialWire(material), nil
}
func profileMaterialFromWire(w profileMaterialWire) (ProfileMaterial, error) {
	m := ProfileMaterial(w)
	_, e := profileMaterialToWire(m)
	return m, e
}

func conflictFactsToWire(f ConflictFacts) (conflictFactsWire, error) {
	if err := validateControllerVerification(f.Verification); err != nil {
		return conflictFactsWire{}, err
	}
	report, err := policyconflict.EncodeReport(f.Report)
	if err != nil {
		return conflictFactsWire{}, err
	}
	return conflictFactsWire{f.State, f.Failure, f.Witnesses, trimFrame(report)}, nil
}
func conflictFactsFromWire(w conflictFactsWire) (ConflictFacts, error) {
	v, e := verificationFromWire(verificationWire{w.State, w.Failure, w.Witnesses})
	if e != nil {
		return ConflictFacts{}, e
	}
	r, e := policyconflict.DecodeReport(frameNested(w.Report))
	if e != nil {
		return ConflictFacts{}, e
	}
	return ConflictFacts{Verification: v, Report: r}, nil
}
func recorderFactsToWire(f RecorderFacts) (recorderFactsWire, error) {
	if err := validateControllerVerification(f.Verification); err != nil {
		return recorderFactsWire{}, err
	}
	if err := validateLogicalRef("recorder facts ref", f.Ref, RecorderEndpointRefSchemaID); err != nil {
		return recorderFactsWire{}, err
	}
	return recorderFactsWire{f.State, f.Failure, f.Witnesses, f.Ref}, nil
}
func recorderFactsFromWire(w recorderFactsWire) (RecorderFacts, error) {
	v, e := verificationFromWire(verificationWire{w.State, w.Failure, w.Witnesses})
	if e != nil {
		return RecorderFacts{}, e
	}
	f := RecorderFacts{v, w.Ref}
	_, e = recorderFactsToWire(f)
	return f, e
}

func recorderCheckpointToWire(c RecorderCheckpoint) (recorderCheckpointWire, error) {
	if err := validateControllerVerification(c.Verification); err != nil {
		return recorderCheckpointWire{}, err
	}
	if err := validateDigest("recorder checkpoint digest", c.Digest); err != nil {
		return recorderCheckpointWire{}, err
	}
	if c.Revisions == nil {
		return recorderCheckpointWire{}, fmt.Errorf("sealedexec: recorder checkpoint revisions must be non-null")
	}
	if len(c.Revisions) == 0 {
		if c.EventChainRoot != "" || c.TerminalSourceSequence != 0 || c.TerminalGlobalSequence != 0 {
			return recorderCheckpointWire{}, fmt.Errorf("sealedexec: empty recorder checkpoint carries terminal facts")
		}
	} else {
		root, err := contextevent.EventChainRoot(c.Revisions)
		if err != nil {
			return recorderCheckpointWire{}, err
		}
		terminal := c.Revisions[len(c.Revisions)-1]
		if c.EventChainRoot != root || c.TerminalSourceSequence != terminal.TerminalSourceSequence || c.TerminalGlobalSequence != terminal.TerminalGlobalSequence {
			return recorderCheckpointWire{}, fmt.Errorf("sealedexec: recorder checkpoint terminal facts mismatch")
		}
	}
	active, err := activeRevisionToWire(c.ActiveRevision, c.Revisions, c.TerminalGlobalSequence)
	if err != nil {
		return recorderCheckpointWire{}, err
	}
	return recorderCheckpointWire{c.State, c.Failure, c.Witnesses, c.Digest, c.Revisions, c.EventChainRoot, c.TerminalSourceSequence, c.TerminalGlobalSequence, active}, nil
}
func recorderCheckpointFromWire(w recorderCheckpointWire) (RecorderCheckpoint, error) {
	v, e := verificationFromWire(verificationWire{w.State, w.Failure, w.Witnesses})
	if e != nil {
		return RecorderCheckpoint{}, e
	}
	active, e := activeRevisionFromWire(w.ActiveRevision)
	if e != nil {
		return RecorderCheckpoint{}, e
	}
	c := RecorderCheckpoint{Verification: v, Digest: w.Digest, Revisions: w.Revisions, EventChainRoot: w.EventChainRoot, TerminalSourceSequence: w.TerminalSourceSequence, TerminalGlobalSequence: w.TerminalGlobalSequence, ActiveRevision: active}
	_, e = recorderCheckpointToWire(c)
	return c, e
}

func activeRevisionToWire(active *ActiveRevision, revisions []contextevent.Revision, terminalGlobal uint64) (json.RawMessage, error) {
	if active == nil {
		return json.RawMessage("null"), nil
	}
	if err := validateDigest("active revision manifest digest", active.ManifestDigest); err != nil {
		return nil, err
	}
	if active.NextSourceSequence == 0 {
		return nil, fmt.Errorf("sealedexec: active revision next_source_sequence must be positive")
	}
	if active.LastGlobalSequence < terminalGlobal {
		return nil, fmt.Errorf("sealedexec: active revision last_global_sequence precedes the complete checkpoint")
	}
	if len(revisions) != 0 && active.Revision != revisions[len(revisions)-1].ManifestRevision+1 {
		return nil, fmt.Errorf("sealedexec: active revision does not immediately follow the complete checkpoint")
	}
	if active.NextSourceSequence == 1 {
		if active.PriorEventDigest != "" {
			return nil, fmt.Errorf("sealedexec: sequence-one active revision cannot carry a prior event digest")
		}
		if len(revisions) == 0 {
			if active.PriorRevision == nil {
				if active.LastGlobalSequence != terminalGlobal {
					return nil, fmt.Errorf("sealedexec: pristine active revision cannot advance the global sequence")
				}
			} else {
				if err := validateActiveRevisionBridge(*active.PriorRevision); err != nil {
					return nil, err
				}
				if active.Revision == 0 || active.PriorRevision.ManifestRevision != active.Revision-1 || active.LastGlobalSequence != active.PriorRevision.TerminalGlobalSequence {
					return nil, fmt.Errorf("sealedexec: sequence-one active revision does not exactly bridge its omitted predecessor")
				}
			}
		} else {
			terminal := revisions[len(revisions)-1]
			want := contextevent.PriorRevision{
				ManifestRevision: terminal.ManifestRevision, ManifestDigest: terminal.ManifestDigest,
				EventRoot: terminal.EventRoot, TerminalSourceSequence: terminal.TerminalSourceSequence,
				TerminalGlobalSequence: terminal.TerminalGlobalSequence,
			}
			if active.PriorRevision == nil || *active.PriorRevision != want || active.Revision != terminal.ManifestRevision+1 {
				return nil, fmt.Errorf("sealedexec: sequence-one active revision does not exactly bridge the complete checkpoint")
			}
			if active.LastGlobalSequence != terminalGlobal {
				return nil, fmt.Errorf("sealedexec: sequence-one active revision cannot advance beyond its predecessor")
			}
		}
	} else {
		if err := validateDigest("active revision prior event digest", active.PriorEventDigest); err != nil {
			return nil, err
		}
		if active.PriorRevision != nil {
			return nil, fmt.Errorf("sealedexec: later active source sequence cannot retain a prior-revision bridge")
		}
		if active.LastGlobalSequence <= terminalGlobal {
			return nil, fmt.Errorf("sealedexec: active events must advance beyond the complete checkpoint global sequence")
		}
	}
	wire := activeRevisionWire{
		Revision: active.Revision, ManifestDigest: active.ManifestDigest,
		NextSourceSequence: active.NextSourceSequence, PriorEventDigest: active.PriorEventDigest,
		LastGlobalSequence: active.LastGlobalSequence, Invalidated: active.Invalidated,
	}
	if active.PriorRevision != nil {
		copy := *active.PriorRevision
		wire.PriorRevision = &copy
	}
	encoded, err := canonjson.Marshal(wire)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(bytes.TrimSuffix(encoded, []byte{'\n'})), nil
}

func validateActiveRevisionBridge(bridge contextevent.PriorRevision) error {
	if err := validateDigest("active revision bridge manifest digest", bridge.ManifestDigest); err != nil {
		return err
	}
	if err := validateDigest("active revision bridge event root", bridge.EventRoot); err != nil {
		return err
	}
	if bridge.TerminalSourceSequence == 0 || bridge.TerminalGlobalSequence == 0 {
		return fmt.Errorf("sealedexec: active revision bridge terminal sequences must be positive")
	}
	return nil
}

func activeRevisionFromWire(raw json.RawMessage) (*ActiveRevision, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("sealedexec: recorder checkpoint active_revision is required")
	}
	if bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	var wire activeRevisionWire
	if err := unmarshalControllerPayload(raw, &wire); err != nil {
		return nil, err
	}
	active := &ActiveRevision{
		Revision: wire.Revision, ManifestDigest: wire.ManifestDigest,
		NextSourceSequence: wire.NextSourceSequence, PriorEventDigest: wire.PriorEventDigest,
		LastGlobalSequence: wire.LastGlobalSequence, Invalidated: wire.Invalidated,
	}
	if wire.PriorRevision != nil {
		copy := *wire.PriorRevision
		active.PriorRevision = &copy
	}
	return active, nil
}

func opaqueFactsToWire(f OpaqueBoundaryFacts) (opaqueFactsWire, error) {
	if err := validateControllerVerification(f.Verification); err != nil {
		return opaqueFactsWire{}, err
	}
	if f.Rows == nil {
		return opaqueFactsWire{}, fmt.Errorf("sealedexec: opaque facts rows must be non-null")
	}
	rows := make([]opaqueIdentityWire, len(f.Rows))
	for i, row := range f.Rows {
		for field, value := range map[string]string{"id": row.ID, "kind": row.Kind, "adapter_id": row.AdapterID, "adapter_version": row.AdapterVersion} {
			if err := requireText(field, value); err != nil {
				return opaqueFactsWire{}, err
			}
		}
		rows[i] = opaqueIdentityWire(row)
		if i > 0 && f.Rows[i-1].ID >= row.ID {
			return opaqueFactsWire{}, fmt.Errorf("sealedexec: opaque facts rows must be ordered and deduplicated")
		}
	}
	return opaqueFactsWire{f.State, f.Failure, f.Witnesses, rows}, nil
}
func opaqueFactsFromWire(w opaqueFactsWire) (OpaqueBoundaryFacts, error) {
	v, e := verificationFromWire(verificationWire{w.State, w.Failure, w.Witnesses})
	if e != nil {
		return OpaqueBoundaryFacts{}, e
	}
	rows := make([]OpaqueIdentity, len(w.Rows))
	for i, row := range w.Rows {
		rows[i] = OpaqueIdentity(row)
	}
	f := OpaqueBoundaryFacts{v, rows}
	_, e = opaqueFactsToWire(f)
	return f, e
}

func providerSessionCheckToWire(c ProviderSessionCheck) providerSessionCheckWire {
	return providerSessionCheckWire(c)
}
func providerSessionCheckFromWire(w providerSessionCheckWire) ProviderSessionCheck {
	return ProviderSessionCheck(w)
}
func validateProviderSessionCheck(c ProviderSessionCheck) error {
	for field, value := range map[string]string{"session_ref": c.SessionRef, "adapter_version": c.AdapterVersion, "workspace_id": c.WorkspaceID} {
		if err := requireText(field, value); err != nil {
			return err
		}
	}
	return validateDigest("profile_digest", c.ProfileDigest)
}
func providerSessionFactsToWire(f ProviderSessionFacts) (providerSessionFactsWire, error) {
	if err := validateControllerVerification(f.Verification); err != nil {
		return providerSessionFactsWire{}, err
	}
	if err := validateProviderSessionCheck(ProviderSessionCheck{f.SessionRef, f.AdapterVersion, f.ProfileDigest, f.WorkspaceID}); err != nil {
		return providerSessionFactsWire{}, err
	}
	return providerSessionFactsWire{f.State, f.Failure, f.Witnesses, f.SessionRef, f.AdapterVersion, f.ProfileDigest, f.WorkspaceID}, nil
}
func providerSessionFactsFromWire(w providerSessionFactsWire) (ProviderSessionFacts, error) {
	v, e := verificationFromWire(verificationWire{w.State, w.Failure, w.Witnesses})
	if e != nil {
		return ProviderSessionFacts{}, e
	}
	f := ProviderSessionFacts{Verification: v, SessionRef: w.SessionRef, AdapterVersion: w.AdapterVersion, ProfileDigest: w.ProfileDigest, WorkspaceID: w.WorkspaceID}
	_, e = providerSessionFactsToWire(f)
	return f, e
}
func expansionFactsToWire(f ExpansionFacts) (expansionFactsWire, error) {
	if err := validateControllerVerification(f.Verification); err != nil {
		return expansionFactsWire{}, err
	}
	if f.State == contextcompile.ResolutionProven {
		if f.Root != "" {
			if err := validateDigest("expansion root", f.Root); err != nil {
				return expansionFactsWire{}, err
			}
		}
	} else if f.Root != "" {
		return expansionFactsWire{}, fmt.Errorf("sealedexec: non-proven expansion facts cannot carry an installed root")
	}
	return expansionFactsWire{f.State, f.Failure, f.Witnesses, f.Root}, nil
}
func expansionFactsFromWire(w expansionFactsWire) (ExpansionFacts, error) {
	v, e := verificationFromWire(verificationWire{w.State, w.Failure, w.Witnesses})
	if e != nil {
		return ExpansionFacts{}, e
	}
	f := ExpansionFacts{v, w.Root}
	_, e = expansionFactsToWire(f)
	return f, e
}

func executionKeyToWire(k ExecutionKey) executionKeyWire {
	return executionKeyWire(k)
}
func executionKeyFromWire(w executionKeyWire) ExecutionKey {
	return ExecutionKey(w)
}
func validateExecutionKey(k ExecutionKey) error {
	for field, value := range map[string]string{"flight": k.Flight, "lane": k.Lane, "epoch": k.Epoch} {
		if err := requireText(field, value); err != nil {
			return err
		}
	}
	return nil
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
func canonicalControlAck(ack ControlAck) (json.RawMessage, error) {
	b, e := EncodeControlAck(ack)
	if e != nil {
		return nil, e
	}
	return trimFrame(b), nil
}

func sessionRecordToWire(r SessionRecord) (sessionRecordWire, error) {
	if err := validateExecutionKey(r.Key); err != nil {
		return sessionRecordWire{}, err
	}
	if err := validateProviderSessionCheck(ProviderSessionCheck{r.SessionRef, r.AdapterVersion, r.ProfileDigest, r.WorkspaceID}); err != nil {
		return sessionRecordWire{}, err
	}
	ack, err := canonicalEventAck(r.LifecycleAck)
	if err != nil {
		return sessionRecordWire{}, err
	}
	return sessionRecordWire{executionKeyToWire(r.Key), r.SessionRef, r.AdapterVersion, r.ProfileDigest, r.WorkspaceID, ack}, nil
}
func sessionRecordFromWire(w sessionRecordWire) (SessionRecord, error) {
	r := SessionRecord{executionKeyFromWire(w.Key), w.SessionRef, w.AdapterVersion, w.ProfileDigest, w.WorkspaceID, w.LifecycleAck}
	_, e := sessionRecordToWire(r)
	return r, e
}

func validateOpaqueEntries(rows []contextcompile.OpaqueEntry) error {
	if rows == nil {
		return fmt.Errorf("sealedexec: opaque rows must be non-null")
	}
	for i, row := range rows {
		if err := requireText("opaque id", row.ID); err != nil {
			return err
		}
		if row.Kind != contextcompile.OpaqueKindHarnessVendorBase {
			return fmt.Errorf("sealedexec: unknown opaque kind %q", row.Kind)
		}
		if err := requireText("opaque adapter id", row.Adapter.ID); err != nil {
			return err
		}
		if err := requireText("opaque adapter version", row.Adapter.Version); err != nil {
			return err
		}
		if row.Disclosures == nil {
			return fmt.Errorf("sealedexec: opaque disclosures must be non-null")
		}
		for j, d := range row.Disclosures {
			if d != contextcompile.DisclosureOpaqueHarnessVendorBase {
				return fmt.Errorf("sealedexec: unknown opaque disclosure %q", d)
			}
			if j > 0 && row.Disclosures[j-1] >= d {
				return fmt.Errorf("sealedexec: opaque disclosures must be sorted and deduplicated")
			}
		}
		if i > 0 && rows[i-1].ID >= row.ID {
			return fmt.Errorf("sealedexec: opaque rows must be ordered and deduplicated")
		}
	}
	return nil
}
func contextQueryToWire(q ContextQuery) contextQueryWire {
	return contextQueryWire{executionKeyToWire(q.Key), q.Ref}
}
func contextQueryFromWire(w contextQueryWire) ContextQuery {
	return ContextQuery{executionKeyFromWire(w.Key), w.Ref}
}
func validateContextQuery(q ContextQuery) error {
	if err := validateExecutionKey(q.Key); err != nil {
		return err
	}
	return requireText("context ref", q.Ref)
}

func contextResolutionToWire(r ContextResolution) (contextResolutionWire, error) {
	if err := validateControllerVerification(r.Verification); err != nil {
		return contextResolutionWire{}, err
	}
	if err := requireText("context resolution ref", r.Ref); err != nil {
		return contextResolutionWire{}, err
	}
	data, err := contextcompile.EncodeDataItem(r.Data)
	if err != nil {
		return contextResolutionWire{}, err
	}
	return contextResolutionWire{r.State, r.Failure, r.Witnesses, r.Ref, trimFrame(data)}, nil
}
func contextResolutionFromWire(w contextResolutionWire) (ContextResolution, error) {
	v, e := verificationFromWire(verificationWire{w.State, w.Failure, w.Witnesses})
	if e != nil {
		return ContextResolution{}, e
	}
	data, e := contextcompile.DecodeDataItem(frameNested(w.Data))
	if e != nil {
		return ContextResolution{}, e
	}
	r := ContextResolution{Verification: v, Ref: w.Ref, Data: data}
	_, e = contextResolutionToWire(r)
	return r, e
}

func epochCheckToWire(c EpochCheck) (epochCheckWire, error) {
	snapshot, err := flightSnapshotToWire(c.Snapshot)
	if err != nil {
		return epochCheckWire{}, err
	}
	resolution, err := contextResolutionToWire(c.Resolution)
	if err != nil {
		return epochCheckWire{}, err
	}
	return epochCheckWire{snapshot, resolution}, nil
}
func epochCheckFromWire(w epochCheckWire) (EpochCheck, error) {
	snapshot, e := flightSnapshotFromWire(w.Snapshot)
	if e != nil {
		return EpochCheck{}, e
	}
	resolution, e := contextResolutionFromWire(w.Resolution)
	if e != nil {
		return EpochCheck{}, e
	}
	return EpochCheck{snapshot, resolution}, nil
}
func flightSnapshotToWire(s FlightStateSnapshot) (flightStateSnapshotWire, error) {
	request, err := EncodeExecutionRequest(s.Request)
	if err != nil {
		return flightStateSnapshotWire{}, err
	}
	if err := validateExecutionKey(s.Key); err != nil {
		return flightStateSnapshotWire{}, err
	}
	for field, value := range map[string]string{"workspace_id": s.WorkspaceID} {
		if err := requireText(field, value); err != nil {
			return flightStateSnapshotWire{}, err
		}
	}
	for field, value := range map[string]string{"candidate_commit": s.CandidateCommit, "candidate_tree": s.CandidateTree} {
		if err := validateGitOID(field, value, false); err != nil {
			return flightStateSnapshotWire{}, err
		}
	}
	for field, value := range map[string]string{"manifest_digest": s.ManifestDigest, "projection_digest": s.ProjectionDigest} {
		if err := validateDigest(field, value); err != nil {
			return flightStateSnapshotWire{}, err
		}
	}
	if s.ExpansionRoot != "" {
		if err := validateDigest("expansion_root", s.ExpansionRoot); err != nil {
			return flightStateSnapshotWire{}, err
		}
	}
	if s.NextSourceSequence == 0 {
		return flightStateSnapshotWire{}, fmt.Errorf("sealedexec: next_source_sequence must be positive")
	}
	if s.PriorEventDigest != "" {
		if err := validateDigest("prior_event_digest", s.PriorEventDigest); err != nil {
			return flightStateSnapshotWire{}, err
		}
	}
	return flightStateSnapshotWire{trimFrame(request), executionKeyToWire(s.Key), s.WorkspaceID, s.CandidateCommit, s.CandidateTree, s.Revision, s.ManifestDigest, s.ProjectionDigest, s.ExpansionRoot, s.NextSourceSequence, s.PriorEventDigest, s.PriorRevision, s.LastGlobalSequence, s.Invalidated}, nil
}
func flightSnapshotFromWire(w flightStateSnapshotWire) (FlightStateSnapshot, error) {
	request, e := DecodeExecutionRequest(bytes.NewReader(frameNested(w.Request)))
	if e != nil {
		return FlightStateSnapshot{}, e
	}
	s := FlightStateSnapshot{Request: request, Key: executionKeyFromWire(w.Key), WorkspaceID: w.WorkspaceID, CandidateCommit: w.CandidateCommit, CandidateTree: w.CandidateTree, Revision: w.Revision, ManifestDigest: w.ManifestDigest, ProjectionDigest: w.ProjectionDigest, ExpansionRoot: w.ExpansionRoot, NextSourceSequence: w.NextSourceSequence, PriorEventDigest: w.PriorEventDigest, PriorRevision: w.PriorRevision, LastGlobalSequence: w.LastGlobalSequence, Invalidated: w.Invalidated}
	_, e = flightSnapshotToWire(s)
	return s, e
}

func expansionInstallToWire(i ExpansionInstall) (expansionInstallWire, error) {
	if err := validateExecutionKey(i.Key); err != nil {
		return expansionInstallWire{}, err
	}
	if err := requireText("request_id", i.RequestID); err != nil {
		return expansionInstallWire{}, err
	}
	if i.ChildRevision != i.ParentRevision+1 {
		return expansionInstallWire{}, fmt.Errorf("sealedexec: child revision must follow parent")
	}
	for field, value := range map[string]string{"parent_manifest_digest": i.ParentManifestDigest, "child_manifest_digest": i.ChildManifestDigest, "expansion_digest": i.ExpansionDigest, "expansion_root": i.ExpansionRoot} {
		if err := validateDigest(field, value); err != nil {
			return expansionInstallWire{}, err
		}
	}
	ack, err := canonicalEventAck(i.TerminalAck)
	if err != nil {
		return expansionInstallWire{}, err
	}
	return expansionInstallWire{executionKeyToWire(i.Key), i.RequestID, i.ParentRevision, i.ParentManifestDigest, i.ChildRevision, i.ChildManifestDigest, i.ExpansionDigest, i.ExpansionRoot, ack}, nil
}
func expansionInstallFromWire(w expansionInstallWire) (ExpansionInstall, error) {
	i := ExpansionInstall{executionKeyFromWire(w.Key), w.RequestID, w.ParentRevision, w.ParentManifestDigest, w.ChildRevision, w.ChildManifestDigest, w.ExpansionDigest, w.ExpansionRoot, w.TerminalAck}
	_, e := expansionInstallToWire(i)
	return i, e
}

func receiptInputsQueryToWire(q ReceiptInputsQuery) (receiptInputsQueryWire, error) {
	request, err := EncodeExecutionRequest(q.Request)
	if err != nil {
		return receiptInputsQueryWire{}, err
	}
	if err := requireText("workspace_id", q.WorkspaceID); err != nil {
		return receiptInputsQueryWire{}, err
	}
	for field, value := range map[string]string{"dispatch_digest": q.DispatchDigest, "event_chain_root": q.EventChainRoot, "result_facts_digest": q.ResultFactsDigest} {
		if err := validateDigest(field, value); err != nil {
			return receiptInputsQueryWire{}, err
		}
	}
	if q.TerminalSourceSequence == 0 || q.TerminalGlobalSequence == 0 {
		return receiptInputsQueryWire{}, fmt.Errorf("sealedexec: terminal sequences must be positive")
	}
	return receiptInputsQueryWire{trimFrame(request), q.WorkspaceID, q.DispatchDigest, q.TerminalRevision, q.TerminalSourceSequence, q.TerminalGlobalSequence, q.EventChainRoot, q.ResultFactsDigest}, nil
}
func receiptInputsQueryFromWire(w receiptInputsQueryWire) (ReceiptInputsQuery, error) {
	request, e := DecodeExecutionRequest(bytes.NewReader(frameNested(w.Request)))
	if e != nil {
		return ReceiptInputsQuery{}, e
	}
	q := ReceiptInputsQuery{request, w.WorkspaceID, w.DispatchDigest, w.TerminalRevision, w.TerminalSourceSequence, w.TerminalGlobalSequence, w.EventChainRoot, w.ResultFactsDigest}
	_, e = receiptInputsQueryToWire(q)
	return q, e
}

func receiptInputsToWire(i ReceiptInputs) (receiptInputsWire, error) {
	if i.Expansions == nil || i.Obligations == nil || i.Evidence == nil || i.ReviewInputs == nil {
		return receiptInputsWire{}, fmt.Errorf("sealedexec: receipt input arrays must be non-null")
	}
	if err := validateReceiptInputRows(i); err != nil {
		return receiptInputsWire{}, err
	}
	if err := validateControllerPrincipal(i.RunnerPrincipal); err != nil {
		return receiptInputsWire{}, err
	}
	return receiptInputsWire(i), nil
}
func receiptInputsFromWire(w receiptInputsWire) (ReceiptInputs, error) {
	i := ReceiptInputs(w)
	_, e := receiptInputsToWire(i)
	return i, e
}

func validateReceiptInputRows(i ReceiptInputs) error {
	for n, row := range i.Expansions {
		if err := requireText("expansion request_id", row.RequestID); err != nil {
			return err
		}
		if row.ChildRevision != row.ParentRevision+1 {
			return fmt.Errorf("sealedexec: expansion child revision gap")
		}
		for _, d := range []string{row.ParentManifestDigest, row.ChildManifestDigest, row.ExpansionDigest} {
			if err := validateDigest("expansion digest", d); err != nil {
				return err
			}
		}
		if n > 0 && !controllerExpansionLess(i.Expansions[n-1], row) {
			return fmt.Errorf("sealedexec: expansion rows must be sorted and deduplicated")
		}
	}
	for n, row := range i.Obligations {
		for _, value := range []string{row.Ref, row.Path, row.AC, row.Producer} {
			if err := requireText("obligation field", value); err != nil {
				return err
			}
		}
		switch row.Kind {
		case artifact.EvidenceStatic, artifact.EvidenceBehavioral, artifact.EvidenceRuntime, artifact.EvidenceAttestation:
		default:
			return fmt.Errorf("sealedexec: unknown obligation evidence kind %q", row.Kind)
		}
		if err := validateDigest("obligation content digest", row.ContentDigest); err != nil {
			return err
		}
		if n > 0 && !controllerObligationLess(i.Obligations[n-1], row) {
			return fmt.Errorf("sealedexec: obligation rows must be sorted and deduplicated")
		}
	}
	for n, row := range i.Evidence {
		if err := requireText("evidence command_id", row.CommandID); err != nil {
			return err
		}
		if len(row.Argv) == 0 {
			return fmt.Errorf("sealedexec: evidence argv must be nonempty")
		}
		for _, arg := range row.Argv {
			if err := requireText("evidence argv", arg); err != nil {
				return err
			}
		}
		switch row.Verdict {
		case countersign.VerdictProven, countersign.VerdictViolated, countersign.VerdictUnproven:
		default:
			return fmt.Errorf("sealedexec: unknown evidence verdict %q", row.Verdict)
		}
		if err := validateDigest("evidence output digest", row.OutputDigest); err != nil {
			return err
		}
		if n > 0 && !controllerEvidenceLess(i.Evidence[n-1], row) {
			return fmt.Errorf("sealedexec: evidence rows must be sorted and deduplicated")
		}
	}
	for n, row := range i.ReviewInputs {
		if err := requireText("review input kind", row.Kind); err != nil {
			return err
		}
		if err := validateDigest("review input digest", row.ContentDigest); err != nil {
			return err
		}
		if n > 0 && !controllerReviewInputLess(i.ReviewInputs[n-1], row) {
			return fmt.Errorf("sealedexec: review input rows must be sorted and deduplicated")
		}
	}
	return nil
}

func controllerExpansionLess(a, b contextreceipt.Expansion) bool {
	if a.RequestID != b.RequestID {
		return a.RequestID < b.RequestID
	}
	if a.ParentRevision != b.ParentRevision {
		return a.ParentRevision < b.ParentRevision
	}
	if a.ParentManifestDigest != b.ParentManifestDigest {
		return a.ParentManifestDigest < b.ParentManifestDigest
	}
	if a.ChildRevision != b.ChildRevision {
		return a.ChildRevision < b.ChildRevision
	}
	if a.ChildManifestDigest != b.ChildManifestDigest {
		return a.ChildManifestDigest < b.ChildManifestDigest
	}
	return a.ExpansionDigest < b.ExpansionDigest
}

func controllerObligationLess(a, b contextreceipt.Obligation) bool {
	return controllerCompareStrings(
		[]string{a.Ref, a.Path, a.AC, string(a.Kind), a.ContentDigest, a.Producer},
		[]string{b.Ref, b.Path, b.AC, string(b.Kind), b.ContentDigest, b.Producer},
	) < 0
}

func controllerEvidenceLess(a, b contextreceipt.Evidence) bool {
	if a.CommandID != b.CommandID {
		return a.CommandID < b.CommandID
	}
	if compared := controllerCompareStrings(a.Argv, b.Argv); compared != 0 {
		return compared < 0
	}
	if a.ExitCode != b.ExitCode {
		return a.ExitCode < b.ExitCode
	}
	if a.Verdict != b.Verdict {
		return a.Verdict < b.Verdict
	}
	return a.OutputDigest < b.OutputDigest
}

func controllerReviewInputLess(a, b contextreceipt.ReviewInput) bool {
	return controllerCompareStrings([]string{a.Kind, a.ContentDigest}, []string{b.Kind, b.ContentDigest}) < 0
}

func controllerCompareStrings(a, b []string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func validateControllerPrincipal(p gp.PrincipalResolution) error {
	if err := p.State.Validate(); err != nil {
		return err
	}
	if err := p.Claim.Validate(); err != nil {
		return err
	}
	derived, err := gp.CanonicalPrincipalID(p.Claim.TrustSource, p.Claim.Subject)
	if err != nil {
		return err
	}
	if p.State == gp.ResolutionAuthenticated {
		if p.PrincipalID != derived {
			return fmt.Errorf("sealedexec: authenticated principal id mismatch")
		}
	} else if p.PrincipalID != "" {
		return fmt.Errorf("sealedexec: non-authenticated principal carries id")
	}
	if len(p.Witnesses) == 0 {
		return fmt.Errorf("sealedexec: principal witnesses must be nonempty")
	}
	for n, w := range p.Witnesses {
		if err := requireText("principal witness code", w.Code); err != nil {
			return err
		}
		if err := requireText("principal witness source_id", w.SourceID); err != nil {
			return err
		}
		if w.EvidenceDigest != "" {
			if err := validateDigest("principal evidence digest", w.EvidenceDigest); err != nil {
				return err
			}
		}
		if n > 0 && controllerCompareStrings(
			[]string{p.Witnesses[n-1].Code, p.Witnesses[n-1].SourceID, p.Witnesses[n-1].EvidenceDigest, p.Witnesses[n-1].Detail},
			[]string{w.Code, w.SourceID, w.EvidenceDigest, w.Detail},
		) >= 0 {
			return fmt.Errorf("sealedexec: principal witnesses must be sorted and deduplicated")
		}
	}
	return nil
}

func receiptAppendToWire(a ReceiptAppend) (receiptAppendWire, error) {
	receipt, err := contextreceipt.EncodeReceipt(a.Receipt)
	if err != nil {
		return receiptAppendWire{}, err
	}
	event, err := contextevent.EncodeEvent(a.Event)
	if err != nil {
		return receiptAppendWire{}, err
	}
	if err := validateReceiptAppend(a, receipt); err != nil {
		return receiptAppendWire{}, err
	}
	return receiptAppendWire{trimFrame(receipt), trimFrame(event)}, nil
}
func receiptAppendFromWire(w receiptAppendWire) (ReceiptAppend, error) {
	receipt, e := contextreceipt.DecodeReceipt(bytes.NewReader(frameNested(w.Receipt)))
	if e != nil {
		return ReceiptAppend{}, e
	}
	event, e := contextevent.DecodeEvent(bytes.NewReader(frameNested(w.Event)))
	if e != nil {
		return ReceiptAppend{}, e
	}
	a := ReceiptAppend{receipt, event}
	_, e = receiptAppendToWire(a)
	return a, e
}
func validateReceiptAppend(a ReceiptAppend, receiptBytes []byte) error {
	if a.Event.Kind != contextevent.KindReceipt {
		return fmt.Errorf("sealedexec: append-receipt event kind must be receipt")
	}
	payload, ok := a.Event.Payload.(*contextevent.ReceiptPayload)
	if !ok {
		return fmt.Errorf("sealedexec: append-receipt payload type mismatch")
	}
	if payload.ReceiptDigest != a.Receipt.Digest || payload.ExecutionEventChainRoot != a.Receipt.EventChainRoot || payload.Role != a.Receipt.Role || payload.Authority != a.Receipt.Authority {
		return fmt.Errorf("sealedexec: append-receipt payload contradicts receipt")
	}
	if payload.Detail.Mode != contextevent.DetailInline || !bytes.Equal(payload.Detail.RedactedJSON, bytes.TrimSuffix(receiptBytes, []byte("\n"))) {
		return fmt.Errorf("sealedexec: append-receipt detail must carry exact canonical receipt bytes")
	}
	return nil
}

func validateControllerStamp(stamp string) error {
	if !strings.HasSuffix(stamp, "Z") {
		return fmt.Errorf("sealedexec: controller stamp must end in Z")
	}
	parsed, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return fmt.Errorf("sealedexec: invalid controller stamp: %w", err)
	}
	if parsed.Location() != time.UTC || parsed.Format(time.RFC3339Nano) != stamp {
		return fmt.Errorf("sealedexec: controller stamp is not normalized UTC RFC3339Nano")
	}
	return nil
}
