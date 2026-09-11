package sealedexec

import (
	"bytes"
	"encoding/json"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextowner"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/execworkspace"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// Domain-to-public conversions keep owning artifact validation and preserve
// typed values until the public validator checks them, before JSON marshaling.
func clonePublicBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	return append([]byte{}, b...)
}
func executionRequestToPublic(value ExecutionRequest) (json.RawMessage, error) {
	b, err := EncodeExecutionRequest(value)
	if err != nil {
		return nil, err
	}
	return trimFrame(b), nil
}
func executionRequestFromPublic(value json.RawMessage) (ExecutionRequest, error) {
	return DecodeExecutionRequest(bytes.NewReader(frameNested(value)))
}
func controllerEventToPublic(value contextevent.Event) (json.RawMessage, error) {
	b, err := contextevent.EncodeEvent(value)
	if err != nil {
		return nil, err
	}
	return trimFrame(b), nil
}
func controllerEventFromPublic(value json.RawMessage) (contextevent.Event, error) {
	return contextevent.DecodeEvent(bytes.NewReader(frameNested(value)))
}
func conflictReportToPublic(value policyconflict.Report) (json.RawMessage, error) {
	b, err := policyconflict.EncodeReport(value)
	if err != nil {
		return nil, err
	}
	return trimFrame(b), nil
}
func conflictReportFromPublic(value json.RawMessage) (policyconflict.Report, error) {
	return policyconflict.DecodeReport(frameNested(value))
}
func handbackRecordToPublic(value HandbackRecord) (json.RawMessage, error) {
	b, err := EncodeHandbackRecord(value)
	if err != nil {
		return nil, err
	}
	return trimFrame(b), nil
}
func handbackRecordFromPublic(value json.RawMessage) (HandbackRecord, error) {
	return DecodeHandbackRecord(bytes.NewReader(frameNested(value)))
}
func quarantineRecordToPublic(value QuarantineRecord) (json.RawMessage, error) {
	b, err := EncodeQuarantineRecord(value)
	if err != nil {
		return nil, err
	}
	return trimFrame(b), nil
}
func quarantineRecordFromPublic(value json.RawMessage) (QuarantineRecord, error) {
	return DecodeQuarantineRecord(bytes.NewReader(frameNested(value)))
}
func abortRecordToPublic(value AbortRecord) (json.RawMessage, error) {
	b, err := EncodeAbortRecord(value)
	if err != nil {
		return nil, err
	}
	return trimFrame(b), nil
}
func abortRecordFromPublic(value json.RawMessage) (AbortRecord, error) {
	return DecodeAbortRecord(bytes.NewReader(frameNested(value)))
}
func controlAckToPublic(value ControlAck) (json.RawMessage, error) {
	b, err := EncodeControlAck(value)
	if err != nil {
		return nil, err
	}
	return trimFrame(b), nil
}
func controlAckFromPublic(value json.RawMessage) (ControlAck, error) {
	return DecodeControlAck(bytes.NewReader(frameNested(value)))
}
func logicalRefToPublic(value LogicalRef) (contextowner.LogicalRef, error) {
	return contextowner.LogicalRef(value), nil
}
func logicalRefFromPublic(value contextowner.LogicalRef) (LogicalRef, error) {
	return LogicalRef(value), nil
}
func executionKeyToPublic(value ExecutionKey) (contextowner.ExecutionKey, error) {
	return contextowner.ExecutionKey(value), nil
}
func executionKeyFromPublic(value contextowner.ExecutionKey) (ExecutionKey, error) {
	return ExecutionKey(value), nil
}
func textToPublic(value string) (string, error)   { return string(value), nil }
func textFromPublic(value string) (string, error) { return string(value), nil }
func providerSessionCheckToPublic(value ProviderSessionCheck) (contextowner.ProviderSessionCheck, error) {
	return contextowner.ProviderSessionCheck(value), nil
}
func providerSessionCheckFromPublic(value contextowner.ProviderSessionCheck) (ProviderSessionCheck, error) {
	return ProviderSessionCheck(value), nil
}
func receiptInputsToPublic(value ReceiptInputs) (contextowner.ReceiptInputs, error) {
	return contextowner.ReceiptInputs(value), nil
}
func receiptInputsFromPublic(value contextowner.ReceiptInputs) (ReceiptInputs, error) {
	return ReceiptInputs(value), nil
}
func authorityQueryToPublic(value contextreceipt.AuthorityQuery) (contextreceipt.AuthorityQuery, error) {
	return contextreceipt.AuthorityQuery(value), nil
}
func authorityQueryFromPublic(value contextreceipt.AuthorityQuery) (contextreceipt.AuthorityQuery, error) {
	return contextreceipt.AuthorityQuery(value), nil
}
func eventAckToPublic(value contextevent.EventAck) (contextevent.EventAck, error) {
	return canonicalEventAck(value)
}
func eventAckFromPublic(value contextevent.EventAck) (contextevent.EventAck, error) {
	return canonicalEventAck(value)
}
func receiptAckToPublic(value contextevent.ReceiptEventAck) (contextevent.ReceiptEventAck, error) {
	return canonicalReceiptAck(value)
}
func receiptAckFromPublic(value contextevent.ReceiptEventAck) (contextevent.ReceiptEventAck, error) {
	return canonicalReceiptAck(value)
}
func verificationToPublic(value Verification) (contextowner.Verification, error) {
	return contextowner.Verification{State: value.State, Failure: contextowner.FailureCode(value.Failure), Witnesses: value.Witnesses}, nil
}
func verificationFromPublic(value contextowner.Verification) (Verification, error) {
	return Verification{State: value.State, Failure: FailureCode(value.Failure), Witnesses: value.Witnesses}, nil
}
func authorityFactsToPublic(value AuthorityFacts) (contextowner.AuthorityFacts, error) {
	return contextowner.AuthorityFacts{State: value.State, Failure: contextowner.FailureCode(value.Failure), Witnesses: value.Witnesses, ManifestRevision: value.ManifestRevision, ManifestDigest: value.ManifestDigest, ProjectionDigest: value.ProjectionDigest, AuthorityDigest: value.AuthorityDigest, AcceptedSpecCommit: value.AcceptedSpecCommit}, nil
}
func authorityFactsFromPublic(value contextowner.AuthorityFacts) (AuthorityFacts, error) {
	return AuthorityFacts{Verification: Verification{State: value.State, Failure: FailureCode(value.Failure), Witnesses: value.Witnesses}, ManifestRevision: value.ManifestRevision, ManifestDigest: value.ManifestDigest, ProjectionDigest: value.ProjectionDigest, AuthorityDigest: value.AuthorityDigest, AcceptedSpecCommit: value.AcceptedSpecCommit}, nil
}
func expansionFactsToPublic(value ExpansionFacts) (contextowner.ExpansionFacts, error) {
	return contextowner.ExpansionFacts{State: value.State, Failure: contextowner.FailureCode(value.Failure), Witnesses: value.Witnesses, Root: value.Root}, nil
}
func expansionFactsFromPublic(value contextowner.ExpansionFacts) (ExpansionFacts, error) {
	return ExpansionFacts{Verification: Verification{State: value.State, Failure: FailureCode(value.Failure), Witnesses: value.Witnesses}, Root: value.Root}, nil
}
func providerSessionFactsToPublic(value ProviderSessionFacts) (contextowner.ProviderSessionFacts, error) {
	return contextowner.ProviderSessionFacts{State: value.State, Failure: contextowner.FailureCode(value.Failure), Witnesses: value.Witnesses, SessionRef: value.SessionRef, AdapterVersion: value.AdapterVersion, ProfileDigest: value.ProfileDigest, WorkspaceID: value.WorkspaceID}, nil
}
func providerSessionFactsFromPublic(value contextowner.ProviderSessionFacts) (ProviderSessionFacts, error) {
	return ProviderSessionFacts{Verification: Verification{State: value.State, Failure: FailureCode(value.Failure), Witnesses: value.Witnesses}, SessionRef: value.SessionRef, AdapterVersion: value.AdapterVersion, ProfileDigest: value.ProfileDigest, WorkspaceID: value.WorkspaceID}, nil
}
func recorderFactsToPublic(value RecorderFacts) (contextowner.RecorderFacts, error) {
	return contextowner.RecorderFacts{State: value.State, Failure: contextowner.FailureCode(value.Failure), Witnesses: value.Witnesses, Ref: contextowner.LogicalRef(value.Ref)}, nil
}
func recorderFactsFromPublic(value contextowner.RecorderFacts) (RecorderFacts, error) {
	return RecorderFacts{Verification: Verification{value.State, FailureCode(value.Failure), value.Witnesses}, Ref: LogicalRef(value.Ref)}, nil
}
func profileMaterialToPublic(value ProfileMaterial) (contextowner.ProfileMaterial, error) {
	return contextowner.ProfileMaterial{Ref: contextowner.LogicalRef(value.Ref), Name: value.Name, AbsoluteExecutable: value.AbsoluteExecutable, AbsoluteEnvRoot: value.AbsoluteEnvRoot, AbsoluteCodexHome: value.AbsoluteCodexHome, Model: value.Model, ClaudeConfigDir: value.ClaudeConfigDir, AdapterVersion: value.AdapterVersion, DecoderProfile: value.DecoderProfile}, nil
}
func profileMaterialFromPublic(value contextowner.ProfileMaterial) (ProfileMaterial, error) {
	return ProfileMaterial{Ref: LogicalRef(value.Ref), Name: value.Name, AbsoluteExecutable: value.AbsoluteExecutable, AbsoluteEnvRoot: value.AbsoluteEnvRoot, AbsoluteCodexHome: value.AbsoluteCodexHome, Model: value.Model, ClaudeConfigDir: value.ClaudeConfigDir, AdapterVersion: value.AdapterVersion, DecoderProfile: value.DecoderProfile}, nil
}
func redactedSegmentToPublic(value RedactedSegment) (contextowner.RedactedSegment, error) {
	return contextowner.RedactedSegment{Schema: value.Schema, MediaType: value.MediaType, RedactionProfile: value.RedactionProfile, Digest: value.Digest, ByteCount: value.ByteCount, Bytes: clonePublicBytes(value.Bytes)}, nil
}
func redactedSegmentFromPublic(value contextowner.RedactedSegment) (RedactedSegment, error) {
	return RedactedSegment{Schema: value.Schema, MediaType: value.MediaType, RedactionProfile: value.RedactionProfile, Digest: value.Digest, ByteCount: value.ByteCount, Bytes: clonePublicBytes(value.Bytes)}, nil
}
func storedSegmentToPublic(value StoredSegment) (contextowner.StoredSegment, error) {
	return contextowner.StoredSegment{Schema: value.Schema, Reference: value.Reference, MediaType: value.MediaType, RedactionProfile: value.RedactionProfile, Digest: value.Digest, ByteCount: value.ByteCount}, nil
}
func storedSegmentFromPublic(value contextowner.StoredSegment) (StoredSegment, error) {
	return StoredSegment{Schema: value.Schema, Reference: value.Reference, MediaType: value.MediaType, RedactionProfile: value.RedactionProfile, Digest: value.Digest, ByteCount: value.ByteCount}, nil
}
func profileQueryToPublic(value ProfileQuery) (contextowner.ProfileQuery, error) {
	grants, err := execworkspace.EncodeGrantSet(value.Grants)
	if err != nil {
		return contextowner.ProfileQuery{}, err
	}
	return contextowner.ProfileQuery{Ref: contextowner.LogicalRef(value.Ref), WorkspacePath: value.WorkspacePath, Grants: trimFrame(grants)}, nil
}
func profileQueryFromPublic(value contextowner.ProfileQuery) (ProfileQuery, error) {
	grants, err := execworkspace.DecodeGrantSet(frameNested(value.Grants))
	if err != nil {
		return ProfileQuery{}, err
	}
	return ProfileQuery{Ref: LogicalRef(value.Ref), WorkspacePath: value.WorkspacePath, Grants: grants}, nil
}
func conflictFactsToPublic(value ConflictFacts) (contextowner.ConflictFacts, error) {
	report, err := policyconflict.EncodeReport(value.Report)
	if err != nil {
		return contextowner.ConflictFacts{}, err
	}
	return contextowner.ConflictFacts{State: value.State, Failure: contextowner.FailureCode(value.Failure), Witnesses: value.Witnesses, Report: trimFrame(report)}, nil
}
func conflictFactsFromPublic(value contextowner.ConflictFacts) (ConflictFacts, error) {
	report, err := policyconflict.DecodeReport(frameNested(value.Report))
	if err != nil {
		return ConflictFacts{}, err
	}
	return ConflictFacts{Verification: Verification{value.State, FailureCode(value.Failure), value.Witnesses}, Report: report}, nil
}
func opaqueEntriesToPublic(value []contextcompile.OpaqueEntry) ([]contextcompile.OpaqueEntry, error) {
	return value, nil
}
func opaqueEntriesFromPublic(value []contextcompile.OpaqueEntry) ([]contextcompile.OpaqueEntry, error) {
	return value, nil
}
func opaqueFactsToPublic(value OpaqueBoundaryFacts) (contextowner.OpaqueBoundaryFacts, error) {
	var rows []contextowner.OpaqueIdentity
	if value.Rows != nil {
		rows = make([]contextowner.OpaqueIdentity, len(value.Rows))
		for i, row := range value.Rows {
			rows[i] = contextowner.OpaqueIdentity(row)
		}
	}
	return contextowner.OpaqueBoundaryFacts{State: value.State, Failure: contextowner.FailureCode(value.Failure), Witnesses: value.Witnesses, Rows: rows}, nil
}
func opaqueFactsFromPublic(value contextowner.OpaqueBoundaryFacts) (OpaqueBoundaryFacts, error) {
	var rows []OpaqueIdentity
	if value.Rows != nil {
		rows = make([]OpaqueIdentity, len(value.Rows))
		for i, row := range value.Rows {
			rows[i] = OpaqueIdentity(row)
		}
	}
	return OpaqueBoundaryFacts{Verification: Verification{value.State, FailureCode(value.Failure), value.Witnesses}, Rows: rows}, nil
}
func recorderCheckpointToPublic(value RecorderCheckpoint) (contextowner.RecorderCheckpoint, error) {
	var active *contextowner.ActiveRevision
	if value.ActiveRevision != nil {
		converted := contextowner.ActiveRevision(*value.ActiveRevision)
		active = &converted
	}
	return contextowner.RecorderCheckpoint{State: value.State, Failure: contextowner.FailureCode(value.Failure), Witnesses: value.Witnesses, Digest: value.Digest, Revisions: value.Revisions, EventChainRoot: value.EventChainRoot, TerminalSourceSequence: value.TerminalSourceSequence, TerminalGlobalSequence: value.TerminalGlobalSequence, ActiveRevision: active}, nil
}
func recorderCheckpointFromPublic(value contextowner.RecorderCheckpoint) (RecorderCheckpoint, error) {
	var active *ActiveRevision
	if value.ActiveRevision != nil {
		converted := ActiveRevision(*value.ActiveRevision)
		active = &converted
	}
	return RecorderCheckpoint{Verification: Verification{value.State, FailureCode(value.Failure), value.Witnesses}, Digest: value.Digest, Revisions: value.Revisions, EventChainRoot: value.EventChainRoot, TerminalSourceSequence: value.TerminalSourceSequence, TerminalGlobalSequence: value.TerminalGlobalSequence, ActiveRevision: active}, nil
}
func sessionRecordToPublic(value SessionRecord) (contextowner.SessionRecord, error) {
	ack, err := canonicalEventAck(value.LifecycleAck)
	if err != nil {
		return contextowner.SessionRecord{}, err
	}
	return contextowner.SessionRecord{Key: contextowner.ExecutionKey(value.Key), SessionRef: value.SessionRef, AdapterVersion: value.AdapterVersion, ProfileDigest: value.ProfileDigest, WorkspaceID: value.WorkspaceID, LifecycleAck: ack}, nil
}
func sessionRecordFromPublic(value contextowner.SessionRecord) (SessionRecord, error) {
	ack, err := canonicalEventAck(value.LifecycleAck)
	if err != nil {
		return SessionRecord{}, err
	}
	return SessionRecord{Key: ExecutionKey(value.Key), SessionRef: value.SessionRef, AdapterVersion: value.AdapterVersion, ProfileDigest: value.ProfileDigest, WorkspaceID: value.WorkspaceID, LifecycleAck: ack}, nil
}
func contextQueryToPublic(value ContextQuery) (contextowner.ContextQuery, error) {
	return contextowner.ContextQuery{Key: contextowner.ExecutionKey(value.Key), Ref: value.Ref}, nil
}
func contextQueryFromPublic(value contextowner.ContextQuery) (ContextQuery, error) {
	return ContextQuery{Key: ExecutionKey(value.Key), Ref: value.Ref}, nil
}
func contextResolutionToPublic(value ContextResolution) (contextowner.ContextResolution, error) {
	if err := contextcompile.RequireDataOnlyWhenProven("sealedexec", value.State, value.Data != (contextcompile.DataItem{})); err != nil {
		return contextowner.ContextResolution{}, err
	}
	var data json.RawMessage
	if value.State == contextcompile.ResolutionProven {
		b, err := contextcompile.EncodeDataItem(value.Data)
		if err != nil {
			return contextowner.ContextResolution{}, err
		}
		data = trimFrame(b)
	}
	return contextowner.ContextResolution{State: value.State, Failure: contextowner.FailureCode(value.Failure), Witnesses: value.Witnesses, Ref: value.Ref, Data: data}, nil
}
func contextResolutionFromPublic(value contextowner.ContextResolution) (ContextResolution, error) {
	var data contextcompile.DataItem
	if value.State == contextcompile.ResolutionProven {
		var err error
		data, err = contextcompile.DecodeDataItem(frameNested(value.Data))
		if err != nil {
			return ContextResolution{}, err
		}
	}
	return ContextResolution{Verification: Verification{value.State, FailureCode(value.Failure), value.Witnesses}, Ref: value.Ref, Data: data}, nil
}
func flightSnapshotToPublic(value FlightStateSnapshot) (contextowner.FlightStateSnapshot, error) {
	request, err := executionRequestToPublic(value.Request)
	if err != nil {
		return contextowner.FlightStateSnapshot{}, err
	}
	return contextowner.FlightStateSnapshot{Request: request, Key: contextowner.ExecutionKey(value.Key), WorkspaceID: value.WorkspaceID, CandidateCommit: value.CandidateCommit, CandidateTree: value.CandidateTree, Revision: value.Revision, ManifestDigest: value.ManifestDigest, ProjectionDigest: value.ProjectionDigest, ExpansionRoot: value.ExpansionRoot, NextSourceSequence: value.NextSourceSequence, PriorEventDigest: value.PriorEventDigest, PriorRevision: value.PriorRevision, LastGlobalSequence: value.LastGlobalSequence, Invalidated: value.Invalidated}, nil
}
func flightSnapshotFromPublic(value contextowner.FlightStateSnapshot) (FlightStateSnapshot, error) {
	request, err := executionRequestFromPublic(value.Request)
	if err != nil {
		return FlightStateSnapshot{}, err
	}
	return FlightStateSnapshot{Request: request, Key: ExecutionKey(value.Key), WorkspaceID: value.WorkspaceID, CandidateCommit: value.CandidateCommit, CandidateTree: value.CandidateTree, Revision: value.Revision, ManifestDigest: value.ManifestDigest, ProjectionDigest: value.ProjectionDigest, ExpansionRoot: value.ExpansionRoot, NextSourceSequence: value.NextSourceSequence, PriorEventDigest: value.PriorEventDigest, PriorRevision: value.PriorRevision, LastGlobalSequence: value.LastGlobalSequence, Invalidated: value.Invalidated}, nil
}
func epochCheckToPublic(value EpochCheck) (contextowner.EpochCheck, error) {
	snapshot, err := flightSnapshotToPublic(value.Snapshot)
	if err != nil {
		return contextowner.EpochCheck{}, err
	}
	resolution, err := contextResolutionToPublic(value.Resolution)
	if err != nil {
		return contextowner.EpochCheck{}, err
	}
	return contextowner.EpochCheck{Snapshot: snapshot, Resolution: resolution}, nil
}
func epochCheckFromPublic(value contextowner.EpochCheck) (EpochCheck, error) {
	snapshot, err := flightSnapshotFromPublic(value.Snapshot)
	if err != nil {
		return EpochCheck{}, err
	}
	resolution, err := contextResolutionFromPublic(value.Resolution)
	if err != nil {
		return EpochCheck{}, err
	}
	return EpochCheck{Snapshot: snapshot, Resolution: resolution}, nil
}
func expansionInstallToPublic(value ExpansionInstall) (contextowner.ExpansionInstall, error) {
	encoded, err := contextcompile.EncodeDataItem(value.Data)
	data := trimFrame(encoded)
	if err != nil {
		return contextowner.ExpansionInstall{}, err
	}
	ack, err := canonicalEventAck(value.TerminalAck)
	if err != nil {
		return contextowner.ExpansionInstall{}, err
	}
	return contextowner.ExpansionInstall{Key: contextowner.ExecutionKey(value.Key), RequestID: value.RequestID, ParentRevision: value.ParentRevision, ParentManifestDigest: value.ParentManifestDigest, ChildRevision: value.ChildRevision, ChildManifestDigest: value.ChildManifestDigest, ExpansionDigest: value.ExpansionDigest, ExpansionRoot: value.ExpansionRoot, Ref: value.Ref, Purpose: value.Purpose, TerminalAck: ack, Data: data}, nil
}
func expansionInstallFromPublic(value contextowner.ExpansionInstall) (ExpansionInstall, error) {
	data, err := contextcompile.DecodeDataItem(frameNested(value.Data))
	if err != nil {
		return ExpansionInstall{}, err
	}
	ack, err := canonicalEventAck(value.TerminalAck)
	if err != nil {
		return ExpansionInstall{}, err
	}
	return ExpansionInstall{Key: ExecutionKey(value.Key), RequestID: value.RequestID, ParentRevision: value.ParentRevision, ParentManifestDigest: value.ParentManifestDigest, ChildRevision: value.ChildRevision, ChildManifestDigest: value.ChildManifestDigest, ExpansionDigest: value.ExpansionDigest, ExpansionRoot: value.ExpansionRoot, Ref: value.Ref, Purpose: value.Purpose, TerminalAck: ack, Data: data}, nil
}
func receiptInputsQueryToPublic(value ReceiptInputsQuery) (contextowner.ReceiptInputsQuery, error) {
	request, err := executionRequestToPublic(value.Request)
	if err != nil {
		return contextowner.ReceiptInputsQuery{}, err
	}
	return contextowner.ReceiptInputsQuery{Request: request, WorkspaceID: value.WorkspaceID, DispatchDigest: value.DispatchDigest, TerminalRevision: value.TerminalRevision, TerminalSourceSequence: value.TerminalSourceSequence, TerminalGlobalSequence: value.TerminalGlobalSequence, EventChainRoot: value.EventChainRoot, ResultFactsDigest: value.ResultFactsDigest}, nil
}
func receiptInputsQueryFromPublic(value contextowner.ReceiptInputsQuery) (ReceiptInputsQuery, error) {
	request, err := executionRequestFromPublic(value.Request)
	if err != nil {
		return ReceiptInputsQuery{}, err
	}
	return ReceiptInputsQuery{Request: request, WorkspaceID: value.WorkspaceID, DispatchDigest: value.DispatchDigest, TerminalRevision: value.TerminalRevision, TerminalSourceSequence: value.TerminalSourceSequence, TerminalGlobalSequence: value.TerminalGlobalSequence, EventChainRoot: value.EventChainRoot, ResultFactsDigest: value.ResultFactsDigest}, nil
}
func receiptAppendToPublic(value ReceiptAppend) (contextowner.ReceiptAppend, error) {
	receipt, err := contextreceipt.EncodeReceipt(value.Receipt)
	if err != nil {
		return contextowner.ReceiptAppend{}, err
	}
	event, err := contextevent.EncodeEvent(value.Event)
	if err != nil {
		return contextowner.ReceiptAppend{}, err
	}
	return contextowner.ReceiptAppend{Receipt: trimFrame(receipt), Event: trimFrame(event)}, nil
}
func receiptAppendFromPublic(value contextowner.ReceiptAppend) (ReceiptAppend, error) {
	receipt, err := contextreceipt.DecodeReceipt(bytes.NewReader(frameNested(value.Receipt)))
	if err != nil {
		return ReceiptAppend{}, err
	}
	event, err := contextevent.DecodeEvent(bytes.NewReader(frameNested(value.Event)))
	if err != nil {
		return ReceiptAppend{}, err
	}
	return ReceiptAppend{Receipt: receipt, Event: event}, nil
}
func receiptVerificationAuthorityToPublic(value contextreceipt.AuthorityFacts) (contextowner.ReceiptVerificationAuthority, error) {
	return contextowner.ReceiptVerificationAuthority{Profile: value.Profile, TrustFact: contextowner.ReceiptVerificationTrustFact{SourceID: value.TrustFact.SourceID, SourceKind: value.TrustFact.SourceKind, Subjects: value.TrustFact.Subjects, EvidenceDigest: value.TrustFact.EvidenceDigest, Available: value.TrustFact.Available, Valid: value.TrustFact.Valid, Reason: value.TrustFact.Reason}, Isolation: value.Isolation, Persistence: value.Persistence}, nil
}
func receiptVerificationAuthorityFromPublic(value contextowner.ReceiptVerificationAuthority) (contextreceipt.AuthorityFacts, error) {
	return contextreceipt.AuthorityFacts{Profile: value.Profile, TrustFact: gp.TrustFact{SourceID: value.TrustFact.SourceID, SourceKind: value.TrustFact.SourceKind, Subjects: value.TrustFact.Subjects, EvidenceDigest: value.TrustFact.EvidenceDigest, Available: value.TrustFact.Available, Valid: value.TrustFact.Valid, Reason: value.TrustFact.Reason}, Isolation: value.Isolation, Persistence: value.Persistence}, nil
}

// Nontransport callers use the same pure fragment predicates before any wire
// conversion or effects; domain execution-request validation stays local.
func validateExecutionKey(key ExecutionKey) error {
	return contextowner.ValidateExecutionKey(contextowner.ExecutionKey(key))
}
func segmentReference(digest string) (string, error) { return contextowner.SegmentReference(digest) }
func validateSegmentReference(reference string) error {
	return contextowner.ValidateSegmentReference(reference)
}
func validateDomainSnapshot(snapshot FlightStateSnapshot) error {
	public, err := flightSnapshotToPublic(snapshot)
	if err != nil {
		return err
	}
	return contextowner.ValidateFlightStateSnapshot(public)
}
func canonicalDomainSegment(segment RedactedSegment) (RedactedSegment, error) {
	public, err := redactedSegmentToPublic(segment)
	if err != nil {
		return RedactedSegment{}, err
	}
	if err = contextowner.ValidateRedactedSegment(public); err != nil {
		return RedactedSegment{}, err
	}
	return redactedSegmentFromPublic(public)
}
