package sealedexec

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextevent"
)

// EncodeExecutionPartial canonically encodes the actual request/run state
// available at an incomplete terminal boundary. I-117/SI-164: once the shared
// flight state carries a terminal snapshot, the partial represents that state's
// actual manifest revision and digest, which an installed embedded expansion
// has moved past the dispatched request revision. Before the shared state
// exists, and for a run that has acknowledged nothing, the original request
// manifest is preserved unchanged.
func EncodeExecutionPartial(request ExecutionRequest, run ExecutionRun) ([]byte, error) {
	if _, err := EncodeExecutionRequest(request); err != nil {
		return nil, fmt.Errorf("sealedexec: encode execution partial request: %w", err)
	}
	if err := validateHandbackRunIdentity(request, run, true); err != nil {
		return nil, fmt.Errorf("sealedexec: encode execution partial run: %w", err)
	}
	_, witnesses, err := completionAuthority(run)
	if err != nil {
		return nil, fmt.Errorf("sealedexec: encode execution partial authority: %w", err)
	}
	// The one shared stream validator also fixes the lowest revision the stream
	// may carry: no acknowledgment may predate the dispatched request.
	last, err := validateRunAcknowledgments(request, run.Acks, true)
	if err != nil {
		return nil, fmt.Errorf("sealedexec: encode execution partial acknowledgments: %w", err)
	}
	revision, digest := request.ManifestRevision, request.ManifestDigest
	if len(run.Acks) != 0 && run.Terminal.Key != (ExecutionKey{}) {
		// The snapshot is authoritative only after it has been cross-matched
		// against the stream that actually reached it, exactly as completion
		// cross-matches its own terminal position.
		// I-118/SI-166 admits exactly one further partial-only shape: the
		// opening position of a child revision whose atomic install succeeded
		// before any child append could be acknowledged. Successful completion
		// stays strict because it appends its terminal event on that child.
		if err := validateRunTerminal(request, run.Terminal, last); err != nil {
			if !installedChildOpening(request, run.Terminal, last) {
				return nil, fmt.Errorf("sealedexec: encode execution partial terminal state: %w", err)
			}
		}
		revision, digest = run.Terminal.Revision, run.Terminal.ManifestDigest
	}
	partial := ExecutionPartial{
		Schema: ExecutionPartialSchemaID, Flight: request.Flight, Lane: request.Lane,
		Epoch: request.Epoch, Session: request.Session, Action: request.Action,
		ManifestRevision: revision, ManifestDigest: digest,
		Adapter: request.Adapter, AdapterVersion: request.AdapterVersion,
		WorkspaceID: run.Workspace.WorkspaceID, AdapterSessionRef: run.AdapterSessionRef,
		Authority: run.Authority, Witnesses: witnesses,
		EventAcks: append(make([]contextevent.EventAck, 0, len(run.Acks)), run.Acks...),
	}
	if err := validateExecutionPartial(partial); err != nil {
		return nil, err
	}
	return canonjson.Marshal(partial)
}

// installedChildOpening reports whether the terminal snapshot is exactly the
// position installExpansionLocked leaves after a successful atomic install
// whose first child append has not been acknowledged (I-118/SI-166). Every
// operand is checked against the live final acknowledgment, so no stale,
// skipped, backward, or fabricated terminal can enter through this arm: the
// snapshot must carry the exact dispatched request and key identity, a
// canonical installed manifest digest, the immediate successor revision opened
// at source one with no prior-event digest and the unchanged never-resetting
// global order, and a non-null bridge that exactly names the last
// acknowledgment's revision, event, terminal source order, and terminal global
// order.
func installedChildOpening(request ExecutionRequest, terminal FlightStateSnapshot, last contextevent.EventAck) bool {
	if terminal.Key != executionKey(request) || terminal.Request.ManifestRevision != request.ManifestRevision ||
		terminal.Request.ManifestDigest != request.ManifestDigest {
		return false
	}
	if validateDigest("execution partial installed child manifest_digest", terminal.ManifestDigest) != nil {
		return false
	}
	if terminal.Revision != last.ManifestRevision+1 || terminal.NextSourceSequence != 1 ||
		terminal.PriorEventDigest != "" || terminal.LastGlobalSequence != last.GlobalSequence {
		return false
	}
	bridge := terminal.PriorRevision
	return bridge != nil && bridge.ManifestRevision == last.ManifestRevision &&
		bridge.EventRoot == last.EventDigest &&
		bridge.TerminalSourceSequence == last.SourceSequence &&
		bridge.TerminalGlobalSequence == last.GlobalSequence
}

// DecodeExecutionPartial strictly decodes canonical incomplete request/run
// state and rejects unknown, missing, null, or contradictory fields.
func DecodeExecutionPartial(reader io.Reader) (ExecutionPartial, error) {
	var partial ExecutionPartial
	raw, err := decodeStrict(reader, &partial)
	if err != nil {
		return ExecutionPartial{}, fmt.Errorf("sealedexec: decode execution partial: %w", err)
	}
	if err := requireFields(raw, "schema", "flight", "lane", "epoch", "session", "action", "manifest_revision", "manifest_digest", "adapter", "adapter_version", "workspace_id", "adapter_session_ref", "authority", "witnesses", "event_acks"); err != nil {
		return ExecutionPartial{}, err
	}
	if err := validateExecutionPartial(partial); err != nil {
		return ExecutionPartial{}, err
	}
	canonical, err := canonjson.Marshal(partial)
	if err != nil {
		return ExecutionPartial{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return ExecutionPartial{}, fmt.Errorf("sealedexec: execution partial is not byte-canonical")
	}
	return partial, nil
}

func validateExecutionPartial(partial ExecutionPartial) error {
	if partial.Schema != ExecutionPartialSchemaID {
		return fmt.Errorf("sealedexec: execution partial schema must be %q", ExecutionPartialSchemaID)
	}
	if err := validateControlIdentity(partial.Flight, partial.Lane, partial.Epoch, partial.Session, "runway-not-carried", partial.WorkspaceID, false); err != nil {
		return err
	}
	switch partial.Action {
	case ActionStart, ActionResume:
	default:
		return fmt.Errorf("sealedexec: unknown execution partial action %q", partial.Action)
	}
	if err := validateDigest("execution partial manifest_digest", partial.ManifestDigest); err != nil {
		return err
	}
	switch partial.Adapter {
	case contextevent.AdapterCodex, contextevent.AdapterClaude:
	default:
		return fmt.Errorf("sealedexec: unknown execution partial adapter %q", partial.Adapter)
	}
	if err := validateControlText("execution partial adapter_version", partial.AdapterVersion, false); err != nil {
		return err
	}
	if err := validateControlText("execution partial adapter_session_ref", partial.AdapterSessionRef, true); err != nil {
		return err
	}
	if partial.Witnesses == nil {
		return fmt.Errorf("sealedexec: execution partial witnesses must be non-null")
	}
	if err := validateSortedTexts("execution partial witnesses", partial.Witnesses); err != nil {
		return err
	}
	switch partial.Authority {
	case contextevent.AuthorityAuthoritative:
		if len(partial.Witnesses) != 0 {
			return fmt.Errorf("sealedexec: authoritative execution partial carries adverse witnesses")
		}
	case contextevent.AuthorityAdvisory:
		if len(partial.Witnesses) == 0 {
			return fmt.Errorf("sealedexec: advisory execution partial lacks explicit witnesses")
		}
	default:
		return fmt.Errorf("sealedexec: unknown execution partial authority %q", partial.Authority)
	}
	if partial.EventAcks == nil {
		return fmt.Errorf("sealedexec: execution partial event_acks must be non-null")
	}
	return validateExecutionPartialAcks(partial)
}

// validateExecutionPartialAcks validates the partial's complete canonical
// acknowledgment stream through the same helper a successful completion uses
// (I-117/SI-164): one fixed execution identity, strictly increasing global
// order, source order contiguous inside each revision, a child revision exactly
// one past its predecessor restarting at source one, and no skipped or backward
// revision. Because the represented manifest revision is the terminal one, a
// nonempty stream must end exactly there, or — for I-118/SI-166's installed
// child whose first append was never acknowledged — exactly one revision before
// it. A skipped, backward, nonmonotonic, or source-gapped stream stays closed,
// as does any wider distance between the stream and the represented revision. A
// decoded partial cannot know the dispatched request revision, so the stream's
// own base is its floor; the encode path applies the request floor before it
// builds the partial.
func validateExecutionPartialAcks(partial ExecutionPartial) error {
	for i, ack := range partial.EventAcks {
		encoded, err := contextevent.EncodeEventAck(ack)
		if err != nil {
			return fmt.Errorf("sealedexec: execution partial event_acks[%d]: %w", i, err)
		}
		canonical, err := contextevent.DecodeEventAck(bytes.NewReader(encoded))
		if err != nil || canonical != ack {
			return fmt.Errorf("sealedexec: execution partial event_acks[%d] is not canonical", i)
		}
	}
	identity := ExecutionRequest{
		Flight: partial.Flight, Lane: partial.Lane, Epoch: partial.Epoch, Session: partial.Session,
	}
	last, err := validateRunAcknowledgments(identity, partial.EventAcks, true)
	if err != nil {
		return fmt.Errorf("sealedexec: execution partial event_acks: %w", err)
	}
	if len(partial.EventAcks) != 0 &&
		last.ManifestRevision != partial.ManifestRevision && last.ManifestRevision+1 != partial.ManifestRevision {
		return fmt.Errorf("sealedexec: execution partial represents manifest revision %d but its acknowledgments end at %d",
			partial.ManifestRevision, last.ManifestRevision)
	}
	return nil
}

// PreservedExecutionForBytes returns the sole controller-owned locator for the
// exact carried bytes. None is represented by a non-nil empty byte slice.
func PreservedExecutionForBytes(state PreservedState, data []byte) (PreservedExecution, error) {
	switch state {
	case PreservedNone:
		if data == nil || len(data) != 0 {
			return PreservedExecution{}, fmt.Errorf("sealedexec: preserved none requires non-null empty bytes")
		}
		return PreservedExecution{State: PreservedNone}, nil
	case PreservedPartial, PreservedFinalized:
		if len(data) == 0 {
			return PreservedExecution{}, fmt.Errorf("sealedexec: preserved %s requires nonempty bytes", state)
		}
		digest := digestBytes(data)
		return PreservedExecution{State: state, Ref: &PreservedExecutionRef{
			Schema: PreservedExecutionRefSchemaID,
			ID:     "controller-preserved/sha256/" + strings.TrimPrefix(digest, "sha256:"),
			Digest: digest,
		}}, nil
	default:
		return PreservedExecution{}, fmt.Errorf("sealedexec: unknown preserved state %q", state)
	}
}

// ValidateQuarantinePreservation proves that the quarantine record and the
// exact controller-carried bytes describe the same preserved execution.
func ValidateQuarantinePreservation(record QuarantineRecord, data []byte) error {
	if data == nil {
		return fmt.Errorf("sealedexec: quarantine preserved bytes must be non-null")
	}
	want, err := PreservedExecutionForBytes(record.Preserved.State, data)
	if err != nil {
		return err
	}
	if !preservedExecutionEqual(record.Preserved, want) {
		return fmt.Errorf("sealedexec: quarantine preserved locator contradicts exact bytes")
	}
	switch record.Preserved.State {
	case PreservedNone:
		return nil
	case PreservedPartial:
		partial, err := DecodeExecutionPartial(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("sealedexec: quarantine partial bytes: %w", err)
		}
		if partial.Flight != record.Flight || partial.Lane != record.Lane || partial.Epoch != record.Epoch || partial.Session != record.Session || partial.WorkspaceID != record.WorkspaceID {
			return fmt.Errorf("sealedexec: quarantine partial bytes contradict record identity")
		}
	case PreservedFinalized:
		result, err := DecodeExecutionResult(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("sealedexec: quarantine finalized bytes: %w", err)
		}
		if result.Flight != record.Flight || result.Lane != record.Lane || result.Epoch != record.Epoch || result.Session != record.Session ||
			result.ATCRunway != record.ATCRunway || result.ExecutionWorkspaceID != record.WorkspaceID ||
			result.InputCommit != record.Repository.Input.Commit || result.InputTree != record.Repository.Input.Tree ||
			record.Repository.Output.State != QuarantineOutputObserved || result.OutputCommit != record.Repository.Output.Commit || result.OutputTree != record.Repository.Output.Tree ||
			record.Receipt.State != QuarantineReceiptDurable || record.Receipt.EventAck == nil || result.Receipt.Digest != record.Receipt.Digest || result.ReceiptEventAck != *record.Receipt.EventAck {
			return fmt.Errorf("sealedexec: quarantine finalized bytes contradict record facts")
		}
	}
	return nil
}

func preservedExecutionEqual(left, right PreservedExecution) bool {
	if left.State != right.State || (left.Ref == nil) != (right.Ref == nil) {
		return false
	}
	return left.Ref == nil || *left.Ref == *right.Ref
}

// EncodeHandbackRecord validates, self-digests, and canonically encodes a
// successful handback record. A nonblank stale digest is rejected.
func EncodeHandbackRecord(record HandbackRecord) ([]byte, error) {
	if err := validateHandbackRecord(record, false); err != nil {
		return nil, err
	}
	digestless := record
	digestless.Digest = ""
	preimage, err := canonjson.Marshal(digestless)
	if err != nil {
		return nil, fmt.Errorf("sealedexec: encode handback digest preimage: %w", err)
	}
	want := digestBytes(preimage)
	if record.Digest != "" && record.Digest != want {
		return nil, fmt.Errorf("sealedexec: handback digest does not match canonical blank-digest preimage")
	}
	record.Digest = want
	return canonjson.Marshal(record)
}

// DecodeHandbackRecord strictly decodes and verifies a canonical handback.
func DecodeHandbackRecord(reader io.Reader) (HandbackRecord, error) {
	var record HandbackRecord
	raw, err := decodeStrict(reader, &record)
	if err != nil {
		return HandbackRecord{}, fmt.Errorf("sealedexec: decode handback: %w", err)
	}
	if err := requireFields(raw, "schema", "flight", "lane", "epoch", "session", "atc_runway", "workspace_id", "receipt", "input", "output", "pre_runway", "post_runway", "disposition", "digest"); err != nil {
		return HandbackRecord{}, err
	}
	if err := validateHandbackRecord(record, true); err != nil {
		return HandbackRecord{}, err
	}
	canonical, err := canonjson.Marshal(record)
	if err != nil {
		return HandbackRecord{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return HandbackRecord{}, fmt.Errorf("sealedexec: handback is not byte-canonical")
	}
	return record, nil
}

// EncodeQuarantineRecord validates, self-digests, and canonically encodes a
// quarantine record.
func EncodeQuarantineRecord(record QuarantineRecord) ([]byte, error) {
	if err := validateQuarantineRecord(record, false); err != nil {
		return nil, err
	}
	digestless := record
	digestless.Digest = ""
	preimage, err := canonjson.Marshal(digestless)
	if err != nil {
		return nil, fmt.Errorf("sealedexec: encode quarantine digest preimage: %w", err)
	}
	want := digestBytes(preimage)
	if record.Digest != "" && record.Digest != want {
		return nil, fmt.Errorf("sealedexec: quarantine digest does not match canonical blank-digest preimage")
	}
	record.Digest = want
	return canonjson.Marshal(record)
}

// DecodeQuarantineRecord strictly decodes and verifies a canonical quarantine.
func DecodeQuarantineRecord(reader io.Reader) (QuarantineRecord, error) {
	var record QuarantineRecord
	raw, err := decodeStrict(reader, &record)
	if err != nil {
		return QuarantineRecord{}, fmt.Errorf("sealedexec: decode quarantine: %w", err)
	}
	if err := requireFields(raw, "schema", "flight", "lane", "epoch", "session", "atc_runway", "workspace_id", "receipt", "repository", "observed", "reason", "preserved", "digest"); err != nil {
		return QuarantineRecord{}, err
	}
	if err := validateQuarantineRecord(record, true); err != nil {
		return QuarantineRecord{}, err
	}
	canonical, err := canonjson.Marshal(record)
	if err != nil {
		return QuarantineRecord{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return QuarantineRecord{}, fmt.Errorf("sealedexec: quarantine is not byte-canonical")
	}
	return record, nil
}

// EncodeAbortRecord validates, self-digests, and canonically encodes an abort.
func EncodeAbortRecord(record AbortRecord) ([]byte, error) {
	if err := validateAbortRecord(record, false); err != nil {
		return nil, err
	}
	digestless := record
	digestless.Digest = ""
	preimage, err := canonjson.Marshal(digestless)
	if err != nil {
		return nil, fmt.Errorf("sealedexec: encode abort digest preimage: %w", err)
	}
	want := digestBytes(preimage)
	if record.Digest != "" && record.Digest != want {
		return nil, fmt.Errorf("sealedexec: abort digest does not match canonical blank-digest preimage")
	}
	record.Digest = want
	return canonjson.Marshal(record)
}

// DecodeAbortRecord strictly decodes and verifies a canonical abort.
func DecodeAbortRecord(reader io.Reader) (AbortRecord, error) {
	var record AbortRecord
	raw, err := decodeStrict(reader, &record)
	if err != nil {
		return AbortRecord{}, fmt.Errorf("sealedexec: decode abort: %w", err)
	}
	if err := requireFields(raw, "schema", "flight", "lane", "epoch", "session", "workspace_id", "quarantine_digest", "owner_decision", "preserved", "disposition", "digest"); err != nil {
		return AbortRecord{}, err
	}
	if err := validateAbortRecord(record, true); err != nil {
		return AbortRecord{}, err
	}
	canonical, err := canonjson.Marshal(record)
	if err != nil {
		return AbortRecord{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return AbortRecord{}, fmt.Errorf("sealedexec: abort is not byte-canonical")
	}
	return record, nil
}

// EncodeControlAck validates, self-digests, and canonically encodes a durable
// controller acknowledgment.
func EncodeControlAck(ack ControlAck) ([]byte, error) {
	if err := validateControlAck(ack, false); err != nil {
		return nil, err
	}
	digestless := ack
	digestless.Digest = ""
	preimage, err := canonjson.Marshal(digestless)
	if err != nil {
		return nil, fmt.Errorf("sealedexec: encode control ack digest preimage: %w", err)
	}
	want := digestBytes(preimage)
	if ack.Digest != "" && ack.Digest != want {
		return nil, fmt.Errorf("sealedexec: control ack digest does not match canonical blank-digest preimage")
	}
	ack.Digest = want
	return canonjson.Marshal(ack)
}

// DecodeControlAck strictly decodes and verifies a canonical control ack.
func DecodeControlAck(reader io.Reader) (ControlAck, error) {
	var ack ControlAck
	raw, err := decodeStrict(reader, &ack)
	if err != nil {
		return ControlAck{}, fmt.Errorf("sealedexec: decode control ack: %w", err)
	}
	if err := requireFields(raw, "schema", "record_schema", "record_digest", "flight", "lane", "epoch", "session", "workspace_id", "disposition", "controller_global_sequence", "digest"); err != nil {
		return ControlAck{}, err
	}
	if err := validateControlAck(ack, true); err != nil {
		return ControlAck{}, err
	}
	canonical, err := canonjson.Marshal(ack)
	if err != nil {
		return ControlAck{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return ControlAck{}, fmt.Errorf("sealedexec: control ack is not byte-canonical")
	}
	return ack, nil
}

func validateHandbackRecord(record HandbackRecord, requireDigest bool) error {
	if record.Schema != ExecutionHandbackSchemaID {
		return fmt.Errorf("sealedexec: handback schema must be %q", ExecutionHandbackSchemaID)
	}
	if err := validateControlIdentity(record.Flight, record.Lane, record.Epoch, record.Session, record.ATCRunway, record.WorkspaceID, false); err != nil {
		return err
	}
	if err := validateDurableReceipt(record.Receipt, record.Flight, record.Lane, record.Epoch, record.Session); err != nil {
		return fmt.Errorf("sealedexec: handback receipt: %w", err)
	}
	if err := validateGitIdentity("handback input", record.Input); err != nil {
		return err
	}
	if err := validateGitIdentity("handback output", record.Output); err != nil {
		return err
	}
	if err := validateRunwayState("pre_runway", record.PreRunway); err != nil {
		return err
	}
	if err := validateRunwayState("post_runway", record.PostRunway); err != nil {
		return err
	}
	if record.PreRunway.Head != record.Input.Commit || record.PreRunway.Tree != record.Input.Tree {
		return fmt.Errorf("sealedexec: handback pre_runway must equal input commit/tree")
	}
	if record.PostRunway.Head != record.Output.Commit || record.PostRunway.Tree != record.Output.Tree {
		return fmt.Errorf("sealedexec: handback post_runway must equal output commit/tree")
	}
	if record.Disposition != ControlDispositionFastForwarded {
		return fmt.Errorf("sealedexec: handback disposition must be fast-forwarded")
	}
	return validateSelfDigest("handback", record.Digest, requireDigest, func() ([]byte, error) {
		copy := record
		copy.Digest = ""
		return canonjson.Marshal(copy)
	})
}

func validateQuarantineRecord(record QuarantineRecord, requireDigest bool) error {
	if record.Schema != ExecutionQuarantineSchemaID {
		return fmt.Errorf("sealedexec: quarantine schema must be %q", ExecutionQuarantineSchemaID)
	}
	if err := validateControlIdentity(record.Flight, record.Lane, record.Epoch, record.Session, record.ATCRunway, record.WorkspaceID, true); err != nil {
		return err
	}
	if err := validateQuarantineReceipt(record); err != nil {
		return err
	}
	if err := validateGitIdentity("quarantine input", record.Repository.Input); err != nil {
		return err
	}
	if err := validateQuarantineOutput(record.Repository.Output); err != nil {
		return err
	}
	if err := validateRepoObservation("runway", record.Observed.Runway); err != nil {
		return err
	}
	if err := validateRepoObservation("child", record.Observed.Child); err != nil {
		return err
	}
	if err := validateRepoObservation("post_runway", record.Observed.PostRunway); err != nil {
		return err
	}
	if err := validateProof(record.Observed.Descendant); err != nil {
		return fmt.Errorf("sealedexec: quarantine descendant: %w", err)
	}
	if err := validateProtectedPaths(record.Observed.ProtectedPaths); err != nil {
		return err
	}
	switch record.Observed.FastForward {
	case FastForwardNotAttempted, FastForwardSucceeded, FastForwardFailed:
	default:
		return fmt.Errorf("sealedexec: unknown fast_forward state %q", record.Observed.FastForward)
	}
	if err := validatePreserved(record.Preserved); err != nil {
		return err
	}
	if record.Receipt.State == QuarantineReceiptDurable && record.Preserved.State != PreservedFinalized {
		return fmt.Errorf("sealedexec: durable quarantine receipt requires finalized preservation")
	}
	if record.Receipt.State == QuarantineReceiptAbsent && record.Preserved.State == PreservedFinalized {
		return fmt.Errorf("sealedexec: absent quarantine receipt forbids finalized preservation")
	}
	if record.Preserved.State == PreservedFinalized && record.Repository.Output.State != QuarantineOutputObserved {
		return fmt.Errorf("sealedexec: finalized preservation requires observed output")
	}
	if err := validateQuarantineReasonFacts(record); err != nil {
		return err
	}
	return validateSelfDigest("quarantine", record.Digest, requireDigest, func() ([]byte, error) {
		copy := record
		copy.Digest = ""
		return canonjson.Marshal(copy)
	})
}

func validateAbortRecord(record AbortRecord, requireDigest bool) error {
	if record.Schema != ExecutionAbortSchemaID {
		return fmt.Errorf("sealedexec: abort schema must be %q", ExecutionAbortSchemaID)
	}
	if err := validateControlIdentity(record.Flight, record.Lane, record.Epoch, record.Session, "runway-not-carried", record.WorkspaceID, true); err != nil {
		return err
	}
	if err := validateDigest("quarantine_digest", record.QuarantineDigest); err != nil {
		return err
	}
	if err := validateGenericLogicalRef("owner_decision", record.OwnerDecision); err != nil {
		return err
	}
	if err := validatePreservedRef(record.Preserved); err != nil {
		return err
	}
	if record.Disposition != ControlDispositionAbortPreserve {
		return fmt.Errorf("sealedexec: abort disposition must be abort-preserve")
	}
	return validateSelfDigest("abort", record.Digest, requireDigest, func() ([]byte, error) {
		copy := record
		copy.Digest = ""
		return canonjson.Marshal(copy)
	})
}

func validateControlAck(ack ControlAck, requireDigest bool) error {
	if ack.Schema != ExecutionControlAckSchemaID {
		return fmt.Errorf("sealedexec: control ack schema must be %q", ExecutionControlAckSchemaID)
	}
	if err := validateControlIdentity(ack.Flight, ack.Lane, ack.Epoch, ack.Session, "runway-not-carried", ack.WorkspaceID, true); err != nil {
		return err
	}
	if err := validateDigest("record_digest", ack.RecordDigest); err != nil {
		return err
	}
	wantDisposition, ok := dispositionForRecordSchema(ack.RecordSchema)
	if !ok {
		return fmt.Errorf("sealedexec: unknown control ack record_schema %q", ack.RecordSchema)
	}
	if ack.Disposition != wantDisposition {
		return fmt.Errorf("sealedexec: control ack disposition %q contradicts record_schema %q", ack.Disposition, ack.RecordSchema)
	}
	if ack.ControllerGlobalSequence == 0 {
		return fmt.Errorf("sealedexec: controller_global_sequence must be positive")
	}
	return validateSelfDigest("control ack", ack.Digest, requireDigest, func() ([]byte, error) {
		copy := ack
		copy.Digest = ""
		return canonjson.Marshal(copy)
	})
}

func validateQuarantineReceipt(record QuarantineRecord) error {
	switch record.Receipt.State {
	case QuarantineReceiptAbsent:
		if record.Receipt.Digest != "" || record.Receipt.EventAck != nil {
			return fmt.Errorf("sealedexec: absent quarantine receipt forbids digest/event_ack")
		}
	case QuarantineReceiptDurable:
		if record.Receipt.EventAck == nil {
			return fmt.Errorf("sealedexec: durable quarantine receipt requires event_ack")
		}
		binding := DurableReceipt{Digest: record.Receipt.Digest, EventAck: *record.Receipt.EventAck}
		if err := validateDurableReceipt(binding, record.Flight, record.Lane, record.Epoch, record.Session); err != nil {
			return fmt.Errorf("sealedexec: durable quarantine receipt: %w", err)
		}
	default:
		return fmt.Errorf("sealedexec: unknown quarantine receipt state %q", record.Receipt.State)
	}
	return nil
}

func validateQuarantineOutput(output QuarantineOutput) error {
	switch output.State {
	case QuarantineOutputAbsent:
		if output.Commit != "" || output.Tree != "" {
			return fmt.Errorf("sealedexec: absent quarantine output forbids commit/tree")
		}
	case QuarantineOutputObserved:
		return validateGitIdentity("quarantine output", GitIdentity{Commit: output.Commit, Tree: output.Tree})
	default:
		return fmt.Errorf("sealedexec: unknown quarantine output state %q", output.State)
	}
	return nil
}

func validateRepoObservation(name string, observation RepoObservation) error {
	switch observation.State {
	case RepositoryObserved:
		return validateGitIdentity(name, GitIdentity{Commit: observation.Commit, Tree: observation.Tree})
	case RepositoryUnproven:
		if observation.Commit != "" || observation.Tree != "" || observation.Clean {
			return fmt.Errorf("sealedexec: unproven %s observation must have empty commit/tree and clean false", name)
		}
	default:
		return fmt.Errorf("sealedexec: unknown %s observation state %q", name, observation.State)
	}
	return nil
}

func validateProof(proof Proof) error {
	if proof.Witnesses == nil {
		return fmt.Errorf("proof witnesses must be non-null")
	}
	if err := validateSortedTexts("proof witnesses", proof.Witnesses); err != nil {
		return err
	}
	switch proof.State {
	case ProofProven:
		if len(proof.Witnesses) != 0 {
			return fmt.Errorf("proven proof must have empty witnesses")
		}
	case ProofViolatedWithWitness, ProofUnproven:
		if len(proof.Witnesses) == 0 {
			return fmt.Errorf("non-proven proof requires witnesses")
		}
	default:
		return fmt.Errorf("unknown proof state %q", proof.State)
	}
	return nil
}

func validateProtectedPaths(paths []string) error {
	if paths == nil {
		return fmt.Errorf("sealedexec: protected_paths must be non-null")
	}
	if err := validateSortedTexts("protected_paths", paths); err != nil {
		return err
	}
	for _, value := range paths {
		if strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || path.Clean(value) != value {
			return fmt.Errorf("sealedexec: protected path %q is not a clean repository path", value)
		}
		parts := strings.Split(value, "/")
		if len(parts) < 4 || parts[0] != ".verdi" || parts[1] != "specs" || parts[len(parts)-1] != "spec.md" {
			return fmt.Errorf("sealedexec: protected path %q is outside .verdi/specs/**/spec.md", value)
		}
	}
	return nil
}

func validatePreserved(preserved PreservedExecution) error {
	switch preserved.State {
	case PreservedNone:
		if preserved.Ref != nil {
			return fmt.Errorf("sealedexec: preserved none forbids ref")
		}
	case PreservedPartial, PreservedFinalized:
		if preserved.Ref == nil {
			return fmt.Errorf("sealedexec: preserved %s requires ref", preserved.State)
		}
		return validatePreservedRef(*preserved.Ref)
	default:
		return fmt.Errorf("sealedexec: unknown preserved state %q", preserved.State)
	}
	return nil
}

func validatePreservedRef(ref PreservedExecutionRef) error {
	if ref.Schema != PreservedExecutionRefSchemaID {
		return fmt.Errorf("sealedexec: preserved ref schema must be %q", PreservedExecutionRefSchemaID)
	}
	if err := validateControlText("preserved ref id", ref.ID, false); err != nil {
		return err
	}
	return validateDigest("preserved ref digest", ref.Digest)
}

func validateQuarantineReasonFacts(record QuarantineRecord) error {
	switch record.Reason {
	case QuarantineRunwayDirty, QuarantineRunwayMoved, QuarantineChildDirty,
		QuarantineNonDescendant, QuarantineProtectedSpecChange,
		QuarantineFastForwardFailed, QuarantinePostVerificationMismatch,
		QuarantineRepositoryVerificationFailed, QuarantineChildOutputMismatch,
		QuarantineHandbackDurabilityFailed:
		if record.Receipt.State != QuarantineReceiptDurable || record.Preserved.State != PreservedFinalized {
			return fmt.Errorf("sealedexec: handback quarantine requires a durable finalized result")
		}
	}

	preRunway := func() bool {
		return record.Observed.Runway.State == RepositoryObserved && record.Observed.Runway.Clean &&
			record.Observed.Runway.Commit == record.Repository.Input.Commit && record.Observed.Runway.Tree == record.Repository.Input.Tree
	}
	preChild := func() bool {
		return record.Repository.Output.State == QuarantineOutputObserved && record.Observed.Child.State == RepositoryObserved && record.Observed.Child.Clean &&
			record.Observed.Child.Commit == record.Repository.Output.Commit && record.Observed.Child.Tree == record.Repository.Output.Tree
	}
	preAll := func() bool {
		return preRunway() && preChild() && record.Observed.Descendant.State == ProofProven && len(record.Observed.ProtectedPaths) == 0
	}
	unprovenChild := func() bool {
		return record.Observed.Child.State == RepositoryUnproven
	}
	unprovenDescendant := func() bool {
		return record.Observed.Descendant.State == ProofUnproven
	}
	unprovenPost := func() bool {
		return record.Observed.PostRunway.State == RepositoryUnproven
	}
	noLaterFacts := func() bool {
		return unprovenChild() && unprovenDescendant() && len(record.Observed.ProtectedPaths) == 0 &&
			record.Observed.FastForward == FastForwardNotAttempted && unprovenPost()
	}
	noObservations := func() bool {
		return record.Observed.Runway.State == RepositoryUnproven && record.Observed.Child.State == RepositoryUnproven &&
			record.Observed.Descendant.State == ProofUnproven && len(record.Observed.ProtectedPaths) == 0 &&
			record.Observed.FastForward == FastForwardNotAttempted && record.Observed.PostRunway.State == RepositoryUnproven
	}
	postIsInput := func() bool {
		return record.Observed.PostRunway.State == RepositoryObserved && record.Observed.PostRunway.Clean &&
			record.Observed.PostRunway.Commit == record.Repository.Input.Commit && record.Observed.PostRunway.Tree == record.Repository.Input.Tree
	}
	postIsOutput := func() bool {
		return record.Repository.Output.State == QuarantineOutputObserved && record.Observed.PostRunway.State == RepositoryObserved &&
			record.Observed.PostRunway.Clean && record.Observed.PostRunway.Commit == record.Repository.Output.Commit &&
			record.Observed.PostRunway.Tree == record.Repository.Output.Tree
	}
	lateAttemptState := func() bool {
		return record.Observed.FastForward == FastForwardNotAttempted || record.Observed.FastForward == FastForwardFailed
	}

	switch record.Reason {
	case QuarantineRunwayDirty:
		initial := record.Observed.Runway.State == RepositoryObserved && !record.Observed.Runway.Clean && noLaterFacts()
		late := preAll() && lateAttemptState() && record.Observed.PostRunway.State == RepositoryObserved && !record.Observed.PostRunway.Clean
		if !initial && !late {
			return fmt.Errorf("sealedexec: runway-dirty requires an initial or late observed dirty runway with truthful attempt facts")
		}
	case QuarantineRunwayMoved:
		initial := record.Observed.Runway.State == RepositoryObserved && record.Observed.Runway.Clean &&
			(record.Observed.Runway.Commit != record.Repository.Input.Commit || record.Observed.Runway.Tree != record.Repository.Input.Tree) && noLaterFacts()
		lateMoved := record.Observed.PostRunway.State == RepositoryObserved && record.Observed.PostRunway.Clean && !postIsInput()
		if record.Observed.FastForward == FastForwardFailed {
			lateMoved = lateMoved && !postIsOutput()
		}
		late := preAll() && lateAttemptState() && lateMoved
		if !initial && !late {
			return fmt.Errorf("sealedexec: runway-moved requires an initial or late clean mismatch with truthful attempt facts")
		}
	case QuarantineChildDirty:
		if !preRunway() || record.Observed.Child.State != RepositoryObserved || record.Observed.Child.Clean ||
			!unprovenDescendant() || len(record.Observed.ProtectedPaths) != 0 || record.Observed.FastForward != FastForwardNotAttempted || !unprovenPost() {
			return fmt.Errorf("sealedexec: child-dirty contradicts repository observations")
		}
	case QuarantineChildOutputMismatch:
		childMismatch := record.Repository.Output.State == QuarantineOutputObserved && record.Observed.Child.State == RepositoryObserved && record.Observed.Child.Clean &&
			(record.Observed.Child.Commit != record.Repository.Output.Commit || record.Observed.Child.Tree != record.Repository.Output.Tree)
		if !preRunway() || !childMismatch || !unprovenDescendant() || len(record.Observed.ProtectedPaths) != 0 ||
			record.Observed.FastForward != FastForwardNotAttempted || !unprovenPost() {
			return fmt.Errorf("sealedexec: child-output-mismatch contradicts repository observations")
		}
	case QuarantineNonDescendant:
		if !preRunway() || !preChild() || record.Observed.Descendant.State != ProofViolatedWithWitness || len(record.Observed.ProtectedPaths) != 0 ||
			record.Observed.FastForward != FastForwardNotAttempted || !unprovenPost() {
			return fmt.Errorf("sealedexec: non-descendant contradicts precheck facts")
		}
	case QuarantineProtectedSpecChange:
		if !preRunway() || !preChild() || record.Observed.Descendant.State != ProofProven || len(record.Observed.ProtectedPaths) == 0 ||
			record.Observed.FastForward != FastForwardNotAttempted || !unprovenPost() {
			return fmt.Errorf("sealedexec: protected-spec-change contradicts precheck facts")
		}
	case QuarantineFastForwardFailed:
		if !preAll() || record.Observed.FastForward != FastForwardFailed || (!postIsInput() && !postIsOutput()) {
			return fmt.Errorf("sealedexec: fast-forward-failed contradicts precheck/attempt facts")
		}
	case QuarantinePostVerificationMismatch:
		if !preAll() || record.Observed.FastForward != FastForwardSucceeded || record.Observed.PostRunway.State != RepositoryObserved ||
			(record.Observed.PostRunway.Clean && record.Observed.PostRunway.Commit == record.Repository.Output.Commit && record.Observed.PostRunway.Tree == record.Repository.Output.Tree) {
			return fmt.Errorf("sealedexec: post-verification-mismatch lacks a postcheck mismatch")
		}
	case QuarantineHandbackDurabilityFailed:
		if !preAll() || record.Observed.FastForward != FastForwardSucceeded || !postIsOutput() {
			return fmt.Errorf("sealedexec: handback-durability-failed requires exact clean prechecks, successful fast-forward, and exact clean output postcheck")
		}
	case QuarantineNonAuthoritative, QuarantineOutputWriteFailed:
		if record.Receipt.State != QuarantineReceiptDurable || record.Preserved.State != PreservedFinalized || !noObservations() {
			return fmt.Errorf("sealedexec: finalized no-handback quarantine facts contradict reason %q", record.Reason)
		}
	case QuarantineExecutionIncomplete:
		if record.Receipt.State != QuarantineReceiptAbsent || record.Repository.Output.State != QuarantineOutputAbsent ||
			(record.Preserved.State != PreservedNone && record.Preserved.State != PreservedPartial) || !noObservations() {
			return fmt.Errorf("sealedexec: execution-incomplete facts contradict phase")
		}
	case QuarantineTerminalDurabilityFailed:
		if record.Receipt.State != QuarantineReceiptAbsent || record.Preserved.State == PreservedFinalized || !noObservations() {
			return fmt.Errorf("sealedexec: terminal-durability-failed facts contradict phase")
		}
	case QuarantineRepositoryVerificationFailed:
		if len(record.Observed.ProtectedPaths) != 0 || !unprovenPost() {
			return fmt.Errorf("sealedexec: repository-verification-failed requires an exact prefix and unproven post-runway")
		}
		beforeRunway := noObservations()
		afterRunway := preRunway() && unprovenChild() && unprovenDescendant() && record.Observed.FastForward == FastForwardNotAttempted
		afterChild := preRunway() && preChild() && unprovenDescendant() && record.Observed.FastForward == FastForwardNotAttempted
		afterRepositoryChecks := preAll() && (record.Observed.FastForward == FastForwardNotAttempted ||
			record.Observed.FastForward == FastForwardFailed || record.Observed.FastForward == FastForwardSucceeded)
		if !beforeRunway && !afterRunway && !afterChild && !afterRepositoryChecks {
			return fmt.Errorf("sealedexec: repository-verification-failed carries a non-prefix repository state")
		}
	default:
		return fmt.Errorf("sealedexec: unknown quarantine reason %q", record.Reason)
	}
	return nil
}

func validateDurableReceipt(receipt DurableReceipt, flight, lane, epoch, session string) error {
	if err := validateDigest("receipt digest", receipt.Digest); err != nil {
		return err
	}
	encoded, err := contextevent.EncodeReceiptEventAck(receipt.EventAck)
	if err != nil {
		return err
	}
	ack, err := contextevent.DecodeReceiptEventAck(bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	if ack.ReceiptDigest != receipt.Digest || ack.Flight != flight || ack.Lane != lane || ack.Epoch != epoch || ack.Session != session {
		return fmt.Errorf("receipt event acknowledgment identity mismatch")
	}
	return nil
}

func validateControlIdentity(flight, lane, epoch, session, runway, workspaceID string, allowEmptySession bool) error {
	for field, value := range map[string]string{"flight": flight, "lane": lane, "epoch": epoch, "workspace_id": workspaceID} {
		if err := validateControlText(field, value, false); err != nil {
			return err
		}
	}
	if err := validateControlText("session", session, allowEmptySession); err != nil {
		return err
	}
	if runway != "runway-not-carried" {
		if err := validateControlText("atc_runway", runway, false); err != nil {
			return err
		}
	}
	return nil
}

func validateGenericLogicalRef(field string, ref LogicalRef) error {
	if err := validateControlText(field+".schema", ref.Schema, false); err != nil {
		return err
	}
	if err := validateControlText(field+".id", ref.ID, false); err != nil {
		return err
	}
	return validateDigest(field+".digest", ref.Digest)
}

func validateControlText(field, value string, allowEmpty bool) error {
	if value == "" {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("sealedexec: %s must be nonempty", field)
	}
	if !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return fmt.Errorf("sealedexec: %s must be trimmed UTF-8", field)
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("sealedexec: %s contains a control character", field)
		}
	}
	return nil
}

func validateGitIdentity(field string, identity GitIdentity) error {
	if err := validateGitOID(field+" commit", identity.Commit, false); err != nil {
		return err
	}
	return validateGitOID(field+" tree", identity.Tree, false)
}

func validateRunwayState(field string, state RunwayState) error {
	if !state.Clean {
		return fmt.Errorf("sealedexec: %s must be clean", field)
	}
	return validateGitIdentity(field, GitIdentity{Commit: state.Head, Tree: state.Tree})
}

func validateSortedTexts(field string, values []string) error {
	for i, value := range values {
		if err := validateControlText(fmt.Sprintf("%s[%d]", field, i), value, false); err != nil {
			return err
		}
		if i > 0 && values[i-1] >= value {
			return fmt.Errorf("sealedexec: %s must be sorted and deduplicated", field)
		}
	}
	return nil
}

func validateSelfDigest(name, digest string, required bool, preimage func() ([]byte, error)) error {
	if !required && digest == "" {
		return nil
	}
	if err := validateDigest(name+" digest", digest); err != nil {
		return err
	}
	raw, err := preimage()
	if err != nil {
		return err
	}
	if digest != digestBytes(raw) {
		return fmt.Errorf("sealedexec: %s digest does not match canonical blank-digest preimage", name)
	}
	return nil
}

func dispositionForRecordSchema(schema string) (ControlDisposition, bool) {
	switch schema {
	case ExecutionHandbackSchemaID:
		return ControlDispositionFastForwarded, true
	case ExecutionQuarantineSchemaID:
		return ControlDispositionQuarantined, true
	case ExecutionAbortSchemaID:
		return ControlDispositionAbortPreserve, true
	default:
		return "", false
	}
}

// ValidateAbortAgainstQuarantine cross-matches the referenced durable record.
func ValidateAbortAgainstQuarantine(abort AbortRecord, quarantine QuarantineRecord) error {
	if err := validateAbortRecord(abort, true); err != nil {
		return err
	}
	if err := validateQuarantineRecord(quarantine, true); err != nil {
		return err
	}
	if abort.QuarantineDigest != quarantine.Digest || abort.Flight != quarantine.Flight || abort.Lane != quarantine.Lane ||
		abort.Epoch != quarantine.Epoch || abort.Session != quarantine.Session || abort.WorkspaceID != quarantine.WorkspaceID {
		return fmt.Errorf("sealedexec: abort identity conflicts with quarantine")
	}
	if quarantine.Preserved.Ref == nil || abort.Preserved != *quarantine.Preserved.Ref {
		return fmt.Errorf("sealedexec: abort preserved reference conflicts with quarantine")
	}
	return nil
}

// ValidateHandbackAck cross-matches an ack to the exact persisted handback.
func ValidateHandbackAck(record HandbackRecord, ack ControlAck) error {
	if err := validateHandbackRecord(record, true); err != nil {
		return err
	}
	return validateMatchingControlAck(record.Schema, record.Digest, record.Flight, record.Lane, record.Epoch, record.Session, record.WorkspaceID, ControlDispositionFastForwarded, ack)
}

// ValidateQuarantineAck cross-matches an ack to the exact quarantine.
func ValidateQuarantineAck(record QuarantineRecord, ack ControlAck) error {
	if err := validateQuarantineRecord(record, true); err != nil {
		return err
	}
	return validateMatchingControlAck(record.Schema, record.Digest, record.Flight, record.Lane, record.Epoch, record.Session, record.WorkspaceID, ControlDispositionQuarantined, ack)
}

// ValidateAbortAck cross-matches an ack to the exact abort disposition.
func ValidateAbortAck(record AbortRecord, ack ControlAck) error {
	if err := validateAbortRecord(record, true); err != nil {
		return err
	}
	return validateMatchingControlAck(record.Schema, record.Digest, record.Flight, record.Lane, record.Epoch, record.Session, record.WorkspaceID, ControlDispositionAbortPreserve, ack)
}

func validateMatchingControlAck(schema, digest, flight, lane, epoch, session, workspaceID string, disposition ControlDisposition, ack ControlAck) error {
	if err := validateControlAck(ack, true); err != nil {
		return err
	}
	if ack.RecordSchema != schema || ack.RecordDigest != digest || ack.Flight != flight || ack.Lane != lane || ack.Epoch != epoch ||
		ack.Session != session || ack.WorkspaceID != workspaceID || ack.Disposition != disposition {
		return fmt.Errorf("sealedexec: control acknowledgment conflicts with persisted record identity")
	}
	return nil
}
