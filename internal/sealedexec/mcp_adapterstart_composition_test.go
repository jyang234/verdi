package sealedexec

import (
	"context"
	"errors"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/countersign"
)

// countingResolver records when the embedded server actually consults the
// context owner. The adapter-start gate is only a real gate if resolution has
// not happened yet while a call is parked, so the count is what separates
// "waiting before the owner is asked" from "asked and then waiting".
type countingResolver struct {
	delegate ContextResolver
	calls    int
}

func (r *countingResolver) ResolveContext(ctx context.Context, ref string) (ContextResolution, error) {
	r.calls++
	return r.delegate.ResolveContext(ctx, ref)
}

// TestAdapterStartGateComposesWithResolutionDataAndReplayRules is the merge
// witness required by the 2026-09-09 integration of origin/main (adapter-start
// gating of the embedded context server, 10380db8/17cff9a7) into the F12
// lifecycle-gate track (SI-182 restart-reconstructible expansions, SI-194's
// data-presence rule).
//
// The two changes meet inside one function: awaitAdapterStart runs at the head
// of ScopedMCP.requestContext, ahead of the state mutex, and everything SI-182
// and SI-194 govern runs under that mutex afterwards. Neither side's tests can
// see that seam on their own — main's gate tests never reach an approved or a
// denied transition, and the branch's SI-182/SI-194 tests run on standalone
// states whose latch is nil. This test pins the composition itself: the gate
// admits nothing early, and once open it changes neither rule.
func TestAdapterStartGateComposesWithResolutionDataAndReplayRules(t *testing.T) {
	t.Run("a gated non-proven resolution stays denied and data-free, and does not disturb a later replayable expansion", func(t *testing.T) {
		req := serviceRequest(t, ActionStart)
		workspace := sharedStateWorkspace(t, req)
		// newExecutionFlightState is the only constructor that arms the latch,
		// so this is a live sealed execution's state, not a standalone one.
		state := newExecutionFlightState(req, workspace, restartPlan{}, "")
		fake := &mcpFake{t: t, request: req, state: state}
		// SI-194: an honest owner answers a denied ref with no data item at
		// all. The fake already refuses to fabricate one; this selects that
		// answer.
		fake.resolveVerification = violated(FailureOutOfScope, "ref is outside the sealed scope")
		resolver := &countingResolver{delegate: fake}
		server, err := NewScopedMCP(ScopedMCPPorts{
			Resolver: resolver, Compiler: NewCanonicalChildCompiler(), Verifier: fake,
			Recorder: fake, Store: fake, Stamps: fake,
		}, state)
		if err != nil {
			t.Fatalf("NewScopedMCP: %v", err)
		}

		// --- while the gate is shut -------------------------------------
		observed := &doneObservedContext{Context: context.Background(), observed: make(chan struct{})}
		denied := make(chan InspectionResult, 1)
		deniedErr := make(chan error, 1)
		go func() {
			result, err := server.Call(observed, ToolRequestContext, []byte(`{"purpose":"needed for implementation","ref":"spec/denied"}`))
			denied <- result
			deniedErr <- err
		}()
		select {
		case result := <-denied:
			t.Fatalf("request_context returned %#v before adapter start was acknowledged", result)
		case <-observed.observed:
		}
		// The gate precedes resolution, not merely acknowledgment: the owner
		// has not been asked, and nothing has been appended or advanced.
		if resolver.calls != 0 {
			t.Fatalf("resolver consulted %d times while adapter start was unacknowledged, want 0", resolver.calls)
		}
		if len(fake.events) != 0 {
			t.Fatalf("gated call appended %d events, want none", len(fake.events))
		}
		if got := state.Snapshot().NextSourceSequence; got != 1 {
			t.Fatalf("gated call advanced the source sequence to %d, want 1", got)
		}

		// --- open the gate ----------------------------------------------
		if _, err := state.append(context.Background(), fake, fake, workspace, contextevent.KindAdapterStart, sharedStartPayload(t, req)); err != nil {
			t.Fatalf("adapter-start append: %v", err)
		}
		if err := <-deniedErr; err != nil {
			t.Fatalf("request_context after adapter start: %v", err)
		}
		result := <-denied

		// SI-194 under the gate: still a denial, still carrying no data item.
		if result.Kind != InspectionContextDenied {
			t.Fatalf("result kind = %q, want %q", result.Kind, InspectionContextDenied)
		}
		if result.Context.Data != (contextcompile.DataItem{}) {
			t.Fatalf("denied inspection carries a data item %#v, want the zero value", result.Context.Data)
		}
		if resolver.calls != 1 {
			t.Fatalf("resolver consulted %d times, want exactly 1 after the gate opened", resolver.calls)
		}
		if len(fake.installed) != 0 {
			t.Fatalf("a denied resolution installed %d expansions, want none", len(fake.installed))
		}
		denialState := state.Snapshot()
		if denialState.Revision != 0 || denialState.ExpansionRoot != "" {
			t.Fatalf("denial advanced the manifest to revision %d / root %q, want 0 / empty", denialState.Revision, denialState.ExpansionRoot)
		}
		if denialState.Invalidated {
			t.Fatal("an honest denial invalidated the epoch")
		}
		// The recorded decision is a no-transition one: equal parent and child
		// manifest digests rather than an invented child.
		decision := lastDecisionPayload(t, fake)
		if decision.Verdict != countersign.VerdictViolated || decision.ReasonCode != "context-denied" {
			t.Fatalf("decision = %q/%q, want violated/context-denied", decision.Verdict, decision.ReasonCode)
		}
		if decision.ChildManifestDigest != decision.ParentManifestDigest {
			t.Fatalf("denial recorded a child digest %q distinct from the parent %q", decision.ChildManifestDigest, decision.ParentManifestDigest)
		}

		// --- a proven expansion, still under the same open gate ----------
		// The parent this replays from is the state the DENIAL left behind, so
		// a green replay here is exactly the property the composition needs:
		// a gated, data-item-free denial does not cost the next expansion its
		// restart-reconstructibility.
		fake.resolveVerification = proven()
		parent := state.Snapshot()
		approved, err := server.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"needed for implementation","ref":"spec/extra"}`))
		if err != nil {
			t.Fatalf("request_context (proven) after a gated denial: %v", err)
		}
		if approved.Kind != InspectionContextApproved {
			t.Fatalf("result kind = %q, want %q", approved.Kind, InspectionContextApproved)
		}
		if len(fake.installed) != 1 {
			t.Fatalf("installed %d expansions, want exactly 1", len(fake.installed))
		}
		install := fake.installed[0]

		// SI-182: the durable row carries the approved facts...
		if install.Ref != "spec/extra" || install.Purpose != "needed for implementation" {
			t.Fatalf("install ref/purpose = %q/%q, want the approved pair", install.Ref, install.Purpose)
		}
		if install.Data == (contextcompile.DataItem{}) {
			t.Fatalf("a proven install carries no data item")
		}
		// ...and those facts plus the post-denial parent state replay every
		// identity the row recorded.
		replay := InstalledExpansionInput{
			Key: parent.Key, ParentRevision: parent.Revision, ParentManifestDigest: parent.ManifestDigest,
			Ref: install.Ref, Purpose: install.Purpose, Item: install.Data,
			PriorExpansionRoot: parent.ExpansionRoot,
		}
		proof, err := ProveInstalledExpansion(replay)
		if err != nil {
			t.Fatalf("ProveInstalledExpansion: %v", err)
		}
		if install.RequestID != proof.RequestID || install.ChildManifestDigest != proof.ChildManifestDigest ||
			install.ExpansionDigest != proof.ExpansionDigest || install.ExpansionRoot != proof.ExpansionRoot {
			t.Fatalf("install %#v does not replay to %#v", install, proof)
		}
		// A row rewritten after the fact still cannot replay under the gate.
		rewritten := replay
		rewritten.Ref = "spec/other"
		if mutated, err := ProveInstalledExpansion(rewritten); err != nil {
			t.Fatalf("ProveInstalledExpansion(rewritten ref): %v", err)
		} else if mutated.ExpansionRoot == install.ExpansionRoot {
			t.Fatal("a rewritten ref replayed to the recorded expansion root")
		}
	})

	t.Run("a failed adapter start refuses an otherwise proven expansion", func(t *testing.T) {
		req := serviceRequest(t, ActionStart)
		workspace := sharedStateWorkspace(t, req)
		state := newExecutionFlightState(req, workspace, restartPlan{}, "")
		fake := &mcpFake{t: t, request: req, state: state}
		server, err := NewScopedMCP(ScopedMCPPorts{
			Resolver: fake, Compiler: NewCanonicalChildCompiler(), Verifier: fake,
			Recorder: fake, Store: fake, Stamps: fake,
		}, state)
		if err != nil {
			t.Fatalf("NewScopedMCP: %v", err)
		}
		// The resolution would be proven and the expansion would install; the
		// gate is the only thing standing in the way, so this isolates it.
		state.failAdapterStart(errors.New("provider exited before init"))
		if _, err := server.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"needed for implementation","ref":"spec/extra"}`)); !errors.Is(err, ErrOperational) {
			t.Fatalf("request_context error = %v, want an operational refusal", err)
		}
		if len(fake.installed) != 0 || len(fake.events) != 0 {
			t.Fatalf("a failed adapter start installed %d expansions and appended %d events, want none", len(fake.installed), len(fake.events))
		}
	})

	t.Run("a standalone context state keeps the ungated branch semantics", func(t *testing.T) {
		// The embedded gate must not have become a precondition of the
		// standalone scoped-context surface: NewFlightState leaves the latch
		// nil, and awaitAdapterStart is then a no-op, so SI-182 and SI-194
		// hold there exactly as they did before the merge.
		req := serviceRequest(t, ActionStart)
		state := NewFlightState(mcpSnapshot(t, req, ""))
		if state.adapterStart != nil {
			t.Fatal("a standalone context state armed an adapter-start latch")
		}
		fake := &mcpFake{t: t, request: req, state: state}
		fake.resolveVerification = unproven(FailureUnavailable, "ref is not resolvable here")
		server, err := NewScopedMCP(ScopedMCPPorts{
			Resolver: fake, Compiler: NewCanonicalChildCompiler(), Verifier: fake,
			Recorder: fake, Store: fake, Stamps: fake,
		}, state)
		if err != nil {
			t.Fatalf("NewScopedMCP: %v", err)
		}
		result, err := server.Call(context.Background(), ToolRequestContext, []byte(`{"purpose":"needed for implementation","ref":"spec/absent"}`))
		if err != nil {
			t.Fatalf("request_context on a standalone state: %v", err)
		}
		if result.Kind != InspectionContextDenied || result.Context.Data != (contextcompile.DataItem{}) {
			t.Fatalf("standalone result = %#v, want a denial with no data item", result)
		}
		if len(fake.installed) != 0 {
			t.Fatalf("standalone denial installed %d expansions, want none", len(fake.installed))
		}
	})
}

// lastDecisionPayload returns the context decision the recorder saw last,
// decoded from the durable envelope rather than from a value the test kept.
func lastDecisionPayload(t *testing.T, fake *mcpFake) *contextevent.ContextDecisionPayload {
	t.Helper()
	for i := len(fake.events) - 1; i >= 0; i-- {
		if fake.events[i].Kind != contextevent.KindContextDecision {
			continue
		}
		payload, ok := fake.events[i].Payload.(*contextevent.ContextDecisionPayload)
		if !ok {
			t.Fatalf("context-decision payload = %T, want *contextevent.ContextDecisionPayload", fake.events[i].Payload)
		}
		return payload
	}
	t.Fatal("no context-decision event was recorded")
	return nil
}
