package sealedexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

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
