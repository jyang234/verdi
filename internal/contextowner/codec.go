package contextowner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

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

var canonicalDigestRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type callWire struct {
	Schema                  string          `json:"schema"`
	Operation               Operation       `json:"operation"`
	ControllerRequestDigest string          `json:"controller_request_digest"`
	Request                 json.RawMessage `json:"request"`
}

type replyWire struct {
	Schema string          `json:"schema"`
	Call   json.RawMessage `json:"call"`
	Result json.RawMessage `json:"result"`
}

// EncodeCall validates and canonically encodes one public owner call.
func EncodeCall(call Call) ([]byte, error) {
	if call.Schema != CallSchemaID {
		return nil, fmt.Errorf("contextowner: owner call schema must be %q", CallSchemaID)
	}
	if !validOperation(call.Operation) {
		return nil, fmt.Errorf("contextowner: unknown owner operation %q", call.Operation)
	}
	if err := validateDigest("controller_request_digest", call.ControllerRequestDigest); err != nil {
		return nil, err
	}
	request, err := encodeRequestArm(call)
	if err != nil {
		return nil, err
	}
	return canonjson.Marshal(callWire{
		Schema: call.Schema, Operation: call.Operation,
		ControllerRequestDigest: call.ControllerRequestDigest, Request: request,
	})
}

// DecodeCall strictly decodes one canonical public owner call.
func DecodeCall(reader io.Reader) (Call, error) {
	var wire callWire
	raw, err := decodeStrict(reader, &wire)
	if err != nil {
		return Call{}, fmt.Errorf("contextowner: decode owner call: %w", err)
	}
	if err := requireMembers(raw, "schema", "operation", "controller_request_digest", "request"); err != nil {
		return Call{}, err
	}
	if wire.Schema != CallSchemaID || !validOperation(wire.Operation) {
		return Call{}, fmt.Errorf("contextowner: invalid owner call envelope")
	}
	call := Call{Schema: wire.Schema, Operation: wire.Operation, ControllerRequestDigest: wire.ControllerRequestDigest}
	if err := decodeRequestArm(wire.Request, &call); err != nil {
		return Call{}, err
	}
	canonical, err := EncodeCall(call)
	if err != nil {
		return Call{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return Call{}, fmt.Errorf("contextowner: owner call is not byte-canonical")
	}
	return call, nil
}

// EncodeReply validates and canonically encodes one public owner reply. The
// nested call is the byte-exact canonical call the owner answered, and the
// result arm is selected by that call's operation: a result for another call
// is refused even when both operations have the same name.
func EncodeReply(reply Reply) ([]byte, error) {
	if reply.Schema != ReplySchemaID {
		return nil, fmt.Errorf("contextowner: owner reply schema must be %q", ReplySchemaID)
	}
	call, err := EncodeCall(reply.Call)
	if err != nil {
		return nil, err
	}
	result, err := encodeResultArm(reply)
	if err != nil {
		return nil, err
	}
	return canonjson.Marshal(replyWire{Schema: reply.Schema, Call: trimFrame(call), Result: result})
}

// DecodeReply strictly decodes one canonical public owner reply.
func DecodeReply(reader io.Reader) (Reply, error) {
	var wire replyWire
	raw, err := decodeStrict(reader, &wire)
	if err != nil {
		return Reply{}, fmt.Errorf("contextowner: decode owner reply: %w", err)
	}
	if err := requireMembers(raw, "schema", "call", "result"); err != nil {
		return Reply{}, err
	}
	if wire.Schema != ReplySchemaID {
		return Reply{}, fmt.Errorf("contextowner: invalid owner reply envelope")
	}
	// The nested call is decoded from its exact bytes, never from a
	// re-canonicalized copy, so a noncanonical nested call is refused here
	// rather than silently repaired.
	call, err := DecodeCall(bytes.NewReader(frameExact(wire.Call)))
	if err != nil {
		return Reply{}, err
	}
	reply := Reply{Schema: wire.Schema, Call: call}
	if err := decodeResultArm(wire.Result, &reply); err != nil {
		return Reply{}, err
	}
	canonical, err := EncodeReply(reply)
	if err != nil {
		return Reply{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return Reply{}, fmt.Errorf("contextowner: owner reply is not byte-canonical")
	}
	return reply, nil
}

func encodeRequestArm(call Call) (json.RawMessage, error) {
	schema := RequestSchema(call.Operation)
	switch call.Operation {
	case OperationVerifyAuthority:
		arm := call.VerifyAuthority
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		request, err := nestedDocument("verify-authority request", ExecutionRequestSchemaID, arm.Request)
		if err != nil {
			return nil, err
		}
		return marshalArm(VerifyAuthorityRequest{Schema: schema, Request: request})
	case OperationResolveProfile:
		arm := call.ResolveProfile
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		query, err := validProfileQuery(arm.Query)
		if err != nil {
			return nil, err
		}
		return marshalArm(ResolveProfileRequest{Schema: schema, Query: query})
	case OperationVerifyConflict:
		arm := call.VerifyConflict
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		report, err := nestedDocument("verify-conflict report", PolicyConflictReportSchemaID, arm.Report)
		if err != nil {
			return nil, err
		}
		return marshalArm(VerifyConflictRequest{Schema: schema, Report: report})
	case OperationResolveRecorder:
		arm := call.ResolveRecorder
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		if err := validateLogicalRef("recorder ref", arm.Ref, RecorderEndpointRefSchemaID); err != nil {
			return nil, err
		}
		return marshalArm(ResolveRecorderRequest{Schema: schema, Ref: arm.Ref})
	case OperationRecorderCheckpoint:
		arm := call.RecorderCheckpoint
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		if err := validateExecutionKey(arm.Key); err != nil {
			return nil, err
		}
		return marshalArm(RecorderCheckpointRequest{Schema: schema, Key: arm.Key})
	case OperationRecorderAppend:
		arm := call.RecorderAppend
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		event, err := nestedDocument("recorder-append event", EventSchemaID, arm.Event)
		if err != nil {
			return nil, err
		}
		return marshalArm(RecorderAppendRequest{Schema: schema, Event: event})
	case OperationStoreRedactedSegment:
		arm := call.StoreRedactedSegment
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		segment, err := validRedactedSegment(arm.Segment)
		if err != nil {
			return nil, err
		}
		return marshalArm(StoreRedactedSegmentRequest{Schema: schema, Segment: segment})
	case OperationResolveRedactedSegment:
		arm := call.ResolveRedactedSegment
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		if err := validateSegmentReference(arm.Reference); err != nil {
			return nil, err
		}
		return marshalArm(ResolveRedactedSegmentRequest{Schema: schema, Reference: arm.Reference})
	case OperationVerifyOpaqueBoundary:
		arm := call.VerifyOpaqueBoundary
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		if err := validateOpaqueEntries(arm.Rows); err != nil {
			return nil, err
		}
		return marshalArm(VerifyOpaqueBoundaryRequest{Schema: schema, Rows: copyOpaqueEntries(arm.Rows)})
	case OperationVerifyProviderSession:
		arm := call.VerifyProviderSession
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		if err := validateProviderSessionCheck(arm.Check); err != nil {
			return nil, err
		}
		return marshalArm(VerifyProviderSessionRequest{Schema: schema, Check: arm.Check})
	case OperationVerifyExpansion:
		arm := call.VerifyExpansion
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		if err := validateExecutionKey(arm.Key); err != nil {
			return nil, err
		}
		return marshalArm(VerifyExpansionRequest{Schema: schema, Key: arm.Key})
	case OperationStoreAdapterSession:
		arm := call.StoreAdapterSession
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		record, err := validSessionRecord(arm.Record)
		if err != nil {
			return nil, err
		}
		return marshalArm(StoreAdapterSessionRequest{Schema: schema, Record: record})
	case OperationNextStamp:
		arm := call.NextStamp
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		return marshalArm(NextStampRequest{Schema: schema})
	case OperationResolveContext:
		arm := call.ResolveContext
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		if err := validateContextQuery(arm.Query); err != nil {
			return nil, err
		}
		return marshalArm(ResolveContextRequest{Schema: schema, Query: arm.Query})
	case OperationVerifyEpoch:
		arm := call.VerifyEpoch
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		check, err := validEpochCheck(arm.Check)
		if err != nil {
			return nil, err
		}
		return marshalArm(VerifyEpochRequest{Schema: schema, Check: check})
	case OperationInstallExpansion:
		arm := call.InstallExpansion
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		install, err := validExpansionInstall(arm.Install)
		if err != nil {
			return nil, err
		}
		return marshalArm(InstallExpansionRequest{Schema: schema, Install: install})
	case OperationResolveReceiptInputs:
		arm := call.ResolveReceiptInputs
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		query, err := validReceiptInputsQuery(arm.Query)
		if err != nil {
			return nil, err
		}
		return marshalArm(ResolveReceiptInputsRequest{Schema: schema, Query: query})
	case OperationAppendReceipt:
		arm := call.AppendReceipt
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		appendValue, err := validReceiptAppend(arm.Append)
		if err != nil {
			return nil, err
		}
		return marshalArm(AppendReceiptRequest{Schema: schema, Append: appendValue})
	case OperationResolveReceiptVerificationAuthority:
		arm := call.ResolveReceiptVerificationAuthority
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		if err := validateReceiptVerificationAuthorityQuery(arm.Query); err != nil {
			return nil, err
		}
		return marshalArm(ResolveReceiptVerificationAuthorityRequest{Schema: schema, Query: arm.Query})
	case OperationPersistHandback:
		arm := call.PersistHandback
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		record, err := nestedDocument("persist-handback record", HandbackRecordSchemaID, arm.Record)
		if err != nil {
			return nil, err
		}
		return marshalArm(PersistHandbackRequest{Schema: schema, Record: record})
	case OperationPersistQuarantine:
		arm := call.PersistQuarantine
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		record, err := nestedDocument("persist-quarantine record", QuarantineRecordSchemaID, arm.Record)
		if err != nil {
			return nil, err
		}
		if arm.PreservedBytes == nil {
			return nil, fmt.Errorf("contextowner: quarantine preserved_bytes must be non-null")
		}
		if err := validateQuarantinePreservation(record, arm.PreservedBytes); err != nil {
			return nil, err
		}
		return marshalArm(PersistQuarantineRequest{
			Schema: schema, Record: record, PreservedBytes: append([]byte{}, arm.PreservedBytes...),
		})
	case OperationPersistAbort:
		arm := call.PersistAbort
		if err := requireOnlyRequestArm(call, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		record, err := nestedDocument("persist-abort record", AbortRecordSchemaID, arm.Record)
		if err != nil {
			return nil, err
		}
		return marshalArm(PersistAbortRequest{Schema: schema, Record: record})
	default:
		return nil, fmt.Errorf("contextowner: unknown owner operation %q", call.Operation)
	}
}

func decodeRequestArm(raw json.RawMessage, call *Call) error {
	schema := RequestSchema(call.Operation)
	switch call.Operation {
	case OperationVerifyAuthority:
		var arm VerifyAuthorityRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.VerifyAuthority = arm
	case OperationResolveProfile:
		var arm ResolveProfileRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.ResolveProfile = arm
	case OperationVerifyConflict:
		var arm VerifyConflictRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.VerifyConflict = arm
	case OperationResolveRecorder:
		var arm ResolveRecorderRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.ResolveRecorder = arm
	case OperationRecorderCheckpoint:
		var arm RecorderCheckpointRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.RecorderCheckpoint = arm
	case OperationRecorderAppend:
		var arm RecorderAppendRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.RecorderAppend = arm
	case OperationStoreRedactedSegment:
		var arm StoreRedactedSegmentRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.StoreRedactedSegment = arm
	case OperationResolveRedactedSegment:
		var arm ResolveRedactedSegmentRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.ResolveRedactedSegment = arm
	case OperationVerifyOpaqueBoundary:
		var arm VerifyOpaqueBoundaryRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.VerifyOpaqueBoundary = arm
	case OperationVerifyProviderSession:
		var arm VerifyProviderSessionRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.VerifyProviderSession = arm
	case OperationVerifyExpansion:
		var arm VerifyExpansionRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.VerifyExpansion = arm
	case OperationStoreAdapterSession:
		var arm StoreAdapterSessionRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.StoreAdapterSession = arm
	case OperationNextStamp:
		var arm NextStampRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.NextStamp = arm
	case OperationResolveContext:
		var arm ResolveContextRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.ResolveContext = arm
	case OperationVerifyEpoch:
		var arm VerifyEpochRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.VerifyEpoch = arm
	case OperationInstallExpansion:
		var arm InstallExpansionRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.InstallExpansion = arm
	case OperationResolveReceiptInputs:
		var arm ResolveReceiptInputsRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.ResolveReceiptInputs = arm
	case OperationAppendReceipt:
		var arm AppendReceiptRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.AppendReceipt = arm
	case OperationResolveReceiptVerificationAuthority:
		var arm ResolveReceiptVerificationAuthorityRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.ResolveReceiptVerificationAuthority = arm
	case OperationPersistHandback:
		var arm PersistHandbackRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.PersistHandback = arm
	case OperationPersistQuarantine:
		var arm PersistQuarantineRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.PersistQuarantine = arm
	case OperationPersistAbort:
		var arm PersistAbortRequest
		if err := unmarshalArm(raw, &arm, schema, call.Operation); err != nil {
			return err
		}
		call.PersistAbort = arm
	default:
		return fmt.Errorf("contextowner: unknown owner operation %q", call.Operation)
	}
	return nil
}

func encodeResultArm(reply Reply) (json.RawMessage, error) {
	operation := reply.Call.Operation
	schema := ResultSchema(operation)
	switch operation {
	case OperationVerifyAuthority:
		arm := reply.VerifyAuthority
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		facts, err := validAuthorityFacts(arm.Facts)
		if err != nil {
			return nil, err
		}
		return marshalArm(VerifyAuthorityResult{Schema: schema, Facts: facts})
	case OperationResolveProfile:
		arm := reply.ResolveProfile
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		if err := validateProfileMaterial(arm.Material); err != nil {
			return nil, err
		}
		return marshalArm(ResolveProfileResult{Schema: schema, Material: arm.Material})
	case OperationVerifyConflict:
		arm := reply.VerifyConflict
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		facts := arm.Facts
		if err := validateVerification("conflict facts", facts.State, facts.Failure, facts.Witnesses); err != nil {
			return nil, err
		}
		report, err := nestedDocument("conflict facts report", PolicyConflictReportSchemaID, facts.Report)
		if err != nil {
			return nil, err
		}
		facts.Report = report
		facts.Witnesses = copyTexts(facts.Witnesses)
		return marshalArm(VerifyConflictResult{Schema: schema, Facts: facts})
	case OperationResolveRecorder:
		arm := reply.ResolveRecorder
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		facts := arm.Facts
		if err := validateVerification("recorder facts", facts.State, facts.Failure, facts.Witnesses); err != nil {
			return nil, err
		}
		if err := validateLogicalRef("recorder facts ref", facts.Ref, RecorderEndpointRefSchemaID); err != nil {
			return nil, err
		}
		facts.Witnesses = copyTexts(facts.Witnesses)
		return marshalArm(ResolveRecorderResult{Schema: schema, Facts: facts})
	case OperationRecorderCheckpoint:
		arm := reply.RecorderCheckpoint
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		checkpoint, err := validRecorderCheckpoint(arm.Checkpoint)
		if err != nil {
			return nil, err
		}
		return marshalArm(RecorderCheckpointResult{Schema: schema, Checkpoint: checkpoint})
	case OperationRecorderAppend:
		arm := reply.RecorderAppend
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		ack, err := canonicalEventAck(arm.Ack)
		if err != nil {
			return nil, err
		}
		return marshalArm(RecorderAppendResult{Schema: schema, Ack: ack})
	case OperationStoreRedactedSegment:
		arm := reply.StoreRedactedSegment
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		if err := validateStoredSegment(arm.Stored); err != nil {
			return nil, err
		}
		return marshalArm(StoreRedactedSegmentResult{Schema: schema, Stored: arm.Stored})
	case OperationResolveRedactedSegment:
		arm := reply.ResolveRedactedSegment
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		segment, err := validRedactedSegment(arm.Segment)
		if err != nil {
			return nil, err
		}
		return marshalArm(ResolveRedactedSegmentResult{Schema: schema, Segment: segment})
	case OperationVerifyOpaqueBoundary:
		arm := reply.VerifyOpaqueBoundary
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		facts, err := validOpaqueBoundaryFacts(arm.Facts)
		if err != nil {
			return nil, err
		}
		return marshalArm(VerifyOpaqueBoundaryResult{Schema: schema, Facts: facts})
	case OperationVerifyProviderSession:
		arm := reply.VerifyProviderSession
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		facts := arm.Facts
		if err := validateVerification("provider session facts", facts.State, facts.Failure, facts.Witnesses); err != nil {
			return nil, err
		}
		if err := validateProviderSessionCheck(ProviderSessionCheck{
			SessionRef: facts.SessionRef, AdapterVersion: facts.AdapterVersion,
			ProfileDigest: facts.ProfileDigest, WorkspaceID: facts.WorkspaceID,
		}); err != nil {
			return nil, err
		}
		facts.Witnesses = copyTexts(facts.Witnesses)
		return marshalArm(VerifyProviderSessionResult{Schema: schema, Facts: facts})
	case OperationVerifyExpansion:
		arm := reply.VerifyExpansion
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		facts := arm.Facts
		if err := validateVerification("expansion facts", facts.State, facts.Failure, facts.Witnesses); err != nil {
			return nil, err
		}
		if facts.State == contextcompile.ResolutionProven {
			if facts.Root != "" {
				if err := validateDigest("expansion root", facts.Root); err != nil {
					return nil, err
				}
			}
		} else if facts.Root != "" {
			return nil, fmt.Errorf("contextowner: non-proven expansion facts cannot carry an installed root")
		}
		facts.Witnesses = copyTexts(facts.Witnesses)
		return marshalArm(VerifyExpansionResult{Schema: schema, Facts: facts})
	case OperationStoreAdapterSession:
		arm := reply.StoreAdapterSession
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		return marshalArm(StoreAdapterSessionResult{Schema: schema})
	case OperationNextStamp:
		arm := reply.NextStamp
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		if err := validateStamp(arm.Stamp); err != nil {
			return nil, err
		}
		return marshalArm(NextStampResult{Schema: schema, Stamp: arm.Stamp})
	case OperationResolveContext:
		arm := reply.ResolveContext
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		resolution, err := validContextResolution(arm.Resolution)
		if err != nil {
			return nil, err
		}
		return marshalArm(ResolveContextResult{Schema: schema, Resolution: resolution})
	case OperationVerifyEpoch:
		arm := reply.VerifyEpoch
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		verification := arm.Verification
		if err := validateVerification("verification", verification.State, verification.Failure, verification.Witnesses); err != nil {
			return nil, err
		}
		verification.Witnesses = copyTexts(verification.Witnesses)
		return marshalArm(VerifyEpochResult{Schema: schema, Verification: verification})
	case OperationInstallExpansion:
		arm := reply.InstallExpansion
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		return marshalArm(InstallExpansionResult{Schema: schema})
	case OperationResolveReceiptInputs:
		arm := reply.ResolveReceiptInputs
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		inputs, err := validReceiptInputs(arm.Inputs)
		if err != nil {
			return nil, err
		}
		return marshalArm(ResolveReceiptInputsResult{Schema: schema, Inputs: inputs})
	case OperationAppendReceipt:
		arm := reply.AppendReceipt
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		ack, err := canonicalReceiptAck(arm.Ack)
		if err != nil {
			return nil, err
		}
		return marshalArm(AppendReceiptResult{Schema: schema, Ack: ack})
	case OperationResolveReceiptVerificationAuthority:
		arm := reply.ResolveReceiptVerificationAuthority
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		authority, err := validReceiptVerificationAuthority(arm.Authority)
		if err != nil {
			return nil, err
		}
		return marshalArm(ResolveReceiptVerificationAuthorityResult{Schema: schema, Authority: authority})
	case OperationPersistHandback:
		arm := reply.PersistHandback
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		ack, err := nestedDocument("persist-handback ack", ControlAckSchemaID, arm.Ack)
		if err != nil {
			return nil, err
		}
		return marshalArm(PersistHandbackResult{Schema: schema, Ack: ack})
	case OperationPersistQuarantine:
		arm := reply.PersistQuarantine
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		ack, err := nestedDocument("persist-quarantine ack", ControlAckSchemaID, arm.Ack)
		if err != nil {
			return nil, err
		}
		return marshalArm(PersistQuarantineResult{Schema: schema, Ack: ack})
	case OperationPersistAbort:
		arm := reply.PersistAbort
		if err := requireOnlyResultArm(reply, arm, schema, arm.Schema); err != nil {
			return nil, err
		}
		ack, err := nestedDocument("persist-abort ack", ControlAckSchemaID, arm.Ack)
		if err != nil {
			return nil, err
		}
		return marshalArm(PersistAbortResult{Schema: schema, Ack: ack})
	default:
		return nil, fmt.Errorf("contextowner: unknown owner operation %q", operation)
	}
}

func decodeResultArm(raw json.RawMessage, reply *Reply) error {
	operation := reply.Call.Operation
	schema := ResultSchema(operation)
	switch operation {
	case OperationVerifyAuthority:
		var arm VerifyAuthorityResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.VerifyAuthority = arm
	case OperationResolveProfile:
		var arm ResolveProfileResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.ResolveProfile = arm
	case OperationVerifyConflict:
		var arm VerifyConflictResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.VerifyConflict = arm
	case OperationResolveRecorder:
		var arm ResolveRecorderResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.ResolveRecorder = arm
	case OperationRecorderCheckpoint:
		var arm RecorderCheckpointResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.RecorderCheckpoint = arm
	case OperationRecorderAppend:
		var arm RecorderAppendResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.RecorderAppend = arm
	case OperationStoreRedactedSegment:
		var arm StoreRedactedSegmentResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.StoreRedactedSegment = arm
	case OperationResolveRedactedSegment:
		var arm ResolveRedactedSegmentResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.ResolveRedactedSegment = arm
	case OperationVerifyOpaqueBoundary:
		var arm VerifyOpaqueBoundaryResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.VerifyOpaqueBoundary = arm
	case OperationVerifyProviderSession:
		var arm VerifyProviderSessionResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.VerifyProviderSession = arm
	case OperationVerifyExpansion:
		var arm VerifyExpansionResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.VerifyExpansion = arm
	case OperationStoreAdapterSession:
		var arm StoreAdapterSessionResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.StoreAdapterSession = arm
	case OperationNextStamp:
		var arm NextStampResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.NextStamp = arm
	case OperationResolveContext:
		var arm ResolveContextResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.ResolveContext = arm
	case OperationVerifyEpoch:
		var arm VerifyEpochResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.VerifyEpoch = arm
	case OperationInstallExpansion:
		var arm InstallExpansionResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.InstallExpansion = arm
	case OperationResolveReceiptInputs:
		var arm ResolveReceiptInputsResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.ResolveReceiptInputs = arm
	case OperationAppendReceipt:
		var arm AppendReceiptResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.AppendReceipt = arm
	case OperationResolveReceiptVerificationAuthority:
		var arm ResolveReceiptVerificationAuthorityResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.ResolveReceiptVerificationAuthority = arm
	case OperationPersistHandback:
		var arm PersistHandbackResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.PersistHandback = arm
	case OperationPersistQuarantine:
		var arm PersistQuarantineResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.PersistQuarantine = arm
	case OperationPersistAbort:
		var arm PersistAbortResult
		if err := unmarshalArm(raw, &arm, schema, operation); err != nil {
			return err
		}
		reply.PersistAbort = arm
	default:
		return fmt.Errorf("contextowner: unknown owner operation %q", operation)
	}
	return nil
}

// requireOnlyRequestArm proves the call carries exactly the operation's arm
// and that the arm declares its derived public schema.
func requireOnlyRequestArm(call Call, selected any, wantSchema, armSchema string) error {
	if armSchema != wantSchema {
		return armSchemaError(call.Operation, "request")
	}
	want := Call{Schema: call.Schema, Operation: call.Operation, ControllerRequestDigest: call.ControllerRequestDigest}
	if !assignRequestArm(&want, call.Operation, selected) {
		return fmt.Errorf("contextowner: unknown owner operation %q", call.Operation)
	}
	if !reflect.DeepEqual(call, want) {
		return fmt.Errorf("contextowner: owner call carries wrong or multiple request arms")
	}
	return nil
}

// requireOnlyResultArm proves the reply carries exactly the arm its own call
// selects, so a result for another call is never accepted.
func requireOnlyResultArm(reply Reply, selected any, wantSchema, armSchema string) error {
	if armSchema != wantSchema {
		return armSchemaError(reply.Call.Operation, "result")
	}
	want := Reply{Schema: reply.Schema, Call: reply.Call}
	if !assignResultArm(&want, reply.Call.Operation, selected) {
		return fmt.Errorf("contextowner: unknown owner operation %q", reply.Call.Operation)
	}
	if !reflect.DeepEqual(reply, want) {
		return fmt.Errorf("contextowner: owner reply carries wrong or multiple result arms")
	}
	return nil
}

func assignRequestArm(call *Call, operation Operation, selected any) bool {
	switch operation {
	case OperationVerifyAuthority:
		call.VerifyAuthority = selected.(VerifyAuthorityRequest)
	case OperationResolveProfile:
		call.ResolveProfile = selected.(ResolveProfileRequest)
	case OperationVerifyConflict:
		call.VerifyConflict = selected.(VerifyConflictRequest)
	case OperationResolveRecorder:
		call.ResolveRecorder = selected.(ResolveRecorderRequest)
	case OperationRecorderCheckpoint:
		call.RecorderCheckpoint = selected.(RecorderCheckpointRequest)
	case OperationRecorderAppend:
		call.RecorderAppend = selected.(RecorderAppendRequest)
	case OperationStoreRedactedSegment:
		call.StoreRedactedSegment = selected.(StoreRedactedSegmentRequest)
	case OperationResolveRedactedSegment:
		call.ResolveRedactedSegment = selected.(ResolveRedactedSegmentRequest)
	case OperationVerifyOpaqueBoundary:
		call.VerifyOpaqueBoundary = selected.(VerifyOpaqueBoundaryRequest)
	case OperationVerifyProviderSession:
		call.VerifyProviderSession = selected.(VerifyProviderSessionRequest)
	case OperationVerifyExpansion:
		call.VerifyExpansion = selected.(VerifyExpansionRequest)
	case OperationStoreAdapterSession:
		call.StoreAdapterSession = selected.(StoreAdapterSessionRequest)
	case OperationNextStamp:
		call.NextStamp = selected.(NextStampRequest)
	case OperationResolveContext:
		call.ResolveContext = selected.(ResolveContextRequest)
	case OperationVerifyEpoch:
		call.VerifyEpoch = selected.(VerifyEpochRequest)
	case OperationInstallExpansion:
		call.InstallExpansion = selected.(InstallExpansionRequest)
	case OperationResolveReceiptInputs:
		call.ResolveReceiptInputs = selected.(ResolveReceiptInputsRequest)
	case OperationAppendReceipt:
		call.AppendReceipt = selected.(AppendReceiptRequest)
	case OperationResolveReceiptVerificationAuthority:
		call.ResolveReceiptVerificationAuthority = selected.(ResolveReceiptVerificationAuthorityRequest)
	case OperationPersistHandback:
		call.PersistHandback = selected.(PersistHandbackRequest)
	case OperationPersistQuarantine:
		call.PersistQuarantine = selected.(PersistQuarantineRequest)
	case OperationPersistAbort:
		call.PersistAbort = selected.(PersistAbortRequest)
	default:
		return false
	}
	return true
}

func assignResultArm(reply *Reply, operation Operation, selected any) bool {
	switch operation {
	case OperationVerifyAuthority:
		reply.VerifyAuthority = selected.(VerifyAuthorityResult)
	case OperationResolveProfile:
		reply.ResolveProfile = selected.(ResolveProfileResult)
	case OperationVerifyConflict:
		reply.VerifyConflict = selected.(VerifyConflictResult)
	case OperationResolveRecorder:
		reply.ResolveRecorder = selected.(ResolveRecorderResult)
	case OperationRecorderCheckpoint:
		reply.RecorderCheckpoint = selected.(RecorderCheckpointResult)
	case OperationRecorderAppend:
		reply.RecorderAppend = selected.(RecorderAppendResult)
	case OperationStoreRedactedSegment:
		reply.StoreRedactedSegment = selected.(StoreRedactedSegmentResult)
	case OperationResolveRedactedSegment:
		reply.ResolveRedactedSegment = selected.(ResolveRedactedSegmentResult)
	case OperationVerifyOpaqueBoundary:
		reply.VerifyOpaqueBoundary = selected.(VerifyOpaqueBoundaryResult)
	case OperationVerifyProviderSession:
		reply.VerifyProviderSession = selected.(VerifyProviderSessionResult)
	case OperationVerifyExpansion:
		reply.VerifyExpansion = selected.(VerifyExpansionResult)
	case OperationStoreAdapterSession:
		reply.StoreAdapterSession = selected.(StoreAdapterSessionResult)
	case OperationNextStamp:
		reply.NextStamp = selected.(NextStampResult)
	case OperationResolveContext:
		reply.ResolveContext = selected.(ResolveContextResult)
	case OperationVerifyEpoch:
		reply.VerifyEpoch = selected.(VerifyEpochResult)
	case OperationInstallExpansion:
		reply.InstallExpansion = selected.(InstallExpansionResult)
	case OperationResolveReceiptInputs:
		reply.ResolveReceiptInputs = selected.(ResolveReceiptInputsResult)
	case OperationAppendReceipt:
		reply.AppendReceipt = selected.(AppendReceiptResult)
	case OperationResolveReceiptVerificationAuthority:
		reply.ResolveReceiptVerificationAuthority = selected.(ResolveReceiptVerificationAuthorityResult)
	case OperationPersistHandback:
		reply.PersistHandback = selected.(PersistHandbackResult)
	case OperationPersistQuarantine:
		reply.PersistQuarantine = selected.(PersistQuarantineResult)
	case OperationPersistAbort:
		reply.PersistAbort = selected.(PersistAbortResult)
	default:
		return false
	}
	return true
}

func armSchemaError(operation Operation, arm string) error {
	return fmt.Errorf("contextowner: owner %s %s schema mismatch", operation, arm)
}

// nestedDocument returns a copy of one exact standalone canonical Verdi
// document. The member must be present, canonical, declare the schema the
// accepted contract fixes for it, and — because the publication rule retains
// every nested value rule and cross-field validation unchanged (correction
// §3.3 step 3) — be a valid instance of that schema, not merely a document
// shaped like one.
func nestedDocument(field, schema string, raw json.RawMessage) (json.RawMessage, error) {
	document, members, err := canonicalNestedObject(field, raw)
	if err != nil {
		return nil, err
	}
	var declared string
	value, present := members["schema"]
	if !present || json.Unmarshal(value, &declared) != nil || declared != schema {
		return nil, fmt.Errorf("contextowner: %s must declare schema %q", field, schema)
	}
	if err := validateNestedInterior(field, schema, document); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), document...), nil
}

// nestedGrantSet returns a copy of one exact canonical execution grant set.
// The accepted private payload carries this operand as the grant-set document
// its own component owns, and that component decodes every value rule, so the
// published arm reuses it rather than restating it.
func nestedGrantSet(field string, raw json.RawMessage) (json.RawMessage, error) {
	document, _, err := canonicalNestedObject(field, raw)
	if err != nil {
		return nil, err
	}
	if _, err := execworkspace.DecodeGrantSet(frameExact(document)); err != nil {
		return nil, fmt.Errorf("contextowner: %s is not a canonical execution grant set: %w", field, err)
	}
	return append(json.RawMessage(nil), document...), nil
}

// canonicalNestedObject proves the member is one present, exact, canonical
// JSON object and returns both its bytes and its members.
func canonicalNestedObject(field string, raw json.RawMessage) ([]byte, map[string]json.RawMessage, error) {
	if rawMissing(raw) {
		return nil, nil, fmt.Errorf("contextowner: %s is absent or null", field)
	}
	canonical, err := canonjson.Marshal(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("contextowner: canonicalize %s: %w", field, err)
	}
	trimmed := bytes.TrimSuffix(canonical, []byte("\n"))
	if !bytes.Equal(raw, trimmed) {
		return nil, nil, fmt.Errorf("contextowner: %s is not one canonical JSON document", field)
	}
	var members map[string]json.RawMessage
	if err := artifact.DecodeExactJSON(raw, &members); err != nil {
		return nil, nil, fmt.Errorf("contextowner: %s must be one canonical JSON object: %w", field, err)
	}
	return trimmed, members, nil
}

func marshalArm(value any) (json.RawMessage, error) {
	raw, err := canonjson.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("contextowner: encode owner arm: %w", err)
	}
	return trimFrame(raw), nil
}

func unmarshalArm(raw json.RawMessage, target any, schema string, operation Operation) error {
	if rawMissing(raw) {
		return fmt.Errorf("contextowner: owner %s arm is absent or null", operation)
	}
	if err := artifact.DecodeExactJSON(raw, target); err != nil {
		return fmt.Errorf("contextowner: decode owner %s arm: %w", operation, err)
	}
	var declared struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(raw, &declared); err != nil {
		return fmt.Errorf("contextowner: decode owner %s arm schema: %w", operation, err)
	}
	if declared.Schema != schema {
		return fmt.Errorf("contextowner: owner %s arm schema mismatch", operation)
	}
	return nil
}

func decodeStrict(reader io.Reader, target any) ([]byte, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if err := artifact.DecodeExactJSON(raw, target); err != nil {
		return nil, err
	}
	return raw, nil
}

func requireMembers(raw []byte, fields ...string) error {
	var object map[string]json.RawMessage
	if err := artifact.DecodeExactJSON(raw, &object); err != nil {
		return fmt.Errorf("contextowner: owner document must be one JSON object: %w", err)
	}
	for _, field := range fields {
		value, present := object[field]
		if !present || rawMissing(value) {
			return fmt.Errorf("contextowner: %s is absent or null", field)
		}
	}
	return nil
}

func rawMissing(raw json.RawMessage) bool {
	return raw == nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func trimFrame(raw []byte) json.RawMessage {
	return append(json.RawMessage(nil), bytes.TrimSuffix(raw, []byte("\n"))...)
}

// frameExact re-frames a nested document as one standalone document without
// re-canonicalizing it, so the nested decoder judges the exact bytes.
func frameExact(raw json.RawMessage) []byte {
	return append(append([]byte(nil), raw...), '\n')
}

func validateDigest(field, value string) error {
	if !canonicalDigestRE.MatchString(value) {
		return fmt.Errorf("contextowner: %s must be a canonical sha256 digest", field)
	}
	return nil
}

func validateGitOID(field, value string, commit bool) error {
	wantLengths := []int{40, 64}
	if commit {
		wantLengths = []int{40}
	}
	validLength := false
	for _, length := range wantLengths {
		validLength = validLength || len(value) == length
	}
	if !validLength || value != strings.ToLower(value) {
		return fmt.Errorf("contextowner: %s must be a full lowercase hexadecimal Git object id", field)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return fmt.Errorf("contextowner: %s must be hexadecimal: %w", field, err)
	}
	return nil
}

func requireText(field, value string) error {
	if value == "" || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return fmt.Errorf("contextowner: %s must be nonempty UTF-8 without surrounding whitespace", field)
	}
	return nil
}

func validateSortedTexts(field string, values []string) error {
	for i, value := range values {
		if err := requireText(fmt.Sprintf("%s[%d]", field, i), value); err != nil {
			return err
		}
		for _, r := range value {
			if r < 0x20 || r == 0x7f {
				return fmt.Errorf("contextowner: %s[%d] contains a control character", field, i)
			}
		}
		if i > 0 && values[i-1] >= value {
			return fmt.Errorf("contextowner: %s must be sorted and deduplicated", field)
		}
	}
	return nil
}

// validateVerification publishes the accepted three-valued rule: a proven fact
// carries no adverse metadata and a non-proven fact carries both a failure
// code and at least one witness.
func validateVerification(name string, state contextcompile.Resolution, failure FailureCode, witnesses []string) error {
	if witnesses == nil {
		return fmt.Errorf("contextowner: %s witnesses must be non-null", name)
	}
	if err := validateSortedTexts(name+" witnesses", witnesses); err != nil {
		return err
	}
	switch state {
	case contextcompile.ResolutionProven:
		if failure != FailureNone || len(witnesses) != 0 {
			return fmt.Errorf("contextowner: proven %s carries adverse metadata", name)
		}
	case contextcompile.ResolutionViolatedWithWitness, contextcompile.ResolutionUnproven:
		if failure == FailureNone {
			return fmt.Errorf("contextowner: non-proven %s has no failure code", name)
		}
		if len(witnesses) == 0 {
			return fmt.Errorf("contextowner: non-proven %s requires witnesses", name)
		}
	default:
		return fmt.Errorf("contextowner: unknown %s state %q", name, state)
	}
	return nil
}

func validateLogicalRef(field string, ref LogicalRef, schema string) error {
	if ref.Schema != schema {
		return fmt.Errorf("contextowner: %s.schema must be %q", field, schema)
	}
	if err := requireText(field+".id", ref.ID); err != nil {
		return err
	}
	return validateDigest(field+".digest", ref.Digest)
}

func validateExecutionKey(key ExecutionKey) error {
	for _, row := range []struct{ field, value string }{
		{"flight", key.Flight}, {"lane", key.Lane}, {"epoch", key.Epoch},
	} {
		if err := requireText(row.field, row.value); err != nil {
			return err
		}
	}
	return nil
}

func validateAbsoluteCleanPath(field, value string) error {
	if !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return fmt.Errorf("contextowner: %s must be absolute and clean", field)
	}
	return nil
}

func cleanPathBelow(parent, child string) bool {
	if !filepath.IsAbs(child) || filepath.Clean(child) != child {
		return false
	}
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != "." && relative != ".." && !filepath.IsAbs(relative) &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func validAuthorityFacts(facts AuthorityFacts) (AuthorityFacts, error) {
	if err := validateVerification("authority facts", facts.State, facts.Failure, facts.Witnesses); err != nil {
		return AuthorityFacts{}, err
	}
	for _, row := range []struct{ field, value string }{
		{"manifest_digest", facts.ManifestDigest},
		{"projection_digest", facts.ProjectionDigest},
		{"authority_digest", facts.AuthorityDigest},
	} {
		if err := validateDigest(row.field, row.value); err != nil {
			return AuthorityFacts{}, err
		}
	}
	if err := validateGitOID("accepted_spec_commit", facts.AcceptedSpecCommit, false); err != nil {
		return AuthorityFacts{}, err
	}
	facts.Witnesses = copyTexts(facts.Witnesses)
	return facts, nil
}

func validProfileQuery(query ProfileQuery) (ProfileQuery, error) {
	if err := validateLogicalRef("profile query ref", query.Ref, ProfileRefSchemaID); err != nil {
		return ProfileQuery{}, err
	}
	if err := validateAbsoluteCleanPath("profile workspace_path", query.WorkspacePath); err != nil {
		return ProfileQuery{}, err
	}
	grants, err := nestedGrantSet("profile query grants", query.Grants)
	if err != nil {
		return ProfileQuery{}, err
	}
	query.Grants = grants
	return query, nil
}

func validateProfileMaterial(material ProfileMaterial) error {
	if err := validateLogicalRef("profile material ref", material.Ref, ProfileRefSchemaID); err != nil {
		return err
	}
	for _, row := range []struct{ field, value string }{
		{"name", material.Name},
		{"adapter_version", material.AdapterVersion},
		{"decoder_profile", material.DecoderProfile},
	} {
		if err := requireText(row.field, row.value); err != nil {
			return err
		}
	}
	for _, row := range []struct{ field, value string }{
		{"absolute_executable", material.AbsoluteExecutable},
		{"absolute_env_root", material.AbsoluteEnvRoot},
	} {
		if err := validateAbsoluteCleanPath(row.field, row.value); err != nil {
			return err
		}
	}
	claudeSelected := material.Model != "" || material.ClaudeConfigDir != ""
	switch {
	case material.AbsoluteCodexHome != "" && !claudeSelected:
		return validateAbsoluteCleanPath("absolute_codex_home", material.AbsoluteCodexHome)
	case material.AbsoluteCodexHome == "" && material.Model != "" && material.ClaudeConfigDir != "":
		if err := requireText("model", material.Model); err != nil {
			return err
		}
		if !cleanPathBelow(material.AbsoluteEnvRoot, material.ClaudeConfigDir) {
			return fmt.Errorf("contextowner: absolute_claude_config_dir must be a clean absolute child of absolute_env_root")
		}
		return nil
	default:
		return fmt.Errorf("contextowner: profile material must select exactly one Codex or Claude arm")
	}
}

func validRedactedSegment(segment RedactedSegment) (RedactedSegment, error) {
	if segment.Schema != RedactedSegmentSchemaID {
		return RedactedSegment{}, fmt.Errorf("contextowner: redacted segment schema must be %q", RedactedSegmentSchemaID)
	}
	if segment.MediaType != contextevent.MediaTypeJSON {
		return RedactedSegment{}, fmt.Errorf("contextowner: redacted segment media_type must be %q", contextevent.MediaTypeJSON)
	}
	if segment.RedactionProfile != contextevent.RedactionProfileStandard {
		return RedactedSegment{}, fmt.Errorf("contextowner: redacted segment redaction_profile must be %q", contextevent.RedactionProfileStandard)
	}
	if segment.Bytes == nil {
		return RedactedSegment{}, fmt.Errorf("contextowner: redacted segment bytes must be non-null")
	}
	if segment.ByteCount != uint64(len(segment.Bytes)) {
		return RedactedSegment{}, fmt.Errorf("contextowner: redacted segment byte_count does not match bytes")
	}
	if err := validateDigest("redacted segment digest", segment.Digest); err != nil {
		return RedactedSegment{}, err
	}
	if segment.Digest != digestBytes(segment.Bytes) {
		return RedactedSegment{}, fmt.Errorf("contextowner: redacted segment digest does not authenticate bytes")
	}
	var raw json.RawMessage
	if err := artifact.DecodeExactJSON(segment.Bytes, &raw); err != nil {
		return RedactedSegment{}, fmt.Errorf("contextowner: decode redacted segment bytes: %w", err)
	}
	canonical, err := canonjson.Marshal(raw)
	if err != nil {
		return RedactedSegment{}, fmt.Errorf("contextowner: canonicalize redacted segment bytes: %w", err)
	}
	if !bytes.Equal(segment.Bytes, bytes.TrimSuffix(canonical, []byte("\n"))) {
		return RedactedSegment{}, fmt.Errorf("contextowner: redacted segment bytes are not canonical JSON")
	}
	segment.Bytes = append([]byte{}, segment.Bytes...)
	return segment, nil
}

func validateStoredSegment(stored StoredSegment) error {
	if stored.Schema != StoredSegmentSchemaID {
		return fmt.Errorf("contextowner: stored segment schema must be %q", StoredSegmentSchemaID)
	}
	if stored.MediaType != contextevent.MediaTypeJSON {
		return fmt.Errorf("contextowner: stored segment media_type must be %q", contextevent.MediaTypeJSON)
	}
	if stored.RedactionProfile != contextevent.RedactionProfileStandard {
		return fmt.Errorf("contextowner: stored segment redaction_profile must be %q", contextevent.RedactionProfileStandard)
	}
	if stored.ByteCount == 0 {
		return fmt.Errorf("contextowner: stored segment byte_count must be positive")
	}
	if err := validateDigest("stored segment digest", stored.Digest); err != nil {
		return err
	}
	want, err := segmentReference(stored.Digest)
	if err != nil {
		return err
	}
	if stored.Reference != want {
		return fmt.Errorf("contextowner: stored segment reference does not match digest")
	}
	return nil
}

func segmentReference(digest string) (string, error) {
	if err := validateDigest("segment digest", digest); err != nil {
		return "", err
	}
	return SegmentReferencePrefix + strings.TrimPrefix(digest, "sha256:"), nil
}

func validateSegmentReference(reference string) error {
	if !strings.HasPrefix(reference, SegmentReferencePrefix) {
		return fmt.Errorf("contextowner: invalid owner segment reference")
	}
	digest := "sha256:" + strings.TrimPrefix(reference, SegmentReferencePrefix)
	want, err := segmentReference(digest)
	if err != nil || reference != want {
		return fmt.Errorf("contextowner: invalid owner segment reference")
	}
	return nil
}

func validRecorderCheckpoint(checkpoint RecorderCheckpoint) (RecorderCheckpoint, error) {
	if err := validateVerification("recorder checkpoint", checkpoint.State, checkpoint.Failure, checkpoint.Witnesses); err != nil {
		return RecorderCheckpoint{}, err
	}
	if err := validateDigest("recorder checkpoint digest", checkpoint.Digest); err != nil {
		return RecorderCheckpoint{}, err
	}
	if checkpoint.Revisions == nil {
		return RecorderCheckpoint{}, fmt.Errorf("contextowner: recorder checkpoint revisions must be non-null")
	}
	if len(checkpoint.Revisions) == 0 {
		if checkpoint.EventChainRoot != "" || checkpoint.TerminalSourceSequence != 0 || checkpoint.TerminalGlobalSequence != 0 {
			return RecorderCheckpoint{}, fmt.Errorf("contextowner: empty recorder checkpoint carries terminal facts")
		}
	} else {
		root, err := contextevent.EventChainRoot(checkpoint.Revisions)
		if err != nil {
			return RecorderCheckpoint{}, err
		}
		terminal := checkpoint.Revisions[len(checkpoint.Revisions)-1]
		if checkpoint.EventChainRoot != root ||
			checkpoint.TerminalSourceSequence != terminal.TerminalSourceSequence ||
			checkpoint.TerminalGlobalSequence != terminal.TerminalGlobalSequence {
			return RecorderCheckpoint{}, fmt.Errorf("contextowner: recorder checkpoint terminal facts mismatch")
		}
	}
	active, err := validActiveRevision(checkpoint.ActiveRevision, checkpoint.Revisions, checkpoint.TerminalGlobalSequence)
	if err != nil {
		return RecorderCheckpoint{}, err
	}
	checkpoint.ActiveRevision = active
	checkpoint.Witnesses = copyTexts(checkpoint.Witnesses)
	checkpoint.Revisions = append([]contextevent.Revision{}, checkpoint.Revisions...)
	return checkpoint, nil
}

func validActiveRevision(active *ActiveRevision, revisions []contextevent.Revision, terminalGlobal uint64) (*ActiveRevision, error) {
	if active == nil {
		return nil, nil
	}
	if err := validateDigest("active revision manifest digest", active.ManifestDigest); err != nil {
		return nil, err
	}
	if active.NextSourceSequence == 0 {
		return nil, fmt.Errorf("contextowner: active revision next_source_sequence must be positive")
	}
	if active.EventAcks == nil {
		return nil, fmt.Errorf("contextowner: active revision event_acks must be non-null")
	}
	if uint64(len(active.EventAcks)) != active.NextSourceSequence-1 {
		return nil, fmt.Errorf("contextowner: active revision event_acks do not cover the adjacent source prefix")
	}
	for i, ack := range active.EventAcks {
		canonical, err := canonicalEventAck(ack)
		if err != nil || canonical != ack {
			return nil, fmt.Errorf("contextowner: active revision event_acks[%d] is invalid", i)
		}
		if ack.ManifestRevision != active.Revision || ack.SourceSequence != uint64(i+1) {
			return nil, fmt.Errorf("contextowner: active revision event_acks[%d] contradicts revision or source order", i)
		}
		if i == 0 {
			if ack.GlobalSequence <= terminalGlobal {
				return nil, fmt.Errorf("contextowner: active revision event_acks do not advance beyond the complete checkpoint")
			}
			continue
		}
		prior := active.EventAcks[i-1]
		if ack.Flight != prior.Flight || ack.Lane != prior.Lane || ack.Epoch != prior.Epoch ||
			ack.Session != prior.Session || ack.GlobalSequence <= prior.GlobalSequence {
			return nil, fmt.Errorf("contextowner: active revision event_acks contradict execution identity or global order")
		}
	}
	if len(active.EventAcks) != 0 {
		final := active.EventAcks[len(active.EventAcks)-1]
		if final.SourceSequence != active.NextSourceSequence-1 ||
			final.EventDigest != active.PriorEventDigest ||
			final.GlobalSequence != active.LastGlobalSequence {
			return nil, fmt.Errorf("contextowner: active revision terminal acknowledgment facts mismatch")
		}
	}
	if active.LastGlobalSequence < terminalGlobal {
		return nil, fmt.Errorf("contextowner: active revision last_global_sequence precedes the complete checkpoint")
	}
	if len(revisions) != 0 && active.Revision != revisions[len(revisions)-1].ManifestRevision+1 {
		return nil, fmt.Errorf("contextowner: active revision does not immediately follow the complete checkpoint")
	}
	if err := validateActiveRevisionBridge(active, revisions, terminalGlobal); err != nil {
		return nil, err
	}
	copied := *active
	copied.EventAcks = append([]contextevent.EventAck{}, active.EventAcks...)
	if active.PriorRevision != nil {
		prior := *active.PriorRevision
		copied.PriorRevision = &prior
	}
	return &copied, nil
}

func validateActiveRevisionBridge(active *ActiveRevision, revisions []contextevent.Revision, terminalGlobal uint64) error {
	if active.NextSourceSequence != 1 {
		if err := validateDigest("active revision prior event digest", active.PriorEventDigest); err != nil {
			return err
		}
		if active.PriorRevision != nil {
			return fmt.Errorf("contextowner: later active source sequence cannot retain a prior-revision bridge")
		}
		if active.LastGlobalSequence <= terminalGlobal {
			return fmt.Errorf("contextowner: active events must advance beyond the complete checkpoint global sequence")
		}
		return nil
	}
	if active.PriorEventDigest != "" {
		return fmt.Errorf("contextowner: sequence-one active revision cannot carry a prior event digest")
	}
	if len(revisions) == 0 {
		if active.PriorRevision == nil {
			if active.LastGlobalSequence != terminalGlobal {
				return fmt.Errorf("contextowner: pristine active revision cannot advance the global sequence")
			}
			return nil
		}
		if err := validatePriorRevisionBridge(*active.PriorRevision); err != nil {
			return err
		}
		if active.Revision == 0 || active.PriorRevision.ManifestRevision != active.Revision-1 ||
			active.LastGlobalSequence != active.PriorRevision.TerminalGlobalSequence {
			return fmt.Errorf("contextowner: sequence-one active revision does not exactly bridge its omitted predecessor")
		}
		return nil
	}
	terminal := revisions[len(revisions)-1]
	want := contextevent.PriorRevision{
		ManifestRevision: terminal.ManifestRevision, ManifestDigest: terminal.ManifestDigest,
		EventRoot: terminal.EventRoot, TerminalSourceSequence: terminal.TerminalSourceSequence,
		TerminalGlobalSequence: terminal.TerminalGlobalSequence,
	}
	if active.PriorRevision == nil || *active.PriorRevision != want ||
		active.Revision != terminal.ManifestRevision+1 {
		return fmt.Errorf("contextowner: sequence-one active revision does not exactly bridge the complete checkpoint")
	}
	if active.LastGlobalSequence != terminalGlobal {
		return fmt.Errorf("contextowner: sequence-one active revision cannot advance beyond its predecessor")
	}
	return nil
}

func validatePriorRevisionBridge(bridge contextevent.PriorRevision) error {
	if err := validateDigest("active revision bridge manifest digest", bridge.ManifestDigest); err != nil {
		return err
	}
	if err := validateDigest("active revision bridge event root", bridge.EventRoot); err != nil {
		return err
	}
	if bridge.TerminalSourceSequence == 0 || bridge.TerminalGlobalSequence == 0 {
		return fmt.Errorf("contextowner: active revision bridge terminal sequences must be positive")
	}
	return nil
}

func validOpaqueBoundaryFacts(facts OpaqueBoundaryFacts) (OpaqueBoundaryFacts, error) {
	if err := validateVerification("opaque facts", facts.State, facts.Failure, facts.Witnesses); err != nil {
		return OpaqueBoundaryFacts{}, err
	}
	if facts.Rows == nil {
		return OpaqueBoundaryFacts{}, fmt.Errorf("contextowner: opaque facts rows must be non-null")
	}
	for i, row := range facts.Rows {
		for _, entry := range []struct{ field, value string }{
			{"id", row.ID}, {"kind", row.Kind},
			{"adapter_id", row.AdapterID}, {"adapter_version", row.AdapterVersion},
		} {
			if err := requireText(entry.field, entry.value); err != nil {
				return OpaqueBoundaryFacts{}, err
			}
		}
		if i > 0 && facts.Rows[i-1].ID >= row.ID {
			return OpaqueBoundaryFacts{}, fmt.Errorf("contextowner: opaque facts rows must be ordered and deduplicated")
		}
	}
	facts.Witnesses = copyTexts(facts.Witnesses)
	facts.Rows = append([]OpaqueIdentity{}, facts.Rows...)
	return facts, nil
}

func validateOpaqueEntries(rows []contextcompile.OpaqueEntry) error {
	if rows == nil {
		return fmt.Errorf("contextowner: opaque rows must be non-null")
	}
	for i, row := range rows {
		if err := requireText("opaque id", row.ID); err != nil {
			return err
		}
		if row.Kind != contextcompile.OpaqueKindHarnessVendorBase {
			return fmt.Errorf("contextowner: unknown opaque kind %q", row.Kind)
		}
		if err := requireText("opaque adapter id", row.Adapter.ID); err != nil {
			return err
		}
		if err := requireText("opaque adapter version", row.Adapter.Version); err != nil {
			return err
		}
		if row.Disclosures == nil {
			return fmt.Errorf("contextowner: opaque disclosures must be non-null")
		}
		for j, disclosure := range row.Disclosures {
			if disclosure != contextcompile.DisclosureOpaqueHarnessVendorBase {
				return fmt.Errorf("contextowner: unknown opaque disclosure %q", disclosure)
			}
			if j > 0 && row.Disclosures[j-1] >= disclosure {
				return fmt.Errorf("contextowner: opaque disclosures must be sorted and deduplicated")
			}
		}
		if i > 0 && rows[i-1].ID >= row.ID {
			return fmt.Errorf("contextowner: opaque rows must be ordered and deduplicated")
		}
	}
	return nil
}

func copyOpaqueEntries(rows []contextcompile.OpaqueEntry) []contextcompile.OpaqueEntry {
	copied := make([]contextcompile.OpaqueEntry, len(rows))
	for i, row := range rows {
		row.Disclosures = append([]contextcompile.DisclosureCode{}, row.Disclosures...)
		copied[i] = row
	}
	return copied
}

func validateProviderSessionCheck(check ProviderSessionCheck) error {
	for _, row := range []struct{ field, value string }{
		{"session_ref", check.SessionRef},
		{"adapter_version", check.AdapterVersion},
		{"workspace_id", check.WorkspaceID},
	} {
		if err := requireText(row.field, row.value); err != nil {
			return err
		}
	}
	return validateDigest("profile_digest", check.ProfileDigest)
}

func validSessionRecord(record SessionRecord) (SessionRecord, error) {
	if err := validateExecutionKey(record.Key); err != nil {
		return SessionRecord{}, err
	}
	if err := validateProviderSessionCheck(ProviderSessionCheck{
		SessionRef: record.SessionRef, AdapterVersion: record.AdapterVersion,
		ProfileDigest: record.ProfileDigest, WorkspaceID: record.WorkspaceID,
	}); err != nil {
		return SessionRecord{}, err
	}
	ack, err := canonicalEventAck(record.LifecycleAck)
	if err != nil {
		return SessionRecord{}, err
	}
	record.LifecycleAck = ack
	return record, nil
}

func validateContextQuery(query ContextQuery) error {
	if err := validateExecutionKey(query.Key); err != nil {
		return err
	}
	return requireText("context ref", query.Ref)
}

func validContextResolution(resolution ContextResolution) (ContextResolution, error) {
	if err := validateVerification("context resolution", resolution.State, resolution.Failure, resolution.Witnesses); err != nil {
		return ContextResolution{}, err
	}
	if err := requireText("context resolution ref", resolution.Ref); err != nil {
		return ContextResolution{}, err
	}
	data, err := nestedDocument("context resolution data", DataItemSchemaID, resolution.Data)
	if err != nil {
		return ContextResolution{}, err
	}
	resolution.Data = data
	resolution.Witnesses = copyTexts(resolution.Witnesses)
	return resolution, nil
}

func validEpochCheck(check EpochCheck) (EpochCheck, error) {
	snapshot, err := validFlightStateSnapshot(check.Snapshot)
	if err != nil {
		return EpochCheck{}, err
	}
	resolution, err := validContextResolution(check.Resolution)
	if err != nil {
		return EpochCheck{}, err
	}
	return EpochCheck{Snapshot: snapshot, Resolution: resolution}, nil
}

func validFlightStateSnapshot(snapshot FlightStateSnapshot) (FlightStateSnapshot, error) {
	request, err := nestedDocument("epoch snapshot request", ExecutionRequestSchemaID, snapshot.Request)
	if err != nil {
		return FlightStateSnapshot{}, err
	}
	if err := validateExecutionKey(snapshot.Key); err != nil {
		return FlightStateSnapshot{}, err
	}
	if err := requireText("workspace_id", snapshot.WorkspaceID); err != nil {
		return FlightStateSnapshot{}, err
	}
	for _, row := range []struct{ field, value string }{
		{"candidate_commit", snapshot.CandidateCommit}, {"candidate_tree", snapshot.CandidateTree},
	} {
		if err := validateGitOID(row.field, row.value, false); err != nil {
			return FlightStateSnapshot{}, err
		}
	}
	for _, row := range []struct{ field, value string }{
		{"manifest_digest", snapshot.ManifestDigest}, {"projection_digest", snapshot.ProjectionDigest},
	} {
		if err := validateDigest(row.field, row.value); err != nil {
			return FlightStateSnapshot{}, err
		}
	}
	if snapshot.ExpansionRoot != "" {
		if err := validateDigest("expansion_root", snapshot.ExpansionRoot); err != nil {
			return FlightStateSnapshot{}, err
		}
	}
	if snapshot.NextSourceSequence == 0 {
		return FlightStateSnapshot{}, fmt.Errorf("contextowner: next_source_sequence must be positive")
	}
	if snapshot.PriorEventDigest != "" {
		if err := validateDigest("prior_event_digest", snapshot.PriorEventDigest); err != nil {
			return FlightStateSnapshot{}, err
		}
	}
	snapshot.Request = request
	if snapshot.PriorRevision != nil {
		prior := *snapshot.PriorRevision
		snapshot.PriorRevision = &prior
	}
	return snapshot, nil
}

func validExpansionInstall(install ExpansionInstall) (ExpansionInstall, error) {
	if err := validateExecutionKey(install.Key); err != nil {
		return ExpansionInstall{}, err
	}
	for _, row := range []struct{ field, value string }{
		{"request_id", install.RequestID},
		{"install ref", install.Ref},
		{"install purpose", install.Purpose},
	} {
		if err := requireText(row.field, row.value); err != nil {
			return ExpansionInstall{}, err
		}
	}
	if install.ChildRevision != install.ParentRevision+1 {
		return ExpansionInstall{}, fmt.Errorf("contextowner: child revision must follow parent")
	}
	for _, row := range []struct{ field, value string }{
		{"parent_manifest_digest", install.ParentManifestDigest},
		{"child_manifest_digest", install.ChildManifestDigest},
		{"expansion_digest", install.ExpansionDigest},
		{"expansion_root", install.ExpansionRoot},
	} {
		if err := validateDigest(row.field, row.value); err != nil {
			return ExpansionInstall{}, err
		}
	}
	data, err := nestedDocument("installed data item", DataItemSchemaID, install.Data)
	if err != nil {
		return ExpansionInstall{}, err
	}
	// The row ref stays a separate operand because the published data-item
	// grammar permits an item to omit its own ref. When the item carries one,
	// the two must agree, or the row would name a context the item does not.
	item, err := contextcompile.DecodeDataItem(frameExact(data))
	if err != nil {
		return ExpansionInstall{}, fmt.Errorf("contextowner: installed data item is not a valid %s document: %w", DataItemSchemaID, err)
	}
	if item.Ref != nil && *item.Ref != install.Ref {
		return ExpansionInstall{}, fmt.Errorf("contextowner: installed data item ref does not match the install ref")
	}
	install.Data = data
	ack, err := canonicalEventAck(install.TerminalAck)
	if err != nil {
		return ExpansionInstall{}, err
	}
	install.TerminalAck = ack
	return install, nil
}

func validReceiptInputsQuery(query ReceiptInputsQuery) (ReceiptInputsQuery, error) {
	request, err := nestedDocument("receipt inputs request", ExecutionRequestSchemaID, query.Request)
	if err != nil {
		return ReceiptInputsQuery{}, err
	}
	if err := requireText("workspace_id", query.WorkspaceID); err != nil {
		return ReceiptInputsQuery{}, err
	}
	for _, row := range []struct{ field, value string }{
		{"dispatch_digest", query.DispatchDigest},
		{"event_chain_root", query.EventChainRoot},
		{"result_facts_digest", query.ResultFactsDigest},
	} {
		if err := validateDigest(row.field, row.value); err != nil {
			return ReceiptInputsQuery{}, err
		}
	}
	if query.TerminalSourceSequence == 0 || query.TerminalGlobalSequence == 0 {
		return ReceiptInputsQuery{}, fmt.Errorf("contextowner: terminal sequences must be positive")
	}
	query.Request = request
	return query, nil
}

func validReceiptAppend(value ReceiptAppend) (ReceiptAppend, error) {
	receipt, err := nestedDocument("append-receipt receipt", ReceiptSchemaID, value.Receipt)
	if err != nil {
		return ReceiptAppend{}, err
	}
	event, err := nestedDocument("append-receipt event", EventSchemaID, value.Event)
	if err != nil {
		return ReceiptAppend{}, err
	}
	if err := validateReceiptAppendBinding(receipt, event); err != nil {
		return ReceiptAppend{}, err
	}
	return ReceiptAppend{Receipt: receipt, Event: event}, nil
}

// validateReceiptAppendBinding publishes the accepted atomic-append rule: the
// event is the receipt's own receipt event, its payload repeats the receipt's
// identity, and its inline detail carries and authenticates the exact
// canonical receipt bytes. Both documents are already valid instances of their
// own schemas here; this is the cross-field relation between them.
func validateReceiptAppendBinding(receiptDocument, eventDocument json.RawMessage) error {
	receipt, err := contextreceipt.DecodeReceipt(bytes.NewReader(frameExact(receiptDocument)))
	if err != nil {
		return fmt.Errorf("contextowner: append-receipt receipt: %w", err)
	}
	event, err := contextevent.DecodeEvent(bytes.NewReader(frameExact(eventDocument)))
	if err != nil {
		return fmt.Errorf("contextowner: append-receipt event: %w", err)
	}
	if event.Kind != contextevent.KindReceipt {
		return fmt.Errorf("contextowner: append-receipt event kind must be %q", contextevent.KindReceipt)
	}
	payload, ok := event.Payload.(*contextevent.ReceiptPayload)
	if !ok {
		return fmt.Errorf("contextowner: append-receipt payload type mismatch")
	}
	if payload.ReceiptDigest != receipt.Digest || payload.ExecutionEventChainRoot != receipt.EventChainRoot ||
		payload.Role != receipt.Role || payload.Authority != receipt.Authority {
		return fmt.Errorf("contextowner: append-receipt payload contradicts receipt")
	}
	if payload.Detail.Mode != contextevent.DetailInline || !bytes.Equal(payload.Detail.RedactedJSON, receiptDocument) {
		return fmt.Errorf("contextowner: append-receipt detail must carry exact canonical receipt bytes")
	}
	if payload.Detail.Digest != digestBytes(payload.Detail.RedactedJSON) {
		return fmt.Errorf("contextowner: append-receipt detail digest does not authenticate exact carried receipt bytes")
	}
	return nil
}

func validReceiptInputs(inputs ReceiptInputs) (ReceiptInputs, error) {
	if inputs.Expansions == nil || inputs.Obligations == nil || inputs.Evidence == nil || inputs.ReviewInputs == nil {
		return ReceiptInputs{}, fmt.Errorf("contextowner: receipt input arrays must be non-null")
	}
	for i, row := range inputs.Expansions {
		if err := requireText("expansion request_id", row.RequestID); err != nil {
			return ReceiptInputs{}, err
		}
		if row.ChildRevision != row.ParentRevision+1 {
			return ReceiptInputs{}, fmt.Errorf("contextowner: expansion child revision gap")
		}
		for _, digest := range []string{row.ParentManifestDigest, row.ChildManifestDigest, row.ExpansionDigest} {
			if err := validateDigest("expansion digest", digest); err != nil {
				return ReceiptInputs{}, err
			}
		}
		if i > 0 && !expansionLess(inputs.Expansions[i-1], row) {
			return ReceiptInputs{}, fmt.Errorf("contextowner: expansion rows must be sorted and deduplicated")
		}
	}
	for i, row := range inputs.Obligations {
		for _, value := range []string{row.Ref, row.Path, row.AC, row.Producer} {
			if err := requireText("obligation field", value); err != nil {
				return ReceiptInputs{}, err
			}
		}
		switch row.Kind {
		case artifact.EvidenceStatic, artifact.EvidenceBehavioral, artifact.EvidenceRuntime, artifact.EvidenceAttestation:
		default:
			return ReceiptInputs{}, fmt.Errorf("contextowner: unknown obligation evidence kind %q", row.Kind)
		}
		if err := validateDigest("obligation content digest", row.ContentDigest); err != nil {
			return ReceiptInputs{}, err
		}
		if i > 0 && !obligationLess(inputs.Obligations[i-1], row) {
			return ReceiptInputs{}, fmt.Errorf("contextowner: obligation rows must be sorted and deduplicated")
		}
	}
	for i, row := range inputs.Evidence {
		if err := requireText("evidence command_id", row.CommandID); err != nil {
			return ReceiptInputs{}, err
		}
		if len(row.Argv) == 0 {
			return ReceiptInputs{}, fmt.Errorf("contextowner: evidence argv must be nonempty")
		}
		for _, argument := range row.Argv {
			if err := requireText("evidence argv", argument); err != nil {
				return ReceiptInputs{}, err
			}
		}
		switch row.Verdict {
		case countersign.VerdictProven, countersign.VerdictViolated, countersign.VerdictUnproven:
		default:
			return ReceiptInputs{}, fmt.Errorf("contextowner: unknown evidence verdict %q", row.Verdict)
		}
		if err := validateDigest("evidence output digest", row.OutputDigest); err != nil {
			return ReceiptInputs{}, err
		}
		if i > 0 && !evidenceLess(inputs.Evidence[i-1], row) {
			return ReceiptInputs{}, fmt.Errorf("contextowner: evidence rows must be sorted and deduplicated")
		}
	}
	for i, row := range inputs.ReviewInputs {
		if err := requireText("review input kind", row.Kind); err != nil {
			return ReceiptInputs{}, err
		}
		if err := validateDigest("review input digest", row.ContentDigest); err != nil {
			return ReceiptInputs{}, err
		}
		if i > 0 && !reviewInputLess(inputs.ReviewInputs[i-1], row) {
			return ReceiptInputs{}, fmt.Errorf("contextowner: review input rows must be sorted and deduplicated")
		}
	}
	if err := validatePrincipal(inputs.RunnerPrincipal); err != nil {
		return ReceiptInputs{}, err
	}
	copied := ReceiptInputs{
		Expansions:   append([]contextreceipt.Expansion{}, inputs.Expansions...),
		Obligations:  append([]contextreceipt.Obligation{}, inputs.Obligations...),
		ReviewInputs: append([]contextreceipt.ReviewInput{}, inputs.ReviewInputs...),
	}
	copied.Evidence = make([]contextreceipt.Evidence, len(inputs.Evidence))
	for i, row := range inputs.Evidence {
		row.Argv = append([]string{}, row.Argv...)
		copied.Evidence[i] = row
	}
	copied.RunnerPrincipal = inputs.RunnerPrincipal
	copied.RunnerPrincipal.Witnesses = append([]gp.Witness{}, inputs.RunnerPrincipal.Witnesses...)
	return copied, nil
}

func expansionLess(a, b contextreceipt.Expansion) bool {
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

func obligationLess(a, b contextreceipt.Obligation) bool {
	return compareTexts(
		[]string{a.Ref, a.Path, a.AC, string(a.Kind), a.ContentDigest, a.Producer},
		[]string{b.Ref, b.Path, b.AC, string(b.Kind), b.ContentDigest, b.Producer},
	) < 0
}

func evidenceLess(a, b contextreceipt.Evidence) bool {
	if a.CommandID != b.CommandID {
		return a.CommandID < b.CommandID
	}
	if compared := compareTexts(a.Argv, b.Argv); compared != 0 {
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

func reviewInputLess(a, b contextreceipt.ReviewInput) bool {
	return compareTexts([]string{a.Kind, a.ContentDigest}, []string{b.Kind, b.ContentDigest}) < 0
}

func compareTexts(a, b []string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}

func validatePrincipal(principal gp.PrincipalResolution) error {
	if err := principal.State.Validate(); err != nil {
		return err
	}
	if err := principal.Claim.Validate(); err != nil {
		return err
	}
	derived, err := gp.CanonicalPrincipalID(principal.Claim.TrustSource, principal.Claim.Subject)
	if err != nil {
		return err
	}
	if principal.State == gp.ResolutionAuthenticated {
		if principal.PrincipalID != derived {
			return fmt.Errorf("contextowner: authenticated principal id mismatch")
		}
	} else if principal.PrincipalID != "" {
		return fmt.Errorf("contextowner: non-authenticated principal carries id")
	}
	if len(principal.Witnesses) == 0 {
		return fmt.Errorf("contextowner: principal witnesses must be nonempty")
	}
	return validateWitnessRows("principal", principal.Witnesses)
}

func validateWitnessRows(name string, witnesses []gp.Witness) error {
	if witnesses == nil {
		return fmt.Errorf("contextowner: %s witnesses must be non-null", name)
	}
	for i, witness := range witnesses {
		if err := requireText(name+" witness code", witness.Code); err != nil {
			return err
		}
		if err := requireText(name+" witness source_id", witness.SourceID); err != nil {
			return err
		}
		if witness.EvidenceDigest != "" {
			if err := validateDigest(name+" witness evidence_digest", witness.EvidenceDigest); err != nil {
				return err
			}
		}
		if i > 0 && compareTexts(
			[]string{witnesses[i-1].Code, witnesses[i-1].SourceID, witnesses[i-1].EvidenceDigest, witnesses[i-1].Detail},
			[]string{witness.Code, witness.SourceID, witness.EvidenceDigest, witness.Detail},
		) >= 0 {
			return fmt.Errorf("contextowner: %s witnesses must be sorted and deduplicated", name)
		}
	}
	return nil
}

func validateReceiptVerificationAuthorityQuery(query contextreceipt.AuthorityQuery) error {
	for _, row := range []struct{ field, value string }{
		{"receipt verification request digest", query.RequestDigest},
		{"receipt verification receipt digest", query.ReceiptDigest},
		{"receipt verification profile digest", query.ProfileRef.Digest},
	} {
		if err := validateDigest(row.field, row.value); err != nil {
			return err
		}
	}
	for _, row := range []struct{ field, value string }{
		{"receipt verification candidate commit", query.CandidateCommit},
		{"receipt verification candidate tree", query.CandidateTree},
	} {
		if len(row.value) != 40 {
			return fmt.Errorf("contextowner: %s must be a SHA-1 Git object", row.field)
		}
		if err := validateGitOID(row.field, row.value, true); err != nil {
			return err
		}
	}
	if query.ProfileRef.Schema != ProfileRefSchemaID {
		return fmt.Errorf("contextowner: receipt verification profile_ref schema must be %q", ProfileRefSchemaID)
	}
	if err := gp.ValidateID(query.ProfileRef.ID); err != nil {
		return fmt.Errorf("contextowner: receipt verification profile_ref id: %w", err)
	}
	if err := query.RunnerClaim.Validate(); err != nil {
		return fmt.Errorf("contextowner: receipt verification runner claim: %w", err)
	}
	return nil
}

func validReceiptVerificationAuthority(authority ReceiptVerificationAuthority) (ReceiptVerificationAuthority, error) {
	if authority.Profile.ProfileBytes == nil {
		return ReceiptVerificationAuthority{}, fmt.Errorf("contextowner: receipt verification profile_bytes must be non-null")
	}
	if err := validateReceiptAuthorityState("profile", authority.Profile.State,
		len(authority.Profile.ProfileBytes) != 0, authority.Profile.Witnesses); err != nil {
		return ReceiptVerificationAuthority{}, err
	}
	if err := validateReceiptTrustFact(authority.TrustFact); err != nil {
		return ReceiptVerificationAuthority{}, err
	}
	isolationPresent := authority.Isolation.ProfileID != "" || authority.Isolation.ProfileDigest != "" ||
		authority.Isolation.Session != "" || authority.Isolation.WorkspaceID != ""
	if err := validateReceiptAuthorityState("isolation", authority.Isolation.State,
		isolationPresent, authority.Isolation.Witnesses); err != nil {
		return ReceiptVerificationAuthority{}, err
	}
	if authority.Isolation.State == contextreceipt.StateProven {
		if err := gp.ValidateID(authority.Isolation.ProfileID); err != nil {
			return ReceiptVerificationAuthority{}, fmt.Errorf("contextowner: receipt verification isolation profile_id: %w", err)
		}
		if err := validateDigest("receipt verification isolation profile_digest", authority.Isolation.ProfileDigest); err != nil {
			return ReceiptVerificationAuthority{}, err
		}
		for _, row := range []struct{ field, value string }{
			{"session", authority.Isolation.Session}, {"workspace_id", authority.Isolation.WorkspaceID},
		} {
			if err := requireText("receipt verification isolation "+row.field, row.value); err != nil {
				return ReceiptVerificationAuthority{}, err
			}
		}
	} else if isolationPresent {
		return ReceiptVerificationAuthority{}, fmt.Errorf("contextowner: non-proven receipt verification isolation carries identity fields")
	}
	if err := validateReceiptPersistence(authority.Persistence); err != nil {
		return ReceiptVerificationAuthority{}, err
	}
	authority.Profile.ProfileBytes = append([]byte{}, authority.Profile.ProfileBytes...)
	authority.Profile.Witnesses = append([]gp.Witness{}, authority.Profile.Witnesses...)
	authority.TrustFact.Subjects = append([]string{}, authority.TrustFact.Subjects...)
	authority.Isolation.Witnesses = append([]gp.Witness{}, authority.Isolation.Witnesses...)
	authority.Persistence.Witnesses = append([]gp.Witness{}, authority.Persistence.Witnesses...)
	return authority, nil
}

func validateReceiptPersistence(persistence contextreceipt.PersistenceAuthority) error {
	present := persistence.ReceiptDigest != "" || persistence.ReceiptEventDigest != "" || persistence.ReceiptAckDigest != ""
	switch persistence.State {
	case contextreceipt.StateProven:
		if !present || persistence.Witnesses == nil || len(persistence.Witnesses) != 0 {
			return fmt.Errorf("contextowner: proven receipt verification persistence requires all digests and empty witnesses")
		}
	case contextreceipt.StateViolated, contextreceipt.StateUnproven:
		if len(persistence.Witnesses) == 0 {
			return fmt.Errorf("contextowner: non-proven receipt verification persistence requires nonempty witnesses")
		}
	default:
		return fmt.Errorf("contextowner: receipt verification persistence has unknown state %q", persistence.State)
	}
	if err := validateWitnessRows("receipt verification persistence", persistence.Witnesses); err != nil {
		return err
	}
	for _, row := range []struct{ field, value string }{
		{"receipt_digest", persistence.ReceiptDigest},
		{"receipt_event_digest", persistence.ReceiptEventDigest},
		{"receipt_ack_digest", persistence.ReceiptAckDigest},
	} {
		if persistence.State == contextreceipt.StateProven && row.value == "" {
			return fmt.Errorf("contextowner: proven receipt verification persistence requires %s", row.field)
		}
		if row.value != "" {
			if err := validateDigest("receipt verification persistence "+row.field, row.value); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateReceiptAuthorityState(name string, state contextreceipt.State, present bool, witnesses []gp.Witness) error {
	switch state {
	case contextreceipt.StateProven:
		if !present || witnesses == nil || len(witnesses) != 0 {
			return fmt.Errorf("contextowner: proven receipt verification %s requires facts and empty witnesses", name)
		}
	case contextreceipt.StateViolated, contextreceipt.StateUnproven:
		if present || len(witnesses) == 0 {
			return fmt.Errorf("contextowner: non-proven receipt verification %s requires no facts and nonempty witnesses", name)
		}
	default:
		return fmt.Errorf("contextowner: receipt verification %s has unknown state %q", name, state)
	}
	return validateWitnessRows("receipt verification "+name, witnesses)
}

func validateReceiptTrustFact(fact ReceiptVerificationTrustFact) error {
	if err := gp.ValidateID(fact.SourceID); err != nil {
		return fmt.Errorf("contextowner: receipt verification trust source: %w", err)
	}
	if err := fact.SourceKind.Validate(); err != nil {
		return err
	}
	if fact.Subjects == nil {
		return fmt.Errorf("contextowner: receipt verification trust subjects must be non-null")
	}
	for i, subject := range fact.Subjects {
		if err := requireText("receipt verification trust subject", subject); err != nil {
			return err
		}
		if i > 0 && fact.Subjects[i-1] >= subject {
			return fmt.Errorf("contextowner: receipt verification trust subjects must be sorted and deduplicated")
		}
	}
	if !fact.Available {
		if fact.Valid || len(fact.Subjects) != 0 || fact.EvidenceDigest != "" || fact.Reason == "" {
			return fmt.Errorf("contextowner: unavailable receipt verification trust fact is contradictory")
		}
		return nil
	}
	if err := validateDigest("receipt verification trust evidence_digest", fact.EvidenceDigest); err != nil {
		return err
	}
	if !fact.Valid && fact.Reason == "" {
		return fmt.Errorf("contextowner: invalid receipt verification trust fact requires a reason")
	}
	return nil
}

func validateStamp(stamp string) error {
	if !strings.HasSuffix(stamp, "Z") {
		return fmt.Errorf("contextowner: owner stamp must end in Z")
	}
	parsed, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return fmt.Errorf("contextowner: invalid owner stamp: %w", err)
	}
	if parsed.Location() != time.UTC || parsed.Format(time.RFC3339Nano) != stamp {
		return fmt.Errorf("contextowner: owner stamp is not normalized UTC RFC3339Nano")
	}
	return nil
}

func canonicalEventAck(ack contextevent.EventAck) (contextevent.EventAck, error) {
	encoded, err := contextevent.EncodeEventAck(ack)
	if err != nil {
		return contextevent.EventAck{}, err
	}
	return contextevent.DecodeEventAck(bytes.NewReader(encoded))
}

func canonicalReceiptAck(ack contextevent.ReceiptEventAck) (contextevent.ReceiptEventAck, error) {
	encoded, err := contextevent.EncodeReceiptEventAck(ack)
	if err != nil {
		return contextevent.ReceiptEventAck{}, err
	}
	return contextevent.DecodeReceiptEventAck(bytes.NewReader(encoded))
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func copyTexts(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

// Published nested-document interiors.
//
// The publication rule replaces only a payload's top-level schema literal and
// retains every other value rule and cross-field validation (correction §3.3
// step 3), so a nested member must be a valid instance of the schema it
// declares. Where the document's owning codec is reachable it is the
// validator, and no rule is restated. internal/sealedexec is deliberately
// unreachable — Task 2 makes it import this publication — so the documents it
// owns carry the minimum deterministic validation that preserves their fixed
// contract: the exact member set, the closed unions and value domains, the
// record-local cross-field rules, and the self-digest that binds every
// remaining member to the document its owner produced.

func validateNestedInterior(field, schema string, document []byte) error {
	framed := frameExact(document)
	var err error
	switch schema {
	case EventSchemaID:
		_, err = contextevent.DecodeEvent(bytes.NewReader(framed))
	case PolicyConflictReportSchemaID:
		_, err = policyconflict.DecodeReport(framed)
	case DataItemSchemaID:
		_, err = contextcompile.DecodeDataItem(framed)
	case ReceiptSchemaID:
		_, err = contextreceipt.DecodeReceipt(bytes.NewReader(framed))
	case ExecutionRequestSchemaID:
		err = validateNestedExecutionRequest(document)
	case HandbackRecordSchemaID:
		err = validateNestedHandback(document)
	case QuarantineRecordSchemaID:
		_, err = decodeNestedQuarantine(document)
	case AbortRecordSchemaID:
		err = validateNestedAbort(document)
	case ControlAckSchemaID:
		err = validateNestedControlAck(document)
	default:
		return fmt.Errorf("contextowner: %s declares unpublished nested schema %q", field, schema)
	}
	if err != nil {
		return fmt.Errorf("contextowner: %s is not a valid %s document: %w", field, schema, err)
	}
	return nil
}

// decodeNestedExact strict-decodes one canonical nested document into its
// published mirror: unknown members are refused, every required member must be
// present and non-null, and the mirror's own canonical encoding must reproduce
// the exact bytes, so a mirror missing a member of the accepted contract
// cannot silently accept the document that carries it.
func decodeNestedExact(document []byte, target any, required ...string) error {
	if err := artifact.DecodeExactJSON(document, target); err != nil {
		return err
	}
	if err := requireMembers(document, required...); err != nil {
		return err
	}
	canonical, err := canonjson.Marshal(target)
	if err != nil {
		return err
	}
	if !bytes.Equal(document, bytes.TrimSuffix(canonical, []byte("\n"))) {
		return fmt.Errorf("contextowner: document is not the canonical encoding of its published members")
	}
	return nil
}

// validateNestedSelfDigest recomputes a self-digested document's digest the
// way its owning codec does: the canonical encoding of the same document with
// its digest member blanked. Every other member is therefore bound to the
// digest, so no member can be altered, added, or dropped undetected.
func validateNestedSelfDigest(name string, document []byte, declared string) error {
	if err := validateDigest(name+" digest", declared); err != nil {
		return err
	}
	var members map[string]json.RawMessage
	if err := artifact.DecodeExactJSON(document, &members); err != nil {
		return err
	}
	members["digest"] = json.RawMessage(`""`)
	want, err := canonjson.Digest(members)
	if err != nil {
		return fmt.Errorf("contextowner: %s digest preimage: %w", name, err)
	}
	if declared != want {
		return fmt.Errorf("contextowner: %s digest does not match its canonical blank-digest preimage", name)
	}
	return nil
}

// requireControlText publishes the control records' text domain: nonempty
// unless the member is explicitly optional, valid trimmed UTF-8, and free of
// control characters.
func requireControlText(field, value string, allowEmpty bool) error {
	if value == "" {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("contextowner: %s must be nonempty", field)
	}
	if !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return fmt.Errorf("contextowner: %s must be trimmed UTF-8", field)
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("contextowner: %s contains a control character", field)
		}
	}
	return nil
}

func validateControlIdentity(flight, lane, epoch, session, runway, workspaceID string, allowEmptySession bool) error {
	for _, row := range []struct{ field, value string }{
		{"flight", flight}, {"lane", lane}, {"epoch", epoch}, {"workspace_id", workspaceID},
	} {
		if err := requireControlText(row.field, row.value, false); err != nil {
			return err
		}
	}
	if err := requireControlText("session", session, allowEmptySession); err != nil {
		return err
	}
	if runway != runwayNotCarried {
		return requireControlText("atc_runway", runway, false)
	}
	return nil
}

func validateNestedGitIdentity(field string, identity nestedGitIdentity) error {
	if err := validateGitOID(field+" commit", identity.Commit, false); err != nil {
		return err
	}
	return validateGitOID(field+" tree", identity.Tree, false)
}

func validateNestedRunwayState(field string, state nestedRunwayState) error {
	if !state.Clean {
		return fmt.Errorf("contextowner: %s must be clean", field)
	}
	return validateNestedGitIdentity(field, nestedGitIdentity{Commit: state.Head, Tree: state.Tree})
}

// validateNestedDurableReceipt binds a receipt digest to the acknowledgment
// that made it durable under one execution identity.
func validateNestedDurableReceipt(digest string, ack contextevent.ReceiptEventAck, flight, lane, epoch, session string) error {
	if err := validateDigest("receipt digest", digest); err != nil {
		return err
	}
	canonical, err := canonicalReceiptAck(ack)
	if err != nil {
		return err
	}
	if canonical != ack {
		return fmt.Errorf("contextowner: receipt event acknowledgment is not canonical")
	}
	if ack.ReceiptDigest != digest || ack.Flight != flight || ack.Lane != lane ||
		ack.Epoch != epoch || ack.Session != session {
		return fmt.Errorf("contextowner: receipt event acknowledgment identity mismatch")
	}
	return nil
}

func validateNestedHandback(document []byte) error {
	var record nestedHandbackRecord
	if err := decodeNestedExact(document, &record,
		"schema", "flight", "lane", "epoch", "session", "atc_runway", "workspace_id",
		"receipt", "input", "output", "pre_runway", "post_runway", "disposition", "digest"); err != nil {
		return err
	}
	if err := validateControlIdentity(record.Flight, record.Lane, record.Epoch, record.Session,
		record.ATCRunway, record.WorkspaceID, false); err != nil {
		return err
	}
	if err := validateNestedDurableReceipt(record.Receipt.Digest, record.Receipt.EventAck,
		record.Flight, record.Lane, record.Epoch, record.Session); err != nil {
		return err
	}
	if err := validateNestedGitIdentity("handback input", record.Input); err != nil {
		return err
	}
	if err := validateNestedGitIdentity("handback output", record.Output); err != nil {
		return err
	}
	if err := validateNestedRunwayState("pre_runway", record.PreRunway); err != nil {
		return err
	}
	if err := validateNestedRunwayState("post_runway", record.PostRunway); err != nil {
		return err
	}
	if record.PreRunway.Head != record.Input.Commit || record.PreRunway.Tree != record.Input.Tree {
		return fmt.Errorf("contextowner: handback pre_runway must equal input commit/tree")
	}
	if record.PostRunway.Head != record.Output.Commit || record.PostRunway.Tree != record.Output.Tree {
		return fmt.Errorf("contextowner: handback post_runway must equal output commit/tree")
	}
	if record.Disposition != dispositionFastForwarded {
		return fmt.Errorf("contextowner: handback disposition must be %q", dispositionFastForwarded)
	}
	return validateNestedSelfDigest("handback", document, record.Digest)
}

func validateNestedAbort(document []byte) error {
	var record nestedAbortRecord
	if err := decodeNestedExact(document, &record,
		"schema", "flight", "lane", "epoch", "session", "workspace_id", "quarantine_digest",
		"owner_decision", "preserved", "disposition", "digest"); err != nil {
		return err
	}
	if err := validateControlIdentity(record.Flight, record.Lane, record.Epoch, record.Session,
		runwayNotCarried, record.WorkspaceID, true); err != nil {
		return err
	}
	if err := validateDigest("quarantine_digest", record.QuarantineDigest); err != nil {
		return err
	}
	for _, row := range []struct{ field, value string }{
		{"owner_decision.schema", record.OwnerDecision.Schema},
		{"owner_decision.id", record.OwnerDecision.ID},
	} {
		if err := requireControlText(row.field, row.value, false); err != nil {
			return err
		}
	}
	if err := validateDigest("owner_decision.digest", record.OwnerDecision.Digest); err != nil {
		return err
	}
	if err := validateNestedPreservedRef(record.Preserved); err != nil {
		return err
	}
	if record.Disposition != dispositionAbortPreserve {
		return fmt.Errorf("contextowner: abort disposition must be %q", dispositionAbortPreserve)
	}
	return validateNestedSelfDigest("abort", document, record.Digest)
}

func validateNestedControlAck(document []byte) error {
	var ack nestedControlAck
	if err := decodeNestedExact(document, &ack,
		"schema", "record_schema", "record_digest", "flight", "lane", "epoch", "session",
		"workspace_id", "disposition", "controller_global_sequence", "digest"); err != nil {
		return err
	}
	if err := validateControlIdentity(ack.Flight, ack.Lane, ack.Epoch, ack.Session,
		runwayNotCarried, ack.WorkspaceID, true); err != nil {
		return err
	}
	if err := validateDigest("record_digest", ack.RecordDigest); err != nil {
		return err
	}
	want, known := dispositionForRecordSchema(ack.RecordSchema)
	if !known {
		return fmt.Errorf("contextowner: unknown control ack record_schema %q", ack.RecordSchema)
	}
	if ack.Disposition != want {
		return fmt.Errorf("contextowner: control ack disposition %q contradicts record_schema %q",
			ack.Disposition, ack.RecordSchema)
	}
	if ack.ControllerGlobalSequence == 0 {
		return fmt.Errorf("contextowner: controller_global_sequence must be positive")
	}
	return validateNestedSelfDigest("control ack", document, ack.Digest)
}

func dispositionForRecordSchema(schema string) (string, bool) {
	switch schema {
	case HandbackRecordSchemaID:
		return dispositionFastForwarded, true
	case QuarantineRecordSchemaID:
		return dispositionQuarantined, true
	case AbortRecordSchemaID:
		return dispositionAbortPreserve, true
	default:
		return "", false
	}
}

func decodeNestedQuarantine(document []byte) (nestedQuarantineRecord, error) {
	var record nestedQuarantineRecord
	if err := decodeNestedExact(document, &record,
		"schema", "flight", "lane", "epoch", "session", "atc_runway", "workspace_id",
		"receipt", "repository", "observed", "reason", "preserved", "digest"); err != nil {
		return nestedQuarantineRecord{}, err
	}
	if err := validateControlIdentity(record.Flight, record.Lane, record.Epoch, record.Session,
		record.ATCRunway, record.WorkspaceID, true); err != nil {
		return nestedQuarantineRecord{}, err
	}
	if err := validateNestedQuarantineReceipt(record); err != nil {
		return nestedQuarantineRecord{}, err
	}
	if err := validateNestedGitIdentity("quarantine input", record.Repository.Input); err != nil {
		return nestedQuarantineRecord{}, err
	}
	if err := validateNestedQuarantineOutput(record.Repository.Output); err != nil {
		return nestedQuarantineRecord{}, err
	}
	for _, row := range []struct {
		name        string
		observation nestedRepoObservation
	}{
		{"runway", record.Observed.Runway}, {"child", record.Observed.Child},
		{"post_runway", record.Observed.PostRunway},
	} {
		if err := validateNestedRepoObservation(row.name, row.observation); err != nil {
			return nestedQuarantineRecord{}, err
		}
	}
	if err := validateNestedProof(record.Observed.Descendant); err != nil {
		return nestedQuarantineRecord{}, err
	}
	if err := validateNestedProtectedPaths(record.Observed.ProtectedPaths); err != nil {
		return nestedQuarantineRecord{}, err
	}
	switch record.Observed.FastForward {
	case fastForwardNotAttempted, fastForwardSucceeded, fastForwardFailed:
	default:
		return nestedQuarantineRecord{}, fmt.Errorf("contextowner: unknown fast_forward state %q", record.Observed.FastForward)
	}
	if err := validateNestedPreserved(record.Preserved); err != nil {
		return nestedQuarantineRecord{}, err
	}
	if record.Receipt.State == quarantineReceiptDurable && record.Preserved.State != preservedFinalized {
		return nestedQuarantineRecord{}, fmt.Errorf("contextowner: durable quarantine receipt requires finalized preservation")
	}
	if record.Receipt.State == quarantineReceiptAbsent && record.Preserved.State == preservedFinalized {
		return nestedQuarantineRecord{}, fmt.Errorf("contextowner: absent quarantine receipt forbids finalized preservation")
	}
	if record.Preserved.State == preservedFinalized && record.Repository.Output.State != quarantineOutputObserved {
		return nestedQuarantineRecord{}, fmt.Errorf("contextowner: finalized preservation requires observed output")
	}
	if err := validateQuarantineReasonFacts(record); err != nil {
		return nestedQuarantineRecord{}, err
	}
	if err := validateNestedSelfDigest("quarantine", document, record.Digest); err != nil {
		return nestedQuarantineRecord{}, err
	}
	return record, nil
}

func validateNestedQuarantineReceipt(record nestedQuarantineRecord) error {
	switch record.Receipt.State {
	case quarantineReceiptAbsent:
		if record.Receipt.Digest != "" || record.Receipt.EventAck != nil {
			return fmt.Errorf("contextowner: absent quarantine receipt forbids digest/event_ack")
		}
	case quarantineReceiptDurable:
		if record.Receipt.EventAck == nil {
			return fmt.Errorf("contextowner: durable quarantine receipt requires event_ack")
		}
		return validateNestedDurableReceipt(record.Receipt.Digest, *record.Receipt.EventAck,
			record.Flight, record.Lane, record.Epoch, record.Session)
	default:
		return fmt.Errorf("contextowner: unknown quarantine receipt state %q", record.Receipt.State)
	}
	return nil
}

func validateNestedQuarantineOutput(output nestedQuarantineOutput) error {
	switch output.State {
	case quarantineOutputAbsent:
		if output.Commit != "" || output.Tree != "" {
			return fmt.Errorf("contextowner: absent quarantine output forbids commit/tree")
		}
	case quarantineOutputObserved:
		return validateNestedGitIdentity("quarantine output",
			nestedGitIdentity{Commit: output.Commit, Tree: output.Tree})
	default:
		return fmt.Errorf("contextowner: unknown quarantine output state %q", output.State)
	}
	return nil
}

func validateNestedRepoObservation(name string, observation nestedRepoObservation) error {
	switch observation.State {
	case repositoryObserved:
		return validateNestedGitIdentity(name,
			nestedGitIdentity{Commit: observation.Commit, Tree: observation.Tree})
	case repositoryUnproven:
		if observation.Commit != "" || observation.Tree != "" || observation.Clean {
			return fmt.Errorf("contextowner: unproven %s observation must have empty commit/tree and clean false", name)
		}
	default:
		return fmt.Errorf("contextowner: unknown %s observation state %q", name, observation.State)
	}
	return nil
}

func validateNestedProof(proof nestedProof) error {
	if proof.Witnesses == nil {
		return fmt.Errorf("contextowner: proof witnesses must be non-null")
	}
	if err := validateSortedTexts("proof witnesses", proof.Witnesses); err != nil {
		return err
	}
	switch proof.State {
	case proofProven:
		if len(proof.Witnesses) != 0 {
			return fmt.Errorf("contextowner: proven proof must have empty witnesses")
		}
	case proofViolatedWithWitness, proofUnproven:
		if len(proof.Witnesses) == 0 {
			return fmt.Errorf("contextowner: non-proven proof requires witnesses")
		}
	default:
		return fmt.Errorf("contextowner: unknown proof state %q", proof.State)
	}
	return nil
}

func validateNestedProtectedPaths(paths []string) error {
	if paths == nil {
		return fmt.Errorf("contextowner: protected_paths must be non-null")
	}
	if err := validateSortedTexts("protected_paths", paths); err != nil {
		return err
	}
	for _, value := range paths {
		if strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || path.Clean(value) != value {
			return fmt.Errorf("contextowner: protected path is not a clean repository path")
		}
		parts := strings.Split(value, "/")
		if len(parts) < 4 || parts[0] != ".verdi" || parts[1] != "specs" || parts[len(parts)-1] != "spec.md" {
			return fmt.Errorf("contextowner: protected path is outside .verdi/specs/**/spec.md")
		}
	}
	return nil
}

func validateNestedPreserved(preserved nestedPreservedExecution) error {
	switch preserved.State {
	case preservedNone:
		if preserved.Ref != nil {
			return fmt.Errorf("contextowner: preserved none forbids ref")
		}
	case preservedPartial, preservedFinalized:
		if preserved.Ref == nil {
			return fmt.Errorf("contextowner: preserved %s requires ref", preserved.State)
		}
		return validateNestedPreservedRef(*preserved.Ref)
	default:
		return fmt.Errorf("contextowner: unknown preserved state %q", preserved.State)
	}
	return nil
}

func validateNestedPreservedRef(ref nestedPreservedExecutionRef) error {
	if ref.Schema != preservedExecutionRefSchemaID {
		return fmt.Errorf("contextowner: preserved ref schema must be %q", preservedExecutionRefSchemaID)
	}
	if err := requireControlText("preserved ref id", ref.ID, false); err != nil {
		return err
	}
	return validateDigest("preserved ref digest", ref.Digest)
}

// validateQuarantineReasonFacts publishes the accepted closed relation between
// a quarantine reason and the observations that justify it. A self-consistent
// record whose reason contradicts its own observed facts is exactly the
// content the private controller codec refuses, so the published arm refuses
// it too rather than handing an owner a decision no fact supports.
func validateQuarantineReasonFacts(record nestedQuarantineRecord) error {
	switch record.Reason {
	case quarantineRunwayDirty, quarantineRunwayMoved, quarantineChildDirty,
		quarantineNonDescendant, quarantineProtectedSpecChange,
		quarantineFastForwardFailed, quarantinePostVerificationMismatch,
		quarantineRepositoryVerificationFailed, quarantineChildOutputMismatch,
		quarantineHandbackDurabilityFailed:
		if record.Receipt.State != quarantineReceiptDurable || record.Preserved.State != preservedFinalized {
			return fmt.Errorf("contextowner: handback quarantine requires a durable finalized result")
		}
	}

	observed := record.Observed
	output := record.Repository.Output
	input := record.Repository.Input
	preRunway := func() bool {
		return observed.Runway.State == repositoryObserved && observed.Runway.Clean &&
			observed.Runway.Commit == input.Commit && observed.Runway.Tree == input.Tree
	}
	preChild := func() bool {
		return output.State == quarantineOutputObserved && observed.Child.State == repositoryObserved &&
			observed.Child.Clean && observed.Child.Commit == output.Commit && observed.Child.Tree == output.Tree
	}
	preAll := func() bool {
		return preRunway() && preChild() && observed.Descendant.State == proofProven &&
			len(observed.ProtectedPaths) == 0
	}
	unprovenChild := func() bool { return observed.Child.State == repositoryUnproven }
	unprovenDescendant := func() bool { return observed.Descendant.State == proofUnproven }
	unprovenPost := func() bool { return observed.PostRunway.State == repositoryUnproven }
	noLaterFacts := func() bool {
		return unprovenChild() && unprovenDescendant() && len(observed.ProtectedPaths) == 0 &&
			observed.FastForward == fastForwardNotAttempted && unprovenPost()
	}
	noObservations := func() bool {
		return observed.Runway.State == repositoryUnproven && observed.Child.State == repositoryUnproven &&
			observed.Descendant.State == proofUnproven && len(observed.ProtectedPaths) == 0 &&
			observed.FastForward == fastForwardNotAttempted && observed.PostRunway.State == repositoryUnproven
	}
	postIsInput := func() bool {
		return observed.PostRunway.State == repositoryObserved && observed.PostRunway.Clean &&
			observed.PostRunway.Commit == input.Commit && observed.PostRunway.Tree == input.Tree
	}
	postIsOutput := func() bool {
		return output.State == quarantineOutputObserved && observed.PostRunway.State == repositoryObserved &&
			observed.PostRunway.Clean && observed.PostRunway.Commit == output.Commit &&
			observed.PostRunway.Tree == output.Tree
	}
	lateAttemptState := func() bool {
		return observed.FastForward == fastForwardNotAttempted || observed.FastForward == fastForwardFailed
	}

	switch record.Reason {
	case quarantineRunwayDirty:
		initial := observed.Runway.State == repositoryObserved && !observed.Runway.Clean && noLaterFacts()
		late := preAll() && lateAttemptState() && observed.PostRunway.State == repositoryObserved &&
			!observed.PostRunway.Clean
		if !initial && !late {
			return fmt.Errorf("contextowner: runway-dirty requires an initial or late observed dirty runway with truthful attempt facts")
		}
	case quarantineRunwayMoved:
		initial := observed.Runway.State == repositoryObserved && observed.Runway.Clean &&
			(observed.Runway.Commit != input.Commit || observed.Runway.Tree != input.Tree) && noLaterFacts()
		lateMoved := observed.PostRunway.State == repositoryObserved && observed.PostRunway.Clean && !postIsInput()
		if observed.FastForward == fastForwardFailed {
			lateMoved = lateMoved && !postIsOutput()
		}
		late := preAll() && lateAttemptState() && lateMoved
		if !initial && !late {
			return fmt.Errorf("contextowner: runway-moved requires an initial or late clean mismatch with truthful attempt facts")
		}
	case quarantineChildDirty:
		if !preRunway() || observed.Child.State != repositoryObserved || observed.Child.Clean ||
			!unprovenDescendant() || len(observed.ProtectedPaths) != 0 ||
			observed.FastForward != fastForwardNotAttempted || !unprovenPost() {
			return fmt.Errorf("contextowner: child-dirty contradicts repository observations")
		}
	case quarantineChildOutputMismatch:
		childMismatch := output.State == quarantineOutputObserved && observed.Child.State == repositoryObserved &&
			observed.Child.Clean && (observed.Child.Commit != output.Commit || observed.Child.Tree != output.Tree)
		if !preRunway() || !childMismatch || !unprovenDescendant() || len(observed.ProtectedPaths) != 0 ||
			observed.FastForward != fastForwardNotAttempted || !unprovenPost() {
			return fmt.Errorf("contextowner: child-output-mismatch contradicts repository observations")
		}
	case quarantineNonDescendant:
		if !preRunway() || !preChild() || observed.Descendant.State != proofViolatedWithWitness ||
			len(observed.ProtectedPaths) != 0 || observed.FastForward != fastForwardNotAttempted || !unprovenPost() {
			return fmt.Errorf("contextowner: non-descendant contradicts precheck facts")
		}
	case quarantineProtectedSpecChange:
		if !preRunway() || !preChild() || observed.Descendant.State != proofProven ||
			len(observed.ProtectedPaths) == 0 || observed.FastForward != fastForwardNotAttempted || !unprovenPost() {
			return fmt.Errorf("contextowner: protected-spec-change contradicts precheck facts")
		}
	case quarantineFastForwardFailed:
		if !preAll() || observed.FastForward != fastForwardFailed || (!postIsInput() && !postIsOutput()) {
			return fmt.Errorf("contextowner: fast-forward-failed contradicts precheck/attempt facts")
		}
	case quarantinePostVerificationMismatch:
		if !preAll() || observed.FastForward != fastForwardSucceeded ||
			observed.PostRunway.State != repositoryObserved ||
			(observed.PostRunway.Clean && observed.PostRunway.Commit == output.Commit &&
				observed.PostRunway.Tree == output.Tree) {
			return fmt.Errorf("contextowner: post-verification-mismatch lacks a postcheck mismatch")
		}
	case quarantineHandbackDurabilityFailed:
		if !preAll() || observed.FastForward != fastForwardSucceeded || !postIsOutput() {
			return fmt.Errorf("contextowner: handback-durability-failed requires exact clean prechecks, successful fast-forward, and exact clean output postcheck")
		}
	case quarantineNonAuthoritative, quarantineOutputWriteFailed:
		if record.Receipt.State != quarantineReceiptDurable || record.Preserved.State != preservedFinalized ||
			!noObservations() {
			return fmt.Errorf("contextowner: finalized no-handback quarantine facts contradict reason %q", record.Reason)
		}
	case quarantineExecutionIncomplete:
		if record.Receipt.State != quarantineReceiptAbsent || output.State != quarantineOutputAbsent ||
			(record.Preserved.State != preservedNone && record.Preserved.State != preservedPartial) ||
			!noObservations() {
			return fmt.Errorf("contextowner: execution-incomplete facts contradict phase")
		}
	case quarantineTerminalDurabilityFailed:
		if record.Receipt.State != quarantineReceiptAbsent || record.Preserved.State == preservedFinalized ||
			!noObservations() {
			return fmt.Errorf("contextowner: terminal-durability-failed facts contradict phase")
		}
	case quarantineRepositoryVerificationFailed:
		if len(observed.ProtectedPaths) != 0 || !unprovenPost() {
			return fmt.Errorf("contextowner: repository-verification-failed requires an exact prefix and unproven post-runway")
		}
		beforeRunway := noObservations()
		afterRunway := preRunway() && unprovenChild() && unprovenDescendant() &&
			observed.FastForward == fastForwardNotAttempted
		afterChild := preRunway() && preChild() && unprovenDescendant() &&
			observed.FastForward == fastForwardNotAttempted
		afterRepositoryChecks := preAll() && (observed.FastForward == fastForwardNotAttempted ||
			observed.FastForward == fastForwardFailed || observed.FastForward == fastForwardSucceeded)
		if !beforeRunway && !afterRunway && !afterChild && !afterRepositoryChecks {
			return fmt.Errorf("contextowner: repository-verification-failed carries a non-prefix repository state")
		}
	default:
		return fmt.Errorf("contextowner: unknown quarantine reason %q", record.Reason)
	}
	return nil
}

// validateQuarantinePreservation publishes the accepted cross-field rule that
// the quarantine record and the exact controller-carried bytes describe one
// preserved execution: the locator names the state and content address of
// those exact bytes, and the bytes are the canonical document that state
// selects, carrying the record's own execution identity.
func validateQuarantinePreservation(record json.RawMessage, data []byte) error {
	decoded, err := decodeNestedQuarantine(record)
	if err != nil {
		return err
	}
	if data == nil {
		return fmt.Errorf("contextowner: quarantine preserved bytes must be non-null")
	}
	want, err := preservedExecutionForBytes(decoded.Preserved.State, data)
	if err != nil {
		return err
	}
	if !preservedExecutionEqual(decoded.Preserved, want) {
		return fmt.Errorf("contextowner: quarantine preserved locator contradicts exact bytes")
	}
	switch decoded.Preserved.State {
	case preservedNone:
		return nil
	case preservedPartial:
		return validatePreservedPartial(decoded, data)
	default:
		return validatePreservedFinalized(decoded, data)
	}
}

// preservedExecutionForBytes derives the sole controller-owned locator for the
// exact carried bytes. None is represented by non-null empty bytes.
func preservedExecutionForBytes(state string, data []byte) (nestedPreservedExecution, error) {
	switch state {
	case preservedNone:
		if data == nil || len(data) != 0 {
			return nestedPreservedExecution{}, fmt.Errorf("contextowner: preserved none requires non-null empty bytes")
		}
		return nestedPreservedExecution{State: preservedNone}, nil
	case preservedPartial, preservedFinalized:
		if len(data) == 0 {
			return nestedPreservedExecution{}, fmt.Errorf("contextowner: preserved %s requires nonempty bytes", state)
		}
		digest := digestBytes(data)
		return nestedPreservedExecution{State: state, Ref: &nestedPreservedExecutionRef{
			Schema: preservedExecutionRefSchemaID,
			ID:     preservedExecutionIDPrefix + strings.TrimPrefix(digest, "sha256:"),
			Digest: digest,
		}}, nil
	default:
		return nestedPreservedExecution{}, fmt.Errorf("contextowner: unknown preserved state %q", state)
	}
}

func preservedExecutionEqual(left, right nestedPreservedExecution) bool {
	if left.State != right.State || (left.Ref == nil) != (right.Ref == nil) {
		return false
	}
	return left.Ref == nil || *left.Ref == *right.Ref
}

// preservedCanonicalDocument proves the carried bytes are exactly one
// canonical standalone document, trailing LF and all, and returns it framed
// as the nested body its published validator decodes.
func preservedCanonicalDocument(data []byte) ([]byte, error) {
	canonical, err := canonjson.Marshal(json.RawMessage(data))
	if err != nil {
		return nil, fmt.Errorf("contextowner: canonicalize quarantine preserved bytes: %w", err)
	}
	if !bytes.Equal(data, canonical) {
		return nil, fmt.Errorf("contextowner: quarantine preserved bytes are not one canonical JSON document")
	}
	return bytes.TrimSuffix(canonical, []byte("\n")), nil
}

func validatePreservedPartial(record nestedQuarantineRecord, data []byte) error {
	document, err := preservedCanonicalDocument(data)
	if err != nil {
		return err
	}
	partial, err := validateNestedExecutionPartial(document)
	if err != nil {
		return fmt.Errorf("contextowner: quarantine partial bytes: %w", err)
	}
	if partial.Flight != record.Flight || partial.Lane != record.Lane || partial.Epoch != record.Epoch ||
		partial.Session != record.Session || partial.WorkspaceID != record.WorkspaceID {
		return fmt.Errorf("contextowner: quarantine partial bytes contradict record identity")
	}
	return nil
}

func validatePreservedFinalized(record nestedQuarantineRecord, data []byte) error {
	document, err := preservedCanonicalDocument(data)
	if err != nil {
		return err
	}
	result, err := validateNestedExecutionResult(document)
	if err != nil {
		return fmt.Errorf("contextowner: quarantine finalized bytes: %w", err)
	}
	if record.Repository.Output.State != quarantineOutputObserved ||
		record.Receipt.State != quarantineReceiptDurable || record.Receipt.EventAck == nil {
		return fmt.Errorf("contextowner: quarantine finalized bytes contradict record facts")
	}
	receipt, err := contextreceipt.DecodeReceipt(bytes.NewReader(frameExact(result.Receipt)))
	if err != nil {
		return fmt.Errorf("contextowner: quarantine finalized receipt: %w", err)
	}
	ack, err := contextevent.DecodeReceiptEventAck(bytes.NewReader(frameExact(result.ReceiptEventAck)))
	if err != nil {
		return fmt.Errorf("contextowner: quarantine finalized receipt acknowledgment: %w", err)
	}
	if result.Flight != record.Flight || result.Lane != record.Lane || result.Epoch != record.Epoch ||
		result.Session != record.Session || result.ATCRunway != record.ATCRunway ||
		result.ExecutionWorkspaceID != record.WorkspaceID ||
		result.InputCommit != record.Repository.Input.Commit || result.InputTree != record.Repository.Input.Tree ||
		result.OutputCommit != record.Repository.Output.Commit || result.OutputTree != record.Repository.Output.Tree ||
		receipt.Digest != record.Receipt.Digest || ack != *record.Receipt.EventAck {
		return fmt.Errorf("contextowner: quarantine finalized bytes contradict record facts")
	}
	return nil
}

// validateNestedExecutionPartial publishes the accepted inspectable
// request/run state: its exact member set, closed action/adapter/authority
// unions, three-valued authority rule, and the complete acknowledgment stream
// contract — one fixed execution identity, strictly increasing global order,
// contiguous source order inside a revision, a child revision restarting at
// source one, no skipped revision, and a stream that ends at the represented
// manifest revision or exactly one before it.
func validateNestedExecutionPartial(document []byte) (nestedExecutionPartial, error) {
	var partial nestedExecutionPartial
	if err := decodeNestedExact(document, &partial,
		"schema", "flight", "lane", "epoch", "session", "action", "manifest_revision",
		"manifest_digest", "adapter", "adapter_version", "workspace_id",
		"adapter_session_ref", "authority", "witnesses", "event_acks"); err != nil {
		return nestedExecutionPartial{}, err
	}
	if partial.Schema != executionPartialSchemaID {
		return nestedExecutionPartial{}, fmt.Errorf("contextowner: execution partial schema must be %q", executionPartialSchemaID)
	}
	if err := validateControlIdentity(partial.Flight, partial.Lane, partial.Epoch, partial.Session,
		runwayNotCarried, partial.WorkspaceID, false); err != nil {
		return nestedExecutionPartial{}, err
	}
	switch partial.Action {
	case actionStart, actionResume:
	default:
		return nestedExecutionPartial{}, fmt.Errorf("contextowner: unknown execution partial action %q", partial.Action)
	}
	if err := validateDigest("execution partial manifest_digest", partial.ManifestDigest); err != nil {
		return nestedExecutionPartial{}, err
	}
	// The accepted partial closes its adapter domain by enumeration rather
	// than through the adapter's own validator, so the publication does too.
	switch partial.Adapter {
	case contextevent.AdapterCodex, contextevent.AdapterClaude:
	default:
		return nestedExecutionPartial{}, fmt.Errorf("contextowner: unknown execution partial adapter %q", partial.Adapter)
	}
	if err := requireControlText("execution partial adapter_version", partial.AdapterVersion, false); err != nil {
		return nestedExecutionPartial{}, err
	}
	if err := requireControlText("execution partial adapter_session_ref", partial.AdapterSessionRef, true); err != nil {
		return nestedExecutionPartial{}, err
	}
	if partial.Witnesses == nil {
		return nestedExecutionPartial{}, fmt.Errorf("contextowner: execution partial witnesses must be non-null")
	}
	if err := validateSortedTexts("execution partial witnesses", partial.Witnesses); err != nil {
		return nestedExecutionPartial{}, err
	}
	switch partial.Authority {
	case contextevent.AuthorityAuthoritative:
		if len(partial.Witnesses) != 0 {
			return nestedExecutionPartial{}, fmt.Errorf("contextowner: authoritative execution partial carries adverse witnesses")
		}
	case contextevent.AuthorityAdvisory:
		if len(partial.Witnesses) == 0 {
			return nestedExecutionPartial{}, fmt.Errorf("contextowner: advisory execution partial lacks explicit witnesses")
		}
	default:
		return nestedExecutionPartial{}, fmt.Errorf("contextowner: unknown execution partial authority %q", partial.Authority)
	}
	if partial.EventAcks == nil {
		return nestedExecutionPartial{}, fmt.Errorf("contextowner: execution partial event_acks must be non-null")
	}
	last, err := validateAcknowledgmentStream(partial.EventAcks,
		partial.Flight, partial.Lane, partial.Epoch, partial.Session)
	if err != nil {
		return nestedExecutionPartial{}, err
	}
	if len(partial.EventAcks) != 0 &&
		last.ManifestRevision != partial.ManifestRevision && last.ManifestRevision+1 != partial.ManifestRevision {
		return nestedExecutionPartial{}, fmt.Errorf(
			"contextowner: execution partial represents manifest revision %d but its acknowledgments end at %d",
			partial.ManifestRevision, last.ManifestRevision)
	}
	return partial, nil
}

// validateAcknowledgmentStream publishes the accepted acknowledgment-stream
// contract shared by every carried run state.
func validateAcknowledgmentStream(acks []contextevent.EventAck, flight, lane, epoch, session string) (contextevent.EventAck, error) {
	if len(acks) == 0 {
		return contextevent.EventAck{}, nil
	}
	canonical := make([]contextevent.EventAck, 0, len(acks))
	for i, ack := range acks {
		roundtripped, err := canonicalEventAck(ack)
		if err != nil {
			return contextevent.EventAck{}, fmt.Errorf("contextowner: event_acks[%d]: %w", i, err)
		}
		if roundtripped != ack {
			return contextevent.EventAck{}, fmt.Errorf("contextowner: event_acks[%d] is not canonical", i)
		}
		if roundtripped.Flight != flight || roundtripped.Lane != lane ||
			roundtripped.Epoch != epoch || roundtripped.Session != session {
			return contextevent.EventAck{}, fmt.Errorf("contextowner: event_acks[%d] identity contradicts the carried state", i)
		}
		canonical = append(canonical, roundtripped)
	}
	for i := 1; i < len(canonical); i++ {
		prior, current := canonical[i-1], canonical[i]
		if current.GlobalSequence <= prior.GlobalSequence {
			return contextevent.EventAck{}, fmt.Errorf("contextowner: event_acks order is discontinuous")
		}
		switch current.ManifestRevision {
		case prior.ManifestRevision:
			if current.SourceSequence != prior.SourceSequence+1 {
				return contextevent.EventAck{}, fmt.Errorf("contextowner: event_acks order is discontinuous")
			}
		case prior.ManifestRevision + 1:
			if current.SourceSequence != 1 {
				return contextevent.EventAck{}, fmt.Errorf("contextowner: event_acks child revision does not restart source order")
			}
		default:
			return contextevent.EventAck{}, fmt.Errorf("contextowner: event_acks skip a manifest revision")
		}
	}
	return canonical[len(canonical)-1], nil
}

// validateNestedExecutionResult publishes the accepted finalized result: its
// exact member set, value domains, the receipt and acknowledgment that are its
// ratified integrity boundary, and the closed authority/verdict/witness rule.
func validateNestedExecutionResult(document []byte) (nestedExecutionResult, error) {
	var result nestedExecutionResult
	if err := decodeNestedExact(document, &result,
		"schema", "verdict", "authority", "witnesses", "flight", "lane", "epoch", "session",
		"atc_runway", "execution_workspace_id", "adapter", "adapter_version",
		"input_commit", "input_tree", "output_commit", "output_tree", "clean",
		"terminal_manifest_digest", "terminal_manifest_revision", "terminal_source_sequence",
		"terminal_global_sequence", "event_chain_root", "receipt", "receipt_event_ack"); err != nil {
		return nestedExecutionResult{}, err
	}
	if result.Schema != executionResultSchemaID {
		return nestedExecutionResult{}, fmt.Errorf("contextowner: execution result schema must be %q", executionResultSchemaID)
	}
	if err := result.Verdict.Validate(); err != nil {
		return nestedExecutionResult{}, fmt.Errorf("contextowner: execution result verdict: %w", err)
	}
	if result.Authority != contextevent.AuthorityAuthoritative && result.Authority != contextevent.AuthorityAdvisory {
		return nestedExecutionResult{}, fmt.Errorf("contextowner: execution result has unknown authority %q", result.Authority)
	}
	for _, row := range []struct{ field, value string }{
		{"flight", result.Flight}, {"lane", result.Lane}, {"epoch", result.Epoch},
		{"session", result.Session}, {"atc_runway", result.ATCRunway},
		{"execution_workspace_id", result.ExecutionWorkspaceID}, {"adapter_version", result.AdapterVersion},
	} {
		if err := requireText("execution result "+row.field, row.value); err != nil {
			return nestedExecutionResult{}, err
		}
	}
	if err := result.Adapter.Validate(); err != nil {
		return nestedExecutionResult{}, err
	}
	for _, row := range []struct{ field, value string }{
		{"input_commit", result.InputCommit}, {"input_tree", result.InputTree},
		{"output_commit", result.OutputCommit}, {"output_tree", result.OutputTree},
	} {
		if err := validateGitOID("execution result "+row.field, row.value, false); err != nil {
			return nestedExecutionResult{}, err
		}
	}
	for _, row := range []struct{ field, value string }{
		{"terminal_manifest_digest", result.TerminalManifestDigest},
		{"event_chain_root", result.EventChainRoot},
	} {
		if err := validateDigest("execution result "+row.field, row.value); err != nil {
			return nestedExecutionResult{}, err
		}
	}
	if result.TerminalSourceSequence == 0 || result.TerminalGlobalSequence == 0 {
		return nestedExecutionResult{}, fmt.Errorf("contextowner: execution result terminal sequences must be positive")
	}
	// The accepted result's witness rule is the sealed request family's
	// plain-text one: nonempty trimmed UTF-8, sorted and deduplicated. It
	// does not exclude control characters, and the publication must not add
	// that exclusion or it would refuse a witness the contract allows.
	if result.Witnesses == nil {
		return nestedExecutionResult{}, fmt.Errorf("contextowner: execution result witnesses must be non-null")
	}
	for i, witness := range result.Witnesses {
		if err := requireText(fmt.Sprintf("execution result witnesses[%d]", i), witness); err != nil {
			return nestedExecutionResult{}, err
		}
		if i > 0 && result.Witnesses[i-1] >= witness {
			return nestedExecutionResult{}, fmt.Errorf("contextowner: execution result witnesses must be sorted and deduplicated")
		}
	}
	receipt, err := contextreceipt.DecodeReceipt(bytes.NewReader(frameExact(result.Receipt)))
	if err != nil {
		return nestedExecutionResult{}, fmt.Errorf("contextowner: execution result receipt: %w", err)
	}
	if receipt.Role != contextreceipt.RoleBuilder && receipt.Role != contextreceipt.RoleReviewer {
		return nestedExecutionResult{}, fmt.Errorf("contextowner: execution result requires a builder or reviewer receipt role")
	}
	if receipt.Authority != result.Authority {
		return nestedExecutionResult{}, fmt.Errorf("contextowner: execution result authority contradicts receipt")
	}
	for _, pair := range []struct{ field, left, right string }{
		{"atc_runway", result.ATCRunway, receipt.ATCRunway},
		{"execution_workspace_id", result.ExecutionWorkspaceID, receipt.ExecutionWorkspaceID},
		{"adapter_version", result.AdapterVersion, receipt.AdapterVersion},
		{"input_commit", result.InputCommit, receipt.InputCommit},
		{"input_tree", result.InputTree, receipt.InputTree},
		{"output_commit", result.OutputCommit, receipt.OutputCommit},
		{"output_tree", result.OutputTree, receipt.OutputTree},
		{"terminal_manifest_digest", result.TerminalManifestDigest, receipt.ManifestDigest},
		{"event_chain_root", result.EventChainRoot, receipt.EventChainRoot},
	} {
		if pair.left != pair.right {
			return nestedExecutionResult{}, fmt.Errorf("contextowner: execution result %s contradicts receipt", pair.field)
		}
	}
	if result.Adapter != receipt.Adapter || result.Clean != receipt.Clean ||
		result.TerminalManifestRevision != receipt.TerminalManifestRevision ||
		result.TerminalSourceSequence != receipt.TerminalSourceSequence ||
		result.TerminalGlobalSequence != receipt.TerminalGlobalSequence {
		return nestedExecutionResult{}, fmt.Errorf("contextowner: execution result typed terminal or repository fact contradicts receipt")
	}
	ack, err := contextevent.DecodeReceiptEventAck(bytes.NewReader(frameExact(result.ReceiptEventAck)))
	if err != nil {
		return nestedExecutionResult{}, fmt.Errorf("contextowner: execution result receipt_event_ack: %w", err)
	}
	if ack.Flight != result.Flight || ack.Lane != result.Lane || ack.Epoch != result.Epoch ||
		ack.Session != result.Session || ack.ManifestRevision != result.TerminalManifestRevision ||
		ack.SourceSequence != result.TerminalSourceSequence+1 ||
		ack.GlobalSequence <= result.TerminalGlobalSequence || ack.ReceiptDigest != receipt.Digest {
		return nestedExecutionResult{}, fmt.Errorf("contextowner: execution result receipt_event_ack identity does not match result and receipt")
	}
	switch result.Authority {
	case contextevent.AuthorityAuthoritative:
		if result.Verdict != contextcompile.ResolutionProven || !result.Clean || len(result.Witnesses) != 0 {
			return nestedExecutionResult{}, fmt.Errorf("contextowner: authoritative result requires proven verdict, clean output, and no adverse witnesses")
		}
	case contextevent.AuthorityAdvisory:
		if result.Verdict == contextcompile.ResolutionProven || len(result.Witnesses) == 0 {
			return nestedExecutionResult{}, fmt.Errorf("contextowner: advisory result requires non-proven verdict and explicit witnesses")
		}
	}
	return result, nil
}

// validateNestedExecutionRequest publishes the accepted sealed request
// contract. Every sub-document the request carries is decoded by the component
// that owns it, so those value rules are reused rather than restated; what is
// published here is the request's own member set, value domains, closed
// start/resume union, and the identity cross-matches that bind the manifest,
// projection, workspace, grants, authority verdict, and continuity to one
// dispatch.
func validateNestedExecutionRequest(document []byte) error {
	var request nestedExecutionRequest
	if err := decodeNestedExact(document, &request,
		"schema", "action", "flight", "lane", "epoch", "manifest_revision", "session",
		"atc_runway", "input_commit", "input_tree", "manifest", "manifest_digest",
		"instruction_projection", "projection_digest", "execution_workspace_request",
		"adapter", "adapter_version", "profile", "grants", "authority_verdict",
		"recorder_endpoint"); err != nil {
		return err
	}
	for _, row := range []struct{ field, value string }{
		{"flight", request.Flight}, {"lane", request.Lane}, {"epoch", request.Epoch},
		{"session", request.Session}, {"atc_runway", request.ATCRunway},
		{"adapter_version", request.AdapterVersion},
	} {
		if err := requireText(row.field, row.value); err != nil {
			return err
		}
	}
	if err := validateGitOID("input_commit", request.InputCommit, true); err != nil {
		return err
	}
	if err := validateGitOID("input_tree", request.InputTree, false); err != nil {
		return err
	}
	if err := contextevent.Adapter(request.Adapter).Validate(); err != nil {
		return err
	}
	manifest, err := contextcompile.DecodeManifest(frameExact(request.Manifest))
	if err != nil {
		return fmt.Errorf("contextowner: request manifest: %w", err)
	}
	if manifest.Digest != request.ManifestDigest {
		return fmt.Errorf("contextowner: request manifest_digest does not match canonical manifest")
	}
	projection, err := validateNestedInstructionProjection(request.InstructionProjection)
	if err != nil {
		return err
	}
	if projection.Digest != request.ProjectionDigest {
		return fmt.Errorf("contextowner: request projection_digest does not match canonical instruction projection")
	}
	if len(manifest.ProjectionFiles) != len(projection.Files) {
		return fmt.Errorf("contextowner: instruction projection does not exactly cover manifest projection_files")
	}
	for i := range manifest.ProjectionFiles {
		if manifest.ProjectionFiles[i].Path != projection.Files[i].Path ||
			manifest.ProjectionFiles[i].Digest != projection.Files[i].ContentDigest {
			return fmt.Errorf("contextowner: instruction projection contradicts manifest projection_files")
		}
	}
	if manifest.Adapter.ID != request.Adapter || manifest.Adapter.Version != request.AdapterVersion {
		return fmt.Errorf("contextowner: request adapter contradicts manifest adapter")
	}
	workspace, err := execworkspace.DecodeSidecar(frameExact(request.ExecutionWorkspaceRequest))
	if err != nil {
		return fmt.Errorf("contextowner: request execution_workspace_request: %w", err)
	}
	if workspace.Shape != execworkspace.ExactSHA || workspace.CommitSHA != request.InputCommit {
		return fmt.Errorf("contextowner: request execution_workspace_request must be exact-sha at input_commit")
	}
	wantWorkspace, err := nestedExecutionWorkspaceRequest(request)
	if err != nil {
		return err
	}
	if !workspace.Equal(wantWorkspace) {
		return fmt.Errorf("contextowner: request execution_workspace_request run identity contradicts dispatch tuple")
	}
	if err := validateLogicalRef("profile", request.Profile, ProfileRefSchemaID); err != nil {
		return err
	}
	if err := validateLogicalRef("recorder_endpoint", request.RecorderEndpoint, RecorderEndpointRefSchemaID); err != nil {
		return err
	}
	grants, err := execworkspace.DecodeGrantSet(frameExact(request.Grants))
	if err != nil {
		return fmt.Errorf("contextowner: request grants: %w", err)
	}
	grantBytes, err := execworkspace.EncodeGrantSet(grants)
	if err != nil {
		return err
	}
	manifestGrantBytes, err := execworkspace.EncodeGrantSet(manifest.Capabilities)
	if err != nil {
		return err
	}
	if !bytes.Equal(grantBytes, manifestGrantBytes) {
		return fmt.Errorf("contextowner: request grants contradict manifest capabilities")
	}
	authority, err := policyconflict.DecodeReport(frameExact(request.AuthorityVerdict))
	if err != nil {
		return fmt.Errorf("contextowner: request authority verdict: %w", err)
	}
	if authority.Input.Target.Kind != policyconflict.TargetAcceptedContext ||
		authority.Input.Target.Accepted == nil ||
		authority.Input.Target.Accepted.ManifestDigest != manifest.Digest {
		return fmt.Errorf("contextowner: request authority verdict does not bind manifest")
	}
	return validateNestedExecutionAction(request, manifest, authority.Digest, digestBytes(grantBytes))
}

// nestedExecutionWorkspaceRequest derives the accepted exact-SHA workspace
// identity from the complete dispatch tuple and input commit.
func nestedExecutionWorkspaceRequest(request nestedExecutionRequest) (execworkspace.Identity, error) {
	encoded, err := canonjson.Marshal(struct {
		Flight  string `json:"flight"`
		Lane    string `json:"lane"`
		Epoch   string `json:"epoch"`
		Session string `json:"session"`
	}{request.Flight, request.Lane, request.Epoch, request.Session})
	if err != nil {
		return execworkspace.Identity{}, fmt.Errorf("contextowner: derive execution workspace run id: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return execworkspace.NewExactIdentity("vatc-"+hex.EncodeToString(sum[:]), request.InputCommit)
}

func validateNestedExecutionAction(request nestedExecutionRequest, manifest contextcompile.Manifest,
	authorityDigest, grantDigest string) error {
	switch request.Action {
	case actionStart:
		if request.Start == nil || !rawMissing(request.Resume) || request.Start.ExpectedSourceSequence != 1 {
			return fmt.Errorf("contextowner: start action requires only start.expected_source_sequence=1")
		}
		return nil
	case actionResume:
		if request.Start != nil || rawMissing(request.Resume) {
			return fmt.Errorf("contextowner: resume action requires only the resume arm")
		}
	default:
		return fmt.Errorf("contextowner: unknown execution action %q", request.Action)
	}
	var resume nestedResumeArm
	if err := decodeNestedExact(request.Resume, &resume, "continuity", "continuity_digest"); err != nil {
		return err
	}
	continuity, err := validateNestedContinuity(resume.Continuity)
	if err != nil {
		return err
	}
	if resume.ContinuityDigest != continuity.Digest {
		return fmt.Errorf("contextowner: resume continuity_digest does not match continuity")
	}
	workspace, err := nestedExecutionWorkspaceRequest(request)
	if err != nil {
		return err
	}
	workspaceDigest, err := nestedWorkspaceRequestDigest(workspace)
	if err != nil {
		return err
	}
	workspaceID, err := workspace.WorkspaceID()
	if err != nil {
		return err
	}
	for _, pair := range []struct{ field, left, right string }{
		{"flight", request.Flight, continuity.Flight},
		{"lane", request.Lane, continuity.Lane},
		{"epoch", request.Epoch, continuity.Epoch},
		{"session", request.Session, continuity.Session},
		{"atc_runway", request.ATCRunway, continuity.ATCRunway},
		{"input_commit", request.InputCommit, continuity.InputCommit},
		{"input_tree", request.InputTree, continuity.InputTree},
		{"adapter", request.Adapter, string(continuity.Adapter)},
		{"adapter_version", request.AdapterVersion, continuity.AdapterVersion},
		{"manifest_digest", request.ManifestDigest, continuity.CurrentManifestDigest},
		{"projection_digest", request.ProjectionDigest, continuity.ProjectionDigest},
		{"workspace_request_digest", workspaceDigest, continuity.ExecutionWorkspaceRequestDigest},
		{"workspace_id", workspaceID, continuity.ExecutionWorkspaceID},
		{"profile_digest", request.Profile.Digest, continuity.ProfileDigest},
		{"grant_digest", grantDigest, continuity.GrantDigest},
		{"authority_verdict_digest", authorityDigest, continuity.AuthorityVerdictDigest},
	} {
		if pair.left != pair.right {
			return fmt.Errorf("contextowner: resume continuity %s contradicts request", pair.field)
		}
	}
	if request.ManifestRevision != continuity.CurrentManifestRevision {
		return fmt.Errorf("contextowner: resume continuity manifest revision contradicts request")
	}
	if manifest.Digest != continuity.CurrentManifestDigest {
		return fmt.Errorf("contextowner: resume continuity manifest digest contradicts the canonical manifest")
	}
	return nil
}

// nestedWorkspaceRequestDigest digests the component-owned canonical
// request-sidecar bytes exactly as the accepted contract does.
func nestedWorkspaceRequestDigest(identity execworkspace.Identity) (string, error) {
	encoded, err := execworkspace.EncodeSidecar(identity)
	if err != nil {
		return "", fmt.Errorf("contextowner: encode execution workspace request: %w", err)
	}
	return digestBytes(encoded), nil
}

func validateNestedContinuity(document json.RawMessage) (nestedExecutionContinuity, error) {
	var continuity nestedExecutionContinuity
	if err := decodeNestedExact(document, &continuity,
		"schema", "flight", "lane", "epoch", "session", "adapter", "adapter_version",
		"atc_runway", "input_commit", "input_tree", "current_commit", "current_tree",
		"execution_workspace_id", "execution_workspace_request_digest", "profile_digest",
		"grant_digest", "authority_verdict_digest", "current_manifest_revision",
		"current_manifest_digest", "projection_digest", "revision_segments",
		"event_chain_root", "expansion_ledger_root", "terminal_source_sequence",
		"terminal_global_sequence", "recorder_checkpoint_digest", "adapter_session_ref",
		"digest"); err != nil {
		return nestedExecutionContinuity{}, err
	}
	if continuity.Schema != executionContinuitySchemaID {
		return nestedExecutionContinuity{}, fmt.Errorf("contextowner: continuity schema must be %q", executionContinuitySchemaID)
	}
	for _, row := range []struct{ field, value string }{
		{"flight", continuity.Flight}, {"lane", continuity.Lane}, {"epoch", continuity.Epoch},
		{"session", continuity.Session}, {"adapter_version", continuity.AdapterVersion},
		{"atc_runway", continuity.ATCRunway}, {"execution_workspace_id", continuity.ExecutionWorkspaceID},
		{"adapter_session_ref", continuity.AdapterSessionRef},
	} {
		if err := requireText("continuity "+row.field, row.value); err != nil {
			return nestedExecutionContinuity{}, err
		}
	}
	if err := continuity.Adapter.Validate(); err != nil {
		return nestedExecutionContinuity{}, err
	}
	for _, row := range []struct {
		field  string
		value  string
		commit bool
	}{
		{"input_commit", continuity.InputCommit, true}, {"input_tree", continuity.InputTree, false},
		{"current_commit", continuity.CurrentCommit, false}, {"current_tree", continuity.CurrentTree, false},
	} {
		if err := validateGitOID("continuity "+row.field, row.value, row.commit); err != nil {
			return nestedExecutionContinuity{}, err
		}
	}
	for _, row := range []struct{ field, value string }{
		{"execution_workspace_request_digest", continuity.ExecutionWorkspaceRequestDigest},
		{"profile_digest", continuity.ProfileDigest},
		{"grant_digest", continuity.GrantDigest},
		{"authority_verdict_digest", continuity.AuthorityVerdictDigest},
		{"current_manifest_digest", continuity.CurrentManifestDigest},
		{"projection_digest", continuity.ProjectionDigest},
		{"event_chain_root", continuity.EventChainRoot},
		{"expansion_ledger_root", continuity.ExpansionLedgerRoot},
		{"recorder_checkpoint_digest", continuity.RecorderCheckpointDigest},
	} {
		if err := validateDigest("continuity "+row.field, row.value); err != nil {
			return nestedExecutionContinuity{}, err
		}
	}
	root, err := contextevent.EventChainRoot(continuity.RevisionSegments)
	if err != nil {
		return nestedExecutionContinuity{}, fmt.Errorf("contextowner: continuity revision_segments: %w", err)
	}
	if root != continuity.EventChainRoot {
		return nestedExecutionContinuity{}, fmt.Errorf("contextowner: continuity event_chain_root does not match revision_segments")
	}
	terminal := continuity.RevisionSegments[len(continuity.RevisionSegments)-1]
	if terminal.ManifestRevision != continuity.CurrentManifestRevision ||
		terminal.ManifestDigest != continuity.CurrentManifestDigest ||
		terminal.TerminalSourceSequence != continuity.TerminalSourceSequence ||
		terminal.TerminalGlobalSequence != continuity.TerminalGlobalSequence {
		return nestedExecutionContinuity{}, fmt.Errorf("contextowner: continuity terminal identity does not match final revision")
	}
	if err := validateNestedSelfDigest("continuity", document, continuity.Digest); err != nil {
		return nestedExecutionContinuity{}, err
	}
	return continuity, nil
}

func validateNestedInstructionProjection(document json.RawMessage) (nestedInstructionProjection, error) {
	var projection nestedInstructionProjection
	if err := decodeNestedExact(document, &projection, "schema", "files", "digest"); err != nil {
		return nestedInstructionProjection{}, err
	}
	if projection.Schema != instructionProjectionSchemaID {
		return nestedInstructionProjection{}, fmt.Errorf("contextowner: instruction projection schema must be %q",
			instructionProjectionSchemaID)
	}
	if len(projection.Files) == 0 {
		return nestedInstructionProjection{}, fmt.Errorf("contextowner: instruction projection files must be non-null and nonempty")
	}
	for i, file := range projection.Files {
		if err := validateProjectionPath(file.Path); err != nil {
			return nestedInstructionProjection{}, err
		}
		if !utf8.ValidString(file.Content) {
			return nestedInstructionProjection{}, fmt.Errorf("contextowner: instruction projection file content must be UTF-8")
		}
		if file.ContentDigest != digestBytes([]byte(file.Content)) {
			return nestedInstructionProjection{}, fmt.Errorf("contextowner: instruction projection content_digest does not match content bytes")
		}
		if i > 0 && projection.Files[i-1].Path >= file.Path {
			return nestedInstructionProjection{}, fmt.Errorf("contextowner: instruction projection files must be sorted and deduplicated by path")
		}
	}
	if err := validateNestedSelfDigest("instruction projection", document, projection.Digest); err != nil {
		return nestedInstructionProjection{}, err
	}
	return projection, nil
}

func validateProjectionPath(value string) error {
	if err := requireText("instruction projection path", value); err != nil {
		return err
	}
	if strings.Contains(value, "\\") || strings.HasPrefix(value, "/") ||
		path.Clean(value) != value || value == "." || strings.HasPrefix(value, "../") {
		return fmt.Errorf("contextowner: instruction projection path must be a clean relative slash path")
	}
	return nil
}
