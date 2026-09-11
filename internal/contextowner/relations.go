package contextowner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// ErrRelationMismatch marks a structurally valid reply whose result
// contradicts one of the accepted contract's fixed request/result identity
// relations (contract §3). A structural decode failure never wraps this
// error (I-89's usability distinction): only a relation-specific mismatch
// does, so a caller can tell the two apart with errors.Is.
var ErrRelationMismatch = errors.New("contextowner: request/result relation mismatch")

// relationOperations is the exact 16-member subset of Operations() the
// accepted contract binds one result member to a request member for
// (contract §3's table). Naming them here, rather than deriving the set
// residually from ValidateRelations' switch, is what lets
// TestHasRelations_Classification catch the two definitions drifting apart.
var relationOperations = map[Operation]bool{
	OperationVerifyAuthority:                     true,
	OperationResolveProfile:                      true,
	OperationVerifyConflict:                      true,
	OperationResolveRecorder:                     true,
	OperationRecorderCheckpoint:                  true,
	OperationRecorderAppend:                      true,
	OperationStoreRedactedSegment:                true,
	OperationResolveRedactedSegment:              true,
	OperationVerifyOpaqueBoundary:                true,
	OperationVerifyProviderSession:               true,
	OperationResolveContext:                      true,
	OperationAppendReceipt:                       true,
	OperationResolveReceiptVerificationAuthority: true,
	OperationPersistHandback:                     true,
	OperationPersistQuarantine:                   true,
	OperationPersistAbort:                        true,
}

// HasRelations reports whether the accepted contract binds one of
// operation's result members to a request member. It is true for exactly
// the 16 operations named above; the remaining six of the 22 published
// operations (verify-expansion, store-adapter-session, next-stamp,
// verify-epoch, install-expansion, resolve-receipt-inputs) answer with a
// fresh fact or a bare acknowledgment and carry no such relation, and an
// operation outside the closed registry has none either.
func HasRelations(operation Operation) bool {
	return relationOperations[operation]
}

// ValidateRelations re-runs, over the published wire, every request/result
// identity relation the accepted controller contract owns -- the one
// reusable Verdi validator contract §3 requires for both the runtime client
// and the retained bridge CLI. It is nil for the six no-relation arms; a
// relation failure wraps ErrRelationMismatch, names the operation and a
// closed relation class, and never echoes an operand value.
//
// The switch is deliberately exhaustive over all 22 published operations,
// mirroring internal/sealedexec's crossMatchOwnerResult: the six no-relation
// operations are named explicitly rather than falling through a default, so
// an operation this package's registry ever grows to include cannot slip
// through unclassified.
//
// Precondition: reply must be one DecodeReply or NewReply produced. This
// function validates relations only; it never re-runs structural validation,
// so a hand-built Reply literal that skipped the decode is not structurally
// validated here and can pass vacuously rather than erroring -- a zero-valued
// call and result compare two empty refs equal, a nil active revision carries
// no acknowledgment to check, and zero declared rows match zero reported
// identities. A caller that checks relations without first decoding therefore
// proves nothing about the reply.
func ValidateRelations(reply Reply) error {
	switch operation := reply.Call.Operation; operation {
	case OperationVerifyAuthority:
		return validateVerifyAuthorityRelation(reply)
	case OperationResolveProfile:
		return validateResolveProfileRelation(reply)
	case OperationVerifyConflict:
		return validateVerifyConflictRelation(reply)
	case OperationResolveRecorder:
		return validateResolveRecorderRelation(reply)
	case OperationRecorderCheckpoint:
		return validateRecorderCheckpointRelation(reply)
	case OperationRecorderAppend:
		return validateRecorderAppendRelation(reply)
	case OperationStoreRedactedSegment:
		return validateStoreRedactedSegmentRelation(reply)
	case OperationResolveRedactedSegment:
		return validateResolveRedactedSegmentRelation(reply)
	case OperationVerifyOpaqueBoundary:
		return validateVerifyOpaqueBoundaryRelation(reply)
	case OperationVerifyProviderSession:
		return validateVerifyProviderSessionRelation(reply)
	case OperationResolveContext:
		return validateResolveContextRelation(reply)
	case OperationAppendReceipt:
		return validateAppendReceiptRelation(reply)
	case OperationResolveReceiptVerificationAuthority:
		return validateResolveReceiptVerificationAuthorityRelation(reply)
	case OperationPersistHandback:
		return validatePersistHandbackRelation(reply)
	case OperationPersistQuarantine:
		return validatePersistQuarantineRelation(reply)
	case OperationPersistAbort:
		return validatePersistAbortRelation(reply)
	case OperationVerifyExpansion, OperationStoreAdapterSession, OperationNextStamp,
		OperationVerifyEpoch, OperationInstallExpansion, OperationResolveReceiptInputs:
		// The accepted contract binds no result member of these six
		// operations to a request member: each answers with a fresh fact or
		// a bare acknowledgment, so there is no relation to validate.
		return nil
	default:
		return fmt.Errorf("contextowner: validate relations: unknown owner operation %q", operation)
	}
}

// relationMismatch names operation and a fixed, closed relation class --
// never an operand value -- and wraps ErrRelationMismatch.
func relationMismatch(operation Operation, class string) error {
	return fmt.Errorf("contextowner: %s: %s: %w", operation, class, ErrRelationMismatch)
}

// validateVerifyAuthorityRelation reproduces the accepted verify-authority
// relation: the freshly recomputed facts' manifest revision/digest,
// projection digest, authority digest, and accepted-spec commit must equal
// the request's.
func validateVerifyAuthorityRelation(reply Reply) error {
	var request struct {
		ManifestRevision uint64          `json:"manifest_revision"`
		ManifestDigest   string          `json:"manifest_digest"`
		ProjectionDigest string          `json:"projection_digest"`
		Manifest         json.RawMessage `json:"manifest"`
		AuthorityVerdict json.RawMessage `json:"authority_verdict"`
	}
	if err := json.Unmarshal(reply.Call.VerifyAuthority.Request, &request); err != nil {
		return fmt.Errorf("contextowner: %s: decode request: %w", OperationVerifyAuthority, err)
	}
	var manifest struct {
		AcceptedSpec struct {
			Commit string `json:"commit"`
		} `json:"accepted_spec"`
	}
	if err := json.Unmarshal(request.Manifest, &manifest); err != nil {
		return fmt.Errorf("contextowner: %s: decode request manifest: %w", OperationVerifyAuthority, err)
	}
	var authorityVerdict struct {
		Digest string `json:"digest"`
	}
	if err := json.Unmarshal(request.AuthorityVerdict, &authorityVerdict); err != nil {
		return fmt.Errorf("contextowner: %s: decode request authority_verdict: %w", OperationVerifyAuthority, err)
	}
	facts := reply.VerifyAuthority.Facts
	if facts.ManifestRevision != request.ManifestRevision || facts.ManifestDigest != request.ManifestDigest ||
		facts.ProjectionDigest != request.ProjectionDigest || facts.AuthorityDigest != authorityVerdict.Digest ||
		facts.AcceptedSpecCommit != manifest.AcceptedSpec.Commit {
		return relationMismatch(OperationVerifyAuthority, "authority facts contradict the request")
	}
	return nil
}

// validateResolveProfileRelation reproduces the accepted resolve-profile
// relation: the resolved profile material's ref must equal the query's ref.
func validateResolveProfileRelation(reply Reply) error {
	if reply.ResolveProfile.Material.Ref != reply.Call.ResolveProfile.Query.Ref {
		return relationMismatch(OperationResolveProfile, "profile material ref contradicts the query")
	}
	return nil
}

// validateVerifyConflictRelation reproduces the accepted verify-conflict
// relation: the adjudicated report must be exactly the submitted report,
// checked both as raw canonical bytes and as the report's own canonical
// re-encoding.
func validateVerifyConflictRelation(reply Reply) error {
	requestReport := reply.Call.VerifyConflict.Report
	resultReport := reply.VerifyConflict.Facts.Report
	if !bytes.Equal(requestReport, resultReport) {
		return relationMismatch(OperationVerifyConflict, "conflict facts report contradicts the request")
	}
	decodedRequest, err := policyconflict.DecodeReport(frameExact(requestReport))
	if err != nil {
		return fmt.Errorf("contextowner: %s: decode request report: %w", OperationVerifyConflict, err)
	}
	decodedResult, err := policyconflict.DecodeReport(frameExact(resultReport))
	if err != nil {
		return fmt.Errorf("contextowner: %s: decode result report: %w", OperationVerifyConflict, err)
	}
	requestBytes, err := policyconflict.EncodeReport(decodedRequest)
	if err != nil {
		return fmt.Errorf("contextowner: %s: encode request report: %w", OperationVerifyConflict, err)
	}
	resultBytes, err := policyconflict.EncodeReport(decodedResult)
	if err != nil {
		return fmt.Errorf("contextowner: %s: encode result report: %w", OperationVerifyConflict, err)
	}
	if !bytes.Equal(requestBytes, resultBytes) {
		return relationMismatch(OperationVerifyConflict, "conflict facts report contradicts the request")
	}
	return nil
}

// validateResolveRecorderRelation reproduces the accepted resolve-recorder
// relation: the resolved recorder facts' ref must equal the request's ref.
func validateResolveRecorderRelation(reply Reply) error {
	if reply.ResolveRecorder.Facts.Ref != reply.Call.ResolveRecorder.Ref {
		return relationMismatch(OperationResolveRecorder, "recorder ref contradicts the request")
	}
	return nil
}

// validateRecorderCheckpointRelation reproduces the accepted
// recorder-checkpoint relation: every acknowledgment in the checkpoint's
// active revision, when present, must carry the request's exact execution
// key. A null active revision carries no acknowledgment and trivially
// passes.
func validateRecorderCheckpointRelation(reply Reply) error {
	key := reply.Call.RecorderCheckpoint.Key
	active := reply.RecorderCheckpoint.Checkpoint.ActiveRevision
	if active == nil {
		return nil
	}
	for _, ack := range active.EventAcks {
		if ack.Flight != key.Flight || ack.Lane != key.Lane || ack.Epoch != key.Epoch {
			return relationMismatch(OperationRecorderCheckpoint, "active revision acknowledgment contradicts the execution key")
		}
	}
	return nil
}

// validateRecorderAppendRelation reproduces the accepted recorder-append
// relation (incumbent validateAck with priorGlobal=0): the acknowledgment,
// canonically re-encoded, must bind exactly the appended event's identity
// and carry a positive global sequence.
func validateRecorderAppendRelation(reply Reply) error {
	event, err := contextevent.DecodeEvent(bytes.NewReader(frameExact(reply.Call.RecorderAppend.Event)))
	if err != nil {
		return fmt.Errorf("contextowner: %s: decode request event: %w", OperationRecorderAppend, err)
	}
	ack, err := canonicalEventAck(reply.RecorderAppend.Ack)
	if err != nil {
		return fmt.Errorf("contextowner: %s: canonicalize result acknowledgment: %w", OperationRecorderAppend, err)
	}
	if ack.Flight != event.Flight || ack.Lane != event.Lane || ack.Epoch != event.Epoch ||
		ack.Session != event.Session || ack.ManifestRevision != event.ManifestRevision ||
		ack.Kind != event.Kind || ack.SourceSequence != event.SourceSequence ||
		ack.EventDigest != event.EventDigest || ack.GlobalSequence == 0 {
		return relationMismatch(OperationRecorderAppend, "event acknowledgment does not bind the appended event")
	}
	return nil
}

// validateStoreRedactedSegmentRelation reproduces the accepted
// store-redacted-segment relation: the stored segment's reference must
// derive from the submitted segment's digest, and every other stored field
// must equal the submitted segment's.
func validateStoreRedactedSegmentRelation(reply Reply) error {
	segment := reply.Call.StoreRedactedSegment.Segment
	stored := reply.StoreRedactedSegment.Stored
	reference, err := segmentReference(segment.Digest)
	if err != nil {
		return fmt.Errorf("contextowner: %s: request segment reference: %w", OperationStoreRedactedSegment, err)
	}
	if stored.Reference != reference || stored.MediaType != segment.MediaType ||
		stored.RedactionProfile != segment.RedactionProfile || stored.ByteCount != segment.ByteCount ||
		stored.Digest != segment.Digest {
		return relationMismatch(OperationStoreRedactedSegment, "stored segment contradicts the request")
	}
	return nil
}

// validateResolveRedactedSegmentRelation reproduces the accepted
// resolve-redacted-segment relation: the reference derived from the
// resolved segment's digest must equal the requested reference.
func validateResolveRedactedSegmentRelation(reply Reply) error {
	reference, err := segmentReference(reply.ResolveRedactedSegment.Segment.Digest)
	if err != nil {
		return fmt.Errorf("contextowner: %s: result segment reference: %w", OperationResolveRedactedSegment, err)
	}
	if reference != reply.Call.ResolveRedactedSegment.Reference {
		return relationMismatch(OperationResolveRedactedSegment, "resolved segment contradicts the reference")
	}
	return nil
}

// validateVerifyOpaqueBoundaryRelation reproduces the accepted
// verify-opaque-boundary relation (incumbent opaqueFactsMatchRows): the
// facts must carry exactly one identity per declared row, in the same
// order, with matching id/kind/adapter id/adapter version.
func validateVerifyOpaqueBoundaryRelation(reply Reply) error {
	rows := reply.Call.VerifyOpaqueBoundary.Rows
	identities := reply.VerifyOpaqueBoundary.Facts.Rows
	if len(rows) != len(identities) {
		return relationMismatch(OperationVerifyOpaqueBoundary, "opaque identities contradict the rows")
	}
	for i := range rows {
		if identities[i].ID != rows[i].ID || identities[i].Kind != rows[i].Kind ||
			identities[i].AdapterID != rows[i].Adapter.ID || identities[i].AdapterVersion != rows[i].Adapter.Version {
			return relationMismatch(OperationVerifyOpaqueBoundary, "opaque identities contradict the rows")
		}
	}
	return nil
}

// validateVerifyProviderSessionRelation reproduces the accepted
// verify-provider-session relation: the proven facts must equal the
// checked session_ref/adapter_version/profile_digest/workspace_id.
func validateVerifyProviderSessionRelation(reply Reply) error {
	check := reply.Call.VerifyProviderSession.Check
	facts := reply.VerifyProviderSession.Facts
	if facts.SessionRef != check.SessionRef || facts.AdapterVersion != check.AdapterVersion ||
		facts.ProfileDigest != check.ProfileDigest || facts.WorkspaceID != check.WorkspaceID {
		return relationMismatch(OperationVerifyProviderSession, "provider-session facts contradict the check")
	}
	return nil
}

// validateResolveContextRelation reproduces the accepted resolve-context
// relation: the resolution's ref must equal the query's ref.
func validateResolveContextRelation(reply Reply) error {
	if reply.ResolveContext.Resolution.Ref != reply.Call.ResolveContext.Query.Ref {
		return relationMismatch(OperationResolveContext, "context resolution ref contradicts the query")
	}
	return nil
}

// validateAppendReceiptRelation reproduces the accepted append-receipt
// relation (incumbent validateReceiptAppendAck): the acknowledgment,
// canonically re-encoded, must bind exactly the appended event's identity
// and the appended receipt's digest.
func validateAppendReceiptRelation(reply Reply) error {
	appendValue := reply.Call.AppendReceipt.Append
	event, err := contextevent.DecodeEvent(bytes.NewReader(frameExact(appendValue.Event)))
	if err != nil {
		return fmt.Errorf("contextowner: %s: decode request event: %w", OperationAppendReceipt, err)
	}
	receipt, err := contextreceipt.DecodeReceipt(bytes.NewReader(frameExact(appendValue.Receipt)))
	if err != nil {
		return fmt.Errorf("contextowner: %s: decode request receipt: %w", OperationAppendReceipt, err)
	}
	ack, err := canonicalReceiptAck(reply.AppendReceipt.Ack)
	if err != nil {
		return fmt.Errorf("contextowner: %s: canonicalize result acknowledgment: %w", OperationAppendReceipt, err)
	}
	if ack.Flight != event.Flight || ack.Lane != event.Lane || ack.Epoch != event.Epoch ||
		ack.Session != event.Session || ack.ManifestRevision != event.ManifestRevision ||
		ack.Kind != event.Kind || ack.SourceSequence != event.SourceSequence ||
		ack.EventDigest != event.EventDigest || ack.ReceiptDigest != receipt.Digest {
		return relationMismatch(OperationAppendReceipt, "receipt acknowledgment does not bind the exact receipt event identity")
	}
	return nil
}

// validateResolveReceiptVerificationAuthorityRelation reproduces the
// accepted resolve-receipt-verification-authority relation: the trust
// fact's source must equal the runner claim's trust source; a proven
// isolation must name the queried profile; and, when present, the
// persisted receipt digest must equal the queried one.
func validateResolveReceiptVerificationAuthorityRelation(reply Reply) error {
	query := reply.Call.ResolveReceiptVerificationAuthority.Query
	authority := reply.ResolveReceiptVerificationAuthority.Authority
	if authority.TrustFact.SourceID != query.RunnerClaim.TrustSource {
		return relationMismatch(OperationResolveReceiptVerificationAuthority, "trust fact source contradicts the runner claim")
	}
	if authority.Isolation.State == contextreceipt.StateProven &&
		(authority.Isolation.ProfileID != query.ProfileRef.ID || authority.Isolation.ProfileDigest != query.ProfileRef.Digest) {
		return relationMismatch(OperationResolveReceiptVerificationAuthority, "isolation profile contradicts the query")
	}
	if authority.Persistence.ReceiptDigest != "" && authority.Persistence.ReceiptDigest != query.ReceiptDigest {
		return relationMismatch(OperationResolveReceiptVerificationAuthority, "persistence receipt contradicts the query")
	}
	return nil
}

// validatePersistHandbackRelation reproduces the accepted persist-handback
// relation (incumbent ValidateHandbackAck / validateMatchingControlAck).
func validatePersistHandbackRelation(reply Reply) error {
	var record nestedHandbackRecord
	if err := json.Unmarshal(reply.Call.PersistHandback.Record, &record); err != nil {
		return fmt.Errorf("contextowner: %s: decode request record: %w", OperationPersistHandback, err)
	}
	return validateControlAckRelation(OperationPersistHandback, HandbackRecordSchemaID, record.Digest,
		record.Flight, record.Lane, record.Epoch, record.Session, record.WorkspaceID,
		dispositionFastForwarded, reply.PersistHandback.Ack)
}

// validatePersistQuarantineRelation reproduces the accepted
// persist-quarantine relation. crossMatchOwnerResult re-runs the
// preserved-bytes/record preservation relation before checking the
// acknowledgment even though the request arm already enforces it
// structurally on every encode (encodeRequestArm's PersistQuarantine case);
// this keeps exact parity with the incumbent bridge rather than assuming
// that structural enforcement upstream makes the second check
// unreachable.
func validatePersistQuarantineRelation(reply Reply) error {
	call := reply.Call.PersistQuarantine
	record, err := decodeNestedQuarantine(call.Record)
	if err != nil {
		return fmt.Errorf("contextowner: %s: decode request record: %w", OperationPersistQuarantine, err)
	}
	if err := validateQuarantinePreservation(call.Record, call.PreservedBytes); err != nil {
		return relationMismatch(OperationPersistQuarantine, "quarantine preservation contradicts the record")
	}
	return validateControlAckRelation(OperationPersistQuarantine, QuarantineRecordSchemaID, record.Digest,
		record.Flight, record.Lane, record.Epoch, record.Session, record.WorkspaceID,
		dispositionQuarantined, reply.PersistQuarantine.Ack)
}

// validatePersistAbortRelation reproduces the accepted persist-abort
// relation (incumbent ValidateAbortAck / validateMatchingControlAck).
func validatePersistAbortRelation(reply Reply) error {
	var record nestedAbortRecord
	if err := json.Unmarshal(reply.Call.PersistAbort.Record, &record); err != nil {
		return fmt.Errorf("contextowner: %s: decode request record: %w", OperationPersistAbort, err)
	}
	return validateControlAckRelation(OperationPersistAbort, AbortRecordSchemaID, record.Digest,
		record.Flight, record.Lane, record.Epoch, record.Session, record.WorkspaceID,
		dispositionAbortPreserve, reply.PersistAbort.Ack)
}

// validateControlAckRelation reproduces the accepted persist-* relation
// shared by handback, quarantine, and abort (incumbent
// validateMatchingControlAck): the durable acknowledgment must bind exactly
// the persisted record's schema, digest, execution identity, and
// disposition. Each document's own structural validity -- including the
// acknowledgment's self-digest -- is already enforced by the public arm
// decode, so this checks only the cross-document relation between them.
func validateControlAckRelation(operation Operation, recordSchema, recordDigest,
	flight, lane, epoch, session, workspaceID, disposition string, ackDocument json.RawMessage) error {
	var ack nestedControlAck
	if err := json.Unmarshal(ackDocument, &ack); err != nil {
		return fmt.Errorf("contextowner: %s: decode result acknowledgment: %w", operation, err)
	}
	if ack.RecordSchema != recordSchema || ack.RecordDigest != recordDigest || ack.Flight != flight ||
		ack.Lane != lane || ack.Epoch != epoch || ack.Session != session || ack.WorkspaceID != workspaceID ||
		ack.Disposition != disposition {
		return relationMismatch(operation, "control acknowledgment does not bind the persisted record")
	}
	return nil
}
