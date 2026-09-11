package sealedexec

import (
	"encoding/json"
	"fmt"

	"github.com/jyang234/verdi/internal/contextowner"
)

// These conversions retain typed execution-domain inputs and owning artifact
// codecs. Only contextowner arm values define operation serialization.
func controllerCallToOwner(value ControllerCall) (contextowner.Call, error) {
	owner := contextowner.Call{Operation: contextowner.Operation(value.Operation)}
	switch value.Operation {
	case ControllerOperationVerifyAuthority:
		if err := requireOnlyCallArm(value, value.VerifyAuthority); err != nil {
			return owner, err
		}
		if value.VerifyAuthority.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := executionRequestToPublic(value.VerifyAuthority.Request)
		if err != nil {
			return owner, err
		}
		owner.VerifyAuthority = contextowner.VerifyAuthorityRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Request: converted}
	case ControllerOperationResolveProfile:
		if err := requireOnlyCallArm(value, value.ResolveProfile); err != nil {
			return owner, err
		}
		if value.ResolveProfile.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := profileQueryToPublic(value.ResolveProfile.Query)
		if err != nil {
			return owner, err
		}
		owner.ResolveProfile = contextowner.ResolveProfileRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Query: converted}
	case ControllerOperationVerifyConflict:
		if err := requireOnlyCallArm(value, value.VerifyConflict); err != nil {
			return owner, err
		}
		if value.VerifyConflict.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := conflictReportToPublic(value.VerifyConflict.Report)
		if err != nil {
			return owner, err
		}
		owner.VerifyConflict = contextowner.VerifyConflictRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Report: converted}
	case ControllerOperationResolveRecorder:
		if err := requireOnlyCallArm(value, value.ResolveRecorder); err != nil {
			return owner, err
		}
		if value.ResolveRecorder.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := logicalRefToPublic(value.ResolveRecorder.Ref)
		if err != nil {
			return owner, err
		}
		owner.ResolveRecorder = contextowner.ResolveRecorderRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Ref: converted}
	case ControllerOperationRecorderCheckpoint:
		if err := requireOnlyCallArm(value, value.RecorderCheckpoint); err != nil {
			return owner, err
		}
		if value.RecorderCheckpoint.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := executionKeyToPublic(value.RecorderCheckpoint.Key)
		if err != nil {
			return owner, err
		}
		owner.RecorderCheckpoint = contextowner.RecorderCheckpointRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Key: converted}
	case ControllerOperationRecorderAppend:
		if err := requireOnlyCallArm(value, value.RecorderAppend); err != nil {
			return owner, err
		}
		if value.RecorderAppend.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := controllerEventToPublic(value.RecorderAppend.Event)
		if err != nil {
			return owner, err
		}
		owner.RecorderAppend = contextowner.RecorderAppendRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Event: converted}
	case ControllerOperationStoreRedactedSegment:
		if err := requireOnlyCallArm(value, value.StoreRedactedSegment); err != nil {
			return owner, err
		}
		if value.StoreRedactedSegment.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := redactedSegmentToPublic(value.StoreRedactedSegment.Segment)
		if err != nil {
			return owner, err
		}
		owner.StoreRedactedSegment = contextowner.StoreRedactedSegmentRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Segment: converted}
	case ControllerOperationResolveRedactedSegment:
		if err := requireOnlyCallArm(value, value.ResolveRedactedSegment); err != nil {
			return owner, err
		}
		if value.ResolveRedactedSegment.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := textToPublic(value.ResolveRedactedSegment.Reference)
		if err != nil {
			return owner, err
		}
		owner.ResolveRedactedSegment = contextowner.ResolveRedactedSegmentRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Reference: converted}
	case ControllerOperationVerifyOpaqueBoundary:
		if err := requireOnlyCallArm(value, value.VerifyOpaqueBoundary); err != nil {
			return owner, err
		}
		if value.VerifyOpaqueBoundary.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := opaqueEntriesToPublic(value.VerifyOpaqueBoundary.Rows)
		if err != nil {
			return owner, err
		}
		owner.VerifyOpaqueBoundary = contextowner.VerifyOpaqueBoundaryRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Rows: converted}
	case ControllerOperationVerifyProviderSession:
		if err := requireOnlyCallArm(value, value.VerifyProviderSession); err != nil {
			return owner, err
		}
		if value.VerifyProviderSession.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := providerSessionCheckToPublic(value.VerifyProviderSession.Check)
		if err != nil {
			return owner, err
		}
		owner.VerifyProviderSession = contextowner.VerifyProviderSessionRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Check: converted}
	case ControllerOperationVerifyExpansion:
		if err := requireOnlyCallArm(value, value.VerifyExpansion); err != nil {
			return owner, err
		}
		if value.VerifyExpansion.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := executionKeyToPublic(value.VerifyExpansion.Key)
		if err != nil {
			return owner, err
		}
		owner.VerifyExpansion = contextowner.VerifyExpansionRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Key: converted}
	case ControllerOperationStoreAdapterSession:
		if err := requireOnlyCallArm(value, value.StoreAdapterSession); err != nil {
			return owner, err
		}
		if value.StoreAdapterSession.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := sessionRecordToPublic(value.StoreAdapterSession.Record)
		if err != nil {
			return owner, err
		}
		owner.StoreAdapterSession = contextowner.StoreAdapterSessionRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Record: converted}
	case ControllerOperationNextStamp:
		if err := requireOnlyCallArm(value, value.NextStamp); err != nil {
			return owner, err
		}
		if value.NextStamp.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		owner.NextStamp = contextowner.NextStampRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation))}
	case ControllerOperationResolveContext:
		if err := requireOnlyCallArm(value, value.ResolveContext); err != nil {
			return owner, err
		}
		if value.ResolveContext.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := contextQueryToPublic(value.ResolveContext.Query)
		if err != nil {
			return owner, err
		}
		owner.ResolveContext = contextowner.ResolveContextRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Query: converted}
	case ControllerOperationVerifyEpoch:
		if err := requireOnlyCallArm(value, value.VerifyEpoch); err != nil {
			return owner, err
		}
		if value.VerifyEpoch.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := epochCheckToPublic(value.VerifyEpoch.Check)
		if err != nil {
			return owner, err
		}
		owner.VerifyEpoch = contextowner.VerifyEpochRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Check: converted}
	case ControllerOperationInstallExpansion:
		if err := requireOnlyCallArm(value, value.InstallExpansion); err != nil {
			return owner, err
		}
		if value.InstallExpansion.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := expansionInstallToPublic(value.InstallExpansion.Install)
		if err != nil {
			return owner, err
		}
		owner.InstallExpansion = contextowner.InstallExpansionRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Install: converted}
	case ControllerOperationResolveReceiptInputs:
		if err := requireOnlyCallArm(value, value.ResolveReceiptInputs); err != nil {
			return owner, err
		}
		if value.ResolveReceiptInputs.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := receiptInputsQueryToPublic(value.ResolveReceiptInputs.Query)
		if err != nil {
			return owner, err
		}
		owner.ResolveReceiptInputs = contextowner.ResolveReceiptInputsRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Query: converted}
	case ControllerOperationAppendReceipt:
		if err := requireOnlyCallArm(value, value.AppendReceipt); err != nil {
			return owner, err
		}
		if value.AppendReceipt.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := receiptAppendToPublic(value.AppendReceipt.Append)
		if err != nil {
			return owner, err
		}
		owner.AppendReceipt = contextowner.AppendReceiptRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Append: converted}
	case ControllerOperationResolveReceiptVerificationAuthority:
		if err := requireOnlyCallArm(value, value.ResolveReceiptVerificationAuthority); err != nil {
			return owner, err
		}
		if value.ResolveReceiptVerificationAuthority.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := authorityQueryToPublic(value.ResolveReceiptVerificationAuthority.Query)
		if err != nil {
			return owner, err
		}
		owner.ResolveReceiptVerificationAuthority = contextowner.ResolveReceiptVerificationAuthorityRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Query: converted}
	case ControllerOperationPersistHandback:
		if err := requireOnlyCallArm(value, value.PersistHandback); err != nil {
			return owner, err
		}
		if value.PersistHandback.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := handbackRecordToPublic(value.PersistHandback.Record)
		if err != nil {
			return owner, err
		}
		owner.PersistHandback = contextowner.PersistHandbackRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Record: converted}
	case ControllerOperationPersistQuarantine:
		if err := requireOnlyCallArm(value, value.PersistQuarantine); err != nil {
			return owner, err
		}
		if value.PersistQuarantine.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := quarantineRecordToPublic(value.PersistQuarantine.Record)
		if err != nil {
			return owner, err
		}
		if err := ValidateQuarantinePreservation(value.PersistQuarantine.Record, value.PersistQuarantine.PreservedBytes); err != nil {
			return owner, err
		}
		owner.PersistQuarantine = contextowner.PersistQuarantineRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Record: converted, PreservedBytes: clonePublicBytes(value.PersistQuarantine.PreservedBytes)}
	case ControllerOperationPersistAbort:
		if err := requireOnlyCallArm(value, value.PersistAbort); err != nil {
			return owner, err
		}
		if value.PersistAbort.Schema != controllerRequestSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := abortRecordToPublic(value.PersistAbort.Record)
		if err != nil {
			return owner, err
		}
		owner.PersistAbort = contextowner.PersistAbortRequest{Schema: contextowner.RequestSchema(contextowner.Operation(value.Operation)), Record: converted}
	default:
		return owner, fmt.Errorf("sealedexec: operation has no public owner arm")
	}
	return owner, nil
}

func controllerCallFromOwner(owner contextowner.Call, value *ControllerCall) error {
	switch value.Operation {
	case ControllerOperationVerifyAuthority:
		converted, err := executionRequestFromPublic(owner.VerifyAuthority.Request)
		if err != nil {
			return err
		}
		value.VerifyAuthority = ControllerVerifyAuthorityRequest{Schema: controllerRequestSchema(value.Operation), Request: converted}
	case ControllerOperationResolveProfile:
		converted, err := profileQueryFromPublic(owner.ResolveProfile.Query)
		if err != nil {
			return err
		}
		value.ResolveProfile = ControllerResolveProfileRequest{Schema: controllerRequestSchema(value.Operation), Query: converted}
	case ControllerOperationVerifyConflict:
		converted, err := conflictReportFromPublic(owner.VerifyConflict.Report)
		if err != nil {
			return err
		}
		value.VerifyConflict = ControllerVerifyConflictRequest{Schema: controllerRequestSchema(value.Operation), Report: converted}
	case ControllerOperationResolveRecorder:
		converted, err := logicalRefFromPublic(owner.ResolveRecorder.Ref)
		if err != nil {
			return err
		}
		value.ResolveRecorder = ControllerResolveRecorderRequest{Schema: controllerRequestSchema(value.Operation), Ref: converted}
	case ControllerOperationRecorderCheckpoint:
		converted, err := executionKeyFromPublic(owner.RecorderCheckpoint.Key)
		if err != nil {
			return err
		}
		value.RecorderCheckpoint = ControllerRecorderCheckpointRequest{Schema: controllerRequestSchema(value.Operation), Key: converted}
	case ControllerOperationRecorderAppend:
		converted, err := controllerEventFromPublic(owner.RecorderAppend.Event)
		if err != nil {
			return err
		}
		value.RecorderAppend = ControllerRecorderAppendRequest{Schema: controllerRequestSchema(value.Operation), Event: converted}
	case ControllerOperationStoreRedactedSegment:
		converted, err := redactedSegmentFromPublic(owner.StoreRedactedSegment.Segment)
		if err != nil {
			return err
		}
		value.StoreRedactedSegment = ControllerStoreRedactedSegmentRequest{Schema: controllerRequestSchema(value.Operation), Segment: converted}
	case ControllerOperationResolveRedactedSegment:
		converted, err := textFromPublic(owner.ResolveRedactedSegment.Reference)
		if err != nil {
			return err
		}
		value.ResolveRedactedSegment = ControllerResolveRedactedSegmentRequest{Schema: controllerRequestSchema(value.Operation), Reference: converted}
	case ControllerOperationVerifyOpaqueBoundary:
		converted, err := opaqueEntriesFromPublic(owner.VerifyOpaqueBoundary.Rows)
		if err != nil {
			return err
		}
		value.VerifyOpaqueBoundary = ControllerVerifyOpaqueBoundaryRequest{Schema: controllerRequestSchema(value.Operation), Rows: converted}
	case ControllerOperationVerifyProviderSession:
		converted, err := providerSessionCheckFromPublic(owner.VerifyProviderSession.Check)
		if err != nil {
			return err
		}
		value.VerifyProviderSession = ControllerVerifyProviderSessionRequest{Schema: controllerRequestSchema(value.Operation), Check: converted}
	case ControllerOperationVerifyExpansion:
		converted, err := executionKeyFromPublic(owner.VerifyExpansion.Key)
		if err != nil {
			return err
		}
		value.VerifyExpansion = ControllerVerifyExpansionRequest{Schema: controllerRequestSchema(value.Operation), Key: converted}
	case ControllerOperationStoreAdapterSession:
		converted, err := sessionRecordFromPublic(owner.StoreAdapterSession.Record)
		if err != nil {
			return err
		}
		value.StoreAdapterSession = ControllerStoreAdapterSessionRequest{Schema: controllerRequestSchema(value.Operation), Record: converted}
	case ControllerOperationNextStamp:
		value.NextStamp = ControllerNextStampRequest{Schema: controllerRequestSchema(value.Operation)}
	case ControllerOperationResolveContext:
		converted, err := contextQueryFromPublic(owner.ResolveContext.Query)
		if err != nil {
			return err
		}
		value.ResolveContext = ControllerResolveContextRequest{Schema: controllerRequestSchema(value.Operation), Query: converted}
	case ControllerOperationVerifyEpoch:
		converted, err := epochCheckFromPublic(owner.VerifyEpoch.Check)
		if err != nil {
			return err
		}
		value.VerifyEpoch = ControllerVerifyEpochRequest{Schema: controllerRequestSchema(value.Operation), Check: converted}
	case ControllerOperationInstallExpansion:
		converted, err := expansionInstallFromPublic(owner.InstallExpansion.Install)
		if err != nil {
			return err
		}
		value.InstallExpansion = ControllerInstallExpansionRequest{Schema: controllerRequestSchema(value.Operation), Install: converted}
	case ControllerOperationResolveReceiptInputs:
		converted, err := receiptInputsQueryFromPublic(owner.ResolveReceiptInputs.Query)
		if err != nil {
			return err
		}
		value.ResolveReceiptInputs = ControllerResolveReceiptInputsRequest{Schema: controllerRequestSchema(value.Operation), Query: converted}
	case ControllerOperationAppendReceipt:
		converted, err := receiptAppendFromPublic(owner.AppendReceipt.Append)
		if err != nil {
			return err
		}
		value.AppendReceipt = ControllerAppendReceiptRequest{Schema: controllerRequestSchema(value.Operation), Append: converted}
	case ControllerOperationResolveReceiptVerificationAuthority:
		converted, err := authorityQueryFromPublic(owner.ResolveReceiptVerificationAuthority.Query)
		if err != nil {
			return err
		}
		value.ResolveReceiptVerificationAuthority = ControllerResolveReceiptVerificationAuthorityRequest{Schema: controllerRequestSchema(value.Operation), Query: converted}
	case ControllerOperationPersistHandback:
		converted, err := handbackRecordFromPublic(owner.PersistHandback.Record)
		if err != nil {
			return err
		}
		value.PersistHandback = ControllerPersistHandbackRequest{Schema: controllerRequestSchema(value.Operation), Record: converted}
	case ControllerOperationPersistQuarantine:
		converted, err := quarantineRecordFromPublic(owner.PersistQuarantine.Record)
		if err != nil {
			return err
		}
		value.PersistQuarantine = ControllerPersistQuarantineRequest{Schema: controllerRequestSchema(value.Operation), Record: converted, PreservedBytes: clonePublicBytes(owner.PersistQuarantine.PreservedBytes)}
		if err := ValidateQuarantinePreservation(value.PersistQuarantine.Record, value.PersistQuarantine.PreservedBytes); err != nil {
			return err
		}
	case ControllerOperationPersistAbort:
		converted, err := abortRecordFromPublic(owner.PersistAbort.Record)
		if err != nil {
			return err
		}
		value.PersistAbort = ControllerPersistAbortRequest{Schema: controllerRequestSchema(value.Operation), Record: converted}
	default:
		return fmt.Errorf("sealedexec: operation has no public owner arm")
	}
	return nil
}

func controllerResultToOwner(value ControllerResult) (contextowner.Reply, error) {
	owner := contextowner.Reply{Call: contextowner.Call{Operation: contextowner.Operation(value.Operation)}}
	if !controllerSuccessArmsMatch(value) {
		return owner, fmt.Errorf("sealedexec: controller success carries wrong or multiple result arms")
	}
	switch value.Operation {
	case ControllerOperationVerifyAuthority:
		if value.VerifyAuthority.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := authorityFactsToPublic(value.VerifyAuthority.Facts)
		if err != nil {
			return owner, err
		}
		owner.VerifyAuthority = contextowner.VerifyAuthorityResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Facts: converted}
	case ControllerOperationResolveProfile:
		if value.ResolveProfile.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := profileMaterialToPublic(value.ResolveProfile.Material)
		if err != nil {
			return owner, err
		}
		owner.ResolveProfile = contextowner.ResolveProfileResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Material: converted}
	case ControllerOperationVerifyConflict:
		if value.VerifyConflict.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := conflictFactsToPublic(value.VerifyConflict.Facts)
		if err != nil {
			return owner, err
		}
		owner.VerifyConflict = contextowner.VerifyConflictResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Facts: converted}
	case ControllerOperationResolveRecorder:
		if value.ResolveRecorder.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := recorderFactsToPublic(value.ResolveRecorder.Facts)
		if err != nil {
			return owner, err
		}
		owner.ResolveRecorder = contextowner.ResolveRecorderResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Facts: converted}
	case ControllerOperationRecorderCheckpoint:
		if value.RecorderCheckpoint.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := recorderCheckpointToPublic(value.RecorderCheckpoint.Checkpoint)
		if err != nil {
			return owner, err
		}
		owner.RecorderCheckpoint = contextowner.RecorderCheckpointResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Checkpoint: converted}
	case ControllerOperationRecorderAppend:
		if value.RecorderAppend.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := eventAckToPublic(value.RecorderAppend.Ack)
		if err != nil {
			return owner, err
		}
		owner.RecorderAppend = contextowner.RecorderAppendResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Ack: converted}
	case ControllerOperationStoreRedactedSegment:
		if value.StoreRedactedSegment.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := storedSegmentToPublic(value.StoreRedactedSegment.Stored)
		if err != nil {
			return owner, err
		}
		owner.StoreRedactedSegment = contextowner.StoreRedactedSegmentResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Stored: converted}
	case ControllerOperationResolveRedactedSegment:
		if value.ResolveRedactedSegment.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := redactedSegmentToPublic(value.ResolveRedactedSegment.Segment)
		if err != nil {
			return owner, err
		}
		owner.ResolveRedactedSegment = contextowner.ResolveRedactedSegmentResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Segment: converted}
	case ControllerOperationVerifyOpaqueBoundary:
		if value.VerifyOpaqueBoundary.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := opaqueFactsToPublic(value.VerifyOpaqueBoundary.Facts)
		if err != nil {
			return owner, err
		}
		owner.VerifyOpaqueBoundary = contextowner.VerifyOpaqueBoundaryResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Facts: converted}
	case ControllerOperationVerifyProviderSession:
		if value.VerifyProviderSession.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := providerSessionFactsToPublic(value.VerifyProviderSession.Facts)
		if err != nil {
			return owner, err
		}
		owner.VerifyProviderSession = contextowner.VerifyProviderSessionResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Facts: converted}
	case ControllerOperationVerifyExpansion:
		if value.VerifyExpansion.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := expansionFactsToPublic(value.VerifyExpansion.Facts)
		if err != nil {
			return owner, err
		}
		owner.VerifyExpansion = contextowner.VerifyExpansionResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Facts: converted}
	case ControllerOperationStoreAdapterSession:
		if value.StoreAdapterSession.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		owner.StoreAdapterSession = contextowner.StoreAdapterSessionResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation))}
	case ControllerOperationNextStamp:
		if value.NextStamp.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := textToPublic(value.NextStamp.Stamp)
		if err != nil {
			return owner, err
		}
		owner.NextStamp = contextowner.NextStampResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Stamp: converted}
	case ControllerOperationResolveContext:
		if value.ResolveContext.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := contextResolutionToPublic(value.ResolveContext.Resolution)
		if err != nil {
			return owner, err
		}
		owner.ResolveContext = contextowner.ResolveContextResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Resolution: converted}
	case ControllerOperationVerifyEpoch:
		if value.VerifyEpoch.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := verificationToPublic(value.VerifyEpoch.Verification)
		if err != nil {
			return owner, err
		}
		owner.VerifyEpoch = contextowner.VerifyEpochResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Verification: converted}
	case ControllerOperationInstallExpansion:
		if value.InstallExpansion.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		owner.InstallExpansion = contextowner.InstallExpansionResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation))}
	case ControllerOperationResolveReceiptInputs:
		if value.ResolveReceiptInputs.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := receiptInputsToPublic(value.ResolveReceiptInputs.Inputs)
		if err != nil {
			return owner, err
		}
		owner.ResolveReceiptInputs = contextowner.ResolveReceiptInputsResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Inputs: converted}
	case ControllerOperationAppendReceipt:
		if value.AppendReceipt.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := receiptAckToPublic(value.AppendReceipt.Ack)
		if err != nil {
			return owner, err
		}
		owner.AppendReceipt = contextowner.AppendReceiptResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Ack: converted}
	case ControllerOperationResolveReceiptVerificationAuthority:
		if value.ResolveReceiptVerificationAuthority.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := receiptVerificationAuthorityToPublic(value.ResolveReceiptVerificationAuthority.Authority)
		if err != nil {
			return owner, err
		}
		owner.ResolveReceiptVerificationAuthority = contextowner.ResolveReceiptVerificationAuthorityResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Authority: converted}
	case ControllerOperationPersistHandback:
		if value.PersistHandback.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := controlAckToPublic(value.PersistHandback.Ack)
		if err != nil {
			return owner, err
		}
		owner.PersistHandback = contextowner.PersistHandbackResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Ack: converted}
	case ControllerOperationPersistQuarantine:
		if value.PersistQuarantine.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := controlAckToPublic(value.PersistQuarantine.Ack)
		if err != nil {
			return owner, err
		}
		owner.PersistQuarantine = contextowner.PersistQuarantineResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Ack: converted}
	case ControllerOperationPersistAbort:
		if value.PersistAbort.Schema != controllerResultSchema(value.Operation) {
			return owner, operationSchemaError(value.Operation)
		}
		converted, err := controlAckToPublic(value.PersistAbort.Ack)
		if err != nil {
			return owner, err
		}
		owner.PersistAbort = contextowner.PersistAbortResult{Schema: contextowner.ResultSchema(contextowner.Operation(value.Operation)), Ack: converted}
	default:
		return owner, fmt.Errorf("sealedexec: operation has no public owner arm")
	}
	return owner, nil
}

func controllerResultFromOwner(owner contextowner.Reply, value *ControllerResult) error {
	switch value.Operation {
	case ControllerOperationVerifyAuthority:
		converted, err := authorityFactsFromPublic(owner.VerifyAuthority.Facts)
		if err != nil {
			return err
		}
		value.VerifyAuthority = ControllerVerifyAuthorityResult{Schema: controllerResultSchema(value.Operation), Facts: converted}
	case ControllerOperationResolveProfile:
		converted, err := profileMaterialFromPublic(owner.ResolveProfile.Material)
		if err != nil {
			return err
		}
		value.ResolveProfile = ControllerResolveProfileResult{Schema: controllerResultSchema(value.Operation), Material: converted}
	case ControllerOperationVerifyConflict:
		converted, err := conflictFactsFromPublic(owner.VerifyConflict.Facts)
		if err != nil {
			return err
		}
		value.VerifyConflict = ControllerVerifyConflictResult{Schema: controllerResultSchema(value.Operation), Facts: converted}
	case ControllerOperationResolveRecorder:
		converted, err := recorderFactsFromPublic(owner.ResolveRecorder.Facts)
		if err != nil {
			return err
		}
		value.ResolveRecorder = ControllerResolveRecorderResult{Schema: controllerResultSchema(value.Operation), Facts: converted}
	case ControllerOperationRecorderCheckpoint:
		converted, err := recorderCheckpointFromPublic(owner.RecorderCheckpoint.Checkpoint)
		if err != nil {
			return err
		}
		value.RecorderCheckpoint = ControllerRecorderCheckpointResult{Schema: controllerResultSchema(value.Operation), Checkpoint: converted}
	case ControllerOperationRecorderAppend:
		converted, err := eventAckFromPublic(owner.RecorderAppend.Ack)
		if err != nil {
			return err
		}
		value.RecorderAppend = ControllerRecorderAppendResult{Schema: controllerResultSchema(value.Operation), Ack: converted}
	case ControllerOperationStoreRedactedSegment:
		converted, err := storedSegmentFromPublic(owner.StoreRedactedSegment.Stored)
		if err != nil {
			return err
		}
		value.StoreRedactedSegment = ControllerStoreRedactedSegmentResult{Schema: controllerResultSchema(value.Operation), Stored: converted}
	case ControllerOperationResolveRedactedSegment:
		converted, err := redactedSegmentFromPublic(owner.ResolveRedactedSegment.Segment)
		if err != nil {
			return err
		}
		value.ResolveRedactedSegment = ControllerResolveRedactedSegmentResult{Schema: controllerResultSchema(value.Operation), Segment: converted}
	case ControllerOperationVerifyOpaqueBoundary:
		converted, err := opaqueFactsFromPublic(owner.VerifyOpaqueBoundary.Facts)
		if err != nil {
			return err
		}
		value.VerifyOpaqueBoundary = ControllerVerifyOpaqueBoundaryResult{Schema: controllerResultSchema(value.Operation), Facts: converted}
	case ControllerOperationVerifyProviderSession:
		converted, err := providerSessionFactsFromPublic(owner.VerifyProviderSession.Facts)
		if err != nil {
			return err
		}
		value.VerifyProviderSession = ControllerVerifyProviderSessionResult{Schema: controllerResultSchema(value.Operation), Facts: converted}
	case ControllerOperationVerifyExpansion:
		converted, err := expansionFactsFromPublic(owner.VerifyExpansion.Facts)
		if err != nil {
			return err
		}
		value.VerifyExpansion = ControllerVerifyExpansionResult{Schema: controllerResultSchema(value.Operation), Facts: converted}
	case ControllerOperationStoreAdapterSession:
		value.StoreAdapterSession = ControllerStoreAdapterSessionResult{Schema: controllerResultSchema(value.Operation)}
	case ControllerOperationNextStamp:
		converted, err := textFromPublic(owner.NextStamp.Stamp)
		if err != nil {
			return err
		}
		value.NextStamp = ControllerNextStampResult{Schema: controllerResultSchema(value.Operation), Stamp: converted}
	case ControllerOperationResolveContext:
		converted, err := contextResolutionFromPublic(owner.ResolveContext.Resolution)
		if err != nil {
			return err
		}
		value.ResolveContext = ControllerResolveContextResult{Schema: controllerResultSchema(value.Operation), Resolution: converted}
	case ControllerOperationVerifyEpoch:
		converted, err := verificationFromPublic(owner.VerifyEpoch.Verification)
		if err != nil {
			return err
		}
		value.VerifyEpoch = ControllerVerifyEpochResult{Schema: controllerResultSchema(value.Operation), Verification: converted}
	case ControllerOperationInstallExpansion:
		value.InstallExpansion = ControllerInstallExpansionResult{Schema: controllerResultSchema(value.Operation)}
	case ControllerOperationResolveReceiptInputs:
		converted, err := receiptInputsFromPublic(owner.ResolveReceiptInputs.Inputs)
		if err != nil {
			return err
		}
		value.ResolveReceiptInputs = ControllerResolveReceiptInputsResult{Schema: controllerResultSchema(value.Operation), Inputs: converted}
	case ControllerOperationAppendReceipt:
		converted, err := receiptAckFromPublic(owner.AppendReceipt.Ack)
		if err != nil {
			return err
		}
		value.AppendReceipt = ControllerAppendReceiptResult{Schema: controllerResultSchema(value.Operation), Ack: converted}
	case ControllerOperationResolveReceiptVerificationAuthority:
		converted, err := receiptVerificationAuthorityFromPublic(owner.ResolveReceiptVerificationAuthority.Authority)
		if err != nil {
			return err
		}
		value.ResolveReceiptVerificationAuthority = ControllerResolveReceiptVerificationAuthorityResult{Schema: controllerResultSchema(value.Operation), Authority: converted}
	case ControllerOperationPersistHandback:
		converted, err := controlAckFromPublic(owner.PersistHandback.Ack)
		if err != nil {
			return err
		}
		value.PersistHandback = ControllerPersistHandbackResult{Schema: controllerResultSchema(value.Operation), Ack: converted}
	case ControllerOperationPersistQuarantine:
		converted, err := controlAckFromPublic(owner.PersistQuarantine.Ack)
		if err != nil {
			return err
		}
		value.PersistQuarantine = ControllerPersistQuarantineResult{Schema: controllerResultSchema(value.Operation), Ack: converted}
	case ControllerOperationPersistAbort:
		converted, err := controlAckFromPublic(owner.PersistAbort.Ack)
		if err != nil {
			return err
		}
		value.PersistAbort = ControllerPersistAbortResult{Schema: controllerResultSchema(value.Operation), Ack: converted}
	default:
		return fmt.Errorf("sealedexec: operation has no public owner arm")
	}
	return nil
}

func encodePublicControllerCallPayload(call ControllerCall) (json.RawMessage, error) {
	if call.Operation == ControllerOperationResolveClaimMCP {
		return encodeClaimControllerCallPayload(call)
	}
	owner, err := controllerCallToOwner(call)
	if err != nil {
		return nil, err
	}
	return contextowner.RequestArm(owner)
}
func decodePublicControllerCallPayload(arm json.RawMessage, call *ControllerCall) error {
	if call.Operation == ControllerOperationResolveClaimMCP {
		return decodeClaimControllerCallPayload(arm, call)
	}
	owner, err := contextowner.NewCall(contextowner.Operation(call.Operation), arm)
	if err != nil {
		return err
	}
	return controllerCallFromOwner(owner, call)
}
func encodePublicControllerSuccessPayload(result ControllerResult) (json.RawMessage, error) {
	if result.Operation == ControllerOperationResolveClaimMCP {
		return encodeClaimControllerSuccessPayload(result)
	}
	owner, err := controllerResultToOwner(result)
	if err != nil {
		return nil, err
	}
	// Structural result validation does not need, and must not invent, a request.
	return contextowner.ResultArm(owner)
}
func decodePublicControllerSuccessPayload(arm json.RawMessage, result *ControllerResult) error {
	if result.Operation == ControllerOperationResolveClaimMCP {
		return decodeClaimControllerSuccessPayload(arm, result)
	}
	owner, err := contextowner.DecodeResultArm(contextowner.Operation(result.Operation), arm)
	if err != nil {
		return err
	}
	return controllerResultFromOwner(owner, result)
}
