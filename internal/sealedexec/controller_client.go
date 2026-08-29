package sealedexec

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
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
	facts := result.VerifyAuthority.Facts
	if err == nil && (facts.ManifestRevision != request.ManifestRevision || facts.ManifestDigest != request.ManifestDigest ||
		facts.ProjectionDigest != request.ProjectionDigest || facts.AuthorityDigest != request.AuthorityVerdict.Digest ||
		facts.AcceptedSpecCommit != request.Manifest.AcceptedSpec.Commit) {
		err = controllerResultMismatch(call.Operation, "authority facts contradict request")
	}
	return facts, err
}

// ResolveProfile returns credential-free profile material for local activation.
func (c *ControllerClient) ResolveProfile(ctx context.Context, query ProfileQuery) (ProfileMaterial, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationResolveProfile}
	call.ResolveProfile = ControllerResolveProfileRequest{Schema: controllerRequestSchema(call.Operation), Query: query}
	result, err := c.invoke(ctx, call)
	material := result.ResolveProfile.Material
	if err == nil && material.Ref != query.Ref {
		err = controllerResultMismatch(call.Operation, "profile material ref contradicts query")
	}
	return material, err
}

// VerifyConflict performs the typed verify-conflict call.
func (c *ControllerClient) VerifyConflict(ctx context.Context, report policyconflict.Report) (ConflictFacts, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationVerifyConflict}
	call.VerifyConflict = ControllerVerifyConflictRequest{Schema: controllerRequestSchema(call.Operation), Report: report}
	result, err := c.invoke(ctx, call)
	facts := result.VerifyConflict.Facts
	if err == nil {
		requestBytes, requestErr := policyconflict.EncodeReport(report)
		resultBytes, resultErr := policyconflict.EncodeReport(facts.Report)
		if requestErr != nil || resultErr != nil || !bytes.Equal(requestBytes, resultBytes) {
			err = controllerResultMismatch(call.Operation, "conflict facts report contradicts request")
		}
	}
	return facts, err
}

// ResolveRecorder performs the logical recorder binding proof.
func (c *ControllerClient) ResolveRecorder(ctx context.Context, ref LogicalRef) (RecorderFacts, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationResolveRecorder}
	call.ResolveRecorder = ControllerResolveRecorderRequest{Schema: controllerRequestSchema(call.Operation), Ref: ref}
	result, err := c.invoke(ctx, call)
	facts := result.ResolveRecorder.Facts
	if err == nil && facts.Ref != ref {
		err = controllerResultMismatch(call.Operation, "recorder ref contradicts request")
	}
	return facts, err
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
	ack := result.RecorderAppend.Ack
	if err == nil {
		err = validateAck(canonicalEvent, ack, 0)
		if err != nil {
			err = controllerResultMismatch(call.Operation, err.Error())
		}
	}
	return ack, err
}

// VerifyOpaqueBoundary proves the ordered identity-only opaque ledger.
func (c *ControllerClient) VerifyOpaqueBoundary(ctx context.Context, rows []contextcompile.OpaqueEntry) (OpaqueBoundaryFacts, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationVerifyOpaqueBoundary}
	call.VerifyOpaqueBoundary = ControllerVerifyOpaqueBoundaryRequest{Schema: controllerRequestSchema(call.Operation), Rows: rows}
	result, err := c.invoke(ctx, call)
	facts := result.VerifyOpaqueBoundary.Facts
	if err == nil && !opaqueFactsMatchRows(rows, facts.Rows) {
		err = controllerResultMismatch(call.Operation, "opaque identities contradict rows")
	}
	return facts, err
}

// VerifyProviderSession proves isolated provider session state.
func (c *ControllerClient) VerifyProviderSession(ctx context.Context, check ProviderSessionCheck) (ProviderSessionFacts, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationVerifyProviderSession}
	call.VerifyProviderSession = ControllerVerifyProviderSessionRequest{Schema: controllerRequestSchema(call.Operation), Check: check}
	result, err := c.invoke(ctx, call)
	facts := result.VerifyProviderSession.Facts
	if err == nil && (facts.SessionRef != check.SessionRef || facts.AdapterVersion != check.AdapterVersion ||
		facts.ProfileDigest != check.ProfileDigest || facts.WorkspaceID != check.WorkspaceID) {
		err = controllerResultMismatch(call.Operation, "provider-session facts contradict check")
	}
	return facts, err
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
	resolution := result.ResolveContext.Resolution
	if err == nil && resolution.Ref != query.Ref {
		err = controllerResultMismatch(call.Operation, "context resolution ref contradicts query")
	}
	return resolution, err
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
	ack := result.AppendReceipt.Ack
	if err == nil {
		err = validateReceiptAppendAck(canonical, ack)
		if err != nil {
			err = controllerResultMismatch(call.Operation, err.Error())
		}
	}
	return ack, err
}

// ResolveReceiptVerificationAuthority obtains the exact read-only selected
// profile, trust, isolation, and persistence facts for one verify request.
func (c *ControllerClient) ResolveReceiptVerificationAuthority(ctx context.Context, query contextreceipt.AuthorityQuery) (contextreceipt.AuthorityFacts, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, Operation: ControllerOperationResolveReceiptVerificationAuthority}
	call.ResolveReceiptVerificationAuthority = ControllerResolveReceiptVerificationAuthorityRequest{Schema: controllerRequestSchema(call.Operation), Query: query}
	result, err := c.invoke(ctx, call)
	authority := result.ResolveReceiptVerificationAuthority.Authority
	if err == nil {
		switch {
		case authority.TrustFact.SourceID != query.RunnerClaim.TrustSource:
			err = controllerResultMismatch(call.Operation, "trust fact source contradicts runner claim")
		case authority.Isolation.State == contextreceipt.StateProven && (authority.Isolation.ProfileID != query.ProfileRef.ID || authority.Isolation.ProfileDigest != query.ProfileRef.Digest):
			err = controllerResultMismatch(call.Operation, "isolation profile contradicts query")
		case authority.Persistence.ReceiptDigest != "" && authority.Persistence.ReceiptDigest != query.ReceiptDigest:
			err = controllerResultMismatch(call.Operation, "persistence receipt contradicts query")
		}
	}
	return authority, err
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
	if err == nil {
		err = ValidateHandbackAck(canonical, result.PersistHandback.Ack)
	}
	return result.PersistHandback.Ack, wrapControllerMatchError(call.Operation, err)
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
	if err == nil {
		err = ValidateQuarantineAck(canonical, result.PersistQuarantine.Ack)
	}
	return result.PersistQuarantine.Ack, wrapControllerMatchError(call.Operation, err)
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
	if err == nil {
		err = ValidateAbortAck(canonical, result.PersistAbort.Ack)
	}
	return result.PersistAbort.Ack, wrapControllerMatchError(call.Operation, err)
}

func canonicalControllerEventValue(event contextevent.Event) (contextevent.Event, error) {
	encoded, err := contextevent.EncodeEvent(event)
	if err != nil {
		return contextevent.Event{}, err
	}
	return contextevent.DecodeEvent(bytes.NewReader(encoded))
}

func canonicalReceiptAppend(appendValue ReceiptAppend) (ReceiptAppend, error) {
	wire, err := receiptAppendToWire(appendValue)
	if err != nil {
		return ReceiptAppend{}, err
	}
	return receiptAppendFromWire(wire)
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

func opaqueFactsMatchRows(rows []contextcompile.OpaqueEntry, identities []OpaqueIdentity) bool {
	if len(rows) != len(identities) {
		return false
	}
	for i := range rows {
		if identities[i].ID != rows[i].ID || identities[i].Kind != string(rows[i].Kind) ||
			identities[i].AdapterID != rows[i].Adapter.ID || identities[i].AdapterVersion != rows[i].Adapter.Version {
			return false
		}
	}
	return true
}

func validateReceiptAppendAck(appendValue ReceiptAppend, ack contextevent.ReceiptEventAck) error {
	canonical, err := canonicalReceiptAck(ack)
	if err != nil {
		return err
	}
	event := appendValue.Event
	if canonical.Flight != event.Flight || canonical.Lane != event.Lane || canonical.Epoch != event.Epoch ||
		canonical.Session != event.Session || canonical.ManifestRevision != event.ManifestRevision ||
		canonical.Kind != event.Kind || canonical.SourceSequence != event.SourceSequence ||
		canonical.EventDigest != event.EventDigest || canonical.ReceiptDigest != appendValue.Receipt.Digest {
		return errors.New("receipt acknowledgment does not bind exact receipt event identity")
	}
	return nil
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

	canceled := c.watchCancellation(ctx)
	if err := writeControllerFrame(c.transport, frame); err != nil {
		c.poisoned = err
		canceled(false)
		return ControllerResult{}, fmt.Errorf("sealedexec controller %s: %w: write call: %v", call.Operation, ErrOperational, err)
	}
	replyFrame, err := c.reader.ReadBytes('\n')
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

func wrapControllerMatchError(operation ControllerOperation, err error) error {
	if err == nil || errors.Is(err, ErrOperational) {
		return err
	}
	return fmt.Errorf("sealedexec controller %s: %w: acknowledgment mismatch: %v", operation, ErrOperational, err)
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
