package contextevent

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/countersign"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

var digestRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type eventWire struct {
	Schema               string          `json:"schema"`
	SourceSequence       uint64          `json:"source_sequence"`
	Flight               string          `json:"flight"`
	Lane                 string          `json:"lane"`
	Epoch                string          `json:"epoch"`
	ManifestRevision     uint64          `json:"manifest_revision"`
	ManifestDigest       string          `json:"manifest_digest"`
	Session              string          `json:"session"`
	ATCRunway            string          `json:"atc_runway"`
	ExecutionWorkspaceID string          `json:"execution_workspace_id"`
	CandidateCommit      string          `json:"candidate_commit"`
	CandidateTree        string          `json:"candidate_tree"`
	Adapter              Adapter         `json:"adapter"`
	AdapterVersion       string          `json:"adapter_version"`
	OccurredAt           string          `json:"occurred_at"`
	Kind                 Kind            `json:"kind"`
	PayloadSchema        string          `json:"payload_schema"`
	Payload              json.RawMessage `json:"payload"`
	PriorEventDigest     string          `json:"prior_event_digest"`
	PriorRevision        *PriorRevision  `json:"prior_revision,omitempty"`
	EventDigest          string          `json:"event_digest"`
}

// EncodeEvent validates, self-digests, and canonically encodes event. A blank
// event_digest is populated in the encoded value; a nonblank mismatch fails.
func EncodeEvent(event Event) ([]byte, error) {
	if err := validateEvent(event, false); err != nil {
		return nil, err
	}
	want, err := eventDigest(event)
	if err != nil {
		return nil, err
	}
	if event.EventDigest != "" && event.EventDigest != want {
		return nil, fmt.Errorf("contextevent: event_digest does not match canonical event")
	}
	event.EventDigest = want
	return canonjson.Marshal(event)
}

// DecodeEvent strictly decodes, validates, digest-checks, and requires the
// input bytes to already be canonical.
func DecodeEvent(reader io.Reader) (Event, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return Event{}, fmt.Errorf("contextevent: read event: %w", err)
	}
	var wire eventWire
	if err := decodeOne(raw, &wire); err != nil {
		return Event{}, fmt.Errorf("contextevent: decode event: %w", err)
	}
	payload, err := decodePayload(wire.Kind, wire.Payload)
	if err != nil {
		return Event{}, err
	}
	event := Event{
		Schema: wire.Schema, SourceSequence: wire.SourceSequence, Flight: wire.Flight,
		Lane: wire.Lane, Epoch: wire.Epoch, ManifestRevision: wire.ManifestRevision,
		ManifestDigest: wire.ManifestDigest, Session: wire.Session, ATCRunway: wire.ATCRunway,
		ExecutionWorkspaceID: wire.ExecutionWorkspaceID, CandidateCommit: wire.CandidateCommit,
		CandidateTree: wire.CandidateTree, Adapter: wire.Adapter, AdapterVersion: wire.AdapterVersion,
		OccurredAt: wire.OccurredAt, Kind: wire.Kind, PayloadSchema: wire.PayloadSchema,
		Payload: payload, PriorEventDigest: wire.PriorEventDigest, PriorRevision: wire.PriorRevision,
		EventDigest: wire.EventDigest,
	}
	if err := validateEvent(event, true); err != nil {
		return Event{}, err
	}
	canonical, err := canonjson.Marshal(event)
	if err != nil {
		return Event{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return Event{}, fmt.Errorf("contextevent: event is not byte-canonical")
	}
	return event, nil
}

// EventChainRoot validates the complete ordered revision array through
// execution-result and returns its canonical digest.
func EventChainRoot(revisions []Revision) (string, error) {
	if err := validateRevisions(revisions); err != nil {
		return "", err
	}
	return canonjson.Digest(revisions)
}

// EncodeEventAck validates and canonically encodes a general acknowledgment.
func EncodeEventAck(ack EventAck) ([]byte, error) {
	if err := validateEventAck(ack); err != nil {
		return nil, err
	}
	return canonjson.Marshal(ack)
}

// DecodeEventAck strictly decodes a canonical general acknowledgment.
func DecodeEventAck(reader io.Reader) (EventAck, error) {
	var ack EventAck
	if err := decodeCanonicalReader(reader, &ack); err != nil {
		return EventAck{}, fmt.Errorf("contextevent: decode event acknowledgment: %w", err)
	}
	if err := validateEventAck(ack); err != nil {
		return EventAck{}, err
	}
	return ack, nil
}

// EncodeReceiptEventAck validates and canonically encodes a receipt event
// acknowledgment.
func EncodeReceiptEventAck(ack ReceiptEventAck) ([]byte, error) {
	if err := validateReceiptEventAck(ack); err != nil {
		return nil, err
	}
	return canonjson.Marshal(ack)
}

// DecodeReceiptEventAck strictly decodes a canonical receipt acknowledgment.
func DecodeReceiptEventAck(reader io.Reader) (ReceiptEventAck, error) {
	var ack ReceiptEventAck
	if err := decodeCanonicalReader(reader, &ack); err != nil {
		return ReceiptEventAck{}, fmt.Errorf("contextevent: decode receipt event acknowledgment: %w", err)
	}
	if err := validateReceiptEventAck(ack); err != nil {
		return ReceiptEventAck{}, err
	}
	return ack, nil
}

func eventDigest(event Event) (string, error) {
	event.EventDigest = ""
	return canonjson.Digest(event)
}

func validateEvent(event Event, requireDigest bool) error {
	if event.Schema != EventSchemaID {
		return fmt.Errorf("contextevent: schema must be %q", EventSchemaID)
	}
	for field, value := range map[string]string{
		"flight": event.Flight, "lane": event.Lane, "epoch": event.Epoch,
		"session": event.Session, "atc_runway": event.ATCRunway,
		"execution_workspace_id": event.ExecutionWorkspaceID,
		"adapter_version":        event.AdapterVersion,
	} {
		if err := requireText(field, value); err != nil {
			return err
		}
	}
	if event.SourceSequence == 0 {
		return fmt.Errorf("contextevent: source_sequence must be positive")
	}
	if err := validateDigest("manifest_digest", event.ManifestDigest); err != nil {
		return err
	}
	if err := validateSHA("candidate_commit", event.CandidateCommit); err != nil {
		return err
	}
	if err := validateSHA("candidate_tree", event.CandidateTree); err != nil {
		return err
	}
	if err := validateAdapter(event.Adapter); err != nil {
		return err
	}
	stamp, err := time.Parse(time.RFC3339Nano, event.OccurredAt)
	if err != nil || stamp.Format(time.RFC3339Nano) != event.OccurredAt || !strings.HasSuffix(event.OccurredAt, "Z") {
		return fmt.Errorf("contextevent: occurred_at must be normalized UTC RFC3339Nano")
	}
	wantSchema, err := PayloadSchema(event.Kind)
	if err != nil {
		return err
	}
	if event.PayloadSchema != wantSchema {
		return fmt.Errorf("contextevent: payload_schema %q does not match kind %q", event.PayloadSchema, event.Kind)
	}
	if err := validatePayload(event.Kind, event.PayloadSchema, event.Payload); err != nil {
		return err
	}
	if err := validateEventPayloadIdentity(event); err != nil {
		return err
	}
	if event.SourceSequence == 1 {
		if event.PriorEventDigest != "" {
			return fmt.Errorf("contextevent: source sequence one forbids prior_event_digest")
		}
		if event.ManifestRevision == 0 {
			if event.PriorRevision != nil {
				return fmt.Errorf("contextevent: manifest revision zero forbids prior_revision")
			}
		} else {
			if event.PriorRevision == nil {
				return fmt.Errorf("contextevent: sequence one of a later revision requires prior_revision")
			}
			if err := validatePriorRevision(*event.PriorRevision, event.ManifestRevision); err != nil {
				return err
			}
		}
	} else {
		if err := validateDigest("prior_event_digest", event.PriorEventDigest); err != nil {
			return err
		}
		if event.PriorRevision != nil {
			return fmt.Errorf("contextevent: only sequence one may carry prior_revision")
		}
	}
	if requireDigest || event.EventDigest != "" {
		if err := validateDigest("event_digest", event.EventDigest); err != nil {
			return err
		}
		want, err := eventDigest(event)
		if err != nil {
			return err
		}
		if event.EventDigest != want {
			return fmt.Errorf("contextevent: event_digest does not match canonical event")
		}
	}
	return nil
}

func validateEventPayloadIdentity(event Event) error {
	switch payload := event.Payload.(type) {
	case *ChildManifestPayload:
		if payload.ParentRevision != event.ManifestRevision || payload.ParentManifestDigest != event.ManifestDigest || payload.ChildRevision != payload.ParentRevision+1 {
			return fmt.Errorf("contextevent: child-manifest payload contradicts envelope revision")
		}
	case *ExecutionResultPayload:
		if payload.ManifestDigest != event.ManifestDigest {
			return fmt.Errorf("contextevent: execution-result manifest_digest contradicts envelope")
		}
	case *AdapterStartPayload:
		if payload.Adapter != event.Adapter || payload.AdapterVersion != event.AdapterVersion || payload.Session != event.Session {
			return fmt.Errorf("contextevent: adapter-start payload contradicts envelope")
		}
	case *AdapterStopPayload:
		if payload.Adapter != event.Adapter || payload.AdapterVersion != event.AdapterVersion || payload.Session != event.Session {
			return fmt.Errorf("contextevent: adapter-stop payload contradicts envelope")
		}
	case *AdapterErrorPayload:
		if payload.Adapter != event.Adapter || payload.AdapterVersion != event.AdapterVersion || payload.Session != event.Session {
			return fmt.Errorf("contextevent: adapter-error payload contradicts envelope")
		}
	}
	return nil
}

func validatePriorRevision(prior PriorRevision, current uint64) error {
	if prior.ManifestRevision+1 != current {
		return fmt.Errorf("contextevent: prior_revision is not the immediate manifest predecessor")
	}
	if err := validateDigest("prior_revision.manifest_digest", prior.ManifestDigest); err != nil {
		return err
	}
	if err := validateDigest("prior_revision.event_root", prior.EventRoot); err != nil {
		return err
	}
	if prior.TerminalSourceSequence == 0 || prior.TerminalGlobalSequence == 0 {
		return fmt.Errorf("contextevent: prior_revision terminal sequences must be positive")
	}
	return nil
}

func validateRevisions(revisions []Revision) error {
	if len(revisions) == 0 {
		return fmt.Errorf("contextevent: revision array must be non-null and nonempty")
	}
	for i, revision := range revisions {
		prefix := fmt.Sprintf("revision_segments[%d]", i)
		if revision.Schema != RevisionSchemaID {
			return fmt.Errorf("contextevent: %s.schema must be %q", prefix, RevisionSchemaID)
		}
		if err := validateDigest(prefix+".manifest_digest", revision.ManifestDigest); err != nil {
			return err
		}
		if err := validateDigest(prefix+".event_root", revision.EventRoot); err != nil {
			return err
		}
		if revision.FirstGlobalSequence == 0 || revision.TerminalGlobalSequence < revision.FirstGlobalSequence || revision.TerminalSourceSequence == 0 {
			return fmt.Errorf("contextevent: %s has incomplete terminal sequence identity", prefix)
		}
		final := i == len(revisions)-1
		if final && revision.TerminalKind != KindExecutionResult {
			return fmt.Errorf("contextevent: final revision must terminate in execution-result")
		}
		if !final && revision.TerminalKind != KindChildManifest {
			return fmt.Errorf("contextevent: non-final revision must terminate in child-manifest")
		}
		if i == 0 {
			if revision.ManifestRevision != 0 {
				return fmt.Errorf("contextevent: revision array must begin at manifest revision zero")
			}
			continue
		}
		prior := revisions[i-1]
		if revision.ManifestRevision != prior.ManifestRevision+1 {
			return fmt.Errorf("contextevent: revision array is not contiguous")
		}
		if revision.FirstGlobalSequence <= prior.TerminalGlobalSequence {
			return fmt.Errorf("contextevent: revision global sequence bridge does not strictly increase")
		}
	}
	return nil
}

func validateEventAck(ack EventAck) error {
	if ack.Schema != AckSchemaID {
		return fmt.Errorf("contextevent: acknowledgment schema must be %q", AckSchemaID)
	}
	return validateAckIdentity(ack.Flight, ack.Lane, ack.Epoch, ack.Session, ack.Kind, ack.SourceSequence, ack.EventDigest, ack.GlobalSequence)
}

func validateReceiptEventAck(ack ReceiptEventAck) error {
	if ack.Schema != ReceiptAckSchemaID {
		return fmt.Errorf("contextevent: receipt acknowledgment schema must be %q", ReceiptAckSchemaID)
	}
	if ack.Kind != KindReceipt {
		return fmt.Errorf("contextevent: receipt acknowledgment kind must be receipt")
	}
	if err := validateAckIdentity(ack.Flight, ack.Lane, ack.Epoch, ack.Session, ack.Kind, ack.SourceSequence, ack.EventDigest, ack.GlobalSequence); err != nil {
		return err
	}
	return validateDigest("receipt_digest", ack.ReceiptDigest)
}

func validateAckIdentity(flight, lane, epoch, session string, kind Kind, source uint64, eventDigest string, global uint64) error {
	for field, value := range map[string]string{"flight": flight, "lane": lane, "epoch": epoch, "session": session} {
		if err := requireText(field, value); err != nil {
			return err
		}
	}
	if !validKind(kind) {
		return fmt.Errorf("contextevent: unknown event kind %q", kind)
	}
	if source == 0 || global == 0 {
		return fmt.Errorf("contextevent: acknowledgment sequences must be positive")
	}
	return validateDigest("event_digest", eventDigest)
}

func decodePayload(kind Kind, raw []byte) (any, error) {
	payload, err := newPayload(kind)
	if err != nil {
		return nil, err
	}
	if err := decodeOne(raw, payload); err != nil {
		return nil, fmt.Errorf("contextevent: decode %s payload: %w", kind, err)
	}
	schema, err := PayloadSchema(kind)
	if err != nil {
		return nil, err
	}
	if err := validatePayload(kind, schema, payload); err != nil {
		return nil, err
	}
	canonical, err := canonjson.Marshal(payload)
	if err != nil {
		return nil, err
	}
	canonical = bytes.TrimSuffix(canonical, []byte("\n"))
	if !bytes.Equal(raw, canonical) {
		return nil, fmt.Errorf("contextevent: %s payload is not byte-canonical", kind)
	}
	return payload, nil
}

func newPayload(kind Kind) (any, error) {
	switch kind {
	case KindFlightPlan:
		return &FlightPlanPayload{}, nil
	case KindInstructionProjection:
		return &InstructionProjectionPayload{}, nil
	case KindChildManifest:
		return &ChildManifestPayload{}, nil
	case KindPrompt:
		return &PromptPayload{}, nil
	case KindProviderMessage:
		return &ProviderMessagePayload{}, nil
	case KindProviderSummary:
		return &ProviderSummaryPayload{}, nil
	case KindToolCall:
		return &ToolCallPayload{}, nil
	case KindToolResult:
		return &ToolResultPayload{}, nil
	case KindRead:
		return &ReadPayload{}, nil
	case KindWrite:
		return &WritePayload{}, nil
	case KindEditDenied:
		return &EditDeniedPayload{}, nil
	case KindContextRequest:
		return &ContextRequestPayload{}, nil
	case KindContextDecision:
		return &ContextDecisionPayload{}, nil
	case KindClaimRequest:
		return &ClaimRequestPayload{}, nil
	case KindClaimDecision:
		return &ClaimDecisionPayload{}, nil
	case KindClaimWait:
		return &ClaimWaitPayload{}, nil
	case KindClaimRelease:
		return &ClaimReleasePayload{}, nil
	case KindCommand:
		return &CommandPayload{}, nil
	case KindTest:
		return &TestPayload{}, nil
	case KindResource:
		return &ResourcePayload{}, nil
	case KindTimeout:
		return &TimeoutPayload{}, nil
	case KindGitStatus:
		return &GitStatusPayload{}, nil
	case KindGitDiff:
		return &GitDiffPayload{}, nil
	case KindGitCommit:
		return &GitCommitPayload{}, nil
	case KindForgeChange:
		return &ForgeChangePayload{}, nil
	case KindGateInput:
		return &GateInputPayload{}, nil
	case KindGateVerdict:
		return &GateVerdictPayload{}, nil
	case KindWitness:
		return &WitnessPayload{}, nil
	case KindFlightPlanDeviation:
		return &FlightPlanDeviationPayload{}, nil
	case KindAdjudication:
		return &AdjudicationPayload{}, nil
	case KindExecutionResult:
		return &ExecutionResultPayload{}, nil
	case KindReceipt:
		return &ReceiptPayload{}, nil
	case KindRetry:
		return &RetryPayload{}, nil
	case KindResume:
		return &ResumePayload{}, nil
	case KindSuspension:
		return &SuspensionPayload{}, nil
	case KindTelemetryGap:
		return &TelemetryGapPayload{}, nil
	case KindAdapterStart:
		return &AdapterStartPayload{}, nil
	case KindAdapterStop:
		return &AdapterStopPayload{}, nil
	case KindAdapterError:
		return &AdapterErrorPayload{}, nil
	default:
		return nil, fmt.Errorf("contextevent: unknown event kind %q", kind)
	}
}

func validatePayload(kind Kind, schema string, payload any) error {
	gotKind, gotSchema, err := payloadIdentity(payload)
	if err != nil {
		return err
	}
	if gotKind != kind {
		return fmt.Errorf("contextevent: typed payload %q does not match event kind %q", gotKind, kind)
	}
	if gotSchema != schema {
		return fmt.Errorf("contextevent: payload schema must be %q", schema)
	}
	return validatePayloadFields(payload)
}

func payloadIdentity(payload any) (Kind, string, error) {
	switch p := payload.(type) {
	case *FlightPlanPayload:
		return KindFlightPlan, p.Schema, nil
	case *InstructionProjectionPayload:
		return KindInstructionProjection, p.Schema, nil
	case *ChildManifestPayload:
		return KindChildManifest, p.Schema, nil
	case *PromptPayload:
		return KindPrompt, p.Schema, nil
	case *ProviderMessagePayload:
		return KindProviderMessage, p.Schema, nil
	case *ProviderSummaryPayload:
		return KindProviderSummary, p.Schema, nil
	case *ToolCallPayload:
		return KindToolCall, p.Schema, nil
	case *ToolResultPayload:
		return KindToolResult, p.Schema, nil
	case *ReadPayload:
		return KindRead, p.Schema, nil
	case *WritePayload:
		return KindWrite, p.Schema, nil
	case *EditDeniedPayload:
		return KindEditDenied, p.Schema, nil
	case *ContextRequestPayload:
		return KindContextRequest, p.Schema, nil
	case *ContextDecisionPayload:
		return KindContextDecision, p.Schema, nil
	case *ClaimRequestPayload:
		return KindClaimRequest, p.Schema, nil
	case *ClaimDecisionPayload:
		return KindClaimDecision, p.Schema, nil
	case *ClaimWaitPayload:
		return KindClaimWait, p.Schema, nil
	case *ClaimReleasePayload:
		return KindClaimRelease, p.Schema, nil
	case *CommandPayload:
		return KindCommand, p.Schema, nil
	case *TestPayload:
		return KindTest, p.Schema, nil
	case *ResourcePayload:
		return KindResource, p.Schema, nil
	case *TimeoutPayload:
		return KindTimeout, p.Schema, nil
	case *GitStatusPayload:
		return KindGitStatus, p.Schema, nil
	case *GitDiffPayload:
		return KindGitDiff, p.Schema, nil
	case *GitCommitPayload:
		return KindGitCommit, p.Schema, nil
	case *ForgeChangePayload:
		return KindForgeChange, p.Schema, nil
	case *GateInputPayload:
		return KindGateInput, p.Schema, nil
	case *GateVerdictPayload:
		return KindGateVerdict, p.Schema, nil
	case *WitnessPayload:
		return KindWitness, p.Schema, nil
	case *FlightPlanDeviationPayload:
		return KindFlightPlanDeviation, p.Schema, nil
	case *AdjudicationPayload:
		return KindAdjudication, p.Schema, nil
	case *ExecutionResultPayload:
		return KindExecutionResult, p.Schema, nil
	case *ReceiptPayload:
		return KindReceipt, p.Schema, nil
	case *RetryPayload:
		return KindRetry, p.Schema, nil
	case *ResumePayload:
		return KindResume, p.Schema, nil
	case *SuspensionPayload:
		return KindSuspension, p.Schema, nil
	case *TelemetryGapPayload:
		return KindTelemetryGap, p.Schema, nil
	case *AdapterStartPayload:
		return KindAdapterStart, p.Schema, nil
	case *AdapterStopPayload:
		return KindAdapterStop, p.Schema, nil
	case *AdapterErrorPayload:
		return KindAdapterError, p.Schema, nil
	default:
		return "", "", fmt.Errorf("contextevent: payload has unregistered type %T", payload)
	}
}

func validatePayloadFields(payload any) error {
	digests := func(values ...string) error {
		for i, value := range values {
			if err := validateDigest(fmt.Sprintf("payload digest[%d]", i), value); err != nil {
				return err
			}
		}
		return nil
	}
	texts := func(values ...string) error {
		for i, value := range values {
			if err := requireText(fmt.Sprintf("payload text[%d]", i), value); err != nil {
				return err
			}
		}
		return nil
	}
	switch p := payload.(type) {
	case *FlightPlanPayload:
		if err := digests(p.ManifestDigest, p.ProjectionDigest, p.DispatchDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *InstructionProjectionPayload:
		if err := digests(p.ManifestDigest, p.ProjectionDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *ChildManifestPayload:
		if err := texts(p.RequestID); err != nil {
			return err
		}
		if p.ChildRevision != p.ParentRevision+1 {
			return fmt.Errorf("contextevent: child revision must immediately follow parent")
		}
		return digests(p.ParentManifestDigest, p.ChildManifestDigest, p.ExpansionDigest)
	case *PromptPayload:
		if err := digests(p.PromptDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *ProviderMessagePayload:
		if err := texts(p.MessageID, p.Role); err != nil {
			return err
		}
		if err := digests(p.MessageDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *ProviderSummaryPayload:
		if err := texts(p.SummaryID); err != nil {
			return err
		}
		if p.Authority != AuthorityAdvisory {
			return fmt.Errorf("contextevent: provider-summary authority must be advisory")
		}
		if err := digests(p.SummaryDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *ToolCallPayload:
		if err := texts(p.CallID, p.ToolName); err != nil {
			return err
		}
		if err := digests(p.ArgumentsDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *ToolResultPayload:
		if err := texts(p.CallID, p.ToolName, p.Status); err != nil {
			return err
		}
		if err := digests(p.OutputDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *ReadPayload:
		if err := texts(p.Resource, p.Classification, p.Decision); err != nil {
			return err
		}
		if err := digests(p.ContentDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *WritePayload:
		if err := texts(p.Path, p.ClaimID); err != nil {
			return err
		}
		return digests(p.BeforeDigest, p.AfterDigest)
	case *EditDeniedPayload:
		if err := texts(p.Operation, p.Path, p.ReasonCode); err != nil {
			return err
		}
		return validateStringSet("witnesses", p.Witnesses)
	case *ContextRequestPayload:
		return texts(p.RequestID, p.Ref, p.Purpose)
	case *ContextDecisionPayload:
		if err := texts(p.RequestID, p.ReasonCode); err != nil {
			return err
		}
		if err := validateVerdict(p.Verdict); err != nil {
			return err
		}
		if err := digests(p.ParentManifestDigest, p.ChildManifestDigest); err != nil {
			return err
		}
		return validateStringSet("witnesses", p.Witnesses)
	case *ClaimRequestPayload:
		if err := texts(p.ClaimID); err != nil {
			return err
		}
		if err := validateStringSet("paths", p.Paths); err != nil {
			return err
		}
		return validateStringSet("shared_resources", p.SharedResources)
	case *ClaimDecisionPayload:
		if err := texts(p.ClaimID, p.ReasonCode); err != nil {
			return err
		}
		if err := validateVerdict(p.Verdict); err != nil {
			return err
		}
		return validateStringSet("witnesses", p.Witnesses)
	case *ClaimWaitPayload:
		if err := texts(p.ClaimID); err != nil {
			return err
		}
		if p.QueuePosition == 0 {
			return fmt.Errorf("contextevent: claim-wait queue_position must be positive")
		}
		return nil
	case *ClaimReleasePayload:
		if err := texts(p.ClaimID); err != nil {
			return err
		}
		if err := validateStringSet("paths", p.Paths); err != nil {
			return err
		}
		return validateStringSet("shared_resources", p.SharedResources)
	case *CommandPayload:
		if err := texts(p.CommandID, p.WorkingDirectory); err != nil {
			return err
		}
		if err := validateOrderedStrings("argv", p.Argv, true); err != nil {
			return err
		}
		return validateStringSet("declared_environment_names", p.DeclaredEnvironmentNames)
	case *TestPayload:
		if err := texts(p.CommandID, p.Suite); err != nil {
			return err
		}
		if err := validateVerdict(p.Verdict); err != nil {
			return err
		}
		if err := digests(p.OutputDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *ResourcePayload:
		return texts(p.OperationID, p.Availability)
	case *TimeoutPayload:
		if err := texts(p.OperationID, p.ReasonCode); err != nil {
			return err
		}
		if p.TimeoutMilliseconds == 0 {
			return fmt.Errorf("contextevent: timeout timeout_milliseconds must be positive")
		}
		return nil
	case *GitStatusPayload:
		if err := texts(p.Branch); err != nil {
			return err
		}
		if err := validateSHA("head", p.Head); err != nil {
			return err
		}
		if err := validateSHA("tree", p.Tree); err != nil {
			return err
		}
		if err := digests(p.EntriesDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *GitDiffPayload:
		if err := validateSHA("base_commit", p.BaseCommit); err != nil {
			return err
		}
		if err := validateSHA("target_commit", p.TargetCommit); err != nil {
			return err
		}
		if err := digests(p.DiffDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *GitCommitPayload:
		if err := validateSHA("commit", p.Commit); err != nil {
			return err
		}
		if err := validateSHA("tree", p.Tree); err != nil {
			return err
		}
		if p.Parents == nil {
			return fmt.Errorf("contextevent: parents must be non-null")
		}
		for i, parent := range p.Parents {
			if err := validateSHA(fmt.Sprintf("parents[%d]", i), parent); err != nil {
				return err
			}
		}
		if err := uniqueStrings("parents", p.Parents); err != nil {
			return err
		}
		return digests(p.MessageDigest)
	case *ForgeChangePayload:
		if err := texts(p.Forge, p.Repository, p.ChangeID, p.Operation, p.SubjectRef); err != nil {
			return err
		}
		if err := validateSHA("candidate_sha", p.CandidateSHA); err != nil {
			return err
		}
		return validatePrincipalResolution(p.PrincipalResolution)
	case *GateInputPayload:
		if err := texts(p.Gate, p.Subject); err != nil {
			return err
		}
		return validateDigestSet("input_digests", p.InputDigests)
	case *GateVerdictPayload:
		if err := texts(p.Gate, p.Subject); err != nil {
			return err
		}
		if err := validateVerdict(p.Verdict); err != nil {
			return err
		}
		return validateStringSet("witnesses", p.Witnesses)
	case *WitnessPayload:
		if err := texts(p.WitnessKind); err != nil {
			return err
		}
		if err := validateAuthority(p.Authority); err != nil {
			return err
		}
		if err := digests(p.WitnessDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *FlightPlanDeviationPayload:
		if err := texts(p.DeviationID, p.RuleID, p.Operation); err != nil {
			return err
		}
		if err := validateVerdict(p.Verdict); err != nil {
			return err
		}
		if err := digests(p.PlanDigest, p.ObservedDigest); err != nil {
			return err
		}
		if err := validateStringSet("witnesses", p.Witnesses); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *AdjudicationPayload:
		if err := texts(p.FindingOrDeviationID, p.Decision); err != nil {
			return err
		}
		if err := validatePrincipalResolution(p.PrincipalResolution); err != nil {
			return err
		}
		if err := digests(p.ReasonDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	case *ExecutionResultPayload:
		if err := validateAuthority(p.Authority); err != nil {
			return err
		}
		if err := validateSHA("input_commit", p.InputCommit); err != nil {
			return err
		}
		if err := validateSHA("output_commit", p.OutputCommit); err != nil {
			return err
		}
		if err := validateSHA("output_tree", p.OutputTree); err != nil {
			return err
		}
		return digests(p.ManifestDigest, p.ResultFactsDigest)
	case *ReceiptPayload:
		if err := validateRole(p.Role); err != nil {
			return err
		}
		if err := validateAuthority(p.Authority); err != nil {
			return err
		}
		if err := digests(p.ReceiptDigest, p.ExecutionEventChainRoot); err != nil {
			return err
		}
		if err := p.Detail.Validate(); err != nil {
			return err
		}
		return nil
	case *RetryPayload:
		if err := texts(p.ReasonCode, p.PriorSession, p.NextSession); err != nil {
			return err
		}
		return digests(p.ContinuityDigest)
	case *ResumePayload:
		if err := texts(p.PriorSession, p.CurrentSession); err != nil {
			return err
		}
		return digests(p.ContinuityDigest, p.ManifestDigest, p.EventChainRoot)
	case *SuspensionPayload:
		if err := texts(p.ReasonCode); err != nil {
			return err
		}
		return digests(p.ContinuityDigest, p.EventChainRoot)
	case *TelemetryGapPayload:
		if err := texts(p.Source, p.ReasonCode); err != nil {
			return err
		}
		if p.FromSequence == 0 || p.ToSequence < p.FromSequence {
			return fmt.Errorf("contextevent: telemetry-gap sequence range is invalid")
		}
		if p.Availability != "unavailable" {
			return fmt.Errorf("contextevent: telemetry-gap availability must be unavailable")
		}
		return nil
	case *AdapterStartPayload:
		if err := p.Adapter.Validate(); err != nil {
			return err
		}
		if err := texts(p.AdapterVersion, p.Session); err != nil {
			return err
		}
		if p.Detail != nil {
			if p.Detail.Mode != DetailInline {
				return fmt.Errorf("contextevent: adapter-start detail must be inline")
			}
			if err := p.Detail.Validate(); err != nil {
				return err
			}
		}
		return digests(p.ProfileDigest, p.WorkspaceRequestDigest)
	case *AdapterStopPayload:
		if err := p.Adapter.Validate(); err != nil {
			return err
		}
		return texts(p.AdapterVersion, p.Session, p.ReasonCode)
	case *AdapterErrorPayload:
		if err := p.Adapter.Validate(); err != nil {
			return err
		}
		if err := texts(p.AdapterVersion, p.Session, p.Operation, p.ReasonCode); err != nil {
			return err
		}
		if err := digests(p.ErrorDigest); err != nil {
			return err
		}
		return p.Detail.Validate()
	default:
		return fmt.Errorf("contextevent: payload has unregistered type %T", payload)
	}
}

func validKind(kind Kind) bool {
	_, err := newPayload(kind)
	return err == nil
}

func validateAdapter(adapter Adapter) error {
	switch adapter {
	case AdapterCodex, AdapterClaude:
		return nil
	default:
		return fmt.Errorf("contextevent: unknown adapter %q", adapter)
	}
}

func validateAuthority(authority Authority) error {
	switch authority {
	case AuthorityAuthoritative, AuthorityAdvisory:
		return nil
	default:
		return fmt.Errorf("contextevent: unknown authority %q", authority)
	}
}

func validateRole(role Role) error {
	switch role {
	case RoleBuilder, RoleReviewer:
		return nil
	default:
		return fmt.Errorf("contextevent: unknown role %q", role)
	}
}

func validateVerdict(verdict countersign.Verdict) error {
	switch verdict {
	case countersign.VerdictProven, countersign.VerdictViolated, countersign.VerdictUnproven:
		return nil
	default:
		return fmt.Errorf("contextevent: unknown verdict %q", verdict)
	}
}

func validatePrincipalResolution(resolution gp.PrincipalResolution) error {
	if err := resolution.State.Validate(); err != nil {
		return fmt.Errorf("contextevent: principal resolution: %w", err)
	}
	if err := resolution.Claim.Validate(); err != nil {
		return fmt.Errorf("contextevent: principal resolution: %w", err)
	}
	derived, err := gp.CanonicalPrincipalID(resolution.Claim.TrustSource, resolution.Claim.Subject)
	if err != nil {
		return err
	}
	if resolution.State == gp.ResolutionAuthenticated {
		if resolution.PrincipalID != derived {
			return fmt.Errorf("contextevent: authenticated principal id does not match claim")
		}
	} else if resolution.PrincipalID != "" {
		return fmt.Errorf("contextevent: non-authenticated resolution carries principal id")
	}
	if len(resolution.Witnesses) == 0 {
		return fmt.Errorf("contextevent: principal resolution witnesses must be non-null and nonempty")
	}
	for i, witness := range resolution.Witnesses {
		if err := requireText(fmt.Sprintf("principal_resolution.witnesses[%d].code", i), witness.Code); err != nil {
			return err
		}
		if err := requireText(fmt.Sprintf("principal_resolution.witnesses[%d].source_id", i), witness.SourceID); err != nil {
			return err
		}
		if witness.EvidenceDigest != "" {
			if err := validateDigest("principal resolution witness evidence_digest", witness.EvidenceDigest); err != nil {
				return err
			}
		}
		if i > 0 && !principalWitnessLess(resolution.Witnesses[i-1], witness) {
			return fmt.Errorf("contextevent: principal resolution witnesses are not strictly ordered")
		}
	}
	return nil
}

func principalWitnessLess(a, b gp.Witness) bool {
	if a.Code != b.Code {
		return a.Code < b.Code
	}
	if a.SourceID != b.SourceID {
		return a.SourceID < b.SourceID
	}
	if a.EvidenceDigest != b.EvidenceDigest {
		return a.EvidenceDigest < b.EvidenceDigest
	}
	return a.Detail < b.Detail
}

func decodeOne(raw []byte, target any) error {
	return artifact.DecodeExactJSON(raw, target)
}

func decodeUniqueJSONValue(raw []byte) (any, error) {
	var preserved json.RawMessage
	if err := artifact.DecodeExactJSON(raw, &preserved); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(preserved))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing data")
		}
		return nil, err
	}
	return value, nil
}

func decodeCanonicalReader(reader io.Reader, target any) error {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if err := decodeOne(raw, target); err != nil {
		return err
	}
	canonical, err := canonjson.Marshal(target)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, canonical) {
		return fmt.Errorf("input is not byte-canonical")
	}
	return nil
}

func requireText(field, value string) error {
	if value == "" || strings.TrimSpace(value) != value {
		return fmt.Errorf("contextevent: %s must be nonempty without surrounding whitespace", field)
	}
	return nil
}

func validateDigest(field, value string) error {
	if !digestRE.MatchString(value) {
		return fmt.Errorf("contextevent: %s must be a canonical sha256 digest", field)
	}
	return nil
}

func validateSHA(field, value string) error {
	if len(value) != 40 && len(value) != 64 {
		return fmt.Errorf("contextevent: %s must be a full 40- or 64-character SHA", field)
	}
	if value != strings.ToLower(value) {
		return fmt.Errorf("contextevent: %s must be lowercase hexadecimal", field)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return fmt.Errorf("contextevent: %s must be hexadecimal: %w", field, err)
	}
	return nil
}

func validateStringSet(field string, values []string) error {
	if values == nil {
		return fmt.Errorf("contextevent: %s must be non-null", field)
	}
	for i, value := range values {
		if err := requireText(fmt.Sprintf("%s[%d]", field, i), value); err != nil {
			return err
		}
		if i > 0 && value <= values[i-1] {
			return fmt.Errorf("contextevent: %s must be sorted and deduplicated", field)
		}
	}
	return nil
}

func validateDigestSet(field string, values []string) error {
	if values == nil {
		return fmt.Errorf("contextevent: %s must be non-null", field)
	}
	for i, value := range values {
		if err := validateDigest(fmt.Sprintf("%s[%d]", field, i), value); err != nil {
			return err
		}
		if i > 0 && value <= values[i-1] {
			return fmt.Errorf("contextevent: %s must be sorted and deduplicated", field)
		}
	}
	return nil
}

func validateOrderedStrings(field string, values []string, nonempty bool) error {
	if values == nil || (nonempty && len(values) == 0) {
		return fmt.Errorf("contextevent: %s must be a non-null%s array", field, map[bool]string{true: " nonempty", false: ""}[nonempty])
	}
	for i, value := range values {
		if err := requireText(fmt.Sprintf("%s[%d]", field, i), value); err != nil {
			return err
		}
	}
	return nil
}

func uniqueStrings(field string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			return fmt.Errorf("contextevent: %s must be deduplicated", field)
		}
		seen[value] = struct{}{}
	}
	return nil
}
