package contextevent

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// DetailMode is the closed redacted-detail representation vocabulary.
type DetailMode string

const (
	DetailInline  DetailMode = "inline"
	DetailSegment DetailMode = "segment"

	MediaTypeJSON            = "application/json"
	RedactionProfileStandard = "verdi.redaction/standard-v1"
	InlineDetailCeiling      = 16384
)

// Detail is the strict already-redacted inline-or-segment union. Resolution,
// redaction, storage, and inline-ceiling selection belong to later services.
type Detail struct {
	Mode             DetailMode      `json:"mode"`
	MediaType        string          `json:"media_type"`
	Digest           string          `json:"digest"`
	RedactionProfile string          `json:"redaction_profile"`
	RedactedJSON     json.RawMessage `json:"redacted_json,omitempty"`
	ByteCount        uint64          `json:"byte_count,omitempty"`
	Reference        string          `json:"reference,omitempty"`
}

// Validate rejects malformed or noncanonical detail union values.
func (d Detail) Validate() error {
	if d.MediaType != MediaTypeJSON {
		return fmt.Errorf("contextevent: detail media_type must be %q", MediaTypeJSON)
	}
	if d.RedactionProfile != RedactionProfileStandard {
		return fmt.Errorf("contextevent: detail redaction_profile must be %q", RedactionProfileStandard)
	}
	if err := validateDigest("detail.digest", d.Digest); err != nil {
		return err
	}
	switch d.Mode {
	case DetailInline:
		if d.RedactedJSON == nil {
			return fmt.Errorf("contextevent: inline detail requires redacted_json")
		}
		if d.ByteCount != 0 || d.Reference != "" {
			return fmt.Errorf("contextevent: inline detail forbids segment fields")
		}
		var value any
		decoder := json.NewDecoder(bytes.NewReader(d.RedactedJSON))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return fmt.Errorf("contextevent: decode inline redacted_json: %w", err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err == nil {
			return fmt.Errorf("contextevent: inline redacted_json has trailing data")
		}
		canonical, err := canonjson.Marshal(value)
		if err != nil {
			return err
		}
		canonical = bytes.TrimSuffix(canonical, []byte("\n"))
		if !bytes.Equal(d.RedactedJSON, canonical) {
			return fmt.Errorf("contextevent: inline redacted_json is not canonical")
		}
		sum := sha256.Sum256(d.RedactedJSON)
		want := fmt.Sprintf("sha256:%x", sum)
		if d.Digest != want {
			return fmt.Errorf("contextevent: inline detail digest does not match redacted_json")
		}
	case DetailSegment:
		if d.RedactedJSON != nil {
			return fmt.Errorf("contextevent: segment detail forbids redacted_json")
		}
		if d.ByteCount == 0 {
			return fmt.Errorf("contextevent: segment detail byte_count must be positive")
		}
		if d.Reference == "" || !utf8.ValidString(d.Reference) {
			return fmt.Errorf("contextevent: segment detail reference must be nonempty UTF-8")
		}
		for _, r := range d.Reference {
			if r < 0x20 || r == 0x7f {
				return fmt.Errorf("contextevent: segment detail reference contains a control character")
			}
		}
	default:
		return fmt.Errorf("contextevent: unknown detail mode %q", d.Mode)
	}
	return nil
}

// PayloadSchema returns the sole payload schema for kind.
func PayloadSchema(kind Kind) (string, error) {
	if !validKind(kind) {
		return "", fmt.Errorf("contextevent: unknown event kind %q", kind)
	}
	return "verdi.context-event-payload/" + string(kind) + "/v1", nil
}

type FlightPlanPayload struct {
	Schema           string `json:"schema"`
	ManifestDigest   string `json:"manifest_digest"`
	ProjectionDigest string `json:"projection_digest"`
	DispatchDigest   string `json:"dispatch_digest"`
	Detail           Detail `json:"detail"`
}

type InstructionProjectionPayload struct {
	Schema           string `json:"schema"`
	ManifestDigest   string `json:"manifest_digest"`
	ProjectionDigest string `json:"projection_digest"`
	Detail           Detail `json:"detail"`
}

type ChildManifestPayload struct {
	Schema               string `json:"schema"`
	RequestID            string `json:"request_id"`
	ParentRevision       uint64 `json:"parent_revision"`
	ParentManifestDigest string `json:"parent_manifest_digest"`
	ChildRevision        uint64 `json:"child_revision"`
	ChildManifestDigest  string `json:"child_manifest_digest"`
	ExpansionDigest      string `json:"expansion_digest"`
}

type PromptPayload struct {
	Schema       string `json:"schema"`
	PromptDigest string `json:"prompt_digest"`
	Detail       Detail `json:"detail"`
}

type ProviderMessagePayload struct {
	Schema        string `json:"schema"`
	MessageID     string `json:"message_id"`
	Role          string `json:"role"`
	MessageDigest string `json:"message_digest"`
	Detail        Detail `json:"detail"`
}

type ProviderSummaryPayload struct {
	Schema        string    `json:"schema"`
	SummaryID     string    `json:"summary_id"`
	SummaryDigest string    `json:"summary_digest"`
	Authority     Authority `json:"authority"`
	Detail        Detail    `json:"detail"`
}

type ToolCallPayload struct {
	Schema          string `json:"schema"`
	CallID          string `json:"call_id"`
	ToolName        string `json:"tool_name"`
	ArgumentsDigest string `json:"arguments_digest"`
	Detail          Detail `json:"detail"`
}

type ToolResultPayload struct {
	Schema       string `json:"schema"`
	CallID       string `json:"call_id"`
	ToolName     string `json:"tool_name"`
	Status       string `json:"status"`
	OutputDigest string `json:"output_digest"`
	Detail       Detail `json:"detail"`
}

type ReadPayload struct {
	Schema         string `json:"schema"`
	Resource       string `json:"resource"`
	Classification string `json:"classification"`
	Decision       string `json:"decision"`
	ContentDigest  string `json:"content_digest"`
	Detail         Detail `json:"detail"`
}

type WritePayload struct {
	Schema       string `json:"schema"`
	Path         string `json:"path"`
	ClaimID      string `json:"claim_id"`
	BeforeDigest string `json:"before_digest"`
	AfterDigest  string `json:"after_digest"`
	ByteCount    uint64 `json:"byte_count"`
}

type EditDeniedPayload struct {
	Schema     string   `json:"schema"`
	Operation  string   `json:"operation"`
	Path       string   `json:"path"`
	ReasonCode string   `json:"reason_code"`
	Witnesses  []string `json:"witnesses"`
}

type ContextRequestPayload struct {
	Schema    string `json:"schema"`
	RequestID string `json:"request_id"`
	Ref       string `json:"ref"`
	Purpose   string `json:"purpose"`
}

type ContextDecisionPayload struct {
	Schema               string              `json:"schema"`
	RequestID            string              `json:"request_id"`
	Verdict              countersign.Verdict `json:"verdict"`
	ReasonCode           string              `json:"reason_code"`
	ParentManifestDigest string              `json:"parent_manifest_digest"`
	ChildManifestDigest  string              `json:"child_manifest_digest"`
	Witnesses            []string            `json:"witnesses"`
}

type ClaimRequestPayload struct {
	Schema          string   `json:"schema"`
	ClaimID         string   `json:"claim_id"`
	Paths           []string `json:"paths"`
	SharedResources []string `json:"shared_resources"`
}

type ClaimDecisionPayload struct {
	Schema     string              `json:"schema"`
	ClaimID    string              `json:"claim_id"`
	Verdict    countersign.Verdict `json:"verdict"`
	ReasonCode string              `json:"reason_code"`
	Witnesses  []string            `json:"witnesses"`
}

type ClaimWaitPayload struct {
	Schema        string `json:"schema"`
	ClaimID       string `json:"claim_id"`
	QueuePosition uint64 `json:"queue_position"`
}

type ClaimReleasePayload struct {
	Schema          string   `json:"schema"`
	ClaimID         string   `json:"claim_id"`
	Paths           []string `json:"paths"`
	SharedResources []string `json:"shared_resources"`
}

type CommandPayload struct {
	Schema                   string   `json:"schema"`
	CommandID                string   `json:"command_id"`
	Argv                     []string `json:"argv"`
	WorkingDirectory         string   `json:"working_directory"`
	DeclaredEnvironmentNames []string `json:"declared_environment_names"`
	TimeoutMilliseconds      uint64   `json:"timeout_milliseconds"`
}

type TestPayload struct {
	Schema               string              `json:"schema"`
	CommandID            string              `json:"command_id"`
	Suite                string              `json:"suite"`
	ExitCode             int                 `json:"exit_code"`
	DurationMilliseconds uint64              `json:"duration_milliseconds"`
	Verdict              countersign.Verdict `json:"verdict"`
	OutputDigest         string              `json:"output_digest"`
	Detail               Detail              `json:"detail"`
}

type ResourcePayload struct {
	Schema          string `json:"schema"`
	OperationID     string `json:"operation_id"`
	CPUMilliseconds uint64 `json:"cpu_milliseconds"`
	PeakRSSBytes    uint64 `json:"peak_rss_bytes"`
	ReadBytes       uint64 `json:"read_bytes"`
	WriteBytes      uint64 `json:"write_bytes"`
	Availability    string `json:"availability"`
}

type TimeoutPayload struct {
	Schema              string `json:"schema"`
	OperationID         string `json:"operation_id"`
	TimeoutMilliseconds uint64 `json:"timeout_milliseconds"`
	ReasonCode          string `json:"reason_code"`
}

type GitStatusPayload struct {
	Schema        string `json:"schema"`
	Head          string `json:"head"`
	Tree          string `json:"tree"`
	Branch        string `json:"branch"`
	Clean         bool   `json:"clean"`
	EntriesDigest string `json:"entries_digest"`
	Detail        Detail `json:"detail"`
}

type GitDiffPayload struct {
	Schema       string `json:"schema"`
	BaseCommit   string `json:"base_commit"`
	TargetCommit string `json:"target_commit"`
	DiffDigest   string `json:"diff_digest"`
	Detail       Detail `json:"detail"`
}

type GitCommitPayload struct {
	Schema        string   `json:"schema"`
	Commit        string   `json:"commit"`
	Tree          string   `json:"tree"`
	Parents       []string `json:"parents"`
	MessageDigest string   `json:"message_digest"`
}

type ForgeChangePayload struct {
	Schema              string                                  `json:"schema"`
	Forge               string                                  `json:"forge"`
	Repository          string                                  `json:"repository"`
	ChangeID            string                                  `json:"change_id"`
	Operation           string                                  `json:"operation"`
	SubjectRef          string                                  `json:"subject_ref"`
	CandidateSHA        string                                  `json:"candidate_sha"`
	PrincipalResolution governanceprincipal.PrincipalResolution `json:"principal_resolution"`
}

type GateInputPayload struct {
	Schema       string   `json:"schema"`
	Gate         string   `json:"gate"`
	Subject      string   `json:"subject"`
	InputDigests []string `json:"input_digests"`
}

type GateVerdictPayload struct {
	Schema    string              `json:"schema"`
	Gate      string              `json:"gate"`
	Subject   string              `json:"subject"`
	Verdict   countersign.Verdict `json:"verdict"`
	Witnesses []string            `json:"witnesses"`
}

type WitnessPayload struct {
	Schema        string    `json:"schema"`
	WitnessKind   string    `json:"witness_kind"`
	WitnessDigest string    `json:"witness_digest"`
	Authority     Authority `json:"authority"`
	Detail        Detail    `json:"detail"`
}

type FlightPlanDeviationPayload struct {
	Schema         string              `json:"schema"`
	DeviationID    string              `json:"deviation_id"`
	PlanDigest     string              `json:"plan_digest"`
	RuleID         string              `json:"rule_id"`
	Operation      string              `json:"operation"`
	ObservedDigest string              `json:"observed_digest"`
	Verdict        countersign.Verdict `json:"verdict"`
	Witnesses      []string            `json:"witnesses"`
	Detail         Detail              `json:"detail"`
}

type AdjudicationPayload struct {
	Schema               string                                  `json:"schema"`
	FindingOrDeviationID string                                  `json:"finding_or_deviation_id"`
	PrincipalResolution  governanceprincipal.PrincipalResolution `json:"principal_resolution"`
	Decision             string                                  `json:"decision"`
	ReasonDigest         string                                  `json:"reason_digest"`
	Detail               Detail                                  `json:"detail"`
}

type ExecutionResultPayload struct {
	Schema            string    `json:"schema"`
	Authority         Authority `json:"authority"`
	InputCommit       string    `json:"input_commit"`
	OutputCommit      string    `json:"output_commit"`
	OutputTree        string    `json:"output_tree"`
	Clean             bool      `json:"clean"`
	ManifestDigest    string    `json:"manifest_digest"`
	ResultFactsDigest string    `json:"result_facts_digest"`
}

type ReceiptPayload struct {
	Schema                  string    `json:"schema"`
	Role                    Role      `json:"role"`
	ReceiptDigest           string    `json:"receipt_digest"`
	Authority               Authority `json:"authority"`
	ExecutionEventChainRoot string    `json:"execution_event_chain_root"`
	Detail                  Detail    `json:"detail"`
}

type RetryPayload struct {
	Schema           string `json:"schema"`
	ReasonCode       string `json:"reason_code"`
	PriorSession     string `json:"prior_session"`
	NextSession      string `json:"next_session"`
	ContinuityDigest string `json:"continuity_digest"`
}

type ResumePayload struct {
	Schema           string `json:"schema"`
	ContinuityDigest string `json:"continuity_digest"`
	PriorSession     string `json:"prior_session"`
	CurrentSession   string `json:"current_session"`
	ManifestDigest   string `json:"manifest_digest"`
	EventChainRoot   string `json:"event_chain_root"`
}

type SuspensionPayload struct {
	Schema           string `json:"schema"`
	ReasonCode       string `json:"reason_code"`
	ContinuityDigest string `json:"continuity_digest"`
	EventChainRoot   string `json:"event_chain_root"`
}

type TelemetryGapPayload struct {
	Schema       string `json:"schema"`
	Source       string `json:"source"`
	FromSequence uint64 `json:"from_sequence"`
	ToSequence   uint64 `json:"to_sequence"`
	ReasonCode   string `json:"reason_code"`
	Availability string `json:"availability"`
}

type AdapterStartPayload struct {
	Schema                 string  `json:"schema"`
	Adapter                Adapter `json:"adapter"`
	AdapterVersion         string  `json:"adapter_version"`
	Session                string  `json:"session"`
	ProfileDigest          string  `json:"profile_digest"`
	WorkspaceRequestDigest string  `json:"workspace_request_digest"`
	Detail                 *Detail `json:"detail,omitempty"`
}

type AdapterStopPayload struct {
	Schema         string  `json:"schema"`
	Adapter        Adapter `json:"adapter"`
	AdapterVersion string  `json:"adapter_version"`
	Session        string  `json:"session"`
	ExitCode       int     `json:"exit_code"`
	ReasonCode     string  `json:"reason_code"`
}

type AdapterErrorPayload struct {
	Schema         string  `json:"schema"`
	Adapter        Adapter `json:"adapter"`
	AdapterVersion string  `json:"adapter_version"`
	Session        string  `json:"session"`
	Operation      string  `json:"operation"`
	ReasonCode     string  `json:"reason_code"`
	ErrorDigest    string  `json:"error_digest"`
	Detail         Detail  `json:"detail"`
}
