package sealedreview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/sealedexec"
)

const (
	RequestSchemaID = "verdi.sealed-review-request/v1"
	ResultSchemaID  = "verdi.sealed-review-result/v1"
)

// PriorReview is the exact R2 receipt/adjudication lineage identity. It never
// carries provider state or prior reviewer conversation.
type PriorReview struct {
	ReceiptDigest      string `json:"receipt_digest"`
	AdjudicationDigest string `json:"adjudication_digest"`
}

// Request is the strict internal start-only sealed-review invocation.
type Request struct {
	Schema           string
	Packet           Packet
	ExecutionRequest sealedexec.ExecutionRequest
	BuilderReceipt   contextreceipt.Receipt
	PriorReview      *PriorReview
}

// Result is returned only after reviewer receipt completion and its
// specialized durable acknowledgment have both succeeded.
type Result struct {
	Schema          string
	Round           Round
	PacketDigest    string
	ExecutionResult sealedexec.ExecutionResult
	ReviewReceipt   contextreceipt.Receipt
	ReceiptEventAck contextevent.ReceiptEventAck
}

// ReviewExecutor is the existing sealed-execution service's review-only
// start boundary.
type ReviewExecutor interface {
	ExecuteReview(context.Context, sealedexec.ExecutionRequest, []contextcompile.DataItem, sealedexec.ReviewLaunch) (sealedexec.ExecutionRun, error)
}

// ReviewCompletion is the existing sealed-execution receipt completion
// boundary.
type ReviewCompletion interface {
	Complete(context.Context, sealedexec.CompletionRequest) (sealedexec.Completion, error)
}

type ServicePorts struct {
	Executor   ReviewExecutor
	Completion ReviewCompletion
}

type completedR0 struct {
	packet          Packet
	receipt         contextreceipt.Receipt
	reviewer        Reviewer
	request         sealedexec.ExecutionRequest
	providerSession string
}

// Service owns the R0-to-R2 runtime lineage. A service instance accepts at
// most one actual R0 receipt for a builder/candidate and never treats caller
// assertions as proof that an R0 occurred.
type Service struct {
	executor   ReviewExecutor
	completion ReviewCompletion
	mu         sync.Mutex
	r0         map[string]completedR0
}

func NewService(ports ServicePorts) (*Service, error) {
	if ports.Executor == nil || ports.Completion == nil {
		return nil, errors.New("sealedreview: executor and completion ports are required")
	}
	return &Service{executor: ports.Executor, completion: ports.Completion, r0: make(map[string]completedR0)}, nil
}

// Review validates every caller-controlled relationship before launch, then
// validates the acknowledged adapter-start projection before receipt
// completion.
func (s *Service) Review(ctx context.Context, request Request) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("sealedreview: review: nil context")
	}
	if s == nil || s.executor == nil || s.completion == nil {
		return Result{}, errors.New("sealedreview: service is not constructed")
	}
	canonicalRequest, err := EncodeRequest(request)
	if err != nil {
		return Result{}, fmt.Errorf("sealedreview: validate request: %w", err)
	}
	request, err = DecodeRequest(bytes.NewReader(canonicalRequest))
	if err != nil {
		return Result{}, fmt.Errorf("sealedreview: strict-roundtrip request: %w", err)
	}
	packetBytes, builderBytes, data, launch, prior, err := s.validateBeforeLaunch(request)
	if err != nil {
		return Result{}, err
	}
	_ = builderBytes

	run, err := s.executor.ExecuteReview(ctx, request.ExecutionRequest, []contextcompile.DataItem{data}, launch)
	if err != nil {
		return Result{}, fmt.Errorf("sealedreview: execute %s: %w", request.Packet.Round, err)
	}
	if err := validateAcknowledgedLaunch(request, packetBytes, launch, run); err != nil {
		return Result{}, err
	}
	if prior != nil && run.AdapterSessionRef == prior.providerSession {
		return Result{}, errors.New("sealedreview: R2 reused the R0 provider session")
	}

	completion, err := s.completion.Complete(ctx, sealedexec.CompletionRequest{
		Request: request.ExecutionRequest, Run: run, ReceiptRole: contextreceipt.RoleReviewer,
		ReviewInputs: packetItemProjection(request.Packet), ReviewOf: []string{request.BuilderReceipt.Digest},
	})
	if err != nil {
		return Result{}, fmt.Errorf("sealedreview: complete %s reviewer receipt: %w", request.Packet.Round, err)
	}
	if err := validateCompletion(request, completion); err != nil {
		return Result{}, err
	}
	if prior != nil {
		if completion.Receipt.DispatchDigest == prior.receipt.DispatchDigest ||
			completion.Receipt.EventChainRoot == prior.receipt.EventChainRoot ||
			completion.Receipt.Digest == prior.receipt.Digest {
			return Result{}, errors.New("sealedreview: R2 reused an R0 dispatch, event root, or receipt")
		}
	}
	if completion.Receipt.ManifestDigest == request.BuilderReceipt.ManifestDigest ||
		completion.Receipt.DispatchDigest == request.BuilderReceipt.DispatchDigest ||
		completion.Receipt.EventChainRoot == request.BuilderReceipt.EventChainRoot ||
		completion.Receipt.ExecutionWorkspaceID == request.BuilderReceipt.ExecutionWorkspaceID ||
		completion.Receipt.Digest == request.BuilderReceipt.Digest {
		return Result{}, errors.New("sealedreview: review reused a builder execution identity")
	}

	result := Result{
		Schema: ResultSchemaID, Round: request.Packet.Round, PacketDigest: request.Packet.Digest,
		ExecutionResult: completion.Result, ReviewReceipt: completion.Receipt, ReceiptEventAck: completion.ReceiptEventAck,
	}
	resultBytes, err := EncodeResult(result)
	if err != nil {
		return Result{}, fmt.Errorf("sealedreview: encode result: %w", err)
	}
	result, err = DecodeResult(bytes.NewReader(resultBytes))
	if err != nil {
		return Result{}, fmt.Errorf("sealedreview: strict-roundtrip result: %w", err)
	}

	if request.Packet.Round == RoundR0 {
		s.mu.Lock()
		key := r0Key(request.BuilderReceipt.Digest, request.Packet.Candidate)
		if _, exists := s.r0[key]; exists {
			s.mu.Unlock()
			return Result{}, errors.New("sealedreview: an actual R0 is already recorded for this builder and candidate")
		}
		s.r0[key] = completedR0{
			packet: clonePacket(request.Packet), receipt: completion.Receipt, reviewer: request.Packet.Reviewer,
			request: request.ExecutionRequest, providerSession: run.AdapterSessionRef,
		}
		s.mu.Unlock()
	}
	return result, nil
}

func (s *Service) validateBeforeLaunch(request Request) ([]byte, []byte, contextcompile.DataItem, sealedexec.ReviewLaunch, *completedR0, error) {
	packetBytes, err := EncodePacket(request.Packet)
	if err != nil {
		return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, fmt.Errorf("sealedreview: packet: %w", err)
	}
	builderBytes, err := contextreceipt.EncodeReceipt(request.BuilderReceipt)
	if err != nil {
		return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, fmt.Errorf("sealedreview: builder receipt: %w", err)
	}
	if request.BuilderReceipt.Role != contextreceipt.RoleBuilder || request.BuilderReceipt.Digest != request.Packet.BuilderReceiptDigest {
		return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, errors.New("sealedreview: packet does not bind the supplied builder receipt")
	}
	if len(request.Packet.Items) < 4 || request.Packet.Items[3].Kind != ItemBuilderReceipt ||
		request.Packet.Items[3].ID != request.BuilderReceipt.Digest || !bytes.Equal(request.Packet.Items[3].Content, builderBytes) {
		return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, errors.New("sealedreview: packet builder-receipt item is not the supplied canonical receipt")
	}
	if request.BuilderReceipt.OutputCommit != request.Packet.Candidate.HeadCommit ||
		request.BuilderReceipt.OutputTree != request.Packet.Candidate.HeadTree || !request.BuilderReceipt.Clean {
		return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, errors.New("sealedreview: builder receipt is stale for the packet candidate")
	}

	exec := request.ExecutionRequest
	if exec.Action != sealedexec.ActionStart || exec.Start == nil || exec.Resume != nil {
		return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, errors.New("sealedreview: reviews are start-only")
	}
	if exec.Lane != request.Packet.Reviewer.Lane || exec.Adapter != request.Packet.Reviewer.Adapter ||
		exec.AdapterVersion != request.Packet.Reviewer.AdapterVersion || exec.Profile.ID != request.Packet.Reviewer.ProfileID ||
		exec.Profile.Digest != request.Packet.Reviewer.ProfileDigest {
		return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, errors.New("sealedreview: execution request reviewer identity contradicts packet")
	}
	if exec.InputCommit != request.Packet.Candidate.HeadCommit || exec.InputTree != request.Packet.Candidate.HeadTree {
		return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, errors.New("sealedreview: execution request candidate contradicts packet")
	}
	if err := validatePacketCompilation(request.Packet, exec); err != nil {
		return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, err
	}
	workspaceID, err := exec.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, fmt.Errorf("sealedreview: derive workspace: %w", err)
	}
	if workspaceID == request.BuilderReceipt.ExecutionWorkspaceID {
		return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, errors.New("sealedreview: review reused the builder workspace")
	}
	data, _, err := contextcompile.BuildDataItem(contextcompile.Candidate{
		ID: "ref:spec/review-packet", Source: contextcompile.SourceDeclaredContext, Ref: "spec/review-packet",
	}, contextcompile.IncludedDeclaredContextRef, packetBytes)
	if err != nil {
		return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, fmt.Errorf("sealedreview: wrap packet data: %w", err)
	}

	launch := sealedexec.ReviewLaunch{Round: string(request.Packet.Round), PacketDigest: request.Packet.Digest, Model: request.Packet.Reviewer.Model}
	switch request.Packet.Round {
	case RoundR0:
		if request.PriorReview != nil {
			return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, errors.New("sealedreview: R0 prior_review must be null")
		}
		return packetBytes, builderBytes, data, launch, nil, nil
	case RoundR2:
		if request.PriorReview == nil {
			return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, errors.New("sealedreview: R2 requires prior_review")
		}
		prior, err := s.actualR0(request)
		if err != nil {
			return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, err
		}
		if err := validateR2Lineage(request, prior); err != nil {
			return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, err
		}
		launch.PriorReview = &sealedexec.ReviewPrior{ReceiptDigest: request.PriorReview.ReceiptDigest, AdjudicationDigest: request.PriorReview.AdjudicationDigest}
		return packetBytes, builderBytes, data, launch, &prior, nil
	default:
		return nil, nil, contextcompile.DataItem{}, sealedexec.ReviewLaunch{}, nil, fmt.Errorf("sealedreview: unknown round %q", request.Packet.Round)
	}
}

func (s *Service) actualR0(request Request) (completedR0, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prior, ok := s.r0[r0Key(request.BuilderReceipt.Digest, request.Packet.Candidate)]
	if !ok {
		return completedR0{}, errors.New("sealedreview: R2 has no actual completed R0 for this builder and candidate")
	}
	return prior, nil
}

func validateR2Lineage(request Request, prior completedR0) error {
	if request.PriorReview.ReceiptDigest != prior.receipt.Digest {
		return errors.New("sealedreview: R2 prior receipt is not the actual R0 receipt")
	}
	if !reflect.DeepEqual(request.Packet.Reviewer, prior.reviewer) {
		return errors.New("sealedreview: R2 configured reviewer identity drifted from R0")
	}
	if request.ExecutionRequest.Session == prior.request.Session {
		return errors.New("sealedreview: R2 reused the R0 Verdi session")
	}
	wantWorkspace, err := request.ExecutionRequest.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		return err
	}
	priorWorkspace, err := prior.request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		return err
	}
	if wantWorkspace == priorWorkspace || request.Packet.Digest == prior.packet.Digest ||
		request.ExecutionRequest.ManifestDigest == prior.request.ManifestDigest {
		return errors.New("sealedreview: R2 reused an R0 workspace, packet, or manifest")
	}
	if len(request.Packet.Items) != 7 || request.Packet.Items[5].Kind != ItemAdjudication || request.Packet.Items[6].Kind != ItemCurrentCandidateEvidence {
		return errors.New("sealedreview: R2 lacks the exact adjudication/current-evidence inventory")
	}
	adjudicationItem := request.Packet.Items[5]
	if adjudicationItem.ID != prior.receipt.Digest || request.PriorReview.AdjudicationDigest != adjudicationItem.ContentDigest {
		return errors.New("sealedreview: R2 prior review does not bind the adjudication item")
	}
	adjudication, err := DecodeAdjudication(bytes.NewReader(adjudicationItem.Content))
	if err != nil {
		return fmt.Errorf("sealedreview: R2 adjudication: %w", err)
	}
	if adjudication.R0ReceiptDigest != prior.receipt.Digest {
		return errors.New("sealedreview: R2 adjudication does not bind the actual R0 receipt")
	}
	if !reflect.DeepEqual(prior.receipt.ReviewInputs, packetItemProjection(prior.packet)) ||
		len(prior.receipt.ReviewOf) != 1 || prior.receipt.ReviewOf[0] != request.BuilderReceipt.Digest {
		return errors.New("sealedreview: actual R0 receipt lacks exact packet or builder lineage")
	}
	return nil
}

func validatePacketCompilation(packet Packet, request sealedexec.ExecutionRequest) error {
	if request.Manifest.Phase != contextcompile.PhaseReview || request.Manifest.Adapter.ID != string(packet.Reviewer.Adapter) ||
		request.Manifest.Adapter.Version != packet.Reviewer.AdapterVersion || request.Manifest.Policy.ProfileID != packet.Reviewer.ProfileID ||
		request.Manifest.Policy.ProfileDigest != packet.Reviewer.ProfileDigest || request.Manifest.GovernanceProfile.ID != packet.Reviewer.ProfileID ||
		request.Manifest.GovernanceProfile.Digest != packet.Reviewer.ProfileDigest {
		return errors.New("sealedreview: compiled manifest reviewer identity contradicts packet")
	}
	manifestBytes, err := contextcompile.EncodeManifest(request.Manifest)
	if err != nil {
		return fmt.Errorf("sealedreview: encode compiled manifest: %w", err)
	}
	projectionBytes, err := sealedexec.EncodeInstructionProjection(request.InstructionProjection)
	if err != nil {
		return fmt.Errorf("sealedreview: encode instruction projection: %w", err)
	}
	if len(request.InstructionProjection.Files) != 1 || len(request.Manifest.ProjectionFiles) != 1 {
		return errors.New("sealedreview: review requires one exact compiled instruction file")
	}
	file := request.InstructionProjection.Files[0]
	binding, err := contextBinding(packet)
	if err != nil {
		return fmt.Errorf("sealedreview: derive packet binding: %w", err)
	}
	compilation := ContextCompileResult{
		ManifestBytes: manifestBytes, ManifestDigest: request.ManifestDigest,
		InstructionProjectionBytes: []byte(file.Content), InstructionProjectionDigest: file.ContentDigest,
		Binding: binding,
	}
	if err := validateContextCompilation(compilation, packet, binding); err != nil {
		return err
	}
	if request.ProjectionDigest != request.InstructionProjection.Digest || len(projectionBytes) == 0 {
		return errors.New("sealedreview: execution projection digest mismatch")
	}
	return nil
}

func validateAcknowledgedLaunch(request Request, packetBytes []byte, launch sealedexec.ReviewLaunch, run sealedexec.ExecutionRun) error {
	if run.ReviewLaunchFacts == nil || run.ReviewLaunchEvent == nil || run.ReviewLaunchAck == nil {
		return errors.New("sealedreview: acknowledged review launch facts are missing")
	}
	workspaceID, err := request.ExecutionRequest.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		return fmt.Errorf("sealedreview: derive review workspace: %w", err)
	}
	want := sealedexec.ReviewLaunchFacts{
		Schema: sealedexec.ReviewLaunchFactsSchemaID, Round: launch.Round, PacketDigest: launch.PacketDigest,
		PriorReview: launch.PriorReview, Lane: request.ExecutionRequest.Lane, Adapter: request.ExecutionRequest.Adapter,
		AdapterVersion: request.ExecutionRequest.AdapterVersion, Model: launch.Model, ProfileID: request.ExecutionRequest.Profile.ID,
		ProfileDigest: request.ExecutionRequest.Profile.Digest, Session: request.ExecutionRequest.Session, WorkspaceID: workspaceID,
	}
	wantBytes, err := sealedexec.EncodeReviewLaunchFacts(want)
	if err != nil {
		return fmt.Errorf("sealedreview: expected launch facts: %w", err)
	}
	gotBytes, err := sealedexec.EncodeReviewLaunchFacts(*run.ReviewLaunchFacts)
	if err != nil {
		return fmt.Errorf("sealedreview: malformed returned launch facts: %w", err)
	}
	if !bytes.Equal(gotBytes, wantBytes) {
		return errors.New("sealedreview: returned launch facts contradict the packet and execution request")
	}
	eventBytes, err := contextevent.EncodeEvent(*run.ReviewLaunchEvent)
	if err != nil {
		return fmt.Errorf("sealedreview: malformed launch event: %w", err)
	}
	event, err := contextevent.DecodeEvent(bytes.NewReader(eventBytes))
	if err != nil {
		return fmt.Errorf("sealedreview: malformed canonical launch event: %w", err)
	}
	ackBytes, err := contextevent.EncodeEventAck(*run.ReviewLaunchAck)
	if err != nil {
		return fmt.Errorf("sealedreview: malformed launch acknowledgment: %w", err)
	}
	ack, err := contextevent.DecodeEventAck(bytes.NewReader(ackBytes))
	if err != nil {
		return fmt.Errorf("sealedreview: malformed canonical launch acknowledgment: %w", err)
	}
	exec := request.ExecutionRequest
	if event.Kind != contextevent.KindAdapterStart || event.Flight != exec.Flight || event.Lane != exec.Lane ||
		event.Epoch != exec.Epoch || event.ManifestRevision != exec.ManifestRevision || event.ManifestDigest != exec.ManifestDigest ||
		event.Session != exec.Session || event.ATCRunway != exec.ATCRunway || event.ExecutionWorkspaceID != workspaceID ||
		event.CandidateCommit != request.Packet.Candidate.HeadCommit || event.CandidateTree != request.Packet.Candidate.HeadTree ||
		event.Adapter != exec.Adapter || event.AdapterVersion != exec.AdapterVersion {
		return errors.New("sealedreview: launch event envelope contradicts request, candidate, manifest, or reviewer")
	}
	if ack.EventDigest != event.EventDigest || ack.SourceSequence != event.SourceSequence || ack.Flight != event.Flight ||
		ack.Lane != event.Lane || ack.Epoch != event.Epoch || ack.Session != event.Session ||
		ack.ManifestRevision != event.ManifestRevision || ack.Kind != event.Kind {
		return errors.New("sealedreview: launch event acknowledgment is mispaired")
	}
	payload, ok := event.Payload.(*contextevent.AdapterStartPayload)
	if !ok || payload.Detail == nil || payload.Detail.Mode != contextevent.DetailInline {
		return errors.New("sealedreview: launch event lacks strict inline review facts")
	}
	if !bytes.Equal(payload.Detail.RedactedJSON, wantBytes) || payload.Detail.Digest != rawDigest(wantBytes) {
		return errors.New("sealedreview: launch event detail contradicts returned launch facts")
	}
	if run.AdapterSessionRef == "" || run.Workspace.WorkspaceID != workspaceID || run.Profile.Ref.ID != request.Packet.Reviewer.ProfileID ||
		run.Profile.Digest != request.Packet.Reviewer.ProfileDigest {
		return errors.New("sealedreview: run identity contradicts acknowledged launch")
	}
	dataPacket, err := DecodePacket(bytes.NewReader(packetBytes))
	if err != nil || dataPacket.Digest != request.Packet.Digest {
		return errors.New("sealedreview: launch packet data is not canonical")
	}
	return nil
}

func validateCompletion(request Request, completion sealedexec.Completion) error {
	resultBytes, err := sealedexec.EncodeExecutionResult(completion.Result)
	if err != nil {
		return fmt.Errorf("sealedreview: malformed execution result: %w", err)
	}
	result, err := sealedexec.DecodeExecutionResult(bytes.NewReader(resultBytes))
	if err != nil {
		return fmt.Errorf("sealedreview: noncanonical execution result: %w", err)
	}
	receiptBytes, err := contextreceipt.EncodeReceipt(completion.Receipt)
	if err != nil {
		return fmt.Errorf("sealedreview: malformed reviewer receipt: %w", err)
	}
	receipt, err := contextreceipt.DecodeReceipt(bytes.NewReader(receiptBytes))
	if err != nil {
		return fmt.Errorf("sealedreview: noncanonical reviewer receipt: %w", err)
	}
	ackBytes, err := contextevent.EncodeReceiptEventAck(completion.ReceiptEventAck)
	if err != nil {
		return fmt.Errorf("sealedreview: malformed receipt event acknowledgment: %w", err)
	}
	ack, err := contextevent.DecodeReceiptEventAck(bytes.NewReader(ackBytes))
	if err != nil {
		return fmt.Errorf("sealedreview: noncanonical receipt event acknowledgment: %w", err)
	}
	exec := request.ExecutionRequest
	if receipt.Role != contextreceipt.RoleReviewer || receipt.Digest != result.Receipt.Digest ||
		!reflect.DeepEqual(receipt, result.Receipt) || !reflect.DeepEqual(ack, result.ReceiptEventAck) ||
		ack.ReceiptDigest != receipt.Digest || !reflect.DeepEqual(receipt, completion.Result.Receipt) ||
		!reflect.DeepEqual(ack, completion.Result.ReceiptEventAck) {
		return errors.New("sealedreview: completion result, receipt, and specialized acknowledgment do not cross-match")
	}
	if len(receipt.ReviewOf) != 1 || receipt.ReviewOf[0] != request.BuilderReceipt.Digest ||
		!reflect.DeepEqual(receipt.ReviewInputs, packetItemProjection(request.Packet)) {
		return errors.New("sealedreview: reviewer receipt does not carry exact builder link and packet projection")
	}
	if result.Flight != exec.Flight || result.Lane != exec.Lane || result.Epoch != exec.Epoch || result.Session != exec.Session ||
		result.Adapter != exec.Adapter || result.AdapterVersion != exec.AdapterVersion || result.InputCommit != exec.InputCommit ||
		result.InputTree != exec.InputTree || result.OutputCommit != request.Packet.Candidate.HeadCommit ||
		result.OutputTree != request.Packet.Candidate.HeadTree || !result.Clean || result.TerminalManifestDigest != exec.ManifestDigest {
		return errors.New("sealedreview: execution result contradicts request, candidate, or manifest")
	}
	if completion.EventChainRoot != receipt.EventChainRoot || completion.Receipt.Digest != receipt.Digest ||
		completion.ReceiptEventAck.ReceiptDigest != receipt.Digest {
		return errors.New("sealedreview: completion summary contradicts immutable reviewer receipt")
	}
	return nil
}

func r0Key(builderDigest string, candidate contextreceipt.Candidate) string {
	return builderDigest + "\x00" + candidate.HeadCommit + "\x00" + candidate.HeadTree
}

type requestDoc struct {
	Schema           *string         `json:"schema"`
	Packet           json.RawMessage `json:"packet"`
	ExecutionRequest json.RawMessage `json:"execution_request"`
	BuilderReceipt   json.RawMessage `json:"builder_receipt"`
	PriorReview      json.RawMessage `json:"prior_review"`
}

func EncodeRequest(request Request) ([]byte, error) {
	if request.Schema != RequestSchemaID {
		return nil, fmt.Errorf("sealedreview: request schema must be %q", RequestSchemaID)
	}
	packet, err := EncodePacket(request.Packet)
	if err != nil {
		return nil, err
	}
	exec, err := sealedexec.EncodeExecutionRequest(request.ExecutionRequest)
	if err != nil {
		return nil, err
	}
	builder, err := contextreceipt.EncodeReceipt(request.BuilderReceipt)
	if err != nil {
		return nil, err
	}
	prior := json.RawMessage("null")
	if request.PriorReview != nil {
		if err := validatePriorReview(*request.PriorReview); err != nil {
			return nil, err
		}
		prior, err = marshalNested(*request.PriorReview)
		if err != nil {
			return nil, err
		}
	}
	schema := request.Schema
	return canonjson.Marshal(requestDoc{Schema: &schema, Packet: trimLF(packet), ExecutionRequest: trimLF(exec), BuilderReceipt: trimLF(builder), PriorReview: prior})
}

func DecodeRequest(reader io.Reader) (Request, error) {
	raw, doc, err := decodeRequestDoc(reader)
	if err != nil {
		return Request{}, err
	}
	if doc.Schema == nil || doc.Packet == nil || doc.ExecutionRequest == nil || doc.BuilderReceipt == nil || doc.PriorReview == nil {
		return Request{}, errors.New("sealedreview: request has an absent mandatory field")
	}
	packet, err := DecodePacket(bytes.NewReader(withLF(doc.Packet)))
	if err != nil {
		return Request{}, fmt.Errorf("sealedreview: request packet: %w", err)
	}
	exec, err := sealedexec.DecodeExecutionRequest(bytes.NewReader(withLF(doc.ExecutionRequest)))
	if err != nil {
		return Request{}, fmt.Errorf("sealedreview: request execution_request: %w", err)
	}
	builder, err := contextreceipt.DecodeReceipt(bytes.NewReader(withLF(doc.BuilderReceipt)))
	if err != nil {
		return Request{}, fmt.Errorf("sealedreview: request builder_receipt: %w", err)
	}
	var prior *PriorReview
	if !bytes.Equal(doc.PriorReview, []byte("null")) {
		var decoded PriorReview
		if err := artifact.DecodeExactJSON(doc.PriorReview, &decoded); err != nil {
			return Request{}, fmt.Errorf("sealedreview: request prior_review: %w", err)
		}
		if err := validatePriorReview(decoded); err != nil {
			return Request{}, err
		}
		prior = &decoded
	}
	request := Request{Schema: *doc.Schema, Packet: packet, ExecutionRequest: exec, BuilderReceipt: builder, PriorReview: prior}
	canonical, err := EncodeRequest(request)
	if err != nil {
		return Request{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return Request{}, errors.New("sealedreview: request is not byte-canonical")
	}
	return request, nil
}

func decodeRequestDoc(reader io.Reader) ([]byte, requestDoc, error) {
	if reader == nil {
		return nil, requestDoc{}, errors.New("sealedreview: decode request: nil reader")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, requestDoc{}, err
	}
	var doc requestDoc
	if err := artifact.DecodeExactJSON(raw, &doc); err != nil {
		return nil, requestDoc{}, fmt.Errorf("sealedreview: decode request: %w", err)
	}
	return raw, doc, nil
}

type resultDoc struct {
	Schema          *string         `json:"schema"`
	Round           *Round          `json:"round"`
	PacketDigest    *string         `json:"packet_digest"`
	ExecutionResult json.RawMessage `json:"execution_result"`
	ReviewReceipt   json.RawMessage `json:"review_receipt"`
	ReceiptEventAck json.RawMessage `json:"receipt_event_ack"`
}

func EncodeResult(result Result) ([]byte, error) {
	if err := validateResultIdentity(result); err != nil {
		return nil, err
	}
	execution, err := sealedexec.EncodeExecutionResult(result.ExecutionResult)
	if err != nil {
		return nil, err
	}
	receipt, err := contextreceipt.EncodeReceipt(result.ReviewReceipt)
	if err != nil {
		return nil, err
	}
	ack, err := contextevent.EncodeReceiptEventAck(result.ReceiptEventAck)
	if err != nil {
		return nil, err
	}
	schema, round, packetDigest := result.Schema, result.Round, result.PacketDigest
	return canonjson.Marshal(resultDoc{
		Schema: &schema, Round: &round, PacketDigest: &packetDigest, ExecutionResult: trimLF(execution),
		ReviewReceipt: trimLF(receipt), ReceiptEventAck: trimLF(ack),
	})
}

func DecodeResult(reader io.Reader) (Result, error) {
	if reader == nil {
		return Result{}, errors.New("sealedreview: decode result: nil reader")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return Result{}, err
	}
	var doc resultDoc
	if err := artifact.DecodeExactJSON(raw, &doc); err != nil {
		return Result{}, fmt.Errorf("sealedreview: decode result: %w", err)
	}
	if doc.Schema == nil || doc.Round == nil || doc.PacketDigest == nil || doc.ExecutionResult == nil || doc.ReviewReceipt == nil || doc.ReceiptEventAck == nil {
		return Result{}, errors.New("sealedreview: result has an absent mandatory field")
	}
	execution, err := sealedexec.DecodeExecutionResult(bytes.NewReader(withLF(doc.ExecutionResult)))
	if err != nil {
		return Result{}, err
	}
	receipt, err := contextreceipt.DecodeReceipt(bytes.NewReader(withLF(doc.ReviewReceipt)))
	if err != nil {
		return Result{}, err
	}
	ack, err := contextevent.DecodeReceiptEventAck(bytes.NewReader(withLF(doc.ReceiptEventAck)))
	if err != nil {
		return Result{}, err
	}
	result := Result{Schema: *doc.Schema, Round: *doc.Round, PacketDigest: *doc.PacketDigest, ExecutionResult: execution, ReviewReceipt: receipt, ReceiptEventAck: ack}
	canonical, err := EncodeResult(result)
	if err != nil {
		return Result{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return Result{}, errors.New("sealedreview: result is not byte-canonical")
	}
	return result, nil
}

func validatePriorReview(prior PriorReview) error {
	if !digestPattern.MatchString(prior.ReceiptDigest) || !digestPattern.MatchString(prior.AdjudicationDigest) {
		return errors.New("sealedreview: prior_review requires canonical receipt and adjudication digests")
	}
	return nil
}

func validateResultIdentity(result Result) error {
	if result.Schema != ResultSchemaID {
		return fmt.Errorf("sealedreview: result schema must be %q", ResultSchemaID)
	}
	if err := validateRound(result.Round); err != nil {
		return err
	}
	if !digestPattern.MatchString(result.PacketDigest) {
		return errors.New("sealedreview: result packet_digest is invalid")
	}
	if result.ReviewReceipt.Role != contextreceipt.RoleReviewer || !reflect.DeepEqual(result.ExecutionResult.Receipt, result.ReviewReceipt) ||
		result.ReceiptEventAck.ReceiptDigest != result.ReviewReceipt.Digest || !reflect.DeepEqual(result.ExecutionResult.ReceiptEventAck, result.ReceiptEventAck) {
		return errors.New("sealedreview: result receipt and acknowledgment do not cross-match")
	}
	return nil
}

func trimLF(raw []byte) json.RawMessage {
	return append(json.RawMessage(nil), bytes.TrimSuffix(raw, []byte("\n"))...)
}

func withLF(raw []byte) []byte {
	return append(append([]byte(nil), raw...), '\n')
}

func marshalNested(value any) (json.RawMessage, error) {
	raw, err := canonjson.Marshal(value)
	return trimLF(raw), err
}
