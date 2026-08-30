package sealedexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/countersign"
)

func TestScopedContextMCPContract_Static(t *testing.T) {
	req := serviceRequest(t, ActionStart)
	state := NewFlightState(mcpSnapshot(t, req, testDigest("empty-expansion")))
	fake := &mcpFake{t: t, request: req, state: state}
	server, err := NewScopedMCP(ScopedMCPPorts{Resolver: fake, Compiler: fake, Verifier: fake, Recorder: fake, Store: fake, Stamps: fake}, state)
	if err != nil {
		t.Fatalf("NewScopedMCP: %v", err)
	}

	tools := server.Tools()
	if len(tools) != 2 || tools[0].Name != ToolGetFlightPlan || tools[1].Name != ToolRequestContext {
		t.Fatalf("tools = %#v", tools)
	}
	wantEmpty := []byte(`{"additionalProperties":false,"properties":{},"type":"object"}`)
	wantRequest := []byte(`{"additionalProperties":false,"properties":{"purpose":{"minLength":1,"type":"string"},"ref":{"minLength":1,"type":"string"}},"required":["ref","purpose"],"type":"object"}`)
	if !bytes.Equal(tools[0].InputSchema, wantEmpty) || !bytes.Equal(tools[1].InputSchema, wantRequest) {
		t.Fatalf("tool schemas = %s / %s", tools[0].InputSchema, tools[1].InputSchema)
	}
	for _, call := range []struct {
		name string
		args []byte
	}{
		{ToolGetFlightPlan, []byte(`{"extra":true}`)},
		{ToolRequestContext, []byte(`{"ref":"spec/extra","purpose":""}`)},
		{ToolRequestContext, []byte(`{"ref":"spec/extra","purpose":"needed","extra":true}`)},
		{"read_file", []byte(`{}`)},
	} {
		if _, err := server.Call(context.Background(), call.name, call.args); !errors.Is(err, ErrOperational) {
			t.Errorf("Call(%q, %s) error = %v, want strict operational refusal", call.name, call.args, err)
		}
	}

	t.Run("flight plan is typed inspection only", func(t *testing.T) {
		result, err := server.Call(context.Background(), ToolGetFlightPlan, []byte(`{}`))
		if err != nil {
			t.Fatalf("get_flight_plan: %v", err)
		}
		if result.Kind != InspectionFlightPlan || result.InstructionAuthority != nil || result.FlightPlan.ManifestDigest != req.ManifestDigest {
			t.Fatalf("flight-plan inspection = %#v", result)
		}
		encoded, err := EncodeInspectionResult(result)
		if err != nil {
			t.Fatalf("EncodeInspectionResult: %v", err)
		}
		decoded, err := DecodeInspectionResult(bytes.NewReader(encoded))
		if err != nil {
			t.Fatalf("DecodeInspectionResult: %v", err)
		}
		if !reflect.DeepEqual(decoded, result) {
			t.Fatalf("inspection round trip = %#v, want %#v", decoded, result)
		}
		noLedger := result
		noLedger.FlightPlan.ExpansionRoot = ""
		encodedNoLedger, err := EncodeInspectionResult(noLedger)
		if err != nil {
			t.Fatalf("EncodeInspectionResult(no installed expansion ledger): %v", err)
		}
		decodedNoLedger, err := DecodeInspectionResult(bytes.NewReader(encodedNoLedger))
		if err != nil || decodedNoLedger.FlightPlan.ExpansionRoot != "" {
			t.Fatalf("no-ledger flight plan roundtrip = %#v/%v", decodedNoLedger, err)
		}
		malformedRoot := result
		malformedRoot.FlightPlan.ExpansionRoot = "relative"
		if _, err := EncodeInspectionResult(malformedRoot); err == nil {
			t.Fatal("encoded flight-plan inspection with malformed installed root")
		}
		for _, bad := range [][]byte{duplicateFirstKey(encoded), withUnknownField(encoded), append(append([]byte(nil), encoded...), []byte("{}\n")...)} {
			if _, err := DecodeInspectionResult(bytes.NewReader(bad)); err == nil {
				t.Fatalf("accepted non-strict inspection result %q", bad)
			}
		}
		wrongArm := result
		wrongArm.Context = ContextInspection{Ref: "secret"}
		if _, err := EncodeInspectionResult(wrongArm); err == nil {
			t.Fatal("encoded flight-plan inspection with a populated context arm")
		}
		deniedWithData := InspectionResult{Kind: InspectionContextDenied, Context: ContextInspection{
			RequestID: "request-1", Ref: "secret/outside", Purpose: "inspect", Witnesses: []string{"out of scope"},
			Data: contextcompile.DataItem{Content: "secret bytes"},
		}}
		if _, err := EncodeInspectionResult(deniedWithData); err == nil {
			t.Fatal("encoded denied inspection carrying data")
		}
	})

	t.Run("approved expansion is fully acknowledged then atomically installed", func(t *testing.T) {
		result, err := server.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"needed for implementation","ref":"spec/extra"}`))
		if err != nil {
			t.Fatalf("request_context: %v", err)
		}
		if result.Kind != InspectionContextApproved || result.InstructionAuthority != nil || result.Context.Data.Content != "declared bytes" {
			t.Fatalf("approved inspection = %#v", result)
		}
		wantKinds := []contextevent.Kind{contextevent.KindContextRequest, contextevent.KindContextDecision, contextevent.KindChildManifest}
		if !reflect.DeepEqual(fake.kinds, wantKinds) || fake.installs != 1 {
			t.Fatalf("events/install = %v/%d", fake.kinds, fake.installs)
		}
		if fake.order[len(fake.order)-2:][0] != "ack:child-manifest" || fake.order[len(fake.order)-1] != "install" {
			t.Fatalf("terminal order = %v", fake.order)
		}
		snapshot := state.Snapshot()
		if snapshot.Revision != 1 || snapshot.NextSourceSequence != 1 || snapshot.PriorRevision == nil || snapshot.PriorRevision.TerminalGlobalSequence != 3 {
			t.Fatalf("installed state = %#v", snapshot)
		}
		priorRevision := *snapshot.PriorRevision
		bridgeFake := &mcpFake{t: t, request: req, kinds: make([]contextevent.Kind, snapshot.LastGlobalSequence)}
		bridgeState := NewFlightState(snapshot)
		bridgeServer := &ScopedMCP{ports: ScopedMCPPorts{Recorder: bridgeFake, Stamps: bridgeFake}, state: bridgeState}
		if err := bridgeServer.appendContextRequest(context.Background(), "bridge-request", "spec/extra", "consume bridge"); err != nil {
			t.Fatalf("append first child event: %v", err)
		}
		if transition := bridgeState.Snapshot(); transition.PriorRevision != nil {
			t.Fatalf("installed revision bridge survived sequence one: %#v", transition.PriorRevision)
		}
		if _, err := server.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"needed again","ref":"spec/extra"}`)); err != nil {
			t.Fatalf("second request_context: %v", err)
		}
		firstChildEvent := fake.events[3]
		if firstChildEvent.ManifestRevision != 1 || firstChildEvent.SourceSequence != 1 || firstChildEvent.PriorRevision == nil || *firstChildEvent.PriorRevision != priorRevision {
			t.Fatalf("first event of child revision = %#v, want exact bridge %#v", firstChildEvent, priorRevision)
		}
	})

	t.Run("request identity binds flight lane and epoch independently", func(t *testing.T) {
		snapshot := mcpSnapshot(t, req, testDigest("empty"))
		base, err := contextRequestID(snapshot, "spec/extra", "inspect")
		if err != nil {
			t.Fatal(err)
		}
		for name, mutate := range map[string]func(*FlightStateSnapshot){
			"flight": func(s *FlightStateSnapshot) { s.Key.Flight = "flight-other" },
			"lane":   func(s *FlightStateSnapshot) { s.Key.Lane = "lane-other" },
			"epoch":  func(s *FlightStateSnapshot) { s.Key.Epoch = "epoch-other" },
		} {
			changed := snapshot
			mutate(&changed)
			got, err := contextRequestID(changed, "spec/extra", "inspect")
			if err != nil {
				t.Fatalf("%s digest: %v", name, err)
			}
			if got == base {
				t.Errorf("%s change did not change context request identity", name)
			}
		}
	})

	t.Run("out of scope denial is recorded without data", func(t *testing.T) {
		localState := NewFlightState(mcpSnapshot(t, req, testDigest("empty")))
		denied := &mcpFake{t: t, request: req, state: localState, resolveVerification: violated(FailureOutOfScope, "ref outside declared scope")}
		local, err := NewScopedMCP(ScopedMCPPorts{Resolver: denied, Compiler: denied, Verifier: denied, Recorder: denied, Store: denied, Stamps: denied}, localState)
		if err != nil {
			t.Fatal(err)
		}
		result, err := local.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"inspect","ref":"secret/outside"}`))
		if err != nil {
			t.Fatalf("denied call: %v", err)
		}
		if result.Kind != InspectionContextDenied || result.Context.Data.Content != "" || !reflect.DeepEqual(denied.kinds, []contextevent.Kind{contextevent.KindContextRequest, contextevent.KindContextDecision}) {
			t.Fatalf("denied result/events = %#v/%v", result, denied.kinds)
		}
		decision, ok := denied.events[1].Payload.(*contextevent.ContextDecisionPayload)
		if !ok || decision.Verdict != countersign.VerdictViolated {
			t.Fatalf("violated denial decision = %#v", denied.events[1].Payload)
		}
	})

	t.Run("unavailable context is recorded as unproven", func(t *testing.T) {
		localState := NewFlightState(mcpSnapshot(t, req, testDigest("empty")))
		unavailable := &mcpFake{t: t, request: req, state: localState, resolveVerification: unproven(FailureUnavailable, "z witness", "a witness", "z witness")}
		local, err := NewScopedMCP(ScopedMCPPorts{Resolver: unavailable, Compiler: unavailable, Verifier: unavailable, Recorder: unavailable, Store: unavailable, Stamps: unavailable}, localState)
		if err != nil {
			t.Fatal(err)
		}
		result, err := local.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"inspect","ref":"spec/extra"}`))
		if err != nil {
			t.Fatalf("unavailable call: %v", err)
		}
		decision, ok := unavailable.events[1].Payload.(*contextevent.ContextDecisionPayload)
		if !ok || decision.Verdict != countersign.VerdictUnproven || !reflect.DeepEqual(decision.Witnesses, []string{"a witness", "z witness"}) {
			t.Fatalf("unavailable denial decision = %#v", unavailable.events[1].Payload)
		}
		if !reflect.DeepEqual(result.Context.Witnesses, []string{"a witness", "z witness"}) {
			t.Fatalf("unavailable inspection witnesses = %v", result.Context.Witnesses)
		}
	})

	t.Run("identity drift invalidates rather than expands", func(t *testing.T) {
		localState := NewFlightState(mcpSnapshot(t, req, testDigest("empty")))
		drift := &mcpFake{t: t, request: req, state: localState, verify: violated(FailureStale, "runway changed")}
		local, err := NewScopedMCP(ScopedMCPPorts{Resolver: drift, Compiler: drift, Verifier: drift, Recorder: drift, Store: drift, Stamps: drift}, localState)
		if err != nil {
			t.Fatal(err)
		}
		result, err := local.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"inspect","ref":"spec/extra"}`))
		if err != nil {
			t.Fatalf("invalidating call: %v", err)
		}
		if result.Kind != InspectionEpochInvalidated || !localState.Snapshot().Invalidated || drift.installs != 0 {
			t.Fatalf("invalidation = %#v, state %#v, installs %d", result, localState.Snapshot(), drift.installs)
		}
		again, err := local.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"inspect again","ref":"spec/extra"}`))
		if err != nil {
			t.Fatalf("inspect invalidated epoch: %v", err)
		}
		if _, err := EncodeInspectionResult(again); err != nil {
			t.Fatalf("encode invalidated inspection: %v; result %#v", err, again)
		}
	})

	t.Run("unavailable epoch recheck invalidates as unproven", func(t *testing.T) {
		localState := NewFlightState(mcpSnapshot(t, req, testDigest("empty")))
		unavailable := &mcpFake{t: t, request: req, state: localState, verify: unproven(FailureUnavailable, "runway verifier unavailable")}
		local, err := NewScopedMCP(ScopedMCPPorts{Resolver: unavailable, Compiler: unavailable, Verifier: unavailable, Recorder: unavailable, Store: unavailable, Stamps: unavailable}, localState)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := local.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"inspect","ref":"spec/extra"}`)); err != nil {
			t.Fatalf("invalidate unavailable epoch: %v", err)
		}
		decision, ok := unavailable.events[1].Payload.(*contextevent.ContextDecisionPayload)
		if !ok || decision.Verdict != countersign.VerdictUnproven {
			t.Fatalf("unavailable epoch decision = %#v", unavailable.events[1].Payload)
		}
	})

	t.Run("recorder rejection cannot advance or install", func(t *testing.T) {
		localState := NewFlightState(mcpSnapshot(t, req, testDigest("empty")))
		rejected := &mcpFake{t: t, request: req, state: localState, appendErrAt: 3}
		local, err := NewScopedMCP(ScopedMCPPorts{Resolver: rejected, Compiler: rejected, Verifier: rejected, Recorder: rejected, Store: rejected, Stamps: rejected}, localState)
		if err != nil {
			t.Fatal(err)
		}
		_, err = local.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"inspect","ref":"spec/extra"}`))
		if !errors.Is(err, ErrOperational) || rejected.installs != 0 || localState.Snapshot().Revision != 0 || !localState.Snapshot().Invalidated {
			t.Fatalf("rejected expansion = error %v, installs %d, state %#v", err, rejected.installs, localState.Snapshot())
		}
		appends := len(rejected.events)
		if _, err := local.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"retry","ref":"spec/extra"}`)); err != nil {
			t.Fatalf("inspect invalidated recorder state: %v", err)
		}
		if len(rejected.events) != appends {
			t.Fatalf("invalidated recorder state admitted %d new events", len(rejected.events)-appends)
		}
	})

	t.Run("expansion store loss after terminal ack invalidates the epoch", func(t *testing.T) {
		localState := NewFlightState(mcpSnapshot(t, req, testDigest("empty")))
		lost := &mcpFake{t: t, request: req, state: localState, storeErr: errors.New("expansion store unavailable")}
		local, err := NewScopedMCP(ScopedMCPPorts{Resolver: lost, Compiler: lost, Verifier: lost, Recorder: lost, Store: lost, Stamps: lost}, localState)
		if err != nil {
			t.Fatal(err)
		}
		_, err = local.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"inspect","ref":"spec/extra"}`))
		if !errors.Is(err, ErrOperational) || !localState.Snapshot().Invalidated || localState.Snapshot().Revision != 0 || lost.installs != 1 {
			t.Fatalf("lost expansion = error %v, state %#v, installs %d", err, localState.Snapshot(), lost.installs)
		}
	})
}

// TestSharedFlightStateSerializesServiceAndMCPAppends proves I-115's single
// append owner. Its recorder prosecutes Amendment 002 §7's durable identity
// (flight,lane,epoch,manifest_revision,source_sequence), so an implementation
// in which the execution service and the embedded scoped MCP each own a
// counter cannot stay green: the two owners collide on the same key.
func TestSharedFlightStateSerializesServiceAndMCPAppends(t *testing.T) {
	req := serviceRequest(t, ActionStart)
	workspace := sharedStateWorkspace(t, req)

	t.Run("one shared state serializes lifecycle and context appends", func(t *testing.T) {
		state := NewFlightState(mcpSnapshot(t, req, ""))
		recorder := &duplicateKeyRecorder{}
		fake := &mcpFake{t: t, request: req, state: state}
		server, err := NewScopedMCP(ScopedMCPPorts{Resolver: fake, Compiler: fake, Verifier: fake, Recorder: recorder, Store: fake, Stamps: fake}, state)
		if err != nil {
			t.Fatalf("NewScopedMCP: %v", err)
		}

		// The service's own lifecycle event opens the shared source order.
		start, err := state.append(context.Background(), recorder, fake, workspace, contextevent.KindAdapterStart, sharedStartPayload(t, req))
		if err != nil {
			t.Fatalf("service adapter-start through shared state: %v", err)
		}
		if start.Event.SourceSequence != 1 || start.Event.ManifestRevision != req.ManifestRevision {
			t.Fatalf("adapter-start identity = revision %d sequence %d, want the request revision at sequence 1", start.Event.ManifestRevision, start.Event.SourceSequence)
		}

		// A successful tool call continues that same order and installs the
		// child manifest.
		result, err := server.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"needed for implementation","ref":"spec/extra"}`))
		if err != nil {
			t.Fatalf("request_context: %v", err)
		}
		if result.Kind != InspectionContextApproved || fake.installs != 1 {
			t.Fatalf("request_context = %#v, installs %d, want one approved installed expansion", result, fake.installs)
		}

		// The next service-owned provider observation must consume the child
		// revision the tool installed, not the original request revision.
		final, err := state.append(context.Background(), recorder, fake, workspace, contextevent.KindProviderMessage, sharedMessagePayload(t))
		if err != nil {
			t.Fatalf("service provider observation through shared state: %v", err)
		}

		wantKinds := []contextevent.Kind{
			contextevent.KindAdapterStart,
			contextevent.KindContextRequest,
			contextevent.KindContextDecision,
			contextevent.KindChildManifest,
			contextevent.KindProviderMessage,
		}
		if got := recorder.kinds(); !reflect.DeepEqual(got, wantKinds) {
			t.Fatalf("acknowledged kinds = %v, want %v", got, wantKinds)
		}
		if recorder.conflicts != 0 || recorder.replays != 0 {
			t.Fatalf("recorder saw %d duplicate-key conflicts and %d replays, want a unique key per event", recorder.conflicts, recorder.replays)
		}
		if err := recorder.assertStrictGlobalOrder(); err != nil {
			t.Fatal(err)
		}

		childRevision := req.ManifestRevision + 1
		if final.Event.ManifestRevision != childRevision || final.Event.SourceSequence != 1 {
			t.Fatalf("provider observation identity = revision %d sequence %d, want child revision %d at sequence 1", final.Event.ManifestRevision, final.Event.SourceSequence, childRevision)
		}
		child := recorder.events[3]
		wantBridge := contextevent.PriorRevision{
			ManifestRevision: req.ManifestRevision, ManifestDigest: req.ManifestDigest,
			EventRoot: child.EventDigest, TerminalSourceSequence: child.SourceSequence,
			TerminalGlobalSequence: recorder.acks[3].GlobalSequence,
		}
		if final.Event.PriorRevision == nil || *final.Event.PriorRevision != wantBridge {
			t.Fatalf("provider observation bridge = %#v, want the exact acknowledged predecessor %#v", final.Event.PriorRevision, wantBridge)
		}
		if snapshot := state.Snapshot(); snapshot.Revision != childRevision || snapshot.NextSourceSequence != 2 || snapshot.PriorRevision != nil || snapshot.LastGlobalSequence != final.Ack.GlobalSequence {
			t.Fatalf("shared state after the transition = %#v", snapshot)
		}
	})

	t.Run("an independent service counter allocates a duplicate key", func(t *testing.T) {
		// The pre-correction shape: scoped MCP advances one state while the
		// execution service allocates from its own. The recorder refuses the
		// collision instead of silently accepting a second event at one key.
		shared := NewFlightState(mcpSnapshot(t, req, ""))
		independent := NewFlightState(mcpSnapshot(t, req, ""))
		recorder := &duplicateKeyRecorder{}
		fake := &mcpFake{t: t, request: req, state: shared}
		server, err := NewScopedMCP(ScopedMCPPorts{Resolver: fake, Compiler: fake, Verifier: fake, Recorder: recorder, Store: fake, Stamps: fake}, shared)
		if err != nil {
			t.Fatalf("NewScopedMCP: %v", err)
		}
		if _, err := server.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"needed for implementation","ref":"spec/extra"}`)); err != nil {
			t.Fatalf("request_context: %v", err)
		}
		if _, err := independent.append(context.Background(), recorder, fake, workspace, contextevent.KindAdapterStart, sharedStartPayload(t, req)); !errors.Is(err, ErrOperational) {
			t.Fatalf("independent service counter append error = %v, want an operational duplicate-key refusal", err)
		}
		if recorder.conflicts != 1 {
			t.Fatalf("recorder duplicate-key conflicts = %d, want exactly the independent allocation", recorder.conflicts)
		}
		if got := independent.Snapshot().NextSourceSequence; got != 1 {
			t.Fatalf("refused append advanced the independent state to sequence %d, want 1", got)
		}
	})
}

// TestSharedFlightStreamContract_Behavioral prosecutes SI-172's retained
// stream: the one shared flight state keeps the complete canonical
// acknowledgment order across service and embedded scoped-MCP appends, hands
// callers isolated copies, invents nothing on a failed append, and never
// records a replayed durable position twice.
func TestSharedFlightStreamContract_Behavioral(t *testing.T) {
	req := serviceRequest(t, ActionStart)
	workspace := sharedStateWorkspace(t, req)

	t.Run("every acknowledged append enters the stream exactly once", func(t *testing.T) {
		state := NewFlightState(mcpSnapshot(t, req, ""))
		recorder := &duplicateKeyRecorder{}
		fake := &mcpFake{t: t, request: req, state: state}
		server, err := NewScopedMCP(ScopedMCPPorts{Resolver: fake, Compiler: fake, Verifier: fake, Recorder: recorder, Store: fake, Stamps: fake}, state)
		if err != nil {
			t.Fatalf("NewScopedMCP: %v", err)
		}
		if _, err := state.append(context.Background(), recorder, fake, workspace, contextevent.KindAdapterStart, sharedStartPayload(t, req)); err != nil {
			t.Fatalf("service adapter-start: %v", err)
		}
		if _, err := server.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"needed for implementation","ref":"spec/extra"}`)); err != nil {
			t.Fatalf("request_context: %v", err)
		}
		if _, err := state.append(context.Background(), recorder, fake, workspace, contextevent.KindProviderMessage, sharedMessagePayload(t)); err != nil {
			t.Fatalf("service provider observation: %v", err)
		}
		terminal := state.Terminal()
		if !reflect.DeepEqual(terminal.Acks, recorder.acks) {
			t.Fatalf("shared stream = %#v, want the exact acknowledged order %#v", terminal.Acks, recorder.acks)
		}
		last := recorder.acks[len(recorder.acks)-1]
		if terminal.Snapshot.Revision != last.ManifestRevision || terminal.Snapshot.NextSourceSequence != last.SourceSequence+1 ||
			terminal.Snapshot.PriorEventDigest != last.EventDigest || terminal.Snapshot.LastGlobalSequence != last.GlobalSequence {
			t.Fatalf("terminal position = %#v, want the last acknowledgment %#v", terminal.Snapshot, last)
		}
	})

	t.Run("callers receive isolated copies of the stream", func(t *testing.T) {
		state := NewFlightState(mcpSnapshot(t, req, ""))
		recorder := &duplicateKeyRecorder{}
		fake := &mcpFake{t: t, request: req, state: state}
		if _, err := state.append(context.Background(), recorder, fake, workspace, contextevent.KindAdapterStart, sharedStartPayload(t, req)); err != nil {
			t.Fatalf("service adapter-start: %v", err)
		}
		copied := state.Terminal()
		if len(copied.Acks) != 1 {
			t.Fatalf("stream = %#v, want exactly the acknowledged adapter-start", copied.Acks)
		}
		copied.Acks[0].GlobalSequence = 99
		copied.Acks = append(copied.Acks, copied.Acks[0])
		if again := state.Terminal(); len(again.Acks) != 1 || again.Acks[0].GlobalSequence != recorder.acks[0].GlobalSequence {
			t.Fatalf("mutating a returned copy changed live state: %#v", again.Acks)
		}
	})

	t.Run("a failed append leaves the stream and position exact", func(t *testing.T) {
		state := NewFlightState(mcpSnapshot(t, req, ""))
		recorder := &duplicateKeyRecorder{}
		fake := &mcpFake{t: t, request: req, state: state}
		if _, err := state.append(context.Background(), recorder, fake, workspace, contextevent.KindAdapterStart, sharedStartPayload(t, req)); err != nil {
			t.Fatalf("service adapter-start: %v", err)
		}
		before := state.Terminal()
		rejecting := &mcpFake{t: t, request: req, appendErrAt: 1}
		if _, err := state.append(context.Background(), rejecting, fake, workspace, contextevent.KindProviderMessage, sharedMessagePayload(t)); !errors.Is(err, ErrOperational) {
			t.Fatalf("rejected append error = %v, want an operational refusal", err)
		}
		after := state.Terminal()
		if !reflect.DeepEqual(after, before) {
			t.Fatalf("rejected append altered shared state: %#v, want %#v", after, before)
		}
	})

	t.Run("an exact replay does not record a second stream row", func(t *testing.T) {
		state := NewFlightState(mcpSnapshot(t, req, ""))
		recorder := &duplicateKeyRecorder{}
		fake := &mcpFake{t: t, request: req, state: state}
		retained := newRetainedEvents()
		first, err := state.appendReplay(context.Background(), recorder, fake, workspace, retained, contextevent.KindAdapterStart, sharedStartPayload(t, req))
		if err != nil {
			t.Fatalf("retained adapter-start: %v", err)
		}
		// The same authenticated position is reopened with its acknowledgment
		// already in the stream; the exact retained bytes replay without a
		// recorder write and must not be counted twice.
		reopened := NewFlightStateAt(mcpSnapshot(t, req, ""), []contextevent.EventAck{first.Ack})
		replayed, err := reopened.appendReplay(context.Background(), recorder, fake, workspace, retained, contextevent.KindAdapterStart, sharedStartPayload(t, req))
		if err != nil {
			t.Fatalf("replayed adapter-start: %v", err)
		}
		if replayed.Ack != first.Ack || recorder.replays != 0 || len(recorder.acks) != 1 {
			t.Fatalf("replay = %#v, recorder replays %d, want the retained acknowledgment without a recorder write", replayed.Ack, recorder.replays)
		}
		if got := reopened.Terminal().Acks; len(got) != 1 || got[0] != first.Ack {
			t.Fatalf("replayed stream = %#v, want exactly one row", got)
		}
	})
}

// duplicateKeyRecorder is Amendment 002 §7's recorder table: an absent key
// stores the exact bytes and allocates the next never-resetting global
// sequence, byte-identical replay returns the original acknowledgment without
// a write, and contradictory bytes at a committed key fail operationally
// without allocating order.
type duplicateKeyRecorder struct {
	committed map[eventKey]recordedEvent
	events    []contextevent.Event
	acks      []contextevent.EventAck
	global    uint64
	conflicts int
	replays   int
}

type recordedEvent struct {
	event contextevent.Event
	ack   contextevent.EventAck
}

func (r *duplicateKeyRecorder) Append(_ context.Context, event contextevent.Event) (contextevent.EventAck, error) {
	key := eventKey{revision: event.ManifestRevision, sequence: event.SourceSequence}
	if prior, ok := r.committed[key]; ok {
		ack, err := contextevent.ValidateReplay(prior.event, prior.ack, event)
		if err != nil {
			r.conflicts++
			return contextevent.EventAck{}, fmt.Errorf("duplicate event identity %+v: %w", key, err)
		}
		r.replays++
		return ack, nil
	}
	r.global++
	ack := contextevent.EventAck{
		Schema: contextevent.AckSchemaID, Flight: event.Flight, Lane: event.Lane, Epoch: event.Epoch,
		Session: event.Session, ManifestRevision: event.ManifestRevision, Kind: event.Kind,
		SourceSequence: event.SourceSequence, EventDigest: event.EventDigest, GlobalSequence: r.global,
	}
	if r.committed == nil {
		r.committed = map[eventKey]recordedEvent{}
	}
	r.committed[key] = recordedEvent{event: event, ack: ack}
	r.events = append(r.events, event)
	r.acks = append(r.acks, ack)
	return ack, nil
}

func (r *duplicateKeyRecorder) kinds() []contextevent.Kind {
	out := make([]contextevent.Kind, len(r.events))
	for i, event := range r.events {
		out[i] = event.Kind
	}
	return out
}

func (r *duplicateKeyRecorder) assertStrictGlobalOrder() error {
	for i := 1; i < len(r.acks); i++ {
		if r.acks[i].GlobalSequence <= r.acks[i-1].GlobalSequence {
			return fmt.Errorf("acknowledged global order %d then %d does not strictly increase", r.acks[i-1].GlobalSequence, r.acks[i].GlobalSequence)
		}
	}
	return nil
}

// sharedStateWorkspace is the exact verified candidate identity the service
// stamps its own lifecycle events with.
func sharedStateWorkspace(t *testing.T, req ExecutionRequest) WorkspaceFacts {
	t.Helper()
	workspaceID, err := req.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatalf("WorkspaceID: %v", err)
	}
	return WorkspaceFacts{WorkspaceID: workspaceID, CurrentCommit: req.InputCommit, CurrentTree: req.InputTree}
}

func sharedStartPayload(t *testing.T, req ExecutionRequest) *contextevent.AdapterStartPayload {
	t.Helper()
	schema, err := contextevent.PayloadSchema(contextevent.KindAdapterStart)
	if err != nil {
		t.Fatalf("adapter-start schema: %v", err)
	}
	workspaceDigest, err := ExecutionWorkspaceRequestDigest(req.ExecutionWorkspaceRequest)
	if err != nil {
		t.Fatalf("workspace request digest: %v", err)
	}
	return &contextevent.AdapterStartPayload{
		Schema: schema, Adapter: req.Adapter, AdapterVersion: req.AdapterVersion,
		Session: req.Session, ProfileDigest: req.Profile.Digest, WorkspaceRequestDigest: workspaceDigest,
	}
}

func sharedMessagePayload(t *testing.T) *contextevent.ProviderMessagePayload {
	t.Helper()
	schema, err := contextevent.PayloadSchema(contextevent.KindProviderMessage)
	if err != nil {
		t.Fatalf("provider-message schema: %v", err)
	}
	encoded, err := canonjson.Marshal(map[string]string{"family": "assistant/text", "text": "shared state witness"})
	if err != nil {
		t.Fatalf("encode provider-message detail: %v", err)
	}
	raw := bytes.TrimSuffix(encoded, []byte("\n"))
	detail := contextevent.Detail{
		Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON,
		Digest: rawDigest(raw), RedactionProfile: contextevent.RedactionProfileStandard, RedactedJSON: raw,
	}
	return &contextevent.ProviderMessagePayload{
		Schema: schema, MessageID: "msg-shared-state:0", Role: "assistant",
		MessageDigest: detail.Digest, Detail: detail,
	}
}

type mcpFake struct {
	t                   *testing.T
	request             ExecutionRequest
	state               *FlightState
	resolveVerification Verification
	verify              Verification
	kinds               []contextevent.Kind
	events              []contextevent.Event
	order               []string
	appendErrAt         int
	storeErr            error
	installs            int
}

func (f *mcpFake) ResolveContext(_ context.Context, ref string) (ContextResolution, error) {
	v := f.resolveVerification
	if v.State == "" {
		v = proven()
	}
	item := validDataItem(f.t)
	item.Content = "declared bytes"
	item.ContentDigest = rawDigest([]byte(item.Content))
	item.Digest = ""
	encoded, err := contextcompile.EncodeDataItem(item)
	if err != nil {
		return ContextResolution{}, err
	}
	item, err = contextcompile.DecodeDataItem(encoded)
	if err != nil {
		return ContextResolution{}, err
	}
	return ContextResolution{Verification: v, Ref: ref, Data: item}, nil
}
func (f *mcpFake) CompileChild(_ context.Context, request ChildCompileRequest) (ChildManifest, error) {
	childRevision := request.Snapshot.Revision + 1
	return ChildManifest{Verification: proven(), RequestID: request.RequestID, ParentRevision: request.Snapshot.Revision, ParentManifestDigest: request.Snapshot.ManifestDigest, ChildRevision: childRevision, ChildManifestDigest: testDigest(fmt.Sprintf("child-manifest-%d", childRevision)), ExpansionDigest: testDigest(fmt.Sprintf("expansion-%d", childRevision)), ExpansionRoot: testDigest(fmt.Sprintf("expanded-root-%d", childRevision))}, nil
}
func (f *mcpFake) VerifyEpoch(context.Context, EpochCheck) (Verification, error) {
	if f.verify.State == "" {
		return proven(), nil
	}
	return f.verify, nil
}
func (f *mcpFake) Append(_ context.Context, event contextevent.Event) (contextevent.EventAck, error) {
	f.kinds = append(f.kinds, event.Kind)
	f.events = append(f.events, event)
	f.order = append(f.order, "append:"+string(event.Kind))
	if f.appendErrAt > 0 && len(f.kinds) == f.appendErrAt {
		return contextevent.EventAck{}, errors.New("recorder rejected event")
	}
	f.order = append(f.order, "ack:"+string(event.Kind))
	return contextevent.EventAck{Schema: contextevent.AckSchemaID, Flight: event.Flight, Lane: event.Lane, Epoch: event.Epoch, Session: event.Session, ManifestRevision: event.ManifestRevision, Kind: event.Kind, SourceSequence: event.SourceSequence, EventDigest: event.EventDigest, GlobalSequence: uint64(len(f.kinds))}, nil
}
func (f *mcpFake) InstallExpansion(context.Context, ExpansionInstall) error {
	f.installs++
	f.order = append(f.order, "install")
	return f.storeErr
}
func (f *mcpFake) NextStamp(context.Context) (string, error) { return "2026-08-27T12:00:00Z", nil }

func mcpSnapshot(t *testing.T, req ExecutionRequest, expansionRoot string) FlightStateSnapshot {
	t.Helper()
	workspaceID, err := req.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatalf("WorkspaceID: %v", err)
	}
	return FlightStateSnapshot{
		Request: req, Key: executionKey(req), WorkspaceID: workspaceID,
		CandidateCommit: req.InputCommit, CandidateTree: req.InputTree,
		Revision: req.ManifestRevision, ManifestDigest: req.ManifestDigest,
		ProjectionDigest: req.ProjectionDigest, ExpansionRoot: expansionRoot,
		NextSourceSequence: 1,
	}
}
