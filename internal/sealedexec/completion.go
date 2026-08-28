package sealedexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
)

// ReceiptInputsResolver supplies the controller-owned terminal receipt rows.
type ReceiptInputsResolver interface {
	ResolveReceiptInputs(context.Context, ReceiptInputsQuery) (ReceiptInputs, error)
}

// ReceiptAppender atomically persists canonical receipt bytes and their exact
// receipt event before returning the specialized durable acknowledgment.
type ReceiptAppender interface {
	AppendReceipt(context.Context, ReceiptAppend) (contextevent.ReceiptEventAck, error)
}

// CompletionPorts are the consumer-defined terminal dependencies. The
// service constructs all events, receipts, and public result bytes itself.
type CompletionPorts struct {
	Workspace WorkspaceVerifier
	Recorder  Recorder
	Inputs    ReceiptInputsResolver
	Receipts  ReceiptAppender
	Stamps    StampSource
}

// CompletionRequest binds one canonical dispatch to U4c's terminal run.
type CompletionRequest struct {
	Request ExecutionRequest
	Run     ExecutionRun
}

// Completion is returned only after the receipt event is durably and
// specifically acknowledged. ResultBytes are the canonical public bytes.
type Completion struct {
	Result          ExecutionResult
	ResultBytes     []byte
	Receipt         contextreceipt.Receipt
	ReceiptEventAck contextevent.ReceiptEventAck
	Revisions       []contextevent.Revision
	EventChainRoot  string
	Verdict         contextcompile.Resolution
	Authority       contextevent.Authority
	Output          WorkspaceFacts
}

// CompletionService owns the acyclic execution-result, receipt, and public
// result terminal order.
type CompletionService struct {
	ports CompletionPorts
}

// NewCompletionService rejects an incomplete terminal dependency set.
func NewCompletionService(ports CompletionPorts) (*CompletionService, error) {
	missing := make([]string, 0, 5)
	if ports.Workspace == nil {
		missing = append(missing, "workspace")
	}
	if ports.Recorder == nil {
		missing = append(missing, "recorder")
	}
	if ports.Inputs == nil {
		missing = append(missing, "inputs")
	}
	if ports.Receipts == nil {
		missing = append(missing, "receipts")
	}
	if ports.Stamps == nil {
		missing = append(missing, "stamps")
	}
	if len(missing) != 0 {
		return nil, fmt.Errorf("sealedexec: new completion service: missing ports %v", missing)
	}
	return &CompletionService{ports: ports}, nil
}

// Complete finalizes one successful U4c run. No public result bytes are
// returned until both terminal events have their required durable acks.
func (s *CompletionService) Complete(ctx context.Context, input CompletionRequest) (Completion, error) {
	if ctx == nil {
		return Completion{}, operational("complete execution", errors.New("nil context"))
	}
	requestBytes, err := EncodeExecutionRequest(input.Request)
	if err != nil {
		return Completion{}, operational("complete execution request", err)
	}
	verdictValue, witnesses, err := completionAuthority(input.Run)
	if err != nil {
		return Completion{}, err
	}
	lastAck, err := validateCompletionRun(input.Request, input.Run)
	if err != nil {
		return Completion{}, err
	}

	child, err := s.ports.Workspace.VerifyWorkspace(ctx, input.Run.Workspace.Path, input.Request.ExecutionWorkspaceRequest)
	if err != nil {
		return Completion{}, operational("observe terminal child", err)
	}
	if err := validateCompletionChild(input.Request, input.Run, child); err != nil {
		return Completion{}, err
	}

	resultPayload, err := newExecutionResultPayload(input.Request, input.Run.Authority, child)
	if err != nil {
		return Completion{}, operational("build execution-result payload", err)
	}
	resultStamp, err := s.ports.Stamps.NextStamp(ctx)
	if err != nil {
		return Completion{}, operational("stamp execution-result", err)
	}
	resultEvent, err := buildEvent(input.Request, child, lastAck.SourceSequence+1, lastAck.EventDigest, nil, resultStamp, contextevent.KindExecutionResult, resultPayload)
	if err != nil {
		return Completion{}, operational("build execution-result event", err)
	}
	resultAck, err := s.ports.Recorder.Append(ctx, resultEvent)
	if err != nil {
		return Completion{}, operational("append execution-result", err)
	}
	if err := validateAck(resultEvent, resultAck, lastAck.GlobalSequence); err != nil {
		return Completion{}, operational("acknowledge execution-result", err)
	}

	checkpoint, err := s.ports.Recorder.Checkpoint(ctx, executionKey(input.Request))
	if err != nil {
		return Completion{}, operational("query completed recorder checkpoint", err)
	}
	revisions, eventRoot, err := validateCompletionCheckpoint(input.Request, resultEvent, resultAck, checkpoint)
	if err != nil {
		return Completion{}, operational("validate completed recorder checkpoint", err)
	}

	query := ReceiptInputsQuery{
		Request:                input.Request,
		WorkspaceID:            child.WorkspaceID,
		DispatchDigest:         digestBytes(requestBytes),
		TerminalRevision:       resultEvent.ManifestRevision,
		TerminalSourceSequence: resultEvent.SourceSequence,
		TerminalGlobalSequence: resultAck.GlobalSequence,
		EventChainRoot:         eventRoot,
		ResultFactsDigest:      resultPayload.ResultFactsDigest,
	}
	receiptInputs, err := s.ports.Inputs.ResolveReceiptInputs(ctx, query)
	if err != nil {
		return Completion{}, operational("resolve terminal receipt inputs", err)
	}
	receipt, receiptBytes, err := buildCompletionReceipt(input.Request, input.Run.Authority, child, query.DispatchDigest, revisions, eventRoot, resultAck, receiptInputs)
	if err != nil {
		return Completion{}, operational("build canonical builder receipt", err)
	}

	receiptStamp, err := s.ports.Stamps.NextStamp(ctx)
	if err != nil {
		return Completion{}, operational("stamp receipt event", err)
	}
	receiptEvent, err := buildReceiptEvent(input.Request, child, resultEvent, receipt, receiptBytes, receiptStamp)
	if err != nil {
		return Completion{}, operational("build receipt event", err)
	}
	receiptAck, err := s.ports.Receipts.AppendReceipt(ctx, ReceiptAppend{Receipt: receipt, Event: receiptEvent})
	if err != nil {
		return Completion{}, operational("append canonical receipt", err)
	}
	if err := validateSpecializedReceiptAck(receiptEvent, receipt, resultAck, receiptAck); err != nil {
		return Completion{}, operational("acknowledge canonical receipt", err)
	}

	result := ExecutionResult{
		Schema: ExecutionResultSchemaID, Verdict: verdictValue, Authority: input.Run.Authority,
		Witnesses: witnesses, Flight: input.Request.Flight, Lane: input.Request.Lane,
		Epoch: input.Request.Epoch, Session: input.Request.Session, ATCRunway: input.Request.ATCRunway,
		ExecutionWorkspaceID: child.WorkspaceID, Adapter: input.Request.Adapter,
		AdapterVersion: input.Request.AdapterVersion, InputCommit: input.Request.InputCommit,
		InputTree: input.Request.InputTree, OutputCommit: child.CurrentCommit, OutputTree: child.CurrentTree,
		Clean: child.Clean, TerminalManifestDigest: resultEvent.ManifestDigest,
		TerminalManifestRevision: resultEvent.ManifestRevision, TerminalSourceSequence: resultEvent.SourceSequence,
		TerminalGlobalSequence: resultAck.GlobalSequence, EventChainRoot: eventRoot,
		Receipt: receipt, ReceiptEventAck: receiptAck,
	}
	resultBytes, err := EncodeExecutionResult(result)
	if err != nil {
		return Completion{}, operational("encode public execution result", err)
	}
	canonicalResult, err := DecodeExecutionResult(bytes.NewReader(resultBytes))
	if err != nil {
		return Completion{}, operational("strict-roundtrip public execution result", err)
	}
	return Completion{
		Result: canonicalResult, ResultBytes: append([]byte(nil), resultBytes...), Receipt: receipt,
		ReceiptEventAck: receiptAck, Revisions: append([]contextevent.Revision(nil), revisions...),
		EventChainRoot: eventRoot, Verdict: verdictValue, Authority: input.Run.Authority, Output: child,
	}, nil
}

func completionAuthority(run ExecutionRun) (contextcompile.Resolution, []string, error) {
	witnesses := sortedUniqueStrings(run.Witnesses)
	switch run.Authority {
	case contextevent.AuthorityAuthoritative:
		if len(witnesses) != 0 {
			return "", nil, verdict("authoritative execution run carries adverse witnesses")
		}
		return contextcompile.ResolutionProven, witnesses, nil
	case contextevent.AuthorityAdvisory:
		if len(witnesses) == 0 {
			return "", nil, verdict("advisory execution run lacks explicit witnesses")
		}
		return contextcompile.ResolutionUnproven, witnesses, nil
	default:
		return "", nil, operational("complete execution authority", fmt.Errorf("unknown authority %q", run.Authority))
	}
}

func validateCompletionRun(request ExecutionRequest, run ExecutionRun) (contextevent.EventAck, error) {
	workspaceDigest, err := ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		return contextevent.EventAck{}, operational("complete workspace request digest", err)
	}
	workspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		return contextevent.EventAck{}, operational("complete workspace id", err)
	}
	if run.Workspace.WorkspaceID != workspaceID || run.Workspace.Path == "" || !run.Workspace.Request.Equal(request.ExecutionWorkspaceRequest) || run.Workspace.RequestDigest != workspaceDigest {
		return contextevent.EventAck{}, verdict("execution run workspace identity contradicts request")
	}
	if run.AdapterSessionRef == "" {
		return contextevent.EventAck{}, verdict("execution run lacks adapter session identity")
	}
	return validateRunAcknowledgments(request, run.Acks, false)
}

func validateRunAcknowledgments(request ExecutionRequest, acknowledgments []contextevent.EventAck, allowEmpty bool) (contextevent.EventAck, error) {
	if len(acknowledgments) == 0 {
		if allowEmpty {
			return contextevent.EventAck{}, nil
		}
		return contextevent.EventAck{}, operational("validate execution run acknowledgments", errors.New("execution run has no acknowledged events"))
	}
	canonical := make([]contextevent.EventAck, 0, len(acknowledgments))
	for _, ack := range acknowledgments {
		encoded, err := contextevent.EncodeEventAck(ack)
		if err != nil {
			return contextevent.EventAck{}, operational("validate execution run acknowledgments", err)
		}
		roundtripped, err := contextevent.DecodeEventAck(bytes.NewReader(encoded))
		if err != nil {
			return contextevent.EventAck{}, operational("validate execution run acknowledgments", err)
		}
		if roundtripped.Flight != request.Flight || roundtripped.Lane != request.Lane || roundtripped.Epoch != request.Epoch ||
			roundtripped.Session != request.Session || roundtripped.ManifestRevision != request.ManifestRevision {
			return contextevent.EventAck{}, verdict("execution run acknowledgment identity contradicts request")
		}
		canonical = append(canonical, roundtripped)
	}
	for i := 1; i < len(canonical); i++ {
		prior, current := canonical[i-1], canonical[i]
		if current.SourceSequence != prior.SourceSequence+1 || current.GlobalSequence <= prior.GlobalSequence {
			return contextevent.EventAck{}, operational("validate execution run acknowledgments", errors.New("execution run acknowledgment order is discontinuous"))
		}
	}
	return canonical[len(canonical)-1], nil
}

func validateCompletionChild(request ExecutionRequest, run ExecutionRun, child WorkspaceFacts) error {
	if err := requireProven("terminal child", child.Verification); err != nil {
		return err
	}
	wantDigest, err := ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		return operational("terminal child workspace digest", err)
	}
	if child.WorkspaceID != run.Workspace.WorkspaceID || child.Path != run.Workspace.Path ||
		!child.Request.Equal(request.ExecutionWorkspaceRequest) || child.RequestDigest != wantDigest {
		return verdict("terminal child identity contradicts execution run")
	}
	if !child.Clean || child.CurrentCommit == "" || child.CurrentTree == "" {
		return verdict("terminal child is dirty or lacks output commit/tree")
	}
	if err := validateGitOID("terminal child output commit", child.CurrentCommit, false); err != nil {
		return operational("terminal child output commit", err)
	}
	if err := validateGitOID("terminal child output tree", child.CurrentTree, false); err != nil {
		return operational("terminal child output tree", err)
	}
	return nil
}

func newExecutionResultPayload(request ExecutionRequest, authority contextevent.Authority, child WorkspaceFacts) (*contextevent.ExecutionResultPayload, error) {
	schema, err := contextevent.PayloadSchema(contextevent.KindExecutionResult)
	if err != nil {
		return nil, err
	}
	payload := contextevent.ExecutionResultPayload{
		Schema: schema, Authority: authority, InputCommit: request.InputCommit,
		OutputCommit: child.CurrentCommit, OutputTree: child.CurrentTree, Clean: child.Clean,
		ManifestDigest: request.ManifestDigest,
	}
	digest, err := canonjson.Digest(payload)
	if err != nil {
		return nil, err
	}
	payload.ResultFactsDigest = digest
	return &payload, nil
}

func validateCompletionCheckpoint(request ExecutionRequest, event contextevent.Event, ack contextevent.EventAck, checkpoint RecorderCheckpoint) ([]contextevent.Revision, string, error) {
	if err := requireProven("completed recorder checkpoint", checkpoint.Verification); err != nil {
		return nil, "", err
	}
	if checkpoint.ActiveRevision != nil {
		return nil, "", errors.New("completed recorder checkpoint carries an active revision")
	}
	root, err := contextevent.EventChainRoot(checkpoint.Revisions)
	if err != nil {
		return nil, "", err
	}
	terminal := checkpoint.Revisions[len(checkpoint.Revisions)-1]
	if checkpoint.EventChainRoot != root || checkpoint.TerminalSourceSequence != ack.SourceSequence || checkpoint.TerminalGlobalSequence != ack.GlobalSequence ||
		terminal.ManifestRevision != request.ManifestRevision || terminal.ManifestDigest != request.ManifestDigest ||
		terminal.TerminalKind != contextevent.KindExecutionResult || terminal.TerminalSourceSequence != ack.SourceSequence ||
		terminal.TerminalGlobalSequence != ack.GlobalSequence || terminal.EventRoot != event.EventDigest {
		return nil, "", errors.New("completed recorder checkpoint does not bind execution-result terminal identity")
	}
	return append([]contextevent.Revision(nil), checkpoint.Revisions...), root, nil
}

func buildCompletionReceipt(request ExecutionRequest, authority contextevent.Authority, child WorkspaceFacts, dispatchDigest string, revisions []contextevent.Revision, root string, resultAck contextevent.EventAck, inputs ReceiptInputs) (contextreceipt.Receipt, []byte, error) {
	workspaceDigest, err := ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		return contextreceipt.Receipt{}, nil, err
	}
	receipt := contextreceipt.Receipt{
		Schema: contextreceipt.SchemaID, Role: contextreceipt.RoleBuilder, Authority: authority,
		ManifestDigest: request.ManifestDigest, DispatchDigest: dispatchDigest, ATCRunway: request.ATCRunway,
		ExecutionWorkspaceRequestDigest: workspaceDigest, ExecutionWorkspaceID: child.WorkspaceID,
		InputCommit: request.InputCommit, InputTree: request.InputTree, OutputCommit: child.CurrentCommit,
		OutputTree: child.CurrentTree, Clean: child.Clean, RevisionSegments: append([]contextevent.Revision(nil), revisions...),
		EventChainRoot: root, TerminalManifestRevision: request.ManifestRevision,
		TerminalSourceSequence: resultAck.SourceSequence, TerminalGlobalSequence: resultAck.GlobalSequence,
		Expansions:                append(make([]contextreceipt.Expansion, 0, len(inputs.Expansions)), inputs.Expansions...),
		Obligations:               append(make([]contextreceipt.Obligation, 0, len(inputs.Obligations)), inputs.Obligations...),
		Evidence:                  append(make([]contextreceipt.Evidence, 0, len(inputs.Evidence)), inputs.Evidence...),
		RunnerPrincipalResolution: inputs.RunnerPrincipal, Adapter: request.Adapter,
		AdapterVersion: request.AdapterVersion, ReviewInputs: append(make([]contextreceipt.ReviewInput, 0, len(inputs.ReviewInputs)), inputs.ReviewInputs...),
	}
	// Preserve explicit non-null empty arrays from the controller boundary.
	if inputs.Expansions == nil {
		receipt.Expansions = nil
	}
	if inputs.Obligations == nil {
		receipt.Obligations = nil
	}
	if inputs.Evidence == nil {
		receipt.Evidence = nil
	}
	if inputs.ReviewInputs == nil {
		receipt.ReviewInputs = nil
	}
	encoded, err := contextreceipt.EncodeReceipt(receipt)
	if err != nil {
		return contextreceipt.Receipt{}, nil, err
	}
	canonical, err := contextreceipt.DecodeReceipt(bytes.NewReader(encoded))
	if err != nil {
		return contextreceipt.Receipt{}, nil, err
	}
	return canonical, encoded, nil
}

func buildReceiptEvent(request ExecutionRequest, child WorkspaceFacts, resultEvent contextevent.Event, receipt contextreceipt.Receipt, receiptBytes []byte, stamp string) (contextevent.Event, error) {
	if len(receiptBytes) == 0 || receiptBytes[len(receiptBytes)-1] != '\n' {
		return contextevent.Event{}, errors.New("canonical receipt lacks trailing newline")
	}
	schema, err := contextevent.PayloadSchema(contextevent.KindReceipt)
	if err != nil {
		return contextevent.Event{}, err
	}
	receiptJSON := append([]byte(nil), receiptBytes[:len(receiptBytes)-1]...)
	payload := &contextevent.ReceiptPayload{
		Schema: schema, Role: contextevent.RoleBuilder, ReceiptDigest: receipt.Digest,
		Authority: receipt.Authority, ExecutionEventChainRoot: receipt.EventChainRoot,
		Detail: contextevent.Detail{
			Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON,
			Digest: digestBytes(receiptJSON), RedactionProfile: contextevent.RedactionProfileStandard,
			RedactedJSON: receiptJSON,
		},
	}
	return buildEvent(request, child, resultEvent.SourceSequence+1, resultEvent.EventDigest, nil, stamp, contextevent.KindReceipt, payload)
}

func validateSpecializedReceiptAck(event contextevent.Event, receipt contextreceipt.Receipt, resultAck contextevent.EventAck, ack contextevent.ReceiptEventAck) error {
	encoded, err := contextevent.EncodeReceiptEventAck(ack)
	if err != nil {
		return err
	}
	canonical, err := contextevent.DecodeReceiptEventAck(bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	if canonical.Flight != event.Flight || canonical.Lane != event.Lane || canonical.Epoch != event.Epoch || canonical.Session != event.Session ||
		canonical.ManifestRevision != event.ManifestRevision || canonical.Kind != contextevent.KindReceipt ||
		canonical.SourceSequence != event.SourceSequence || canonical.EventDigest != event.EventDigest ||
		canonical.GlobalSequence <= resultAck.GlobalSequence || canonical.ReceiptDigest != receipt.Digest {
		return errors.New("receipt acknowledgment does not bind receipt event and finalized receipt")
	}
	return nil
}

func sortedUniqueStrings(values []string) []string {
	result := append(make([]string, 0, len(values)), values...)
	sort.Strings(result)
	if len(result) < 2 {
		return result
	}
	write := 1
	for _, value := range result[1:] {
		if value != result[write-1] {
			result[write] = value
			write++
		}
	}
	return result[:write]
}
