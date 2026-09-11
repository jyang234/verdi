package sealedexec

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextowner"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// ControllerClient serializes the one inherited FD-3 request/reply stream.
// Its zero value is unusable; construct it with NewControllerClient.
type ControllerClient struct {
	mu        sync.Mutex
	transport io.ReadWriteCloser
	reader    *bufio.Reader
	next      uint64
	poisoned  error
}

// NewControllerClient binds one already-connected bidirectional transport.
func NewControllerClient(transport io.ReadWriter) (*ControllerClient, error) {
	if nilInterface(transport) {
		return nil, fmt.Errorf("sealedexec: controller transport is nil")
	}
	closable, ok := transport.(io.ReadWriteCloser)
	if !ok || nilInterface(closable) {
		return nil, fmt.Errorf("sealedexec: controller transport must be bidirectional and closable")
	}
	return &ControllerClient{transport: closable, reader: bufio.NewReader(closable), next: 1}, nil
}

// Usable reports whether this sequential controller capability remains safe
// for another typed call without exposing transport details.
func (c *ControllerClient) Usable() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.transport != nil && c.reader != nil && c.poisoned == nil
}

// VerifyAuthority performs the typed verify-authority call.
func (c *ControllerClient) VerifyAuthority(ctx context.Context, request ExecutionRequest) (AuthorityFacts, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationVerifyAuthority}
	call.VerifyAuthority = ControllerVerifyAuthorityRequest{Schema: controllerRequestSchema(call.Operation), Request: request}
	result, err := c.invoke(ctx, call)
	return result.VerifyAuthority.Facts, err
}

// ResolveProfile returns credential-free profile material for local activation.
func (c *ControllerClient) ResolveProfile(ctx context.Context, query ProfileQuery) (ProfileMaterial, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationResolveProfile}
	call.ResolveProfile = ControllerResolveProfileRequest{Schema: controllerRequestSchema(call.Operation), Query: query}
	result, err := c.invoke(ctx, call)
	return result.ResolveProfile.Material, err
}

// VerifyConflict performs the typed verify-conflict call.
func (c *ControllerClient) VerifyConflict(ctx context.Context, report policyconflict.Report) (ConflictFacts, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationVerifyConflict}
	call.VerifyConflict = ControllerVerifyConflictRequest{Schema: controllerRequestSchema(call.Operation), Report: report}
	result, err := c.invoke(ctx, call)
	return result.VerifyConflict.Facts, err
}

// ResolveRecorder performs the logical recorder binding proof.
func (c *ControllerClient) ResolveRecorder(ctx context.Context, ref LogicalRef) (RecorderFacts, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationResolveRecorder}
	call.ResolveRecorder = ControllerResolveRecorderRequest{Schema: controllerRequestSchema(call.Operation), Ref: ref}
	result, err := c.invoke(ctx, call)
	return result.ResolveRecorder.Facts, err
}

// RecorderCheckpoint queries the complete durable revision checkpoint.
func (c *ControllerClient) RecorderCheckpoint(ctx context.Context, key ExecutionKey) (RecorderCheckpoint, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationRecorderCheckpoint}
	call.RecorderCheckpoint = ControllerRecorderCheckpointRequest{Schema: controllerRequestSchema(call.Operation), Key: key}
	result, err := c.invoke(ctx, call)
	return result.RecorderCheckpoint.Checkpoint, err
}

// RecorderAppend atomically appends one canonical context event.
func (c *ControllerClient) RecorderAppend(ctx context.Context, event contextevent.Event) (contextevent.EventAck, error) {
	canonicalEvent, err := canonicalControllerEventValue(event)
	if err != nil {
		return contextevent.EventAck{}, controllerResultMismatch(ControllerOperationRecorderAppend, fmt.Sprintf("invalid event: %v", err))
	}
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationRecorderAppend}
	call.RecorderAppend = ControllerRecorderAppendRequest{Schema: controllerRequestSchema(call.Operation), Event: canonicalEvent}
	result, err := c.invoke(ctx, call)
	return result.RecorderAppend.Ack, err
}

// StoreRedactedSegment stores one canonical redacted JSON segment.
func (c *ControllerClient) StoreRedactedSegment(ctx context.Context, segment RedactedSegment) (StoredSegment, error) {
	canonical, err := canonicalDomainSegment(segment)
	if err != nil {
		return StoredSegment{}, controllerResultMismatch(ControllerOperationStoreRedactedSegment, fmt.Sprintf("invalid segment: %v", err))
	}
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationStoreRedactedSegment}
	call.StoreRedactedSegment = ControllerStoreRedactedSegmentRequest{Schema: controllerRequestSchema(call.Operation), Segment: canonical}
	result, err := c.invoke(ctx, call)
	return result.StoreRedactedSegment.Stored, err
}

// ResolveRedactedSegment resolves and revalidates one controller-owned segment.
func (c *ControllerClient) ResolveRedactedSegment(ctx context.Context, reference string) (RedactedSegment, error) {
	if err := validateSegmentReference(reference); err != nil {
		return RedactedSegment{}, controllerResultMismatch(ControllerOperationResolveRedactedSegment, fmt.Sprintf("invalid reference: %v", err))
	}
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationResolveRedactedSegment}
	call.ResolveRedactedSegment = ControllerResolveRedactedSegmentRequest{Schema: controllerRequestSchema(call.Operation), Reference: reference}
	result, err := c.invoke(ctx, call)
	return result.ResolveRedactedSegment.Segment, err
}

// VerifyOpaqueBoundary proves the ordered identity-only opaque ledger.
func (c *ControllerClient) VerifyOpaqueBoundary(ctx context.Context, rows []contextcompile.OpaqueEntry) (OpaqueBoundaryFacts, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationVerifyOpaqueBoundary}
	call.VerifyOpaqueBoundary = ControllerVerifyOpaqueBoundaryRequest{Schema: controllerRequestSchema(call.Operation), Rows: rows}
	result, err := c.invoke(ctx, call)
	return result.VerifyOpaqueBoundary.Facts, err
}

// VerifyProviderSession proves isolated provider session state.
func (c *ControllerClient) VerifyProviderSession(ctx context.Context, check ProviderSessionCheck) (ProviderSessionFacts, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationVerifyProviderSession}
	call.VerifyProviderSession = ControllerVerifyProviderSessionRequest{Schema: controllerRequestSchema(call.Operation), Check: check}
	result, err := c.invoke(ctx, call)
	return result.VerifyProviderSession.Facts, err
}

// VerifyExpansion proves the current expansion ledger root.
func (c *ControllerClient) VerifyExpansion(ctx context.Context, key ExecutionKey) (ExpansionFacts, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationVerifyExpansion}
	call.VerifyExpansion = ControllerVerifyExpansionRequest{Schema: controllerRequestSchema(call.Operation), Key: key}
	result, err := c.invoke(ctx, call)
	return result.VerifyExpansion.Facts, err
}

// StoreAdapterSession persists the acknowledged adapter-session identity.
func (c *ControllerClient) StoreAdapterSession(ctx context.Context, record SessionRecord) error {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationStoreAdapterSession}
	call.StoreAdapterSession = ControllerStoreAdapterSessionRequest{Schema: controllerRequestSchema(call.Operation), Record: record}
	_, err := c.invoke(ctx, call)
	return err
}

// NextStamp obtains the next controller-owned provenance stamp.
func (c *ControllerClient) NextStamp(ctx context.Context) (string, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationNextStamp}
	call.NextStamp = ControllerNextStampRequest{Schema: controllerRequestSchema(call.Operation)}
	result, err := c.invoke(ctx, call)
	return result.NextStamp.Stamp, err
}

// ResolveContext obtains one identity-bound context resolution.
func (c *ControllerClient) ResolveContext(ctx context.Context, query ContextQuery) (ContextResolution, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationResolveContext}
	call.ResolveContext = ControllerResolveContextRequest{Schema: controllerRequestSchema(call.Operation), Query: query}
	result, err := c.invoke(ctx, call)
	return result.ResolveContext.Resolution, err
}

// VerifyEpoch proves that the supplied expansion state is still current.
func (c *ControllerClient) VerifyEpoch(ctx context.Context, check EpochCheck) (Verification, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationVerifyEpoch}
	call.VerifyEpoch = ControllerVerifyEpochRequest{Schema: controllerRequestSchema(call.Operation), Check: check}
	result, err := c.invoke(ctx, call)
	return result.VerifyEpoch.Verification, err
}

// InstallExpansion atomically persists an acknowledged child transition.
func (c *ControllerClient) InstallExpansion(ctx context.Context, install ExpansionInstall) error {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationInstallExpansion}
	call.InstallExpansion = ControllerInstallExpansionRequest{Schema: controllerRequestSchema(call.Operation), Install: install}
	_, err := c.invoke(ctx, call)
	return err
}

// ResolveReceiptInputs obtains the exact terminal builder-receipt operands.
func (c *ControllerClient) ResolveReceiptInputs(ctx context.Context, query ReceiptInputsQuery) (ReceiptInputs, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationResolveReceiptInputs}
	call.ResolveReceiptInputs = ControllerResolveReceiptInputsRequest{Schema: controllerRequestSchema(call.Operation), Query: query}
	result, err := c.invoke(ctx, call)
	return result.ResolveReceiptInputs.Inputs, err
}

// AppendReceipt atomically persists the canonical receipt bytes and event.
func (c *ControllerClient) AppendReceipt(ctx context.Context, appendValue ReceiptAppend) (contextevent.ReceiptEventAck, error) {
	canonical, err := canonicalReceiptAppend(appendValue)
	if err != nil {
		return contextevent.ReceiptEventAck{}, controllerResultMismatch(ControllerOperationAppendReceipt, fmt.Sprintf("invalid append: %v", err))
	}
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationAppendReceipt}
	call.AppendReceipt = ControllerAppendReceiptRequest{Schema: controllerRequestSchema(call.Operation), Append: canonical}
	result, err := c.invoke(ctx, call)
	return result.AppendReceipt.Ack, err
}

// ResolveReceiptVerificationAuthority obtains the exact read-only selected
// profile, trust, isolation, and persistence facts for one verify request.
func (c *ControllerClient) ResolveReceiptVerificationAuthority(ctx context.Context, query contextreceipt.AuthorityQuery) (contextreceipt.AuthorityFacts, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationResolveReceiptVerificationAuthority}
	call.ResolveReceiptVerificationAuthority = ControllerResolveReceiptVerificationAuthorityRequest{Schema: controllerRequestSchema(call.Operation), Query: query}
	result, err := c.invoke(ctx, call)
	return result.ResolveReceiptVerificationAuthority.Authority, err
}

// PersistHandback persists one exact successful handback record.
func (c *ControllerClient) PersistHandback(ctx context.Context, record HandbackRecord) (ControlAck, error) {
	canonical, err := canonicalHandbackRecord(record)
	if err != nil {
		return ControlAck{}, controllerResultMismatch(ControllerOperationPersistHandback, fmt.Sprintf("invalid record: %v", err))
	}
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationPersistHandback}
	call.PersistHandback = ControllerPersistHandbackRequest{Schema: controllerRequestSchema(call.Operation), Record: canonical}
	result, err := c.invoke(ctx, call)
	return result.PersistHandback.Ack, err
}

// PersistQuarantine persists one exact quarantine record/bytes pair.
func (c *ControllerClient) PersistQuarantine(ctx context.Context, record QuarantineRecord, preservedBytes []byte) (ControlAck, error) {
	canonical, err := canonicalQuarantineRecord(record)
	if err != nil {
		return ControlAck{}, controllerResultMismatch(ControllerOperationPersistQuarantine, fmt.Sprintf("invalid record: %v", err))
	}
	if err := ValidateQuarantinePreservation(canonical, preservedBytes); err != nil {
		return ControlAck{}, controllerResultMismatch(ControllerOperationPersistQuarantine, fmt.Sprintf("invalid preservation: %v", err))
	}
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationPersistQuarantine}
	call.PersistQuarantine = ControllerPersistQuarantineRequest{Schema: controllerRequestSchema(call.Operation), Record: canonical, PreservedBytes: append([]byte{}, preservedBytes...)}
	result, err := c.invoke(ctx, call)
	return result.PersistQuarantine.Ack, err
}

// PersistAbort persists one exact abort-preserve disposition.
func (c *ControllerClient) PersistAbort(ctx context.Context, record AbortRecord) (ControlAck, error) {
	canonical, err := canonicalAbortRecord(record)
	if err != nil {
		return ControlAck{}, controllerResultMismatch(ControllerOperationPersistAbort, fmt.Sprintf("invalid record: %v", err))
	}
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationPersistAbort}
	call.PersistAbort = ControllerPersistAbortRequest{Schema: controllerRequestSchema(call.Operation), Record: canonical}
	result, err := c.invoke(ctx, call)
	return result.PersistAbort.Ack, err
}

// ResolveClaimMCP obtains the ATC-owned claim registration for this exact
// invocation. The result carries no bearer, credential, provider state, plan
// content, claim decision, or identity beyond the request digest, and a
// registration bound to any other request is refused. There is no fallback: an
// unavailable, malformed, stale, or contradictory answer is operational.
func (c *ControllerClient) ResolveClaimMCP(ctx context.Context, query ClaimMCPQuery) (ClaimMCPRegistration, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationResolveClaimMCP}
	call.ResolveClaimMCP = ControllerResolveClaimMCPRequest{Schema: controllerRequestSchema(call.Operation), Query: query}
	result, err := c.invoke(ctx, call)
	registration := result.ResolveClaimMCP.Registration
	if err == nil && registration.RequestDigest != query.RequestDigest {
		err = controllerResultMismatch(call.Operation, "claim registration digest contradicts query")
	}
	if err != nil {
		return ClaimMCPRegistration{}, err
	}
	return registration, nil
}

func canonicalControllerEventValue(event contextevent.Event) (contextevent.Event, error) {
	encoded, err := contextevent.EncodeEvent(event)
	if err != nil {
		return contextevent.Event{}, err
	}
	return contextevent.DecodeEvent(bytes.NewReader(encoded))
}

func canonicalReceiptAppend(value ReceiptAppend) (ReceiptAppend, error) {
	arm, err := receiptAppendToPublic(value)
	if err != nil {
		return ReceiptAppend{}, err
	}
	call := contextowner.Call{Operation: contextowner.OperationAppendReceipt, AppendReceipt: contextowner.AppendReceiptRequest{Schema: contextowner.RequestSchema(contextowner.OperationAppendReceipt), Append: arm}}
	if _, err = contextowner.RequestArm(call); err != nil {
		return ReceiptAppend{}, err
	}
	return receiptAppendFromPublic(arm)
}

func canonicalHandbackRecord(record HandbackRecord) (HandbackRecord, error) {
	encoded, err := EncodeHandbackRecord(record)
	if err != nil {
		return HandbackRecord{}, err
	}
	return DecodeHandbackRecord(bytes.NewReader(encoded))
}

func canonicalQuarantineRecord(record QuarantineRecord) (QuarantineRecord, error) {
	encoded, err := EncodeQuarantineRecord(record)
	if err != nil {
		return QuarantineRecord{}, err
	}
	return DecodeQuarantineRecord(bytes.NewReader(encoded))
}

func canonicalAbortRecord(record AbortRecord) (AbortRecord, error) {
	encoded, err := EncodeAbortRecord(record)
	if err != nil {
		return AbortRecord{}, err
	}
	return DecodeAbortRecord(bytes.NewReader(encoded))
}

func controllerResultMismatch(operation ControllerOperation, detail string) error {
	return fmt.Errorf("sealedexec controller %s: %w: result identity mismatch: %s", operation, ErrOperational, detail)
}

func (c *ControllerClient) invoke(ctx context.Context, call ControllerCall) (ControllerResult, error) {
	if ctx == nil {
		return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: nil context", call.Operation, ErrOperational)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.poisoned != nil {
		return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: client unusable after transport failure: %v", call.Operation, ErrOperational, c.poisoned)
	}
	if err := ctx.Err(); err != nil {
		c.poisoned = err
		return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: canceled before transport: %v", call.Operation, ErrOperational, err)
	}
	call.CallSequence = c.next
	frame, err := EncodeControllerCall(call)
	if err != nil {
		return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: encode call: %v", call.Operation, ErrOperational, err)
	}

	var ownerCall contextowner.Call
	if call.Operation != ControllerOperationResolveClaimMCP {
		// The frame has already been validated and emitted canonically. Extract
		// the exact bytes sent, then recompute their legacy standalone identity.
		var wire controllerCallWire
		if err := json.Unmarshal(frame, &wire); err != nil {
			return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: extract encoded call: %v", call.Operation, ErrOperational, err)
		}
		ownerCall, err = contextowner.NewCall(contextowner.Operation(call.Operation), wire.Payload)
		if err != nil {
			return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: local call: %v", call.Operation, ErrOperational, err)
		}
	}
	canceled := c.watchCancellation(ctx)
	if err := writeControllerFrame(c.transport, frame); err != nil {
		c.poisoned = err
		canceled(false)
		return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: write call: %v", call.Operation, ErrOperational, err)
	}
	replyFrame, err := readControllerReply(c.reader)
	wasCanceled := canceled(true)
	if err != nil {
		c.poisoned = err
		return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: read reply: %v", call.Operation, ErrOperational, err)
	}
	if wasCanceled || ctx.Err() != nil {
		c.poisoned = ctx.Err()
		return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: canceled transport: %v", call.Operation, ErrOperational, ctx.Err())
	}
	reply, err := DecodeControllerResult(bytesReader(replyFrame))
	if err != nil {
		c.poisoned = err
		return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: decode reply: %v", call.Operation, ErrOperational, err)
	}
	if reply.CallSequence != call.CallSequence {
		err := fmt.Errorf("reply sequence %d, want %d", reply.CallSequence, call.CallSequence)
		c.poisoned = err
		return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: %v", call.Operation, ErrOperational, err)
	}
	if reply.Operation != call.Operation {
		err := fmt.Errorf("reply operation %q, want %q", reply.Operation, call.Operation)
		c.poisoned = err
		return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: %v", call.Operation, ErrOperational, err)
	}
	c.next++
	if reply.Error != nil {
		return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: %s: %s", call.Operation, ErrOperational, reply.Error.Code, strings.Join(reply.Error.Witnesses, "; "))
	}
	if call.Operation != ControllerOperationResolveClaimMCP {
		arm, err := encodePublicControllerSuccessPayload(reply)
		if err == nil {
			_, err = contextowner.NewReply(ownerCall, arm)
		}
		if err != nil {
			if !errors.Is(err, contextowner.ErrRelationMismatch) {
				c.poisoned = err
			}
			return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: %w", call.Operation, ErrOperational, err)
		}
	}
	return reply, nil
}

// watchCancellation returns a completion function. Cancellation closes the
// transport exactly once to unblock an outstanding I/O; either outcome
// permanently poisons this sequential capability.
func (c *ControllerClient) watchCancellation(ctx context.Context) func(bool) bool {
	if ctx.Done() == nil {
		return func(bool) bool { return false }
	}
	done := make(chan struct{})
	var state atomic.Uint32 // 0 active, 1 canceled, 2 completed
	go func() {
		select {
		case <-ctx.Done():
			if state.CompareAndSwap(0, 1) {
				_ = c.transport.Close()
			}
		case <-done:
		}
	}()
	return func(complete bool) bool {
		if complete {
			state.CompareAndSwap(0, 2)
		}
		close(done)
		return state.Load() == 1
	}
}

func writeControllerFrame(writer io.Writer, frame []byte) error {
	n, err := writer.Write(frame)
	if n < 0 || n > len(frame) {
		return fmt.Errorf("invalid write count %d", n)
	}
	if err != nil {
		return err
	}
	if n != len(frame) {
		return io.ErrShortWrite
	}
	return nil
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

type byteReader struct{ data []byte }

func bytesReader(data []byte) *byteReader { return &byteReader{data: data} }
func (r *byteReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}
