package contextevent

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

const (
	digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	digestC = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	shaA    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	shaB    = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestContextEventEnvelopeContract_Static(t *testing.T) {
	t.Parallel()

	event := eventFixture(t, KindPrompt, AdapterCodex)
	encoded, err := EncodeEvent(event)
	if err != nil {
		t.Fatalf("EncodeEvent() error = %v", err)
	}
	decoded, err := DecodeEvent(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeEvent() error = %v", err)
	}
	if decoded.EventDigest == "" || decoded.SourceSequence != 1 || decoded.PriorRevision != nil {
		t.Fatalf("decoded event lost identity: %#v", decoded)
	}
	reencoded, err := EncodeEvent(decoded)
	if err != nil {
		t.Fatalf("EncodeEvent(decoded) error = %v", err)
	}
	if !bytes.Equal(encoded, reencoded) {
		t.Fatalf("event round trip changed bytes\nfirst: %s\nagain: %s", encoded, reencoded)
	}

	continued := eventFixture(t, KindToolCall, AdapterCodex)
	continued.SourceSequence = 2
	continued.PriorEventDigest = decoded.EventDigest
	if _, err := EncodeEvent(continued); err != nil {
		t.Fatalf("EncodeEvent(continued) error = %v", err)
	}

	bridge := eventFixture(t, KindPrompt, AdapterCodex)
	bridge.ManifestRevision = 1
	bridge.ManifestDigest = digestB
	bridge.PriorRevision = &PriorRevision{
		ManifestRevision:       0,
		ManifestDigest:         digestA,
		EventRoot:              digestC,
		TerminalSourceSequence: 9,
		TerminalGlobalSequence: 19,
	}
	if _, err := EncodeEvent(bridge); err != nil {
		t.Fatalf("EncodeEvent(bridge) error = %v", err)
	}

	revisions := []Revision{
		{Schema: RevisionSchemaID, ManifestRevision: 0, ManifestDigest: digestA, FirstGlobalSequence: 11, TerminalGlobalSequence: 19, TerminalSourceSequence: 9, TerminalKind: KindChildManifest, EventRoot: digestB},
		{Schema: RevisionSchemaID, ManifestRevision: 1, ManifestDigest: digestB, FirstGlobalSequence: 20, TerminalGlobalSequence: 24, TerminalSourceSequence: 5, TerminalKind: KindExecutionResult, EventRoot: digestC},
	}
	root, err := EventChainRoot(revisions)
	if err != nil {
		t.Fatalf("EventChainRoot() error = %v", err)
	}
	if !strings.HasPrefix(root, "sha256:") {
		t.Fatalf("EventChainRoot() = %q, want canonical digest", root)
	}
	interleaved := append([]Revision(nil), revisions...)
	interleaved[1].FirstGlobalSequence = 23
	interleaved[1].TerminalGlobalSequence = 24
	if _, err := EventChainRoot(interleaved); err != nil {
		t.Fatalf("EventChainRoot(cross-flight interleaving gap) error = %v", err)
	}
	overlapped := append([]Revision(nil), revisions...)
	overlapped[1].FirstGlobalSequence = revisions[0].TerminalGlobalSequence
	if _, err := EventChainRoot(overlapped); err == nil {
		t.Fatal("EventChainRoot(overlapping cross-revision global sequence) error = nil")
	}
	if _, err := EventChainRoot([]Revision{revisions[0]}); err == nil {
		t.Fatal("EventChainRoot(non-final child-manifest) error = nil")
	}

	ack := EventAck{Schema: AckSchemaID, Flight: "flight-1", Lane: "builder", Epoch: "epoch-1", Session: "session-1", ManifestRevision: 0, Kind: KindPrompt, SourceSequence: 1, EventDigest: decoded.EventDigest, GlobalSequence: 11}
	ackBytes, err := EncodeEventAck(ack)
	if err != nil {
		t.Fatalf("EncodeEventAck() error = %v", err)
	}
	if _, err := DecodeEventAck(bytes.NewReader(ackBytes)); err != nil {
		t.Fatalf("DecodeEventAck() error = %v", err)
	}
	if _, err := EncodeEventAck(EventAck{}); err == nil {
		t.Fatal("EncodeEventAck(zero) error = nil")
	}

	receiptAck := ReceiptEventAck{Schema: ReceiptAckSchemaID, Flight: "flight-1", Lane: "builder", Epoch: "epoch-1", Session: "session-1", ManifestRevision: 1, Kind: KindReceipt, SourceSequence: 6, EventDigest: digestA, GlobalSequence: 25, ReceiptDigest: digestB}
	receiptAckBytes, err := EncodeReceiptEventAck(receiptAck)
	if err != nil {
		t.Fatalf("EncodeReceiptEventAck() error = %v", err)
	}
	if _, err := DecodeReceiptEventAck(bytes.NewReader(receiptAckBytes)); err != nil {
		t.Fatalf("DecodeReceiptEventAck() error = %v", err)
	}
	receiptAck.Kind = KindPrompt
	if _, err := EncodeReceiptEventAck(receiptAck); err == nil {
		t.Fatal("EncodeReceiptEventAck(non-receipt) error = nil")
	}
}

func TestContextEventRegistryContract_Static(t *testing.T) {
	t.Parallel()

	wantKinds := []Kind{
		KindFlightPlan, KindInstructionProjection, KindChildManifest, KindPrompt,
		KindProviderMessage, KindProviderSummary, KindToolCall, KindToolResult,
		KindRead, KindWrite, KindEditDenied, KindContextRequest, KindContextDecision,
		KindClaimRequest, KindClaimDecision, KindClaimWait, KindClaimRelease,
		KindCommand, KindTest, KindResource, KindTimeout, KindGitStatus, KindGitDiff,
		KindGitCommit, KindForgeChange, KindGateInput, KindGateVerdict, KindWitness,
		KindFlightPlanDeviation, KindAdjudication, KindExecutionResult, KindReceipt,
		KindRetry, KindResume, KindSuspension, KindTelemetryGap, KindAdapterStart,
		KindAdapterStop, KindAdapterError,
	}
	if len(wantKinds) != 39 {
		t.Fatalf("test inventory count = %d, want 39", len(wantKinds))
	}
	for _, kind := range wantKinds {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			got, err := PayloadSchema(kind)
			if err != nil {
				t.Fatalf("PayloadSchema(%q) error = %v", kind, err)
			}
			want := "verdi.context-event-payload/" + string(kind) + "/v1"
			if got != want {
				t.Fatalf("PayloadSchema(%q) = %q, want %q", kind, got, want)
			}
		})
	}
	if _, err := PayloadSchema(Kind("hidden-reasoning")); err == nil {
		t.Fatal("PayloadSchema(unknown) error = nil")
	}

	inline := inlineDetail(t)
	if err := inline.Validate(); err != nil {
		t.Fatalf("inline Detail.Validate() error = %v", err)
	}
	lfDigest, err := canonjson.Digest(map[string]any{"message": "redacted"})
	if err != nil {
		t.Fatal(err)
	}
	lfDetail := inline
	lfDetail.Digest = lfDigest
	if err := lfDetail.Validate(); err == nil {
		t.Fatal("inline Detail.Validate() accepted digest over redacted_json plus framing LF")
	}
	segment := Detail{Mode: DetailSegment, MediaType: MediaTypeJSON, Digest: digestA, RedactionProfile: RedactionProfileStandard, ByteCount: 123, Reference: "vatc-segment:flight-1/0001"}
	if err := segment.Validate(); err != nil {
		t.Fatalf("segment Detail.Validate() error = %v", err)
	}
	badDetails := []Detail{
		{},
		{Mode: DetailInline, MediaType: "text/plain", Digest: inline.Digest, RedactionProfile: RedactionProfileStandard, RedactedJSON: inline.RedactedJSON},
		{Mode: DetailInline, MediaType: MediaTypeJSON, Digest: digestA, RedactionProfile: "custom", RedactedJSON: inline.RedactedJSON},
		{Mode: DetailInline, MediaType: MediaTypeJSON, Digest: inline.Digest, RedactionProfile: RedactionProfileStandard, RedactedJSON: inline.RedactedJSON, Reference: "forbidden"},
		{Mode: DetailSegment, MediaType: MediaTypeJSON, Digest: digestA, RedactionProfile: RedactionProfileStandard, Reference: "missing-count"},
		{Mode: DetailSegment, MediaType: MediaTypeJSON, Digest: digestA, RedactionProfile: RedactionProfileStandard, ByteCount: 1, Reference: "bad\nreference"},
	}
	for i, detail := range badDetails {
		if err := detail.Validate(); err == nil {
			t.Errorf("bad Detail[%d].Validate() error = nil", i)
		}
	}

	t.Run("receipt self digest and represented detail digest are distinct domains", func(t *testing.T) {
		event := eventFixture(t, KindReceipt, AdapterCodex)
		payload := event.Payload.(*ReceiptPayload)
		payload.ReceiptDigest = digestA
		if payload.Detail.Digest == payload.ReceiptDigest {
			t.Fatal("fixture did not produce distinct receipt and detail digests")
		}
		if _, err := EncodeEvent(event); err != nil {
			t.Fatalf("EncodeEvent(distinct valid digest domains) error = %v", err)
		}

		payload.Detail.Digest = digestB
		if _, err := EncodeEvent(event); err == nil {
			t.Fatal("EncodeEvent(malformed represented-byte digest) error = nil")
		}
	})

	t.Run("explicitly ratified enums are closed", func(t *testing.T) {
		tests := []struct {
			name   string
			kind   Kind
			mutate func(any)
		}{
			{"provider summary authority", KindProviderSummary, func(payload any) { payload.(*ProviderSummaryPayload).Authority = Authority("operator") }},
			{"context verdict", KindContextDecision, func(payload any) { payload.(*ContextDecisionPayload).Verdict = countersign.Verdict("pass") }},
			{"receipt role", KindReceipt, func(payload any) { payload.(*ReceiptPayload).Role = Role("operator") }},
			{"telemetry availability", KindTelemetryGap, func(payload any) { payload.(*TelemetryGapPayload).Availability = "partial" }},
		}
		for _, tt := range tests {
			event := eventFixture(t, tt.kind, AdapterCodex)
			tt.mutate(event.Payload)
			if _, err := EncodeEvent(event); err == nil {
				t.Errorf("EncodeEvent(%s unknown enum) error = nil", tt.name)
			}
		}
	})

	t.Run("unratified scalar vocabularies remain required recorded text", func(t *testing.T) {
		tests := []struct {
			name string
			kind Kind
			set  func(any, string)
		}{
			{"provider role", KindProviderMessage, func(payload any, value string) { payload.(*ProviderMessagePayload).Role = value }},
			{"tool status", KindToolResult, func(payload any, value string) { payload.(*ToolResultPayload).Status = value }},
			{"read classification", KindRead, func(payload any, value string) { payload.(*ReadPayload).Classification = value }},
			{"read decision", KindRead, func(payload any, value string) { payload.(*ReadPayload).Decision = value }},
			{"resource availability", KindResource, func(payload any, value string) { payload.(*ResourcePayload).Availability = value }},
			{"forge operation", KindForgeChange, func(payload any, value string) { payload.(*ForgeChangePayload).Operation = value }},
			{"adjudication decision", KindAdjudication, func(payload any, value string) { payload.(*AdjudicationPayload).Decision = value }},
		}
		for _, tt := range tests {
			event := eventFixture(t, tt.kind, AdapterCodex)
			tt.set(event.Payload, "recorded-value-without-invented-semantics")
			if _, err := EncodeEvent(event); err != nil {
				t.Errorf("EncodeEvent(%s nonempty data) error = %v", tt.name, err)
			}
			tt.set(event.Payload, "")
			if _, err := EncodeEvent(event); err == nil {
				t.Errorf("EncodeEvent(%s empty data) error = nil", tt.name)
			}
		}
	})

	t.Run("payload construction is registered pointer only", func(t *testing.T) {
		event := eventFixture(t, KindPrompt, AdapterCodex)
		event.Payload = *event.Payload.(*PromptPayload)
		if _, err := EncodeEvent(event); err == nil {
			t.Fatal("EncodeEvent(value payload) error = nil")
		}
	})
}

func TestContextEventRegistryContract_Behavioral(t *testing.T) {
	t.Parallel()

	for _, adapter := range []Adapter{AdapterCodex, AdapterClaude} {
		adapter := adapter
		for _, kind := range allFixtureKinds() {
			kind := kind
			t.Run(string(adapter)+"/"+string(kind), func(t *testing.T) {
				t.Parallel()
				event := eventFixture(t, kind, adapter)
				encoded, err := EncodeEvent(event)
				if err != nil {
					t.Fatalf("EncodeEvent(%q) error = %v", kind, err)
				}
				decoded, err := DecodeEvent(bytes.NewReader(encoded))
				if err != nil {
					t.Fatalf("DecodeEvent(%q) error = %v", kind, err)
				}
				if decoded.Kind != kind || decoded.Adapter != adapter || decoded.PayloadSchema != "verdi.context-event-payload/"+string(kind)+"/v1" {
					t.Fatalf("decoded registry mismatch: %#v", decoded)
				}

				var top map[string]json.RawMessage
				if err := json.Unmarshal(encoded, &top); err != nil {
					t.Fatal(err)
				}
				var payload map[string]json.RawMessage
				if err := json.Unmarshal(top["payload"], &payload); err != nil {
					t.Fatal(err)
				}
				payload["forbidden_detail_routing"] = json.RawMessage(`true`)
				top["payload"], err = canonjson.Marshal(payload)
				if err != nil {
					t.Fatal(err)
				}
				unknownFieldBytes, err := canonjson.Marshal(top)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := DecodeEvent(bytes.NewReader(unknownFieldBytes)); err == nil {
					t.Fatalf("DecodeEvent(%q payload unknown field) error = nil", kind)
				}
			})
		}
	}

	summary := payloadFixture(t, KindProviderSummary).(*ProviderSummaryPayload)
	if summary.Authority != AuthorityAdvisory {
		t.Fatalf("provider-summary authority = %q, want advisory", summary.Authority)
	}
	gap := payloadFixture(t, KindTelemetryGap).(*TelemetryGapPayload)
	if gap.Availability != "unavailable" || gap.FromSequence == 0 || gap.ToSequence < gap.FromSequence {
		t.Fatalf("telemetry-gap does not represent missing observable telemetry: %#v", gap)
	}
}

func TestContextEventCodecRejectsInvalidBoundaries(t *testing.T) {
	t.Parallel()

	base := eventFixture(t, KindPrompt, AdapterCodex)
	tests := []struct {
		name   string
		mutate func(*Event)
	}{
		{"unknown adapter", func(e *Event) { e.Adapter = Adapter("other") }},
		{"sequence zero", func(e *Event) { e.SourceSequence = 0 }},
		{"first has predecessor", func(e *Event) { e.PriorEventDigest = digestA }},
		{"later lacks predecessor", func(e *Event) { e.SourceSequence = 2 }},
		{"revision zero has bridge", func(e *Event) {
			e.PriorRevision = &PriorRevision{ManifestDigest: digestA, EventRoot: digestB, TerminalSourceSequence: 1, TerminalGlobalSequence: 1}
		}},
		{"revision one lacks bridge", func(e *Event) { e.ManifestRevision = 1; e.ManifestDigest = digestB }},
		{"payload schema mismatch", func(e *Event) { e.PayloadSchema = "verdi.context-event-payload/read/v1" }},
		{"typed payload mismatch", func(e *Event) { e.Kind = KindRead }},
		{"invalid occurrence stamp", func(e *Event) { e.OccurredAt = "2026-08-27T12:00:00-04:00" }},
		{"mismatched self digest", func(e *Event) { e.EventDigest = digestA }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := base
			tt.mutate(&event)
			if _, err := EncodeEvent(event); err == nil {
				t.Fatal("EncodeEvent() error = nil")
			}
		})
	}

	encoded, err := EncodeEvent(base)
	if err != nil {
		t.Fatal(err)
	}
	badDocuments := [][]byte{
		append(append([]byte(nil), encoded...), []byte("{}")...),
		bytes.Replace(encoded, []byte(`"flight":"flight-1"`), []byte(`"unknown":true,"flight":"flight-1"`), 1),
		bytes.Replace(encoded, []byte(`"flight":"flight-1"`), []byte(`"flight":"flight-1","flight":"flight-1"`), 1),
		bytes.Replace(encoded, []byte(`{"adapter"`), []byte("{ \"adapter\""), 1),
	}
	for i, raw := range badDocuments {
		if _, err := DecodeEvent(bytes.NewReader(raw)); err == nil {
			t.Errorf("DecodeEvent(bad document %d) error = nil", i)
		}
	}
	if _, err := DecodeEvent(errorReader{}); err == nil {
		t.Fatal("DecodeEvent(read error) error = nil")
	}
	if _, err := DecodeEventAck(bytes.NewReader([]byte(`{"schema":"verdi.context-event-ack/v1"}`))); err == nil {
		t.Fatal("DecodeEventAck(incomplete) error = nil")
	}
	if _, err := DecodeReceiptEventAck(bytes.NewReader([]byte(`null`))); err == nil {
		t.Fatal("DecodeReceiptEventAck(null) error = nil")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, bytes.ErrTooLarge }

func allFixtureKinds() []Kind {
	return []Kind{
		KindFlightPlan, KindInstructionProjection, KindChildManifest, KindPrompt,
		KindProviderMessage, KindProviderSummary, KindToolCall, KindToolResult,
		KindRead, KindWrite, KindEditDenied, KindContextRequest, KindContextDecision,
		KindClaimRequest, KindClaimDecision, KindClaimWait, KindClaimRelease,
		KindCommand, KindTest, KindResource, KindTimeout, KindGitStatus, KindGitDiff,
		KindGitCommit, KindForgeChange, KindGateInput, KindGateVerdict, KindWitness,
		KindFlightPlanDeviation, KindAdjudication, KindExecutionResult, KindReceipt,
		KindRetry, KindResume, KindSuspension, KindTelemetryGap, KindAdapterStart,
		KindAdapterStop, KindAdapterError,
	}
}

func eventFixture(t *testing.T, kind Kind, adapter Adapter) Event {
	t.Helper()
	payload := payloadFixture(t, kind)
	switch lifecycle := payload.(type) {
	case *AdapterStartPayload:
		lifecycle.Adapter = adapter
	case *AdapterStopPayload:
		lifecycle.Adapter = adapter
	case *AdapterErrorPayload:
		lifecycle.Adapter = adapter
	}
	return Event{
		Schema:               EventSchemaID,
		SourceSequence:       1,
		Flight:               "flight-1",
		Lane:                 "builder",
		Epoch:                "epoch-1",
		ManifestRevision:     0,
		ManifestDigest:       digestA,
		Session:              "session-1",
		ATCRunway:            "/runway/flight-1",
		ExecutionWorkspaceID: "workspace-1",
		CandidateCommit:      shaA,
		CandidateTree:        shaB,
		Adapter:              adapter,
		AdapterVersion:       "1.2.3",
		OccurredAt:           "2026-08-27T16:00:00.123456789Z",
		Kind:                 kind,
		PayloadSchema:        "verdi.context-event-payload/" + string(kind) + "/v1",
		Payload:              payload,
	}
}

func inlineDetail(t *testing.T) Detail {
	t.Helper()
	raw := json.RawMessage(`{"message":"redacted"}`)
	digest := sha256.Sum256(raw)
	digestText := fmt.Sprintf("sha256:%x", digest)
	return Detail{Mode: DetailInline, MediaType: MediaTypeJSON, Digest: digestText, RedactionProfile: RedactionProfileStandard, RedactedJSON: raw}
}

func principalResolutionFixture(t *testing.T) governanceprincipal.PrincipalResolution {
	t.Helper()
	claim := governanceprincipal.PrincipalClaim{TrustSource: "ci-runner", Subject: "runner@example.com"}
	id, err := governanceprincipal.CanonicalPrincipalID(claim.TrustSource, claim.Subject)
	if err != nil {
		t.Fatal(err)
	}
	return governanceprincipal.PrincipalResolution{
		Claim: claim, PrincipalID: id, State: governanceprincipal.ResolutionAuthenticated,
		Witnesses: []governanceprincipal.Witness{{Code: "trust-subject-verified", SourceID: "ci-runner", EvidenceDigest: digestA}},
	}
}

func payloadFixture(t *testing.T, kind Kind) any {
	t.Helper()
	schema := "verdi.context-event-payload/" + string(kind) + "/v1"
	detail := inlineDetail(t)
	witnesses := []string{"witness:a", "witness:b"}
	switch kind {
	case KindFlightPlan:
		return &FlightPlanPayload{Schema: schema, ManifestDigest: digestA, ProjectionDigest: digestB, DispatchDigest: digestC, Detail: detail}
	case KindInstructionProjection:
		return &InstructionProjectionPayload{Schema: schema, ManifestDigest: digestA, ProjectionDigest: digestB, Detail: detail}
	case KindChildManifest:
		return &ChildManifestPayload{Schema: schema, RequestID: "request-1", ParentRevision: 0, ParentManifestDigest: digestA, ChildRevision: 1, ChildManifestDigest: digestB, ExpansionDigest: digestC}
	case KindPrompt:
		return &PromptPayload{Schema: schema, PromptDigest: digestA, Detail: detail}
	case KindProviderMessage:
		return &ProviderMessagePayload{Schema: schema, MessageID: "message-1", Role: "assistant", MessageDigest: digestA, Detail: detail}
	case KindProviderSummary:
		return &ProviderSummaryPayload{Schema: schema, SummaryID: "summary-1", SummaryDigest: digestA, Authority: AuthorityAdvisory, Detail: detail}
	case KindToolCall:
		return &ToolCallPayload{Schema: schema, CallID: "call-1", ToolName: "exec_command", ArgumentsDigest: digestA, Detail: detail}
	case KindToolResult:
		return &ToolResultPayload{Schema: schema, CallID: "call-1", ToolName: "exec_command", Status: "completed", OutputDigest: digestA, Detail: detail}
	case KindRead:
		return &ReadPayload{Schema: schema, Resource: "internal/example.go", Classification: "project-data", Decision: "allowed", ContentDigest: digestA, Detail: detail}
	case KindWrite:
		return &WritePayload{Schema: schema, Path: "internal/example.go", ClaimID: "claim-1", BeforeDigest: digestA, AfterDigest: digestB, ByteCount: 42}
	case KindEditDenied:
		return &EditDeniedPayload{Schema: schema, Operation: "write", Path: "docs/design/specs/00.md", ReasonCode: "protected-path", Witnesses: witnesses}
	case KindContextRequest:
		return &ContextRequestPayload{Schema: schema, RequestID: "request-1", Ref: "spec/example", Purpose: "validate contract"}
	case KindContextDecision:
		return &ContextDecisionPayload{Schema: schema, RequestID: "request-1", Verdict: countersign.VerdictProven, ReasonCode: "in-scope", ParentManifestDigest: digestA, ChildManifestDigest: digestB, Witnesses: witnesses}
	case KindClaimRequest:
		return &ClaimRequestPayload{Schema: schema, ClaimID: "claim-1", Paths: []string{"internal/a.go"}, SharedResources: []string{"go.mod"}}
	case KindClaimDecision:
		return &ClaimDecisionPayload{Schema: schema, ClaimID: "claim-1", Verdict: countersign.VerdictProven, ReasonCode: "available", Witnesses: witnesses}
	case KindClaimWait:
		return &ClaimWaitPayload{Schema: schema, ClaimID: "claim-1", QueuePosition: 1}
	case KindClaimRelease:
		return &ClaimReleasePayload{Schema: schema, ClaimID: "claim-1", Paths: []string{"internal/a.go"}, SharedResources: []string{"go.mod"}}
	case KindCommand:
		return &CommandPayload{Schema: schema, CommandID: "command-1", Argv: []string{"go", "test", "./..."}, WorkingDirectory: "/workspace", DeclaredEnvironmentNames: []string{"VERDI_E2E_PORT_BASE"}, TimeoutMilliseconds: 60000}
	case KindTest:
		return &TestPayload{Schema: schema, CommandID: "command-1", Suite: "unit", ExitCode: 0, DurationMilliseconds: 1234, Verdict: countersign.VerdictProven, OutputDigest: digestA, Detail: detail}
	case KindResource:
		return &ResourcePayload{Schema: schema, OperationID: "command-1", CPUMilliseconds: 100, PeakRSSBytes: 2048, ReadBytes: 10, WriteBytes: 20, Availability: "available"}
	case KindTimeout:
		return &TimeoutPayload{Schema: schema, OperationID: "command-1", TimeoutMilliseconds: 60000, ReasonCode: "deadline-exceeded"}
	case KindGitStatus:
		return &GitStatusPayload{Schema: schema, Head: shaA, Tree: shaB, Branch: "feature/u4a", Clean: true, EntriesDigest: digestA, Detail: detail}
	case KindGitDiff:
		return &GitDiffPayload{Schema: schema, BaseCommit: shaA, TargetCommit: shaB, DiffDigest: digestA, Detail: detail}
	case KindGitCommit:
		return &GitCommitPayload{Schema: schema, Commit: shaB, Tree: shaA, Parents: []string{shaA}, MessageDigest: digestA}
	case KindForgeChange:
		return &ForgeChangePayload{Schema: schema, Forge: "github", Repository: "OWNER/verdi", ChangeID: "123", Operation: "opened", SubjectRef: "refs/heads/feature/u4a", CandidateSHA: shaA, PrincipalResolution: principalResolutionFixture(t)}
	case KindGateInput:
		return &GateInputPayload{Schema: schema, Gate: "verify", Subject: shaA, InputDigests: []string{digestA, digestB}}
	case KindGateVerdict:
		return &GateVerdictPayload{Schema: schema, Gate: "verify", Subject: shaA, Verdict: countersign.VerdictProven, Witnesses: witnesses}
	case KindWitness:
		return &WitnessPayload{Schema: schema, WitnessKind: "test-output", WitnessDigest: digestA, Authority: AuthorityAuthoritative, Detail: detail}
	case KindFlightPlanDeviation:
		return &FlightPlanDeviationPayload{Schema: schema, DeviationID: "deviation-1", PlanDigest: digestA, RuleID: "rule-1", Operation: "read", ObservedDigest: digestB, Verdict: countersign.VerdictViolated, Witnesses: witnesses, Detail: detail}
	case KindAdjudication:
		return &AdjudicationPayload{Schema: schema, FindingOrDeviationID: "deviation-1", PrincipalResolution: principalResolutionFixture(t), Decision: "reject", ReasonDigest: digestA, Detail: detail}
	case KindExecutionResult:
		return &ExecutionResultPayload{Schema: schema, Authority: AuthorityAuthoritative, InputCommit: shaA, OutputCommit: shaB, OutputTree: shaA, Clean: true, ManifestDigest: digestA, ResultFactsDigest: digestB}
	case KindReceipt:
		return &ReceiptPayload{Schema: schema, Role: RoleBuilder, ReceiptDigest: detail.Digest, Authority: AuthorityAuthoritative, ExecutionEventChainRoot: digestB, Detail: detail}
	case KindRetry:
		return &RetryPayload{Schema: schema, ReasonCode: "transient-adapter-error", PriorSession: "session-1", NextSession: "session-2", ContinuityDigest: digestA}
	case KindResume:
		return &ResumePayload{Schema: schema, ContinuityDigest: digestA, PriorSession: "session-1", CurrentSession: "session-2", ManifestDigest: digestB, EventChainRoot: digestC}
	case KindSuspension:
		return &SuspensionPayload{Schema: schema, ReasonCode: "owner-decision", ContinuityDigest: digestA, EventChainRoot: digestB}
	case KindTelemetryGap:
		return &TelemetryGapPayload{Schema: schema, Source: "provider-stream", FromSequence: 4, ToSequence: 6, ReasonCode: "unavailable", Availability: "unavailable"}
	case KindAdapterStart:
		return &AdapterStartPayload{Schema: schema, Adapter: AdapterCodex, AdapterVersion: "1.2.3", Session: "session-1", ProfileDigest: digestA, WorkspaceRequestDigest: digestB}
	case KindAdapterStop:
		return &AdapterStopPayload{Schema: schema, Adapter: AdapterCodex, AdapterVersion: "1.2.3", Session: "session-1", ExitCode: 0, ReasonCode: "completed"}
	case KindAdapterError:
		return &AdapterErrorPayload{Schema: schema, Adapter: AdapterCodex, AdapterVersion: "1.2.3", Session: "session-1", Operation: "stream", ReasonCode: "decode-failed", ErrorDigest: digestA, Detail: detail}
	default:
		t.Fatalf("payloadFixture: unknown kind %q", kind)
		return nil
	}
}
