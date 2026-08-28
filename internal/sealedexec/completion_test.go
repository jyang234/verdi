package sealedexec

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/execworkspace"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

func TestExecutionCompletionService_Behavioral(t *testing.T) {
	t.Run("authoritative completion preserves terminal order and every identity", func(t *testing.T) {
		fixture := newCompletionFixture(t, contextevent.AuthorityAuthoritative)
		completion, err := fixture.service().Complete(context.Background(), fixture.requestValue())
		if err != nil {
			t.Fatalf("Complete: %v", err)
		}
		decoded, err := DecodeExecutionResult(bytes.NewReader(completion.ResultBytes))
		if err != nil {
			t.Fatalf("DecodeExecutionResult: %v", err)
		}
		fixture.calls = append(fixture.calls, "public-result")
		wantOrder := []string{
			"observe-child",
			"append-execution-result",
			"checkpoint",
			"resolve-receipt-inputs",
			"receipt-roundtrip",
			"append-receipt",
			"public-result",
		}
		if !reflect.DeepEqual(fixture.calls, wantOrder) {
			t.Fatalf("terminal order = %q, want %q", fixture.calls, wantOrder)
		}
		assertCompletionIdentity(t, fixture, completion, decoded)
	})

	t.Run("advisory completion remains advisory with explicit witnesses", func(t *testing.T) {
		fixture := newCompletionFixture(t, contextevent.AuthorityAdvisory)
		completion, err := fixture.service().Complete(context.Background(), fixture.requestValue())
		if err != nil {
			t.Fatalf("Complete advisory: %v", err)
		}
		if completion.Result.Authority != contextevent.AuthorityAdvisory || completion.Result.Verdict != contextcompile.ResolutionUnproven {
			t.Fatalf("advisory result authority/verdict = %q/%q", completion.Result.Authority, completion.Result.Verdict)
		}
		if got := completion.Result.Witnesses; !reflect.DeepEqual(got, []string{"runner authority unproven"}) {
			t.Fatalf("advisory witnesses = %q", got)
		}
		if completion.Receipt.Authority != contextreceipt.AuthorityAdvisory || completion.Receipt.RunnerPrincipalResolution.State != gp.ResolutionUnproven {
			t.Fatalf("advisory receipt authority/principal = %q/%q", completion.Receipt.Authority, completion.Receipt.RunnerPrincipalResolution.State)
		}
	})

	mutations := []struct {
		name      string
		configure func(*completionFixture)
		wantLast  string
		wantClass error
	}{
		{
			name: "rejected result acknowledgment",
			configure: func(f *completionFixture) {
				f.recorder.appendErr = errors.New("recorder rejected execution-result")
			},
			wantLast: "append-execution-result", wantClass: ErrOperational,
		},
		{
			name: "mismatched result acknowledgment",
			configure: func(f *completionFixture) {
				f.recorder.mutateAck = func(ack *contextevent.EventAck) { ack.EventDigest = testDigest("wrong-event") }
			},
			wantLast: "append-execution-result", wantClass: ErrOperational,
		},
		{
			name: "truncated revision array",
			configure: func(f *completionFixture) {
				f.request.ManifestRevision = 1
				f.run.Acks[0].ManifestRevision = 1
				f.recorder.mutateCheckpoint = func(checkpoint *RecorderCheckpoint) {
					checkpoint.Revisions = checkpoint.Revisions[1:]
				}
			},
			wantLast: "checkpoint", wantClass: ErrOperational,
		},
		{
			name: "wrong event chain root",
			configure: func(f *completionFixture) {
				f.recorder.mutateCheckpoint = func(checkpoint *RecorderCheckpoint) { checkpoint.EventChainRoot = testDigest("wrong-root") }
			},
			wantLast: "checkpoint", wantClass: ErrOperational,
		},
		{
			name: "bad runner principal",
			configure: func(f *completionFixture) {
				f.inputs.inputs.RunnerPrincipal = completionPrincipal(t, contextevent.AuthorityAdvisory)
			},
			wantLast: "resolve-receipt-inputs", wantClass: ErrOperational,
		},
		{
			name: "noncanonical receipt operands",
			configure: func(f *completionFixture) {
				f.inputs.inputs.Obligations = nil
			},
			wantLast: "resolve-receipt-inputs", wantClass: ErrOperational,
		},
		{
			name: "missing specialized acknowledgment",
			configure: func(f *completionFixture) {
				f.receipts.missingAck = true
			},
			wantLast: "append-receipt", wantClass: ErrOperational,
		},
		{
			name: "mismatched specialized acknowledgment",
			configure: func(f *completionFixture) {
				f.receipts.mutateAck = func(ack *contextevent.ReceiptEventAck) { ack.ReceiptDigest = testDigest("wrong-receipt") }
			},
			wantLast: "append-receipt", wantClass: ErrOperational,
		},
		{
			name: "recorder loss before checkpoint",
			configure: func(f *completionFixture) {
				f.recorder.checkpointErr = errors.New("recorder unavailable")
			},
			wantLast: "checkpoint", wantClass: ErrOperational,
		},
		{
			name: "advisory facts cannot be upgraded",
			configure: func(f *completionFixture) {
				f.run.Authority = contextevent.AuthorityAuthoritative
				f.run.Witnesses = []string{"runner authority unproven"}
			},
			wantLast: "", wantClass: ErrVerdict,
		},
	}
	for _, mutation := range mutations {
		mutation := mutation
		t.Run(mutation.name, func(t *testing.T) {
			fixture := newCompletionFixture(t, contextevent.AuthorityAuthoritative)
			mutation.configure(fixture)
			completion, err := fixture.service().Complete(context.Background(), fixture.requestValue())
			if !errors.Is(err, mutation.wantClass) {
				t.Fatalf("Complete error = %v, want class %v", err, mutation.wantClass)
			}
			if len(completion.ResultBytes) != 0 {
				t.Fatalf("failure exposed public result bytes: %q", completion.ResultBytes)
			}
			if got := lastCompletionCall(fixture.calls); got != mutation.wantLast {
				t.Fatalf("last call = %q, want %q; transcript %q", got, mutation.wantLast, fixture.calls)
			}
			for _, call := range fixture.calls {
				if call == "public-result" {
					t.Fatalf("failure reached public result encoding: %q", fixture.calls)
				}
			}
		})
	}
}

type completionFixture struct {
	t         *testing.T
	request   ExecutionRequest
	run       ExecutionRun
	calls     []string
	workspace *completionWorkspaceFake
	recorder  *completionRecorderFake
	inputs    *completionInputsFake
	receipts  *completionReceiptFake
	stamps    *completionStampFake
}

func newCompletionFixture(t *testing.T, authority contextevent.Authority) *completionFixture {
	t.Helper()
	request := validExecutionRequest(t, ActionStart)
	workspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	workspaceDigest, err := ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		t.Fatal(err)
	}
	runWitnesses := []string{}
	if authority == contextevent.AuthorityAdvisory {
		runWitnesses = []string{"runner authority unproven"}
	}
	fixture := &completionFixture{t: t, request: request}
	fixture.run = ExecutionRun{
		Authority: authority,
		Witnesses: runWitnesses,
		Workspace: WorkspaceFacts{
			Verification:  provenVerification(),
			WorkspaceID:   workspaceID,
			Path:          "/tmp/verdi-execution-child",
			Request:       request.ExecutionWorkspaceRequest,
			RequestDigest: workspaceDigest,
			CurrentCommit: request.InputCommit,
			CurrentTree:   request.InputTree,
			Clean:         true,
		},
		AdapterSessionRef: "codex-session-1",
		Acks: []contextevent.EventAck{{
			Schema: contextevent.AckSchemaID, Flight: request.Flight, Lane: request.Lane,
			Epoch: request.Epoch, Session: request.Session, ManifestRevision: request.ManifestRevision,
			Kind: contextevent.KindAdapterStop, SourceSequence: 3,
			EventDigest: testDigest("adapter-stop"), GlobalSequence: 3,
		}},
	}
	fixture.workspace = &completionWorkspaceFake{fixture: fixture, facts: WorkspaceFacts{
		Verification:  provenVerification(),
		WorkspaceID:   workspaceID,
		Path:          fixture.run.Workspace.Path,
		Request:       request.ExecutionWorkspaceRequest,
		RequestDigest: workspaceDigest,
		CurrentCommit: testSHA2,
		CurrentTree:   testTree2,
		Clean:         true,
	}}
	fixture.recorder = &completionRecorderFake{fixture: fixture}
	fixture.inputs = &completionInputsFake{fixture: fixture, inputs: ReceiptInputs{
		Expansions:      []contextreceipt.Expansion{},
		Obligations:     []contextreceipt.Obligation{},
		Evidence:        []contextreceipt.Evidence{},
		ReviewInputs:    []contextreceipt.ReviewInput{},
		RunnerPrincipal: completionPrincipal(t, authority),
	}}
	fixture.receipts = &completionReceiptFake{fixture: fixture}
	fixture.stamps = &completionStampFake{stamps: []string{"2026-08-28T12:34:56Z", "2026-08-28T12:34:57Z"}}
	return fixture
}

func (f *completionFixture) service() *CompletionService {
	f.t.Helper()
	service, err := NewCompletionService(CompletionPorts{
		Workspace: f.workspace,
		Recorder:  f.recorder,
		Inputs:    f.inputs,
		Receipts:  f.receipts,
		Stamps:    f.stamps,
	})
	if err != nil {
		f.t.Fatalf("NewCompletionService: %v", err)
	}
	return service
}

func (f *completionFixture) requestValue() CompletionRequest {
	return CompletionRequest{Request: f.request, Run: f.run}
}

func completionPrincipal(t *testing.T, authority contextevent.Authority) gp.PrincipalResolution {
	t.Helper()
	state := gp.ResolutionAuthenticated
	if authority == contextevent.AuthorityAdvisory {
		state = gp.ResolutionUnproven
	}
	resolution := gp.PrincipalResolution{
		State:     state,
		Claim:     gp.PrincipalClaim{TrustSource: "fixture", Subject: "runner-1"},
		Witnesses: []gp.Witness{{Code: "authenticated", SourceID: "fixture", EvidenceDigest: testDigest("principal")}},
	}
	if state == gp.ResolutionAuthenticated {
		principalID, err := gp.CanonicalPrincipalID(resolution.Claim.TrustSource, resolution.Claim.Subject)
		if err != nil {
			t.Fatal(err)
		}
		resolution.PrincipalID = principalID
	}
	return resolution
}

func provenVerification() Verification {
	return Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}}
}

type completionWorkspaceFake struct {
	fixture *completionFixture
	facts   WorkspaceFacts
	err     error
}

func (f *completionWorkspaceFake) VerifyWorkspace(_ context.Context, workspacePath string, request execworkspace.Identity) (WorkspaceFacts, error) {
	f.fixture.calls = append(f.fixture.calls, "observe-child")
	if workspacePath != f.fixture.run.Workspace.Path || workspacePath == f.fixture.run.Workspace.WorkspaceID || !request.Equal(f.fixture.request.ExecutionWorkspaceRequest) {
		f.fixture.t.Fatalf("workspace observation query = (%q,%#v), want materialized path %q", workspacePath, request, f.fixture.run.Workspace.Path)
	}
	return f.facts, f.err
}

type completionRecorderFake struct {
	fixture          *completionFixture
	event            contextevent.Event
	ack              contextevent.EventAck
	appendErr        error
	checkpointErr    error
	mutateAck        func(*contextevent.EventAck)
	mutateCheckpoint func(*RecorderCheckpoint)
}

func (f *completionRecorderFake) Append(_ context.Context, event contextevent.Event) (contextevent.EventAck, error) {
	f.fixture.calls = append(f.fixture.calls, "append-execution-result")
	if event.Kind != contextevent.KindExecutionResult {
		f.fixture.t.Fatalf("completion recorder event kind = %q", event.Kind)
	}
	f.event = event
	if f.appendErr != nil {
		return contextevent.EventAck{}, f.appendErr
	}
	last := f.fixture.run.Acks[len(f.fixture.run.Acks)-1]
	f.ack = contextevent.EventAck{
		Schema: contextevent.AckSchemaID, Flight: event.Flight, Lane: event.Lane,
		Epoch: event.Epoch, Session: event.Session, ManifestRevision: event.ManifestRevision,
		Kind: event.Kind, SourceSequence: event.SourceSequence, EventDigest: event.EventDigest,
		GlobalSequence: last.GlobalSequence + 1,
	}
	if f.mutateAck != nil {
		f.mutateAck(&f.ack)
	}
	return f.ack, nil
}

func (f *completionRecorderFake) Checkpoint(_ context.Context, key ExecutionKey) (RecorderCheckpoint, error) {
	f.fixture.calls = append(f.fixture.calls, "checkpoint")
	if key != executionKey(f.fixture.request) {
		f.fixture.t.Fatalf("checkpoint key = %#v", key)
	}
	if f.checkpointErr != nil {
		return RecorderCheckpoint{}, f.checkpointErr
	}
	revision := contextevent.Revision{
		Schema: contextevent.RevisionSchemaID, ManifestRevision: f.event.ManifestRevision,
		ManifestDigest: f.event.ManifestDigest, FirstGlobalSequence: 1,
		TerminalGlobalSequence: f.ack.GlobalSequence, TerminalSourceSequence: f.event.SourceSequence,
		TerminalKind: contextevent.KindExecutionResult, EventRoot: f.event.EventDigest,
	}
	revisions := []contextevent.Revision{revision}
	if revision.ManifestRevision > 0 {
		prior := contextevent.Revision{
			Schema: contextevent.RevisionSchemaID, ManifestRevision: 0,
			ManifestDigest: testDigest("manifest-0"), FirstGlobalSequence: 1,
			TerminalGlobalSequence: 1, TerminalSourceSequence: 1,
			TerminalKind: contextevent.KindChildManifest, EventRoot: testDigest("child-manifest-event"),
		}
		revision.FirstGlobalSequence = prior.TerminalGlobalSequence + 1
		revisions = []contextevent.Revision{prior, revision}
	}
	root, err := contextevent.EventChainRoot(revisions)
	if err != nil {
		f.fixture.t.Fatal(err)
	}
	checkpoint := RecorderCheckpoint{
		Verification: provenVerification(), Digest: testDigest("checkpoint"),
		Revisions: revisions, EventChainRoot: root,
		TerminalSourceSequence: revision.TerminalSourceSequence,
		TerminalGlobalSequence: revision.TerminalGlobalSequence,
	}
	if f.mutateCheckpoint != nil {
		f.mutateCheckpoint(&checkpoint)
	}
	return checkpoint, nil
}

type completionInputsFake struct {
	fixture *completionFixture
	inputs  ReceiptInputs
	err     error
	query   ReceiptInputsQuery
}

func (f *completionInputsFake) ResolveReceiptInputs(_ context.Context, query ReceiptInputsQuery) (ReceiptInputs, error) {
	f.fixture.calls = append(f.fixture.calls, "resolve-receipt-inputs")
	f.query = query
	return f.inputs, f.err
}

type completionReceiptFake struct {
	fixture    *completionFixture
	append     ReceiptAppend
	missingAck bool
	mutateAck  func(*contextevent.ReceiptEventAck)
	err        error
}

func (f *completionReceiptFake) AppendReceipt(_ context.Context, appendValue ReceiptAppend) (contextevent.ReceiptEventAck, error) {
	receiptBytes, err := contextreceipt.EncodeReceipt(appendValue.Receipt)
	if err != nil {
		f.fixture.t.Fatalf("append receipt is not encodable: %v", err)
	}
	decoded, err := contextreceipt.DecodeReceipt(bytes.NewReader(receiptBytes))
	if err != nil || decoded.Digest != appendValue.Receipt.Digest {
		f.fixture.t.Fatalf("append receipt strict roundtrip = %#v, %v", decoded, err)
	}
	f.fixture.calls = append(f.fixture.calls, "receipt-roundtrip", "append-receipt")
	f.append = appendValue
	if f.err != nil {
		return contextevent.ReceiptEventAck{}, f.err
	}
	if f.missingAck {
		return contextevent.ReceiptEventAck{}, nil
	}
	event := appendValue.Event
	last := f.fixture.recorder.ack
	ack := contextevent.ReceiptEventAck{
		Schema: contextevent.ReceiptAckSchemaID, Flight: event.Flight, Lane: event.Lane,
		Epoch: event.Epoch, Session: event.Session, ManifestRevision: event.ManifestRevision,
		Kind: event.Kind, SourceSequence: event.SourceSequence, EventDigest: event.EventDigest,
		GlobalSequence: last.GlobalSequence + 1, ReceiptDigest: appendValue.Receipt.Digest,
	}
	if f.mutateAck != nil {
		f.mutateAck(&ack)
	}
	return ack, nil
}

type completionStampFake struct {
	stamps []string
	next   int
}

func (f *completionStampFake) NextStamp(context.Context) (string, error) {
	if f.next >= len(f.stamps) {
		return "", errors.New("stamp fixture exhausted")
	}
	stamp := f.stamps[f.next]
	f.next++
	return stamp, nil
}

func assertCompletionIdentity(t *testing.T, fixture *completionFixture, completion Completion, decoded ExecutionResult) {
	t.Helper()
	if !reflect.DeepEqual(decoded, completion.Result) {
		t.Fatalf("decoded public result differs from completion result")
	}
	request, run := fixture.request, fixture.run
	if decoded.Flight != request.Flight || decoded.Lane != request.Lane || decoded.Epoch != request.Epoch || decoded.Session != request.Session ||
		decoded.ATCRunway != request.ATCRunway || decoded.ExecutionWorkspaceID != run.Workspace.WorkspaceID ||
		decoded.InputCommit != request.InputCommit || decoded.InputTree != request.InputTree ||
		decoded.OutputCommit != testSHA2 || decoded.OutputTree != testTree2 || !decoded.Clean {
		t.Fatalf("public result identity mismatch: %#v", decoded)
	}
	resultEvent := fixture.recorder.event
	payload, ok := resultEvent.Payload.(*contextevent.ExecutionResultPayload)
	if !ok {
		t.Fatalf("execution-result payload type = %T", resultEvent.Payload)
	}
	digestless := *payload
	digestless.ResultFactsDigest = ""
	wantFactsDigest, err := canonjson.Digest(digestless)
	if err != nil {
		t.Fatal(err)
	}
	if payload.ResultFactsDigest != wantFactsDigest || fixture.inputs.query.ResultFactsDigest != wantFactsDigest {
		t.Fatalf("result facts digest = payload %q query %q want %q", payload.ResultFactsDigest, fixture.inputs.query.ResultFactsDigest, wantFactsDigest)
	}
	canonicalRequest, err := EncodeExecutionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if fixture.inputs.query.DispatchDigest != rawDigest(canonicalRequest) {
		t.Fatalf("dispatch digest = %q", fixture.inputs.query.DispatchDigest)
	}
	if fixture.inputs.query.WorkspaceID != run.Workspace.WorkspaceID || fixture.inputs.query.TerminalRevision != decoded.TerminalManifestRevision ||
		fixture.inputs.query.TerminalSourceSequence != decoded.TerminalSourceSequence || fixture.inputs.query.TerminalGlobalSequence != decoded.TerminalGlobalSequence ||
		fixture.inputs.query.EventChainRoot != decoded.EventChainRoot {
		t.Fatalf("receipt input query identity mismatch: %#v", fixture.inputs.query)
	}
	receiptBytes, err := contextreceipt.EncodeReceipt(completion.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	receiptJSON := bytes.TrimSuffix(receiptBytes, []byte("\n"))
	receiptPayload, ok := fixture.receipts.append.Event.Payload.(*contextevent.ReceiptPayload)
	if !ok {
		t.Fatalf("receipt payload type = %T", fixture.receipts.append.Event.Payload)
	}
	if !bytes.Equal(receiptPayload.Detail.RedactedJSON, receiptJSON) || receiptPayload.Detail.Digest != rawDigest(receiptBytes) ||
		receiptPayload.ReceiptDigest != completion.Receipt.Digest || receiptPayload.ExecutionEventChainRoot != completion.EventChainRoot {
		t.Fatalf("receipt event detail/identity mismatch: %#v", receiptPayload)
	}
	if completion.ReceiptEventAck != decoded.ReceiptEventAck || completion.ReceiptEventAck.ReceiptDigest != completion.Receipt.Digest ||
		completion.ReceiptEventAck.SourceSequence != decoded.TerminalSourceSequence+1 || completion.ReceiptEventAck.GlobalSequence <= decoded.TerminalGlobalSequence {
		t.Fatalf("specialized ack mismatch: %#v", completion.ReceiptEventAck)
	}
}

func lastCompletionCall(calls []string) string {
	if len(calls) == 0 {
		return ""
	}
	return calls[len(calls)-1]
}
