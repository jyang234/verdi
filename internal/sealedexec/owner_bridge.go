// The public owner bridge: the one translation seam between Verdi's private
// FD-3 controller payloads and the deliberately public owner wire published in
// internal/contextowner (VATC F12 controller-owner bridge correction §3,
// sibling invention-ledger row SI-176).
//
// The bridge translates. It does not decide, persist, launch, or supervise
// anything: a call and a reply carry no authority, and neither function here
// reads a store, opens FD 3, consults a clock, or touches a file. Authority
// continues to come from the committed plan, fresh repository facts, the
// configured profile, and the durable stores an ATC owner actually holds.
//
// The mapping is §3.3's mechanical publication rule applied to bytes rather
// than to types: the public arm IS the accepted private payload with its one
// top-level schema literal replaced. Expressing it that way — instead of
// restating 44 field lists in a second projection — is what makes a private
// refactor that leaves the projection unchanged require no bridge change, and
// what makes a projection drift impossible to introduce quietly.
package sealedexec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextowner"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/policyconflict"
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

// ownerCallFrame is the public call envelope's exact member set. It exists to
// serialize a candidate document INTO contextowner.DecodeCall, which remains
// the sole validator of the public wire; nothing here decides whether a call
// is legal, and no second operation registry is introduced.
type ownerCallFrame struct {
	Schema                  string                 `json:"schema"`
	Operation               contextowner.Operation `json:"operation"`
	ControllerRequestDigest string                 `json:"controller_request_digest"`
	Request                 json.RawMessage        `json:"request"`
}

// OwnerBridgeOperation resolves one caller-supplied operand against the closed
// published registry, returning the controller operation it names.
//
// Both registries must accept it. They hold the same 22 names by construction,
// and requiring agreement is what turns a future divergence into a refusal
// here rather than into a call no owner can answer.
func OwnerBridgeOperation(name string) (ControllerOperation, bool) {
	operation := ControllerOperation(name)
	if !containsControllerOperation(operation) {
		return "", false
	}
	for _, published := range contextowner.Operations() {
		if published == contextowner.Operation(operation) {
			return operation, true
		}
	}
	return "", false
}

// DecodeOwnerCall publishes one private controller request payload as the
// public owner call an ATC owner answers (§3.1).
//
// privateRequest is the standalone canonical encoding of the payload that
// crossed the validated FD-3 controller envelope, including its one trailing
// LF. Those exact bytes are what the returned digest binds, so the payload is
// required to already be canonical rather than repaired into canonical form: a
// bridge that silently re-canonicalized its input would publish a digest for
// bytes the controller never sent.
func DecodeOwnerCall(operation ControllerOperation, privateRequest []byte) (contextowner.Call, error) {
	published, ok := OwnerBridgeOperation(string(operation))
	if !ok {
		return contextowner.Call{}, ownerRefusal(ErrOwnerRequestRefused, "operation is not published on the owner wire")
	}
	public := contextowner.Operation(published)

	_, payload, err := ownerPrivateRequest(published, privateRequest)
	if err != nil {
		return contextowner.Call{}, err
	}
	arm, err := republishSchema(payload, controllerRequestSchema(published), contextowner.RequestSchema(public))
	if err != nil {
		return contextowner.Call{}, ownerRefusal(ErrOwnerRequestRefused, "request payload does not declare its controller schema")
	}
	document, err := canonjson.Marshal(ownerCallFrame{
		Schema:                  contextowner.CallSchemaID,
		Operation:               public,
		ControllerRequestDigest: digestBytes(privateRequest),
		Request:                 arm,
	})
	if err != nil {
		return contextowner.Call{}, ownerRefusal(ErrOwnerRequestRefused, "public owner call could not be encoded")
	}
	// contextowner owns the public contract: it revalidates every published
	// member and requires the document to be byte-canonical, so a projection
	// this bridge got wrong cannot leave here as a well-formed call.
	call, err := contextowner.DecodeCall(bytes.NewReader(document))
	if err != nil {
		return contextowner.Call{}, ownerRefusal(ErrOwnerRequestRefused, "published call is not a valid public owner call")
	}
	return call, nil
}

// EncodeOwnerReply encodes one public owner reply as the exact canonical
// private result payload the existing FD-3 codec expects (§3.2).
//
// The nested call gives this function the complete request context without a
// daemon, token registry, temporary file, or hidden process memory: the
// private request is mechanically reconstructed from the published request
// arm, its digest is recomputed, and every private request/result identity
// relation the controller contract owns is re-run. A reply cannot be accepted
// for another call merely because both operations have the same name.
func EncodeOwnerReply(operation ControllerOperation, reply contextowner.Reply) ([]byte, error) {
	published, ok := OwnerBridgeOperation(string(operation))
	if !ok {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "operation is not published on the owner wire")
	}
	public := contextowner.Operation(published)
	if reply.Call.Operation != public {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "nested call operation contradicts the invoked operation")
	}

	replyBytes, err := contextowner.EncodeReply(reply)
	if err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "reply is not a valid public owner reply")
	}
	if len(replyBytes) >= OwnerFrameCeiling {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "reply reaches the controller frame ceiling")
	}
	callBytes, err := contextowner.EncodeCall(reply.Call)
	if err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "nested call is not a valid public owner call")
	}
	requestArm, err := ownerMember(callBytes, "request")
	if err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "nested call carries no request arm")
	}
	resultArm, err := ownerMember(replyBytes, "result")
	if err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "reply carries no result arm")
	}

	// The private request is rebuilt from the published arm and re-validated
	// through the owning private codec, then its digest is recomputed over the
	// standalone bytes. Without this recomputation a well-formed reply
	// carrying another call's request would be encoded as if the owner had
	// answered this one.
	requestPayload, err := republishSchema(requestArm, contextowner.RequestSchema(public), controllerRequestSchema(published))
	if err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "request arm does not declare its published schema")
	}
	privateRequest := frameNested(requestPayload)
	if digestBytes(privateRequest) != reply.Call.ControllerRequestDigest {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "recomputed controller request digest does not bind the nested call")
	}
	call, _, err := ownerPrivateRequest(published, privateRequest)
	if err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "reconstructed private request is not a valid controller request")
	}

	resultPayload, err := republishSchema(resultArm, contextowner.ResultSchema(public), controllerResultSchema(published))
	if err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "result arm does not declare its published schema")
	}
	result := ControllerResult{Operation: published}
	if err := decodeControllerSuccessPayload(resultPayload, &result); err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "result arm is not a valid private controller result")
	}
	encoded, err := encodeControllerSuccessPayload(result)
	if err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "private controller result could not be encoded")
	}
	privateResult := frameNested(encoded)
	if !bytes.Equal(privateResult, frameNested(resultPayload)) {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "result arm is not byte-canonical")
	}
	if err := crossMatchOwnerResult(call, result); err != nil {
		return nil, ownerRefusal(ErrOwnerReplyRefused, err.Error())
	}
	if len(privateResult) >= OwnerFrameCeiling {
		return nil, ownerRefusal(ErrOwnerReplyRefused, "private result reaches the controller frame ceiling")
	}
	return privateResult, nil
}

// ownerPrivateRequest validates one standalone private request payload through
// the owning controller codec and returns both the typed call and the exact
// canonical payload bytes.
func ownerPrivateRequest(operation ControllerOperation, privateRequest []byte) (ControllerCall, []byte, error) {
	if len(privateRequest) == 0 {
		return ControllerCall{}, nil, ownerRefusal(ErrOwnerRequestRefused, "request payload is empty")
	}
	if len(privateRequest) >= OwnerFrameCeiling {
		return ControllerCall{}, nil, ownerRefusal(ErrOwnerRequestRefused, "request payload reaches the controller frame ceiling")
	}
	if !bytes.HasSuffix(privateRequest, []byte("\n")) || bytes.HasSuffix(privateRequest, []byte("\n\n")) {
		return ControllerCall{}, nil, ownerRefusal(ErrOwnerRequestRefused, "request payload does not carry exactly one trailing LF")
	}
	call := ControllerCall{Operation: operation}
	if err := decodeControllerCallPayload(trimFrame(privateRequest), &call); err != nil {
		return ControllerCall{}, nil, ownerRefusal(ErrOwnerRequestRefused, "request payload is not a valid private controller request")
	}
	payload, err := encodeControllerCallPayload(call)
	if err != nil {
		return ControllerCall{}, nil, ownerRefusal(ErrOwnerRequestRefused, "request payload could not be re-encoded canonically")
	}
	canonical := frameNested(payload)
	if !bytes.Equal(canonical, privateRequest) {
		return ControllerCall{}, nil, ownerRefusal(ErrOwnerRequestRefused, "request payload is not byte-canonical")
	}
	return call, canonical, nil
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

// ownerMember extracts one member of a canonical document this package just
// produced. The bytes are already proven canonical by the codec that emitted
// them, so this is decomposition rather than a second, looser validation of
// untrusted input.
func ownerMember(document []byte, name string) (json.RawMessage, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSuffix(document, []byte("\n")), &members); err != nil {
		return nil, err
	}
	value, ok := members[name]
	if !ok || rawMissing(value) {
		return nil, fmt.Errorf("sealedexec: %s is absent or null", name)
	}
	return value, nil
}

// crossMatchOwnerResult re-runs every private request/result identity relation
// the accepted controller contract owns, exactly as ControllerClient enforces
// them on the sealed side.
//
// Running them here does not remove either endpoint's own checks: the sealed
// client still validates its final request/result identity, and this bridge
// refuses a contradicting pair before the private result is ever framed. The
// six operations in the final case carry no such relation in the accepted
// contract; naming them keeps the switch exhaustive rather than silent.
func crossMatchOwnerResult(call ControllerCall, result ControllerResult) error {
	switch call.Operation {
	case ControllerOperationVerifyAuthority:
		request, facts := call.VerifyAuthority.Request, result.VerifyAuthority.Facts
		if facts.ManifestRevision != request.ManifestRevision || facts.ManifestDigest != request.ManifestDigest ||
			facts.ProjectionDigest != request.ProjectionDigest || facts.AuthorityDigest != request.AuthorityVerdict.Digest ||
			facts.AcceptedSpecCommit != request.Manifest.AcceptedSpec.Commit {
			return errors.New("authority facts contradict the request")
		}
	case ControllerOperationResolveProfile:
		if result.ResolveProfile.Material.Ref != call.ResolveProfile.Query.Ref {
			return errors.New("profile material ref contradicts the query")
		}
	case ControllerOperationVerifyConflict:
		requestBytes, requestErr := policyconflict.EncodeReport(call.VerifyConflict.Report)
		resultBytes, resultErr := policyconflict.EncodeReport(result.VerifyConflict.Facts.Report)
		if requestErr != nil || resultErr != nil || !bytes.Equal(requestBytes, resultBytes) {
			return errors.New("conflict facts report contradicts the request")
		}
	case ControllerOperationResolveRecorder:
		if result.ResolveRecorder.Facts.Ref != call.ResolveRecorder.Ref {
			return errors.New("recorder ref contradicts the request")
		}
	case ControllerOperationRecorderCheckpoint:
		key := call.RecorderCheckpoint.Key
		if active := result.RecorderCheckpoint.Checkpoint.ActiveRevision; active != nil {
			for _, ack := range active.EventAcks {
				if ack.Flight != key.Flight || ack.Lane != key.Lane || ack.Epoch != key.Epoch {
					return errors.New("active revision acknowledgment contradicts the execution key")
				}
			}
		}
	case ControllerOperationRecorderAppend:
		if err := validateAck(call.RecorderAppend.Event, result.RecorderAppend.Ack, 0); err != nil {
			return errors.New("event acknowledgment does not bind the appended event")
		}
	case ControllerOperationStoreRedactedSegment:
		segment, stored := call.StoreRedactedSegment.Segment, result.StoreRedactedSegment.Stored
		wantReference, err := segmentReference(segment.Digest)
		if err != nil || stored.Reference != wantReference || stored.MediaType != segment.MediaType ||
			stored.RedactionProfile != segment.RedactionProfile || stored.ByteCount != segment.ByteCount ||
			stored.Digest != segment.Digest {
			return errors.New("stored segment contradicts the request")
		}
	case ControllerOperationResolveRedactedSegment:
		wantReference, err := segmentReference(result.ResolveRedactedSegment.Segment.Digest)
		if err != nil || wantReference != call.ResolveRedactedSegment.Reference {
			return errors.New("resolved segment contradicts the reference")
		}
	case ControllerOperationVerifyOpaqueBoundary:
		if !opaqueFactsMatchRows(call.VerifyOpaqueBoundary.Rows, result.VerifyOpaqueBoundary.Facts.Rows) {
			return errors.New("opaque identities contradict the rows")
		}
	case ControllerOperationVerifyProviderSession:
		check, facts := call.VerifyProviderSession.Check, result.VerifyProviderSession.Facts
		if facts.SessionRef != check.SessionRef || facts.AdapterVersion != check.AdapterVersion ||
			facts.ProfileDigest != check.ProfileDigest || facts.WorkspaceID != check.WorkspaceID {
			return errors.New("provider-session facts contradict the check")
		}
	case ControllerOperationResolveContext:
		if result.ResolveContext.Resolution.Ref != call.ResolveContext.Query.Ref {
			return errors.New("context resolution ref contradicts the query")
		}
	case ControllerOperationAppendReceipt:
		if err := validateReceiptAppendAck(call.AppendReceipt.Append, result.AppendReceipt.Ack); err != nil {
			return errors.New("receipt acknowledgment does not bind the exact receipt event identity")
		}
	case ControllerOperationResolveReceiptVerificationAuthority:
		query := call.ResolveReceiptVerificationAuthority.Query
		authority := result.ResolveReceiptVerificationAuthority.Authority
		switch {
		case authority.TrustFact.SourceID != query.RunnerClaim.TrustSource:
			return errors.New("trust fact source contradicts the runner claim")
		case authority.Isolation.State == contextreceipt.StateProven &&
			(authority.Isolation.ProfileID != query.ProfileRef.ID || authority.Isolation.ProfileDigest != query.ProfileRef.Digest):
			return errors.New("isolation profile contradicts the query")
		case authority.Persistence.ReceiptDigest != "" && authority.Persistence.ReceiptDigest != query.ReceiptDigest:
			return errors.New("persistence receipt contradicts the query")
		}
	case ControllerOperationPersistHandback:
		if err := ValidateHandbackAck(call.PersistHandback.Record, result.PersistHandback.Ack); err != nil {
			return errors.New("handback acknowledgment does not bind the record")
		}
	case ControllerOperationPersistQuarantine:
		if err := ValidateQuarantinePreservation(call.PersistQuarantine.Record, call.PersistQuarantine.PreservedBytes); err != nil {
			return errors.New("quarantine preservation contradicts the record")
		}
		if err := ValidateQuarantineAck(call.PersistQuarantine.Record, result.PersistQuarantine.Ack); err != nil {
			return errors.New("quarantine acknowledgment does not bind the record")
		}
	case ControllerOperationPersistAbort:
		if err := ValidateAbortAck(call.PersistAbort.Record, result.PersistAbort.Ack); err != nil {
			return errors.New("abort acknowledgment does not bind the record")
		}
	case ControllerOperationVerifyExpansion, ControllerOperationStoreAdapterSession, ControllerOperationNextStamp,
		ControllerOperationVerifyEpoch, ControllerOperationInstallExpansion, ControllerOperationResolveReceiptInputs:
		// The accepted contract binds no result member of these operations to
		// a request member: their answer is a fresh fact or a bare
		// acknowledgment, and inventing a relation here would be new
		// controller semantics rather than a re-run of an existing check.
	}
	return nil
}
