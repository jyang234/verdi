package sealedexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/execworkspace"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

func TestExecutionCompletionService_Behavioral(t *testing.T) {
	t.Run("reviewer completion changes only receipt role link and packet projection", func(t *testing.T) {
		fixture := newCompletionFixture(t, contextevent.AuthorityAuthoritative)
		inputs := []contextreceipt.ReviewInput{
			{Kind: "accepted-spec", ContentDigest: testDigest("review-spec")},
			{Kind: "builder-receipt", ContentDigest: testDigest("builder-receipt-item")},
		}
		fixture.inputs.inputs.ReviewInputs = append([]contextreceipt.ReviewInput(nil), inputs...)
		request := fixture.requestValue()
		request.ReceiptRole = contextreceipt.RoleReviewer
		request.ReviewInputs = append([]contextreceipt.ReviewInput(nil), inputs...)
		request.ReviewOf = []string{testDigest("builder-receipt")}
		completion, err := fixture.service().Complete(context.Background(), request)
		if err != nil {
			t.Fatalf("Complete reviewer: %v", err)
		}
		if completion.Receipt.Role != contextreceipt.RoleReviewer || !reflect.DeepEqual(completion.Receipt.ReviewInputs, inputs) || !reflect.DeepEqual(completion.Receipt.ReviewOf, request.ReviewOf) {
			t.Fatalf("reviewer receipt role/inputs/link = %q/%#v/%#v", completion.Receipt.Role, completion.Receipt.ReviewInputs, completion.Receipt.ReviewOf)
		}
		payload, ok := fixture.receipts.append.Event.Payload.(*contextevent.ReceiptPayload)
		if !ok || payload.Role != contextevent.RoleReviewer {
			t.Fatalf("reviewer receipt event payload = %#v", fixture.receipts.append.Event.Payload)
		}
		if got := fixture.calls; !reflect.DeepEqual(got, []string{"observe-child", "append-execution-result", "checkpoint", "resolve-receipt-inputs", "receipt-roundtrip", "append-receipt"}) {
			t.Fatalf("reviewer terminal order = %q", got)
		}
	})

	reviewerMutations := []struct {
		name      string
		configure func(*completionFixture, *CompletionRequest)
	}{
		{"missing builder link", func(_ *completionFixture, request *CompletionRequest) { request.ReviewOf = nil }},
		{"multiple builder links", func(_ *completionFixture, request *CompletionRequest) {
			request.ReviewOf = append(request.ReviewOf, testDigest("other-builder"))
		}},
		{"missing packet projection", func(_ *completionFixture, request *CompletionRequest) { request.ReviewInputs = nil }},
		{"controller packet projection mismatch", func(f *completionFixture, _ *CompletionRequest) {
			f.inputs.inputs.ReviewInputs[0].ContentDigest = testDigest("wrong-packet-item")
		}},
	}
	for _, mutation := range reviewerMutations {
		mutation := mutation
		t.Run("reviewer refuses "+mutation.name, func(t *testing.T) {
			fixture := newCompletionFixture(t, contextevent.AuthorityAuthoritative)
			inputs := []contextreceipt.ReviewInput{{Kind: "accepted-spec", ContentDigest: testDigest("review-spec")}}
			fixture.inputs.inputs.ReviewInputs = append([]contextreceipt.ReviewInput(nil), inputs...)
			request := fixture.requestValue()
			request.ReceiptRole = contextreceipt.RoleReviewer
			request.ReviewInputs = append([]contextreceipt.ReviewInput(nil), inputs...)
			request.ReviewOf = []string{testDigest("builder-receipt")}
			mutation.configure(fixture, &request)
			completion, err := fixture.service().Complete(context.Background(), request)
			if !errors.Is(err, ErrOperational) {
				t.Fatalf("Complete reviewer mutation error = %v, want operational", err)
			}
			if completion.Receipt.Digest != "" || fixture.receipts.append.Receipt.Digest != "" {
				t.Fatalf("reviewer mutation minted receipt: completion=%q append=%q", completion.Receipt.Digest, fixture.receipts.append.Receipt.Digest)
			}
		})
	}

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
				f.run.Terminal = completionTerminal(f.request, f.request.ManifestDigest, f.run.Acks)
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
			name: "active recorder revision",
			configure: func(f *completionFixture) {
				f.recorder.mutateCheckpoint = func(checkpoint *RecorderCheckpoint) {
					checkpoint.ActiveRevision = &ActiveRevision{Revision: 3, ManifestDigest: testDigest("active-manifest"), NextSourceSequence: 2, PriorEventDigest: testDigest("active-event"), LastGlobalSequence: checkpoint.TerminalGlobalSequence + 1}
				}
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

// TestReceiptDigestDomains verifies that the inline receipt detail carries two
// distinct digest domains: the self-digest (sha256 of inline bytes as-is) and
// the represented-byte digest (sha256 of receipt with Digest field cleared), and
// that the inline bytes are byte-canonical on re-encode.
func TestReceiptDigestDomains(t *testing.T) {
	fixture := newCompletionFixture(t, contextevent.AuthorityAuthoritative)
	completion, err := fixture.service().Complete(context.Background(), fixture.requestValue())
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	receiptPayload, ok := fixture.receipts.append.Event.Payload.(*contextevent.ReceiptPayload)
	if !ok {
		t.Fatalf("receipt payload type = %T", fixture.receipts.append.Event.Payload)
	}
	if receiptPayload.Detail.Mode != contextevent.DetailInline {
		t.Fatalf("receipt detail mode = %q, want inline", receiptPayload.Detail.Mode)
	}
	inlineBytes := []byte(receiptPayload.Detail.RedactedJSON)

	// Self-digest domain: Detail.Digest = sha256(inlineBytes as-is).
	wantSelfDigest := digestBytes(inlineBytes)
	if receiptPayload.Detail.Digest != wantSelfDigest {
		t.Fatalf("self-digest = %q, want %q", receiptPayload.Detail.Digest, wantSelfDigest)
	}

	// Represented-byte domain: receipt.Digest is content-addressed (Digest="" cleared in preimage).
	representedDigest := completion.Receipt.Digest
	if representedDigest == "" {
		t.Fatalf("receipt.Digest is empty")
	}

	// The two domains must be distinct (different preimages: self has Digest field embedded;
	// represented has Digest="" cleared).
	if receiptPayload.Detail.Digest == representedDigest {
		t.Fatalf("self-digest and represented-byte digest must be distinct, both = %q", representedDigest)
	}

	// Strict re-decode/re-encode: inline bytes with LF appended must be byte-canonical.
	representedBytes := append(append([]byte(nil), inlineBytes...), '\n')
	reDecoded, rerr := contextreceipt.DecodeReceipt(bytes.NewReader(representedBytes))
	if rerr != nil {
		t.Fatalf("re-decode inline receipt: %v", rerr)
	}
	reEncoded, rerr := contextreceipt.EncodeReceipt(reDecoded)
	if rerr != nil {
		t.Fatalf("re-encode inline receipt: %v", rerr)
	}
	if !bytes.Equal(representedBytes, reEncoded) {
		t.Fatalf("inline receipt is not byte-canonical on re-encode")
	}
}

// TestExecutionCompletionSharedStream_Behavioral prosecutes SI-163: completion
// consumes the one shared canonical acknowledgment stream, refuses a stream
// that hides the embedded scoped-MCP transition or contradicts the terminal
// snapshot, and binds every terminal artifact to the actual post-expansion
// manifest revision and digest rather than the original request revision.
func TestExecutionCompletionSharedStream_Behavioral(t *testing.T) {
	t.Run("completion binds the actual post-expansion terminal revision", func(t *testing.T) {
		fixture := newExpandedCompletionFixture(t)
		childDigest := fixture.run.Terminal.ManifestDigest
		if childDigest == fixture.request.ManifestDigest {
			t.Fatal("expanded fixture must install a distinct child manifest digest")
		}
		completion, err := fixture.service().Complete(context.Background(), fixture.requestValue())
		if err != nil {
			t.Fatalf("Complete after an embedded expansion: %v", err)
		}
		last := fixture.run.Acks[len(fixture.run.Acks)-1]
		event := fixture.recorder.event
		if event.ManifestRevision != last.ManifestRevision || event.ManifestDigest != childDigest ||
			event.SourceSequence != last.SourceSequence+1 || event.PriorEventDigest != last.EventDigest {
			t.Fatalf("execution-result identity = revision %d digest %q sequence %d prior %q, want the shared terminal position",
				event.ManifestRevision, event.ManifestDigest, event.SourceSequence, event.PriorEventDigest)
		}
		payload, ok := event.Payload.(*contextevent.ExecutionResultPayload)
		if !ok || payload.ManifestDigest != childDigest {
			t.Fatalf("execution-result payload = %#v, want the actual terminal manifest digest %q", event.Payload, childDigest)
		}
		if completion.Result.TerminalManifestRevision != last.ManifestRevision || completion.Result.TerminalManifestDigest != childDigest ||
			completion.Result.TerminalSourceSequence != event.SourceSequence {
			t.Fatalf("public result terminal = revision %d digest %q sequence %d", completion.Result.TerminalManifestRevision, completion.Result.TerminalManifestDigest, completion.Result.TerminalSourceSequence)
		}
		if completion.Receipt.TerminalManifestRevision != last.ManifestRevision || completion.Receipt.ManifestDigest != childDigest {
			t.Fatalf("receipt terminal = revision %d digest %q, want the actual child revision/digest", completion.Receipt.TerminalManifestRevision, completion.Receipt.ManifestDigest)
		}
		receiptEvent := fixture.receipts.append.Event
		if receiptEvent.ManifestRevision != last.ManifestRevision || receiptEvent.ManifestDigest != childDigest || receiptEvent.SourceSequence != event.SourceSequence+1 {
			t.Fatalf("receipt event = revision %d digest %q sequence %d", receiptEvent.ManifestRevision, receiptEvent.ManifestDigest, receiptEvent.SourceSequence)
		}
	})

	mutations := []struct {
		name      string
		configure func(*completionFixture)
		wantLast  string
	}{
		{
			name: "service-only acknowledgment subset hides the transition",
			configure: func(f *completionFixture) {
				f.run.Acks = []contextevent.EventAck{f.run.Acks[0], f.run.Acks[4], f.run.Acks[7]}
			},
		},
		{
			name:      "stale terminal revision",
			configure: func(f *completionFixture) { f.run.Terminal.Revision = f.request.ManifestRevision },
		},
		{
			name:      "stale terminal next source sequence",
			configure: func(f *completionFixture) { f.run.Terminal.NextSourceSequence-- },
		},
		{
			name:      "stale terminal prior event digest",
			configure: func(f *completionFixture) { f.run.Terminal.PriorEventDigest = testDigest("wrong-prior-event") },
		},
		{
			name:      "stale terminal global sequence",
			configure: func(f *completionFixture) { f.run.Terminal.LastGlobalSequence-- },
		},
		{
			name:      "terminal snapshot from another execution",
			configure: func(f *completionFixture) { f.run.Terminal.Key.Flight = "other-flight" },
		},
		{
			name: "tampered terminal manifest digest",
			configure: func(f *completionFixture) {
				honest := f.run.Terminal.ManifestDigest
				f.run.Terminal.ManifestDigest = f.request.ManifestDigest
				f.recorder.mutateCheckpoint = func(checkpoint *RecorderCheckpoint) {
					checkpoint.Revisions[len(checkpoint.Revisions)-1].ManifestDigest = honest
				}
			},
			wantLast: "checkpoint",
		},
		{
			name: "child transition skips a manifest revision",
			configure: func(f *completionFixture) {
				for i := 4; i < len(f.run.Acks); i++ {
					f.run.Acks[i].ManifestRevision++
				}
				f.run.Terminal.Revision++
			},
		},
		{
			name: "child revision does not restart source order",
			configure: func(f *completionFixture) {
				for i := 4; i < len(f.run.Acks); i++ {
					f.run.Acks[i].SourceSequence += 4
				}
				f.run.Terminal.NextSourceSequence += 4
			},
		},
		{
			name:      "source sequence gap inside one revision",
			configure: func(f *completionFixture) { f.run.Acks[1].SourceSequence++ },
		},
		{
			name:      "global order does not strictly increase",
			configure: func(f *completionFixture) { f.run.Acks[4].GlobalSequence = f.run.Acks[3].GlobalSequence },
		},
	}
	for _, mutation := range mutations {
		mutation := mutation
		t.Run("refuses "+mutation.name, func(t *testing.T) {
			fixture := newExpandedCompletionFixture(t)
			mutation.configure(fixture)
			completion, err := fixture.service().Complete(context.Background(), fixture.requestValue())
			if err == nil {
				t.Fatal("Complete accepted a contradictory shared acknowledgment stream")
			}
			if len(completion.ResultBytes) != 0 || completion.Receipt.Digest != "" || fixture.receipts.append.Receipt.Digest != "" {
				t.Fatalf("refused completion minted terminal artifacts: bytes=%d receipt=%q", len(completion.ResultBytes), completion.Receipt.Digest)
			}
			if got := lastCompletionCall(fixture.calls); got != mutation.wantLast {
				t.Fatalf("last call = %q, want %q; transcript %q", got, mutation.wantLast, fixture.calls)
			}
		})
	}
}

// newExpandedCompletionFixture is one authoritative run whose shared stream
// carries the embedded scoped-MCP transitions: the service opens the request
// revision, MCP appends request/decision/child-manifest, the remaining
// lifecycle events continue inside the installed child revision, and a second
// denied request installs no child. Only the complete stream is continuous —
// the service-owned subset alone has a hole where each transition ran.
func newExpandedCompletionFixture(t *testing.T) *completionFixture {
	t.Helper()
	fixture := newCompletionFixture(t, contextevent.AuthorityAuthoritative)
	request := fixture.request
	parent, child := request.ManifestRevision, request.ManifestRevision+1
	childDigest := testDigest("child-manifest-1")
	fixture.run.Acks = []contextevent.EventAck{
		completionAck(request, parent, 1, 1, contextevent.KindAdapterStart, testDigest("adapter-start")),
		completionAck(request, parent, 2, 2, contextevent.KindContextRequest, testDigest("context-request")),
		completionAck(request, parent, 3, 3, contextevent.KindContextDecision, testDigest("context-decision")),
		completionAck(request, parent, 4, 4, contextevent.KindChildManifest, testDigest("child-manifest")),
		completionAck(request, child, 1, 5, contextevent.KindProviderMessage, testDigest("provider-message")),
		completionAck(request, child, 2, 6, contextevent.KindContextRequest, testDigest("denied-request")),
		completionAck(request, child, 3, 7, contextevent.KindContextDecision, testDigest("denied-decision")),
		completionAck(request, child, 4, 8, contextevent.KindAdapterStop, testDigest("adapter-stop")),
	}
	fixture.run.Terminal = completionTerminal(request, childDigest, fixture.run.Acks)
	// The controller's terminal rows name the one installed transition; the
	// completed checkpoint carries both revisions.
	fixture.inputs.inputs.Expansions = []contextreceipt.Expansion{{
		RequestID: "context-request:expanded", ParentRevision: parent,
		ParentManifestDigest: testDigest("manifest-0"), ChildRevision: child,
		ChildManifestDigest: childDigest, ExpansionDigest: testDigest("expansion-1"),
	}}
	return fixture
}

// completionAck is one canonical row of the shared acknowledgment stream.
func completionAck(request ExecutionRequest, revision, sequence, global uint64, kind contextevent.Kind, digest string) contextevent.EventAck {
	return contextevent.EventAck{
		Schema: contextevent.AckSchemaID, Flight: request.Flight, Lane: request.Lane,
		Epoch: request.Epoch, Session: request.Session, ManifestRevision: revision,
		Kind: kind, SourceSequence: sequence, EventDigest: digest, GlobalSequence: global,
	}
}

// completionTerminal is the immutable terminal position the shared flight state
// reached at the end of the given stream.
func completionTerminal(request ExecutionRequest, manifestDigest string, stream []contextevent.EventAck) FlightStateSnapshot {
	last := stream[len(stream)-1]
	return FlightStateSnapshot{
		Request: request, Key: executionKey(request), Revision: last.ManifestRevision,
		ManifestDigest: manifestDigest, ProjectionDigest: request.ProjectionDigest,
		NextSourceSequence: last.SourceSequence + 1, PriorEventDigest: last.EventDigest,
		LastGlobalSequence: last.GlobalSequence,
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
	fixture.run.Terminal = completionTerminal(request, request.ManifestDigest, fixture.run.Acks)
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
	ack        contextevent.ReceiptEventAck
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
	f.ack = ack
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
	if !bytes.Equal(receiptPayload.Detail.RedactedJSON, receiptJSON) || receiptPayload.Detail.Digest != rawDigest(receiptJSON) ||
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

// completionSegmentFake is the hermetic §6 controller segment namespace.
type completionSegmentFake struct {
	rows         map[string]RedactedSegment
	storeCalls   int
	resolveCalls int
}

func newCompletionSegmentFake() *completionSegmentFake {
	return &completionSegmentFake{rows: map[string]RedactedSegment{}}
}

func (f *completionSegmentFake) StoreRedactedSegment(_ context.Context, segment RedactedSegment) (StoredSegment, error) {
	f.storeCalls++
	reference, err := segmentReference(segment.Digest)
	if err != nil {
		return StoredSegment{}, err
	}
	if existing, ok := f.rows[reference]; ok && !bytes.Equal(existing.Bytes, segment.Bytes) {
		return StoredSegment{}, errors.New("segment reference collision")
	}
	f.rows[reference] = segment
	return StoredSegment{
		Schema: storedSegmentSchemaID, MediaType: segment.MediaType,
		RedactionProfile: segment.RedactionProfile, Digest: segment.Digest,
		ByteCount: segment.ByteCount, Reference: reference,
	}, nil
}

func (f *completionSegmentFake) ResolveRedactedSegment(_ context.Context, reference string) (RedactedSegment, error) {
	f.resolveCalls++
	segment, ok := f.rows[reference]
	if !ok {
		return RedactedSegment{}, errors.New("unknown segment reference")
	}
	return segment, nil
}

func (f *completionFixture) serviceWithSegments(store SegmentStore) *CompletionService {
	f.t.Helper()
	service, err := NewCompletionService(CompletionPorts{
		Workspace: f.workspace, Recorder: f.recorder, Inputs: f.inputs,
		Receipts: f.receipts, Stamps: f.stamps, Segments: store,
	})
	if err != nil {
		f.t.Fatalf("NewCompletionService: %v", err)
	}
	return service
}

// padReceiptInputs grows the canonical receipt past the inline ceiling using
// evidence rows only; every other terminal fact is unchanged.
func padReceiptInputs(fixture *completionFixture, rows int) {
	evidence := make([]contextreceipt.Evidence, 0, rows)
	for i := 0; i < rows; i++ {
		evidence = append(evidence, contextreceipt.Evidence{
			CommandID:    fmt.Sprintf("command-%04d", i),
			Argv:         []string{"go", "test", fmt.Sprintf("./pkg/%04d/%s", i, strings.Repeat("p", 40))},
			ExitCode:     0,
			Verdict:      countersign.VerdictProven,
			OutputDigest: testDigest(fmt.Sprintf("evidence-%04d", i)),
		})
	}
	fixture.inputs.inputs.Evidence = evidence
}

func TestReceiptSegmentLifecycle(t *testing.T) {
	t.Run("canonical receipt at or above the ceiling stores before append", func(t *testing.T) {
		fixture := newCompletionFixture(t, contextevent.AuthorityAuthoritative)
		fixture.run.Profile.PolicySecretValues = [][]byte{[]byte("fixture-classified-secret")}
		padReceiptInputs(fixture, 120)
		store := newCompletionSegmentFake()
		completion, err := fixture.serviceWithSegments(store).Complete(context.Background(), fixture.requestValue())
		if err != nil {
			t.Fatalf("Complete: %v", err)
		}
		receiptBytes, err := contextreceipt.EncodeReceipt(completion.Receipt)
		if err != nil {
			t.Fatalf("EncodeReceipt: %v", err)
		}
		receiptJSON := receiptBytes[:len(receiptBytes)-1]
		if len(receiptJSON) < contextevent.InlineDetailCeiling+1 {
			t.Fatalf("padded canonical receipt is %d bytes, want at least %d", len(receiptJSON), contextevent.InlineDetailCeiling+1)
		}
		payload, ok := fixture.receipts.append.Event.Payload.(*contextevent.ReceiptPayload)
		if !ok {
			t.Fatalf("receipt payload type = %T", fixture.receipts.append.Event.Payload)
		}
		if payload.Detail.Mode != contextevent.DetailSegment {
			t.Fatalf("receipt detail mode = %q, want segment", payload.Detail.Mode)
		}
		if store.storeCalls == 0 {
			t.Fatal("oversized receipt was appended without storing its segment")
		}
		if payload.Detail.ByteCount != uint64(len(receiptJSON)) {
			t.Fatalf("segment byte_count = %d, want %d", payload.Detail.ByteCount, len(receiptJSON))
		}
		// Represented-byte domain: detail.digest covers the exact receipt JSON.
		if payload.Detail.Digest != digestBytes(receiptJSON) {
			t.Fatalf("segment detail digest = %q, want the represented receipt digest", payload.Detail.Digest)
		}
		// The two domains stay distinct and are never compared for equality.
		if payload.Detail.Digest == completion.Receipt.Digest {
			t.Fatal("represented-byte digest collided with the receipt self-digest")
		}
		resolved, err := store.ResolveRedactedSegment(context.Background(), payload.Detail.Reference)
		if err != nil {
			t.Fatalf("ResolveRedactedSegment: %v", err)
		}
		if !bytes.Equal(resolved.Bytes, receiptJSON) {
			t.Fatal("stored segment bytes are not the exact canonical receipt")
		}
		if store.resolveCalls == 0 {
			t.Fatal("specialized acknowledgment did not resolve the segment detail")
		}
	})

	t.Run("oversized receipt without a segment store refuses", func(t *testing.T) {
		fixture := newCompletionFixture(t, contextevent.AuthorityAuthoritative)
		padReceiptInputs(fixture, 120)
		if _, err := fixture.service().Complete(context.Background(), fixture.requestValue()); err == nil {
			t.Fatal("Complete error = nil, want a refusal for an unstorable receipt")
		} else if !strings.Contains(err.Error(), "segment store") {
			t.Fatalf("Complete error = %v, want a missing segment store witness", err)
		}
	})

	t.Run("inline receipt keeps the existing representation", func(t *testing.T) {
		fixture := newCompletionFixture(t, contextevent.AuthorityAuthoritative)
		fixture.run.Profile.PolicySecretValues = [][]byte{[]byte("fixture-classified-secret")}
		store := newCompletionSegmentFake()
		if _, err := fixture.serviceWithSegments(store).Complete(context.Background(), fixture.requestValue()); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		payload, ok := fixture.receipts.append.Event.Payload.(*contextevent.ReceiptPayload)
		if !ok {
			t.Fatalf("receipt payload type = %T", fixture.receipts.append.Event.Payload)
		}
		if payload.Detail.Mode != contextevent.DetailInline || store.storeCalls != 0 {
			t.Fatalf("inline receipt mode/store calls = %q/%d, want inline/0", payload.Detail.Mode, store.storeCalls)
		}
	})

	t.Run("unresolvable or contradicting segment detail refuses acknowledgment", func(t *testing.T) {
		fixture := newCompletionFixture(t, contextevent.AuthorityAuthoritative)
		fixture.run.Profile.PolicySecretValues = [][]byte{[]byte("fixture-classified-secret")}
		padReceiptInputs(fixture, 120)
		store := newCompletionSegmentFake()
		service := fixture.serviceWithSegments(store)
		if _, err := service.Complete(context.Background(), fixture.requestValue()); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		event := fixture.receipts.append.Event
		payload := event.Payload.(*contextevent.ReceiptPayload)
		receipt := fixture.receipts.append.Receipt
		ack := fixture.receipts.ack

		t.Run("resolution failure", func(t *testing.T) {
			empty := newCompletionSegmentFake()
			bare := fixture.serviceWithSegments(empty)
			if err := bare.validateSpecializedReceiptAck(context.Background(), event, receipt, fixture.recorder.ack, ack); err == nil {
				t.Fatal("error = nil, want a refusal for an unresolvable segment")
			}
		})

		t.Run("no segment store", func(t *testing.T) {
			if err := fixture.service().validateSpecializedReceiptAck(context.Background(), event, receipt, fixture.recorder.ack, ack); err == nil {
				t.Fatal("error = nil, want a refusal without a segment store")
			}
		})

		t.Run("represented bytes contradict the detail digest", func(t *testing.T) {
			mutated := event
			mutatedPayload := *payload
			mutatedPayload.Detail.Digest = testDigest("other-detail")
			mutated.Payload = &mutatedPayload
			if err := service.validateSpecializedReceiptAck(context.Background(), mutated, receipt, fixture.recorder.ack, ack); err == nil {
				t.Fatal("error = nil, want a digest-domain refusal")
			}
		})

		t.Run("represented receipt contradicts the finalized receipt", func(t *testing.T) {
			other := receipt
			other.Digest = testDigest("other-receipt")
			if err := service.validateSpecializedReceiptAck(context.Background(), event, other, fixture.recorder.ack, ack); err == nil {
				t.Fatal("error = nil, want a receipt cross-match refusal")
			}
		})
	})
}
